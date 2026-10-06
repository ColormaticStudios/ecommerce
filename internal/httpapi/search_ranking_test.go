package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"
	searchservice "ecommerce/internal/search"
	"ecommerce/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func searchRankingRouter(t *testing.T) *gin.Engine {
	t.Helper()
	db := catalogTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SearchRankingProfile{}, &models.SearchSynonymSet{}, &models.SearchTypoToleranceProfile{}, &models.SearchDocument{}, &models.SearchIndexState{}, &models.SearchSalesSignal{}, &models.Product{}))
	service := searchservice.NewService(db, nil, nil)
	_, err := service.CreateRankingProfile(context.Background(), searchservice.RankingProfileInput{Name: "default", IsDefault: true, Weights: searchservice.RankingWeights{TokenCoverage: 10, Name: 10, ExactPhrase: 10}}, nil)
	require.NoError(t, err)
	_, err = service.CreateTypoToleranceProfile(context.Background(), searchservice.TypoToleranceProfileInput{Name: "default", MinimumTokenLength: 4, OneEditMinimumLength: 4, TwoEditMinimumLength: 8, IsActive: true})
	require.NoError(t, err)
	product := models.Product{BaseModel: models.BaseModel{ID: 42}, Name: "Canvas Bag", IsPublished: true, Related: []models.Product{}}
	require.NoError(t, db.Create(&product).Error)
	require.NoError(t, db.Create(&models.LocalizedEntityValue{EntityType: "product", EntityID: 42, LocaleID: 1, Field: "name", Value: product.Name}).Error)
	payload, err := json.Marshal(product)
	require.NoError(t, err)
	now := time.Now().UTC()
	require.NoError(t, searchservice.NewDatabaseBackend(db).Upsert(context.Background(), models.SearchDocument{EntityType: "product", EntityID: 42, PayloadJSON: string(payload), SearchableText: "canvas bag", NormalizedName: "canvas bag", Active: true, Version: 1, SourceCreatedAt: now, SourceUpdatedAt: now, IndexedAt: now}))
	endpoints, err := httpapi.NewCatalogEndpoints(db, nil)
	require.NoError(t, err)
	policies, err := httpapi.ContractPolicySet()
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	require.NoError(t, httpapi.RegisterStrict(router, &httpapi.Server{CatalogEndpoints: endpoints}, httpapi.RegisterStrictOptions{Strict: httpapi.StrictOptions{Policies: policies}, Security: httpapi.SecurityOptions{Authenticator: httpapi.JWTAuthenticator{Secret: []byte("secret")}}, RequestContext: httpapi.RequestContextOptions{NewID: func() string { return "ranking-request" }}}))
	return router
}

