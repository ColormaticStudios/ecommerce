package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"ecommerce/internal/apicontract"
	providerops "ecommerce/internal/services/providerops"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type adminProviderCredentialRequest struct {
	ProviderType        string            `json:"provider_type" binding:"required"`
	ProviderID          string            `json:"provider_id" binding:"required"`
	Environment         string            `json:"environment" binding:"required"`
	Label               string            `json:"label"`
	SecretData          map[string]string `json:"secret_data" binding:"required"`
	SupportedCurrencies []string          `json:"supported_currencies,omitempty"`
	SettlementCurrency  string            `json:"settlement_currency,omitempty"`
	FxMode              string            `json:"fx_mode,omitempty"`
}

type adminProviderCredentialResponse struct {
	ID                  uint     `json:"id"`
	ProviderType        string   `json:"provider_type"`
	ProviderID          string   `json:"provider_id"`
	Environment         string   `json:"environment"`
	Label               string   `json:"label"`
	KeyVersion          string   `json:"key_version"`
	SupportedCurrencies []string `json:"supported_currencies"`
	SettlementCurrency  string   `json:"settlement_currency,omitempty"`
	FxMode              string   `json:"fx_mode"`
	LastRotatedAt       string   `json:"last_rotated_at"`
	UpdatedAt           string   `json:"updated_at"`
}

type adminProviderWebhookStatusSummaryResponse struct {
	PendingCount    int64 `json:"pending_count"`
	ProcessedCount  int64 `json:"processed_count"`
	DeadLetterCount int64 `json:"dead_letter_count"`
	RejectedCount   int64 `json:"rejected_count"`
}

type adminProviderOperationsOverviewResponse struct {
	RuntimeEnvironment          string                                    `json:"runtime_environment"`
	CredentialServiceConfigured bool                                      `json:"credential_service_configured"`
	WebhookEvents               adminProviderWebhookStatusSummaryResponse `json:"webhook_events"`
}

type adminProviderReconciliationRunRequest struct {
	ProviderType string `json:"provider_type" binding:"required"`
	ProviderID   string `json:"provider_id" binding:"required"`
}

type adminProviderReconciliationDriftResponse struct {
	ID                uint   `json:"id"`
	EntityType        string `json:"entity_type"`
	EntityID          uint   `json:"entity_id"`
	ProviderReference string `json:"provider_reference"`
	Severity          string `json:"severity"`
	FieldName         string `json:"field_name"`
	ExpectedValue     string `json:"expected_value"`
	ActualValue       string `json:"actual_value"`
	Message           string `json:"message"`
}

type adminProviderReconciliationRunResponse struct {
	ID           uint                                       `json:"id"`
	ProviderType string                                     `json:"provider_type"`
	ProviderID   string                                     `json:"provider_id"`
	Environment  string                                     `json:"environment"`
	Trigger      string                                     `json:"trigger"`
	Status       string                                     `json:"status"`
	CheckedCount int                                        `json:"checked_count"`
	DriftCount   int                                        `json:"drift_count"`
	ErrorCount   int                                        `json:"error_count"`
	StartedAt    string                                     `json:"started_at"`
	FinishedAt   *string                                    `json:"finished_at,omitempty"`
	Drifts       []adminProviderReconciliationDriftResponse `json:"drifts,omitempty"`
}

type adminProviderReconciliationRunPageResponse struct {
	Data       []adminProviderReconciliationRunResponse `json:"data"`
	Pagination apicontract.Pagination                   `json:"pagination"`
}

func ListAdminProviderCredentials(db *gorm.DB, service *providerops.CredentialService) gin.HandlerFunc {
	return func(c *gin.Context) {
		credentials, err := service.List(c.Request.Context(), db, c.Query("provider_type"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load provider credentials"})
			return
		}

		response := make([]adminProviderCredentialResponse, 0, len(credentials))
		for _, credential := range credentials {
			response = append(response, serializeProviderCredential(credential))
		}
		c.JSON(http.StatusOK, gin.H{"data": response})
	}
}

func UpsertAdminProviderCredential(db *gorm.DB, service *providerops.CredentialService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req adminProviderCredentialRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		credential, err := service.Store(c.Request.Context(), db, providerops.StoreCredentialInput{
			ProviderType: req.ProviderType,
			ProviderID:   req.ProviderID,
			Environment:  req.Environment,
			Label:        req.Label,
			SecretData:   req.SecretData,
			Metadata: providerops.CredentialMetadata{
				SupportedCurrencies: req.SupportedCurrencies,
				SettlementCurrency:  req.SettlementCurrency,
				FXMode:              req.FxMode,
			},
		})
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, providerops.ErrCredentialServiceUnconfigured) {
				status = http.StatusPreconditionFailed
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"credential": serializeProviderCredential(credential)})
	}
}

