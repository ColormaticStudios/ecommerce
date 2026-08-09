package providerops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"ecommerce/internal/dbcontext"
	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrProviderOperationNotFound           = errors.New("provider operation not found")
	ErrIdempotencyFingerprintConflict      = errors.New("provider operation idempotency fingerprint conflict")
	ErrInvalidProviderOperationStatus      = errors.New("invalid provider operation status")
	ErrInvalidProviderAttemptOutcome       = errors.New("invalid provider operation attempt outcome")
	ErrInvalidProviderOperationTransition  = errors.New("invalid provider operation transition")
	ErrProviderOperationTransitionConflict = errors.New("provider operation transition conflict")
	ErrInvalidReconciliationCaseState      = errors.New("invalid provider reconciliation case state")
)

type OperationService struct {
	db *gorm.DB
}

type PrepareOperationInput struct {
	OperationKey       string
	ParentOperationID  *uint
	InitialStatus      string
	ProviderType       string
	ProviderID         string
	Environment        string
	Operation          string
	IdempotencyKey     string
	RequestFingerprint string
	Request            any
	CorrelationID      string
	EntityType         string
	EntityID           uint
	MetadataJSON       string
	RequestJSON        string
}

type ListOperationsInput struct {
	ProviderType string
	ProviderID   string
	Environment  string
	Operation    string
	Status       string
	EntityType   string
	EntityID     uint
	Page         int
	Limit        int
}

type TransitionOperationInput struct {
	OperationID       uint
	ExpectedStatus    string
	Status            string
	ProviderReference string
	MetadataJSON      string
}

type AppendAttemptInput struct {
	OperationID             uint
	Phase                   string
	Outcome                 string
	ProviderOutcome         string
	ProviderReference       string
	OperationKey            string
	ResultJSON              string
	Retryable               bool
	RequestPayloadRedacted  string
	ResponsePayloadRedacted string
	ErrorMessage            string
	StartedAt               time.Time
	FinishedAt              time.Time
}

type CreateReconciliationCaseInput struct {
	OperationID     uint
	AttemptID       *uint
	Reason          string
	CaseType        string
	ProviderOutcome string
	OperationKey    string
	DetailsJSON     string
	NextAttemptAt   *time.Time
}

func NewOperationService(db *gorm.DB) *OperationService {
	return &OperationService{db: db}
}