func rankingRequest(t *testing.T, router *gin.Engine, method, path, body, role string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if role != "" {
		request.Header.Set("Authorization", "Bearer "+signedToken(t, "secret", "admin", role))
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
func assertRankingProblem(t *testing.T, response *httptest.ResponseRecorder, status int) {
	t.Helper()
	require.Equal(t, status, response.Code, response.Body.String())
	require.Equal(t, httpapi.ProblemMediaType, response.Header().Get("Content-Type"))
	var problem httpapi.Problem
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &problem))
	require.Equal(t, "ranking-request", problem.CorrelationID)
}
func TestSearchRankingRegisteredCRUDAndValidation(t *testing.T) {
	router := searchRankingRouter(t)
	path := "/api/v1/admin/search/ranking-profiles"
	for _, role := range []string{"", "customer"} {
		status := 401
		if role != "" {
			status = 403
		}
		assertRankingProblem(t, rankingRequest(t, router, "GET", path, "", role), status)
	}
	for _, invalidBody := range []string{`{"name":"broken","weights":{"name":10}}`, `{"name":"broken","weights":null}`, `{"name":"broken"}`, `{"name":`} {
		assertRankingProblem(t, rankingRequest(t, router, "POST", path, invalidBody, "admin"), 400)
	}
	weights := apicontract.SearchRankingWeights{Name: 5, Recency: 10}
	input := apicontract.SearchRankingProfileInput{Name: "new_arrivals", Weights: rankingWeightsInput(weights)}
	body, err := json.Marshal(input)
	require.NoError(t, err)
	created := rankingRequest(t, router, "POST", path, string(body), "admin")
	require.Equal(t, 201, created.Code, created.Body.String())
	var profile apicontract.SearchRankingProfile
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &profile))
	require.False(t, profile.IsDefault)
	require.Equal(t, 1, profile.Version)
	profilePath := fmt.Sprintf("%s/%d", path, profile.Id)
	updated := rankingRequest(t, router, "PATCH", profilePath, `{"is_default":true}`, "admin")
	require.Equal(t, 200, updated.Code, updated.Body.String())
	require.NoError(t, json.Unmarshal(updated.Body.Bytes(), &profile))
	require.True(t, profile.IsDefault)
	require.Equal(t, 2, profile.Version)
	require.Equal(t, weights, profile.Weights)
	assertRankingProblem(t, rankingRequest(t, router, "PATCH", profilePath, `{"is_default":false}`, "admin"), 409)
	assertRankingProblem(t, rankingRequest(t, router, "DELETE", profilePath, "", "admin"), 409)
	invalidWeights := weights
	invalidWeights.Name = 101
	patchBody, err := json.Marshal(apicontract.SearchRankingProfilePatch{Weights: rankingWeightsPointer(invalidWeights)})
	require.NoError(t, err)
	assertRankingProblem(t, rankingRequest(t, router, "PATCH", profilePath, string(patchBody), "admin"), 422)
	require.Equal(t, 200, rankingRequest(t, router, "PATCH", path+"/1", `{"is_default":true}`, "admin").Code)
	require.Equal(t, 204, rankingRequest(t, router, "DELETE", profilePath, "", "admin").Code)
	assertRankingProblem(t, rankingRequest(t, router, "GET", profilePath, "", "admin"), 404)
	require.Equal(t, 200, rankingRequest(t, router, "GET", path, "", "admin").Code)
}
func TestSearchRankingExplainIsAdminOnlyAndPublicMetadata(t *testing.T) {
	router := searchRankingRouter(t)
	adminPath := "/api/v1/admin/search/products?q=canvas"
	assertRankingProblem(t, rankingRequest(t, router, "GET", adminPath, "", ""), 401)
	assertRankingProblem(t, rankingRequest(t, router, "GET", adminPath, "", "customer"), 403)
	admin := rankingRequest(t, router, "GET", adminPath, "", "admin")
	require.Equal(t, 200, admin.Code, admin.Body.String())
	var explanation apicontract.AdminProductSearchResponse
	require.NoError(t, json.Unmarshal(admin.Body.Bytes(), &explanation))
	require.Len(t, explanation.Explanations, 1)
	require.Equal(t, 42, explanation.Explanations[0].ProductId)
	require.Len(t, explanation.Explanations[0].Components, 8)
	total := 0.0
	for _, component := range explanation.Explanations[0].Components {
		require.False(t, math.IsNaN(component.Value))
		total += component.Contribution
	}
	require.InDelta(t, total, explanation.Explanations[0].Score, 0.000001)
	public := rankingRequest(t, router, "GET", "/api/v1/search/products?q=canvas&ranking_profile=default", "", "")
	require.Equal(t, 200, public.Code, public.Body.String())
	require.False(t, bytes.Contains(public.Body.Bytes(), []byte(`"explanations"`)))
	var result apicontract.ProductSearchResponse
	require.NoError(t, json.Unmarshal(public.Body.Bytes(), &result))
	require.Equal(t, "default", result.Metadata.RankingProfile)
	require.Equal(t, 1, result.Metadata.RankingProfileVersion)
	assertRankingProblem(t, rankingRequest(t, router, "GET", "/api/v1/search/products?ranking_profile=unknown", "", ""), 404)
	assertRankingProblem(t, rankingRequest(t, router, "GET", "/api/v1/search/products?ranking_profile=%20", "", ""), 400)
}

func rankingWeightsInput(value apicontract.SearchRankingWeights) apicontract.SearchRankingWeightsInput {
	return apicontract.SearchRankingWeightsInput{TokenCoverage: &value.TokenCoverage, ExactPhrase: &value.ExactPhrase, Name: &value.Name, Brand: &value.Brand, Attributes: &value.Attributes, Recency: &value.Recency, Availability: &value.Availability, Sales: &value.Sales}
}
func rankingWeightsPointer(value apicontract.SearchRankingWeights) *apicontract.SearchRankingWeightsInput {
	input := rankingWeightsInput(value)
	return &input
}
