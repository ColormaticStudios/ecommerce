package httpapi

import (
	"context"
	"errors"

	"ecommerce/internal/apicontract"
	localizationservice "ecommerce/internal/services/localization"
	"ecommerce/models"
)

func (e *CatalogEndpoints) ListBrands(ctx context.Context, _ apicontract.ListBrandsRequestObject) (apicontract.ListBrandsResponseObject, error) {
	values, err := e.catalogAdmin.ListBrands(ctx, true, "")
	if err != nil {
		return nil, err
	}
	valuesContract, err := e.brandsContract(ctx, values, true)
	if err != nil {
		return nil, err
	}
	return apicontract.ListBrands200JSONResponse{Data: valuesContract}, nil
}

func (e *CatalogEndpoints) ListAdminBrands(ctx context.Context, request apicontract.ListAdminBrandsRequestObject) (apicontract.ListAdminBrandsResponseObject, error) {
	query := ""
	if request.Params.Q != nil {
		query = *request.Params.Q
	}
	values, err := e.catalogAdmin.ListBrands(ctx, false, query)
	if err != nil {
		return nil, err
	}
	valuesContract, err := e.brandsContract(ctx, values, false)
	if err != nil {
		return nil, err
	}
	return apicontract.ListAdminBrands200JSONResponse{Data: valuesContract}, nil
}

func (e *CatalogEndpoints) CreateAdminBrand(ctx context.Context, request apicontract.CreateAdminBrandRequestObject) (apicontract.CreateAdminBrandResponseObject, error) {
	if request.Body == nil {
		return nil, errors.New("brand body is required")
	}
	value, err := e.catalogAdmin.CreateBrand(ctx, *request.Body)
	if err != nil {
		return nil, err
	}
	contract, err := e.brandContract(ctx, value, false)
	if err != nil {
		return nil, err
	}
	return apicontract.CreateAdminBrand201JSONResponse(contract), nil
}

func (e *CatalogEndpoints) UpdateAdminBrand(ctx context.Context, request apicontract.UpdateAdminBrandRequestObject) (apicontract.UpdateAdminBrandResponseObject, error) {
	if request.Id < 1 || request.Body == nil {
		return nil, errors.New("valid brand id and body are required")
	}
	value, err := e.catalogAdmin.UpdateBrand(ctx, uint(request.Id), *request.Body)
	if err != nil {
		return nil, err
	}
	contract, err := e.brandContract(ctx, value, false)
	if err != nil {
		return nil, err
	}
	return apicontract.UpdateAdminBrand200JSONResponse(contract), nil
}

func (e *CatalogEndpoints) DeleteAdminBrand(ctx context.Context, request apicontract.DeleteAdminBrandRequestObject) (apicontract.DeleteAdminBrandResponseObject, error) {
	if request.Id < 1 {
		return nil, errors.New("brand id must be positive")
	}
	if err := e.catalogAdmin.DeleteBrand(ctx, uint(request.Id)); err != nil {
		return nil, err
	}
	return apicontract.DeleteAdminBrand200JSONResponse{Message: "Brand deleted"}, nil
}

func (e *CatalogEndpoints) brandsContract(ctx context.Context, values []models.Brand, localized bool) ([]apicontract.Brand, error) {
	result := make([]apicontract.Brand, 0, len(values))
	for _, value := range values {
		contract, err := e.brandContract(ctx, value, localized)
		if err != nil {
			return nil, err
		}
		result = append(result, contract)
	}
	return result, nil
}

func (e *CatalogEndpoints) brandContract(ctx context.Context, value models.Brand, localized bool) (apicontract.Brand, error) {
	result := apicontract.Brand{Id: int(value.ID), Name: value.Name, Slug: value.Slug, Description: value.Description, IsActive: value.IsActive}
	if e.media != nil {
		if url, err := e.media.BrandLogoURL(value.ID); err == nil && url != "" {
			result.LogoUrl = &url
		}
	}
	if localized {
		resolution, err := e.resolveEntityLocalization(ctx, localizationservice.EntityTypeBrand, value.ID)
		if err != nil {
			return apicontract.Brand{}, err
		}
		result.Name = resolution.Fields["name"]
		if description, ok := resolution.Fields["description"]; ok {
			result.Description = &description
		} else {
			result.Description = nil
		}
		contract := contractEntityResolution(resolution)
		result.Localization = &contract
	}
	return result, nil
}

