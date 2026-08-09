package httpapi_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"
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
	require.NoError(t, db.AutoMigrate(&models.Brand{}, &models.Category{}, &models.ProductAttribute{}))
	return db
}

func TestNewCatalogEndpointsRequiresDatabase(t *testing.T) {
	_, err := httpapi.NewCatalogEndpoints(nil, nil)
	require.Error(t, err)
}

func TestCatalogEndpointsCoversStrictCatalogFamily(t *testing.T) {
	operations := []string{
		"ListBrands", "ListCategories", "ListProductAttributes", "ListProducts", "GetProduct",
		"ListAdminBrands", "CreateAdminBrand", "UpdateAdminBrand", "DeleteAdminBrand",
		"ListAdminCategories", "CreateAdminCategory", "UpdateAdminCategory", "DeleteAdminCategory",
		"ListAdminProductAttributes", "CreateAdminProductAttribute", "UpdateAdminProductAttribute", "DeleteAdminProductAttribute",
		"ListAdminProducts", "CreateProduct", "DeleteProduct", "GetAdminProduct", "UpdateProduct", "DiscardProductDraft",
		"AttachProductMedia", "UpdateProductMediaOrder", "DetachProductMedia", "PublishProduct", "UpdateProductRelated", "UnpublishProduct",
		"CreateAdminInventoryAdjustment", "ListAdminInventoryAlerts", "AckAdminInventoryAlert", "ResolveAdminInventoryAlert",
		"RunAdminInventoryReconciliation", "ListAdminInventoryReservations", "ListAdminInventoryThresholds", "UpsertAdminInventoryThreshold",
		"DeleteAdminInventoryThreshold", "GetAdminInventoryTimeline", "ListAdminPurchaseOrders", "CreateAdminPurchaseOrder",
		"CancelAdminPurchaseOrder", "IssueAdminPurchaseOrder", "ReceiveAdminPurchaseOrder",
		"ListAdminDiscountAudit", "ListAdminDiscountCampaigns", "CreateAdminDiscountCampaign", "UpdateAdminDiscountCampaign",
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
