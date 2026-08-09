package httpapi

import (
	"context"
	"errors"

	"ecommerce/internal/apicontract"
)

func (e *CatalogEndpoints) ListAdminProducts(ctx context.Context, request apicontract.ListAdminProductsRequestObject) (apicontract.ListAdminProductsResponseObject, error) {
	input := adminListInput(request.Params)
	result, err := e.catalog.ListProducts(ctx, input)
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	for index := range result.Products {
		if result.Products[index].DraftUpdatedAt == nil {
			continue
		}
		result.Products[index], err = e.catalogAdmin.GetProduct(ctx, result.Products[index].ID, true)
		if err != nil {
			return nil, catalogEndpointError(err)
		}
	}
	products, err := e.productsToContract(ctx, result.Products, true)
	if err != nil {
		return nil, err
	}
	return apicontract.ListAdminProducts200JSONResponse{Data: products, Pagination: apicontract.Pagination{Page: input.Page, Limit: input.Limit, Total: int(result.Total), TotalPages: result.TotalPages}}, nil
}
func (e *CatalogEndpoints) GetAdminProduct(ctx context.Context, request apicontract.GetAdminProductRequestObject) (apicontract.GetAdminProductResponseObject, error) {
	if request.Id < 1 {
		return nil, errors.New("product id must be positive")
	}
	value, err := e.catalogAdmin.GetProduct(ctx, uint(request.Id), true)
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	contract, err := e.productToContract(ctx, value, true)
	if err != nil {
		return nil, err
	}
	return apicontract.GetAdminProduct200JSONResponse(contract), nil
}
func (e *CatalogEndpoints) CreateProduct(ctx context.Context, request apicontract.CreateProductRequestObject) (apicontract.CreateProductResponseObject, error) {
	if request.Body == nil {
		return nil, errors.New("product body is required")
	}
	value, err := e.catalogAdmin.CreateProduct(ctx, *request.Body)
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	contract, err := e.productToContract(ctx, value, true)
	if err != nil {
		return nil, err
	}
	return apicontract.CreateProduct201JSONResponse(contract), nil
}
func (e *CatalogEndpoints) UpdateProduct(ctx context.Context, request apicontract.UpdateProductRequestObject) (apicontract.UpdateProductResponseObject, error) {
	if request.Id < 1 || request.Body == nil {
		return nil, errors.New("valid product id and body are required")
	}
	value, err := e.catalogAdmin.UpdateProduct(ctx, uint(request.Id), *request.Body)
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	contract, err := e.productToContract(ctx, value, true)
	if err != nil {
		return nil, err
	}
	return apicontract.UpdateProduct200JSONResponse(contract), nil
}
func (e *CatalogEndpoints) DeleteProduct(ctx context.Context, request apicontract.DeleteProductRequestObject) (apicontract.DeleteProductResponseObject, error) {
	if request.Id < 1 {
		return nil, errors.New("product id must be positive")
	}
	cleanup, err := e.catalogAdmin.DeleteProduct(ctx, uint(request.Id))
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	if e.media != nil {
		for _, id := range cleanup {
			_ = e.media.DeleteIfOrphan(id)
		}
	}
	return apicontract.DeleteProduct200JSONResponse{Message: "Product deleted successfully"}, nil
}
func (e *CatalogEndpoints) PublishProduct(ctx context.Context, request apicontract.PublishProductRequestObject) (apicontract.PublishProductResponseObject, error) {
	if request.Id < 1 {
		return nil, errors.New("product id must be positive")
	}
	value, err := e.catalogAdmin.PublishProduct(ctx, uint(request.Id))
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	contract, err := e.productToContract(ctx, value, true)
	if err != nil {
		return nil, err
	}
	return apicontract.PublishProduct200JSONResponse(contract), nil
}
func (e *CatalogEndpoints) UnpublishProduct(ctx context.Context, request apicontract.UnpublishProductRequestObject) (apicontract.UnpublishProductResponseObject, error) {
	if request.Id < 1 {
		return nil, errors.New("product id must be positive")
	}
	value, err := e.catalogAdmin.UnpublishProduct(ctx, uint(request.Id))
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	contract, err := e.productToContract(ctx, value, true)
	if err != nil {
		return nil, err
	}
	return apicontract.UnpublishProduct200JSONResponse(contract), nil
}
func (e *CatalogEndpoints) DiscardProductDraft(ctx context.Context, request apicontract.DiscardProductDraftRequestObject) (apicontract.DiscardProductDraftResponseObject, error) {
	if request.Id < 1 {
		return nil, errors.New("product id must be positive")
	}
	value, err := e.catalogAdmin.DiscardProductDraft(ctx, uint(request.Id))
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	contract, err := e.productToContract(ctx, value, true)
	if err != nil {
		return nil, err
	}
	return apicontract.DiscardProductDraft200JSONResponse(contract), nil
}
func (e *CatalogEndpoints) UpdateProductRelated(ctx context.Context, request apicontract.UpdateProductRelatedRequestObject) (apicontract.UpdateProductRelatedResponseObject, error) {
	if request.Id < 1 || request.Body == nil {
		return nil, errors.New("valid product id and related-products body are required")
	}
	value, err := e.catalogAdmin.ReplaceRelated(ctx, uint(request.Id), request.Body.RelatedIds)
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	contract, err := e.productToContract(ctx, value, true)
	if err != nil {
		return nil, err
	}
	return apicontract.UpdateProductRelated200JSONResponse(contract), nil
}
func (e *CatalogEndpoints) AttachProductMedia(ctx context.Context, request apicontract.AttachProductMediaRequestObject) (apicontract.AttachProductMediaResponseObject, error) {
	if request.Id < 1 || request.Body == nil {
		return nil, errors.New("valid product id and media body are required")
	}
	value, err := e.catalogAdmin.AttachMedia(ctx, uint(request.Id), request.Body.MediaIds)
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	contract, err := e.productToContract(ctx, value, true)
	if err != nil {
		return nil, err
	}
	return apicontract.AttachProductMedia200JSONResponse(contract), nil
}
func (e *CatalogEndpoints) UpdateProductMediaOrder(ctx context.Context, request apicontract.UpdateProductMediaOrderRequestObject) (apicontract.UpdateProductMediaOrderResponseObject, error) {
	if request.Id < 1 || request.Body == nil {
		return nil, errors.New("valid product id and media order body are required")
	}
	value, err := e.catalogAdmin.ReorderMedia(ctx, uint(request.Id), request.Body.MediaIds)
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	contract, err := e.productToContract(ctx, value, true)
	if err != nil {
		return nil, err
	}
	return apicontract.UpdateProductMediaOrder200JSONResponse(contract), nil
}
func (e *CatalogEndpoints) DetachProductMedia(ctx context.Context, request apicontract.DetachProductMediaRequestObject) (apicontract.DetachProductMediaResponseObject, error) {
	if request.Id < 1 || request.MediaId == "" {
		return nil, errors.New("valid product and media ids are required")
	}
	value, cleanup, err := e.catalogAdmin.DetachMedia(ctx, uint(request.Id), request.MediaId)
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	if e.media != nil {
		for _, id := range cleanup {
			_ = e.media.DeleteIfOrphan(id)
		}
	}
	contract, err := e.productToContract(ctx, value, true)
	if err != nil {
		return nil, err
	}
	return apicontract.DetachProductMedia200JSONResponse(contract), nil
}
