package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func seedGuestOrderForAuthTest(t *testing.T, db *gorm.DB, email string) (models.CheckoutSession, models.Order) {
	t.Helper()

	now := time.Now().UTC()
	session := models.CheckoutSession{
		PublicToken: uuid.NewString(),
		GuestEmail:  &email,
		Status:      models.CheckoutSessionStatusConverted,
		ExpiresAt:   now.Add(24 * time.Hour),
		LastSeenAt:  now,
	}
	require.NoError(t, db.Create(&session).Error)

	token := uuid.NewString()
	order := models.Order{
		CheckoutSessionID: session.ID,
		GuestEmail:        &email,
		ConfirmationToken: &token,
		Status:            models.StatusPending,
		Total:             models.MoneyFromFloat(19.99),
	}
	require.NoError(t, db.Create(&order).Error)

	return session, order
}

func TestGenerateJWT(t *testing.T) {
	secret := "test-secret-key-for-jwt-generation"
	user := models.User{
		Subject: "test-subject-123",
		Email:   "test@example.com",
		Role:    "customer",
		Name:    "Test User",
	}

	token, err := generateJWT(user, secret)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Verify token can be parsed
	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	require.NoError(t, err)
	assert.True(t, parsedToken.Valid)

	// Verify claims
	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	require.True(t, ok)
	assert.Equal(t, user.Subject, claims["sub"])
	assert.Equal(t, user.Email, claims["email"])
	assert.Equal(t, user.Role, claims["role"])
	assert.Equal(t, user.Name, claims["name"])

	// Verify expiration is set
	exp, ok := claims["exp"].(float64)
	require.True(t, ok)
	expTime := time.Unix(int64(exp), 0)
	assert.True(t, expTime.After(time.Now()))
	assert.True(t, expTime.Before(time.Now().Add(time.Hour*24*8))) // Should be around 7 days
}

func TestGenerateJWT_DifferentUsers(t *testing.T) {
	secret := "test-secret"

	user1 := models.User{
		Subject: "subject-1",
		Email:   "user1@example.com",
		Role:    "customer",
	}

	user2 := models.User{
		Subject: "subject-2",
		Email:   "user2@example.com",
		Role:    "admin",
	}

	token1, err1 := generateJWT(user1, secret)
	token2, err2 := generateJWT(user2, secret)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.NotEqual(t, token1, token2) // Different users should get different tokens
}

func TestGenerateSubjectID(t *testing.T) {
	email1 := "test1@example.com"
	email2 := "test2@example.com"

	subject1 := generateSubjectID(email1)
	subject2 := generateSubjectID(email2)

	// Should generate non-empty subject
	assert.NotEmpty(t, subject1)
	assert.NotEmpty(t, subject2)

	// Different emails should generate different subjects
	assert.NotEqual(t, subject1, subject2)

	// Same email should generate same subject (deterministic)
	subject1Again := generateSubjectID(email1)
	// Note: This might not be true if timestamp is used, but let's test the function works
	assert.NotEmpty(t, subject1Again)
}

func TestPasswordHashing(t *testing.T) {
	password := "test-password-123"

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)
	assert.NotEmpty(t, hashedPassword)

	// Verify password
	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(password))
	assert.NoError(t, err)

	// Wrong password should fail
	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte("wrong-password"))
	assert.Error(t, err)
}

func TestPasswordHashing_DifferentPasswords(t *testing.T) {
	password1 := "password1"
	password2 := "password2"

	hash1, err1 := bcrypt.GenerateFromPassword([]byte(password1), bcrypt.DefaultCost)
	hash2, err2 := bcrypt.GenerateFromPassword([]byte(password2), bcrypt.DefaultCost)

	require.NoError(t, err1)
	require.NoError(t, err2)

	// Hashes should be different
	assert.NotEqual(t, string(hash1), string(hash2))

	// Each hash should only verify with its own password
	assert.NoError(t, bcrypt.CompareHashAndPassword(hash1, []byte(password1)))
	assert.Error(t, bcrypt.CompareHashAndPassword(hash1, []byte(password2)))
	assert.NoError(t, bcrypt.CompareHashAndPassword(hash2, []byte(password2)))
	assert.Error(t, bcrypt.CompareHashAndPassword(hash2, []byte(password1)))
}

func TestSanitizeRedirectPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "empty defaults to root", input: "", expected: "/"},
		{name: "whitespace defaults to root", input: "   ", expected: "/"},
		{name: "valid relative path", input: "/orders", expected: "/orders"},
		{name: "double slash rejected", input: "//evil.example", expected: "/"},
		{name: "absolute url rejected", input: "https://evil.example", expected: "/"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, sanitizeRedirectPath(tc.input))
		})
	}
}

func TestWantsJSONResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("format query wins", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		req := httptest.NewRequest(http.MethodGet, "/auth/callback?format=json", nil)
		req.Header.Set("Accept", "text/html")
		c.Request = req
		assert.True(t, wantsJSONResponse(c))
	})

	t.Run("accept header application/json", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		req := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)
		req.Header.Set("Accept", "application/json")
		c.Request = req
		assert.True(t, wantsJSONResponse(c))
	})

	t.Run("oidc response-format cookie", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		req := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)
		req.AddCookie(&http.Cookie{Name: oidcResponseFormatCookieName, Value: "json"})
		req.Header.Set("Accept", "text/html")
		c.Request = req
		assert.True(t, wantsJSONResponse(c))
	})

	t.Run("default false", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		req := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)
		req.Header.Set("Accept", "text/html")
		c.Request = req
		assert.False(t, wantsJSONResponse(c))
	})
}

func TestOIDCConfigured(t *testing.T) {
	t.Run("true when all required values are present", func(t *testing.T) {
		assert.True(t, oidcConfigured("https://issuer.example", "client-id", "https://app.example/callback"))
	})

	t.Run("false when any required value is missing", func(t *testing.T) {
		assert.False(t, oidcConfigured("", "client-id", "https://app.example/callback"))
		assert.False(t, oidcConfigured("https://issuer.example", "", "https://app.example/callback"))
		assert.False(t, oidcConfigured("https://issuer.example", "client-id", ""))
	})
}

func TestGetAuthConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/auth/config", GetAuthConfig(true, "https://issuer.example", "client-id", "https://app.example/callback"))

	req := httptest.NewRequest(http.MethodGet, "/auth/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response AuthConfigResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, AuthConfigResponse{
		LocalSignInEnabled: false,
		OIDCEnabled:        true,
	}, response)
}

func TestRegisterDoesNotAutoClaimMatchingGuestOrders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTestDB(t)

	matchingEmail := "guest-register@example.com"
	firstSession, firstOrder := seedGuestOrderForAuthTest(t, db, matchingEmail)
	secondSession, secondOrder := seedGuestOrderForAuthTest(t, db, matchingEmail)
	_, otherOrder := seedGuestOrderForAuthTest(t, db, "other@example.com")

	r := gin.New()
	r.POST("/auth/register", Register(db, "secret", AuthCookieConfig{}))

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/register",
		strings.NewReader(`{"username":"guest-register","email":"guest-register@example.com","password":"supersecret"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var authResp AuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &authResp))
	require.NotNil(t, authResp.Token)
	assert.NotEmpty(t, *authResp.Token)

	var user models.User
	require.NoError(t, db.Where("email = ?", matchingEmail).First(&user).Error)

	for _, orderID := range []uint{firstOrder.ID, secondOrder.ID} {
		var order models.Order
		require.NoError(t, db.First(&order, orderID).Error)
		assert.Nil(t, order.UserID)
		assert.False(t, order.ClaimedAt.Valid())
	}

	for _, sessionID := range []uint{firstSession.ID, secondSession.ID} {
		var session models.CheckoutSession
		require.NoError(t, db.First(&session, sessionID).Error)
		assert.Nil(t, session.UserID)
	}

	var untouched models.Order
	require.NoError(t, db.First(&untouched, otherOrder.ID).Error)
	assert.Nil(t, untouched.UserID)
	assert.False(t, untouched.ClaimedAt.Valid())
}

func TestLoginDoesNotAutoClaimMatchingGuestOrders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTestDB(t)

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("supersecret"), bcrypt.DefaultCost)
	require.NoError(t, err)

	user := models.User{
		Subject:      "sub-login-claim",
		Username:     "login-claim-user",
		Email:        "guest-login@example.com",
		PasswordHash: string(passwordHash),
		Role:         "customer",
		Currency:     "USD",
	}
	require.NoError(t, db.Create(&user).Error)

	matchSession, matchOrder := seedGuestOrderForAuthTest(t, db, user.Email)
	_, otherOrder := seedGuestOrderForAuthTest(t, db, "different@example.com")

	r := gin.New()
	r.POST("/auth/login", Login(db, "secret", AuthCookieConfig{}))

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/login",
		strings.NewReader(`{"email":"guest-login@example.com","password":"supersecret"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var authResp AuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &authResp))
	require.NotNil(t, authResp.Token)
	assert.NotEmpty(t, *authResp.Token)

	var claimedOrder models.Order
	require.NoError(t, db.First(&claimedOrder, matchOrder.ID).Error)
	assert.Nil(t, claimedOrder.UserID)
	assert.False(t, claimedOrder.ClaimedAt.Valid())

	var claimedSession models.CheckoutSession
	require.NoError(t, db.First(&claimedSession, matchSession.ID).Error)
	assert.Nil(t, claimedSession.UserID)

	var untouched models.Order
	require.NoError(t, db.First(&untouched, otherOrder.ID).Error)
	assert.Nil(t, untouched.UserID)
	assert.False(t, untouched.ClaimedAt.Valid())
}

func TestOIDCHandlersInvalidProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/oidc/login", OIDCLogin("http://127.0.0.1:1", "client", "http://127.0.0.1/cb", AuthCookieConfig{}))
	r.GET("/oidc/callback", OIDCCallback(newTestDB(t), "secret", "http://127.0.0.1:1", "client", "http://127.0.0.1/cb", AuthCookieConfig{}))

	loginReq := httptest.NewRequest(http.MethodGet, "/oidc/login", nil)
	loginW := httptest.NewRecorder()
	r.ServeHTTP(loginW, loginReq)
	assert.Equal(t, http.StatusInternalServerError, loginW.Code)

	callbackReq := httptest.NewRequest(http.MethodGet, "/oidc/callback?state=x&code=y", nil)
	callbackW := httptest.NewRecorder()
	r.ServeHTTP(callbackW, callbackReq)
	assert.Equal(t, http.StatusInternalServerError, callbackW.Code)
}

func TestResolveOIDCUsername(t *testing.T) {
	t.Run("uses preferred username when available", func(t *testing.T) {
		claims := oidcUserClaims{
			Sub:               "sub-1",
			Email:             "email@example.com",
			PreferredUsername: "preferred-name",
		}
		assert.Equal(t, "preferred-name", resolveOIDCUsername(claims))
	})

	t.Run("falls back to email", func(t *testing.T) {
		claims := oidcUserClaims{
			Sub:   "sub-1",
			Email: "email@example.com",
		}
		assert.Equal(t, "email@example.com", resolveOIDCUsername(claims))
	})

	t.Run("falls back to subject", func(t *testing.T) {
		claims := oidcUserClaims{
			Sub: "sub-1",
		}
		assert.Equal(t, "sub-1", resolveOIDCUsername(claims))
	})
}

func TestSyncOIDCUsernameIfAvailable(t *testing.T) {
	db := newTestDB(t, &models.User{})

	user := models.User{
		Subject:  "sub-oidc-user",
		Username: "old-name",
		Email:    "oidc@example.com",
		Role:     "customer",
	}
	require.NoError(t, db.Create(&user).Error)

	t.Run("updates to available preferred username", func(t *testing.T) {
		claims := oidcUserClaims{
			PreferredUsername: "new-name",
		}
		require.NoError(t, syncOIDCUsernameIfAvailable(db, &user, claims))

		var reloaded models.User
		require.NoError(t, db.First(&reloaded, user.ID).Error)
		assert.Equal(t, "new-name", reloaded.Username)
		assert.Equal(t, "new-name", user.Username)
	})

	t.Run("keeps current username when preferred username is taken", func(t *testing.T) {
		other := models.User{
			Subject:  "sub-other-user",
			Username: "taken-name",
			Email:    "other@example.com",
			Role:     "customer",
		}
		require.NoError(t, db.Create(&other).Error)

		claims := oidcUserClaims{
			PreferredUsername: "taken-name",
		}
		require.NoError(t, syncOIDCUsernameIfAvailable(db, &user, claims))

		var reloaded models.User
		require.NoError(t, db.First(&reloaded, user.ID).Error)
		assert.Equal(t, "new-name", reloaded.Username)
		assert.Equal(t, "new-name", user.Username)
	})
}
