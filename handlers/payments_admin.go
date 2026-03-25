package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ecommerce/internal/media"
	orderservice "ecommerce/internal/services/orders"
	paymentservice "ecommerce/internal/services/payments"
	providerops "ecommerce/internal/services/providerops"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AdminOrderPaymentAmountRequest struct {
	Amount *models.Money `json:"amount"`
}

type paymentTransactionRecordResponse struct {
	ID                  uint         `json:"id"`
	Operation           string       `json:"operation"`
	ProviderTxnID       string       `json:"provider_txn_id"`
	IdempotencyKey      string       `json:"idempotency_key"`
	Amount              models.Money `json:"amount"`
	Status              string       `json:"status"`
	RawResponseRedacted string       `json:"raw_response_redacted"`
	CreatedAt           string       `json:"created_at"`
	UpdatedAt           string       `json:"updated_at"`
}

type paymentIntentRecordResponse struct {
	ID               uint                               `json:"id"`
	OrderID          uint                               `json:"order_id"`
	SnapshotID       uint                               `json:"snapshot_id"`
	Provider         string                             `json:"provider"`
	Status           string                             `json:"status"`
	AuthorizedAmount models.Money                       `json:"authorized_amount"`
	CapturedAmount   models.Money                       `json:"captured_amount"`
	RefundableAmount models.Money                       `json:"refundable_amount"`
	Currency         string                             `json:"currency"`
	Version          int                                `json:"version"`
	CreatedAt        string                             `json:"created_at"`
	UpdatedAt        string                             `json:"updated_at"`
	Transactions     []paymentTransactionRecordResponse `json:"transactions"`
}

type orderPaymentLedgerResponse struct {
	OrderID uint                          `json:"order_id"`
	Intents []paymentIntentRecordResponse `json:"intents"`
}

func GetAdminOrderPayments(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		orderID, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
			return
		}

		var order models.Order
		if err := db.Select("id").First(&order, orderID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load order"})
			return
		}

		intents, err := paymentservice.GetOrderPaymentLedger(db, order.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load payment ledger"})
			return
		}

		c.JSON(http.StatusOK, orderPaymentLedgerResponse{
			OrderID: order.ID,
			Intents: serializePaymentIntents(intents),
		})
	}
}

func CaptureAdminOrderPayment(db *gorm.DB, providerRegistry paymentservice.ProviderRegistry, mediaServices ...*media.Service) gin.HandlerFunc {
	return adminOrderPaymentLifecycle(db, providerRegistry, paymentLifecycleOperationCapture, mediaServices...)
}

func VoidAdminOrderPayment(db *gorm.DB, providerRegistry paymentservice.ProviderRegistry, mediaServices ...*media.Service) gin.HandlerFunc {
	return adminOrderPaymentLifecycle(db, providerRegistry, paymentLifecycleOperationVoid, mediaServices...)
}

func RefundAdminOrderPayment(db *gorm.DB, providerRegistry paymentservice.ProviderRegistry, mediaServices ...*media.Service) gin.HandlerFunc {
	return adminOrderPaymentLifecycle(db, providerRegistry, paymentLifecycleOperationRefund, mediaServices...)
}

type paymentLifecycleOperation string

const (
	paymentLifecycleOperationCapture paymentLifecycleOperation = "capture"
	paymentLifecycleOperationVoid    paymentLifecycleOperation = "void"
	paymentLifecycleOperationRefund  paymentLifecycleOperation = "refund"
)

