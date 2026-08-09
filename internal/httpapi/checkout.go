package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/checkoutplugins"
	"ecommerce/internal/media"
	"ecommerce/internal/requestctx"
	checkoutservice "ecommerce/internal/services/checkout"
	orderservice "ecommerce/internal/services/orders"
	paymentservice "ecommerce/internal/services/payments"
	providerops "ecommerce/internal/services/providerops"
	shippingservice "ecommerce/internal/services/shipping"
	taxservice "ecommerce/internal/services/tax"
	webhookservice "ecommerce/internal/services/webhooks"
	"ecommerce/models"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"gorm.io/gorm"
)

type CheckoutProviderEndpointsOptions struct {
	DB              *gorm.DB
	Media           *media.Service
	CheckoutPlugins *checkoutplugins.Manager
	ProviderRuntime *providerops.Runtime
	Webhooks        *webhookservice.Service
	Renderer        Renderer
}

// CheckoutProviderEndpoints is the embeddable generated-strict endpoint family
// for carts, checkout, orders, provider operations, and webhooks.
type CheckoutProviderEndpoints struct {
	db       *gorm.DB
	media    *media.Service
	plugins  *checkoutplugins.Manager
	runtime  *providerops.Runtime
	webhooks *webhookservice.Service
	checkout *checkoutservice.Service
	orders   *orderservice.Service
	payments *paymentservice.Service
	shipping *shippingservice.Service
	tax      *taxservice.Service
	catalog  *providerops.CatalogService
	queries  *providerops.QueryService
	admin    *providerops.AdminService
	cases    *providerops.CaseService
	overview *providerops.OverviewService
	renderer Renderer
}

func NewCheckoutProviderEndpoints(options CheckoutProviderEndpointsOptions) (*CheckoutProviderEndpoints, error) {
	if options.DB == nil {
		return nil, errors.New("checkout provider database is required")
	}
	plugins := options.CheckoutPlugins
	if plugins == nil {
		plugins = checkoutplugins.NewDefaultManager()
	}
	runtime := options.ProviderRuntime
	if runtime == nil {
		runtime = providerops.NewRuntime(options.DB, providerops.RuntimeConfig{})
	} else {
		runtime.BindDatabase(options.DB)
	}
	webhooks := options.Webhooks
	if webhooks == nil {
		webhooks = webhookservice.NewService(options.DB, runtime.PaymentProviders, runtime.ShippingProviders, nil)
	}
	resolver := providerops.NewRuntimeLifecycleResolver(runtime.Executor.Lifecycles(), runtime.PaymentProviders, runtime.ShippingProviders, runtime.TaxProviders)
	return &CheckoutProviderEndpoints{
		db: options.DB, media: options.Media, plugins: plugins, runtime: runtime, webhooks: webhooks,
		checkout: checkoutservice.NewService(options.DB), orders: orderservice.NewService(options.DB), payments: paymentservice.NewService(options.DB),
		shipping: shippingservice.NewService(options.DB), tax: taxservice.NewService(options.DB), catalog: providerops.NewCatalogService(options.DB, plugins),
		queries: providerops.NewQueryService(options.DB), admin: providerops.NewAdminService(options.DB, runtime.Executor, resolver), cases: providerops.NewCaseService(options.DB),
		overview: providerops.NewOverviewService(options.DB, runtime.Environment, runtime.Credentials, webhooks.MaxAttempts), renderer: options.Renderer,
	}, nil
}

