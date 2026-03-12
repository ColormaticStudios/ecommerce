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
}

func TestAllowHeadersReturnsCopy(t *testing.T) {
	headers := AllowHeaders()
	headers[0] = "Modified"

	assert.Equal(t, "Origin", AllowHeaders()[0])
}
