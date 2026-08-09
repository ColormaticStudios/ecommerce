package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRepositoryStrictCutoverPrerequisites(t *testing.T) {
	require.NoError(t, validateRepository(filepath.Join("..", "..")))
}
