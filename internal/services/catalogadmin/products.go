package catalogadmin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/apperror"
	"ecommerce/internal/media"
	"ecommerce/models"

	"gorm.io/gorm"
)

func (s *Service) GetProduct(ctx context.Context, id uint, draft bool) (models.Product, error) {
	db := s.db.WithContext(ctx)
	var product models.Product
	if err := db.Preload("Related").Preload("Categories").Preload("Variants").First(&product, id).Error; err != nil {
		return product, err
	}
	if !draft || product.DraftUpdatedAt == nil {
		return product, nil
	}
	var value models.ProductDraft
	if err := db.Preload("VariantDrafts").Preload("RelatedDrafts").Preload("CategoryDrafts").Where("product_id = ?", id).First(&value).Error; err != nil {
		return product, err
	}
	product.SKU, product.Name, product.Subtitle, product.Description = value.SKU, value.Name, value.Subtitle, value.Description
	product.Price, product.Stock, product.BrandID = value.Price, value.Stock, value.BrandID
	_ = json.Unmarshal([]byte(value.ImagesJSON), &product.Images)
	product.Variants = make([]models.ProductVariant, 0, len(value.VariantDrafts))
	for _, item := range value.VariantDrafts {
		if !item.IsDeleted {
			product.Variants = append(product.Variants, models.ProductVariant{BaseModel: item.BaseModel, ProductID: id, SKU: item.SKU, Title: item.Title, Price: item.Price, CompareAtPrice: item.CompareAtPrice, Stock: item.Stock, Position: item.Position, IsPublished: item.IsPublished, WeightGrams: item.WeightGrams, LengthCm: item.LengthCm, WidthCm: item.WidthCm, HeightCm: item.HeightCm})
		}
	}
	categoryIDs := make([]uint, 0, len(value.CategoryDrafts))
	for _, item := range value.CategoryDrafts {
		categoryIDs = append(categoryIDs, item.CategoryID)
	}
	if len(categoryIDs) != 0 {
		_ = db.Where("id IN ?", categoryIDs).Find(&product.Categories).Error
	} else {
		product.Categories = []models.Category{}
	}
	relatedIDs := make([]uint, 0, len(value.RelatedDrafts))
	for _, item := range value.RelatedDrafts {
		relatedIDs = append(relatedIDs, item.RelatedProductID)
	}
	if len(relatedIDs) != 0 {
		_ = db.Where("id IN ?", relatedIDs).Find(&product.Related).Error
	} else {
		product.Related = []models.Product{}
	}
	return product, nil
}

func (s *Service) CreateProduct(ctx context.Context, input apicontract.ProductUpsertInput) (models.Product, error) {
	if err := validateProductInput(input); err != nil {
		return models.Product{}, err
	}
	now := time.Now().UTC()
	price, stock := productSummary(input)
	product := models.Product{SKU: strings.TrimSpace(input.Sku), Name: strings.TrimSpace(input.Name), Subtitle: input.Subtitle, Description: strings.TrimSpace(input.Description), Price: models.MoneyFromFloat(price), Stock: stock, Images: append([]string(nil), input.Images...), IsPublished: false, DraftUpdatedAt: &now}
	if input.BrandId != nil {
		id := uint(*input.BrandId)
		product.BrandID = &id
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Select("*").Create(&product).Error; err != nil {
			return err
		}
		// GORM applies the model's default:true tag to a false bool during Create,
		// including selected zero-value fields. New admin products are drafts, so
		// persist and restore the required false value explicitly.
		if err := tx.Model(&product).Update("is_published", false).Error; err != nil {
			return err
		}
		product.IsPublished = false
		return replaceDraft(tx, product, input)
	})
	if err != nil {
		return models.Product{}, err
	}
	return s.GetProduct(ctx, product.ID, true)
}

func (s *Service) UpdateProduct(ctx context.Context, id uint, input apicontract.ProductUpsertInput) (models.Product, error) {
	if err := validateProductInput(input); err != nil {
		return models.Product{}, err
	}
	db := s.db.WithContext(ctx)
	var product models.Product
	if err := db.First(&product, id).Error; err != nil {
		return product, err
	}
	now := time.Now().UTC()
	hadDraft := product.DraftUpdatedAt != nil
	product.DraftUpdatedAt = &now
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&product).Update("draft_updated_at", now).Error; err != nil {
			return err
		}
		if err := replaceDraft(tx, product, input); err != nil {
			return err
		}
		if !hadDraft {
			return copyProductMediaRole(tx, id, media.RoleProductImage, media.RoleProductDraftImage)
		}
		return nil
	}); err != nil {
		return product, err
	}
	return s.GetProduct(ctx, id, true)
}

