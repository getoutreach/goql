package goql

import (
	"strings"
)

// Fields is a type that is intended to be used to allow sparse field sets when rendering by
// specifying the fields within the underlying map. Take the following desired GraphQL operation
// for example:
//
//	query {
//		roleCollection {
//			collection {
//				id
//				name
//				parentRole {
//					id
//				}
//			}
//		}
//	}
//
// The following variable would denote those fields being requested, given that the "keep" tag
// was specified on roleCollection and collection (since they're always necessary in the above
// query):
//
//	f := graphql.Fields{
//		"id": true,
//		"name": true,
//		"parentRole": graphql.Fields{
//			"id": true,
//		},
//	}
//
// Any omitted fields or fields explicitly set to false will not be included in the resulting
// query. If fields is passed as nil, all fields will be rendered on the operation.
type Fields map[string]interface{}

// Union is a function that takes the union (as in the union of two sets) of two Fields types.
// If performance is a worry, it is advantageous to pass the larger of the two Fields types as
// the first parameter (formal parameter x).
func Union(x, y Fields) Fields {
	for k, v := range y {
		// Current key in y is not a key in x, so set it and continue.
		if _, exists := x[k]; !exists {
			x[k] = v
			continue
		}

		// Value for current key in y is Fields value on on x.
		if xInner, xOk := x[k].(Fields); xOk {
			// Only merge the values if they're both Fields, if the
			// current value of the key from y isn't a Fields type,
			// we opt to keep value from the corresponding key in x
			// since it has more information.
			if yInner, yOk := v.(Fields); yOk {
				x[k] = Union(xInner, yInner)
			}
			continue
		}

		// Value for current key in y is not a Fields type value on x, so
		// we can safely overwrite it.
		x[k] = v
	}

	return x
}

// FieldsFromURLQueryParam uses FieldsFromDelimitedList in an opinionated fashion, assuming
// your fields are separated by a comma and the subfields are separated by a period. See
// the documentation for FieldsFromDelimitedList for a more granular description on how
// this process works.
func FieldsFromURLQueryParam(raw string) Fields {
	return FieldsFromDelimitedList(raw, ",", ".")
}

// FieldsFromDelimitedList is meant to be used to transform a URL query parameter in a Fields
// type variable to get the sparse fieldset functionality from an HTTP API.
//
// The fieldDelimiter is the delimiter that separates each individual field entry, and usually
// would be a comma (,). The subFieldDelimiter is the delimiter that separates nested field
// entries, and usually would be a period. Take the following as an example of how the inputs
// of this function correlate to the output:
//
// list: id,name,parent.id,parent.parentOfParent.id
// fieldDelimiter: ,
// subFieldDelimiter: .
// ---
// Output:
//
//	graphql.Fields{
//		"id": true,
//		"name": true,
//		"parent": graphql.Fields{
//			"id": true,
//			"parentOfParent": graphql.Fields{
//				"id": true,
//			},
//		},
//	}
func FieldsFromDelimitedList(list, fieldDelimiter, subFieldDelimiter string) Fields {
	if list == "" {
		return nil
	}

	fields := make(Fields)

	rawFields := strings.Split(list, fieldDelimiter)
	for i := range rawFields {
		addRawFieldToFields(rawFields[i], subFieldDelimiter, fields)
	}

	return fields
}

// addRawFieldToFields recursively adds fields to a given Fields type given a raw field string
// that could have delimited subfields that are delimited by the given delimiter.
func addRawFieldToFields(raw, delimiter string, fields Fields) {
	count := strings.Count(raw, delimiter)
	if count > 0 {
		split := strings.Index(raw, delimiter)
		current := raw[:split] //nolint:gocritic // Why: Its what it needs to be.
		leftover := raw[split+1:]

		if _, ok := fields[current].(Fields); !ok {
			fields[current] = make(Fields)
		}
		addRawFieldToFields(leftover, delimiter, fields[current].(Fields))
	} else {
		fields[raw] = true
	}
}
