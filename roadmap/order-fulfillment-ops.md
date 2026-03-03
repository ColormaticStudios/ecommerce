# Order Fulfillment Operations Roadmap

## Current Baseline
- Order lifecycle is single-field and coarse in `models.Order.Status`:
  - `PENDING`, `PAID`, `FAILED`, `SHIPPED`, `DELIVERED`, `CANCELLED`, `REFUNDED`.
- Stock commitment and release are tied directly to order status transitions in `internal/services/orders/service.go`:
  - Stock is deducted when moving into a stock-committed status (`PAID`, `SHIPPED`, `DELIVERED`).
  - Stock is replenished when leaving those statuses.
- Fulfillment is not first-class:
  - No pick/pack/ship workflow objects.
  - No partial shipment model.
  - No backorder allocation model.
  - No warehouse/location model.
- Admin can patch order status (`PATCH /api/v1/admin/orders/{id}/status`) but there is no transition guardrail table or audit history for fulfillment sub-steps.
- This project is pre-release, so breaking schema/API changes are acceptable.

## Goals
- Introduce a production-grade fulfillment domain for pick/pack/ship operations.
- Support partial shipments for split fulfillment and mixed in-stock/backordered orders.
- Support backorder lifecycle with explicit promise/availability tracking.
- Replace coarse status semantics with deterministic order + fulfillment transition rules.
- Add warehouse/location inventory ownership and reservation support.
- Keep handler layers thin and move fulfillment behavior to reusable `internal/` services.

## Non-Goals
- Carrier procurement optimization (rate shopping intelligence) beyond basic provider hooks.
- Warehouse robotics/WMS device protocol integration in first cut.
- Full returns/RMA workflow (handled in a dedicated roadmap after outbound fulfillment stabilization).
- Marketplace multi-merchant fulfillment routing.

## Delivery Order
1. P0: Fulfillment domain foundation (entities, states, invariants).
2. P1: Pick/pack/ship execution and shipment creation.
3. P2: Partial shipments and backorder lifecycle.
4. P3: Warehouse/location inventory and allocation engine.
5. P4: Hardening, observability, and operational controls.

## Cross-Roadmap Alignment
- Guest checkout:
  - Fulfillment consumes canonical checkout order records from `roadmap/guest-checkout.md` (`/api/v1/checkout/*` for customer mutations).
  - Customer order status reads remain compatible with guest/auth order ownership model.
- Catalog depth:
  - Fulfillment line identity must use `product_variant_id` from `roadmap/product-catalog-depth.md` rather than `product_id`.
  - Inventory reservation/availability is variant-based.
- Providers:
  - Shipping label purchase/tracking should align with provider abstractions in `roadmap/providers.md`.
  - Shipment event ingestion should reuse provider webhook reliability primitives.
- Discounts/promotions:
  - Fulfillment quantities and shipment value views use already-finalized order totals; no fulfillment-time repricing.

## P0: Fulfillment Domain Foundation
### Scope
- Add first-class fulfillment entities and normalize status responsibilities:
  - `orders` captures commercial/payment status.
  - `fulfillment_orders` captures fulfillment execution readiness.
  - `fulfillment_shipments` captures outbound parcel/consignment lifecycle.
- Define explicit, enforceable transition graph for fulfillment status fields.
- Add immutable status history for order and fulfillment transitions.

### Deliverables
- New fulfillment state enums and transition validation layer in `internal/services/fulfillment`.
- New models/tables:
  - `fulfillment_orders`
  - `fulfillment_order_items`
  - `fulfillment_status_history`
- `orders.status` redesign (breaking):
  - Replace shipping-driven states with commercial states only:
    - `DRAFT`, `PLACED`, `PAYMENT_PENDING`, `PAID`, `CANCELLED`, `REFUNDED`.
  - Move shipping/progress semantics to fulfillment entities.
- Migration/backfill plan:
  - Existing `orders.status=SHIPPED|DELIVERED` backfilled to `orders.status=PAID` plus derived fulfillment rows where possible.
  - Existing orders without enough detail default to `fulfillment_status=UNFULFILLED`.

### Done Criteria
- Invalid fulfillment transition attempts are rejected consistently by service-layer guards.
- Existing order status patch endpoint cannot bypass fulfillment transition rules.
- Order and fulfillment status histories are persisted for every accepted transition.
- Regression tests cover valid/invalid transition matrices.

## P1: Pick/Pack/Ship Execution
### Scope
- Model warehouse execution steps:
  - pick task creation/assignment,
  - packing completion,
  - shipment creation and tracking attachment.
- Add admin operations to advance fulfillment workflow per fulfillment order.

### Deliverables
- New tables/models:
  - `pick_tasks`
  - `pack_tasks`
  - `fulfillment_shipments`
  - `shipment_items`
  - `shipment_tracking_events`
