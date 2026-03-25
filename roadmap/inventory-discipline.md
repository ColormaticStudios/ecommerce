# Inventory Discipline Roadmap

## Current Baseline
- Inventory is represented as a single integer field `models.Product.Stock` in `models/product.go`.
- Stock checks happen in cart/order flows and stock mutations are tied to order status transitions:
  - Validation in `handlers/cart.go` and order handlers.
  - Deduct/replenish behavior in `internal/services/orders/service.go`.
- There is no explicit reservation model, so the same `stock` field is used as both sellable availability and physical on-hand proxy.
- There is no first-class inventory ledger/audit table for manual adjustments, order allocations, or restocks.
- There is no purchase order workflow or supplier object in API/UI.
- Low-stock messaging exists in storefront/admin UI, but there is no alerting workflow, threshold policy, acknowledgment, or task queue.

## Goals
- Separate inventory quantities into explicit buckets (on-hand, reserved, available) with deterministic formulas.
- Prevent oversells under concurrent checkout/order mutations.
- Introduce inventory reservations with lifecycle controls (create, confirm/consume, release, expire).
- Add low-stock alerts with configurable thresholds and operational acknowledgment.
- Add purchase order and restock receiving workflows.
- Add audited inventory adjustments with immutable history and reason codes.
- Keep handlers thin by implementing inventory behavior in `internal/services/inventory`.

## Non-Goals
- Supplier EDI/ERP integrations in initial phases.
- Forecasting/ML demand planning.
- Multi-tenant inventory sharding beyond current single-store assumptions.
- Warehouse robotics/bin-scanning integrations.

## Delivery Order
1. P0: Inventory quantity model + ledger foundation.
2. P1: Reservation lifecycle and oversell prevention.
3. P2: Low-stock alerting and operational triage.
4. P3: Purchase orders and receiving workflows.
5. P4: Audited manual adjustments, reconciliation, and hardening.

## Cross-Roadmap Alignment
- Order fulfillment (`roadmap/order-fulfillment-ops.md`):
  - Inventory reservation records become the source of truth for `ALLOCATED` and shipment consumption events.
  - Allocation identity is `product_variant_id` from P0 onward.
- Product catalog depth baseline:
  - Inventory identity must align with the implemented variant-first catalog contracts (`product_variant_id`) and should not add new `product_id`-based ownership paths.
- Checkout baseline:
  - Reservation ownership should support both `user_id` and checkout session/order identity, including guest-origin orders.
- Provider platform baseline:
  - No pricing/tax side effects; inventory mutations remain internal and independent of payment/shipping/tax providers.

## P0: Quantity Model and Ledger Foundation
### Scope
- Introduce canonical inventory quantity buckets and immutable movement ledger.
- Replace product-level stock ownership with variant-level inventory as the canonical model.

### Deliverables
- New tables/models:
  - `inventory_items` (`product_variant_id` canonical).
  - `inventory_levels` (`on_hand`, `reserved`, `available` as persisted or derived fields).
  - `inventory_movements` (append-only ledger with `movement_type`, `quantity_delta`, `reference_type`, `reference_id`, `reason_code`, actor metadata).
- Inventory service in `internal/services/inventory`:
  - `GetAvailability(product_variant_id)`.
  - `ApplyMovement(...)` with transaction + row locking.
  - Invariant checks (`on_hand >= 0`, `reserved >= 0`, `available = on_hand - reserved`).
- Contract cut:
- Replace legacy product-level `stock` response dependency with variant availability surfaces in the same phase.

### Done Criteria
- Every stock mutation path writes an `inventory_movements` row.
- Inventory invariants are enforced in service-level transactions.
- OpenAPI and frontend consumers compile against variant-first inventory fields without legacy stock adapters.
- Tests cover movement append behavior and invariant rejection for invalid deltas.

## P1: Reservations and Oversell Prevention
### Scope
- Add reservation lifecycle for cart/checkout/order placement.
- Ensure concurrent purchase attempts cannot oversell.

