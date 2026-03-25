package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"ecommerce/internal/dbcontext"
	checkoutservice "ecommerce/internal/services/checkout"
	orderservice "ecommerce/internal/services/orders"
	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const CheckoutSnapshotTTL = 15 * time.Minute

var (
	ErrSnapshotExpired             = errors.New("checkout snapshot has expired")
	ErrSnapshotNotFound            = errors.New("checkout snapshot not found")
	ErrSnapshotOrderMismatch       = errors.New("checkout snapshot no longer matches the order")
	ErrSnapshotAlreadyBound        = errors.New("checkout snapshot is already bound to a different order")
	ErrActivePaymentIntentExists   = errors.New("an active payment intent already exists for this order")
	ErrPaymentIntentNotFound       = errors.New("payment intent not found")
	ErrCaptureNotAllowed           = errors.New("payment intent cannot be captured")
	ErrVoidNotAllowed              = errors.New("payment intent cannot be voided")
	ErrRefundNotAllowed            = errors.New("payment intent cannot be refunded")
	ErrAmountMustBePositive        = errors.New("amount must be greater than zero")
	ErrAmountExceedsAvailable      = errors.New("amount exceeds available balance")
	ErrProviderTransactionNotFound = errors.New("provider transaction not found")
)

type SnapshotItemInput struct {
	ProductVariantID uint
	VariantSKU       string
	VariantTitle     string
	Quantity         int
	Price            models.Money
}

type CreateCheckoutSnapshotInput struct {
	CheckoutSessionID     uint
	Currency              string
	Subtotal              models.Money
	ShippingAmount        models.Money
	TaxAmount             models.Money
	Total                 models.Money
	PaymentProviderID     string
	ShippingProviderID    string
	TaxProviderID         string
	PaymentData           map[string]string
	ShippingData          map[string]string
	TaxData               map[string]string
	PaymentMethodDisplay  string
	ShippingAddressPretty string
	Items                 []SnapshotItemInput
	Now                   time.Time
}

func CreateCheckoutSnapshot(db *gorm.DB, input CreateCheckoutSnapshotInput) (models.OrderCheckoutSnapshot, error) {
	paymentDataJSON, err := marshalStringMap(input.PaymentData)
	if err != nil {
		return models.OrderCheckoutSnapshot{}, err
	}
	shippingDataJSON, err := marshalStringMap(input.ShippingData)
	if err != nil {
		return models.OrderCheckoutSnapshot{}, err
	}
	taxDataJSON, err := marshalStringMap(input.TaxData)
	if err != nil {
		return models.OrderCheckoutSnapshot{}, err
	}

	now := input.Now.UTC()
	snapshot := models.OrderCheckoutSnapshot{
		CheckoutSessionID:     input.CheckoutSessionID,
		Currency:              input.Currency,
		Subtotal:              input.Subtotal,
		ShippingAmount:        input.ShippingAmount,
		TaxAmount:             input.TaxAmount,
		Total:                 input.Total,
		PaymentProviderID:     input.PaymentProviderID,
		ShippingProviderID:    input.ShippingProviderID,
		TaxProviderID:         input.TaxProviderID,
		PaymentDataJSON:       paymentDataJSON,
		ShippingDataJSON:      shippingDataJSON,
		TaxDataJSON:           taxDataJSON,
		PaymentMethodDisplay:  input.PaymentMethodDisplay,
		ShippingAddressPretty: input.ShippingAddressPretty,
		ExpiresAt:             now.Add(CheckoutSnapshotTTL),
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&snapshot).Error; err != nil {
			return err
		}

		items := make([]models.OrderCheckoutSnapshotItem, 0, len(input.Items))
		for _, item := range input.Items {
			items = append(items, models.OrderCheckoutSnapshotItem{
				SnapshotID:       snapshot.ID,
				ProductVariantID: item.ProductVariantID,
				VariantSKU:       item.VariantSKU,
				VariantTitle:     item.VariantTitle,
				Quantity:         item.Quantity,
				Price:            item.Price,
			})
		}
		if len(items) > 0 {
			if err := tx.Create(&items).Error; err != nil {
				return err
			}
			snapshot.Items = items
		}
		return nil
	}); err != nil {
		return models.OrderCheckoutSnapshot{}, err
	}

	return snapshot, nil
}

