package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"ecommerce/internal/media"
	"ecommerce/models"

	"gorm.io/gorm"
)

const seoEntityTypeProduct = "product"

type productCatalogDraft struct {
	SKU               string
	Name              string
	Subtitle          *string
	Description       string
	Price             float64
	Stock             int
	Images            []string
	RelatedIDs        []uint
	BrandID           *uint
	Options           []productOptionDraftData
	Variants          []productVariantDraftData
	Attributes        []productAttributeValueDraftData
	SEO               productSEODraftData
	DefaultVariantSKU string
}

type productOptionDraftData struct {
	SourceID    *uint
	Name        string
	Position    int
	DisplayType string
	Deleted     bool
	Values      []productOptionValueDraftData
}

type productOptionValueDraftData struct {
	SourceID *uint
	Value    string
	Position int
	Deleted  bool
}

type productVariantDraftData struct {
	SourceID       *uint
	SKU            string
	Title          string
	Price          float64
	CompareAtPrice *float64
	Stock          int
	Position       int
	IsPublished    bool
	WeightGrams    *int
	LengthCm       *float64
	WidthCm        *float64
	HeightCm       *float64
	Deleted        bool
	Selections     []productVariantSelectionDraftData
}

type productVariantSelectionDraftData struct {
	SourceOptionValueID *uint
	OptionName          string
	OptionValue         string
	Position            int
}

type productAttributeValueDraftData struct {
	ProductAttributeID uint
	TextValue          *string
	NumberValue        *float64
	BooleanValue       *bool
	EnumValue          *string
	Position           int
	Deleted            bool
}

type productSEODraftData struct {
	Title          *string
	Description    *string
	CanonicalPath  *string
	OgImageMediaID *string
	NoIndex        bool
}

func normalizeCatalogMerchandisingFields(
	sku string,
	name string,
	description string,
	price float64,
	stock int,
	images []string,
	relatedIDs []uint,
) (string, string, string, float64, int, []string, []uint) {
	normalizedSKU := strings.TrimSpace(sku)
	normalizedName := strings.TrimSpace(name)
	normalizedDescription := strings.TrimSpace(description)
	normalizedPrice := price
	normalizedStock := stock
	normalizedImages := make([]string, 0, len(images))
	normalizedRelatedIDs := make([]uint, 0, len(relatedIDs))
	if normalizedStock < 0 {
		normalizedStock = 0
	}

	seenImages := make(map[string]struct{}, len(images))
	for _, image := range images {
		value := strings.TrimSpace(image)
		if value == "" {
			continue
		}
		if _, exists := seenImages[value]; exists {
			continue
		}
		seenImages[value] = struct{}{}
		normalizedImages = append(normalizedImages, value)
	}

	seenRelated := make(map[uint]struct{}, len(relatedIDs))
	for _, relatedID := range relatedIDs {
		if relatedID == 0 {
			continue
		}
		if _, exists := seenRelated[relatedID]; exists {
			continue
		}
		seenRelated[relatedID] = struct{}{}
		normalizedRelatedIDs = append(normalizedRelatedIDs, relatedID)
	}

	return normalizedSKU, normalizedName, normalizedDescription, normalizedPrice, normalizedStock, normalizedImages, normalizedRelatedIDs
}

func normalizeProductCatalogDraft(input productCatalogDraft) productCatalogDraft {
	normalizedSKU, normalizedName, normalizedDescription, normalizedPrice, normalizedStock, normalizedImages, normalizedRelatedIDs := normalizeCatalogMerchandisingFields(
		input.SKU,
		input.Name,
		input.Description,
		input.Price,
		input.Stock,
		input.Images,
		input.RelatedIDs,
	)

	normalized := productCatalogDraft{
		SKU:               normalizedSKU,
		Name:              normalizedName,
		Subtitle:          trimOptionalString(input.Subtitle),
		Description:       normalizedDescription,
		Price:             normalizedPrice,
		Stock:             normalizedStock,
		Images:            normalizedImages,
		RelatedIDs:        normalizedRelatedIDs,
		BrandID:           input.BrandID,
		Options:           make([]productOptionDraftData, 0, len(input.Options)),
		Variants:          make([]productVariantDraftData, 0, len(input.Variants)),
		Attributes:        make([]productAttributeValueDraftData, 0, len(input.Attributes)),
		SEO:               normalizeProductSEO(input.SEO),
		DefaultVariantSKU: strings.TrimSpace(input.DefaultVariantSKU),
	}

	for idx, option := range input.Options {
		normalized.Options = append(normalized.Options, normalizeOptionDraft(option, idx))
	}
	sort.SliceStable(normalized.Options, func(i, j int) bool {
		if normalized.Options[i].Position == normalized.Options[j].Position {
			return normalized.Options[i].Name < normalized.Options[j].Name
		}
		return normalized.Options[i].Position < normalized.Options[j].Position
	})

	for idx, variant := range input.Variants {
		normalized.Variants = append(normalized.Variants, normalizeVariantDraft(variant, idx))
	}
	sort.SliceStable(normalized.Variants, func(i, j int) bool {
		if normalized.Variants[i].Position == normalized.Variants[j].Position {
			return normalized.Variants[i].SKU < normalized.Variants[j].SKU
		}
		return normalized.Variants[i].Position < normalized.Variants[j].Position
	})

	for idx, attribute := range input.Attributes {
		value := attribute
		if value.Position <= 0 {
			value.Position = idx + 1
		}
		normalized.Attributes = append(normalized.Attributes, value)
	}
	sort.SliceStable(normalized.Attributes, func(i, j int) bool {
		if normalized.Attributes[i].Position == normalized.Attributes[j].Position {
			return normalized.Attributes[i].ProductAttributeID < normalized.Attributes[j].ProductAttributeID
		}
		return normalized.Attributes[i].Position < normalized.Attributes[j].Position
	})

	hasLiveDefaultVariant := false
	for _, variant := range normalized.Variants {
		if variant.Deleted {
			continue
		}
		if normalized.DefaultVariantSKU == "" && !hasLiveDefaultVariant {
			normalized.DefaultVariantSKU = variant.SKU
		}
		if variant.SKU == normalized.DefaultVariantSKU {
			hasLiveDefaultVariant = true
		}
	}
	if !hasLiveDefaultVariant {
		normalized.DefaultVariantSKU = ""
	}

	return normalized
}

func normalizeOptionDraft(input productOptionDraftData, idx int) productOptionDraftData {
	normalized := productOptionDraftData{
		SourceID:    input.SourceID,
		Name:        strings.TrimSpace(input.Name),
		Position:    input.Position,
		DisplayType: strings.TrimSpace(input.DisplayType),
		Deleted:     input.Deleted,
		Values:      make([]productOptionValueDraftData, 0, len(input.Values)),
	}
	if normalized.Position <= 0 {
		normalized.Position = idx + 1
	}
	if normalized.DisplayType == "" {
		normalized.DisplayType = "select"
	}
	for valueIdx, value := range input.Values {
		normalized.Values = append(normalized.Values, normalizeOptionValueDraft(value, valueIdx))
	}
	sort.SliceStable(normalized.Values, func(i, j int) bool {
		if normalized.Values[i].Position == normalized.Values[j].Position {
			return normalized.Values[i].Value < normalized.Values[j].Value
		}
		return normalized.Values[i].Position < normalized.Values[j].Position
	})
	return normalized
}

