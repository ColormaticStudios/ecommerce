package media

import (
	"errors"

	"ecommerce/models"
)

func (s *Service) ProductMediaURLs(productID uint) ([]string, error) {
	var refs []models.MediaReference
	if err := s.DB.Where("owner_type = ? AND owner_id = ? AND role = ?",
		OwnerTypeProduct, productID, RoleProductImage).
		Order("position asc").
		Order("created_at asc").
		Order("id asc").
		Find(&refs).Error; err != nil {
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

	mediaByID := make(map[string]models.MediaObject, len(mediaObjs))
	for _, obj := range mediaObjs {
		mediaByID[obj.ID] = obj
	}

	urls := make([]string, 0, len(refs))
	for _, ref := range refs {
		obj, ok := mediaByID[ref.MediaID]
		if !ok {
			continue
		}
		if obj.Status != StatusReady || obj.OriginalPath == "" {
			continue
		}
		urls = append(urls, s.PublicURLFor(obj.OriginalPath))
	}
	return urls, nil
}

func (s *Service) ProductMediaURLsByProductIDs(productIDs []uint) (map[uint][]string, error) {
	urlsByProduct := make(map[uint][]string)
	if len(productIDs) == 0 {
		return urlsByProduct, nil
	}

	uniqueIDs := make([]uint, 0, len(productIDs))
	seen := make(map[uint]struct{}, len(productIDs))
	for _, id := range productIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}

	if len(uniqueIDs) == 0 {
		return urlsByProduct, nil
	}

	var refs []models.MediaReference
	if err := s.DB.Where("owner_type = ? AND owner_id IN ? AND role = ?",
		OwnerTypeProduct, uniqueIDs, RoleProductImage).
		Order("owner_id asc").
		Order("position asc").
		Order("created_at asc").
		Order("id asc").
		Find(&refs).Error; err != nil {
		return nil, err
	}

	if len(refs) == 0 {
		return urlsByProduct, nil
	}

	mediaIDs := make([]string, 0, len(refs))
	mediaSeen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if _, ok := mediaSeen[ref.MediaID]; ok {
			continue
		}
		mediaSeen[ref.MediaID] = struct{}{}
		mediaIDs = append(mediaIDs, ref.MediaID)
	}

	if len(mediaIDs) == 0 {
		return urlsByProduct, nil
	}

	var mediaObjs []models.MediaObject
	if err := s.DB.Where("id IN ?", mediaIDs).Find(&mediaObjs).Error; err != nil {
		return nil, err
	}

	mediaByID := make(map[string]models.MediaObject, len(mediaObjs))
	for _, obj := range mediaObjs {
		mediaByID[obj.ID] = obj
	}

	for _, ref := range refs {
		obj, ok := mediaByID[ref.MediaID]
		if !ok {
			continue
		}
		if obj.Status != StatusReady || obj.OriginalPath == "" {
			continue
		}
		urlsByProduct[ref.OwnerID] = append(urlsByProduct[ref.OwnerID], s.PublicURLFor(obj.OriginalPath))
	}

	return urlsByProduct, nil
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

func (s *Service) StorefrontHeroImage(storefrontID uint) (string, string, error) {
	var ref models.MediaReference
	if err := s.DB.Where("owner_type = ? AND owner_id = ? AND role = ?",
		OwnerTypeStorefront, storefrontID, RoleStorefrontHero).First(&ref).Error; err != nil {
		return "", "", err
	}

	var mediaObj models.MediaObject
	if err := s.DB.Where("id = ?", ref.MediaID).First(&mediaObj).Error; err != nil {
		return "", "", err
	}
	if mediaObj.Status != StatusReady || mediaObj.OriginalPath == "" {
		return "", "", errors.New("media not ready")
	}

	return ref.MediaID, s.PublicURLFor(mediaObj.OriginalPath), nil
}