func RotateAdminProviderCredential(db *gorm.DB, service *providerops.CredentialService) gin.HandlerFunc {
	return func(c *gin.Context) {
		credentialID, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid credential ID"})
			return
		}

		credential, err := service.Rotate(c.Request.Context(), db, credentialID)
		if err != nil {
			switch {
			case errors.Is(err, providerops.ErrProviderCredentialNotFound):
				c.JSON(http.StatusNotFound, gin.H{"error": "Provider credential not found"})
			case errors.Is(err, providerops.ErrCredentialServiceUnconfigured):
				c.JSON(http.StatusPreconditionFailed, gin.H{"error": err.Error()})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to rotate provider credential"})
			}
			return
		}
		c.JSON(http.StatusOK, gin.H{"credential": serializeProviderCredential(credential)})
	}
}

func GetAdminProviderOperationsOverview(service *providerops.OverviewService) gin.HandlerFunc {
	return func(c *gin.Context) {
		overview, err := service.Get(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load provider operations overview"})
			return
		}
		c.JSON(http.StatusOK, adminProviderOperationsOverviewResponse{
			RuntimeEnvironment:          overview.RuntimeEnvironment,
			CredentialServiceConfigured: overview.CredentialServiceConfigured,
			WebhookEvents: adminProviderWebhookStatusSummaryResponse{
				PendingCount:    overview.WebhookEvents.PendingCount,
				ProcessedCount:  overview.WebhookEvents.ProcessedCount,
				DeadLetterCount: overview.WebhookEvents.DeadLetterCount,
				RejectedCount:   overview.WebhookEvents.RejectedCount,
			},
		})
	}
}

func ListAdminProviderReconciliationRuns(service *providerops.ReconciliationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, limit, _ := parsePagination(c, 20)
		runs, total, err := service.ListRuns(c.Request.Context(), c.Query("provider_type"), c.Query("provider_id"), page, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load provider reconciliation runs"})
			return
		}

		totalPages := int(total) / limit
		if int(total)%limit > 0 {
			totalPages++
		}
		response := adminProviderReconciliationRunPageResponse{
			Data: make([]adminProviderReconciliationRunResponse, 0, len(runs)),
			Pagination: apicontract.Pagination{
				Page:       page,
				Limit:      limit,
				Total:      int(total),
				TotalPages: totalPages,
			},
		}
		for _, run := range runs {
			response.Data = append(response.Data, serializeProviderReconciliationRun(run, false))
		}
		c.JSON(http.StatusOK, response)
	}
}

func CreateAdminProviderReconciliationRun(service *providerops.ReconciliationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req adminProviderReconciliationRunRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		run, _, err := service.Run(c.Request.Context(), providerops.ReconciliationRunInput{
			ProviderType: req.ProviderType,
			ProviderID:   req.ProviderID,
			Trigger:      models.ProviderReconciliationTriggerManual,
		})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"run": serializeProviderReconciliationRun(run, true)})
	}
}

func GetAdminProviderReconciliationRun(service *providerops.ReconciliationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		runID, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reconciliation run ID"})
			return
		}

		run, err := service.GetRun(c.Request.Context(), uint(runID))
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Provider reconciliation run not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load provider reconciliation run"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"run": serializeProviderReconciliationRun(run, true)})
	}
}

func serializeProviderCredential(credential providerops.StoredCredential) adminProviderCredentialResponse {
	return adminProviderCredentialResponse{
		ID:                  credential.Record.ID,
		ProviderType:        credential.Record.ProviderType,
		ProviderID:          credential.Record.ProviderID,
		Environment:         credential.Record.Environment,
		Label:               credential.Record.Label,
		KeyVersion:          credential.Record.KeyVersion,
		SupportedCurrencies: credential.Metadata.SupportedCurrencies,
		SettlementCurrency:  credential.Metadata.SettlementCurrency,
		FxMode:              credential.Metadata.FXMode,
		LastRotatedAt:       credential.Record.LastRotatedAt.UTC().Format(timeRFC3339JSON),
		UpdatedAt:           credential.Record.UpdatedAt.UTC().Format(timeRFC3339JSON),
	}
}

func serializeProviderReconciliationRun(run models.ProviderReconciliationRun, includeDrifts bool) adminProviderReconciliationRunResponse {
	response := adminProviderReconciliationRunResponse{
		ID:           run.ID,
		ProviderType: run.ProviderType,
		ProviderID:   run.ProviderID,
		Environment:  run.Environment,
		Trigger:      run.Trigger,
		Status:       run.Status,
		CheckedCount: run.CheckedCount,
		DriftCount:   run.DriftCount,
		ErrorCount:   run.ErrorCount,
		StartedAt:    run.StartedAt.UTC().Format(timeRFC3339JSON),
	}
	if run.FinishedAt != nil {
		value := run.FinishedAt.UTC().Format(timeRFC3339JSON)
		response.FinishedAt = &value
	}
	if includeDrifts {
		response.Drifts = make([]adminProviderReconciliationDriftResponse, 0, len(run.Drifts))
		for _, drift := range run.Drifts {
			response.Drifts = append(response.Drifts, adminProviderReconciliationDriftResponse{
				ID:                drift.ID,
				EntityType:        drift.EntityType,
				EntityID:          drift.EntityID,
				ProviderReference: drift.ProviderReference,
				Severity:          drift.Severity,
				FieldName:         drift.FieldName,
				ExpectedValue:     drift.ExpectedValue,
				ActualValue:       drift.ActualValue,
				Message:           drift.Message,
			})
		}
	}
	return response
}