- Status model:
  - `fulfillment_orders.status`: `UNFULFILLED`, `ALLOCATED`, `PICKING`, `PACKING`, `READY_TO_SHIP`, `PARTIALLY_SHIPPED`, `SHIPPED`, `CANCELLED`.
  - `fulfillment_shipments.status`: `LABEL_PURCHASED`, `IN_TRANSIT`, `DELIVERED`, `EXCEPTION`, `RETURNED`.
- API additions (admin-focused):
  - Allocate fulfillment order.
  - Start/complete pick.
  - Start/complete pack.
  - Create shipment + assign shipped quantities.
  - Record tracking event.

### Done Criteria
- A fulfillment order cannot move to `PACKING` unless pick is completed.
- Shipment creation requires packed quantities and prevents over-shipment.
- Tracking events are idempotent by provider event ID.
- Admin can view fulfillment order with task and shipment timeline in one response.

## P2: Partial Shipments and Backorders
### Scope
- Permit one order to spawn multiple shipments over time.
- Split line quantities into:
  - allocated/in-pick/in-pack/shipped,
  - backordered.
- Add backorder promise and release flow.

### Deliverables
- New tables/models:
  - `backorder_lines`
  - `backorder_events`
  - `inventory_reservations`
- Allocation algorithm updates:
  - Allow partial allocation when full line quantity is unavailable.
  - Create backorder line for remainder with ETA metadata.
- Customer-facing order fulfillment summary:
  - Show shipped vs backordered quantities per line.
  - Show estimated ship date for backordered quantities when known.
- New transition rules:
  - `PARTIALLY_SHIPPED` when at least one shipment exists but unshipped quantity remains.
  - `SHIPPED` only when all fulfillable quantities are shipped or explicitly cancelled.

### Done Criteria
- One order with mixed stock can generate first shipment while remainder is backordered.
- Backorder release can allocate newly available stock without recreating order lines.
- Duplicate allocation/release requests are idempotent.
- Integration tests cover:
  - full in-stock shipment,
  - mixed partial+backorder shipment,
  - cancellation of remaining backordered quantity.

## P3: Warehouse and Location Support
### Scope
- Introduce multi-warehouse inventory ownership.
- Support location-aware allocation and picking.
- Add explicit transfer and reservation semantics.

### Deliverables
- New tables/models:
  - `warehouses`
  - `warehouse_locations`
  - `inventory_levels` (variant x location)
  - `inventory_movements`
  - `inventory_transfers`
- Fulfillment routing service:
  - Chooses source warehouse/location by policy (priority list, nearest, manual override).
  - Persists chosen source per fulfillment order item.
- Admin APIs:
  - CRUD warehouses/locations.
  - Adjust/move stock with audit trail.
  - Reassign fulfillment source before pick starts.

### Done Criteria
- Fulfillment allocations reference concrete `warehouse_location_id`.
- Pick tasks include source location and fail if stock moved below reserved quantity.
- Inventory movement ledger can reconstruct on-hand and reserved quantities.
- Concurrency tests verify no double-reservation under parallel allocations.

## P4: Hardening and Ops
### Scope
- Reliability, observability, and operational controls for fulfillment at scale.
- Reconciliation jobs for order vs fulfillment vs inventory consistency.

### Deliverables
- Idempotency keys for admin mutate endpoints affecting allocation/shipment.
- Scheduled reconciliation workers:
  - Reserved vs shipped quantity integrity checks.
  - Backorder stale-state detection.
  - Shipment status drift against provider truth.
- Metrics/logging:
  - Pick latency, pack latency, time-to-first-shipment, backorder aging.
  - Transition failure counts by reason code.
- Runbooks and admin diagnostics endpoints.

### Done Criteria
- Reconciliation detects and reports mismatches with actionable entity IDs.
- Duplicate shipment mutation requests return stable results.
- Operational dashboards expose backlog by warehouse and fulfillment status.
- Failure-injection tests validate retry and idempotency behavior.

## Data Model Changes
1. `orders` (breaking status simplification)
- Replace shipping-oriented status values with payment/commercial lifecycle values.
- Keep totals/snapshot identity untouched.

2. `fulfillment_orders`
- Fields: `id`, `order_id`, `status`, `priority`, `allocated_at`, `picked_at`, `packed_at`, `closed_at`, timestamps.

3. `fulfillment_order_items`
- Fields: `id`, `fulfillment_order_id`, `order_item_id`, `product_variant_id`, `qty_ordered`, `qty_allocated`, `qty_shipped`, `qty_backordered`, `warehouse_location_id`.

4. `pick_tasks`
- Fields: `id`, `fulfillment_order_id`, `status`, `assignee`, `started_at`, `completed_at`.

5. `pack_tasks`
- Fields: `id`, `fulfillment_order_id`, `status`, `assignee`, `started_at`, `completed_at`.

