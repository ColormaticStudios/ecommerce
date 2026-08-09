package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/requestctx"
	accountservice "ecommerce/internal/services/account"
)

func (s *AccountEndpoints) GetProfile(ctx context.Context, _ apicontract.GetProfileRequestObject) (apicontract.GetProfileResponseObject, error) {
	principal, err := requestctx.RequirePrincipal(ctx)
	if err != nil {
		problem := s.contractProblem(ctx, http.StatusUnauthorized, problemError(http.StatusUnauthorized, "authentication_required", "Authentication is required.", err))
		return apicontract.GetProfile401ApplicationProblemPlusJSONResponse{AuthenticationRequiredProblemApplicationProblemPlusJSONResponse: apicontract.AuthenticationRequiredProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	user, err := s.accounts.GetProfile(ctx, principal.Subject)
	if err != nil {
		if errors.Is(err, accountservice.ErrUserNotFound) {
			problem := s.contractProblem(ctx, http.StatusUnauthorized, problemError(http.StatusUnauthorized, "authentication_required", "The authenticated account is unavailable.", err))
			return apicontract.GetProfile401ApplicationProblemPlusJSONResponse{AuthenticationRequiredProblemApplicationProblemPlusJSONResponse: apicontract.AuthenticationRequiredProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.GetProfile500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	return apicontract.GetProfile200JSONResponse(modelUser(user)), nil
}

func (s *AccountEndpoints) UpdateProfile(ctx context.Context, request apicontract.UpdateProfileRequestObject) (apicontract.UpdateProfileResponseObject, error) {
	principal, err := requestctx.RequirePrincipal(ctx)
	if err != nil {
		problem := s.contractProblem(ctx, http.StatusUnauthorized, problemError(http.StatusUnauthorized, "authentication_required", "Authentication is required.", err))
		return apicontract.UpdateProfile401ApplicationProblemPlusJSONResponse{AuthenticationRequiredProblemApplicationProblemPlusJSONResponse: apicontract.AuthenticationRequiredProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	if request.Body == nil {
		problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "invalid_request", "A profile update body is required.", nil))
		return apicontract.UpdateProfile400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	user, err := s.accounts.UpdateProfile(ctx, principal.Subject, accountservice.UpdateProfileInput{Name: request.Body.Name, Currency: request.Body.Currency, ProfilePhotoURL: request.Body.ProfilePhotoUrl})
	if err != nil {
		switch {
		case errors.Is(err, accountservice.ErrInvalidCurrency):
			problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "invalid_currency", err.Error(), err))
			return apicontract.UpdateProfile400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
		case errors.Is(err, accountservice.ErrUserNotFound):
			problem := s.contractProblem(ctx, http.StatusUnauthorized, problemError(http.StatusUnauthorized, "authentication_required", "The authenticated account is unavailable.", err))
			return apicontract.UpdateProfile401ApplicationProblemPlusJSONResponse{AuthenticationRequiredProblemApplicationProblemPlusJSONResponse: apicontract.AuthenticationRequiredProblemApplicationProblemPlusJSONResponse(problem)}, nil
		default:
			problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
			return apicontract.UpdateProfile500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
	}
	return apicontract.UpdateProfile200JSONResponse(modelUser(user)), nil
}

func (s *AccountEndpoints) ListUsers(ctx context.Context, request apicontract.ListUsersRequestObject) (apicontract.ListUsersResponseObject, error) {
	page, limit := 1, 10
	if request.Params.Page != nil {
		page = *request.Params.Page
	}
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	if page < 1 || limit < 1 || limit > 100 {
		problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "invalid_pagination", "Pagination values are outside the supported range.", nil))
		return apicontract.ListUsers400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	result, err := s.accounts.ListUsers(ctx, accountservice.ListUsersInput{Page: page, Limit: limit, Query: strings.TrimSpace(stringValue(request.Params.Q))})
	if err != nil {
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.ListUsers500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	users := make([]apicontract.User, 0, len(result.Users))
	for _, user := range result.Users {
		users = append(users, modelUser(user))
	}
	return apicontract.ListUsers200JSONResponse{Data: users, Pagination: apicontract.Pagination{Page: result.Page, Limit: result.Limit, Total: int(result.Total), TotalPages: result.TotalPages}}, nil
}

func (s *AccountEndpoints) UpdateUserRole(ctx context.Context, request apicontract.UpdateUserRoleRequestObject) (apicontract.UpdateUserRoleResponseObject, error) {
	if request.Id < 1 || request.Body == nil {
		problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "invalid_request", "A valid user ID and role are required.", nil))
		return apicontract.UpdateUserRole400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	user, err := s.accounts.UpdateUserRole(ctx, uint(request.Id), string(request.Body.Role))
	if err != nil {
		if errors.Is(err, accountservice.ErrInvalidRole) || errors.Is(err, accountservice.ErrUserNotFound) {
			problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "invalid_user_role", err.Error(), err))
			return apicontract.UpdateUserRole400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.UpdateUserRole500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	return apicontract.UpdateUserRole200JSONResponse(modelUser(user)), nil
}

func (s *AccountEndpoints) GetAdminWebsiteSettings(ctx context.Context, _ apicontract.GetAdminWebsiteSettingsRequestObject) (apicontract.GetAdminWebsiteSettingsResponseObject, error) {
	settings, err := s.accounts.GetWebsiteSettings(ctx)
	if err != nil {
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.GetAdminWebsiteSettings500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	return apicontract.GetAdminWebsiteSettings200JSONResponse(modelWebsiteSettings(settings)), nil
}

func (s *AccountEndpoints) UpdateWebsiteSettings(ctx context.Context, request apicontract.UpdateWebsiteSettingsRequestObject) (apicontract.UpdateWebsiteSettingsResponseObject, error) {
	if request.Body == nil {
		problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "invalid_request", "A website settings body is required.", nil))
		return apicontract.UpdateWebsiteSettings400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	input := request.Body.Settings
	if input.ClearOidcClientSecret && strings.TrimSpace(input.OidcClientSecret) != "" {
		problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "invalid_oidc_secret_update", "OIDC client secret cannot be set and cleared in the same request.", nil))
		return apicontract.UpdateWebsiteSettings400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	settings, err := s.accounts.UpdateWebsiteSettings(ctx, accountservice.WebsiteSettingsInput{SiteTitle: input.SiteTitle, AllowGuestCheckout: input.AllowGuestCheckout, CouponCodesEnabled: input.CouponCodesEnabled, OIDCProvider: input.OidcProvider, OIDCClientID: input.OidcClientId, OIDCClientSecret: input.OidcClientSecret, ClearOIDCClientSecret: input.ClearOidcClientSecret, OIDCRedirectURI: input.OidcRedirectUri})
	if err != nil {
		if errors.Is(err, accountservice.ErrCredentialServiceUnconfigured) {
			problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "credential_encryption_unavailable", err.Error(), err))
			return apicontract.UpdateWebsiteSettings400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.UpdateWebsiteSettings500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	return apicontract.UpdateWebsiteSettings200JSONResponse(modelWebsiteSettings(settings)), nil
}
