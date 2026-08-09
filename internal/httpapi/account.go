package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/requestctx"
	accountservice "ecommerce/internal/services/account"
	"ecommerce/internal/services/accountdata"
	authservice "ecommerce/internal/services/auth"
	"ecommerce/models"
)

// AccountStrictServer is the generated strict subset owned by AccountEndpoints.
// Embedding *AccountEndpoints in a larger server promotes this complete route family.
type AccountStrictServer interface {
	GetAdminWebsiteSettings(context.Context, apicontract.GetAdminWebsiteSettingsRequestObject) (apicontract.GetAdminWebsiteSettingsResponseObject, error)
	UpdateWebsiteSettings(context.Context, apicontract.UpdateWebsiteSettingsRequestObject) (apicontract.UpdateWebsiteSettingsResponseObject, error)
	ListUsers(context.Context, apicontract.ListUsersRequestObject) (apicontract.ListUsersResponseObject, error)
	UpdateUserRole(context.Context, apicontract.UpdateUserRoleRequestObject) (apicontract.UpdateUserRoleResponseObject, error)
	GetAuthConfig(context.Context, apicontract.GetAuthConfigRequestObject) (apicontract.GetAuthConfigResponseObject, error)
	Login(context.Context, apicontract.LoginRequestObject) (apicontract.LoginResponseObject, error)
	Logout(context.Context, apicontract.LogoutRequestObject) (apicontract.LogoutResponseObject, error)
	OidcCallback(context.Context, apicontract.OidcCallbackRequestObject) (apicontract.OidcCallbackResponseObject, error)
	OidcLogin(context.Context, apicontract.OidcLoginRequestObject) (apicontract.OidcLoginResponseObject, error)
	Register(context.Context, apicontract.RegisterRequestObject) (apicontract.RegisterResponseObject, error)
	GetProfile(context.Context, apicontract.GetProfileRequestObject) (apicontract.GetProfileResponseObject, error)
	UpdateProfile(context.Context, apicontract.UpdateProfileRequestObject) (apicontract.UpdateProfileResponseObject, error)
	ListSavedAddresses(context.Context, apicontract.ListSavedAddressesRequestObject) (apicontract.ListSavedAddressesResponseObject, error)
	CreateSavedAddress(context.Context, apicontract.CreateSavedAddressRequestObject) (apicontract.CreateSavedAddressResponseObject, error)
	DeleteSavedAddress(context.Context, apicontract.DeleteSavedAddressRequestObject) (apicontract.DeleteSavedAddressResponseObject, error)
	SetDefaultAddress(context.Context, apicontract.SetDefaultAddressRequestObject) (apicontract.SetDefaultAddressResponseObject, error)
	ListSavedPaymentMethods(context.Context, apicontract.ListSavedPaymentMethodsRequestObject) (apicontract.ListSavedPaymentMethodsResponseObject, error)
	CreateSavedPaymentMethod(context.Context, apicontract.CreateSavedPaymentMethodRequestObject) (apicontract.CreateSavedPaymentMethodResponseObject, error)
	DeleteSavedPaymentMethod(context.Context, apicontract.DeleteSavedPaymentMethodRequestObject) (apicontract.DeleteSavedPaymentMethodResponseObject, error)
	SetDefaultPaymentMethod(context.Context, apicontract.SetDefaultPaymentMethodRequestObject) (apicontract.SetDefaultPaymentMethodResponseObject, error)
	GetAdminPreview(context.Context, apicontract.GetAdminPreviewRequestObject) (apicontract.GetAdminPreviewResponseObject, error)
	StartAdminPreview(context.Context, apicontract.StartAdminPreviewRequestObject) (apicontract.StartAdminPreviewResponseObject, error)
	StopAdminPreview(context.Context, apicontract.StopAdminPreviewRequestObject) (apicontract.StopAdminPreviewResponseObject, error)
}

type AccountAuthService interface {
	Config(context.Context) (authservice.Config, error)
	Register(context.Context, authservice.RegisterInput) (authservice.Session, error)
	Login(context.Context, authservice.LoginInput) (authservice.Session, error)
	Logout(context.Context) error
	OIDCLogin(context.Context, authservice.OIDCLoginInput) (authservice.OIDCLoginResult, error)
	OIDCCallback(context.Context, authservice.OIDCCallbackInput) (authservice.OIDCCallbackResult, error)
}

type AccountService interface {
	GetProfile(context.Context, string) (models.User, error)
	UpdateProfile(context.Context, string, accountservice.UpdateProfileInput) (models.User, error)
	ListUsers(context.Context, accountservice.ListUsersInput) (accountservice.UserPage, error)
	UpdateUserRole(context.Context, uint, string) (models.User, error)
	GetWebsiteSettings(context.Context) (models.WebsiteSettings, error)
	UpdateWebsiteSettings(context.Context, accountservice.WebsiteSettingsInput) (models.WebsiteSettings, error)
}

type AccountDataService interface {
	ListSavedAddresses(context.Context, uint) ([]models.SavedAddress, error)
	CreateSavedAddress(context.Context, uint, accountdata.CreateSavedAddressInput) (models.SavedAddress, error)
	DeleteSavedAddress(context.Context, uint, uint) error
	SetDefaultAddress(context.Context, uint, uint) (models.SavedAddress, error)
	ListSavedPaymentMethods(context.Context, uint) ([]models.SavedPaymentMethod, error)
	CreateSavedPaymentMethod(context.Context, uint, accountdata.CreateSavedPaymentMethodInput) (models.SavedPaymentMethod, error)
	DeleteSavedPaymentMethod(context.Context, uint, uint) error
	SetDefaultPaymentMethod(context.Context, uint, uint) (models.SavedPaymentMethod, error)
}

