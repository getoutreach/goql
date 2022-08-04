package goql

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// computeLevenshteinDistance computes the levenshtein distance between the two
// strings passed as an argument. The return value is the levenshtein distance.
func computeLevenshteinDistance(a, b string) int {
	// Custom local min function, since math.Min takes float64
	min := func(a, b uint16) uint16 {
		if a < b {
			return a
		}
		return b
	}

	// If a is empty, the distance is the length of b.
	if a == "" {
		return utf8.RuneCountInString(b)
	}

	// If b is empty, the distance is the length of a.
	if b == "" {
		return utf8.RuneCountInString(a)
	}

	// If the two strings are equal there is no need to computer the distance, it will
	// be 0.
	if a == b {
		return 0
	}

	s1 := []rune(a)
	s2 := []rune(b)

	// Swap to save on memory complexity: O(min(a,b)) instead of O(a)
	if len(s1) > len(s2) {
		s1, s2 = s2, s1
	}
	lenS1 := len(s1)
	lenS2 := len(s2)

	// init the row
	x := make([]uint16, lenS1+1)

	// Start from 1 because index 0 is already 0.
	for i := 1; i < len(x); i++ {
		x[i] = uint16(i)
	}

	// Make a dummy bounds check to prevent the 2 bounds check down below.
	// The one inside the loop is particularly costly.
	_ = x[lenS1]

	// Fill in the rest
	for i := 1; i <= lenS2; i++ {
		prev := uint16(i)
		for j := 1; j <= lenS1; j++ {
			current := x[j-1] // match
			if s2[i-1] != s1[j-1] {
				current = min(min(x[j-1]+1, prev+1), x[j]+1)
			}
			x[j-1] = prev
			prev = current
		}
		x[lenS1] = prev
	}

	return int(x[lenS1])
}

// percentageMatch uses the Levenshtein distance to compute a percentage match
// between two strings.
func percentageMatch(expected, actual string) float64 {
	dist := computeLevenshteinDistance(expected, actual)
	return 1 - float64(dist/len(expected))
}

// minPercentageMatch is the minimum percentage that two output operations from
// being marshaled can match without resulting in a test error. The reason this
// is used is because we can't always count on the variables being set in the
// same order on the output string operations. If the variables are ordered
// differently, the operations are fundamentally the same, but comparison of the
// two strings will render a false result.
const minPercentageMatch = 0.95

