// Package graphql_test exports a Server that facilitates the testing of client
// integrations of GraphQL by mocking a GraphQL server.
package graphql_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pkg/errors"
)

// Server is a type that contains one exported struct field - a URL that points to a
// httptest.Server that will mock a GraphQL server that can be used to test client
// integrations.
type Server struct {
	URL string

	mutations []Operation
	queries   []Operation
	errors    []OperationError

	t      *testing.T
	server *httptest.Server
}

// NewServer returns a configured Server. If useDefaultOperations is set to true then
// default queries and mutations will be registered in the server. The type returned
// contains a closing function which should be immediately registered using t.Cleanup
// after calling NewServer, example:
//
//	ts := graphql_test.NewServer(t, true)
//	t.Cleanup(ts.Close)
//
// This will ensure that no resources are dangling.
func NewServer(t *testing.T, useDefaultOperations bool) *Server { //nolint:funlen
	s := Server{
		t: t,
	}

	if useDefaultOperations {
		s.RegisterQuery(Operation{
			Identifier: QueryGetEntity.operationName(),
			Variables:  QueryGetEntity.Variables(),
			Response:   QueryGetEntity.ExpectedResponse(),
		})

		s.RegisterMutation(Operation{
			Identifier: MutationCreateEntity.operationName(),
			Variables:  MutationCreateEntity.Variables(),
			Response:   MutationCreateEntity.ExpectedResponse(),
		})

		s.RegisterMutation(Operation{
			Identifier: MutationUpdateEntity.operationName(),
			Variables:  MutationUpdateEntity.Variables(),
			Response:   MutationUpdateEntity.ExpectedResponse(),
		})

		s.RegisterMutation(Operation{
			Identifier: MutationDeleteEntity.operationName(),
			Variables:  MutationDeleteEntity.Variables(),
			Response:   MutationDeleteEntity.ExpectedResponse(),
		})
	}

	var mux http.ServeMux
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var reqBody Request
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			s.respondError(w, http.StatusInternalServerError, errors.Wrap(err, "decode request body"), nil)
			return
		}

		switch {
		case strings.HasPrefix(strings.TrimSpace(reqBody.Query), "mutation"):
			for i := range s.mutations {
				if strings.Contains(reqBody.Query, s.mutations[i].Identifier) {
					if s.equalVariables(s.mutations[i].Variables, reqBody.Variables) {
						s.respond(w, http.StatusOK, s.mutations[i].Response)
						return
					}
				}
			}
		case strings.HasPrefix(strings.TrimSpace(reqBody.Query), "query"):
			for i := range s.queries {
				if strings.Contains(reqBody.Query, s.queries[i].Identifier) {
					if s.equalVariables(s.queries[i].Variables, reqBody.Variables) {
						s.respond(w, http.StatusOK, s.queries[i].Response)
						return
					}
				}
			}
		case strings.HasPrefix(strings.TrimSpace(reqBody.Query), "error"):
			for i := range s.errors {
				if strings.Contains(reqBody.Query, s.errors[i].Identifier) {
					s.respondError(w, s.errors[i].Status, s.errors[i].Error, s.errors[i].Extensions)
					return
				}
			}
		}

		s.respondError(w, http.StatusNotFound, errors.New("operation not found"), nil)
	})

	s.server = httptest.NewServer(&mux)
	s.URL = s.server.URL

	return &s
}

// Close closes the underlying httptest.Server.
func (s *Server) Close() {
	s.server.Close()
}

// Mutations returns the registered mutations that the server will accept and respond
// to.
func (s *Server) Mutations() []Operation {
	return s.mutations
}

// Queries returns the registered queries that the server will accept and respond to.
func (s *Server) Queries() []Operation {
	return s.queries
}

// RegisterQuery registers an Operation as a query that the server will recognize and
// respond to.
func (s *Server) RegisterQuery(operation Operation) {
	operation.opType = opQuery
	s.queries = append(s.queries, operation)
}

// RegisterMutation registers an Operation as a mutation that the server will recognize
// and respond to.
func (s *Server) RegisterMutation(operation Operation) {
	operation.opType = opMutation
	s.mutations = append(s.mutations, operation)
}

// RegisterError registers an OperationError as an error that the server will recognize
// and respond to.
func (s *Server) RegisterError(operation OperationError) {
	s.errors = append(s.errors, operation)
}

// Do takes a Request, performs it using the underlying httptest.Server, and returns a
// Response.
func (s *Server) Do(r Request) Response {
	s.t.Helper()

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		s.t.Fatalf("encode graphql request body: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, s.URL, &buf)
	if err != nil {
		s.t.Fatalf("create graphql request: %v", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		s.t.Errorf("do graphql request: %v", err)
	}
	defer res.Body.Close()

	var resBody Response
	if err := json.NewDecoder(res.Body).Decode(&resBody); err != nil {
		s.t.Errorf("decode graphql response body: %v", err)
	}

	return resBody
}
