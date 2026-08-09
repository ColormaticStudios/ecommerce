package httpapi_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"ecommerce/internal/httpapi"
	"ecommerce/internal/requestctx"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoundaryRendersGeneratedStrictError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(
		httpapi.RequestContextMiddleware(httpapi.RequestContextOptions{NewID: func() string { return "req-1" }}),
		httpapi.Boundary(httpapi.Renderer{}),
	)
	router.GET("/failure", func(ctx *gin.Context) {
		ctx.Error(errors.New("decoder internals"))
		ctx.Status(http.StatusBadRequest)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/failure", nil))

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, httpapi.ProblemMediaType, recorder.Header().Get("Content-Type"))
	assert.NotContains(t, recorder.Body.String(), "decoder internals")
	var problem httpapi.Problem
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &problem))
	assert.Equal(t, "req-1", problem.CorrelationID)
}

func TestBoundaryRecoversWithoutLeakingPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(httpapi.Boundary(httpapi.Renderer{}))
	router.GET("/panic", func(*gin.Context) { panic("database password") })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Equal(t, httpapi.ProblemMediaType, recorder.Header().Get("Content-Type"))
	assert.NotContains(t, recorder.Body.String(), "database password")
}

func TestRequestContextBridgesLegacyPrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set("userID", "subject-1")
		ctx.Set("userEmail", "person@example.com")
		ctx.Set("userRole", "admin")
		ctx.Next()
	})
	router.Use(httpapi.RequestContextMiddleware(httpapi.RequestContextOptions{
		NewID:            func() string { return "req-1" },
		ResolvePrincipal: httpapi.LegacyGinPrincipal,
	}))
	router.GET("/principal", func(ctx *gin.Context) {
		principal, ok := requestctx.PrincipalFrom(ctx.Request.Context())
		require.True(t, ok)
		assert.Equal(t, "subject-1", principal.Subject)
		assert.True(t, principal.HasRole("admin"))
		ctx.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/principal", nil))
	assert.Equal(t, "req-1", recorder.Header().Get(httpapi.RequestIDHeader))
	assert.Equal(t, "req-1", recorder.Header().Get(httpapi.CorrelationIDHeader))
}

func TestGeneratedBindingErrorHandlerUsesProblemMediaType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/binding", func(ctx *gin.Context) {
		httpapi.GeneratedBindingErrorHandler(httpapi.Renderer{})(ctx, errors.New("strconv details"), http.StatusBadRequest)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/binding", nil))
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, httpapi.ProblemMediaType, recorder.Header().Get("Content-Type"))
	assert.NotContains(t, recorder.Body.String(), "strconv details")
}
