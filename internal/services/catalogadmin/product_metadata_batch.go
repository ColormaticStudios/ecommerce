package catalogadmin

import (
	"context"
	"ecommerce/internal/apicontract"
	"ecommerce/internal/media"
	"ecommerce/models"
	"gorm.io/gorm"
)

// ProductInputs hydrates a result page with a fixed set of association queries.
// Drafts remain authoritative; live metadata uses the same conversion as exports.
func (s *Service) ProductInputs(ctx context.Context, ids []uint) (map[uint]apicontract.ProductUpsertInput, error) {
	out := map[uint]apicontract.ProductUpsertInput{}
	if len(ids) == 0 {
		return out, nil
	}
	db := s.db.WithContext(ctx)
	var products []models.Product
	if err := db.Preload("Categories").Preload("Related").Where("id IN ?", ids).Find(&products).Error; err != nil {
		return nil, err
	}
	if len(products) != len(ids) {
		return nil, gorm.ErrRecordNotFound
	}
	liveIDs, draftIDs := []uint{}, []uint{}
	for _, p := range products {
		if p.DraftUpdatedAt != nil {
			draftIDs = append(draftIDs, p.ID)
		} else {
			liveIDs = append(liveIDs, p.ID)
		}
	}
	if len(draftIDs) > 0 {
		var drafts []models.ProductDraft
		if err := completeDraftQuery(db).Where("product_id IN ?", draftIDs).Find(&drafts).Error; err != nil {
			return nil, err
		}
		if len(drafts) != len(draftIDs) {
			return nil, gorm.ErrRecordNotFound
		}
		for _, d := range drafts {
			input, err := inputFromDraft(d)
			if err != nil {
				return nil, err
			}
			out[d.ProductID] = input
		}
	}
	if len(liveIDs) == 0 {
		return out, nil
	}
	ordered := func(tx *gorm.DB) *gorm.DB { return tx.Order("position ASC, id ASC") }
	var options []models.ProductOption
	if err := db.Preload("Values", ordered).Where("product_id IN ?", liveIDs).Order("position ASC, id ASC").Find(&options).Error; err != nil {
		return nil, err
	}
	var variants []models.ProductVariant
	if err := db.Preload("OptionValueLinks", func(tx *gorm.DB) *gorm.DB { return tx.Order("id ASC") }).Where("product_id IN ?", liveIDs).Order("position ASC, id ASC").Find(&variants).Error; err != nil {
		return nil, err
	}
	var attrs []models.ProductAttributeValue
	if err := db.Where("product_id IN ?", liveIDs).Order("position ASC, id ASC").Find(&attrs).Error; err != nil {
		return nil, err
	}
	var seo []models.SEOMetadata
	if err := db.Where("entity_type = ? AND entity_id IN ?", productSEOEntityType, liveIDs).Find(&seo).Error; err != nil {
		return nil, err
	}
	values := []uint{}
	for _, v := range variants {
		for _, link := range v.OptionValueLinks {
			values = append(values, link.ProductOptionValueID)
		}
	}
	valueMap, err := loadLiveOptionValueMap(db, values)
	if err != nil {
		return nil, err
	}
	optionsByID := map[uint][]models.ProductOption{}
	for _, v := range options {
		optionsByID[v.ProductID] = append(optionsByID[v.ProductID], v)
	}
	variantsByID := map[uint][]models.ProductVariant{}
	for _, v := range variants {
		variantsByID[v.ProductID] = append(variantsByID[v.ProductID], v)
	}
	attrsByID := map[uint][]models.ProductAttributeValue{}
	for _, v := range attrs {
		attrsByID[v.ProductID] = append(attrsByID[v.ProductID], v)
	}
	seoByID := map[uint]models.SEOMetadata{}
	for _, v := range seo {
		seoByID[v.EntityID] = v
	}
	mediaURLs := map[uint][]string{}
	if s.media != nil {
		if values, err := s.media.OwnerMediaURLs(ctx, media.OwnerTypeProduct, liveIDs, media.RoleProductImage); err == nil {
			mediaURLs = values
		}
	}
	for _, p := range products {
		if p.DraftUpdatedAt != nil {
			continue
		}
		input, err := inputFromLiveProduct(p, optionsByID[p.ID], variantsByID[p.ID], attrsByID[p.ID], seoByID[p.ID], valueMap, nil)
		if err != nil {
			return nil, err
		}
		if len(mediaURLs[p.ID]) > 0 {
			input.Images = mediaURLs[p.ID]
		}
		out[p.ID] = input
	}
	return out, nil
}
