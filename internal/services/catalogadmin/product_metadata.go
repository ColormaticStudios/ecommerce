package catalogadmin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"ecommerce/internal/apicontract"
	"ecommerce/models"
	"gorm.io/gorm"
)

// ProductInput returns the complete editable draft, or the live document when
// no draft exists. Consumers must retain this document for partial edits.
func (s *Service) ProductInput(ctx context.Context, id uint) (apicontract.ProductUpsertInput, error) {
	db := s.db.WithContext(ctx)
	var product models.Product
	if err := db.First(&product, id).Error; err != nil {
		return apicontract.ProductUpsertInput{}, err
	}
	if product.DraftUpdatedAt == nil {
		return LoadLiveProductUpsertInput(db, s.media, id)
	}
	var draft models.ProductDraft
	if err := completeDraftQuery(db).Where("product_id = ?", id).First(&draft).Error; err != nil {
		return apicontract.ProductUpsertInput{}, err
	}
	return inputFromDraft(draft)
}

func completeDraftQuery(db *gorm.DB) *gorm.DB {
	ordered := func(tx *gorm.DB) *gorm.DB { return tx.Order("position asc, id asc") }
	return db.Preload("OptionDrafts", ordered).Preload("OptionDrafts.ValueDrafts", ordered).Preload("VariantDrafts", ordered).Preload("VariantDrafts.OptionValueDraftLinks", ordered).Preload("AttributeDrafts", ordered).Preload("CategoryDrafts", ordered).Preload("RelatedDrafts", ordered)
}

func inputFromDraft(draft models.ProductDraft) (apicontract.ProductUpsertInput, error) {
	input := apicontract.ProductUpsertInput{Sku: draft.SKU, Name: draft.Name, Subtitle: draft.Subtitle, Description: draft.Description, Seo: apicontract.ProductSEOInput{Title: draft.SeoTitle, Description: draft.SeoDescription, CanonicalPath: draft.SeoCanonicalPath, OgImageMediaId: draft.SeoOgImageMediaID, Noindex: &draft.SeoNoIndex}, Options: []apicontract.ProductOptionInput{}, Attributes: []apicontract.ProductAttributeValueInput{}, Variants: []apicontract.ProductVariantInput{}}
	if err := json.Unmarshal([]byte(draft.ImagesJSON), &input.Images); err != nil {
		return apicontract.ProductUpsertInput{}, fmt.Errorf("decode product draft images: %w", err)
	}
	for _, item := range draft.CategoryDrafts {
		input.CategoryIds = append(input.CategoryIds, int(item.CategoryID))
	}
	for _, item := range draft.RelatedDrafts {
		input.RelatedProductIds = append(input.RelatedProductIds, int(item.RelatedProductID))
	}
	if draft.BrandID != nil {
		id := int(*draft.BrandID)
		input.BrandId = &id
	}
	if draft.DefaultVariantSKU != "" {
		input.DefaultVariantSku = &draft.DefaultVariantSKU
	}
	for _, option := range draft.OptionDrafts {
		if option.IsDeleted {
			continue
		}
		value := apicontract.ProductOptionInput{Name: option.Name, Position: &option.Position, DisplayType: &option.DisplayType, Values: []apicontract.ProductOptionValueInput{}}
		for _, item := range option.ValueDrafts {
			if !item.IsDeleted {
				value.Values = append(value.Values, apicontract.ProductOptionValueInput{Value: item.Value, Position: &item.Position})
			}
		}
		input.Options = append(input.Options, value)
	}
	for _, variant := range draft.VariantDrafts {
		if variant.IsDeleted {
			continue
		}
		value := apicontract.ProductVariantInput{Sku: variant.SKU, Title: variant.Title, Price: variant.Price.Float64(), UnitCost: moneyFloatPtr(variant.UnitCost), CompareAtPrice: moneyFloatPtr(variant.CompareAtPrice), Stock: variant.Stock, Position: &variant.Position, IsPublished: &variant.IsPublished, WeightGrams: variant.WeightGrams, LengthCm: variant.LengthCm, WidthCm: variant.WidthCm, HeightCm: variant.HeightCm, Selections: []apicontract.ProductVariantSelectionInput{}}
		for _, item := range variant.OptionValueDraftLinks {
			value.Selections = append(value.Selections, apicontract.ProductVariantSelectionInput{OptionName: item.OptionName, OptionValue: item.OptionValue, Position: &item.Position})
		}
		input.Variants = append(input.Variants, value)
	}
	for _, item := range draft.AttributeDrafts {
		if !item.IsDeleted {
			input.Attributes = append(input.Attributes, apicontract.ProductAttributeValueInput{ProductAttributeId: int(item.ProductAttributeID), TextValue: item.TextValue, NumberValue: item.NumberValue, BooleanValue: item.BooleanValue, EnumValue: item.EnumValue, Position: &item.Position})
		}
	}
	return input, nil
}

