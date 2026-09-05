package reliability_test

import (
	"errors"
	"fmt"
	"testing"

	"ecommerce/internal/reliability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyPreservesErrorChain(t *testing.T) {
	sentinel := errors.New("provider unavailable")
	err := reliability.Classify(fmt.Errorf("send: %w", sentinel), reliability.ClassRetryable)

	assert.ErrorIs(t, err, sentinel)
	class, ok := reliability.ErrorClassOf(err)
	require.True(t, ok)
	assert.Equal(t, reliability.ClassRetryable, class)
}

func TestClassifyNilAndUnclassifiedErrors(t *testing.T) {
	assert.NoError(t, reliability.Classify(nil, reliability.ClassTerminal))
	class, ok := reliability.ErrorClassOf(errors.New("plain"))
	assert.False(t, ok)
	assert.Empty(t, class)
}

func TestClassifyRejectsUnknownClass(t *testing.T) {
	sentinel := errors.New("plain")
	err := reliability.Classify(sentinel, reliability.ErrorClass("unknown"))
	assert.ErrorIs(t, err, sentinel)
	class, ok := reliability.ErrorClassOf(err)
	assert.True(t, ok)
	assert.Equal(t, reliability.ClassTerminal, class)
}
