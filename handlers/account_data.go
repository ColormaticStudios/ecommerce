package handlers

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"unicode"

	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

type CreateSavedPaymentMethodRequest struct {
	CardholderName string `json:"cardholder_name" binding:"required"`
	CardNumber     string `json:"card_number" binding:"required"`
	ExpMonth       int    `json:"exp_month" binding:"required,min=1,max=12"`
	ExpYear        int    `json:"exp_year" binding:"required,min=2000,max=2200"`
	Nickname       string `json:"nickname"`
	SetDefault     bool   `json:"set_default"`
}

type CreateSavedAddressRequest struct {
	Label      string `json:"label"`
	FullName   string `json:"full_name" binding:"required"`
	Line1      string `json:"line1" binding:"required"`
	Line2      string `json:"line2"`
	City       string `json:"city" binding:"required"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code" binding:"required"`
	Country    string `json:"country" binding:"required,len=2"`
	Phone      string `json:"phone"`
	SetDefault bool   `json:"set_default"`
}

func getAuthenticatedUser(db *gorm.DB, c *gin.Context) (*models.User, bool) {
	subject := c.GetString("userID")
	if subject == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return nil, false
	}

	var user models.User
	if err := db.Where("subject = ?", subject).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return nil, false
	}
	return &user, true
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

func paymentMethodDisplay(brand, last4 string) string {
	if last4 == "" {
		return strings.TrimSpace(brand)
	}
	if brand == "" {
		brand = "Card"
	}
	return brand + " •••• " + last4
}

func addressPretty(a models.SavedAddress) string {
	parts := []string{
		strings.TrimSpace(a.Line1),
		strings.TrimSpace(a.Line2),
		strings.TrimSpace(a.City),
		strings.TrimSpace(a.State),
		strings.TrimSpace(a.PostalCode),
		strings.ToUpper(strings.TrimSpace(a.Country)),
	}
	filtered := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return strings.Join(filtered, ", ")
}

func friendlyValidationError(err error, labels map[string]string) string {
	var validationErrs validator.ValidationErrors
	if !errors.As(err, &validationErrs) {
		return "Invalid request payload"
	}

	missing := make([]string, 0)
	other := make([]string, 0)

	for _, fieldErr := range validationErrs {
		label, ok := labels[fieldErr.Field()]
		if !ok {
			label = fieldErr.Field()
		}

		switch fieldErr.Tag() {
		case "required":
			missing = append(missing, label)
		case "email":
			other = append(other, label+" must be a valid email address")
		case "len":
			other = append(other, label+" has an invalid length")
		case "min":
			other = append(other, label+" is too small")
		case "max":
			other = append(other, label+" is too large")
		default:
			other = append(other, label+" is invalid")
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		if len(missing) == 1 {
			return missing[0] + " is required"
		}
		return "Missing required fields: " + strings.Join(missing, ", ")
	}
	if len(other) > 0 {
		return other[0]
	}
	return "Invalid request payload"
}

func GetSavedPaymentMethods(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		var methods []models.SavedPaymentMethod
		if err := db.
			Where("user_id = ?", user.ID).
			Order("is_default DESC, created_at DESC").
			Find(&methods).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load payment methods"})
			return
		}

		c.JSON(http.StatusOK, methods)
	}
}

func CreateSavedPaymentMethod(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		var req CreateSavedPaymentMethodRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": friendlyValidationError(err, map[string]string{
				"CardholderName": "Cardholder name",
				"CardNumber":     "Card number",
				"ExpMonth":       "Expiration month",
				"ExpYear":        "Expiration year",
			})})
			return
		}

		cardDigits := digitsOnly(req.CardNumber)
		if len(cardDigits) < 12 || len(cardDigits) > 19 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Card number must be 12 to 19 digits"})
			return
		}

		brand := detectCardBrand(cardDigits)
		last4 := cardDigits[len(cardDigits)-4:]

		var count int64
		if err := db.Model(&models.SavedPaymentMethod{}).Where("user_id = ?", user.ID).Count(&count).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create payment method"})
			return
		}

		method := models.SavedPaymentMethod{
			UserID:         user.ID,
			Type:           "card",
			Brand:          brand,
			Last4:          last4,
			ExpMonth:       req.ExpMonth,
			ExpYear:        req.ExpYear,
			CardholderName: strings.TrimSpace(req.CardholderName),
			Nickname:       strings.TrimSpace(req.Nickname),
			IsDefault:      req.SetDefault || count == 0,
		}

		tx := db.Begin()
		if method.IsDefault {
			if err := tx.Model(&models.SavedPaymentMethod{}).
				Where("user_id = ?", user.ID).
				Update("is_default", false).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create payment method"})
				return
			}
		}
		if err := tx.Create(&method).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create payment method"})
			return
		}
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create payment method"})
			return
		}

		c.JSON(http.StatusCreated, method)
	}
}

