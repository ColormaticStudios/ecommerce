# Provider Roadmap

## Current Baseline
- Checkout plugins currently support quote/resolve only (`internal/checkoutplugins`).
- `POST /api/v1/me/orders/{id}/pay` is effectively a placeholder flow, not a full payment lifecycle.
- `orders` stores high-level values only (`status`, totals, display fields); there is no provider transaction ledger.
- No webhook ingestion pipeline exists yet (signature verification, replay protection, async retries, dead-letter handling).

## Goals
- Make payment operations correct under retries, concurrency, and partial failures.
- Introduce provider-agnostic interfaces for payment, shipping, and tax.
- Establish auditable state transitions and transaction history.
- Create safe operational foundations: idempotency, secrets handling, reconciliation, observability.

## Non-Goals (for this roadmap)
- Building a full accounting system.
- Supporting every provider feature from day one.
- Solving cross-store marketplace settlement in initial phases.

## Delivery Order (Strict)
1. P0: Correctness foundation.
2. P1: Payment lifecycle APIs.
3. P2: Webhooks and event processing.
4. P3: Shipping and tax production model.
5. P4: Security and operational hardening.

## Cross-Roadmap Alignment
- Guest checkout:
  - Use checkout-session routes from `roadmap/guest-checkout.md` (`/api/v1/checkout/*`) for customer-facing mutations.
  - Keep `/api/v1/me/orders*` as read/account surfaces only.
- Catalog depth:
  - Snapshot and payment amounts are based on variant-backed order items (`product_variant_id`) from `roadmap/product-catalog-depth.md`.
- Discounts/promotions:
  - Snapshot totals include applied campaign/level adjustments from `roadmap/discounts-promotions.md`.

## P0: Payment Correctness Foundation
### Scope
- Add idempotency for inbound mutate APIs and outbound provider calls.
- Add trusted checkout snapshot with expiry.
- Introduce payment intent + transaction ledger tables.
- Add order status history/audit trail table.
- Add correlation IDs and structured logs for all payment requests.

### Deliverables
- OpenAPI updates for snapshot + idempotency-aware payment initiation.
- Schema migrations for idempotency keys, snapshots, intents, transactions, status history.
- Service layer that enforces:
  - one active intent per order,
  - snapshot binding on authorization,
  - deterministic handling of duplicate requests.

### Done Criteria
- Duplicate `Idempotency-Key` returns the same status/body.
- Expired snapshot authorization is rejected.
- Attempted payment against changed order totals is rejected.
- Logs for each payment request include `correlation_id`, `order_id`, `intent_id` (if present).

## P1: Payment Lifecycle APIs
### Scope
- Replace monolithic pay flow with explicit operations:
  - authorize,
  - capture,
  - void,
  - refund.
- Introduce provider adapter interface for lifecycle operations.
- Add concurrency guards for double capture/refund.

### Deliverables
- Compatibility wrapper for `POST /api/v1/checkout/orders/{id}/submit-payment` (temporary).
- New lifecycle endpoints and admin ledger endpoint.
- Provider abstraction with stable request/response models.

### Done Criteria
- Lifecycle operations are fully idempotent.
- Double-capture and duplicate-refund attempts are blocked safely.
- Ledger endpoint returns complete transaction history per order.

## P2: Webhooks and Signature Verification
### Scope
- Add `POST /api/v1/webhooks/{provider}` endpoint.
- Verify provider signatures per provider implementation.
- Persist raw webhook event records.
- Process events asynchronously with retry/backoff and dead-letter path.

### Deliverables
- Webhook event store with unique `(provider, provider_event_id)`.
- Worker/job pipeline for webhook processing.
- Admin endpoint for event inspection/replay status.

### Done Criteria
- Replayed provider event is a no-op.
- Invalid signatures are rejected and logged.
- Poison events are visible with attempt count and last error.

## P3: Shipping and Tax Production Model
### Shipping Scope
- Rate shopping and selected service persistence.
- Label purchase and tracking events ingestion.

### Tax Scope
- Line-level tax breakdown.
- Inclusive/exclusive pricing support.
- Nexus/exemption configuration and finalize/export flows.

### Done Criteria
- Chosen shipping service is immutable for a finalized shipment.
- Tracking events update shipment state idempotently.
- Tax finalization stores line-level result tied to the order snapshot.

## P4: Security and Ops Hardening
### Scope
- Encrypted provider credentials with key versioning and rotation path.
- Sandbox/production credential separation.
- Multi-currency and FX policy decisions.
- Scheduled reconciliation jobs comparing local state vs provider truth.

