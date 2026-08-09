package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/apperror"
	"ecommerce/internal/media"
	catalogservice "ecommerce/internal/services/catalog"
	catalogadminservice "ecommerce/internal/services/catalogadmin"
	discountservice "ecommerce/internal/services/discounts"
	inventoryservice "ecommerce/internal/services/inventory"
	"ecommerce/models"

	"gorm.io/gorm"
)

// CatalogEndpoints is an embeddable strict endpoint family for storefront
// catalog, administrative catalog, inventory, purchase order, and discount
// operations. It contains no Gin handlers and can be embedded in a composed
// StrictServerInterface implementation.
type CatalogEndpoints struct {
	db           *gorm.DB
	media        *media.Service
	catalog      *catalogservice.Service
	catalogAdmin *catalogadminservice.Service
	inventory    *inventoryservice.Service
	discounts    *discountservice.Service
}

func NewCatalogEndpoints(db *gorm.DB, mediaService *media.Service) (*CatalogEndpoints, error) {
	if db == nil {
		return nil, errors.New("catalog database is required")
	}
	return &CatalogEndpoints{
		db:           db,
		media:        mediaService,
		catalog:      catalogservice.NewService(db, mediaService),
		catalogAdmin: catalogadminservice.NewService(db, mediaService),
		inventory:    inventoryservice.NewService(db),
		discounts:    discountservice.NewService(db),
	}, nil
}

func (e *CatalogEndpoints) ListProducts(ctx context.Context, request apicontract.ListProductsRequestObject) (apicontract.ListProductsResponseObject, error) {
	input := publicListInput(request.Params)
	result, err := e.catalog.ListProducts(ctx, input)
	if err != nil {
		return nil, catalogEndpointError(fmt.Errorf("list public products: %w", err))
	}
	products, err := e.productsToContract(ctx, result.Products, false)
	if err != nil {
		return nil, err
	}
	return publicProductListResponse{
		Data:       products,
		Pagination: apicontract.Pagination{Page: input.Page, Limit: input.Limit, Total: int(result.Total), TotalPages: result.TotalPages},
	}, nil
}

func (e *CatalogEndpoints) GetProduct(ctx context.Context, request apicontract.GetProductRequestObject) (apicontract.GetProductResponseObject, error) {
	if request.Id < 1 {
		return nil, errors.New("product id must be positive")
	}
	product, err := e.catalog.GetProductByID(ctx, strconv.Itoa(request.Id), false)
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	value, err := e.productToContract(ctx, product, false)
	if err != nil {
		return nil, err
	}
	return publicProductResponse{Product: value}, nil
}

type publicProductListResponse struct {
	Data       []apicontract.Product
	Pagination apicontract.Pagination
}

func (response publicProductListResponse) VisitListProductsResponse(w http.ResponseWriter) error {
	data := make([]map[string]any, 0, len(response.Data))
	for _, product := range response.Data {
		payload, err := publicProductPayload(product)
		if err != nil {
			return err
		}
		data = append(data, payload)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	return json.NewEncoder(w).Encode(map[string]any{"data": data, "pagination": response.Pagination})
}

type publicProductResponse struct {
	Product apicontract.Product
}

func (response publicProductResponse) VisitGetProductResponse(w http.ResponseWriter) error {
	payload, err := publicProductPayload(response.Product)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	return json.NewEncoder(w).Encode(payload)
}

func publicProductPayload(product apicontract.Product) (map[string]any, error) {
	encoded, err := json.Marshal(product)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		return nil, err
	}
	delete(payload, "is_published")
	delete(payload, "has_draft_changes")
	delete(payload, "draft_updated_at")
	return payload, nil
}

