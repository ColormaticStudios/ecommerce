package handlers

import (
	"net/http"
	"strings"

	"ecommerce/internal/checkoutplugins"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type updateCheckoutPluginRequest struct {
	Enabled *bool `json:"enabled" binding:"required"`
}

func parseProviderType(value string) (checkoutplugins.ProviderType, bool) {
	switch strings.TrimSpace(value) {
	case string(checkoutplugins.ProviderTypePayment):
		return checkoutplugins.ProviderTypePayment, true
	case string(checkoutplugins.ProviderTypeShipping):
		return checkoutplugins.ProviderTypeShipping, true
	case string(checkoutplugins.ProviderTypeTax):
		return checkoutplugins.ProviderTypeTax, true
	default:
		return "", false
	}
}

func syncCheckoutProviderSettings(db *gorm.DB, pluginManager *checkoutplugins.Manager) error {
	if db == nil || pluginManager == nil {
		return nil
	}
	if !db.Migrator().HasTable(&models.CheckoutProviderSetting{}) {
		return nil
	}

	var records []models.CheckoutProviderSetting
	if err := db.Find(&records).Error; err != nil {
		return err
	}

	settings := make([]checkoutplugins.ProviderSetting, 0, len(records))
	for _, record := range records {
		providerType, ok := parseProviderType(record.ProviderType)
		if !ok {
			continue
		}
		settings = append(settings, checkoutplugins.ProviderSetting{
			Type:    providerType,
			ID:      record.ProviderID,
			Enabled: record.Enabled,
		})
	}

	pluginManager.ReplaceSettings(settings)
	return persistCheckoutProviderSettings(db, pluginManager)
}

func persistCheckoutProviderSettings(db *gorm.DB, pluginManager *checkoutplugins.Manager) error {
	if db == nil || pluginManager == nil {
		return nil
	}
	if !db.Migrator().HasTable(&models.CheckoutProviderSetting{}) {
		return nil
	}

	settings := pluginManager.ListSettings()
	for _, setting := range settings {
		record := models.CheckoutProviderSetting{
			ProviderType: string(setting.Type),
			ProviderID:   setting.ID,
			Enabled:      setting.Enabled,
		}
		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "provider_type"},
				{Name: "provider_id"},
			},
			DoUpdates: clause.Assignments(map[string]any{
				"enabled": setting.Enabled,
			}),
		}).Create(&record).Error; err != nil {
			return err
		}
	}

	return nil
}

func ListAdminCheckoutPlugins(pluginManager *checkoutplugins.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		payments, shippings, taxes := pluginManager.ListForAdmin()
		c.JSON(http.StatusOK, gin.H{
			"payment":  payments,
			"shipping": shippings,
			"tax":      taxes,
		})
	}
}

func UpdateAdminCheckoutPlugin(db *gorm.DB, pluginManager *checkoutplugins.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		providerType, ok := parseProviderType(c.Param("type"))
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported provider type"})
			return
		}

		providerID := strings.TrimSpace(c.Param("id"))
		if providerID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Provider ID is required"})
			return
		}

		var req updateCheckoutPluginRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.Enabled == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "enabled is required"})
			return
		}

		if err := pluginManager.SetProviderEnabled(providerType, providerID, *req.Enabled); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := persistCheckoutProviderSettings(db, pluginManager); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save provider settings"})
			return
		}

		payments, shippings, taxes := pluginManager.ListForAdmin()
		c.JSON(http.StatusOK, gin.H{
			"payment":  payments,
			"shipping": shippings,
			"tax":      taxes,
		})
	}
}
