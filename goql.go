// Package goql is a low-level client interface that can communicate with a running
// GraphQL server.
package goql

import (
	"context"
	"net/http"
)

// ErrorMapper is a type that is used for error mapping functions. The status code and Errors
// are sent as parameters and it is the functions responsibility to map the Errors into a type
// that implements the error interface.
type ErrorMapper func(int, Errors) error

// Client contains all of the necessary fields and receiver functions to carry out requests
// to a GraphQL server in an idiomatic way.
type Client struct {
	url         string
	httpClient  *http.Client
	errorMapper ErrorMapper
}

// ClientOptions is the type passed to NewClient that allows for configuration of the client.
//
// HTTPClient is an optional http.Client that the GraphQL client will use underneath the hood.
// If this field is omitted or nil then the client will use http.DefaultClient.
//
// ErrorMapper allows the Errors type potentially returned from the GraphQL server to be
// mapped to a different type that implements the error interface, optionally. The status
// code of the response from the GraphQL server is also passed to this function to attempt
// to give more context to the callee. If omitted or nil the Errors type will be returned
// in the case of any errors that came from the GraphQL server. See the documentation for
// the Errors type for more information as to what can be done with this mapping function.
type ClientOptions struct {
	HTTPClient  *http.Client
	ErrorMapper ErrorMapper
}

// DefaultClientOptions is a variable that can be passed for the ClientOptions when calling
// NewClient that will trigger use of all of the default options.
var DefaultClientOptions = ClientOptions{
	HTTPClient:  nil,
	ErrorMapper: nil,
}

// defaultErrorMapper shallow returns the Errors type that came from the response of a GraphQL
// server invocation.
var defaultErrorMapper = func(_ int, errs Errors) error {
	return errs
}

// NewClient returns a configured pointer to a Client. If httpClient is nil, http.DefaultClient
// will be used in place of it. If errorMapper is omitted the Errors type will be returned in
// the case of any errors that came from the GraphQL server.
func NewClient(clientURL string, options ClientOptions) *Client {
	// If HTTPClient was omitted or nil, use http.DefaultClient.
	if options.HTTPClient == nil {
		options.HTTPClient = http.DefaultClient
	}

	// if ErrorMapper was omitted or nil, use defaultErrorMapper.
	if options.ErrorMapper == nil {
		options.ErrorMapper = defaultErrorMapper
	}

	return &Client{
		url:         clientURL,
		httpClient:  options.HTTPClient,
		errorMapper: options.ErrorMapper,
	}
}

// QueryWithHeaders performs a query type of request to retrieve data from a GraphQL server. q should
// be passed by reference and all variables defined in the struct tag of q should exist within the
// variables map as well.
func (c *Client) QueryWithHeaders(ctx context.Context, operation *Operation, headers http.Header) error {
	if headers == nil {
		headers = http.Header{}
	}

	return c.doStruct(ctx, opQuery, operation, headers)
}

// Query is a wrapper around QueryWithHeaders that passes no headers.
func (c *Client) Query(ctx context.Context, operation *Operation) error {
	return c.QueryWithHeaders(ctx, operation, nil)
}

// MutateWithHeaders performs a mutate type of request to mutate and retrieve data from a GraphQL server.
// q should be passed by reference and all variables defined in the struct tag of q should exist within
// the variables map as well.
func (c *Client) MutateWithHeaders(ctx context.Context, operation *Operation, headers http.Header) error {
	if headers == nil {
		headers = http.Header{}
	}

	return c.doStruct(ctx, opMutation, operation, headers)
}

// Mutate is a wrapper around MutateWithHeaders that passes no headers.
func (c *Client) Mutate(ctx context.Context, operation *Operation) error {
	return c.MutateWithHeaders(ctx, operation, nil)
}

// CustomOperationWithHeaders takes a query in the form of a string and attempts to marshal the response
// into the resp parameter, which should be passed by reference (as a pointer). If nil is passed as the
// actual parameter for the formal parameter resp, the response is discarded.
func (c *Client) CustomOperationWithHeaders(ctx context.Context, query string, variables map[string]interface{},
	resp interface{}, headers http.Header) error {
	if headers == nil {
		headers = http.Header{}
	}

	return c.doCustom(ctx, query, variables, resp, headers)
}

// CustomOperation is a wrapper around CustomOperationWithHeaders that passes no headers.
func (c *Client) CustomOperation(ctx context.Context, query string, variables map[string]interface{}, resp interface{}) error {
	return c.CustomOperationWithHeaders(ctx, query, variables, resp, nil)
}
