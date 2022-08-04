package goql

import (
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// structTag is the name of the struct tag that this package uses to extract extra information
// from to build GraphQL operations.
const structTag = "goql"

// Compiled regular expressions.
var (
	// reName matches a simple field or model name.
	// e.g. id
	reName = regexp.MustCompile(`^\w+$`)

	// reDecl matches a model name with arguments and their types.
	// e.g. getUser(name:$name<String!>,age:$age<Int>)
	reDecl     = regexp.MustCompile(`^(?P<name>\w+)(?P<args>\((?:\w+:\$\w+<\[?\w+!?]?>,?)*\))$`)
	reDeclName = reDecl.SubexpIndex("name")
	reDeclArgs = reDecl.SubexpIndex("args")

	// reParam extracts parameters from a model name with arguments and their types (reDecl).
	// e.g. getUser(name:$name<String!>,age:$age<Int>) -> [name:$name<String!>, age:$age<Int>]
	reParam     = regexp.MustCompile(`(?P<name>\w+):\$(?P<arg>\w+)<(?P<kind>\[?\w+!?]?)>`)
	reParamName = reParam.SubexpIndex("name")
	reParamArg  = reParam.SubexpIndex("arg")
	reParamKind = reParam.SubexpIndex("kind")

	// reDirective matches a skip, include, or alias directive with arguments. It's worth noting
	// here that an alias isn't actually a directive in GraphQL, but it's easiest to deal with it
	// as if it were one here.
	// e.g. @alias(fieldAlias) | @include($includeName) | @skip($skipID)
	reDirective     = regexp.MustCompile(`^@(?P<name>\w+)(?P<arg>\(\$?\w+\))$`)
	reDirectiveName = reDirective.SubexpIndex("name")
	reDirectiveArg  = reDirective.SubexpIndex("arg")
)

// keep tag is used to denote a field that is always kept despite whatever the sparse fieldset
// information says.
const keepTag = "keep"

// token represents arguments and variables used throughout a GraphQL query.
type token struct {
	Kind string
	Name string
	Arg  string
}

// tokenize takes a slice of tokens and returns a string representation of them.
func tokenize(tokens []token) string {
	tn := make([]string, 0, len(tokens))
	for _, t := range tokens {
		tn = append(tn, fmt.Sprintf("%s: $%s", t.Name, t.Arg))
	}
	return strings.Join(tn, ", ")
}

// declaration is a data structure that represents a field or model in a GraphQL
// operation.
type declaration struct {
	Name     string
	Alias    string
	Tokens   []token
	Template string
}

// tokenize is a receiver function of the Declaration type which takes the information
// contained within and writes it to any type that implements the io.Writer interface.
func (d declaration) tokenize(w io.Writer) {
	if d.Alias != "" {
		fmt.Fprintf(w, "%s: ", d.Alias) //nolint:errcheck
	}

	if d.Template != "" {
		fmt.Fprintf(w, "%s(%s)", d.Name, tokenize(d.Tokens)) //nolint:errcheck
		return
	}
	io.WriteString(w, d.Name) //nolint:errcheck
}

// directiveEnum is an alias to a string that has distinct constant values defined
// for it allowing it to act as if it were a classic enum.
type directiveEnum string

// Directive constants using the DirectiveEnum type.
const (
	directiveAlias   = directiveEnum("alias")
	directiveSkip    = directiveEnum("skip")
	directiveInclude = directiveEnum("include")
)

// directive is a data structure that represents a directive for a field or model
// in a GraphQL operation.
type directive struct {
	Type     directiveEnum
	Token    token
	Template string
}

// tokenize is a receiver function of the Directive type which takes the information
// contained within and writes it to any type that implements the io.Writer interface.
func (d *directive) tokenize(w io.Writer) {
	fmt.Fprintf(w, "@%s(if: $%s)", d.Type, d.Token.Arg) //nolint:errcheck
}

// field is a data structure that represents a field or model in a GraphQL query.
type field struct {
	Decl       declaration
	Directives []directive
	Fields     []field

	// Keep, if set to true, tells the marshaling process to ignore whatever is
	// contained in the sparse fieldset information about the current field and
	// to always render it. Keep is automatically set to true if the marshaler
	// detects that the current field is an operation declaration.
	Keep bool
}

// tokens recurses through a field to gather all tokens contained within the root
// field as well as all of it's children fields.
func (f *field) tokens() []token {
	var tokens []token

	// Get the tokens from the declaration and directives of the current token.
	tokens = append(tokens, f.Decl.Tokens...)
	for _, directive := range f.Directives {
		if (directive.Token != token{}) {
			tokens = append(tokens, directive.Token)
		}
	}

	// Recurse through children tokens.
	for _, field := range f.Fields {
		tokens = append(tokens, field.tokens()...)
	}

	return tokens
}

// argsFromTokens takes a slice of tokens, validates that there are not conflicting type
// statements, and returns a slice of strings whose values are in the form of:
// "$<arg>: <Type>" which can be joined by strings.Join(args, ", ") to render the correct
// format to pass to either query(...) or mutation(...) at the top-level of a GraphQL
// operation.
func argsFromTokens(tokens []token) ([]string, error) {
	// len(tokens) might be too big, but it's at least the max size it could be.
	argsMap := make(map[string]string, len(tokens))

	// Make sure we don't duplicate variables if they're used more than once, and if
	// they are used more than once, validate their types are the same.
	for _, token := range tokens {
		if kind, exists := argsMap[token.Arg]; exists {
			if token.Kind != kind {
				return nil, fmt.Errorf("argument $%s cannot have more than one type", token.Arg)
			}
			continue
		}

		argsMap[token.Arg] = token.Kind
	}

	// This slice will contain values in the form of $<arg>: <Type> which can be joined
	// with strings.Join(args, ", ") by the caller to achieve the correct format.
	args := make([]string, 0, len(argsMap))

	for arg, kind := range argsMap {
		args = append(args, fmt.Sprintf("$%s: %s", arg, kind))
	}

	return args, nil
}

// tokenizeWithFields recurses through a field to write all of the information
// contained within the root field as well as all of it's children field to any
// type that implements the io.Writer interface. Unlike tokenize method,
// tokenizeWithFields only writes the data of a field if the declared name of
// the latter exist in the passed fieldset or if the field has the tag `keep`
// switched on.
//
// Returns a bool denoting whether or not the field was written and an error.
func (f *field) tokenizeWithFields(w io.Writer, fields interface{}) (bool, error) { //nolint:funlen
	var write bool

	switch ts := fields.(type) {
	case bool:
		write = ts
		if len(f.Fields) > 0 {
			return false, fmt.Errorf("field %s set to true in sparse fieldset map has children fields, needs submap for children fields", f.Decl.Name) //nolint:lll // Why:long fixed string
		}
	case Fields:
		write = true
		if len(f.Fields) == 0 {
			return false, fmt.Errorf("field %s set to a submap of fields in sparse fieldset map has no children fields, needs to be set to true or false", f.Decl.Name) //nolint:lll // Why:long fixed string
		}
	default:
		// Include case when fields equals nil
		write = false
	}

	if f.Keep {
		write = true
	}

	if !write {
		return false, nil
	}

	f.Decl.tokenize(w)
	for _, directive := range f.Directives {
		io.WriteString(w, " ") //nolint:errcheck
		directive.tokenize(w)
	}

	if len(f.Fields) > 0 {
		io.WriteString(w, " {\n") //nolint:errcheck

		for _, field := range f.Fields {
			var written bool
			var err error

			switch ts := fields.(type) {
			case Fields:
				written, err = field.tokenizeWithFields(w, ts[field.Decl.Name])
			default:
				written, err = field.tokenizeWithFields(w, nil)
			}

			if err != nil {
				return false, err
			}

			if written {
				io.WriteString(w, "\n") //nolint:errcheck
			}
		}
		io.WriteString(w, "}") //nolint:errcheck
	}

	return write, nil
}

// tokenizeAsRoot skips tokenization for the declaration of the receiver field.
// It writes the given declaration name to the writer interface and continues
// the regular tokenization process for the field
func (f *field) tokenizeAsRoot(w io.Writer, declName string, fields Fields) (bool, error) {
	io.WriteString(w, declName) //nolint:errcheck
	return f.tokenize(w, fields)
}

// tokenizeAsLeaf tokenizes the declaration of the receiver field and continues
// the regular tokenization process for the field
func (f *field) tokenizeAsLeaf(w io.Writer, fields Fields) (bool, error) {
	f.Decl.tokenize(w)
	return f.tokenize(w, fields)
}

// tokenize recurses through a field to write all of the information contained
// within the root field as well as all of it's children field to any type that
// implements the io.Writer interface.
//
// Returns a bool denoting whether or not the field was written and an error.
func (f *field) tokenize(w io.Writer, fields Fields) (bool, error) { //nolint:gocyclo
	for _, directive := range f.Directives {
		io.WriteString(w, " ") //nolint:errcheck
		directive.tokenize(w)
	}

	var written bool
	var err error

	if len(f.Fields) > 0 {
		io.WriteString(w, " {\n") //nolint:errcheck

		for _, field := range f.Fields {
			if fields == nil {
				written, err = field.tokenizeAsLeaf(w, nil)
			} else {
				written, err = field.tokenizeWithFields(w, fields)
			}

			if err != nil {
				return false, err
			}

			if written {
				io.WriteString(w, "\n") //nolint:errcheck
			}
		}
		io.WriteString(w, "}") //nolint:errcheck
	}

	return true, nil
}

// splitTag takes a tag and splits it into directives and declarations.
func splitTag(tag string) []string {
	var sb strings.Builder
	var split []string

	var inArgs bool
	for _, r := range tag {
		// This will allow us to ignore commas inside of argument lists.
		if strings.ContainsRune("()", r) {
			inArgs = !inArgs
		}

		// If we encounter a comma and we're not inside an argument list,
		// add the current split value and reset the string builder to
		// start to gather the next.
		if r == ',' && !inArgs {
			split = append(split, sb.String())
			sb.Reset()
			continue
		}
		sb.WriteRune(r)
	}

	// There will be one tag leftover that still needs added.
	split = append(split, sb.String())

	return split
}

// parseTag takes the value of a graphql tag and parses it into various declarations
// and directives.
func parseTag(tag string) (field, error) { //nolint:funlen
	var f field
	var alias string

	for _, item := range splitTag(tag) {
		item = strings.TrimSpace(item)

		switch {
		case item == "":
			continue
		case reName.MatchString(item) && item != keepTag:
			// The explicit check that the string isn't a keep tag is necessary
			// because reName matches the string "keep". This might be a problem?
			f.Decl = declaration{Name: item}
		case reDecl.MatchString(item):
			f.Decl = parseDecl(item)
			f.Keep = true
		case reDirective.MatchString(item):
			dir, err := parseDirective(item)
			if err != nil {
				return field{}, err
			}

			if dir.Type == directiveAlias {
				alias = dir.Template
				continue
			}

			f.Directives = append(f.Directives, dir)
		case item == keepTag:
			f.Keep = true
		default:
			return field{}, fmt.Errorf("failed to parse tag \"%s\"", tag)
		}
	}

	f.Decl.Alias = alias

	// sort directives to check for duplication
	sort.Slice(f.Directives, func(i, j int) bool {
		return f.Directives[i].Type < f.Directives[j].Type
	})

	// check for duplicate directives
	j := 0
	for i := 1; i < len(f.Directives); i++ {
		x, y := &f.Directives[i], f.Directives[j]
		if x.Type == y.Type {
			return field{}, fmt.Errorf("duplicate directive in tag \"%s\"", x.Type)
		}
		j++
	}

	return f, nil
}

// parseDecl takes a declaration retrieved from a graphql struct tag and parses it
// into a Declaration.
func parseDecl(s string) declaration {
	var tokens []token

	matches := reDecl.FindStringSubmatch(s)
	name := matches[reDeclName]

	params := strings.Trim(matches[reDeclArgs], "()")
	paramMatches := reParam.FindAllStringSubmatch(params, -1)
	for _, match := range paramMatches {
		tokens = append(tokens, token{
			Kind: match[reParamKind],
			Name: match[reParamName],
			Arg:  match[reParamArg],
		})
	}

	template := reParam.ReplaceAllStringFunc(params, func(param string) string {
		if i := strings.Index(param, "["); i != -1 {
			return param[:i]
		}
		return param
	})

	return declaration{
		Name:     name,
		Tokens:   tokens,
		Template: template,
	}
}

// parseDirective takes a declaration retrieved from a graphql struct tag and parses it
// into a Directive.
func parseDirective(s string) (directive, error) {
	matches := reDirective.FindStringSubmatch(s)

	dir := directive{
		Type:     directiveEnum(matches[reDirectiveName]),
		Template: strings.Trim(matches[reDirectiveArg], "()"),
	}

	switch dir.Type {
	case directiveAlias:
		// there can't be variables in aliases (they're technically not a directive,
		// it's just easiest to deal with them as if they were one).
	case directiveInclude, directiveSkip:
		if strings.HasPrefix(dir.Template, "$") {
			dir.Token = token{
				Kind: "Boolean!",
				Arg:  dir.Template[1:],
			}
		}
	default:
		return directive{}, fmt.Errorf("unknown directive in tag \"%s\"", dir.Type)
	}

	return dir, nil
}

// node represents any given struct type or it's fields.
type node struct {
	Name string
	Type reflect.Type
	Tag  string
}

// visit defines a function signature used when "visiting" each node in a tree
// of nodes.
type visit func(n *node) error

// structNode ensures a given type is a struct type and resolves it to a node.
func structNode(s interface{}) (node, error) {
	st := deref(reflect.TypeOf(s))

	if st.Kind() != reflect.Struct {
		return node{}, fmt.Errorf("expecting struct type, got %s", st.Kind())
	}

	return node{
		Name: st.Name(),
		Type: st,
	}, nil
}

// listFields takes a reflect.Type parameter that should be a struct type and resolves
// all of it's fields into nodes.
func listFields(st reflect.Type) []node {
	fields := make([]node, 0, st.NumField())
	for i := 0; i < st.NumField(); i++ {
		field := st.Field(i)

		// skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		tag := field.Tag.Get(structTag)
		if tag == "-" {
			continue
		}

		fields = append(fields, node{
			Name: field.Name,
			Type: deref(field.Type),
			Tag:  tag,
		})
	}
	return fields
}

// walker performs the visit function on the passed in node and each of its children,
// recursively.
func walker(n node, visitFn visit) error {
	// Visit the current node.
	if err := visitFn(&n); err != nil {
		return err
	}

	// Tell visit we're done with this node and it's children nodes.
	defer func() {
		// this will never error when nil is passed
		_ = visitFn(nil) //nolint:errcheck
	}()

	switch n.Type.Kind() { //nolint:exhaustive
	case reflect.Struct:
		for _, field := range listFields(n.Type) {
			if err := walker(field, visitFn); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array, reflect.Ptr:
		t := deref(n.Type.Elem())
		n := node{
			Name: t.Name(),
			Type: t,
		}

		if t.Kind() == reflect.Struct {
			for _, field := range listFields(n.Type) {
				if err := walker(field, visitFn); err != nil {
					return err
				}
			}
			break
		}
		if err := walker(n, visitFn); err != nil {
			return err
		}
	default:
	}

	return nil
}

// walk takes a struct type and a Visit function and walks through the entire type
// performing the Visit function on each field.
func walk(s interface{}, visit visit) error {
	n, err := structNode(s)
	if err != nil {
		return err
	}

	return walker(n, visit)
}

// deref dereferences a reflection type if it is a pointer, double pointer, etc.
func deref(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	return t
}

// MarshalQuery takes a variable that must be a struct type and constructs a GraphQL
// operation using it's fields and graphql struct tags that can be used as a GraphQL
// query operation.
func MarshalQuery(q interface{}, fields Fields) (string, error) {
	return marshal(q, "query", fields)
}

// MarshalMutation takes a variable that must be a struct type and constructs a GraphQL
// operation using it's fields and graphql struct tags that can be used as a GraphQL
// mutation operation.
func MarshalMutation(q interface{}, fields Fields) (string, error) {
	return marshal(q, "mutation", fields)
}

// cache stores the resulting tree of types who have already been through the marshaling
// process.
var cache sync.Map

// marshal takes a variable that must be a struct type and constructs a GraphQL operation
// using it's fields and graphql struct tags. The wrapper variable defines what type of
// GraphQL operation will be returned ("query" or "mutation", although this is not
// explicitly checked since this function is only called from within this package).
func marshal(q interface{}, wrapper string, fields Fields) (string, error) { //nolint:funlen
	var operation *field
	rt := reflect.TypeOf(q)

	// Check to see if this type has already been built.
	if cachedOperation, hit := cache.Load(rt); hit {
		// Cache hit, use the tree that was already built.
		operation = cachedOperation.(*field)
	} else {
		// Not in cache, need to build by walking through the type and then store it in the
		// cache for later use.
		var st stack

		// The visit func that gets passed to Walk handles the stack management while walking
		// through the root node and all of it's children to create the declarations, directives,
		// and their tokens which are used to create the GraphQL operation.
		visitFn := func(n *node) error {
			if n != nil {
				f, err := parseTag(n.Tag)
				if err != nil {
					return err
				}

				if f.Decl.Name == "" {
					f.Decl.Name = toLowerCamelCase(n.Name)
				}
				st.push(&f)
			} else {
				// don't pop the root node
				if st.length() == 1 {
					return nil
				}

				// add most recent node to parent
				nf := st.pop()
				st.apply(func(f *field) {
					f.Fields = append(f.Fields, *nf)
				})
			}

			return nil
		}

		// Walk through the given struct.
		if err := walk(q, visitFn); err != nil {
			return "", err
		}

		// The top of the stack at this point will be the top-level field with all of
		// the inner fields as children.
		operation = st.top()

		// Store this built tree for the operation in the cache.
		cache.Store(rt, operation)
	}

	// Get the args from the tokens contained in operation and it's children.
	args, err := argsFromTokens(operation.tokens())
	if err != nil {
		return "", err
	}

	// The top-level declaration will be the name of the struct (q), we don't need that. We
	// need either "query" or "mutation" at the root-level of the operation.
	declName := wrapper

	// If there are arguments, add them to the root-level "query" or "mutation" operation identifier
	// within parenthesis.
	if len(args) > 0 {
		declName = fmt.Sprintf("%s(%s)", declName, strings.Join(args, ", "))
	}

	var b strings.Builder

	// Construct the actual operation from the fields gathered while walking through q's nodes.
	if _, err := operation.tokenizeAsRoot(&b, declName, fields); err != nil {
		return "", err
	}

	return b.String(), nil
}
