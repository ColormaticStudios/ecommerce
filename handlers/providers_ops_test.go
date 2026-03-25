package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ecommerce/internal/apicontract"
	providerops "ecommerce/internal/services/providerops"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdminProviderCredentialEndpointDoesNotExposePlaintext(t *testing.T) {
	dbModels := []any{
		&models.User{},
		&models.ProviderCredential{},
		&models.ProviderCallAudit{},
		&models.ProviderReconciliationRun{},
		&models.ProviderReconciliationDrift{},
	}
	gin.SetMode(gin.TestMode)
	db := newTestDB(t, dbModels...)
	runtime := providerops.NewRuntime(db, providerops.RuntimeConfig{
		Environment: models.ProviderEnvironmentSandbox,
		Credentials: mustCredentialService(t, map[string][]byte{
			"v1": []byte("0123456789abcdef0123456789abcdef"),
		}, "v1"),
	})
	r := gin.New()
	server, err := NewGeneratedAPIServer(db, nil, GeneratedAPIServerConfig{
		JWTSecret:       generatedTestJWTSecret,
		ProviderRuntime: runtime,
	})
	require.NoError(t, err)
	apicontract.RegisterHandlers(r, server)

	admin := models.User{
		Username: "provider-admin",
		Email:    "provider-admin@example.com",
		Role:     "admin",
		Subject:  "provider-admin-subject",
	}
	require.NoError(t, db.Create(&admin).Error)
	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)

	body := map[string]any{
		"provider_type":        "payment",
		"provider_id":          "dummy-card",
		"environment":          "sandbox",
		"label":                "sandbox card",
		"secret_data":          map[string]string{"api_key": "plaintext-secret-value"},
		"supported_currencies": []string{"USD"},
		"settlement_currency":  "USD",
		"fx_mode":              "same_currency_only",
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	req := adminLifecycleRequest(
		t,
		r,
		http.MethodPost,
		"/api/v1/admin/providers/credentials",
		string(rawBody),
		adminToken,
		"",
	)
	require.Equal(t, http.StatusOK, req.Code)
	require.NotContains(t, req.Body.String(), "plaintext-secret-value")

	var payload struct {
		Credential struct {
			ID uint `json:"id"`
		} `json:"credential"`
	}
	require.NoError(t, json.Unmarshal(req.Body.Bytes(), &payload))

	var stored models.ProviderCredential
	require.NoError(t, db.First(&stored, payload.Credential.ID).Error)
	require.NotContains(t, stored.SecretEnvelopeJSON, "plaintext-secret-value")

	listReq, err := http.NewRequest(http.MethodGet, "/api/v1/admin/providers/credentials", nil)
	require.NoError(t, err)
	listReq.Header.Set("Authorization", "Bearer "+adminToken)
	listW := httptest.NewRecorder()
	r.ServeHTTP(listW, listReq)
	require.Equal(t, http.StatusOK, listW.Code)
	require.NotContains(t, listW.Body.String(), "plaintext-secret-value")
}

func TestAdminProviderOperationsOverviewIncludesRuntimeAndWebhookHealth(t *testing.T) {
	dbModels := []any{
		&models.User{},
		&models.ProviderCredential{},
		&models.ProviderCallAudit{},
		&models.ProviderReconciliationRun{},
		&models.ProviderReconciliationDrift{},
		&models.WebhookEvent{},
	}
	gin.SetMode(gin.TestMode)
	db := newTestDB(t, dbModels...)
	runtime := providerops.NewRuntime(db, providerops.RuntimeConfig{
		Environment: models.ProviderEnvironmentProduction,
		Credentials: mustCredentialService(t, map[string][]byte{
			"v1": []byte("0123456789abcdef0123456789abcdef"),
		}, "v1"),
	})
	r := gin.New()
	server, err := NewGeneratedAPIServer(db, nil, GeneratedAPIServerConfig{
		JWTSecret:       generatedTestJWTSecret,
		ProviderRuntime: runtime,
	})
	require.NoError(t, err)
	apicontract.RegisterHandlers(r, server)

	admin := models.User{
		Username: "provider-ops-admin",
		Email:    "provider-ops-admin@example.com",
		Role:     "admin",
		Subject:  "provider-ops-admin-subject",
	}
	require.NoError(t, db.Create(&admin).Error)
	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)

	now := time.Now().UTC()
	require.NoError(t, db.Create(&models.WebhookEvent{
		Provider:        "dummy-card",
		ProviderEventID: "evt-pending",
		EventType:       "payment.authorized",
		SignatureValid:  true,
		Payload:         "{}",
		ReceivedAt:      now,
	}).Error)
	require.NoError(t, db.Create(&models.WebhookEvent{
		Provider:        "dummy-card",
		ProviderEventID: "evt-processed",
		EventType:       "payment.captured",
		SignatureValid:  true,
		Payload:         "{}",
		ReceivedAt:      now,
		ProcessedAt:     &now,
	}).Error)
	require.NoError(t, db.Create(&models.WebhookEvent{
		Provider:        "dummy-card",
		ProviderEventID: "evt-dead",
		EventType:       "payment.failed",
		SignatureValid:  true,
		Payload:         "{}",
		ReceivedAt:      now,
		AttemptCount:    3,
		LastError:       "boom",
	}).Error)
	require.NoError(t, db.Create(&models.WebhookEvent{
		Provider:        "dummy-card",
		ProviderEventID: "evt-rejected",
		EventType:       "signature.invalid",
		SignatureValid:  false,
		Payload:         "{}",
		ReceivedAt:      now,
		LastError:       "invalid webhook signature",
	}).Error)

	req := adminLifecycleRequest(
		t,
		r,
		http.MethodGet,
		"/api/v1/admin/providers/overview",
		"",
		adminToken,
		"",
	)
	require.Equal(t, http.StatusOK, req.Code)

	var payload struct {
		RuntimeEnvironment          string `json:"runtime_environment"`
		CredentialServiceConfigured bool   `json:"credential_service_configured"`
		WebhookEvents               struct {
			PendingCount    int `json:"pending_count"`
			ProcessedCount  int `json:"processed_count"`
			DeadLetterCount int `json:"dead_letter_count"`
			RejectedCount   int `json:"rejected_count"`
		} `json:"webhook_events"`
	}
	require.NoError(t, json.Unmarshal(req.Body.Bytes(), &payload))
	require.Equal(t, models.ProviderEnvironmentProduction, payload.RuntimeEnvironment)
	require.True(t, payload.CredentialServiceConfigured)
	require.Equal(t, 1, payload.WebhookEvents.PendingCount)
	require.Equal(t, 1, payload.WebhookEvents.ProcessedCount)
	require.Equal(t, 1, payload.WebhookEvents.DeadLetterCount)
	require.Equal(t, 1, payload.WebhookEvents.RejectedCount)
}

func mustCredentialService(t *testing.T, keyring map[string][]byte, activeKeyVersion string) *providerops.CredentialService {
	t.Helper()
	service, err := providerops.NewCredentialService(keyring, activeKeyVersion)
	require.NoError(t, err)
	return service
}
