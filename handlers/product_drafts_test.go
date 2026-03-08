package handlers

import (
	"testing"
	"time"

	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizedProductDraftRoundTrip(t *testing.T) {
	db := newTestDB(t)

	related := models.Product{
		SKU:         "REL-1",
		Name:        "Related",
		Description: "related",
		Price:       models.MoneyFromFloat(5),
		Stock:       3,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&related).Error)

	product := models.Product{
		SKU:         "BASE-1",
		Name:        "Base",
		Description: "base",
		Price:       models.MoneyFromFloat(10),
		Stock:       2,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&product).Error)

	brand := models.Brand{Name: "Acme", Slug: "acme"}
	require.NoError(t, db.Create(&brand).Error)

	attribute := models.ProductAttribute{
		Key:        "material",
		Slug:       "material",
		Type:       "text",
		Filterable: true,
	}
	require.NoError(t, db.Create(&attribute).Error)

	cotton := "Cotton"
	seoTitle := "SEO title"
	canonical := "/products/base-1"
	colorPrice := 12.5

	draft := productCatalogDraft{
		SKU:         "BASE-1-DRAFT",
		Name:        "Draft Product",
		Description: "draft description",
		Price:       29.99,
		Stock:       7,
		Images:      []string{"img-a", "img-b", "img-a"},
		RelatedIDs:  []uint{related.ID, related.ID},
		BrandID:     &brand.ID,
		Options: []productOptionDraftData{
			{
				Name:        "Color",
				DisplayType: "swatch",
				Values: []productOptionValueDraftData{
					{Value: "Blue", Position: 2},
					{Value: "Red", Position: 1},
				},
			},
		},
		Variants: []productVariantDraftData{
			{
				SKU:         "VAR-RED",
				Title:       "Red Variant",
				Price:       colorPrice,
				Stock:       5,
				Position:    2,
				IsPublished: true,
				Selections: []productVariantSelectionDraftData{
					{OptionName: "Color", OptionValue: "Red"},
				},
			},
			{
				SKU:         "VAR-BLUE",
				Title:       "Blue Variant",
				Price:       11.5,
				Stock:       4,
				Position:    1,
				IsPublished: true,
				Selections: []productVariantSelectionDraftData{
					{OptionName: "Color", OptionValue: "Blue"},
				},
			},
		},
		Attributes: []productAttributeValueDraftData{
			{
				ProductAttributeID: attribute.ID,
				TextValue:          &cotton,
			},
		},
		SEO: productSEODraftData{
			Title:         &seoTitle,
			CanonicalPath: &canonical,
		},
		DefaultVariantSKU: "VAR-RED",
	}

	now := time.Now().UTC()
	require.NoError(t, saveNormalizedProductDraft(db, product, draft, now))

	loaded, hasDraft, err := loadNormalizedProductDraft(db, product)
	require.NoError(t, err)
	require.True(t, hasDraft)

	assert.Equal(t, "BASE-1-DRAFT", loaded.SKU)
	assert.Equal(t, []string{"img-a", "img-b"}, loaded.Images)
	assert.Equal(t, []uint{related.ID}, loaded.RelatedIDs)
	require.Len(t, loaded.Options, 1)
	require.Len(t, loaded.Options[0].Values, 2)
	assert.Equal(t, "Red", loaded.Options[0].Values[0].Value)
	require.Len(t, loaded.Variants, 2)
	assert.Equal(t, "VAR-BLUE", loaded.Variants[0].SKU)
	assert.Equal(t, canonical, *loaded.SEO.CanonicalPath)
	assert.Equal(t, "VAR-RED", loaded.DefaultVariantSKU)

	var stored models.Product
	require.NoError(t, db.First(&stored, product.ID).Error)
	require.NotNil(t, stored.DraftUpdatedAt)
}

