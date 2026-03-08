package handlers

import (
	"errors"
	"fmt"
	"strings"

	"ecommerce/internal/apicontract"
	"ecommerce/models"

	"gorm.io/gorm"
)

func catalogDraftFromUpsertInput(input apicontract.ProductUpsertInput) productCatalogDraft {
	draft := productCatalogDraft{
		SKU:         strings.TrimSpace(input.Sku),
		Name:        strings.TrimSpace(input.Name),
		Subtitle:    input.Subtitle,
		Description: strings.TrimSpace(input.Description),
		Images:      append([]string(nil), input.Images...),
		RelatedIDs:  make([]uint, 0, len(input.RelatedProductIds)),
		Options:     make([]productOptionDraftData, 0, len(input.Options)),
		Variants:    make([]productVariantDraftData, 0, len(input.Variants)),
		Attributes:  make([]productAttributeValueDraftData, 0, len(input.Attributes)),
		SEO: productSEODraftData{
			Title:          input.Seo.Title,
			Description:    input.Seo.Description,
			CanonicalPath:  input.Seo.CanonicalPath,
			OgImageMediaID: input.Seo.OgImageMediaId,
			NoIndex:        input.Seo.Noindex != nil && *input.Seo.Noindex,
		},
	}

	if input.BrandId != nil {
		brandID := uint(*input.BrandId)
		draft.BrandID = &brandID
	}
	if input.DefaultVariantSku != nil {
		draft.DefaultVariantSKU = strings.TrimSpace(*input.DefaultVariantSku)
	}

	for _, id := range input.RelatedProductIds {
		if id <= 0 {
			continue
		}
		draft.RelatedIDs = append(draft.RelatedIDs, uint(id))
	}

	for _, option := range input.Options {
		optionDraft := productOptionDraftData{
			Name: option.Name,
		}
		if option.Position != nil {
			optionDraft.Position = *option.Position
		}
		if option.DisplayType != nil {
			optionDraft.DisplayType = *option.DisplayType
		}
		for _, value := range option.Values {
			valueDraft := productOptionValueDraftData{
				Value: value.Value,
			}
			if value.Position != nil {
				valueDraft.Position = *value.Position
			}
			optionDraft.Values = append(optionDraft.Values, valueDraft)
		}
		draft.Options = append(draft.Options, optionDraft)
	}

	for _, variant := range input.Variants {
		variantDraft := productVariantDraftData{
			SKU:            variant.Sku,
			Title:          variant.Title,
			Price:          variant.Price,
			CompareAtPrice: variant.CompareAtPrice,
			Stock:          variant.Stock,
			IsPublished:    variant.IsPublished == nil || *variant.IsPublished,
			WeightGrams:    variant.WeightGrams,
			LengthCm:       variant.LengthCm,
			WidthCm:        variant.WidthCm,
			HeightCm:       variant.HeightCm,
		}
		if variant.Position != nil {
			variantDraft.Position = *variant.Position
		}
		for _, selection := range variant.Selections {
			selectionDraft := productVariantSelectionDraftData{
				OptionName:  selection.OptionName,
				OptionValue: selection.OptionValue,
			}
			if selection.Position != nil {
				selectionDraft.Position = *selection.Position
			}
			variantDraft.Selections = append(variantDraft.Selections, selectionDraft)
		}
		draft.Variants = append(draft.Variants, variantDraft)
	}

	for _, attribute := range input.Attributes {
		attributeDraft := productAttributeValueDraftData{
			ProductAttributeID: uint(attribute.ProductAttributeId),
			TextValue:          attribute.TextValue,
			NumberValue:        attribute.NumberValue,
			BooleanValue:       attribute.BooleanValue,
			EnumValue:          attribute.EnumValue,
		}
		if attribute.Position != nil {
			attributeDraft.Position = *attribute.Position
		}
		draft.Attributes = append(draft.Attributes, attributeDraft)
	}

	return normalizeProductCatalogDraft(draft)
}

func deriveCatalogMerchandising(draft productCatalogDraft) (productCatalogDraft, error) {
	normalized := normalizeProductCatalogDraft(draft)

	var selected *productVariantDraftData
	for i := range normalized.Variants {
		variant := &normalized.Variants[i]
		if variant.Deleted {
			continue
		}
		if selected == nil {
			selected = variant
		}
		if normalized.DefaultVariantSKU != "" && variant.SKU == normalized.DefaultVariantSKU {
			selected = variant
			break
		}
	}

	if selected == nil {
		return normalized, errors.New("At least one variant is required")
	}

	normalized.DefaultVariantSKU = selected.SKU
	normalized.Price = selected.Price
	normalized.Stock = selected.Stock
	return normalized, nil
}