func checkoutEndpointError(err error) error {
	var stockErr *orderservice.InsufficientStockError
	switch {
	case errors.Is(err, checkoutservice.ErrInvalidQuantity):
		return problemError(http.StatusBadRequest, "invalid_quantity", err.Error(), err)
	case errors.Is(err, checkoutservice.ErrVariantNotFound), errors.Is(err, checkoutservice.ErrCartItemNotFound):
		return problemError(http.StatusNotFound, "not_found", err.Error(), err)
	case errors.Is(err, orderservice.ErrOrderNotFound):
		return problemError(http.StatusNotFound, "not_found", "Order not found", err)
	case errors.Is(err, checkoutservice.ErrIdempotencyConflict):
		return problemError(http.StatusConflict, "idempotency_key_conflict", err.Error(), err)
	case errors.Is(err, checkoutservice.ErrIdempotencyInProgress):
		return problemError(http.StatusConflict, "idempotency_in_progress", err.Error(), err)
	case errors.Is(err, orderservice.ErrInvalidOrderStatus), errors.Is(err, orderservice.ErrOrderRequiresItems), errors.Is(err, orderservice.ErrInvalidOrderItem):
		return problemError(http.StatusBadRequest, "invalid_order", err.Error(), err)
	case errors.Is(err, orderservice.ErrOrderCannotBeCanceled):
		return problemError(http.StatusBadRequest, "order_cannot_be_cancelled", "Order cannot be cancelled", err)
	case errors.Is(err, orderservice.ErrCheckoutConverted), errors.Is(err, orderservice.ErrCheckoutOrderStale):
		return problemError(http.StatusConflict, "checkout_state_conflict", err.Error(), err)
	case errors.Is(err, orderservice.ErrOrderPaymentSubmitted):
		return problemError(http.StatusBadRequest, "payment_already_submitted", err.Error(), err)
	case errors.Is(err, paymentservice.ErrSnapshotExpired), errors.Is(err, paymentservice.ErrSnapshotNotFound):
		return problemError(http.StatusBadRequest, "invalid_checkout_snapshot", err.Error(), err)
	case errors.Is(err, paymentservice.ErrSnapshotAlreadyBound):
		return problemError(http.StatusConflict, "checkout_snapshot_conflict", "checkout snapshot is already bound to another order", err)
	case errors.Is(err, paymentservice.ErrSnapshotOrderMismatch), errors.Is(err, paymentservice.ErrActivePaymentIntentExists):
		return problemError(http.StatusConflict, "checkout_snapshot_conflict", err.Error(), err)
	case errors.As(err, &stockErr):
		return ErrorProblem(Problem{
			Type: TypeInvalidRequest, Title: http.StatusText(http.StatusBadRequest), Status: http.StatusBadRequest,
			Code: "insufficient_stock", Detail: "Insufficient stock", LegacyError: "Insufficient stock",
			ProductVariantID: stockErr.ProductVariantID, Requested: stockErr.Requested, Available: stockErr.Available,
		}, err)
	case errors.Is(err, gorm.ErrRecordNotFound):
		return problemError(http.StatusBadRequest, "variant_not_found", "Product variant not found", err)
	default:
		return err
	}
}

func principalAccountID(ctx context.Context) (uint, error) {
	principal, err := requestctx.RequirePrincipal(ctx)
	if errors.Is(err, requestctx.ErrPrincipalRequired) {
		if _, ok := checkoutservice.SessionFromContext(ctx); ok {
			return 0, nil
		}
	}
	if err != nil || principal.AccountID == 0 {
		return 0, problemError(http.StatusUnauthorized, "authentication_required", "Authentication is required.", err)
	}
	return principal.AccountID, nil
}

