package httpcors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllowHeadersIncludesCheckoutMutationHeaders(t *testing.T) {
	headers := AllowHeaders()

	assert.Contains(t, headers, "Idempotency-Key")
	assert.Contains(t, headers, "X-CSRF-Token")
	assert.Contains(t, headers, "Authorization")
	assert.Contains(t, headers, "X-Request-ID")
	assert.Contains(t, headers, "X-Correlation-ID")
}

func TestExposeHeadersIncludesTusResumeOffset(t *testing.T) {
	assert.Contains(t, ExposeHeaders(), "Upload-Offset")
	assert.Contains(t, ExposeHeaders(), "X-Request-ID")
	assert.Contains(t, ExposeHeaders(), "X-Correlation-ID")
}

func TestAllowHeadersReturnsCopy(t *testing.T) {
	headers := AllowHeaders()
	headers[0] = "Modified"

	assert.Equal(t, "Origin", AllowHeaders()[0])
}
