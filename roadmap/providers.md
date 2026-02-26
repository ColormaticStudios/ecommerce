**Current Baseline (repo-specific)**
- Checkout plugins currently support quote/resolve only (`internal/checkoutplugins`), and `POST /api/v1/me/orders/{id}/pay` is mock-like.
- `orders` has only high-level fields (`status`, `total`, display strings), no provider transaction ledger/history.
- No webhook ingestion/signature/idempotency pipeline yet.

**Priority Roadmap**

1. **P0: Payment correctness foundation (do first)**
- Add idempotency framework for inbound and outbound calls.
- Add trusted checkout snapshot + expiry.
- Add payment intent + transaction ledger tables.
- Add order state history/audit trail table.
- Add correlation IDs + structured logging baseline.

2. **P1: Payment lifecycle APIs (authorize/capture/void/refund)**
- Replace monolithic `/orders/{id}/pay` flow with explicit lifecycle operations.
- Implement provider adapter interface with `Authorize/Capture/Void/Refund`.
- Enforce single active payment intent per order and concurrency guards.

3. **P2: Webhooks + signature verification**
- Add `/api/v1/webhooks/{provider}` with per-provider signature verifier.
- Store raw webhook events, process asynchronously, idempotent by provider event ID.
- Handle retries/backoff + dead-letter for poison events.

4. **P3: Shipping and tax production model**
- Shipping: rate shopping, selected service persistence, label/tracking/events.
- Tax: line-level breakdown, inclusive/exclusive pricing flags, nexus/exemption config, finalization/export.

5. **P4: Security/ops hardening**
- Encrypted provider credentials, key rotation, sandbox/prod split.
- Multi-currency + FX policy + address normalization.
- Scheduled reconciliation jobs comparing local/payment/shipping/tax provider truth.

---

**Interfaces to Introduce**

```go
type PaymentProvider interface {
    Authorize(ctx context.Context, req AuthorizeRequest) (AuthorizeResult, error)
    Capture(ctx context.Context, req CaptureRequest) (CaptureResult, error)
    Void(ctx context.Context, req VoidRequest) (VoidResult, error)
    Refund(ctx context.Context, req RefundRequest) (RefundResult, error)
    VerifyWebhook(ctx context.Context, sigHeaders map[string]string, body []byte) (WebhookEvent, error)
    GetTransaction(ctx context.Context, providerTxnID string) (ProviderTransaction, error)
}
```

```go
type ShippingProvider interface {
    QuoteRates(ctx context.Context, req ShippingQuoteRequest) ([]ShippingRate, error)
    BuyLabel(ctx context.Context, req BuyLabelRequest) (Shipment, error)
    VerifyWebhook(ctx context.Context, headers map[string]string, body []byte) (ShipmentEvent, error)
}
```

```go
type TaxProvider interface {
    QuoteTax(ctx context.Context, req TaxQuoteRequest) (TaxQuote, error)
    FinalizeTax(ctx context.Context, req TaxFinalizeRequest) (TaxFinalized, error)
    ExportReport(ctx context.Context, req TaxExportRequest) (io.ReadCloser, error)
}
```

```go
type IdempotencyStore interface {
    Execute(ctx context.Context, scope, key string, fn func() (any, error)) (any, error)
}
```

---

**DB Tables / Changes (priority order)**

1. `idempotency_keys`
- `scope`, `idempotency_key`, `request_hash`, `status`, `response_code`, `response_body`, `expires_at`
- Unique index: `(scope, idempotency_key)`

2. `order_checkout_snapshots` + `order_checkout_snapshot_items`
- Snapshot of line items, shipping/tax inputs, totals, currency, expiry, provider selections
- Enforce payment/capture against snapshot ID, not live cart/order mutation

3. `payment_intents`
- `order_id`, `snapshot_id`, `provider`, `status`, `authorized_amount`, `captured_amount`, `currency`, `version`
- Unique: one active intent per order

4. `payment_transactions`
- `payment_intent_id`, `operation` (`authorize|capture|void|refund`), `provider_txn_id`, `amount`, `status`, `raw_response_redacted`
- Unique guards for double-capture/refund (e.g. `(payment_intent_id, operation, idempotency_key)`)

5. `order_status_history`
- `order_id`, `from_status`, `to_status`, `reason`, `source` (`api|webhook|job`), `actor`, `correlation_id`

6. `webhook_events`
- `provider`, `provider_event_id`, `event_type`, `signature_valid`, `payload`, `received_at`, `processed_at`, `attempt_count`, `last_error`
- Unique `(provider, provider_event_id)`

7. `provider_call_audit`
- Outbound request/response metadata, latency, correlation, redacted payloads

8. Shipping tables
- `shipments`, `shipment_rates`, `shipment_packages`, `shipment_events`, `tracking_events`

9. Tax tables
- `order_tax_lines` (line-level), `tax_jurisdiction_rules`, `tax_nexus_configs`, `tax_exports`

10. Credentials/secrets
- `provider_credentials` with encrypted blob, key version, env (`sandbox|production`), tenant/store scope

---

**Endpoint Changes**

1. **Replace/extend payment APIs**
- Keep `POST /api/v1/me/orders/{id}/pay` temporarily as compatibility wrapper.
- Add:
  - `POST /api/v1/me/orders/{id}/payments/authorize`
  - `POST /api/v1/admin/orders/{id}/payments/{intentId}/capture`
  - `POST /api/v1/admin/orders/{id}/payments/{intentId}/void`
  - `POST /api/v1/admin/orders/{id}/payments/{intentId}/refund`
  - `GET /api/v1/admin/orders/{id}/payments` (ledger/history)

2. **Checkout snapshot**
- `POST /api/v1/me/checkout/quote` -> return `snapshot_id`, `expires_at`, totals, selected provider metadata.
- Payment authorize requires `snapshot_id`.

3. **Webhooks**
- `POST /api/v1/webhooks/{provider}` (public, signature required)
- `GET /api/v1/admin/webhooks/events` (debug/replay status)

4. **Shipping**
- `POST /api/v1/me/orders/{id}/shipping/rates`
- `POST /api/v1/admin/orders/{id}/shipping/labels`
- `GET /api/v1/me/orders/{id}/shipping/tracking`

5. **Tax**
- `POST /api/v1/me/orders/{id}/tax/finalize`
- `GET /api/v1/admin/tax/reports/export`

6. **Idempotency**
- Require `Idempotency-Key` header for mutate endpoints above.

---

**Execution Plan in This Repo**
1. Update `api/openapi.yaml` first for P0+P1 surfaces.
2. Add models + `AutoMigrate` entries.
3. Implement services under `internal/` (`payments`, `idempotency`, `webhooks`, `shipping`, `tax`).
4. Wire handlers in generated server adapter.
5. Regenerate contract (`make openapi-gen`), then verify (`make openapi-check`).
6. Add integration tests:
- duplicate idempotency key returns cached response
- retry-safe authorize/capture/refund
- webhook replay no-op (idempotent)
- snapshot expiry and amount mismatch rejection
- race test for double capture/refund prevention