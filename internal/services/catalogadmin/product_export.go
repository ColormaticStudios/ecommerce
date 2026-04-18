package catalogadmin

import (
	"fmt"
	"sort"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/media"
	"ecommerce/models"

	"gorm.io/gorm"
)

const productSEOEntityType = "product"

type liveOptionValueRecord struct {
	models.ProductOptionValue
	OptionName string
}

func LoadLiveProductUpsertInput(db *gorm.DB, mediaService *media.Service, productID uint) (apicontract.ProductUpsertInput, error) {
	var product models.Product
	if err := db.Preload("Related").First(&product, productID).Error; err != nil {
		return apicontract.ProductUpsertInput{}, err
	}

	var options []models.ProductOption
	if err := db.
		Preload("Values", func(tx *gorm.DB) *gorm.DB { return tx.Order("position asc").Order("id asc") }).
		Where("product_id = ?", product.ID).
		Order("position asc").
		Order("id asc").
		Find(&options).Error; err != nil {
		return apicontract.ProductUpsertInput{}, err
	}

	var variants []models.ProductVariant
	if err := db.
		Preload("OptionValueLinks", func(tx *gorm.DB) *gorm.DB { return tx.Order("id asc") }).
		Where("product_id = ?", product.ID).
		Order("position asc").
		Order("id asc").
		Find(&variants).Error; err != nil {
		return apicontract.ProductUpsertInput{}, err
	}

	var attributeValues []models.ProductAttributeValue
	if err := db.
		Preload("ProductAttribute").
		Where("product_id = ?", product.ID).
		Order("position asc").
		Order("id asc").
		Find(&attributeValues).Error; err != nil {
		return apicontract.ProductUpsertInput{}, err
	}

	var seo models.SEOMetadata
	seoErr := db.Where("entity_type = ? AND entity_id = ?", productSEOEntityType, product.ID).First(&seo).Error
	if seoErr != nil && seoErr != gorm.ErrRecordNotFound {
		return apicontract.ProductUpsertInput{}, seoErr
	}

	valueIDs := make([]uint, 0)
	for _, option := range options {
		for _, value := range option.Values {
			valueIDs = append(valueIDs, value.ID)
		}
	}
	valueMap, err := loadLiveOptionValueMap(db, valueIDs)
	if err != nil {
		return apicontract.ProductUpsertInput{}, err
	}

	input := apicontract.ProductUpsertInput{
		Attributes:        make([]apicontract.ProductAttributeValueInput, 0, len(attributeValues)),
		Description:       product.Description,
		Images:            append([]string(nil), product.Images...),
		Name:              product.Name,
		Options:           make([]apicontract.ProductOptionInput, 0, len(options)),
		RelatedProductIds: make([]int, 0, len(product.Related)),
		Seo: apicontract.ProductSEOInput{
			CanonicalPath:  seo.CanonicalPath,
			Description:    seo.Description,
			Noindex:        boolPtr(seo.NoIndex),
			OgImageMediaId: seo.OgImageMediaID,
			Title:          seo.Title,
		},
		Sku:      product.SKU,
		Subtitle: product.Subtitle,
		Variants: make([]apicontract.ProductVariantInput, 0, len(variants)),
	}
	if product.BrandID != nil {
		brandID := int(*product.BrandID)
		input.BrandId = &brandID
	}

	for _, related := range product.Related {
		input.RelatedProductIds = append(input.RelatedProductIds, int(related.ID))
	}
	sort.Ints(input.RelatedProductIds)

	if mediaService != nil {
		if urls, err := mediaService.ProductMediaURLsByRole(product.ID, media.RoleProductImage); err == nil && len(urls) > 0 {
			input.Images = urls
		}
	}

	for _, option := range options {
		position := option.Position
		displayType := option.DisplayType
		optionInput := apicontract.ProductOptionInput{
			DisplayType: &displayType,
			Name:        option.Name,
			Position:    &position,
			Values:      make([]apicontract.ProductOptionValueInput, 0, len(option.Values)),
		}
		for _, value := range option.Values {
			valuePosition := value.Position
			optionInput.Values = append(optionInput.Values, apicontract.ProductOptionValueInput{
				Position: &valuePosition,
				Value:    value.Value,
			})
		}
		input.Options = append(input.Options, optionInput)
	}

	defaultVariantSKU := ""
	for _, variant := range variants {
		if product.DefaultVariantID != nil && variant.ID == *product.DefaultVariantID {
			defaultVariantSKU = variant.SKU
		}

		position := variant.Position
		isPublished := variant.IsPublished
		variantInput := apicontract.ProductVariantInput{
			CompareAtPrice: moneyFloatPtr(variant.CompareAtPrice),
			HeightCm:       variant.HeightCm,
			IsPublished:    &isPublished,
			LengthCm:       variant.LengthCm,
			Position:       &position,
			Price:          variant.Price.Float64(),
			Selections:     make([]apicontract.ProductVariantSelectionInput, 0, len(variant.OptionValueLinks)),
			Sku:            variant.SKU,
			Stock:          variant.Stock,
			Title:          variant.Title,
			WeightGrams:    variant.WeightGrams,
			WidthCm:        variant.WidthCm,
		}

		for _, link := range variant.OptionValueLinks {
			value, ok := valueMap[link.ProductOptionValueID]
			if !ok {
				return apicontract.ProductUpsertInput{}, fmt.Errorf("option value %d not found for variant %d", link.ProductOptionValueID, variant.ID)
			}
			selectionPosition := value.Position
			variantInput.Selections = append(variantInput.Selections, apicontract.ProductVariantSelectionInput{
				OptionName:  value.OptionName,
				OptionValue: value.Value,
				Position:    &selectionPosition,
			})
		}
		input.Variants = append(input.Variants, variantInput)
	}

	if defaultVariantSKU != "" {
		input.DefaultVariantSku = &defaultVariantSKU
	}

	for _, attribute := range attributeValues {
		position := attribute.Position
		input.Attributes = append(input.Attributes, apicontract.ProductAttributeValueInput{
			BooleanValue:       attribute.BooleanValue,
			EnumValue:          attribute.EnumValue,
			NumberValue:        attribute.NumberValue,
			Position:           &position,
			ProductAttributeId: int(attribute.ProductAttributeID),
			TextValue:          attribute.TextValue,
		})
	}

	return input, nil
}

