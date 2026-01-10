package media

import (
	"errors"

	"ecommerce/models"
)

func (s *Service) ProductMediaURLs(productID uint) ([]string, error) {
	var refs []models.MediaReference
	if err := s.DB.Where("owner_type = ? AND owner_id = ? AND role = ?",
		OwnerTypeProduct, productID, RoleProductImage).Find(&refs).Error; err != nil {
		return nil, err
	}

	if len(refs) == 0 {
		return []string{}, nil
	}

	mediaIDs := make([]string, 0, len(refs))
	for _, ref := range refs {
		mediaIDs = append(mediaIDs, ref.MediaID)
	}

	var mediaObjs []models.MediaObject
	if err := s.DB.Where("id IN ?", mediaIDs).Find(&mediaObjs).Error; err != nil {
		return nil, err
	}

	urls := make([]string, 0, len(mediaObjs))
	for _, obj := range mediaObjs {
		if obj.Status != StatusReady || obj.OriginalPath == "" {
			continue
		}
		urls = append(urls, s.PublicURLFor(obj.OriginalPath))
	}
	return urls, nil
}

func (s *Service) UserProfilePhotoURL(userID uint) (string, error) {
	var ref models.MediaReference
	if err := s.DB.Where("owner_type = ? AND owner_id = ? AND role = ?",
		OwnerTypeUser, userID, RoleProfilePhoto).First(&ref).Error; err != nil {
		return "", err
	}

	var mediaObj models.MediaObject
	if err := s.DB.Where("id = ?", ref.MediaID).First(&mediaObj).Error; err != nil {
		return "", err
	}
	if mediaObj.Status != StatusReady || mediaObj.OriginalPath == "" {
		return "", errors.New("media not ready")
	}

	var thumb models.MediaVariant
	if err := s.DB.Where("media_id = ? AND label = ?", mediaObj.ID, "thumb_512").First(&thumb).Error; err == nil {
		return s.PublicURLFor(thumb.Path), nil
	}

	return s.PublicURLFor(mediaObj.OriginalPath), nil
}
