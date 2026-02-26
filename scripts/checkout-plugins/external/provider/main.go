package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type request struct {
	Action string `json:"action"`
	Input  struct {
		ProviderID   string            `json:"provider_id"`
		ProviderType string            `json:"provider_type"`
		Currency     string            `json:"currency"`
		Subtotal     float64           `json:"subtotal"`
		TaxableBase  float64           `json:"taxable_base"`
		Data         map[string]string `json:"data"`
	} `json:"input"`
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
	if len(os.Args) < 2 {
		writeJSON(quoteResponse{Valid: false, Error: "provider kind argument is required"})
		os.Exit(1)
	}
	providerKind := os.Args[1]

	payload, err := io.ReadAll(os.Stdin)
	if err != nil {
		writeJSON(quoteResponse{Valid: false, Error: "unable to read request"})
		os.Exit(1)
	}

	var req request
	if err := json.Unmarshal(payload, &req); err != nil {
		writeJSON(quoteResponse{Valid: false, Error: "invalid request json"})
		os.Exit(1)
	}
	if req.Input.Data == nil {
		req.Input.Data = map[string]string{}
	}

	switch providerKind {
	case "payment":
		handlePayment(req)
	case "shipping":
		handleShipping(req)
	case "tax":
		handleTax(req)
	default:
		writeJSON(quoteResponse{Valid: false, Error: "unknown provider kind"})
		os.Exit(1)
	}
}

func handlePayment(req request) {
	token := strings.TrimSpace(req.Input.Data["token"])
	simulate3DS := strings.EqualFold(strings.TrimSpace(req.Input.Data["simulate_3ds"]), "true")

	if req.Action == "quote" {
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
		return
	}

	if req.Action == "resolve" {
		if token == "" {
			writeJSON(resolveResponse{Valid: false, Error: "payment token is required"})
			return
		}
		masked := token
		if len(masked) > 4 {
			masked = masked[len(masked)-4:]
		}
		writeJSON(resolveResponse{Valid: true, PaymentDisplay: "ExternalPay Token •••• " + masked})
		return
	}

	writeJSON(resolveResponse{Valid: false, Error: "unsupported action"})
}

func handleShipping(req request) {
	if req.Action == "quote" {
		zone := strings.ToLower(strings.TrimSpace(req.Input.Data["zone"]))
		speed := strings.ToLower(strings.TrimSpace(req.Input.Data["speed"]))
		amount := 7.49
		if speed == "overnight" {
			amount = 24.99
		}
		if zone == "international" {
			amount += 14.00
		}
		states := []state{{Code: "external_rating", Severity: "info", Message: "Shipping rate calculated by external plugin."}}
		writeJSON(quoteResponse{Valid: true, Amount: amount, States: states})
		return
	}

	if req.Action == "resolve" {
		line1 := strings.TrimSpace(req.Input.Data["line1"])
		city := strings.TrimSpace(req.Input.Data["city"])
		country := strings.ToUpper(strings.TrimSpace(req.Input.Data["country"]))
		if line1 == "" || city == "" || country == "" {
			writeJSON(resolveResponse{Valid: false, Error: "line1, city, and country are required"})
			return
		}
		address := fmt.Sprintf("%s, %s, %s", line1, city, country)
		writeJSON(resolveResponse{Valid: true, ShippingAddress: address})
		return
	}

	writeJSON(resolveResponse{Valid: false, Error: "unsupported action"})
}

func handleTax(req request) {
	if req.Action != "quote" {
		writeJSON(resolveResponse{Valid: true})
		return
	}

	rate := 7.0
	if raw := strings.TrimSpace(req.Input.Data["rate_percent"]); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil && parsed >= 0 {
			rate = parsed
		}
	}
	amount := req.Input.TaxableBase * (rate / 100)
	writeJSON(quoteResponse{Valid: true, Amount: amount, States: []state{{Code: "external_tax", Severity: "info", Message: "External tax rate applied."}}})
}

func writeJSON(payload any) {
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(payload); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "encode response: %v\n", err)
	}
}
