// Package httpapi implements the OpenAPI-first HTTP boundary.
//
// The concrete Server embeds four disjoint strict endpoint families. The
// compile-time StrictServerInterface assertion on Server makes a contract
// change fail compilation until every new operation is assigned exactly once;
// no fallback implementation or synthetic Gin forwarding is used.
//
// RegisterStrict is the only production registration path. It installs request
// metadata, problem rendering, contract-derived authentication/authorization,
// CSRF and body-limit policy before generated request binding, then registers
// the generated strict adapter.
package httpapi