func validateProductInput(input apicontract.ProductUpsertInput) error {
	if strings.TrimSpace(input.Sku) == "" {
		return invalidInput("invalid_product", "Product SKU is required.")
	}
	if strings.TrimSpace(input.Name) == "" {
		return invalidInput("invalid_product", "Product name is required.")
	}
	if len(input.Variants) == 0 {
		return invalidInput("invalid_product", "At least one product variant is required.")
	}
	seen := map[string]struct{}{}
	for _, value := range input.Variants {
		sku := strings.TrimSpace(value.Sku)
		if sku == "" {
			return invalidInput("invalid_product_variant", "Variant SKU is required.")
		}
		if value.Price < 0 || value.Stock < 0 {
			return invalidInput("invalid_product_variant", "Variant price and stock cannot be negative.")
		}
		if _, ok := seen[sku]; ok {
			return invalidInput("invalid_product_variant", "Variant SKUs must be unique.")
		}
		seen[sku] = struct{}{}
	}
	return nil
}
func productSummary(input apicontract.ProductUpsertInput) (float64, int) {
	value := input.Variants[0]
	if input.DefaultVariantSku != nil {
		for _, item := range input.Variants {
			if item.Sku == *input.DefaultVariantSku {
				value = item
				break
			}
		}
	}
	return value.Price, value.Stock
}
func replaceDraft(tx *gorm.DB, product models.Product, input apicontract.ProductUpsertInput) error {
	var old models.ProductDraft
	if err := tx.Where("product_id = ?", product.ID).First(&old).Error; err == nil {
		if err := deleteDraft(tx, old.ID); err != nil {
			return err
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	price, stock := productSummary(input)
	images, _ := json.Marshal(input.Images)
	defaultSKU := ""
	if input.DefaultVariantSku != nil {
		defaultSKU = *input.DefaultVariantSku
	}
	draft := models.ProductDraft{ProductID: product.ID, Version: 1, SKU: strings.TrimSpace(input.Sku), DefaultVariantSKU: defaultSKU, Name: strings.TrimSpace(input.Name), Subtitle: input.Subtitle, Description: strings.TrimSpace(input.Description), Price: models.MoneyFromFloat(price), Stock: stock, ImagesJSON: string(images), SeoTitle: input.Seo.Title, SeoDescription: input.Seo.Description, SeoCanonicalPath: input.Seo.CanonicalPath, SeoOgImageMediaID: input.Seo.OgImageMediaId}
	if input.Seo.Noindex != nil {
		draft.SeoNoIndex = *input.Seo.Noindex
	}
	if input.BrandId != nil {
		id := uint(*input.BrandId)
		draft.BrandID = &id
	}
	if err := tx.Select("*").Create(&draft).Error; err != nil {
		return err
	}
	for index, item := range input.Variants {
		published := true
		if item.IsPublished != nil {
			published = *item.IsPublished
		}
		position := index + 1
		if item.Position != nil {
			position = *item.Position
		}
		var compare *models.Money
		if item.CompareAtPrice != nil {
			value := models.MoneyFromFloat(*item.CompareAtPrice)
			compare = &value
		}
		value := models.ProductVariantDraft{ProductDraftID: draft.ID, SKU: strings.TrimSpace(item.Sku), Title: strings.TrimSpace(item.Title), Price: models.MoneyFromFloat(item.Price), CompareAtPrice: compare, Stock: item.Stock, Position: position, IsPublished: published, WeightGrams: item.WeightGrams, LengthCm: item.LengthCm, WidthCm: item.WidthCm, HeightCm: item.HeightCm}
		if err := tx.Select("*").Create(&value).Error; err != nil {
			return err
		}
	}
	for index, id := range input.CategoryIds {
		if id > 0 {
			if err := tx.Create(&models.ProductCategoryDraft{ProductDraftID: draft.ID, CategoryID: uint(id), Position: index + 1}).Error; err != nil {
				return err
			}
		}
	}
	for index, id := range input.RelatedProductIds {
		if id > 0 && uint(id) != product.ID {
			if err := tx.Create(&models.ProductRelatedDraft{ProductDraftID: draft.ID, RelatedProductID: uint(id), Position: index + 1}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
func deleteDraft(tx *gorm.DB, id uint) error {
	for _, value := range []any{&models.ProductVariantDraft{}, &models.ProductRelatedDraft{}, &models.ProductCategoryDraft{}, &models.ProductAttributeValueDraft{}, &models.ProductOptionDraft{}} {
		if err := tx.Where("product_draft_id = ?", id).Delete(value).Error; err != nil {
			return err
		}
	}
	return tx.Delete(&models.ProductDraft{}, id).Error
}

func (s *Service) PublishProduct(ctx context.Context, id uint) (models.Product, error) {
	db := s.db.WithContext(ctx)
	var product models.Product
	if err := db.First(&product, id).Error; err != nil {
		return product, err
	}
	var draft models.ProductDraft
	if err := db.Preload("VariantDrafts").Preload("RelatedDrafts").Preload("CategoryDrafts").Where("product_id = ?", id).First(&draft).Error; err != nil {
		return product, err
	}
	var removedMediaIDs []string
	err := db.Transaction(func(tx *gorm.DB) error {
		var oldLiveRefs []models.MediaReference
		if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeProduct, id, media.RoleProductImage).
			Order("position asc, created_at asc, id asc").Find(&oldLiveRefs).Error; err != nil {
			return err
		}
		var draftRefs []models.MediaReference
		if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeProduct, id, media.RoleProductDraftImage).
			Order("position asc, created_at asc, id asc").Find(&draftRefs).Error; err != nil {
			return err
		}

		product.SKU, product.Name, product.Subtitle, product.Description, product.Price, product.Stock, product.BrandID = draft.SKU, draft.Name, draft.Subtitle, draft.Description, draft.Price, draft.Stock, draft.BrandID
		product.IsPublished, product.DraftUpdatedAt = true, nil
		_ = json.Unmarshal([]byte(draft.ImagesJSON), &product.Images)
		if err := tx.Select("*").Save(&product).Error; err != nil {
			return err
		}
		if err := tx.Where("product_id = ?", id).Delete(&models.ProductVariant{}).Error; err != nil {
			return err
		}
		for _, item := range draft.VariantDrafts {
			if item.IsDeleted {
				continue
			}
			value := models.ProductVariant{ProductID: id, SKU: item.SKU, Title: item.Title, Price: item.Price, CompareAtPrice: item.CompareAtPrice, Stock: item.Stock, Position: item.Position, IsPublished: item.IsPublished, WeightGrams: item.WeightGrams, LengthCm: item.LengthCm, WidthCm: item.WidthCm, HeightCm: item.HeightCm}
			if err := tx.Select("*").Create(&value).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("product_id = ?", id).Delete(&models.ProductCategory{}).Error; err != nil {
			return err
		}
		for _, item := range draft.CategoryDrafts {
			if err := tx.Create(&models.ProductCategory{ProductID: id, CategoryID: item.CategoryID}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&product).Association("Related").Clear(); err != nil {
			return err
		}
		if len(draft.RelatedDrafts) != 0 {
			ids := make([]uint, 0, len(draft.RelatedDrafts))
			for _, item := range draft.RelatedDrafts {
				ids = append(ids, item.RelatedProductID)
			}
			var related []models.Product
			if err := tx.Where("id IN ?", ids).Find(&related).Error; err != nil {
				return err
			}
			if err := tx.Model(&product).Association("Related").Replace(related); err != nil {
				return err
			}
		}
		if err := replaceProductMediaRole(tx, id, media.RoleProductImage, draftRefs); err != nil {
			return err
		}
		if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeProduct, id, media.RoleProductDraftImage).Delete(&models.MediaReference{}).Error; err != nil {
			return err
		}
		removedMediaIDs = removedReferenceIDs(oldLiveRefs, draftRefs)
		return deleteDraft(tx, draft.ID)
	})
	if err != nil {
		return product, err
	}
	s.cleanupMedia(removedMediaIDs)
	return s.GetProduct(ctx, id, false)
}
func (s *Service) UnpublishProduct(ctx context.Context, id uint) (models.Product, error) {
	db := s.db.WithContext(ctx)
	var product models.Product
	if err := db.Preload("Related").Preload("Categories").Preload("Variants").First(&product, id).Error; err != nil {
		return product, err
	}
	if product.DraftUpdatedAt == nil {
		input := productInputFromLive(product)
		if err := db.Transaction(func(tx *gorm.DB) error {
			now := time.Now().UTC()
			if err := tx.Model(&product).Updates(map[string]any{"is_published": false, "draft_updated_at": now}).Error; err != nil {
				return err
			}
			if err := replaceDraft(tx, product, input); err != nil {
				return err
			}
			return copyProductMediaRole(tx, id, media.RoleProductImage, media.RoleProductDraftImage)
		}); err != nil {
			return product, err
		}
	} else if err := db.Model(&product).Update("is_published", false).Error; err != nil {
		return product, err
	}
	return s.GetProduct(ctx, id, true)
}
func productInputFromLive(product models.Product) apicontract.ProductUpsertInput {
	input := apicontract.ProductUpsertInput{Sku: product.SKU, Name: product.Name, Subtitle: product.Subtitle, Description: product.Description, Images: product.Images, Seo: apicontract.ProductSEOInput{}}
	for _, item := range product.Variants {
		published := item.IsPublished
		input.Variants = append(input.Variants, apicontract.ProductVariantInput{Sku: item.SKU, Title: item.Title, Price: item.Price.Float64(), Stock: item.Stock, IsPublished: &published, Position: &item.Position})
	}
	for _, item := range product.Categories {
		input.CategoryIds = append(input.CategoryIds, int(item.ID))
	}
	for _, item := range product.Related {
		input.RelatedProductIds = append(input.RelatedProductIds, int(item.ID))
	}
	return input
}
func (s *Service) DiscardProductDraft(ctx context.Context, id uint) (models.Product, error) {
	db := s.db.WithContext(ctx)
	var product models.Product
	if err := db.First(&product, id).Error; err != nil {
		return product, err
	}
	if product.DraftUpdatedAt != nil && product.IsPublished {
		var draft models.ProductDraft
		if err := db.Where("product_id = ?", id).First(&draft).Error; err != nil {
			return product, err
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := deleteDraft(tx, draft.ID); err != nil {
				return err
			}
			return tx.Model(&product).Update("draft_updated_at", nil).Error
		}); err != nil {
			return product, err
		}
	}
	return s.GetProduct(ctx, id, product.DraftUpdatedAt != nil)
}
func (s *Service) DeleteProduct(ctx context.Context, id uint) ([]string, error) {
	db := s.db.WithContext(ctx)
	var product models.Product
	if err := db.First(&product, id).Error; err != nil {
		return nil, err
	}
	var refs []models.MediaReference
	_ = db.Where("owner_type = ? AND owner_id = ?", media.OwnerTypeProduct, id).Find(&refs).Error
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("owner_type = ? AND owner_id = ?", media.OwnerTypeProduct, id).Delete(&models.MediaReference{}).Error; err != nil {
			return err
		}
		return tx.Delete(&product).Error
	})
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		ids = append(ids, ref.MediaID)
	}
	return ids, err
}

func (s *Service) ReplaceRelated(ctx context.Context, id uint, relatedIDs []int) (models.Product, error) {
	db := s.db.WithContext(ctx)
	var product models.Product
	if err := db.Preload("Related").Preload("Categories").Preload("Variants").First(&product, id).Error; err != nil {
		return product, err
	}
	if product.DraftUpdatedAt == nil {
		if err := db.Transaction(func(tx *gorm.DB) error {
			return ensureProductDraft(tx, &product)
		}); err != nil {
			return product, err
		}
	}
	var draft models.ProductDraft
	if err := db.Where("product_id = ?", id).First(&draft).Error; err != nil {
		return product, err
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("product_draft_id = ?", draft.ID).Delete(&models.ProductRelatedDraft{}).Error; err != nil {
			return err
		}
		seen := map[uint]struct{}{}
		for position, rawID := range relatedIDs {
			if rawID < 1 || uint(rawID) == id {
				return invalidInput("invalid_related_product", "Related product ID is invalid.")
			}
			relatedID := uint(rawID)
			if _, exists := seen[relatedID]; exists {
				continue
			}
			seen[relatedID] = struct{}{}
			var count int64
			if err := tx.Model(&models.Product{}).Where("id = ?", relatedID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return apperror.New(apperror.KindInvalidInput, "invalid_related_product", "Related product does not exist.")
			}
			if err := tx.Create(&models.ProductRelatedDraft{ProductDraftID: draft.ID, RelatedProductID: relatedID, Position: position + 1}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return product, err
	}
	return s.GetProduct(ctx, id, true)
}
func (s *Service) AttachMedia(ctx context.Context, id uint, mediaIDs []string) (models.Product, error) {
	if len(mediaIDs) == 0 {
		return models.Product{}, invalidInput("media_ids_required", "Media IDs are required.")
	}
	if s.media == nil {
		return models.Product{}, errors.New("media service is unavailable")
	}

	normalized, err := s.validateProductMedia(ctx, mediaIDs)
	if err != nil {
		return models.Product{}, err
	}
	product, err := s.GetProduct(ctx, id, false)
	if err != nil {
		return models.Product{}, err
	}
	db := s.db.WithContext(ctx)
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := ensureProductDraft(tx, &product); err != nil {
			return err
		}
		var refs []models.MediaReference
		if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeProduct, id, media.RoleProductDraftImage).
			Order("position asc, created_at asc, id asc").Find(&refs).Error; err != nil {
			return err
		}
		seen := make(map[string]struct{}, len(refs)+len(normalized))
		for _, ref := range refs {
			seen[ref.MediaID] = struct{}{}
		}
		for _, mediaID := range normalized {
			if _, exists := seen[mediaID]; exists {
				continue
			}
			refs = append(refs, models.MediaReference{MediaID: mediaID, Position: len(refs)})
			seen[mediaID] = struct{}{}
		}
		return replaceProductMediaRole(tx, id, media.RoleProductDraftImage, refs)
	}); err != nil {
		return models.Product{}, err
	}
	return s.GetProduct(ctx, id, true)
}

