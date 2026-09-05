package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"ecommerce/internal/httpapi"
	"ecommerce/internal/requestctx"
	"ecommerce/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccessLoggerIncludesRequestAndActorContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	router := gin.New()
	router.Use(httpapi.RequestContextMiddleware(httpapi.RequestContextOptions{NewID: func() string { return "request-1" }}))
	router.Use(middleware.AccessLogger(logger))
	router.GET("/things/:id", func(ctx *gin.Context) {
		requestContext := requestctx.WithOperation(ctx.Request.Context(), "GetThing")
		requestContext = requestctx.WithPrincipal(requestContext, requestctx.Principal{
			Subject: "subject-1", AccountID: 42, Roles: []string{"admin"}, AuthMethod: "bearer",
		})
		ctx.Request = ctx.Request.WithContext(requestContext)
		ctx.Status(http.StatusCreated)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/things/7", nil))

	var record map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &record))
	assert.Equal(t, "http_request_completed", record["event"])
	assert.Equal(t, "request-1", record["request_id"])
	assert.Equal(t, "request-1", record["correlation_id"])
	assert.Equal(t, "/things/:id", record["route"])
	assert.Equal(t, "GetThing", record["operation_id"])
	assert.Equal(t, float64(http.StatusCreated), record["status_code"])
	assert.Contains(t, record, "latency_ms")
	assert.Equal(t, "subject-1", record["actor_subject"])
	assert.Equal(t, float64(42), record["actor_account_id"])
	assert.Equal(t, "bearer", record["auth_method"])
}