func TestPublishNormalizedProductDraftReplacesLiveCatalogState(t *testing.T) {
	db := newTestDB(t)

	product := models.Product{
		SKU:         "LIVE-1",
		Name:        "Live",
		Description: "live",
		Price:       models.MoneyFromFloat(10),
		Stock:       1,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&product).Error)

	oldOption := models.ProductOption{ProductID: product.ID, Name: "Old", Position: 1, DisplayType: "select"}
	require.NoError(t, db.Create(&oldOption).Error)
	oldValue := models.ProductOptionValue{ProductOptionID: oldOption.ID, Value: "Legacy", Position: 1}
	require.NoError(t, db.Create(&oldValue).Error)
	oldVariant := models.ProductVariant{
		ProductID:   product.ID,
		SKU:         "OLD-SKU",
		Title:       "Legacy Variant",
		Price:       models.MoneyFromFloat(9),
		Stock:       2,
		Position:    1,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&oldVariant).Error)
	require.NoError(t, db.Create(&models.ProductVariantOptionValue{
		ProductVariantID:     oldVariant.ID,
		ProductOptionValueID: oldValue.ID,
	}).Error)
	require.NoError(t, db.Create(&models.SEOMetadata{
		EntityType:    seoEntityTypeProduct,
		EntityID:      product.ID,
		CanonicalPath: stringPtr("/old"),
	}).Error)

	draft := productCatalogDraft{
		SKU:         "LIVE-1-NEW",
		Name:        "Published Name",
		Description: "published description",
		Price:       20,
		Stock:       8,
		Options: []productOptionDraftData{
			{
				Name: "Size",
				Values: []productOptionValueDraftData{
					{Value: "S"},
					{Value: "M"},
				},
			},
		},
		Variants: []productVariantDraftData{
			{
				SKU:         "SIZE-S",
				Title:       "Small",
				Price:       20,
				Stock:       3,
				Selections:  []productVariantSelectionDraftData{{OptionName: "Size", OptionValue: "S"}},
				IsPublished: true,
			},
			{
				SKU:         "SIZE-M",
				Title:       "Medium",
				Price:       21,
				Stock:       5,
				Selections:  []productVariantSelectionDraftData{{OptionName: "Size", OptionValue: "M"}},
				IsPublished: true,
			},
		},
		SEO: productSEODraftData{
			CanonicalPath: stringPtr("/new"),
			NoIndex:       true,
		},
		DefaultVariantSKU: "SIZE-M",
	}

	require.NoError(t, saveNormalizedProductDraft(db, product, draft, time.Now().UTC()))
	require.NoError(t, publishNormalizedProductDraft(db, &product, draft))

	var reloaded models.Product
	require.NoError(t, db.First(&reloaded, product.ID).Error)
	assert.Equal(t, "LIVE-1-NEW", reloaded.SKU)
	assert.Equal(t, "Published Name", reloaded.Name)
	assert.Nil(t, reloaded.DraftUpdatedAt)
	require.NotNil(t, reloaded.DefaultVariantID)

	var variants []models.ProductVariant
	require.NoError(t, db.Where("product_id = ?", product.ID).Order("position asc").Find(&variants).Error)
	require.Len(t, variants, 2)
	assert.Equal(t, "SIZE-S", variants[0].SKU)

	var seo models.SEOMetadata
	require.NoError(t, db.Where("entity_type = ? AND entity_id = ?", seoEntityTypeProduct, product.ID).First(&seo).Error)
	assert.Equal(t, "/new", *seo.CanonicalPath)
	assert.True(t, seo.NoIndex)

	var draftCount int64
	require.NoError(t, db.Model(&models.ProductDraft{}).Where("product_id = ?", product.ID).Count(&draftCount).Error)
	assert.Zero(t, draftCount)
}

