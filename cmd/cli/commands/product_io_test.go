package commands

import (
	"testing"

	"ecommerce/internal/apicontract"
	"ecommerce/models"
)

func TestLoadLiveProductUpsertInputIncludesCatalogStructures(t *testing.T) {
	db := newTestDB(t,
		&models.Brand{},
		&models.Product{},
		&models.ProductOption{},
		&models.ProductOptionValue{},
		&models.ProductVariant{},
		&models.ProductVariantOptionValue{},
		&models.ProductAttribute{},
		&models.ProductAttributeValue{},
		&models.SEOMetadata{},
	)

	brand := models.Brand{Name: "Acme", Slug: "acme", IsActive: true}
	if err := db.Create(&brand).Error; err != nil {
		t.Fatalf("create brand: %v", err)
	}

	related := models.Product{
		SKU:         "REL-1",
		Name:        "Related",
		Description: "Related product",
		Price:       models.MoneyFromFloat(9.99),
		Stock:       1,
	}
	if err := db.Create(&related).Error; err != nil {
		t.Fatalf("create related product: %v", err)
	}

	product := models.Product{
		SKU:         "PROD-1",
		Name:        "Main",
		Description: "Main product",
		Price:       models.MoneyFromFloat(19.99),
		Stock:       7,
		BrandID:     &brand.ID,
		Related:     []models.Product{related},
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}

	option := models.ProductOption{ProductID: product.ID, Name: "Size", Position: 1, DisplayType: "select"}
	if err := db.Create(&option).Error; err != nil {
		t.Fatalf("create option: %v", err)
	}
	valueS := models.ProductOptionValue{ProductOptionID: option.ID, Value: "S", Position: 1}
	valueM := models.ProductOptionValue{ProductOptionID: option.ID, Value: "M", Position: 2}
	if err := db.Create(&valueS).Error; err != nil {
		t.Fatalf("create option value S: %v", err)
	}
	if err := db.Create(&valueM).Error; err != nil {
		t.Fatalf("create option value M: %v", err)
	}

	variant := models.ProductVariant{
		ProductID:   product.ID,
		SKU:         "PROD-1-M",
		Title:       "Main / M",
		Price:       models.MoneyFromFloat(21.99),
		Stock:       4,
		Position:    1,
		IsPublished: true,
	}
	if err := db.Select("*").Create(&variant).Error; err != nil {
		t.Fatalf("create variant: %v", err)
	}
	if err := db.Create(&models.ProductVariantOptionValue{
		ProductVariantID:     variant.ID,
		ProductOptionValueID: valueM.ID,
	}).Error; err != nil {
		t.Fatalf("create variant selection: %v", err)
	}
	if err := db.Model(&product).Update("default_variant_id", variant.ID).Error; err != nil {
		t.Fatalf("set default variant: %v", err)
	}

	attribute := models.ProductAttribute{Key: "material", Slug: "material", Type: "text", Filterable: true}
	if err := db.Create(&attribute).Error; err != nil {
		t.Fatalf("create attribute: %v", err)
	}
	textValue := "Cotton"
	if err := db.Create(&models.ProductAttributeValue{
		ProductID:          product.ID,
		ProductAttributeID: attribute.ID,
		TextValue:          &textValue,
		Position:           1,
	}).Error; err != nil {
		t.Fatalf("create attribute value: %v", err)
	}

	title := "SEO title"
	canonical := "/products/main"
	noIndex := false
	if err := db.Create(&models.SEOMetadata{
		EntityType:    productSEOEntityType,
		EntityID:      product.ID,
		Title:         &title,
		CanonicalPath: &canonical,
		NoIndex:       noIndex,
	}).Error; err != nil {
		t.Fatalf("create seo metadata: %v", err)
	}

	input, err := loadLiveProductUpsertInput(db, nil, product.ID)
	if err != nil {
		t.Fatalf("load live product input: %v", err)
	}

	if input.BrandId == nil || *input.BrandId != int(brand.ID) {
		t.Fatalf("expected brand id %d, got %+v", brand.ID, input.BrandId)
	}
	if input.DefaultVariantSku == nil || *input.DefaultVariantSku != variant.SKU {
		t.Fatalf("expected default variant sku %q, got %+v", variant.SKU, input.DefaultVariantSku)
	}
	if len(input.Options) != 1 || len(input.Options[0].Values) != 2 {
		t.Fatalf("expected one option with two values, got %+v", input.Options)
	}
	if len(input.Variants) != 1 {
		t.Fatalf("expected one variant, got %+v", input.Variants)
	}
	if len(input.Variants[0].Selections) != 1 || input.Variants[0].Selections[0].OptionValue != "M" {
		t.Fatalf("expected variant selection M, got %+v", input.Variants[0].Selections)
	}
	if len(input.Attributes) != 1 || input.Attributes[0].ProductAttributeId != int(attribute.ID) {
		t.Fatalf("expected one attribute value, got %+v", input.Attributes)
	}
	if input.Seo.Title == nil || *input.Seo.Title != title {
		t.Fatalf("expected seo title %q, got %+v", title, input.Seo.Title)
	}
	if len(input.RelatedProductIds) != 1 || input.RelatedProductIds[0] != int(related.ID) {
		t.Fatalf("expected related product %d, got %+v", related.ID, input.RelatedProductIds)
	}
}

