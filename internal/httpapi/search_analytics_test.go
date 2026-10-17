package httpapi_test

import (
	"ecommerce/internal/apicontract"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchAnalyticsRegisteredOwnershipConsentAndAuthorization(t *testing.T) {
	router := searchRankingRouter(t)
	path := "/api/v1/search/impressions"
	searchResponse := rankingRequest(t, router, "GET", "/api/v1/search/products?q=canvas&page=1&limit=10", "", "")
	require.Equal(t, 200, searchResponse.Code, searchResponse.Body.String())
	var searchResult apicontract.ProductSearchResponse
	require.NoError(t, json.Unmarshal(searchResponse.Body.Bytes(), &searchResult))
	require.NotNil(t, searchResult.ResultToken)
	adminSearch := rankingRequest(t, router, "GET", "/api/v1/search/products?q=canvas", "", "admin")
	require.Equal(t, 200, adminSearch.Code, adminSearch.Body.String())
	var excluded apicontract.ProductSearchResponse
	require.NoError(t, json.Unmarshal(adminSearch.Body.Bytes(), &excluded))
	require.Nil(t, excluded.ResultToken)
	body := fmt.Sprintf(`{"event_id":%q,"consent":true,"result_token":%q}`, uuid.NewString(), *searchResult.ResultToken)
	assertRankingProblem(t, rankingRequest(t, router, "POST", path, body, "admin"), 403)
	assertRankingProblem(t, rankingRequest(t, router, "POST", path, `{"consent":false,"filters":{}}`, ""), 400)
	assertRankingProblem(t, rankingRequest(t, router, "POST", path, `{"consent":true,"filters":{}}`, ""), 400)
	assertRankingProblem(t, rankingRequest(t, router, "POST", path, fmt.Sprintf(`{"event_id":%q,"consent":true,"filters":null}`, uuid.NewString()), ""), 400)
	response := rankingRequest(t, router, "POST", path, body, "")
	require.Equal(t, 200, response.Code, response.Body.String())
	var im apicontract.SearchImpressionResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &im))
	require.NotEmpty(t, im.ImpressionId)
	event := fmt.Sprintf(`{"session_token":%q,"impression_id":%q,"event_id":%q,"type":"click","product_id":42,"position":1}`, im.SessionToken, im.ImpressionId, uuid.NewString())
	assertRankingProblem(t, rankingRequest(t, router, "POST", "/api/v1/search/events", event, "admin"), 403)
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

func TestSearchAnalyticsClientBudgetCannotBeResetWithSessionOrForwardedHeader(t *testing.T) {
	router := searchRankingRouter(t)
	// Only explicitly trusted ingress may determine ClientIP.
	require.NoError(t, router.SetTrustedProxies(nil))
	searchResponse := rankingRequest(t, router, "GET", "/api/v1/search/products?q=canvas", "", "")
	require.Equal(t, 200, searchResponse.Code, searchResponse.Body.String())
	var snapshot apicontract.ProductSearchResponse
	require.NoError(t, json.Unmarshal(searchResponse.Body.Bytes(), &snapshot))
	require.NotNil(t, snapshot.ResultToken)
	post := func(ip, forwarded string) *httptest.ResponseRecorder {
		body := fmt.Sprintf(`{"event_id":%q,"consent":true,"result_token":%q}`, uuid.NewString(), *snapshot.ResultToken)
		request := httptest.NewRequest("POST", "/api/v1/search/impressions", strings.NewReader(body))
		request.RemoteAddr = ip + ":1234"
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Forwarded-For", forwarded)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	var first apicontract.SearchImpressionResponse
	for i := 0; i < 20; i++ {
		// Every request gets a fresh session; neither sessions nor spoofed IPs
		// supply the admission identity.
		response := post("192.0.2.1", fmt.Sprintf("198.51.100.%d", i+1))
		require.Equal(t, 200, response.Code, response.Body.String())
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &fields))
		require.NotContains(t, fields, "result")
		if i == 0 {
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &first))
		}
	}
	limited := post("192.0.2.1", "203.0.113.1")
	assertRankingProblem(t, limited, 429)
	require.NotEmpty(t, limited.Header().Get("Retry-After"))
	require.Equal(t, 200, post("192.0.2.2", "203.0.113.1").Code)
	// Exhausting analytics writes does not starve searches or consent withdrawal.
	require.Equal(t, 200, rankingRequest(t, router, "GET", "/api/v1/search/products?q=canvas", "", "").Code)
	metrics := rankingRequest(t, router, "GET", "/api/v1/admin/search/analytics?limit=1&page=2", "", "admin")
	require.Equal(t, 200, metrics.Code, metrics.Body.String())
	var analytics apicontract.SearchAnalytics
	require.NoError(t, json.Unmarshal(metrics.Body.Bytes(), &analytics))
	require.Equal(t, 21, analytics.Searches)
	require.Empty(t, analytics.Queries)
	require.Equal(t, 1, analytics.Pagination.Total)
	revoke := rankingRequest(t, router, "POST", "/api/v1/search/consent/revoke", fmt.Sprintf(`{"session_token":%q}`, first.SessionToken), "")
	require.Equal(t, 204, revoke.Code, revoke.Body.String())
}