func TestPublishNormalizedProductDraftSkipsDeletedChildren(t *testing.T) {
	db := newTestDB(t)

	product := models.Product{
		SKU:         "LIVE-DEL-1",
		Name:        "Live",
		Description: "live",
		Price:       models.MoneyFromFloat(10),
		Stock:       1,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&product).Error)

	attribute := models.ProductAttribute{
		Key:  "material",
		Slug: "material",
		Type: "text",
	}
	require.NoError(t, db.Create(&attribute).Error)

	cotton := "Cotton"
	draft := productCatalogDraft{
		SKU:         "LIVE-DEL-1",
		Name:        "Live",
		Description: "live",
		Price:       10,
		Stock:       1,
		Options: []productOptionDraftData{
			{
				Name: "Size",
				Values: []productOptionValueDraftData{
					{Value: "S"},
					{Value: "M", Deleted: true},
				},
			},
			{
				Name:    "Hidden",
				Deleted: true,
				Values: []productOptionValueDraftData{
					{Value: "Ghost"},
				},
			},
		},
		Variants: []productVariantDraftData{
			{
				SKU:         "SIZE-S",
				Title:       "Small",
				Price:       10,
				Stock:       1,
				IsPublished: true,
				Selections:  []productVariantSelectionDraftData{{OptionName: "Size", OptionValue: "S"}},
			},
			{
				SKU:         "SIZE-M",
				Title:       "Medium",
				Price:       11,
				Stock:       1,
				IsPublished: true,
				Deleted:     true,
				Selections:  []productVariantSelectionDraftData{{OptionName: "Size", OptionValue: "M"}},
			},
		},
		Attributes: []productAttributeValueDraftData{
			{
				ProductAttributeID: attribute.ID,
				TextValue:          &cotton,
			},
			{
				ProductAttributeID: attribute.ID,
				TextValue:          stringPtr("Deleted"),
				Deleted:            true,
			},
		},
		DefaultVariantSKU: "SIZE-M",
	}

	require.NoError(t, saveNormalizedProductDraft(db, product, draft, time.Now().UTC()))
	require.NoError(t, publishNormalizedProductDraft(db, &product, draft))

	var options []models.ProductOption
	require.NoError(t, db.Where("product_id = ?", product.ID).Order("position asc").Find(&options).Error)
	require.Len(t, options, 1)
	assert.Equal(t, "Size", options[0].Name)

	var values []models.ProductOptionValue
	require.NoError(t, db.Where("product_option_id = ?", options[0].ID).Order("position asc").Find(&values).Error)
	require.Len(t, values, 1)
	assert.Equal(t, "S", values[0].Value)

	var variants []models.ProductVariant
	require.NoError(t, db.Where("product_id = ?", product.ID).Order("position asc").Find(&variants).Error)
	require.Len(t, variants, 1)
	assert.Equal(t, "SIZE-S", variants[0].SKU)

	var reloaded models.Product
	require.NoError(t, db.First(&reloaded, product.ID).Error)
	require.NotNil(t, reloaded.DefaultVariantID)
	assert.Equal(t, variants[0].ID, *reloaded.DefaultVariantID)

	var attributes []models.ProductAttributeValue
	require.NoError(t, db.Where("product_id = ?", product.ID).Order("position asc").Find(&attributes).Error)
	require.Len(t, attributes, 1)
	assert.Equal(t, cotton, *attributes[0].TextValue)
}

func TestBuildProductContractExcludesUnpublishedVariants(t *testing.T) {
	db := newTestDB(t)

	product := models.Product{
		SKU:         "PUB-1",
		Name:        "Published",
		Description: "published",
		Price:       models.MoneyFromFloat(10),
		Stock:       1,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&product).Error)

	option := models.ProductOption{ProductID: product.ID, Name: "Color", Position: 1, DisplayType: "swatch"}
	require.NoError(t, db.Create(&option).Error)
	hiddenValue := models.ProductOptionValue{ProductOptionID: option.ID, Value: "Hidden", Position: 1}
	require.NoError(t, db.Create(&hiddenValue).Error)
	liveValue := models.ProductOptionValue{ProductOptionID: option.ID, Value: "Blue", Position: 2}
	require.NoError(t, db.Create(&liveValue).Error)

	hiddenVariant := models.ProductVariant{
		ProductID:   product.ID,
		SKU:         "PUB-1-HIDDEN",
		Title:       "Hidden",
		Price:       models.MoneyFromFloat(11),
		Stock:       0,
		Position:    1,
		IsPublished: false,
	}
	require.NoError(t, db.Select("*").Create(&hiddenVariant).Error)
	require.NoError(t, db.Model(&hiddenVariant).Update("is_published", false).Error)
	require.NoError(t, db.First(&hiddenVariant, hiddenVariant.ID).Error)
	assert.False(t, hiddenVariant.IsPublished)
	require.NoError(t, db.Create(&models.ProductVariantOptionValue{
		ProductVariantID:     hiddenVariant.ID,
		ProductOptionValueID: hiddenValue.ID,
	}).Error)

	liveVariant := models.ProductVariant{
		ProductID:   product.ID,
		SKU:         "PUB-1-BLUE",
		Title:       "Blue",
		Price:       models.MoneyFromFloat(13),
		Stock:       7,
		Position:    2,
		IsPublished: true,
	}
	require.NoError(t, db.Select("*").Create(&liveVariant).Error)
	require.NoError(t, db.Create(&models.ProductVariantOptionValue{
		ProductVariantID:     liveVariant.ID,
		ProductOptionValueID: liveValue.ID,
	}).Error)
	require.NoError(t, db.Model(&product).Update("default_variant_id", hiddenVariant.ID).Error)
	require.NoError(t, db.First(&product, product.ID).Error)

	contract, err := buildProductContract(db, nil, product, false, false, false)
	require.NoError(t, err)

	require.NotNil(t, contract.DefaultVariantId)
	assert.Equal(t, int(liveVariant.ID), *contract.DefaultVariantId)
	require.NotNil(t, contract.DefaultVariantSku)
	assert.Equal(t, liveVariant.SKU, *contract.DefaultVariantSku)
	require.Len(t, contract.Variants, 1)
	require.NotNil(t, contract.Variants[0].Id)
	assert.Equal(t, int(liveVariant.ID), *contract.Variants[0].Id)
	assert.Equal(t, liveVariant.SKU, contract.Variants[0].Sku)
	require.Len(t, contract.Options, 1)
	require.Len(t, contract.Options[0].Values, 1)
	assert.Equal(t, "Blue", contract.Options[0].Values[0].Value)
	assert.Equal(t, liveVariant.Stock, contract.Stock)
}