// RequestFingerprint returns a stable SHA-256 fingerprint of a JSON-encodable
// provider request. Callers may also supply a precomputed lowercase hex digest.
func RequestFingerprint(request any) (string, error) {
	encoded, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("marshal provider operation request: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

// PrepareOperation durably reserves an idempotency key. Reusing the same key
// and fingerprint returns the original operation; reusing it for a different
// request returns ErrIdempotencyFingerprintConflict.
func (s *OperationService) PrepareOperation(ctx context.Context, input PrepareOperationInput) (models.ProviderOperation, error) {
	db, err := s.database(ctx)
	if err != nil {
		return models.ProviderOperation{}, err
	}

	providerType, err := normalizeProviderType(input.ProviderType)
	if err != nil {
		return models.ProviderOperation{}, err
	}
	environment, err := normalizeProviderEnvironment(input.Environment)
	if err != nil {
		return models.ProviderOperation{}, err
	}
	providerID := strings.TrimSpace(input.ProviderID)
	operationName := strings.TrimSpace(input.Operation)
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if providerID == "" {
		return models.ProviderOperation{}, errors.New("provider id is required")
	}
	if operationName == "" {
		return models.ProviderOperation{}, errors.New("provider operation is required")
	}
	if idempotencyKey == "" {
		return models.ProviderOperation{}, errors.New("provider operation idempotency key is required")
	}

	fingerprint := strings.ToLower(strings.TrimSpace(input.RequestFingerprint))
	if fingerprint == "" {
		if input.Request == nil {
			return models.ProviderOperation{}, errors.New("provider operation request fingerprint is required")
		}
		fingerprint, err = RequestFingerprint(input.Request)
		if err != nil {
			return models.ProviderOperation{}, err
		}
	}
	if !validSHA256Fingerprint(fingerprint) {
		return models.ProviderOperation{}, errors.New("provider operation request fingerprint must be a SHA-256 hex digest")
	}

	operationKey := strings.TrimSpace(input.OperationKey)
	if operationKey == "" {
		operationKey = defaultOperationKey(providerType, providerID, environment, operationName, idempotencyKey)
	}
	initialStatus := strings.ToUpper(strings.TrimSpace(input.InitialStatus))
	if initialStatus == "" {
		initialStatus = models.ProviderOperationStatusPrepared
	}
	if initialStatus != models.ProviderOperationStatusPrepared && initialStatus != models.ProviderOperationStatusCompensationPrepared {
		return models.ProviderOperation{}, ErrInvalidProviderOperationStatus
	}
	requestJSON := normalizedJSONObject(input.RequestJSON)
	if input.RequestJSON == "" && input.Request != nil {
		encoded, marshalErr := json.Marshal(input.Request)
		if marshalErr != nil {
			return models.ProviderOperation{}, fmt.Errorf("marshal provider operation request: %w", marshalErr)
		}
		requestJSON = string(encoded)
	}

	operation := models.ProviderOperation{
		OperationKey:       operationKey,
		ParentOperationID:  input.ParentOperationID,
		ProviderType:       providerType,
		ProviderID:         providerID,
		Environment:        environment,
		Operation:          operationName,
		IdempotencyKey:     idempotencyKey,
		RequestFingerprint: fingerprint,
		CorrelationID:      strings.TrimSpace(input.CorrelationID),
		EntityType:         strings.TrimSpace(input.EntityType),
		EntityID:           input.EntityID,
		Status:             initialStatus,
		RequestJSON:        requestJSON,
		ResultJSON:         "{}",
		MetadataJSON:       normalizedJSONObject(input.MetadataJSON),
		Version:            1,
	}

	result := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&operation)
	if result.Error != nil {
		return models.ProviderOperation{}, result.Error
	}
	if result.RowsAffected == 0 {
		lookupErr := db.Where(
			"provider_type = ? AND provider_id = ? AND environment = ? AND operation = ? AND idempotency_key = ?",
			providerType, providerID, environment, operationName, idempotencyKey,
		).First(&operation).Error
		if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			lookupErr = db.Where("operation_key = ?", operationKey).First(&operation).Error
		}
		if lookupErr != nil {
			return models.ProviderOperation{}, lookupErr
		}
		if operation.RequestFingerprint != fingerprint {
			return models.ProviderOperation{}, ErrIdempotencyFingerprintConflict
		}
	}
	return operation, nil
}

func (s *OperationService) GetOperation(ctx context.Context, operationID uint) (models.ProviderOperation, error) {
	db, err := s.database(ctx)
	if err != nil {
		return models.ProviderOperation{}, err
	}
	var operation models.ProviderOperation
	err = db.Preload("Attempts", func(query *gorm.DB) *gorm.DB {
		return query.Order("attempt_number ASC")
	}).Preload("ReconciliationCases", func(query *gorm.DB) *gorm.DB {
		return query.Order("opened_at ASC, id ASC")
	}).First(&operation, operationID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.ProviderOperation{}, ErrProviderOperationNotFound
	}
	return operation, err
}

