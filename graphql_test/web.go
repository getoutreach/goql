package graphql_test

import (
	"encoding/json"
	"net/http"
)

type Request struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type ResponseError struct {
	Message    string   `json:"message"`
	Path       []string `json:"path"`
	Extensions struct{} `json:"extensions"`
}

type Response struct {
	Data   interface{}     `json:"data"`
	Errors []ResponseError `json:"errors,omitempty"`
}

func (s *Server) respondError(w http.ResponseWriter, status int, errs ...error) {
	s.t.Helper()

	res := Response{
		Data: nil,
	}

	for i := range errs {
		res.Errors = append(res.Errors, ResponseError{
			Message: errs[i].Error(),
		})
	}

	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(res); err != nil {
		s.t.Errorf("encode graphql error response: %v", err)
	}
}

func (s *Server) respond(w http.ResponseWriter, status int, data interface{}) {
	s.t.Helper()

	res := Response{
		Data:   data,
		Errors: nil,
	}

	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(res); err != nil {
		s.t.Errorf("encode graphql response: %v", err)
	}
}
