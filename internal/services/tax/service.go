package tax

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"ecommerce/internal/dbcontext"
	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrTaxLinesNotFound = errors.New("tax lines not found")

type FinalizeInput struct {
	Order            models.Order
	Snapshot         models.OrderCheckoutSnapshot
	InclusivePricing *bool
	IdempotencyKey   string
	OperationKey     string
	CorrelationID    string
}

type TaxFinalizationPlan struct {
	Input            FinalizeInput
	Request          FinalizeTaxRequest
	StoredResult     TaxFinalized
	AlreadyFinalized bool
}

func PrepareTaxFinalization(db *gorm.DB, input FinalizeInput) (TaxFinalizationPlan, error) {
	if strings.TrimSpace(input.Snapshot.TaxProviderID) == "" {
		return TaxFinalizationPlan{}, fmt.Errorf("snapshot tax provider is required")
	}
	var existing []models.OrderTaxLine
	if err := db.Where("order_id = ? AND snapshot_id = ?", input.Order.ID, input.Snapshot.ID).Order("id ASC").Find(&existing).Error; err != nil {
		return TaxFinalizationPlan{}, err
	}
	if len(existing) > 0 {
		return TaxFinalizationPlan{Input: input, StoredResult: fromStoredLines(existing, input.Snapshot.TaxProviderID, input.Snapshot.Currency), AlreadyFinalized: true}, nil
	}

	request, err := BuildTaxFinalizationRequest(db, input)
	if err != nil {
		return TaxFinalizationPlan{}, err
	}
	return TaxFinalizationPlan{Input: input, Request: request}, nil
}

func BuildTaxFinalizationRequest(db *gorm.DB, input FinalizeInput) (FinalizeTaxRequest, error) {
	taxData, err := unmarshalStringMap(input.Snapshot.TaxDataJSON)
	if err != nil {
		return FinalizeTaxRequest{}, err
	}
	shippingData, err := unmarshalStringMap(input.Snapshot.ShippingDataJSON)
	if err != nil {
		return FinalizeTaxRequest{}, err
	}
	data := mergeTaxData(taxData, shippingData)
	config, err := resolveNexusConfig(db, input.Snapshot.TaxProviderID, data)
	if err != nil {
		return FinalizeTaxRequest{}, err
	}
	if config != nil && strings.TrimSpace(data["tax_exempt"]) == "" && strings.TrimSpace(config.ExemptionCode) != "" {
		data["tax_exempt"] = "true"
	}

	inclusivePricing := false
	if input.InclusivePricing != nil {
		inclusivePricing = *input.InclusivePricing
	} else if config != nil {
		inclusivePricing = config.InclusivePricing
	}
	items := make([]LineInput, 0, len(input.Snapshot.Items))
	for _, item := range input.Snapshot.Items {
		snapshotItemID, productVariantID := item.ID, item.ProductVariantID
		items = append(items, LineInput{SnapshotItemID: &snapshotItemID, LineType: models.TaxLineTypeItem, ProductVariantID: &productVariantID, Quantity: item.Quantity, Amount: item.Price.Mul(item.Quantity)})
	}
	operationKey := strings.TrimSpace(input.OperationKey)
	if operationKey == "" {
		operationKey = fmt.Sprintf("tax-finalize-snapshot-%d", input.Snapshot.ID)
	}
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = operationKey
	}
	return FinalizeTaxRequest{
		Provider: input.Snapshot.TaxProviderID, Currency: input.Snapshot.Currency, IdempotencyKey: idempotencyKey,
		OperationKey: operationKey, CorrelationID: input.CorrelationID, Data: data, Items: items,
		ShippingAmount: input.Snapshot.ShippingAmount, ExpectedTaxAmount: input.Snapshot.TaxAmount, InclusivePricing: inclusivePricing,
	}, nil
}

