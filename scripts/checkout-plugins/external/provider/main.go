package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type request struct {
	Action       string            `json:"action"`
	ProviderID   string            `json:"provider_id,omitempty"`
	ProviderType string            `json:"provider_type,omitempty"`
	Environment  string            `json:"environment,omitempty"`
	Credentials  map[string]string `json:"credentials,omitempty"`
	Input        json.RawMessage   `json:"input"`
}

type legacyInput struct {
	ProviderID   string            `json:"provider_id"`
	ProviderType string            `json:"provider_type"`
	Currency     string            `json:"currency"`
	Subtotal     float64           `json:"subtotal"`
	TaxableBase  float64           `json:"taxable_base"`
	Data         map[string]string `json:"data"`
}

type webhookInput struct {
	Headers    map[string]string `json:"headers,omitempty"`
	BodyBase64 string            `json:"body_base64"`
}

type paymentWebhookPayload struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		ProviderTxnID string `json:"provider_txn_id"`
	} `json:"data"`
}

type shippingWebhookPayload struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		ProviderShipmentID string `json:"provider_shipment_id"`
		TrackingNumber     string `json:"tracking_number"`
		Status             string `json:"status"`
		Location           string `json:"location"`
		Description        string `json:"description"`
		OccurredAt         string `json:"occurred_at"`
	} `json:"data"`
}

type state struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type quoteResponse struct {
	Valid  bool    `json:"valid"`
	Amount float64 `json:"amount,omitempty"`
	States []state `json:"states,omitempty"`
	Error  string  `json:"error,omitempty"`
}

type resolveResponse struct {
	Valid           bool    `json:"valid"`
	PaymentDisplay  string  `json:"payment_display,omitempty"`
	ShippingAddress string  `json:"shipping_address,omitempty"`
	States          []state `json:"states,omitempty"`
	Error           string  `json:"error,omitempty"`
}

