# Returns and RMA Roadmap

## Current Baseline
- There is no first-class return domain in the current API or data model.
- Order lifecycle includes coarse refund semantics (`orders.status=REFUNDED`) but no item-level return lifecycle.
- No RMA creation/approval flow exists for customers or admins.
- No canonical return reason catalog exists.
- No customer-facing return status tracking exists.
- Exchange/store-credit outcomes are not modeled as explicit return resolutions.
- This project is pre-release, so additive and breaking contract/schema work is acceptable.

## Goals
- Introduce a configurable returns capability that is disabled by default.
- Add an end-to-end RMA flow: request, approval/rejection, receive, inspect, and disposition.
- Support standardized return reasons for analytics and policy enforcement.
- Support multiple return resolutions: exchange, store credit, and refund.
- Add refund orchestration that integrates with payment/provider capabilities.
- Provide customer-facing return tracking timeline and state visibility.
- Keep handlers thin and place return business logic in reusable `internal/` services.

## Non-Goals
- Building reverse-logistics carrier optimization in first cut.
- Supporting cross-order consolidated RMAs in initial rollout.
- Repricing order totals end-to-end for full post-order accounting in this roadmap.
- Marketplace multi-merchant returns routing in first cut.

## Delivery Order
1. P0: Returns domain foundation and feature gate.
2. P1: RMA request/triage and return reasons.
3. P2: Resolution handling (exchange, store credit, refund orchestration).
4. P3: Customer-facing return tracking and notifications.
5. P4: Hardening, operations, and policy enforcement.

## Cross-Roadmap Alignment
- Order fulfillment operations:
  - Depends on `roadmap/order-fulfillment-ops.md` for shipment and delivered-item context.
  - Return eligibility is scoped to shipped/delivered line items and shipped quantities.
- Checkout baseline:
  - Customer return surface must work for both authenticated and guest-origin orders where ownership is session/token based in the implemented checkout flow.
- Catalog depth:
  - Product catalog depth is already the repo baseline; return lines must use `product_variant_id` as canonical line identity.
- Provider platform baseline:
  - Refund and store-credit funding flows must align with the implemented provider abstractions and idempotency patterns.
- Discounts/promotions:
  - Refund calculations should consume finalized order totals/allocations from the established totals pipeline, not ad-hoc recomputation.

## P0: Returns Foundation and Feature Gate
### Scope
- Define first-class return entities and state machine.
- Add global returns toggle with explicit default-off behavior.
- Establish eligibility primitives required by later phases.

### Deliverables
- Add storefront/runtime setting:
  - `returns.enabled` boolean with default `false`.
- Add new models/tables:
  - `returns`
  - `return_items`
  - `return_events`
  - `return_reasons`
- Return status enum (initial):
  - `REQUESTED`, `APPROVED`, `REJECTED`, `AWAITING_RECEIPT`, `RECEIVED`, `INSPECTED`, `RESOLVED`, `CANCELLED`.
- Eligibility service in `internal/services/returns`:
  - Validates line-level eligibility by order state, shipped quantity, window, and prior returned quantity.

### Done Criteria
- When `returns.enabled=false`, all return-create endpoints reject with stable machine-readable error code.
- When enabled, valid eligible lines can create RMA requests.
- Ineligible requests are rejected with deterministic reason codes.
- Transition guardrails reject invalid state transitions.

## P1: RMA Request and Return Reasons
### Scope
- Customer/admin RMA request creation and triage workflow.
- Canonical reason taxonomy and reason capture.

### Deliverables
- API additions (customer + admin):
  - Create return request for eligible order items.
  - Admin approve/reject RMA with reason/notes.
  - Admin/customer read return details and timeline.
- Reason model:
  - System reason catalog (`DAMAGED`, `WRONG_ITEM`, `NOT_AS_DESCRIBED`, `SIZE_FIT`, `CHANGED_MIND`, `OTHER`).
  - Optional free-text details with length and sanitization constraints.
- Audit/event stream:
  - Persist actor/source/reason on each transition.

### Done Criteria
- Return requests require at least one eligible line and quantity > 0.
- Admin triage can approve/reject with persisted rationale.
- Invalid reasons or invalid quantities are rejected consistently.
- Tests cover valid and invalid creation/triage paths.

## P2: Resolution Paths (Exchange, Store Credit, Refund)
### Scope
- Add disposition outcomes and orchestration paths after inspection.
- Support partial item outcomes per return.

### Deliverables
- Resolution model:
  - Per-item disposition: `REFUND`, `EXCHANGE`, `STORE_CREDIT`, `REJECT`.
- Exchange flow:
  - Link to replacement `product_variant_id` and quantity.
  - Generate replacement order action or fulfillment instruction.
- Store credit flow:
  - Create customer credit ledger entry with expiration/policy metadata.
- Refund orchestration:
  - Create refund intent records and provider refund execution path.
  - Idempotency keys for refund operations.
  - Partial refund support aligned to item-level return quantities.

### Done Criteria
- A single return can resolve mixed item outcomes (for example refund one item, exchange another).
- Duplicate refund execution requests are idempotent.
- Refund amount does not exceed refundable balance for returned quantities.
- Exchange/store-credit records are traceable from return detail response.

## P3: Customer Return Tracking
### Scope
- Customer-facing visibility for return lifecycle and expected next step.
- Event timeline and status detail surfaces.

### Deliverables
- Customer endpoints/UI payload support:
  - Return list for customer-owned orders.
  - Return detail with status timeline, line items, and resolution summary.
  - ETA/state hints (`awaiting dropoff`, `in transit`, `received`, `processing refund`, `resolved`).
