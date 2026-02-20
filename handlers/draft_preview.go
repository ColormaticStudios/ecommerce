package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	draftPreviewCookieName = "draft_preview_token"
	draftPreviewContextKey = "draft_preview_session"
	defaultDraftPreviewTTL = 30 * time.Minute
)

type draftPreviewClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

type draftPreviewSession struct {
	Subject   string
	ExpiresAt time.Time
}

type DraftPreviewSessionResponse struct {
	Active    bool       `json:"active"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func appendVaryHeader(header http.Header, value string) {
	current := header.Get("Vary")
	if current == "" {
		header.Set("Vary", value)
		return
	}
	for _, existing := range strings.Split(current, ",") {
		if strings.EqualFold(strings.TrimSpace(existing), value) {
			return
		}
	}
	header.Set("Vary", current+", "+value)
}

func applyDraftPreviewResponseHeaders(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Robots-Tag", "noindex")
	appendVaryHeader(c.Writer.Header(), "Cookie")
	appendVaryHeader(c.Writer.Header(), "Authorization")
}

func buildDraftPreviewToken(subject string, role string, secret string, ttl time.Duration) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(ttl)
	claims := draftPreviewClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

func parseDraftPreviewCookie(c *gin.Context, secret string) (draftPreviewSession, bool) {
	tokenString, err := c.Cookie(draftPreviewCookieName)
	if err != nil || strings.TrimSpace(tokenString) == "" {
		return draftPreviewSession{}, false
	}

	claims := &draftPreviewClaims{}
	token, parseErr := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(secret), nil
		},
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	if parseErr != nil || token == nil || !token.Valid {
		return draftPreviewSession{}, false
	}
	if claims.Subject == "" || claims.Role != "admin" || claims.ExpiresAt == nil {
		return draftPreviewSession{}, false
	}

	return draftPreviewSession{
		Subject:   claims.Subject,
		ExpiresAt: claims.ExpiresAt.Time,
	}, true
}

func setDraftPreviewCookie(c *gin.Context, token string, cfg AuthCookieConfig, ttl time.Duration) {
	maxAge := int(ttl.Seconds())
	c.SetSameSite(cfg.SameSite)
	c.SetCookie(
		draftPreviewCookieName,
		token,
		maxAge,
		"/",
		cfg.Domain,
		cfg.Secure,
		true,
	)
}

func clearDraftPreviewCookie(c *gin.Context, cfg AuthCookieConfig) {
	c.SetSameSite(cfg.SameSite)
	c.SetCookie(
		draftPreviewCookieName,
		"",
		-1,
		"/",
		cfg.Domain,
		cfg.Secure,
		true,
	)
}

func enableDraftPreviewContext(c *gin.Context, secret string) bool {
	session, ok := parseDraftPreviewCookie(c, secret)
	if !ok {
		return false
	}
	c.Set(draftPreviewContextKey, session)
	applyDraftPreviewResponseHeaders(c)
	return true
}

func previewSessionFromContext(c *gin.Context) (draftPreviewSession, bool) {
	value, exists := c.Get(draftPreviewContextKey)
	if !exists {
		return draftPreviewSession{}, false
	}
	session, ok := value.(draftPreviewSession)
	if !ok {
		return draftPreviewSession{}, false
	}
	return session, true
}

func isDraftPreviewActive(c *gin.Context) bool {
	_, ok := previewSessionFromContext(c)
	return ok
}

func StartDraftPreviewSession(jwtSecret string, cookieCfg AuthCookieConfig, ttl time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject := c.GetString("userID")
		role := c.GetString("userRole")
		if subject == "" || role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: insufficient permissions"})
			return
		}

		signed, expiresAt, err := buildDraftPreviewToken(subject, role, jwtSecret, ttl)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start draft preview"})
			return
		}

		setDraftPreviewCookie(c, signed, cookieCfg, ttl)
		c.JSON(http.StatusOK, DraftPreviewSessionResponse{
			Active:    true,
			ExpiresAt: &expiresAt,
		})
	}
}

func GetDraftPreviewSessionStatus(jwtSecret string, cookieCfg AuthCookieConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		session, active := parseDraftPreviewCookie(c, jwtSecret)
		if !active {
			if _, err := c.Cookie(draftPreviewCookieName); err == nil {
				clearDraftPreviewCookie(c, cookieCfg)
			}
			c.JSON(http.StatusOK, DraftPreviewSessionResponse{Active: false})
			return
		}
		c.JSON(http.StatusOK, DraftPreviewSessionResponse{
			Active:    true,
			ExpiresAt: &session.ExpiresAt,
		})
	}
}

func StopDraftPreviewSession(cookieCfg AuthCookieConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		clearDraftPreviewCookie(c, cookieCfg)
		c.JSON(http.StatusOK, DraftPreviewSessionResponse{Active: false})
	}
}
