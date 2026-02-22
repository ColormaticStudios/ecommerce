package handlers

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type oidcIntegrationProvider struct {
	t         *testing.T
	clientID  string
	server    *httptest.Server
	private   *rsa.PrivateKey
	keyID     string
	codeToSub map[string]oidcUserClaims
	mu        sync.Mutex
}

func startOIDCIntegrationProvider(t *testing.T, clientID string) *oidcIntegrationProvider {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	p := &oidcIntegrationProvider{
		t:         t,
		clientID:  clientID,
		private:   privateKey,
		keyID:     "test-key-id-1",
		codeToSub: map[string]oidcUserClaims{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", p.handleDiscovery)
	mux.HandleFunc("/token", p.handleToken)
	mux.HandleFunc("/keys", p.handleJWKS)
	p.server = httptest.NewServer(mux)
	t.Cleanup(p.server.Close)

	return p
}

func (p *oidcIntegrationProvider) issuer() string {
	return p.server.URL
}

func (p *oidcIntegrationProvider) registerCode(code string, claims oidcUserClaims) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.codeToSub[code] = claims
}

func (p *oidcIntegrationProvider) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
	resp := map[string]any{
		"issuer":                 p.issuer(),
		"token_endpoint":         p.issuer() + "/token",
		"jwks_uri":               p.issuer() + "/keys",
		"authorization_endpoint": p.issuer() + "/auth",
		"response_types_supported": []string{
			"code",
		},
		"subject_types_supported": []string{
			"public",
		},
		"id_token_signing_alg_values_supported": []string{
			"RS256",
		},
	}
	writeJSON(w, resp)
}

func (p *oidcIntegrationProvider) handleToken(w http.ResponseWriter, r *http.Request) {
	require.NoError(p.t, r.ParseForm())
	code := strings.TrimSpace(r.FormValue("code"))
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	p.mu.Lock()
	claims, ok := p.codeToSub[code]
	p.mu.Unlock()
	if !ok {
		http.Error(w, "unknown code", http.StatusBadRequest)
		return
	}

	token, err := p.signedIDToken(claims)
	require.NoError(p.t, err)

	writeJSON(w, map[string]any{
		"access_token": "fake-access-token",
		"token_type":   "Bearer",
		"expires_in":   3600,
		"id_token":     token,
	})
}

func (p *oidcIntegrationProvider) signedIDToken(claims oidcUserClaims) (string, error) {
	now := time.Now()
	tokenClaims := jwt.MapClaims{
		"iss":                p.issuer(),
		"aud":                p.clientID,
		"sub":                claims.Sub,
		"email":              claims.Email,
		"name":               claims.Name,
		"preferred_username": claims.PreferredUsername,
		"iat":                now.Unix(),
		"exp":                now.Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, tokenClaims)
	token.Header["kid"] = p.keyID
	return token.SignedString(p.private)
}

func (p *oidcIntegrationProvider) handleJWKS(w http.ResponseWriter, _ *http.Request) {
	pub := p.private.PublicKey
	keys := map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"alg": "RS256",
				"use": "sig",
				"kid": p.keyID,
				"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
			},
		},
	}
	writeJSON(w, keys)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func TestOIDCCallbackIntegrationCreatesUserWithPreferredUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const clientID = "oidc-client-id"
	const callbackURL = "http://localhost:3000/api/v1/auth/oidc/callback"
	provider := startOIDCIntegrationProvider(t, clientID)
	db := newTestDB(t, &models.User{})

	provider.registerCode("new-user-code", oidcUserClaims{
		Sub:               "oidc-sub-new-1",
		Email:             "new-oidc@example.com",
		Name:              "OIDC New User",
		PreferredUsername: "oidc-new-handle",
	})

	r := gin.New()
	r.GET("/oidc/callback", OIDCCallback(db, "jwt-secret", provider.issuer(), clientID, callbackURL, AuthCookieConfig{}))

	req := httptest.NewRequest(http.MethodGet, "/oidc/callback?state=state-1&code=new-user-code&format=json", nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookieName, Value: "state-1"})
	req.AddCookie(&http.Cookie{Name: oidcRedirectCookieName, Value: "/profile"})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var response AuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "oidc-sub-new-1", response.User.Subject)
	assert.Equal(t, "oidc-new-handle", response.User.Username)

	var dbUser models.User
	require.NoError(t, db.Where("subject = ?", "oidc-sub-new-1").First(&dbUser).Error)
	assert.Equal(t, "oidc-new-handle", dbUser.Username)
	assert.Equal(t, "new-oidc@example.com", dbUser.Email)
}

