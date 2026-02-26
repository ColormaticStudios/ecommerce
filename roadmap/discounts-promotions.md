# Discounts and Promotions Roadmap

## Current Baseline
- The catalog and checkout flows do not yet have a first-class discount or promotion engine.
- Pricing is currently sourced from `models.Product.Price` and exposed as `price` in product/cart/order API responses.
- Order creation snapshots current product price into `models.OrderItem.Price` in `handlers/orders.go`.
- Current checkout quote/payment routes are still `/api/v1/me/*`, but this roadmap targets the guest-checkout replacement routes under `/api/v1/checkout/*`.
- API routing is OpenAPI-generated (`api/openapi.yaml` -> `internal/apicontract/openapi.gen.go` -> `handlers/generated_api_server.go`).
- Schema changes are currently managed by GORM `AutoMigrate` in `main.go` (no separate migration framework yet).
- There is no shared scheduling system for recurring campaign activation/deactivation.
- There is no admin archive for expired discounts/promotions.

## Goals
- Support per-product discounts with explicit start/end dates.
- Support cross-product promotions (for example: buy A, get B; spend X across products, get Y).
- Support multi-level promotions where one campaign applies different discount levels to different targets.
- Automatically remove expired discounts/promotions from storefront eligibility.
- Preserve expired campaigns in an admin-only past promotions/discounts view.
- Support schedulable, repeating, and reusable promotions.
- Support applying promotions to product categories.
- Provide rich feature configuration while keeping evaluation deterministic and auditable.

## Non-Goals
- Building a full loyalty/points system in the initial rollout.
- Launching personalized ML-driven offer targeting.
- Supporting every promotion pattern in phase 1.

## Delivery Order
1. P0: Core domain model + single-use discount evaluation.
2. P1: Promotion rule engine + cross-product promotions.
3. P2: Scheduling, recurring runs, and lifecycle archival.
4. P3: Reusable templates, advanced controls, and admin UX depth.
5. P4: Hardening, observability, and performance.

## Cross-Roadmap Alignment
- Checkout surface:
  - Use session checkout endpoints from `roadmap/guest-checkout.md` (`/api/v1/checkout/*`), not legacy `/api/v1/me/checkout/*`.
- Catalog depth:
  - Evaluate promotions against cart/order line `product_variant_id` once `roadmap/product-catalog-depth.md` P2 lands.
  - Campaign targets remain product/category level; variant-level eligibility is resolved through parent product/category joins.
- Categories:
  - Category-targeted promotions depend on `roadmap/product-categories.md` category and product-category join model.
- Providers:
  - Promotion-adjusted totals flow into checkout snapshot/payment lifecycle in `roadmap/providers.md`.

## P0: Core Domain Model and Product Discounts
### Scope
- Define campaign entities and evaluation boundaries.
- Implement per-product discounts with fixed/percent modes.
- Enforce validity windows (`starts_at`, `ends_at`) during price display and checkout.
- Add admin CRUD for active product discounts.

### Deliverables
- Schema for discounts/promotions with status and validity windows.
- Storefront price calculation integration for per-product discounts.
- Checkout quote integration so discount evaluation participates in `POST /api/v1/checkout/quote`.
- Order creation recalculation so `POST /api/v1/checkout/orders` persists discounted `OrderItem.Price` snapshot.
- Admin endpoints for create/update/disable of product discounts via generated OpenAPI handlers.
- Response contract redesign (breaking changes allowed):
  - Return explicit discount fields (for example `base_price`, `discount_amount`, `final_price`, `applied_campaigns`).
  - Remove legacy fallback requirements once frontend/generated clients switch to new schemas.

### Done Criteria
- A valid product discount changes product/listing/cart price consistently.
- An expired discount never applies in storefront or checkout.
- A future-start discount is invisible until `starts_at`.
- Invalid payloads (negative values, invalid windows) are rejected.

## P1: Promotion Rule Engine and Cross-Product Promotions
### Scope
- Introduce promotion rules with condition/action model.
- Support cross-product actions and triggers across cart lines.
- Add category-targeted promotions.
- Support tiered promotion levels within one campaign (for example, 10% for some targets and 20% for others).
- Define deterministic conflict handling (priority, exclusivity, stacking).

