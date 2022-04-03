# goql

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/getoutreach/goql)
[![Generated via Bootstrap](https://img.shields.io/badge/Outreach-Bootstrap-%235951ff)](https://github.com/getoutreach/bootstrap)

A GraphQL client package written in Go.

## Contributing

Please read the [CONTRIBUTING.md](CONTRIBUTING.md) document for guidelines on developing and contributing changes.

## High-level Overview

<!--- Block(overview) -->

goql is a GraphQL client library with built-in two-way marshaling support via struct tags. This is key because it allows
for strongly typed GraphQL queries as opposed to variables containing a string representation of the query. This also
facilitates more advanced features, such as sparse field sets.

For complete documentation see the generated [pkg.go documentation](https://pkg.go.dev/github.com/getoutreach/goql). For
a complete guide on the struct tag syntax, see the documentation found below under
[Defining GraphQL Operations](#defining-graphql-operations).

## Installation

In the root of your project repository (same directory as your `go.mod` and `go.sum` files):

```shell
go get github.com/getoutreach/goql
```

After that you should be able to import it anywhere within your project.

## Defining GraphQL Operations

GraphQL operations can be defined by using normal Go struct types along with the help of struct tags. For example:

```go
type QueryUserCollection struct {
	UserCollection struct {
		Collection []struct {
			ID   string
			Name string
		} `goql:"keep"`
	} `goql:"userCollection(filter:$filter<[Filter]>,sort:$sort<[String]>,size:$size<Int>,before:$before<String>,after:$after<String>)"`
}
```

when passed through the GraphQL query marshaller renders the following string:

```graphql
query (
  $filter: [Filter]
  $sort: [String]
  $size: Int
  $before: String
  $after: String
) {
  userCollection(
    filter: $filter
    sort: $sort
    size: $size
    before: $before
    after: $after
  ) {
    collection {
      id
      name
    }
  }
}
```

Here's the high-level steps to go through when first defining a GraphQL operation:

1. Create a struct that will act as a wrapper for the entire operation. The top-level model will be the only immediate
   child struct field of this wrapper struct (e.g. `QueryUserCollection`'s only immediate child is `UserCollection` which
   together represents the `query($filter: [Filter], ...) { userCollection(filter: $filter, ...) { ... } }` part of the
   output).
2. Define all of the fields and sub-models of the top-level model as struct fields within the top-level model (e.g.
   `UserCollection` contains children fields `[]Collection`, `ID`, and `Name`). All types should match the types described
   in the schema of the query. - `ID` in GraphQL is a `string` in Go. - Any type with the non-null (`!`) restriction in GraphQL should be a non-pointer type in Go. Conversely, any type
   in GraphQL without this restriction should be nullable (a pointer type) in Go. - If the field is an integral part of the operation, e.g. `UserCollection`, and `Collection` fields in the struct
   above, add the `goql:"keep"` tag to them to tell the marshaler to always include these fields. This is necessary
   in order for sparse field sets to work. However, in the example above the keep tag can actually be omitted from the
   `UserCollection` part of the query as it already defines an operation declaration, which the marshaler already sees
   as an integral part of the operation and implicitly marks it to be kept (that is why the `keep` tag is left off of
   that portion, but on `Collection` still).
3. Iterate through the fields and add `goql` struct tags to further define the structure of the operation by
   modifying declarations, adding aliases, variables, or directives to each field. See the immediately proceeding section,
   [GraphQL Struct Tag Syntax](#graphql-struct-tag-syntax), for more information on these struct tags and how to define
   them.

### GraphQL Struct Tag Syntax

The following components can be used alone or together, separated by a comma within in the tag, to define a `goql`
struct tag for a field or model on an operation:

- `modelName(arg:$var<Type>, arg2:$var2<Type2!>, ...)`
  - Defines the name and argument list for a model. This is close to what you would see in a normal GraphQL operation,
    with a little syntactic sugar added to define the types of variables since they're needed in the wrapper of the
    operation when defining the variables used throughout it. This component implicitly defines the keep tag for the
    field as well, given that operation declarations are necessary regardless of sparse fieldset instructions.
  - `` MyModel struct `goql:"myModel(page:$page<Int!>)"` `` -> `query($page: Int!) { myModel(page: $page) { ...`
- `fieldNameOverride`
  - Overrides the name of a field, by default the lower camel-case version of the name of the struct field is used.
  - `` Name string `goql:"username"` `` -> `username`
- `@alias(desiredAlias)`
  - Adds an alias for a field or model, which will change the returned key in the JSON response from the GraphQL
    server. See [the GraphQL documentation on aliases](https://graphql.org/learn/queries/#aliases) for more information.
  - An alias is required when an operation name set by a goql tag diverges from the struct field name. Without an
    alias in that situation the data would not be able to be marshaled back into the struct field after the operation
    succeeds, resulting in a silent "error". As an example, `` Role *Role `goql:"createRole(...)"` `` would need an
    alias since createRole (operation name) != Role (struct field name).
  - `` Name string `goql:"@alias(username)"` `` -> `username: name`
- `@include($flag)`
  - Adds an include directive to the field or model. See
    [the GraphQL documentation on directives](https://graphql.org/learn/queries/#directives) for more information. Note
    that the variable passed to this directive in the struct tag does not have a type proceeding it in square brackets.
    This is because these directive variables always have the type of `Boolean!`, so it is implied and therefore not
    necessary.
  - `` Name string `goql:"@include($withName)"` `` -> `name @include(if: $withName)`
- `@skip($flag)`
  - Adds a skip directive to the field or model. See
    [the GraphQL documentation on directives](https://graphql.org/learn/queries/#directives) for more information. Note
    that the variable passed to this directive in the struct tag does not have a type proceeding it in square brackets.
    This is because these directive variables always have the type of `Boolean!`, so it is implied and therefore not
    necessary.
  - `` Name string `goql:"@skip($withoutName)"` `` -> `name @skip(if: $withoutName)`
- `keep`
  - Tells the marshaler to keep this field regardless of what is requested in terms of sparse field sets.

Here is an example of using multiple struct tags together:

`` Name string `goql:"@alias(username),@include($withName)"` `` -> `username: name @include(if: $withName)`

Rules:

- The same component cannot be defined more than once in a singular struct tag.
  - `` Name string `goql:"@include($withName),@include($withName2)"` `` would result in an error because an include
    directive was defined twice on the same struct tag.
- All defined variables must only have one type each associated with them.
  - `` MyModel struct `goql:"myModel(page:$page<Int!>,pageSize:$page<Int>)"` `` would result in an error, since
    $page is defined to have both the type of `Int!` and `Int`.
  - `` MyModel struct `goql:"myModel(page:$page<Int!>),@include($page)"` `` would also result in an error, since
    $page is defined to have the type of both `Int!` and `Boolean!` (implicit when used in the include directive).

<!--- EndBlock(overview) -->
