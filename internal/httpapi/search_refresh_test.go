package httpapi_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"
	"ecommerce/internal/jobs"
	"ecommerce/internal/search"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func searchRefreshRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db := catalogTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.JobQueue{}, &models.JobAttempt{}, &models.JobDeadLetter{}))
	runtime := jobs.NewRuntime(db, jobs.Config{})
	endpoints, err := httpapi.NewCatalogEndpoints(db, nil, runtime)
	require.NoError(t, err)
	policies, err := httpapi.ContractPolicySet()
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	require.NoError(t, httpapi.RegisterStrict(router, &httpapi.Server{CatalogEndpoints: endpoints}, httpapi.RegisterStrictOptions{Strict: httpapi.StrictOptions{Policies: policies}, Security: httpapi.SecurityOptions{Authenticator: httpapi.JWTAuthenticator{Secret: []byte("secret"), ResolveAccountID: func(context.Context, string) (uint, error) { return 17, nil }}}, RequestContext: httpapi.RequestContextOptions{NewID: func() string { return "ranking-request" }}}))
	return router, db
}

func TestSearchRefreshRegisteredAuthorizationAndDailyQueue(t *testing.T) {
	for _, signal := range []string{"sales", "conversion"} {
		t.Run(signal, func(t *testing.T) {
			router, db := searchRefreshRouter(t)
			path := "/api/v1/admin/search/refresh-" + signal
			assertRankingProblem(t, rankingRequest(t, router, "POST", path, "", ""), 401)
			assertRankingProblem(t, rankingRequest(t, router, "POST", path, "", "customer"), 403)
			var count int64
			require.NoError(t, db.Model(&models.JobQueue{}).Count(&count).Error)
			require.Zero(t, count)

			before := time.Now().UTC().Truncate(24 * time.Hour)
			response := rankingRequest(t, router, "POST", path, "", "admin")
			require.Equal(t, 202, response.Code, response.Body.String())
			var accepted apicontract.SearchRefreshAccepted
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &accepted))
			require.NotEmpty(t, accepted.JobId)
			require.Equal(t, apicontract.SearchRefreshAcceptedStatusQueued, accepted.Status)
			var job models.JobQueue
			require.NoError(t, db.First(&job, "id = ?", accepted.JobId).Error)
			require.Equal(t, "search."+signal+"_refresh", job.JobType)
			require.Equal(t, "pending", job.Status)
			require.Zero(t, job.AttemptCount)
			require.Equal(t, "ranking-request", job.CorrelationID)
			var payload search.SalesRefreshPayload
			require.NoError(t, json.Unmarshal([]byte(job.PayloadJSON), &payload))
			require.Equal(t, 1, payload.Version)
			require.True(t, payload.AsOf.Equal(before) || payload.AsOf.Equal(time.Now().UTC().Truncate(24*time.Hour)))
			require.Equal(t, signal+":"+payload.AsOf.Format("2006-01-02"), *job.IdempotencyKey)

			// Manual requests converge with the scheduler even after today's job finishes.
			require.NoError(t, db.Model(&job).Update("status", "succeeded").Error)
			repeated := rankingRequest(t, router, "POST", path, "", "admin")
			require.Equal(t, 202, repeated.Code, repeated.Body.String())
			var replay apicontract.SearchRefreshAccepted
			require.NoError(t, json.Unmarshal(repeated.Body.Bytes(), &replay))
			require.Equal(t, accepted.JobId, replay.JobId)
			require.NoError(t, db.Model(&models.JobQueue{}).Count(&count).Error)
			require.Equal(t, int64(1), count)
			require.NoError(t, db.First(&job, "id = ?", accepted.JobId).Error)
			require.Equal(t, "succeeded", job.Status)
		})
	}
}

func TestSearchRefreshQueueFailureUsesSafeProblem(t *testing.T) {
	router, db := searchRefreshRouter(t)
	require.NoError(t, db.Migrator().DropTable(&models.JobQueue{}))
	for _, signal := range []string{"sales", "conversion"} {
		response := rankingRequest(t, router, "POST", "/api/v1/admin/search/refresh-"+signal, "", "admin")
		assertRankingProblem(t, response, 500)
		require.False(t, strings.Contains(response.Body.String(), "no such table"))
	}
}

func TestSearchRefreshDailyPayloadConflict(t *testing.T) {
	router, db := searchRefreshRouter(t)
	for _, signal := range []string{"sales", "conversion"} {
		path := "/api/v1/admin/search/refresh-" + signal
		response := rankingRequest(t, router, "POST", path, "", "admin")
		require.Equal(t, 202, response.Code, response.Body.String())
		var accepted apicontract.SearchRefreshAccepted
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &accepted))
		require.NoError(t, db.Model(&models.JobQueue{}).Where("id = ?", accepted.JobId).Update("payload_fingerprint", "conflicting-payload").Error)
		response = rankingRequest(t, router, "POST", path, "", "admin")
		assertRankingProblem(t, response, 409)
		var problem httpapi.Problem
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &problem))
		require.Equal(t, "idempotency_conflict", problem.Code)
	}
}
