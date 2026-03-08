package handlers

import (
	"fmt"
	"net/http"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/media"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func respondAdminProduct(c *gin.Context, db *gorm.DB, mediaService *media.Service, productID uint) {
	var product models.Product
	if err := db.Preload("Related").First(&product, productID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	contract, err := buildProductContract(db, mediaService, product, true, true, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render product draft"})
		return
	}
	c.JSON(http.StatusOK, contract)
}

func buildProductContract(
	db *gorm.DB,
	mediaService *media.Service,
	source models.Product,
	preview bool,
	includeRelated bool,
	includeDraftMeta bool,
) (apicontract.Product, error) {
	var (
		draft    productCatalogDraft
		hasDraft bool
		err      error
	)

	if preview {
		draft, hasDraft, err = loadNormalizedProductDraft(db, source)
	} else {
		draft, err = loadPublishedProductCatalogData(db, source, true)
	}
	if err != nil {
		return apicontract.Product{}, err
	}

	draft, err = deriveCatalogMerchandising(draft)
	if err != nil {
		return apicontract.Product{}, err
	}

	brand, err := loadBrandContract(db, draft.BrandID)
	if err != nil {
		return apicontract.Product{}, err
	}

	attributeMeta, err := loadAttributeMetadata(db, draft.Attributes)
	if err != nil {
		return apicontract.Product{}, err
	}

	images, coverImage := contractProductImages(mediaService, source.ID, preview && hasDraft, draft.Images)
	related, err := loadRelatedContracts(db, mediaService, draft.RelatedIDs, source.Related, preview, includeRelated)
	if err != nil {
		return apicontract.Product{}, err
	}

	priceRange := buildProductPriceRange(draft)
	defaultVariantID := contractDefaultVariantID(source, draft, preview)
	defaultVariantSKU := optionalString(draft.DefaultVariantSKU)

	product := apicontract.Product{
		Attributes:        buildAttributeContracts(draft, attributeMeta),
		Brand:             brand,
		CoverImage:        coverImage,
		CreatedAt:         source.CreatedAt,
		DefaultVariantId:  defaultVariantID,
		DefaultVariantSku: defaultVariantSKU,
		DeletedAt:         toContractDeletedAt(source.DeletedAt),
		Description:       draft.Description,
		DraftUpdatedAt:    source.DraftUpdatedAt,
		Id:                int(source.ID),
		Images:            images,
		Name:              draft.Name,
		Options:           buildOptionContracts(draft),
		Price:             draft.Price,
		PriceRange:        priceRange,
		RelatedProducts:   related,
		Seo:               buildSEOContract(draft.SEO),
		Sku:               draft.SKU,
		Stock:             draft.Stock,
		Subtitle:          draft.Subtitle,
		UpdatedAt:         source.UpdatedAt,
		Variants:          buildVariantContracts(draft),
	}

	if includeDraftMeta {
		published := productIsPubliclyVisible(source)
		product.IsPublished = &published
		product.HasDraftChanges = &hasDraft
	}
	if !includeDraftMeta {
		product.DraftUpdatedAt = nil
	}

	return product, nil
}

func loadBrandContract(db *gorm.DB, brandID *uint) (*apicontract.Brand, error) {
	if brandID == nil {
		return nil, nil
	}

	var brand models.Brand
	if err := db.First(&brand, *brandID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &apicontract.Brand{
		Description: brand.Description,
		Id:          int(brand.ID),
		IsActive:    brand.IsActive,
		LogoMediaId: brand.LogoMediaID,
		Name:        brand.Name,
		Slug:        brand.Slug,
	}, nil
}

func loadAttributeMetadata(db *gorm.DB, values []productAttributeValueDraftData) (map[uint]models.ProductAttribute, error) {
	ids := make([]uint, 0, len(values))
	seen := map[uint]struct{}{}
	for _, value := range values {
		if value.Deleted || value.ProductAttributeID == 0 {
			continue
		}
		if _, exists := seen[value.ProductAttributeID]; exists {
			continue
		}
		seen[value.ProductAttributeID] = struct{}{}
		ids = append(ids, value.ProductAttributeID)
	}
	if len(ids) == 0 {
		return map[uint]models.ProductAttribute{}, nil
	}

	var attributes []models.ProductAttribute
	if err := db.Where("id IN ?", ids).Find(&attributes).Error; err != nil {
		return nil, err
	}

	byID := make(map[uint]models.ProductAttribute, len(attributes))
	for _, attribute := range attributes {
		byID[attribute.ID] = attribute
	}

	for _, id := range ids {
		if _, exists := byID[id]; !exists {
			return nil, fmt.Errorf("product attribute %d not found", id)
		}
	}

	return byID, nil
}

func contractProductImages(
	mediaService *media.Service,
	productID uint,
	useDraftRole bool,
	fallback []string,
) ([]string, *string) {
	images := append([]string(nil), fallback...)
	role := media.RoleProductImage
	if useDraftRole {
		role = media.RoleProductDraftImage
	}

	if mediaService != nil {
		mediaURLs, err := mediaService.ProductMediaURLsByRole(productID, role)
		if err == nil && len(mediaURLs) > 0 {
			images = mediaURLs
		}
	}

	var coverImage *string
	if len(images) > 0 {
		coverImage = &images[0]
	}
	return images, coverImage
}

func loadRelatedContracts(
	db *gorm.DB,
	mediaService *media.Service,
	relatedIDs []uint,
	relatedProducts []models.Product,
	preview bool,
	includeRelated bool,
) ([]apicontract.RelatedProduct, error) {
	if !includeRelated {
		return []apicontract.RelatedProduct{}, nil
	}

	var source []models.Product
	if preview {
		loaded, err := relatedProductsByIDs(db, relatedIDs)
		if err != nil {
			return nil, err
		}
		source = loaded
	} else {
		source = append([]models.Product(nil), relatedProducts...)
	}

	related := make([]apicontract.RelatedProduct, 0, len(source))
	for i := range source {
		role := media.RoleProductImage
		fallback := source[i].Images
		if preview && productHasDraft(source[i]) {
			role = media.RoleProductDraftImage
		}
		applyProductMediaWithRole(&source[i], mediaService, role, fallback)
		related = append(related, toContractRelatedProduct(source[i]))
	}

	return related, nil
}

func buildOptionContracts(draft productCatalogDraft) []apicontract.ProductOption {
	options := make([]apicontract.ProductOption, 0, len(draft.Options))
	for _, option := range draft.Options {
		if option.Deleted {
			continue
		}

		entry := apicontract.ProductOption{
			DisplayType: option.DisplayType,
			Name:        option.Name,
			Position:    option.Position,
			Values:      make([]apicontract.ProductOptionValue, 0, len(option.Values)),
		}
		if option.SourceID != nil {
			id := int(*option.SourceID)
			entry.Id = &id
		}
		for _, value := range option.Values {
			if value.Deleted {
				continue
			}
			valueEntry := apicontract.ProductOptionValue{
				Position: value.Position,
				Value:    value.Value,
			}
			if value.SourceID != nil {
				id := int(*value.SourceID)
				valueEntry.Id = &id
			}
			entry.Values = append(entry.Values, valueEntry)
		}
		options = append(options, entry)
	}
	return options
}

func buildVariantContracts(draft productCatalogDraft) []apicontract.ProductVariant {
	variants := make([]apicontract.ProductVariant, 0, len(draft.Variants))
	for _, variant := range draft.Variants {
		if variant.Deleted {
			continue
		}

		entry := apicontract.ProductVariant{
			CompareAtPrice: variant.CompareAtPrice,
			HeightCm:       variant.HeightCm,
			IsPublished:    variant.IsPublished,
			LengthCm:       variant.LengthCm,
			Position:       variant.Position,
			Price:          variant.Price,
			Selections:     make([]apicontract.ProductVariantSelection, 0, len(variant.Selections)),
			Sku:            variant.SKU,
			Stock:          variant.Stock,
			Title:          variant.Title,
			WeightGrams:    variant.WeightGrams,
			WidthCm:        variant.WidthCm,
		}
		if variant.SourceID != nil {
			id := int(*variant.SourceID)
			entry.Id = &id
		}
		for _, selection := range variant.Selections {
			selectionEntry := apicontract.ProductVariantSelection{
				OptionName:  selection.OptionName,
				OptionValue: selection.OptionValue,
				Position:    selection.Position,
			}
			if selection.SourceOptionValueID != nil {
				id := int(*selection.SourceOptionValueID)
				selectionEntry.ProductOptionValueId = &id
			}
			entry.Selections = append(entry.Selections, selectionEntry)
		}
		variants = append(variants, entry)
	}
	return variants
}

func buildAttributeContracts(
	draft productCatalogDraft,
	attributeMeta map[uint]models.ProductAttribute,
) []apicontract.ProductAttributeValue {
	values := make([]apicontract.ProductAttributeValue, 0, len(draft.Attributes))
	for _, attribute := range draft.Attributes {
		if attribute.Deleted {
			continue
		}
		meta := attributeMeta[attribute.ProductAttributeID]
		values = append(values, apicontract.ProductAttributeValue{
			BooleanValue:       attribute.BooleanValue,
			EnumValue:          attribute.EnumValue,
			Key:                meta.Key,
			NumberValue:        attribute.NumberValue,
			Position:           attribute.Position,
			ProductAttributeId: int(attribute.ProductAttributeID),
			Slug:               meta.Slug,
			TextValue:          attribute.TextValue,
			Type:               meta.Type,
		})
	}
	return values
}

func buildSEOContract(seo productSEODraftData) apicontract.ProductSEO {
	return apicontract.ProductSEO{
		CanonicalPath:  seo.CanonicalPath,
		Description:    seo.Description,
		Noindex:        boolPtr(seo.NoIndex),
		OgImageMediaId: seo.OgImageMediaID,
		Title:          seo.Title,
	}
}

func buildProductPriceRange(draft productCatalogDraft) apicontract.ProductPriceRange {
	minPrice := draft.Price
	maxPrice := draft.Price
	hasVariant := false

	for _, variant := range draft.Variants {
		if variant.Deleted {
			continue
		}
		if !hasVariant {
			minPrice = variant.Price
			maxPrice = variant.Price
			hasVariant = true
			continue
		}
		if variant.Price < minPrice {
			minPrice = variant.Price
		}
		if variant.Price > maxPrice {
			maxPrice = variant.Price
		}
	}

	return apicontract.ProductPriceRange{
		Max: maxPrice,
		Min: minPrice,
	}
}

func contractDefaultVariantID(_ models.Product, draft productCatalogDraft, preview bool) *int {
	if !preview {
		for _, variant := range draft.Variants {
			if variant.Deleted || variant.SKU != draft.DefaultVariantSKU || variant.SourceID == nil {
				continue
			}
			id := int(*variant.SourceID)
			return &id
		}
		for _, variant := range draft.Variants {
			if variant.Deleted || variant.SourceID == nil {
				continue
			}
			id := int(*variant.SourceID)
			return &id
		}
		return nil
	}

	for _, variant := range draft.Variants {
		if variant.Deleted || variant.SKU != draft.DefaultVariantSKU || variant.SourceID == nil {
			continue
		}
		id := int(*variant.SourceID)
		return &id
	}
	return nil
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}