func GetCheckoutSnapshotForSession(db *gorm.DB, checkoutSessionID, snapshotID uint) (models.OrderCheckoutSnapshot, error) {
	var snapshot models.OrderCheckoutSnapshot
	err := db.Where("id = ? AND checkout_session_id = ?", snapshotID, checkoutSessionID).
		Preload("Items").
		First(&snapshot).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.OrderCheckoutSnapshot{}, ErrSnapshotNotFound
	}
	if err != nil {
		return models.OrderCheckoutSnapshot{}, err
	}
	return snapshot, nil
}

func BindSnapshotToOrder(tx *gorm.DB, snapshot *models.OrderCheckoutSnapshot, orderID uint, now time.Time) error {
	if snapshot == nil {
		return fmt.Errorf("snapshot is required")
	}
	if snapshot.OrderID != nil && *snapshot.OrderID != orderID {
		return ErrSnapshotAlreadyBound
	}
	if snapshot.OrderID != nil && *snapshot.OrderID == orderID {
		return nil
	}

	timestamp := now.UTC()
	result := tx.Model(&models.OrderCheckoutSnapshot{}).
		Where("id = ? AND order_id IS NULL", snapshot.ID).
		Updates(map[string]any{
			"order_id":      orderID,
			"authorized_at": timestamp,
			"updated_at":    timestamp,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var current models.OrderCheckoutSnapshot
		if err := tx.Select("id", "order_id", "authorized_at").
			First(&current, snapshot.ID).Error; err != nil {
			return err
		}
		if current.OrderID != nil && *current.OrderID != orderID {
			return ErrSnapshotAlreadyBound
		}
	}

	orderIDCopy := orderID
	snapshot.OrderID = &orderIDCopy
	snapshot.AuthorizedAt = &timestamp
	return nil
}

func ValidateSnapshotForOrder(snapshot *models.OrderCheckoutSnapshot, order *models.Order, now time.Time) error {
	if snapshot == nil {
		return fmt.Errorf("snapshot is required")
	}
	if order == nil {
		return fmt.Errorf("order is required")
	}
	if snapshot.ExpiresAt.Before(now.UTC()) {
		return ErrSnapshotExpired
	}
	if snapshot.OrderID != nil && *snapshot.OrderID != order.ID {
		return ErrSnapshotAlreadyBound
	}
	if snapshot.Subtotal != order.Total && snapshot.Total != order.Total {
		return ErrSnapshotOrderMismatch
	}

	snapshotItems := make([]models.OrderCheckoutSnapshotItem, len(snapshot.Items))
	copy(snapshotItems, snapshot.Items)
	orderItems := make([]models.OrderItem, len(order.Items))
	copy(orderItems, order.Items)

	slices.SortFunc(snapshotItems, func(a, b models.OrderCheckoutSnapshotItem) int {
		if a.ProductVariantID != b.ProductVariantID {
			if a.ProductVariantID < b.ProductVariantID {
				return -1
			}
			return 1
		}
		if a.Quantity != b.Quantity {
			if a.Quantity < b.Quantity {
				return -1
			}
			return 1
		}
		if a.Price != b.Price {
			if a.Price < b.Price {
				return -1
			}
			return 1
		}
		return 0
	})
	slices.SortFunc(orderItems, func(a, b models.OrderItem) int {
		if a.ProductVariantID != b.ProductVariantID {
			if a.ProductVariantID < b.ProductVariantID {
				return -1
			}
			return 1
		}
		if a.Quantity != b.Quantity {
			if a.Quantity < b.Quantity {
				return -1
			}
			return 1
		}
		if a.Price != b.Price {
			if a.Price < b.Price {
				return -1
			}
			return 1
		}
		return 0
	})

	if len(snapshotItems) != len(orderItems) {
		return ErrSnapshotOrderMismatch
	}
	for i := range snapshotItems {
		if snapshotItems[i].ProductVariantID != orderItems[i].ProductVariantID ||
			snapshotItems[i].Quantity != orderItems[i].Quantity ||
			snapshotItems[i].Price != orderItems[i].Price {
			return ErrSnapshotOrderMismatch
		}
	}

	return nil
}

func PrepareAuthorizedPaymentIntent(
	tx *gorm.DB,
	orderID uint,
	snapshot models.OrderCheckoutSnapshot,
	idempotencyKey string,
) (models.PaymentIntent, models.PaymentTransaction, error) {
	var existing models.PaymentIntent
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Transactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC, id ASC")
		}).
		Where("order_id = ?", orderID).
		Order("id DESC").
		First(&existing).Error
	if err == nil && models.IsActivePaymentIntentStatus(existing.Status) {
		if existing.Provider == snapshot.PaymentProviderID &&
			existing.SnapshotID == snapshot.ID &&
			existing.AuthorizedAmount == snapshot.Total {
			if existing.Status == models.PaymentIntentStatusRequiresAction {
				if txn, ok := authorizeTransaction(existing.Transactions); ok && txn.Status == models.PaymentTransactionStatusPending {
					return existing, txn, nil
				}
			}
			if existing.Status == models.PaymentIntentStatusAuthorized {
				if txn, ok := authorizeTransaction(existing.Transactions); ok && txn.Status == models.PaymentTransactionStatusSucceeded {
					return existing, txn, nil
				}
			}
		}
		return models.PaymentIntent{}, models.PaymentTransaction{}, ErrActivePaymentIntentExists
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.PaymentIntent{}, models.PaymentTransaction{}, err
	}

	intent := models.PaymentIntent{
		OrderID:          orderID,
		SnapshotID:       snapshot.ID,
		Provider:         snapshot.PaymentProviderID,
		Status:           models.PaymentIntentStatusRequiresAction,
		AuthorizedAmount: snapshot.Total,
		CapturedAmount:   0,
		Currency:         snapshot.Currency,
		Version:          1,
	}
	if err := tx.Create(&intent).Error; err != nil {
		return models.PaymentIntent{}, models.PaymentTransaction{}, err
	}

	txn := models.PaymentTransaction{
		PaymentIntentID:     intent.ID,
		Operation:           models.PaymentTransactionOperationAuthorize,
		ProviderTxnID:       "",
		IdempotencyKey:      idempotencyKey,
		Amount:              snapshot.Total,
		Status:              models.PaymentTransactionStatusPending,
		RawResponseRedacted: "",
	}
	if err := tx.Create(&txn).Error; err != nil {
		return models.PaymentIntent{}, models.PaymentTransaction{}, err
	}
	intent.Transactions = []models.PaymentTransaction{txn}
	return intent, txn, nil
}