- Notification hooks:
  - Event-based triggers for approval, receipt, refund completed, exchange shipped.

### Done Criteria
- Customer can view return status history without admin privileges.
- Guest-order return tracking works with the same ownership model used by the implemented checkout flow.
- Timeline is append-only and ordered by event time.
- Regression tests cover access control and status visibility.

## P4: Hardening and Policy Controls
### Scope
- Reliability, abuse controls, and operational visibility.
- Return policy and fraud guardrails.

### Deliverables
- Policy engine primitives:
  - Return window by product/category.
  - Non-returnable item flags.
  - Max return quantity enforcement against purchase history.
- Reconciliation workers:
  - Return quantity vs refunded/exchanged/store-credit consistency checks.
  - Stuck-return detection (no event movement past threshold).
- Metrics and logging:
  - Return request rate, approval rate, reason distribution, refund latency.
  - Resolution mix (refund vs exchange vs store credit).

### Done Criteria
- Policy violations are rejected with explicit reason codes.
- Reconciliation jobs produce actionable mismatch records.
- Admin operations are idempotent where duplicate execution is possible.
- Failure-injection tests validate retry and idempotency behavior.

## Data Model Changes
1. `storefront_settings` (`config_json` / `draft_config_json`)
- Add `returns.enabled` boolean with default `false`.

2. `returns`
- Fields: `id`, `order_id`, `requester_type`, `requester_ref`, `status`, `requested_at`, `approved_at`, `resolved_at`, timestamps.

3. `return_items`
- Fields: `id`, `return_id`, `order_item_id`, `product_variant_id`, `quantity_requested`, `quantity_received`, `disposition`, `resolution_ref`, `reason_code`, `reason_detail`.

4. `return_reasons`
- Fields: `code` (unique), `label`, `is_active`, `sort_order`, `requires_detail`.

5. `return_events`
- Fields: `id`, `return_id`, `from_status`, `to_status`, `event_type`, `actor`, `source`, `notes`, `created_at`.

6. `refund_intents` (or provider-aligned refund records)
- Fields: `id`, `return_id`, `order_id`, `provider`, `provider_refund_id`, `amount`, `currency`, `status`, `idempotency_key`, timestamps.

7. `store_credit_ledger`
- Fields: `id`, `customer_ref`, `return_id`, `amount`, `currency`, `expires_at`, `status`, timestamps.

8. `exchange_links`
- Fields: `id`, `return_item_id`, `replacement_variant_id`, `replacement_quantity`, `replacement_order_id`, `status`, timestamps.

## Endpoint/API Plan
1. Customer return endpoints
- `GET /api/v1/returns`
- `POST /api/v1/returns`
- `GET /api/v1/returns/{id}`
- `POST /api/v1/returns/{id}/cancel`

2. Admin return triage/operations
- `GET /api/v1/admin/returns`
- `GET /api/v1/admin/returns/{id}`
- `POST /api/v1/admin/returns/{id}/approve`
- `POST /api/v1/admin/returns/{id}/reject`
- `POST /api/v1/admin/returns/{id}/receive`
- `POST /api/v1/admin/returns/{id}/inspect`
- `POST /api/v1/admin/returns/{id}/resolve`

3. Return reason catalog
- `GET /api/v1/returns/reasons`
- `GET /api/v1/admin/returns/reasons`
- `POST /api/v1/admin/returns/reasons`
- `PATCH /api/v1/admin/returns/reasons/{code}`

4. Feature gate behavior
- When `returns.enabled=false`:
  - Customer create/cancel/read endpoints return feature-disabled error (except historical read paths if policy allows read-only visibility).
  - Admin endpoints remain available for operational backoffice if explicitly desired; otherwise gated by same toggle.

## Execution Workflow in This Repo
1. Update `api/openapi.yaml` first for returns endpoints and schemas.
2. Run `make openapi-gen` and commit generated files:
- `internal/apicontract/openapi.gen.go`
- `frontend/src/lib/api/generated/openapi.ts`
3. Add/reshape models in `models/` and register migration steps in `internal/migrations`.
4. Implement return orchestration in `internal/services/returns`; keep `handlers/` thin.
5. Add/extend settings plumbing for `returns.enabled` in storefront config.
6. Run `make openapi-check`.
7. Run backend tests with sandbox cache:
- `GOCACHE=/tmp/go-build go test ./...`
8. Run frontend checks for touched paths:
- `cd frontend && bun run check && bun run lint`

## Risk Register
- Eligibility drift risk:
  - Returnable quantity can desync from fulfillment/refund records without strict transactional checks.
- Financial risk:
  - Incorrect refund computation for mixed discount/tax/shipping allocations can over-refund.
- Abuse risk:
  - Repeated return creation attempts and reason abuse without rate limits/policy guards.
- Operational risk:
  - Returns stuck in mid-state without event monitoring and reconciliation alerts.
- Toggle rollout risk:
  - Missing default handling could accidentally enable returns in production; enforce explicit default and tests.

## Immediate Next Slice
1. Add `returns.enabled` to storefront settings model and defaults with explicit default `false`.
2. Draft OpenAPI for minimal customer/admin return creation and detail endpoints plus feature-disabled error schema.
3. Add baseline models for `returns`, `return_items`, and `return_events` with migration scaffolding.
4. Implement eligibility checker and one guarded endpoint (`POST /api/v1/returns`) with valid/invalid path tests.
