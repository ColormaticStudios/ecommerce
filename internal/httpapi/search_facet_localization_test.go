package httpapi

import (
	"context"
	"ecommerce/internal/requestctx"
	searchservice "ecommerce/internal/search"
	localizationservice "ecommerce/internal/services/localization"
	"ecommerce/models"
	"fmt"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestSearchFacetLocalizationBatchesSelectedZeroCountValues(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Brand{}, &models.Category{}, &models.ProductAttribute{}, &models.Locale{}, &models.LocaleMarketDefault{}, &models.LocalizedEntityValue{}))
	require.NoError(t, db.Create(&models.Locale{BaseModel: models.BaseModel{ID: 1}, Code: "en-US", Name: "English", IsDefault: true, IsEnabled: true}).Error)
	require.NoError(t, db.Create(&models.Locale{BaseModel: models.BaseModel{ID: 2}, Code: "fr", Name: "French", IsEnabled: true}).Error)
	category := models.Category{Name: "Shoes", Slug: "chaussures-été"}
	attribute := models.ProductAttribute{Key: "Color", Slug: "couleur-été", Type: "enum", Filterable: true}
	require.NoError(t, db.Create(&category).Error)
	require.NoError(t, db.Create(&attribute).Error)
	require.NoError(t, db.Create(&[]models.LocalizedEntityValue{{EntityType: "category", EntityID: category.ID, LocaleID: 1, Field: "name", Value: "Shoes"}, {EntityType: "product_attribute", EntityID: attribute.ID, LocaleID: 2, Field: "key", Value: "Couleur"}}).Error)
	values := []searchservice.FacetValue{}
	for i := 0; i < 30; i++ {
		brand := models.Brand{Name: "Canonical", Slug: fmt.Sprintf("marque-été-%d", i)}
		require.NoError(t, db.Create(&brand).Error)
		require.NoError(t, db.Create(&models.LocalizedEntityValue{EntityType: "brand", EntityID: brand.ID, LocaleID: 2, Field: "name", Value: fmt.Sprintf("Marque %d", i)}).Error)
		values = append(values, searchservice.FacetValue{Value: searchservice.NormalizeQuery(brand.Slug), Label: "canonical", Count: 0, Selected: true, Disabled: true})
	}
	values = append(values, searchservice.FacetValue{Value: "unknown brand", Label: "Unknown brand", Count: 0, Selected: true, Disabled: true})
	e := &CatalogEndpoints{db: db, localization: localizationservice.NewService(db)}
	ctx := requestctx.WithLocaleResolution(context.Background(), requestctx.LocaleResolution{ResolvedLocale: "fr"})
	queries := 0
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("facet_queries", func(*gorm.DB) { queries++ }))
	facets := []searchservice.Facet{{Name: "brand", Type: "term", Values: values}, {Name: "category", Type: "term", Values: []searchservice.FacetValue{{Value: "chaussures été", Count: 0, Selected: true}, {Value: "unknown category", Label: "unknown-category", Count: 0, Selected: true, Disabled: true}}}, {Name: "attribute:couleur été", Label: "Color", Type: "attribute", Values: []searchservice.FacetValue{{Value: "red", Count: 0, Selected: true}}}, {Name: "attribute:unknown color", Type: "attribute", Values: []searchservice.FacetValue{{Value: "red", Count: 0, Selected: true}}}}
	result, err := e.localizeSearchFacets(ctx, facets)
	require.NoError(t, err)
	require.LessOrEqual(t, queries, 12)
	require.Equal(t, "Marque 0", result[0].Values[0].Label)
	require.Equal(t, "marque été 0", result[0].Values[0].Value)
	require.True(t, result[0].Values[0].Selected)
	require.True(t, result[0].Values[0].Disabled)
	require.Zero(t, result[0].Values[0].Count)
	require.Equal(t, values[30], result[0].Values[30], "unknown selected brands stay labeled and removable")
	require.Equal(t, "Shoes", result[1].Values[0].Label)
	require.Equal(t, "chaussures été", result[1].Values[0].Value)
	require.Equal(t, "unknown-category", result[1].Values[1].Label)
	require.True(t, result[1].Values[1].Selected)
	require.True(t, result[1].Values[1].Disabled)
	require.Equal(t, "Couleur", result[2].Label)
	require.Equal(t, "red", result[2].Values[0].Value)
	require.Equal(t, "unknown color", result[3].Label)
	firstQueries := queries
	facets[0].Values = facets[0].Values[:1]
	_, err = e.localizeSearchFacets(ctx, facets)
	require.NoError(t, err)
	require.Equal(t, firstQueries, queries-firstQueries)
}

func TestSearchSelectedFacetCanonicalNamesWithoutLocalizationRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Brand{}, &models.Category{}, &models.ProductAttribute{}, &models.Locale{}, &models.LocaleMarketDefault{}, &models.LocalizedEntityValue{}))
	require.NoError(t, db.Create(&models.Locale{Code: "en-US", Name: "English", IsDefault: true, IsEnabled: true}).Error)
	require.NoError(t, db.Create(&models.Brand{Name: "Acme Corp", Slug: "acme-corp"}).Error)
	require.NoError(t, db.Create(&models.Category{Name: "Summer Shoes", Slug: "summer-shoes"}).Error)
	require.NoError(t, db.Create(&models.ProductAttribute{Key: "Material", Slug: "material", Type: "enum", Filterable: true}).Error)
	e := &CatalogEndpoints{db: db, localization: localizationservice.NewService(db)}
	facets := []searchservice.Facet{
		{Name: "brand", Values: []searchservice.FacetValue{{Value: "acme corp", Label: "acme corp", Selected: true, Disabled: true}}},
		{Name: "category", Values: []searchservice.FacetValue{{Value: "summer shoes", Label: "summer shoes", Selected: true, Disabled: true}}},
		{Name: "attribute:material", Label: "material"},
	}
	result, err := e.localizeSearchFacets(context.Background(), facets)
	require.NoError(t, err)
	require.Equal(t, "Acme Corp", result[0].Values[0].Label)
	require.Equal(t, "Summer Shoes", result[1].Values[0].Label)
	require.Equal(t, "Material", result[2].Label)
	require.Equal(t, "acme corp", result[0].Values[0].Value)
	require.True(t, result[0].Values[0].Selected)
	require.True(t, result[0].Values[0].Disabled)
}
