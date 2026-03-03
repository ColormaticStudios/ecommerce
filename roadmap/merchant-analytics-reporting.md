# Merchant Analytics and Reporting Roadmap

## Current Baseline
- The platform has catalog, cart, checkout, orders, promotions, and provider abstractions, but no dedicated analytics/reporting domain yet.
- Order pricing is snapshot-based at order creation time, which is a suitable source for immutable reporting facts.
- Checkout surface is moving toward `/api/v1/checkout/*` as the canonical mutation/read flow for cart and order lifecycle.
- OpenAPI is contract-first (`api/openapi.yaml` -> generated backend/frontend artifacts).
- There is no standardized event/fact pipeline for analytics dimensions (channel, campaign, cohort, tax jurisdiction, margin components).
- There are no export-oriented finance/tax reporting endpoints with period locking and reproducibility semantics.

## Goals
- Deliver merchant-facing analytics for:
- Sales and margin reporting (gross sales, net sales, COGS, gross margin).
- Promotion performance (discount cost, uplift, attach rate, AOV impact).
- Funnel and drop-off analytics (session to cart to checkout steps to paid order).
- Cohort and LTV analytics (acquisition cohort retention, cumulative revenue, margin LTV).
- Exportable finance/tax reports (period CSV/JSON exports with reproducible totals).
- Provide deterministic, auditable reporting outputs that tie back to immutable order/payment/tax snapshots.
- Keep handler layer thin and put reporting logic in reusable internal services.

## Non-Goals
- Real-time stream processing with sub-second freshness in the first release.
- Full BI dashboard builder (arbitrary user-authored SQL/visualization engine).
- Multi-entity consolidated accounting (inter-company eliminations, advanced GL close workflows).
- Replacing merchant accounting software; exports are designed for import/reconciliation.

## Delivery Order
1. P0: Reporting foundation and canonical facts.
2. P1: Sales, margin, and promotion reporting.
3. P2: Funnel/drop-off instrumentation and analytics.
4. P3: Cohort/LTV analytics and lifecycle views.
5. P4: Finance/tax exports, hardening, and operational quality.

## Cross-Roadmap Alignment
- Checkout surface:
- Treat `/api/v1/checkout/*` as canonical event source; do not add new analytics dependencies on legacy `/api/v1/me/*` checkout endpoints.
- Purchasable identifier:
- Use `product_variant_id` as canonical line identifier; do not add new analytics dependencies on `product_id` purchase identity.
- Shared ownership/session model:
- Funnel facts use `checkout_session_id`; customer lifecycle facts use `user_id` when present and support guest fallback keys.
- Shared totals pipeline:
- Reporting totals derive from catalog pricing + promotions + provider tax/shipping/payment snapshots to avoid recomputation drift.
- Contract simplification:
- Remove superseded identifiers/surfaces in the same phase they are replaced; avoid long-lived analytics translation layers.

## P0: Reporting Foundation and Canonical Facts
### Scope
- Define analytics fact tables/views and aggregation grain.
- Establish ingestion jobs from transactional tables into analytics facts.
- Add report period and timezone normalization rules.
- Add admin report access boundaries and query guardrails.

### Deliverables
- Data model for analytics facts and dimensions:
- `analytics_order_facts` (order-level immutable facts).
- `analytics_order_item_facts` (item-level revenue, discount, tax, cost, margin components).
- `analytics_checkout_facts` (funnel step events keyed by `checkout_session_id`).
- `analytics_customer_facts` (customer/cohort key materialization).
- Dimension tables for date, channel, promo campaign, category, geography/tax jurisdiction.
- ETL/backfill job in `internal/analytics` with idempotent upsert by source entity + version hash.
- Admin read endpoints for freshness metadata (last materialized timestamp, lag).
- Retention policy and partition/index strategy for analytics tables.

### Done Criteria
- Backfill can materialize existing orders without duplicates.
- Re-running ingestion is idempotent and leaves totals unchanged.
- Timezone and report window boundaries are deterministic per merchant config.
- Freshness endpoint reports lag and job status accurately.

## P1: Sales, Margin, and Promotion Reporting
### Scope
- Implement sales and margin metrics by day/week/month and key dimensions.
- Implement promo performance reporting tied to campaign identifiers.
- Support drill-down from aggregate to order and line-item evidence.

