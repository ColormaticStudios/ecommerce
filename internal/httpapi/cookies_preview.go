package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/requestctx"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	sessionCookieName      = "session_token"
	csrfCookieName         = "csrf_token"
	draftPreviewCookieName = "draft_preview_token"
)

const (
	sessionTTL      = 7 * 24 * time.Hour
	draftPreviewTTL = 30 * time.Minute
)

type CookieConfig struct {
	Secure   bool
	Domain   string
	SameSite http.SameSite
}

func (config CookieConfig) withDefaults() CookieConfig {
	if config.SameSite == 0 {
		config.SameSite = http.SameSiteLaxMode
	}
	return config
}

func (s *AccountEndpoints) sessionCookies(token string) []*http.Cookie {
	return []*http.Cookie{
		s.cookie(sessionCookieName, token, sessionTTL, true),
		s.cookie(csrfCookieName, uuid.NewString(), sessionTTL, false),
	}
}

func (s *AccountEndpoints) clearSessionCookies() []*http.Cookie {
	return []*http.Cookie{
		s.expiredCookie(sessionCookieName, true),
		s.expiredCookie(csrfCookieName, false),
	}
}

func (s *AccountEndpoints) cookie(name, value string, ttl time.Duration, httpOnly bool) *http.Cookie {
	return &http.Cookie{Name: name, Value: value, Path: "/", Domain: s.cookies.Domain, MaxAge: int(ttl.Seconds()), Expires: time.Now().Add(ttl), Secure: s.cookies.Secure, HttpOnly: httpOnly, SameSite: s.cookies.SameSite}
}

func (s *AccountEndpoints) expiredCookie(name string, httpOnly bool) *http.Cookie {
	return &http.Cookie{Name: name, Path: "/", Domain: s.cookies.Domain, MaxAge: -1, Expires: time.Unix(1, 0), Secure: s.cookies.Secure, HttpOnly: httpOnly, SameSite: s.cookies.SameSite}
}

func addResponseCookies(writer http.ResponseWriter, cookies []*http.Cookie) {
	for _, cookie := range cookies {
		http.SetCookie(writer, cookie)
	}
}

type registerSessionResponse struct {
	body    apicontract.Register201JSONResponse
	cookies []*http.Cookie
}

func (response registerSessionResponse) VisitRegisterResponse(writer http.ResponseWriter) error {
	addResponseCookies(writer, response.cookies)
	return writeJSONResponse(writer, http.StatusCreated, response.body.Body)
}

type loginSessionResponse struct {
	body    apicontract.Login200JSONResponse
	cookies []*http.Cookie
}

func (response loginSessionResponse) VisitLoginResponse(writer http.ResponseWriter) error {
	addResponseCookies(writer, response.cookies)
	return writeJSONResponse(writer, http.StatusOK, response.body.Body)
}

type logoutSessionResponse struct {
	body    apicontract.Logout200JSONResponse
	cookies []*http.Cookie
}

func (response logoutSessionResponse) VisitLogoutResponse(writer http.ResponseWriter) error {
	addResponseCookies(writer, response.cookies)
	return writeJSONResponse(writer, http.StatusOK, response.body.Body)
}

type oidcCallbackJSONSessionResponse struct {
	body    apicontract.OidcCallback200JSONResponse
	cookies []*http.Cookie
}

func (response oidcCallbackJSONSessionResponse) VisitOidcCallbackResponse(writer http.ResponseWriter) error {
	addResponseCookies(writer, response.cookies)
	return writeJSONResponse(writer, http.StatusOK, response.body.Body)
}

type oidcCallbackRedirectSessionResponse struct {
	location string
	cookies  []*http.Cookie
}

func (response oidcCallbackRedirectSessionResponse) VisitOidcCallbackResponse(writer http.ResponseWriter) error {
	addResponseCookies(writer, response.cookies)
	writer.Header().Set("Location", response.location)
	writer.WriteHeader(http.StatusFound)
	return nil
}

type draftPreviewClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func (s *AccountEndpoints) GetAdminPreview(ctx context.Context, _ apicontract.GetAdminPreviewRequestObject) (apicontract.GetAdminPreviewResponseObject, error) {
	metadata, _ := requestctx.MetadataFrom(ctx)
	token := metadata.Cookies[draftPreviewCookieName]
	if token == "" {
		return apicontract.GetAdminPreview200JSONResponse{Body: apicontract.DraftPreviewSessionResponse{Active: false}}, nil
	}
	claims, err := s.parseDraftPreviewToken(token)
	if err != nil {
		return getAdminPreviewCookieResponse{body: apicontract.GetAdminPreview200JSONResponse{Body: apicontract.DraftPreviewSessionResponse{Active: false}}, cookie: s.expiredCookie(draftPreviewCookieName, true)}, nil
	}
	expiresAt := claims.ExpiresAt.Time
	return apicontract.GetAdminPreview200JSONResponse{Body: apicontract.DraftPreviewSessionResponse{Active: true, ExpiresAt: &expiresAt}}, nil
}

func (s *AccountEndpoints) StartAdminPreview(ctx context.Context, _ apicontract.StartAdminPreviewRequestObject) (apicontract.StartAdminPreviewResponseObject, error) {
	principal, err := requestctx.RequirePrincipal(ctx)
	if err != nil || !principal.HasRole("admin") {
		return nil, problemError(http.StatusForbidden, "forbidden", "Administrator access is required.", err)
	}
	if s.jwtSecret == "" {
		return nil, errors.New("JWT secret is required for draft preview")
	}
	now := time.Now().UTC()
	expiresAt := now.Add(draftPreviewTTL)
	claims := draftPreviewClaims{Role: "admin", RegisteredClaims: jwt.RegisteredClaims{Subject: principal.Subject, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(expiresAt)}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, err
	}
	return startAdminPreviewCookieResponse{
		body:   apicontract.StartAdminPreview200JSONResponse{Body: apicontract.DraftPreviewSessionResponse{Active: true, ExpiresAt: &expiresAt}},
		cookie: s.cookie(draftPreviewCookieName, token, draftPreviewTTL, true),
	}, nil
}

func (s *AccountEndpoints) StopAdminPreview(context.Context, apicontract.StopAdminPreviewRequestObject) (apicontract.StopAdminPreviewResponseObject, error) {
	return stopAdminPreviewCookieResponse{body: apicontract.StopAdminPreview200JSONResponse{Body: apicontract.DraftPreviewSessionResponse{Active: false}}, cookie: s.expiredCookie(draftPreviewCookieName, true)}, nil
}

func (s *AccountEndpoints) parseDraftPreviewToken(tokenText string) (*draftPreviewClaims, error) {
	return parseDraftPreviewToken(tokenText, s.jwtSecret)
}

func validDraftPreviewToken(tokenText, secret string) bool {
	_, err := parseDraftPreviewToken(tokenText, secret)
	return err == nil
}

func parseDraftPreviewToken(tokenText, secret string) (*draftPreviewClaims, error) {
	claims := &draftPreviewClaims{}
	token, err := jwt.ParseWithClaims(tokenText, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid draft preview signing method")
		}
		return []byte(secret), nil
	}, jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid || claims.Subject == "" || claims.Role != "admin" || claims.ExpiresAt == nil {
		return nil, errors.New("invalid draft preview token")
	}
	return claims, nil
}

type getAdminPreviewCookieResponse struct {
	body   apicontract.GetAdminPreview200JSONResponse
	cookie *http.Cookie
}

func (response getAdminPreviewCookieResponse) VisitGetAdminPreviewResponse(writer http.ResponseWriter) error {
	http.SetCookie(writer, response.cookie)
	return writeJSONResponse(writer, http.StatusOK, response.body.Body)
}

type startAdminPreviewCookieResponse struct {
	body   apicontract.StartAdminPreview200JSONResponse
	cookie *http.Cookie
}

func (response startAdminPreviewCookieResponse) VisitStartAdminPreviewResponse(writer http.ResponseWriter) error {
	http.SetCookie(writer, response.cookie)
	return writeJSONResponse(writer, http.StatusOK, response.body.Body)
}

type stopAdminPreviewCookieResponse struct {
	body   apicontract.StopAdminPreview200JSONResponse
	cookie *http.Cookie
}

func (response stopAdminPreviewCookieResponse) VisitStopAdminPreviewResponse(writer http.ResponseWriter) error {
	http.SetCookie(writer, response.cookie)
	return writeJSONResponse(writer, http.StatusOK, response.body.Body)
}

func writeJSONResponse(writer http.ResponseWriter, status int, body any) error {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	return json.NewEncoder(writer).Encode(body)
}