func main() {
	payload, err := io.ReadAll(os.Stdin)
	if err != nil {
		writeJSON(map[string]string{"error": "unable to read request"})
		os.Exit(1)
	}

	var req request
	if err := json.Unmarshal(payload, &req); err != nil {
		writeJSON(map[string]string{"error": "invalid request json"})
		os.Exit(1)
	}

	switch req.Action {
	case "quote":
		handleQuote(req)
	case "resolve":
		handleResolve(req)
	case "payment.authorize":
		requireCredential(req, "api_key")
		writeJSON(map[string]string{
			"ProviderTxnID":       "externalpay-auth",
			"RawResponseRedacted": `{"status":"authorized"}`,
		})
	case "payment.capture":
		requireCredential(req, "api_key")
		writeJSON(map[string]string{
			"ProviderTxnID":       "externalpay-capture",
			"RawResponseRedacted": `{"status":"captured"}`,
		})
	case "payment.void":
		requireCredential(req, "api_key")
		writeJSON(map[string]string{
			"ProviderTxnID":       "externalpay-void",
			"RawResponseRedacted": `{"status":"voided"}`,
		})
	case "payment.refund":
		requireCredential(req, "api_key")
		writeJSON(map[string]string{
			"ProviderTxnID":       "externalpay-refund",
			"RawResponseRedacted": `{"status":"refunded"}`,
		})
	case "payment.get_transaction":
		requireCredential(req, "api_key")
		writeJSON(map[string]any{
			"ProviderTxnID": "externalpay-auth",
			"Operation":     "AUTHORIZE",
			"Amount":        42.5,
			"Currency":      "USD",
			"Status":        "SUCCEEDED",
		})
	case "payment.verify_webhook", "payment.parse_webhook":
		requireCredential(req, "api_key")
		event := paymentEvent(req)
		writeJSON(map[string]string{
			"Provider":        "externalpay",
			"ProviderEventID": event.ID,
			"EventType":       event.Type,
			"ProviderTxnID":   firstNonEmpty(event.Data.ProviderTxnID, "externalpay-auth"),
		})
	case "shipping.quote_rates":
		requireCredential(req, "carrier_key")
		writeJSON([]map[string]any{{
			"ProviderRateID": "externalcarrier-standard",
			"ServiceCode":    "standard",
			"ServiceName":    "Standard",
			"Amount":         7.49,
			"Currency":       "USD",
			"ExpiresAt":      "2030-01-01T00:00:00Z",
		}})
	case "shipping.buy_label":
		requireCredential(req, "carrier_key")
		writeJSON(map[string]string{
			"ProviderShipmentID": "externalcarrier-shipment",
			"TrackingNumber":     "EXTERNAL123",
			"TrackingURL":        "https://tracking.example.test/externalcarrier",
			"LabelURL":           "https://labels.example.test/externalcarrier.pdf",
			"ServiceCode":        "standard",
			"ServiceName":        "Standard",
		})
	case "shipping.get_shipment":
		requireCredential(req, "carrier_key")
		writeJSON(map[string]string{
			"ProviderShipmentID": "externalcarrier-shipment",
			"TrackingNumber":     "EXTERNAL123",
			"Status":             "DELIVERED",
			"ServiceCode":        "standard",
			"ServiceName":        "Standard",
		})
	case "shipping.verify_webhook", "shipping.parse_webhook":
		requireCredential(req, "carrier_key")
		event := shippingEvent(req)
		writeJSON(map[string]string{
			"Provider":           "externalcarrier",
			"ProviderEventID":    event.ID,
			"EventType":          firstNonEmpty(event.Type, "tracking.delivered"),
			"ProviderShipmentID": firstNonEmpty(event.Data.ProviderShipmentID, "externalcarrier-shipment"),
			"TrackingNumber":     firstNonEmpty(event.Data.TrackingNumber, "EXTERNAL123"),
			"Status":             firstNonEmpty(strings.ToUpper(strings.TrimSpace(event.Data.Status)), "DELIVERED"),
			"Location":           firstNonEmpty(event.Data.Location, "Sample Warehouse"),
			"Description":        firstNonEmpty(event.Data.Description, "Delivered"),
			"OccurredAt":         firstNonEmpty(event.Data.OccurredAt, "2030-01-01T00:00:00Z"),
			"RawPayload":         "{}",
		})
	case "tax.quote_tax":
		requireCredential(req, "tax_key")
		writeJSON(8.4)
	case "tax.finalize_tax":
		requireCredential(req, "tax_key")
		writeJSON(map[string]any{
			"Provider":         "externalrate",
			"Currency":         "USD",
			"InclusivePricing": false,
			"TotalTax":         8.4,
			"Lines": []map[string]any{{
				"LineType":           "ITEM",
				"Quantity":           1,
				"Jurisdiction":       "US",
				"TaxCode":            "external_goods",
				"TaxName":            "External Tax",
				"TaxableAmount":      120,
				"TaxAmount":          8.4,
				"TaxRateBasisPoints": 700,
				"Inclusive":          false,
			}},
		})
	case "tax.export_report":
		requireCredential(req, "tax_key")
		writeJSON(map[string]string{
			"content": "order_id,snapshot_id,line_type,jurisdiction,tax_name,tax_amount,taxable_amount,inclusive\n1,1,ITEM,US,External Tax,8.40,120.00,false\n",
		})
	default:
		writeJSON(map[string]string{"error": "unsupported action"})
		os.Exit(1)
	}
}

func handleQuote(req request) {
	var input legacyInput
	mustDecodeInput(req.Input, &input)
	if input.Data == nil {
		input.Data = map[string]string{}
	}

	switch providerKind(req, input) {
	case "payment":
		token := strings.TrimSpace(input.Data["token"])
		simulate3DS := strings.EqualFold(strings.TrimSpace(input.Data["simulate_3ds"]), "true")
		resp := quoteResponse{Valid: true}
		if token == "" {
			resp.Valid = false
			resp.States = append(resp.States, state{Code: "missing_token", Severity: "error", Message: "Payment token is required."})
		}
		if strings.EqualFold(token, "fail") {
			resp.Valid = false
			resp.States = append(resp.States, state{Code: "declined", Severity: "error", Message: "External gateway simulated a decline."})
		}
		if simulate3DS {
			resp.States = append(resp.States, state{Code: "requires_action", Severity: "info", Message: "3DS challenge required before confirmation."})
		}
		writeJSON(resp)
	case "shipping":
		zone := strings.ToLower(strings.TrimSpace(input.Data["zone"]))
		speed := strings.ToLower(strings.TrimSpace(input.Data["speed"]))
		amount := 7.49
		if speed == "overnight" {
			amount = 24.99
		}
		if zone == "international" {
			amount += 14.00
		}
		writeJSON(quoteResponse{
			Valid:  true,
			Amount: amount,
			States: []state{{Code: "external_rating", Severity: "info", Message: "Shipping rate calculated by external plugin."}},
		})
	case "tax":
		rate := 7.0
		if raw := strings.TrimSpace(input.Data["rate_percent"]); raw != "" {
			if parsed, err := strconv.ParseFloat(raw, 64); err == nil && parsed >= 0 {
				rate = parsed
			}
		}
		writeJSON(quoteResponse{
			Valid:  true,
			Amount: input.TaxableBase * (rate / 100),
			States: []state{{Code: "external_tax", Severity: "info", Message: "External tax rate applied."}},
		})
	default:
		writeJSON(resolveResponse{Valid: false, Error: "unsupported provider type"})
		os.Exit(1)
	}
}

