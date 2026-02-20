package handlers

import (
	"encoding/json"
	"strings"
	"time"

	"ecommerce/internal/media"
	"ecommerce/models"

	"gorm.io/gorm"
)

type productDraftData struct {
	SKU         string   `json:"sku"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       float64  `json:"price"`
	Stock       int      `json:"stock"`
	Images      []string `json:"images"`
	RelatedIDs  []uint   `json:"related_ids"`
}

func normalizeProductDraftData(input productDraftData) productDraftData {
	normalized := productDraftData{
		SKU:         strings.TrimSpace(input.SKU),
		Name:        strings.TrimSpace(input.Name),
		Description: strings.TrimSpace(input.Description),
		Price:       input.Price,
		Stock:       input.Stock,
		Images:      make([]string, 0, len(input.Images)),
		RelatedIDs:  make([]uint, 0, len(input.RelatedIDs)),
	}
	if normalized.Stock < 0 {
		normalized.Stock = 0
	}

	seenImages := make(map[string]struct{}, len(input.Images))
	for _, image := range input.Images {
		value := strings.TrimSpace(image)
		if value == "" {
			continue
		}
		if _, exists := seenImages[value]; exists {
			continue
		}
		seenImages[value] = struct{}{}
		normalized.Images = append(normalized.Images, value)
	}

	seenRelated := make(map[uint]struct{}, len(input.RelatedIDs))
	for _, relatedID := range input.RelatedIDs {
		if relatedID == 0 {
			continue
		}
		if _, exists := seenRelated[relatedID]; exists {
			continue
		}
		seenRelated[relatedID] = struct{}{}
		normalized.RelatedIDs = append(normalized.RelatedIDs, relatedID)
	}

	return normalized
}

func productHasDraft(product models.Product) bool {
	return strings.TrimSpace(product.DraftData) != ""
}

func productIsPubliclyVisible(product models.Product) bool {
	return product.IsPublished
}

func parseProductDraftData(raw string) (*productDraftData, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	var parsed productDraftData
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil, err
	}
	normalized := normalizeProductDraftData(parsed)
	return &normalized, nil
}

func encodeProductDraftData(draft productDraftData) (string, error) {
	normalized := normalizeProductDraftData(draft)
	payload, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func productRelatedIDs(product models.Product) []uint {
	ids := make([]uint, 0, len(product.Related))
	seen := make(map[uint]struct{}, len(product.Related))
	for _, related := range product.Related {
		if related.ID == 0 {
			continue
		}
		if _, exists := seen[related.ID]; exists {
			continue
		}
		seen[related.ID] = struct{}{}
		ids = append(ids, related.ID)
	}
	return ids
}

func productDraftFromPublished(product models.Product) productDraftData {
	return normalizeProductDraftData(productDraftData{
		SKU:         product.SKU,
		Name:        product.Name,
		Description: product.Description,
		Price:       product.Price.Float64(),
		Stock:       product.Stock,
		Images:      append([]string(nil), product.Images...),
		RelatedIDs:  productRelatedIDs(product),
	})
}

func editableProductDraftData(product models.Product) (productDraftData, bool, error) {
	draft, err := parseProductDraftData(product.DraftData)
	if err != nil {
		return productDraftData{}, false, err
	}
	if draft == nil {
		return productDraftFromPublished(product), false, nil
	}
	return *draft, true, nil
}

func applyDraftDataToProduct(product models.Product, draft productDraftData) models.Product {
	normalized := normalizeProductDraftData(draft)
	product.SKU = normalized.SKU
	product.Name = normalized.Name
	product.Description = normalized.Description
	product.Price = models.MoneyFromFloat(normalized.Price)
	product.Stock = normalized.Stock
	product.Images = append([]string(nil), normalized.Images...)
	return product
}

func loadProductMediaReferences(tx *gorm.DB, productID uint, role string) ([]models.MediaReference, error) {
	if !hasMediaReferenceTable(tx) {
		return []models.MediaReference{}, nil
	}

	var refs []models.MediaReference
	if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?",
		media.OwnerTypeProduct, productID, role).
		Order("position asc").
		Order("id asc").
		Find(&refs).Error; err != nil {
		return nil, err
	}
	return refs, nil
}

func replaceProductMediaReferences(tx *gorm.DB, productID uint, role string, refs []models.MediaReference) error {
	if !hasMediaReferenceTable(tx) {
		return nil
	}

	if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?",
		media.OwnerTypeProduct, productID, role).
		Delete(&models.MediaReference{}).Error; err != nil {
		return err
	}

	for _, ref := range refs {
		if err := tx.Create(&models.MediaReference{
			MediaID:   ref.MediaID,
			OwnerType: media.OwnerTypeProduct,
			OwnerID:   productID,
			Role:      role,
			Position:  ref.Position,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func copyProductMediaRole(tx *gorm.DB, productID uint, fromRole string, toRole string) error {
	refs, err := loadProductMediaReferences(tx, productID, fromRole)
	if err != nil {
		return err
	}
	return replaceProductMediaReferences(tx, productID, toRole, refs)
}

func upsertProductDraft(tx *gorm.DB, product *models.Product, draft productDraftData) error {
	payload, err := encodeProductDraftData(draft)
	if err != nil {
		return err
	}
	now := time.Now()
	updates := map[string]any{
		"draft_data":       payload,
		"draft_updated_at": now,
	}
	if err := tx.Model(product).Updates(updates).Error; err != nil {
		return err
	}
	product.DraftData = payload
	product.DraftUpdatedAt = &now
	return nil
}

func ensureProductDraft(tx *gorm.DB, product *models.Product) (productDraftData, bool, error) {
	draft, hasDraft, err := editableProductDraftData(*product)
	if err != nil {
		return productDraftData{}, false, err
	}
	if hasDraft {
		return draft, true, nil
	}

	if err := upsertProductDraft(tx, product, draft); err != nil {
		return productDraftData{}, false, err
	}
	if err := copyProductMediaRole(tx, product.ID, media.RoleProductImage, media.RoleProductDraftImage); err != nil {
		return productDraftData{}, false, err
	}
	return draft, false, nil
}

func cleanupMediaIDs(mediaService *media.Service, mediaIDs []string) {
	if mediaService == nil || len(mediaIDs) == 0 {
		return
	}
	seen := make(map[string]struct{}, len(mediaIDs))
	for _, mediaID := range mediaIDs {
		if mediaID == "" {
			continue
		}
		if _, exists := seen[mediaID]; exists {
			continue
		}
		seen[mediaID] = struct{}{}
		_ = mediaService.DeleteIfOrphan(mediaID)
	}
}

func hasMediaReferenceTable(tx *gorm.DB) bool {
	return tx != nil && tx.Migrator().HasTable(&models.MediaReference{})
}
