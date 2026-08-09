package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"ecommerce/internal/apicontract"
	checkoutservice "ecommerce/internal/services/checkout"
	inventoryservice "ecommerce/internal/services/inventory"
	paymentservice "ecommerce/internal/services/payments"
	providerops "ecommerce/internal/services/providerops"
	"ecommerce/models"

	"gorm.io/gorm"
)

const checkoutReservationTTL = 180 * 24 * time.Hour

func (e *CheckoutProviderEndpoints) authorizeCheckoutOrderPayment(
	ctx context.Context,
	r apicontract.AuthorizeCheckoutOrderPaymentRequestObject,
) (apicontract.AuthorizeCheckoutOrderPaymentResponseObject, error) {
	if r.Body == nil {
		return nil, problemError(400, "invalid_request", "Payment authorization body is required", nil)
	}
	session, ok := checkoutservice.SessionFromContext(ctx)
	if !ok {
		return nil, problemError(400, "checkout_session_required", "A checkout session is required", nil)
	}
	scope := fmt.Sprintf("checkout_order_payment_authorize:%d", r.Id)
	key := stringPtr(r.Params.IdempotencyKey)
	idempotency, err := e.checkout.BeginIdempotency(ctx, session.ID, scope, key, r.Body, correlationID(ctx))
	if err != nil {
		return nil, checkoutEndpointError(err)
	}
	if idempotency.Replay {
		var replay apicontract.ProcessPaymentResponse
		if err := json.Unmarshal([]byte(idempotency.Record.ResponseBody), &replay); err != nil {
			return nil, err
		}
		return apicontract.AuthorizeCheckoutOrderPayment200JSONResponse(replay), nil
	}
	client := ""
	if request, ok := ctx.(interface{ ClientIP() string }); ok {
		client = request.ClientIP()
	}
	if !allowCheckoutSubmission("authorize_payment", session.PublicToken, client, time.Now().UTC()) {
		_ = e.checkout.ReleaseIdempotency(ctx, idempotency.Record)
		return nil, problemError(429, "checkout_rate_limited", "Too many checkout attempts. Please wait and try again.", nil)
	}

	order, err := e.orders.PayableCheckoutOrder(ctx, uint(r.Id), session)
	if err != nil {
		_ = e.checkout.ReleaseIdempotency(ctx, idempotency.Record)
		return nil, checkoutEndpointError(err)
	}
	if order.UserID == nil && (order.GuestEmail == nil || strings.TrimSpace(*order.GuestEmail) == "") {
		_ = e.checkout.ReleaseIdempotency(ctx, idempotency.Record)
		return nil, problemError(400, "guest_email_required", "Guest email is required", nil)
	}
	snapshot, err := e.payments.GetCheckoutSnapshotForSession(ctx, session.ID, uint(r.Body.SnapshotId))
	if err != nil {
		_ = e.checkout.ReleaseIdempotency(ctx, idempotency.Record)
		return nil, checkoutEndpointError(err)
	}
	if err := paymentservice.ValidateSnapshotForOrder(&snapshot, &order, time.Now().UTC()); err != nil {
		_ = e.checkout.ReleaseIdempotency(ctx, idempotency.Record)
		return nil, checkoutEndpointError(err)
	}
	providerKey := strings.TrimSpace(key)
	if providerKey == "" {
		providerKey = fmt.Sprintf("authorize-order-%d-snapshot-%d", order.ID, snapshot.ID)
	}

	var intent models.PaymentIntent
	var transaction models.PaymentTransaction
	err = e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := paymentservice.BindSnapshotToOrder(tx, &snapshot, order.ID, time.Now().UTC()); err != nil {
			return err
		}
		if err := inventoryservice.ReserveOrderItems(tx, order, "checkout-payment:"+providerKey, time.Now().UTC().Add(checkoutReservationTTL)); err != nil {
			return err
		}
		var prepareErr error
		intent, transaction, prepareErr = paymentservice.PrepareAuthorizedPaymentIntent(tx, order.ID, snapshot, providerKey)
		return prepareErr
	})
	if err != nil {
		_ = e.checkout.ReleaseIdempotency(ctx, idempotency.Record)
		return nil, checkoutEndpointError(err)
	}
	providerRequest, err := paymentservice.PreparedAuthorizationRequest(intent, transaction, snapshot, correlationID(ctx))
	if err != nil {
		_ = e.checkout.ReleaseIdempotency(ctx, idempotency.Record)
		return nil, err
	}
	fingerprint, err := providerops.PaymentRequestFingerprint(providerRequest)
	if err != nil {
		_ = e.checkout.ReleaseIdempotency(ctx, idempotency.Record)
		return nil, err
	}
	operationKey := providerops.PaymentOperationKey(scope, providerKey)
	operation, executeErr := e.runtime.Executor.ExecutePaymentAuthorize(ctx, providerops.PaymentMutationInput{
		Prepare:             providerops.PrepareOperationInput{OperationKey: operationKey, ProviderType: models.ProviderTypePayment, ProviderID: intent.Provider, Environment: e.runtime.Environment, Operation: "authorize", IdempotencyKey: providerKey, RequestFingerprint: fingerprint, Request: providerRequest, CorrelationID: correlationID(ctx), EntityType: "payment_intent", EntityID: intent.ID},
		Registry:            e.runtime.PaymentProviders,
		Request:             providerRequest,
		DomainTransactionID: transaction.ID,
		Finalize: func(finalizeCtx context.Context, _ models.ProviderOperation, result paymentservice.ProviderOperationResult) error {
			return e.db.WithContext(finalizeCtx).Transaction(func(tx *gorm.DB) error {
				if _, _, err := paymentservice.FinalizePreparedAuthorization(tx, intent.ID, result); err != nil {
					return err
				}
				var locked models.Order
				if err := tx.Preload("Items").First(&locked, order.ID).Error; err != nil {
					return err
				}
				return paymentservice.ApplyAuthorizedCheckoutState(tx, &locked, snapshot, correlationID(finalizeCtx))
			})
		},
	})
	if executeErr != nil {
		if operation.Status == models.ProviderOperationStatusFinalizeRetry || errors.Is(executeErr, providerops.ErrPaymentEffectCompensated) {
			_ = e.checkout.ReleaseIdempotency(ctx, idempotency.Record)
			return nil, executeErr
		}
		if providerops.PaymentOperationRecoverable(operation) {
			_ = e.checkout.ReleaseIdempotency(ctx, idempotency.Record)
			return apicontract.AuthorizeCheckoutOrderPayment202JSONResponse(providerAccepted(operation, "Payment authorization pending reconciliation")), nil
		}
		_ = e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := paymentservice.MarkPreparedAuthorizationFailed(tx, intent.ID, executeErr.Error()); err != nil {
				return err
			}
			return inventoryservice.ReleaseReservationsForOrder(tx, order.ID, "payment-authorize-failed")
		})
		_ = e.checkout.ReleaseIdempotency(ctx, idempotency.Record)
		return nil, executeErr
	}
	if providerops.PaymentOperationRecoverable(operation) {
		_ = e.checkout.ReleaseIdempotency(ctx, idempotency.Record)
		return apicontract.AuthorizeCheckoutOrderPayment202JSONResponse(providerAccepted(operation, "Payment authorization pending reconciliation")), nil
	}
	owner := order.UserID
	order, err = e.orders.Get(ctx, order.ID, func() *uint {
		if owner != nil {
			return owner
		}
		zero := uint(0)
		return &zero
	}())
	if err != nil {
		return nil, checkoutEndpointError(err)
	}
	response := apicontract.ProcessPaymentResponse{Message: "Order submitted and pending confirmation", Order: orderContract(order, owner)}
	if err := e.checkout.CompleteIdempotency(ctx, idempotency.Record, 200, response); err != nil {
		return nil, err
	}
	return apicontract.AuthorizeCheckoutOrderPayment200JSONResponse(response), nil
}
