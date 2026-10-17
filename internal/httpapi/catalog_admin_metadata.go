package httpapi

import (
	"context"
	"sort"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/media"
	"ecommerce/models"
	"gorm.io/gorm"
)

type adminMetadataKey struct{}
type adminMetadata struct {
	inputs      map[uint]apicontract.ProductUpsertInput
	brands      map[uint]apicontract.Brand
	definitions map[int]models.ProductAttribute
	logos       map[uint][]string
}

func (e *CatalogEndpoints) adminMetadataContext(ctx context.Context, products []models.Product) (context.Context, error) {
	ids := []uint{}
	for _, p := range products {
		ids = append(ids, p.ID)
	}
	inputs, err := e.catalogAdmin.ProductInputs(ctx, ids)
	if err != nil {
		return ctx, err
	}
	metadata := &adminMetadata{inputs: inputs, brands: map[uint]apicontract.Brand{}, definitions: map[int]models.ProductAttribute{}}
	ctx = context.WithValue(ctx, adminMetadataKey{}, metadata)
	if e.media != nil {
		draftIDs := []uint{}
		for _, p := range products {
			if p.DraftUpdatedAt != nil {
				draftIDs = append(draftIDs, p.ID)
			}
		}
		urls, err := e.media.OwnerMediaURLs(ctx, media.OwnerTypeProduct, draftIDs, media.RoleProductDraftImage)
		if err != nil {
			return ctx, err
		}
		for id, images := range urls {
			input := metadata.inputs[id]
			input.Images = images
			metadata.inputs[id] = input
		}
	}
	brandIDs, attributeIDs := map[int]bool{}, map[int]bool{}
	for _, input := range inputs {
		if input.BrandId != nil {
			brandIDs[*input.BrandId] = true
		}
		for _, a := range input.Attributes {
			attributeIDs[a.ProductAttributeId] = true
		}
	}
	brands := []int{}
	for id := range brandIDs {
		brands = append(brands, id)
	}
	if e.media != nil {
		ids := []uint{}
		for _, id := range brands {
			ids = append(ids, uint(id))
		}
		metadata.logos, err = e.media.OwnerMediaURLs(ctx, media.OwnerTypeBrand, ids, media.RoleBrandLogo)
		if err != nil {
			return ctx, err
		}
	}
	if len(brands) > 0 {
		var rows []models.Brand
		if err := e.db.WithContext(ctx).Where("id IN ?", brands).Find(&rows).Error; err != nil {
			return ctx, err
		}
		if len(rows) != len(brands) {
			return ctx, gorm.ErrRecordNotFound
		}
		for _, row := range rows {
			value, err := e.brandContract(ctx, row, false)
			if err != nil {
				return ctx, err
			}
			metadata.brands[row.ID] = value
		}
	}
	attrs := []int{}
	for id := range attributeIDs {
		attrs = append(attrs, id)
	}
	if len(attrs) > 0 {
		var rows []models.ProductAttribute
		if err := e.db.WithContext(ctx).Where("id IN ?", attrs).Find(&rows).Error; err != nil {
			return ctx, err
		}
		if len(rows) != len(attrs) {
			return ctx, gorm.ErrRecordNotFound
		}
		for _, row := range rows {
			metadata.definitions[int(row.ID)] = row
		}
	}
	return context.WithValue(ctx, adminMetadataKey{}, metadata), nil
}

// Complete administrator documents are required by the replace-draft API and
// consumers which edit a single field through a read/modify/write operation.
func (e *CatalogEndpoints) hydrateAdminProduct(ctx context.Context, id uint, result *apicontract.Product) error {
	metadata, _ := ctx.Value(adminMetadataKey{}).(*adminMetadata)
	var input apicontract.ProductUpsertInput
	if metadata != nil {
		var ok bool
		input, ok = metadata.inputs[id]
		if !ok {
			return gorm.ErrRecordNotFound
		}
	} else {
		var err error
		input, err = e.catalogAdmin.ProductInput(ctx, id)
		if err != nil {
			return err
		}
	}
	result.DefaultVariantSku = input.DefaultVariantSku
	sort.SliceStable(result.Variants, func(i, j int) bool { return result.Variants[i].Position < result.Variants[j].Position })
	result.Seo = apicontract.ProductSEO{Title: input.Seo.Title, Description: input.Seo.Description, CanonicalPath: input.Seo.CanonicalPath, OgImageMediaId: input.Seo.OgImageMediaId, Noindex: input.Seo.Noindex}
	if input.BrandId != nil {
		if metadata != nil {
			value := metadata.brands[uint(*input.BrandId)]
			result.Brand = &value
		} else {
			var brand models.Brand
			if err := e.db.WithContext(ctx).First(&brand, *input.BrandId).Error; err != nil {
				return err
			}
			value, err := e.brandContract(ctx, brand, false)
			if err != nil {
				return err
			}
			result.Brand = &value
		}
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
		if metadata != nil {
			definition = metadata.definitions[item.ProductAttributeId]
		} else if err := e.db.WithContext(ctx).First(&definition, item.ProductAttributeId).Error; err != nil {
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
