package tax

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"ecommerce/models"
)

var ErrUnknownTaxProvider = fmt.Errorf("unknown tax provider")

type TaxProvider interface {
	QuoteTax(ctx context.Context, req QuoteTaxRequest) (models.Money, error)
	FinalizeTax(ctx context.Context, req FinalizeTaxRequest) (TaxFinalized, error)
	ExportReport(ctx context.Context, req ExportReportRequest) (io.ReadCloser, error)
}

type ProviderRegistry interface {
	Provider(providerID string) (TaxProvider, error)
}

type QuoteTaxRequest struct {
	Provider string
	Data     map[string]string
	Base     models.Money
}

type FinalizeTaxRequest struct {
	Provider          string
	Currency          string
	Data              map[string]string
	Items             []LineInput
	ShippingAmount    models.Money
	ExpectedTaxAmount models.Money
	InclusivePricing  bool
}

type LineInput struct {
	SnapshotItemID   *uint
	LineType         string
	ProductVariantID *uint
	Quantity         int
	Amount           models.Money
}

type TaxLine struct {
	SnapshotItemID     *uint
	LineType           string
	ProductVariantID   *uint
	Quantity           int
	Jurisdiction       string
	TaxCode            string
	TaxName            string
	TaxableAmount      models.Money
	TaxAmount          models.Money
	TaxRateBasisPoints int
	Inclusive          bool
}

type TaxFinalized struct {
	Provider         string
	Currency         string
	InclusivePricing bool
	TotalTax         models.Money
	Lines            []TaxLine
}

type ExportReportRequest struct {
	Provider string
	Lines    []models.OrderTaxLine
}

type DefaultProviderRegistry struct {
	providers map[string]TaxProvider
}

func NewDefaultProviderRegistry() *DefaultProviderRegistry {
	dummy := dummyTaxProvider{}
	return &DefaultProviderRegistry{
		providers: map[string]TaxProvider{
			"dummy-us-tax": dummy,
			"dummy-vat":    dummy,
		},
	}
}

func (r *DefaultProviderRegistry) Provider(providerID string) (TaxProvider, error) {
	if r == nil {
		return nil, ErrUnknownTaxProvider
	}
	provider, ok := r.providers[strings.TrimSpace(providerID)]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownTaxProvider, providerID)
	}
	return provider, nil
}

type dummyTaxProvider struct{}

func (dummyTaxProvider) QuoteTax(_ context.Context, req QuoteTaxRequest) (models.Money, error) {
	rate, _ := resolveRate(req.Provider, req.Data)
	return models.MoneyFromFloat(req.Base.Float64() * float64(rate) / 10000), nil
}

func (dummyTaxProvider) FinalizeTax(_ context.Context, req FinalizeTaxRequest) (TaxFinalized, error) {
	rateBps, jurisdiction := resolveRate(req.Provider, req.Data)
	lines := make([]TaxLine, 0, len(req.Items)+1)
	var totalTax models.Money
	for _, item := range req.Items {
		taxableAmount, taxAmount := calculateLineTax(item.Amount, rateBps, req.InclusivePricing)
		lines = append(lines, TaxLine{
			SnapshotItemID:     item.SnapshotItemID,
			LineType:           item.LineType,
			ProductVariantID:   item.ProductVariantID,
			Quantity:           item.Quantity,
			Jurisdiction:       jurisdiction,
			TaxCode:            defaultTaxCode(req.Provider, item.LineType),
			TaxName:            defaultTaxName(req.Provider),
			TaxableAmount:      taxableAmount,
			TaxAmount:          taxAmount,
			TaxRateBasisPoints: rateBps,
			Inclusive:          req.InclusivePricing,
		})
		totalTax += taxAmount
	}

	if req.ShippingAmount > 0 {
		taxableAmount, taxAmount := calculateLineTax(req.ShippingAmount, rateBps, req.InclusivePricing)
		lines = append(lines, TaxLine{
			LineType:           models.TaxLineTypeShipping,
			Quantity:           1,
			Jurisdiction:       jurisdiction,
			TaxCode:            defaultTaxCode(req.Provider, models.TaxLineTypeShipping),
			TaxName:            defaultTaxName(req.Provider),
			TaxableAmount:      taxableAmount,
			TaxAmount:          taxAmount,
			TaxRateBasisPoints: rateBps,
			Inclusive:          req.InclusivePricing,
		})
		totalTax += taxAmount
	}

	if len(lines) > 0 && !req.InclusivePricing && req.ExpectedTaxAmount >= 0 && totalTax != req.ExpectedTaxAmount {
		delta := req.ExpectedTaxAmount - totalTax
		lines[len(lines)-1].TaxAmount += delta
		totalTax += delta
	}

	return TaxFinalized{
		Provider:         req.Provider,
		Currency:         req.Currency,
		InclusivePricing: req.InclusivePricing,
		TotalTax:         totalTax,
		Lines:            lines,
	}, nil
}

