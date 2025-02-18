package goql

import (
	"strings"
	"testing"

	"github.com/pmezard/go-difflib/difflib"
)

// TestMarshalQuery tests the MarshalQuery function.
func TestMarshalQuery(t *testing.T) {
	tt := []struct {
		Name           string
		Input          interface{}
		Fields         Fields
		Option         marshalOption
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
				} `goql:"testQuery(id:$id<ID!>,list:$list<[List!]>,list2:$list2<[List2!]!>)"`
			}{},
			Fields: nil,
			ExpectedOutput: `query($id: ID!, $list: [List!], $list2: [List2!]!) {
testQuery(id: $id, list: $list, list2: $list2) {
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
		{
			Name: "SimpleWithJSON",
			Input: struct {
				TestQuery struct {
					FieldOne string `json:"differentField"`
				}
			}{},
			Fields: nil,
			Option: OptFallbackJSONTag,
			ExpectedOutput: `query {
testQuery {
differentField
}
}`,
		},
		{
			Name: "JSONOverriddenByGoqlTag",
			Input: struct {
				TestQuery struct {
					FieldOne string `json:"differentField" goql:"overrideName"`
				}
			}{},
			Fields: nil,
			Option: OptFallbackJSONTag,
			ExpectedOutput: `query {
testQuery {
overrideName
}
}`,
		},
	}

	for _, test := range tt {
		fn := func(t *testing.T) {
			t.Parallel()

			actualOutput, err := MarshalQueryWithOptions(test.Input, test.Fields, test.Option)
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

			if trimmedExpectedOutput != trimmedActualOutput {
				x := difflib.UnifiedDiff{
					A:        difflib.SplitLines(trimmedExpectedOutput),
					B:        difflib.SplitLines(trimmedActualOutput),
					FromFile: "expected",
					ToFile:   "actual",
					Context:  5,
				}
				text, _ := difflib.GetUnifiedDiffString(x)
				t.Fatalf("expected does not match actual:\n%s\n", text)
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
		Option         marshalOption
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
		{
			Name: "SimpleWithJSON",
			Input: struct {
				TestQuery struct {
					FieldOne string `json:"differentField"`
				}
			}{},
			Fields: nil,
			Option: OptFallbackJSONTag,
			ExpectedOutput: `mutation {
testQuery {
differentField
}
}`,
		},
		{
			Name: "JSONOverriddenByGoqlTag",
			Input: struct {
				TestQuery struct {
					FieldOne string `json:"differentField" goql:"overrideName"`
				}
			}{},
			Fields: nil,
			Option: OptFallbackJSONTag,
			ExpectedOutput: `mutation {
testQuery {
overrideName
}
}`,
		},
	}

	for _, test := range tt {
		fn := func(t *testing.T) {
			t.Parallel()

			actualOutput, err := MarshalMutationWithOptions(test.Input, test.Fields, test.Option)
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

			if trimmedExpectedOutput != trimmedActualOutput {
				x := difflib.UnifiedDiff{
					A:        difflib.SplitLines(trimmedExpectedOutput),
					B:        difflib.SplitLines(trimmedActualOutput),
					FromFile: "expected",
					ToFile:   "actual",
					Context:  5,
				}
				text, _ := difflib.GetUnifiedDiffString(x)
				t.Fatalf("expected does not match actual:\n%s\n", text)
			}
		}
		t.Run(test.Name, fn)
	}
}