func (s *Service) ReorderMedia(ctx context.Context, id uint, mediaIDs []string) (models.Product, error) {
	product, err := s.GetProduct(ctx, id, false)
	if err != nil {
		return models.Product{}, err
	}
	normalized, err := normalizedUniqueMediaIDs(mediaIDs)
	if err != nil {
		return models.Product{}, err
	}
	db := s.db.WithContext(ctx)
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := ensureProductDraft(tx, &product); err != nil {
			return err
		}
		var refs []models.MediaReference
		if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeProduct, id, media.RoleProductDraftImage).Find(&refs).Error; err != nil {
			return err
		}
		if len(refs) != len(normalized) {
			return invalidInput("invalid_media_order", "Media order must contain every attached media item exactly once.")
		}
		byID := make(map[string]models.MediaReference, len(refs))
		for _, ref := range refs {
			byID[ref.MediaID] = ref
		}
		ordered := make([]models.MediaReference, 0, len(normalized))
		for position, mediaID := range normalized {
			ref, exists := byID[mediaID]
			if !exists {
				return invalidInput("invalid_media_order", "Media order contains unattached media.")
			}
			ref.Position = position
			ordered = append(ordered, ref)
		}
		return replaceProductMediaRole(tx, id, media.RoleProductDraftImage, ordered)
	}); err != nil {
		return models.Product{}, err
	}
	return s.GetProduct(ctx, id, true)
}