func (dummyTaxProvider) ExportReport(_ context.Context, req ExportReportRequest) (io.ReadCloser, error) {
	builder := &strings.Builder{}
	writer := csv.NewWriter(builder)
	if err := writer.Write([]string{"order_id", "snapshot_id", "line_type", "jurisdiction", "tax_name", "tax_amount", "taxable_amount", "inclusive"}); err != nil {
		return nil, err
	}
	for _, line := range req.Lines {
		if err := writer.Write([]string{
			fmt.Sprintf("%d", line.OrderID),
			fmt.Sprintf("%d", line.SnapshotID),
			line.LineType,
			line.Jurisdiction,
			line.TaxName,
			line.TaxAmount.String(),
			line.TaxableAmount.String(),
			fmt.Sprintf("%t", line.Inclusive),
		}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return io.NopCloser(strings.NewReader(builder.String())), nil
}

func resolveRate(provider string, data map[string]string) (int, string) {
	if strings.EqualFold(strings.TrimSpace(data["tax_exempt"]), "true") {
		return 0, jurisdictionFromData(data)
	}

	switch strings.TrimSpace(provider) {
	case "dummy-us-tax":
		state := strings.ToUpper(strings.TrimSpace(data["state"]))
		switch state {
		case "CA":
			return 850, state
		case "NY":
			return 888, state
		case "TX":
			return 625, state
		default:
			if state == "" {
				state = "US"
			}
			return 500, state
		}
	case "dummy-vat":
		raw := strings.TrimSpace(data["vat_rate"])
		if raw == "" {
			return 2000, strings.ToUpper(strings.TrimSpace(data["vat_country"]))
		}
		var rate float64
		_, _ = fmt.Sscanf(raw, "%f", &rate)
		if rate < 0 {
			rate = 0
		}
		return int(rate * 100), strings.ToUpper(strings.TrimSpace(data["vat_country"]))
	default:
		return 0, jurisdictionFromData(data)
	}
}

func defaultTaxCode(provider string, lineType string) string {
	if lineType == models.TaxLineTypeShipping {
		return "shipping"
	}
	switch provider {
	case "dummy-vat":
		return "vat_goods"
	default:
		return "sales_goods"
	}
}

func defaultTaxName(provider string) string {
	switch provider {
	case "dummy-vat":
		return "VAT"
	default:
		return "Sales Tax"
	}
}

func jurisdictionFromData(data map[string]string) string {
	if state := strings.ToUpper(strings.TrimSpace(data["state"])); state != "" {
		return state
	}
	return strings.ToUpper(strings.TrimSpace(data["vat_country"]))
}

func calculateLineTax(amount models.Money, rateBps int, inclusive bool) (models.Money, models.Money) {
	if rateBps <= 0 || amount <= 0 {
		return amount, 0
	}
	rate := float64(rateBps) / 10000
	if !inclusive {
		return amount, models.MoneyFromFloat(amount.Float64() * rate)
	}
	taxableAmount := models.MoneyFromFloat(amount.Float64() / (1 + rate))
	return taxableAmount, amount - taxableAmount
}