func (s *OperationService) ListOperations(ctx context.Context, input ListOperationsInput) ([]models.ProviderOperation, int64, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, 0, err
	}
	query := db.Model(&models.ProviderOperation{})
	if strings.TrimSpace(input.ProviderType) != "" {
		providerType, normalizeErr := normalizeProviderType(input.ProviderType)
		if normalizeErr != nil {
			return nil, 0, normalizeErr
		}
		query = query.Where("provider_type = ?", providerType)
	}
	if value := strings.TrimSpace(input.ProviderID); value != "" {
		query = query.Where("provider_id = ?", value)
	}
	if strings.TrimSpace(input.Environment) != "" {
		environment, normalizeErr := normalizeProviderEnvironment(input.Environment)
		if normalizeErr != nil {
			return nil, 0, normalizeErr
		}
		query = query.Where("environment = ?", environment)
	}
	if value := strings.TrimSpace(input.Operation); value != "" {
		query = query.Where("operation = ?", value)
	}
	if value := strings.ToUpper(strings.TrimSpace(input.Status)); value != "" {
		if !validOperationStatus(value) {
			return nil, 0, ErrInvalidProviderOperationStatus
		}
		query = query.Where("status = ?", value)
	}
	if value := strings.TrimSpace(input.EntityType); value != "" {
		query = query.Where("entity_type = ?", value)
	}
	if input.EntityID != 0 {
		query = query.Where("entity_id = ?", input.EntityID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, limit := input.Page, input.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var operations []models.ProviderOperation
	if err := query.Order("created_at DESC, id DESC").Offset((page - 1) * limit).Limit(limit).Find(&operations).Error; err != nil {
		return nil, 0, err
	}
	return operations, total, nil
}

// TransitionOperation performs a compare-and-update against ExpectedStatus so
// stale workers cannot overwrite a transition completed by another worker.
func (s *OperationService) TransitionOperation(ctx context.Context, input TransitionOperationInput) (models.ProviderOperation, error) {
	db, err := s.database(ctx)
	if err != nil {
		return models.ProviderOperation{}, err
	}
	from := strings.ToUpper(strings.TrimSpace(input.ExpectedStatus))
	to := strings.ToUpper(strings.TrimSpace(input.Status))
	if !validOperationStatus(from) || !validOperationStatus(to) {
		return models.ProviderOperation{}, ErrInvalidProviderOperationStatus
	}
	if !allowedOperationTransition(from, to) {
		return models.ProviderOperation{}, ErrInvalidProviderOperationTransition
	}

	updates := map[string]any{"status": to}
	if input.ProviderReference != "" {
		updates["provider_reference"] = strings.TrimSpace(input.ProviderReference)
	}
	if input.MetadataJSON != "" {
		updates["metadata_json"] = normalizedJSONObject(input.MetadataJSON)
	}
	result := db.Model(&models.ProviderOperation{}).
		Where("id = ? AND status = ?", input.OperationID, from).
		Updates(updates)
	if result.Error != nil {
		return models.ProviderOperation{}, result.Error
	}
	if result.RowsAffected == 0 {
		var count int64
		if err := db.Model(&models.ProviderOperation{}).Where("id = ?", input.OperationID).Count(&count).Error; err != nil {
			return models.ProviderOperation{}, err
		}
		if count == 0 {
			return models.ProviderOperation{}, ErrProviderOperationNotFound
		}
		return models.ProviderOperation{}, ErrProviderOperationTransitionConflict
	}
	return s.GetOperation(ctx, input.OperationID)
}

func (s *OperationService) AppendAttempt(ctx context.Context, input AppendAttemptInput) (models.ProviderOperationAttempt, error) {
	db, err := s.database(ctx)
	if err != nil {
		return models.ProviderOperationAttempt{}, err
	}
	outcome := strings.ToUpper(strings.TrimSpace(input.Outcome))
	if !validAttemptOutcome(outcome) {
		return models.ProviderOperationAttempt{}, ErrInvalidProviderAttemptOutcome
	}
	startedAt := input.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	finishedAt := input.FinishedAt
	if finishedAt.IsZero() {
		finishedAt = time.Now().UTC()
	}
	if finishedAt.Before(startedAt) {
		return models.ProviderOperationAttempt{}, errors.New("provider operation attempt finished before it started")
	}

	var attempt models.ProviderOperationAttempt
	err = db.Transaction(func(tx *gorm.DB) error {
		var operation models.ProviderOperation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&operation, input.OperationID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrProviderOperationNotFound
			}
			return err
		}
		var lastAttempt int
		if err := tx.Model(&models.ProviderOperationAttempt{}).
			Where("provider_operation_id = ?", operation.ID).
			Select("COALESCE(MAX(attempt_number), 0)").Scan(&lastAttempt).Error; err != nil {
			return err
		}
		attempt = models.ProviderOperationAttempt{
			ProviderOperationID:     operation.ID,
			AttemptNumber:           lastAttempt + 1,
			Phase:                   defaultIfEmpty(strings.TrimSpace(input.Phase), "provider"),
			Outcome:                 outcome,
			ProviderOutcome:         normalizeProviderOutcome(input.ProviderOutcome),
			ProviderReference:       strings.TrimSpace(input.ProviderReference),
			OperationKey:            defaultIfEmpty(strings.TrimSpace(input.OperationKey), operation.OperationKey),
			ResultJSON:              normalizedJSONObject(input.ResultJSON),
			Retryable:               input.Retryable,
			RequestPayloadRedacted:  strings.TrimSpace(input.RequestPayloadRedacted),
			ResponsePayloadRedacted: strings.TrimSpace(input.ResponsePayloadRedacted),
			ErrorMessage:            strings.TrimSpace(input.ErrorMessage),
			StartedAt:               startedAt,
			FinishedAt:              finishedAt,
		}
		return tx.Create(&attempt).Error
	})
	return attempt, err
}