6. `fulfillment_shipments`
- Fields: `id`, `fulfillment_order_id`, `carrier`, `service_level`, `tracking_number`, `provider_shipment_id`, `status`, `label_url`, `shipped_at`, `delivered_at`.

7. `shipment_items`
- Fields: `id`, `shipment_id`, `order_item_id`, `product_variant_id`, `quantity`.

8. `backorder_lines`
- Fields: `id`, `fulfillment_order_item_id`, `quantity`, `status`, `expected_ship_date`, `reason_code`.

9. `warehouses` / `warehouse_locations`
- Warehouse metadata and location hierarchy/bin identifiers.

10. `inventory_levels` / `inventory_reservations` / `inventory_movements`
- Per-variant per-location on-hand/reserved tracking and immutable movement ledger.

11. `fulfillment_status_history`
- Fields: `entity_type`, `entity_id`, `from_status`, `to_status`, `reason`, `source`, `actor`, `correlation_id`, `created_at`.

## Endpoint/API Plan
1. Order + fulfillment read model
- Extend order response with `fulfillment_summary`:
  - `fulfillment_status`
  - `total_qty`, `shipped_qty`, `backordered_qty`
  - `shipments[]` summary
- Add admin fulfillment detail endpoint:
  - `GET /api/v1/admin/orders/{id}/fulfillment`

2. Fulfillment execution endpoints (admin)
- `POST /api/v1/admin/orders/{id}/fulfillment/allocate`
- `POST /api/v1/admin/orders/{id}/fulfillment/pick/start`
- `POST /api/v1/admin/orders/{id}/fulfillment/pick/complete`
- `POST /api/v1/admin/orders/{id}/fulfillment/pack/start`
- `POST /api/v1/admin/orders/{id}/fulfillment/pack/complete`
- `POST /api/v1/admin/orders/{id}/fulfillment/shipments`
- `POST /api/v1/admin/shipments/{shipmentId}/tracking-events`

3. Backorder endpoints
- `POST /api/v1/admin/orders/{id}/fulfillment/backorders/{backorderId}/release`
- `POST /api/v1/admin/orders/{id}/fulfillment/backorders/{backorderId}/cancel`

4. Warehouse/location endpoints
- `GET /api/v1/admin/warehouses`
- `POST /api/v1/admin/warehouses`
- `POST /api/v1/admin/warehouses/{id}/locations`
- `POST /api/v1/admin/inventory/adjustments`
- `POST /api/v1/admin/inventory/transfers`

5. Breaking compatibility policy
- Remove use of `PATCH /api/v1/admin/orders/{id}/status` for shipping progress once fulfillment endpoints land.
- Keep temporary mapping in one phase only:
  - `PAID -> UNFULFILLED`
  - `SHIPPED -> SHIPPED`
  - `DELIVERED -> SHIPPED + delivered shipment event`

## Execution Workflow in This Repo
1. Update `api/openapi.yaml` first for fulfillment endpoints and schema changes.
2. Run `make openapi-gen` and commit generated files:
- `internal/apicontract/openapi.gen.go`
- `frontend/src/lib/api/generated/openapi.ts`
3. Add/reshape models in `models/` following existing conventions (`BaseModel`, `Money` where relevant).
4. Add migration steps in `internal/migrations` including backfills from legacy order statuses.
5. Implement fulfillment services in `internal/services/fulfillment`; keep `handlers/` thin.
6. Update `handlers/orders_*` to read from fulfillment projections rather than coarse `orders.status` shipping semantics.
7. Run `make openapi-check`.
8. Run backend tests with sandbox cache:
- `GOCACHE=/tmp/go-build go test ./...`
9. Run frontend checks for touched API/UI paths:
- `cd frontend && bun run check && bun run lint`

## Risk Register
- Status migration risk:
  - Legacy `SHIPPED/DELIVERED` orders may not have shipment-level artifacts for perfect backfill.
- Reservation consistency risk:
  - Allocation, cancellation, and transfer paths can drift without strict transaction boundaries.
- API churn risk:
  - Breaking transition from `admin/orders/{id}/status` can leave stale admin callers.
- Performance risk:
  - Location-level inventory joins can degrade list/allocate performance without indexes.
- Operational risk:
  - Backorder growth can become invisible without aging metrics and alerting.

## Immediate Next Slice
1. Define canonical fulfillment status enums and transition matrix (order vs fulfillment vs shipment) in `api/openapi.yaml`.
2. Add minimal schema/models for `fulfillment_orders`, `fulfillment_order_items`, `fulfillment_status_history`.
3. Implement read-only admin fulfillment endpoint (`GET /api/v1/admin/orders/{id}/fulfillment`) with derived summary from existing orders.
4. Add first mutate endpoint (`POST /api/v1/admin/orders/{id}/fulfillment/allocate`) with idempotency key and transition validation.