### Deliverables
- Promotion engine that consumes cart/order snapshot and returns applied adjustments.
- Rule primitives:
  - Product and variant conditions (`product_id`, `product_variant_id`, quantity, subtotal threshold).
  - Category conditions (`category_id`, quantity/subtotal threshold).
  - Brand conditions (`brand_id`, quantity/subtotal threshold).
  - Actions (percent off, fixed off, fixed price for target item, free item by SKU).
- Promotion level model:
  - Multiple levels per campaign with independent target mappings.
  - Level-specific action configs (for example, level A = 10%, level B = 20%).
- Configuration for stacking/exclusivity per campaign.
- Admin preview endpoint: evaluate promotion against sample cart.

### Done Criteria
- Cross-product promotion applies only when trigger conditions are met.
- Category promotion applies to matching category products only.
- Multi-level campaign applies the correct level per matched product/category target.
- Engine output is stable for same input (deterministic ordering).
- Conflicting promotions resolve according to documented priority rules.

## P2: Scheduling, Recurrence, and Expiration Archival
### Scope
- Add campaign schedules (one-time and repeating).
- Add activation/deactivation workers for scheduled transitions.
- Add automatic archival of expired campaigns to admin-only history.
- Define scheduler execution model for this repo (in-process ticker worker vs external worker) and document startup/wiring.

### Deliverables
- Schedule model supporting:
  - One-time windows.
  - Recurrence rule (daily/weekly/monthly with timezone).
  - Optional end-of-recurrence cutoff.
- Background jobs:
  - Activate scheduled campaigns.
  - Deactivate expired campaigns.
  - Move ended campaigns to archive state.
- Admin endpoints/pages:
  - Active campaigns.
  - Upcoming campaigns.
  - Past discounts/promotions (read-only archive).

### Done Criteria
- Repeating campaign activates/deactivates correctly across at least 3 cycles.
- Expired campaigns are removed from storefront eligibility automatically.
- Expired campaigns are visible in admin archive with full metadata.
- Job retries are idempotent and do not duplicate state transitions.

## P3: Reusable Templates and Rich Customizability
### Scope
- Support reusable promotion templates and cloning.
- Add richer constraints and metadata for business control.
- Improve editor ergonomics without compromising rule safety.

### Deliverables
- `promotion_templates` with parameterized rule blocks.
- Clone/create-from-template workflows.
- Advanced controls:
  - Usage caps (global/per-customer).
  - Customer segment filters.
  - Channel filters (web/app/admin).
  - Coupon code binding (optional).
- Validation and linting for admin-authored rules.

### Done Criteria
- Admin can create a new campaign from template in one flow.
- Usage caps are enforced under concurrent checkout traffic.
- Invalid or contradictory rule configs are blocked with actionable errors.
- Optional code-based campaigns and automatic campaigns can coexist.

## P4: Hardening and Operational Quality
### Scope
- Improve reliability, auditing, and monitoring.
- Add performance protections for high-cardinality promotion sets.
- Provide reconciliation/reporting views.

### Deliverables
- Audit history for campaign lifecycle and rule edits.
- Metrics and logs for evaluation latency, match rate, and failure rate.
- Caching/index strategy for rule lookup and category targeting.
- Reconciliation job comparing expected active campaigns vs runtime state.

### Done Criteria
- Promotion evaluation meets defined latency SLO at target cart size.
- Every campaign state change is traceable by actor/source/time.
- Reconciliation detects and reports schedule drift.
- Runbooks exist for failed scheduler jobs and stale campaign state.

## Core Evaluation Rules (Initial Contract)
1. Prices are computed server-side from authoritative data.
2. Eligibility is evaluated against a checkout/cart snapshot, not mutable client totals.
3. Campaign precedence:
- `exclusive` campaigns block lower-priority campaigns.
- Non-exclusive campaigns may stack only when stack policy allows it.
4. Inside a single campaign, only one level may apply per line item unless a level explicitly allows additive behavior.
5. Final item price cannot drop below zero; order total floors at zero.
6. All applied adjustments must include campaign and level references for auditability.

## Data Model Changes
1. `discount_campaigns`
- Fields: `id`, `name`, `type` (`product_discount|promotion`), `status`, `starts_at`, `ends_at`, `timezone`, `is_archived`, `priority`, `is_exclusive`, `created_by`, `updated_by`.

2. `discount_rules`
- Fields: `campaign_id`, `condition_json`, `action_json`, `stack_policy`, `max_applications_per_order`.

