package httpapi_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"
	"ecommerce/internal/requestctx"
	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func catalogTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Brand{}, &models.Category{}, &models.ProductAttribute{}, &models.Locale{}, &models.LocaleMarketDefault{}, &models.LocalizedEntityValue{}))
	require.NoError(t, db.Select("*").Create(&models.Locale{Code: "en-US", Name: "English (United States)", IsDefault: true, IsEnabled: true}).Error)
	return db
}

func TestCatalogEndpointsLocalizesPublicMetadataAndReportsFieldSources(t *testing.T) {
	db := catalogTestDB(t)
	var english models.Locale
	require.NoError(t, db.Where("code = ?", "en-US").First(&english).Error)
	french := models.Locale{Code: "fr", Name: "French", IsEnabled: true, FallbackLocaleID: &english.ID}
	require.NoError(t, db.Select("*").Create(&french).Error)
	description := "Everyday bags"
	brand := models.Brand{Name: "Northstar", Slug: "northstar", Description: &description, IsActive: true}
	require.NoError(t, db.Select("*").Create(&brand).Error)
	require.NoError(t, db.Select("*").Create(&models.LocalizedEntityValue{EntityType: "brand", EntityID: brand.ID, LocaleID: english.ID, Field: "name", Value: brand.Name}).Error)
	require.NoError(t, db.Select("*").Create(&models.LocalizedEntityValue{EntityType: "brand", EntityID: brand.ID, LocaleID: english.ID, Field: "description", Value: description}).Error)
	require.NoError(t, db.Select("*").Create(&models.LocalizedEntityValue{EntityType: "brand", EntityID: brand.ID, LocaleID: french.ID, Field: "name", Value: "Étoile du Nord"}).Error)

	ctx := requestctx.WithLocaleResolution(context.Background(), requestctx.LocaleResolution{RequestedLocale: "fr", ResolvedLocale: "fr", Source: "explicit", FallbackChain: []string{"fr", "en-US"}})
	endpoints, err := httpapi.NewCatalogEndpoints(db, nil)
	require.NoError(t, err)
	response, err := endpoints.ListBrands(ctx, apicontract.ListBrandsRequestObject{})
	require.NoError(t, err)
	brands := apicontract.BrandListResponse(response.(apicontract.ListBrands200JSONResponse))
	require.Len(t, brands.Data, 1)
	require.Equal(t, "Étoile du Nord", brands.Data[0].Name)
	require.Equal(t, description, *brands.Data[0].Description)
	require.NotNil(t, brands.Data[0].Localization)
	require.Equal(t, "fr", brands.Data[0].Localization.SourceLocales["name"])
	require.Equal(t, "en-US", brands.Data[0].Localization.SourceLocales["description"])
	require.True(t, brands.Data[0].Localization.UsedFallback)
}

func TestNewCatalogEndpointsRequiresDatabase(t *testing.T) {
	_, err := httpapi.NewCatalogEndpoints(nil, nil)
	require.Error(t, err)
}

