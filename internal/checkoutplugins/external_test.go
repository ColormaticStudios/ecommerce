package checkoutplugins

import (
	"os"
	"path/filepath"
	"testing"

	"ecommerce/models"
)

func TestLoadBundledExternalPluginManifests(t *testing.T) {
	manager := NewDefaultManager()
	loaded, err := manager.LoadExternalPluginsFromDir(filepath.Join("..", "..", "scripts", "checkout-plugins", "external", "manifests"))
	if err != nil {
		t.Fatalf("load bundled manifests: %v", err)
	}
	if loaded < 3 {
		t.Fatalf("expected at least 3 bundled manifests, got %d", loaded)
	}
}

func TestLoadExternalPluginsAndUseForQuoteAndResolve(t *testing.T) {
	dir := t.TempDir()

	scriptPath := filepath.Join(dir, "plugin.sh")
	script := `#!/usr/bin/env bash
set -euo pipefail
payload="$(cat)"
if [[ "$payload" == *'"action":"quote"'* ]]; then
  if [[ "$payload" == *'"provider_type":"shipping"'* ]]; then
    echo '{"valid":true,"amount":4.75,"states":[{"code":"quoted","severity":"info","message":"external shipping quote"}]}'
    exit 0
  fi
  echo '{"valid":true,"amount":0,"states":[{"code":"quoted","severity":"info","message":"external payment quote"}]}'
  exit 0
fi
if [[ "$payload" == *'"provider_type":"payment"'* ]]; then
  echo '{"valid":true,"payment_display":"ExtPay •••• 9999"}'
  exit 0
fi
if [[ "$payload" == *'"provider_type":"shipping"'* ]]; then
  echo '{"valid":true,"shipping_address":"42 External Ave, Test City, US"}'
  exit 0
fi
echo '{"valid":true}'
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	paymentManifest := `{
  "id":"ext-pay",
  "type":"payment",
  "name":"Ext Pay",
  "description":"External payment plugin",
  "command":"./plugin.sh",
  "fields":[{"key":"token","label":"Token","type":"text","required":true}]
}`
	shippingManifest := `{
  "id":"ext-ship",
  "type":"shipping",
  "name":"Ext Ship",
  "description":"External shipping plugin",
  "command":"./plugin.sh",
  "fields":[{"key":"line1","label":"Line 1","type":"text","required":true}]
}`

	if err := os.WriteFile(filepath.Join(dir, "payment.json"), []byte(paymentManifest), 0o644); err != nil {
		t.Fatalf("write payment manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "shipping.json"), []byte(shippingManifest), 0o644); err != nil {
		t.Fatalf("write shipping manifest: %v", err)
	}

	manager := NewDefaultManager()
	loaded, err := manager.LoadExternalPluginsFromDir(dir)
	if err != nil {
		t.Fatalf("load manifests: %v", err)
	}
	if loaded != 2 {
		t.Fatalf("expected 2 plugins loaded, got %d", loaded)
	}

	quote := manager.Quote(QuoteRequest{
		Subtotal:     models.MoneyFromFloat(100),
		PaymentID:    "ext-pay",
		ShippingID:   "ext-ship",
		TaxID:        "client-selected-tax-provider-should-be-ignored",
		PaymentData:  map[string]string{"token": "tok_1"},
		ShippingData: map[string]string{"line1": "42 External Ave"},
		TaxData:      map[string]string{"state": "CA"},
	})
	if !quote.Valid {
		t.Fatalf("expected quote to be valid")
	}
	if quote.Shipping != models.MoneyFromFloat(4.75) {
		t.Fatalf("expected shipping from external plugin, got %s", quote.Shipping.String())
	}

	details, err := manager.ResolveCheckoutDetails(QuoteRequest{
		Subtotal:     models.MoneyFromFloat(100),
		PaymentID:    "ext-pay",
		ShippingID:   "ext-ship",
		TaxID:        "client-selected-tax-provider-should-be-ignored",
		PaymentData:  map[string]string{"token": "tok_1"},
		ShippingData: map[string]string{"line1": "42 External Ave"},
		TaxData:      map[string]string{"state": "CA"},
	})
	if err != nil {
		t.Fatalf("resolve details: %v", err)
	}
	if details.PaymentDisplay != "ExtPay •••• 9999" {
		t.Fatalf("unexpected payment display: %q", details.PaymentDisplay)
	}
	if details.ShippingAddress != "42 External Ave, Test City, US" {
		t.Fatalf("unexpected shipping address: %q", details.ShippingAddress)
	}
}

func TestExternalPluginArgsResolveRelativeToManifestDir(t *testing.T) {
	dir := t.TempDir()

	scriptPath := filepath.Join(dir, "plugin.sh")
	script := `#!/usr/bin/env bash
set -euo pipefail
payload="$(cat)"
if [[ "$payload" == *'"action":"quote"'* ]]; then
  echo '{"valid":true,"amount":6.25}'
  exit 0
fi
echo '{"valid":true,"shipping_address":"42 Relative Path Way, Test City, US"}'
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	manifest := `{
  "id":"ext-relative-ship",
  "type":"shipping",
  "name":"Ext Relative Ship",
  "description":"External shipping plugin with relative script path",
  "command":"sh",
  "args":["./plugin.sh"],
  "fields":[{"key":"line1","label":"Line 1","type":"text","required":true}]
}`
	if err := os.WriteFile(filepath.Join(dir, "shipping.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	manager := NewDefaultManager()
	loaded, err := manager.LoadExternalPluginsFromDir(dir)
	if err != nil {
		t.Fatalf("load manifests: %v", err)
	}
	if loaded != 1 {
		t.Fatalf("expected 1 plugin loaded, got %d", loaded)
	}

	quote := manager.Quote(QuoteRequest{
		Subtotal:   models.MoneyFromFloat(100),
		PaymentID:  "dummy-card",
		ShippingID: "ext-relative-ship",
		PaymentData: map[string]string{
			"cardholder_name": "Alex Tester",
			"card_number":     "4242424242424242",
			"exp_month":       "12",
			"exp_year":        "2099",
		},
		ShippingData: map[string]string{"line1": "42 Relative Path Way"},
		TaxData:      map[string]string{"state": "CA"},
	})
	if !quote.Valid {
		t.Fatalf("expected quote to be valid")
	}
	if quote.Shipping != models.MoneyFromFloat(6.25) {
		t.Fatalf("expected shipping from external plugin, got %s", quote.Shipping.String())
	}
}
