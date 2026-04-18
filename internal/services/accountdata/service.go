package accountdata

import (
	"errors"
	"strings"
	"unicode"

	"ecommerce/models"

	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

type CreateSavedPaymentMethodInput struct {
	CardholderName string
	CardNumber     string
	ExpMonth       int
	ExpYear        int
	Nickname       string
	SetDefault     bool
}

type CreateSavedAddressInput struct {
	Label      string
	FullName   string
	Line1      string
	Line2      string
	City       string
	State      string
	PostalCode string
	Country    string
	Phone      string
	SetDefault bool
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) ListSavedPaymentMethods(userID uint) ([]models.SavedPaymentMethod, error) {
	var methods []models.SavedPaymentMethod
	if err := s.db.
		Where("user_id = ?", userID).
		Order("is_default DESC, created_at DESC").
		Find(&methods).Error; err != nil {
		return nil, err
	}
	return methods, nil
}

func (s *Service) CreateSavedPaymentMethod(userID uint, input CreateSavedPaymentMethodInput) (models.SavedPaymentMethod, error) {
	cardDigits := digitsOnly(input.CardNumber)
	if len(cardDigits) < 12 || len(cardDigits) > 19 {
		return models.SavedPaymentMethod{}, errors.New("Card number must be 12 to 19 digits")
	}
	if input.ExpMonth < 1 || input.ExpMonth > 12 {
		return models.SavedPaymentMethod{}, errors.New("Expiration month is invalid")
	}
	if input.ExpYear < 2000 || input.ExpYear > 2200 {
		return models.SavedPaymentMethod{}, errors.New("Expiration year is invalid")
	}

	var count int64
	if err := s.db.Model(&models.SavedPaymentMethod{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return models.SavedPaymentMethod{}, err
	}

	method := models.SavedPaymentMethod{
		UserID:         userID,
		Type:           "card",
		Brand:          detectCardBrand(cardDigits),
		Last4:          cardDigits[len(cardDigits)-4:],
		ExpMonth:       input.ExpMonth,
		ExpYear:        input.ExpYear,
		CardholderName: strings.TrimSpace(input.CardholderName),
		Nickname:       strings.TrimSpace(input.Nickname),
		IsDefault:      input.SetDefault || count == 0,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if method.IsDefault {
			if err := tx.Model(&models.SavedPaymentMethod{}).
				Where("user_id = ?", userID).
				Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Create(&method).Error
	})
	return method, err
}

func (s *Service) DeleteSavedPaymentMethod(userID uint, methodID uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var method models.SavedPaymentMethod
		if err := tx.Where("id = ? AND user_id = ?", methodID, userID).First(&method).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("payment method not found")
			}
			return err
		}

		if err := tx.Delete(&method).Error; err != nil {
			return err
		}
		if method.IsDefault {
			var replacement models.SavedPaymentMethod
			if err := tx.Where("user_id = ?", userID).Order("created_at DESC").First(&replacement).Error; err == nil {
				if err := tx.Model(&replacement).Update("is_default", true).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *Service) SetDefaultPaymentMethod(userID uint, methodID uint) (models.SavedPaymentMethod, error) {
	var method models.SavedPaymentMethod
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND user_id = ?", methodID, userID).First(&method).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("payment method not found")
			}
			return err
		}
		if err := tx.Model(&models.SavedPaymentMethod{}).
			Where("user_id = ?", userID).
			Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&method).Update("is_default", true).Error
	})
	if err != nil {
		return models.SavedPaymentMethod{}, err
	}
	method.IsDefault = true
	return method, nil
}

func (s *Service) ListSavedAddresses(userID uint) ([]models.SavedAddress, error) {
	var addresses []models.SavedAddress
	if err := s.db.
		Where("user_id = ?", userID).
		Order("is_default DESC, created_at DESC").
		Find(&addresses).Error; err != nil {
		return nil, err
	}
	return addresses, nil
}

func (s *Service) CreateSavedAddress(userID uint, input CreateSavedAddressInput) (models.SavedAddress, error) {
	country := strings.ToUpper(strings.TrimSpace(input.Country))
	if len(country) != 2 {
		return models.SavedAddress{}, errors.New("Country must be a 2-letter code")
	}

	var count int64
	if err := s.db.Model(&models.SavedAddress{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return models.SavedAddress{}, err
	}

	address := models.SavedAddress{
		UserID:     userID,
		Label:      strings.TrimSpace(input.Label),
		FullName:   strings.TrimSpace(input.FullName),
		Line1:      strings.TrimSpace(input.Line1),
		Line2:      strings.TrimSpace(input.Line2),
		City:       strings.TrimSpace(input.City),
		State:      strings.TrimSpace(input.State),
		PostalCode: strings.TrimSpace(input.PostalCode),
		Country:    country,
		Phone:      strings.TrimSpace(input.Phone),
		IsDefault:  input.SetDefault || count == 0,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if address.IsDefault {
			if err := tx.Model(&models.SavedAddress{}).
				Where("user_id = ?", userID).
				Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Create(&address).Error
	})
	return address, err
}

func (s *Service) DeleteSavedAddress(userID uint, addressID uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var address models.SavedAddress
		if err := tx.Where("id = ? AND user_id = ?", addressID, userID).First(&address).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("address not found")
			}
			return err
		}

		if err := tx.Delete(&address).Error; err != nil {
			return err
		}
		if address.IsDefault {
			var replacement models.SavedAddress
			if err := tx.Where("user_id = ?", userID).Order("created_at DESC").First(&replacement).Error; err == nil {
				if err := tx.Model(&replacement).Update("is_default", true).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *Service) SetDefaultAddress(userID uint, addressID uint) (models.SavedAddress, error) {
	var address models.SavedAddress
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND user_id = ?", addressID, userID).First(&address).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("address not found")
			}
			return err
		}
		if err := tx.Model(&models.SavedAddress{}).
			Where("user_id = ?", userID).
			Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&address).Update("is_default", true).Error
	})
	if err != nil {
		return models.SavedAddress{}, err
	}
	address.IsDefault = true
	return address, nil
}

func digitsOnly(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func detectCardBrand(number string) string {
	switch {
	case strings.HasPrefix(number, "4"):
		return "Visa"
	case strings.HasPrefix(number, "34"), strings.HasPrefix(number, "37"):
		return "American Express"
	case strings.HasPrefix(number, "5"):
		return "Mastercard"
	case strings.HasPrefix(number, "6"):
		return "Discover"
	default:
		return "Card"
	}
}