func ProductContractToUpsertInput(product apicontract.Product) apicontract.ProductUpsertInput {
	input := apicontract.ProductUpsertInput{
		Attributes:        make([]apicontract.ProductAttributeValueInput, 0, len(product.Attributes)),
		Description:       product.Description,
		Images:            append([]string(nil), product.Images...),
		Name:              product.Name,
		Options:           make([]apicontract.ProductOptionInput, 0, len(product.Options)),
		RelatedProductIds: make([]int, 0, len(product.RelatedProducts)),
		Seo: apicontract.ProductSEOInput{
			CanonicalPath:  product.Seo.CanonicalPath,
			Description:    product.Seo.Description,
			Noindex:        product.Seo.Noindex,
			OgImageMediaId: product.Seo.OgImageMediaId,
			Title:          product.Seo.Title,
		},
		Sku:      product.Sku,
		Subtitle: product.Subtitle,
		Variants: make([]apicontract.ProductVariantInput, 0, len(product.Variants)),
	}
	if product.Brand != nil {
		brandID := product.Brand.Id
		input.BrandId = &brandID
	}
	if product.DefaultVariantSku != nil {
		value := *product.DefaultVariantSku
		input.DefaultVariantSku = &value
	}

	for _, related := range product.RelatedProducts {
		input.RelatedProductIds = append(input.RelatedProductIds, related.Id)
	}
	sort.Ints(input.RelatedProductIds)

	for _, option := range product.Options {
		position := option.Position
		displayType := option.DisplayType
		entry := apicontract.ProductOptionInput{
			DisplayType: &displayType,
			Name:        option.Name,
			Position:    &position,
			Values:      make([]apicontract.ProductOptionValueInput, 0, len(option.Values)),
		}
		for _, value := range option.Values {
			valuePosition := value.Position
			entry.Values = append(entry.Values, apicontract.ProductOptionValueInput{
				Position: &valuePosition,
				Value:    value.Value,
			})
		}
		input.Options = append(input.Options, entry)
	}

	for _, variant := range product.Variants {
		position := variant.Position
		isPublished := variant.IsPublished
		entry := apicontract.ProductVariantInput{
			CompareAtPrice: variant.CompareAtPrice,
			HeightCm:       variant.HeightCm,
			IsPublished:    &isPublished,
			LengthCm:       variant.LengthCm,
			Position:       &position,
			Price:          variant.Price,
			Selections:     make([]apicontract.ProductVariantSelectionInput, 0, len(variant.Selections)),
			Sku:            variant.Sku,
			Stock:          variant.Stock,
			Title:          variant.Title,
			WeightGrams:    variant.WeightGrams,
			WidthCm:        variant.WidthCm,
		}
		for _, selection := range variant.Selections {
			selectionPosition := selection.Position
			entry.Selections = append(entry.Selections, apicontract.ProductVariantSelectionInput{
				OptionName:  selection.OptionName,
				OptionValue: selection.OptionValue,
				Position:    &selectionPosition,
			})
		}
		input.Variants = append(input.Variants, entry)
	}

	for _, attribute := range product.Attributes {
		position := attribute.Position
		input.Attributes = append(input.Attributes, apicontract.ProductAttributeValueInput{
			BooleanValue:       attribute.BooleanValue,
			EnumValue:          attribute.EnumValue,
			NumberValue:        attribute.NumberValue,
			Position:           &position,
			ProductAttributeId: attribute.ProductAttributeId,
			TextValue:          attribute.TextValue,
		})
	}

	return input
}

func loadLiveOptionValueMap(db *gorm.DB, ids []uint) (map[uint]liveOptionValueRecord, error) {
	if len(ids) == 0 {
		return map[uint]liveOptionValueRecord{}, nil
	}

	var rows []liveOptionValueRecord
	if err := db.
		Table("product_option_values").
		Select("product_option_values.*, product_options.name AS option_name").
		Joins("JOIN product_options ON product_options.id = product_option_values.product_option_id").
		Where("product_option_values.id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[uint]liveOptionValueRecord, len(rows))
	for _, row := range rows {
		result[row.ID] = row
	}
	return result, nil
}

func moneyFloatPtr(value *models.Money) *float64 {
	if value == nil {
		return nil
	}
	result := value.Float64()
	return &result
}

func boolPtr(value bool) *bool {
	result := value
	return &result
}
