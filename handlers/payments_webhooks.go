package handlers

import (
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"ecommerce/internal/apicontract"
	paymentservice "ecommerce/internal/services/payments"
	shippingservice "ecommerce/internal/services/shipping"
	webhookservice "ecommerce/internal/services/webhooks"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type webhookEventRecordResponse struct {
	ID              uint    `json:"id"`
	Provider        string  `json:"provider"`
	ProviderEventID string  `json:"provider_event_id"`
	EventType       string  `json:"event_type"`
	SignatureValid  bool    `json:"signature_valid"`
	Payload         string  `json:"payload"`
	ReceivedAt      string  `json:"received_at"`
	ProcessedAt     *string `json:"processed_at"`
	AttemptCount    int     `json:"attempt_count"`
	LastError       string  `json:"last_error"`
	Status          string  `json:"status"`
}

type webhookEventPageResponse struct {
	Data       []webhookEventRecordResponse `json:"data"`
	Pagination apicontract.Pagination       `json:"pagination"`
}

func ReceiveWebhookEvent(db *gorm.DB, service *webhookservice.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider := strings.TrimSpace(c.Param("provider"))
		if provider == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Provider is required"})
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read webhook body"})
			return
		}
		if len(body) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Webhook body is required"})
			return
		}

		event, duplicate, err := service.ReceiveWebhook(c.Request.Context(), provider, flattenHeaders(c.Request.Header), body)
		if err != nil {
			switch {
			case errors.Is(err, paymentservice.ErrInvalidWebhookSignature):
				log.Printf(
					"webhook_receive result=failure correlation_id=%s provider=%s reason=%q",
					checkoutCorrelationID(c, ""),
					provider,
					err.Error(),
				)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid webhook signature"})
			case errors.Is(err, shippingservice.ErrInvalidShippingWebhookSignature):
				log.Printf(
					"webhook_receive result=failure correlation_id=%s provider=%s reason=%q",
					checkoutCorrelationID(c, ""),
					provider,
					err.Error(),
				)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid webhook signature"})
			case errors.Is(err, paymentservice.ErrUnknownPaymentProvider):
				c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown webhook provider"})
			case errors.Is(err, shippingservice.ErrUnknownShippingProvider):
				c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown webhook provider"})
			default:
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			}
			return
		}

		log.Printf(
			"webhook_receive result=success correlation_id=%s provider=%s event_id=%d provider_event_id=%s duplicate=%t",
			checkoutCorrelationID(c, ""),
			provider,
			event.ID,
			event.ProviderEventID,
			duplicate,
		)
		c.JSON(http.StatusOK, gin.H{
			"message":           "Webhook accepted",
			"event_id":          event.ID,
			"provider_event_id": event.ProviderEventID,
			"duplicate":         duplicate,
		})
	}
}

func ListAdminWebhookEvents(db *gorm.DB, service *webhookservice.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, limit, _ := parsePagination(c, 20)
		events, total, err := webhookservice.ListEvents(
			db,
			c.Query("provider"),
			c.Query("status"),
			page,
			limit,
			service.MaxAttempts,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load webhook events"})
			return
		}

		totalPages := int(total) / limit
		if int(total)%limit > 0 {
			totalPages++
		}
		response := webhookEventPageResponse{
			Data: make([]webhookEventRecordResponse, 0, len(events)),
			Pagination: apicontract.Pagination{
				Page:       page,
				Limit:      limit,
				Total:      int(total),
				TotalPages: totalPages,
			},
		}
		for _, event := range events {
			response.Data = append(response.Data, serializeWebhookEvent(event, service.MaxAttempts))
		}
		c.JSON(http.StatusOK, response)
	}
}

func serializeWebhookEvent(event models.WebhookEvent, maxAttempts int) webhookEventRecordResponse {
	var processedAt *string
	if event.ProcessedAt != nil {
		value := event.ProcessedAt.UTC().Format(timeRFC3339JSON)
		processedAt = &value
	}

	return webhookEventRecordResponse{
		ID:              event.ID,
		Provider:        event.Provider,
		ProviderEventID: event.ProviderEventID,
		EventType:       event.EventType,
		SignatureValid:  event.SignatureValid,
		Payload:         event.Payload,
		ReceivedAt:      event.ReceivedAt.UTC().Format(timeRFC3339JSON),
		ProcessedAt:     processedAt,
		AttemptCount:    event.AttemptCount,
		LastError:       event.LastError,
		Status:          webhookservice.EventStatus(&event, maxAttempts),
	}
}

func flattenHeaders(headers http.Header) map[string]string {
	flattened := make(map[string]string, len(headers))
	for key, values := range headers {
		if len(values) == 0 {
			continue
		}
		flattened[key] = values[0]
	}
	return flattened
}

const timeRFC3339JSON = "2006-01-02T15:04:05Z07:00"
