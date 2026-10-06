package httpapi

import (
	"context"
	"errors"
	"net/http"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/requestctx"
	searchservice "ecommerce/internal/search"
	"ecommerce/models"
)

func (e *CatalogEndpoints) ListAdminSearchSynonyms(ctx context.Context, _ apicontract.ListAdminSearchSynonymsRequestObject) (apicontract.ListAdminSearchSynonymsResponseObject, error) {
	values, err := e.search.ListSynonymSets(ctx)
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	result := make([]apicontract.SearchSynonymSet, 0, len(values))
	for _, value := range values {
		result = append(result, searchSynonymSetContract(value))
	}
	return apicontract.ListAdminSearchSynonyms200JSONResponse{Data: result}, nil
}

func (e *CatalogEndpoints) CreateAdminSearchSynonym(ctx context.Context, request apicontract.CreateAdminSearchSynonymRequestObject) (apicontract.CreateAdminSearchSynonymResponseObject, error) {
	if request.Body == nil {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "A synonym set body is required.", nil)
	}
	active := true
	if request.Body.IsActive != nil {
		active = *request.Body.IsActive
	}
	value, err := e.search.CreateSynonymSet(ctx, searchservice.SynonymSetInput{
		Name: request.Body.Name, Direction: string(request.Body.Direction), Terms: request.Body.Terms,
		IsActive: active, UpdatedBy: searchConfigurationActorID(ctx),
	})
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	return apicontract.CreateAdminSearchSynonym201JSONResponse(searchSynonymSetContract(value)), nil
}

func (e *CatalogEndpoints) GetAdminSearchSynonym(ctx context.Context, request apicontract.GetAdminSearchSynonymRequestObject) (apicontract.GetAdminSearchSynonymResponseObject, error) {
	if request.Id < 1 {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "Synonym set ID must be positive.", nil)
	}
	value, err := e.search.GetSynonymSet(ctx, uint(request.Id))
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	return apicontract.GetAdminSearchSynonym200JSONResponse(searchSynonymSetContract(value)), nil
}

func (e *CatalogEndpoints) UpdateAdminSearchSynonym(ctx context.Context, request apicontract.UpdateAdminSearchSynonymRequestObject) (apicontract.UpdateAdminSearchSynonymResponseObject, error) {
	if request.Id < 1 || request.Body == nil {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "A valid synonym set ID and body are required.", nil)
	}
	current, err := e.search.GetSynonymSet(ctx, uint(request.Id))
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	input := searchservice.SynonymSetInput{
		Name: current.Name, Direction: current.Direction, Terms: current.Terms,
		IsActive: current.IsActive, UpdatedBy: searchConfigurationActorID(ctx),
	}
	if request.Body.Name != nil {
		input.Name = *request.Body.Name
	}
	if request.Body.Direction != nil {
		input.Direction = string(*request.Body.Direction)
	}
	if request.Body.Terms != nil {
		input.Terms = *request.Body.Terms
	}
	if request.Body.IsActive != nil {
		input.IsActive = *request.Body.IsActive
	}
	value, err := e.search.UpdateSynonymSet(ctx, uint(request.Id), input)
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	return apicontract.UpdateAdminSearchSynonym200JSONResponse(searchSynonymSetContract(value)), nil
}

func (e *CatalogEndpoints) DeleteAdminSearchSynonym(ctx context.Context, request apicontract.DeleteAdminSearchSynonymRequestObject) (apicontract.DeleteAdminSearchSynonymResponseObject, error) {
	if request.Id < 1 {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "Synonym set ID must be positive.", nil)
	}
	if err := e.search.DeleteSynonymSet(ctx, uint(request.Id)); err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	return apicontract.DeleteAdminSearchSynonym204Response{}, nil
}

func (e *CatalogEndpoints) ListAdminSearchTypoProfiles(ctx context.Context, _ apicontract.ListAdminSearchTypoProfilesRequestObject) (apicontract.ListAdminSearchTypoProfilesResponseObject, error) {
	values, err := e.search.ListTypoToleranceProfiles(ctx)
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	result := make([]apicontract.SearchTypoToleranceProfile, 0, len(values))
	for _, value := range values {
		result = append(result, searchTypoProfileContract(value))
	}
	return apicontract.ListAdminSearchTypoProfiles200JSONResponse{Data: result}, nil
}

func (e *CatalogEndpoints) CreateAdminSearchTypoProfile(ctx context.Context, request apicontract.CreateAdminSearchTypoProfileRequestObject) (apicontract.CreateAdminSearchTypoProfileResponseObject, error) {
	if request.Body == nil {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "A typo tolerance profile body is required.", nil)
	}
	input := searchservice.TypoToleranceProfileInput{
		Name: request.Body.Name, MinimumTokenLength: searchservice.DefaultMinimumTokenLength,
		OneEditMinimumLength: searchservice.DefaultOneEditMinimumLength,
		TwoEditMinimumLength: searchservice.DefaultTwoEditMinimumLength,
		UpdatedBy:            searchConfigurationActorID(ctx),
	}
	if request.Body.MinimumTokenLength != nil {
		input.MinimumTokenLength = *request.Body.MinimumTokenLength
	}
	if request.Body.OneEditMinimumLength != nil {
		input.OneEditMinimumLength = *request.Body.OneEditMinimumLength
	}
	if request.Body.TwoEditMinimumLength != nil {
		input.TwoEditMinimumLength = *request.Body.TwoEditMinimumLength
	}
	if request.Body.StrictMode != nil {
		input.StrictMode = *request.Body.StrictMode
	}
	if request.Body.IsActive != nil {
		input.IsActive = *request.Body.IsActive
	}
	value, err := e.search.CreateTypoToleranceProfile(ctx, input)
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	return apicontract.CreateAdminSearchTypoProfile201JSONResponse(searchTypoProfileContract(value)), nil
}