func AuthorizePreparedPaymentIntent(
	ctx context.Context,
	db *gorm.DB,
	registry ProviderRegistry,
	intentID uint,
	snapshot models.OrderCheckoutSnapshot,
	correlationID string,
) (models.PaymentIntent, models.PaymentTransaction, error) {
	var preparedIntent models.PaymentIntent
	if err := db.Preload("Transactions", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at ASC, id ASC")
	}).First(&preparedIntent, intentID).Error; err != nil {
		return models.PaymentIntent{}, models.PaymentTransaction{}, err
	}

	authorizeTxn, ok := authorizeTransaction(preparedIntent.Transactions)
	if !ok {
		return models.PaymentIntent{}, models.PaymentTransaction{}, fmt.Errorf("authorize transaction not found for intent %d", preparedIntent.ID)
	}
	if preparedIntent.Status == models.PaymentIntentStatusAuthorized &&
		authorizeTxn.Status == models.PaymentTransactionStatusSucceeded {
		return preparedIntent, authorizeTxn, nil
	}
	if preparedIntent.Status != models.PaymentIntentStatusRequiresAction ||
		authorizeTxn.Status != models.PaymentTransactionStatusPending {
		return models.PaymentIntent{}, models.PaymentTransaction{}, ErrActivePaymentIntentExists
	}

	provider, err := registry.Provider(snapshot.PaymentProviderID)
	if err != nil {
		return models.PaymentIntent{}, models.PaymentTransaction{}, err
	}
	paymentData, err := unmarshalStringMap(snapshot.PaymentDataJSON)
	if err != nil {
		return models.PaymentIntent{}, models.PaymentTransaction{}, err
	}

	providerResult, err := provider.Authorize(ctx, AuthorizeRequest{
		OrderID:              preparedIntent.OrderID,
		SnapshotID:           snapshot.ID,
		Amount:               snapshot.Total,
		Currency:             snapshot.Currency,
		Provider:             snapshot.PaymentProviderID,
		IdempotencyKey:       authorizeTxn.IdempotencyKey,
		CorrelationID:        correlationID,
		PaymentMethodDisplay: snapshot.PaymentMethodDisplay,
		PaymentData:          paymentData,
	})
	if err != nil {
		markErr := db.Transaction(func(tx *gorm.DB) error {
			return markPreparedAuthorizationFailed(tx, preparedIntent.ID, err.Error())
		})
		if markErr != nil {
			return models.PaymentIntent{}, models.PaymentTransaction{}, fmt.Errorf("%w: %v", err, markErr)
		}
		return models.PaymentIntent{}, models.PaymentTransaction{}, err
	}

	var finalizedIntent models.PaymentIntent
	var finalizedTxn models.PaymentTransaction
	if err := db.Transaction(func(tx *gorm.DB) error {
		var finalizeErr error
		finalizedIntent, finalizedTxn, finalizeErr = finalizePreparedAuthorization(tx, preparedIntent.ID, providerResult)
		return finalizeErr
	}); err != nil {
		return models.PaymentIntent{}, models.PaymentTransaction{}, err
	}
	return finalizedIntent, finalizedTxn, nil
}

