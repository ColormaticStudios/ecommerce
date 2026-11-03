package httpapi_test

import (
	"ecommerce/internal/apicontract"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConfigurationHistoryRegisteredSecuritySnapshotsAndPagination(t *testing.T) {
	router := searchRankingRouter(t)
	path := "/api/v1/admin/search/configuration-history"
	assertRankingProblem(t, rankingRequest(t, router, "GET", path, "", ""), 401)
	assertRankingProblem(t, rankingRequest(t, router, "GET", path, "", "customer"), 403)
	for _, query := range []string{"?page=0", "?page=1000001", "?limit=0", "?limit=101", "?entity_id=1", "?kind=invalid"} {
		assertRankingProblem(t, rankingRequest(t, router, "GET", path+query, "", "admin"), 400)
	}
	response := rankingRequest(t, router, "POST", "/api/v1/admin/search/synonyms", `{"name":"History test","direction":"bi","terms":["shoe","trainer"]}`, "admin")
	require.Equal(t, 201, response.Code, response.Body.String())
	var value apicontract.SearchSynonymSet
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &value))
	response = rankingRequest(t, router, "DELETE", fmt.Sprintf("/api/v1/admin/search/synonyms/%d", value.Id), "", "admin")
	require.Equal(t, 204, response.Code, response.Body.String())
	response = rankingRequest(t, router, "GET", fmt.Sprintf("%s?kind=synonym&entity_id=%d&page=1&limit=1", path, value.Id), "", "admin")
	require.Equal(t, 200, response.Code, response.Body.String())
	var result apicontract.SearchConfigurationAuditListResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	require.EqualValues(t, 2, result.Pagination.Total)
	require.Len(t, result.Items, 1)
	require.Equal(t, apicontract.SearchConfigurationAuditOperation("delete"), result.Items[0].Operation)
	require.Equal(t, 17, *result.Items[0].ActorId)
	require.Nil(t, result.Items[0].After)
	require.NotNil(t, result.Items[0].Before)
}