func publicListInput(params apicontract.ListProductsParams) catalogservice.ListProductsInput {
	input := catalogservice.ListProductsInput{Page: 1, Limit: 10, SortField: "created_at", SortOrder: "desc"}
	if params.Q != nil {
		input.SearchTerm = strings.TrimSpace(*params.Q)
	}
	input.MinPrice, input.MaxPrice = params.MinPrice, params.MaxPrice
	if params.BrandSlug != nil {
		input.BrandSlug = strings.TrimSpace(*params.BrandSlug)
	}
	if params.CategorySlug != nil {
		input.CategorySlugs = append([]string(nil), (*params.CategorySlug)...)
	}
	input.HasVariantStock = params.HasVariantStock
	if params.Attribute != nil {
		input.Attribute = *params.Attribute
	}
	if params.Sort != nil {
		input.SortField = string(*params.Sort)
	}
	if params.Order != nil {
		input.SortOrder = string(*params.Order)
	}
	if params.Page != nil && *params.Page > 0 {
		input.Page = *params.Page
	}
	if params.Limit != nil && *params.Limit > 0 {
		input.Limit = min(*params.Limit, 100)
	}
	return input
}

func adminListInput(params apicontract.ListAdminProductsParams) catalogservice.ListProductsInput {
	input := catalogservice.ListProductsInput{Page: 1, Limit: 10, SortField: "created_at", SortOrder: "desc", Preview: true}
	if params.Q != nil {
		input.SearchTerm = strings.TrimSpace(*params.Q)
	}
	input.MinPrice, input.MaxPrice = params.MinPrice, params.MaxPrice
	if params.BrandSlug != nil {
		input.BrandSlug = strings.TrimSpace(*params.BrandSlug)
	}
	if params.CategorySlug != nil {
		input.CategorySlugs = append([]string(nil), (*params.CategorySlug)...)
	}
	if params.CategoryId != nil {
		for _, id := range *params.CategoryId {
			if id > 0 {
				input.CategoryIDs = append(input.CategoryIDs, uint(id))
			}
		}
	}
	if params.IncludeInactiveCategories != nil {
		input.IncludeInactiveCategories = *params.IncludeInactiveCategories
	}
	input.HasVariantStock = params.HasVariantStock
	if params.Attribute != nil {
		input.Attribute = *params.Attribute
	}
	if params.Sort != nil {
		input.SortField = string(*params.Sort)
	}
	if params.Order != nil {
		input.SortOrder = string(*params.Order)
	}
	if params.Page != nil && *params.Page > 0 {
		input.Page = *params.Page
	}
	if params.Limit != nil && *params.Limit > 0 {
		input.Limit = min(*params.Limit, 100)
	}
	return input
}