### Deliverables
- Sales report endpoints:
- Gross sales, net sales, refunds, taxes collected, shipping collected.
- Units sold, AOV, conversion-to-paid count.
- Margin report endpoints:
- COGS by item snapshot, gross margin amount/percent, discount cost attribution.
- Promotion performance endpoints:
- Campaign usage count, discounted revenue, promo cost, incremental AOV proxy, attach rate.
- Sorting/filtering by date range, channel, category, campaign, product variant.
- Frontend reporting pages using generated API types in `frontend/src/lib/api/generated/openapi.ts`.

### Done Criteria
- Aggregates reconcile to order facts within accepted tolerance (exact match for currency minor units).
- Promo report only counts promotions recorded in immutable order snapshots.
- Filters produce stable totals regardless of pagination/order.
- Invalid query windows/dimensions are rejected with structured errors.

## P2: Funnel and Drop-Off Analytics
### Scope
- Capture canonical funnel steps and abandonment points.
- Attribute drop-off to step and primary reason buckets.
- Provide conversion rates and median time-between-steps.

### Deliverables
- Funnel event schema:
- `session_started`, `cart_viewed`, `checkout_started`, `shipping_submitted`, `payment_submitted`, `order_placed`, `payment_failed`, `abandoned`.
- Ingestion from API handlers/services into analytics event writes (async buffered path preferred).
- Funnel reporting endpoints:
- Step conversion matrix.
- Drop-off by step/reason/channel/device.
- Time-to-complete distributions.
- Admin UI for funnel visualization and date/channel filters.

### Done Criteria
- Funnel totals are consistent with checkout/order counts for the same period.
- Duplicate client retries do not inflate funnel counts.
- Drop-off reasons are mapped from explicit provider/validation failure categories.
- Event ingestion failure paths are observable and retry-safe.

## P3: Cohort and LTV Analytics
### Scope
- Add cohort definitions and lifecycle metrics.
- Provide revenue and margin LTV over fixed cohort intervals.
- Support guest-to-account stitching when identity becomes available.

### Deliverables
- Cohort definition support:
- Acquisition cohort by first paid order date.
- Optional campaign/channel acquisition cohort.
- LTV metrics endpoints:
- Cumulative revenue LTV (D30/D60/D90/custom windows).
- Cumulative margin LTV.
- Repeat purchase rate and retention curve.
- Identity stitching logic for guest checkout conversion to user account.
- Export endpoint for cohort tables used in external BI/accounting workflows.

### Done Criteria
- Cohort membership is immutable after first-paid-order cutoff rules.
- LTV curves are reproducible for the same as-of date and window.
- Guest-to-user stitching avoids double-counting customer lifetime totals.
- Cohort exports reconcile with corresponding API aggregates.

## P4: Finance/Tax Exports and Operational Hardening
### Scope
- Deliver accounting-grade export flows for finance and tax workflows.
- Add report versioning/period lock semantics.
- Harden reliability, observability, and performance.

### Deliverables
- Export jobs/endpoints:
- Sales journal export (CSV/JSON) with order, tax, shipping, discount, payment breakdown.
- Tax liability export by jurisdiction/rate code/period.
- Refund/chargeback export where data is available.
- Report period lock model (`open`, `locked`) with export version metadata.
- Audit log for report generation parameters and actor identity.
- SLOs/alerts for ETL lag, export failures, and reconciliation drift.
- Runbooks in `wiki/` for reconciliation and export incident handling.

### Done Criteria
- Locked period exports are reproducible byte-for-byte for same export format/version.
- Tax export totals reconcile with order/payment tax snapshots.
- Large period exports complete within defined timeout/SLO budgets.
- Alerts fire for stale materialization, failed exports, and reconciliation mismatches.

## Data Model Changes
1. `analytics_order_facts`
- Grain: one row per order snapshot version.
- Fields: `order_id`, `merchant_id`, `placed_at`, `currency`, `gross_sales_minor`, `net_sales_minor`, `discount_minor`, `tax_minor`, `shipping_minor`, `refund_minor`, `payment_status`, `channel`, `source_hash`, `materialized_at`.