func PersistTaxFinalization(db *gorm.DB, input FinalizeInput, result TaxFinalized, now time.Time) (TaxFinalized, error) {
	var stored TaxFinalized
	err := db.Transaction(func(tx *gorm.DB) error {
		var snapshot models.OrderCheckoutSnapshot
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").First(&snapshot, input.Snapshot.ID).Error; err != nil {
			return err
		}
		var existing []models.OrderTaxLine
		if err := tx.Where("order_id = ? AND snapshot_id = ?", input.Order.ID, input.Snapshot.ID).Order("id ASC").Find(&existing).Error; err != nil {
			return err
		}
		if len(existing) > 0 {
			stored = fromStoredLines(existing, input.Snapshot.TaxProviderID, input.Snapshot.Currency)
			return nil
		}
		timestamp := now.UTC()
		records := make([]models.OrderTaxLine, 0, len(result.Lines))
		for _, line := range result.Lines {
			records = append(records, models.OrderTaxLine{
				OrderID: input.Order.ID, SnapshotID: input.Snapshot.ID, SnapshotItemID: line.SnapshotItemID, LineType: line.LineType,
				TaxProviderID: input.Snapshot.TaxProviderID, ProductVariantID: line.ProductVariantID, Jurisdiction: line.Jurisdiction,
				TaxCode: line.TaxCode, TaxName: line.TaxName, Quantity: line.Quantity, TaxableAmount: line.TaxableAmount,
				TaxAmount: line.TaxAmount, TaxRateBasisPoints: line.TaxRateBasisPoints, Inclusive: line.Inclusive, FinalizedAt: timestamp,
			})
		}
		if len(records) > 0 {
			if err := tx.Create(&records).Error; err != nil {
				return err
			}
		}
		stored = result
		return nil
	})
	return stored, err
}

func FinalizeOrderTax(ctx context.Context, db *gorm.DB, registry ProviderRegistry, input FinalizeInput) (TaxFinalized, error) {
	plan, err := PrepareTaxFinalization(db, input)
	if err != nil || plan.AlreadyFinalized {
		return plan.StoredResult, err
	}
	provider, err := registry.Provider(plan.Request.Provider)
	if err != nil {
		return TaxFinalized{}, err
	}
	result, err := provider.FinalizeTax(ctx, plan.Request)
	if err != nil {
		return TaxFinalized{}, err
	}
	return PersistTaxFinalization(db, input, result, time.Now().UTC())
}

func ComputeTaxFinalization(ctx context.Context, db *gorm.DB, registry ProviderRegistry, input FinalizeInput) (TaxFinalized, error) {
	request, err := BuildTaxFinalizationRequest(db, input)
	if err != nil {
		return TaxFinalized{}, err
	}
	provider, err := registry.Provider(request.Provider)
	if err != nil {
		return TaxFinalized{}, err
	}
	return provider.FinalizeTax(ctx, request)
}

func LoadOrderTaxLines(db *gorm.DB, orderID uint) ([]models.OrderTaxLine, error) {
	var lines []models.OrderTaxLine
	if err := db.Where("order_id = ?", orderID).Order("snapshot_id ASC, id ASC").Find(&lines).Error; err != nil {
		return nil, err
	}
	return lines, nil
}

type ExportInput struct {
	Provider string
	Start    *time.Time
	End      *time.Time
	Format   string
}

func ExportOrderTaxes(
	ctx context.Context,
	tx *gorm.DB,
	registry ProviderRegistry,
	input ExportInput,
) (models.TaxExport, io.ReadCloser, error) {
	ctx = dbcontext.WithDB(ctx, tx)

	query := tx.Model(&models.OrderTaxLine{})
	if provider := strings.TrimSpace(input.Provider); provider != "" {
		query = query.Where("tax_provider_id = ?", provider)
	}
	if input.Start != nil {
		query = query.Where("finalized_at >= ?", input.Start.UTC())
	}
	if input.End != nil {
		query = query.Where("finalized_at <= ?", input.End.UTC())
	}

	var lines []models.OrderTaxLine
	if err := query.Order("finalized_at ASC, id ASC").Find(&lines).Error; err != nil {
		return models.TaxExport{}, nil, err
	}

	providerID := exportProviderID(lines, input.Provider)

	provider, err := registry.Provider(providerID)
	if err != nil && providerID != "all" {
		return models.TaxExport{}, nil, err
	}

	var report io.ReadCloser
	if providerID == "all" {
		provider = dummyTaxProvider{}
	}
	report, err = provider.ExportReport(ctx, ExportReportRequest{
		Provider: providerID,
		Lines:    lines,
	})
	if err != nil {
		return models.TaxExport{}, nil, err
	}

	content, err := io.ReadAll(report)
	if err != nil {
		_ = report.Close()
		return models.TaxExport{}, nil, err
	}
	_ = report.Close()

	filters := map[string]any{
		"provider": providerID,
		"format":   strings.TrimSpace(input.Format),
	}
	if input.Start != nil {
		filters["start"] = input.Start.UTC().Format(time.RFC3339)
	}
	if input.End != nil {
		filters["end"] = input.End.UTC().Format(time.RFC3339)
	}
	filtersJSON, err := json.Marshal(filters)
	if err != nil {
		return models.TaxExport{}, nil, err
	}

	record := models.TaxExport{
		Provider:    providerID,
		Format:      firstNonEmpty(strings.TrimSpace(input.Format), "csv"),
		FiltersJSON: string(filtersJSON),
		RowCount:    len(lines),
		Contents:    string(content),
		ExportedAt:  time.Now().UTC(),
	}
	if err := tx.Create(&record).Error; err != nil {
		return models.TaxExport{}, nil, err
	}

	return record, io.NopCloser(strings.NewReader(string(content))), nil
}

