package httpapi

import (
	"errors"

	"ecommerce/internal/apicontract"
)

// Server is the concrete generated strict server. Its four embedded endpoint
// families are disjoint; the compile-time assertion fails whenever the OpenAPI
// contract gains an operation that has not been assigned to a family.
type Server struct {
	*AccountEndpoints
	*CatalogEndpoints
	*CmsMediaEndpoints
	*CheckoutProviderEndpoints
}

var _ apicontract.StrictServerInterface = (*Server)(nil)

func NewServer(account *AccountEndpoints, catalog *CatalogEndpoints, cmsMedia *CmsMediaEndpoints, checkoutProvider *CheckoutProviderEndpoints) (*Server, error) {
	if account == nil {
		return nil, errors.New("account endpoints are required")
	}
	if catalog == nil {
		return nil, errors.New("catalog endpoints are required")
	}
	if cmsMedia == nil {
		return nil, errors.New("CMS/media endpoints are required")
	}
	if checkoutProvider == nil {
		return nil, errors.New("checkout/provider endpoints are required")
	}
	return &Server{AccountEndpoints: account, CatalogEndpoints: catalog, CmsMediaEndpoints: cmsMedia, CheckoutProviderEndpoints: checkoutProvider}, nil
}
