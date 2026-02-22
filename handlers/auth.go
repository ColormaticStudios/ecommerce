package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"ecommerce/models"

	"github.com/coreos/go-oidc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

const SessionCookieName = "session_token"
const csrfCookieName = "csrf_token"
const oidcStateCookieName = "oidc_state"
const oidcRedirectCookieName = "oidc_redirect"

type AuthCookieConfig struct {
	Secure   bool
	Domain   string
	SameSite http.SameSite
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Name     string `json:"name"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	User models.User `json:"user"`
}

type oidcUserClaims struct {
	Email             string `json:"email"`
	Sub               string `json:"sub"`
	Name              string `json:"name"`
	Picture           string `json:"picture"`
	Locale            string `json:"locale"`
	PreferredUsername string `json:"preferred_username"`
}

func setSessionCookie(c *gin.Context, token string, cfg AuthCookieConfig) {
	maxAge := int((7 * 24 * time.Hour).Seconds())
	c.SetSameSite(cfg.SameSite)
	c.SetCookie(
		SessionCookieName,
		token,
		maxAge,
		"/",
		cfg.Domain,
		cfg.Secure,
		true,
	)
}

func clearSessionCookie(c *gin.Context, cfg AuthCookieConfig) {
	c.SetSameSite(cfg.SameSite)
	c.SetCookie(
		SessionCookieName,
		"",
		-1,
		"/",
		cfg.Domain,
		cfg.Secure,
		true,
	)
}

func setCSRFCookie(c *gin.Context, token string, cfg AuthCookieConfig) {
	maxAge := int((7 * 24 * time.Hour).Seconds())
	c.SetSameSite(cfg.SameSite)
	c.SetCookie(
		csrfCookieName,
		token,
		maxAge,
		"/",
		cfg.Domain,
		cfg.Secure,
		false,
	)
}

func clearCSRFCookie(c *gin.Context, cfg AuthCookieConfig) {
	c.SetSameSite(cfg.SameSite)
	c.SetCookie(
		csrfCookieName,
		"",
		-1,
		"/",
		cfg.Domain,
		cfg.Secure,
		false,
	)
}

func setOIDCStateCookie(c *gin.Context, state string, cfg AuthCookieConfig) {
	c.SetSameSite(cfg.SameSite)
	c.SetCookie(
		oidcStateCookieName,
		state,
		300,
		"/",
		cfg.Domain,
		cfg.Secure,
		true,
	)
}

func clearOIDCStateCookie(c *gin.Context, cfg AuthCookieConfig) {
	c.SetSameSite(cfg.SameSite)
	c.SetCookie(
		oidcStateCookieName,
		"",
		-1,
		"/",
		cfg.Domain,
		cfg.Secure,
		true,
	)
}

func setOIDCRedirectCookie(c *gin.Context, redirectPath string, cfg AuthCookieConfig) {
	c.SetSameSite(cfg.SameSite)
	c.SetCookie(
		oidcRedirectCookieName,
		redirectPath,
		300,
		"/",
		cfg.Domain,
		cfg.Secure,
		true,
	)
}

func clearOIDCRedirectCookie(c *gin.Context, cfg AuthCookieConfig) {
	c.SetSameSite(cfg.SameSite)
	c.SetCookie(
		oidcRedirectCookieName,
		"",
		-1,
		"/",
		cfg.Domain,
		cfg.Secure,
		true,
	)
}

func sanitizeRedirectPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "//") {
		return path
	}
	return "/"
}

func wantsJSONResponse(c *gin.Context) bool {
	if strings.EqualFold(c.Query("format"), "json") {
		return true
	}
	accept := strings.ToLower(c.GetHeader("Accept"))
	return strings.Contains(accept, "application/json")
}

func resolveOIDCUsername(claims oidcUserClaims) string {
	if preferred := strings.TrimSpace(claims.PreferredUsername); preferred != "" {
		return preferred
	}
	if email := strings.TrimSpace(claims.Email); email != "" {
		return email
	}
	return strings.TrimSpace(claims.Sub)
}

func syncOIDCUsernameIfAvailable(db *gorm.DB, user *models.User, claims oidcUserClaims) error {
	candidate := strings.TrimSpace(claims.PreferredUsername)
	if candidate == "" || candidate == user.Username {
		return nil
	}

	var existingUser models.User
	err := db.Where("username = ? AND id <> ?", candidate, user.ID).First(&existingUser).Error
	if err == nil {
		// Username is already used by another account; keep current username.
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if err := db.Model(user).Update("username", candidate).Error; err != nil {
		return err
	}
	user.Username = candidate
	return nil
}

// Register creates a new user account
func Register(db *gorm.DB, jwtSecret string, cookieCfg AuthCookieConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RegisterRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Check if email already exists
		var existingUser models.User
		if err := db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
			return
		}

		// Check if username already exists
		if err := db.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Username already taken"})
			return
		}

		// Hash password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}

		// Generate a unique subject ID for email/password users
		subject := generateSubjectID(req.Email)

		// Create user
		user := models.User{
			Subject:      subject,
			Username:     req.Username,
			Email:        req.Email,
			PasswordHash: string(hashedPassword),
			Name:         req.Name,
			Role:         "customer",
			Currency:     "USD",
		}

		if err := db.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		// Generate JWT token
		token, err := generateJWT(user, jwtSecret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		// Clear password hash from response
		user.PasswordHash = ""

		setSessionCookie(c, token, cookieCfg)
		setCSRFCookie(c, uuid.NewString(), cookieCfg)
		c.JSON(http.StatusCreated, AuthResponse{User: user})
	}
}

// Login authenticates a user and returns a JWT token
func Login(db *gorm.DB, jwtSecret string, cookieCfg AuthCookieConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Find user by email
		var user models.User
		if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
			return
		}

		// Check if user has a password (email/password user)
		if user.PasswordHash == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "This account uses OIDC authentication"})
			return
		}

		// Verify password
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
			return
		}

		// Generate JWT token
		token, err := generateJWT(user, jwtSecret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		// Clear password hash from response
		user.PasswordHash = ""

		setSessionCookie(c, token, cookieCfg)
		setCSRFCookie(c, uuid.NewString(), cookieCfg)
		c.JSON(http.StatusOK, AuthResponse{User: user})
	}
}

// generateJWT creates a JWT token for the user
func generateJWT(user models.User, secret string) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.Subject,
		"email": user.Email,
		"role":  user.Role,
		"name":  user.Name,
		"exp":   time.Now().Add(time.Hour * 24 * 7).Unix(), // 7 days
		"iat":   time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// generateSubjectID generates a unique subject ID for email/password users
func generateSubjectID(email string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.ToLower(email))).String()
}

// OIDCLogin redirects the user to the OIDC provider’s authorization endpoint
func OIDCLogin(oidcProvider string, clientID string, redirectURI string, cookieCfg AuthCookieConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		provider, err := oidc.NewProvider(ctx, oidcProvider)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create OIDC provider"})
			return
		}

		oauth2Config := oauth2.Config{
			ClientID:     clientID,
			ClientSecret: "", // secret is not needed for implicit flow in this example
			RedirectURL:  redirectURI,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		}

		state := uuid.NewString()
		postLoginRedirect := sanitizeRedirectPath(c.Query("redirect"))
		setOIDCStateCookie(c, state, cookieCfg)
		setOIDCRedirectCookie(c, postLoginRedirect, cookieCfg)
		authURL := oauth2Config.AuthCodeURL(state)
		c.Redirect(http.StatusFound, authURL)
	}
}

// OIDCCallback exchanges the authorization code for tokens, validates the ID token,
// creates/updates the local user record, and returns a JWT for the API.
func OIDCCallback(db *gorm.DB, jwtSecret string, oidcProvider string, clientID string, redirectURI string, cookieCfg AuthCookieConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		provider, err := oidc.NewProvider(ctx, oidcProvider)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create OIDC provider"})
			return
		}

		state := c.Query("state")
		expectedState, err := c.Cookie(oidcStateCookieName)
		if state == "" || err != nil || state != expectedState {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid OIDC state"})
			return
		}
		clearOIDCStateCookie(c, cookieCfg)

		oauth2Config := oauth2.Config{
			ClientID:     clientID,
			ClientSecret: "", // secret may be needed for confidential clients
			RedirectURL:  redirectURI,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		}

		code := c.Query("code")
		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing code in callback"})
			return
		}

		token, err := oauth2Config.Exchange(ctx, code)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "token exchange failed"})
			return
		}

		// Validate the ID token
		verifier := provider.Verifier(&oidc.Config{ClientID: clientID})
		rawIDToken, ok := token.Extra("id_token").(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no id_token in token response"})
			return
		}

		idToken, err := verifier.Verify(ctx, rawIDToken)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "failed to verify id_token"})
			return
		}

		var claims oidcUserClaims
		if err := idToken.Claims(&claims); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse id_token claims"})
			return
		}

		// Find or create the user
		var user models.User
		if err := db.Where("subject = ?", claims.Sub).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				user = models.User{
					Subject:  claims.Sub,
					Email:    claims.Email,
					Name:     claims.Name,
					Username: resolveOIDCUsername(claims),
					Role:     "customer",
				}
				if err := db.Create(&user).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
					return
				}
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "db query failed"})
				return
			}
		} else if err := syncOIDCUsernameIfAvailable(db, &user, claims); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to sync OIDC username"})
			return
		}

		// Generate JWT for API
		tokenString, err := generateJWT(user, jwtSecret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate JWT"})
			return
		}

		setSessionCookie(c, tokenString, cookieCfg)
		setCSRFCookie(c, uuid.NewString(), cookieCfg)

		postLoginRedirect := "/"
		if redirectCookie, err := c.Cookie(oidcRedirectCookieName); err == nil {
			postLoginRedirect = sanitizeRedirectPath(redirectCookie)
		}
		clearOIDCRedirectCookie(c, cookieCfg)

		if wantsJSONResponse(c) {
			c.JSON(http.StatusOK, AuthResponse{User: user})
			return
		}

		c.Redirect(http.StatusFound, postLoginRedirect)
	}
}

func Logout(cookieCfg AuthCookieConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		clearSessionCookie(c, cookieCfg)
		clearCSRFCookie(c, cookieCfg)
		clearDraftPreviewCookie(c, cookieCfg)
		c.JSON(http.StatusOK, gin.H{"message": "Logged out"})
	}
}