func TestLoadPublishedProductCatalogDataDoesNotSynthesizeVariantWhenAllPublishedVariantsAreHidden(t *testing.T) {
	db := newTestDB(t)

	product := models.Product{
		SKU:         "PUB-ONLY-HIDDEN",
		Name:        "Published Hidden",
		Description: "published hidden",
		Price:       models.MoneyFromFloat(10),
		Stock:       1,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&product).Error)

	option := models.ProductOption{ProductID: product.ID, Name: "Color", Position: 1, DisplayType: "swatch"}
	require.NoError(t, db.Create(&option).Error)
	hiddenValue := models.ProductOptionValue{ProductOptionID: option.ID, Value: "Hidden", Position: 1}
	require.NoError(t, db.Create(&hiddenValue).Error)

	hiddenVariant := models.ProductVariant{
		ProductID:   product.ID,
		SKU:         "PUB-ONLY-HIDDEN-VAR",
		Title:       "Hidden",
		Price:       models.MoneyFromFloat(11),
		Stock:       0,
		Position:    1,
		IsPublished: false,
	}
	require.NoError(t, db.Select("*").Create(&hiddenVariant).Error)
	require.NoError(t, db.Model(&hiddenVariant).Update("is_published", false).Error)
	require.NoError(t, db.Create(&models.ProductVariantOptionValue{
		ProductVariantID:     hiddenVariant.ID,
		ProductOptionValueID: hiddenValue.ID,
	}).Error)
	require.NoError(t, db.Model(&product).Update("default_variant_id", hiddenVariant.ID).Error)
	require.NoError(t, db.First(&product, product.ID).Error)

	draft, err := loadPublishedProductCatalogData(db, product, true)
	require.NoError(t, err)

	assert.Empty(t, draft.DefaultVariantSKU)
	assert.Empty(t, draft.Variants)
	assert.Empty(t, draft.Options)
}

