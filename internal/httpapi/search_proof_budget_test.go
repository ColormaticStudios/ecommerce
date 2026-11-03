package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"ecommerce/internal/apicontract"
	"github.com/stretchr/testify/require"
)

func TestSearchAnalyticsValidOversizedExposureReturnsSearchWithoutToken(t *testing.T) {
	router := searchRankingRouter(t)
	query := url.Values{"q": {"canvas"}, "page": {"1"}, "limit": {"10"}}
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("attribute%02d", i) + strings.Repeat("x", 109)
		for j := 0; j < 20; j++ {
			query.Set(fmt.Sprintf("attribute[%s][%d]", key, j), strings.Repeat("v", 120))
		}
	}
	response := rankingRequest(t, router, "GET", "/api/v1/search/products?"+query.Encode(), "", "")
	require.Equal(t, 200, response.Code, response.Body.String())
	var result apicontract.ProductSearchResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	require.Nil(t, result.ResultToken)
	require.Empty(t, result.Items)
	require.False(t, result.Metadata.Degraded)
	require.Equal(t, 0, result.Pagination.Total)
	selected := 0
	for _, facet := range result.Facets {
		if strings.HasPrefix(facet.Name, "attribute:") {
			for _, value := range facet.Values {
				if value.Selected {
					selected++
				}
			}
		}
	}
	require.Equal(t, 20, selected, "accepted filters stay visible and removable")
	metrics := rankingRequest(t, router, "GET", "/api/v1/admin/search/analytics", "", "admin")
	require.Equal(t, 200, metrics.Code, metrics.Body.String())
	var analytics apicontract.SearchAnalytics
	require.NoError(t, json.Unmarshal(metrics.Body.Bytes(), &analytics))
	require.Zero(t, analytics.Searches, "omitting a proof must not mutate collected analytics")
}