func adminOrderPaymentLifecycle(
	db *gorm.DB,
	providerRegistry paymentservice.ProviderRegistry,
	operation paymentLifecycleOperation,
	mediaServices ...*media.Service,
) gin.HandlerFunc {
	mediaService := resolveMediaService(mediaServices...)
	if providerRegistry == nil {
		providerRegistry = paymentservice.NewDefaultProviderRegistry()
	}
	return func(c *gin.Context) {
		adminUser, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		orderID, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
			return
		}
		intentID, err := parseUintParam(c, "intentId")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment intent ID"})
			return
		}

		var req AdminOrderPaymentAmountRequest
		if operation != paymentLifecycleOperationVoid {
			if err := bindOptionalStrictJSON(c, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}

		correlationID := checkoutCorrelationID(c, "")
		scope := fmt.Sprintf("admin_order_payment_%s:%d:%d:%d", operation, adminUser.ID, orderID, intentID)
		idempotencyRecord, handled, err := beginScopedIdempotency(db, c, scope, req, correlationID)
		if err != nil {
			log.Printf(
				"admin_order_payment_%s result=failure correlation_id=%s admin_user_id=%d order_id=%d intent_id=%d reason=%q",
				operation,
				correlationID,
				adminUser.ID,
				orderID,
				intentID,
				err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process payment request"})
			return
		}
		if handled {
			replayCorrelationID := correlationID
			if idempotencyRecord != nil && strings.TrimSpace(idempotencyRecord.CorrelationID) != "" {
				replayCorrelationID = checkoutCorrelationID(c, idempotencyRecord.CorrelationID)
			}
			log.Printf(
				"admin_order_payment_%s result=replay correlation_id=%s admin_user_id=%d order_id=%d intent_id=%d",
				operation,
				replayCorrelationID,
				adminUser.ID,
				orderID,
				intentID,
			)
			return
		}

		var (
			order            models.Order
			intent           models.PaymentIntent
			transaction      models.PaymentTransaction
			responsePayload  any
			responseStatus   int
			lifecycleMessage string
		)

		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Preload("Items.ProductVariant").
				Preload("Items.ProductVariant.Product").
				First(&order, orderID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					responseStatus = http.StatusNotFound
					responsePayload = gin.H{"error": "Order not found"}
					return nil
				}
				return err
			}

			intent, err = paymentservice.GetPaymentIntentForUpdate(tx, order.ID, intentID)
			if err != nil {
				if errors.Is(err, paymentservice.ErrPaymentIntentNotFound) {
					responseStatus = http.StatusNotFound
					responsePayload = gin.H{"error": "Payment intent not found"}
					return nil
				}
				return err
			}

			if idempotencyRecord != nil {
				if err := tx.Model(&models.IdempotencyKey{}).
					Where("id = ?", idempotencyRecord.ID).
					Update("payment_intent_id", intent.ID).Error; err != nil {
					return err
				}
				intentIDCopy := intent.ID
				idempotencyRecord.PaymentIntentID = &intentIDCopy
			}

			switch operation {
			case paymentLifecycleOperationCapture:
				transaction, err = paymentservice.CapturePaymentIntent(
					c.Request.Context(),
					tx,
					providerRegistry,
					&intent,
					req.Amount,
					strings.TrimSpace(c.GetHeader("Idempotency-Key")),
					correlationID,
				)
				if err != nil {
					return mapLifecycleError(err, &responseStatus, &responsePayload)
				}
				lifecycleMessage = "Payment captured"
				if order.Status != models.StatusPaid {
					fromStatus := order.Status
					if err := orderservice.ApplyStatusTransition(tx, &order, models.StatusPaid); err != nil {
						return mapLifecycleError(err, &responseStatus, &responsePayload)
					}
					if err := paymentservice.AppendOrderStatusHistory(
						tx,
						order.ID,
						fromStatus,
						order.Status,
						"payment_captured",
						"admin",
						adminActor(adminUser),
						correlationID,
					); err != nil {
						return err
					}
				}
			case paymentLifecycleOperationVoid:
				transaction, err = paymentservice.VoidPaymentIntent(
					c.Request.Context(),
					tx,
					providerRegistry,
					&intent,
					strings.TrimSpace(c.GetHeader("Idempotency-Key")),
					correlationID,
				)
				if err != nil {
					return mapLifecycleError(err, &responseStatus, &responsePayload)
				}
				lifecycleMessage = "Payment voided"
				if order.Status != models.StatusCancelled {
					fromStatus := order.Status
					if err := orderservice.ApplyStatusTransition(tx, &order, models.StatusCancelled); err != nil {
						return mapLifecycleError(err, &responseStatus, &responsePayload)
					}
					if err := paymentservice.AppendOrderStatusHistory(
						tx,
						order.ID,
						fromStatus,
						order.Status,
						"payment_voided",
						"admin",
						adminActor(adminUser),
						correlationID,
					); err != nil {
						return err
					}
				}
			case paymentLifecycleOperationRefund:
				transaction, err = paymentservice.RefundPaymentIntent(
					c.Request.Context(),
					tx,
					providerRegistry,
					&intent,
					req.Amount,
					strings.TrimSpace(c.GetHeader("Idempotency-Key")),
					correlationID,
				)
				if err != nil {
					return mapLifecycleError(err, &responseStatus, &responsePayload)
				}
				lifecycleMessage = "Payment refunded"
				if intent.Status == models.PaymentIntentStatusRefunded && order.Status != models.StatusRefunded {
					fromStatus := order.Status
					if err := orderservice.ApplyStatusTransition(tx, &order, models.StatusRefunded); err != nil {
						return mapLifecycleError(err, &responseStatus, &responsePayload)
					}
					if err := paymentservice.AppendOrderStatusHistory(
						tx,
						order.ID,
						fromStatus,
						order.Status,
						"payment_refunded",
						"admin",
						adminActor(adminUser),
						correlationID,
					); err != nil {
						return err
					}
				}
			default:
				return fmt.Errorf("unsupported payment lifecycle operation: %s", operation)
			}

			intent.Transactions = append(intent.Transactions, transaction)
			return nil
		})
		if err != nil {
			log.Printf(
				"admin_order_payment_%s result=failure correlation_id=%s admin_user_id=%d order_id=%d intent_id=%d reason=%q",
				operation,
				correlationID,
				adminUser.ID,
				orderID,
				intentID,
				err.Error(),
			)
			writeCheckoutJSON(db, c, idempotencyRecord, http.StatusInternalServerError, gin.H{"error": "Failed to process payment request"})
			return
		}
		if responseStatus != 0 {
			reason := "request_rejected"
			if payload, ok := responsePayload.(gin.H); ok {
				if value, ok := payload["error"].(string); ok && strings.TrimSpace(value) != "" {
					reason = value
				}
			}
			log.Printf(
				"admin_order_payment_%s result=failure correlation_id=%s admin_user_id=%d order_id=%d intent_id=%d reason=%q",
				operation,
				correlationID,
				adminUser.ID,
				orderID,
				intentID,
				reason,
			)
			writeCheckoutJSON(db, c, idempotencyRecord, responseStatus, responsePayload)
			return
		}

		if err := db.Preload("Items.ProductVariant").
			Preload("Items.ProductVariant.Product").
			First(&order, order.ID).Error; err != nil {
			writeCheckoutJSON(db, c, idempotencyRecord, http.StatusInternalServerError, gin.H{"error": "Failed to load order"})
			return
		}
		applyOrderMediaToOrder(&order, mediaService)
		applyOrderCapabilities(&order, nil)
		responseOrder, err := buildOrderResponse(db, mediaService, order)
		if err != nil {
			writeCheckoutJSON(db, c, idempotencyRecord, http.StatusInternalServerError, gin.H{"error": "Failed to render order"})
			return
		}

		payload := gin.H{
			"message":        lifecycleMessage,
			"order":          responseOrder,
			"payment_intent": serializePaymentIntent(intent),
			"transaction":    serializePaymentTransaction(transaction),
		}
		log.Printf(
			"admin_order_payment_%s result=success correlation_id=%s admin_user_id=%d order_id=%d intent_id=%d",
			operation,
			correlationID,
			adminUser.ID,
			order.ID,
			intent.ID,
		)
		writeCheckoutJSON(db, c, idempotencyRecord, http.StatusOK, payload)
	}
}

