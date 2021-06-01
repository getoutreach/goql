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

// Token represents arguments and variables used throughout a GraphQL query.
type Token struct {
	Kind string
	Name string
	Arg  string
}

// tokenize takes a slice of tokens and returns a string representation of them.
func tokenize(tokens []Token) string {
	tn := make([]string, 0, len(tokens))
	for _, token := range tokens {
		tn = append(tn, fmt.Sprintf("%s: $%s", token.Name, token.Arg))
	}
	return strings.Join(tn, ", ")
}

// Declaration is a data structure that represents a field or model in a GraphQL
// operation.
type Declaration struct {
	Name     string
	Alias    string
	Tokens   []Token
	Template string
}

// Tokenize is a receiver function of the Declaration type which takes the information
// contained within and writes it to any type that implements the io.Writer interface.
func (d Declaration) Tokenize(w io.Writer) {
	if d.Alias != "" {
		fmt.Fprintf(w, "%s: ", d.Alias) //nolint:errcheck
	}

	if d.Template != "" {
		fmt.Fprintf(w, "%s(%s)", d.Name, tokenize(d.Tokens)) //nolint:errcheck
		return
	}
	io.WriteString(w, d.Name) //nolint:errcheck
}

// DirectiveEnum is an alias to a string that has distinct constant values defined
// for it allowing it to act as if it were a classic enum.
type DirectiveEnum string

// Directive constants using the DirectiveEnum type.
const (
	DirectiveAlias   = DirectiveEnum("alias")
	DirectiveSkip    = DirectiveEnum("skip")
	DirectiveInclude = DirectiveEnum("include")
)

// Directive is a data structure that represents a directive for a field or model
// in a GraphQL operation.
type Directive struct {
	Type     DirectiveEnum
	Token    Token
	Template string
}

// Tokenize is a receiver function of the Directive type which takes the information
// contained within and writes it to any type that implements the io.Writer interface.
func (d *Directive) Tokenize(w io.Writer) {
	fmt.Fprintf(w, "@%s(if: $%s)", d.Type, d.Token.Arg) //nolint:errcheck
}

// Field is a data structure that represents a field or model in a GraphQL query.
type Field struct {
	Decl       Declaration
	Directives []Directive
	Fields     []Field

	// Keep, if set to true, tells the marshaling process to ignore whatever is
	// contained in the sparse fieldset information about the current field and
	// to always render it. Keep is automatically set to true if the marshaler
	// detects that the current field is an operation declaration.
	Keep bool
}

// Tokens recurses through a field to gather all tokens contained within the root
// field as well as all of it's children fields.
func (f *Field) Tokens() []Token {
	var tokens []Token

	// Get the tokens from the declaration and directives of the current token.
	tokens = append(tokens, f.Decl.Tokens...)
	for _, directive := range f.Directives {
		if (directive.Token != Token{}) {
			tokens = append(tokens, directive.Token)
		}
	}

	// Recurse through children tokens.
	for _, field := range f.Fields {
		tokens = append(tokens, field.Tokens()...)
	}

	return tokens
}

