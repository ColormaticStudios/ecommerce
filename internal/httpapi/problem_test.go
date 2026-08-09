package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"
	"ecommerce/internal/requestctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdaptContractLegacyError(t *testing.T) {
	code := "email_taken"
	problem, err := httpapi.AdaptContract(apicontract.Error{Error: "email already exists", Code: &code}, http.StatusConflict)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, problem.Status)
	assert.Equal(t, "email_taken", problem.Code)
	assert.Equal(t, "email already exists", problem.Detail)
}

func TestAdaptContractFutureGeneratedProblemShape(t *testing.T) {
	type generatedProblem struct {
		Type   string `json:"type"`
		Title  string `json:"title"`
		Status int    `json:"status"`
		Code   string `json:"code"`
	}
	problem, err := httpapi.AdaptContract(generatedProblem{
		Type: "urn:problem:teapot", Title: "Teapot", Status: http.StatusTeapot, Code: "teapot",
	}, http.StatusInternalServerError)
	require.NoError(t, err)
	assert.Equal(t, http.StatusTeapot, problem.Status)
	assert.Equal(t, "teapot", problem.Code)
}

func TestRendererAdaptsAndWritesLegacyContractError(t *testing.T) {
	code := "email_taken"
	recorder := httptest.NewRecorder()
	err := (httpapi.Renderer{}).RenderContract(recorder, context.Background(), http.StatusConflict, apicontract.Error{
		Error: "email already exists",
		Code:  &code,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.Equal(t, httpapi.ProblemMediaType, recorder.Header().Get("Content-Type"))

	var problem httpapi.Problem
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &problem))
	assert.Equal(t, "email_taken", problem.Code)
}

func TestRendererDoesNotLeakInternalError(t *testing.T) {
	ctx := requestctx.WithMetadata(context.Background(), requestctx.Metadata{
		RequestID: "req-1", CorrelationID: "corr-1",
	})
	recorder := httptest.NewRecorder()
	httpapi.Renderer{}.Render(recorder, ctx, http.StatusInternalServerError, errors.New("sql: secret table failed"))

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Equal(t, httpapi.ProblemMediaType, recorder.Header().Get("Content-Type"))
	assert.NotContains(t, recorder.Body.String(), "secret table")
	var problem httpapi.Problem
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &problem))
	assert.Equal(t, "internal_error", problem.Code)
	assert.Equal(t, "corr-1", problem.CorrelationID)
	assert.Equal(t, "urn:request:req-1", problem.Instance)
}

func TestRendererUsesExplicitSafeProblem(t *testing.T) {
	recorder := httptest.NewRecorder()
	err := httpapi.ErrorProblem(httpapi.Problem{
		Status: http.StatusUnprocessableEntity,
		Code:   "invalid_email",
		Detail: "Email is invalid.",
	}, errors.New("validator details"))
	httpapi.Renderer{}.Render(recorder, context.Background(), http.StatusInternalServerError, err)

	var problem httpapi.Problem
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &problem))
	assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	assert.Equal(t, "invalid_email", problem.Code)
	assert.Equal(t, "Email is invalid.", problem.Detail)
}
