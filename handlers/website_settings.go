package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"ecommerce/internal/services/providerops"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	websiteOIDCSecretScope       = "website_oidc_client_secret"
	websiteOIDCSecretDataKey     = "client_secret"
	websiteOIDCSecretPlaceholder = "********"
)

type WebsiteSettingsPayload struct {
	AllowGuestCheckout         bool   `json:"allow_guest_checkout"`
	OIDCProvider               string `json:"oidc_provider"`
	OIDCClientID               string `json:"oidc_client_id"`
	OIDCClientSecret           string `json:"oidc_client_secret"`
	OIDCClientSecretConfigured bool   `json:"oidc_client_secret_configured"`
	ClearOIDCClientSecret      bool   `json:"clear_oidc_client_secret"`
	OIDCRedirectURI            string `json:"oidc_redirect_uri"`
}

type WebsiteSettingsResponse struct {
	Settings  WebsiteSettingsPayload `json:"settings"`
	UpdatedAt time.Time              `json:"updated_at"`
}

type UpsertWebsiteSettingsRequest struct {
	Settings WebsiteSettingsPayload `json:"settings" binding:"required"`
}

func normalizeWebsiteSettings(input WebsiteSettingsPayload) WebsiteSettingsPayload {
	return WebsiteSettingsPayload{
		AllowGuestCheckout:    input.AllowGuestCheckout,
		OIDCProvider:          strings.TrimSpace(input.OIDCProvider),
		OIDCClientID:          strings.TrimSpace(input.OIDCClientID),
		OIDCClientSecret:      strings.TrimSpace(input.OIDCClientSecret),
		ClearOIDCClientSecret: input.ClearOIDCClientSecret,
		OIDCRedirectURI:       strings.TrimSpace(input.OIDCRedirectURI),
	}
}

func loadOrCreateWebsiteSettings(db *gorm.DB) (models.WebsiteSettings, error) {
	var settings models.WebsiteSettings
	err := db.First(&settings, models.WebsiteSettingsSingletonID).Error
	if err == nil {
		return settings, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.WebsiteSettings{}, err
	}
	settings = models.WebsiteSettings{
		ID:                 models.WebsiteSettingsSingletonID,
		AllowGuestCheckout: true,
	}
	if err := db.Select("*").Create(&settings).Error; err != nil {
		return models.WebsiteSettings{}, err
	}
	return settings, nil
}

func websiteSettingsPayload(settings models.WebsiteSettings) WebsiteSettingsPayload {
	return WebsiteSettingsPayload{
		AllowGuestCheckout:         settings.AllowGuestCheckout,
		OIDCProvider:               settings.OIDCProvider,
		OIDCClientID:               settings.OIDCClientID,
		OIDCClientSecret:           "",
		OIDCClientSecretConfigured: strings.TrimSpace(settings.OIDCClientSecretEnvelopeJSON) != "",
		ClearOIDCClientSecret:      false,
		OIDCRedirectURI:            settings.OIDCRedirectURI,
	}
}

func websiteSettingsResponse(settings models.WebsiteSettings) WebsiteSettingsResponse {
	return WebsiteSettingsResponse{
		Settings:  websiteSettingsPayload(settings),
		UpdatedAt: settings.UpdatedAt,
	}
}

func GetAdminWebsiteSettings(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, err := loadOrCreateWebsiteSettings(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load website settings"})
			return
		}
		c.JSON(http.StatusOK, websiteSettingsResponse(settings))
	}
}

func UpsertWebsiteSettings(db *gorm.DB) gin.HandlerFunc {
	return UpsertWebsiteSettingsWithCredentials(db, nil)
}

func UpsertWebsiteSettingsWithCredentials(db *gorm.DB, credentials *providerops.CredentialService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req UpsertWebsiteSettingsRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		normalized := normalizeWebsiteSettings(req.Settings)
		var settings models.WebsiteSettings
		if err := db.Transaction(func(tx *gorm.DB) error {
			current, err := loadOrCreateWebsiteSettings(tx)
			if err != nil {
				return err
			}
			current.AllowGuestCheckout = normalized.AllowGuestCheckout
			current.OIDCProvider = normalized.OIDCProvider
			current.OIDCClientID = normalized.OIDCClientID
			current.OIDCRedirectURI = normalized.OIDCRedirectURI
			if normalized.ClearOIDCClientSecret {
				current.OIDCClientSecretEnvelopeJSON = ""
				current.OIDCClientSecretKeyVersion = ""
			}
			if normalized.OIDCClientSecret != "" && normalized.OIDCClientSecret != websiteOIDCSecretPlaceholder {
				envelopeJSON, keyVersion, err := encryptWebsiteOIDCClientSecret(credentials, normalized.OIDCClientSecret)
				if err != nil {
					return err
				}
				current.OIDCClientSecretEnvelopeJSON = envelopeJSON
				current.OIDCClientSecretKeyVersion = keyVersion
			}
			if err := tx.Select("*").Save(&current).Error; err != nil {
				return err
			}
			settings = current
			return nil
		}); err != nil {
			if errors.Is(err, providerops.ErrCredentialServiceUnconfigured) {
				c.JSON(http.StatusPreconditionFailed, gin.H{"error": "Provider credential encryption is not configured"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save website settings"})
			return
		}
		c.JSON(http.StatusOK, websiteSettingsResponse(settings))
	}
}

func encryptWebsiteOIDCClientSecret(credentials *providerops.CredentialService, secret string) (string, string, error) {
	return credentials.EncryptSecretData(websiteOIDCSecretScope, map[string]string{
		websiteOIDCSecretDataKey: secret,
	})
}

func decryptWebsiteOIDCClientSecret(credentials *providerops.CredentialService, settings models.WebsiteSettings) (string, error) {
	if strings.TrimSpace(settings.OIDCClientSecretEnvelopeJSON) == "" {
		return "", nil
	}
	secretData, err := credentials.DecryptSecretData(websiteOIDCSecretScope, settings.OIDCClientSecretEnvelopeJSON)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(secretData[websiteOIDCSecretDataKey]), nil
}
