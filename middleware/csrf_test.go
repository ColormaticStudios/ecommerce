package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCSRFMiddleware_AllowsSafeMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRFMiddleware())
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCSRFMiddleware_AllowsBearerClients(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRFMiddleware())
	r.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCSRFMiddleware_RejectsMissingTokenForSessionAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRFMiddleware())
	r.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCSRFMiddleware_RejectsMismatchedToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRFMiddleware())
	r.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "cookie-token"})
	req.Header.Set("X-CSRF-Token", "header-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCSRFMiddleware_AllowsMatchingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRFMiddleware())
	r.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-token"})
	req.Header.Set("X-CSRF-Token", "csrf-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