func normalizeOptionValueDraft(input productOptionValueDraftData, idx int) productOptionValueDraftData {
	normalized := input
	normalized.Value = strings.TrimSpace(input.Value)
	if normalized.Position <= 0 {
		normalized.Position = idx + 1
	}
	return normalized
}

func normalizeVariantDraft(input productVariantDraftData, idx int) productVariantDraftData {
	normalized := productVariantDraftData{
		SourceID:       input.SourceID,
		SKU:            strings.TrimSpace(input.SKU),
		Title:          strings.TrimSpace(input.Title),
		Price:          input.Price,
		CompareAtPrice: input.CompareAtPrice,
		Stock:          input.Stock,
		Position:       input.Position,
		IsPublished:    input.IsPublished,
		WeightGrams:    input.WeightGrams,
		LengthCm:       input.LengthCm,
		WidthCm:        input.WidthCm,
		HeightCm:       input.HeightCm,
		Deleted:        input.Deleted,
		Selections:     make([]productVariantSelectionDraftData, 0, len(input.Selections)),
	}
	if normalized.Stock < 0 {
		normalized.Stock = 0
	}
	if normalized.Position <= 0 {
		normalized.Position = idx + 1
	}
	for selIdx, selection := range input.Selections {
		next := selection
		next.OptionName = strings.TrimSpace(next.OptionName)
		next.OptionValue = strings.TrimSpace(next.OptionValue)
		if next.Position <= 0 {
			next.Position = selIdx + 1
		}
		normalized.Selections = append(normalized.Selections, next)
	}
	sort.SliceStable(normalized.Selections, func(i, j int) bool {
		if normalized.Selections[i].Position == normalized.Selections[j].Position {
			left := normalized.Selections[i].OptionName + ":" + normalized.Selections[i].OptionValue
			right := normalized.Selections[j].OptionName + ":" + normalized.Selections[j].OptionValue
			return left < right
		}
		return normalized.Selections[i].Position < normalized.Selections[j].Position
	})
	return normalized
}

func normalizeProductSEO(input productSEODraftData) productSEODraftData {
	return productSEODraftData{
		Title:          trimOptionalString(input.Title),
		Description:    trimOptionalString(input.Description),
		CanonicalPath:  normalizeCanonicalPath(trimOptionalString(input.CanonicalPath)),
		OgImageMediaID: trimOptionalString(input.OgImageMediaID),
		NoIndex:        input.NoIndex,
	}
}

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func productHasDraft(product models.Product) bool {
	return product.DraftUpdatedAt != nil
}

