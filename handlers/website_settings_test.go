package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ecommerce/internal/services/providerops"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebsiteSettingsDefaultsAndUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTestDB(t, &models.WebsiteSettings{})
	credentials := mustWebsiteTestCredentialService(t)

	r := gin.New()
	r.GET("/admin/website", GetAdminWebsiteSettings(db))
	r.PUT("/admin/website", UpsertWebsiteSettingsWithCredentials(db, credentials))

	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, httptest.NewRequest(http.MethodGet, "/admin/website", nil))
	require.Equal(t, http.StatusOK, getW.Code)

	var defaults WebsiteSettingsResponse
	require.NoError(t, json.Unmarshal(getW.Body.Bytes(), &defaults))
	assert.True(t, defaults.Settings.AllowGuestCheckout)
	assert.Empty(t, defaults.Settings.OIDCProvider)

	const rawClientSecret = "plain-client-secret-value"
	body := `{"settings":{"allow_guest_checkout":false,"oidc_provider":" https://issuer.example ","oidc_client_id":" client-id ","oidc_client_secret":" ` + rawClientSecret + ` ","oidc_client_secret_configured":false,"clear_oidc_client_secret":false,"oidc_redirect_uri":" https://shop.example/api/v1/auth/oidc/callback "}}`
	putReq := httptest.NewRequest(http.MethodPut, "/admin/website", strings.NewReader(body))
	putReq.Header.Set("Content-Type", "application/json")
	putW := httptest.NewRecorder()
	r.ServeHTTP(putW, putReq)
	require.Equal(t, http.StatusOK, putW.Code)

	var updated WebsiteSettingsResponse
	require.NoError(t, json.Unmarshal(putW.Body.Bytes(), &updated))
	assert.False(t, updated.Settings.AllowGuestCheckout)
	assert.Equal(t, "https://issuer.example", updated.Settings.OIDCProvider)
	assert.Equal(t, "client-id", updated.Settings.OIDCClientID)
	assert.Empty(t, updated.Settings.OIDCClientSecret)
	assert.True(t, updated.Settings.OIDCClientSecretConfigured)
	assert.Equal(t, "https://shop.example/api/v1/auth/oidc/callback", updated.Settings.OIDCRedirectURI)
	assert.NotContains(t, putW.Body.String(), rawClientSecret)

	var stored models.WebsiteSettings
	require.NoError(t, db.First(&stored, models.WebsiteSettingsSingletonID).Error)
	assert.False(t, stored.AllowGuestCheckout)
	assert.Equal(t, "https://issuer.example", stored.OIDCProvider)
	assert.NotEmpty(t, stored.OIDCClientSecretEnvelopeJSON)
	assert.NotContains(t, stored.OIDCClientSecretEnvelopeJSON, rawClientSecret)
	decrypted, err := decryptWebsiteOIDCClientSecret(credentials, stored)
	require.NoError(t, err)
	assert.Equal(t, rawClientSecret, decrypted)
}

func mustWebsiteTestCredentialService(t *testing.T) *providerops.CredentialService {
	t.Helper()
	service, err := providerops.NewCredentialService(map[string][]byte{
		"v1": []byte("0123456789abcdef"),
	}, "v1")
	require.NoError(t, err)
	return service
}
