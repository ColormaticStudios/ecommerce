package httpapi

import (
	"errors"
	"net/http"
	"testing"

	"ecommerce/internal/services/orders"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckoutEndpointErrorMapsGuestClaimErrors(t *testing.T) {
	testCases := []struct {
		name           string
		err            error
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "already claimed conflict",
			err:            orders.ErrOrderAlreadyClaimed,
			expectedStatus: http.StatusConflict,
			expectedCode:   "order_already_claimed",
		},
		{
			name:           "invalid claim bad request",
			err:            orders.ErrInvalidClaim,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "invalid_claim",
		},
		{
			name:           "unknown order not found",
			err:            orders.ErrOrderNotFound,
			expectedStatus: http.StatusNotFound,
			expectedCode:   "not_found",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := checkoutEndpointError(testCase.err)
			require.Error(t, err)

			var problem *ProblemError
			require.ErrorAs(t, err, &problem)
			assert.Equal(t, testCase.expectedStatus, problem.Problem.Status)
			assert.Equal(t, testCase.expectedCode, problem.Problem.Code)
			assert.Equal(t, "errors."+testCase.expectedCode, problem.Problem.MessageKey)
			assert.ErrorIs(t, err, testCase.err)
		})
	}
}

func TestCheckoutEndpointErrorPassesThroughUnknownErrors(t *testing.T) {
	unknown := errors.New("storage failure")

	err := checkoutEndpointError(unknown)

	var problem *ProblemError
	assert.False(t, errors.As(err, &problem))
	assert.ErrorIs(t, err, unknown)
}