func handleResolve(req request) {
	var input legacyInput
	mustDecodeInput(req.Input, &input)
	if input.Data == nil {
		input.Data = map[string]string{}
	}

	switch providerKind(req, input) {
	case "payment":
		token := strings.TrimSpace(input.Data["token"])
		if token == "" {
			writeJSON(resolveResponse{Valid: false, Error: "payment token is required"})
			return
		}
		masked := token
		if len(masked) > 4 {
			masked = masked[len(masked)-4:]
		}
		writeJSON(resolveResponse{Valid: true, PaymentDisplay: "ExternalPay Token •••• " + masked})
	case "shipping":
		line1 := strings.TrimSpace(input.Data["line1"])
		city := strings.TrimSpace(input.Data["city"])
		country := strings.ToUpper(strings.TrimSpace(input.Data["country"]))
		if line1 == "" || city == "" || country == "" {
			writeJSON(resolveResponse{Valid: false, Error: "line1, city, and country are required"})
			return
		}
		writeJSON(resolveResponse{Valid: true, ShippingAddress: fmt.Sprintf("%s, %s, %s", line1, city, country)})
	case "tax":
		writeJSON(resolveResponse{Valid: true})
	default:
		writeJSON(resolveResponse{Valid: false, Error: "unsupported provider type"})
		os.Exit(1)
	}
}

func paymentEvent(req request) paymentWebhookPayload {
	var input webhookInput
	mustDecodeInput(req.Input, &input)
	event := paymentWebhookPayload{
		ID:   "evt-externalpay",
		Type: "payment.captured",
	}
	body, err := base64.StdEncoding.DecodeString(strings.TrimSpace(input.BodyBase64))
	if err == nil && len(body) > 0 {
		_ = json.Unmarshal(body, &event)
	}
	return event
}

func shippingEvent(req request) shippingWebhookPayload {
	var input webhookInput
	mustDecodeInput(req.Input, &input)
	event := shippingWebhookPayload{
		ID:   "evt-externalcarrier",
		Type: "tracking.delivered",
	}
	body, err := base64.StdEncoding.DecodeString(strings.TrimSpace(input.BodyBase64))
	if err == nil && len(body) > 0 {
		_ = json.Unmarshal(body, &event)
	}
	return event
}

func providerKind(req request, input legacyInput) string {
	if req.ProviderType != "" {
		return strings.ToLower(strings.TrimSpace(req.ProviderType))
	}
	if input.ProviderType != "" {
		return strings.ToLower(strings.TrimSpace(input.ProviderType))
	}
	if len(os.Args) > 1 {
		return strings.ToLower(strings.TrimSpace(os.Args[1]))
	}
	return ""
}

func requireCredential(req request, key string) {
	if strings.TrimSpace(req.Credentials[key]) == "" {
		writeJSON(map[string]string{"error": "missing credential: " + key})
		os.Exit(1)
	}
}

func mustDecodeInput(raw json.RawMessage, into any) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	if err := json.Unmarshal(raw, into); err != nil {
		writeJSON(map[string]string{"error": "invalid input payload"})
		os.Exit(1)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func writeJSON(payload any) {
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(payload); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "encode response: %v\n", err)
	}
}
