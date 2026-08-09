// Package requestctx carries transport-neutral request identity and metadata.
package requestctx

import (
	"context"
	"errors"
	"maps"
	"slices"
	"time"
)

var ErrPrincipalRequired = errors.New("authenticated principal required")

// Principal identifies an authenticated caller without depending on an HTTP
// framework or persistence model. Subject is the stable identity-provider
// identifier. AccountID is optional and is zero until the subject has been
// resolved to a local account.
type Principal struct {
	Subject    string
	Email      string
	AccountID  uint
	Roles      []string
	AuthMethod string
}

func (p Principal) Authenticated() bool {
	return p.Subject != ""
}

func (p Principal) HasRole(role string) bool {
	return slices.Contains(p.Roles, role)
}

func (p Principal) clone() Principal {
	p.Roles = slices.Clone(p.Roles)
	return p
}

// Metadata describes one request independently of its transport.
type Metadata struct {
	RequestID     string
	CorrelationID string
	OperationID   string
	Method        string
	Path          string
	StartedAt     time.Time
	Cookies       map[string]string
	Headers       map[string]string
	RawBody       []byte
	DraftPreview  bool
}

type principalKey struct{}
type metadataKey struct{}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, principal.clone())
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(Principal)
	if !ok || !principal.Authenticated() {
		return Principal{}, false
	}
	return principal.clone(), true
}

func RequirePrincipal(ctx context.Context) (Principal, error) {
	principal, ok := PrincipalFrom(ctx)
	if !ok {
		return Principal{}, ErrPrincipalRequired
	}
	return principal, nil
}

func WithMetadata(ctx context.Context, metadata Metadata) context.Context {
	metadata.Cookies = maps.Clone(metadata.Cookies)
	metadata.Headers = maps.Clone(metadata.Headers)
	metadata.RawBody = slices.Clone(metadata.RawBody)
	return context.WithValue(ctx, metadataKey{}, metadata)
}

func MetadataFrom(ctx context.Context) (Metadata, bool) {
	metadata, ok := ctx.Value(metadataKey{}).(Metadata)
	metadata.Cookies = maps.Clone(metadata.Cookies)
	metadata.Headers = maps.Clone(metadata.Headers)
	metadata.RawBody = slices.Clone(metadata.RawBody)
	return metadata, ok
}

// WithOperation returns a context whose metadata names the generated OpenAPI
// operation currently being executed.
func WithOperation(ctx context.Context, operationID string) context.Context {
	metadata, _ := MetadataFrom(ctx)
	metadata.OperationID = operationID
	return WithMetadata(ctx, metadata)
}
