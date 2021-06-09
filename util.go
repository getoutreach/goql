package goql

import (
	"strings"
	"unicode"
)

// specialStrings contains keys of strings who will not render to lower camel case
// given the formatting rules of toLowerCamelCase, so the desired output is stored
// in the value of each key.
var specialStrings = map[string]string{
	"ID": "id",
}

// toLowerCamelCase takes a string and transforms it into lower camel case format.
func toLowerCamelCase(in string) string {
	if in == "" {
		return in
	}

	// Look to see if this is a special string where the rules this function employs
	// won't output the desired format.
	if out, found := specialStrings[in]; found {
		return out
	}

	var sb strings.Builder
	sb.Grow(len(in))

	// Write the first letter of the string as lowercase.
	sb.WriteRune(unicode.ToLower(rune(in[0])))

	// Flag to keep track of whether or not the next variable should be capitalized.
	var capNextRune bool

	// Iterate through the rest of the string.
	for _, r := range in[1:] {
		if !unicode.IsLetter(r) {
			if unicode.IsSpace(r) || strings.ContainsRune("-_.", r) {
				capNextRune = true
				continue // Skip spaces, -, _, and . and capitalize the next letter.
			}
		}

		if capNextRune {
			sb.WriteRune(unicode.ToUpper(r))
			capNextRune = false
			continue
		}
		sb.WriteRune(r)
	}

	return sb.String()
}

// stack is a type that contains fields and receiver methods that implement the stack
// data structure. This data structure is used in the marshaling process.
type stack struct {
	items []*field
}

// length returns the length of the stack.
func (s stack) length() int {
	return len(s.items)
}

// push pushes a value onto the stack.
func (s *stack) push(f *field) { //nolint:gocritic
	s.items = append(s.items, f)
}

// pop pops a value off of the top of the stack.
func (s *stack) pop() *field {
	l := len(s.items)
	front, items := s.items[l-1], s.items[:l-1]
	s.items = items
	return front
}

// top returns the element on top of the stack.
func (s stack) top() *field {
	return s.items[len(s.items)-1]
}

// apply allows changes to the element on top of the stack.
func (s stack) apply(fn func(f *field)) {
	if len(s.items) > 0 {
		fn(s.items[len(s.items)-1])
	}
}