func persistDraftMetadata(tx *gorm.DB, draft models.ProductDraft, input apicontract.ProductUpsertInput) error {
	if err := validateProductMetadata(tx, input); err != nil {
		return err
	}
	for i, item := range input.Options {
		position := i + 1
		if item.Position != nil {
			position = *item.Position
		}
		display := "select"
		if item.DisplayType != nil {
			display = *item.DisplayType
		}
		option := models.ProductOptionDraft{ProductDraftID: draft.ID, Name: item.Name, Position: position, DisplayType: display}
		if err := tx.Create(&option).Error; err != nil {
			return err
		}
		for j, value := range item.Values {
			position := j + 1
			if value.Position != nil {
				position = *value.Position
			}
			if err := tx.Create(&models.ProductOptionValueDraft{ProductOptionDraftID: option.ID, Value: value.Value, Position: position}).Error; err != nil {
				return err
			}
		}
	}
	var variants []models.ProductVariantDraft
	if err := tx.Where("product_draft_id = ?", draft.ID).Find(&variants).Error; err != nil {
		return err
	}
	variantIDs := map[string]uint{}
	for _, variant := range variants {
		variantIDs[variant.SKU] = variant.ID
	}
	for _, variant := range input.Variants {
		for i, item := range variant.Selections {
			position := i + 1
			if item.Position != nil {
				position = *item.Position
			}
			if err := tx.Create(&models.ProductVariantOptionValueDraft{ProductVariantDraftID: variantIDs[strings.TrimSpace(variant.Sku)], OptionName: item.OptionName, OptionValue: item.OptionValue, Position: position}).Error; err != nil {
				return err
			}
		}
	}
	for i, item := range input.Attributes {
		position := i + 1
		if item.Position != nil {
			position = *item.Position
		}
		if err := tx.Create(&models.ProductAttributeValueDraft{ProductDraftID: draft.ID, ProductAttributeID: uint(item.ProductAttributeId), TextValue: item.TextValue, NumberValue: item.NumberValue, BooleanValue: item.BooleanValue, EnumValue: item.EnumValue, Position: position}).Error; err != nil {
			return err
		}
	}
	return nil
}

func validateProductMetadata(tx *gorm.DB, input apicontract.ProductUpsertInput) error {
	options := map[string]map[string]bool{}
	for _, option := range input.Options {
		if strings.TrimSpace(option.Name) == "" || options[option.Name] != nil {
			return invalidInput("invalid_product_option", "Option names must be nonempty and unique.")
		}
		options[option.Name] = map[string]bool{}
		for _, value := range option.Values {
			if strings.TrimSpace(value.Value) == "" || options[option.Name][value.Value] {
				return invalidInput("invalid_product_option", "Option values must be nonempty and unique within the option.")
			}
			options[option.Name][value.Value] = true
		}
	}
	for _, variant := range input.Variants {
		seen := map[string]bool{}
		for _, selection := range variant.Selections {
			if !options[selection.OptionName][selection.OptionValue] || seen[selection.OptionName] {
				return invalidInput("invalid_product_variant", "Variant selections must name an existing option value once per option.")
			}
			seen[selection.OptionName] = true
		}
	}
	seen := map[int]bool{}
	for _, value := range input.Attributes {
		if value.ProductAttributeId < 1 || seen[value.ProductAttributeId] {
			return invalidInput("invalid_product_attribute", "Attribute IDs must be positive and unique.")
		}
		seen[value.ProductAttributeId] = true
		var definition models.ProductAttribute
		if err := tx.First(&definition, value.ProductAttributeId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return invalidInput("invalid_product_attribute", "Attribute definition does not exist.")
			}
			return err
		}
		count := 0
		if value.TextValue != nil {
			count++
		}
		if value.NumberValue != nil {
			count++
		}
		if value.BooleanValue != nil {
			count++
		}
		if value.EnumValue != nil {
			count++
		}
		valid := count == 1
		switch definition.Type {
		case "text":
			valid = valid && value.TextValue != nil
		case "number":
			valid = valid && value.NumberValue != nil && !math.IsNaN(*value.NumberValue) && !math.IsInf(*value.NumberValue, 0)
		case "boolean":
			valid = valid && value.BooleanValue != nil
		case "enum":
			valid = valid && value.EnumValue != nil
			if valid {
				found := false
				for _, item := range definition.EnumValues {
					if item == *value.EnumValue {
						found = true
					}
				}
				valid = found
			}
		default:
			valid = false
		}
		if !valid {
			return invalidInput("invalid_product_attribute", "Attribute value must match its definition type.")
		}
	}
	return nil
}

