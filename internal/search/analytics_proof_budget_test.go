package search

import (
	"fmt"
	"strings"
	"testing"

	"ecommerce/models"
	"github.com/stretchr/testify/require"
)

func TestAnalyticsProofOversizedFiltersOmitOptionalToken(t *testing.T) {
	service, _ := analyticsFixture(t)
	filters := Filters{Page: 1, Limit: 10, AttributeValues: map[string][]string{}}
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("attribute%02d", i) + strings.Repeat("x", 109)
		for j := 0; j < 20; j++ {
			filters.AttributeValues[key] = append(filters.AttributeValues[key], strings.Repeat("v", 120))
		}
	}
	proof, err := service.SignResultSnapshot(Result{}, filters, 0)
	require.NoError(t, err)
	require.Empty(t, proof, "proof-size limits must not turn a valid search into an analytics error")
	filters.Limit = 101
	_, err = service.SignResultSnapshot(Result{}, filters, 0)
	require.ErrorIs(t, err, ErrAnalyticsInvalid)
	filters.Limit = 10
	_, err = service.SignResultSnapshot(Result{Products: []models.Product{{}}}, filters, 0)
	require.ErrorIs(t, err, ErrAnalyticsInvalid, "invalid exposure identities remain errors")
}
