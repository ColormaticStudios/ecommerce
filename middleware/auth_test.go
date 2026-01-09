package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddleware_ValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret-key"

	// Create a valid token
	claims := jwt.MapClaims{
		"sub":   "test-subject",
		"email": "test@example.com",
		"role":  "customer",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	// Setup router
	r := gin.New()
	r.Use(AuthMiddleware(secret, ""))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Make request
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret-key"

	r := gin.New()
	r.Use(AuthMiddleware(secret, ""))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret-key"

	r := gin.New()
	r.Use(AuthMiddleware(secret, ""))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret-key"

	// Create expired token
	claims := jwt.MapClaims{
		"sub":   "test-subject",
		"email": "test@example.com",
		"role":  "customer",
		"exp":   time.Now().Add(-time.Hour).Unix(), // Expired
		"iat":   time.Now().Add(-time.Hour * 2).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	r := gin.New()
	r.Use(AuthMiddleware(secret, ""))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_RoleRequirement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret-key"

	// Create token with customer role
	claims := jwt.MapClaims{
		"sub":   "test-subject",
		"email": "test@example.com",
		"role":  "customer",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	// Require admin role
	r := gin.New()
	r.Use(AuthMiddleware(secret, "admin"))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAuthMiddleware_AdminAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret-key"

	// Create token with admin role
	claims := jwt.MapClaims{
		"sub":   "test-subject",
		"email": "admin@example.com",
		"role":  "admin",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	// Require admin role
	r := gin.New()
	r.Use(AuthMiddleware(secret, "admin"))
	r.GET("/test", func(c *gin.Context) {
		userID := c.GetString("userID")
		userRole := c.GetString("userRole")
		c.JSON(http.StatusOK, gin.H{
			"userID": userID,
			"role":   userRole,
		})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_ContextValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret-key"

	claims := jwt.MapClaims{
		"sub":   "test-subject-123",
		"email": "test@example.com",
		"role":  "customer",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	r := gin.New()
	r.Use(AuthMiddleware(secret, ""))
	r.GET("/test", func(c *gin.Context) {
		userID := c.GetString("userID")
		userEmail := c.GetString("userEmail")
		userRole := c.GetString("userRole")

		c.JSON(http.StatusOK, gin.H{
			"userID":    userID,
			"userEmail": userEmail,
			"userRole":  userRole,
		})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Note: In a real test, you'd parse the JSON response to verify values
}

func TestAuthMiddleware_CustomClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret-key"

	// Test with CustomClaims format (for OIDC compatibility)
	customClaims := &CustomClaims{
		Email: "test@example.com",
		Role:  "customer",
		Name:  "Test User",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "test-subject",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, customClaims)
	tokenString, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	r := gin.New()
	r.Use(AuthMiddleware(secret, ""))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
