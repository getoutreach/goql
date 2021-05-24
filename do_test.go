package goql

import (
	"context"
	"net/http"
	"testing"

	"github.com/getoutreach/goql/graphql_test"
)

// TestErrorsErrorInterface tests the Error method on the Errors type, which
// implements the error interface.
func TestErrorsErrorInterface(t *testing.T) {
	t.Parallel()

	err := Errors{
		{
			Message: "Foo",
		},
		{
			Message: "Bar",
		},
		{
			Message: "Baz",
		},
	}

	var expected string
	for i := range err {
		expected += err[i].Message

		if i != len(err)-1 {
			expected += ", "
		}
	}

	if e, a := expected, err.Error(); e != a {
		t.Errorf("expected error interface to return \"%s\", got \"%s\"", e, a)
	}

	err = Errors{}
	expected = ""

	if e, a := expected, err.Error(); e != a {
		t.Errorf("expected error interface to return \"%s\", got \"%s\"", e, a)
	}
}

// TestDoCustom tests the doCustom pointer receiver function on the Client type.
func TestDoCustom(t *testing.T) {
	t.Skip()

	ts := graphql_test.NewServer(t, true)
	t.Cleanup(ts.Close)

	client := NewClient(ts.URL, DefaultClientOptions)

	var testOperation graphql_test.GetEntity
	testQuery, err := MarshalQuery(testOperation, nil)
	if err != nil {
		t.Fatalf("error marshaling test query: %v", err)
	}

	tt := []struct {
		Name             string
		Query            string
		Variables        map[string]interface{}
		ResponseType     interface{}
		Headers          http.Header
		ExpectedResponse interface{}
		ShouldErr        bool
	}{
		{
			Name:             "SuccessReturnData",
			Query:            testQuery,
			Variables:        testOperation.Variables(),
			ResponseType:     &testOperation,
			Headers:          http.Header{},
			ExpectedResponse: testOperation.ExpectedResponse(),
			ShouldErr:        false,
		},
		{
			Name:             "SuccessDiscardData",
			Query:            testQuery,
			Variables:        testOperation.Variables(),
			ResponseType:     nil,
			Headers:          http.Header{},
			ExpectedResponse: nil,
			ShouldErr:        false,
		},
		{
			Name:             "ErrorInvalidOperation",
			Query:            "foobarbaz",
			Variables:        nil,
			ResponseType:     nil,
			Headers:          http.Header{},
			ExpectedResponse: nil,
			ShouldErr:        true,
		},
	}

	for _, test := range tt {
		test := test

		fn := func(t *testing.T) {
			t.Parallel()

			if err := client.doCustom(context.Background(), test.Query, test.Variables, test.ResponseType, test.Headers); err != nil {
				if test.ShouldErr {
					return
				}

				t.Fatalf("error doing graphql custom operation: %v", err)
			}

			if test.ExpectedResponse != nil {
				ts.DiffResponse(test.ExpectedResponse, test.ResponseType)
			}
		}
		t.Run(test.Name, fn)
	}
}

// TestDoStruct tests the doStruct pointer receiver function on the Client type.
func TestDoStruct(t *testing.T) {
	ts := graphql_test.NewServer(t, true)
	t.Cleanup(ts.Close)

	client := NewClient(ts.URL, DefaultClientOptions)

	tt := []struct {
		Name             string
		OperationType    int
		Operation        *Operation
		Headers          http.Header
		ExpectedResponse interface{}
		ShouldErr        bool
	}{
		{
			Name:          "SuccessQuery",
			OperationType: opQuery,
			Operation: &Operation{
				OperationType: &graphql_test.GetEntity{},
				Fields:        nil,
				Variables:     graphql_test.QueryGetEntity.Variables(),
			},
			Headers:          http.Header{},
			ExpectedResponse: graphql_test.QueryGetEntity.ExpectedResponse(),
			ShouldErr:        false,
		},
		{
			Name:          "SuccessMutation",
			OperationType: opMutation,
			Operation: &Operation{
				OperationType: &graphql_test.UpdateEntity{},
				Fields:        nil,
				Variables:     graphql_test.MutationUpdateEntity.Variables(),
			},
			Headers:          http.Header{},
			ExpectedResponse: graphql_test.MutationUpdateEntity.ExpectedResponse(),
			ShouldErr:        false,
		},
		{
			Name:          "ErrorOperationExistence",
			OperationType: opQuery,
			Operation: &Operation{
				OperationType: &struct {
					FooBarBaz string
				}{},
				Fields:    nil,
				Variables: nil,
			},
			Headers:          http.Header{},
			ExpectedResponse: nil,
			ShouldErr:        true,
		},
		{
			Name:          "ErrorInvalidOperationQuery",
			OperationType: opQuery,
			Operation: &Operation{
				OperationType: 32,
				Fields:        nil,
				Variables:     nil,
			},
			Headers:          http.Header{},
			ExpectedResponse: nil,
			ShouldErr:        true,
		},
		{
			Name:          "ErrorInvalidOperationMutation",
			OperationType: opMutation,
			Operation: &Operation{
				OperationType: "beep",
				Fields:        nil,
				Variables:     nil,
			},
			Headers:          http.Header{},
			ExpectedResponse: nil,
			ShouldErr:        true,
		},
	}

	for _, test := range tt {
		test := test

		fn := func(t *testing.T) {
			t.Parallel()

			if err := client.doStruct(context.Background(), test.OperationType, test.Operation, test.Headers); err != nil {
				if test.ShouldErr {
					return
				}

				t.Fatalf("error doing graphql custom operation: %v", err)
			}

			if test.ExpectedResponse != nil {
				ts.DiffResponse(test.ExpectedResponse, test.Operation.OperationType)
			}
		}
		t.Run(test.Name, fn)
	}
}
