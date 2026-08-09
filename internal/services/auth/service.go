package auth

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	accountservice "ecommerce/internal/services/account"
	"ecommerce/models"

	"github.com/coreos/go-oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

var (
	ErrLocalSignInDisabled = errors.New("local sign in is disabled")
	ErrInvalidInput        = errors.New("invalid authentication input")
	ErrEmailConflict       = errors.New("email already registered")
	ErrUsernameConflict    = errors.New("username already taken")
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrOIDCNotConfigured   = errors.New("OIDC is not configured")
	ErrInvalidOIDCState    = errors.New("invalid OIDC state")
	ErrMissingOIDCCode     = errors.New("missing code in callback")
)

type WebsiteService interface {
	GetWebsiteSettings(ctx context.Context) (models.WebsiteSettings, error)
	OIDCClientSecret(ctx context.Context, settings models.WebsiteSettings) (string, error)
}

type Service struct {
	db                 *gorm.DB
	jwtSecret          string
	disableLocalSignIn bool
	website            WebsiteService
	statesMu           sync.Mutex
	states             map[string]oidcState
	now                func() time.Time
	newProvider        func(context.Context, string) (*oidc.Provider, error)
}

type RegisterInput struct {
	Username string
	Email    string
	Password string
	Name     string
}

type LoginInput struct {
	Email    string
	Password string
}

type Session struct {
	Token string
	User  models.User
}

type Config struct {
	LocalSignInEnabled bool
	OIDCEnabled        bool
}

type OIDCLoginInput struct {
	RedirectPath string
	JSONResponse bool
}

type OIDCLoginResult struct {
	AuthorizationURL string
	State            string
}

type OIDCCallbackInput struct {
	Code         string
	State        string
	JSONResponse bool
}

type OIDCCallbackResult struct {
	Session      Session
	RedirectPath string
	JSONResponse bool
}

type oidcState struct {
	RedirectPath string
	JSONResponse bool
	ExpiresAt    time.Time
}