func serializePaymentIntents(intents []models.PaymentIntent) []paymentIntentRecordResponse {
	serialized := make([]paymentIntentRecordResponse, 0, len(intents))
	for _, intent := range intents {
		serialized = append(serialized, serializePaymentIntent(intent))
	}
	return serialized
}

func serializePaymentIntent(intent models.PaymentIntent) paymentIntentRecordResponse {
	return paymentIntentRecordResponse{
		ID:               intent.ID,
		OrderID:          intent.OrderID,
		SnapshotID:       intent.SnapshotID,
		Provider:         intent.Provider,
		Status:           intent.Status,
		AuthorizedAmount: intent.AuthorizedAmount,
		CapturedAmount:   intent.CapturedAmount,
		RefundableAmount: intent.CapturedAmount - paymentservice.RefundedAmount(intent),
		Currency:         intent.Currency,
		Version:          intent.Version,
		CreatedAt:        intent.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:        intent.UpdatedAt.UTC().Format(time.RFC3339),
		Transactions:     serializePaymentTransactions(intent.Transactions),
	}
}

func serializePaymentTransactions(transactions []models.PaymentTransaction) []paymentTransactionRecordResponse {
	serialized := make([]paymentTransactionRecordResponse, 0, len(transactions))
	for _, txn := range transactions {
		serialized = append(serialized, serializePaymentTransaction(txn))
	}
	return serialized
}