func validateProductCatalogDraft(tx *gorm.DB, draft productCatalogDraft, productID uint) error {
	normalized, err := deriveCatalogMerchandising(draft)
	if err != nil {
		return err
	}

	if strings.TrimSpace(normalized.SKU) == "" {
		return errors.New("Product SKU is required")
	}
	if strings.TrimSpace(normalized.Name) == "" {
		return errors.New("Product name is required")
	}
	if err := ensureUniqueProductSKU(tx, normalized.SKU, productID); err != nil {
		return err
	}
	if normalized.BrandID != nil {
		var count int64
		if err := tx.Model(&models.Brand{}).Where("id = ?", *normalized.BrandID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errors.New("Selected brand does not exist")
		}
	}
	for _, relatedID := range normalized.RelatedIDs {
		if productID != 0 && relatedID == productID {
			return errors.New("Product cannot be related to itself")
		}
	}
	if err := validateProductOptions(normalized); err != nil {
		return err
	}
	if err := validateProductVariants(tx, normalized, productID); err != nil {
		return err
	}
	if err := validateProductAttributes(tx, normalized.Attributes); err != nil {
		return err
	}
	if err := validateCatalogSEO(tx, normalized.SEO, productID); err != nil {
		return err
	}
	return nil
}

func validateProductOptions(draft productCatalogDraft) error {
	seenOptions := map[string]struct{}{}
	for _, option := range draft.Options {
		if option.Deleted {
			continue
		}
		name := strings.TrimSpace(option.Name)
		if name == "" {
			return errors.New("Option name is required")
		}
		key := strings.ToLower(name)
		if _, exists := seenOptions[key]; exists {
			return fmt.Errorf("Duplicate option name %q", name)
		}
		seenOptions[key] = struct{}{}

		seenValues := map[string]struct{}{}
		for _, value := range option.Values {
			if value.Deleted {
				continue
			}
			label := strings.TrimSpace(value.Value)
			if label == "" {
				return fmt.Errorf("Option %q has an empty value", name)
			}
			valueKey := strings.ToLower(label)
			if _, exists := seenValues[valueKey]; exists {
				return fmt.Errorf("Option %q has duplicate value %q", name, label)
			}
			seenValues[valueKey] = struct{}{}
		}
		if len(seenValues) == 0 {
			return fmt.Errorf("Option %q must include at least one value", name)
		}
	}
	return nil
}

func validateProductVariants(tx *gorm.DB, draft productCatalogDraft, productID uint) error {
	activeOptions := make([]productOptionDraftData, 0, len(draft.Options))
	optionValues := map[string]map[string]struct{}{}
	for _, option := range draft.Options {
		if option.Deleted {
			continue
		}
		activeOptions = append(activeOptions, option)
		values := map[string]struct{}{}
		for _, value := range option.Values {
			if value.Deleted {
				continue
			}
			values[strings.ToLower(strings.TrimSpace(value.Value))] = struct{}{}
		}
		optionValues[strings.ToLower(strings.TrimSpace(option.Name))] = values
	}

	activeVariants := make([]productVariantDraftData, 0, len(draft.Variants))
	seenSKUs := map[string]struct{}{}
	seenCombinations := map[string]struct{}{}
	skus := make([]string, 0, len(draft.Variants))

	for _, variant := range draft.Variants {
		if variant.Deleted {
			continue
		}
		activeVariants = append(activeVariants, variant)

		sku := strings.TrimSpace(variant.SKU)
		if sku == "" {
			return errors.New("Variant SKU is required")
		}
		if strings.TrimSpace(variant.Title) == "" {
			return fmt.Errorf("Variant %q requires a title", sku)
		}
		if variant.Price <= 0 {
			return fmt.Errorf("Variant %q price must be greater than 0", sku)
		}

		skuKey := strings.ToLower(sku)
		if _, exists := seenSKUs[skuKey]; exists {
			return fmt.Errorf("Duplicate variant SKU %q", sku)
		}
		seenSKUs[skuKey] = struct{}{}
		skus = append(skus, skuKey)

		combinationKey, err := variantCombinationKey(variant, activeOptions, optionValues)
		if err != nil {
			return err
		}
		if _, exists := seenCombinations[combinationKey]; exists {
			return fmt.Errorf("Duplicate variant combination for %q", sku)
		}
		seenCombinations[combinationKey] = struct{}{}
	}

	if len(activeVariants) == 0 {
		return errors.New("At least one variant is required")
	}
	if len(activeOptions) == 0 && len(activeVariants) > 1 {
		return errors.New("Multiple variants require at least one option")
	}

	var existingCount int64
	if len(skus) > 0 {
		if err := tx.Model(&models.ProductVariant{}).
			Where("lower(sku) IN ?", skus).
			Where("product_id <> ?", productID).
			Count(&existingCount).Error; err != nil {
			return err
		}
	}
	if existingCount > 0 {
		return errors.New("Variant SKU already exists")
	}

	defaultFound := draft.DefaultVariantSKU == ""
	for _, variant := range activeVariants {
		if variant.SKU == draft.DefaultVariantSKU {
			defaultFound = true
			break
		}
	}
	if !defaultFound {
		return errors.New("Default variant SKU must match one of the variants")
	}

	return nil
}