// CreateReconciliationCase creates at most one open case for an operation and
// marks an UNKNOWN operation as requiring reconciliation in the same transaction.
func (s *OperationService) CreateReconciliationCase(ctx context.Context, input CreateReconciliationCaseInput) (models.ProviderReconciliationCase, error) {
	db, err := s.database(ctx)
	if err != nil {
		return models.ProviderReconciliationCase{}, err
	}
	var reconciliationCase models.ProviderReconciliationCase
	err = db.Transaction(func(tx *gorm.DB) error {
		var operation models.ProviderOperation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&operation, input.OperationID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrProviderOperationNotFound
			}
			return err
		}
		if operation.Status != models.ProviderOperationStatusUnknown && operation.Status != models.ProviderOperationStatusReconciliationRequired {
			return ErrInvalidReconciliationCaseState
		}
		if input.AttemptID != nil {
			var count int64
			if err := tx.Model(&models.ProviderOperationAttempt{}).
				Where("id = ? AND provider_operation_id = ?", *input.AttemptID, operation.ID).
				Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return errors.New("provider operation attempt does not belong to operation")
			}
		}

		openKey := models.ProviderReconciliationCaseStatusOpen
		reconciliationCase = models.ProviderReconciliationCase{
			ProviderOperationID: operation.ID,
			AttemptID:           input.AttemptID,
			OpenKey:             &openKey,
			Status:              models.ProviderReconciliationCaseStatusOpen,
			Reason:              strings.TrimSpace(input.Reason),
			CaseType:            defaultIfEmpty(strings.TrimSpace(input.CaseType), "ambiguous_outcome"),
			ProviderOutcome:     normalizeProviderOutcome(input.ProviderOutcome),
			OperationKey:        defaultIfEmpty(strings.TrimSpace(input.OperationKey), operation.OperationKey),
			DetailsJSON:         normalizedJSONObject(input.DetailsJSON),
			ResolutionJSON:      "{}",
			NextAttemptAt:       input.NextAttemptAt,
			OpenedAt:            time.Now().UTC(),
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reconciliationCase)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			if err := tx.Where("provider_operation_id = ? AND open_key = ?", operation.ID, openKey).
				First(&reconciliationCase).Error; err != nil {
				return err
			}
		}
		if operation.Status == models.ProviderOperationStatusUnknown {
			result = tx.Model(&models.ProviderOperation{}).
				Where("id = ? AND status = ?", operation.ID, models.ProviderOperationStatusUnknown).
				Update("status", models.ProviderOperationStatusReconciliationRequired)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrProviderOperationTransitionConflict
			}
		}
		return nil
	})
	return reconciliationCase, err
}

