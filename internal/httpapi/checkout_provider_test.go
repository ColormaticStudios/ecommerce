package httpapi_test

import (
	"reflect"
	"testing"

	"ecommerce/internal/httpapi"

	"github.com/stretchr/testify/assert"
)

func TestCheckoutProviderEndpointsCoversStrictFamily(t *testing.T) {
	operations := []string{
		"GetCheckoutCart", "AddCheckoutCartItem", "DeleteCheckoutCartItem", "UpdateCheckoutCartItem", "GetCheckoutCartSummary",
		"CreateCheckoutOrder", "AuthorizeCheckoutOrderPayment", "QuoteCheckoutOrderShippingRates", "GetCheckoutOrderShippingTracking", "FinalizeCheckoutOrderTax",
		"ListCheckoutSessionPlugins", "QuoteCheckoutSession", "GetCart", "AddCartItem", "DeleteCartItem", "UpdateCartItem",
		"ListCheckoutPlugins", "QuoteCheckout", "ListUserOrders", "CreateOrder", "ClaimGuestOrder", "GetUserOrder", "CancelUserOrder",
		"ListAdminCheckoutPlugins", "UpdateAdminCheckoutPlugin", "ListAdminOrders", "GetAdminOrder", "GetAdminOrderPayments",
		"CaptureAdminOrderPayment", "RefundAdminOrderPayment", "VoidAdminOrderPayment", "CreateAdminOrderShippingLabel", "UpdateOrderStatus", "ExportAdminTaxReport",
		"ListAdminProviderCredentials", "UpsertAdminProviderCredential", "RotateAdminProviderCredential", "GetAdminProviderOperationsOverview",
		"ListAdminProviderOperations", "GetAdminProviderOperation", "QueryAdminProviderOperationOutcome", "RetryFinalizeAdminProviderOperation", "RetryCompensationAdminProviderOperation",
		"ListAdminProviderReconciliationCases", "GetAdminProviderReconciliationCase", "UpdateAdminProviderReconciliationCase",
		"ListAdminProviderReconciliationRuns", "CreateAdminProviderReconciliationRun", "GetAdminProviderReconciliationRun",
		"ListAdminWebhookEvents", "ReceiveWebhookEvent",
	}
	typeOfEndpoints := reflect.TypeOf((*httpapi.CheckoutProviderEndpoints)(nil))
	for _, operation := range operations {
		_, ok := typeOfEndpoints.MethodByName(operation)
		assert.Truef(t, ok, "CheckoutProviderEndpoints must implement %s", operation)
	}
}

func TestNewCheckoutProviderEndpointsRequiresDatabase(t *testing.T) {
	_, err := httpapi.NewCheckoutProviderEndpoints(httpapi.CheckoutProviderEndpointsOptions{})
	assert.Error(t, err)
}