func exportProviderID(lines []models.OrderTaxLine, requestedProvider string) string {
	providerID := strings.TrimSpace(requestedProvider)
	if providerID != "" {
		return providerID
	}
	if len(lines) == 0 {
		return "all"
	}

	seen := map[string]struct{}{}
	for _, line := range lines {
		candidate := strings.TrimSpace(line.TaxProviderID)
		if candidate == "" {
			continue
		}
		seen[candidate] = struct{}{}
		if len(seen) > 1 {
			return "all"
		}
		providerID = candidate
	}
	if providerID == "" {
		return "all"
	}
	return providerID
}

func resolveNexusConfig(tx *gorm.DB, provider string, data map[string]string) (*models.TaxNexusConfig, error) {
	country := strings.ToUpper(strings.TrimSpace(data["country"]))
	if country == "" {
		country = strings.ToUpper(strings.TrimSpace(data["vat_country"]))
	}
	if country == "" {
		country = "US"
	}
	state := strings.ToUpper(strings.TrimSpace(data["state"]))

	var config models.TaxNexusConfig
	err := tx.Where("provider = ? AND country = ? AND state = ? AND active = ?", provider, country, state, true).
		First(&config).Error
	if err == nil {
		return &config, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if state == "" {
		return nil, nil
	}

	err = tx.Where("provider = ? AND country = ? AND state = '' AND active = ?", provider, country, true).
		First(&config).Error
	if err == nil {
		return &config, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return nil, err
}

func fromStoredLines(lines []models.OrderTaxLine, provider string, currency string) TaxFinalized {
	result := TaxFinalized{
		Provider: provider,
		Currency: currency,
		Lines:    make([]TaxLine, 0, len(lines)),
	}
	for _, line := range lines {
		result.TotalTax += line.TaxAmount
		result.Lines = append(result.Lines, TaxLine{
			SnapshotItemID:     line.SnapshotItemID,
			LineType:           line.LineType,
			ProductVariantID:   line.ProductVariantID,
			Quantity:           line.Quantity,
			Jurisdiction:       line.Jurisdiction,
			TaxCode:            line.TaxCode,
			TaxName:            line.TaxName,
			TaxableAmount:      line.TaxableAmount,
			TaxAmount:          line.TaxAmount,
			TaxRateBasisPoints: line.TaxRateBasisPoints,
			Inclusive:          line.Inclusive,
		})
		if line.Inclusive {
			result.InclusivePricing = true
		}
	}
	return result
}

func mergeTaxData(taxData map[string]string, shippingData map[string]string) map[string]string {
	merged := make(map[string]string, len(taxData)+len(shippingData)+1)
	for key, value := range taxData {
		merged[key] = value
	}
	if strings.TrimSpace(merged["state"]) == "" && strings.TrimSpace(shippingData["state"]) != "" {
		merged["state"] = strings.ToUpper(strings.TrimSpace(shippingData["state"]))
	}
	if strings.TrimSpace(merged["postal_code"]) == "" && strings.TrimSpace(shippingData["postal_code"]) != "" {
		merged["postal_code"] = strings.TrimSpace(shippingData["postal_code"])
	}
	if strings.TrimSpace(merged["country"]) == "" && strings.TrimSpace(shippingData["country"]) != "" {
		merged["country"] = strings.ToUpper(strings.TrimSpace(shippingData["country"]))
	}
	return merged
}

func unmarshalStringMap(value string) (map[string]string, error) {
	if value == "" {
		return map[string]string{}, nil
	}
	var data map[string]string
	if err := json.Unmarshal([]byte(value), &data); err != nil {
		return nil, err
	}
	if data == nil {
		data = map[string]string{}
	}
	return data, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
