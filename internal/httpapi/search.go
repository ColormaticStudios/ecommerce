package httpapi

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"ecommerce/internal/apicontract"
	searchservice "ecommerce/internal/search"
)

var searchPriceRangePattern = regexp.MustCompile(`^(?:[0-9]+(?:\.[0-9]{1,2})?:(?:[0-9]+(?:\.[0-9]{1,2})?)?|:(?:[0-9]+(?:\.[0-9]{1,2})?))$`)

const maxSearchPrice = 9999999999.99

func (e *CatalogEndpoints) SearchProducts(ctx context.Context, request apicontract.SearchProductsRequestObject) (apicontract.SearchProductsResponseObject, error) {
	params := request.Params
	if err := validateSearchProductParams(params); err != nil {
		return nil, problemError(http.StatusBadRequest, "invalid_search_query", err.Error(), err)
	}
	input := searchservice.Filters{Page: 1, Limit: 10, SortField: "created_at", SortOrder: "desc"}
	if params.Q != nil {
		input.Query = strings.TrimSpace(*params.Q)
	}
	input.MinPrice, input.MaxPrice = params.MinPrice, params.MaxPrice
	for _, bound := range []*float64{input.MinPrice, input.MaxPrice} {
		if bound != nil && (*bound < 0 || *bound > maxSearchPrice || math.IsNaN(*bound) || math.IsInf(*bound, 0)) {
			return nil, problemError(http.StatusBadRequest, "invalid_price_range", "Price boundaries must be within the supported range.", nil)
		}
	}
	if input.MinPrice != nil && input.MaxPrice != nil && *input.MinPrice > *input.MaxPrice {
		return nil, problemError(http.StatusBadRequest, "invalid_price_range", "Minimum price cannot exceed maximum price.", nil)
	}
	if params.PriceRange != nil && len(*params.PriceRange) > 0 && (input.MinPrice != nil || input.MaxPrice != nil) {
		return nil, problemError(http.StatusBadRequest, "invalid_price_range", "Price ranges cannot be combined with minimum or maximum price.", nil)
	}
	if params.BrandSlug != nil {
		input.BrandSlugs = append([]string(nil), (*params.BrandSlug)...)
	}
	if params.CategorySlug != nil {
		input.CategorySlugs = append([]string(nil), (*params.CategorySlug)...)
	}
	if params.HasVariantStock != nil {
		input.StockAvailability = append([]bool(nil), (*params.HasVariantStock)...)
	}
	if params.PriceRange != nil {
		input.PriceRanges = make([]searchservice.PriceRange, 0, len(*params.PriceRange))
		for _, raw := range *params.PriceRange {
			value, err := parseSearchPriceRange(raw)
			if err != nil {
				return nil, problemError(http.StatusBadRequest, "invalid_price_range", err.Error(), err)
			}
			input.PriceRanges = append(input.PriceRanges, value)
		}
	}
	if params.Attribute != nil {
		input.AttributeValues = *params.Attribute
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
	facets := make([]apicontract.SearchFacet, 0, len(result.Facets))
	for _, facet := range result.Facets {
		values := make([]apicontract.SearchFacetValue, 0, len(facet.Values))
		for _, value := range facet.Values {
			contract := apicontract.SearchFacetValue{
				Value: value.Value, Count: int(value.Count), Selected: value.Selected, Disabled: value.Disabled,
				MinPrice: value.MinPrice, MaxPrice: value.MaxPrice,
			}
			if value.Label != "" {
				label := value.Label
				contract.Label = &label
			}
			values = append(values, contract)
		}
		contract := apicontract.SearchFacet{Name: facet.Name, Type: apicontract.SearchFacetType(facet.Type), Values: values}
		if facet.Label != "" {
			label := facet.Label
			contract.Label = &label
		}
		facets = append(facets, contract)
	}
	rewrites := make([]apicontract.SearchAppliedRewrite, 0, len(result.AppliedRewrites))
	for _, rewrite := range result.AppliedRewrites {
		rewrites = append(rewrites, apicontract.SearchAppliedRewrite{
			Kind: apicontract.SearchAppliedRewriteKind(rewrite.Kind), Original: rewrite.Original, Replacement: rewrite.Replacement,
		})
	}
	var didYouMean *string
	if result.DidYouMean != "" {
		value := result.DidYouMean
		didYouMean = &value
	}
	return apicontract.SearchProducts200JSONResponse{
		Items:      products,
		Facets:     facets,
		Metadata:   apicontract.ProductSearchMetadata{NormalizedQuery: result.NormalizedQuery, IndexedAt: result.IndexedAt, AppliedRewrites: rewrites, DidYouMean: didYouMean, Relaxed: result.Relaxed},
		Pagination: apicontract.Pagination{Page: input.Page, Limit: input.Limit, Total: int(result.Total), TotalPages: result.TotalPages},
	}, nil
}

func validateSearchProductParams(params apicontract.SearchProductsParams) error {
	if params.Q != nil {
		if utf8.RuneCountInString(*params.Q) > 200 || len(strings.Fields(searchservice.NormalizeQuery(*params.Q))) > 20 {
			return fmt.Errorf("query must be at most 200 characters and 20 terms")
		}
	}
	for _, values := range []*[]string{params.BrandSlug, params.CategorySlug} {
		if values == nil {
			continue
		}
		if len(*values) > 20 {
			return fmt.Errorf("a facet can contain at most 20 selected values")
		}
		for _, value := range *values {
			if utf8.RuneCountInString(value) > 120 || searchservice.NormalizeQuery(value) == "" {
				return fmt.Errorf("facet values must contain text and be at most 120 characters")
			}
		}
	}
	if params.HasVariantStock != nil && len(*params.HasVariantStock) > 2 {
		return fmt.Errorf("stock facet can contain at most two selected values")
	}
	if params.PriceRange != nil && len(*params.PriceRange) > 10 {
		return fmt.Errorf("price facet can contain at most ten selected ranges")
	}
	if params.Attribute != nil {
		if len(*params.Attribute) > 20 {
			return fmt.Errorf("at most 20 attributes can be filtered")
		}
		for key, values := range *params.Attribute {
			if utf8.RuneCountInString(key) > 120 || searchservice.NormalizeQuery(key) == "" || len(values) == 0 || len(values) > 20 {
				return fmt.Errorf("attribute filters require a valid key and one to 20 values")
			}
			for _, value := range values {
				if utf8.RuneCountInString(value) > 120 || searchservice.NormalizeQuery(value) == "" {
					return fmt.Errorf("attribute values must contain text and be at most 120 characters")
				}
			}
		}
	}
	if params.Page != nil && *params.Page < 1 || params.Limit != nil && (*params.Limit < 1 || *params.Limit > 100) {
		return fmt.Errorf("page must be positive and limit must be between 1 and 100")
	}
	if params.Sort != nil {
		switch *params.Sort {
		case apicontract.SearchProductsParamsSortRelevance, apicontract.SearchProductsParamsSortPrice,
			apicontract.SearchProductsParamsSortName, apicontract.SearchProductsParamsSortCreatedAt:
		default:
			return fmt.Errorf("unsupported search sort field")
		}
	}
	if params.Order != nil && *params.Order != apicontract.SearchProductsParamsOrderAsc && *params.Order != apicontract.SearchProductsParamsOrderDesc {
		return fmt.Errorf("unsupported search sort order")
	}
	return nil
}

func parseSearchPriceRange(raw string) (searchservice.PriceRange, error) {
	if !searchPriceRangePattern.MatchString(raw) {
		return searchservice.PriceRange{}, fmt.Errorf("price range %q must use min:max with at least one boundary", raw)
	}
	parts := strings.Split(raw, ":")
	result := searchservice.PriceRange{}
	if parts[0] != "" {
		value, err := strconv.ParseFloat(parts[0], 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > maxSearchPrice {
			return searchservice.PriceRange{}, fmt.Errorf("price range %q has an invalid minimum", raw)
		}
		result.Min = &value
	}
	if parts[1] != "" {
		value, err := strconv.ParseFloat(parts[1], 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > maxSearchPrice {
			return searchservice.PriceRange{}, fmt.Errorf("price range %q has an invalid maximum", raw)
		}
		result.Max = &value
	}
	if result.Min != nil && result.Max != nil && *result.Min > *result.Max {
		return searchservice.PriceRange{}, fmt.Errorf("price range %q has a minimum above its maximum", raw)
	}
	return result, nil
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
