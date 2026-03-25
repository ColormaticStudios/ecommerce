package models

import "time"

const (
	PaymentTransactionOperationAuthorize = "AUTHORIZE"
	PaymentTransactionOperationCapture   = "CAPTURE"
	PaymentTransactionOperationVoid      = "VOID"
	PaymentTransactionOperationRefund    = "REFUND"
)

const (
	PaymentTransactionStatusPending   = "PENDING"
	PaymentTransactionStatusSucceeded = "SUCCEEDED"
	PaymentTransactionStatusFailed    = "FAILED"
)

type PaymentTransaction struct {
	ID                  uint `gorm:"primaryKey"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
	PaymentIntentID     uint   `gorm:"not null;index:idx_payment_txn_intent_operation_key,unique"`
	Operation           string `gorm:"not null;index:idx_payment_txn_intent_operation_key,unique"`
	ProviderTxnID       string `gorm:"not null;index"`
	IdempotencyKey      string `gorm:"not null;index:idx_payment_txn_intent_operation_key,unique"`
	Amount              Money  `gorm:"type:numeric(12,2);not null"`
	Status              string `gorm:"not null;index"`
	RawResponseRedacted string `gorm:"type:text;not null;default:''"`
}
