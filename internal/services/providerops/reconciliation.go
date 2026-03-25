package providerops

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	paymentservice "ecommerce/internal/services/payments"
	shippingservice "ecommerce/internal/services/shipping"
	taxservice "ecommerce/internal/services/tax"
	"ecommerce/models"

	"gorm.io/gorm"
)

type ReconciliationService struct {
	db                *gorm.DB
	environment       string
	paymentProviders  paymentservice.ProviderRegistry
	shippingProviders shippingservice.ProviderRegistry
	taxProviders      taxservice.ProviderRegistry
}

type ReconciliationRunInput struct {
	ProviderType string
	ProviderID   string
	Trigger      string
}

type ScheduledRunSummary struct {
	RunCount int `json:"run_count"`
}

func NewReconciliationService(
	db *gorm.DB,
	environment string,
	paymentProviders paymentservice.ProviderRegistry,
	shippingProviders shippingservice.ProviderRegistry,
	taxProviders taxservice.ProviderRegistry,
) *ReconciliationService {
	return &ReconciliationService{
		db:                db,
		environment:       environment,
		paymentProviders:  paymentProviders,
		shippingProviders: shippingProviders,
		taxProviders:      taxProviders,
	}
}

func (s *ReconciliationService) Run(ctx context.Context, input ReconciliationRunInput) (models.ProviderReconciliationRun, []models.ProviderReconciliationDrift, error) {
	if s == nil || s.db == nil {
		return models.ProviderReconciliationRun{}, nil, fmt.Errorf("reconciliation service is not configured")
	}

	providerType, err := normalizeProviderType(input.ProviderType)
	if err != nil {
		return models.ProviderReconciliationRun{}, nil, err
	}
	providerID := strings.TrimSpace(input.ProviderID)
	if providerID == "" {
		return models.ProviderReconciliationRun{}, nil, fmt.Errorf("provider id is required")
	}
	trigger := strings.ToUpper(strings.TrimSpace(input.Trigger))
	if trigger == "" {
		trigger = models.ProviderReconciliationTriggerManual
	}

	run := models.ProviderReconciliationRun{
		ProviderType: providerType,
		ProviderID:   providerID,
		Environment:  s.environment,
		Trigger:      trigger,
		Status:       models.ProviderReconciliationStatusSucceeded,
		StartedAt:    time.Now().UTC(),
	}
	var drifts []models.ProviderReconciliationDrift
	checkedCount := 0

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&run).Error; err != nil {
			return err
		}

		switch providerType {
		case models.ProviderTypePayment:
			checkedCount, drifts, err = s.reconcilePayments(ctx, providerID)
		case models.ProviderTypeShipping:
			checkedCount, drifts, err = s.reconcileShipments(ctx, providerID)
		case models.ProviderTypeTax:
			checkedCount, drifts, err = s.reconcileTaxes(ctx, providerID)
		default:
			err = ErrInvalidProviderType
		}
		if err != nil {
			run.Status = models.ProviderReconciliationStatusFailed
			run.ErrorCount++
		}

		run.CheckedCount = checkedCount
		run.DriftCount = len(drifts)
		finishedAt := time.Now().UTC()
		run.FinishedAt = &finishedAt
		summary, summaryErr := json.Marshal(map[string]any{
			"provider_type": providerType,
			"provider_id":   providerID,
			"environment":   s.environment,
			"trigger":       trigger,
			"checked_count": run.CheckedCount,
			"drift_count":   run.DriftCount,
			"error_count":   run.ErrorCount,
		})
		if summaryErr != nil {
			return summaryErr
		}
		run.SummaryJSON = string(summary)

		for i := range drifts {
			drifts[i].RunID = run.ID
		}
		if len(drifts) > 0 {
			if err := tx.Create(&drifts).Error; err != nil {
				return err
			}
		}
		return tx.Model(&models.ProviderReconciliationRun{}).
			Where("id = ?", run.ID).
			Updates(map[string]any{
				"status":        run.Status,
				"checked_count": run.CheckedCount,
				"drift_count":   run.DriftCount,
				"error_count":   run.ErrorCount,
				"finished_at":   run.FinishedAt,
				"summary_json":  run.SummaryJSON,
			}).Error
	})
	if err != nil {
		return models.ProviderReconciliationRun{}, nil, err
	}

	if loadErr := s.db.WithContext(ctx).Preload("Drifts", func(db *gorm.DB) *gorm.DB {
		return db.Order("id ASC")
	}).First(&run, run.ID).Error; loadErr == nil {
		drifts = run.Drifts
	}
	if err != nil {
		return run, drifts, err
	}
	return run, drifts, nil
}

