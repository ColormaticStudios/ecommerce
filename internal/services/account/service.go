package account

import (
	"context"
	"errors"
	"strings"

	"ecommerce/internal/services/providerops"
	"ecommerce/models"

	"gorm.io/gorm"
)

const (
	websiteOIDCSecretScope       = "website_oidc_client_secret"
	websiteOIDCSecretDataKey     = "client_secret"
	websiteOIDCSecretPlaceholder = "********"
)

var (
	ErrUserNotFound                  = errors.New("user not found")
	ErrInvalidCurrency               = errors.New("invalid currency code")
	ErrInvalidRole                   = errors.New("invalid user role")
	ErrCredentialServiceUnconfigured = errors.New("provider credential encryption is not configured")
)

type Service struct {
	db          *gorm.DB
	credentials *providerops.CredentialService
}

type UpdateProfileInput struct {
	Name            *string
	Currency        *string
	ProfilePhotoURL *string
}

type ListUsersInput struct {
	Page  int
	Limit int
	Query string
}

type UserPage struct {
	Users      []models.User
	Page       int
	Limit      int
	Total      int64
	TotalPages int
}

type WebsiteSettingsInput struct {
	SiteTitle             string
	AllowGuestCheckout    bool
	CouponCodesEnabled    bool
	OIDCProvider          string
	OIDCClientID          string
	OIDCClientSecret      string
	ClearOIDCClientSecret bool
	OIDCRedirectURI       string
}

func NewService(db *gorm.DB, credentials *providerops.CredentialService) *Service {
	return &Service{db: db, credentials: credentials}
}

func (s *Service) UserBySubject(ctx context.Context, subject string) (models.User, error) {
	var user models.User
	if err := s.db.WithContext(ctx).Where("subject = ?", strings.TrimSpace(subject)).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.User{}, ErrUserNotFound
		}
		return models.User{}, err
	}
	return user, nil
}

func (s *Service) GetProfile(ctx context.Context, subject string) (models.User, error) {
	return s.UserBySubject(ctx, subject)
}

func (s *Service) UpdateProfile(ctx context.Context, subject string, input UpdateProfileInput) (models.User, error) {
	user, err := s.UserBySubject(ctx, subject)
	if err != nil {
		return models.User{}, err
	}
	if input.Currency != nil {
		currency := strings.ToUpper(strings.TrimSpace(*input.Currency))
		if currency != "" && !validCurrency(currency) {
			return models.User{}, ErrInvalidCurrency
		}
		if currency != "" {
			user.Currency = currency
		}
	}
	if input.Name != nil && strings.TrimSpace(*input.Name) != "" {
		user.Name = strings.TrimSpace(*input.Name)
	}
	if input.ProfilePhotoURL != nil && strings.TrimSpace(*input.ProfilePhotoURL) != "" {
		user.ProfilePhoto = strings.TrimSpace(*input.ProfilePhotoURL)
	}
	if err := s.db.WithContext(ctx).Save(&user).Error; err != nil {
		return models.User{}, err
	}
	return user, nil
}

func (s *Service) ListUsers(ctx context.Context, input ListUsersInput) (UserPage, error) {
	page, limit := input.Page, input.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	query := s.db.WithContext(ctx).Model(&models.User{})
	if search := strings.TrimSpace(input.Query); search != "" {
		like := "%" + strings.ToLower(search) + "%"
		query = query.Where(
			`CAST(id AS TEXT) LIKE ? OR LOWER(username) LIKE ? OR LOWER(email) LIKE ? OR LOWER(COALESCE(name, '')) LIKE ? OR LOWER(subject) LIKE ? OR LOWER(role) LIKE ?`,
			like, like, like, like, like, like,
		)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return UserPage{}, err
	}
	users := []models.User{}
	if err := query.Order("created_at DESC").Offset((page - 1) * limit).Limit(limit).Find(&users).Error; err != nil {
		return UserPage{}, err
	}
	return UserPage{Users: users, Page: page, Limit: limit, Total: total, TotalPages: (int(total) + limit - 1) / limit}, nil
}