func variantCombinationKey(
	variant productVariantDraftData,
	options []productOptionDraftData,
	optionValues map[string]map[string]struct{},
) (string, error) {
	if len(options) == 0 {
		if len(variant.Selections) > 0 {
			return "", fmt.Errorf("Variant %q cannot define selections without product options", variant.SKU)
		}
		return "default", nil
	}

	selected := map[string]string{}
	for _, selection := range variant.Selections {
		optionKey := strings.ToLower(strings.TrimSpace(selection.OptionName))
		if optionKey == "" {
			return "", fmt.Errorf("Variant %q has a selection with no option name", variant.SKU)
		}
		if _, exists := optionValues[optionKey]; !exists {
			return "", fmt.Errorf("Variant %q references unknown option %q", variant.SKU, selection.OptionName)
		}
		valueKey := strings.ToLower(strings.TrimSpace(selection.OptionValue))
		if valueKey == "" {
			return "", fmt.Errorf("Variant %q has an empty selection value for %q", variant.SKU, selection.OptionName)
		}
		if _, exists := optionValues[optionKey][valueKey]; !exists {
			return "", fmt.Errorf("Variant %q references unknown option value %q=%q", variant.SKU, selection.OptionName, selection.OptionValue)
		}
		if _, exists := selected[optionKey]; exists {
			return "", fmt.Errorf("Variant %q repeats option %q", variant.SKU, selection.OptionName)
		}
		selected[optionKey] = valueKey
	}

	parts := make([]string, 0, len(options))
	for _, option := range options {
		optionKey := strings.ToLower(strings.TrimSpace(option.Name))
		valueKey, exists := selected[optionKey]
		if !exists {
			return "", fmt.Errorf("Variant %q is missing a selection for option %q", variant.SKU, option.Name)
		}
		parts = append(parts, optionKey+"="+valueKey)
	}

	return strings.Join(parts, "|"), nil
}

func validateProductAttributes(tx *gorm.DB, attributes []productAttributeValueDraftData) error {
	if len(attributes) == 0 {
		return nil
	}

	ids := make([]uint, 0, len(attributes))
	seen := map[uint]struct{}{}
	for _, attribute := range attributes {
		if attribute.Deleted {
			continue
		}
		if attribute.ProductAttributeID == 0 {
			return errors.New("Product attribute ID is required")
		}
		if _, exists := seen[attribute.ProductAttributeID]; exists {
			return fmt.Errorf("Duplicate product attribute %d", attribute.ProductAttributeID)
		}
		seen[attribute.ProductAttributeID] = struct{}{}
		ids = append(ids, attribute.ProductAttributeID)
	}
	if len(ids) == 0 {
		return nil
	}

	var definitions []models.ProductAttribute
	if err := tx.Where("id IN ?", ids).Find(&definitions).Error; err != nil {
		return err
	}
	byID := make(map[uint]models.ProductAttribute, len(definitions))
	for _, definition := range definitions {
		byID[definition.ID] = definition
	}

	for _, attribute := range attributes {
		if attribute.Deleted {
			continue
		}
		definition, exists := byID[attribute.ProductAttributeID]
		if !exists {
			return fmt.Errorf("Product attribute %d does not exist", attribute.ProductAttributeID)
		}
		if err := validateAttributeValue(definition, attribute); err != nil {
			return err
		}
	}
	return nil
}

func validateAttributeValue(definition models.ProductAttribute, value productAttributeValueDraftData) error {
	setCount := 0
	if value.TextValue != nil {
		setCount++
	}
	if value.NumberValue != nil {
		setCount++
	}
	if value.BooleanValue != nil {
		setCount++
	}
	if value.EnumValue != nil {
		setCount++
	}
	if setCount != 1 {
		return fmt.Errorf("Attribute %q must provide exactly one typed value", definition.Key)
	}

	switch definition.Type {
	case "text":
		if value.TextValue == nil {
			return fmt.Errorf("Attribute %q requires a text value", definition.Key)
		}
	case "number":
		if value.NumberValue == nil {
			return fmt.Errorf("Attribute %q requires a numeric value", definition.Key)
		}
	case "boolean":
		if value.BooleanValue == nil {
			return fmt.Errorf("Attribute %q requires a boolean value", definition.Key)
		}
	case "enum":
		if value.EnumValue == nil {
			return fmt.Errorf("Attribute %q requires an enum value", definition.Key)
		}
	default:
		return fmt.Errorf("Attribute %q has unsupported type %q", definition.Key, definition.Type)
	}

	return nil
}
