package httpapi

import (
	"context"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/requestctx"
	accountservice "ecommerce/internal/services/account"
	"ecommerce/internal/services/accountdata"
	authservice "ecommerce/internal/services/auth"
	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type embeddedAccountEndpoints struct{ *AccountEndpoints }

var _ AccountStrictServer = (*embeddedAccountEndpoints)(nil)

type stubAccountAuth struct {
	registerInput authservice.RegisterInput
}

func (s *stubAccountAuth) Config(context.Context) (authservice.Config, error) {
	return authservice.Config{LocalSignInEnabled: true}, nil
}
func (s *stubAccountAuth) Register(_ context.Context, input authservice.RegisterInput) (authservice.Session, error) {
	s.registerInput = input
	return authservice.Session{Token: "token", User: models.User{BaseModel: models.BaseModel{ID: 3}, Subject: "subject-3", Username: input.Username, Email: input.Email, Name: input.Name, Role: "customer", Currency: "USD"}}, nil
}
func (s *stubAccountAuth) Login(context.Context, authservice.LoginInput) (authservice.Session, error) {
	return authservice.Session{}, nil
}
func (s *stubAccountAuth) Logout(context.Context) error { return nil }
func (s *stubAccountAuth) OIDCLogin(context.Context, authservice.OIDCLoginInput) (authservice.OIDCLoginResult, error) {
	return authservice.OIDCLoginResult{AuthorizationURL: "https://issuer.example/authorize"}, nil
}
func (s *stubAccountAuth) OIDCCallback(context.Context, authservice.OIDCCallbackInput) (authservice.OIDCCallbackResult, error) {
	return authservice.OIDCCallbackResult{}, nil
}

type stubAccounts struct {
	listInput accountservice.ListUsersInput
}

func (s *stubAccounts) GetProfile(_ context.Context, subject string) (models.User, error) {
	return models.User{BaseModel: models.BaseModel{ID: 7}, Subject: subject, Username: "ada", Email: "ada@example.com", Role: "customer", Currency: "USD"}, nil
}
func (s *stubAccounts) UpdateProfile(context.Context, string, accountservice.UpdateProfileInput) (models.User, error) {
	return models.User{}, nil
}
func (s *stubAccounts) ListUsers(_ context.Context, input accountservice.ListUsersInput) (accountservice.UserPage, error) {
	s.listInput = input
	return accountservice.UserPage{Users: []models.User{{BaseModel: models.BaseModel{ID: 9}, Subject: "subject-9", Username: "grace", Email: "grace@example.com", Role: "admin", Currency: "USD"}}, Page: input.Page, Limit: input.Limit, Total: 1, TotalPages: 1}, nil
}
func (s *stubAccounts) UpdateUserRole(context.Context, uint, string) (models.User, error) {
	return models.User{}, nil
}
func (s *stubAccounts) GetWebsiteSettings(context.Context) (models.WebsiteSettings, error) {
	return models.WebsiteSettings{ID: models.WebsiteSettingsSingletonID, SiteTitle: "Shop"}, nil
}
func (s *stubAccounts) UpdateWebsiteSettings(context.Context, accountservice.WebsiteSettingsInput) (models.WebsiteSettings, error) {
	return models.WebsiteSettings{}, nil
}

type stubAccountData struct {
	addressUserID uint
	addressInput  accountdata.CreateSavedAddressInput
}

func (s *stubAccountData) ListSavedAddresses(context.Context, uint) ([]models.SavedAddress, error) {
	return []models.SavedAddress{}, nil
}
func (s *stubAccountData) CreateSavedAddress(_ context.Context, userID uint, input accountdata.CreateSavedAddressInput) (models.SavedAddress, error) {
	s.addressUserID, s.addressInput = userID, input
	return models.SavedAddress{BaseModel: models.BaseModel{ID: 12}, UserID: userID, FullName: input.FullName, Line1: input.Line1, City: input.City, PostalCode: input.PostalCode, Country: input.Country}, nil
}
func (s *stubAccountData) DeleteSavedAddress(context.Context, uint, uint) error { return nil }
func (s *stubAccountData) SetDefaultAddress(context.Context, uint, uint) (models.SavedAddress, error) {
	return models.SavedAddress{}, nil
}
func (s *stubAccountData) ListSavedPaymentMethods(context.Context, uint) ([]models.SavedPaymentMethod, error) {
	return []models.SavedPaymentMethod{}, nil
}
func (s *stubAccountData) CreateSavedPaymentMethod(context.Context, uint, accountdata.CreateSavedPaymentMethodInput) (models.SavedPaymentMethod, error) {
	return models.SavedPaymentMethod{}, nil
}
func (s *stubAccountData) DeleteSavedPaymentMethod(context.Context, uint, uint) error {
	return nil
}
func (s *stubAccountData) SetDefaultPaymentMethod(context.Context, uint, uint) (models.SavedPaymentMethod, error) {
	return models.SavedPaymentMethod{}, nil
}

func newStubAccountEndpoints(t *testing.T) (*AccountEndpoints, *stubAccountAuth, *stubAccounts, *stubAccountData) {
	t.Helper()
	auth := &stubAccountAuth{}
	accounts := &stubAccounts{}
	data := &stubAccountData{}
	endpoints, err := NewAccountEndpoints(AccountEndpointsOptions{Auth: auth, Accounts: accounts, AccountData: data})
	require.NoError(t, err)
	return endpoints, auth, accounts, data
}

func TestAccountStrictServerOwnsExactGeneratedOperationSubset(t *testing.T) {
	typeOfSubset := reflect.TypeOf((*AccountStrictServer)(nil)).Elem()
	actual := make([]string, 0, typeOfSubset.NumMethod())
	for i := 0; i < typeOfSubset.NumMethod(); i++ {
		actual = append(actual, typeOfSubset.Method(i).Name)
	}
	sort.Strings(actual)
	expected := []string{
		"CreateSavedAddress", "CreateSavedPaymentMethod", "DeleteSavedAddress", "DeleteSavedPaymentMethod",
		"GetAdminPreview", "GetAdminWebsiteSettings", "GetAuthConfig", "GetProfile", "ListSavedAddresses", "ListSavedPaymentMethods",
		"ListUsers", "Login", "Logout", "OidcCallback", "OidcLogin", "Register", "SetDefaultAddress",
		"SetDefaultPaymentMethod", "StartAdminPreview", "StopAdminPreview", "UpdateProfile", "UpdateUserRole", "UpdateWebsiteSettings",
	}
	sort.Strings(expected)
	assert.Equal(t, expected, actual)
}

func TestNewAccountEndpointsRequiresEveryService(t *testing.T) {
	_, err := NewAccountEndpoints(AccountEndpointsOptions{})
	require.EqualError(t, err, "account auth service is required")

	_, err = NewAccountEndpoints(AccountEndpointsOptions{Auth: &stubAccountAuth{}})
	require.EqualError(t, err, "account service is required")

	_, err = NewAccountEndpoints(AccountEndpointsOptions{Auth: &stubAccountAuth{}, Accounts: &stubAccounts{}})
	require.EqualError(t, err, "account data service is required")
}

func TestAccountEndpointsUseGeneratedBodyAndQueryValues(t *testing.T) {
	endpoints, auth, accounts, data := newStubAccountEndpoints(t)
	name := "Ada Lovelace"
	response, err := endpoints.Register(context.Background(), apicontract.RegisterRequestObject{Body: &apicontract.RegisterRequest{Username: "ada", Email: "ada@example.com", Password: "password", Name: &name}})
	require.NoError(t, err)
	_, ok := response.(registerSessionResponse)
	require.True(t, ok)
	assert.Equal(t, authservice.RegisterInput{Username: "ada", Email: "ada@example.com", Password: "password", Name: name}, auth.registerInput)

	page, limit, query := 2, 25, "grace"
	usersResponse, err := endpoints.ListUsers(context.Background(), apicontract.ListUsersRequestObject{Params: apicontract.ListUsersParams{Page: &page, Limit: &limit, Q: &query}})
	require.NoError(t, err)
	_, ok = usersResponse.(apicontract.ListUsers200JSONResponse)
	require.True(t, ok)
	assert.Equal(t, accountservice.ListUsersInput{Page: 2, Limit: 25, Query: "grace"}, accounts.listInput)

	ctx := requestctx.WithPrincipal(context.Background(), requestctx.Principal{Subject: "subject-7"})
	setDefault := true
	addressResponse, err := endpoints.CreateSavedAddress(ctx, apicontract.CreateSavedAddressRequestObject{Body: &apicontract.CreateSavedAddressRequest{FullName: name, Line1: "123 Main", City: "Portland", PostalCode: "97201", Country: "US", SetDefault: &setDefault}})
	require.NoError(t, err)
	_, ok = addressResponse.(apicontract.CreateSavedAddress201JSONResponse)
	require.True(t, ok)
	assert.EqualValues(t, 7, data.addressUserID)
	assert.True(t, data.addressInput.SetDefault)
	assert.Equal(t, "123 Main", data.addressInput.Line1)
}

func TestAccountEndpointsRegisterWritesSessionAndCSRFCookies(t *testing.T) {
	endpoints, _, _, _ := newStubAccountEndpoints(t)
	name := "Ada Lovelace"
	response, err := endpoints.Register(context.Background(), apicontract.RegisterRequestObject{Body: &apicontract.RegisterRequest{Username: "ada", Email: "ada@example.com", Password: "password", Name: &name}})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	require.NoError(t, response.VisitRegisterResponse(recorder))
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 2)
	assert.Equal(t, "session_token", cookies[0].Name)
	assert.Equal(t, "token", cookies[0].Value)
	assert.True(t, cookies[0].HttpOnly)
	assert.Equal(t, "csrf_token", cookies[1].Name)
	assert.NotEmpty(t, cookies[1].Value)
	assert.False(t, cookies[1].HttpOnly)
	assert.Len(t, recorder.Header().Values("Set-Cookie"), 2)
}

func TestAccountEndpointsOIDCLoginPreservesRedirectLocation(t *testing.T) {
	endpoints, _, _, _ := newStubAccountEndpoints(t)
	response, err := endpoints.OidcLogin(context.Background(), apicontract.OidcLoginRequestObject{})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	require.NoError(t, response.VisitOidcLoginResponse(recorder))
	assert.Equal(t, 302, recorder.Code)
	assert.Equal(t, "https://issuer.example/authorize", recorder.Header().Get("Location"))
}

func TestAccountEndpointsReturnTypedProblemWithoutPrincipal(t *testing.T) {
	endpoints, _, _, _ := newStubAccountEndpoints(t)
	response, err := endpoints.GetProfile(context.Background(), apicontract.GetProfileRequestObject{})
	require.NoError(t, err)
	problemResponse, ok := response.(apicontract.GetProfile401ApplicationProblemPlusJSONResponse)
	require.True(t, ok)
	problem := apicontract.Problem(problemResponse.AuthenticationRequiredProblemApplicationProblemPlusJSONResponse)
	assert.Equal(t, int32(401), problem.Status)
	assert.Equal(t, "authentication_required", problem.Code)
}