func ApplyAuthorizedCheckoutState(
	tx *gorm.DB,
	order *models.Order,
	snapshot models.OrderCheckoutSnapshot,
	correlationID string,
) error {
	if tx == nil {
		return fmt.Errorf("transaction is required")
	}
	if order == nil {
		return fmt.Errorf("order is required")
	}

	fromStatus := order.Status
	order.Status = models.StatusPending
	order.Total = snapshot.Total
	order.PaymentMethodDisplay = snapshot.PaymentMethodDisplay
	order.ShippingAddressPretty = snapshot.ShippingAddressPretty
	if err := tx.Save(order).Error; err != nil {
		return err
	}
	if err := AppendOrderStatusHistory(
		tx,
		order.ID,
		fromStatus,
		order.Status,
		"payment_authorized",
		"checkout",
		"customer",
		correlationID,
	); err != nil {
		return err
	}
	if err := checkoutservice.ClearOrderedItemsFromCart(tx, order.CheckoutSessionID, order.Items); err != nil {
		return err
	}
	return tx.Model(&models.CheckoutSession{}).
		Where("id = ?", order.CheckoutSessionID).
		Updates(map[string]any{
			"status":      models.CheckoutSessionStatusConverted,
			"guest_email": order.GuestEmail,
		}).Error
}

func GetPaymentIntentForUpdate(tx *gorm.DB, orderID, intentID uint) (models.PaymentIntent, error) {
	var intent models.PaymentIntent
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Transactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC, id ASC")
		}).
		Where("id = ? AND order_id = ?", intentID, orderID).
		First(&intent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.PaymentIntent{}, ErrPaymentIntentNotFound
	}
	if err != nil {
		return models.PaymentIntent{}, err
	}
	return intent, nil
}

func GetOrderPaymentLedger(db *gorm.DB, orderID uint) ([]models.PaymentIntent, error) {
	var intents []models.PaymentIntent
	if err := db.Preload("Transactions", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at ASC, id ASC")
	}).
		Where("order_id = ?", orderID).
		Order("created_at ASC, id ASC").
		Find(&intents).Error; err != nil {
		return nil, err
	}
	return intents, nil
}

func CapturePaymentIntent(
	ctx context.Context,
	tx *gorm.DB,
	registry ProviderRegistry,
	intent *models.PaymentIntent,
	amount *models.Money,
	idempotencyKey string,
	correlationID string,
) (models.PaymentTransaction, error) {
	ctx = dbcontext.WithDB(ctx, tx)

	if intent == nil {
		return models.PaymentTransaction{}, fmt.Errorf("payment intent is required")
	}
	if intent.Status != models.PaymentIntentStatusAuthorized && intent.Status != models.PaymentIntentStatusPartiallyCaptured {
		return models.PaymentTransaction{}, ErrCaptureNotAllowed
	}

	remaining := intent.AuthorizedAmount - intent.CapturedAmount
	captureAmount, err := resolveLifecycleAmount(amount, remaining)
	if err != nil {
		return models.PaymentTransaction{}, err
	}

	provider, err := registry.Provider(intent.Provider)
	if err != nil {
		return models.PaymentTransaction{}, err
	}
	providerResult, err := provider.Capture(ctx, CaptureRequest{
		OrderID:          intent.OrderID,
		IntentID:         intent.ID,
		Amount:           captureAmount,
		Currency:         intent.Currency,
		Provider:         intent.Provider,
		IdempotencyKey:   idempotencyKey,
		CorrelationID:    correlationID,
		ProviderTxnIDRef: latestProviderTxnID(intent.Transactions, models.PaymentTransactionOperationAuthorize),
	})
	if err != nil {
		return models.PaymentTransaction{}, err
	}

	txn := models.PaymentTransaction{
		PaymentIntentID:     intent.ID,
		Operation:           models.PaymentTransactionOperationCapture,
		ProviderTxnID:       providerResult.ProviderTxnID,
		IdempotencyKey:      idempotencyKey,
		Amount:              captureAmount,
		Status:              models.PaymentTransactionStatusSucceeded,
		RawResponseRedacted: providerResult.RawResponseRedacted,
	}
	if err := tx.Create(&txn).Error; err != nil {
		return models.PaymentTransaction{}, err
	}

	intent.CapturedAmount += captureAmount
	if intent.CapturedAmount >= intent.AuthorizedAmount {
		intent.CapturedAmount = intent.AuthorizedAmount
		intent.Status = models.PaymentIntentStatusCaptured
	} else {
		intent.Status = models.PaymentIntentStatusPartiallyCaptured
	}
	intent.Version++
	if err := tx.Save(intent).Error; err != nil {
		return models.PaymentTransaction{}, err
	}
	intent.Transactions = append(intent.Transactions, txn)
	return txn, nil
}

