package apperror_test

import (
	"errors"
	"fmt"
	"testing"

	"ecommerce/internal/apperror"

	"github.com/stretchr/testify/require"
)

func TestValidationIncludesDefensiveIssueCopies(t *testing.T) {
	issues := []apperror.Issue{{
		Path:   "/items/0/quantity",
		Code:   "minimum",
		Detail: "Quantity must be at least 1.",
	}}

	err := apperror.Validation("validation_failed", "One or more values are invalid.", issues...)
	issues[0].Path = "/changed"

	require.Equal(t, apperror.KindValidation, apperror.KindOf(err))
	require.Equal(t, "validation_failed", apperror.CodeOf(err))
	require.Equal(t, "/items/0/quantity", apperror.IssuesOf(err)[0].Path)

	returned := apperror.IssuesOf(err)
	returned[0].Path = "/also-changed"
	require.Equal(t, "/items/0/quantity", apperror.IssuesOf(err)[0].Path)
}

func TestWrapRetainsCauseWithoutExposingIt(t *testing.T) {
	cause := errors.New("database password appeared in an internal error")
	err := apperror.Wrap(
		apperror.KindUnavailable,
		"dependency_unavailable",
		"A required dependency is unavailable.",
		cause,
	)
	wrapped := fmt.Errorf("complete checkout: %w", err)

	require.ErrorIs(t, wrapped, cause)
	require.True(t, apperror.IsKind(wrapped, apperror.KindUnavailable))
	require.Equal(t, "dependency_unavailable", apperror.CodeOf(wrapped))
	require.NotContains(t, err.Error(), cause.Error())
}

func TestUnknownErrorDefaultsToInternalClassification(t *testing.T) {
	err := errors.New("unclassified")

	require.Equal(t, apperror.KindInternal, apperror.KindOf(err))
	require.Empty(t, apperror.CodeOf(err))
	require.Nil(t, apperror.IssuesOf(err))
	require.False(t, apperror.IsKind(err, apperror.KindNotFound))
}

func TestWithIssuesDoesNotMutateOriginalError(t *testing.T) {
	original := apperror.New(apperror.KindInvalidInput, "invalid_request", "The request is invalid.")
	withIssues := original.WithIssues(apperror.Issue{Path: "", Code: "malformed", Detail: "Malformed input."})

	require.Empty(t, original.Issues)
	require.Len(t, withIssues.Issues, 1)
}