func (e *CatalogEndpoints) GetAdminSearchTypoProfile(ctx context.Context, request apicontract.GetAdminSearchTypoProfileRequestObject) (apicontract.GetAdminSearchTypoProfileResponseObject, error) {
	if request.Id < 1 {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "Typo tolerance profile ID must be positive.", nil)
	}
	value, err := e.search.GetTypoToleranceProfile(ctx, uint(request.Id))
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	return apicontract.GetAdminSearchTypoProfile200JSONResponse(searchTypoProfileContract(value)), nil
}

func (e *CatalogEndpoints) UpdateAdminSearchTypoProfile(ctx context.Context, request apicontract.UpdateAdminSearchTypoProfileRequestObject) (apicontract.UpdateAdminSearchTypoProfileResponseObject, error) {
	if request.Id < 1 || request.Body == nil {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "A valid typo tolerance profile ID and body are required.", nil)
	}
	current, err := e.search.GetTypoToleranceProfile(ctx, uint(request.Id))
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	input := searchservice.TypoToleranceProfileInput{
		Name: current.Name, MinimumTokenLength: current.MinimumTokenLength,
		OneEditMinimumLength: current.OneEditMinimumLength, TwoEditMinimumLength: current.TwoEditMinimumLength,
		StrictMode: current.StrictMode, IsActive: current.IsActive, UpdatedBy: searchConfigurationActorID(ctx),
	}
	if request.Body.Name != nil {
		input.Name = *request.Body.Name
	}
	if request.Body.MinimumTokenLength != nil {
		input.MinimumTokenLength = *request.Body.MinimumTokenLength
	}
	if request.Body.OneEditMinimumLength != nil {
		input.OneEditMinimumLength = *request.Body.OneEditMinimumLength
	}
	if request.Body.TwoEditMinimumLength != nil {
		input.TwoEditMinimumLength = *request.Body.TwoEditMinimumLength
	}
	if request.Body.StrictMode != nil {
		input.StrictMode = *request.Body.StrictMode
	}
	if request.Body.IsActive != nil {
		input.IsActive = *request.Body.IsActive
	}
	value, err := e.search.UpdateTypoToleranceProfile(ctx, uint(request.Id), input)
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	return apicontract.UpdateAdminSearchTypoProfile200JSONResponse(searchTypoProfileContract(value)), nil
}

func (e *CatalogEndpoints) DeleteAdminSearchTypoProfile(ctx context.Context, request apicontract.DeleteAdminSearchTypoProfileRequestObject) (apicontract.DeleteAdminSearchTypoProfileResponseObject, error) {
	if request.Id < 1 {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "Typo tolerance profile ID must be positive.", nil)
	}
	if err := e.search.DeleteTypoToleranceProfile(ctx, uint(request.Id)); err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	return apicontract.DeleteAdminSearchTypoProfile204Response{}, nil
}

func searchSynonymSetContract(value searchservice.SynonymSet) apicontract.SearchSynonymSet {
	return apicontract.SearchSynonymSet{
		Id: int(value.ID), Name: value.Name, Direction: apicontract.SearchSynonymDirection(value.Direction),
		Terms: value.Terms, IsActive: value.IsActive, UpdatedBy: searchConfigurationUpdatedBy(value.UpdatedBy),
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func searchTypoProfileContract(value models.SearchTypoToleranceProfile) apicontract.SearchTypoToleranceProfile {
	return apicontract.SearchTypoToleranceProfile{
		Id: int(value.ID), Name: value.Name, MinimumTokenLength: value.MinimumTokenLength,
		OneEditMinimumLength: value.OneEditMinimumLength, TwoEditMinimumLength: value.TwoEditMinimumLength,
		StrictMode: value.StrictMode, IsActive: value.IsActive, UpdatedBy: searchConfigurationUpdatedBy(value.UpdatedBy),
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func searchConfigurationActorID(ctx context.Context) *uint {
	principal, ok := requestctx.PrincipalFrom(ctx)
	if !ok || principal.AccountID == 0 {
		return nil
	}
	value := principal.AccountID
	return &value
}

func searchConfigurationUpdatedBy(value *uint) *int {
	if value == nil {
		return nil
	}
	result := int(*value)
	return &result
}

func searchConfigurationEndpointError(err error) error {
	switch {
	case errors.Is(err, searchservice.ErrConfigurationInvalid):
		return problemError(http.StatusUnprocessableEntity, "invalid_search_configuration", err.Error(), err)
	case errors.Is(err, searchservice.ErrConfigurationNotFound):
		return problemError(http.StatusNotFound, "search_configuration_not_found", "The requested search configuration was not found.", err)
	case errors.Is(err, searchservice.ErrConfigurationConflict), errors.Is(err, searchservice.ErrActiveTypoProfileRequired), errors.Is(err, searchservice.ErrDefaultRankingProfileRequired):
		return problemError(http.StatusConflict, "search_configuration_conflict", err.Error(), err)
	default:
		return catalogEndpointError(err)
	}
}
