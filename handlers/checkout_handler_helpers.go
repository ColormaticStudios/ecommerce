package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	paymentservice "ecommerce/internal/services/payments"
	providerops "ecommerce/internal/services/providerops"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type handlerResponseCapture struct {
	status  int
	payload any
}

func (c *handlerResponseCapture) Respond(status int, payload any) {
	c.status = status
	c.payload = payload
}

func (c handlerResponseCapture) Handled() bool {
	return c.status != 0
}

func (c handlerResponseCapture) Status() int {
	return c.status
}

func (c handlerResponseCapture) Payload() any {
	return c.payload
}

func (c handlerResponseCapture) ErrorReason() string {
	return handlerResponseReason(c.payload)
}

type handlerResponseWriter struct {
	written bool
	write   func(status int, payload any)
}

func newHandlerResponseWriter(write func(status int, payload any)) *handlerResponseWriter {
	return &handlerResponseWriter{write: write}
}

func (w *handlerResponseWriter) Respond(status int, payload any) {
	w.written = true
	w.write(status, payload)
}

func (w handlerResponseWriter) Written() bool {
	return w.written
}

func handlerResponseReason(payload any) string {
	if payload == nil {
		return "request_rejected"
	}
	if fields, ok := payload.(gin.H); ok {
		if value, ok := fields["error"].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "request_rejected"
}

func mapProviderConfigurationError(err error, respond func(status int, payload any)) bool {
	switch {
	case errors.Is(err, providerops.ErrProviderCredentialWrongEnvironment):
		respond(http.StatusConflict, gin.H{"error": "Provider credential is not configured for this environment"})
		return true
	case errors.Is(err, providerops.ErrUnsupportedProviderCurrency):
		respond(http.StatusBadRequest, gin.H{"error": "Provider does not support the requested currency"})
		return true
	default:
		return false
	}
}

func mapCheckoutSnapshotLookupError(err error, respond func(status int, payload any)) bool {
	if errors.Is(err, paymentservice.ErrSnapshotNotFound) {
		respond(http.StatusBadRequest, gin.H{"error": "Checkout snapshot not found"})
		return true
	}
	return false
}

func mapCheckoutSnapshotValidationError(err error, respond func(status int, payload any)) bool {
	switch {
	case errors.Is(err, paymentservice.ErrSnapshotExpired):
		respond(http.StatusBadRequest, gin.H{"error": "Checkout snapshot has expired"})
		return true
	case errors.Is(err, paymentservice.ErrSnapshotOrderMismatch):
		respond(http.StatusConflict, gin.H{"error": "Checkout snapshot no longer matches the order"})
		return true
	case errors.Is(err, paymentservice.ErrSnapshotAlreadyBound):
		respond(http.StatusConflict, gin.H{"error": "Checkout snapshot is already bound to another order"})
		return true
	default:
		return false
	}
}

func loadCheckoutSnapshotForOrder(
	tx *gorm.DB,
	sessionID uint,
	snapshotID uint,
	order *models.Order,
	now time.Time,
	respond func(status int, payload any),
) (models.OrderCheckoutSnapshot, bool, error) {
	snapshot, err := paymentservice.GetCheckoutSnapshotForSession(tx, sessionID, snapshotID)
	if err != nil {
		if mapCheckoutSnapshotLookupError(err, respond) {
			return models.OrderCheckoutSnapshot{}, true, nil
		}
		return models.OrderCheckoutSnapshot{}, false, err
	}
	if err := paymentservice.ValidateSnapshotForOrder(&snapshot, order, now); err != nil {
		if mapCheckoutSnapshotValidationError(err, respond) {
			return models.OrderCheckoutSnapshot{}, true, nil
		}
		return models.OrderCheckoutSnapshot{}, false, err
	}
	return snapshot, false, nil
}