func (s *ReconciliationService) ListRuns(ctx context.Context, providerType, providerID string, page, limit int) ([]models.ProviderReconciliationRun, int64, error) {
	query := s.db.WithContext(ctx).Model(&models.ProviderReconciliationRun{})
	if strings.TrimSpace(providerType) != "" {
		normalized, err := normalizeProviderType(providerType)
		if err != nil {
			return nil, 0, err
		}
		query = query.Where("provider_type = ?", normalized)
	}
	if strings.TrimSpace(providerID) != "" {
		query = query.Where("provider_id = ?", strings.TrimSpace(providerID))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var runs []models.ProviderReconciliationRun
	if err := query.Order("started_at DESC, id DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&runs).Error; err != nil {
		return nil, 0, err
	}
	return runs, total, nil
}

func (s *ReconciliationService) GetRun(ctx context.Context, runID uint) (models.ProviderReconciliationRun, error) {
	var run models.ProviderReconciliationRun
	err := s.db.WithContext(ctx).
		Preload("Drifts", func(db *gorm.DB) *gorm.DB {
			return db.Order("id ASC")
		}).
		First(&run, runID).Error
	return run, err
}

func (s *ReconciliationService) RunScheduled(ctx context.Context) (ScheduledRunSummary, error) {
	targets, err := s.discoverTargets(ctx)
	if err != nil {
		return ScheduledRunSummary{}, err
	}
	slices.SortFunc(targets, func(a, b ReconciliationRunInput) int {
		if a.ProviderType != b.ProviderType {
			if a.ProviderType < b.ProviderType {
				return -1
			}
			return 1
		}
		if a.ProviderID < b.ProviderID {
			return -1
		}
		if a.ProviderID > b.ProviderID {
			return 1
		}
		return 0
	})

	runCount := 0
	for _, target := range targets {
		target.Trigger = models.ProviderReconciliationTriggerScheduled
		if _, _, err := s.Run(ctx, target); err != nil {
			return ScheduledRunSummary{RunCount: runCount}, err
		}
		runCount++
	}
	return ScheduledRunSummary{RunCount: runCount}, nil
}

func (s *ReconciliationService) discoverTargets(ctx context.Context) ([]ReconciliationRunInput, error) {
	seen := map[string]struct{}{}
	result := []ReconciliationRunInput{}

	appendTarget := func(providerType, providerID string) {
		key := providerType + "|" + providerID
		if _, ok := seen[key]; ok || strings.TrimSpace(providerID) == "" {
			return
		}
		seen[key] = struct{}{}
		result = append(result, ReconciliationRunInput{
			ProviderType: providerType,
			ProviderID:   providerID,
		})
	}

	var paymentProviders []string
	if err := s.db.WithContext(ctx).Model(&models.PaymentIntent{}).Distinct().Pluck("provider", &paymentProviders).Error; err != nil {
		return nil, err
	}
	for _, providerID := range paymentProviders {
		appendTarget(models.ProviderTypePayment, providerID)
	}

	var shippingProviders []string
	if err := s.db.WithContext(ctx).Model(&models.Shipment{}).Distinct().Pluck("provider", &shippingProviders).Error; err != nil {
		return nil, err
	}
	for _, providerID := range shippingProviders {
		appendTarget(models.ProviderTypeShipping, providerID)
	}

	var taxProviders []string
	if err := s.db.WithContext(ctx).Model(&models.OrderTaxLine{}).Distinct().Pluck("tax_provider_id", &taxProviders).Error; err != nil {
		return nil, err
	}
	for _, providerID := range taxProviders {
		appendTarget(models.ProviderTypeTax, providerID)
	}

	return result, nil
}

func (s *ReconciliationService) reconcilePayments(ctx context.Context, providerID string) (int, []models.ProviderReconciliationDrift, error) {
	provider, err := s.paymentProviders.Provider(providerID)
	if err != nil {
		return 0, nil, err
	}
	lookupProvider, ok := provider.(paymentservice.TransactionLookupProvider)
	if !ok {
		return 0, nil, fmt.Errorf("payment provider %s does not support reconciliation lookup", providerID)
	}

	var intents []models.PaymentIntent
	if err := s.db.WithContext(ctx).
		Where("provider = ?", providerID).
		Preload("Transactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC, id ASC")
		}).
		Find(&intents).Error; err != nil {
		return 0, nil, err
	}

	drifts := []models.ProviderReconciliationDrift{}
	checkedCount := 0
	for _, intent := range intents {
		for _, txn := range intent.Transactions {
			checkedCount++
			truth, lookupErr := lookupProvider.GetTransaction(ctx, txn.ProviderTxnID)
			if lookupErr != nil {
				drifts = append(drifts, newDrift("payment_transaction", txn.ID, txn.ProviderTxnID, "lookup", "", "", lookupErr.Error()))
				continue
			}
			if truth.Operation != txn.Operation {
				drifts = append(drifts, newDrift("payment_transaction", txn.ID, txn.ProviderTxnID, "operation", truth.Operation, txn.Operation, "payment operation drift"))
			}
			if truth.Status != txn.Status {
				drifts = append(drifts, newDrift("payment_transaction", txn.ID, txn.ProviderTxnID, "status", truth.Status, txn.Status, "payment status drift"))
			}
			if truth.Amount != txn.Amount {
				drifts = append(drifts, newDrift("payment_transaction", txn.ID, txn.ProviderTxnID, "amount", truth.Amount.String(), txn.Amount.String(), "payment amount drift"))
			}
			if truth.Currency != "" && truth.Currency != intent.Currency {
				drifts = append(drifts, newDrift("payment_transaction", txn.ID, txn.ProviderTxnID, "currency", truth.Currency, intent.Currency, "payment currency drift"))
			}
		}
	}
	return checkedCount, drifts, nil
}