func (s *Service) DetachMedia(ctx context.Context, id uint, mediaID string) (models.Product, []string, error) {
	mediaID = strings.TrimSpace(mediaID)
	if mediaID == "" {
		return models.Product{}, nil, invalidInput("invalid_media", "Media ID is required.")
	}
	product, err := s.GetProduct(ctx, id, false)
	if err != nil {
		return models.Product{}, nil, err
	}
	db := s.db.WithContext(ctx)
	removed := false
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := ensureProductDraft(tx, &product); err != nil {
			return err
		}
		var refs []models.MediaReference
		if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeProduct, id, media.RoleProductDraftImage).
			Order("position asc, created_at asc, id asc").Find(&refs).Error; err != nil {
			return err
		}
		kept := make([]models.MediaReference, 0, len(refs))
		for _, ref := range refs {
			if ref.MediaID == mediaID {
				removed = true
				continue
			}
			ref.Position = len(kept)
			kept = append(kept, ref)
		}
		if !removed {
			return apperror.New(apperror.KindNotFound, "product_media_not_found", "Product media attachment was not found.")
		}
		return replaceProductMediaRole(tx, id, media.RoleProductDraftImage, kept)
	}); err != nil {
		return models.Product{}, nil, err
	}
	value, err := s.GetProduct(ctx, id, true)
	if err != nil {
		return models.Product{}, nil, err
	}
	return value, []string{mediaID}, nil
}

