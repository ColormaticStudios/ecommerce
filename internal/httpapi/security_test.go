package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ecommerce/internal/httpapi"
	"ecommerce/internal/requestctx"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func signedToken(t *testing.T, secret, subject, role string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": subject, "role": role, "email": "admin@example.com", "exp": time.Now().Add(time.Hour).Unix(),
	})
	text, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return text
}

func TestJWTAuthenticatorResolvesBearerAndCookiePrincipals(t *testing.T) {
	auth := httpapi.JWTAuthenticator{Secret: []byte("secret")}
	token := signedToken(t, "secret", "subject-1", "admin")

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	principal, present, err := auth.Authenticate(request)
	require.NoError(t, err)
	assert.True(t, present)
	assert.Equal(t, "subject-1", principal.Subject)
	assert.Equal(t, "bearer", principal.AuthMethod)
	assert.True(t, principal.HasRole("admin"))

	request = httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	principal, present, err = auth.Authenticate(request)
	require.NoError(t, err)
	assert.True(t, present)
	assert.Equal(t, "cookie", principal.AuthMethod)
}

func TestJWTAuthenticatorDistinguishesMissingAndInvalidCredentials(t *testing.T) {
	auth := httpapi.JWTAuthenticator{Secret: []byte("secret")}
	_, present, err := auth.Authenticate(httptest.NewRequest(http.MethodGet, "/", nil))
	require.NoError(t, err)
	assert.False(t, present)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer invalid")
	_, present, err = auth.Authenticate(request)
	assert.True(t, present)
	assert.ErrorIs(t, err, httpapi.ErrInvalidAuthenticationToken)
}

func TestOperationSecurityRunsBeforeBodyBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	policies, err := httpapi.NewPolicySet(httpapi.OperationPolicy{
		OperationID: "CreateAdminBrand", Access: httpapi.AccessAuthenticated, Roles: []string{"admin"},
		CSRF: httpapi.CSRFRequired, MaxBodyBytes: 8,
	})
	require.NoError(t, err)
	security, err := httpapi.OperationSecurityMiddleware(policies, httpapi.Renderer{}, httpapi.SecurityOptions{
		Authenticator: httpapi.JWTAuthenticator{Secret: []byte("secret")},
	})
	require.NoError(t, err)

	router := gin.New()
	router.Use(httpapi.RequestContextMiddleware(httpapi.RequestContextOptions{NewID: func() string { return "req-1" }}), security)
	router.POST("/api/v1/admin/brands", func(ctx *gin.Context) {
		principal, ok := requestctx.PrincipalFrom(ctx.Request.Context())
		require.True(t, ok)
		assert.Equal(t, "CreateAdminBrand", mustMetadata(t, ctx).OperationID)
		_, readErr := ctx.GetRawData()
		assert.Error(t, readErr)
		assert.True(t, principal.HasRole("admin"))
		ctx.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/brands", bytes.NewBufferString(`{"name":"too long"}`))
	request.Header.Set("Authorization", "Bearer "+signedToken(t, "secret", "subject-1", "admin"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
}

func TestOperationSecurityPreservesExactWebhookBodyAndSignatureHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	policies, err := httpapi.ContractPolicySet()
	require.NoError(t, err)
	security, err := httpapi.OperationSecurityMiddleware(policies, httpapi.Renderer{}, httpapi.SecurityOptions{})
	require.NoError(t, err)

	router := gin.New()
	router.Use(httpapi.RequestContextMiddleware(httpapi.RequestContextOptions{NewID: func() string { return "req-webhook" }}), security)
	router.POST("/api/v1/webhooks/:provider", func(ctx *gin.Context) {
		metadata := mustMetadata(t, ctx)
		assert.Equal(t, "ReceiveWebhookEvent", metadata.OperationID)
		assert.Equal(t, "sig-v1=a,b", metadata.Headers["X-Provider-Signature"])
		assert.Equal(t, []byte("{\n  \"amount\": 10.00, \"note\": \"a\\u0026b\"\n}"), metadata.RawBody)
		boundBody, readErr := ctx.GetRawData()
		require.NoError(t, readErr)
		assert.Equal(t, metadata.RawBody, boundBody)
		ctx.Status(http.StatusNoContent)
	})

	body := []byte("{\n  \"amount\": 10.00, \"note\": \"a\\u0026b\"\n}")
	request := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/dummy", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Provider-Signature", "sig-v1=a,b")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
}

func TestOperationSecurityRejectsCookieMutationWithoutCSRF(t *testing.T) {
	gin.SetMode(gin.TestMode)
	policies, err := httpapi.NewPolicySet(httpapi.OperationPolicy{
		OperationID: "CreateAdminBrand", Access: httpapi.AccessAuthenticated, Roles: []string{"admin"},
		CSRF: httpapi.CSRFRequired, MaxBodyBytes: 1024,
	})
	require.NoError(t, err)
	security, err := httpapi.OperationSecurityMiddleware(policies, httpapi.Renderer{}, httpapi.SecurityOptions{
		Authenticator: httpapi.JWTAuthenticator{Secret: []byte("secret")},
	})
	require.NoError(t, err)
	router := gin.New()
	router.Use(httpapi.RequestContextMiddleware(httpapi.RequestContextOptions{NewID: func() string { return "req-1" }}), security)
	router.POST("/api/v1/admin/brands", func(ctx *gin.Context) { t.Fatal("handler must not run") })

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/brands", nil)
	request.AddCookie(&http.Cookie{Name: "session_token", Value: signedToken(t, "secret", "subject-1", "admin")})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	var problem httpapi.Problem
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &problem))
	assert.Equal(t, "csrf_failed", problem.Code)
}

func mustMetadata(t *testing.T, ctx *gin.Context) requestctx.Metadata {
	t.Helper()
	metadata, ok := requestctx.MetadataFrom(ctx.Request.Context())
	require.True(t, ok)
	return metadata
}
