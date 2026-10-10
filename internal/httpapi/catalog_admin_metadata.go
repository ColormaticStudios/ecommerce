package httpapi

import (
	"context"
	"sort"

	"ecommerce/internal/apicontract"
	"ecommerce/models"
)

// Complete administrator documents are required by the replace-draft API and
// consumers which edit a single field through a read/modify/write operation.
func (e *CatalogEndpoints) hydrateAdminProduct(ctx context.Context, id uint, result *apicontract.Product) error {
	input, err := e.catalogAdmin.ProductInput(ctx, id)
	if err != nil {
		return err
	}
	result.DefaultVariantSku = input.DefaultVariantSku
	sort.SliceStable(result.Variants, func(i, j int) bool { return result.Variants[i].Position < result.Variants[j].Position })
	result.Seo = apicontract.ProductSEO{Title: input.Seo.Title, Description: input.Seo.Description, CanonicalPath: input.Seo.CanonicalPath, OgImageMediaId: input.Seo.OgImageMediaId, Noindex: input.Seo.Noindex}
	if input.BrandId != nil {
		var brand models.Brand
		if err := e.db.WithContext(ctx).First(&brand, *input.BrandId).Error; err != nil {
			return err
		}
		value, err := e.brandContract(ctx, brand, false)
		if err != nil {
			return err
		}
		result.Brand = &value
	} else {
		result.Brand = nil
	}
	for _, item := range input.Options {
		position := 0
		if item.Position != nil {
			position = *item.Position
		}
		display := "select"
		if item.DisplayType != nil {
			display = *item.DisplayType
		}
		option := apicontract.ProductOption{Name: item.Name, Position: position, DisplayType: display, Values: []apicontract.ProductOptionValue{}}
		for _, value := range item.Values {
			position := 0
			if value.Position != nil {
				position = *value.Position
			}
			option.Values = append(option.Values, apicontract.ProductOptionValue{Value: value.Value, Position: position})
		}
		result.Options = append(result.Options, option)
	}
	for _, item := range input.Attributes {
		var definition models.ProductAttribute
		if err := e.db.WithContext(ctx).First(&definition, item.ProductAttributeId).Error; err != nil {
			return err
		}
		position := 0
		if item.Position != nil {
			position = *item.Position
		}
		result.Attributes = append(result.Attributes, apicontract.ProductAttributeValue{ProductAttributeId: item.ProductAttributeId, Key: definition.Key, Slug: definition.Slug, Type: definition.Type, Position: position, TextValue: item.TextValue, NumberValue: item.NumberValue, BooleanValue: item.BooleanValue, EnumValue: item.EnumValue})
	}
	for i := range result.Variants {
		for _, variant := range input.Variants {
			if variant.Sku != result.Variants[i].Sku {
				continue
			}
			for _, selection := range variant.Selections {
				position := 0
				if selection.Position != nil {
					position = *selection.Position
				}
				result.Variants[i].Selections = append(result.Variants[i].Selections, apicontract.ProductVariantSelection{OptionName: selection.OptionName, OptionValue: selection.OptionValue, Position: position})
			}
		}
	}
	return nil
}