func (s *Service) validateProductMedia(ctx context.Context, mediaIDs []string) ([]string, error) {
	normalized, err := normalizedUniqueMediaIDs(mediaIDs)
	if err != nil {
		return nil, err
	}
	for _, mediaID := range normalized {
		object, err := s.media.WaitUntilReady(ctx, mediaID, 2*time.Second)
		if err != nil {
			switch {
			case errors.Is(err, media.ErrMediaNotFound):
				return nil, apperror.Wrap(apperror.KindNotFound, "media_not_found", "Media not found: "+mediaID, err)
			case errors.Is(err, media.ErrMediaProcessingFailed):
				return nil, apperror.Wrap(apperror.KindValidation, "media_processing_failed", "Media processing failed: "+mediaID, err)
			case errors.Is(err, media.ErrMediaStillProcessing):
				return nil, apperror.Wrap(apperror.KindConflict, "media_still_processing", "Media is still processing: "+mediaID, err)
			default:
				return nil, err
			}
		}
		if !strings.HasPrefix(object.MimeType, "image/") {
			return nil, invalidInput("invalid_product_media", "Product media must be an image.")
		}
	}
	return normalized, nil
}

func normalizedUniqueMediaIDs(mediaIDs []string) ([]string, error) {
	normalized := make([]string, 0, len(mediaIDs))
	seen := make(map[string]struct{}, len(mediaIDs))
	for _, rawID := range mediaIDs {
		mediaID := strings.TrimSpace(rawID)
		if mediaID == "" {
			return nil, invalidInput("invalid_media", "Media ID is required.")
		}
		if _, exists := seen[mediaID]; exists {
			return nil, invalidInput("invalid_media", "Media IDs must be unique.")
		}
		seen[mediaID] = struct{}{}
		normalized = append(normalized, mediaID)
	}
	return normalized, nil
}

