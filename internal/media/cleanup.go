package media

import (
	"os"
	"path/filepath"

	"ecommerce/models"
)

func (s *Service) DeleteIfOrphan(mediaID string) error {
	var count int64
	if err := s.DB.Model(&models.MediaReference{}).Where("media_id = ?", mediaID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	var mediaObj models.MediaObject
	if err := s.DB.Where("id = ?", mediaID).First(&mediaObj).Error; err != nil {
		return err
	}

	var variants []models.MediaVariant
	if err := s.DB.Where("media_id = ?", mediaID).Find(&variants).Error; err != nil {
		return err
	}

	for _, variant := range variants {
		_ = os.Remove(s.LocalPath(variant.Path))
	}
	if mediaObj.OriginalPath != "" {
		_ = os.Remove(s.LocalPath(mediaObj.OriginalPath))
	}

	_ = os.RemoveAll(filepath.Join(s.MediaRoot, mediaID))

	if err := s.DB.Where("media_id = ?", mediaID).Delete(&models.MediaVariant{}).Error; err != nil {
		return err
	}
	if err := s.DB.Where("id = ?", mediaID).Delete(&models.MediaObject{}).Error; err != nil {
		return err
	}

	return nil
}
