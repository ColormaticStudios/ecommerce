package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func decodeStorefrontResponse(t *testing.T, recorder *httptest.ResponseRecorder) StorefrontSettingsResponse {
	t.Helper()
	var resp StorefrontSettingsResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	return resp
}

func TestStorefrontDraftLifecycleAndHeroMediaPromotion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTestDB(t, &models.StorefrontSettings{}, &models.MediaObject{}, &models.MediaReference{})
	mediaService := media.NewService(db, "", "http://localhost:3000/media", nil)

	initial := defaultStorefrontSettings()
	for i := range initial.HomepageSections {
		if initial.HomepageSections[i].Type == "hero" && initial.HomepageSections[i].Hero != nil {
			initial.HomepageSections[i].Hero.BackgroundImageMediaID = "old-hero"
			initial.HomepageSections[i].Hero.BackgroundImageUrl = ""
			break
		}
	}
	initialRaw, err := json.Marshal(initial)
	require.NoError(t, err)

	require.NoError(t, db.Create(&models.StorefrontSettings{
		ID:               models.StorefrontSettingsSingletonID,
		ConfigJSON:       string(initialRaw),
		PublishedUpdated: time.Now(),
	}).Error)
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

	r := gin.New()
	r.GET("/public", GetStorefrontSettings(db, mediaService))
	r.GET("/admin", GetAdminStorefrontSettings(db, mediaService))
	r.PUT("/admin", UpsertStorefrontSettings(db, mediaService))
	r.POST("/admin/publish", PublishStorefrontSettings(db, mediaService))
	r.DELETE("/admin/draft", DiscardStorefrontDraft(db, mediaService))

	publicBefore := httptest.NewRecorder()
	r.ServeHTTP(publicBefore, httptest.NewRequest(http.MethodGet, "/public", nil))
	require.Equal(t, http.StatusOK, publicBefore.Code)
	publicBeforeBody := decodeStorefrontResponse(t, publicBefore)
	require.NotEmpty(t, publicBeforeBody.Settings.SiteTitle)
	assert.False(t, publicBeforeBody.HasDraftChanges)

	draft := defaultStorefrontSettings()
	draft.SiteTitle = "Draft storefront"
	for i := range draft.HomepageSections {
		section := &draft.HomepageSections[i]
		if section.Type == "hero" && section.Hero != nil {
			section.Hero.BackgroundImageMediaID = "new-hero"
			section.Hero.BackgroundImageUrl = ""
		}
	}
	draftBody, err := json.Marshal(UpsertStorefrontSettingsRequest{Settings: draft})
	require.NoError(t, err)

	upsertReq := httptest.NewRequest(http.MethodPut, "/admin", strings.NewReader(string(draftBody)))
	upsertReq.Header.Set("Content-Type", "application/json")
	upsertResp := httptest.NewRecorder()
	r.ServeHTTP(upsertResp, upsertReq)
	require.Equal(t, http.StatusOK, upsertResp.Code)
	upsertBody := decodeStorefrontResponse(t, upsertResp)
	assert.True(t, upsertBody.HasDraftChanges)
	assert.Equal(t, "Draft storefront", upsertBody.Settings.SiteTitle)

	publicAfterDraft := httptest.NewRecorder()
	r.ServeHTTP(publicAfterDraft, httptest.NewRequest(http.MethodGet, "/public", nil))
	require.Equal(t, http.StatusOK, publicAfterDraft.Code)
	publicAfterDraftBody := decodeStorefrontResponse(t, publicAfterDraft)
	assert.Equal(t, publicBeforeBody.Settings.SiteTitle, publicAfterDraftBody.Settings.SiteTitle)

	var liveRefs []models.MediaReference
	require.NoError(t, db.Where("owner_type = ? AND owner_id = ? AND role = ?",
		media.OwnerTypeStorefront, models.StorefrontSettingsSingletonID, media.RoleStorefrontHero).
		Find(&liveRefs).Error)
	require.Len(t, liveRefs, 1)
	assert.Equal(t, "old-hero", liveRefs[0].MediaID)

	var draftRefs []models.MediaReference
	require.NoError(t, db.Where("owner_type = ? AND owner_id = ? AND role = ?",
		media.OwnerTypeStorefront, models.StorefrontSettingsSingletonID, media.RoleStorefrontHeroDraft).
		Find(&draftRefs).Error)
	require.Len(t, draftRefs, 1)
	assert.Equal(t, "new-hero", draftRefs[0].MediaID)

	publishResp := httptest.NewRecorder()
	r.ServeHTTP(publishResp, httptest.NewRequest(http.MethodPost, "/admin/publish", nil))
	require.Equal(t, http.StatusOK, publishResp.Code)
	publishBody := decodeStorefrontResponse(t, publishResp)
	assert.False(t, publishBody.HasDraftChanges)
	assert.Equal(t, "Draft storefront", publishBody.Settings.SiteTitle)

	publicAfterPublish := httptest.NewRecorder()
	r.ServeHTTP(publicAfterPublish, httptest.NewRequest(http.MethodGet, "/public", nil))
	require.Equal(t, http.StatusOK, publicAfterPublish.Code)
	publicAfterPublishBody := decodeStorefrontResponse(t, publicAfterPublish)
	assert.Equal(t, "Draft storefront", publicAfterPublishBody.Settings.SiteTitle)

	require.NoError(t, db.Where("owner_type = ? AND owner_id = ? AND role = ?",
		media.OwnerTypeStorefront, models.StorefrontSettingsSingletonID, media.RoleStorefrontHero).
		Find(&liveRefs).Error)
	require.Len(t, liveRefs, 1)
	assert.Equal(t, "new-hero", liveRefs[0].MediaID)

	var draftCount int64
	require.NoError(t, db.Model(&models.MediaReference{}).Where("owner_type = ? AND owner_id = ? AND role = ?",
		media.OwnerTypeStorefront, models.StorefrontSettingsSingletonID, media.RoleStorefrontHeroDraft).
		Count(&draftCount).Error)
	assert.EqualValues(t, 0, draftCount)

	draft.SiteTitle = "Should be discarded"
	secondDraftRaw, err := json.Marshal(UpsertStorefrontSettingsRequest{Settings: draft})
	require.NoError(t, err)
	secondUpsertReq := httptest.NewRequest(http.MethodPut, "/admin", strings.NewReader(string(secondDraftRaw)))
	secondUpsertReq.Header.Set("Content-Type", "application/json")
	secondUpsertResp := httptest.NewRecorder()
	r.ServeHTTP(secondUpsertResp, secondUpsertReq)
	require.Equal(t, http.StatusOK, secondUpsertResp.Code)

	discardResp := httptest.NewRecorder()
	r.ServeHTTP(discardResp, httptest.NewRequest(http.MethodDelete, "/admin/draft", nil))
	require.Equal(t, http.StatusOK, discardResp.Code)
	discardBody := decodeStorefrontResponse(t, discardResp)
	assert.False(t, discardBody.HasDraftChanges)
	assert.Equal(t, "Draft storefront", discardBody.Settings.SiteTitle)
}