func ensureProductDraft(tx *gorm.DB, product *models.Product) error {
	if product.DraftUpdatedAt != nil {
		var count int64
		if err := tx.Model(&models.ProductDraft{}).Where("product_id = ?", product.ID).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return nil
		}
	}
	now := time.Now().UTC()
	if err := replaceDraft(tx, *product, productInputFromLive(*product)); err != nil {
		return err
	}
	if err := copyProductMediaRole(tx, product.ID, media.RoleProductImage, media.RoleProductDraftImage); err != nil {
		return err
	}
	if err := tx.Model(product).Update("draft_updated_at", now).Error; err != nil {
		return err
	}
	product.DraftUpdatedAt = &now
	return nil
}

func copyProductMediaRole(tx *gorm.DB, productID uint, fromRole, toRole string) error {
	var refs []models.MediaReference
	if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeProduct, productID, fromRole).
		Order("position asc, created_at asc, id asc").Find(&refs).Error; err != nil {
		return err
	}
	return replaceProductMediaRole(tx, productID, toRole, refs)
}

func replaceProductMediaRole(tx *gorm.DB, productID uint, role string, refs []models.MediaReference) error {
	if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeProduct, productID, role).Delete(&models.MediaReference{}).Error; err != nil {
		return err
	}
	for position, ref := range refs {
		if err := tx.Create(&models.MediaReference{MediaID: ref.MediaID, OwnerType: media.OwnerTypeProduct, OwnerID: productID, Role: role, Position: position}).Error; err != nil {
			return err
		}
	}
	return nil
}

func removedReferenceIDs(previous, current []models.MediaReference) []string {
	active := make(map[string]struct{}, len(current))
	for _, ref := range current {
		active[ref.MediaID] = struct{}{}
	}
	removed := make([]string, 0, len(previous))
	for _, ref := range previous {
		if _, exists := active[ref.MediaID]; !exists {
			removed = append(removed, ref.MediaID)
		}
	}
	return removed
}
