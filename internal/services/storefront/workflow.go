package storefront

import (
	"errors"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
)

func HasDraft(record models.StorefrontSettings) bool {
	return record.DraftConfigJSON != nil && *record.DraftConfigJSON != ""
}

func DraftJSON(record models.StorefrontSettings) string {
	if record.DraftConfigJSON == nil {
		return ""
	}
	return *record.DraftConfigJSON
}

func LoadOrCreateRecord(db *gorm.DB, defaultConfigJSON string) (models.StorefrontSettings, error) {
	var record models.StorefrontSettings
	err := db.First(&record, models.StorefrontSettingsSingletonID).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.StorefrontSettings{}, err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		record = models.StorefrontSettings{
			ID:               models.StorefrontSettingsSingletonID,
			ConfigJSON:       defaultConfigJSON,
			PublishedUpdated: time.Now(),
		}
		if createErr := db.Create(&record).Error; createErr != nil {
			return models.StorefrontSettings{}, createErr
		}
		return record, nil
	}

	if record.PublishedUpdated.IsZero() {
		published := record.UpdatedAt
		if published.IsZero() {
			published = time.Now()
		}
		if updateErr := db.Model(&record).Update("published_updated", published).Error; updateErr == nil {
			record.PublishedUpdated = published
		}
	}
	return record, nil
}

func SaveDraft(tx *gorm.DB, record *models.StorefrontSettings, payload string, now time.Time) error {
	if record == nil {
		return gorm.ErrInvalidData
	}
	return tx.Model(record).Updates(map[string]any{
		"draft_config_json": payload,
		"draft_updated_at":  now,
	}).Error
}

func PublishDraft(tx *gorm.DB, record *models.StorefrontSettings, now time.Time) error {
	if record == nil {
		return gorm.ErrInvalidData
	}
	nilDraft := (*string)(nil)
	nilTime := (*time.Time)(nil)
	return tx.Model(record).Updates(map[string]any{
		"config_json":       DraftJSON(*record),
		"draft_config_json": nilDraft,
		"draft_updated_at":  nilTime,
		"published_updated": now,
	}).Error
}

func DiscardDraft(tx *gorm.DB, record *models.StorefrontSettings) error {
	if record == nil {
		return gorm.ErrInvalidData
	}
	nilDraft := (*string)(nil)
	nilTime := (*time.Time)(nil)
	return tx.Model(record).Updates(map[string]any{
		"draft_config_json": nilDraft,
		"draft_updated_at":  nilTime,
	}).Error
}