func VoidPaymentIntent(
	ctx context.Context,
	tx *gorm.DB,
	registry ProviderRegistry,
	intent *models.PaymentIntent,
	idempotencyKey string,
	correlationID string,
) (models.PaymentTransaction, error) {
	ctx = dbcontext.WithDB(ctx, tx)

	if intent == nil {
		return models.PaymentTransaction{}, fmt.Errorf("payment intent is required")
	}
	if intent.Status != models.PaymentIntentStatusAuthorized || intent.CapturedAmount > 0 {
		return models.PaymentTransaction{}, ErrVoidNotAllowed
	}

	provider, err := registry.Provider(intent.Provider)
	if err != nil {
		return models.PaymentTransaction{}, err
	}
	voidAmount := intent.AuthorizedAmount
	providerResult, err := provider.Void(ctx, VoidRequest{
		OrderID:          intent.OrderID,
		IntentID:         intent.ID,
		Amount:           voidAmount,
		Currency:         intent.Currency,
		Provider:         intent.Provider,
		IdempotencyKey:   idempotencyKey,
		CorrelationID:    correlationID,
		ProviderTxnIDRef: latestProviderTxnID(intent.Transactions, models.PaymentTransactionOperationAuthorize),
	})
	if err != nil {
		return models.PaymentTransaction{}, err
	}

	txn := models.PaymentTransaction{
		PaymentIntentID:     intent.ID,
		Operation:           models.PaymentTransactionOperationVoid,
		ProviderTxnID:       providerResult.ProviderTxnID,
		IdempotencyKey:      idempotencyKey,
		Amount:              voidAmount,
		Status:              models.PaymentTransactionStatusSucceeded,
		RawResponseRedacted: providerResult.RawResponseRedacted,
	}
	if err := tx.Create(&txn).Error; err != nil {
		return models.PaymentTransaction{}, err
	}

	intent.Status = models.PaymentIntentStatusVoided
	intent.Version++
	if err := tx.Save(intent).Error; err != nil {
		return models.PaymentTransaction{}, err
	}
	intent.Transactions = append(intent.Transactions, txn)
	return txn, nil
}

func RefundPaymentIntent(
	ctx context.Context,
	tx *gorm.DB,
	registry ProviderRegistry,
	intent *models.PaymentIntent,
	amount *models.Money,
	idempotencyKey string,
	correlationID string,
) (models.PaymentTransaction, error) {
	ctx = dbcontext.WithDB(ctx, tx)

	if intent == nil {
		return models.PaymentTransaction{}, fmt.Errorf("payment intent is required")
	}
	if intent.CapturedAmount <= 0 {
		return models.PaymentTransaction{}, ErrRefundNotAllowed
	}
	if intent.Status != models.PaymentIntentStatusCaptured &&
		intent.Status != models.PaymentIntentStatusPartiallyCaptured &&
		intent.Status != models.PaymentIntentStatusRefunded {
		return models.PaymentTransaction{}, ErrRefundNotAllowed
	}

	refunded := refundedAmount(intent.Transactions)
	remaining := intent.CapturedAmount - refunded
	refundAmount, err := resolveLifecycleAmount(amount, remaining)
	if err != nil {
		return models.PaymentTransaction{}, err
	}

	provider, err := registry.Provider(intent.Provider)
	if err != nil {
		return models.PaymentTransaction{}, err
	}
	providerResult, err := provider.Refund(ctx, RefundRequest{
		OrderID:          intent.OrderID,
		IntentID:         intent.ID,
		Amount:           refundAmount,
		Currency:         intent.Currency,
		Provider:         intent.Provider,
		IdempotencyKey:   idempotencyKey,
		CorrelationID:    correlationID,
		ProviderTxnIDRef: latestProviderTxnID(intent.Transactions, models.PaymentTransactionOperationCapture),
	})
	if err != nil {
		return models.PaymentTransaction{}, err
	}

	txn := models.PaymentTransaction{
		PaymentIntentID:     intent.ID,
		Operation:           models.PaymentTransactionOperationRefund,
		ProviderTxnID:       providerResult.ProviderTxnID,
		IdempotencyKey:      idempotencyKey,
		Amount:              refundAmount,
		Status:              models.PaymentTransactionStatusSucceeded,
		RawResponseRedacted: providerResult.RawResponseRedacted,
	}
	if err := tx.Create(&txn).Error; err != nil {
		return models.PaymentTransaction{}, err
	}

	intent.Version++
	if refunded+refundAmount >= intent.CapturedAmount {
		intent.Status = models.PaymentIntentStatusRefunded
	}
	if err := tx.Save(intent).Error; err != nil {
		return models.PaymentTransaction{}, err
	}
	intent.Transactions = append(intent.Transactions, txn)
	return txn, nil
}

