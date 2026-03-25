package webhooks

import (
	"context"
	"io"
	"log"
	"testing"
	"time"

	"ecommerce/internal/migrations"
	paymentservice "ecommerce/internal/services/payments"
	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newWebhookTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, migrations.RunWithoutContract(db))
	return db
}

func waitForWebhookState(
	t *testing.T,
	db *gorm.DB,
	eventID uint,
	check func(models.WebhookEvent) bool,
) models.WebhookEvent {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var event models.WebhookEvent
		if err := db.First(&event, eventID).Error; err == nil && check(event) {
			return event
		}
		time.Sleep(10 * time.Millisecond)
	}

	var event models.WebhookEvent
	require.NoError(t, db.First(&event, eventID).Error)
	require.True(t, check(event), "webhook event %d did not reach expected state: %+v", eventID, event)
	return event
}

func TestReceiveWebhookDuplicateReenqueuesDeadLetteredEvent(t *testing.T) {
	db := newWebhookTestDB(t)
	service := NewService(db, nil, nil, log.New(io.Discard, "", 0))
	service.StartProcessor()
	t.Cleanup(func() {
		close(service.Queue)
	})

	ctx := context.Background()
	body := []byte(`{"id":"evt-duplicate-recovery","type":"payment.captured","data":{"provider_txn_id":"dummy-card|CAPTURE|1|1|USD|10.00|dup-recovery"}}`)

	event, duplicate, err := service.ReceiveWebhook(ctx, "dummy-card", map[string]string{"X-Dummy-Signature": "valid"}, body)
	require.NoError(t, err)
	assert.False(t, duplicate)

	waitForWebhookState(t, db, event.ID, func(event models.WebhookEvent) bool {
		return event.ProcessedAt == nil &&
			event.AttemptCount >= service.MaxAttempts &&
			event.LastError == paymentservice.ErrProviderTransactionNotFound.Error()
	})

	session := models.CheckoutSession{
		PublicToken: "webhook-duplicate-recovery",
		Status:      models.CheckoutSessionStatusConverted,
		ExpiresAt:   time.Now().UTC().Add(time.Hour),
		LastSeenAt:  time.Now().UTC(),
	}
	require.NoError(t, db.Create(&session).Error)

	order := models.Order{
		CheckoutSessionID: session.ID,
		Total:             models.MoneyFromFloat(10),
		Status:            models.StatusPending,
	}
	require.NoError(t, db.Create(&order).Error)

	intent := models.PaymentIntent{
		OrderID:          order.ID,
		SnapshotID:       1,
		Provider:         "dummy-card",
		Status:           models.PaymentIntentStatusAuthorized,
		AuthorizedAmount: models.MoneyFromFloat(10),
		CapturedAmount:   0,
		Currency:         "USD",
		Version:          1,
	}
	require.NoError(t, db.Create(&intent).Error)
	require.NoError(t, db.Create(&models.PaymentTransaction{
		PaymentIntentID:     intent.ID,
		Operation:           models.PaymentTransactionOperationCapture,
		ProviderTxnID:       "dummy-card|CAPTURE|1|1|USD|10.00|dup-recovery",
		IdempotencyKey:      "dup-recovery",
		Amount:              models.MoneyFromFloat(10),
		Status:              models.PaymentTransactionStatusPending,
		RawResponseRedacted: "{}",
	}).Error)

	duplicateEvent, duplicate, err := service.ReceiveWebhook(ctx, "dummy-card", map[string]string{"X-Dummy-Signature": "valid"}, body)
	require.NoError(t, err)
	assert.True(t, duplicate)
	assert.Equal(t, event.ID, duplicateEvent.ID)

	waitForWebhookState(t, db, event.ID, func(event models.WebhookEvent) bool {
		return event.ProcessedAt != nil && event.AttemptCount == 1 && event.LastError == ""
	})

	var txn models.PaymentTransaction
	require.NoError(t, db.Where("provider_txn_id = ?", "dummy-card|CAPTURE|1|1|USD|10.00|dup-recovery").First(&txn).Error)
	assert.Equal(t, models.PaymentTransactionStatusSucceeded, txn.Status)

	var refreshedIntent models.PaymentIntent
	require.NoError(t, db.First(&refreshedIntent, intent.ID).Error)
	assert.Equal(t, models.PaymentIntentStatusCaptured, refreshedIntent.Status)
	assert.Equal(t, txn.Amount, refreshedIntent.CapturedAmount)
	assert.True(t, refreshedIntent.CapturedAmount > 0)

	var refreshedOrder models.Order
	require.NoError(t, db.First(&refreshedOrder, order.ID).Error)
	assert.Equal(t, models.StatusPaid, refreshedOrder.Status)
}

