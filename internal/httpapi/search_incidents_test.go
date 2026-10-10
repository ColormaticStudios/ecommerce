package httpapi_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func incidentRouter(t *testing.T) *gin.Engine {
	db := catalogTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SearchIndexIncident{}, &models.SearchFreshnessObservation{}))
	opened := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	recovered := opened.Add(time.Hour)
	require.NoError(t, db.Create(&[]models.SearchIndexIncident{{IndexName: "products", Reason: "index_lag", OpenedAt: opened, DetectedAt: opened, LastObservedAt: recovered, RecoveredAt: &recovered, MaxLagSeconds: 600, MaxPendingJobs: 5, DocumentCount: 20}, {IndexName: "products", Reason: "missing_baseline", OpenedAt: recovered, DetectedAt: recovered, LastObservedAt: recovered}}).Error)
	endpoints, err := httpapi.NewCatalogEndpoints(db, nil)
	require.NoError(t, err)
	policies, err := httpapi.ContractPolicySet()
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	require.NoError(t, httpapi.RegisterStrict(router, &httpapi.Server{CatalogEndpoints: endpoints}, httpapi.RegisterStrictOptions{Strict: httpapi.StrictOptions{Policies: policies}, Security: httpapi.SecurityOptions{Authenticator: httpapi.JWTAuthenticator{Secret: []byte("secret"), ResolveAccountID: func(context.Context, string) (uint, error) { return 17, nil }}}, RequestContext: httpapi.RequestContextOptions{NewID: func() string { return "ranking-request" }}}))
	return router
}
func TestSearchIncidentsRegisteredAuthorizationFilteringAndPagination(t *testing.T) {
	router := incidentRouter(t)
	path := "/api/v1/admin/search/incidents"
	assertRankingProblem(t, rankingRequest(t, router, "GET", path, "", ""), 401)
	assertRankingProblem(t, rankingRequest(t, router, "GET", path, "", "customer"), 403)
	for _, query := range []string{"?page=0", "?limit=101", "?status=bad"} {
		assertRankingProblem(t, rankingRequest(t, router, "GET", path+query, "", "admin"), 400)
	}
	response := rankingRequest(t, router, "GET", path+"?page=1&limit=1", "", "admin")
	require.Equal(t, 200, response.Code, response.Body.String())
	var data apicontract.SearchIndexIncidentListResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &data))
	require.Len(t, data.Data, 1)
	require.Equal(t, 2, data.Pagination.Total)
	require.Equal(t, 2, data.Pagination.TotalPages)
	require.Equal(t, "open", string(data.Data[0].Status))
	response = rankingRequest(t, router, "GET", path+"?status=resolved", "", "admin")
	require.Equal(t, 200, response.Code, response.Body.String())
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &data))
	require.Len(t, data.Data, 1)
	require.Equal(t, int64(3600), data.Data[0].DurationSeconds)
	require.Equal(t, int64(600), data.Data[0].MaxLagSeconds)
	response = rankingRequest(t, router, "GET", path+"?page=3&limit=1", "", "admin")
	require.Equal(t, 200, response.Code, response.Body.String())
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &data))
	require.Empty(t, data.Data)
}