func RefundedAmount(intent models.PaymentIntent) models.Money {
	return refundedAmount(intent.Transactions)
}

func ApplyWebhookPaymentEvent(
	tx *gorm.DB,
	event VerifiedWebhookEvent,
	correlationID string,
) (models.PaymentTransaction, models.PaymentIntent, models.Order, error) {
	if tx == nil {
		return models.PaymentTransaction{}, models.PaymentIntent{}, models.Order{}, fmt.Errorf("transaction is required")
	}
	if strings.TrimSpace(event.ProviderTxnID) == "" {
		return models.PaymentTransaction{}, models.PaymentIntent{}, models.Order{}, fmt.Errorf("provider transaction id is required")
	}

	targetTxnStatus, err := webhookTransactionStatus(event.EventType)
	if err != nil {
		return models.PaymentTransaction{}, models.PaymentIntent{}, models.Order{}, err
	}

	var txn models.PaymentTransaction
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("provider_txn_id = ?", strings.TrimSpace(event.ProviderTxnID)).
		First(&txn).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.PaymentTransaction{}, models.PaymentIntent{}, models.Order{}, ErrProviderTransactionNotFound
		}
		return models.PaymentTransaction{}, models.PaymentIntent{}, models.Order{}, err
	}

	var intent models.PaymentIntent
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Transactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC, id ASC")
		}).
		First(&intent, txn.PaymentIntentID).Error; err != nil {
		return models.PaymentTransaction{}, models.PaymentIntent{}, models.Order{}, err
	}

	var order models.Order
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, intent.OrderID).Error; err != nil {
		return models.PaymentTransaction{}, models.PaymentIntent{}, models.Order{}, err
	}

	if txn.Status != targetTxnStatus {
		txn.Status = targetTxnStatus
		if err := tx.Save(&txn).Error; err != nil {
			return models.PaymentTransaction{}, models.PaymentIntent{}, models.Order{}, err
		}
	}
	replaceTransaction(&intent.Transactions, txn)

	capturedAmount, nextStatus := deriveWebhookIntentState(intent)
	if intent.CapturedAmount != capturedAmount || intent.Status != nextStatus {
		intent.CapturedAmount = capturedAmount
		intent.Status = nextStatus
		intent.Version++
		if err := tx.Save(&intent).Error; err != nil {
			return models.PaymentTransaction{}, models.PaymentIntent{}, models.Order{}, err
		}
	}

	if err := applyWebhookOrderStatus(tx, &order, intent, event.Provider, correlationID); err != nil {
		return models.PaymentTransaction{}, models.PaymentIntent{}, models.Order{}, err
	}

	return txn, intent, order, nil
}

func AppendOrderStatusHistory(
	tx *gorm.DB,
	orderID uint,
	fromStatus, toStatus, reason, source, actor, correlationID string,
) error {
	entry := models.OrderStatusHistory{
		OrderID:       orderID,
		FromStatus:    fromStatus,
		ToStatus:      toStatus,
		Reason:        reason,
		Source:        source,
		Actor:         actor,
		CorrelationID: correlationID,
	}
	return tx.Create(&entry).Error
}

func resolveLifecycleAmount(requested *models.Money, available models.Money) (models.Money, error) {
	if available <= 0 {
		return 0, ErrAmountExceedsAvailable
	}
	if requested == nil {
		return available, nil
	}
	if *requested <= 0 {
		return 0, ErrAmountMustBePositive
	}
	if *requested > available {
		return 0, ErrAmountExceedsAvailable
	}
	return *requested, nil
}

