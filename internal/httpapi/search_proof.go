package httpapi

import (
	"context"
	"net/http"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/requestctx"
	searchservice "ecommerce/internal/search"
)

func (e *CatalogEndpoints) CreateSearchProof(ctx context.Context, request apicontract.CreateSearchProofRequestObject) (apicontract.CreateSearchProofResponseObject, error) {
	if principal, ok := requestctx.PrincipalFrom(ctx); ok && principal.HasRole("admin") {
		return nil, problemError(http.StatusForbidden, "search_analytics_admin_excluded", "Administrators are excluded from shopper analytics.", nil)
	}
	if request.Body == nil || !bool(request.Body.Consent) {
		return nil, analyticsEndpointError(searchservice.ErrAnalyticsInvalid)
	}
	release, err := e.admitSearchClient(ctx, true)
	if err != nil {
		return nil, err
	}
	defer release()
	token, err := e.search.BrowserResultProof(request.Body.SnapshotToken, request.Body.EventId.String())
	if err != nil {
		return nil, analyticsEndpointError(err)
	}
	return apicontract.CreateSearchProof200JSONResponse{
		Body:    apicontract.SearchProofResponse{ResultToken: token},
		Headers: apicontract.CreateSearchProof200ResponseHeaders{CacheControl: "private, no-store"},
	}, nil
}
