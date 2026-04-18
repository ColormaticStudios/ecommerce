package handlers

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"unicode"

	accountdataservice "ecommerce/internal/services/accountdata"
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

func getAuthenticatedUserWithNotFound(db *gorm.DB, c *gin.Context) (*models.User, bool) {
	subject := c.GetString("userID")
	if subject == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return nil, false
	}

	var user models.User
	if err := db.Where("subject = ?", subject).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired authentication token"})
			return nil, false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load authenticated user"})
		return nil, false
	}
	return &user, true
}

func getAuthenticatedUser(db *gorm.DB, c *gin.Context) (*models.User, bool) {
	return getAuthenticatedUserWithNotFound(db, c)
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

		methods, err := accountdataservice.NewService(db).ListSavedPaymentMethods(user.ID)
		if err != nil {
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
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": friendlyValidationError(err, map[string]string{
				"CardholderName": "Cardholder name",
				"CardNumber":     "Card number",
				"ExpMonth":       "Expiration month",
				"ExpYear":        "Expiration year",
			})})
			return
		}

		method, err := accountdataservice.NewService(db).CreateSavedPaymentMethod(user.ID, accountdataservice.CreateSavedPaymentMethodInput{
			CardholderName: req.CardholderName,
			CardNumber:     req.CardNumber,
			ExpMonth:       req.ExpMonth,
			ExpYear:        req.ExpYear,
			Nickname:       req.Nickname,
			SetDefault:     req.SetDefault,
		})
		if err != nil {
			if err.Error() == "Card number must be 12 to 19 digits" ||
				err.Error() == "Expiration month is invalid" ||
				err.Error() == "Expiration year is invalid" {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
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

		methodID, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment method ID"})
			return
		}

		if err := accountdataservice.NewService(db).DeleteSavedPaymentMethod(user.ID, methodID); err != nil {
			if err.Error() == "payment method not found" {
				c.JSON(http.StatusNotFound, gin.H{"error": "Payment method not found"})
				return
			}
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

		methodID, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment method ID"})
			return
		}

		method, err := accountdataservice.NewService(db).SetDefaultPaymentMethod(user.ID, methodID)
		if err != nil {
			if err.Error() == "payment method not found" {
				c.JSON(http.StatusNotFound, gin.H{"error": "Payment method not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update payment method"})
			return
		}
		c.JSON(http.StatusOK, method)
	}
}

func GetSavedAddresses(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		addresses, err := accountdataservice.NewService(db).ListSavedAddresses(user.ID)
		if err != nil {
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
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": friendlyValidationError(err, map[string]string{
				"FullName":   "Full name",
				"Line1":      "Address line 1",
				"City":       "City",
				"PostalCode": "Postal code",
				"Country":    "Country",
			})})
			return
		}

		address, err := accountdataservice.NewService(db).CreateSavedAddress(user.ID, accountdataservice.CreateSavedAddressInput{
			Label:      req.Label,
			FullName:   req.FullName,
			Line1:      req.Line1,
			Line2:      req.Line2,
			City:       req.City,
			State:      req.State,
			PostalCode: req.PostalCode,
			Country:    req.Country,
			Phone:      req.Phone,
			SetDefault: req.SetDefault,
		})
		if err != nil {
			if err.Error() == "Country must be a 2-letter code" {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
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

		addressID, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid address ID"})
			return
		}

		if err := accountdataservice.NewService(db).DeleteSavedAddress(user.ID, addressID); err != nil {
			if err.Error() == "address not found" {
				c.JSON(http.StatusNotFound, gin.H{"error": "Address not found"})
				return
			}
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

		addressID, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid address ID"})
			return
		}

		address, err := accountdataservice.NewService(db).SetDefaultAddress(user.ID, addressID)
		if err != nil {
			if err.Error() == "address not found" {
				c.JSON(http.StatusNotFound, gin.H{"error": "Address not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update address"})
			return
		}
		c.JSON(http.StatusOK, address)
	}
}
