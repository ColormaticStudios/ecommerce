package commands

import (
	"context"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"
)

func catalogGetProduct(ctx context.Context, id uint) (apicontract.Product, error) {
	return withCatalogEndpoints(ctx, func(ctx context.Context, e *httpapi.CatalogEndpoints) (apicontract.Product, error) {
		response, err := e.GetAdminProduct(ctx, apicontract.GetAdminProductRequestObject{Id: int(id)})
		if err != nil {
			return apicontract.Product{}, err
		}
		return apicontract.Product(response.(apicontract.GetAdminProduct200JSONResponse)), nil
	})
}
func catalogUpdateProduct(ctx context.Context, id uint, body apicontract.ProductUpsertInput) (apicontract.Product, error) {
	return withCatalogEndpoints(ctx, func(ctx context.Context, e *httpapi.CatalogEndpoints) (apicontract.Product, error) {
		response, err := e.UpdateProduct(ctx, apicontract.UpdateProductRequestObject{Id: int(id), Body: &body})
		if err != nil {
			return apicontract.Product{}, err
		}
		return apicontract.Product(response.(apicontract.UpdateProduct200JSONResponse)), nil
	})
}
func catalogDiscardProduct(ctx context.Context, id uint) (apicontract.Product, error) {
	return withCatalogEndpoints(ctx, func(ctx context.Context, e *httpapi.CatalogEndpoints) (apicontract.Product, error) {
		response, err := e.DiscardProductDraft(ctx, apicontract.DiscardProductDraftRequestObject{Id: int(id)})
		if err != nil {
			return apicontract.Product{}, err
		}
		return apicontract.Product(response.(apicontract.DiscardProductDraft200JSONResponse)), nil
	})
}
func catalogPublishProduct(ctx context.Context, id uint) (apicontract.Product, error) {
	return withCatalogEndpoints(ctx, func(ctx context.Context, e *httpapi.CatalogEndpoints) (apicontract.Product, error) {
		response, err := e.PublishProduct(ctx, apicontract.PublishProductRequestObject{Id: int(id)})
		if err != nil {
			return apicontract.Product{}, err
		}
		return apicontract.Product(response.(apicontract.PublishProduct200JSONResponse)), nil
	})
}
func catalogUnpublishProduct(ctx context.Context, id uint) (apicontract.Product, error) {
	return withCatalogEndpoints(ctx, func(ctx context.Context, e *httpapi.CatalogEndpoints) (apicontract.Product, error) {
		response, err := e.UnpublishProduct(ctx, apicontract.UnpublishProductRequestObject{Id: int(id)})
		if err != nil {
			return apicontract.Product{}, err
		}
		return apicontract.Product(response.(apicontract.UnpublishProduct200JSONResponse)), nil
	})
}
