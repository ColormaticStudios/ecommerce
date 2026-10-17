package httpapi_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"ecommerce/internal/apicontract"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSearchCacheableProofRegisteredContract(t *testing.T) {
	router := searchRankingRouter(t)
	response := rankingRequest(t, router, "GET", "/api/v1/search/products?q=canvas", "", "")
	require.Equal(t, 200, response.Code, response.Body.String())
	var result apicontract.ProductSearchResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	require.NotNil(t, result.ResultSnapshot)
	event := uuid.NewString()
	body := fmt.Sprintf(`{"snapshot_token":%q,"consent":true,"event_id":%q}`, *result.ResultSnapshot, event)
	path := "/api/v1/search/proofs"
	response = rankingRequest(t, router, "POST", path, body, "")
	require.Equal(t, 200, response.Code, response.Body.String())
	require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
	var proof apicontract.SearchProofResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &proof))
	require.NotEmpty(t, proof.ResultToken)
	impression := fmt.Sprintf(`{"result_token":%q,"consent":true,"event_id":%q}`, proof.ResultToken, event)
	require.Equal(t, 200, rankingRequest(t, router, "POST", "/api/v1/search/impressions", impression, "").Code)
	assertRankingProblem(t, rankingRequest(t, router, "POST", path, body, "admin"), 403)
	assertRankingProblem(t, rankingRequest(t, router, "POST", path, fmt.Sprintf(`{"snapshot_token":%q,"consent":false,"event_id":%q}`, *result.ResultSnapshot, event), ""), 400)
	assertRankingProblem(t, rankingRequest(t, router, "POST", path, fmt.Sprintf(`{"snapshot_token":%q,"consent":true,"event_id":%q}`, *result.ResultToken, event), ""), 400)
	response = rankingRequest(t, router, "GET", "/api/v1/search/suggestions?q=canvas", "", "")
	require.Equal(t, 200, response.Code, response.Body.String())
	var suggestions map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &suggestions))
	require.NotContains(t, suggestions, "popular")
	require.NotContains(t, suggestions, "trending")
}
