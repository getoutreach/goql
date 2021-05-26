package graphql_test

import (
	"encoding/json"
	"reflect"

	"github.com/google/go-cmp/cmp"
)

// DiffResponse takes the expected and actual response, expected coming from Operation.Response
// and actual coming from an attempted marshal into the __type of__ Operation.Response and
// compares them with cmp.Diff. This function transforms each parameter into a generic type,
// map[string]interface{}, before comparing since type information is always lost when attempting
// to marshaling to the __type of__ Operation.Response, making it very hard to diff the actual
// response with Operation.Response for a given Operation ran on the Server.
func (s *Server) DiffResponse(expected, actual interface{}) {
	s.t.Helper()

	expectedMap, actualMap := make(map[string]interface{}), make(map[string]interface{})
	rte, rta := reflect.TypeOf(expected), reflect.TypeOf(actual)

	necessaryType := reflect.TypeOf(map[string]interface{}{})

	if rte != necessaryType {
		b, err := json.Marshal(expected)
		if err != nil {
			s.t.Fatalf("marshal expected type into bytes: %v", err)
		}

		if err := json.Unmarshal(b, &expectedMap); err != nil {
			s.t.Fatalf("unmarshal expected data into map: %v", err)
		}
	} else {
		expectedMap = expected.(map[string]interface{})
	}

	if rta != necessaryType {
		b, err := json.Marshal(actual)
		if err != nil {
			s.t.Fatalf("marshal actual type into bytes: %v", err)
		}

		if err := json.Unmarshal(b, &actualMap); err != nil {
			s.t.Fatalf("unmarshal actual data into map: %v", err)
		}
	} else {
		actualMap = actual.(map[string]interface{})
	}

	if d := cmp.Diff(expectedMap, actualMap); d != "" {
		s.t.Errorf("unexpected difference between expected and actual response data:\n%s", d)
	}
}

// equalVariables takes two variables and makes sure they are equal in length and
// each contain the same keys. The values of the keys are not checked.
func (s *Server) equalVariables(x, y map[string]interface{}) bool {
	if len(x) != len(y) {
		return false
	}

	for k := range x {
		if _, exists := y[k]; !exists {
			return false
		}
	}

	for k := range y {
		if _, exists := x[k]; !exists {
			return false
		}
	}

	return true
}