### Done Criteria
- Credentials are never logged in plain text.
- Reconciliation jobs can detect and report drift (payment/shipping/tax).
- Runbooks exist for webhook outage and reconciliation mismatch.

## Interfaces to Introduce

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

## Data Model Changes (Priority Order)
1. `idempotency_keys`
- Fields: `scope`, `idempotency_key`, `request_hash`, `status`, `response_code`, `response_body`, `expires_at`
- Unique index: `(scope, idempotency_key)`

2. `order_checkout_snapshots` and `order_checkout_snapshot_items`
- Snapshot line items, shipping/tax inputs, totals, currency, expiry, selected providers.
- Payment/authorization references snapshot ID, never mutable live cart state.

3. `payment_intents`
- Fields: `order_id`, `snapshot_id`, `provider`, `status`, `authorized_amount`, `captured_amount`, `currency`, `version`
- Constraint: single active intent per order.

4. `payment_transactions`
- Fields: `payment_intent_id`, `operation`, `provider_txn_id`, `amount`, `status`, `raw_response_redacted`
- Duplicate guard: `(payment_intent_id, operation, idempotency_key)`.

5. `order_status_history`
- Fields: `order_id`, `from_status`, `to_status`, `reason`, `source`, `actor`, `correlation_id`.

6. `webhook_events`
- Fields: `provider`, `provider_event_id`, `event_type`, `signature_valid`, `payload`, `received_at`, `processed_at`, `attempt_count`, `last_error`
- Unique index: `(provider, provider_event_id)`.

7. `provider_call_audit`
- Outbound request/response metadata, latency, correlation, redacted payloads.

8. Shipping tables
- `shipments`, `shipment_rates`, `shipment_packages`, `shipment_events`, `tracking_events`.

9. Tax tables
- `order_tax_lines`, `tax_jurisdiction_rules`, `tax_nexus_configs`, `tax_exports`.

10. `provider_credentials`
- Encrypted secret blob, key version, environment (`sandbox|production`), tenant/store scope.

## Endpoint Plan
1. Payment lifecycle
- Keep `POST /api/v1/checkout/orders/{id}/submit-payment` as a compatibility wrapper (temporary).
- Add:
  - `POST /api/v1/checkout/orders/{id}/payments/authorize`
  - `POST /api/v1/admin/orders/{id}/payments/{intentId}/capture`
  - `POST /api/v1/admin/orders/{id}/payments/{intentId}/void`
  - `POST /api/v1/admin/orders/{id}/payments/{intentId}/refund`
  - `GET /api/v1/admin/orders/{id}/payments`

2. Checkout snapshot
- `POST /api/v1/checkout/quote` returns `snapshot_id`, `expires_at`, totals, provider metadata.
- Authorization must provide `snapshot_id`.

3. Webhooks
- `POST /api/v1/webhooks/{provider}` (public, signature required).
- `GET /api/v1/admin/webhooks/events` (inspection/replay status).

4. Shipping
- `POST /api/v1/checkout/orders/{id}/shipping/rates`
- `POST /api/v1/admin/orders/{id}/shipping/labels`
- `GET /api/v1/checkout/orders/{id}/shipping/tracking`

5. Tax
- `POST /api/v1/checkout/orders/{id}/tax/finalize`
- `GET /api/v1/admin/tax/reports/export`

6. Idempotency policy
- Require `Idempotency-Key` on all mutation endpoints listed above.

## Execution Workflow in This Repo
1. Update `api/openapi.yaml` first (required for any API shape changes).
2. Run `make openapi-gen` and commit generated artifacts.
3. Add/adjust models and `AutoMigrate` entries.
4. Implement services under `internal/` (`payments`, `idempotency`, `webhooks`, `shipping`, `tax`).
5. Wire handlers in the generated server adapter layer.
6. Run `make openapi-check`.
7. Add integration tests for:
- idempotency replay returns cached response,
- retry-safe authorize/capture/refund,
- webhook replay idempotency,
- snapshot expiry and amount mismatch rejection,
- race safety for double capture/refund.

## Risk Register
- Provider behavior differences (partial capture/refund semantics) can leak into abstractions.
- Missing reconciliation early can hide silent drift.
- If webhook processing is synchronous, provider retries can amplify outages.
- If snapshot and order mutation boundaries are unclear, payment correctness will regress.

## Immediate Next Slice (Suggested)
1. Ship P0 schema + idempotency middleware.
2. Add checkout snapshot endpoint and authorization binding.
3. Introduce payment intent/transactions tables and ledger read endpoint.