func publishDraftMetadata(tx *gorm.DB, productID uint, draftID uint) error {
	var draft models.ProductDraft
	if err := completeDraftQuery(tx).First(&draft, draftID).Error; err != nil {
		return err
	}
	input, err := inputFromDraft(draft)
	if err != nil {
		return err
	}
	variantIDs := tx.Unscoped().Model(&models.ProductVariant{}).Select("id").Where("product_id = ?", productID)
	if err := tx.Unscoped().Where("product_variant_id IN (?)", variantIDs).Delete(&models.ProductVariantOptionValue{}).Error; err != nil {
		return err
	}
	optionIDs := tx.Model(&models.ProductOption{}).Select("id").Where("product_id = ?", productID)
	if err := tx.Unscoped().Where("product_option_id IN (?)", optionIDs).Delete(&models.ProductOptionValue{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("product_id = ?", productID).Delete(&models.ProductOption{}).Error; err != nil {
		return err
	}
	valueIDs := map[string]map[string]uint{}
	for i, item := range input.Options {
		position := i + 1
		if item.Position != nil {
			position = *item.Position
		}
		display := "select"
		if item.DisplayType != nil {
			display = *item.DisplayType
		}
		option := models.ProductOption{ProductID: productID, Name: item.Name, Position: position, DisplayType: display}
		if err := tx.Create(&option).Error; err != nil {
			return err
		}
		valueIDs[item.Name] = map[string]uint{}
		for j, value := range item.Values {
			position := j + 1
			if value.Position != nil {
				position = *value.Position
			}
			row := models.ProductOptionValue{ProductOptionID: option.ID, Value: value.Value, Position: position}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			valueIDs[item.Name][value.Value] = row.ID
		}
	}
	var variants []models.ProductVariant
	if err := tx.Where("product_id = ?", productID).Find(&variants).Error; err != nil {
		return err
	}
	bySKU := map[string]uint{}
	for _, variant := range variants {
		bySKU[variant.SKU] = variant.ID
	}
	for _, variant := range input.Variants {
		for _, selection := range variant.Selections {
			valueID := valueIDs[selection.OptionName][selection.OptionValue]
			if valueID == 0 {
				return invalidInput("invalid_product_variant", fmt.Sprintf("Unknown option selection %s / %s.", selection.OptionName, selection.OptionValue))
			}
			if err := tx.Create(&models.ProductVariantOptionValue{ProductVariantID: bySKU[variant.Sku], ProductOptionValueID: valueID}).Error; err != nil {
				return err
			}
		}
	}
	if err := tx.Unscoped().Where("product_id = ?", productID).Delete(&models.ProductAttributeValue{}).Error; err != nil {
		return err
	}
	for i, item := range input.Attributes {
		position := i + 1
		if item.Position != nil {
			position = *item.Position
		}
		if err := tx.Create(&models.ProductAttributeValue{ProductID: productID, ProductAttributeID: uint(item.ProductAttributeId), TextValue: item.TextValue, NumberValue: item.NumberValue, BooleanValue: item.BooleanValue, EnumValue: item.EnumValue, Position: position}).Error; err != nil {
			return err
		}
	}
	var seo models.SEOMetadata
	err = tx.Where("entity_type = ? AND entity_id = ?", productSEOEntityType, productID).First(&seo).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	seo.EntityType, seo.EntityID = productSEOEntityType, productID
	seo.Title, seo.Description, seo.CanonicalPath, seo.OgImageMediaID, seo.NoIndex = draft.SeoTitle, draft.SeoDescription, draft.SeoCanonicalPath, draft.SeoOgImageMediaID, draft.SeoNoIndex
	return tx.Save(&seo).Error
}
