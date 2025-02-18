package goql

import "testing"

// TestToLowerCamelCase tests the toLowerCamelCase function.
func TestToLowerCamelCase(t *testing.T) {
	tt := []struct {
		Name           string
		Input          string
		ExpectedOutput string
	}{
		{
			Name:           "SpaceSeparator",
			Input:          "lower camel case this",
			ExpectedOutput: "lowerCamelCaseThis",
		},
		{
			Name:           "HyphenSeparator",
			Input:          "lower-camel-case-this",
			ExpectedOutput: "lowerCamelCaseThis",
		},
		{
			Name:           "UnderscoreSeparator",
			Input:          "lower_camel_case_this",
			ExpectedOutput: "lowerCamelCaseThis",
		},
		{
			Name:           "PeriodSeparator",
			Input:          "lower.camel.case.this",
			ExpectedOutput: "lowerCamelCaseThis",
		},
		{
			Name:           "StartsWithCapitalLatter",
			Input:          "LowerCamelCaseThis",
			ExpectedOutput: "lowerCamelCaseThis",
		},
	}

	for _, test := range tt {
		fn := func(t *testing.T) {
			t.Parallel()

			if e, a := test.ExpectedOutput, toLowerCamelCase(test.Input); e != a {
				t.Errorf("expected output to be %s, got %s", e, a)
			}
		}
		t.Run(test.Name, fn)
	}
}

// TestStack tests the stack type and its receiver functions.
func TestStack(t *testing.T) {
	t.Parallel()

	// Test stack to be used throughout this test.
	var st stack

	// Test field to be used throughout this test.
	f := field{
		Decl: declaration{
			Name:  "user",
			Alias: "",
			Tokens: []token{
				{
					Kind: "String!",
					Name: "name",
					Arg:  "name",
				},
			},
			Template: "getUser",
		},
		Directives: []directive{
			{
				Type: "include",
				Token: token{
					Kind: "Boolean!",
					Name: "ifAdmin",
					Arg:  "ifAdmin",
				},
				Template: "",
			},
		},
		Keep: true,
	}

	// expectedLength keeps track of what the length of the stack should be at
	// any given time.
	expectedLength := 0

	if e, a := expectedLength, st.length(); e != a {
		t.Errorf("expected length to be %d, got %d", e, a)
	}

	for i := 0; i < 3; i++ {
		st.push(&f)
		expectedLength++
	}

	if e, a := expectedLength, st.length(); e != a {
		t.Errorf("expected length to be %d, got %d", e, a)
	}

	if e, a := f.Decl.Template, st.top().Decl.Template; e != a {
		t.Errorf("expected declaration template on top to be %s, got %s", e, a)
	}

	for i := 0; i < expectedLength-1; i++ {
		_ = st.pop()
		expectedLength--
	}

	if e, a := expectedLength, st.length(); e != a {
		t.Errorf("expected length to be %d, got %d", e, a)
	}

	newDeclTemplate := "getUser2"
	st.apply(func(f *field) {
		f.Decl.Template = newDeclTemplate
	})

	if e, a := newDeclTemplate, st.top().Decl.Template; e != a {
		t.Errorf("expected declaration template on top to be %s, got %s", e, a)
	}
}

// BenchmarkStack benchmarks the receiver functions for the stack type.
func BenchmarkStack(b *testing.B) {
	var st stack
	f := field{
		Decl: declaration{
			Name:  "user",
			Alias: "",
			Tokens: []token{
				{
					Kind: "String!",
					Name: "name",
					Arg:  "name",
				},
			},
			Template: "getUser",
		},
		Directives: []directive{
			{
				Type: "include",
				Token: token{
					Kind: "Boolean!",
					Name: "ifAdmin",
					Arg:  "ifAdmin",
				},
				Template: "",
			},
		},
		Keep: true,
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		st.push(&f)
		_ = st.top()
		st.apply(func(f *field) {
			f.Decl.Template = "getUser2"
		})
		_ = st.pop()
	}
}