type AccountEndpointsOptions struct {
	Auth        AccountAuthService
	Accounts    AccountService
	AccountData AccountDataService
	Renderer    Renderer
	JWTSecret   string
	Cookies     CookieConfig
}

type AccountEndpoints struct {
	auth        AccountAuthService
	accounts    AccountService
	accountData AccountDataService
	renderer    Renderer
	jwtSecret   string
	cookies     CookieConfig
}

var (
	_ AccountStrictServer = (*AccountEndpoints)(nil)
	_ AccountAuthService  = (*authservice.Service)(nil)
	_ AccountService      = (*accountservice.Service)(nil)
	_ AccountDataService  = (*accountdata.Service)(nil)
)

func NewAccountEndpoints(options AccountEndpointsOptions) (*AccountEndpoints, error) {
	if interfaceIsNil(options.Auth) {
		return nil, errors.New("account auth service is required")
	}
	if interfaceIsNil(options.Accounts) {
		return nil, errors.New("account service is required")
	}
	if interfaceIsNil(options.AccountData) {
		return nil, errors.New("account data service is required")
	}
	return &AccountEndpoints{
		auth: options.Auth, accounts: options.Accounts, accountData: options.AccountData,
		renderer: options.Renderer, jwtSecret: options.JWTSecret, cookies: options.Cookies.withDefaults(),
	}, nil
}

func (s *AccountEndpoints) principalUser(ctx context.Context) (models.User, error) {
	principal, err := requestctx.RequirePrincipal(ctx)
	if err != nil {
		return models.User{}, problemError(http.StatusUnauthorized, "authentication_required", "Authentication is required.", err)
	}
	user, err := s.accounts.GetProfile(ctx, principal.Subject)
	if errors.Is(err, accountservice.ErrUserNotFound) {
		return models.User{}, problemError(http.StatusUnauthorized, "authentication_required", "The authenticated account is unavailable.", err)
	}
	return user, err
}

func problemError(status int, code, detail string, cause error) error {
	return ErrorProblem(Problem{Status: status, Code: code, Detail: detail}, cause)
}

func (s *AccountEndpoints) contractProblem(ctx context.Context, status int, err error) apicontract.Problem {
	problem := s.renderer.FromError(ctx, status, err)
	contract := apicontract.Problem{Type: problem.Type, Title: problem.Title, Status: int32(problem.Status), Detail: problem.Detail, Code: problem.Code, CorrelationId: problem.CorrelationID}
	if problem.Instance != "" {
		contract.Instance = &problem.Instance
	}
	if len(problem.Errors) > 0 {
		issues := make([]apicontract.ValidationIssue, 0, len(problem.Errors))
		for _, issue := range problem.Errors {
			issues = append(issues, apicontract.ValidationIssue{Path: issue.Field, Code: issue.Code, Detail: issue.Message})
		}
		contract.Errors = &issues
	}
	return contract
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func modelUser(user models.User) apicontract.User {
	name := user.Name
	photo := user.ProfilePhoto
	var deletedAt *time.Time
	if user.DeletedAt.Valid {
		value := user.DeletedAt.Time
		deletedAt = &value
	}
	return apicontract.User{Id: int(user.ID), Subject: user.Subject, Username: user.Username, Email: user.Email, Name: &name, ProfilePhotoUrl: &photo, Role: apicontract.UserRole(user.Role), Currency: user.Currency, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt, DeletedAt: deletedAt}
}

func modelAddress(address models.SavedAddress) apicontract.SavedAddress {
	var deletedAt *time.Time
	if address.DeletedAt.Valid {
		value := address.DeletedAt.Time
		deletedAt = &value
	}
	return apicontract.SavedAddress{Id: int(address.ID), UserId: int(address.UserID), Label: address.Label, FullName: address.FullName, Line1: address.Line1, Line2: address.Line2, City: address.City, State: address.State, PostalCode: address.PostalCode, Country: address.Country, Phone: address.Phone, IsDefault: address.IsDefault, CreatedAt: address.CreatedAt, UpdatedAt: address.UpdatedAt, DeletedAt: deletedAt}
}

func modelPaymentMethod(method models.SavedPaymentMethod) apicontract.SavedPaymentMethod {
	var deletedAt *time.Time
	if method.DeletedAt.Valid {
		value := method.DeletedAt.Time
		deletedAt = &value
	}
	return apicontract.SavedPaymentMethod{Id: int(method.ID), UserId: int(method.UserID), Type: method.Type, Brand: method.Brand, Last4: method.Last4, ExpMonth: method.ExpMonth, ExpYear: method.ExpYear, CardholderName: method.CardholderName, Nickname: method.Nickname, IsDefault: method.IsDefault, CreatedAt: method.CreatedAt, UpdatedAt: method.UpdatedAt, DeletedAt: deletedAt}
}

func modelWebsiteSettings(settings models.WebsiteSettings) apicontract.WebsiteSettingsResponse {
	return apicontract.WebsiteSettingsResponse{Settings: apicontract.WebsiteSettings{SiteTitle: settings.SiteTitle, AllowGuestCheckout: settings.AllowGuestCheckout, CouponCodesEnabled: settings.CouponCodesEnabled, OidcProvider: settings.OIDCProvider, OidcClientId: settings.OIDCClientID, OidcClientSecret: "", OidcClientSecretConfigured: strings.TrimSpace(settings.OIDCClientSecretEnvelopeJSON) != "", ClearOidcClientSecret: false, OidcRedirectUri: settings.OIDCRedirectURI}, UpdatedAt: settings.UpdatedAt}
}
