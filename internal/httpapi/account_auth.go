package httpapi

import (
	"context"
	"errors"
	"net/http"

	"ecommerce/internal/apicontract"
	authservice "ecommerce/internal/services/auth"
)

type oidcLoginRedirectResponse struct{ location string }

func (response oidcLoginRedirectResponse) VisitOidcLoginResponse(w http.ResponseWriter) error {
	w.Header().Set("Location", response.location)
	w.WriteHeader(http.StatusFound)
	return nil
}

type oidcCallbackRedirectResponse struct{ location string }

func (response oidcCallbackRedirectResponse) VisitOidcCallbackResponse(w http.ResponseWriter) error {
	w.Header().Set("Location", response.location)
	w.WriteHeader(http.StatusFound)
	return nil
}

func (s *AccountEndpoints) GetAuthConfig(ctx context.Context, _ apicontract.GetAuthConfigRequestObject) (apicontract.GetAuthConfigResponseObject, error) {
	config, err := s.auth.Config(ctx)
	if err != nil {
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.GetAuthConfig500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	return apicontract.GetAuthConfig200JSONResponse{LocalSignInEnabled: config.LocalSignInEnabled, OidcEnabled: config.OIDCEnabled}, nil
}

func (s *AccountEndpoints) Register(ctx context.Context, request apicontract.RegisterRequestObject) (apicontract.RegisterResponseObject, error) {
	if request.Body == nil {
		problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "invalid_request", "A registration body is required.", nil))
		return apicontract.Register400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	session, err := s.auth.Register(ctx, authservice.RegisterInput{Username: request.Body.Username, Email: request.Body.Email, Password: request.Body.Password, Name: stringValue(request.Body.Name)})
	if err != nil {
		switch {
		case errors.Is(err, authservice.ErrEmailConflict), errors.Is(err, authservice.ErrUsernameConflict):
			problem := s.contractProblem(ctx, http.StatusConflict, problemError(http.StatusConflict, "account_conflict", err.Error(), err))
			return apicontract.Register409ApplicationProblemPlusJSONResponse{ConflictProblemApplicationProblemPlusJSONResponse: apicontract.ConflictProblemApplicationProblemPlusJSONResponse(problem)}, nil
		case errors.Is(err, authservice.ErrLocalSignInDisabled):
			return nil, authRouteNotFound(err)
		case errors.Is(err, authservice.ErrInvalidInput):
			problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "invalid_request", err.Error(), err))
			return apicontract.Register400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
		default:
			problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
			return apicontract.Register500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
	}
	token := session.Token
	body := apicontract.Register201JSONResponse{Body: apicontract.AuthResponse{Token: &token, User: modelUser(session.User)}}
	return registerSessionResponse{body: body, cookies: s.sessionCookies(token)}, nil
}

func (s *AccountEndpoints) Login(ctx context.Context, request apicontract.LoginRequestObject) (apicontract.LoginResponseObject, error) {
	if request.Body == nil {
		problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "invalid_request", "A login body is required.", nil))
		return apicontract.Login400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	session, err := s.auth.Login(ctx, authservice.LoginInput{Email: request.Body.Email, Password: request.Body.Password})
	if err != nil {
		switch {
		case errors.Is(err, authservice.ErrInvalidCredentials):
			problem := s.contractProblem(ctx, http.StatusUnauthorized, problemError(http.StatusUnauthorized, "invalid_credentials", "Invalid email or password.", err))
			return apicontract.Login401ApplicationProblemPlusJSONResponse{AuthenticationRequiredProblemApplicationProblemPlusJSONResponse: apicontract.AuthenticationRequiredProblemApplicationProblemPlusJSONResponse(problem)}, nil
		case errors.Is(err, authservice.ErrLocalSignInDisabled):
			return nil, authRouteNotFound(err)
		case errors.Is(err, authservice.ErrInvalidInput):
			problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "invalid_request", err.Error(), err))
			return apicontract.Login400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
		default:
			problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
			return apicontract.Login500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
	}
	token := session.Token
	body := apicontract.Login200JSONResponse{Body: apicontract.AuthResponse{Token: &token, User: modelUser(session.User)}}
	return loginSessionResponse{body: body, cookies: s.sessionCookies(token)}, nil
}

func (s *AccountEndpoints) Logout(ctx context.Context, _ apicontract.LogoutRequestObject) (apicontract.LogoutResponseObject, error) {
	if err := s.auth.Logout(ctx); err != nil {
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.Logout500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	return logoutSessionResponse{body: apicontract.Logout200JSONResponse{Body: apicontract.MessageResponse{Message: "Logged out"}}, cookies: s.clearSessionCookies()}, nil
}

func (s *AccountEndpoints) OidcLogin(ctx context.Context, request apicontract.OidcLoginRequestObject) (apicontract.OidcLoginResponseObject, error) {
	result, err := s.auth.OIDCLogin(ctx, authservice.OIDCLoginInput{RedirectPath: stringValue(request.Params.Redirect), JSONResponse: request.Params.ResponseFormat != nil})
	if err != nil {
		if errors.Is(err, authservice.ErrOIDCNotConfigured) {
			return nil, authRouteNotFound(err)
		}
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.OidcLogin500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	return oidcLoginRedirectResponse{location: result.AuthorizationURL}, nil
}

func (s *AccountEndpoints) OidcCallback(ctx context.Context, request apicontract.OidcCallbackRequestObject) (apicontract.OidcCallbackResponseObject, error) {
	config, err := s.auth.Config(ctx)
	if err != nil {
		return nil, err
	}
	if !config.OIDCEnabled {
		return nil, authRouteNotFound(authservice.ErrOIDCNotConfigured)
	}
	result, err := s.auth.OIDCCallback(ctx, authservice.OIDCCallbackInput{Code: stringValue(request.Params.Code), State: stringValue(request.Params.State), JSONResponse: request.Params.Format != nil})
	if err != nil {
		switch {
		case errors.Is(err, authservice.ErrOIDCNotConfigured):
			return nil, authRouteNotFound(err)
		case errors.Is(err, authservice.ErrMissingOIDCCode):
			problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "invalid_oidc_callback", err.Error(), err))
			return apicontract.OidcCallback400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
		case errors.Is(err, authservice.ErrInvalidOIDCState), errors.Is(err, authservice.ErrInvalidCredentials):
			problem := s.contractProblem(ctx, http.StatusUnauthorized, problemError(http.StatusUnauthorized, "invalid_oidc_callback", "The OIDC callback could not be authenticated.", err))
			return apicontract.OidcCallback401ApplicationProblemPlusJSONResponse{AuthenticationRequiredProblemApplicationProblemPlusJSONResponse: apicontract.AuthenticationRequiredProblemApplicationProblemPlusJSONResponse(problem)}, nil
		default:
			problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
			return apicontract.OidcCallback500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
	}
	if result.JSONResponse {
		token := result.Session.Token
		body := apicontract.OidcCallback200JSONResponse{Body: apicontract.AuthResponse{Token: &token, User: modelUser(result.Session.User)}}
		return oidcCallbackJSONSessionResponse{body: body, cookies: s.sessionCookies(token)}, nil
	}
	return oidcCallbackRedirectSessionResponse{location: result.RedirectPath, cookies: s.sessionCookies(result.Session.Token)}, nil
}

func authRouteNotFound(cause error) error {
	return problemError(http.StatusNotFound, "not_found", "The requested authentication route is unavailable.", cause)
}
