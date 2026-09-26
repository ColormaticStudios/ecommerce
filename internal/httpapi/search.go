package httpapi

import (
	"context"
	"net/http"
	"strings"

	"ecommerce/internal/apicontract"
	searchservice "ecommerce/internal/search"
)

func (e *CatalogEndpoints) SearchProducts(ctx context.Context, request apicontract.SearchProductsRequestObject) (apicontract.SearchProductsResponseObject, error) {
	params := request.Params
	input := searchservice.Filters{Page: 1, Limit: 10, SortField: "created_at", SortOrder: "desc"}
	if params.Q != nil {
		input.Query = strings.TrimSpace(*params.Q)
	}
	input.MinPrice, input.MaxPrice = params.MinPrice, params.MaxPrice
	if input.MinPrice != nil && input.MaxPrice != nil && *input.MinPrice > *input.MaxPrice {
		return nil, problemError(http.StatusBadRequest, "invalid_price_range", "Minimum price cannot exceed maximum price.", nil)
	}
	if params.BrandSlug != nil {
		input.BrandSlug = strings.TrimSpace(*params.BrandSlug)
	}
	if params.CategorySlug != nil {
		input.CategorySlugs = append([]string(nil), (*params.CategorySlug)...)
	}
	input.HasVariantStock = params.HasVariantStock
	if params.Attribute != nil {
		input.Attributes = *params.Attribute
	}
	if params.Sort != nil {
		input.SortField = string(*params.Sort)
	}
	if params.Order != nil {
		input.SortOrder = string(*params.Order)
	}
	if params.Page != nil {
		input.Page = *params.Page
	}
	if params.Limit != nil {
		input.Limit = *params.Limit
	}

	result, err := e.search.Search(ctx, input)
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	products, err := e.productsToContract(ctx, result.Products, false)
	if err != nil {
		return nil, err
	}
	return apicontract.SearchProducts200JSONResponse{
		Items:      products,
		Facets:     []apicontract.SearchFacet{},
		Metadata:   apicontract.ProductSearchMetadata{NormalizedQuery: result.NormalizedQuery, IndexedAt: result.IndexedAt},
		Pagination: apicontract.Pagination{Page: input.Page, Limit: input.Limit, Total: int(result.Total), TotalPages: result.TotalPages},
	}, nil
}

func (e *CatalogEndpoints) GetSearchSuggestions(ctx context.Context, request apicontract.GetSearchSuggestionsRequestObject) (apicontract.GetSearchSuggestionsResponseObject, error) {
	limit := 8
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	result, err := e.search.Suggest(ctx, request.Params.Q, limit)
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	return apicontract.GetSearchSuggestions200JSONResponse{
		Suggestions: result.Suggestions, Corrections: result.Corrections, Popular: result.Popular,
	}, nil
}

func (e *CatalogEndpoints) GetAdminSearchFreshness(ctx context.Context, _ apicontract.GetAdminSearchFreshnessRequestObject) (apicontract.GetAdminSearchFreshnessResponseObject, error) {
	value, err := e.search.Freshness(ctx)
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	return apicontract.GetAdminSearchFreshness200JSONResponse{
		Status: apicontract.SearchFreshnessStatus(value.Status), LagSeconds: int64(value.Lag.Seconds()),
		PendingJobs: value.PendingJobs, DocumentCount: value.DocumentCount,
		LastIndexedAt: value.LastIndexedAt, LastFullReindexAt: value.LastFullReindexAt,
	}, nil
}

func (e *CatalogEndpoints) CreateAdminSearchReindex(ctx context.Context, _ apicontract.CreateAdminSearchReindexRequestObject) (apicontract.CreateAdminSearchReindexResponseObject, error) {
	job, err := e.search.EnqueueFullReindex(ctx)
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	return apicontract.CreateAdminSearchReindex202JSONResponse{JobId: job.ID, Status: apicontract.Queued}, nil
}