func (s *Service) UpdateUserRole(ctx context.Context, userID uint, role string) (models.User, error) {
	role = strings.TrimSpace(role)
	if role != "admin" && role != "customer" {
		return models.User{}, ErrInvalidRole
	}
	var user models.User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.User{}, ErrUserNotFound
		}
		return models.User{}, err
	}
	user.Role = role
	if err := s.db.WithContext(ctx).Save(&user).Error; err != nil {
		return models.User{}, err
	}
	return user, nil
}

func (s *Service) GetWebsiteSettings(ctx context.Context) (models.WebsiteSettings, error) {
	return loadOrCreateWebsiteSettings(s.db.WithContext(ctx))
}

func (s *Service) UpdateWebsiteSettings(ctx context.Context, input WebsiteSettingsInput) (models.WebsiteSettings, error) {
	var result models.WebsiteSettings
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := loadOrCreateWebsiteSettings(tx)
		if err != nil {
			return err
		}
		current.SiteTitle = strings.TrimSpace(input.SiteTitle)
		current.AllowGuestCheckout = input.AllowGuestCheckout
		current.CouponCodesEnabled = input.CouponCodesEnabled
		current.OIDCProvider = strings.TrimSpace(input.OIDCProvider)
		current.OIDCClientID = strings.TrimSpace(input.OIDCClientID)
		current.OIDCRedirectURI = strings.TrimSpace(input.OIDCRedirectURI)
		if input.ClearOIDCClientSecret {
			current.OIDCClientSecretEnvelopeJSON = ""
			current.OIDCClientSecretKeyVersion = ""
		}
		secret := strings.TrimSpace(input.OIDCClientSecret)
		if secret != "" && secret != websiteOIDCSecretPlaceholder {
			envelope, version, err := s.credentials.EncryptSecretData(websiteOIDCSecretScope, map[string]string{websiteOIDCSecretDataKey: secret})
			if err != nil {
				if errors.Is(err, providerops.ErrCredentialServiceUnconfigured) {
					return ErrCredentialServiceUnconfigured
				}
				return err
			}
			current.OIDCClientSecretEnvelopeJSON = envelope
			current.OIDCClientSecretKeyVersion = version
		}
		if err := tx.Select("*").Save(&current).Error; err != nil {
			return err
		}
		result = current
		return nil
	})
	return result, err
}

func (s *Service) OIDCClientSecret(ctx context.Context, settings models.WebsiteSettings) (string, error) {
	if strings.TrimSpace(settings.OIDCClientSecretEnvelopeJSON) == "" {
		return "", nil
	}
	data, err := s.credentials.DecryptSecretData(websiteOIDCSecretScope, settings.OIDCClientSecretEnvelopeJSON)
	if err != nil {
		if errors.Is(err, providerops.ErrCredentialServiceUnconfigured) {
			return "", ErrCredentialServiceUnconfigured
		}
		return "", err
	}
	return strings.TrimSpace(data[websiteOIDCSecretDataKey]), nil
}

func loadOrCreateWebsiteSettings(db *gorm.DB) (models.WebsiteSettings, error) {
	var settings models.WebsiteSettings
	err := db.First(&settings, models.WebsiteSettingsSingletonID).Error
	if err == nil {
		return settings, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.WebsiteSettings{}, err
	}
	settings = models.WebsiteSettings{ID: models.WebsiteSettingsSingletonID, SiteTitle: "Ecommerce", AllowGuestCheckout: true, CouponCodesEnabled: true}
	if err := db.Select("*").Create(&settings).Error; err != nil {
		return models.WebsiteSettings{}, err
	}
	return settings, nil
}

func validCurrency(currency string) bool {
	switch currency {
	case "USD", "EUR", "GBP", "JPY", "CAD", "AUD", "CHF", "CNY", "INR", "BRL":
		return true
	default:
		return false
	}
}