func TestOIDCCallbackIntegrationUpdatesExistingUsernameFromPreferredUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const clientID = "oidc-client-id"
	const callbackURL = "http://localhost:3000/api/v1/auth/oidc/callback"
	provider := startOIDCIntegrationProvider(t, clientID)
	db := newTestDB(t, &models.User{})

	existing := models.User{
		Subject:  "oidc-sub-existing-1",
		Username: "old-handle",
		Email:    "existing@example.com",
		Name:     "Existing User",
		Role:     "customer",
	}
	require.NoError(t, db.Create(&existing).Error)

	provider.registerCode("existing-user-code", oidcUserClaims{
		Sub:               "oidc-sub-existing-1",
		Email:             "existing@example.com",
		Name:              "Existing User",
		PreferredUsername: "new-handle",
	})

	r := gin.New()
	r.GET("/oidc/callback", OIDCCallback(db, "jwt-secret", provider.issuer(), clientID, callbackURL, AuthCookieConfig{}))

	req := httptest.NewRequest(http.MethodGet, "/oidc/callback?state=state-2&code=existing-user-code&format=json", nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookieName, Value: "state-2"})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var response AuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "new-handle", response.User.Username)

	var dbUser models.User
	require.NoError(t, db.Where("subject = ?", "oidc-sub-existing-1").First(&dbUser).Error)
	assert.Equal(t, "new-handle", dbUser.Username)
}

func TestOIDCCallbackIntegrationKeepsExistingUsernameWhenPreferredUsernameTaken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const clientID = "oidc-client-id"
	const callbackURL = "http://localhost:3000/api/v1/auth/oidc/callback"
	provider := startOIDCIntegrationProvider(t, clientID)
	db := newTestDB(t, &models.User{})

	targetUser := models.User{
		Subject:  "oidc-sub-existing-2",
		Username: "stable-handle",
		Email:    "existing2@example.com",
		Name:     "Existing User 2",
		Role:     "customer",
	}
	require.NoError(t, db.Create(&targetUser).Error)

	conflictUser := models.User{
		Subject:  "oidc-sub-conflict-1",
		Username: "taken-handle",
		Email:    "conflict@example.com",
		Name:     "Conflict User",
		Role:     "customer",
	}
	require.NoError(t, db.Create(&conflictUser).Error)

	provider.registerCode("existing-user-code-conflict", oidcUserClaims{
		Sub:               "oidc-sub-existing-2",
		Email:             "existing2@example.com",
		Name:              "Existing User 2",
		PreferredUsername: "taken-handle",
	})

	r := gin.New()
	r.GET("/oidc/callback", OIDCCallback(db, "jwt-secret", provider.issuer(), clientID, callbackURL, AuthCookieConfig{}))

	req := httptest.NewRequest(http.MethodGet, "/oidc/callback?state=state-3&code=existing-user-code-conflict&format=json", nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookieName, Value: "state-3"})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var response AuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "stable-handle", response.User.Username)

	var dbUser models.User
	require.NoError(t, db.Where("subject = ?", "oidc-sub-existing-2").First(&dbUser).Error)
	assert.Equal(t, "stable-handle", dbUser.Username)
}