func TestReceiveWebhookPersistsRejectedSignatureAttemptsIndividually(t *testing.T) {
	db := newWebhookTestDB(t)
	service := NewService(db, nil, nil, log.New(io.Discard, "", 0))

	body := []byte(`{"id":"evt-rejected","type":"payment.captured","data":{"provider_txn_id":"dummy-card|CAPTURE|1|1|USD|10.00|rejected"}}`)
	event, duplicate, err := service.ReceiveWebhook(
		context.Background(),
		"dummy-card",
		map[string]string{"X-Dummy-Signature": "invalid"},
		body,
	)
	require.ErrorIs(t, err, paymentservice.ErrInvalidWebhookSignature)
	assert.False(t, duplicate)
	assert.Zero(t, event.ID)

	event, duplicate, err = service.ReceiveWebhook(
		context.Background(),
		"dummy-card",
		map[string]string{"X-Dummy-Signature": "invalid"},
		[]byte(`{"id":"evt-rejected-2","type":"payment.captured","data":{"provider_txn_id":"dummy-card|CAPTURE|1|1|USD|10.00|rejected-2"}}`),
	)
	require.ErrorIs(t, err, paymentservice.ErrInvalidWebhookSignature)
	assert.False(t, duplicate)
	assert.Zero(t, event.ID)

	var events []models.WebhookEvent
	require.NoError(t, db.Order("id ASC").Find(&events).Error)
	require.Len(t, events, 2)
	assert.NotEqual(t, events[0].ProviderEventID, events[1].ProviderEventID)
	for i, event := range events {
		assert.Contains(t, event.ProviderEventID, "signature.invalid.")
		assert.False(t, event.SignatureValid, "event %d", i)
		assert.Equal(t, "signature.invalid", event.EventType, "event %d", i)
		assert.Equal(t, EventStatusRejected, EventStatus(&event, service.MaxAttempts), "event %d", i)
		assert.Equal(t, 1, event.AttemptCount, "event %d", i)
		assert.Equal(t, paymentservice.ErrInvalidWebhookSignature.Error(), event.LastError, "event %d", i)
	}
	assert.Contains(t, events[0].Payload, "evt-rejected")
	assert.Contains(t, events[1].Payload, "evt-rejected-2")
}

func TestProcessStoredEventAppliesPaymentWebhookState(t *testing.T) {
	db := newWebhookTestDB(t)
	service := NewService(db, nil, nil, log.New(io.Discard, "", 0))
	service.StartProcessor()
	t.Cleanup(func() {
		close(service.Queue)
	})

	session := models.CheckoutSession{
		PublicToken: "webhook-payment-sync",
		Status:      models.CheckoutSessionStatusConverted,
		ExpiresAt:   time.Now().UTC().Add(time.Hour),
		LastSeenAt:  time.Now().UTC(),
	}
	require.NoError(t, db.Create(&session).Error)

	order := models.Order{
		CheckoutSessionID: session.ID,
		Total:             models.MoneyFromFloat(15),
		Status:            models.StatusPending,
	}
	require.NoError(t, db.Create(&order).Error)

	intent := models.PaymentIntent{
		OrderID:          order.ID,
		SnapshotID:       1,
		Provider:         "dummy-card",
		Status:           models.PaymentIntentStatusAuthorized,
		AuthorizedAmount: models.MoneyFromFloat(15),
		CapturedAmount:   0,
		Currency:         "USD",
		Version:          1,
	}
	require.NoError(t, db.Create(&intent).Error)

	txn := models.PaymentTransaction{
		PaymentIntentID:     intent.ID,
		Operation:           models.PaymentTransactionOperationCapture,
		ProviderTxnID:       "dummy-card|CAPTURE|2|1|USD|15.00|payment-sync",
		IdempotencyKey:      "payment-sync",
		Amount:              models.MoneyFromFloat(15),
		Status:              models.PaymentTransactionStatusPending,
		RawResponseRedacted: "{}",
	}
	require.NoError(t, db.Create(&txn).Error)

	event, duplicate, err := service.ReceiveWebhook(
		context.Background(),
		"dummy-card",
		map[string]string{"X-Dummy-Signature": "valid"},
		[]byte(`{"id":"evt-payment-sync","type":"payment.captured","data":{"provider_txn_id":"dummy-card|CAPTURE|2|1|USD|15.00|payment-sync"}}`),
	)
	require.NoError(t, err)
	assert.False(t, duplicate)

	waitForWebhookState(t, db, event.ID, func(event models.WebhookEvent) bool {
		return event.ProcessedAt != nil
	})

	var refreshedTxn models.PaymentTransaction
	require.NoError(t, db.First(&refreshedTxn, txn.ID).Error)
	assert.Equal(t, models.PaymentTransactionStatusSucceeded, refreshedTxn.Status)

	var refreshedIntent models.PaymentIntent
	require.NoError(t, db.First(&refreshedIntent, intent.ID).Error)
	assert.Equal(t, models.PaymentIntentStatusCaptured, refreshedIntent.Status)
	assert.Equal(t, refreshedTxn.Amount, refreshedIntent.CapturedAmount)
	assert.True(t, refreshedIntent.CapturedAmount > 0)

	var refreshedOrder models.Order
	require.NoError(t, db.First(&refreshedOrder, order.ID).Error)
	assert.Equal(t, models.StatusPaid, refreshedOrder.Status)

	var history []models.OrderStatusHistory
	require.NoError(t, db.Where("order_id = ?", order.ID).Find(&history).Error)
	require.Len(t, history, 1)
	assert.Equal(t, "payment_captured", history[0].Reason)
	assert.Equal(t, "webhook", history[0].Source)
	assert.Equal(t, "provider:dummy-card", history[0].Actor)
}
