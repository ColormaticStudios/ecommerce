package handlers

import (
	"strings"

	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func checkoutMode(user *models.User) string {
	if user == nil {
		return "guest"
	}
	return "authenticated"
}

func checkoutUserID(user *models.User) any {
	if user == nil {
		return "none"
	}
	return user.ID
}

func checkoutGuestEmail(email *string) string {
	if email == nil {
		return ""
	}
	return *email
}

func checkoutCorrelationID(c *gin.Context, existing string) string {
	if trimmed := strings.TrimSpace(existing); trimmed != "" {
		return trimmed
	}
	if c == nil {
		return uuid.NewString()
	}
	if value, ok := c.Get("checkout_correlation_id"); ok {
		if correlationID, ok := value.(string); ok && strings.TrimSpace(correlationID) != "" {
			return correlationID
		}
	}
	correlationID := uuid.NewString()
	c.Set("checkout_correlation_id", correlationID)
	return correlationID
}
