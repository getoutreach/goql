package goql

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/getoutreach/goql/graphql_test"
)

// TestNewClient tests the NewClient function.
func TestNewClient(t *testing.T) {
	// This value is needed in one of the table tests and needs to be replicated
	// within the same table test so it needs to be abstracted out into a variable.
	specificError := errors.New("this is a specific error")

	tt := []struct {
		Name                    string
		URL                     string
		Options                 ClientOptions
		SpecificErrorFromMapper error
	}{
		{
			Name:    "GoodDefaultOptions",
			URL:     "http://localhost:3403/graphql",
			Options: DefaultClientOptions,
			SpecificErrorFromMapper: Errors{
				{
					Message: "foo",
				},
				{
					Message: "bar",
				},
			},
		},
		{
			Name: "GoodCustomOptions",
			URL:  "http://localhost:3403/graphql",
			Options: ClientOptions{
				HTTPClient: &http.Client{
					Timeout: time.Duration(465) * time.Second,
				},

				ErrorMapper: func(_ int, _ Errors) error {
					return specificError
				},
			},
			SpecificErrorFromMapper: specificError,
		},
	}

	for _, test := range tt {
		test := test

		fn := func(t *testing.T) {
			t.Parallel()

			client := NewClient(test.URL, test.Options)

			if e, a := test.URL, client.url; e != a {
				t.Errorf("expected url to be \"%s\", got \"%s\"", e, a)
			}

			// Default options were provided, so the HTTP client should be http.DefaultClient
			// and the errorMapper should return whatever is passed in the second argument to
			// the ErrorMapper type.
			if reflect.DeepEqual(test.Options, DefaultClientOptions) {
				if e, a := http.DefaultClient.Timeout, client.httpClient.Timeout; e != a {
					t.Errorf("expected client's http client timeout to be \"%s\", got \"%s\"", e.String(), a.String())
				}

				if e, a := test.SpecificErrorFromMapper.Error(), client.errorMapper(http.StatusBadRequest,
					test.SpecificErrorFromMapper.(Errors)).Error(); e != a { //nolint:errorlint // Why: test code
					t.Errorf("expected error returned from error mapper to be \"%s\", got \"%s\"", e, a)
				}

				return
			}

			// Else, custom ClientOptions were set on the Client.
			if e, a := test.Options.HTTPClient.Timeout, client.httpClient.Timeout; e != a {
				t.Errorf("expected client's http client timeout to be \"%s\", got \"%s\"", e.String(), a.String())
			}

			if e, a := test.SpecificErrorFromMapper.Error(), client.errorMapper(http.StatusBadRequest, Errors{}).Error(); e != a {
				t.Errorf("expected error returned from error mapper to be \"%s\", got \"%s\"", e, a)
			}
		}
		t.Run(test.Name, fn)
	}
}

// TestQueryWithHeaders tests the QueryWithHeaders pointer receiver function on the Client
// type. Since this is mostly a pass-through function to *Client.doStruct, this test is
// intentionally kept simple.
func TestQueryWithHeaders(t *testing.T) {
	t.Parallel()

	ts := graphql_test.NewServer(t, true)
	t.Cleanup(ts.Close)

	client := NewClient(ts.URL, DefaultClientOptions)

	var GetEntity graphql_test.GetEntity
	operation := Operation{
		OperationType: &GetEntity,
		Fields:        nil,
		Variables:     GetEntity.Variables(),
	}
	headers := http.Header{}

	if err := client.QueryWithHeaders(context.Background(), &operation, headers); err != nil {
		t.Fatalf("error running query with headers: %v", err)
	}

	ts.DiffResponse(GetEntity.ExpectedResponse(), GetEntity)
}

