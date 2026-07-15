package cms

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"ecommerce/internal/media"
	"ecommerce/models"

	"gorm.io/gorm"
)

func collectPayloadMediaIDs(payload PagePayload) []string {
	blocks, _ := payload["blocks"].([]any)
	seen := map[string]bool{}
	ids := make([]string, 0)
	add := func(value any) {
		id, ok := value.(string)
		id = strings.TrimSpace(id)
		if ok && id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, rawBlock := range blocks {
		block, ok := rawBlock.(map[string]any)
		if !ok {
			continue
		}
		add(block["image_media_id"])
		add(block["media_id"])
		if images, ok := block["images"].([]any); ok {
			for _, rawImage := range images {
				if image, ok := rawImage.(map[string]any); ok {
					add(image["media_id"])
				}
			}
		}
		if categoryMedia, ok := block["category_media_ids"].(map[string]any); ok {
			for _, mediaID := range categoryMedia {
				add(mediaID)
			}
		}
	}
	return ids
}

func payloadFromVersion(version models.CMSEntryVersion) (PagePayload, error) {
	var payload PagePayload
	if err := json.Unmarshal([]byte(version.PayloadJSON), &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func syncContentMediaReferences(tx *gorm.DB, entryID uint, payload PagePayload, role string) ([]string, error) {
	return syncPayloadMediaReferences(tx, media.OwnerTypeCMSEntry, entryID, payload, role)
}

func deleteContentMediaReferences(tx *gorm.DB, entryID uint) ([]string, error) {
	return deleteRoleContentMediaReferences(tx, entryID, "")
}

func deleteRoleContentMediaReferences(tx *gorm.DB, entryID uint, role string) ([]string, error) {
	if !tx.Migrator().HasTable(&models.MediaReference{}) {
		return nil, nil
	}
	query := tx.Where("owner_type = ? AND owner_id = ?", media.OwnerTypeCMSEntry, entryID)
	if role != "" {
		query = query.Where("role = ?", role)
	}
	var refs []models.MediaReference
	if err := query.Find(&refs).Error; err != nil {
		return nil, err
	}
	if err := query.Delete(&models.MediaReference{}).Error; err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(refs))
	seen := map[string]bool{}
	for _, ref := range refs {
		if ref.MediaID != "" && !seen[ref.MediaID] {
			seen[ref.MediaID] = true
			ids = append(ids, ref.MediaID)
		}
	}
	return ids, nil
}

func syncVariantMediaReferences(tx *gorm.DB, variantID uint, payload PagePayload) ([]string, error) {
	return syncPayloadMediaReferences(tx, media.OwnerTypeCMSPageVariant, variantID, payload, media.RoleCMSDraftContent)
}

func syncPayloadMediaReferences(tx *gorm.DB, ownerType string, ownerID uint, payload PagePayload, role string) ([]string, error) {
	if !tx.Migrator().HasTable(&models.MediaReference{}) {
		return nil, nil
	}
	ids := collectPayloadMediaIDs(payload)
	for _, id := range ids {
		var object models.MediaObject
		if err := tx.First(&object, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("%w: media %q was not found", ErrInvalidPage, id)
			}
			return nil, err
		}
		if object.Status != media.StatusReady || strings.TrimSpace(object.OriginalPath) == "" {
			return nil, fmt.Errorf("%w: media %q is not ready", ErrInvalidPage, id)
		}
		if !strings.HasPrefix(object.MimeType, "image/") {
			return nil, fmt.Errorf("%w: media %q must be an image", ErrInvalidPage, id)
		}
	}
	var existing []models.MediaReference
	if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?", ownerType, ownerID, role).
		Order("position ASC, id ASC").Find(&existing).Error; err != nil {
		return nil, err
	}
	if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?", ownerType, ownerID, role).
		Delete(&models.MediaReference{}).Error; err != nil {
		return nil, err
	}
	for position, id := range ids {
		if err := tx.Create(&models.MediaReference{
			MediaID: id, OwnerType: ownerType, OwnerID: ownerID, Role: role, Position: position,
		}).Error; err != nil {
			return nil, err
		}
	}
	active := map[string]bool{}
	for _, id := range ids {
		active[id] = true
	}
	removed := make([]string, 0)
	for _, ref := range existing {
		if !active[ref.MediaID] {
			removed = append(removed, ref.MediaID)
		}
	}
	return removed, nil
}

func (s *Service) cleanupOrphanMedia(ids []string) {
	if s.media == nil {
		return
	}
	for _, id := range ids {
		_ = s.media.DeleteIfOrphan(id)
	}
}