func (s *ReconciliationService) reconcileShipments(ctx context.Context, providerID string) (int, []models.ProviderReconciliationDrift, error) {
	provider, err := s.shippingProviders.Provider(providerID)
	if err != nil {
		return 0, nil, err
	}
	lookupProvider, ok := provider.(shippingservice.ShipmentLookupProvider)
	if !ok {
		return 0, nil, fmt.Errorf("shipping provider %s does not support reconciliation lookup", providerID)
	}

	var shipments []models.Shipment
	if err := s.db.WithContext(ctx).Where("provider = ?", providerID).Order("id ASC").Find(&shipments).Error; err != nil {
		return 0, nil, err
	}

	drifts := []models.ProviderReconciliationDrift{}
	checkedCount := 0
	for _, shipment := range shipments {
		checkedCount++
		truth, lookupErr := lookupProvider.GetShipment(ctx, shipment.ProviderShipmentID)
		if lookupErr != nil {
			drifts = append(drifts, newDrift("shipment", shipment.ID, shipment.ProviderShipmentID, "lookup", "", "", lookupErr.Error()))
			continue
		}
		if truth.Status != shipment.Status {
			drifts = append(drifts, newDrift("shipment", shipment.ID, shipment.ProviderShipmentID, "status", truth.Status, shipment.Status, "shipment status drift"))
		}
		if truth.ServiceCode != "" && truth.ServiceCode != shipment.ServiceCode {
			drifts = append(drifts, newDrift("shipment", shipment.ID, shipment.ProviderShipmentID, "service_code", truth.ServiceCode, shipment.ServiceCode, "shipment service drift"))
		}
		if truth.TrackingNumber != "" && truth.TrackingNumber != shipment.TrackingNumber {
			drifts = append(drifts, newDrift("shipment", shipment.ID, shipment.ProviderShipmentID, "tracking_number", truth.TrackingNumber, shipment.TrackingNumber, "shipment tracking drift"))
		}
	}
	return checkedCount, drifts, nil
}

