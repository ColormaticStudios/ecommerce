package localization

import (
	"context"
	"errors"
	"strings"
	"time"

	"ecommerce/internal/media"
	"ecommerce/models"

	"gorm.io/gorm"
)

const maxTranslationKeyUsages = 50

type UsageInput struct {
	Route             string
	Component         string
	Description       string
	Position          int
	ScreenshotMediaID string
}

type UsageRecord struct {
	models.TranslationKeyUsage
	ScreenshotMediaID string
	ScreenshotURL     string
}

func (s *Service) ListUsages(ctx context.Context, keyID uint, mediaService *media.Service) ([]UsageRecord, error) {
	if keyID == 0 {
		return nil, ErrInvalidTranslation
	}
	var keyCount int64
	if err := s.db.WithContext(ctx).Model(&models.TranslationKey{}).Where("id = ?", keyID).Count(&keyCount).Error; err != nil {
		return nil, err
	}
	if keyCount != 1 {
		return nil, ErrTranslationKeyNotFound
	}
	var usages []models.TranslationKeyUsage
	if err := s.db.WithContext(ctx).Where("translation_key_id = ?", keyID).Order("position ASC, id ASC").Find(&usages).Error; err != nil {
		return nil, err
	}
	result := make([]UsageRecord, 0, len(usages))
	for _, usage := range usages {
		record := UsageRecord{TranslationKeyUsage: usage}
		if mediaService != nil {
			var object models.MediaObject
			err := s.db.WithContext(ctx).Table("media_objects AS mo").
				Select("mo.*").
				Joins("JOIN media_references AS mr ON mr.media_id = mo.id").
				Where("mr.owner_type = ? AND mr.owner_id = ? AND mr.role = ? AND mo.status = ?", media.OwnerTypeTranslationUsage, usage.ID, media.RoleTranslationScreenshot, media.StatusReady).
				First(&object).Error
			if err == nil && object.OriginalPath != "" {
				record.ScreenshotMediaID = object.ID
				record.ScreenshotURL = mediaService.PublicURLFor(object.OriginalPath)
			} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}
		}
		result = append(result, record)
	}
	return result, nil
}

func (s *Service) ReplaceUsages(ctx context.Context, keyID uint, inputs []UsageInput, mediaService *media.Service) ([]UsageRecord, error) {
	if keyID == 0 || len(inputs) > maxTranslationKeyUsages {
		return nil, ErrInvalidTranslation
	}
	mediaIDs := make(map[string]bool)
	for index := range inputs {
		inputs[index].Route = strings.TrimSpace(inputs[index].Route)
		inputs[index].Component = strings.TrimSpace(inputs[index].Component)
		inputs[index].Description = strings.TrimSpace(inputs[index].Description)
		inputs[index].ScreenshotMediaID = strings.TrimSpace(inputs[index].ScreenshotMediaID)
		if len(inputs[index].Route) > 512 || len(inputs[index].Component) > 255 || len(inputs[index].Description) > 2000 || inputs[index].Position < 0 || (inputs[index].Route == "" && inputs[index].Component == "") {
			return nil, ErrInvalidTranslation
		}
		if inputs[index].ScreenshotMediaID != "" {
			if mediaService == nil {
				return nil, ErrInvalidTranslation
			}
			object, err := mediaService.WaitUntilReady(ctx, inputs[index].ScreenshotMediaID, 5*time.Second)
			if err != nil || !strings.HasPrefix(object.MimeType, "image/") {
				return nil, ErrInvalidTranslation
			}
			mediaIDs[inputs[index].ScreenshotMediaID] = true
		}
	}
	var removed []string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var keyCount int64
		if err := tx.Model(&models.TranslationKey{}).Where("id = ?", keyID).Count(&keyCount).Error; err != nil {
			return err
		}
		if keyCount != 1 {
			return ErrTranslationKeyNotFound
		}
		var oldUsages []models.TranslationKeyUsage
		if err := tx.Where("translation_key_id = ?", keyID).Find(&oldUsages).Error; err != nil {
			return err
		}
		oldIDs := make([]uint, 0, len(oldUsages))
		for _, usage := range oldUsages {
			oldIDs = append(oldIDs, usage.ID)
		}
		if len(oldIDs) > 0 {
			var refs []models.MediaReference
			if err := tx.Where("owner_type = ? AND owner_id IN ? AND role = ?", media.OwnerTypeTranslationUsage, oldIDs, media.RoleTranslationScreenshot).Find(&refs).Error; err != nil {
				return err
			}
			for _, ref := range refs {
				if !mediaIDs[ref.MediaID] {
					removed = append(removed, ref.MediaID)
				}
			}
			if err := tx.Where("owner_type = ? AND owner_id IN ? AND role = ?", media.OwnerTypeTranslationUsage, oldIDs, media.RoleTranslationScreenshot).Delete(&models.MediaReference{}).Error; err != nil {
				return err
			}
			if err := tx.Delete(&models.TranslationKeyUsage{}, oldIDs).Error; err != nil {
				return err
			}
		}
		for _, input := range inputs {
			usage := models.TranslationKeyUsage{TranslationKeyID: keyID, Route: input.Route, Component: input.Component, Description: input.Description, Position: input.Position}
			if err := tx.Select("*").Create(&usage).Error; err != nil {
				return err
			}
			if input.ScreenshotMediaID != "" {
				ref := models.MediaReference{MediaID: input.ScreenshotMediaID, OwnerType: media.OwnerTypeTranslationUsage, OwnerID: usage.ID, Role: media.RoleTranslationScreenshot}
				if err := tx.Select("*").Create(&ref).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if mediaService != nil {
		for _, id := range removed {
			_ = mediaService.DeleteIfOrphan(id)
		}
	}
	return s.ListUsages(ctx, keyID, mediaService)
}