func productIsPubliclyVisible(product models.Product) bool {
	return product.IsPublished
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

func saveEditableProductCatalogDraft(tx *gorm.DB, product *models.Product, draft productCatalogDraft) error {
	baseline, _, err := loadNormalizedProductDraft(tx, *product)
	if err != nil {
		return err
	}

	merged := hydrateDraftSourceIDs(draft, baseline)
	normalized, err := deriveCatalogMerchandising(merged)
	if err != nil {
		return err
	}

	now := time.Now()
	if err := saveNormalizedProductDraft(tx, *product, normalized, now); err != nil {
		return err
	}
	product.DraftUpdatedAt = &now
	return nil
}

func ensureProductCatalogDraft(tx *gorm.DB, product *models.Product) (bool, error) {
	if productHasDraft(*product) {
		return true, nil
	}

	live, err := loadPublishedProductCatalogData(tx, *product, false)
	if err != nil {
		return false, err
	}

	now := time.Now()
	if err := saveNormalizedProductDraft(tx, *product, live, now); err != nil {
		return false, err
	}
	if err := copyProductMediaRole(tx, product.ID, media.RoleProductImage, media.RoleProductDraftImage); err != nil {
		return false, err
	}
	product.DraftUpdatedAt = &now
	return false, nil
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

func loadNormalizedProductDraft(tx *gorm.DB, product models.Product) (productCatalogDraft, bool, error) {
	var record models.ProductDraft
	err := tx.
		Preload("OptionDrafts", func(db *gorm.DB) *gorm.DB { return db.Order("position asc").Order("id asc") }).
		Preload("OptionDrafts.ValueDrafts", func(db *gorm.DB) *gorm.DB { return db.Order("position asc").Order("id asc") }).
		Preload("VariantDrafts", func(db *gorm.DB) *gorm.DB { return db.Order("position asc").Order("id asc") }).
		Preload("VariantDrafts.OptionValueDraftLinks", func(db *gorm.DB) *gorm.DB { return db.Order("position asc").Order("id asc") }).
		Preload("AttributeDrafts", func(db *gorm.DB) *gorm.DB { return db.Order("position asc").Order("id asc") }).
		Preload("RelatedDrafts", func(db *gorm.DB) *gorm.DB { return db.Order("position asc").Order("id asc") }).
		Where("product_id = ?", product.ID).
		First(&record).Error
	if err == nil {
		return normalizedDraftFromRecord(record), true, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return productCatalogDraft{}, false, err
	}

	live, err := loadPublishedProductCatalogData(tx, product, false)
	if err != nil {
		return productCatalogDraft{}, false, err
	}
	return normalizeProductCatalogDraft(live), false, nil
}

func saveNormalizedProductDraft(tx *gorm.DB, product models.Product, draft productCatalogDraft, now time.Time) error {
	normalized := normalizeProductCatalogDraft(draft)
	imagesJSON, err := encodeStringSlice(normalized.Images)
	if err != nil {
		return err
	}

	var record models.ProductDraft
	err = tx.Where("product_id = ?", product.ID).First(&record).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	updates := map[string]any{
		"product_id":            product.ID,
		"sku":                   normalized.SKU,
		"default_variant_sku":   normalized.DefaultVariantSKU,
		"name":                  normalized.Name,
		"subtitle":              normalized.Subtitle,
		"description":           normalized.Description,
		"price":                 models.MoneyFromFloat(normalized.Price),
		"stock":                 normalized.Stock,
		"images_json":           imagesJSON,
		"brand_id":              normalized.BrandID,
		"seo_title":             normalized.SEO.Title,
		"seo_description":       normalized.SEO.Description,
		"seo_canonical_path":    normalized.SEO.CanonicalPath,
		"seo_og_image_media_id": normalized.SEO.OgImageMediaID,
		"seo_no_index":          normalized.SEO.NoIndex,
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		record = models.ProductDraft{
			ProductID:         product.ID,
			Version:           1,
			SKU:               normalized.SKU,
			DefaultVariantSKU: normalized.DefaultVariantSKU,
			Name:              normalized.Name,
			Subtitle:          normalized.Subtitle,
			Description:       normalized.Description,
			Price:             models.MoneyFromFloat(normalized.Price),
			Stock:             normalized.Stock,
			ImagesJSON:        imagesJSON,
			BrandID:           normalized.BrandID,
			SeoTitle:          normalized.SEO.Title,
			SeoDescription:    normalized.SEO.Description,
			SeoCanonicalPath:  normalized.SEO.CanonicalPath,
			SeoOgImageMediaID: normalized.SEO.OgImageMediaID,
			SeoNoIndex:        normalized.SEO.NoIndex,
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
	} else {
		updates["version"] = record.Version + 1
		if err := tx.Model(&record).Updates(updates).Error; err != nil {
			return err
		}
	}
	if err := replaceProductDraftChildren(tx, record.ID, normalized); err != nil {
		return err
	}
	return tx.Model(&models.Product{}).Where("id = ?", product.ID).Updates(map[string]any{
		"draft_updated_at": now,
	}).Error
}

func replaceProductDraftChildren(tx *gorm.DB, productDraftID uint, draft productCatalogDraft) error {
	if err := deleteProductDraftChildren(tx, productDraftID); err != nil {
		return err
	}

	optionValueDraftIDByKey := make(map[string]uint)
	for _, option := range draft.Options {
		record := models.ProductOptionDraft{
			ProductDraftID:        productDraftID,
			SourceProductOptionID: option.SourceID,
			Name:                  option.Name,
			Position:              option.Position,
			DisplayType:           option.DisplayType,
			IsDeleted:             option.Deleted,
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		for _, value := range option.Values {
			valueRecord := models.ProductOptionValueDraft{
				ProductOptionDraftID:       record.ID,
				SourceProductOptionValueID: value.SourceID,
				Value:                      value.Value,
				Position:                   value.Position,
				IsDeleted:                  value.Deleted,
			}
			if err := tx.Create(&valueRecord).Error; err != nil {
				return err
			}
			optionValueDraftIDByKey[draftOptionValueKey(option.Name, value.Value)] = valueRecord.ID
			if value.SourceID != nil {
				optionValueDraftIDByKey[draftOptionSourceKey(*value.SourceID)] = valueRecord.ID
			}
		}
	}

	for _, variant := range draft.Variants {
		record := models.ProductVariantDraft{
			ProductDraftID:         productDraftID,
			SourceProductVariantID: variant.SourceID,
			SKU:                    variant.SKU,
			Title:                  variant.Title,
			Price:                  models.MoneyFromFloat(variant.Price),
			CompareAtPrice:         moneyPtrFromFloatPtr(variant.CompareAtPrice),
			Stock:                  variant.Stock,
			Position:               variant.Position,
			IsPublished:            variant.IsPublished,
			WeightGrams:            variant.WeightGrams,
			LengthCm:               variant.LengthCm,
			WidthCm:                variant.WidthCm,
			HeightCm:               variant.HeightCm,
			IsDeleted:              variant.Deleted,
		}
		if err := tx.Select("*").Create(&record).Error; err != nil {
			return err
		}
		for _, selection := range variant.Selections {
			link := models.ProductVariantOptionValueDraft{
				ProductVariantDraftID:      record.ID,
				SourceProductOptionValueID: selection.SourceOptionValueID,
				OptionName:                 selection.OptionName,
				OptionValue:                selection.OptionValue,
				Position:                   selection.Position,
			}
			if selection.SourceOptionValueID != nil {
				if draftID, ok := optionValueDraftIDByKey[draftOptionSourceKey(*selection.SourceOptionValueID)]; ok {
					link.ProductOptionValueDraftID = &draftID
				}
			}
			if link.ProductOptionValueDraftID == nil {
				if draftID, ok := optionValueDraftIDByKey[draftOptionValueKey(selection.OptionName, selection.OptionValue)]; ok {
					link.ProductOptionValueDraftID = &draftID
				}
			}
			if err := tx.Create(&link).Error; err != nil {
				return err
			}
		}
	}

	for _, attribute := range draft.Attributes {
		record := models.ProductAttributeValueDraft{
			ProductDraftID:     productDraftID,
			ProductAttributeID: attribute.ProductAttributeID,
			TextValue:          attribute.TextValue,
			NumberValue:        attribute.NumberValue,
			BooleanValue:       attribute.BooleanValue,
			EnumValue:          attribute.EnumValue,
			Position:           attribute.Position,
			IsDeleted:          attribute.Deleted,
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
	}

	for idx, relatedID := range draft.RelatedIDs {
		record := models.ProductRelatedDraft{
			ProductDraftID:   productDraftID,
			RelatedProductID: relatedID,
			Position:         idx + 1,
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
	}

	return nil
}

func deleteProductDraftChildren(tx *gorm.DB, productDraftID uint) error {
	if err := tx.Unscoped().Where("product_draft_id = ?", productDraftID).Delete(&models.ProductRelatedDraft{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("product_draft_id = ?", productDraftID).Delete(&models.ProductAttributeValueDraft{}).Error; err != nil {
		return err
	}

	var variantDraftIDs []uint
	if err := tx.Model(&models.ProductVariantDraft{}).Where("product_draft_id = ?", productDraftID).Pluck("id", &variantDraftIDs).Error; err != nil {
		return err
	}
	if len(variantDraftIDs) > 0 {
		if err := tx.Unscoped().Where("product_variant_draft_id IN ?", variantDraftIDs).Delete(&models.ProductVariantOptionValueDraft{}).Error; err != nil {
			return err
		}
	}
	if err := tx.Unscoped().Where("product_draft_id = ?", productDraftID).Delete(&models.ProductVariantDraft{}).Error; err != nil {
		return err
	}

	var optionDraftIDs []uint
	if err := tx.Model(&models.ProductOptionDraft{}).Where("product_draft_id = ?", productDraftID).Pluck("id", &optionDraftIDs).Error; err != nil {
		return err
	}
	if len(optionDraftIDs) > 0 {
		if err := tx.Unscoped().Where("product_option_draft_id IN ?", optionDraftIDs).Delete(&models.ProductOptionValueDraft{}).Error; err != nil {
			return err
		}
	}
	return tx.Unscoped().Where("product_draft_id = ?", productDraftID).Delete(&models.ProductOptionDraft{}).Error
}

func deleteNormalizedProductDraft(tx *gorm.DB, productID uint) error {
	var record models.ProductDraft
	if err := tx.Where("product_id = ?", productID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if err := deleteProductDraftChildren(tx, record.ID); err != nil {
		return err
	}
	return tx.Unscoped().Delete(&record).Error
}

func normalizedDraftFromRecord(record models.ProductDraft) productCatalogDraft {
	images, _ := decodeStringSlice(record.ImagesJSON)
	result := productCatalogDraft{
		SKU:               record.SKU,
		DefaultVariantSKU: record.DefaultVariantSKU,
		Name:              record.Name,
		Subtitle:          record.Subtitle,
		Description:       record.Description,
		Price:             record.Price.Float64(),
		Stock:             record.Stock,
		Images:            images,
		BrandID:           record.BrandID,
		Options:           make([]productOptionDraftData, 0, len(record.OptionDrafts)),
		Variants:          make([]productVariantDraftData, 0, len(record.VariantDrafts)),
		Attributes:        make([]productAttributeValueDraftData, 0, len(record.AttributeDrafts)),
		RelatedIDs:        make([]uint, 0, len(record.RelatedDrafts)),
		SEO: productSEODraftData{
			Title:          record.SeoTitle,
			Description:    record.SeoDescription,
			CanonicalPath:  record.SeoCanonicalPath,
			OgImageMediaID: record.SeoOgImageMediaID,
			NoIndex:        record.SeoNoIndex,
		},
	}

	for _, option := range record.OptionDrafts {
		entry := productOptionDraftData{
			SourceID:    option.SourceProductOptionID,
			Name:        option.Name,
			Position:    option.Position,
			DisplayType: option.DisplayType,
			Deleted:     option.IsDeleted,
			Values:      make([]productOptionValueDraftData, 0, len(option.ValueDrafts)),
		}
		for _, value := range option.ValueDrafts {
			entry.Values = append(entry.Values, productOptionValueDraftData{
				SourceID: value.SourceProductOptionValueID,
				Value:    value.Value,
				Position: value.Position,
				Deleted:  value.IsDeleted,
			})
		}
		result.Options = append(result.Options, entry)
	}

	for _, variant := range record.VariantDrafts {
		entry := productVariantDraftData{
			SourceID:       variant.SourceProductVariantID,
			SKU:            variant.SKU,
			Title:          variant.Title,
			Price:          variant.Price.Float64(),
			CompareAtPrice: floatPtrFromMoneyPtr(variant.CompareAtPrice),
			Stock:          variant.Stock,
			Position:       variant.Position,
			IsPublished:    variant.IsPublished,
			WeightGrams:    variant.WeightGrams,
			LengthCm:       variant.LengthCm,
			WidthCm:        variant.WidthCm,
			HeightCm:       variant.HeightCm,
			Deleted:        variant.IsDeleted,
			Selections:     make([]productVariantSelectionDraftData, 0, len(variant.OptionValueDraftLinks)),
		}
		for _, link := range variant.OptionValueDraftLinks {
			entry.Selections = append(entry.Selections, productVariantSelectionDraftData{
				SourceOptionValueID: link.SourceProductOptionValueID,
				OptionName:          link.OptionName,
				OptionValue:         link.OptionValue,
				Position:            link.Position,
			})
		}
		result.Variants = append(result.Variants, entry)
	}

	for _, attribute := range record.AttributeDrafts {
		result.Attributes = append(result.Attributes, productAttributeValueDraftData{
			ProductAttributeID: attribute.ProductAttributeID,
			TextValue:          attribute.TextValue,
			NumberValue:        attribute.NumberValue,
			BooleanValue:       attribute.BooleanValue,
			EnumValue:          attribute.EnumValue,
			Position:           attribute.Position,
			Deleted:            attribute.IsDeleted,
		})
	}

	for _, related := range record.RelatedDrafts {
		result.RelatedIDs = append(result.RelatedIDs, related.RelatedProductID)
	}

	return normalizeProductCatalogDraft(result)
}

func loadPublishedProductCatalogData(tx *gorm.DB, product models.Product, publicOnly bool) (productCatalogDraft, error) {
	result := productCatalogDraft{
		SKU:         product.SKU,
		Name:        product.Name,
		Subtitle:    product.Subtitle,
		Description: product.Description,
		Price:       product.Price.Float64(),
		Stock:       product.Stock,
		Images:      append([]string(nil), product.Images...),
		RelatedIDs:  productRelatedIDs(product),
		BrandID:     product.BrandID,
	}

	var options []models.ProductOption
	if err := tx.Where("product_id = ?", product.ID).Order("position asc").Order("id asc").Find(&options).Error; err != nil {
		return productCatalogDraft{}, err
	}
	optionIDs := make([]uint, 0, len(options))
	for _, option := range options {
		optionIDs = append(optionIDs, option.ID)
	}
	valuesByOptionID := make(map[uint][]models.ProductOptionValue)
	if len(optionIDs) > 0 {
		var values []models.ProductOptionValue
		if err := tx.Where("product_option_id IN ?", optionIDs).Order("position asc").Order("id asc").Find(&values).Error; err != nil {
			return productCatalogDraft{}, err
		}
		for _, value := range values {
			valuesByOptionID[value.ProductOptionID] = append(valuesByOptionID[value.ProductOptionID], value)
		}
	}
	var variants []models.ProductVariant
	variantQuery := tx.Where("product_id = ?", product.ID)
	var totalVariantCount int64
	if err := tx.Model(&models.ProductVariant{}).Where("product_id = ?", product.ID).Count(&totalVariantCount).Error; err != nil {
		return productCatalogDraft{}, err
	}
	if publicOnly {
		variantQuery = variantQuery.Where("is_published = ?", true)
	}
	if err := variantQuery.Order("position asc").Order("id asc").Find(&variants).Error; err != nil {
		return productCatalogDraft{}, err
	}
	variantIDs := make([]uint, 0, len(variants))
	for _, variant := range variants {
		variantIDs = append(variantIDs, variant.ID)
	}
	linksByVariantID := make(map[uint][]models.ProductVariantOptionValue)
	var publishedValueIDs map[uint]struct{}
	if len(variantIDs) > 0 {
		var links []models.ProductVariantOptionValue
		if err := tx.Where("product_variant_id IN ?", variantIDs).Order("id asc").Find(&links).Error; err != nil {
			return productCatalogDraft{}, err
		}
		if publicOnly {
			publishedValueIDs = make(map[uint]struct{}, len(links))
		}
		for _, link := range links {
			linksByVariantID[link.ProductVariantID] = append(linksByVariantID[link.ProductVariantID], link)
			if publicOnly {
				publishedValueIDs[link.ProductOptionValueID] = struct{}{}
			}
		}
	}
	for _, option := range options {
		entry := productOptionDraftData{
			SourceID:    &option.ID,
			Name:        option.Name,
			Position:    option.Position,
			DisplayType: option.DisplayType,
		}
		for _, value := range valuesByOptionID[option.ID] {
			if publicOnly {
				if _, ok := publishedValueIDs[value.ID]; !ok {
					continue
				}
			}
			valueCopy := value.ID
			entry.Values = append(entry.Values, productOptionValueDraftData{
				SourceID: &valueCopy,
				Value:    value.Value,
				Position: value.Position,
			})
		}
		if publicOnly && len(entry.Values) == 0 {
			continue
		}
		result.Options = append(result.Options, entry)
	}
	valueMetaByID := make(map[uint]models.ProductOptionValue)
	if len(optionIDs) > 0 {
		var values []models.ProductOptionValue
		if err := tx.Where("product_option_id IN ?", optionIDs).Find(&values).Error; err != nil {
			return productCatalogDraft{}, err
		}
		for _, value := range values {
			valueMetaByID[value.ID] = value
		}
	}
	optionMetaByID := make(map[uint]models.ProductOption)
	for _, option := range options {
		optionMetaByID[option.ID] = option
	}

	for _, variant := range variants {
		variantID := variant.ID
		entry := productVariantDraftData{
			SourceID:       &variantID,
			SKU:            variant.SKU,
			Title:          variant.Title,
			Price:          variant.Price.Float64(),
			CompareAtPrice: floatPtrFromMoneyPtr(variant.CompareAtPrice),
			Stock:          variant.Stock,
			Position:       variant.Position,
			IsPublished:    variant.IsPublished,
			WeightGrams:    variant.WeightGrams,
			LengthCm:       variant.LengthCm,
			WidthCm:        variant.WidthCm,
			HeightCm:       variant.HeightCm,
		}
		for idx, link := range linksByVariantID[variant.ID] {
			value := valueMetaByID[link.ProductOptionValueID]
			option := optionMetaByID[value.ProductOptionID]
			valueID := value.ID
			entry.Selections = append(entry.Selections, productVariantSelectionDraftData{
				SourceOptionValueID: &valueID,
				OptionName:          option.Name,
				OptionValue:         value.Value,
				Position:            idx + 1,
			})
		}
		result.Variants = append(result.Variants, entry)
	}

	if len(result.Variants) == 0 && (!publicOnly || totalVariantCount == 0) {
		result.Variants = append(result.Variants, productVariantDraftData{
			SKU:         product.SKU,
			Title:       product.Name,
			Price:       product.Price.Float64(),
			Stock:       product.Stock,
			Position:    1,
			IsPublished: product.IsPublished,
		})
		if result.DefaultVariantSKU == "" {
			result.DefaultVariantSKU = product.SKU
		}
	}

	var attributes []models.ProductAttributeValue
	if err := tx.Where("product_id = ?", product.ID).Order("position asc").Order("id asc").Find(&attributes).Error; err != nil {
		return productCatalogDraft{}, err
	}
	for _, attribute := range attributes {
		result.Attributes = append(result.Attributes, productAttributeValueDraftData{
			ProductAttributeID: attribute.ProductAttributeID,
			TextValue:          attribute.TextValue,
			NumberValue:        attribute.NumberValue,
			BooleanValue:       attribute.BooleanValue,
			EnumValue:          attribute.EnumValue,
			Position:           attribute.Position,
		})
	}

	var seo models.SEOMetadata
	if err := tx.Where("entity_type = ? AND entity_id = ?", seoEntityTypeProduct, product.ID).First(&seo).Error; err == nil {
		result.SEO = productSEODraftData{
			Title:          seo.Title,
			Description:    seo.Description,
			CanonicalPath:  seo.CanonicalPath,
			OgImageMediaID: seo.OgImageMediaID,
			NoIndex:        seo.NoIndex,
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return productCatalogDraft{}, err
	}

	if product.DefaultVariantID != nil {
		var variant models.ProductVariant
		defaultQuery := tx
		if publicOnly {
			defaultQuery = defaultQuery.Where("is_published = ?", true)
		}
		if err := defaultQuery.First(&variant, *product.DefaultVariantID).Error; err == nil {
			result.DefaultVariantSKU = variant.SKU
		}
	}

	return normalizeProductCatalogDraft(result), nil
}

func publishNormalizedProductDraft(tx *gorm.DB, product *models.Product, draft productCatalogDraft) error {
	normalized := normalizeProductCatalogDraft(draft)
	if product.IsPublished {
		live, err := loadPublishedProductCatalogData(tx, *product, false)
		if err != nil {
			return err
		}
		normalized = hydrateDraftSourceIDs(normalized, live)
	}

	var related []models.Product
	if len(normalized.RelatedIDs) > 0 {
		if err := tx.Where("id IN ?", normalized.RelatedIDs).Find(&related).Error; err != nil {
			return err
		}
	}

	updates := map[string]any{
		"sku":                normalized.SKU,
		"name":               normalized.Name,
		"subtitle":           normalized.Subtitle,
		"description":        normalized.Description,
		"price":              models.MoneyFromFloat(normalized.Price),
		"stock":              normalized.Stock,
		"images":             normalized.Images,
		"brand_id":           normalized.BrandID,
		"is_published":       true,
		"draft_updated_at":   nil,
		"default_variant_id": nil,
	}
	if err := tx.Model(product).Updates(updates).Error; err != nil {
		return err
	}
	if err := tx.Model(product).Association("Related").Replace(related); err != nil {
		return err
	}
	if err := replacePublishedCatalogChildren(tx, product.ID, normalized); err != nil {
		return err
	}
	if err := deleteNormalizedProductDraft(tx, product.ID); err != nil {
		return err
	}
	product.DraftUpdatedAt = nil
	product.IsPublished = true
	return nil
}

func replacePublishedCatalogChildren(tx *gorm.DB, productID uint, draft productCatalogDraft) error {
	if err := upsertPublishedSEO(tx, productID, draft.SEO); err != nil {
		return err
	}

	existingVariantIDs, err := publishedVariantIDs(tx, productID)
	if err != nil {
		return err
	}
	if err := deletePublishedVariantOptionLinks(tx, existingVariantIDs); err != nil {
		return err
	}
	if err := deletePublishedOptions(tx, productID); err != nil {
		return err
	}

	liveOptionValueIDByKey := make(map[string]uint)
	for _, option := range draft.Options {
		if option.Deleted {
			continue
		}
		record := models.ProductOption{
			ProductID:   productID,
			Name:        option.Name,
			Position:    option.Position,
			DisplayType: option.DisplayType,
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		for _, value := range option.Values {
			if value.Deleted {
				continue
			}
			valueRecord := models.ProductOptionValue{
				ProductOptionID: record.ID,
				Value:           value.Value,
				Position:        value.Position,
			}
			if err := tx.Create(&valueRecord).Error; err != nil {
				return err
			}
			liveOptionValueIDByKey[draftOptionValueKey(option.Name, value.Value)] = valueRecord.ID
			if value.SourceID != nil {
				liveOptionValueIDByKey[draftOptionSourceKey(*value.SourceID)] = valueRecord.ID
			}
		}
	}

	var defaultVariantID *uint
	keptVariantIDs := make(map[uint]struct{}, len(draft.Variants))
	for _, variant := range draft.Variants {
		if variant.Deleted {
			continue
		}
		record := models.ProductVariant{
			ProductID:      productID,
			SKU:            variant.SKU,
			Title:          variant.Title,
			Price:          models.MoneyFromFloat(variant.Price),
			CompareAtPrice: moneyPtrFromFloatPtr(variant.CompareAtPrice),
			Stock:          variant.Stock,
			Position:       variant.Position,
			IsPublished:    variant.IsPublished,
			WeightGrams:    variant.WeightGrams,
			LengthCm:       variant.LengthCm,
			WidthCm:        variant.WidthCm,
			HeightCm:       variant.HeightCm,
		}
		if variant.SourceID != nil && containsUint(existingVariantIDs, *variant.SourceID) {
			record.ID = *variant.SourceID
			if err := tx.Model(&models.ProductVariant{}).
				Where("id = ? AND product_id = ?", record.ID, productID).
				Select(
					"sku",
					"title",
					"price",
					"compare_at_price",
					"stock",
					"position",
					"is_published",
					"weight_grams",
					"length_cm",
					"width_cm",
					"height_cm",
				).
				Updates(&record).Error; err != nil {
				return err
			}
		} else {
			record.ID = 0
			if err := tx.Select("*").Create(&record).Error; err != nil {
				return err
			}
		}
		keptVariantIDs[record.ID] = struct{}{}
		if record.SKU == draft.DefaultVariantSKU {
			id := record.ID
			defaultVariantID = &id
		} else if defaultVariantID == nil {
			id := record.ID
			defaultVariantID = &id
		}
		for _, selection := range variant.Selections {
			liveValueID := uint(0)
			if selection.SourceOptionValueID != nil {
				liveValueID = liveOptionValueIDByKey[draftOptionSourceKey(*selection.SourceOptionValueID)]
			}
			if liveValueID == 0 {
				liveValueID = liveOptionValueIDByKey[draftOptionValueKey(selection.OptionName, selection.OptionValue)]
			}
			if liveValueID == 0 {
				return fmt.Errorf("variant %q references unknown option value %q=%q", variant.SKU, selection.OptionName, selection.OptionValue)
			}
			link := models.ProductVariantOptionValue{
				ProductVariantID:     record.ID,
				ProductOptionValueID: liveValueID,
			}
			if err := tx.Create(&link).Error; err != nil {
				return err
			}
		}
	}
	if err := deletePublishedStaleVariants(tx, existingVariantIDs, keptVariantIDs); err != nil {
		return err
	}

	if err := deletePublishedAttributes(tx, productID); err != nil {
		return err
	}
	for _, attribute := range draft.Attributes {
		if attribute.Deleted {
			continue
		}
		record := models.ProductAttributeValue{
			ProductID:          productID,
			ProductAttributeID: attribute.ProductAttributeID,
			TextValue:          attribute.TextValue,
			NumberValue:        attribute.NumberValue,
			BooleanValue:       attribute.BooleanValue,
			EnumValue:          attribute.EnumValue,
			Position:           attribute.Position,
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
	}

	if defaultVariantID != nil {
		if err := tx.Model(&models.Product{}).Where("id = ?", productID).Update("default_variant_id", *defaultVariantID).Error; err != nil {
			return err
		}
	}

	return nil
}

func hydrateDraftSourceIDs(draft productCatalogDraft, baseline productCatalogDraft) productCatalogDraft {
	normalized := normalizeProductCatalogDraft(draft)
	baseline = normalizeProductCatalogDraft(baseline)

	normalized.Options = hydrateOptionSourceIDs(normalized.Options, baseline.Options)
	normalized.Variants = hydrateVariantSourceIDs(normalized.Variants, baseline.Variants, normalized.Options, baseline.Options)

	return normalizeProductCatalogDraft(normalized)
}

func hydrateOptionSourceIDs(options []productOptionDraftData, baseline []productOptionDraftData) []productOptionDraftData {
	activeBaseline := make([]productOptionDraftData, 0, len(baseline))
	usedOptions := make(map[uint]struct{}, len(baseline))
	for _, option := range baseline {
		if option.Deleted {
			continue
		}
		activeBaseline = append(activeBaseline, option)
	}

	result := make([]productOptionDraftData, 0, len(options))
	activeIndex := 0
	for _, option := range options {
		next := option
		var matched *productOptionDraftData
		if next.SourceID != nil {
			for i := range activeBaseline {
				if activeBaseline[i].SourceID != nil && *activeBaseline[i].SourceID == *next.SourceID {
					matched = &activeBaseline[i]
					break
				}
			}
		}
		if matched == nil {
			matched = matchOptionByName(next.Name, activeBaseline, usedOptions)
		}
		if matched == nil {
			matched = matchOptionByPosition(next.Position, activeBaseline, usedOptions)
		}
		if matched == nil {
			matched = matchOptionByActiveIndex(activeIndex, activeBaseline, usedOptions)
		}
		if matched != nil && matched.SourceID != nil {
			next.SourceID = matched.SourceID
			usedOptions[*matched.SourceID] = struct{}{}
			next.Values = hydrateOptionValueSourceIDs(next.Values, matched.Values)
		}
		result = append(result, next)
		if !next.Deleted {
			activeIndex++
		}
	}
	return result
}

func hydrateOptionValueSourceIDs(values []productOptionValueDraftData, baseline []productOptionValueDraftData) []productOptionValueDraftData {
	activeBaseline := make([]productOptionValueDraftData, 0, len(baseline))
	usedValues := make(map[uint]struct{}, len(baseline))
	for _, value := range baseline {
		if value.Deleted {
			continue
		}
		activeBaseline = append(activeBaseline, value)
	}

	result := make([]productOptionValueDraftData, 0, len(values))
	activeIndex := 0
	for _, value := range values {
		next := value
		var matched *productOptionValueDraftData
		if next.SourceID != nil {
			for i := range activeBaseline {
				if activeBaseline[i].SourceID != nil && *activeBaseline[i].SourceID == *next.SourceID {
					matched = &activeBaseline[i]
					break
				}
			}
		}
		if matched == nil {
			matched = matchOptionValueByLabel(next.Value, activeBaseline, usedValues)
		}
		if matched == nil {
			matched = matchOptionValueByPosition(next.Position, activeBaseline, usedValues)
		}
		if matched == nil {
			matched = matchOptionValueByActiveIndex(activeIndex, activeBaseline, usedValues)
		}
		if matched != nil && matched.SourceID != nil {
			next.SourceID = matched.SourceID
			usedValues[*matched.SourceID] = struct{}{}
		}
		result = append(result, next)
		if !next.Deleted {
			activeIndex++
		}
	}
	return result
}

func hydrateVariantSourceIDs(
	variants []productVariantDraftData,
	baselineVariants []productVariantDraftData,
	options []productOptionDraftData,
	baselineOptions []productOptionDraftData,
) []productVariantDraftData {
	activeBaseline := make([]productVariantDraftData, 0, len(baselineVariants))
	usedVariants := make(map[uint]struct{}, len(baselineVariants))
	for _, variant := range baselineVariants {
		if variant.Deleted {
			continue
		}
		activeBaseline = append(activeBaseline, variant)
	}

	baselineSelectionKeys := make(map[uint]string, len(activeBaseline))
	for _, variant := range activeBaseline {
		if variant.SourceID == nil {
			continue
		}
		if key := variantSelectionSourceKey(variant); key != "" {
			baselineSelectionKeys[*variant.SourceID] = key
		}
	}

	result := make([]productVariantDraftData, 0, len(variants))
	activeIndex := 0
	for _, variant := range variants {
		next := variant
		next.Selections = hydrateVariantSelectionSourceIDs(next.Selections, options)

		var matched *productVariantDraftData
		if next.SourceID != nil {
			for i := range activeBaseline {
				if activeBaseline[i].SourceID != nil && *activeBaseline[i].SourceID == *next.SourceID {
					matched = &activeBaseline[i]
					break
				}
			}
		}
		if matched == nil {
			if key := variantSelectionSourceKey(next); key != "" {
				matched = matchVariantBySelectionKey(key, activeBaseline, baselineSelectionKeys, usedVariants)
			}
		}
		if matched == nil {
			matched = matchVariantBySKU(next.SKU, activeBaseline, usedVariants)
		}
		if matched == nil && len(options) == 0 && len(activeBaseline) == 1 {
			matched = matchVariantByActiveIndex(0, activeBaseline, usedVariants)
		}
		if matched == nil {
			matched = matchVariantByPosition(next.Position, activeBaseline, usedVariants)
		}
		if matched == nil {
			matched = matchVariantByActiveIndex(activeIndex, activeBaseline, usedVariants)
		}
		if matched != nil && matched.SourceID != nil {
			next.SourceID = matched.SourceID
			usedVariants[*matched.SourceID] = struct{}{}
		}
		result = append(result, next)
		if !next.Deleted {
			activeIndex++
		}
	}
	return result
}

func hydrateVariantSelectionSourceIDs(
	selections []productVariantSelectionDraftData,
	options []productOptionDraftData,
) []productVariantSelectionDraftData {
	sourceByKey := make(map[string]uint)
	for _, option := range options {
		for _, value := range option.Values {
			if value.SourceID == nil {
				continue
			}
			sourceByKey[draftOptionValueKey(option.Name, value.Value)] = *value.SourceID
		}
	}

	result := make([]productVariantSelectionDraftData, 0, len(selections))
	for _, selection := range selections {
		next := selection
		if next.SourceOptionValueID == nil {
			if sourceID, ok := sourceByKey[draftOptionValueKey(next.OptionName, next.OptionValue)]; ok {
				next.SourceOptionValueID = &sourceID
			}
		}
		result = append(result, next)
	}
	return result
}

func matchOptionByName(name string, baseline []productOptionDraftData, used map[uint]struct{}) *productOptionDraftData {
	key := strings.ToLower(strings.TrimSpace(name))
	for i := range baseline {
		if baseline[i].SourceID == nil {
			continue
		}
		if _, exists := used[*baseline[i].SourceID]; exists {
			continue
		}
		if strings.ToLower(strings.TrimSpace(baseline[i].Name)) == key {
			return &baseline[i]
		}
	}
	return nil
}

func matchOptionByPosition(position int, baseline []productOptionDraftData, used map[uint]struct{}) *productOptionDraftData {
	for i := range baseline {
		if baseline[i].SourceID == nil {
			continue
		}
		if _, exists := used[*baseline[i].SourceID]; exists {
			continue
		}
		if baseline[i].Position == position {
			return &baseline[i]
		}
	}
	return nil
}

func matchOptionByActiveIndex(index int, baseline []productOptionDraftData, used map[uint]struct{}) *productOptionDraftData {
	active := 0
	for i := range baseline {
		if baseline[i].SourceID == nil {
			continue
		}
		if _, exists := used[*baseline[i].SourceID]; exists {
			continue
		}
		if active == index {
			return &baseline[i]
		}
		active++
	}
	return nil
}

func matchOptionValueByLabel(label string, baseline []productOptionValueDraftData, used map[uint]struct{}) *productOptionValueDraftData {
	key := strings.ToLower(strings.TrimSpace(label))
	for i := range baseline {
		if baseline[i].SourceID == nil {
			continue
		}
		if _, exists := used[*baseline[i].SourceID]; exists {
			continue
		}
		if strings.ToLower(strings.TrimSpace(baseline[i].Value)) == key {
			return &baseline[i]
		}
	}
	return nil
}

func matchOptionValueByPosition(position int, baseline []productOptionValueDraftData, used map[uint]struct{}) *productOptionValueDraftData {
	for i := range baseline {
		if baseline[i].SourceID == nil {
			continue
		}
		if _, exists := used[*baseline[i].SourceID]; exists {
			continue
		}
		if baseline[i].Position == position {
			return &baseline[i]
		}
	}
	return nil
}

func matchOptionValueByActiveIndex(index int, baseline []productOptionValueDraftData, used map[uint]struct{}) *productOptionValueDraftData {
	active := 0
	for i := range baseline {
		if baseline[i].SourceID == nil {
			continue
		}
		if _, exists := used[*baseline[i].SourceID]; exists {
			continue
		}
		if active == index {
			return &baseline[i]
		}
		active++
	}
	return nil
}

func matchVariantBySelectionKey(
	key string,
	baseline []productVariantDraftData,
	baselineSelectionKeys map[uint]string,
	used map[uint]struct{},
) *productVariantDraftData {
	for i := range baseline {
		if baseline[i].SourceID == nil {
			continue
		}
		if _, exists := used[*baseline[i].SourceID]; exists {
			continue
		}
		if baselineSelectionKeys[*baseline[i].SourceID] == key {
			return &baseline[i]
		}
	}
	return nil
}

func matchVariantBySKU(sku string, baseline []productVariantDraftData, used map[uint]struct{}) *productVariantDraftData {
	key := strings.ToLower(strings.TrimSpace(sku))
	for i := range baseline {
		if baseline[i].SourceID == nil {
			continue
		}
		if _, exists := used[*baseline[i].SourceID]; exists {
			continue
		}
		if strings.ToLower(strings.TrimSpace(baseline[i].SKU)) == key {
			return &baseline[i]
		}
	}
	return nil
}

func matchVariantByPosition(position int, baseline []productVariantDraftData, used map[uint]struct{}) *productVariantDraftData {
	for i := range baseline {
		if baseline[i].SourceID == nil {
			continue
		}
		if _, exists := used[*baseline[i].SourceID]; exists {
			continue
		}
		if baseline[i].Position == position {
			return &baseline[i]
		}
	}
	return nil
}

func matchVariantByActiveIndex(index int, baseline []productVariantDraftData, used map[uint]struct{}) *productVariantDraftData {
	active := 0
	for i := range baseline {
		if baseline[i].SourceID == nil {
			continue
		}
		if _, exists := used[*baseline[i].SourceID]; exists {
			continue
		}
		if active == index {
			return &baseline[i]
		}
		active++
	}
	return nil
}

func variantSelectionSourceKey(variant productVariantDraftData) string {
	parts := make([]string, 0, len(variant.Selections))
	for _, selection := range variant.Selections {
		if selection.SourceOptionValueID == nil {
			return ""
		}
		parts = append(parts, fmt.Sprintf("%d", *selection.SourceOptionValueID))
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}

func publishedVariantIDs(tx *gorm.DB, productID uint) ([]uint, error) {
	var ids []uint
	if err := tx.Model(&models.ProductVariant{}).Where("product_id = ?", productID).Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func deletePublishedVariantOptionLinks(tx *gorm.DB, variantIDs []uint) error {
	if len(variantIDs) == 0 {
		return nil
	}
	return tx.Unscoped().Where("product_variant_id IN ?", variantIDs).Delete(&models.ProductVariantOptionValue{}).Error
}

func deletePublishedOptions(tx *gorm.DB, productID uint) error {
	var optionIDs []uint
	if err := tx.Model(&models.ProductOption{}).Where("product_id = ?", productID).Pluck("id", &optionIDs).Error; err != nil {
		return err
	}
	if len(optionIDs) > 0 {
		if err := tx.Unscoped().Where("product_option_id IN ?", optionIDs).Delete(&models.ProductOptionValue{}).Error; err != nil {
			return err
		}
	}
	return tx.Unscoped().Where("product_id = ?", productID).Delete(&models.ProductOption{}).Error
}

func deletePublishedAttributes(tx *gorm.DB, productID uint) error {
	return tx.Unscoped().Where("product_id = ?", productID).Delete(&models.ProductAttributeValue{}).Error
}

func deletePublishedStaleVariants(tx *gorm.DB, existingIDs []uint, kept map[uint]struct{}) error {
	stale := make([]uint, 0, len(existingIDs))
	for _, id := range existingIDs {
		if _, ok := kept[id]; ok {
			continue
		}
		stale = append(stale, id)
	}
	if len(stale) == 0 {
		return nil
	}
	return tx.Unscoped().Where("id IN ?", stale).Delete(&models.ProductVariant{}).Error
}

func containsUint(ids []uint, target uint) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func discardNormalizedProductDraft(tx *gorm.DB, product *models.Product) error {
	if product.IsPublished {
		if err := deleteNormalizedProductDraft(tx, product.ID); err != nil {
			return err
		}
		if err := tx.Model(product).Updates(map[string]any{
			"draft_updated_at": nil,
		}).Error; err != nil {
			return err
		}
		product.DraftUpdatedAt = nil
		return nil
	}

	live, err := loadPublishedProductCatalogData(tx, *product, false)
	if err != nil {
		return err
	}
	now := time.Now()
	if err := saveNormalizedProductDraft(tx, *product, live, now); err != nil {
		return err
	}
	if err := tx.Model(product).Updates(map[string]any{
		"draft_updated_at": now,
	}).Error; err != nil {
		return err
	}
	product.DraftUpdatedAt = &now
	return nil
}

func deletePublishedCatalogChildren(tx *gorm.DB, productID uint) error {
	var variantIDs []uint
	if err := tx.Model(&models.ProductVariant{}).Where("product_id = ?", productID).Pluck("id", &variantIDs).Error; err != nil {
		return err
	}
	if len(variantIDs) > 0 {
		if err := tx.Unscoped().Where("product_variant_id IN ?", variantIDs).Delete(&models.ProductVariantOptionValue{}).Error; err != nil {
			return err
		}
	}
	if err := tx.Unscoped().Where("product_id = ?", productID).Delete(&models.ProductVariant{}).Error; err != nil {
		return err
	}

	var optionIDs []uint
	if err := tx.Model(&models.ProductOption{}).Where("product_id = ?", productID).Pluck("id", &optionIDs).Error; err != nil {
		return err
	}
	if len(optionIDs) > 0 {
		if err := tx.Unscoped().Where("product_option_id IN ?", optionIDs).Delete(&models.ProductOptionValue{}).Error; err != nil {
			return err
		}
	}
	if err := tx.Unscoped().Where("product_id = ?", productID).Delete(&models.ProductOption{}).Error; err != nil {
		return err
	}

	if err := tx.Unscoped().Where("product_id = ?", productID).Delete(&models.ProductAttributeValue{}).Error; err != nil {
		return err
	}

	return tx.Unscoped().Where("entity_type = ? AND entity_id = ?", seoEntityTypeProduct, productID).Delete(&models.SEOMetadata{}).Error
}

func upsertPublishedSEO(tx *gorm.DB, productID uint, seo productSEODraftData) error {
	normalized := normalizeProductSEO(seo)
	if normalized.Title == nil && normalized.Description == nil && normalized.CanonicalPath == nil && normalized.OgImageMediaID == nil && !normalized.NoIndex {
		return tx.Unscoped().Where("entity_type = ? AND entity_id = ?", seoEntityTypeProduct, productID).Delete(&models.SEOMetadata{}).Error
	}
	record := models.SEOMetadata{
		EntityType:     seoEntityTypeProduct,
		EntityID:       productID,
		Title:          normalized.Title,
		Description:    normalized.Description,
		CanonicalPath:  normalized.CanonicalPath,
		OgImageMediaID: normalized.OgImageMediaID,
		NoIndex:        normalized.NoIndex,
	}

	var existing models.SEOMetadata
	if err := tx.Where("entity_type = ? AND entity_id = ?", seoEntityTypeProduct, productID).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&record).Error
		}
		return err
	}
	return tx.Model(&existing).Updates(map[string]any{
		"title":             normalized.Title,
		"description":       normalized.Description,
		"canonical_path":    normalized.CanonicalPath,
		"og_image_media_id": normalized.OgImageMediaID,
		"no_index":          normalized.NoIndex,
	}).Error
}

func encodeStringSlice(values []string) (string, error) {
	payload, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func decodeStringSlice(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
		return nil, err
	}
	return values, nil
}

func draftOptionValueKey(optionName, value string) string {
	return strings.ToLower(strings.TrimSpace(optionName)) + "\x00" + strings.ToLower(strings.TrimSpace(value))
}

func draftOptionSourceKey(sourceID uint) string {
	return fmt.Sprintf("source:%d", sourceID)
}

func moneyPtrFromFloatPtr(value *float64) *models.Money {
	if value == nil {
		return nil
	}
	money := models.MoneyFromFloat(*value)
	return &money
}

func floatPtrFromMoneyPtr(value *models.Money) *float64 {
	if value == nil {
		return nil
	}
	floatValue := value.Float64()
	return &floatValue
}
