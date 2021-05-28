package graphql_test

import "time"

// Operation Type Constants
const (
	opQuery = iota + 1
	opMutation
)

// Operation is a general type that encompasses the Operation type and Response which
// is of the same type, but with data.
type Operation struct {
	// opType denotes whether the operation is a query or a mutation, using the opQuery
	// and opMutation constants. This is unexported as it is set by the *Sever.RegisterQuery
	// and *Server.RegisterMutation functions, respectively.
	opType int

	// Identifier helps identify the operation in a request when coming through the Server.
	// For example, if your operation looks like this:
	//
	//	query {
	//		myOperation(foo: $foo) {
	//			fieldOne
	//			fieldTwo
	//		}
	//	}
	//
	// Then this field should be set to myOperation. It can also be more specific, a simple
	// strings.Contains check occurs to match operations. A more specific example of a
	// valid Identifier for the same operation given above would be myOperation(foo: $foo).
	Identifier string

	// Variables represents the map of variables that should be passed along with the
	// operation whenever it is invoked on the Server.
	Variables map[string]interface{}

	// Response represents the response that should be returned whenever the server makes
	// a match on Operation.opType, Operation.Name, and Operation.Variables.
	Response interface{}
}

// --------------------------------------------------------- //
// --- DEFAULT OPERATIONS ARE DEFINED BELOW THIS COMMENT --- //
// --------------------------------------------------------- //

// now is used to render consistent timestamps in default queries and mutations.
var now = time.Now()

// General Types Used in Default Mutations and Queries
type (
	Entity struct {
		ID         int       `json:"id"`
		FieldOne   string    `json:"fieldOne"`
		FieldTwo   string    `json:"fieldTwo"`
		CreatedAt  time.Time `json:"createdAt"`
		ModifiedAt time.Time `json:"modifiedAt"`
	}

	ShallowEntity struct {
		ID int `json:"id"`
	}
)

var MutationDeleteEntity DeleteEntity

type DeleteEntity struct {
	ShallowEntity `goql:"deleteEntity(id:$id<ID!>)"`
}

func (*DeleteEntity) operationName() string {
	return "deleteEntity"
}

func (*DeleteEntity) ExpectedResponse() ShallowEntity {
	return ShallowEntity{
		ID: 1,
	}
}

func (*DeleteEntity) Variables() map[string]interface{} {
	return map[string]interface{}{
		"id": 1,
	}
}

var MutationUpdateEntity UpdateEntity

type UpdateEntity struct {
	Entity `goql:"updateEntity(id:$id<ID!>,entity:$entity<Entity!>)"`
}

func (*UpdateEntity) operationName() string {
	return "updateEntity"
}

func (*UpdateEntity) ExpectedResponse() Entity {
	return Entity{
		ID:         1,
		FieldOne:   "foo",
		FieldTwo:   "bar",
		CreatedAt:  now,
		ModifiedAt: now,
	}
}

func (*UpdateEntity) Variables() map[string]interface{} {
	return map[string]interface{}{
		"id": 1,
		"entity": Entity{
			FieldOne: "foo",
			FieldTwo: "bar",
		},
	}
}

var MutationCreateEntity CreateEntity

type CreateEntity struct {
	Entity `goql:"createEntity(entity:$entity<Entity!>)"`
}

func (*CreateEntity) operationName() string {
	return "createEntity"
}

func (*CreateEntity) ExpectedResponse() Entity {
	return Entity{
		ID:         2,
		FieldOne:   "baz",
		FieldTwo:   "quux",
		CreatedAt:  now,
		ModifiedAt: now,
	}
}

func (*CreateEntity) Variables() map[string]interface{} {
	return map[string]interface{}{
		"entity": Entity{
			FieldOne: "baz",
			FieldTwo: "quux",
		},
	}
}

var QueryGetEntity GetEntity

type GetEntity struct {
	Entity `goql:"getEntity(id:$id<ID!>)"`
}

func (*GetEntity) operationName() string {
	return "getEntity"
}

func (*GetEntity) ExpectedResponse() Entity {
	return Entity{
		ID:         1,
		FieldOne:   "foo",
		FieldTwo:   "bar",
		CreatedAt:  now,
		ModifiedAt: now,
	}
}

func (*GetEntity) Variables() map[string]interface{} {
	return map[string]interface{}{
		"id": 1,
	}
}
