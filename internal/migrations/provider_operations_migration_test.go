package migrations

import (
	"fmt"
	"testing"
	"time"

	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type frozenProviderBackfillPaymentIntent struct {
	ID       uint `gorm:"primaryKey"`
	Provider string
}

func (frozenProviderBackfillPaymentIntent) TableName() string { return "payment_intents" }

type frozenProviderBackfillPaymentTransaction struct {
	ID                  uint `gorm:"primaryKey"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
	PaymentIntentID     uint
	Operation           string
	ProviderTxnID       string
	IdempotencyKey      string
	Status              string
	RawResponseRedacted string
}

func (frozenProviderBackfillPaymentTransaction) TableName() string {
	return "payment_transactions"
}

type frozenProviderBackfillShipment struct {
	ID                 uint `gorm:"primaryKey"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
	OrderID            uint
	Provider           string
	ProviderShipmentID string
	Status             string
	PurchasedAt        *time.Time
	FinalizedAt        *time.Time
}

func (frozenProviderBackfillShipment) TableName() string { return "shipments" }

type frozenProviderBackfillTaxLine struct {
	ID            uint `gorm:"primaryKey"`
	OrderID       uint
	SnapshotID    uint
	TaxProviderID string
	FinalizedAt   time.Time
}

func (frozenProviderBackfillTaxLine) TableName() string { return "order_tax_lines" }

type frozenProviderBackfillAudit struct {
	ID                      uint `gorm:"primaryKey"`
	CreatedAt               time.Time
	UpdatedAt               time.Time
	ProviderType            string
	ProviderID              string
	Environment             string
	Operation               string
	CorrelationID           string
	IdempotencyKey          string
	Status                  string
	RequestPayloadRedacted  string
	ResponsePayloadRedacted string
	ErrorMessage            string
}

func (frozenProviderBackfillAudit) TableName() string { return "provider_call_audits" }

func TestProviderOperationBackfillReplaysHistoricalMutationsIdempotently(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&frozenProviderBackfillPaymentIntent{},
		&frozenProviderBackfillPaymentTransaction{},
		&frozenProviderBackfillShipment{},
		&frozenProviderBackfillTaxLine{},
		&frozenProviderBackfillAudit{},
		&models.ProviderOperation{},
		&models.ProviderOperationAttempt{},
		&models.ProviderReconciliationCase{},
	))

	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(&frozenProviderBackfillPaymentIntent{ID: 11, Provider: "card-prod"}).Error)
	require.NoError(t, db.Create(&frozenProviderBackfillPaymentTransaction{
		ID: 21, CreatedAt: now, UpdatedAt: now.Add(time.Minute), PaymentIntentID: 11,
		Operation: "AUTHORIZE", IdempotencyKey: "checkout-21", Status: models.PaymentTransactionStatusPending,
	}).Error)
	purchasedAt := now.Add(2 * time.Minute)
	require.NoError(t, db.Create(&frozenProviderBackfillShipment{
		ID: 31, CreatedAt: now, UpdatedAt: purchasedAt, OrderID: 41, Provider: "ground",
		ProviderShipmentID: "shipment-provider-31", Status: models.ShipmentStatusLabelPurchased, PurchasedAt: &purchasedAt,
	}).Error)
	require.NoError(t, db.Create(&frozenProviderBackfillTaxLine{
		ID: 51, OrderID: 41, SnapshotID: 61, TaxProviderID: "tax-us", FinalizedAt: now.Add(3 * time.Minute),
	}).Error)
	require.NoError(t, db.Create(&frozenProviderBackfillAudit{
		ID: 71, CreatedAt: now, UpdatedAt: now.Add(time.Minute), ProviderType: models.ProviderTypePayment,
		ProviderID: "card-prod", Environment: models.ProviderEnvironmentProduction, Operation: "AUTHORIZE",
		IdempotencyKey: "checkout-21", Status: models.ProviderCallStatusFailed, ErrorMessage: "request timeout",
	}).Error)
	require.NoError(t, db.Create(&frozenProviderBackfillAudit{
		ID: 72, CreatedAt: now, UpdatedAt: now.Add(4 * time.Minute), ProviderType: models.ProviderTypeShipping,
		ProviderID: "ground", Environment: models.ProviderEnvironmentSandbox, Operation: "cancel_label",
		IdempotencyKey: "cancel-72", Status: models.ProviderCallStatusFailed, ErrorMessage: "deadline exceeded after dispatch",
	}).Error)

	require.NoError(t, backfillProviderOperations(db))
	require.NoError(t, providerOperationBackfillReady(db))
	require.NoError(t, backfillProviderOperations(db))

	var operations []models.ProviderOperation
	require.NoError(t, db.Order("operation_key").Find(&operations).Error)
	require.Len(t, operations, 4)

	byKey := make(map[string]models.ProviderOperation, len(operations))
	for _, operation := range operations {
		byKey[operation.OperationKey] = operation
	}
	payment := byKey["legacy:payment_transaction:21"]
	assert.Equal(t, models.ProviderEnvironmentProduction, payment.Environment)
	assert.Equal(t, models.ProviderOperationStatusReconciliationRequired, payment.Status)
	assert.Equal(t, models.ProviderOutcomeUnknown, payment.ProviderOutcome)
	assert.Nil(t, payment.CompletedAt)

	shipment := byKey["legacy:shipment:31"]
	assert.Equal(t, models.ProviderOperationStatusCompleted, shipment.Status)
	assert.Equal(t, "shipment-provider-31", shipment.ProviderReference)
	require.NotNil(t, shipment.CompletedAt)

	taxKey := fmt.Sprintf("legacy:tax_finalization:41:61:%s", historicalKeyPart("tax-us"))
	assert.Equal(t, models.ProviderOperationStatusCompleted, byKey[taxKey].Status)
	assert.Equal(t, models.ProviderOperationStatusReconciliationRequired, byKey["legacy:provider_call_audit:72"].Status)

	var attemptCount int64
	require.NoError(t, db.Model(&models.ProviderOperationAttempt{}).Count(&attemptCount).Error)
	assert.EqualValues(t, 4, attemptCount)
	var caseCount int64
	require.NoError(t, db.Model(&models.ProviderReconciliationCase{}).Where("status = ?", models.ProviderReconciliationCaseStatusOpen).Count(&caseCount).Error)
	assert.EqualValues(t, 2, caseCount)
}