func (e *CheckoutProviderEndpoints) GetCheckoutCart(ctx context.Context, _ apicontract.GetCheckoutCartRequestObject) (apicontract.GetCheckoutCartResponseObject, error) {
	id, err := principalAccountID(ctx)
	if err != nil {
		return nil, err
	}
	cart, err := e.checkout.Cart(ctx, id)
	if err != nil {
		return nil, err
	}
	return apicontract.GetCheckoutCart200JSONResponse(cartContract(cart)), nil
}
func (e *CheckoutProviderEndpoints) AddCheckoutCartItem(ctx context.Context, request apicontract.AddCheckoutCartItemRequestObject) (apicontract.AddCheckoutCartItemResponseObject, error) {
	id, err := principalAccountID(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, errors.New("cart item body is required")
	}
	cart, err := e.checkout.AddCartItem(ctx, id, uint(request.Body.ProductVariantId), request.Body.Quantity)
	if err != nil {
		return nil, checkoutEndpointError(err)
	}
	return apicontract.AddCheckoutCartItem200JSONResponse(cartContract(cart)), nil
}
func (e *CheckoutProviderEndpoints) UpdateCheckoutCartItem(ctx context.Context, request apicontract.UpdateCheckoutCartItemRequestObject) (apicontract.UpdateCheckoutCartItemResponseObject, error) {
	id, err := principalAccountID(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil || request.ItemId < 1 {
		return nil, errors.New("valid cart item and body are required")
	}
	item, err := e.checkout.UpdateCartItem(ctx, id, uint(request.ItemId), request.Body.Quantity)
	if err != nil {
		return nil, checkoutEndpointError(err)
	}
	return apicontract.UpdateCheckoutCartItem200JSONResponse(cartItemContract(item)), nil
}
func (e *CheckoutProviderEndpoints) DeleteCheckoutCartItem(ctx context.Context, request apicontract.DeleteCheckoutCartItemRequestObject) (apicontract.DeleteCheckoutCartItemResponseObject, error) {
	id, err := principalAccountID(ctx)
	if err != nil {
		return nil, err
	}
	if request.ItemId < 1 {
		return nil, errors.New("cart item id must be positive")
	}
	if err := e.checkout.DeleteCartItem(ctx, id, uint(request.ItemId)); err != nil {
		return nil, checkoutEndpointError(err)
	}
	return apicontract.DeleteCheckoutCartItem200JSONResponse{Message: "Cart item deleted"}, nil
}
func (e *CheckoutProviderEndpoints) GetCheckoutCartSummary(ctx context.Context, _ apicontract.GetCheckoutCartSummaryRequestObject) (apicontract.GetCheckoutCartSummaryResponseObject, error) {
	id := uint(0)
	if principal, ok := requestctx.PrincipalFrom(ctx); ok {
		id = principal.AccountID
	}
	count, err := e.checkout.CartItemCount(ctx, id)
	if err != nil {
		return nil, err
	}
	return apicontract.GetCheckoutCartSummary200JSONResponse{ItemCount: count}, nil
}

func (e *CheckoutProviderEndpoints) GetCart(ctx context.Context, _ apicontract.GetCartRequestObject) (apicontract.GetCartResponseObject, error) {
	id, err := principalAccountID(ctx)
	if err != nil {
		return nil, err
	}
	cart, err := e.checkout.Cart(ctx, id)
	if err != nil {
		return nil, err
	}
	return apicontract.GetCart200JSONResponse(cartContract(cart)), nil
}
func (e *CheckoutProviderEndpoints) AddCartItem(ctx context.Context, r apicontract.AddCartItemRequestObject) (apicontract.AddCartItemResponseObject, error) {
	id, err := principalAccountID(ctx)
	if err != nil {
		return nil, err
	}
	if r.Body == nil {
		return nil, errors.New("cart item body is required")
	}
	cart, err := e.checkout.AddCartItem(ctx, id, uint(r.Body.ProductVariantId), r.Body.Quantity)
	if err != nil {
		return nil, checkoutEndpointError(err)
	}
	return apicontract.AddCartItem200JSONResponse(cartContract(cart)), nil
}
func (e *CheckoutProviderEndpoints) UpdateCartItem(ctx context.Context, r apicontract.UpdateCartItemRequestObject) (apicontract.UpdateCartItemResponseObject, error) {
	id, err := principalAccountID(ctx)
	if err != nil {
		return nil, err
	}
	if r.Body == nil {
		return nil, errors.New("cart item body is required")
	}
	item, err := e.checkout.UpdateCartItem(ctx, id, uint(r.ItemId), r.Body.Quantity)
	if err != nil {
		return nil, checkoutEndpointError(err)
	}
	return apicontract.UpdateCartItem200JSONResponse(cartItemContract(item)), nil
}
func (e *CheckoutProviderEndpoints) DeleteCartItem(ctx context.Context, r apicontract.DeleteCartItemRequestObject) (apicontract.DeleteCartItemResponseObject, error) {
	id, err := principalAccountID(ctx)
	if err != nil {
		return nil, err
	}
	if err = e.checkout.DeleteCartItem(ctx, id, uint(r.ItemId)); err != nil {
		return nil, checkoutEndpointError(err)
	}
	return apicontract.DeleteCartItem200JSONResponse{Message: "Cart item deleted"}, nil
}

func (e *CheckoutProviderEndpoints) ListCheckoutSessionPlugins(ctx context.Context, _ apicontract.ListCheckoutSessionPluginsRequestObject) (apicontract.ListCheckoutSessionPluginsResponseObject, error) {
	catalog, err := e.pluginCatalog(ctx, false)
	if err != nil {
		return nil, err
	}
	return apicontract.ListCheckoutSessionPlugins200JSONResponse(catalog), nil
}
func (e *CheckoutProviderEndpoints) ListCheckoutPlugins(ctx context.Context, _ apicontract.ListCheckoutPluginsRequestObject) (apicontract.ListCheckoutPluginsResponseObject, error) {
	catalog, err := e.pluginCatalog(ctx, false)
	if err != nil {
		return nil, err
	}
	return apicontract.ListCheckoutPlugins200JSONResponse(catalog), nil
}
func (e *CheckoutProviderEndpoints) ListAdminCheckoutPlugins(ctx context.Context, _ apicontract.ListAdminCheckoutPluginsRequestObject) (apicontract.ListAdminCheckoutPluginsResponseObject, error) {
	catalog, err := e.pluginCatalog(ctx, true)
	if err != nil {
		return nil, err
	}
	return apicontract.ListAdminCheckoutPlugins200JSONResponse(catalog), nil
}
func (e *CheckoutProviderEndpoints) UpdateAdminCheckoutPlugin(ctx context.Context, r apicontract.UpdateAdminCheckoutPluginRequestObject) (apicontract.UpdateAdminCheckoutPluginResponseObject, error) {
	if r.Body == nil {
		return nil, errors.New("checkout plugin body is required")
	}
	typ, err := checkoutPluginProviderType(string(r.Type))
	if err != nil {
		return nil, err
	}
	if err = e.catalog.SetEnabled(ctx, typ, strings.TrimSpace(r.Id), r.Body.Enabled); err != nil {
		if errors.Is(err, providerops.ErrInvalidCheckoutProviderUpdate) {
			problem := providerAdminProblem(ctx, e.renderer, http.StatusBadRequest, "invalid_checkout_provider_update", err.Error(), err)
			return apicontract.UpdateAdminCheckoutPlugin400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
		return nil, err
	}
	catalog, err := e.pluginCatalog(ctx, true)
	if err != nil {
		return nil, err
	}
	return apicontract.UpdateAdminCheckoutPlugin200JSONResponse(catalog), nil
}

func (e *CheckoutProviderEndpoints) QuoteCheckoutSession(ctx context.Context, r apicontract.QuoteCheckoutSessionRequestObject) (apicontract.QuoteCheckoutSessionResponseObject, error) {
	q, err := e.quote(ctx, r.Body)
	if err != nil {
		return nil, err
	}
	return apicontract.QuoteCheckoutSession200JSONResponse(q), nil
}
func (e *CheckoutProviderEndpoints) QuoteCheckout(ctx context.Context, r apicontract.QuoteCheckoutRequestObject) (apicontract.QuoteCheckoutResponseObject, error) {
	q, err := e.quote(ctx, r.Body)
	if err != nil {
		return nil, err
	}
	return apicontract.QuoteCheckout200JSONResponse(q), nil
}

func (e *CheckoutProviderEndpoints) quote(ctx context.Context, body *apicontract.CheckoutQuoteRequest) (apicontract.CheckoutQuoteResponse, error) {
	userID, err := principalAccountID(ctx)
	if err != nil {
		return apicontract.CheckoutQuoteResponse{}, err
	}
	if body == nil {
		return apicontract.CheckoutQuoteResponse{}, errors.New("checkout quote body is required")
	}
	cart, err := e.checkout.Cart(ctx, userID)
	if err != nil {
		return apicontract.CheckoutQuoteResponse{}, err
	}
	subtotal := models.Money(0)
	items := make([]paymentservice.SnapshotItemInput, 0, len(cart.Items))
	for _, item := range cart.Items {
		subtotal += item.ProductVariant.Price.Mul(item.Quantity)
		items = append(items, paymentservice.SnapshotItemInput{ProductVariantID: item.ProductVariantID, VariantSKU: item.ProductVariant.SKU, VariantTitle: item.ProductVariant.Title, Quantity: item.Quantity, Price: item.ProductVariant.Price})
	}
	paymentData, shippingData, taxData := mapValue(body.PaymentData), mapValue(body.ShippingData), mapValue(body.TaxData)
	quote := e.plugins.Quote(checkoutplugins.QuoteRequest{Subtotal: subtotal, PaymentID: body.PaymentProviderId, ShippingID: body.ShippingProviderId, TaxID: body.TaxProviderId, PaymentData: paymentData, ShippingData: shippingData, TaxData: taxData})
	response := apicontract.CheckoutQuoteResponse{Currency: quote.Currency, Subtotal: quote.Subtotal.Float64(), Shipping: quote.Shipping.Float64(), Tax: quote.Tax.Float64(), Total: quote.Total.Float64(), Valid: quote.Valid, PaymentStates: pluginStates(quote.PaymentStates), ShippingStates: pluginStates(quote.ShippingStates), TaxStates: pluginStates(quote.TaxStates)}
	if quote.Valid {
		resolved, err := checkoutservice.ResolveProviderSelection(e.plugins, subtotal, checkoutservice.ProviderSelection{PaymentProviderID: body.PaymentProviderId, ShippingProviderID: body.ShippingProviderId, TaxProviderID: body.TaxProviderId, PaymentData: paymentData, ShippingData: shippingData, TaxData: taxData})
		if err != nil {
			return response, err
		}
		session, err := e.checkout.SessionForUser(ctx, userID, false)
		if err != nil {
			return response, err
		}
		snapshot, err := e.payments.CreateCheckoutSnapshot(ctx, paymentservice.CreateCheckoutSnapshotInput{CheckoutSessionID: session.ID, Currency: quote.Currency, Subtotal: quote.Subtotal, ShippingAmount: quote.Shipping, TaxAmount: quote.Tax, Total: quote.Total, PaymentProviderID: body.PaymentProviderId, ShippingProviderID: body.ShippingProviderId, TaxProviderID: body.TaxProviderId, PaymentData: paymentData, ShippingData: shippingData, TaxData: taxData, PaymentMethodDisplay: resolved.PaymentDisplay, ShippingAddressPretty: resolved.ShippingAddress, Items: items, Now: time.Now().UTC()})
		if err != nil {
			return response, err
		}
		sid := int(snapshot.ID)
		response.SnapshotId = &sid
		response.ExpiresAt = &snapshot.ExpiresAt
	}
	return response, nil
}

func (e *CheckoutProviderEndpoints) ListAdminOrders(ctx context.Context, r apicontract.ListAdminOrdersRequestObject) (apicontract.ListAdminOrdersResponseObject, error) {
	page, err := e.orders.List(ctx, orderservice.ListInput{Query: stringPtr(r.Params.Q), Page: intPtr(r.Params.Page), Limit: intPtr(r.Params.Limit)})
	if err != nil {
		return nil, err
	}
	return apicontract.ListAdminOrders200JSONResponse(orderPageContract(page, nil)), nil
}
func (e *CheckoutProviderEndpoints) GetAdminOrder(ctx context.Context, r apicontract.GetAdminOrderRequestObject) (apicontract.GetAdminOrderResponseObject, error) {
	o, err := e.orders.Get(ctx, uint(r.Id), nil)
	if err != nil {
		return nil, checkoutEndpointError(err)
	}
	return apicontract.GetAdminOrder200JSONResponse(orderContract(o, nil)), nil
}
func (e *CheckoutProviderEndpoints) ListUserOrders(ctx context.Context, r apicontract.ListUserOrdersRequestObject) (apicontract.ListUserOrdersResponseObject, error) {
	id, err := principalAccountID(ctx)
	if err != nil {
		return nil, err
	}
	start, err := datePtr(r.Params.StartDate, false)
	if err != nil {
		return nil, problemError(http.StatusBadRequest, "invalid_start_date", "Invalid start_date, expected YYYY-MM-DD", err)
	}
	end, err := datePtr(r.Params.EndDate, true)
	if err != nil {
		return nil, problemError(http.StatusBadRequest, "invalid_end_date", "Invalid end_date, expected YYYY-MM-DD", err)
	}
	if start != nil && end != nil && end.Before(*start) {
		return nil, problemError(http.StatusBadRequest, "invalid_date_range", "end_date must be on or after start_date", nil)
	}
	page, err := e.orders.List(ctx, orderservice.ListInput{UserID: &id, Status: enumString(r.Params.Status), StartDate: start, EndDate: end, Page: intPtr(r.Params.Page), Limit: intPtr(r.Params.Limit)})
	if err != nil {
		return nil, checkoutEndpointError(err)
	}
	return apicontract.ListUserOrders200JSONResponse(orderPageContract(page, &id)), nil
}
func (e *CheckoutProviderEndpoints) GetUserOrder(ctx context.Context, r apicontract.GetUserOrderRequestObject) (apicontract.GetUserOrderResponseObject, error) {
	id, err := principalAccountID(ctx)
	if err != nil {
		return nil, err
	}
	o, err := e.orders.Get(ctx, uint(r.Id), &id)
	if err != nil {
		return nil, checkoutEndpointError(err)
	}
	return apicontract.GetUserOrder200JSONResponse(orderContract(o, &id)), nil
}
func (e *CheckoutProviderEndpoints) CancelUserOrder(ctx context.Context, r apicontract.CancelUserOrderRequestObject) (apicontract.CancelUserOrderResponseObject, error) {
	id, err := principalAccountID(ctx)
	if err != nil {
		return nil, err
	}
	o, err := e.orders.Cancel(ctx, uint(r.Id), id)
	if err != nil {
		return nil, checkoutEndpointError(err)
	}
	return apicontract.CancelUserOrder200JSONResponse(orderContract(o, &id)), nil
}
func (e *CheckoutProviderEndpoints) ClaimGuestOrder(ctx context.Context, r apicontract.ClaimGuestOrderRequestObject) (apicontract.ClaimGuestOrderResponseObject, error) {
	id, err := principalAccountID(ctx)
	if err != nil {
		return nil, err
	}
	if r.Body == nil {
		return nil, errors.New("claim body is required")
	}
	o, replay, err := e.orders.ClaimGuest(ctx, id, string(r.Body.Email), r.Body.ConfirmationToken)
	if err != nil {
		return nil, err
	}
	message := "Order linked to your account"
	if replay {
		message = "Order already linked to this account"
	}
	return apicontract.ClaimGuestOrder200JSONResponse{Message: message, Order: orderContract(o, &id)}, nil
}
func (e *CheckoutProviderEndpoints) CreateOrder(ctx context.Context, r apicontract.CreateOrderRequestObject) (apicontract.CreateOrderResponseObject, error) {
	id, err := principalAccountID(ctx)
	if err != nil {
		return nil, err
	}
	if r.Body == nil {
		return nil, errors.New("order body is required")
	}
	session, err := e.checkout.SessionForUser(ctx, id, false)
	if err != nil {
		return nil, err
	}
	items := make([]orderservice.CreateItemInput, 0, len(r.Body.Items))
	for _, item := range r.Body.Items {
		items = append(items, orderservice.CreateItemInput{ProductVariantID: uint(item.ProductVariantId), Quantity: item.Quantity})
	}
	o, err := e.orders.Create(ctx, session.ID, &id, nil, items)
	if err != nil {
		return nil, err
	}
	return apicontract.CreateOrder201JSONResponse(orderContract(o, &id)), nil
}
func (e *CheckoutProviderEndpoints) CreateCheckoutOrder(ctx context.Context, r apicontract.CreateCheckoutOrderRequestObject) (apicontract.CreateCheckoutOrderResponseObject, error) {
	id, err := principalAccountID(ctx)
	if err != nil {
		return nil, err
	}
	session, ok := checkoutservice.SessionFromContext(ctx)
	if !ok {
		return nil, problemError(http.StatusBadRequest, "checkout_session_required", "A checkout session is required", nil)
	}
	var userID *uint
	var guestEmail *string
	if id != 0 {
		userID = &id
	} else {
		if r.Body == nil || r.Body.GuestEmail == nil || strings.TrimSpace(string(*r.Body.GuestEmail)) == "" {
			return nil, problemError(http.StatusBadRequest, "guest_email_required", "Guest email is required", nil)
		}
		address, parseErr := mail.ParseAddress(strings.ToLower(strings.TrimSpace(string(*r.Body.GuestEmail))))
		if parseErr != nil {
			return nil, problemError(http.StatusBadRequest, "invalid_guest_email", "Guest email is invalid", parseErr)
		}
		normalized := strings.ToLower(strings.TrimSpace(address.Address))
		guestEmail = &normalized
	}

	idempotency, err := e.checkout.BeginIdempotency(ctx, session.ID, "checkout_order_create", stringPtr(r.Params.IdempotencyKey), r.Body, correlationID(ctx))
	if err != nil {
		return nil, checkoutEndpointError(err)
	}
	if idempotency.Replay {
		var replay apicontract.Order
		if err := json.Unmarshal([]byte(idempotency.Record.ResponseBody), &replay); err != nil {
			return nil, err
		}
		return apicontract.CreateCheckoutOrder201JSONResponse(replay), nil
	}
	client := ""
	if request, ok := ctx.(interface{ ClientIP() string }); ok {
		client = request.ClientIP()
	}
	if !allowCheckoutSubmission("create_order", session.PublicToken, client, time.Now().UTC()) {
		_ = e.checkout.ReleaseIdempotency(ctx, idempotency.Record)
		return nil, problemError(http.StatusTooManyRequests, "checkout_rate_limited", "Too many checkout attempts. Please wait and try again.", nil)
	}

	cart, err := e.checkout.Cart(ctx, id)
	if err != nil {
		_ = e.checkout.ReleaseIdempotency(ctx, idempotency.Record)
		return nil, err
	}
	items := make([]orderservice.CreateItemInput, 0, len(cart.Items))
	for _, item := range cart.Items {
		items = append(items, orderservice.CreateItemInput{ProductVariantID: item.ProductVariantID, Quantity: item.Quantity})
	}
	o, created, err := e.orders.CreateOrReplaceOpen(ctx, cart.CheckoutSessionID, userID, guestEmail, items)
	if err != nil {
		_ = e.checkout.ReleaseIdempotency(ctx, idempotency.Record)
		return nil, checkoutEndpointError(err)
	}
	if guestEmail != nil {
		if err := e.db.WithContext(ctx).Model(&models.CheckoutSession{}).Where("id = ?", session.ID).Updates(map[string]any{"guest_email": *guestEmail, "last_seen_at": time.Now().UTC()}).Error; err != nil {
			return nil, err
		}
	}
	response := orderContract(o, userID)
	status := http.StatusCreated
	if !created && idempotency.Record == nil {
		status = http.StatusOK
	}
	if err := e.checkout.CompleteIdempotency(ctx, idempotency.Record, status, response); err != nil {
		return nil, err
	}
	if status == http.StatusOK {
		return createCheckoutOrder200JSONResponse(response), nil
	}
	return apicontract.CreateCheckoutOrder201JSONResponse(response), nil
}

type createCheckoutOrder200JSONResponse apicontract.Order

func (response createCheckoutOrder200JSONResponse) VisitCreateCheckoutOrderResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	return json.NewEncoder(w).Encode(response)
}
func (e *CheckoutProviderEndpoints) UpdateOrderStatus(ctx context.Context, r apicontract.UpdateOrderStatusRequestObject) (apicontract.UpdateOrderStatusResponseObject, error) {
	if r.Body == nil {
		return nil, errors.New("order status body is required")
	}
	o, err := e.orders.UpdateStatus(ctx, uint(r.Id), string(r.Body.Status))
	if err != nil {
		return nil, checkoutEndpointError(err)
	}
	return apicontract.UpdateOrderStatus200JSONResponse(orderContract(o, nil)), nil
}

func cartContract(cart models.Cart) apicontract.Cart {
	items := make([]apicontract.CartItem, 0, len(cart.Items))
	for _, item := range cart.Items {
		items = append(items, cartItemContract(item))
	}
	userID := 0
	if cart.CheckoutSession.UserID != nil {
		userID = int(*cart.CheckoutSession.UserID)
	}
	return apicontract.Cart{Id: int(cart.ID), UserId: userID, Items: items, CreatedAt: cart.CreatedAt, UpdatedAt: cart.UpdatedAt, DeletedAt: deletedAt(cart.DeletedAt)}
}
func cartItemContract(item models.CartItem) apicontract.CartItem {
	return apicontract.CartItem{Id: int(item.ID), CartId: int(item.CartID), ProductVariantId: int(item.ProductVariantID), Quantity: item.Quantity, Product: basicProductContract(item.ProductVariant.Product), ProductVariant: basicVariantContract(item.ProductVariant), CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, DeletedAt: deletedAt(item.DeletedAt)}
}
func basicProductContract(p models.Product) apicontract.Product {
	return apicontract.Product{Id: int(p.ID), Sku: p.SKU, Name: p.Name, Description: p.Description, Price: p.Price.Float64(), Stock: p.Stock, Images: append([]string(nil), p.Images...), Attributes: []apicontract.ProductAttributeValue{}, Categories: []apicontract.Category{}, Options: []apicontract.ProductOption{}, RelatedProducts: []apicontract.RelatedProduct{}, Variants: []apicontract.ProductVariant{}, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt, DeletedAt: deletedAt(p.DeletedAt)}
}
func basicVariantContract(v models.ProductVariant) apicontract.ProductVariant {
	id := int(v.ID)
	return apicontract.ProductVariant{Id: &id, Sku: v.SKU, Title: v.Title, Price: v.Price.Float64(), Stock: v.Stock, Position: v.Position, IsPublished: v.IsPublished}
}
func orderContract(o models.Order, owner *uint) apicontract.Order {
	items := make([]apicontract.OrderItem, 0, len(o.Items))
	for _, item := range o.Items {
		items = append(items, apicontract.OrderItem{Id: int(item.ID), OrderId: int(item.OrderID), ProductVariantId: int(item.ProductVariantID), VariantSku: item.VariantSKU, VariantTitle: item.VariantTitle, Quantity: item.Quantity, Price: item.Price.Float64(), ProductVariant: basicVariantContract(item.ProductVariant), Product: basicProductContract(item.ProductVariant.Product), CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, DeletedAt: deletedAt(item.DeletedAt)})
	}
	var uid *int
	if o.UserID != nil {
		v := int(*o.UserID)
		uid = &v
	}
	var email *openapi_types.Email
	if o.GuestEmail != nil {
		v := openapi_types.Email(*o.GuestEmail)
		email = &v
	}
	return apicontract.Order{Id: int(o.ID), UserId: uid, CheckoutSessionId: int(o.CheckoutSessionID), GuestEmail: email, ConfirmationToken: o.ConfirmationToken, Total: o.Total.Float64(), Status: apicontract.OrderStatus(o.Status), Items: items, PaymentMethodDisplay: optionalString(o.PaymentMethodDisplay), ShippingAddressPretty: optionalString(o.ShippingAddressPretty), CanCancel: owner != nil && o.UserID != nil && *owner == *o.UserID && models.IsUserCancelableOrderStatus(o.Status), CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt, DeletedAt: deletedAt(o.DeletedAt)}
}
func orderPageContract(p orderservice.Page, owner *uint) apicontract.OrderPage {
	data := make([]apicontract.Order, 0, len(p.Orders))
	for _, o := range p.Orders {
		data = append(data, orderContract(o, owner))
	}
	return apicontract.OrderPage{Data: data, Pagination: apicontract.Pagination{Page: p.Page, Limit: p.Limit, Total: int(p.Total), TotalPages: p.TotalPages}}
}

func stringPtr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
func intPtr(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
func enumString[T ~string](v *T) string {
	if v == nil {
		return ""
	}
	return string(*v)
}
func datePtr(raw *string, end bool) (*time.Time, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	v, err := time.Parse("2006-01-02", *raw)
	if err != nil {
		return nil, err
	}
	if end {
		v = v.Add(24*time.Hour - time.Nanosecond)
	}
	return &v, nil
}
func mapValue(v *map[string]string) map[string]string {
	if v == nil {
		return map[string]string{}
	}
	return *v
}
func checkoutPluginProviderType(v string) (checkoutplugins.ProviderType, error) {
	switch checkoutplugins.ProviderType(v) {
	case checkoutplugins.ProviderTypePayment, checkoutplugins.ProviderTypeShipping, checkoutplugins.ProviderTypeTax:
		return checkoutplugins.ProviderType(v), nil
	default:
		return "", fmt.Errorf("unsupported provider type: %s", v)
	}
}