func TestCatalogEndpointsCoversStrictCatalogFamily(t *testing.T) {
	operations := []string{
		"ListBrands", "ListCategories", "ListProductAttributes", "ListProducts", "GetProduct", "SearchProducts", "GetSearchSuggestions",
		"GetAdminSearchFreshness", "CreateAdminSearchReindex", "SearchAdminProducts",
		"ListAdminSearchRankingProfiles", "CreateAdminSearchRankingProfile", "GetAdminSearchRankingProfile", "UpdateAdminSearchRankingProfile", "DeleteAdminSearchRankingProfile",
		"ListAdminSearchSynonyms", "CreateAdminSearchSynonym", "GetAdminSearchSynonym", "UpdateAdminSearchSynonym", "DeleteAdminSearchSynonym",
		"ListAdminSearchTypoProfiles", "CreateAdminSearchTypoProfile", "GetAdminSearchTypoProfile", "UpdateAdminSearchTypoProfile", "DeleteAdminSearchTypoProfile",
		"ListAdminBrands", "CreateAdminBrand", "UpdateAdminBrand", "DeleteAdminBrand",
		"ListAdminCategories", "CreateAdminCategory", "UpdateAdminCategory", "DeleteAdminCategory",
		"ListAdminProductAttributes", "CreateAdminProductAttribute", "UpdateAdminProductAttribute", "DeleteAdminProductAttribute",
		"ListAdminProducts", "CreateProduct", "DeleteProduct", "GetAdminProduct", "UpdateProduct", "DiscardProductDraft",
		"AttachProductMedia", "UpdateProductMediaOrder", "DetachProductMedia", "PublishProduct", "UpdateProductRelated", "UnpublishProduct",
		"CreateAdminInventoryAdjustment", "ListAdminInventoryAlerts", "AckAdminInventoryAlert", "ResolveAdminInventoryAlert",
		"RunAdminInventoryReconciliation", "ListAdminInventoryReservations", "ListAdminInventoryThresholds", "UpsertAdminInventoryThreshold",
		"DeleteAdminInventoryThreshold", "GetAdminInventoryTimeline", "ListAdminPurchaseOrders", "CreateAdminPurchaseOrder",
		"CancelAdminPurchaseOrder", "IssueAdminPurchaseOrder", "ReceiveAdminPurchaseOrder",
		"GetActiveDiscountCampaign", "ListAdminDiscountAudit", "ListAdminDiscountCampaigns", "CreateAdminDiscountCampaign", "UpdateAdminDiscountCampaign",
		"ArchiveAdminDiscountCampaign", "DisableAdminDiscountCampaign", "ScheduleAdminDiscountCampaign", "ListAdminDiscountHistory",
		"RunAdminDiscountLifecycle", "GetAdminDiscountMetrics", "CreateAdminPromotionCampaign", "PreviewAdminPromotion",
		"RunAdminDiscountReconciliation", "ListAdminPromotionTemplates", "CreateAdminPromotionTemplate", "InstantiateAdminPromotionTemplate",
	}
	typeOfEndpoints := reflect.TypeOf((*httpapi.CatalogEndpoints)(nil))
	for _, operation := range operations {
		_, ok := typeOfEndpoints.MethodByName(operation)
		assert.Truef(t, ok, "CatalogEndpoints must implement %s", operation)
	}
}

func TestCatalogEndpointsPublicMetadataFiltersInactiveRows(t *testing.T) {
	db := catalogTestDB(t)
	require.NoError(t, db.Select("*").Create(&models.Brand{Name: "Active", Slug: "active", IsActive: true}).Error)
	inactiveBrand := models.Brand{Name: "Inactive", Slug: "inactive", IsActive: false}
	require.NoError(t, db.Select("*").Create(&inactiveBrand).Error)
	require.NoError(t, db.Model(&inactiveBrand).Update("is_active", false).Error)
	require.NoError(t, db.Select("*").Create(&models.Category{Name: "Active", Slug: "active", Path: "/active", IsActive: true}).Error)
	inactiveCategory := models.Category{Name: "Inactive", Slug: "inactive", Path: "/inactive", IsActive: false}
	require.NoError(t, db.Select("*").Create(&inactiveCategory).Error)
	require.NoError(t, db.Model(&inactiveCategory).Update("is_active", false).Error)
	require.NoError(t, db.Select("*").Create(&models.ProductAttribute{Key: "Color", Slug: "color", Type: "enum", Filterable: true}).Error)
	require.NoError(t, db.Select("*").Create(&models.ProductAttribute{Key: "Internal", Slug: "internal", Type: "text", Filterable: false}).Error)

	endpoints, err := httpapi.NewCatalogEndpoints(db, nil)
	require.NoError(t, err)
	brandsResponse, err := endpoints.ListBrands(context.Background(), apicontract.ListBrandsRequestObject{})
	require.NoError(t, err)
	brands := apicontract.BrandListResponse(brandsResponse.(apicontract.ListBrands200JSONResponse))
	require.Len(t, brands.Data, 1)
	assert.Equal(t, "active", brands.Data[0].Slug)

	categoriesResponse, err := endpoints.ListCategories(context.Background(), apicontract.ListCategoriesRequestObject{})
	require.NoError(t, err)
	categories := apicontract.CategoryListResponse(categoriesResponse.(apicontract.ListCategories200JSONResponse))
	require.Len(t, categories.Data, 1)
	assert.Equal(t, "active", categories.Data[0].Slug)

	attributesResponse, err := endpoints.ListProductAttributes(context.Background(), apicontract.ListProductAttributesRequestObject{})
	require.NoError(t, err)
	attributes := apicontract.ProductAttributeDefinitionListResponse(attributesResponse.(apicontract.ListProductAttributes200JSONResponse))
	require.Len(t, attributes.Data, 1)
	assert.Equal(t, "color", attributes.Data[0].Slug)
}