func (e *CatalogEndpoints) ListCategories(ctx context.Context, _ apicontract.ListCategoriesRequestObject) (apicontract.ListCategoriesResponseObject, error) {
	values, err := e.catalogAdmin.ListCategories(ctx, false, "")
	if err != nil {
		return nil, err
	}
	valuesContract, err := e.categoriesContract(ctx, values, true)
	if err != nil {
		return nil, err
	}
	return apicontract.ListCategories200JSONResponse{Data: valuesContract}, nil
}

func (e *CatalogEndpoints) ListAdminCategories(ctx context.Context, request apicontract.ListAdminCategoriesRequestObject) (apicontract.ListAdminCategoriesResponseObject, error) {
	query, inactive := "", false
	if request.Params.Q != nil {
		query = *request.Params.Q
	}
	if request.Params.IncludeInactive != nil {
		inactive = *request.Params.IncludeInactive
	}
	values, err := e.catalogAdmin.ListCategories(ctx, inactive, query)
	if err != nil {
		return nil, err
	}
	valuesContract, err := e.categoriesContract(ctx, values, false)
	if err != nil {
		return nil, err
	}
	return apicontract.ListAdminCategories200JSONResponse{Data: valuesContract}, nil
}

func (e *CatalogEndpoints) CreateAdminCategory(ctx context.Context, request apicontract.CreateAdminCategoryRequestObject) (apicontract.CreateAdminCategoryResponseObject, error) {
	if request.Body == nil {
		return nil, errors.New("category body is required")
	}
	value, err := e.catalogAdmin.CreateCategory(ctx, *request.Body)
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	contract, err := e.categoryContract(ctx, value, false)
	if err != nil {
		return nil, err
	}
	return apicontract.CreateAdminCategory201JSONResponse(contract), nil
}

func (e *CatalogEndpoints) UpdateAdminCategory(ctx context.Context, request apicontract.UpdateAdminCategoryRequestObject) (apicontract.UpdateAdminCategoryResponseObject, error) {
	if request.Id < 1 || request.Body == nil {
		return nil, errors.New("valid category id and body are required")
	}
	value, err := e.catalogAdmin.UpdateCategory(ctx, uint(request.Id), *request.Body)
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	contract, err := e.categoryContract(ctx, value, false)
	if err != nil {
		return nil, err
	}
	return apicontract.UpdateAdminCategory200JSONResponse(contract), nil
}

func (e *CatalogEndpoints) DeleteAdminCategory(ctx context.Context, request apicontract.DeleteAdminCategoryRequestObject) (apicontract.DeleteAdminCategoryResponseObject, error) {
	if request.Id < 1 {
		return nil, errors.New("category id must be positive")
	}
	if err := e.catalogAdmin.DeleteCategory(ctx, uint(request.Id)); err != nil {
		return nil, catalogEndpointError(err)
	}
	return apicontract.DeleteAdminCategory200JSONResponse{Message: "Category deleted"}, nil
}

func (e *CatalogEndpoints) categoriesContract(ctx context.Context, values []models.Category, localized bool) ([]apicontract.Category, error) {
	result := make([]apicontract.Category, 0, len(values))
	for _, value := range values {
		contract, err := e.categoryContract(ctx, value, localized)
		if err != nil {
			return nil, err
		}
		result = append(result, contract)
	}
	return result, nil
}

