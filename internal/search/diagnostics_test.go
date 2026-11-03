package search

import (
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestErrorDiagnosticIncludesSafeMessageAndSQLState(t *testing.T) {
	err := fmt.Errorf("refresh: %w", &pgconn.PgError{Code: "22P02", Message: `invalid input syntax for type numeric: 'private-cost'`, Detail: "password=do-not-log", Hint: "raw-query"})
	value := ErrorDiagnostic(err)
	require.Contains(t, value, "SQLSTATE 22P02")
	require.Contains(t, value, "invalid input syntax for type numeric")
	require.NotContains(t, value, "private-cost")
	require.NotContains(t, value, "do-not-log")
	require.NotContains(t, value, "raw-query")
	value = ErrorDiagnostic(fmt.Errorf("cannot connect postgres://account:private-password@db/internal token=private-token"))
	require.Contains(t, value, "cannot connect")
	require.NotContains(t, value, "private-password")
	require.NotContains(t, value, "private-token")
}
