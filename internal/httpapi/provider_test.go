package httpapi_test

import (
	"context"
	"fmt"
	"testing"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"
	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRetryFinalizeProviderOperationUsesAcceptedResponse(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProviderOperation{}, &models.ProviderOperationAttempt{}, &models.ProviderReconciliationCase{}))
	operation := models.ProviderOperation{OperationKey: "payment:test", ProviderType: models.ProviderTypePayment, ProviderID: "dummy-card", Environment: models.ProviderEnvironmentSandbox, Operation: "capture", IdempotencyKey: "test", RequestFingerprint: "hash", EntityType: "payment_intent", EntityID: 1, Status: models.ProviderOperationStatusFinalizeRetry, Version: 1}
	require.NoError(t, db.Create(&operation).Error)
	endpoints, err := httpapi.NewCheckoutProviderEndpoints(httpapi.CheckoutProviderEndpointsOptions{DB: db})
	require.NoError(t, err)
	response, err := endpoints.RetryFinalizeAdminProviderOperation(context.Background(), apicontract.RetryFinalizeAdminProviderOperationRequestObject{Id: int(operation.ID)})
	require.NoError(t, err)
	accepted, ok := response.(apicontract.RetryFinalizeAdminProviderOperation202JSONResponse)
	require.True(t, ok)
	assert.Equal(t, operation.ID, uint(accepted.Operation.Id))
}
