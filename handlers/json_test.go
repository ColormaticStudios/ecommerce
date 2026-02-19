package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type strictBindRequest struct {
	Email string `json:"email" binding:"required,email"`
	Count int    `json:"count" binding:"required,min=1"`
}

func newJSONContext(body string) *gin.Context {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx
}

func TestBindStrictJSON_EnforcesValidationTags(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var req strictBindRequest
	err := bindStrictJSON(newJSONContext(`{"count":1}`), &req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Email")

	err = bindStrictJSON(newJSONContext(`{"email":"not-an-email","count":1}`), &req)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "email")
}

func TestBindStrictJSON_RejectsUnknownFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var req strictBindRequest
	err := bindStrictJSON(newJSONContext(`{"email":"ok@example.com","count":1,"extra":true}`), &req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown field")
}

func TestBindStrictJSON_RejectsMultipleJSONObjects(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var req strictBindRequest
	err := bindStrictJSON(newJSONContext(`{"email":"ok@example.com","count":1}{"email":"x@y.com","count":2}`), &req)
	require.Error(t, err)
	assert.Equal(t, "request body must contain a single JSON object", err.Error())
}

func TestBindStrictJSON_AcceptsValidPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var req strictBindRequest
	err := bindStrictJSON(newJSONContext(`{"email":"ok@example.com","count":2}`), &req)
	require.NoError(t, err)
	assert.Equal(t, "ok@example.com", req.Email)
	assert.Equal(t, 2, req.Count)
}