func serializePaymentTransaction(txn models.PaymentTransaction) paymentTransactionRecordResponse {
	return paymentTransactionRecordResponse{
		ID:                  txn.ID,
		Operation:           txn.Operation,
		ProviderTxnID:       txn.ProviderTxnID,
		IdempotencyKey:      txn.IdempotencyKey,
		Amount:              txn.Amount,
		Status:              txn.Status,
		RawResponseRedacted: txn.RawResponseRedacted,
		CreatedAt:           txn.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:           txn.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func mapLifecycleError(err error, responseStatus *int, responsePayload *any) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, paymentservice.ErrPaymentIntentNotFound):
		*responseStatus = http.StatusNotFound
		*responsePayload = gin.H{"error": "Payment intent not found"}
		return nil
	case errors.Is(err, paymentservice.ErrCaptureNotAllowed),
		errors.Is(err, paymentservice.ErrVoidNotAllowed),
		errors.Is(err, paymentservice.ErrRefundNotAllowed),
		errors.Is(err, paymentservice.ErrAmountExceedsAvailable):
		*responseStatus = http.StatusConflict
		*responsePayload = gin.H{"error": err.Error()}
		return nil
	case errors.Is(err, paymentservice.ErrAmountMustBePositive):
		*responseStatus = http.StatusBadRequest
		*responsePayload = gin.H{"error": err.Error()}
		return nil
	case errors.Is(err, providerops.ErrProviderCredentialWrongEnvironment):
		*responseStatus = http.StatusConflict
		*responsePayload = gin.H{"error": "Provider credential is not configured for this environment"}
		return nil
	case errors.Is(err, providerops.ErrUnsupportedProviderCurrency):
		*responseStatus = http.StatusBadRequest
		*responsePayload = gin.H{"error": "Provider does not support the requested currency"}
		return nil
	default:
		var stockErr *orderservice.InsufficientStockError
		if errors.As(err, &stockErr) {
			*responseStatus = http.StatusBadRequest
			*responsePayload = gin.H{
				"error":              "Insufficient stock",
				"product_variant_id": stockErr.ProductVariantID,
				"product_name":       stockErr.ProductName,
				"requested":          stockErr.Requested,
				"available":          stockErr.Available,
			}
			return nil
		}
		return err
	}
}

func adminActor(user *models.User) string {
	if user == nil {
		return "admin"
	}
	if strings.TrimSpace(user.Subject) != "" {
		return "admin:" + strings.TrimSpace(user.Subject)
	}
	return fmt.Sprintf("admin:%d", user.ID)
}

func bindOptionalStrictJSON(c *gin.Context, target any) error {
	if c.Request.Body == nil {
		return nil
	}
	if c.Request.ContentLength == 0 {
		return nil
	}
	return bindStrictJSON(c, target)
}

func parseUintParam(c *gin.Context, name string) (uint, error) {
	value, err := strconv.ParseUint(c.Param(name), 10, 32)
	if err != nil {
		return 0, err
	}
	return uint(value), nil
}