// argsFromTokens takes a slice of tokens, validates that there are not conflicting type
// statements, and returns a slice of strings whose values are in the form of:
// "$<arg>: <Type>" which can be joined by strings.Join(args, ", ") to render the correct
// format to pass to either query(...) or mutation(...) at the top-level of a GraphQL
// operation.
func argsFromTokens(tokens []Token) ([]string, error) {
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

// Tokenize recurses through a field to write all of the information contained
// within the root field as well as all of it's children field to any type that
// implements the io.Writer interface.
//
// Returns a bool denoting whether or not the field was written and an error.
func (f *Field) Tokenize(w io.Writer, fields Fields) (bool, error) { //nolint:gocyclo
	var write bool

	if f.Keep || fields == nil {
		write = true
	} else if desired, exists := fields[f.Decl.Name]; exists {
		switch ts := desired.(type) {
		case bool:
			if ts {
				write = true

				if len(f.Fields) > 0 {
					return false, fmt.Errorf("field %s set to true in sparse fieldset map has children fields, needs submap for children fields", f.Decl.Name)
				}
			}
		case Fields:
			if len(f.Fields) == 0 {
				return false, fmt.Errorf("field %s set to a submap of fields in sparse fieldset map has no children fields, needs to be set to true or false", f.Decl.Name)
			}

			write = true
			fields = ts
		default:
			write = false
		}
	}

	if write {
		f.Decl.Tokenize(w)
		for _, directive := range f.Directives {
			io.WriteString(w, " ") //nolint:errcheck
			directive.Tokenize(w)
		}

		if len(f.Fields) > 0 {
			io.WriteString(w, " {\n") //nolint:errcheck
			for _, field := range f.Fields {
				written, err := field.Tokenize(w, fields)
				if err != nil {
					return false, err
				}

				if written {
					io.WriteString(w, "\n") //nolint:errcheck
				}
			}
			io.WriteString(w, "}") //nolint:errcheck
		}
	}

	return write, nil
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
func parseTag(tag string) (Field, error) { //nolint:funlen
	var field Field
	var alias string

	for _, item := range splitTag(tag) {
		item = strings.TrimSpace(item)

		switch {
		case item == "":
			continue
		case reName.MatchString(item) && item != keepTag:
			// The explicit check that the string isn't a keep tag is necessary
			// because reName matches the string "keep". This might be a problem?
			field.Decl = Declaration{Name: item}
		case reDecl.MatchString(item):
			field.Decl = parseDecl(item)
			field.Keep = true
		case reDirective.MatchString(item):
			dir, err := parseDirective(item)
			if err != nil {
				return Field{}, err
			}

			if dir.Type == DirectiveAlias {
				alias = dir.Template
				continue
			}

			field.Directives = append(field.Directives, dir)
		case item == keepTag:
			field.Keep = true
		default:
			return Field{}, fmt.Errorf("failed to parse tag \"%s\"", tag)
		}
	}

	field.Decl.Alias = alias

	// sort directives to check for duplication
	sort.Slice(field.Directives, func(i, j int) bool {
		return field.Directives[i].Type < field.Directives[j].Type
	})

	// check for duplicate directives
	j := 0
	for i := 1; i < len(field.Directives); i++ {
		x, y := &field.Directives[i], field.Directives[j]
		if x.Type == y.Type {
			return Field{}, fmt.Errorf("duplicate directive in tag \"%s\"", x.Type)
		}
		j++
	}

	return field, nil
}

// parseDecl takes a declaration retrieved from a graphql struct tag and parses it
// into a Declaration.
func parseDecl(s string) Declaration {
	var tokens []Token

	matches := reDecl.FindStringSubmatch(s)
	name := matches[reDeclName]

	params := strings.Trim(matches[reDeclArgs], "()")
	paramMatches := reParam.FindAllStringSubmatch(params, -1)
	for _, match := range paramMatches {
		tokens = append(tokens, Token{
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

	return Declaration{
		Name:     name,
		Tokens:   tokens,
		Template: template,
	}
}

// parseDirective takes a declaration retrieved from a graphql struct tag and parses it
// into a Directive.
func parseDirective(s string) (Directive, error) {
	matches := reDirective.FindStringSubmatch(s)

	dir := Directive{
		Type:     DirectiveEnum(matches[reDirectiveName]),
		Template: strings.Trim(matches[reDirectiveArg], "()"),
	}

	switch dir.Type {
	case DirectiveAlias:
		// there can't be variables in aliases (they're technically not a directive,
		// it's just easiest to deal with them as if they were one).
	case DirectiveInclude, DirectiveSkip:
		if strings.HasPrefix(dir.Template, "$") {
			dir.Token = Token{
				Kind: "Boolean!",
				Arg:  dir.Template[1:],
			}
		}
	default:
		return Directive{}, fmt.Errorf("unknown directive in tag \"%s\"", dir.Type)
	}

	return dir, nil
}

// Node represents any given struct type or it's fields.
type Node struct {
	Name string
	Type reflect.Type
	Tag  string
}

// Visit defines a function signature used when "visiting" each node in a tree
// of nodes.
type Visit func(node *Node) error

// structNode ensures a given type is a struct type and resolves it to a node.
func structNode(s interface{}) (Node, error) {
	st := deref(reflect.TypeOf(s))

	if st.Kind() != reflect.Struct {
		return Node{}, fmt.Errorf("expecting struct type, got %s", st.Kind())
	}

	return Node{
		Name: st.Name(),
		Type: st,
	}, nil
}

// listFields takes a reflect.Type parameter that should be a struct type and resolves
// all of it's fields into nodes.
func listFields(st reflect.Type) []Node {
	fields := make([]Node, 0, st.NumField())
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

		fields = append(fields, Node{
			Name: field.Name,
			Type: deref(field.Type),
			Tag:  tag,
		})
	}
	return fields
}

// walk performs the visit function on the passed in node and each of its children,
// recursively.
func walk(node Node, visit Visit) error {
	// Visit the current node.
	if err := visit(&node); err != nil {
		return err
	}

	// Tell visit we're done with this node and it's children nodes.
	defer func() {
		// this will never error when nil is passed
		_ = visit(nil) //nolint:errcheck
	}()

	switch node.Type.Kind() { //nolint:exhaustive
	case reflect.Struct:
		for _, field := range listFields(node.Type) {
			if err := walk(field, visit); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array, reflect.Ptr:
		t := deref(node.Type.Elem())
		n := Node{
			Name: t.Name(),
			Type: t,
		}

		if t.Kind() == reflect.Struct {
			for _, field := range listFields(n.Type) {
				if err := walk(field, visit); err != nil {
					return err
				}
			}
			break
		}
		if err := walk(n, visit); err != nil {
			return err
		}
	default:
	}

	return nil
}

// Walk takes a struct type and a Visit function and walks through the entire type
// performing the Visit function on each field.
func Walk(s interface{}, visit Visit) error {
	n, err := structNode(s)
	if err != nil {
		return err
	}

	return walk(n, visit)
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
	var operation *Field
	rt := reflect.TypeOf(q)

	// Check to see if this type has already been built.
	if cachedOperation, hit := cache.Load(rt); hit {
		// Cache hit, use the tree that was already built.
		operation = cachedOperation.(*Field)
	} else {
		// Not in cache, need to build by walking through the type and then store it in the
		// cache for later use.
		var st stack

		// The visit func that gets passed to Walk handles the stack management while walking
		// through the root node and all of it's children to create the declarations, directives,
		// and their tokens which are used to create the GraphQL operation.
		visit := func(node *Node) error {
			if node != nil {
				field, err := parseTag(node.Tag)
				if err != nil {
					return err
				}

				if field.Decl.Name == "" {
					field.Decl.Name = toLowerCamelCase(node.Name)
				}
				st.push(&field)
			} else {
				// don't pop the root node
				if st.length() == 1 {
					return nil
				}

				// add most recent node to parent
				field := st.pop()
				st.apply(func(f *Field) {
					f.Fields = append(f.Fields, *field)
				})
			}

			return nil
		}

		// Walk through the given struct.
		if err := Walk(q, visit); err != nil {
			return "", err
		}

		// The top of the stack at this point will be the top-level field with all of
		// the inner fields as children.
		operation = st.top()

		// Store this built tree for the operation in the cache.
		cache.Store(rt, operation)
	}

	// Get the args from the tokens contained in operation and it's children.
	args, err := argsFromTokens(operation.Tokens())
	if err != nil {
		return "", err
	}

	// The top-level declaration will be the name of the struct (q), we don't need that. We
	// need either "query" or "mutation" at the root-level of the operation.
	operation.Decl.Name = wrapper

	// Explicitly set the root node to keep because we need it to build the rest of the query,
	// regardless of the sparse fieldset instructions passed via the fields parameter.
	operation.Keep = true

	// If there are arguments, add them to the root-level "query" or "mutation" operation identifier
	// within parenthesis.
	if len(args) > 0 {
		operation.Decl.Name = fmt.Sprintf("%s(%s)", operation.Decl.Name, strings.Join(args, ", "))
	}

	var b strings.Builder

	// Construct the actual operation from the fields gathered while walking through q's nodes.
	if _, err := operation.Tokenize(&b, fields); err != nil {
		return "", err
	}

	return b.String(), nil
}