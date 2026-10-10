package httpapi_test

import (
	"ecommerce/internal/apicontract"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSearchAnalyticsRegisteredOwnershipConsentAndAuthorization(t *testing.T) {
	router := searchRankingRouter(t)
	path := "/api/v1/search/impressions"
	body := fmt.Sprintf(`{"event_id":%q,"consent":true,"filters":{"q":"canvas","page":1,"limit":10}}`, uuid.NewString())
	assertRankingProblem(t, rankingRequest(t, router, "POST", path, body, "admin"), 400)
	assertRankingProblem(t, rankingRequest(t, router, "POST", path, `{"consent":false,"filters":{}}`, ""), 400)
	assertRankingProblem(t, rankingRequest(t, router, "POST", path, `{"consent":true,"filters":{}}`, ""), 400)
	assertRankingProblem(t, rankingRequest(t, router, "POST", path, fmt.Sprintf(`{"event_id":%q,"consent":true,"filters":null}`, uuid.NewString()), ""), 400)
	response := rankingRequest(t, router, "POST", path, body, "")
	require.Equal(t, 200, response.Code, response.Body.String())
	var im apicontract.SearchImpressionResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &im))
	require.Len(t, im.Result.Items, 1)
	event := fmt.Sprintf(`{"session_token":%q,"impression_id":%q,"event_id":%q,"type":"click","product_id":42,"position":1}`, im.SessionToken, im.ImpressionId, uuid.NewString())
	response = rankingRequest(t, router, "POST", "/api/v1/search/events", event, "")
	require.Equal(t, 200, response.Code, response.Body.String())
	response = rankingRequest(t, router, "POST", "/api/v1/search/events", event, "")
	require.Equal(t, 200, response.Code, response.Body.String())
	for _, role := range []string{"", "customer"} {
		status := 401
		if role != "" {
			status = 403
		}
		for _, path := range []string{"/api/v1/admin/search/analytics", "/api/v1/admin/search/operations"} {
			assertRankingProblem(t, rankingRequest(t, router, "GET", path, "", role), status)
		}
	}
	metrics := rankingRequest(t, router, "GET", "/api/v1/admin/search/analytics", "", "admin")
	require.Equal(t, 200, metrics.Code, metrics.Body.String())
	var analytics apicontract.SearchAnalytics
	require.NoError(t, json.Unmarshal(metrics.Body.Bytes(), &analytics))
	require.Equal(t, 1, analytics.Searches)
	require.Equal(t, 1, analytics.Clicks)
	require.Equal(t, float64(1), analytics.CtrAt5)
	response = rankingRequest(t, router, "POST", "/api/v1/search/consent/revoke", fmt.Sprintf(`{"session_token":%q}`, im.SessionToken), "")
	require.Equal(t, 204, response.Code, response.Body.String())
	assertRankingProblem(t, rankingRequest(t, router, "POST", "/api/v1/search/events", event, ""), 400)
}
