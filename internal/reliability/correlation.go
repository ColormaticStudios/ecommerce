// Package reliability provides transport-neutral reliability primitives shared
// by HTTP requests, background jobs, and provider operations.
package reliability

import (
	"context"
	"net/http"
	"strings"
	"unicode"

	"ecommerce/internal/requestctx"
)

const (
	RequestIDHeader     = "X-Request-ID"
	CorrelationIDHeader = "X-Correlation-ID"
)

// Correlation identifies a single request and the wider operation it belongs
// to. A background job should normally retain CorrelationID and create its own
// RequestID when it starts an execution attempt.
type Correlation struct {
	RequestID     string
	CorrelationID string
}

// FromContext reads correlation metadata without exposing the wider request
// metadata contract to callers.
func FromContext(ctx context.Context) Correlation {
	metadata, _ := requestctx.MetadataFrom(ctx)
	return Correlation{RequestID: metadata.RequestID, CorrelationID: metadata.CorrelationID}
}

// WithCorrelation attaches correlation values while preserving other request
// metadata already present on the context.
func WithCorrelation(ctx context.Context, correlation Correlation) context.Context {
	metadata, _ := requestctx.MetadataFrom(ctx)
	metadata.RequestID = correlation.RequestID
	metadata.CorrelationID = correlation.CorrelationID
	return requestctx.WithMetadata(ctx, metadata)
}

// ExtractHTTP returns valid correlation values supplied by an HTTP caller.
// Trust policy is intentionally enforced by the calling middleware.
func ExtractHTTP(header http.Header) Correlation {
	return Correlation{
		RequestID:     ValidExternalID(header.Get(RequestIDHeader)),
		CorrelationID: ValidExternalID(header.Get(CorrelationIDHeader)),
	}
}

// InjectHTTP propagates non-empty correlation values to an outbound request.
func InjectHTTP(ctx context.Context, header http.Header) {
	correlation := FromContext(ctx)
	if correlation.RequestID != "" {
		header.Set(RequestIDHeader, correlation.RequestID)
	}
	if correlation.CorrelationID != "" {
		header.Set(CorrelationIDHeader, correlation.CorrelationID)
	}
}

// ValidExternalID accepts compact opaque IDs while rejecting whitespace,
// control characters, and unbounded input.
func ValidExternalID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return ""
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return ""
		}
	}
	return value
}
