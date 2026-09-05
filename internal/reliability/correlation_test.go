package reliability_test

import (
	"context"
	"net/http"
	"testing"

	"ecommerce/internal/reliability"
	"github.com/stretchr/testify/assert"
)

func TestCorrelationContextAndHTTPPropagation(t *testing.T) {
	ctx := reliability.WithCorrelation(context.Background(), reliability.Correlation{
		RequestID: "request-1", CorrelationID: "checkout-1",
	})
	assert.Equal(t, reliability.Correlation{RequestID: "request-1", CorrelationID: "checkout-1"}, reliability.FromContext(ctx))

	header := make(http.Header)
	reliability.InjectHTTP(ctx, header)
	assert.Equal(t, "request-1", header.Get(reliability.RequestIDHeader))
	assert.Equal(t, "checkout-1", header.Get(reliability.CorrelationIDHeader))
	assert.Equal(t, reliability.FromContext(ctx), reliability.ExtractHTTP(header))
}

func TestExtractHTTPRejectsUnsafeIDs(t *testing.T) {
	header := make(http.Header)
	header.Set(reliability.RequestIDHeader, "contains whitespace")
	header.Set(reliability.CorrelationIDHeader, string(make([]byte, 129)))
	assert.Empty(t, reliability.ExtractHTTP(header))
}
