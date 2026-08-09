package providerops

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrProviderReconciliationCaseNotFound      = errors.New("provider reconciliation case not found")
	ErrProviderReconciliationCaseConflict      = errors.New("provider reconciliation case is already resolved")
	ErrInvalidProviderReconciliationCaseUpdate = errors.New("invalid provider reconciliation case update")
)

type ListReconciliationCasesInput struct {
	Status       string
	ProviderType string
	ProviderID   string
	CaseType     string
	Page         int
	Limit        int
}

type UpdateReconciliationCaseInput struct {
	AssignedTo     *string
	Status         *string
	Outcome        *string
	ResolutionNote *string
}

type ReconciliationCaseRecord struct {
	Case           models.ProviderReconciliationCase
	Operation      models.ProviderOperation
	AssignedTo     string
	ResolutionNote string
}

type CaseService struct {
	db *gorm.DB
}

func NewCaseService(db *gorm.DB) *CaseService {
	return &CaseService{db: db}
}

func (s *CaseService) List(ctx context.Context, input ListReconciliationCasesInput) ([]ReconciliationCaseRecord, int64, error) {
	if s == nil || s.db == nil {
		return nil, 0, errors.New("provider reconciliation case service is not configured")
	}
	query := s.db.WithContext(nonNilContext(ctx)).
		Table("provider_reconciliation_cases AS cases").
		Joins("JOIN provider_operations AS operations ON operations.id = cases.provider_operation_id")
	if status := strings.ToUpper(strings.TrimSpace(input.Status)); status != "" {
		if status != models.ProviderReconciliationCaseStatusOpen && status != models.ProviderReconciliationCaseStatusResolved {
			return nil, 0, ErrInvalidProviderReconciliationCaseUpdate
		}
		query = query.Where("cases.status = ?", status)
	}
	if strings.TrimSpace(input.ProviderType) != "" {
		providerType, err := normalizeProviderType(input.ProviderType)
		if err != nil {
			return nil, 0, err
		}
		query = query.Where("operations.provider_type = ?", providerType)
	}
	if providerID := strings.TrimSpace(input.ProviderID); providerID != "" {
		query = query.Where("operations.provider_id = ?", providerID)
	}
	if caseType := strings.TrimSpace(input.CaseType); caseType != "" {
		query = query.Where("cases.case_type = ?", caseType)
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
	if limit > 100 {
		limit = 100
	}
	var ids []uint
	if err := query.Select("cases.id").Order("cases.opened_at DESC, cases.id DESC").
		Offset((page - 1) * limit).Limit(limit).Scan(&ids).Error; err != nil {
		return nil, 0, err
	}
	if len(ids) == 0 {
		return []ReconciliationCaseRecord{}, total, nil
	}

	var cases []models.ProviderReconciliationCase
	if err := s.db.WithContext(nonNilContext(ctx)).Where("id IN ?", ids).Find(&cases).Error; err != nil {
		return nil, 0, err
	}
	caseByID := make(map[uint]models.ProviderReconciliationCase, len(cases))
	operationIDs := make([]uint, 0, len(cases))
	for _, reconciliationCase := range cases {
		caseByID[reconciliationCase.ID] = reconciliationCase
		operationIDs = append(operationIDs, reconciliationCase.ProviderOperationID)
	}
	var operations []models.ProviderOperation
	if err := s.db.WithContext(nonNilContext(ctx)).Where("id IN ?", operationIDs).Find(&operations).Error; err != nil {
		return nil, 0, err
	}
	operationByID := make(map[uint]models.ProviderOperation, len(operations))
	for _, operation := range operations {
		operationByID[operation.ID] = operation
	}

	records := make([]ReconciliationCaseRecord, 0, len(ids))
	for _, id := range ids {
		reconciliationCase, ok := caseByID[id]
		if !ok {
			continue
		}
		records = append(records, buildReconciliationCaseRecord(reconciliationCase, operationByID[reconciliationCase.ProviderOperationID]))
	}
	return records, total, nil
}

func (s *CaseService) Get(ctx context.Context, caseID uint) (ReconciliationCaseRecord, error) {
	if s == nil || s.db == nil {
		return ReconciliationCaseRecord{}, errors.New("provider reconciliation case service is not configured")
	}
	var reconciliationCase models.ProviderReconciliationCase
	if err := s.db.WithContext(nonNilContext(ctx)).First(&reconciliationCase, caseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ReconciliationCaseRecord{}, ErrProviderReconciliationCaseNotFound
		}
		return ReconciliationCaseRecord{}, err
	}
	var operation models.ProviderOperation
	if err := s.db.WithContext(nonNilContext(ctx)).First(&operation, reconciliationCase.ProviderOperationID).Error; err != nil {
		return ReconciliationCaseRecord{}, err
	}
	return buildReconciliationCaseRecord(reconciliationCase, operation), nil
}

