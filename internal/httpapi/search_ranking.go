package httpapi

import (
	"context"
	"ecommerce/internal/apicontract"
	searchservice "ecommerce/internal/search"
	"fmt"
	"net/http"
)

func (e *CatalogEndpoints) ListAdminSearchRankingProfiles(ctx context.Context, _ apicontract.ListAdminSearchRankingProfilesRequestObject) (apicontract.ListAdminSearchRankingProfilesResponseObject, error) {
	values, err := e.search.ListRankingProfiles(ctx)
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	result := make([]apicontract.SearchRankingProfile, 0, len(values))
	for _, value := range values {
		result = append(result, searchRankingProfileContract(value))
	}
	return apicontract.ListAdminSearchRankingProfiles200JSONResponse{Data: result}, nil
}
func (e *CatalogEndpoints) CreateAdminSearchRankingProfile(ctx context.Context, request apicontract.CreateAdminSearchRankingProfileRequestObject) (apicontract.CreateAdminSearchRankingProfileResponseObject, error) {
	if request.Body == nil {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "A ranking profile body is required.", nil)
	}
	weights, err := searchRankingWeightsInput(request.Body.Weights)
	if err != nil {
		return nil, problemError(http.StatusBadRequest, "invalid_request", err.Error(), err)
	}
	input := searchservice.RankingProfileInput{Name: request.Body.Name, Weights: weights}
	if request.Body.IsDefault != nil {
		input.IsDefault = *request.Body.IsDefault
	}
	value, err := e.search.CreateRankingProfile(ctx, input, searchConfigurationActorID(ctx))
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	return apicontract.CreateAdminSearchRankingProfile201JSONResponse(searchRankingProfileContract(value)), nil
}
func (e *CatalogEndpoints) GetAdminSearchRankingProfile(ctx context.Context, request apicontract.GetAdminSearchRankingProfileRequestObject) (apicontract.GetAdminSearchRankingProfileResponseObject, error) {
	if request.Id < 1 {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "Ranking profile ID must be positive.", nil)
	}
	value, err := e.search.GetRankingProfile(ctx, uint(request.Id))
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	return apicontract.GetAdminSearchRankingProfile200JSONResponse(searchRankingProfileContract(value)), nil
}
func (e *CatalogEndpoints) UpdateAdminSearchRankingProfile(ctx context.Context, request apicontract.UpdateAdminSearchRankingProfileRequestObject) (apicontract.UpdateAdminSearchRankingProfileResponseObject, error) {
	if request.Id < 1 || request.Body == nil {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "A valid ranking profile ID and body are required.", nil)
	}
	input := searchservice.RankingProfilePatch{Name: request.Body.Name, IsDefault: request.Body.IsDefault}
	if request.Body.Weights != nil {
		weights, err := searchRankingWeightsInput(*request.Body.Weights)
		if err != nil {
			return nil, problemError(http.StatusBadRequest, "invalid_request", err.Error(), err)
		}
		input.Weights = &weights
	}
	value, err := e.search.UpdateRankingProfile(ctx, uint(request.Id), input, searchConfigurationActorID(ctx))
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	return apicontract.UpdateAdminSearchRankingProfile200JSONResponse(searchRankingProfileContract(value)), nil
}
func (e *CatalogEndpoints) DeleteAdminSearchRankingProfile(ctx context.Context, request apicontract.DeleteAdminSearchRankingProfileRequestObject) (apicontract.DeleteAdminSearchRankingProfileResponseObject, error) {
	if request.Id < 1 {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "Ranking profile ID must be positive.", nil)
	}
	if err := e.search.DeleteRankingProfile(ctx, uint(request.Id)); err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	return apicontract.DeleteAdminSearchRankingProfile204Response{}, nil
}
func searchRankingWeightsInput(value apicontract.SearchRankingWeightsInput) (searchservice.RankingWeights, error) {
	for _, weight := range []*float64{value.TokenCoverage, value.ExactPhrase, value.Name, value.Brand, value.Attributes, value.Recency, value.Availability, value.Sales} {
		if weight == nil {
			return searchservice.RankingWeights{}, fmt.Errorf("all eight ranking weights are required and must not be null")
		}
	}
	return searchservice.RankingWeights{TokenCoverage: *value.TokenCoverage, ExactPhrase: *value.ExactPhrase, Name: *value.Name, Brand: *value.Brand, Attributes: *value.Attributes, Recency: *value.Recency, Availability: *value.Availability, Sales: *value.Sales}, nil
}
func searchRankingProfileContract(value searchservice.RankingProfile) apicontract.SearchRankingProfile {
	weights := value.Weights
	return apicontract.SearchRankingProfile{Id: int(value.ID), Name: value.Name, IsDefault: value.IsDefault, Version: value.Version, UpdatedBy: searchConfigurationUpdatedBy(value.UpdatedBy), CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, Weights: apicontract.SearchRankingWeights{TokenCoverage: weights.TokenCoverage, ExactPhrase: weights.ExactPhrase, Name: weights.Name, Brand: weights.Brand, Attributes: weights.Attributes, Recency: weights.Recency, Availability: weights.Availability, Sales: weights.Sales}}
}
