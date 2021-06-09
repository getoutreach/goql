package goql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
)

// Valid operation types that the client can perform against the GraphQL server. These
// help create the correct type of query string.
const (
	// opQuery denotes that a query wrapper needs to wrap the created GraphQL query.
	opQuery = iota

	// opMutation denotes that a mutation wrapper needs to wrap the created GraphQL query.
	opMutation
)

// Operation is an encapsulation of all of the elements that are used to compose an operation
// using struct tags. The OperationType field should always be passed by reference in order for
// the data to be able to be marshaled back into it.
type Operation struct {
	OperationType interface{}
	Fields        Fields
	Variables     map[string]interface{}
}

// request is the type that contains the structure of a request that a GraphQL server expects.
type request struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

// Error is the type that contains the structure of an error returned from a GraphQL server. The
// Extensions key is intentionally left as a json.RawMessage so that it can optionally be handled
// and marshaled into whatever type necessary by the ErrorMapper passed to the client.
type Error struct {
	Message    string          `json:"message"`
	Path       []string        `json:"path"`
	Extensions json.RawMessage `json:"extensions"`
}

// Errors is a type alias for a slice of Error, which is what is returned in the response of a
// request to a GraphQL server. More information is available on the Error type's documentation.
type Errors []Error

// Error is a value receiver function on the Errors type which implements the error interface for
// its receiver. This allows the type to be returned as a normal error, but it can also be asserted
// to it's original type if desired.
func (e Errors) Error() string {
	errs := make([]string, 0, len(e))
	for i := range e {
		errs = append(errs, e[i].Message)
	}
	return strings.Join(errs, ", ")
}

// response is the type that contains the structure of a response from a GraphQL server.
type response struct {
	// Data uses json.RawMessage to delay decoding of itself since we don't
	// know the type of it at compile time.
	Data   json.RawMessage `json:"data"`
	Errors Errors          `json:"errors,omitempty"`
}

// doCustom takes a query as a string and performs a GraphQL operation. The response
// will be marshaled into the resp parameter that should have been passed by reference.
// If nil is passed as the actual parameter for the resp formal parameter, the response
// is discarded.
func (c *Client) doCustom(ctx context.Context, query string, variables map[string]interface{}, resp interface{}, headers http.Header) error {
	var buf bytes.Buffer

	// Create the request body using the constructed query or mutation.
	if err := json.NewEncoder(&buf).Encode(request{ //nolint:gocritic
		Query:     query,
		Variables: variables,
	}); err != nil {
		return err
	}

	// Do the request and get the "data" key of the response back as a json.RawMessage. Errors
	// returned in the response from GraphQL are handled inside of c.do.
	data, err := c.do(ctx, &buf, headers)
	if err != nil {
		return err
	}

	// Unmarshal the "data" key of the response into the desired struct that was passed in
	// by reference, if it was not passed in an nil.
	if resp != nil {
		if err := json.Unmarshal(data, resp); err != nil {
			return err
		}
	}

	return nil
}

// doStruct performs a request with a and retrieves a response from the GraphQL server
// configured in the receiver.
func (c *Client) doStruct(ctx context.Context, operationType int, operation *Operation, headers http.Header) error {
	var queryStr string
	var err error

	// Determine which type of operation was requested and construct the appropriate query
	// or mutation.
	switch operationType {
	case opQuery:
		if queryStr, err = MarshalQuery(operation.OperationType, operation.Fields); err != nil {
			return err
		}
	case opMutation:
		if queryStr, err = MarshalMutation(operation.OperationType, operation.Fields); err != nil {
			return err
		}
	}

	// Create the request body using the constructed query or mutation.
	var buf bytes.Buffer
	if err = json.NewEncoder(&buf).Encode(request{ //nolint:gocritic
		Query:     queryStr,
		Variables: operation.Variables,
	}); err != nil {
		return err
	}

	// Do the request and get the "data" key of the response back as a json.RawMessage. Errors
	// returned in the response from GraphQL are handled inside of c.do.
	data, err := c.do(ctx, &buf, headers)
	if err != nil {
		return err
	}

	// Unmarshal the "data" key of the response into the desired struct that was passed in
	// by reference.
	if err := json.Unmarshal(data, operation.OperationType); err != nil {
		return err
	}

	return nil
}

// do performs a GraphQL operation given a request body and headers. The "data" key of the
// GraphQL response is returned as a json.RawMessage for the caller to unmarshal. The errors
// returned in the response, if any, are dealt with in this function and returned as an
// error type, using c.errorMapper.
func (c *Client) do(ctx context.Context, body io.Reader, headers http.Header) (json.RawMessage, error) { //nolint:funlen
	// Create a request to query the GraphQL server located at the configured URL.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, body)
	if err != nil {
		return nil, err
	}

	// Close the request body once this function returns.
	defer func() {
		if err = req.Body.Close(); err != nil {
			log.Error(ctx, "close request body", events.NewErrorInfo(err))
		}
	}()

	// Add headers if they exist.
	req.Header = headers

	// The Content-Type of this request will always be application/json as per the GraphQL specification.
	req.Header.Set("Content-Type", "application/json")

	// We don't want this header to be set because then we won't get the luxury of the transport automatically
	// decoding the response body for us, if it is encoded.
	req.Header.Del("Accept-Encoding")

	// Do the GraphQL request using the HTTP client that was configured for this GraphQL client.
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Close the response body once this function returns.
	defer func() {
		if err = resp.Body.Close(); err != nil {
			log.Error(ctx, "close response body", events.NewErrorInfo(err))
		}
	}()

	var gqlResp response

	// Create two copies from the response buffer, one to decode and one to fall back on if the decoding
	// fails for any reason.
	var fallbackCopy bytes.Buffer
	decoderCopy := io.TeeReader(resp.Body, &fallbackCopy)

	// Attempt to decode the response from the GraphQL server.
	if err := json.NewDecoder(decoderCopy).Decode(&gqlResp); err != nil {
		// If the decode attempt failed, dump the body and return.
		b, err := ioutil.ReadAll(&fallbackCopy)
		if err != nil {
			log.Error(ctx, "read non-200 status response body from graphql server",
				events.Err(err), log.F{
					"statusCode": resp.StatusCode,
				})
		}

		return nil, fmt.Errorf("unknown response format with status %d received from graphql server: %s",
			resp.StatusCode, b)
	}

	// If an error occurred, return it immediately.
	if len(gqlResp.Errors) > 0 {
		return nil, c.errorMapper(resp.StatusCode, gqlResp.Errors)
	}

	return gqlResp.Data, nil
}