// TestQuery tests the Query pointer receiver function on the Client type. Since this is
// mostly a pass-through function to *Client.doStruct, this test is intentionally kept
// simple.
func TestQuery(t *testing.T) {
	t.Parallel()

	ts := graphql_test.NewServer(t, true)
	t.Cleanup(ts.Close)

	client := NewClient(ts.URL, DefaultClientOptions)

	var GetEntity graphql_test.GetEntity
	operation := Operation{
		OperationType: &GetEntity,
		Fields:        nil,
		Variables:     GetEntity.Variables(),
	}

	if err := client.Query(context.Background(), &operation); err != nil {
		t.Fatalf("error running query: %v", err)
	}

	ts.DiffResponse(GetEntity.ExpectedResponse(), GetEntity)
}

// TestMutateWithHeaders tests the MutateWithHeaders pointer receiver function on the Client
// type. Since this is mostly a pass-through function to *Client.doStruct, this test is
// intentionally kept simple.
func TestMutateWithHeaders(t *testing.T) {
	t.Parallel()

	ts := graphql_test.NewServer(t, true)
	t.Cleanup(ts.Close)

	client := NewClient(ts.URL, DefaultClientOptions)

	var CreateEntity graphql_test.CreateEntity
	operation := Operation{
		OperationType: &CreateEntity,
		Fields:        nil,
		Variables:     CreateEntity.Variables(),
	}
	headers := http.Header{}

	if err := client.MutateWithHeaders(context.Background(), &operation, headers); err != nil {
		t.Fatalf("error running mutate with headers: %v", err)
	}

	ts.DiffResponse(CreateEntity.ExpectedResponse(), CreateEntity)
}

// TestMutate tests the Mutate pointer receiver function on the Client type. Since this is
// mostly a pass-through function to *Client.doStruct, this test is intentionally kept
// simple.
func TestMutate(t *testing.T) {
	t.Parallel()

	ts := graphql_test.NewServer(t, true)
	t.Cleanup(ts.Close)

	client := NewClient(ts.URL, DefaultClientOptions)

	var CreateEntity graphql_test.CreateEntity
	operation := Operation{
		OperationType: &CreateEntity,
		Fields:        nil,
		Variables:     CreateEntity.Variables(),
	}

	if err := client.Mutate(context.Background(), &operation); err != nil {
		t.Fatalf("error running mutate: %v", err)
	}

	ts.DiffResponse(CreateEntity.ExpectedResponse(), CreateEntity)
}

// TestCustomOperationWithHeaders tests the CustomOperationWithHeaders pointer receiver
// function on the Client type. Since this is mostly a pass-through function to
// *Client.doCustom, this test is intentionally kept simple.
func TestCustomOperationWithHeaders(t *testing.T) {
	t.Parallel()

	ts := graphql_test.NewServer(t, true)
	t.Cleanup(ts.Close)

	client := NewClient(ts.URL, DefaultClientOptions)

	var testOperation graphql_test.GetEntity
	testQuery, err := MarshalQuery(testOperation, nil)
	if err != nil {
		t.Fatalf("error marshaling test query: %v", err)
	}
	headers := http.Header{}

	if err := client.CustomOperationWithHeaders(context.Background(), testQuery, testOperation.Variables(),
		&testOperation, headers); err != nil {
		t.Fatalf("error running custom operation with headers: %v", err)
	}

	ts.DiffResponse(testOperation.ExpectedResponse(), testOperation)
}

// TestCustomOperation tests the CustomOperation pointer receiver function on the Client
// type. Since this is mostly a pass-through function to *Client.doCustom, this test is
// intentionally kept simple.
func TestCustomOperation(t *testing.T) {
	t.Parallel()

	ts := graphql_test.NewServer(t, true)
	t.Cleanup(ts.Close)

	client := NewClient(ts.URL, DefaultClientOptions)

	var testOperation graphql_test.GetEntity
	testQuery, err := MarshalQuery(testOperation, nil)
	if err != nil {
		t.Fatalf("error marshaling test query: %v", err)
	}

	if err := client.CustomOperation(context.Background(), testQuery, testOperation.Variables(), &testOperation); err != nil {
		t.Fatalf("error running custom operation: %v", err)
	}

	ts.DiffResponse(testOperation.ExpectedResponse(), testOperation)
}