func TestProductContractToUpsertInputPreservesDraftSections(t *testing.T) {
	defaultSKU := "PROD-1-M"
	optionPosition := 1
	valuePosition := 1
	variantPosition := 1
	selectionPosition := 1
	attributePosition := 1
	isPublished := true
	brandID := 42

	contract := apicontract.Product{
		Id:                1,
		Sku:               "PROD-1",
		Name:              "Main",
		Description:       "Draft",
		DefaultVariantSku: &defaultSKU,
		Brand:             &apicontract.Brand{Id: brandID, Name: "Acme", Slug: "acme"},
		Options: []apicontract.ProductOption{
			{
				Name:        "Size",
				DisplayType: "select",
				Position:    optionPosition,
				Values: []apicontract.ProductOptionValue{
					{Value: "M", Position: valuePosition},
				},
			},
		},
		Variants: []apicontract.ProductVariant{
			{
				Sku:         "PROD-1-M",
				Title:       "Main / M",
				Price:       21.99,
				Stock:       4,
				Position:    variantPosition,
				IsPublished: isPublished,
				Selections: []apicontract.ProductVariantSelection{
					{OptionName: "Size", OptionValue: "M", Position: selectionPosition},
				},
			},
		},
		Attributes: []apicontract.ProductAttributeValue{
			{ProductAttributeId: 7, TextValue: stringPtr("Cotton"), Position: attributePosition},
		},
		Seo: apicontract.ProductSEO{Title: stringPtr("SEO")},
		RelatedProducts: []apicontract.RelatedProduct{
			{Id: 9},
		},
	}

	input := productContractToUpsertInput(contract)

	if input.BrandId == nil || *input.BrandId != brandID {
		t.Fatalf("expected brand id %d, got %+v", brandID, input.BrandId)
	}
	if input.DefaultVariantSku == nil || *input.DefaultVariantSku != defaultSKU {
		t.Fatalf("expected default variant sku %q, got %+v", defaultSKU, input.DefaultVariantSku)
	}
	if len(input.Options) != 1 || input.Options[0].Position == nil || *input.Options[0].Position != optionPosition {
		t.Fatalf("expected option position %d, got %+v", optionPosition, input.Options)
	}
	if len(input.Variants) != 1 || input.Variants[0].IsPublished == nil || !*input.Variants[0].IsPublished {
		t.Fatalf("expected published variant, got %+v", input.Variants)
	}
	if len(input.Attributes) != 1 || input.Attributes[0].Position == nil || *input.Attributes[0].Position != attributePosition {
		t.Fatalf("expected attribute position %d, got %+v", attributePosition, input.Attributes)
	}
	if len(input.RelatedProductIds) != 1 || input.RelatedProductIds[0] != 9 {
		t.Fatalf("expected related product id 9, got %+v", input.RelatedProductIds)
	}
}

func stringPtr(value string) *string {
	return &value
}