func (s *ReconciliationService) reconcileTaxes(ctx context.Context, providerID string) (int, []models.ProviderReconciliationDrift, error) {
	var lines []models.OrderTaxLine
	if err := s.db.WithContext(ctx).
		Where("tax_provider_id = ?", providerID).
		Order("snapshot_id ASC, id ASC").
		Find(&lines).Error; err != nil {
		return 0, nil, err
	}

	grouped := map[uint][]models.OrderTaxLine{}
	for _, line := range lines {
		grouped[line.SnapshotID] = append(grouped[line.SnapshotID], line)
	}

	drifts := []models.ProviderReconciliationDrift{}
	checkedCount := 0
	for snapshotID, snapshotLines := range grouped {
		checkedCount++
		var snapshot models.OrderCheckoutSnapshot
		if err := s.db.WithContext(ctx).Preload("Items").First(&snapshot, snapshotID).Error; err != nil {
			drifts = append(drifts, newDrift("tax_snapshot", snapshotID, fmt.Sprintf("%d", snapshotID), "snapshot", "", "", err.Error()))
			continue
		}
		orderID := uint(0)
		if snapshot.OrderID != nil {
			orderID = *snapshot.OrderID
		}
		expected, err := taxservice.ComputeTaxFinalization(ctx, s.db, s.taxProviders, taxservice.FinalizeInput{
			Order:            models.Order{BaseModel: models.BaseModel{ID: orderID}},
			Snapshot:         snapshot,
			InclusivePricing: inclusivePricingPointer(snapshotLines),
		})
		if err != nil {
			drifts = append(drifts, newDrift("tax_snapshot", snapshotID, fmt.Sprintf("%d", snapshotID), "lookup", "", "", err.Error()))
			continue
		}

		var actualTotal models.Money
		for _, line := range snapshotLines {
			actualTotal += line.TaxAmount
		}
		if expected.TotalTax != actualTotal {
			drifts = append(drifts, newDrift("tax_snapshot", snapshotID, fmt.Sprintf("%d", snapshotID), "total_tax", expected.TotalTax.String(), actualTotal.String(), "tax total drift"))
		}

		expectedLines := comparableTaxLines(expected.Lines)
		actualLines := comparableStoredTaxLines(snapshotLines)
		if len(expectedLines) != len(actualLines) {
			drifts = append(drifts, newDrift("tax_snapshot", snapshotID, fmt.Sprintf("%d", snapshotID), "line_count", fmt.Sprintf("%d", len(expectedLines)), fmt.Sprintf("%d", len(actualLines)), "tax line count drift"))
			continue
		}
		for i := range expectedLines {
			if expectedLines[i] != actualLines[i] {
				drifts = append(drifts, newDrift("tax_snapshot", snapshotID, fmt.Sprintf("%d", snapshotID), fmt.Sprintf("line_%d", i), expectedLines[i], actualLines[i], "tax line drift"))
			}
		}
	}

	return checkedCount, drifts, nil
}

func comparableTaxLines(lines []taxservice.TaxLine) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		result = append(result, fmt.Sprintf(
			"%d|%s|%s|%s|%s|%s|%d|%s|%s|%d|%t",
			derefUint(line.SnapshotItemID),
			line.LineType,
			derefUintString(line.ProductVariantID),
			line.Jurisdiction,
			line.TaxCode,
			line.TaxName,
			line.Quantity,
			line.TaxableAmount.String(),
			line.TaxAmount.String(),
			line.TaxRateBasisPoints,
			line.Inclusive,
		))
	}
	slices.Sort(result)
	return result
}

func comparableStoredTaxLines(lines []models.OrderTaxLine) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		result = append(result, fmt.Sprintf(
			"%d|%s|%s|%s|%s|%s|%d|%s|%s|%d|%t",
			derefUint(line.SnapshotItemID),
			line.LineType,
			derefUintString(line.ProductVariantID),
			line.Jurisdiction,
			line.TaxCode,
			line.TaxName,
			line.Quantity,
			line.TaxableAmount.String(),
			line.TaxAmount.String(),
			line.TaxRateBasisPoints,
			line.Inclusive,
		))
	}
	slices.Sort(result)
	return result
}

func inclusivePricingPointer(lines []models.OrderTaxLine) *bool {
	for _, line := range lines {
		if line.Inclusive {
			value := true
			return &value
		}
	}
	value := false
	return &value
}

func newDrift(entityType string, entityID uint, providerReference string, fieldName string, expected string, actual string, message string) models.ProviderReconciliationDrift {
	return models.ProviderReconciliationDrift{
		EntityType:        entityType,
		EntityID:          entityID,
		ProviderReference: providerReference,
		Severity:          models.ProviderDriftSeverityError,
		FieldName:         fieldName,
		ExpectedValue:     expected,
		ActualValue:       actual,
		Message:           message,
	}
}

func derefUint(value *uint) uint {
	if value == nil {
		return 0
	}
	return *value
}

func derefUintString(value *uint) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%d", *value)
}
