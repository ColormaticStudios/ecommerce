package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"
	"ecommerce/internal/requestctx"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestStrictPolicyMiddlewareRecordsOperationAndAuthorizes(t *testing.T) {
	set, err := httpapi.NewPolicySet(httpapi.OperationPolicy{
		OperationID: "adminThing", Access: httpapi.AccessAuthenticated,
		Roles: []string{"admin"}, CSRF: httpapi.CSRFRequired, MaxBodyBytes: 1024,
	})
	require.NoError(t, err)

	called := false
	next := apicontract.StrictHandlerFunc(func(ctx *gin.Context, request interface{}) (interface{}, error) {
		called = true
		metadata, ok := requestctx.MetadataFrom(ctx.Request.Context())
		require.True(t, ok)
		assert.Equal(t, "adminThing", metadata.OperationID)
		return "response", nil
	})
	handler := httpapi.StrictPolicyMiddleware(set)(next, "adminThing")

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	requestContext := requestctx.WithPrincipal(context.Background(), requestctx.Principal{Subject: "subject-1", Roles: []string{"admin"}})
	ctx.Request = request.WithContext(requestContext)
	response, err := handler(ctx, struct{}{})
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "response", response)
}

func TestStrictErrorMiddlewarePreservesEndpointProblemStatus(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/products/42", nil)

	endpointError := httpapi.ErrorProblem(httpapi.Problem{
		Status: http.StatusNotFound,
		Code:   "not_found",
		Detail: "The requested catalog resource was not found.",
	}, gorm.ErrRecordNotFound)
	handler := httpapi.StrictErrorMiddleware(httpapi.Renderer{})(func(*gin.Context, interface{}) (interface{}, error) {
		return nil, endpointError
	}, "GetProduct")

	response, err := handler(ctx, struct{}{})
	require.NoError(t, err)
	assert.Nil(t, response)
	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Equal(t, httpapi.ProblemMediaType, recorder.Header().Get("Content-Type"))
	assert.True(t, ctx.IsAborted())
}

func TestStrictPolicyMiddlewareRejectsMissingPrincipal(t *testing.T) {
	set, err := httpapi.NewPolicySet(httpapi.OperationPolicy{
		OperationID: "adminThing", Access: httpapi.AccessAuthenticated,
		CSRF: httpapi.CSRFRequired, MaxBodyBytes: 1024,
	})
	require.NoError(t, err)
	handler := httpapi.StrictPolicyMiddleware(set)(func(*gin.Context, interface{}) (interface{}, error) {
		t.Fatal("next handler must not run")
		return nil, nil
	}, "adminThing")

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin", nil)
	_, err = handler(ctx, struct{}{})
	require.Error(t, err)
	var problemError *httpapi.ProblemError
	require.ErrorAs(t, err, &problemError)
	assert.Equal(t, http.StatusUnauthorized, problemError.Problem.Status)
}