func (s *CaseService) Update(ctx context.Context, caseID uint, input UpdateReconciliationCaseInput) (ReconciliationCaseRecord, error) {
	if s == nil || s.db == nil {
		return ReconciliationCaseRecord{}, errors.New("provider reconciliation case service is not configured")
	}
	if input.AssignedTo == nil && input.Status == nil && input.Outcome == nil && input.ResolutionNote == nil {
		return ReconciliationCaseRecord{}, ErrInvalidProviderReconciliationCaseUpdate
	}

	err := s.db.WithContext(nonNilContext(ctx)).Transaction(func(tx *gorm.DB) error {
		var reconciliationCase models.ProviderReconciliationCase
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&reconciliationCase, caseID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrProviderReconciliationCaseNotFound
			}
			return err
		}
		if reconciliationCase.Status == models.ProviderReconciliationCaseStatusResolved {
			return ErrProviderReconciliationCaseConflict
		}

		details, err := decodeJSONObject(reconciliationCase.DetailsJSON)
		if err != nil {
			return err
		}
		updates := map[string]any{}
		if input.AssignedTo != nil {
			assignedTo := strings.TrimSpace(*input.AssignedTo)
			if assignedTo == "" {
				delete(details, "assigned_to")
			} else {
				details["assigned_to"] = assignedTo
			}
			encoded, err := json.Marshal(details)
			if err != nil {
				return err
			}
			updates["details_json"] = string(encoded)
		}

		if input.Status != nil {
			status := strings.ToUpper(strings.TrimSpace(*input.Status))
			if status != models.ProviderReconciliationCaseStatusResolved || input.Outcome == nil || input.ResolutionNote == nil {
				return ErrInvalidProviderReconciliationCaseUpdate
			}
			outcome := strings.ToUpper(strings.TrimSpace(*input.Outcome))
			if !validReconciliationCaseOutcome(outcome) || strings.TrimSpace(*input.ResolutionNote) == "" {
				return ErrInvalidProviderReconciliationCaseUpdate
			}
			resolution, err := decodeJSONObject(reconciliationCase.ResolutionJSON)
			if err != nil {
				return err
			}
			resolution["resolution_note"] = strings.TrimSpace(*input.ResolutionNote)
			if assignedTo, ok := details["assigned_to"].(string); ok && assignedTo != "" {
				resolution["assigned_to"] = assignedTo
			}
			encoded, err := json.Marshal(resolution)
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			updates["status"] = status
			updates["outcome"] = outcome
			updates["resolution_json"] = string(encoded)
			updates["resolved_at"] = &now
			updates["open_key"] = nil
			updates["next_attempt_at"] = nil
		} else if input.Outcome != nil || input.ResolutionNote != nil {
			return ErrInvalidProviderReconciliationCaseUpdate
		}
		if len(updates) == 0 {
			return ErrInvalidProviderReconciliationCaseUpdate
		}
		return tx.Model(&models.ProviderReconciliationCase{}).
			Where("id = ? AND status = ?", caseID, models.ProviderReconciliationCaseStatusOpen).
			Updates(updates).Error
	})
	if err != nil {
		return ReconciliationCaseRecord{}, err
	}
	return s.Get(ctx, caseID)
}

func DescribeReconciliationCase(reconciliationCase models.ProviderReconciliationCase, operation models.ProviderOperation) ReconciliationCaseRecord {
	return buildReconciliationCaseRecord(reconciliationCase, operation)
}

func buildReconciliationCaseRecord(reconciliationCase models.ProviderReconciliationCase, operation models.ProviderOperation) ReconciliationCaseRecord {
	details, _ := decodeJSONObject(reconciliationCase.DetailsJSON)
	resolution, _ := decodeJSONObject(reconciliationCase.ResolutionJSON)
	assignedTo, _ := details["assigned_to"].(string)
	resolutionNote, _ := resolution["resolution_note"].(string)
	return ReconciliationCaseRecord{
		Case:           reconciliationCase,
		Operation:      operation,
		AssignedTo:     assignedTo,
		ResolutionNote: resolutionNote,
	}
}

func decodeJSONObject(raw string) (map[string]any, error) {
	value := map[string]any{}
	if strings.TrimSpace(raw) == "" {
		return value, nil
	}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, err
	}
	return value, nil
}

func validReconciliationCaseOutcome(outcome string) bool {
	switch outcome {
	case models.ProviderReconciliationCaseOutcomeConfirmedSucceeded,
		models.ProviderReconciliationCaseOutcomeConfirmedFailed,
		models.ProviderReconciliationCaseOutcomeRetryRequired,
		models.ProviderReconciliationCaseOutcomeManualReview:
		return true
	default:
		return false
	}
}