func (s *OperationService) database(ctx context.Context) (*gorm.DB, error) {
	if s == nil {
		return nil, errors.New("provider operation service is not configured")
	}
	db := s.db
	if tx := dbcontext.GetDB(ctx); tx != nil {
		db = tx
	}
	if db == nil {
		return nil, errors.New("provider operation service is not configured")
	}
	return db.WithContext(dbcontext.OrBackground(ctx)), nil
}

func defaultOperationKey(providerType, providerID, environment, operation, idempotencyKey string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{providerType, providerID, environment, operation, idempotencyKey}, "\x00")))
	return "op_" + hex.EncodeToString(sum[:16])
}

func validSHA256Fingerprint(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validOperationStatus(status string) bool {
	switch status {
	case models.ProviderOperationStatusPrepared,
		models.ProviderOperationStatusExecuting,
		models.ProviderOperationStatusOutcomeUnknown,
		models.ProviderOperationStatusProviderSucceeded,
		models.ProviderOperationStatusFinalizing,
		models.ProviderOperationStatusFinalizeRetry,
		models.ProviderOperationStatusCompensationPrepared,
		models.ProviderOperationStatusCompensating,
		models.ProviderOperationStatusCompensationSucceeded,
		models.ProviderOperationStatusCompensationRetry,
		models.ProviderOperationStatusCompleted,
		models.ProviderOperationStatusFailed,
		models.ProviderOperationStatusReconciling,
		models.ProviderOperationStatusReconciliationRequired:
		return true
	default:
		return false
	}
}

func validAttemptOutcome(outcome string) bool {
	switch outcome {
	case models.ProviderOperationAttemptOutcomeSucceeded,
		models.ProviderOperationAttemptOutcomeFailed,
		models.ProviderOperationAttemptOutcomeUnknown:
		return true
	default:
		return false
	}
}

func allowedOperationTransition(from, to string) bool {
	switch from {
	case models.ProviderOperationStatusPrepared:
		return to == models.ProviderOperationStatusExecuting || to == models.ProviderOperationStatusFailed
	case models.ProviderOperationStatusExecuting:
		return to == models.ProviderOperationStatusProviderSucceeded || to == models.ProviderOperationStatusFailed || to == models.ProviderOperationStatusOutcomeUnknown
	case models.ProviderOperationStatusOutcomeUnknown:
		return to == models.ProviderOperationStatusReconciling || to == models.ProviderOperationStatusReconciliationRequired || to == models.ProviderOperationStatusProviderSucceeded || to == models.ProviderOperationStatusFailed
	case models.ProviderOperationStatusReconciling:
		return to == models.ProviderOperationStatusProviderSucceeded || to == models.ProviderOperationStatusFailed || to == models.ProviderOperationStatusReconciliationRequired
	case models.ProviderOperationStatusProviderSucceeded:
		return to == models.ProviderOperationStatusFinalizing
	case models.ProviderOperationStatusFinalizing:
		return to == models.ProviderOperationStatusCompleted || to == models.ProviderOperationStatusFinalizeRetry
	case models.ProviderOperationStatusFinalizeRetry:
		return to == models.ProviderOperationStatusFinalizing
	case models.ProviderOperationStatusCompensationPrepared:
		return to == models.ProviderOperationStatusCompensating || to == models.ProviderOperationStatusFailed
	case models.ProviderOperationStatusCompensating:
		return to == models.ProviderOperationStatusCompensationSucceeded || to == models.ProviderOperationStatusCompensationRetry || to == models.ProviderOperationStatusOutcomeUnknown
	case models.ProviderOperationStatusCompensationRetry:
		return to == models.ProviderOperationStatusCompensating
	case models.ProviderOperationStatusCompensationSucceeded:
		return to == models.ProviderOperationStatusFinalizing || to == models.ProviderOperationStatusCompleted
	case models.ProviderOperationStatusFailed:
		return to == models.ProviderOperationStatusExecuting || to == models.ProviderOperationStatusCompensating
	case models.ProviderOperationStatusReconciliationRequired:
		return to == models.ProviderOperationStatusReconciling
	default:
		return false
	}
}

func normalizedJSONObject(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "{}"
	}
	return value
}