func DeleteSavedPaymentMethod(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		id := c.Param("id")
		var method models.SavedPaymentMethod
		if err := db.Where("id = ? AND user_id = ?", id, user.ID).First(&method).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Payment method not found"})
			return
		}

		tx := db.Begin()
		if err := tx.Delete(&method).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete payment method"})
			return
		}
		if method.IsDefault {
			var replacement models.SavedPaymentMethod
			if err := tx.Where("user_id = ?", user.ID).Order("created_at DESC").First(&replacement).Error; err == nil {
				_ = tx.Model(&replacement).Update("is_default", true).Error
			}
		}
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete payment method"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Payment method deleted"})
	}
}

func SetDefaultPaymentMethod(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		id := c.Param("id")
		var method models.SavedPaymentMethod
		if err := db.Where("id = ? AND user_id = ?", id, user.ID).First(&method).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Payment method not found"})
			return
		}

		tx := db.Begin()
		if err := tx.Model(&models.SavedPaymentMethod{}).
			Where("user_id = ?", user.ID).
			Update("is_default", false).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update payment method"})
			return
		}
		if err := tx.Model(&method).Update("is_default", true).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update payment method"})
			return
		}
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update payment method"})
			return
		}

		method.IsDefault = true
		c.JSON(http.StatusOK, method)
	}
}

func GetSavedAddresses(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		var addresses []models.SavedAddress
		if err := db.
			Where("user_id = ?", user.ID).
			Order("is_default DESC, created_at DESC").
			Find(&addresses).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load addresses"})
			return
		}

		c.JSON(http.StatusOK, addresses)
	}
}

func CreateSavedAddress(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		var req CreateSavedAddressRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": friendlyValidationError(err, map[string]string{
				"FullName":   "Full name",
				"Line1":      "Address line 1",
				"City":       "City",
				"PostalCode": "Postal code",
				"Country":    "Country",
			})})
			return
		}

		country := strings.ToUpper(strings.TrimSpace(req.Country))
		if len(country) != 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Country must be a 2-letter code"})
			return
		}

		var count int64
		if err := db.Model(&models.SavedAddress{}).Where("user_id = ?", user.ID).Count(&count).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create address"})
			return
		}

		address := models.SavedAddress{
			UserID:     user.ID,
			Label:      strings.TrimSpace(req.Label),
			FullName:   strings.TrimSpace(req.FullName),
			Line1:      strings.TrimSpace(req.Line1),
			Line2:      strings.TrimSpace(req.Line2),
			City:       strings.TrimSpace(req.City),
			State:      strings.TrimSpace(req.State),
			PostalCode: strings.TrimSpace(req.PostalCode),
			Country:    country,
			Phone:      strings.TrimSpace(req.Phone),
			IsDefault:  req.SetDefault || count == 0,
		}

		tx := db.Begin()
		if address.IsDefault {
			if err := tx.Model(&models.SavedAddress{}).
				Where("user_id = ?", user.ID).
				Update("is_default", false).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create address"})
				return
			}
		}
		if err := tx.Create(&address).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create address"})
			return
		}
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create address"})
			return
		}

		c.JSON(http.StatusCreated, address)
	}
}

func DeleteSavedAddress(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		id := c.Param("id")
		var address models.SavedAddress
		if err := db.Where("id = ? AND user_id = ?", id, user.ID).First(&address).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Address not found"})
			return
		}

		tx := db.Begin()
		if err := tx.Delete(&address).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete address"})
			return
		}
		if address.IsDefault {
			var replacement models.SavedAddress
			if err := tx.Where("user_id = ?", user.ID).Order("created_at DESC").First(&replacement).Error; err == nil {
				_ = tx.Model(&replacement).Update("is_default", true).Error
			}
		}
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete address"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Address deleted"})
	}
}

func SetDefaultAddress(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		id := c.Param("id")
		var address models.SavedAddress
		if err := db.Where("id = ? AND user_id = ?", id, user.ID).First(&address).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Address not found"})
			return
		}

		tx := db.Begin()
		if err := tx.Model(&models.SavedAddress{}).
			Where("user_id = ?", user.ID).
			Update("is_default", false).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update address"})
			return
		}
		if err := tx.Model(&address).Update("is_default", true).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update address"})
			return
		}
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update address"})
			return
		}

		address.IsDefault = true
		c.JSON(http.StatusOK, address)
	}
}