### Deliverables
- New table/model:
  - `inventory_reservations` (`status`: `ACTIVE`, `CONSUMED`, `RELEASED`, `EXPIRED`; quantity; expiration timestamp; owner fields).
- Reservation workflows:
  - Reserve on checkout confirmation (or explicit pre-auth step).
  - Consume reservation on successful order payment/commit.
  - Release on cancellation/failure/expiry.
- Concurrency controls:
  - `SELECT ... FOR UPDATE` style locking or equivalent in inventory service.
  - Idempotency keys on reservation create/consume/release mutations.
- API additions (admin and internal mutation surfaces):
  - Reservation inspect/list endpoint for troubleshooting.
  - Optional reservation refresh endpoint for checkout extension.

### Done Criteria
- Parallel checkout attempts for the same item cannot drive `available` below zero.
- Duplicate consume/release requests are idempotent.
- Expired reservations are automatically released by background worker.
- Integration tests validate high-contention purchase scenarios and rollback safety.

## P2: Low-Stock Alerts
### Scope
- Add low-stock threshold configuration and alert lifecycle.
- Provide actionable alerts instead of only passive UI labels.

### Deliverables
- New tables/models:
  - `inventory_thresholds` (default + per-product override).
  - `inventory_alerts` (`LOW_STOCK`, `OUT_OF_STOCK`, `RECOVERY`; status `OPEN`, `ACKED`, `RESOLVED`).
- Alert generation:
  - Trigger on movement/reservation events crossing thresholds.
  - De-duplicate repeated alerts for same product/state window.
- Admin APIs/UI:
  - List/acknowledge/resolve alerts.
  - Configure thresholds.
- Notification hooks:
  - Structured event output for email/webhook notifier integration (implementation can stay internal first).

### Done Criteria
- Threshold crossing generates exactly one open alert until resolved/recovered.
- Admin can acknowledge and resolve alerts with actor attribution.
- Recovery events close prior low-stock/out-of-stock alerts.
- Tests cover threshold up/down crossings and deduplication behavior.

## P3: Purchase Orders and Restock Receiving
### Scope
- Implement supplier purchase order lifecycle and receiving into on-hand inventory.
- Tie receiving to ledger and alert recovery.

### Deliverables
- New tables/models:
  - `suppliers`.
  - `purchase_orders` and `purchase_order_items`.
  - `inventory_receipts` and `inventory_receipt_items`.
- PO lifecycle states:
  - `DRAFT`, `ISSUED`, `PARTIALLY_RECEIVED`, `RECEIVED`, `CANCELLED`.
- Receiving workflow:
  - Record received quantity per line.
  - Generate `RESTOCK_RECEIPT` movements.
  - Optionally auto-release backorder/reservation queues (when available in fulfillment integration).
- Admin APIs/UI:
  - Create/edit/issue PO.
  - Receive against PO with partial receipts.

### Done Criteria
- Receiving updates `on_hand` and `available` immediately and writes immutable receipt + movement records.
- PO cannot be marked `RECEIVED` unless all open quantities are accounted for.
- Partial receipt flows are supported without data loss.
- Integration tests cover draft->issued->partial->received and cancel flows.

## P4: Audited Adjustments and Reconciliation Hardening
### Scope
- Add strict adjustment controls and periodic reconciliation.
- Improve operational confidence and incident debugging.

### Deliverables
- New table/model:
  - `inventory_adjustments` with required reason code, notes, actor, and approval metadata (if approval policy enabled).
- Adjustment types:
  - `CYCLE_COUNT_GAIN`, `CYCLE_COUNT_LOSS`, `DAMAGE`, `SHRINKAGE`, `RETURN_RESTOCK`, `CORRECTION`.
- Reconciliation jobs:
  - Compare derived balances from `inventory_movements` with materialized levels.
  - Detect reservation drift (stuck ACTIVE reservations past expiry SLA).