func TestPublishNormalizedProductDraftPreservesExistingVariantIDs(t *testing.T) {
	db := newTestDB(t)

	product := models.Product{
		SKU:         "LIVE-KEEP-1",
		Name:        "Live",
		Description: "live",
		Price:       models.MoneyFromFloat(10),
		Stock:       1,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&product).Error)

	option := models.ProductOption{ProductID: product.ID, Name: "Size", Position: 1, DisplayType: "select"}
	require.NoError(t, db.Create(&option).Error)
	smallValue := models.ProductOptionValue{ProductOptionID: option.ID, Value: "S", Position: 1}
	mediumValue := models.ProductOptionValue{ProductOptionID: option.ID, Value: "M", Position: 2}
	require.NoError(t, db.Create(&smallValue).Error)
	require.NoError(t, db.Create(&mediumValue).Error)

	smallVariant := models.ProductVariant{
		ProductID:   product.ID,
		SKU:         "KEEP-S",
		Title:       "Small",
		Price:       models.MoneyFromFloat(10),
		Stock:       2,
		Position:    1,
		IsPublished: true,
	}
	mediumVariant := models.ProductVariant{
		ProductID:   product.ID,
		SKU:         "KEEP-M",
		Title:       "Medium",
		Price:       models.MoneyFromFloat(11),
		Stock:       3,
		Position:    2,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&smallVariant).Error)
	require.NoError(t, db.Create(&mediumVariant).Error)
	require.NoError(t, db.Create(&models.ProductVariantOptionValue{
		ProductVariantID:     smallVariant.ID,
		ProductOptionValueID: smallValue.ID,
	}).Error)
	require.NoError(t, db.Create(&models.ProductVariantOptionValue{
		ProductVariantID:     mediumVariant.ID,
		ProductOptionValueID: mediumValue.ID,
	}).Error)
	require.NoError(t, db.Model(&product).Update("default_variant_id", mediumVariant.ID).Error)

	user := models.User{
		Subject:  "keep-user",
		Username: "keep-user",
		Email:    "keep@example.com",
		Role:     "customer",
		Currency: "USD",
	}
	require.NoError(t, db.Create(&user).Error)
	cart := models.Cart{UserID: user.ID}
	require.NoError(t, db.Create(&cart).Error)
	require.NoError(t, db.Create(&models.CartItem{CartID: cart.ID, ProductVariantID: smallVariant.ID, Quantity: 1}).Error)
	order := models.Order{UserID: user.ID, Total: models.MoneyFromFloat(11), Status: "PENDING"}
	require.NoError(t, db.Create(&order).Error)
	require.NoError(t, db.Create(&models.OrderItem{
		OrderID:          order.ID,
		ProductVariantID: mediumVariant.ID,
		VariantSKU:       mediumVariant.SKU,
		VariantTitle:     mediumVariant.Title,
		Quantity:         1,
		Price:            mediumVariant.Price,
	}).Error)

	incoming := productCatalogDraft{
		SKU:         "LIVE-KEEP-1",
		Name:        "Live updated",
		Description: "live updated",
		Price:       10,
		Stock:       1,
		Options: []productOptionDraftData{
			{
				Name: "Size",
				Values: []productOptionValueDraftData{
					{Value: "S", Position: 1},
					{Value: "M", Position: 2},
				},
			},
		},
		Variants: []productVariantDraftData{
			{
				SKU:         "KEEP-S-UPDATED",
				Title:       "Small Updated",
				Price:       12,
				Stock:       5,
				Position:    1,
				IsPublished: true,
				Selections:  []productVariantSelectionDraftData{{OptionName: "Size", OptionValue: "S", Position: 1}},
			},
			{
				SKU:         "KEEP-M-UPDATED",
				Title:       "Medium Updated",
				Price:       14,
				Stock:       6,
				Position:    2,
				IsPublished: true,
				Selections:  []productVariantSelectionDraftData{{OptionName: "Size", OptionValue: "M", Position: 1}},
			},
		},
		DefaultVariantSKU: "KEEP-M-UPDATED",
	}

	require.NoError(t, saveEditableProductCatalogDraft(db, &product, incoming))
	savedDraft, hasDraft, err := loadNormalizedProductDraft(db, product)
	require.NoError(t, err)
	require.True(t, hasDraft)
	require.Len(t, savedDraft.Variants, 2)
	require.NotNil(t, savedDraft.Variants[0].SourceID)
	require.NotNil(t, savedDraft.Variants[1].SourceID)
	assert.Equal(t, smallVariant.ID, *savedDraft.Variants[0].SourceID)
	assert.Equal(t, mediumVariant.ID, *savedDraft.Variants[1].SourceID)

	require.NoError(t, publishNormalizedProductDraft(db, &product, savedDraft))

	var reloadedVariants []models.ProductVariant
	require.NoError(t, db.Where("product_id = ?", product.ID).Order("position asc").Find(&reloadedVariants).Error)
	require.Len(t, reloadedVariants, 2)
	assert.Equal(t, smallVariant.ID, reloadedVariants[0].ID)
	assert.Equal(t, "KEEP-S-UPDATED", reloadedVariants[0].SKU)
	assert.Equal(t, mediumVariant.ID, reloadedVariants[1].ID)
	assert.Equal(t, "KEEP-M-UPDATED", reloadedVariants[1].SKU)

	var reloadedCartItem models.CartItem
	require.NoError(t, db.Preload("ProductVariant").First(&reloadedCartItem).Error)
	assert.Equal(t, smallVariant.ID, reloadedCartItem.ProductVariant.ID)
	assert.Equal(t, "KEEP-S-UPDATED", reloadedCartItem.ProductVariant.SKU)

	var reloadedOrderItem models.OrderItem
	require.NoError(t, db.Preload("ProductVariant").First(&reloadedOrderItem).Error)
	assert.Equal(t, mediumVariant.ID, reloadedOrderItem.ProductVariant.ID)
	assert.Equal(t, "KEEP-M-UPDATED", reloadedOrderItem.ProductVariant.SKU)
}

