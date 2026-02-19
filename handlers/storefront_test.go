package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ecommerce/internal/media"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpsertStorefrontSettingsIsAtomicOnSaveFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTestDB(t, &models.StorefrontSettings{}, &models.MediaObject{}, &models.MediaReference{})
	mediaService := media.NewService(db, "", "", nil)

	initialSettings := defaultStorefrontSettings()
	initialPayload, err := json.Marshal(initialSettings)
	require.NoError(t, err)

	initialRecord := models.StorefrontSettings{
		ID:         models.StorefrontSettingsSingletonID,
		ConfigJSON: string(initialPayload),
	}
	require.NoError(t, db.Create(&initialRecord).Error)

	require.NoError(t, db.Create(&models.MediaObject{
		ID:           "old-hero",
		OriginalPath: "hero/old.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    12,
		Status:       media.StatusReady,
	}).Error)
	require.NoError(t, db.Create(&models.MediaObject{
		ID:           "new-hero",
		OriginalPath: "hero/new.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    12,
		Status:       media.StatusReady,
	}).Error)
	require.NoError(t, db.Create(&models.MediaReference{
		MediaID:   "old-hero",
		OwnerType: media.OwnerTypeStorefront,
		OwnerID:   models.StorefrontSettingsSingletonID,
		Role:      media.RoleStorefrontHero,
		Position:  0,
	}).Error)

	updated := defaultStorefrontSettings()
	foundHero := false
	for i := range updated.HomepageSections {
		section := &updated.HomepageSections[i]
		if section.Type != "hero" || section.Hero == nil {
			continue
		}
		section.Hero.BackgroundImageMediaID = "new-hero"
		section.Hero.BackgroundImageUrl = ""
		foundHero = true
	}
	require.True(t, foundHero, "expected default storefront to include at least one hero section")

	reqBody, err := json.Marshal(UpsertStorefrontSettingsRequest{Settings: updated})
	require.NoError(t, err)

	const callbackName = "test_force_storefront_update_failure"
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Schema == nil {
			return
		}
		if tx.Statement.Schema.Name == "StorefrontSettings" {
			tx.AddError(errors.New("forced storefront save failure"))
		}
	}))
	defer func() {
		_ = db.Callback().Update().Remove(callbackName)
	}()

	r := gin.New()
	r.PUT("/api/v1/admin/storefront", UpsertStorefrontSettings(db, mediaService))

	httpReq := httptest.NewRequest(http.MethodPut, "/api/v1/admin/storefront", strings.NewReader(string(reqBody)))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var refs []models.MediaReference
	require.NoError(t, db.Where(
		"owner_type = ? AND owner_id = ? AND role = ?",
		media.OwnerTypeStorefront,
		models.StorefrontSettingsSingletonID,
		media.RoleStorefrontHero,
	).Order("position asc").Find(&refs).Error)
	require.Len(t, refs, 1)
	assert.Equal(t, "old-hero", refs[0].MediaID)

	var after models.StorefrontSettings
	require.NoError(t, db.First(&after, models.StorefrontSettingsSingletonID).Error)
	assert.Equal(t, string(initialPayload), after.ConfigJSON)
}
