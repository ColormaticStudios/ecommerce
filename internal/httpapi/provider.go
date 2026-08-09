package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/checkoutplugins"
	"ecommerce/internal/requestctx"
	orderservice "ecommerce/internal/services/orders"
	paymentservice "ecommerce/internal/services/payments"
	providerops "ecommerce/internal/services/providerops"
	shippingservice "ecommerce/internal/services/shipping"
	taxservice "ecommerce/internal/services/tax"
	webhookservice "ecommerce/internal/services/webhooks"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (e *CheckoutProviderEndpoints) ProviderBindingMiddleware() apicontract.MiddlewareFunc {
	return func(ctx *gin.Context) {
		if ctx.Request == nil || ctx.Request.Method != http.MethodPost || ctx.Request.ContentLength > 0 {
			return
		}
		path := ctx.Request.URL.Path
		if !strings.Contains(path, "/admin/orders/") || !strings.Contains(path, "/payments/") ||
			(!strings.HasSuffix(path, "/capture") && !strings.HasSuffix(path, "/refund")) {
			return
		}
		ctx.Request.Body = io.NopCloser(strings.NewReader("{}"))
		ctx.Request.ContentLength = 2
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
}

func (e *CheckoutProviderEndpoints) pluginCatalog(ctx context.Context, admin bool) (apicontract.CheckoutPluginCatalog, error) {
	payments, shipping, tax, err := e.catalog.List(ctx, admin)
	if err != nil {
		return apicontract.CheckoutPluginCatalog{}, err
	}
	return apicontract.CheckoutPluginCatalog{Payment: pluginDefinitions(payments), Shipping: pluginDefinitions(shipping), Tax: pluginDefinitions(tax)}, nil
}
func pluginDefinitions(values []checkoutplugins.Definition) []apicontract.CheckoutPlugin {
	out := make([]apicontract.CheckoutPlugin, 0, len(values))
	for _, v := range values {
		fields := make([]apicontract.CheckoutPluginField, 0, len(v.Fields))
		for _, f := range v.Fields {
			field := apicontract.CheckoutPluginField{Key: f.Key, Label: f.Label, Type: apicontract.CheckoutPluginFieldType(f.Type), Required: f.Required}
			if f.Placeholder != "" {
				field.Placeholder = &f.Placeholder
			}
			if f.HelpText != "" {
				field.HelpText = &f.HelpText
			}
			if len(f.Options) > 0 {
				opts := make([]apicontract.CheckoutPluginFieldOption, 0, len(f.Options))
				for _, o := range f.Options {
					opts = append(opts, apicontract.CheckoutPluginFieldOption{Value: o.Value, Label: o.Label})
				}
				field.Options = &opts
			}
			fields = append(fields, field)
		}
		out = append(out, apicontract.CheckoutPlugin{Id: v.ID, Type: apicontract.CheckoutPluginType(v.Type), Name: v.Name, Description: v.Description, Status: v.Status, Enabled: v.Enabled, Fields: fields, States: pluginStates(v.States)})
	}
	return out
}
func pluginStates(values []checkoutplugins.State) []apicontract.CheckoutPluginState {
	out := make([]apicontract.CheckoutPluginState, 0, len(values))
	for _, v := range values {
		out = append(out, apicontract.CheckoutPluginState{Code: v.Code, Severity: apicontract.CheckoutPluginStateSeverity(v.Severity), Message: v.Message})
	}
	return out
}

func (e *CheckoutProviderEndpoints) GetAdminOrderPayments(ctx context.Context, r apicontract.GetAdminOrderPaymentsRequestObject) (apicontract.GetAdminOrderPaymentsResponseObject, error) {
	if r.Id < 1 {
		return nil, errors.New("order id must be positive")
	}
	if _, err := e.orders.Get(ctx, uint(r.Id), nil); err != nil {
		return nil, err
	}
	intents, err := e.payments.GetOrderPaymentLedger(ctx, uint(r.Id))
	if err != nil {
		return nil, err
	}
	return apicontract.GetAdminOrderPayments200JSONResponse{OrderId: r.Id, Intents: paymentIntentsContract(intents)}, nil
}
func paymentIntentsContract(values []models.PaymentIntent) []apicontract.PaymentIntentRecord {
	out := make([]apicontract.PaymentIntentRecord, 0, len(values))
	for _, v := range values {
		out = append(out, paymentIntentContract(v))
	}
	return out
}
func paymentIntentContract(v models.PaymentIntent) apicontract.PaymentIntentRecord {
	return apicontract.PaymentIntentRecord{Id: int(v.ID), OrderId: int(v.OrderID), SnapshotId: int(v.SnapshotID), Provider: v.Provider, Status: apicontract.PaymentIntentRecordStatus(v.Status), AuthorizedAmount: v.AuthorizedAmount.Float64(), CapturedAmount: v.CapturedAmount.Float64(), RefundableAmount: (v.CapturedAmount - paymentservice.RefundedAmount(v)).Float64(), Currency: v.Currency, Version: v.Version, Transactions: paymentTransactionsContract(v.Transactions), CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}
func paymentTransactionsContract(values []models.PaymentTransaction) []apicontract.PaymentTransactionRecord {
	out := make([]apicontract.PaymentTransactionRecord, 0, len(values))
	for _, v := range values {
		out = append(out, apicontract.PaymentTransactionRecord{Id: int(v.ID), Operation: apicontract.PaymentTransactionRecordOperation(v.Operation), ProviderTxnId: v.ProviderTxnID, IdempotencyKey: v.IdempotencyKey, Amount: v.Amount.Float64(), Status: apicontract.PaymentTransactionRecordStatus(v.Status), RawResponseRedacted: v.RawResponseRedacted, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt})
	}
	return out
}

func (e *CheckoutProviderEndpoints) QuoteCheckoutOrderShippingRates(ctx context.Context, r apicontract.QuoteCheckoutOrderShippingRatesRequestObject) (apicontract.QuoteCheckoutOrderShippingRatesResponseObject, error) {
	id, err := principalAccountID(ctx)
	if err != nil {
		return nil, err
	}
	if r.Body == nil {
		return nil, errors.New("shipping quote body is required")
	}
	order, err := e.orders.Get(ctx, uint(r.Id), &id)
	if err != nil {
		return nil, err
	}
	snapshot, err := e.payments.GetCheckoutSnapshotForSession(ctx, order.CheckoutSessionID, uint(r.Body.SnapshotId))
	if err != nil {
		return nil, err
	}
	if err = paymentservice.ValidateSnapshotForOrder(&snapshot, &order, time.Now().UTC()); err != nil {
		if errors.Is(err, paymentservice.ErrSnapshotAlreadyBound) || errors.Is(err, paymentservice.ErrSnapshotOrderMismatch) {
			problem := providerAdminProblem(ctx, e.renderer, http.StatusConflict, "checkout_snapshot_conflict", err.Error(), err)
			return apicontract.QuoteCheckoutOrderShippingRates409ApplicationProblemPlusJSONResponse{ConflictProblemApplicationProblemPlusJSONResponse: apicontract.ConflictProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
		return nil, err
	}
	plan, err := e.shipping.PrepareRateQuote(ctx, order, snapshot)
	if err != nil {
		return nil, err
	}
	rates := plan.StoredRates
	if len(rates) == 0 {
		quoted, err := shippingservice.QuotePreparedRates(ctx, e.runtime.ShippingProviders, plan)
		if err != nil {
			return nil, err
		}
		rates, err = e.shipping.PersistQuotedRates(ctx, plan, quoted)
		if err != nil {
			return nil, err
		}
	}
	return apicontract.QuoteCheckoutOrderShippingRates200JSONResponse{OrderId: r.Id, SnapshotId: int(snapshot.ID), Provider: snapshot.ShippingProviderID, Rates: shipmentRatesContract(rates)}, nil
}
func (e *CheckoutProviderEndpoints) GetCheckoutOrderShippingTracking(ctx context.Context, r apicontract.GetCheckoutOrderShippingTrackingRequestObject) (apicontract.GetCheckoutOrderShippingTrackingResponseObject, error) {
	id, err := principalAccountID(ctx)
	if err != nil {
		return nil, err
	}
	order, err := e.orders.Get(ctx, uint(r.Id), &id)
	if errors.Is(err, orderservice.ErrOrderNotFound) {
		problem := providerAdminProblem(ctx, e.renderer, http.StatusNotFound, "order_not_found", "Order not found", err)
		return apicontract.GetCheckoutOrderShippingTracking404ApplicationProblemPlusJSONResponse{NotFoundProblemApplicationProblemPlusJSONResponse: apicontract.NotFoundProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	if err != nil {
		return nil, err
	}
	shipments, err := e.shipping.GetOrderShipments(ctx, order.ID)
	if err != nil {
		return nil, err
	}
	out := make([]apicontract.Shipment, 0, len(shipments))
	for _, v := range shipments {
		out = append(out, shipmentContract(v))
	}
	return apicontract.GetCheckoutOrderShippingTracking200JSONResponse{OrderId: r.Id, Shipments: out}, nil
}
func shipmentRatesContract(values []models.ShipmentRate) []apicontract.ShipmentRate {
	out := make([]apicontract.ShipmentRate, 0, len(values))
	for _, v := range values {
		var shipmentID *int
		if v.ShipmentID != nil {
			x := int(*v.ShipmentID)
			shipmentID = &x
		}
		out = append(out, apicontract.ShipmentRate{Id: int(v.ID), Provider: v.Provider, ProviderRateId: v.ProviderRateID, ServiceCode: v.ServiceCode, ServiceName: v.ServiceName, Amount: v.Amount.Float64(), Currency: v.Currency, Selected: v.Selected, ShipmentId: shipmentID, ExpiresAt: v.ExpiresAt})
	}
	return out
}
func shipmentContract(v models.Shipment) apicontract.Shipment {
	rates := shipmentRatesContract(v.Rates)
	packages := make([]apicontract.ShipmentPackage, 0, len(v.Packages))
	for _, p := range v.Packages {
		packages = append(packages, apicontract.ShipmentPackage{Id: int(p.ID), Reference: p.Reference, WeightGrams: p.WeightGrams, LengthCm: p.LengthCM, WidthCm: p.WidthCM, HeightCm: p.HeightCM})
	}
	events := make([]apicontract.TrackingEvent, 0, len(v.TrackingEvents))
	for _, x := range v.TrackingEvents {
		events = append(events, apicontract.TrackingEvent{Id: int(x.ID), Provider: x.Provider, ProviderEventId: x.ProviderEventID, Status: apicontract.TrackingEventStatus(x.Status), TrackingNumber: x.TrackingNumber, Location: x.Location, Description: x.Description, OccurredAt: x.OccurredAt})
	}
	return apicontract.Shipment{Id: int(v.ID), OrderId: int(v.OrderID), SnapshotId: int(v.SnapshotID), Provider: v.Provider, ShipmentRateId: int(v.ShipmentRateID), ProviderShipmentId: v.ProviderShipmentID, Status: apicontract.ShipmentStatus(v.Status), Currency: v.Currency, ServiceCode: v.ServiceCode, ServiceName: v.ServiceName, Amount: v.Amount.Float64(), ShippingAddressPretty: v.ShippingAddressPretty, TrackingNumber: v.TrackingNumber, TrackingUrl: v.TrackingURL, LabelUrl: v.LabelURL, PurchasedAt: v.PurchasedAt, FinalizedAt: v.FinalizedAt, DeliveredAt: v.DeliveredAt, Rates: rates, Packages: packages, TrackingEvents: events}
}

func (e *CheckoutProviderEndpoints) CreateAdminOrderShippingLabel(ctx context.Context, r apicontract.CreateAdminOrderShippingLabelRequestObject) (apicontract.CreateAdminOrderShippingLabelResponseObject, error) {
	if r.Body == nil {
		return nil, errors.New("shipping label body is required")
	}
	pkg := shippingservice.PackageInput{}
	if r.Body.Package != nil {
		p := r.Body.Package
		if p.Reference != nil {
			pkg.Reference = *p.Reference
		}
		if p.WeightGrams != nil {
			pkg.WeightGrams = *p.WeightGrams
		}
		if p.LengthCm != nil {
			pkg.LengthCM = *p.LengthCm
		}
		if p.WidthCm != nil {
			pkg.WidthCM = *p.WidthCm
		}
		if p.HeightCm != nil {
			pkg.HeightCM = *p.HeightCm
		}
	}
	prepared, err := e.shipping.PrepareLabelPurchase(ctx, uint(r.Id), uint(r.Body.RateId), pkg, r.Params.IdempotencyKey)
	if errors.Is(err, shippingservice.ErrShipmentServiceImmutable) {
		problem := providerAdminProblem(ctx, e.renderer, http.StatusConflict, "shipment_service_immutable", err.Error(), err)
		return apicontract.CreateAdminOrderShippingLabel409ApplicationProblemPlusJSONResponse{ConflictProblemApplicationProblemPlusJSONResponse: apicontract.ConflictProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	if err != nil {
		return nil, err
	}
	shipment := prepared.Shipment
	if !prepared.AlreadyDone {
		request := shippingservice.BuyLabelRequest{OrderID: uint(r.Id), SnapshotID: prepared.Snapshot.ID, Provider: prepared.Rate.Provider, Rate: prepared.Rate, ShippingAddressPretty: prepared.Snapshot.ShippingAddressPretty, Package: pkg, IdempotencyKey: prepared.EffectiveKey, OperationKey: "shipping-label:" + prepared.EffectiveKey, CorrelationID: correlationID(ctx)}
		operation, err := e.runtime.Executor.ExecuteDurableShippingLabel(ctx, providerops.DurableShippingLabelInput{ShippingLabelOperationInput: providerops.ShippingLabelOperationInput{Prepare: providerops.PrepareOperationInput{OperationKey: request.OperationKey, ProviderType: models.ProviderTypeShipping, ProviderID: request.Provider, Environment: e.runtime.Environment, Operation: "buy_label", IdempotencyKey: request.IdempotencyKey, Request: request, CorrelationID: request.CorrelationID, EntityType: "shipment", EntityID: prepared.Shipment.ID}, Registry: e.runtime.ShippingProviders, Request: request, Finalize: func(finalizeCtx context.Context, _ models.ProviderOperation, result shippingservice.ProviderShipment) error {
			var finalErr error
			shipment, finalErr = e.shipping.FinalizeLabelPurchase(finalizeCtx, prepared.Shipment.ID, result, time.Now().UTC())
			return finalErr
		}}})
		if err != nil {
			problem := providerAdminProblem(ctx, e.renderer, http.StatusInternalServerError, "shipping_label_creation_failed", "Failed to create shipping label", err)
			return apicontract.CreateAdminOrderShippingLabel500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
		if providerops.PaymentOperationRecoverable(operation) {
			return nil, errors.New("shipping label operation accepted but response contract has no 202 variant")
		}
	}
	if len(shipment.Rates) == 0 {
		shipment, err = e.shipping.GetShipment(ctx, shipment.ID)
		if err != nil {
			return nil, err
		}
	}
	return apicontract.CreateAdminOrderShippingLabel200JSONResponse{Message: "Shipping label purchased", Shipment: shipmentContract(shipment)}, nil
}

func (e *CheckoutProviderEndpoints) FinalizeCheckoutOrderTax(ctx context.Context, r apicontract.FinalizeCheckoutOrderTaxRequestObject) (apicontract.FinalizeCheckoutOrderTaxResponseObject, error) {
	id, err := principalAccountID(ctx)
	if err != nil {
		return nil, err
	}
	if r.Body == nil {
		return nil, errors.New("tax finalization body is required")
	}
	order, err := e.orders.Get(ctx, uint(r.Id), &id)
	if err != nil {
		return nil, err
	}
	snapshot, err := e.payments.GetCheckoutSnapshotForSession(ctx, order.CheckoutSessionID, uint(r.Body.SnapshotId))
	if err != nil {
		return nil, err
	}
	if err = paymentservice.ValidateSnapshotForOrder(&snapshot, &order, time.Now().UTC()); err != nil {
		if errors.Is(err, paymentservice.ErrSnapshotAlreadyBound) || errors.Is(err, paymentservice.ErrSnapshotOrderMismatch) {
			problem := providerAdminProblem(ctx, e.renderer, http.StatusConflict, "checkout_snapshot_conflict", err.Error(), err)
			return apicontract.FinalizeCheckoutOrderTax409ApplicationProblemPlusJSONResponse{ConflictProblemApplicationProblemPlusJSONResponse: apicontract.ConflictProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
		return nil, err
	}
	key := "tax-finalize:" + fmt.Sprint(snapshot.ID)
	if r.Params.IdempotencyKey != nil && strings.TrimSpace(*r.Params.IdempotencyKey) != "" {
		key = *r.Params.IdempotencyKey
	}
	input := taxservice.FinalizeInput{Order: order, Snapshot: snapshot, InclusivePricing: r.Body.InclusivePricing, IdempotencyKey: key, OperationKey: key, CorrelationID: correlationID(ctx)}
	plan, err := e.tax.PrepareFinalization(ctx, input)
	if err != nil {
		return nil, err
	}
	result := plan.StoredResult
	if !plan.AlreadyFinalized {
		operation, err := e.runtime.Executor.ExecuteDurableTaxFinalization(ctx, providerops.DurableTaxFinalizationInput{TaxFinalizationOperationInput: providerops.TaxFinalizationOperationInput{Prepare: providerops.PrepareOperationInput{OperationKey: key, ProviderType: models.ProviderTypeTax, ProviderID: snapshot.TaxProviderID, Environment: e.runtime.Environment, Operation: "finalize_tax", IdempotencyKey: key, Request: plan.Request, CorrelationID: input.CorrelationID, EntityType: "order", EntityID: order.ID}, Registry: e.runtime.TaxProviders, Request: plan.Request, Finalize: func(finalizeCtx context.Context, _ models.ProviderOperation, r taxservice.TaxFinalized) error {
			var finalErr error
			result, finalErr = e.tax.PersistFinalization(finalizeCtx, input, r, time.Now().UTC())
			return finalErr
		}}})
		if err != nil {
			return nil, err
		}
		if result.Provider == "" {
			if err = json.Unmarshal([]byte(operation.ResultJSON), &result); err != nil {
				return nil, err
			}
			result, err = e.tax.PersistFinalization(ctx, input, result, time.Now().UTC())
			if err != nil {
				return nil, err
			}
		}
	}
	return apicontract.FinalizeCheckoutOrderTax200JSONResponse(taxFinalizeContract(order.ID, snapshot, result)), nil
}
func taxFinalizeContract(orderID uint, snapshot models.OrderCheckoutSnapshot, result taxservice.TaxFinalized) apicontract.CheckoutOrderTaxFinalizeResponse {
	lines := make([]apicontract.TaxLine, 0, len(result.Lines))
	for _, v := range result.Lines {
		var sid, pid *int
		if v.SnapshotItemID != nil {
			x := int(*v.SnapshotItemID)
			sid = &x
		}
		if v.ProductVariantID != nil {
			x := int(*v.ProductVariantID)
			pid = &x
		}
		lines = append(lines, apicontract.TaxLine{SnapshotItemId: sid, LineType: apicontract.TaxLineLineType(v.LineType), ProductVariantId: pid, Quantity: v.Quantity, Jurisdiction: v.Jurisdiction, TaxCode: v.TaxCode, TaxName: v.TaxName, TaxableAmount: v.TaxableAmount.Float64(), TaxAmount: v.TaxAmount.Float64(), TaxRateBasisPoints: v.TaxRateBasisPoints, Inclusive: v.Inclusive})
	}
	return apicontract.CheckoutOrderTaxFinalizeResponse{Message: "Taxes finalized", OrderId: int(orderID), SnapshotId: int(snapshot.ID), Provider: result.Provider, Currency: result.Currency, InclusivePricing: result.InclusivePricing, TotalTax: result.TotalTax.Float64(), Lines: lines}
}

func (e *CheckoutProviderEndpoints) ListAdminWebhookEvents(ctx context.Context, r apicontract.ListAdminWebhookEventsRequestObject) (apicontract.ListAdminWebhookEventsResponseObject, error) {
	page, limit := intPtr(r.Params.Page), intPtr(r.Params.Limit)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	events, total, err := e.webhooks.ListEvents(ctx, stringPtr(r.Params.Provider), enumString(r.Params.Status), page, limit)
	if err != nil {
		return nil, err
	}
	data := make([]apicontract.WebhookEventRecord, 0, len(events))
	for _, v := range events {
		data = append(data, webhookContract(v, e.webhooks.MaxAttempts))
	}
	pages := int(total) / limit
	if int(total)%limit != 0 {
		pages++
	}
	return apicontract.ListAdminWebhookEvents200JSONResponse{Data: data, Pagination: apicontract.Pagination{Page: page, Limit: limit, Total: int(total), TotalPages: pages}}, nil
}
func (e *CheckoutProviderEndpoints) ReceiveWebhookEvent(ctx context.Context, r apicontract.ReceiveWebhookEventRequestObject) (apicontract.ReceiveWebhookEventResponseObject, error) {
	if strings.TrimSpace(r.Provider) == "" || r.Body == nil {
		return nil, errors.New("provider and webhook body are required")
	}
	metadata, _ := requestctx.MetadataFrom(ctx)
	if len(metadata.RawBody) == 0 {
		return nil, errors.New("exact webhook request body is unavailable")
	}
	event, duplicate, err := e.webhooks.ReceiveWebhook(ctx, r.Provider, metadata.Headers, metadata.RawBody)
	if errors.Is(err, paymentservice.ErrInvalidWebhookSignature) || errors.Is(err, shippingservice.ErrInvalidShippingWebhookSignature) {
		problem := providerAdminProblem(ctx, e.renderer, http.StatusUnauthorized, "invalid_webhook_signature", "Invalid webhook signature", err)
		return apicontract.ReceiveWebhookEvent401ApplicationProblemPlusJSONResponse{AuthenticationRequiredProblemApplicationProblemPlusJSONResponse: apicontract.AuthenticationRequiredProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	if err != nil {
		return nil, err
	}
	return apicontract.ReceiveWebhookEvent200JSONResponse{Message: "Webhook accepted", EventId: int(event.ID), ProviderEventId: event.ProviderEventID, Duplicate: duplicate}, nil
}
func webhookContract(v models.WebhookEvent, maxAttempts int) apicontract.WebhookEventRecord {
	return apicontract.WebhookEventRecord{Id: int(v.ID), Provider: v.Provider, ProviderEventId: v.ProviderEventID, EventType: v.EventType, SignatureValid: v.SignatureValid, Payload: v.Payload, ReceivedAt: v.ReceivedAt, ProcessedAt: v.ProcessedAt, AttemptCount: v.AttemptCount, LastError: v.LastError, Status: apicontract.WebhookEventRecordStatus(webhookservice.EventStatus(&v, maxAttempts))}
}

func correlationID(ctx context.Context) string {
	if m, ok := requestctx.MetadataFrom(ctx); ok {
		return m.CorrelationID
	}
	return ""
}

func (e *CheckoutProviderEndpoints) ExportAdminTaxReport(ctx context.Context, r apicontract.ExportAdminTaxReportRequestObject) (apicontract.ExportAdminTaxReportResponseObject, error) {
	format := enumString(r.Params.Format)
	if format == "" {
		format = "csv"
	}
	_, body, err := taxservice.ExportOrderTaxes(ctx, e.db.WithContext(ctx), e.runtime.TaxProviders, taxservice.ExportInput{Provider: stringPtr(r.Params.Provider), Start: r.Params.StartDate, End: r.Params.EndDate, Format: format})
	if err != nil {
		return nil, err
	}
	contents, err := io.ReadAll(body)
	closeErr := body.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return apicontract.ExportAdminTaxReport200TextcsvResponse{Body: strings.NewReader(string(contents)), ContentLength: int64(len(contents))}, nil
}
func (e *CheckoutProviderEndpoints) AuthorizeCheckoutOrderPayment(ctx context.Context, r apicontract.AuthorizeCheckoutOrderPaymentRequestObject) (apicontract.AuthorizeCheckoutOrderPaymentResponseObject, error) {
	userID, err := principalAccountID(ctx)
	if err != nil {
		return nil, err
	}
	if r.Body == nil {
		return nil, errors.New("payment authorization body is required")
	}
	order, err := e.orders.Get(ctx, uint(r.Id), &userID)
	if err != nil {
		return nil, err
	}
	snapshot, err := e.payments.GetCheckoutSnapshotForSession(ctx, order.CheckoutSessionID, uint(r.Body.SnapshotId))
	if err != nil {
		return nil, err
	}
	if err := paymentservice.ValidateSnapshotForOrder(&snapshot, &order, time.Now().UTC()); err != nil {
		switch {
		case errors.Is(err, paymentservice.ErrSnapshotExpired):
			problem := providerAdminProblem(ctx, e.renderer, http.StatusBadRequest, "checkout_snapshot_expired", err.Error(), err)
			return apicontract.AuthorizeCheckoutOrderPayment400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
		case errors.Is(err, paymentservice.ErrSnapshotAlreadyBound), errors.Is(err, paymentservice.ErrSnapshotOrderMismatch):
			problem := providerAdminProblem(ctx, e.renderer, http.StatusConflict, "checkout_snapshot_conflict", err.Error(), err)
			return apicontract.AuthorizeCheckoutOrderPayment409ApplicationProblemPlusJSONResponse{ConflictProblemApplicationProblemPlusJSONResponse: apicontract.ConflictProblemApplicationProblemPlusJSONResponse(problem)}, nil
		default:
			return nil, err
		}
	}
	idempotencyKey := fmt.Sprintf("authorize-order-%d-snapshot-%d", order.ID, snapshot.ID)
	if r.Params.IdempotencyKey != nil && strings.TrimSpace(*r.Params.IdempotencyKey) != "" {
		idempotencyKey = strings.TrimSpace(*r.Params.IdempotencyKey)
	}
	operationKey := providerops.PaymentOperationKey(fmt.Sprintf("checkout_authorize:%d:%d", order.ID, snapshot.ID), idempotencyKey)
	var intent models.PaymentIntent
	var transaction models.PaymentTransaction
	err = e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var prepareErr error
		intent, transaction, prepareErr = paymentservice.PrepareAuthorizedPaymentIntent(tx, order.ID, snapshot, idempotencyKey)
		if prepareErr != nil {
			return prepareErr
		}
		return paymentservice.BindSnapshotToOrder(tx, &snapshot, order.ID, time.Now().UTC())
	})
	if err != nil {
		return nil, err
	}
	providerRequest, err := paymentservice.PreparedAuthorizationRequest(intent, transaction, snapshot, correlationID(ctx))
	if err != nil {
		return nil, err
	}
	fingerprint, err := providerops.PaymentRequestFingerprint(providerRequest)
	if err != nil {
		return nil, err
	}
	operation, executeErr := e.runtime.Executor.ExecutePaymentAuthorize(ctx, providerops.PaymentMutationInput{
		Prepare:             providerops.PrepareOperationInput{OperationKey: operationKey, ProviderType: models.ProviderTypePayment, ProviderID: intent.Provider, Environment: e.runtime.Environment, Operation: "authorize", IdempotencyKey: idempotencyKey, RequestFingerprint: fingerprint, Request: providerRequest, CorrelationID: correlationID(ctx), EntityType: "payment_intent", EntityID: intent.ID},
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
			return nil, executeErr
		}
		if providerops.PaymentOperationRecoverable(operation) {
			return apicontract.AuthorizeCheckoutOrderPayment202JSONResponse(providerAccepted(operation, "Payment authorization pending reconciliation")), nil
		}
		_ = e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			return paymentservice.MarkPreparedAuthorizationFailed(tx, intent.ID, executeErr.Error())
		})
		return nil, executeErr
	}
	if providerops.PaymentOperationRecoverable(operation) {
		return apicontract.AuthorizeCheckoutOrderPayment202JSONResponse(providerAccepted(operation, "Payment authorization pending reconciliation")), nil
	}
	order, err = e.orders.Get(ctx, order.ID, &userID)
	if err != nil {
		return nil, err
	}
	return apicontract.AuthorizeCheckoutOrderPayment200JSONResponse{Message: "Payment authorized", Order: orderContract(order, &userID)}, nil
}

type adminPaymentResult struct {
	order       models.Order
	intent      models.PaymentIntent
	transaction models.PaymentTransaction
	operation   models.ProviderOperation
	accepted    bool
}

func (e *CheckoutProviderEndpoints) runAdminPayment(ctx context.Context, orderID, intentID uint, operation, idempotencyKey string, amount *float64) (adminPaymentResult, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return adminPaymentResult{}, errors.New("idempotency key is required")
	}
	result := adminPaymentResult{}
	var providerRequest any
	err := e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&result.order, orderID).Error; err != nil {
			return err
		}
		intent, err := paymentservice.GetPaymentIntentForUpdate(tx, orderID, intentID)
		if err != nil {
			return err
		}
		result.intent = intent
		var requested *models.Money
		if amount != nil {
			value := models.MoneyFromFloat(*amount)
			requested = &value
		}
		switch operation {
		case "capture":
			var request paymentservice.CaptureRequest
			result.transaction, request, err = paymentservice.PrepareCapturePaymentIntent(tx, &result.intent, requested, idempotencyKey, correlationID(ctx))
			providerRequest = request
		case "void":
			var request paymentservice.VoidRequest
			result.transaction, request, err = paymentservice.PrepareVoidPaymentIntent(tx, &result.intent, idempotencyKey, correlationID(ctx))
			providerRequest = request
		case "refund":
			var request paymentservice.RefundRequest
			result.transaction, request, err = paymentservice.PrepareRefundPaymentIntent(tx, &result.intent, requested, idempotencyKey, correlationID(ctx))
			providerRequest = request
		default:
			err = errors.New("unsupported payment operation")
		}
		return err
	})
	if err != nil {
		return result, err
	}
	operationKey := providerops.PaymentOperationKey(fmt.Sprintf("admin_%s:%d:%d", operation, orderID, intentID), idempotencyKey)
	fingerprint, err := providerops.PaymentRequestFingerprint(providerRequest)
	if err != nil {
		return result, err
	}
	prepare := providerops.PrepareOperationInput{OperationKey: operationKey, ProviderType: models.ProviderTypePayment, ProviderID: result.intent.Provider, Environment: e.runtime.Environment, Operation: operation, IdempotencyKey: idempotencyKey, RequestFingerprint: fingerprint, Request: providerRequest, CorrelationID: correlationID(ctx), EntityType: "payment_intent", EntityID: intentID}
	finalize := func(finalizeCtx context.Context, _ models.ProviderOperation, providerResult paymentservice.ProviderOperationResult) error {
		return e.db.WithContext(finalizeCtx).Transaction(func(tx *gorm.DB) error {
			var err error
			targetStatus, reason := "", ""
			switch operation {
			case "capture":
				result.intent, result.transaction, err = paymentservice.FinalizeCapturePaymentIntent(tx, intentID, result.transaction.ID, providerResult)
				targetStatus, reason = models.StatusPaid, "payment_captured"
			case "void":
				result.intent, result.transaction, err = paymentservice.FinalizeVoidPaymentIntent(tx, intentID, result.transaction.ID, providerResult)
				targetStatus, reason = models.StatusCancelled, "payment_voided"
			case "refund":
				result.intent, result.transaction, err = paymentservice.FinalizeRefundPaymentIntent(tx, intentID, result.transaction.ID, providerResult)
				if result.intent.Status == models.PaymentIntentStatusRefunded {
					targetStatus, reason = models.StatusRefunded, "payment_refunded"
				}
			}
			if err != nil || targetStatus == "" {
				return err
			}
			var order models.Order
			if err := tx.First(&order, orderID).Error; err != nil {
				return err
			}
			if order.Status == targetStatus {
				return nil
			}
			from := order.Status
			if err := orderservice.ApplyStatusTransition(tx, &order, targetStatus); err != nil {
				return err
			}
			return paymentservice.AppendOrderStatusHistory(tx, order.ID, from, targetStatus, reason, "admin", "admin", correlationID(finalizeCtx))
		})
	}
	input := providerops.PaymentMutationInput{Prepare: prepare, Registry: e.runtime.PaymentProviders, Request: providerRequest, DomainTransactionID: result.transaction.ID, Finalize: finalize}
	switch operation {
	case "capture":
		result.operation, err = e.runtime.Executor.ExecutePaymentCapture(ctx, input)
	case "void":
		result.operation, err = e.runtime.Executor.ExecutePaymentVoid(ctx, input)
	case "refund":
		result.operation, err = e.runtime.Executor.ExecutePaymentRefund(ctx, input)
	}
	if err != nil && !providerops.PaymentOperationRecoverable(result.operation) {
		return result, err
	}
	result.accepted = providerops.PaymentOperationRecoverable(result.operation)
	if !result.accepted {
		intents, loadErr := e.payments.GetOrderPaymentLedger(ctx, orderID)
		if loadErr != nil {
			return result, loadErr
		}
		for _, intent := range intents {
			if intent.ID == intentID {
				result.intent = intent
				for _, txn := range intent.Transactions {
					if txn.ID == result.transaction.ID {
						result.transaction = txn
					}
				}
			}
		}
		result.order, err = e.orders.Get(ctx, orderID, nil)
	}
	return result, err
}

func providerAccepted(operation models.ProviderOperation, message string) apicontract.ProviderOperationAcceptedEnvelope {
	return apicontract.ProviderOperationAcceptedEnvelope{Message: message, OperationKey: operation.OperationKey, Status: apicontract.ProviderOperationStatus(operation.Status)}
}
func adminPaymentEnvelope(v adminPaymentResult, message string) apicontract.AdminOrderPaymentLifecycleResponse {
	return apicontract.AdminOrderPaymentLifecycleResponse{Message: message, Order: orderContract(v.order, nil), PaymentIntent: paymentIntentContract(v.intent), Transaction: paymentTransactionsContract([]models.PaymentTransaction{v.transaction})[0]}
}

func (e *CheckoutProviderEndpoints) CaptureAdminOrderPayment(ctx context.Context, r apicontract.CaptureAdminOrderPaymentRequestObject) (apicontract.CaptureAdminOrderPaymentResponseObject, error) {
	var amount *float64
	if r.Body != nil {
		amount = r.Body.Amount
	}
	v, err := e.runAdminPayment(ctx, uint(r.Id), uint(r.IntentId), "capture", r.Params.IdempotencyKey, amount)
	if errors.Is(err, paymentservice.ErrCaptureNotAllowed) || errors.Is(err, paymentservice.ErrAmountExceedsAvailable) || errors.Is(err, providerops.ErrIdempotencyFingerprintConflict) {
		problem := providerAdminProblem(ctx, e.renderer, http.StatusConflict, "payment_lifecycle_conflict", err.Error(), err)
		return apicontract.CaptureAdminOrderPayment409ApplicationProblemPlusJSONResponse{ConflictProblemApplicationProblemPlusJSONResponse: apicontract.ConflictProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	if err != nil {
		return nil, err
	}
	if v.accepted {
		return apicontract.CaptureAdminOrderPayment202JSONResponse(providerAccepted(v.operation, "Payment capture pending reconciliation")), nil
	}
	return apicontract.CaptureAdminOrderPayment200JSONResponse(adminPaymentEnvelope(v, "Payment captured")), nil
}
func (e *CheckoutProviderEndpoints) VoidAdminOrderPayment(ctx context.Context, r apicontract.VoidAdminOrderPaymentRequestObject) (apicontract.VoidAdminOrderPaymentResponseObject, error) {
	v, err := e.runAdminPayment(ctx, uint(r.Id), uint(r.IntentId), "void", r.Params.IdempotencyKey, nil)
	if errors.Is(err, paymentservice.ErrVoidNotAllowed) || errors.Is(err, providerops.ErrIdempotencyFingerprintConflict) {
		problem := providerAdminProblem(ctx, e.renderer, http.StatusConflict, "payment_lifecycle_conflict", err.Error(), err)
		return apicontract.VoidAdminOrderPayment409ApplicationProblemPlusJSONResponse{ConflictProblemApplicationProblemPlusJSONResponse: apicontract.ConflictProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	if err != nil {
		return nil, err
	}
	if v.accepted {
		return apicontract.VoidAdminOrderPayment202JSONResponse(providerAccepted(v.operation, "Payment void pending reconciliation")), nil
	}
	return apicontract.VoidAdminOrderPayment200JSONResponse(adminPaymentEnvelope(v, "Payment voided")), nil
}
func (e *CheckoutProviderEndpoints) RefundAdminOrderPayment(ctx context.Context, r apicontract.RefundAdminOrderPaymentRequestObject) (apicontract.RefundAdminOrderPaymentResponseObject, error) {
	var amount *float64
	if r.Body != nil {
		amount = r.Body.Amount
	}
	v, err := e.runAdminPayment(ctx, uint(r.Id), uint(r.IntentId), "refund", r.Params.IdempotencyKey, amount)
	if errors.Is(err, paymentservice.ErrRefundNotAllowed) || errors.Is(err, paymentservice.ErrAmountExceedsAvailable) || errors.Is(err, providerops.ErrIdempotencyFingerprintConflict) {
		problem := providerAdminProblem(ctx, e.renderer, http.StatusConflict, "payment_lifecycle_conflict", err.Error(), err)
		return apicontract.RefundAdminOrderPayment409ApplicationProblemPlusJSONResponse{ConflictProblemApplicationProblemPlusJSONResponse: apicontract.ConflictProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	if err != nil {
		return nil, err
	}
	if v.accepted {
		return apicontract.RefundAdminOrderPayment202JSONResponse(providerAccepted(v.operation, "Payment refund pending reconciliation")), nil
	}
	return apicontract.RefundAdminOrderPayment200JSONResponse(adminPaymentEnvelope(v, "Payment refunded")), nil
}

var _ = gorm.ErrRecordNotFound