// TestMarshalQuery tests the MarshalQuery function.
func TestMarshalQuery(t *testing.T) {
	tt := []struct {
		Name           string
		Input          interface{}
		Fields         Fields
		ExpectedOutput string // IfExpectedOutput == "", it implies an error.
	}{
		{
			Name: "Simple",
			Input: struct {
				TestQuery struct {
					FieldOne string
					FieldTwo string
				}
			}{},
			Fields: nil,
			ExpectedOutput: `query {
testQuery {
fieldOne
fieldTwo
}
}`,
		},
		{
			Name: "WithVariables",
			Input: struct {
				TestQuery struct {
					FieldOne string
					FieldTwo string
				} `goql:"testQuery(id:$id<ID!>)"`
			}{},
			Fields: nil,
			ExpectedOutput: `query($id: ID!) {
testQuery(id: $id) {
fieldOne
fieldTwo
}
}`,
		},
		{
			Name: "WithArrayVariables",
			Input: struct {
				TestQuery struct {
					FieldOne string
					FieldTwo string
				} `goql:"testQuery(id:$id<ID!>,list:$list<[List!]>)"`
			}{},
			Fields: nil,
			ExpectedOutput: `query($id: ID!, $list: [List!]) {
testQuery(id: $id, list: $list) {
fieldOne
fieldTwo
}
}`,
		},
		{
			Name: "WithNameOverride",
			Input: struct {
				TestQuery struct {
					FieldOne string `goql:"FieldOneOverride"`
					FieldTwo string
				}
			}{},
			Fields: nil,
			ExpectedOutput: `query {
testQuery {
FieldOneOverride
fieldTwo
}
}`,
		},
		{
			Name: "WithAlias",
			Input: struct {
				TestQuery struct {
					FieldOne string `goql:"@alias(fieldOneAlias)"`
					FieldTwo string
				}
			}{},
			Fields: nil,
			ExpectedOutput: `query {
testQuery {
fieldOneAlias: fieldOne
fieldTwo
}
}`,
		},
		{
			Name: "WithSkipDirective",
			Input: struct {
				TestQuery struct {
					FieldOne string
					FieldTwo string `goql:"@skip($ifCondition)"`
				}
			}{},
			Fields: nil,
			ExpectedOutput: `query($ifCondition: Boolean!) {
testQuery {
fieldOne
fieldTwo @skip(if: $ifCondition)
}
}`,
		},
		{
			Name: "WithIncludeDirective",
			Input: struct {
				TestQuery struct {
					FieldOne string
					FieldTwo string `goql:"@include($ifCondition)"`
				}
			}{},
			Fields: nil,
			ExpectedOutput: `query($ifCondition: Boolean!) {
testQuery {
fieldOne
fieldTwo @include(if: $ifCondition)
}
}`,
		},
		{
			Name: "SimpleWithSparseFieldset",
			Input: struct {
				TestQuery struct {
					FieldOne string
					FieldTwo string
				} `goql:"keep"`
			}{},
			Fields: Fields{
				"fieldOne": true,
			},
			ExpectedOutput: `query {
testQuery {
fieldOne
}
}`,
		},
		{
			Name: "Complex",
			Input: struct {
				TestQuery struct {
					FieldOne string `goql:"@alias(fooBar)"`
					FieldTwo string `goql:"@skip($ifCondition)"`
				} `goql:"testQuery(id:$id<ID!>)"`
			}{},
			Fields: nil,
			ExpectedOutput: `query($id: ID!, $ifCondition: Boolean!) {
testQuery(id: $id) {
fooBar: fieldOne
fieldTwo @skip(if: $ifCondition)
}
}`,
		},
		{
			Name: "ComplexWithSparseFieldset",
			Input: struct {
				TestQuery struct {
					FieldOne string `goql:"@alias(fooBar)"`
					FieldTwo string `goql:"@skip($ifCondition)"`
				} `goql:"testQuery(id:$id<ID!>)"`
			}{},
			Fields: Fields{
				"fieldTwo": true,
			},
			ExpectedOutput: `query($id: ID!, $ifCondition: Boolean!) {
testQuery(id: $id) {
fieldTwo @skip(if: $ifCondition)
}
}`,
		},
		{
			Name: "WithNestedStruct",
			Input: struct {
				TestQuery struct {
					FieldOne    string
					FieldTwo    string
					NestedField struct {
						NestedFieldOne string
						NestedFieldTwo string
					}
				} `goql:"testQuery(id:$id<ID!>)"`
			}{},
			Fields: nil,
			ExpectedOutput: `query($id: ID!) {
testQuery(id: $id) {
fieldOne
fieldTwo
nestedField {
nestedFieldOne
nestedFieldTwo
}
}
}`,
		},
		{
			Name: "WithNestedStructAndSparseFieldset",
			Input: struct {
				TestQuery struct {
					FieldOne    string
					FieldTwo    string
					FieldThree  string
					NestedField struct {
						NestedFieldOne   string
						NestedFieldTwo   string
						NestedFieldThree string
					}
				} `goql:"testQuery(id:$id<ID!>)"`
			}{},
			Fields: Fields{
				"fieldOne": true,
				"fieldTwo": true,
				"nestedField": Fields{
					"nestedFieldOne": true,
				},
			},
			ExpectedOutput: `query($id: ID!) {
testQuery(id: $id) {
fieldOne
fieldTwo
nestedField {
nestedFieldOne
}
}
}`,
		},
		{
			Name: "WithNestedStructAndSparseFieldsetAndKeepTag",
			Input: struct {
				TestQuery struct {
					FieldOne    string
					FieldTwo    string
					NestedField struct {
						NestedFieldOne   string `goql:"keep"`
						NestedFieldTwo   string
						NestedFieldThree string
					} `goql:"keep"`
				} `goql:"testQuery(id:$id<ID!>)"`
			}{},
			Fields: Fields{
				"fieldOne": true,
				"fieldTwo": true,
			},
			ExpectedOutput: `query($id: ID!) {
testQuery(id: $id) {
fieldOne
fieldTwo
nestedField {
nestedFieldOne
}
}
}`,
		},
		{
			Name: "WithNestedStructAndSparseFieldsetAndKeepTagWithNestedFieldsIncluded",
			Input: struct {
				TestQuery struct {
					FieldOne    string
					FieldTwo    string
					FieldThree  string
					NestedField struct {
						FieldOne string
						FieldTwo string
						FieldXYZ string
						FieldABC string `goql:"keep"`
					} `goql:"keep"`
				} `goql:"testQuery(id:$id<ID!>)"`
			}{},
			Fields: Fields{
				"fieldOne": true,
				"fieldTwo": true,
				"nestedField": Fields{
					"fieldXYZ": true,
				},
			},
			ExpectedOutput: `query($id: ID!) {
testQuery(id: $id) {
fieldOne
fieldTwo
nestedField {
fieldXYZ
fieldABC
}
}
}`,
		},
	}

	for _, test := range tt {
		test := test

		fn := func(t *testing.T) {
			t.Parallel()

			actualOutput, err := MarshalQuery(test.Input, test.Fields)
			if err != nil {
				if test.ExpectedOutput == "" {
					// The error was expected, return without reporting anything.
					return
				}

				t.Fatalf("error marshaling query: %v", err)
			}

			trimmedExpectedOutput, trimmedActualOutput := strings.TrimSpace(test.ExpectedOutput), strings.TrimSpace(actualOutput)

			if e, a := len(trimmedExpectedOutput), len(trimmedActualOutput); e != a {
				t.Errorf("expected length of output to be %d, got %d", e, a)
			}

			percentMatch := percentageMatch(trimmedExpectedOutput, trimmedActualOutput)
			if percentMatch < minPercentageMatch {
				t.Errorf("expected percentage match to be %f, got %f", minPercentageMatch, percentMatch)
			}
		}
		t.Run(test.Name, fn)
	}
}