2. `analytics_order_item_facts`
- Grain: one row per order item snapshot.
- Fields: `order_id`, `order_item_id`, `product_variant_id`, `product_id`, `category_id`, `qty`, `item_gross_minor`, `item_discount_minor`, `item_tax_minor`, `item_cogs_minor`, `item_margin_minor`, `campaign_id`, `materialized_at`.

3. `analytics_checkout_events`
- Grain: one row per canonical checkout event.
- Fields: `checkout_session_id`, `event_name`, `event_at`, `channel`, `device_type`, `user_id`, `reason_code`, `idempotency_key`, `source_hash`.

4. `analytics_customer_facts`
- Grain: one row per customer identity key.
- Fields: `customer_key`, `user_id`, `first_paid_order_at`, `acquisition_channel`, `acquisition_campaign_id`, `lifetime_revenue_minor`, `lifetime_margin_minor`, `last_order_at`.

5. `reporting_period_locks`
- Fields: `merchant_id`, `period_start`, `period_end`, `timezone`, `status`, `locked_at`, `locked_by`, `export_version`.

6. `report_export_runs`
- Fields: `id`, `merchant_id`, `report_type`, `format`, `params_json`, `period_start`, `period_end`, `status`, `storage_key`, `checksum`, `created_by`, `created_at`, `completed_at`.

## Endpoint/API Plan
1. Sales and margin
- `GET /api/v1/admin/reports/sales`
- `GET /api/v1/admin/reports/margins`
- `GET /api/v1/admin/reports/promotions`

2. Funnel and cohort
- `GET /api/v1/admin/reports/funnel`
- `GET /api/v1/admin/reports/cohorts`
- `GET /api/v1/admin/reports/ltv`

3. Export and operations
- `POST /api/v1/admin/reports/exports`
- `GET /api/v1/admin/reports/exports/{id}`
- `POST /api/v1/admin/reports/period-locks`
- `GET /api/v1/admin/reports/freshness`

4. Contract notes
- Breaking response evolution is acceptable when it removes ambiguous or duplicated reporting shapes.
- Query params must require explicit `from`, `to`, `timezone`, and `group_by` where relevant.
- Currency amounts exposed as minor units plus currency code to avoid float drift.

## Execution Workflow in This Repo
1. Update `api/openapi.yaml` first for new report/extract endpoints and schemas.
2. Run `make openapi-gen` and commit generated files:
- `internal/apicontract/openapi.gen.go`
- `frontend/src/lib/api/generated/openapi.ts`
3. Add/modify migrations in `internal/migrations` for analytics/reporting tables and indexes.
4. Implement service logic in `internal/analytics` and keep `handlers/` orchestration-only.
5. Implement frontend report pages/components in `frontend/src/routes/...` using generated types.
6. Run `make openapi-check`.
7. Run backend tests with sandbox-safe cache:
- `GOCACHE=/tmp/go-build go test ./...`
8. Run frontend checks:
- `cd frontend && bun run check && bun run lint`

## Risk Register
- Cost/margin data quality risk: COGS may be missing or stale for historical orders.
- Mitigation: explicit `unknown_cogs` handling and quality flags in margin endpoints.
- Identity stitching risk: guest and authenticated sessions may over/under merge.
- Mitigation: deterministic stitching rules with audit fields and reversible backfill.
- Attribution bias risk: promo uplift inferred from observational data.
- Mitigation: label as directional metric and separate from accounting-grade totals.
- Late-arriving data risk: refunds/chargebacks posted after initial period close.
- Mitigation: period lock states plus adjustment exports for post-lock movements.
- Performance risk: wide aggregate queries on large merchants.
- Mitigation: partitioning/materialized aggregates and async export jobs.

## Immediate Next Slice
1. Define P0 analytics schema migration set and create `internal/analytics` materialization job skeleton.
2. Add `GET /api/v1/admin/reports/freshness` in OpenAPI and wire generated handler/service.
3. Implement sales aggregate endpoint (`GET /api/v1/admin/reports/sales`) against `analytics_order_facts`.
4. Add integration tests for idempotent materialization and sales reconciliation.
5. Ship a minimal frontend sales report view backed by generated API client.
