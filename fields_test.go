package goql

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestFieldsFromDelimitedList tests the FieldsFromDelimitedList function.
func TestFieldsFromDelimitedList(t *testing.T) {
	tt := []struct {
		Name              string
		Input             string
		FieldDelimiter    string
		SubfieldDelimiter string
		ExpectedOutput    Fields
	}{
		{
			Name:              "EmptyInput",
			Input:             "",
			FieldDelimiter:    ",",
			SubfieldDelimiter: ".",
			ExpectedOutput:    nil,
		},
		{
			Name:              "OneLevel",
			Input:             "foo|bar|baz",
			FieldDelimiter:    "|",
			SubfieldDelimiter: ";",
			ExpectedOutput: Fields{
				"foo": true,
				"bar": true,
				"baz": true,
			},
		},
		{
			Name:              "TwoLevels",
			Input:             "foo/bar|bar/baz|baz/foo",
			FieldDelimiter:    "|",
			SubfieldDelimiter: "/",
			ExpectedOutput: Fields{
				"foo": Fields{
					"bar": true,
				},
				"bar": Fields{
					"baz": true,
				},
				"baz": Fields{
					"foo": true,
				},
			},
		},
		{
			Name:              "ThreeLevels",
			Input:             "foo_bar_baz-bar_baz_foo-baz_foo_bar",
			FieldDelimiter:    "-",
			SubfieldDelimiter: "_",
			ExpectedOutput: Fields{
				"foo": Fields{
					"bar": Fields{
						"baz": true,
					},
				},
				"bar": Fields{
					"baz": Fields{
						"foo": true,
					},
				},
				"baz": Fields{
					"foo": Fields{
						"bar": true,
					},
				},
			},
		},
	}

	for _, test := range tt {
		fn := func(t *testing.T) {
			t.Parallel()

			d := cmp.Diff(test.ExpectedOutput, FieldsFromDelimitedList(test.Input, test.FieldDelimiter, test.SubfieldDelimiter))
			if d != "" {
				t.Errorf("unexpected difference between expected output and actual output:\n%s", d)
			}
		}
		t.Run(test.Name, fn)
	}
}

// TestFieldsFromURLQueryParam tests the FieldsFromURLQueryParam function.
func TestFieldsFromURLQueryParam(t *testing.T) {
	tt := []struct {
		Name           string
		Input          string
		ExpectedOutput Fields
	}{
		{
			Name:  "OneLevel",
			Input: "foo,bar,baz",
			ExpectedOutput: Fields{
				"foo": true,
				"bar": true,
				"baz": true,
			},
		},
		{
			Name:  "TwoLevels",
			Input: "foo.bar,bar.baz,baz.foo",
			ExpectedOutput: Fields{
				"foo": Fields{
					"bar": true,
				},
				"bar": Fields{
					"baz": true,
				},
				"baz": Fields{
					"foo": true,
				},
			},
		},
		{
			Name:  "ThreeLevels",
			Input: "foo.bar.baz,bar.baz.foo,baz.foo.bar",
			ExpectedOutput: Fields{
				"foo": Fields{
					"bar": Fields{
						"baz": true,
					},
				},
				"bar": Fields{
					"baz": Fields{
						"foo": true,
					},
				},
				"baz": Fields{
					"foo": Fields{
						"bar": true,
					},
				},
			},
		},
	}

	for _, test := range tt {
		fn := func(t *testing.T) {
			t.Parallel()

			d := cmp.Diff(test.ExpectedOutput, FieldsFromURLQueryParam(test.Input))
			if d != "" {
				t.Errorf("unexpected difference between expected output and actual output:\n%s", d)
			}
		}
		t.Run(test.Name, fn)
	}
}

// TestUnion tests the Union function.
func TestUnion(t *testing.T) {
	tt := []struct {
		Name           string
		Input          []Fields
		ExpectedOutput Fields
	}{
		{
			Name: "SameFields",
			Input: []Fields{
				{
					"foo": true,
					"bar": true,
					"baz": Fields{
						"baax": true,
					},
				},
				{
					"foo": true,
					"bar": true,
					"baz": Fields{
						"baax": true,
					},
				},
			},
			ExpectedOutput: Fields{
				"foo": true,
				"bar": true,
				"baz": Fields{
					"baax": true,
				},
			},
		},
	}

	for _, test := range tt {
		fn := func(t *testing.T) {
			t.Parallel()

			actual := test.Input[0]
			for i := 1; i < len(test.Input); i++ {
				actual = Union(actual, test.Input[i])
			}

			if d := cmp.Diff(test.ExpectedOutput, actual); d != "" {
				t.Errorf("unexpected difference between expected output and actual output:\n%s", d)
			}
		}
		t.Run(test.Name, fn)
	}
}
