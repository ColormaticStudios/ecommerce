package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const csrfCookieName = "csrf_token"

func CSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}

		// Non-browser/API clients using bearer tokens don't rely on cookies for auth.
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(strings.TrimSpace(authHeader), "Bearer ") {
			c.Next()
			return
		}

		sessionToken, err := c.Cookie(sessionCookieName)
		if err != nil || sessionToken == "" {
			c.Next()
			return
		}

		csrfCookie, err := c.Cookie(csrfCookieName)
		if err != nil || csrfCookie == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Missing CSRF token"})
			return
		}

		csrfHeader := c.GetHeader("X-CSRF-Token")
		if csrfHeader == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Missing CSRF token"})
			return
		}

		if subtle.ConstantTimeCompare([]byte(csrfCookie), []byte(csrfHeader)) != 1 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Invalid CSRF token"})
			return
		}

		c.Next()
	}
}
