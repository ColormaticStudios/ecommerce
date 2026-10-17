package httpapi

import (
	"context"

	"ecommerce/internal/media"
	"ecommerce/internal/requestctx"
	localizationservice "ecommerce/internal/services/localization"
	"ecommerce/models"
)

type publicMetadataKey struct{}

type publicMetadata struct {
	localized map[localizationservice.EntityReference]localizationservice.EntityResolution
	images    map[uint][]string
	logos     map[uint][]string
}

// A public page resolves its entity copy and published media together. The
// shared resolver retains field fallback and entity existence validation;
// this context is request-owned and never caches catalog data across requests.
func (e *CatalogEndpoints) publicMetadataContext(ctx context.Context, products []models.Product) (context.Context, error) {
	missing := []uint{}
	for _, product := range products {
		if product.ID != 0 && product.Related == nil && product.Categories == nil && product.Variants == nil {
			missing = append(missing, product.ID)
		}
	}
	if len(missing) > 0 {
		var loaded []models.Product
		if err := e.db.WithContext(ctx).Preload("Related").Preload("Categories").Preload("Variants").Where("id IN ?", missing).Find(&loaded).Error; err != nil {
			return ctx, err
		}
		byID := map[uint]models.Product{}
		for _, product := range loaded {
			byID[product.ID] = product
		}
		for i, product := range products {
			if loaded, ok := byID[product.ID]; ok {
				products[i] = loaded
			}
		}
	}
	refs := []localizationservice.EntityReference{}
	ids, brandIDs := []uint{}, []uint{}
	seen := map[localizationservice.EntityReference]bool{}
	add := func(kind localizationservice.EntityType, id uint) {
		ref := localizationservice.EntityReference{EntityType: kind, EntityID: id}
		if !seen[ref] {
			seen[ref] = true
			refs = append(refs, ref)
		}
	}
	for _, product := range products {
		ids = append(ids, product.ID)
		add(localizationservice.EntityTypeProduct, product.ID)
		if product.Brand != nil {
			brandIDs = append(brandIDs, product.Brand.ID)
			add(localizationservice.EntityTypeBrand, product.Brand.ID)
		}
		for _, category := range product.Categories {
			add(localizationservice.EntityTypeCategory, category.ID)
		}
		for _, related := range product.Related {
			add(localizationservice.EntityTypeProduct, related.ID)
		}
		for _, variant := range product.Variants {
			if variant.IsPublished {
				add(localizationservice.EntityTypeProductVariant, variant.ID)
			}
		}
	}
	input := localizationservice.ResolutionInput{}
	if resolution, ok := requestctx.LocaleResolutionFrom(ctx); ok {
		input.ExplicitLocale = resolution.ResolvedLocale
	}
	localized, err := e.localization.ResolveEntities(ctx, refs, input)
	if err != nil {
		return ctx, err
	}
	metadata := &publicMetadata{localized: localized}
	if e.media != nil {
		// Media is optional presentation data. Match single-product delivery's
		// controlled fallback on lookup failure rather than failing the page.
		metadata.images, _ = e.media.OwnerMediaURLs(ctx, media.OwnerTypeProduct, ids, media.RoleProductImage)
		metadata.logos, _ = e.media.OwnerMediaURLs(ctx, media.OwnerTypeBrand, brandIDs, media.RoleBrandLogo)
	}
	return context.WithValue(ctx, publicMetadataKey{}, metadata), nil
}