func (e *CatalogEndpoints) categoryContract(ctx context.Context, value models.Category, localized bool) (apicontract.Category, error) {
	var parent *int
	if value.ParentID != nil {
		converted := int(*value.ParentID)
		parent = &converted
	}
	result := apicontract.Category{Id: int(value.ID), Name: value.Name, Slug: value.Slug, Description: value.Description, IsActive: value.IsActive, SortOrder: value.SortOrder, ParentId: parent, Path: value.Path, Depth: value.Depth}
	if localized {
		resolution, err := e.resolveEntityLocalization(ctx, localizationservice.EntityTypeCategory, value.ID)
		if err != nil {
			return apicontract.Category{}, err
		}
		result.Name = resolution.Fields["name"]
		if description, ok := resolution.Fields["description"]; ok {
			result.Description = &description
		} else {
			result.Description = nil
		}
		contract := contractEntityResolution(resolution)
		result.Localization = &contract
	}
	return result, nil
}

func (e *CatalogEndpoints) ListProductAttributes(ctx context.Context, _ apicontract.ListProductAttributesRequestObject) (apicontract.ListProductAttributesResponseObject, error) {
	values, err := e.catalogAdmin.ListAttributes(ctx, true)
	if err != nil {
		return nil, err
	}
	return apicontract.ListProductAttributes200JSONResponse{Data: attributesContract(values)}, nil
}

func (e *CatalogEndpoints) ListAdminProductAttributes(ctx context.Context, _ apicontract.ListAdminProductAttributesRequestObject) (apicontract.ListAdminProductAttributesResponseObject, error) {
	values, err := e.catalogAdmin.ListAttributes(ctx, false)
	if err != nil {
		return nil, err
	}
	return apicontract.ListAdminProductAttributes200JSONResponse{Data: attributesContract(values)}, nil
}

func (e *CatalogEndpoints) CreateAdminProductAttribute(ctx context.Context, request apicontract.CreateAdminProductAttributeRequestObject) (apicontract.CreateAdminProductAttributeResponseObject, error) {
	if request.Body == nil {
		return nil, errors.New("product attribute body is required")
	}
	value, err := e.catalogAdmin.CreateAttribute(ctx, *request.Body)
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	return apicontract.CreateAdminProductAttribute201JSONResponse(attributeContract(value)), nil
}

func (e *CatalogEndpoints) UpdateAdminProductAttribute(ctx context.Context, request apicontract.UpdateAdminProductAttributeRequestObject) (apicontract.UpdateAdminProductAttributeResponseObject, error) {
	if request.Id < 1 || request.Body == nil {
		return nil, errors.New("valid product attribute id and body are required")
	}
	value, err := e.catalogAdmin.UpdateAttribute(ctx, uint(request.Id), *request.Body)
	if err != nil {
		return nil, catalogEndpointError(err)
	}
	return apicontract.UpdateAdminProductAttribute200JSONResponse(attributeContract(value)), nil
}

func (e *CatalogEndpoints) DeleteAdminProductAttribute(ctx context.Context, request apicontract.DeleteAdminProductAttributeRequestObject) (apicontract.DeleteAdminProductAttributeResponseObject, error) {
	if request.Id < 1 {
		return nil, errors.New("product attribute id must be positive")
	}
	if err := e.catalogAdmin.DeleteAttribute(ctx, uint(request.Id)); err != nil {
		return nil, catalogEndpointError(err)
	}
	return apicontract.DeleteAdminProductAttribute200JSONResponse{Message: "Product attribute deleted"}, nil
}

func attributesContract(values []models.ProductAttribute) []apicontract.ProductAttributeDefinition {
	result := make([]apicontract.ProductAttributeDefinition, 0, len(values))
	for _, value := range values {
		result = append(result, attributeContract(value))
	}
	return result
}

func attributeContract(value models.ProductAttribute) apicontract.ProductAttributeDefinition {
	enums := []string(value.EnumValues)
	if enums == nil {
		enums = []string{}
	}
	return apicontract.ProductAttributeDefinition{Id: int(value.ID), Key: value.Key, Slug: value.Slug, Type: apicontract.ProductAttributeDefinitionType(value.Type), Filterable: value.Filterable, Sortable: value.Sortable, EnumValues: enums}
}
