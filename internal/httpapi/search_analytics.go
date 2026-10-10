package httpapi

import (
	"context"
	"errors"
	"net/http"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/requestctx"
	searchservice "ecommerce/internal/search"
)

func analyticsEndpointError(err error) error {
	if errors.Is(err, searchservice.ErrAnalyticsInvalid) {
		return problemError(http.StatusBadRequest, "invalid_search_event", "The search event is invalid or expired.", err)
	}
	return searchConfigurationEndpointError(err)
}
func (e *CatalogEndpoints) CreateSearchImpression(ctx context.Context, r apicontract.CreateSearchImpressionRequestObject) (apicontract.CreateSearchImpressionResponseObject, error) {
	if principal, ok := requestctx.PrincipalFrom(ctx); ok && principal.HasRole("admin") {
		return nil, analyticsEndpointError(searchservice.ErrAnalyticsInvalid)
	}
	if r.Body == nil || r.Body.Filters == nil || !bool(r.Body.Consent) {
		return nil, analyticsEndpointError(searchservice.ErrAnalyticsInvalid)
	}
	f, err := searchProductFilters(previewSearchParams(*r.Body.Filters), false)
	if err != nil {
		return nil, err
	}
	token := ""
	if r.Body.SessionToken != nil {
		token = *r.Body.SessionToken
	}
	value, err := e.search.CreateImpressionIdempotent(ctx, true, token, r.Body.EventId.String(), f)
	if err != nil {
		return nil, analyticsEndpointError(err)
	}
	result, err := e.searchResultContract(ctx, value.Result, f)
	if err != nil {
		return nil, err
	}
	return apicontract.CreateSearchImpression200JSONResponse{ImpressionId: value.ID, SessionToken: value.SessionToken, Result: apicontract.ProductSearchResponse{Items: result.Items, Facets: result.Facets, Metadata: result.Metadata, Pagination: result.Pagination}}, nil
}
func (e *CatalogEndpoints) CreateSearchEvent(ctx context.Context, r apicontract.CreateSearchEventRequestObject) (apicontract.CreateSearchEventResponseObject, error) {
	if principal, ok := requestctx.PrincipalFrom(ctx); ok && principal.HasRole("admin") {
		return nil, analyticsEndpointError(searchservice.ErrAnalyticsInvalid)
	}
	if r.Body == nil || r.Body.Type != "click" || r.Body.ProductId < 1 {
		return nil, analyticsEndpointError(searchservice.ErrAnalyticsInvalid)
	}
	b := r.Body
	id, err := e.search.RecordClick(ctx, searchservice.ClickInput{SessionToken: b.SessionToken, ImpressionID: b.ImpressionId, EventID: b.EventId.String(), ProductID: uint(b.ProductId), Position: b.Position})
	if err != nil {
		return nil, analyticsEndpointError(err)
	}
	return apicontract.CreateSearchEvent200JSONResponse{EventId: id, ClickId: id}, nil
}
func (e *CatalogEndpoints) GetAdminSearchAnalytics(ctx context.Context, r apicontract.GetAdminSearchAnalyticsRequestObject) (apicontract.GetAdminSearchAnalyticsResponseObject, error) {
	days := 7
	if r.Params.Days != nil {
		days = *r.Params.Days
	}
	v, err := e.search.Analytics(ctx, days)
	if err != nil {
		return nil, analyticsEndpointError(err)
	}
	queries := make([]apicontract.SearchQueryMetrics, 0, len(v.Queries))
	for _, m := range v.Queries {
		queries = append(queries, apicontract.SearchQueryMetrics{ZeroResultRate: m.ZeroResultRate, CtrAt5: m.CTRAt5, CtrAt10: m.CTRAt10, ConversionAt5: m.ConversionAt5, ConversionAt10: m.ConversionAt10, P95LatencyMs: m.P95LatencyMs, P99LatencyMs: m.P99LatencyMs, Query: m.Query, Searches: int(m.Searches), UniqueSessions: int(m.UniqueSessions), ZeroResults: int(m.ZeroResults), Clicks: int(m.Clicks), AddToCarts: int(m.AddToCarts), PaidOrders: int(m.PaidOrders), Ctr: float32(m.CTR), ConversionRate: float32(m.ConversionRate), AverageLatencyMs: float32(m.AverageLatencyMs)})
	}
	return apicontract.GetAdminSearchAnalytics200JSONResponse{ZeroResultRate: v.ZeroResultRate, CtrAt5: v.CTRAt5, CtrAt10: v.CTRAt10, ConversionAt5: v.ConversionAt5, ConversionAt10: v.ConversionAt10, P95LatencyMs: v.P95LatencyMs, P99LatencyMs: v.P99LatencyMs, FacetUsage: v.FacetUsage, Days: v.Days, Searches: int(v.Searches), UniqueSessions: int(v.UniqueSessions), ZeroResults: int(v.ZeroResults), Clicks: int(v.Clicks), AddToCarts: int(v.AddToCarts), PaidOrders: int(v.PaidOrders), Ctr: float32(v.CTR), ConversionRate: float32(v.ConversionRate), AverageLatencyMs: float32(v.AverageLatencyMs), Queries: queries, Popular: v.Popular, Trending: v.Trending}, nil
}
func (e *CatalogEndpoints) GetAdminSearchOperations(ctx context.Context, _ apicontract.GetAdminSearchOperationsRequestObject) (apicontract.GetAdminSearchOperationsResponseObject, error) {
	v, err := e.search.Operations(ctx)
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	return apicontract.GetAdminSearchOperations200JSONResponse{MaxConcurrent: v.MaxConcurrent, SearchTimeoutMs: v.SearchTimeoutMS, CircuitFailureThreshold: v.CircuitFailureThreshold, CircuitOpenMs: v.CircuitOpenMS, ReindexQueueLimit: v.ReindexQueueLimit, ActiveSearches: v.ActiveSearches, CircuitState: apicontract.SearchOperationsStatusCircuitState(v.CircuitState), ConsecutiveFailures: v.ConsecutiveFailures, CircuitOpenUntil: v.CircuitOpenUntil, PendingReindexes: int(v.PendingReindexes)}, nil
}

func (e *CatalogEndpoints) RevokeSearchConsent(ctx context.Context, r apicontract.RevokeSearchConsentRequestObject) (apicontract.RevokeSearchConsentResponseObject, error) {
	if r.Body == nil {
		return nil, analyticsEndpointError(searchservice.ErrAnalyticsInvalid)
	}
	if err := e.search.RevokeConsent(ctx, r.Body.SessionToken); err != nil {
		return nil, analyticsEndpointError(err)
	}
	return apicontract.RevokeSearchConsent204Response{}, nil
}