func refundedAmount(transactions []models.PaymentTransaction) models.Money {
	var refunded models.Money
	for _, txn := range transactions {
		if txn.Operation == models.PaymentTransactionOperationRefund &&
			txn.Status == models.PaymentTransactionStatusSucceeded {
			refunded += txn.Amount
		}
	}
	return refunded
}

func latestProviderTxnID(transactions []models.PaymentTransaction, operation string) string {
	for i := len(transactions) - 1; i >= 0; i-- {
		if transactions[i].Operation == operation {
			return transactions[i].ProviderTxnID
		}
	}
	return ""
}

func authorizeTransaction(transactions []models.PaymentTransaction) (models.PaymentTransaction, bool) {
	for i := len(transactions) - 1; i >= 0; i-- {
		if transactions[i].Operation == models.PaymentTransactionOperationAuthorize {
			return transactions[i], true
		}
	}
	return models.PaymentTransaction{}, false
}

func finalizePreparedAuthorization(
	tx *gorm.DB,
	intentID uint,
	providerResult ProviderOperationResult,
) (models.PaymentIntent, models.PaymentTransaction, error) {
	var intent models.PaymentIntent
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Transactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC, id ASC")
		}).
		First(&intent, intentID).Error; err != nil {
		return models.PaymentIntent{}, models.PaymentTransaction{}, err
	}

	authorizeTxn, ok := authorizeTransaction(intent.Transactions)
	if !ok {
		return models.PaymentIntent{}, models.PaymentTransaction{}, fmt.Errorf("authorize transaction not found for intent %d", intent.ID)
	}
	if intent.Status == models.PaymentIntentStatusAuthorized &&
		authorizeTxn.Status == models.PaymentTransactionStatusSucceeded {
		return intent, authorizeTxn, nil
	}

	authorizeTxn.ProviderTxnID = providerResult.ProviderTxnID
	authorizeTxn.Status = models.PaymentTransactionStatusSucceeded
	authorizeTxn.RawResponseRedacted = providerResult.RawResponseRedacted
	if err := tx.Model(&models.PaymentTransaction{}).
		Where("id = ?", authorizeTxn.ID).
		Updates(map[string]any{
			"provider_txn_id":       authorizeTxn.ProviderTxnID,
			"status":                authorizeTxn.Status,
			"raw_response_redacted": authorizeTxn.RawResponseRedacted,
		}).Error; err != nil {
		return models.PaymentIntent{}, models.PaymentTransaction{}, err
	}

	intent.Status = models.PaymentIntentStatusAuthorized
	if err := tx.Model(&models.PaymentIntent{}).
		Where("id = ?", intent.ID).
		Update("status", intent.Status).Error; err != nil {
		return models.PaymentIntent{}, models.PaymentTransaction{}, err
	}
	replaceTransaction(&intent.Transactions, authorizeTxn)
	return intent, authorizeTxn, nil
}

func markPreparedAuthorizationFailed(tx *gorm.DB, intentID uint, failure string) error {
	var intent models.PaymentIntent
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Transactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC, id ASC")
		}).
		First(&intent, intentID).Error; err != nil {
		return err
	}

	if intent.Status == models.PaymentIntentStatusFailed {
		return nil
	}

	authorizeTxn, ok := authorizeTransaction(intent.Transactions)
	if !ok {
		return fmt.Errorf("authorize transaction not found for intent %d", intent.ID)
	}
	if authorizeTxn.Status == models.PaymentTransactionStatusPending {
		if err := tx.Model(&models.PaymentTransaction{}).
			Where("id = ?", authorizeTxn.ID).
			Updates(map[string]any{
				"status":                models.PaymentTransactionStatusFailed,
				"raw_response_redacted": failure,
			}).Error; err != nil {
			return err
		}
	}
	return tx.Model(&models.PaymentIntent{}).
		Where("id = ?", intent.ID).
		Update("status", models.PaymentIntentStatusFailed).Error
}