func TestDiscardNormalizedProductDraftRestoresUnpublishedSnapshot(t *testing.T) {
	db := newTestDB(t)

	product := models.Product{
		SKU:         "UNPUB-1",
		Name:        "Base",
		Description: "base",
		Price:       models.MoneyFromFloat(8),
		Stock:       2,
		IsPublished: false,
	}
	require.NoError(t, db.Create(&product).Error)

	liveOption := models.ProductOption{ProductID: product.ID, Name: "Color", Position: 1, DisplayType: "select"}
	require.NoError(t, db.Create(&liveOption).Error)
	liveValue := models.ProductOptionValue{ProductOptionID: liveOption.ID, Value: "Black", Position: 1}
	require.NoError(t, db.Create(&liveValue).Error)

	modified := productCatalogDraft{
		SKU:         "UNPUB-1-MOD",
		Name:        "Modified",
		Description: "modified",
		Price:       14,
		Stock:       9,
		Options: []productOptionDraftData{
			{
				Name: "Color",
				Values: []productOptionValueDraftData{
					{Value: "White"},
				},
			},
		},
	}

	require.NoError(t, saveNormalizedProductDraft(db, product, modified, time.Now().UTC()))
	require.NoError(t, discardNormalizedProductDraft(db, &product))

	loaded, hasDraft, err := loadNormalizedProductDraft(db, product)
	require.NoError(t, err)
	_ = hasDraft
	assert.Equal(t, "UNPUB-1", loaded.SKU)
	assert.Equal(t, "Base", loaded.Name)
	require.Len(t, loaded.Options, 1)
	require.Len(t, loaded.Options[0].Values, 1)
	assert.Equal(t, "Black", loaded.Options[0].Values[0].Value)
}

func TestValidateProductCatalogDraftRejectsDuplicateCanonicalPath(t *testing.T) {
	db := newTestDB(t)

	existing := models.Product{
		SKU:         "CANONICAL-EXISTING",
		Name:        "Existing",
		Description: "existing",
		Price:       models.MoneyFromFloat(10),
		Stock:       1,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&existing).Error)
	require.NoError(t, db.Create(&models.SEOMetadata{
		EntityType:    seoEntityTypeProduct,
		EntityID:      existing.ID,
		CanonicalPath: stringPtr("/products/shared-path"),
	}).Error)

	target := models.Product{
		SKU:         "CANONICAL-TARGET",
		Name:        "Target",
		Description: "target",
		Price:       models.MoneyFromFloat(12),
		Stock:       2,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&target).Error)

	draft := productCatalogDraft{
		SKU:         target.SKU,
		Name:        target.Name,
		Description: target.Description,
		Variants: []productVariantDraftData{
			{
				SKU:         "CANONICAL-TARGET-DEFAULT",
				Title:       "Default",
				Price:       12,
				Stock:       2,
				IsPublished: true,
			},
		},
		SEO: productSEODraftData{
			CanonicalPath: stringPtr("products/shared-path"),
		},
	}

	err := validateProductCatalogDraft(db, draft, target.ID)
	require.EqualError(t, err, "SEO canonical path already exists")
}

func stringPtr(value string) *string {
	return &value
}
