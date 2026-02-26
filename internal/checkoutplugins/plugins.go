package checkoutplugins

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ecommerce/models"
)

type ProviderType string

const (
	ProviderTypePayment  ProviderType = "payment"
	ProviderTypeShipping ProviderType = "shipping"
	ProviderTypeTax      ProviderType = "tax"
)

type FieldType string

const (
	FieldTypeText     FieldType = "text"
	FieldTypeNumber   FieldType = "number"
	FieldTypeCheckbox FieldType = "checkbox"
	FieldTypeSelect   FieldType = "select"
)

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeveritySuccess Severity = "success"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

type FieldOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type FieldDefinition struct {
	Key         string        `json:"key"`
	Label       string        `json:"label"`
	Type        FieldType     `json:"type"`
	Required    bool          `json:"required"`
	Placeholder string        `json:"placeholder,omitempty"`
	HelpText    string        `json:"help_text,omitempty"`
	Options     []FieldOption `json:"options,omitempty"`
}

type State struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

type Definition struct {
	ID          string            `json:"id"`
	Type        ProviderType      `json:"type"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Status      string            `json:"status"`
	Enabled     bool              `json:"enabled"`
	Fields      []FieldDefinition `json:"fields"`
	States      []State           `json:"states"`
}

type QuoteRequest struct {
	Subtotal     models.Money
	PaymentID    string
	ShippingID   string
	TaxID        string
	PaymentData  map[string]string
	ShippingData map[string]string
	TaxData      map[string]string
}

type QuoteResult struct {
	Currency       string
	Subtotal       models.Money
	Shipping       models.Money
	Tax            models.Money
	Total          models.Money
	PaymentStates  []State
	ShippingStates []State
	TaxStates      []State
	Valid          bool
}

type CheckoutDetails struct {
	PaymentDisplay  string
	ShippingAddress string
}

type ProviderSetting struct {
	Type    ProviderType
	ID      string
	Enabled bool
}

type Manager struct {
	mu sync.RWMutex

	payments  map[string]Definition
	shippings map[string]Definition
	taxes     map[string]Definition

	externalPayments  map[string]*externalProvider
	externalShippings map[string]*externalProvider
	externalTaxes     map[string]*externalProvider

	enabled map[string]bool
}

func NewDefaultManager() *Manager {
	m := &Manager{
		payments:  make(map[string]Definition),
		shippings: make(map[string]Definition),
		taxes:     make(map[string]Definition),

		externalPayments:  make(map[string]*externalProvider),
		externalShippings: make(map[string]*externalProvider),
		externalTaxes:     make(map[string]*externalProvider),

		enabled: make(map[string]bool),
	}

	m.registerPayment(Definition{
		ID:          "dummy-card",
		Type:        ProviderTypePayment,
		Name:        "Dummy Card Gateway",
		Description: "Simulates card-based authorization with test outcomes.",
		Status:      "available",
		Fields: []FieldDefinition{
			{Key: "cardholder_name", Label: "Cardholder name", Type: FieldTypeText, Required: true, Placeholder: "Alex Merchant"},
			{Key: "card_number", Label: "Card number", Type: FieldTypeText, Required: true, Placeholder: "4242424242424242", HelpText: "Use a number ending in 0000 to simulate a decline."},
			{Key: "exp_month", Label: "Exp month", Type: FieldTypeNumber, Required: true, Placeholder: "12"},
			{Key: "exp_year", Label: "Exp year", Type: FieldTypeNumber, Required: true, Placeholder: strconv.Itoa(time.Now().Year() + 1)},
		},
		States: []State{{Code: "sandbox_mode", Severity: SeverityInfo, Message: "Sandbox only. No real payment capture."}},
	})

	m.registerPayment(Definition{
		ID:          "dummy-wallet",
		Type:        ProviderTypePayment,
		Name:        "Dummy Wallet",
		Description: "Simulates redirect wallet payments.",
		Status:      "available",
		Fields: []FieldDefinition{
			{Key: "wallet_email", Label: "Wallet account email", Type: FieldTypeText, Required: true, Placeholder: "buyer@example.com"},
			{Key: "requires_redirect", Label: "Requires redirect", Type: FieldTypeCheckbox, Required: false, HelpText: "Enable to preview a requires_action state."},
		},
		States: []State{{Code: "instant_settlement", Severity: SeveritySuccess, Message: "Settlement callback simulated instantly."}},
	})

	m.registerShipping(Definition{
		ID:          "dummy-ground",
		Type:        ProviderTypeShipping,
		Name:        "Dummy Ground Carrier",
		Description: "Calculates shipping by destination and service level.",
		Status:      "available",
		Fields: []FieldDefinition{
			{Key: "full_name", Label: "Recipient name", Type: FieldTypeText, Required: true},
			{Key: "line1", Label: "Address line 1", Type: FieldTypeText, Required: true},
			{Key: "line2", Label: "Address line 2", Type: FieldTypeText, Required: false},
			{Key: "city", Label: "City", Type: FieldTypeText, Required: true},
			{Key: "state", Label: "State/Province", Type: FieldTypeText, Required: false},
			{Key: "postal_code", Label: "Postal code", Type: FieldTypeText, Required: true},
			{Key: "country", Label: "Country", Type: FieldTypeText, Required: true, Placeholder: "US"},
			{
				Key:      "service_level",
				Label:    "Service level",
				Type:     FieldTypeSelect,
				Required: true,
				Options: []FieldOption{
					{Value: "standard", Label: "Standard (5.99)"},
					{Value: "express", Label: "Express (15.99)"},
				},
			},
		},
		States: []State{{Code: "tracking_delayed", Severity: SeverityWarning, Message: "Tracking updates may lag by 5-10 minutes in sandbox."}},
	})

	m.registerShipping(Definition{
		ID:          "dummy-pickup",
		Type:        ProviderTypeShipping,
		Name:        "Dummy In-Store Pickup",
		Description: "No shipping fee; requires pickup location details.",
		Status:      "available",
		Fields: []FieldDefinition{
			{
				Key:      "pickup_location",
				Label:    "Pickup location",
				Type:     FieldTypeSelect,
				Required: true,
				Options: []FieldOption{
					{Value: "downtown", Label: "Downtown Hub"},
					{Value: "airport", Label: "Airport Desk"},
				},
			},
			{Key: "pickup_contact", Label: "Contact name", Type: FieldTypeText, Required: true},
			{
				Key:      "state",
				Label:    "Pickup state",
				Type:     FieldTypeSelect,
				Required: true,
				Options:  usStateOptions(),
			},
			{Key: "postal_code", Label: "Pickup postal code", Type: FieldTypeText, Required: false},
		},
		States: []State{{Code: "id_required", Severity: SeverityInfo, Message: "Government-issued ID is required at pickup."}},
	})

	m.registerTax(Definition{
		ID:          "dummy-us-tax",
		Type:        ProviderTypeTax,
		Name:        "Dummy US Sales Tax",
		Description: "Applies state-based sales tax rates.",
		Status:      "available",
		Fields: []FieldDefinition{
			{Key: "state", Label: "Tax state", Type: FieldTypeText, Required: true, Placeholder: "CA"},
			{Key: "postal_code", Label: "Tax postal code", Type: FieldTypeText, Required: false},
			{Key: "tax_exempt", Label: "Tax exempt", Type: FieldTypeCheckbox, Required: false},
		},
		States: []State{{Code: "estimate_only", Severity: SeverityInfo, Message: "Rates are estimates for sandbox testing."}},
	})

	m.registerTax(Definition{
		ID:          "dummy-vat",
		Type:        ProviderTypeTax,
		Name:        "Dummy VAT",
		Description: "Applies flat VAT percentage.",
		Status:      "available",
		Fields: []FieldDefinition{
			{Key: "vat_country", Label: "VAT country", Type: FieldTypeText, Required: true, Placeholder: "DE"},
			{Key: "vat_rate", Label: "VAT rate (%)", Type: FieldTypeNumber, Required: false, Placeholder: "20"},
		},
		States: []State{{Code: "manual_validation", Severity: SeverityWarning, Message: "VAT IDs are not validated in this dummy provider."}},
	})

	return m
}

func (m *Manager) List() (payments []Definition, shippings []Definition, taxes []Definition) {
	return m.list(false)
}

func (m *Manager) ListForAdmin() (payments []Definition, shippings []Definition, taxes []Definition) {
	return m.list(true)
}

func (m *Manager) list(includeDisabled bool) (payments []Definition, shippings []Definition, taxes []Definition) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	payments = m.listByTypeLocked(ProviderTypePayment, includeDisabled)
	shippings = m.listByTypeLocked(ProviderTypeShipping, includeDisabled)
	taxes = m.listByTypeLocked(ProviderTypeTax, includeDisabled)
	return payments, shippings, taxes
}

func (m *Manager) ListSettings() []ProviderSetting {
	m.mu.RLock()
	defer m.mu.RUnlock()

	settings := make([]ProviderSetting, 0, len(m.payments)+len(m.shippings)+len(m.taxes)+len(m.externalPayments)+len(m.externalShippings)+len(m.externalTaxes))
	appendSettings := func(providerType ProviderType, registry map[string]Definition) {
		for id := range registry {
			settings = append(settings, ProviderSetting{
				Type:    providerType,
				ID:      id,
				Enabled: m.isProviderEnabledLocked(providerType, id),
			})
		}
	}
	appendExternalSettings := func(providerType ProviderType, registry map[string]*externalProvider) {
		for id := range registry {
			settings = append(settings, ProviderSetting{
				Type:    providerType,
				ID:      id,
				Enabled: m.isProviderEnabledLocked(providerType, id),
			})
		}
	}

	appendSettings(ProviderTypePayment, m.payments)
	appendSettings(ProviderTypeShipping, m.shippings)
	appendSettings(ProviderTypeTax, m.taxes)
	appendExternalSettings(ProviderTypePayment, m.externalPayments)
	appendExternalSettings(ProviderTypeShipping, m.externalShippings)
	appendExternalSettings(ProviderTypeTax, m.externalTaxes)
	return settings
}

func (m *Manager) ReplaceSettings(settings []ProviderSetting) {
	m.mu.Lock()
	defer m.mu.Unlock()

	provided := make(map[string]bool, len(settings))
	for _, setting := range settings {
		key := providerSettingKey(setting.Type, setting.ID)
		provided[key] = setting.Enabled
	}

	applyRegistry := func(providerType ProviderType, registry map[string]Definition) {
		for id := range registry {
			key := providerSettingKey(providerType, id)
			if enabled, ok := provided[key]; ok {
				m.enabled[key] = enabled
			}
		}
	}
	applyExternalRegistry := func(providerType ProviderType, registry map[string]*externalProvider) {
		for id := range registry {
			key := providerSettingKey(providerType, id)
			if enabled, ok := provided[key]; ok {
				m.enabled[key] = enabled
			}
		}
	}

	applyRegistry(ProviderTypePayment, m.payments)
	applyRegistry(ProviderTypeShipping, m.shippings)
	applyRegistry(ProviderTypeTax, m.taxes)
	applyExternalRegistry(ProviderTypePayment, m.externalPayments)
	applyExternalRegistry(ProviderTypeShipping, m.externalShippings)
	applyExternalRegistry(ProviderTypeTax, m.externalTaxes)

	m.normalizeTaxSelectionLocked()
}

func (m *Manager) SetProviderEnabled(providerType ProviderType, providerID string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	registry, externalRegistry := m.providerRegistries(providerType)
	if _, ok := registry[providerID]; !ok {
		if _, extOk := externalRegistry[providerID]; !extOk {
			return fmt.Errorf("unknown %s provider: %s", providerType, providerID)
		}
	}

	if providerType == ProviderTypeTax {
		if enabled {
			for id := range registry {
				m.enabled[providerSettingKey(ProviderTypeTax, id)] = false
			}
			for id := range externalRegistry {
				m.enabled[providerSettingKey(ProviderTypeTax, id)] = false
			}
			m.enabled[providerSettingKey(providerType, providerID)] = true
			return nil
		}
		if m.enabled[providerSettingKey(providerType, providerID)] {
			return fmt.Errorf("cannot disable the active tax provider; activate another tax provider first")
		}
		m.enabled[providerSettingKey(providerType, providerID)] = false
		m.normalizeTaxSelectionLocked()
		return nil
	}

	m.enabled[providerSettingKey(providerType, providerID)] = enabled
	return nil
}

func (m *Manager) Quote(req QuoteRequest) QuoteResult {
	result := QuoteResult{
		Currency: "USD",
		Subtotal: req.Subtotal,
		Valid:    true,
	}

	paymentProvider, paymentErr := m.resolveProvider(ProviderTypePayment, req.PaymentID, req.PaymentData, "payment")
	if paymentErr != nil {
		result.Valid = false
		result.PaymentStates = append(result.PaymentStates, State{Code: "payment_invalid", Severity: SeverityError, Message: paymentErr.Error()})
	} else {
		result.PaymentStates = append(result.PaymentStates, paymentProvider.states...)
	}

	shippingProvider, shippingErr := m.resolveProvider(ProviderTypeShipping, req.ShippingID, req.ShippingData, "shipping")
	if shippingErr != nil {
		result.Valid = false
		result.ShippingStates = append(result.ShippingStates, State{Code: "shipping_invalid", Severity: SeverityError, Message: shippingErr.Error()})
	} else {
		result.ShippingStates = append(result.ShippingStates, shippingProvider.states...)
	}

	autoSelectedTax := true
	effectiveTaxData := inferTaxData(req.TaxData, req.ShippingData)
	if shippingProvider != nil {
		effectiveTaxData = inferTaxData(effectiveTaxData, shippingProvider.data)
	}
	taxProvider, taxErr := m.resolveProvider(ProviderTypeTax, req.TaxID, effectiveTaxData, "tax")
	if taxErr != nil {
		result.Valid = false
		result.TaxStates = append(result.TaxStates, State{Code: "tax_invalid", Severity: SeverityError, Message: taxErr.Error()})
	} else {
		if autoSelectedTax {
			result.TaxStates = append(result.TaxStates, State{
				Code:     "tax_auto_selected",
				Severity: SeverityInfo,
				Message:  fmt.Sprintf("Tax calculated with %s.", taxProvider.def.Name),
			})
		}
		result.TaxStates = append(result.TaxStates, taxProvider.states...)
	}

	if paymentProvider != nil {
		if paymentProvider.external == nil {
			result.PaymentStates = append(result.PaymentStates, evaluatePayment(paymentProvider.def, paymentProvider.data)...)
		} else {
			externalResult, err := paymentProvider.external.quote(req.Subtotal.Float64(), req.Subtotal.Float64(), result.Currency, paymentProvider.data)
			if err != nil {
				result.PaymentStates = append(result.PaymentStates, State{Code: "payment_external_error", Severity: SeverityError, Message: err.Error()})
			} else {
				result.PaymentStates = append(result.PaymentStates, externalResult.States...)
				if !externalResult.Valid {
					result.PaymentStates = append(result.PaymentStates, State{Code: "payment_external_invalid", Severity: SeverityError, Message: paymentProvider.def.Name + " rejected payment input."})
				}
			}
		}
	}

	if shippingProvider != nil {
		if shippingProvider.external == nil {
			shippingAmount, states := evaluateShipping(shippingProvider.def, shippingProvider.data)
			result.Shipping = shippingAmount
			result.ShippingStates = append(result.ShippingStates, states...)
		} else {
			externalResult, err := shippingProvider.external.quote(req.Subtotal.Float64(), req.Subtotal.Float64(), result.Currency, shippingProvider.data)
			if err != nil {
				result.ShippingStates = append(result.ShippingStates, State{Code: "shipping_external_error", Severity: SeverityError, Message: err.Error()})
			} else {
				result.ShippingStates = append(result.ShippingStates, externalResult.States...)
				result.Shipping = models.MoneyFromFloat(externalResult.Amount)
				if !externalResult.Valid {
					result.ShippingStates = append(result.ShippingStates, State{Code: "shipping_external_invalid", Severity: SeverityError, Message: shippingProvider.def.Name + " rejected shipping input."})
				}
			}
		}
	}

	if taxProvider != nil {
		taxableBase := req.Subtotal + result.Shipping
		if taxProvider.external == nil {
			taxAmount, states := evaluateTax(taxProvider.def, taxProvider.data, taxableBase)
			result.Tax = taxAmount
			result.TaxStates = append(result.TaxStates, states...)
		} else {
			externalResult, err := taxProvider.external.quote(req.Subtotal.Float64(), taxableBase.Float64(), result.Currency, taxProvider.data)
			if err != nil {
				result.TaxStates = append(result.TaxStates, State{Code: "tax_external_error", Severity: SeverityError, Message: err.Error()})
			} else {
				result.TaxStates = append(result.TaxStates, externalResult.States...)
				result.Tax = models.MoneyFromFloat(externalResult.Amount)
				if !externalResult.Valid {
					result.TaxStates = append(result.TaxStates, State{Code: "tax_external_invalid", Severity: SeverityError, Message: taxProvider.def.Name + " rejected tax input."})
				}
			}
		}
	}

	if hasErrorState(result.PaymentStates) || hasErrorState(result.ShippingStates) || hasErrorState(result.TaxStates) {
		result.Valid = false
	}

	result.Total = req.Subtotal + result.Shipping + result.Tax
	return result
}

func inferTaxData(base map[string]string, shipping map[string]string) map[string]string {
	if base == nil {
		base = map[string]string{}
	}
	if shipping == nil {
		return base
	}

	clone := make(map[string]string, len(base)+2)
	for key, value := range base {
		clone[key] = value
	}
	if strings.TrimSpace(clone["state"]) == "" && strings.TrimSpace(shipping["state"]) != "" {
		clone["state"] = strings.ToUpper(strings.TrimSpace(shipping["state"]))
	}
	if strings.TrimSpace(clone["postal_code"]) == "" && strings.TrimSpace(shipping["postal_code"]) != "" {
		clone["postal_code"] = strings.TrimSpace(shipping["postal_code"])
	}
	return clone
}

func (m *Manager) ResolveCheckoutDetails(req QuoteRequest) (CheckoutDetails, error) {
	quote := m.Quote(req)
	if !quote.Valid {
		return CheckoutDetails{}, fmt.Errorf("selected providers are invalid")
	}

	details := CheckoutDetails{}

	paymentProvider, err := m.resolveProvider(ProviderTypePayment, req.PaymentID, req.PaymentData, "payment")
	if err != nil {
		return CheckoutDetails{}, err
	}
	shippingProvider, err := m.resolveProvider(ProviderTypeShipping, req.ShippingID, req.ShippingData, "shipping")
	if err != nil {
		return CheckoutDetails{}, err
	}

	if paymentProvider.external == nil {
		details.PaymentDisplay = paymentDisplayFromProvider(paymentProvider.def, paymentProvider.data)
	} else {
		response, err := paymentProvider.external.resolve(req.Subtotal.Float64(), req.Subtotal.Float64(), "USD", paymentProvider.data)
		if err != nil {
			return CheckoutDetails{}, err
		}
		details.PaymentDisplay = strings.TrimSpace(response.PaymentDisplay)
	}

	if shippingProvider.external == nil {
		details.ShippingAddress = shippingAddressFromProvider(shippingProvider.def, shippingProvider.data)
	} else {
		response, err := shippingProvider.external.resolve(req.Subtotal.Float64(), req.Subtotal.Float64(), "USD", shippingProvider.data)
		if err != nil {
			return CheckoutDetails{}, err
		}
		details.ShippingAddress = strings.TrimSpace(response.ShippingAddress)
	}

	if details.PaymentDisplay == "" {
		return CheckoutDetails{}, fmt.Errorf("unable to resolve payment display")
	}
	if details.ShippingAddress == "" {
		return CheckoutDetails{}, fmt.Errorf("unable to resolve shipping address")
	}

	return details, nil
}

func (m *Manager) registerPayment(def Definition) {
	m.payments[def.ID] = def
	m.enabled[providerSettingKey(ProviderTypePayment, def.ID)] = true
}

func (m *Manager) registerShipping(def Definition) {
	m.shippings[def.ID] = def
	m.enabled[providerSettingKey(ProviderTypeShipping, def.ID)] = true
}

func (m *Manager) registerTax(def Definition) {
	m.taxes[def.ID] = def
	m.enabled[providerSettingKey(ProviderTypeTax, def.ID)] = !m.hasActiveTaxLocked()
}

type resolvedProvider struct {
	def      Definition
	data     map[string]string
	states   []State
	external *externalProvider
}

func (m *Manager) resolveProvider(providerType ProviderType, providerID string, data map[string]string, providerLabel string) (*resolvedProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	providerID = strings.TrimSpace(providerID)
	if providerType == ProviderTypeTax {
		providerID = m.defaultProviderID(providerType)
	}

	registry, externalRegistry := m.providerRegistries(providerType)
	if providerID == "" {
		return nil, fmt.Errorf("%s provider is required", providerLabel)
	}
	if !m.isProviderEnabledLocked(providerType, providerID) {
		return nil, fmt.Errorf("%s provider is disabled: %s", providerLabel, providerID)
	}
	if def, ok := registry[providerID]; ok {
		validatedData, states, err := validateProviderInput(def, data)
		if err != nil {
			return nil, err
		}
		return &resolvedProvider{def: def, data: validatedData, states: states}, nil
	}
	if ext, ok := externalRegistry[providerID]; ok {
		validatedData, states, err := validateProviderInput(ext.definition, data)
		if err != nil {
			return nil, err
		}
		return &resolvedProvider{def: ext.definition, data: validatedData, states: states, external: ext}, nil
	}
	return nil, fmt.Errorf("unknown %s provider: %s", providerLabel, providerID)
}

func (m *Manager) defaultProviderID(providerType ProviderType) string {
	registry, externalRegistry := m.providerRegistries(providerType)
	type providerCandidate struct {
		id   string
		name string
	}
	candidates := make([]providerCandidate, 0, len(registry)+len(externalRegistry))
	for id, def := range registry {
		if !m.isProviderEnabledLocked(providerType, id) {
			continue
		}
		candidates = append(candidates, providerCandidate{id: id, name: def.Name})
	}
	for id, provider := range externalRegistry {
		if !m.isProviderEnabledLocked(providerType, id) {
			continue
		}
		candidates = append(candidates, providerCandidate{id: id, name: provider.definition.Name})
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].name == candidates[j].name {
			return candidates[i].id < candidates[j].id
		}
		return candidates[i].name < candidates[j].name
	})
	return candidates[0].id
}

func (m *Manager) providerRegistries(providerType ProviderType) (map[string]Definition, map[string]*externalProvider) {
	switch providerType {
	case ProviderTypePayment:
		return m.payments, m.externalPayments
	case ProviderTypeShipping:
		return m.shippings, m.externalShippings
	default:
		return m.taxes, m.externalTaxes
	}
}

func (m *Manager) hasActiveTaxLocked() bool {
	for id := range m.taxes {
		if m.enabled[providerSettingKey(ProviderTypeTax, id)] {
			return true
		}
	}
	for id := range m.externalTaxes {
		if m.enabled[providerSettingKey(ProviderTypeTax, id)] {
			return true
		}
	}
	return false
}

func (m *Manager) isProviderEnabledLocked(providerType ProviderType, providerID string) bool {
	return m.enabled[providerSettingKey(providerType, providerID)]
}

func (m *Manager) listByTypeLocked(providerType ProviderType, includeDisabled bool) []Definition {
	registry, externalRegistry := m.providerRegistries(providerType)
	values := make([]Definition, 0, len(registry)+len(externalRegistry))
	for _, def := range registry {
		withState := def
		withState.Enabled = m.isProviderEnabledLocked(providerType, def.ID)
		if !includeDisabled && !withState.Enabled {
			continue
		}
		values = append(values, withState)
	}
	for _, plugin := range externalRegistry {
		withState := plugin.definition
		withState.Enabled = m.isProviderEnabledLocked(providerType, withState.ID)
		if !includeDisabled && !withState.Enabled {
			continue
		}
		values = append(values, withState)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].Name < values[j].Name })
	return values
}

func (m *Manager) normalizeTaxSelectionLocked() {
	type providerCandidate struct {
		id   string
		name string
	}

	active := make([]providerCandidate, 0)
	all := make([]providerCandidate, 0, len(m.taxes)+len(m.externalTaxes))

	for id, def := range m.taxes {
		candidate := providerCandidate{id: id, name: def.Name}
		all = append(all, candidate)
		if m.enabled[providerSettingKey(ProviderTypeTax, id)] {
			active = append(active, candidate)
		}
	}
	for id, def := range m.externalTaxes {
		candidate := providerCandidate{id: id, name: def.definition.Name}
		all = append(all, candidate)
		if m.enabled[providerSettingKey(ProviderTypeTax, id)] {
			active = append(active, candidate)
		}
	}

	if len(all) == 0 {
		return
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].name == all[j].name {
			return all[i].id < all[j].id
		}
		return all[i].name < all[j].name
	})

	chosenID := ""
	if len(active) > 0 {
		sort.Slice(active, func(i, j int) bool {
			if active[i].name == active[j].name {
				return active[i].id < active[j].id
			}
			return active[i].name < active[j].name
		})
		chosenID = active[0].id
	} else {
		chosenID = all[0].id
	}

	for _, candidate := range all {
		m.enabled[providerSettingKey(ProviderTypeTax, candidate.id)] = candidate.id == chosenID
	}
}

func providerSettingKey(providerType ProviderType, providerID string) string {
	return string(providerType) + ":" + providerID
}

func (m *Manager) resolveProviderData(registry map[string]Definition, providerID string, data map[string]string, providerLabel string) (*Definition, map[string]string, []State, error) {
	if providerID == "" {
		return nil, nil, nil, fmt.Errorf("%s provider is required", providerLabel)
	}
	def, ok := registry[providerID]
	if !ok {
		return nil, nil, nil, fmt.Errorf("unknown %s provider: %s", providerLabel, providerID)
	}
	validatedData, states, err := validateProviderInput(def, data)
	if err != nil {
		return nil, nil, states, err
	}
	return &def, validatedData, states, nil
}

func validateProviderInput(def Definition, data map[string]string) (map[string]string, []State, error) {
	if data == nil {
		data = map[string]string{}
	}
	states := make([]State, 0, len(def.States))
	states = append(states, def.States...)
	for _, field := range def.Fields {
		if !field.Required {
			continue
		}
		if strings.TrimSpace(data[field.Key]) == "" {
			return nil, states, fmt.Errorf("%s is required for %s", field.Label, def.Name)
		}
	}
	return data, states, nil
}

func evaluatePayment(def Definition, data map[string]string) []State {
	states := []State{}
	switch def.ID {
	case "dummy-card":
		number := digitsOnly(data["card_number"])
		if len(number) < 12 || len(number) > 19 {
			states = append(states, State{Code: "invalid_card", Severity: SeverityError, Message: "Card number must be between 12 and 19 digits."})
		}
		if strings.HasSuffix(number, "0000") {
			states = append(states, State{Code: "card_declined", Severity: SeverityError, Message: "Simulated decline: use a non-0000 suffix to approve."})
		}
		year, _ := strconv.Atoi(strings.TrimSpace(data["exp_year"]))
		month, _ := strconv.Atoi(strings.TrimSpace(data["exp_month"]))
		now := time.Now()
		if year < now.Year() || (year == now.Year() && month < int(now.Month())) {
			states = append(states, State{Code: "card_expired", Severity: SeverityError, Message: "Card expiry must be in the future."})
		}
	case "dummy-wallet":
		if strings.EqualFold(strings.TrimSpace(data["requires_redirect"]), "true") {
			states = append(states, State{Code: "requires_action", Severity: SeverityInfo, Message: "Wallet provider requires a redirect before confirmation."})
		}
	}
	return states
}

func evaluateShipping(def Definition, data map[string]string) (models.Money, []State) {
	states := []State{}
	switch def.ID {
	case "dummy-ground":
		service := strings.ToLower(strings.TrimSpace(data["service_level"]))
		country := strings.ToUpper(strings.TrimSpace(data["country"]))
		amount := models.MoneyFromFloat(5.99)
		if service == "express" {
			amount = models.MoneyFromFloat(15.99)
		}
		if country != "" && country != "US" {
			amount += models.MoneyFromFloat(12.50)
			states = append(states, State{Code: "cross_border", Severity: SeverityWarning, Message: "Cross-border surcharge was applied in sandbox mode."})
		}
		return amount, states
	case "dummy-pickup":
		states = append(states, State{Code: "pickup_only", Severity: SeverityInfo, Message: "Order will be held for pickup for 7 days."})
		return 0, states
	default:
		return 0, states
	}
}

func evaluateTax(def Definition, data map[string]string, taxableBase models.Money) (models.Money, []State) {
	states := []State{}
	if taxableBase < 0 {
		taxableBase = 0
	}

	switch def.ID {
	case "dummy-us-tax":
		if strings.EqualFold(strings.TrimSpace(data["tax_exempt"]), "true") {
			states = append(states, State{Code: "tax_exempt", Severity: SeveritySuccess, Message: "Tax exemption applied."})
			return 0, states
		}
		state := strings.ToUpper(strings.TrimSpace(data["state"]))
		rate := 0.05
		switch state {
		case "CA":
			rate = 0.085
		case "NY":
			rate = 0.08875
		case "TX":
			rate = 0.0625
		}
		return models.MoneyFromFloat(taxableBase.Float64() * rate), states
	case "dummy-vat":
		rate := 0.2
		if rawRate := strings.TrimSpace(data["vat_rate"]); rawRate != "" {
			if customRate, err := strconv.ParseFloat(rawRate, 64); err == nil && customRate >= 0 {
				rate = customRate / 100
			}
		}
		return models.MoneyFromFloat(taxableBase.Float64() * rate), states
	default:
		return 0, states
	}
}

func paymentDisplayFromProvider(def Definition, data map[string]string) string {
	switch def.ID {
	case "dummy-card":
		number := digitsOnly(data["card_number"])
		if len(number) >= 4 {
			brand := detectCardBrand(number)
			return brand + " •••• " + number[len(number)-4:]
		}
		return "Card"
	case "dummy-wallet":
		email := strings.TrimSpace(data["wallet_email"])
		if email == "" {
			return "Dummy Wallet"
		}
		return "Dummy Wallet " + email
	default:
		return def.Name
	}
}

func shippingAddressFromProvider(def Definition, data map[string]string) string {
	switch def.ID {
	case "dummy-ground":
		parts := []string{
			strings.TrimSpace(data["line1"]),
			strings.TrimSpace(data["line2"]),
			strings.TrimSpace(data["city"]),
			strings.TrimSpace(data["state"]),
			strings.TrimSpace(data["postal_code"]),
			strings.ToUpper(strings.TrimSpace(data["country"])),
		}
		filtered := make([]string, 0, len(parts))
		for _, part := range parts {
			if part != "" {
				filtered = append(filtered, part)
			}
		}
		return strings.Join(filtered, ", ")
	case "dummy-pickup":
		location := strings.TrimSpace(data["pickup_location"])
		contact := strings.TrimSpace(data["pickup_contact"])
		state := strings.ToUpper(strings.TrimSpace(data["state"]))
		if location == "" {
			location = "Store"
		}
		suffix := ""
		if state != "" {
			suffix = " (" + state + ")"
		}
		if contact == "" {
			return "Pickup at " + location + suffix
		}
		return "Pickup at " + location + suffix + " for " + contact
	default:
		return ""
	}
}

func usStateOptions() []FieldOption {
	return []FieldOption{
		{Value: "AL", Label: "Alabama"},
		{Value: "AK", Label: "Alaska"},
		{Value: "AZ", Label: "Arizona"},
		{Value: "AR", Label: "Arkansas"},
		{Value: "CA", Label: "California"},
		{Value: "CO", Label: "Colorado"},
		{Value: "CT", Label: "Connecticut"},
		{Value: "DE", Label: "Delaware"},
		{Value: "FL", Label: "Florida"},
		{Value: "GA", Label: "Georgia"},
		{Value: "HI", Label: "Hawaii"},
		{Value: "ID", Label: "Idaho"},
		{Value: "IL", Label: "Illinois"},
		{Value: "IN", Label: "Indiana"},
		{Value: "IA", Label: "Iowa"},
		{Value: "KS", Label: "Kansas"},
		{Value: "KY", Label: "Kentucky"},
		{Value: "LA", Label: "Louisiana"},
		{Value: "ME", Label: "Maine"},
		{Value: "MD", Label: "Maryland"},
		{Value: "MA", Label: "Massachusetts"},
		{Value: "MI", Label: "Michigan"},
		{Value: "MN", Label: "Minnesota"},
		{Value: "MS", Label: "Mississippi"},
		{Value: "MO", Label: "Missouri"},
		{Value: "MT", Label: "Montana"},
		{Value: "NE", Label: "Nebraska"},
		{Value: "NV", Label: "Nevada"},
		{Value: "NH", Label: "New Hampshire"},
		{Value: "NJ", Label: "New Jersey"},
		{Value: "NM", Label: "New Mexico"},
		{Value: "NY", Label: "New York"},
		{Value: "NC", Label: "North Carolina"},
		{Value: "ND", Label: "North Dakota"},
		{Value: "OH", Label: "Ohio"},
		{Value: "OK", Label: "Oklahoma"},
		{Value: "OR", Label: "Oregon"},
		{Value: "PA", Label: "Pennsylvania"},
		{Value: "RI", Label: "Rhode Island"},
		{Value: "SC", Label: "South Carolina"},
		{Value: "SD", Label: "South Dakota"},
		{Value: "TN", Label: "Tennessee"},
		{Value: "TX", Label: "Texas"},
		{Value: "UT", Label: "Utah"},
		{Value: "VT", Label: "Vermont"},
		{Value: "VA", Label: "Virginia"},
		{Value: "WA", Label: "Washington"},
		{Value: "WV", Label: "West Virginia"},
		{Value: "WI", Label: "Wisconsin"},
		{Value: "WY", Label: "Wyoming"},
		{Value: "DC", Label: "District of Columbia"},
	}
}

func hasErrorState(states []State) bool {
	for _, state := range states {
		if state.Severity == SeverityError {
			return true
		}
	}
	return false
}

func digitsOnly(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func detectCardBrand(number string) string {
	switch {
	case strings.HasPrefix(number, "4"):
		return "Visa"
	case strings.HasPrefix(number, "34"), strings.HasPrefix(number, "37"):
		return "American Express"
	case strings.HasPrefix(number, "5"):
		return "Mastercard"
	case strings.HasPrefix(number, "6"):
		return "Discover"
	default:
		return "Card"
	}
}
