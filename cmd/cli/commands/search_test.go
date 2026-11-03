package commands

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRootIncludesSearchReindexCommand(t *testing.T) {
	root := newRootCmd()
	searchCommand, _, err := root.Find([]string{"search", "reindex"})
	require.NoError(t, err)
	require.Equal(t, "reindex", searchCommand.Name())
}
