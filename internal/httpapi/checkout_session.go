package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/requestctx"
	checkoutservice "ecommerce/internal/services/checkout"
	paymentservice "ecommerce/internal/services/payments"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	checkoutSessionCookieName = "checkout_session"
	checkoutCSRFCookieName    = "csrf_token"
)

type checkoutSessionMode struct {
	create         bool
	allowConverted bool
}

var checkoutSessionOperations = map[string]checkoutSessionMode{
	"GetCheckoutCart":                  {create: true},
	"GetCheckoutCartSummary":           {},
	"AddCheckoutCartItem":              {create: true},
	"UpdateCheckoutCartItem":           {create: true},
	"DeleteCheckoutCartItem":           {create: true},
	"ListCheckoutSessionPlugins":       {},
	"QuoteCheckoutSession":             {create: true},
	"CreateCheckoutOrder":              {create: true},
	"AuthorizeCheckoutOrderPayment":    {create: true, allowConverted: true},
	"QuoteCheckoutOrderShippingRates":  {create: true, allowConverted: true},
	"GetCheckoutOrderShippingTracking": {create: true, allowConverted: true},
	"FinalizeCheckoutOrderTax":         {create: true, allowConverted: true},
	"GetCart":                          {create: true},
	"AddCartItem":                      {create: true},
	"UpdateCartItem":                   {create: true},
	"DeleteCartItem":                   {create: true},
	"QuoteCheckout":                    {create: true},
	"CreateOrder":                      {create: true},
}

func (e *CheckoutProviderEndpoints) CheckoutSessionMiddleware() apicontract.StrictMiddlewareFunc {
	return func(next apicontract.StrictHandlerFunc, operationID string) apicontract.StrictHandlerFunc {
		mode, applies := checkoutSessionOperations[operationID]
		if !applies {
			return next
		}
		return func(ctx *gin.Context, request interface{}) (interface{}, error) {
			userID := uint(0)
			if principal, ok := requestctx.PrincipalFrom(ctx.Request.Context()); ok {
				userID = principal.AccountID
			}
			metadata, _ := requestctx.MetadataFrom(ctx.Request.Context())
			resolved, err := e.checkout.ResolveSession(ctx.Request.Context(), checkoutservice.ResolveSessionInput{
				UserID: userID, Token: metadata.Cookies[checkoutSessionCookieName], Create: mode.create, AllowConverted: mode.allowConverted,
			})
			if errors.Is(err, checkoutservice.ErrGuestCheckoutDisabled) {
				return nil, problemError(http.StatusForbidden, "guest_checkout_disabled", "Guest checkout is disabled", err)
			}
			if err != nil {
				return nil, err
			}
			if resolved.Session != nil {
				requestContext := checkoutservice.WithSession(ctx.Request.Context(), *resolved.Session)
				ctx.Request = ctx.Request.WithContext(requestContext)
				if resolved.SetCookie {
					setCheckoutCookie(ctx, checkoutSessionCookieName, resolved.Session.PublicToken, true)
					if metadata.Cookies[checkoutCSRFCookieName] == "" {
						setCheckoutCookie(ctx, checkoutCSRFCookieName, uuid.NewString(), false)
					}
				}
			}
			switch operationID {
			case "AuthorizeCheckoutOrderPayment":
				typed, ok := request.(apicontract.AuthorizeCheckoutOrderPaymentRequestObject)
				if !ok {
					return nil, errors.New("invalid payment authorization request")
				}
				return e.authorizeCheckoutOrderPayment(ctx, typed)
			case "QuoteCheckoutOrderShippingRates":
				typed, ok := request.(apicontract.QuoteCheckoutOrderShippingRatesRequestObject)
				if !ok || typed.Body == nil {
					return nil, problemError(http.StatusBadRequest, "invalid_request", "Shipping quote body is required", nil)
				}
				if err := e.validateCheckoutSnapshot(ctx, uint(typed.Id), uint(typed.Body.SnapshotId)); err != nil {
					return nil, checkoutEndpointError(err)
				}
			case "FinalizeCheckoutOrderTax":
				typed, ok := request.(apicontract.FinalizeCheckoutOrderTaxRequestObject)
				if !ok || typed.Body == nil {
					return nil, problemError(http.StatusBadRequest, "invalid_request", "Tax finalization body is required", nil)
				}
				if err := e.validateCheckoutSnapshot(ctx, uint(typed.Id), uint(typed.Body.SnapshotId)); err != nil {
					return nil, checkoutEndpointError(err)
				}
			}
			response, err := next(ctx, request)
			if err != nil {
				return nil, checkoutEndpointError(err)
			}
			return response, nil
		}
	}
}

func (e *CheckoutProviderEndpoints) validateCheckoutSnapshot(ctx context.Context, orderID, snapshotID uint) error {
	ownerID, err := principalAccountID(ctx)
	if err != nil {
		return err
	}
	order, err := e.orders.Get(ctx, orderID, &ownerID)
	if err != nil {
		return err
	}
	snapshot, err := e.payments.GetCheckoutSnapshotForSession(ctx, order.CheckoutSessionID, snapshotID)
	if err != nil {
		return err
	}
	return paymentservice.ValidateSnapshotForOrder(&snapshot, &order, time.Now().UTC())
}

func setCheckoutCookie(ctx *gin.Context, name, value string, httpOnly bool) {
	http.SetCookie(ctx.Writer, &http.Cookie{
		Name: name, Value: value, Path: "/", MaxAge: int(checkoutservice.SessionTTL.Seconds()),
		Expires: time.Now().UTC().Add(checkoutservice.SessionTTL), HttpOnly: httpOnly, SameSite: http.SameSiteLaxMode,
	})
}