- Admin diagnostics:
  - Inventory timeline endpoint (movements + reservations + adjustments per item).

### Done Criteria
- Manual quantity edits cannot occur outside adjustment APIs.
- Every adjustment event has immutable actor/reason/timestamp trail.
- Reconciliation job reports mismatches with actionable entity IDs.
- Tests cover invalid reason rejection, approval policy enforcement, and reconciliation mismatch detection.

## Data Model Changes
1. `inventory_items`
- Identity row for tracked sellable item by `product_variant_id`.

2. `inventory_levels`
- Snapshot quantities per item: `on_hand`, `reserved`, `available`, timestamps.

3. `inventory_movements`
- Immutable ledger with directional deltas and business reference metadata.

4. `inventory_reservations`
- Reservation ownership and lifecycle, including expiry.

5. `inventory_thresholds` / `inventory_alerts`
- Low-stock policy and event tracking.

6. `suppliers`, `purchase_orders`, `purchase_order_items`, `inventory_receipts`
- Restock procurement and receiving records.

7. `inventory_adjustments`
- Audited manual correction surface.

## Endpoint/API Plan
1. Inventory admin endpoints (new)
- `GET /api/v1/admin/inventory/items/{id}`: level + recent movement summary.
- `GET /api/v1/admin/inventory/items/{id}/timeline`: movements/reservations/adjustments.
- `POST /api/v1/admin/inventory/reservations` and status mutation endpoints (consume/release/expire) for controlled workflows.
- `GET /api/v1/admin/inventory/alerts` + `POST /api/v1/admin/inventory/alerts/{id}/ack` + `POST /api/v1/admin/inventory/alerts/{id}/resolve`.
- `POST /api/v1/admin/purchase-orders`, `POST /api/v1/admin/purchase-orders/{id}/issue`, `POST /api/v1/admin/purchase-orders/{id}/receive`.
- `POST /api/v1/admin/inventory/adjustments`.

2. Contract generation
- Update `api/openapi.yaml` first for all new request/response schemas.
- Regenerate:
  - `internal/apicontract/openapi.gen.go`
  - `frontend/src/lib/api/generated/openapi.ts`

## Execution Workflow in This Repo
1. Update `api/openapi.yaml` first for each phase that changes payloads.
2. Run `make openapi-gen`.
3. Implement backend models/migrations:
- Add migrations in `internal/migrations` and register model updates.
- Implement reusable services in `internal/services/inventory`.
- Keep HTTP handlers thin in `handlers/`.
4. Implement frontend admin/storefront changes in `frontend/src/` using generated types.
5. Run `make openapi-check`.
6. Run formatters for touched files:
- Backend: `gofmt -w <file>` (or `gofmt -w .` for broad updates).
- Frontend: `cd frontend && bun x prettier -w <file>`.
7. Run tests for touched areas:
- Backend: `GOCACHE=/tmp/go-build go test ./...` (or targeted packages first).
- Frontend: `cd frontend && bun run check && bun run lint`.

## Risk Register
- Migration risk: moving from single `products.stock` to service-driven quantities can break storefront/admin assumptions.
- Contention risk: reservation locking strategy may reduce throughput if lock scope is too coarse.
- Drift risk: background expiry/reconciliation failures can leave stale reservations and suppressed availability.
- Contract break risk: existing tests and API clients may depend on product-level stock fields and status-tied decrement timing.

## Immediate Next Slice
1. Implement P0 schema + service skeleton only:
- Add `inventory_items`, `inventory_levels`, `inventory_movements` migrations and models.
- Introduce `internal/services/inventory` with invariant-checked movement API.
2. Route existing order stock mutations through inventory service using `product_variant_id` ownership.
3. Add backend tests for movement append, invariant checks, and variant-level availability reads.
4. Defer reservation endpoints until P1 after P0 variant inventory model is stable.
