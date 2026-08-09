package providerops

import (
	"context"
	"errors"

	"ecommerce/models"

	"gorm.io/gorm"
)

type QueryService struct {
	db         *gorm.DB
	operations *OperationService
}

func NewQueryService(db *gorm.DB) *QueryService {
	return &QueryService{db: db, operations: NewOperationService(db)}
}

func (s *QueryService) ListOperations(ctx context.Context, input ListOperationsInput) ([]models.ProviderOperation, int64, error) {
	if s == nil || s.operations == nil {
		return nil, 0, errors.New("provider operation query service is not configured")
	}
	return s.operations.ListOperations(ctx, input)
}

func (s *QueryService) GetOperation(ctx context.Context, operationID uint) (models.ProviderOperation, error) {
	if s == nil || s.operations == nil {
		return models.ProviderOperation{}, errors.New("provider operation query service is not configured")
	}
	return s.operations.GetOperation(ctx, operationID)
}

func (s *QueryService) CompensationOperationID(ctx context.Context, operationID uint) (*uint, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("provider operation query service is not configured")
	}
	var child models.ProviderOperation
	err := s.db.WithContext(nonNilContext(ctx)).Select("id").
		Where("parent_operation_id = ?", operationID).
		Order("id ASC").First(&child).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &child.ID, nil
}