3. `discount_levels`
- Fields: `id`, `campaign_id`, `name`, `priority`, `action_json`, `stack_policy`, `max_applications_per_order`.

4. `discount_targets`
- Fields: `campaign_id`, `level_id`, `target_type` (`product|category|brand`), `target_id`.

5. `discount_schedules`
- Fields: `campaign_id`, `schedule_type` (`one_time|recurring`), `rrule`, `window_start`, `window_end`, `until_at`, `timezone`, `last_run_at`, `next_run_at`.

6. `discount_redemptions`
- Fields: `campaign_id`, `level_id`, `order_id`, `customer_id`, `applied_amount`, `applied_at`, `evaluation_snapshot_hash`.

7. `discount_state_history`
- Fields: `campaign_id`, `from_status`, `to_status`, `reason`, `source`, `actor`, `changed_at`.

8. `promotion_templates`
- Fields: `id`, `name`, `description`, `template_json`, `is_active`.

## Repo Integration Notes
1. OpenAPI and generated handlers
- Add discount/promotion operations to `api/openapi.yaml`.
- Regenerate and wire through `internal/apicontract/openapi.gen.go` and `handlers/generated_api_server.go`.

2. Handler boundaries
- Keep HTTP handlers thin; place evaluation/scheduling logic in `internal/` packages (for example `internal/discounts`).
- Reuse strict payload binding/validation patterns already used in current handlers.

3. Model registration
- Register new GORM models in `main.go` `AutoMigrate` list until a dedicated migration system is introduced.

4. Frontend contract safety
- Current frontend pages read `product.price` directly.
- Switch frontend in one coordinated contract cut to derived price fields from generated OpenAPI types.
- Align all cart/checkout integrations to `/api/v1/checkout/*` endpoints.

## Endpoint/API Plan
1. Admin campaign management
- `POST /api/v1/admin/discounts/campaigns`
- `PATCH /api/v1/admin/discounts/campaigns/{id}`
- `POST /api/v1/admin/discounts/campaigns/{id}/levels`
- `PATCH /api/v1/admin/discounts/campaigns/{id}/levels/{levelId}`
- `GET /api/v1/admin/discounts/campaigns?status=active|scheduled|archived`
- `POST /api/v1/admin/discounts/campaigns/{id}/archive`

2. Admin scheduling and template flows
- `POST /api/v1/admin/discounts/campaigns/{id}/schedule`
- `POST /api/v1/admin/discounts/templates`
- `POST /api/v1/admin/discounts/templates/{id}/instantiate`

3. Storefront/checkout evaluation
- `POST /api/v1/checkout/discounts/evaluate`
- `POST /api/v1/checkout/discounts/confirm`
- Extend `POST /api/v1/checkout/quote` response to include applied discount/promotion breakdown.

4. Reporting/history
- `GET /api/v1/admin/discounts/history`
- `GET /api/v1/admin/discounts/redemptions`

## Execution Workflow in This Repo
1. Update `api/openapi.yaml` first for any request/response changes.
2. Run `make openapi-gen` and commit generated contract files.
3. Implement schema and service logic (keep handlers thin).
4. Run `make openapi-check`.
5. Run backend/frontend tests for touched areas.

## Testing Strategy
- Unit tests:
  - Rule parsing/validation.
  - Deterministic evaluation ordering.
  - Stacking/exclusivity precedence.
- Integration tests:
  - Product discount apply/remove by time window.
  - Cross-product and category promotion eligibility.
  - Multi-level campaign eligibility maps correct products/categories to correct levels.
  - Recurrence activation/deactivation worker behavior.
  - Expiration archival visibility in admin history.
- Concurrency tests:
  - Usage cap enforcement under concurrent checkout submissions.
- Regression tests:
  - Ensure expired campaigns never appear in storefront evaluation results.

## Risk Register
- Recurrence semantics (timezone, DST, missed runs) can produce inconsistent activation.
- Rich rule flexibility can create ambiguous or contradictory configurations.
- Without strict precedence rules, stacking behavior becomes unpredictable.
- High campaign volume can degrade evaluation latency without indexing/caching.

## Immediate Next Slice
1. Implement P0 schema + per-product discount evaluation path.
2. Add admin CRUD for product discounts and validity windows.
3. Add checkout-side recalculation guard and core tests for expiry enforcement.