func marshalStringMap(value map[string]string) (string, error) {
	if len(value) == 0 {
		return "", nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func webhookTransactionStatus(eventType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "payment.authorized",
		"payment.captured",
		"payment.voided",
		"payment.refunded":
		return models.PaymentTransactionStatusSucceeded, nil
	case "payment.failed",
		"payment.authorization_failed",
		"payment.capture_failed",
		"payment.void_failed",
		"payment.refund_failed":
		return models.PaymentTransactionStatusFailed, nil
	default:
		return "", fmt.Errorf("unsupported payment webhook event type: %s", eventType)
	}
}

func replaceTransaction(transactions *[]models.PaymentTransaction, txn models.PaymentTransaction) {
	if transactions == nil {
		return
	}
	for i := range *transactions {
		if (*transactions)[i].ID == txn.ID {
			(*transactions)[i] = txn
			return
		}
	}
	*transactions = append(*transactions, txn)
}

func deriveWebhookIntentState(intent models.PaymentIntent) (models.Money, string) {
	var (
		capturedAmount     models.Money
		refunded           models.Money
		authorizeSucceeded bool
		authorizeFailed    bool
		voidSucceeded      bool
	)

	for _, txn := range intent.Transactions {
		switch txn.Operation {
		case models.PaymentTransactionOperationAuthorize:
			if txn.Status == models.PaymentTransactionStatusSucceeded {
				authorizeSucceeded = true
			}
			if txn.Status == models.PaymentTransactionStatusFailed {
				authorizeFailed = true
			}
		case models.PaymentTransactionOperationCapture:
			if txn.Status == models.PaymentTransactionStatusSucceeded {
				capturedAmount += txn.Amount
			}
		case models.PaymentTransactionOperationRefund:
			if txn.Status == models.PaymentTransactionStatusSucceeded {
				refunded += txn.Amount
			}
		case models.PaymentTransactionOperationVoid:
			if txn.Status == models.PaymentTransactionStatusSucceeded {
				voidSucceeded = true
			}
		}
	}

	if capturedAmount > intent.AuthorizedAmount {
		capturedAmount = intent.AuthorizedAmount
	}

	switch {
	case capturedAmount > 0 && refunded >= capturedAmount:
		return capturedAmount, models.PaymentIntentStatusRefunded
	case capturedAmount >= intent.AuthorizedAmount && intent.AuthorizedAmount > 0:
		return capturedAmount, models.PaymentIntentStatusCaptured
	case capturedAmount > 0:
		return capturedAmount, models.PaymentIntentStatusPartiallyCaptured
	case voidSucceeded:
		return 0, models.PaymentIntentStatusVoided
	case authorizeSucceeded:
		return 0, models.PaymentIntentStatusAuthorized
	case authorizeFailed:
		return 0, models.PaymentIntentStatusFailed
	default:
		return intent.CapturedAmount, intent.Status
	}
}

func applyWebhookOrderStatus(
	tx *gorm.DB,
	order *models.Order,
	intent models.PaymentIntent,
	provider string,
	correlationID string,
) error {
	if tx == nil {
		return fmt.Errorf("transaction is required")
	}
	if order == nil {
		return fmt.Errorf("order is required")
	}

	targetStatus := ""
	reason := ""
	switch intent.Status {
	case models.PaymentIntentStatusCaptured, models.PaymentIntentStatusPartiallyCaptured:
		if order.Status != models.StatusPaid &&
			order.Status != models.StatusShipped &&
			order.Status != models.StatusDelivered &&
			order.Status != models.StatusRefunded {
			targetStatus = models.StatusPaid
			reason = "payment_captured"
		}
	case models.PaymentIntentStatusVoided:
		if order.Status != models.StatusCancelled {
			targetStatus = models.StatusCancelled
			reason = "payment_voided"
		}
	case models.PaymentIntentStatusRefunded:
		if order.Status != models.StatusRefunded {
			targetStatus = models.StatusRefunded
			reason = "payment_refunded"
		}
	case models.PaymentIntentStatusFailed:
		if order.Status != models.StatusFailed {
			targetStatus = models.StatusFailed
			reason = "payment_failed"
		}
	}

	if targetStatus == "" {
		return nil
	}

	fromStatus := order.Status
	if err := orderservice.ApplyStatusTransition(tx, order, targetStatus); err != nil {
		return err
	}
	return AppendOrderStatusHistory(
		tx,
		order.ID,
		fromStatus,
		order.Status,
		reason,
		"webhook",
		"provider:"+strings.TrimSpace(provider),
		correlationID,
	)
}

func unmarshalStringMap(value string) (map[string]string, error) {
	if value == "" {
		return map[string]string{}, nil
	}
	var data map[string]string
	if err := json.Unmarshal([]byte(value), &data); err != nil {
		return nil, err
	}
	if data == nil {
		data = map[string]string{}
	}
	return data, nil
}
