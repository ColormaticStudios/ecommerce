package payments

import (
	"errors"
	"fmt"
	"strings"

	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func PrepareCapturePaymentIntent(
	tx *gorm.DB,
	intent *models.PaymentIntent,
	amount *models.Money,
	idempotencyKey string,
	correlationID string,
) (models.PaymentTransaction, CaptureRequest, error) {
	if intent == nil {
		return models.PaymentTransaction{}, CaptureRequest{}, fmt.Errorf("payment intent is required")
	}
	if existing, ok, err := existingLifecycleTransaction(intent, models.PaymentTransactionOperationCapture, idempotencyKey, amount); err != nil {
		return models.PaymentTransaction{}, CaptureRequest{}, err
	} else if ok {
		return existing, CaptureRequest{
			OrderID: intent.OrderID, IntentID: intent.ID, Amount: existing.Amount, Currency: intent.Currency,
			Provider: intent.Provider, CorrelationID: correlationID,
			ProviderTxnIDRef: latestProviderTxnID(intent.Transactions, models.PaymentTransactionOperationAuthorize),
		}, nil
	}
	if intent.Status != models.PaymentIntentStatusAuthorized && intent.Status != models.PaymentIntentStatusPartiallyCaptured {
		return models.PaymentTransaction{}, CaptureRequest{}, ErrCaptureNotAllowed
	}
	remaining := intent.AuthorizedAmount - intent.CapturedAmount
	captureAmount, err := resolveLifecycleAmount(amount, remaining)
	if err != nil {
		return models.PaymentTransaction{}, CaptureRequest{}, err
	}
	request := CaptureRequest{
		OrderID:          intent.OrderID,
		IntentID:         intent.ID,
		Amount:           captureAmount,
		Currency:         intent.Currency,
		Provider:         intent.Provider,
		CorrelationID:    correlationID,
		ProviderTxnIDRef: latestProviderTxnID(intent.Transactions, models.PaymentTransactionOperationAuthorize),
	}
	txn, err := prepareLifecycleTransaction(tx, intent, models.PaymentTransactionOperationCapture, captureAmount, idempotencyKey)
	return txn, request, err
}

func PrepareVoidPaymentIntent(
	tx *gorm.DB,
	intent *models.PaymentIntent,
	idempotencyKey string,
	correlationID string,
) (models.PaymentTransaction, VoidRequest, error) {
	if intent == nil {
		return models.PaymentTransaction{}, VoidRequest{}, fmt.Errorf("payment intent is required")
	}
	if existing, ok, err := existingLifecycleTransaction(intent, models.PaymentTransactionOperationVoid, idempotencyKey, nil); err != nil {
		return models.PaymentTransaction{}, VoidRequest{}, err
	} else if ok {
		return existing, VoidRequest{
			OrderID: intent.OrderID, IntentID: intent.ID, Amount: existing.Amount, Currency: intent.Currency,
			Provider: intent.Provider, CorrelationID: correlationID,
			ProviderTxnIDRef: latestProviderTxnID(intent.Transactions, models.PaymentTransactionOperationAuthorize),
		}, nil
	}
	if intent.Status != models.PaymentIntentStatusAuthorized || intent.CapturedAmount > 0 {
		return models.PaymentTransaction{}, VoidRequest{}, ErrVoidNotAllowed
	}
	request := VoidRequest{
		OrderID:          intent.OrderID,
		IntentID:         intent.ID,
		Amount:           intent.AuthorizedAmount,
		Currency:         intent.Currency,
		Provider:         intent.Provider,
		CorrelationID:    correlationID,
		ProviderTxnIDRef: latestProviderTxnID(intent.Transactions, models.PaymentTransactionOperationAuthorize),
	}
	txn, err := prepareLifecycleTransaction(tx, intent, models.PaymentTransactionOperationVoid, request.Amount, idempotencyKey)
	return txn, request, err
}

func PrepareRefundPaymentIntent(
	tx *gorm.DB,
	intent *models.PaymentIntent,
	amount *models.Money,
	idempotencyKey string,
	correlationID string,
) (models.PaymentTransaction, RefundRequest, error) {
	if intent == nil {
		return models.PaymentTransaction{}, RefundRequest{}, fmt.Errorf("payment intent is required")
	}
	if existing, ok, err := existingLifecycleTransaction(intent, models.PaymentTransactionOperationRefund, idempotencyKey, amount); err != nil {
		return models.PaymentTransaction{}, RefundRequest{}, err
	} else if ok {
		return existing, RefundRequest{
			OrderID: intent.OrderID, IntentID: intent.ID, Amount: existing.Amount, Currency: intent.Currency,
			Provider: intent.Provider, CorrelationID: correlationID,
			ProviderTxnIDRef: latestProviderTxnID(intent.Transactions, models.PaymentTransactionOperationCapture),
		}, nil
	}
	if intent.CapturedAmount <= 0 {
		return models.PaymentTransaction{}, RefundRequest{}, ErrRefundNotAllowed
	}
	if intent.Status != models.PaymentIntentStatusCaptured &&
		intent.Status != models.PaymentIntentStatusPartiallyCaptured &&
		intent.Status != models.PaymentIntentStatusRefunded {
		return models.PaymentTransaction{}, RefundRequest{}, ErrRefundNotAllowed
	}
	remaining := intent.CapturedAmount - refundedAmount(intent.Transactions)
	refundAmount, err := resolveLifecycleAmount(amount, remaining)
	if err != nil {
		return models.PaymentTransaction{}, RefundRequest{}, err
	}
	request := RefundRequest{
		OrderID:          intent.OrderID,
		IntentID:         intent.ID,
		Amount:           refundAmount,
		Currency:         intent.Currency,
		Provider:         intent.Provider,
		CorrelationID:    correlationID,
		ProviderTxnIDRef: latestProviderTxnID(intent.Transactions, models.PaymentTransactionOperationCapture),
	}
	txn, err := prepareLifecycleTransaction(tx, intent, models.PaymentTransactionOperationRefund, refundAmount, idempotencyKey)
	return txn, request, err
}

func PrepareAuthorizationVoidCompensation(
	tx *gorm.DB,
	intentID uint,
	idempotencyKey string,
	providerTxnID string,
	correlationID string,
) (models.PaymentTransaction, VoidRequest, error) {
	intent, err := lockPaymentIntent(tx, intentID)
	if err != nil {
		return models.PaymentTransaction{}, VoidRequest{}, err
	}
	request := VoidRequest{
		OrderID:          intent.OrderID,
		IntentID:         intent.ID,
		Amount:           intent.AuthorizedAmount,
		Currency:         intent.Currency,
		Provider:         intent.Provider,
		CorrelationID:    correlationID,
		ProviderTxnIDRef: strings.TrimSpace(providerTxnID),
	}
	txn, err := prepareLifecycleTransaction(tx, &intent, models.PaymentTransactionOperationVoid, request.Amount, idempotencyKey)
	return txn, request, err
}

func PrepareCaptureRefundCompensation(
	tx *gorm.DB,
	intentID, captureTransactionID uint,
	idempotencyKey string,
	providerTxnID string,
	correlationID string,
) (models.PaymentTransaction, RefundRequest, error) {
	intent, err := lockPaymentIntent(tx, intentID)
	if err != nil {
		return models.PaymentTransaction{}, RefundRequest{}, err
	}
	captureTxn, ok := transactionByID(intent.Transactions, captureTransactionID)
	if !ok || captureTxn.Operation != models.PaymentTransactionOperationCapture {
		return models.PaymentTransaction{}, RefundRequest{}, fmt.Errorf("capture transaction not found for intent %d", intent.ID)
	}
	request := RefundRequest{
		OrderID:          intent.OrderID,
		IntentID:         intent.ID,
		Amount:           captureTxn.Amount,
		Currency:         intent.Currency,
		Provider:         intent.Provider,
		CorrelationID:    correlationID,
		ProviderTxnIDRef: strings.TrimSpace(providerTxnID),
	}
	txn, err := prepareLifecycleTransaction(tx, &intent, models.PaymentTransactionOperationRefund, request.Amount, idempotencyKey)
	return txn, request, err
}

func PreparedAuthorizationRequest(intent models.PaymentIntent, transaction models.PaymentTransaction, snapshot models.OrderCheckoutSnapshot, correlationID string) (AuthorizeRequest, error) {
	paymentData, err := unmarshalStringMap(snapshot.PaymentDataJSON)
	if err != nil {
		return AuthorizeRequest{}, err
	}
	return AuthorizeRequest{
		OrderID: intent.OrderID, SnapshotID: snapshot.ID, Amount: snapshot.Total, Currency: snapshot.Currency,
		Provider: snapshot.PaymentProviderID, CorrelationID: correlationID, PaymentMethodDisplay: snapshot.PaymentMethodDisplay,
		PaymentData: paymentData,
	}, nil
}

func FinalizePreparedAuthorization(tx *gorm.DB, intentID uint, providerResult ProviderOperationResult) (models.PaymentIntent, models.PaymentTransaction, error) {
	return finalizePreparedAuthorization(tx, intentID, providerResult)
}

func FinalizeCapturePaymentIntent(tx *gorm.DB, intentID, transactionID uint, providerResult ProviderOperationResult) (models.PaymentIntent, models.PaymentTransaction, error) {
	intent, txn, alreadyFinalized, err := lockLifecycleTransaction(tx, intentID, transactionID, models.PaymentTransactionOperationCapture)
	if err != nil || alreadyFinalized {
		return intent, txn, err
	}
	if intent.Status != models.PaymentIntentStatusAuthorized && intent.Status != models.PaymentIntentStatusPartiallyCaptured {
		return models.PaymentIntent{}, models.PaymentTransaction{}, ErrCaptureNotAllowed
	}
	if err := markLifecycleTransactionSucceeded(tx, &txn, providerResult); err != nil {
		return models.PaymentIntent{}, models.PaymentTransaction{}, err
	}
	intent.CapturedAmount += txn.Amount
	if intent.CapturedAmount >= intent.AuthorizedAmount {
		intent.CapturedAmount = intent.AuthorizedAmount
		intent.Status = models.PaymentIntentStatusCaptured
	} else {
		intent.Status = models.PaymentIntentStatusPartiallyCaptured
	}
	intent.Version++
	if err := tx.Save(&intent).Error; err != nil {
		return models.PaymentIntent{}, models.PaymentTransaction{}, err
	}
	replaceTransaction(&intent.Transactions, txn)
	return intent, txn, nil
}

func FinalizeVoidPaymentIntent(tx *gorm.DB, intentID, transactionID uint, providerResult ProviderOperationResult) (models.PaymentIntent, models.PaymentTransaction, error) {
	intent, txn, alreadyFinalized, err := lockLifecycleTransaction(tx, intentID, transactionID, models.PaymentTransactionOperationVoid)
	if err != nil || alreadyFinalized {
		return intent, txn, err
	}
	if intent.Status != models.PaymentIntentStatusAuthorized || intent.CapturedAmount > 0 {
		return models.PaymentIntent{}, models.PaymentTransaction{}, ErrVoidNotAllowed
	}
	if err := markLifecycleTransactionSucceeded(tx, &txn, providerResult); err != nil {
		return models.PaymentIntent{}, models.PaymentTransaction{}, err
	}
	intent.Status = models.PaymentIntentStatusVoided
	intent.Version++
	if err := tx.Save(&intent).Error; err != nil {
		return models.PaymentIntent{}, models.PaymentTransaction{}, err
	}
	replaceTransaction(&intent.Transactions, txn)
	return intent, txn, nil
}

func FinalizeRefundPaymentIntent(tx *gorm.DB, intentID, transactionID uint, providerResult ProviderOperationResult) (models.PaymentIntent, models.PaymentTransaction, error) {
	intent, txn, alreadyFinalized, err := lockLifecycleTransaction(tx, intentID, transactionID, models.PaymentTransactionOperationRefund)
	if err != nil || alreadyFinalized {
		return intent, txn, err
	}
	if intent.CapturedAmount <= 0 {
		return models.PaymentIntent{}, models.PaymentTransaction{}, ErrRefundNotAllowed
	}
	if intent.Status != models.PaymentIntentStatusCaptured && intent.Status != models.PaymentIntentStatusPartiallyCaptured && intent.Status != models.PaymentIntentStatusRefunded {
		return models.PaymentIntent{}, models.PaymentTransaction{}, ErrRefundNotAllowed
	}
	if err := markLifecycleTransactionSucceeded(tx, &txn, providerResult); err != nil {
		return models.PaymentIntent{}, models.PaymentTransaction{}, err
	}
	replaceTransaction(&intent.Transactions, txn)
	intent.Version++
	if refundedAmount(intent.Transactions) >= intent.CapturedAmount {
		intent.Status = models.PaymentIntentStatusRefunded
	}
	if err := tx.Save(&intent).Error; err != nil {
		return models.PaymentIntent{}, models.PaymentTransaction{}, err
	}
	return intent, txn, nil
}

func FinalizeAuthorizationVoidCompensation(
	tx *gorm.DB,
	intentID, voidTransactionID uint,
	authorizeResult, voidResult ProviderOperationResult,
) (models.PaymentIntent, models.PaymentTransaction, error) {
	if _, _, err := FinalizePreparedAuthorization(tx, intentID, authorizeResult); err != nil {
		return models.PaymentIntent{}, models.PaymentTransaction{}, err
	}
	return FinalizeVoidPaymentIntent(tx, intentID, voidTransactionID, voidResult)
}

func FinalizeCaptureRefundCompensation(
	tx *gorm.DB,
	intentID, captureTransactionID, refundTransactionID uint,
	captureResult, refundResult ProviderOperationResult,
) (models.PaymentIntent, models.PaymentTransaction, error) {
	if _, _, err := FinalizeCapturePaymentIntent(tx, intentID, captureTransactionID, captureResult); err != nil {
		return models.PaymentIntent{}, models.PaymentTransaction{}, err
	}
	return FinalizeRefundPaymentIntent(tx, intentID, refundTransactionID, refundResult)
}

func MarkPreparedAuthorizationFailed(tx *gorm.DB, intentID uint, failure string) error {
	return markPreparedAuthorizationFailed(tx, intentID, failure)
}

func MarkPreparedPaymentFailure(tx *gorm.DB, intentID, transactionID uint, failure string) error {
	intent, err := lockPaymentIntent(tx, intentID)
	if err != nil {
		return err
	}
	txn, ok := transactionByID(intent.Transactions, transactionID)
	if !ok {
		return fmt.Errorf("payment transaction not found for intent %d", intent.ID)
	}
	if txn.Status != models.PaymentTransactionStatusPending {
		return nil
	}
	return tx.Model(&models.PaymentTransaction{}).Where("id = ?", txn.ID).Updates(map[string]any{
		"status":                models.PaymentTransactionStatusFailed,
		"raw_response_redacted": strings.TrimSpace(failure),
	}).Error
}

func existingLifecycleTransaction(intent *models.PaymentIntent, operation, idempotencyKey string, requestedAmount *models.Money) (models.PaymentTransaction, bool, error) {
	key := strings.TrimSpace(idempotencyKey)
	for _, txn := range intent.Transactions {
		if txn.Operation != operation || txn.IdempotencyKey != key {
			continue
		}
		if requestedAmount != nil && txn.Amount != *requestedAmount {
			return models.PaymentTransaction{}, false, ErrAmountExceedsAvailable
		}
		return txn, true, nil
	}
	return models.PaymentTransaction{}, false, nil
}

func prepareLifecycleTransaction(tx *gorm.DB, intent *models.PaymentIntent, operation string, amount models.Money, idempotencyKey string) (models.PaymentTransaction, error) {
	key := strings.TrimSpace(idempotencyKey)
	for _, txn := range intent.Transactions {
		if txn.Operation == operation && txn.IdempotencyKey == key {
			if txn.Amount != amount {
				return models.PaymentTransaction{}, ErrAmountExceedsAvailable
			}
			return txn, nil
		}
	}
	txn := models.PaymentTransaction{
		PaymentIntentID: intent.ID,
		Operation:       operation,
		IdempotencyKey:  key,
		Amount:          amount,
		Status:          models.PaymentTransactionStatusPending,
	}
	if err := tx.Create(&txn).Error; err != nil {
		return models.PaymentTransaction{}, err
	}
	intent.Transactions = append(intent.Transactions, txn)
	return txn, nil
}

func lockPaymentIntent(tx *gorm.DB, intentID uint) (models.PaymentIntent, error) {
	var intent models.PaymentIntent
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Transactions", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at ASC, id ASC")
	}).First(&intent, intentID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.PaymentIntent{}, ErrPaymentIntentNotFound
	}
	return intent, err
}