type oidcUserClaims struct {
	Email             string `json:"email"`
	Sub               string `json:"sub"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
}

func NewService(db *gorm.DB, jwtSecret string, disableLocalSignIn bool, website WebsiteService) *Service {
	return &Service{
		db: db, jwtSecret: jwtSecret, disableLocalSignIn: disableLocalSignIn, website: website,
		states: map[string]oidcState{}, now: time.Now, newProvider: oidc.NewProvider,
	}
}

func (s *Service) Config(ctx context.Context) (Config, error) {
	settings, err := s.website.GetWebsiteSettings(ctx)
	if err != nil {
		return Config{}, err
	}
	return Config{LocalSignInEnabled: !s.disableLocalSignIn, OIDCEnabled: oidcConfigured(settings)}, nil
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (Session, error) {
	if s.disableLocalSignIn {
		return Session{}, ErrLocalSignInDisabled
	}
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Name = strings.TrimSpace(input.Name)
	if len(input.Username) < 3 || len(input.Username) > 50 || !validEmail(input.Email) || len(input.Password) < 6 {
		return Session{}, ErrInvalidInput
	}

	db := s.db.WithContext(ctx)
	var existing models.User
	if err := db.Where("email = ?", input.Email).First(&existing).Error; err == nil {
		return Session{}, ErrEmailConflict
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return Session{}, err
	}
	if err := db.Where("username = ?", input.Username).First(&existing).Error; err == nil {
		return Session{}, ErrUsernameConflict
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return Session{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return Session{}, err
	}
	user := models.User{Subject: uuid.NewSHA1(uuid.NameSpaceOID, []byte(input.Email)).String(), Username: input.Username, Email: input.Email, PasswordHash: string(hash), Name: input.Name, Role: "customer", Currency: "USD"}
	if err := db.Create(&user).Error; err != nil {
		return Session{}, err
	}
	return s.session(user)
}

func (s *Service) Login(ctx context.Context, input LoginInput) (Session, error) {
	if s.disableLocalSignIn {
		return Session{}, ErrLocalSignInDisabled
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if !validEmail(input.Email) || input.Password == "" {
		return Session{}, ErrInvalidInput
	}
	var user models.User
	if err := s.db.WithContext(ctx).Where("email = ?", input.Email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Session{}, ErrInvalidCredentials
		}
		return Session{}, err
	}
	if user.PasswordHash == "" || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)) != nil {
		return Session{}, ErrInvalidCredentials
	}
	return s.session(user)
}

func (s *Service) Logout(context.Context) error { return nil }

func (s *Service) OIDCLogin(ctx context.Context, input OIDCLoginInput) (OIDCLoginResult, error) {
	settings, err := s.website.GetWebsiteSettings(ctx)
	if err != nil {
		return OIDCLoginResult{}, err
	}
	if !oidcConfigured(settings) {
		return OIDCLoginResult{}, ErrOIDCNotConfigured
	}
	provider, err := s.newProvider(ctx, settings.OIDCProvider)
	if err != nil {
		return OIDCLoginResult{}, err
	}
	state := uuid.NewString()
	redirect := sanitizeRedirectPath(input.RedirectPath)
	s.statesMu.Lock()
	s.states[state] = oidcState{RedirectPath: redirect, JSONResponse: input.JSONResponse, ExpiresAt: s.now().Add(5 * time.Minute)}
	s.statesMu.Unlock()
	config := oauth2.Config{ClientID: settings.OIDCClientID, RedirectURL: settings.OIDCRedirectURI, Endpoint: provider.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "profile", "email"}}
	return OIDCLoginResult{AuthorizationURL: config.AuthCodeURL(state), State: state}, nil
}

func (s *Service) OIDCCallback(ctx context.Context, input OIDCCallbackInput) (OIDCCallbackResult, error) {
	if strings.TrimSpace(input.Code) == "" {
		return OIDCCallbackResult{}, ErrMissingOIDCCode
	}
	state, ok := s.takeState(strings.TrimSpace(input.State))
	if !ok {
		return OIDCCallbackResult{}, ErrInvalidOIDCState
	}
	settings, err := s.website.GetWebsiteSettings(ctx)
	if err != nil {
		return OIDCCallbackResult{}, err
	}
	if !oidcConfigured(settings) {
		return OIDCCallbackResult{}, ErrOIDCNotConfigured
	}
	secret, err := s.website.OIDCClientSecret(ctx, settings)
	if err != nil {
		return OIDCCallbackResult{}, err
	}
	provider, err := s.newProvider(ctx, settings.OIDCProvider)
	if err != nil {
		return OIDCCallbackResult{}, err
	}
	config := oauth2.Config{ClientID: settings.OIDCClientID, ClientSecret: secret, RedirectURL: settings.OIDCRedirectURI, Endpoint: provider.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "profile", "email"}}
	token, err := config.Exchange(ctx, input.Code)
	if err != nil {
		return OIDCCallbackResult{}, err
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return OIDCCallbackResult{}, errors.New("OIDC token response did not contain an id_token")
	}
	idToken, err := provider.Verifier(&oidc.Config{ClientID: settings.OIDCClientID}).Verify(ctx, rawIDToken)
	if err != nil {
		return OIDCCallbackResult{}, ErrInvalidCredentials
	}
	var claims oidcUserClaims
	if err := idToken.Claims(&claims); err != nil {
		return OIDCCallbackResult{}, err
	}
	user, err := s.upsertOIDCUser(ctx, claims)
	if err != nil {
		return OIDCCallbackResult{}, err
	}
	session, err := s.session(user)
	if err != nil {
		return OIDCCallbackResult{}, err
	}
	return OIDCCallbackResult{Session: session, RedirectPath: state.RedirectPath, JSONResponse: input.JSONResponse || state.JSONResponse}, nil
}

func (s *Service) takeState(value string) (oidcState, bool) {
	s.statesMu.Lock()
	defer s.statesMu.Unlock()
	state, ok := s.states[value]
	delete(s.states, value)
	return state, ok && s.now().Before(state.ExpiresAt)
}

func (s *Service) upsertOIDCUser(ctx context.Context, claims oidcUserClaims) (models.User, error) {
	if strings.TrimSpace(claims.Sub) == "" || !validEmail(strings.TrimSpace(claims.Email)) {
		return models.User{}, ErrInvalidCredentials
	}
	db := s.db.WithContext(ctx)
	var user models.User
	err := db.Where("subject = ?", claims.Sub).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		username := strings.TrimSpace(claims.PreferredUsername)
		if username == "" {
			username = strings.TrimSpace(claims.Email)
		}
		user = models.User{Subject: claims.Sub, Email: strings.ToLower(strings.TrimSpace(claims.Email)), Name: strings.TrimSpace(claims.Name), Username: username, Role: "customer", Currency: "USD"}
		return user, db.Create(&user).Error
	}
	if err != nil {
		return models.User{}, err
	}
	candidate := strings.TrimSpace(claims.PreferredUsername)
	if candidate != "" && candidate != user.Username {
		var count int64
		if err := db.Model(&models.User{}).Where("username = ? AND id <> ?", candidate, user.ID).Count(&count).Error; err != nil {
			return models.User{}, err
		}
		if count == 0 {
			user.Username = candidate
			if err := db.Save(&user).Error; err != nil {
				return models.User{}, err
			}
		}
	}
	return user, nil
}

func (s *Service) session(user models.User) (Session, error) {
	claims := jwt.MapClaims{"sub": user.Subject, "email": user.Email, "role": user.Role, "name": user.Name, "exp": s.now().Add(7 * 24 * time.Hour).Unix(), "iat": s.now().Unix()}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.jwtSecret))
	if err != nil {
		return Session{}, err
	}
	user.PasswordHash = ""
	return Session{Token: token, User: user}, nil
}

func oidcConfigured(settings models.WebsiteSettings) bool {
	return strings.TrimSpace(settings.OIDCProvider) != "" && strings.TrimSpace(settings.OIDCClientID) != "" && strings.TrimSpace(settings.OIDCRedirectURI) != ""
}

func sanitizeRedirectPath(path string) string {
	path = strings.TrimSpace(path)
	if path != "" && strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "//") {
		return path
	}
	return "/"
}

func validEmail(value string) bool {
	parsed, err := url.Parse("mailto:" + value)
	return err == nil && parsed.Opaque != "" && strings.Contains(value, "@") && !strings.ContainsAny(value, " \t\r\n")
}

var _ WebsiteService = (*accountservice.Service)(nil)