func (e *CatalogEndpoints) productsToContract(ctx context.Context, products []models.Product, admin bool) ([]apicontract.Product, error) {
	result := make([]apicontract.Product, 0, len(products))
	for _, product := range products {
		value, err := e.productToContract(ctx, product, admin)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func (e *CatalogEndpoints) productToContract(ctx context.Context, product models.Product, admin bool) (apicontract.Product, error) {
	db := e.db.WithContext(ctx)
	if product.ID != 0 && product.Related == nil && product.Categories == nil && product.Variants == nil {
		if err := db.Preload("Related").Preload("Categories").Preload("Variants").First(&product, product.ID).Error; err != nil {
			return apicontract.Product{}, err
		}
	}
	images := append([]string(nil), product.Images...)
	role := media.RoleProductImage
	if admin && product.DraftUpdatedAt != nil {
		role = media.RoleProductDraftImage
	}
	if e.media != nil {
		if urls, err := e.media.ProductMediaURLsByRole(product.ID, role); err == nil && len(urls) != 0 {
			images = urls
		}
	}
	var cover *string
	if len(images) != 0 {
		cover = &images[0]
	}
	categories := make([]apicontract.Category, 0, len(product.Categories))
	for _, category := range product.Categories {
		categories = append(categories, categoryContract(category))
	}
	related := make([]apicontract.RelatedProduct, 0, len(product.Related))
	for _, value := range product.Related {
		price := value.Price.Float64()
		description := value.Description
		related = append(related, apicontract.RelatedProduct{Id: int(value.ID), Sku: value.SKU, Name: value.Name, Description: &description, Price: &price, Stock: value.Stock, CoverImage: value.CoverImage})
	}
	variants := make([]apicontract.ProductVariant, 0, len(product.Variants))
	minPrice, maxPrice := product.Price.Float64(), product.Price.Float64()
	for _, value := range product.Variants {
		if !admin && !value.IsPublished {
			continue
		}
		id := int(value.ID)
		price := value.Price.Float64()
		if len(variants) == 0 || price < minPrice {
			minPrice = price
		}
		if len(variants) == 0 || price > maxPrice {
			maxPrice = price
		}
		var compareAt *float64
		if value.CompareAtPrice != nil {
			converted := value.CompareAtPrice.Float64()
			compareAt = &converted
		}
		variants = append(variants, apicontract.ProductVariant{Id: &id, Sku: value.SKU, Title: value.Title, Price: price, CompareAtPrice: compareAt, Stock: value.Stock, Position: value.Position, IsPublished: value.IsPublished, WeightGrams: value.WeightGrams, LengthCm: value.LengthCm, WidthCm: value.WidthCm, HeightCm: value.HeightCm, Selections: []apicontract.ProductVariantSelection{}})
	}
	var defaultVariantID *int
	if product.DefaultVariantID != nil {
		converted := int(*product.DefaultVariantID)
		defaultVariantID = &converted
	}
	var defaultVariantSKU *string
	for _, variant := range variants {
		if defaultVariantID != nil && variant.Id != nil && *variant.Id == *defaultVariantID {
			sku := variant.Sku
			defaultVariantSKU = &sku
			break
		}
	}
	if defaultVariantSKU == nil && len(variants) != 0 {
		sku := variants[0].Sku
		defaultVariantSKU = &sku
	}
	published, draft := product.IsPublished, product.DraftUpdatedAt != nil
	result := apicontract.Product{Id: int(product.ID), Sku: product.SKU, Name: product.Name, Subtitle: product.Subtitle, Description: product.Description, Price: product.Price.Float64(), Stock: product.Stock, Images: images, CoverImage: cover, Categories: categories, RelatedProducts: related, Options: []apicontract.ProductOption{}, Attributes: []apicontract.ProductAttributeValue{}, Seo: apicontract.ProductSEO{}, PriceRange: apicontract.ProductPriceRange{Min: minPrice, Max: maxPrice}, DefaultVariantId: defaultVariantID, DefaultVariantSku: defaultVariantSKU, Variants: variants, CreatedAt: product.CreatedAt, UpdatedAt: product.UpdatedAt, DeletedAt: deletedAt(product.DeletedAt)}
	if product.Brand != nil {
		brand := e.brandContract(*product.Brand)
		result.Brand = &brand
	}
	if admin {
		result.IsPublished, result.HasDraftChanges, result.DraftUpdatedAt = &published, &draft, product.DraftUpdatedAt
	}
	return result, nil
}

func catalogEndpointError(err error) error {
	if err == nil {
		return nil
	}
	if appErr, ok := apperror.As(err); ok {
		status := http.StatusInternalServerError
		switch appErr.Kind {
		case apperror.KindInvalidInput:
			status = http.StatusBadRequest
		case apperror.KindValidation:
			status = http.StatusUnprocessableEntity
		case apperror.KindNotFound:
			status = http.StatusNotFound
		case apperror.KindConflict, apperror.KindFailedPrecondition:
			status = http.StatusConflict
		case apperror.KindUnavailable:
			status = http.StatusServiceUnavailable
		case apperror.KindTimeout:
			status = http.StatusGatewayTimeout
		}
		return problemError(status, appErr.Code, appErr.Detail, err)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return problemError(http.StatusNotFound, "not_found", "The requested catalog resource was not found.", err)
	}
	return err
}

func deletedAt(value gorm.DeletedAt) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}