// TestMarshalMutation tests the MarshalMutation function.
func TestMarshalMutation(t *testing.T) {
	tt := []struct {
		Name           string
		Input          interface{}
		Fields         Fields
		ExpectedOutput string // IfExpectedOutput == "", it implies an error.
	}{
		{
			Name: "Simple",
			Input: struct {
				TestMutation struct {
					FieldOne string
					FieldTwo string
				}
			}{},
			Fields: nil,
			ExpectedOutput: `mutation {
testMutation {
fieldOne
fieldTwo
}
}`,
		},
		{
			Name: "WithVariables",
			Input: struct {
				TestMutation struct {
					FieldOne string
					FieldTwo string
				} `goql:"testMutation(id:$id<ID!>)"`
			}{},
			Fields: nil,
			ExpectedOutput: `mutation($id: ID!) {
testMutation(id: $id) {
fieldOne
fieldTwo
}
}`,
		},
		{
			Name: "WithNameOverride",
			Input: struct {
				TestMutation struct {
					FieldOne string `goql:"FieldOneOverride"`
					FieldTwo string
				}
			}{},
			Fields: nil,
			ExpectedOutput: `mutation {
testMutation {
FieldOneOverride
fieldTwo
}
}`,
		},
		{
			Name: "WithAlias",
			Input: struct {
				TestMutation struct {
					FieldOne string `goql:"@alias(fieldOneAlias)"`
					FieldTwo string
				}
			}{},
			Fields: nil,
			ExpectedOutput: `mutation {
testMutation {
fieldOneAlias: fieldOne
fieldTwo
}
}`,
		},
		{
			Name: "WithSkipDirective",
			Input: struct {
				TestMutation struct {
					FieldOne string
					FieldTwo string `goql:"@skip($ifCondition)"`
				}
			}{},
			Fields: nil,
			ExpectedOutput: `mutation($ifCondition: Boolean!) {
testMutation {
fieldOne
fieldTwo @skip(if: $ifCondition)
}
}`,
		},
		{
			Name: "WithIncludeDirective",
			Input: struct {
				TestMutation struct {
					FieldOne string
					FieldTwo string `goql:"@include($ifCondition)"`
				}
			}{},
			Fields: nil,
			ExpectedOutput: `mutation($ifCondition: Boolean!) {
testMutation {
fieldOne
fieldTwo @include(if: $ifCondition)
}
}`,
		},
		{
			Name: "SimpleWithSparseFieldset",
			Input: struct {
				TestMutation struct {
					FieldOne string
					FieldTwo string
				} `goql:"keep"`
			}{},
			Fields: Fields{
				"fieldOne": true,
			},
			ExpectedOutput: `mutation {
testMutation {
fieldOne
}
}`,
		},
		{
			Name: "Complex",
			Input: struct {
				TestMutation struct {
					FieldOne string `goql:"@alias(fooBar)"`
					FieldTwo string `goql:"@skip($ifCondition)"`
				} `goql:"testMutation(id:$id<ID!>)"`
			}{},
			Fields: nil,
			ExpectedOutput: `mutation($id: ID!, $ifCondition: Boolean!) {
testMutation(id: $id) {
fooBar: fieldOne
fieldTwo @skip(if: $ifCondition)
}
}`,
		},
		{
			Name: "ComplexWithSparseFieldset",
			Input: struct {
				TestMutation struct {
					FieldOne string `goql:"@alias(fooBar)"`
					FieldTwo string `goql:"@skip($ifCondition)"`
				} `goql:"testMutation(id:$id<ID!>)"`
			}{},
			Fields: Fields{
				"fieldTwo": true,
			},
			ExpectedOutput: `mutation($id: ID!, $ifCondition: Boolean!) {
testMutation(id: $id) {
fieldTwo @skip(if: $ifCondition)
}
}`,
		},
		{
			Name: "WithNestedStruct",
			Input: struct {
				TestMutation struct {
					FieldOne    string
					FieldTwo    string
					NestedField struct {
						NestedFieldOne string
						NestedFieldTwo string
					}
				} `goql:"testMutation(id:$id<ID!>)"`
			}{},
			Fields: nil,
			ExpectedOutput: `mutation($id: ID!) {
testMutation(id: $id) {
fieldOne
fieldTwo
nestedField {
nestedFieldOne
nestedFieldTwo
}
}
}`,
		},
		{
			Name: "WithNestedStructAndSparseFieldset",
			Input: struct {
				TestMutation struct {
					FieldOne    string
					FieldTwo    string
					FieldThree  string
					NestedField struct {
						NestedFieldOne   string
						NestedFieldTwo   string
						NestedFieldThree string
					}
				} `goql:"testMutation(id:$id<ID!>)"`
			}{},
			Fields: Fields{
				"fieldOne": true,
				"fieldTwo": true,
				"nestedField": Fields{
					"nestedFieldOne": true,
				},
			},
			ExpectedOutput: `mutation($id: ID!) {
testMutation(id: $id) {
fieldOne
fieldTwo
nestedField {
nestedFieldOne
}
}
}`,
		},
		{
			Name: "WithNestedStructAndSparseFieldsetAndKeepTag",
			Input: struct {
				TestMutation struct {
					FieldOne    string
					FieldTwo    string
					NestedField struct {
						NestedFieldOne   string `goql:"keep"`
						NestedFieldTwo   string
						NestedFieldThree string
					} `goql:"keep"`
				} `goql:"testMutation(id:$id<ID!>)"`
			}{},
			Fields: Fields{
				"fieldOne": true,
				"fieldTwo": true,
			},
			ExpectedOutput: `mutation($id: ID!) {
testMutation(id: $id) {
fieldOne
fieldTwo
nestedField {
nestedFieldOne
}
}
}`,
		},
		{
			Name: "WithNestedStructAndSparseFieldsetAndKeepTagWithNestedFieldsIncluded",
			Input: struct {
				TestMutation struct {
					FieldOne    string
					FieldTwo    string
					FieldThree  string
					NestedField struct {
						FieldOne string
						FieldTwo string
						FieldXYZ string
						FieldABC string `goql:"keep"`
					} `goql:"keep"`
				} `goql:"testMutation(id:$id<ID!>)"`
			}{},
			Fields: Fields{
				"fieldOne": true,
				"fieldTwo": true,
				"nestedField": Fields{
					"fieldXYZ": true,
				},
			},
			ExpectedOutput: `mutation($id: ID!) {
testMutation(id: $id) {
fieldOne
fieldTwo
nestedField {
fieldXYZ
fieldABC
}
}
}`,
		},
	}

	for _, test := range tt {
		test := test

		fn := func(t *testing.T) {
			t.Parallel()

			actualOutput, err := MarshalMutation(test.Input, test.Fields)
			if err != nil {
				if test.ExpectedOutput == "" {
					// The error was expected, return without reporting anything.
					return
				}

				t.Fatalf("error marshaling mutation: %v", err)
			}

			trimmedExpectedOutput, trimmedActualOutput := strings.TrimSpace(test.ExpectedOutput), strings.TrimSpace(actualOutput)

			if e, a := len(trimmedExpectedOutput), len(trimmedActualOutput); e != a {
				t.Errorf("expected length of output to be %d, got %d", e, a)
			}

			percentMatch := percentageMatch(trimmedExpectedOutput, trimmedActualOutput)
			if percentMatch < minPercentageMatch {
				t.Errorf("expected percentage match to be %f, got %f", minPercentageMatch, percentMatch)
			}
		}
		t.Run(test.Name, fn)
	}
}