func lockLifecycleTransaction(tx *gorm.DB, intentID, transactionID uint, operation string) (models.PaymentIntent, models.PaymentTransaction, bool, error) {
	intent, err := lockPaymentIntent(tx, intentID)
	if err != nil {
		return models.PaymentIntent{}, models.PaymentTransaction{}, false, err
	}
	txn, ok := transactionByID(intent.Transactions, transactionID)
	if !ok || txn.Operation != operation {
		return models.PaymentIntent{}, models.PaymentTransaction{}, false, fmt.Errorf("%s transaction not found for intent %d", strings.ToLower(operation), intent.ID)
	}
	if txn.Status == models.PaymentTransactionStatusSucceeded {
		return intent, txn, true, nil
	}
	if txn.Status != models.PaymentTransactionStatusPending {
		return models.PaymentIntent{}, models.PaymentTransaction{}, false, fmt.Errorf("%s transaction is not pending", strings.ToLower(operation))
	}
	return intent, txn, false, nil
}

func markLifecycleTransactionSucceeded(tx *gorm.DB, txn *models.PaymentTransaction, providerResult ProviderOperationResult) error {
	txn.ProviderTxnID = providerResult.ProviderTxnID
	txn.Status = models.PaymentTransactionStatusSucceeded
	txn.RawResponseRedacted = providerResult.RawResponseRedacted
	return tx.Model(&models.PaymentTransaction{}).Where("id = ? AND status = ?", txn.ID, models.PaymentTransactionStatusPending).Updates(map[string]any{
		"provider_txn_id":       txn.ProviderTxnID,
		"status":                txn.Status,
		"raw_response_redacted": txn.RawResponseRedacted,
	}).Error
}

func transactionByID(transactions []models.PaymentTransaction, transactionID uint) (models.PaymentTransaction, bool) {
	for _, txn := range transactions {
		if txn.ID == transactionID {
			return txn, true
		}
	}
	return models.PaymentTransaction{}, false
}
