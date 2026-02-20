package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const previewTestSecret = "preview-test-secret"

func previewContextWithCookie(t *testing.T, cookie *http.Cookie) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	c.Request = req
	return c
}

func TestParseDraftPreviewCookieValidAndExpired(t *testing.T) {
	validToken, expiresAt, err := buildDraftPreviewToken("subject-1", "admin", previewTestSecret, 10*time.Minute)
	require.NoError(t, err)

	validCtx := previewContextWithCookie(t, &http.Cookie{
		Name:  draftPreviewCookieName,
		Value: validToken,
	})
	session, ok := parseDraftPreviewCookie(validCtx, previewTestSecret)
	require.True(t, ok)
	assert.Equal(t, "subject-1", session.Subject)
	assert.WithinDuration(t, expiresAt, session.ExpiresAt, time.Second)

	expiredToken, _, err := buildDraftPreviewToken("subject-1", "admin", previewTestSecret, -time.Minute)
	require.NoError(t, err)
	expiredCtx := previewContextWithCookie(t, &http.Cookie{
		Name:  draftPreviewCookieName,
		Value: expiredToken,
	})
	_, expiredOk := parseDraftPreviewCookie(expiredCtx, previewTestSecret)
	assert.False(t, expiredOk)
}

func TestParseDraftPreviewCookieRejectsNonAdminRole(t *testing.T) {
	token, _, err := buildDraftPreviewToken("subject-2", "customer", previewTestSecret, 10*time.Minute)
	require.NoError(t, err)

	ctx := previewContextWithCookie(t, &http.Cookie{
		Name:  draftPreviewCookieName,
		Value: token,
	})
	_, ok := parseDraftPreviewCookie(ctx, previewTestSecret)
	assert.False(t, ok)
}

func TestEnableDraftPreviewContextSetsHeaders(t *testing.T) {
	token, _, err := buildDraftPreviewToken("subject-3", "admin", previewTestSecret, 10*time.Minute)
	require.NoError(t, err)

	ctx := previewContextWithCookie(t, &http.Cookie{
		Name:  draftPreviewCookieName,
		Value: token,
	})
	active := enableDraftPreviewContext(ctx, previewTestSecret)
	require.True(t, active)

	_, present := previewSessionFromContext(ctx)
	assert.True(t, present)
	assert.Equal(t, "private, no-store", ctx.Writer.Header().Get("Cache-Control"))
	assert.Equal(t, "noindex", ctx.Writer.Header().Get("X-Robots-Tag"))
	assert.Contains(t, ctx.Writer.Header().Get("Vary"), "Cookie")
}
