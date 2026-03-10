# User Interactivity (Comments, Reviews, and Moderation) Roadmap

## Current Baseline
- There is no first-class product review model, comment thread model, or moderation case workflow in the current API surface.
- Product detail page (`frontend/src/routes/product/[id=int]/+page.svelte`) does not expose a native reviews/comments UX or reviewer trust signals.
- There is no admin queue for abusive/spam user-generated content (UGC) triage.
- Merchant/admin tools do not currently include review policy configuration, keyword policies, escalation paths, or enforcement history views.
- Search and merchandising roadmaps mention testimonial/review blocks, but there is no canonical source domain for those signals yet.

## Goals
- Ship product reviews and Q&A-style comments with clear, low-friction UX for buyers and guests (where policy allows).
- Provide robust moderation operations: queueing, triage, actioning, appeal/reversal support, and auditable enforcement logs.
- Add abuse controls (rate limiting, duplicate/spam detection, blocklists, and trust scoring) that reduce moderator load.
- Provide storefront trust signals (verified purchase, helpful votes, moderation badges, report flows) without exposing sensitive moderation metadata.
- Keep handlers thin and centralize domain logic in reusable services.

## Non-Goals
- Building a full social network (follows, DMs, creator feeds) in this roadmap.
- Building AI-generated review responses as a requirement for initial release.
- Supporting arbitrary UGC on every entity type (initial scope: products + order-linked review verification + product discussion threads).
- Replacing legal/compliance case management systems outside content moderation scope.

## Delivery Order
1. P0: Core UGC domain model and OpenAPI contracts.
2. P1: Reviews UX and write/read flows.
3. P2: Comment threads, reporting, and moderation queue.
4. P3: Advanced moderation tooling and enforcement automation.
5. P4: Reputation, trust surfacing, and optimization/hardening.

## Cross-Roadmap Alignment
- Product catalog depth baseline:
  - Canonical item linkage should be `product_id` for display and `order_item`/`product_variant_id` for verified purchase checks.
- Search and discoverability (`roadmap/search-discoverability-quality.md`):
  - Review rating aggregates and helpfulness should be indexable signals for ranking/faceting in later phases.
- Merchant analytics/reporting (`roadmap/merchant-analytics-reporting.md`):
  - Moderation throughput, abuse rates, and review conversion impact become shared analytics dimensions.
- Legal/compliance/security (`roadmap/legal-compliance-security.md`):
  - Moderation and user-reporting data retention/deletion rules must align with privacy and audit requirements.
- Ecommerce CMS (`roadmap/ecommerce-cms.md`):
  - CMS testimonial/review blocks should consume published, policy-compliant review summaries from this domain.

## P0: UGC Foundation and Contracts
### Scope
- Define schema and API contracts for reviews/comments/reporting/moderation actions.
- Introduce service boundaries and policy-driven write gating.

### Deliverables
- OpenAPI additions in `api/openapi.yaml`:
  - `GET /api/v1/products/{id}/reviews`
  - `POST /api/v1/products/{id}/reviews`
  - `PATCH /api/v1/reviews/{reviewId}`
  - `POST /api/v1/reviews/{reviewId}/report`
  - `GET /api/v1/products/{id}/comments`
  - `POST /api/v1/products/{id}/comments`
  - `POST /api/v1/comments/{commentId}/report`
  - Admin moderation endpoints under `/api/v1/admin/moderation/*`.
- New domain services:
  - `internal/services/reviews/`
  - `internal/services/comments/`
  - `internal/services/moderation/`
- Thin handlers in `handlers/` that delegate to service methods.
- Base policy settings model for:
  - guest posting enablement,
  - profanity rules,
  - minimum/maximum content length,
  - attachment permissions,
  - auto-hide thresholds.

### Done Criteria
- OpenAPI contracts compile and generated code is current.
- Service interfaces cover create/list/edit/report/moderate flows.
- Policy gates reject invalid submissions with structured error responses.
- No direct business logic duplication across handlers.

## P1: Reviews UX and Core User Flows
### Scope
- Implement product review write/read UX with strong baseline usability.
- Add reviewer trust indicators and controlled edit rules.

### Deliverables
- Product reviews model and APIs with:
  - rating (1-5), title, body, media attachments (optional), reviewer display name, timestamps.
- Verified purchase signal:
  - derived from successful order history for same account (or other configured identity binding).
- UX behaviors in `frontend/src/routes/product/[id=int]/+page.svelte` and related components:
  - rating summary and distribution,
  - sorting (`most_recent`, `highest_rating`, `lowest_rating`, `most_helpful`),
  - filters (`verified_only`, star bucket),
  - inline form validation and optimistic feedback,
  - clear pending/approved/rejected status messaging to author.
- Helpfulness voting (`helpful` / `not_helpful`) with idempotent vote update behavior.
- Edit/delete constraints:
  - limited edit window,
  - immutable verified purchase marker,
  - soft-delete support.

### Done Criteria
- Authenticated users can submit and edit reviews within policy constraints.
- Review summaries (average rating + counts) remain consistent with stored approved reviews.
- Helpful votes are race-safe and idempotent.
- Frontend handles empty/loading/error states without blocking product purchase flow.

## P2: Comments, Reporting, and Moderation Queue
### Scope
- Add product discussion comments and integrated report flows.
- Ship admin moderation queue for triage operations.

### Deliverables
- Comment thread support:
  - top-level comments and one-level replies,
  - optional merchant/staff badge,
  - sorting by newest/top.
- Public report flow:
  - reason codes (`spam`, `harassment`, `hate`, `off_topic`, `sensitive_data`, `other`),
  - optional note,
  - deduped repeated reports by same actor/content pair.
- Admin queue UI + APIs:
  - filter by severity/type/age/reporter count,
  - bulk actions (`approve`, `hide`, `remove`, `warn_user`, `mute_user`),
  - assignment and status states (`new`, `in_review`, `resolved`, `escalated`).
- Moderation audit trail:
  - actor, action, target content, rationale, timestamps, reversible marker.

### Done Criteria
- Reports create moderation queue items deterministically.
- Admin can complete end-to-end triage without DB-level interventions.
- Every moderation action is auditable and queryable.
- Hidden/removed content behavior is clear to authors and readers.

## P3: Advanced Moderation and Abuse Controls
### Scope
- Reduce abuse volume through automation and stronger policy enforcement.
- Improve moderator productivity and action consistency.

### Deliverables
- Automatic pre-moderation checks:
  - banned phrase matcher,
  - URL/domain allow/deny checks,
  - duplicate text fingerprinting,
  - velocity/rate-limit controls per account/device/IP.
- Risk scoring pipeline:
  - combine account age, prior moderation history, report volume, and content heuristics.
- Queue prioritization:
  - severity-weighted sorting + SLA labels.
- Moderator tooling:
  - side-by-side history view for repeat offenders,
  - template responses,
  - reversible enforcement actions with expiration windows.
- Enforcement primitives:
  - temporary posting cooldown,
  - product-specific mute,
  - global UGC suspension.

### Done Criteria
- Automated checks intercept a measurable class of abusive submissions before public display.
- Queue backlog and median time-to-resolution improve versus baseline.
- Enforcement reversals are supported with full audit lineage.
- False-positive review process exists and is test-covered.

## P4: Trust Layer, Analytics, and Hardening
### Scope
- Improve buyer confidence and operator visibility.
- Harden reliability and policy operations for scale.

### Deliverables
- Storefront trust UX:
  - reviewer badges (`verified purchase`, `top contributor`, `staff response`),
  - transparent moderation marker where content is removed/edited by policy,
  - abuse-report confirmation and status for reporters.
- Admin policy console:
  - configurable thresholds, blocked phrases, review requirements, cooldown windows.
- Analytics and reporting:
  - review coverage by product,
  - moderation action rates,
  - false-positive/appeal rates,
  - median triage time,
  - conversion correlation for products with high-quality reviews.
- Reliability and operations:
  - idempotency keys for mutation endpoints,
  - backfill/recompute job for rating aggregates,
  - runbooks for moderation queue incidents and abuse spikes.

### Done Criteria
- Trust signals are visible and understandable on product pages.
- Policy changes propagate without redeploy.
- Aggregates remain correct after replay/backfill jobs.
- Operational dashboards support incident triage and workload planning.

## Data Model Changes
1. `product_reviews`
- Fields: `id`, `product_id`, `user_id`, `order_id` (nullable), `rating`, `title`, `body`, `status`, `verified_purchase`, `helpful_count`, `not_helpful_count`, `edited_at`, timestamps, soft delete.

2. `product_review_votes`
- Fields: `id`, `review_id`, `user_id`, `vote` (`helpful|not_helpful`), timestamps.
- Unique key: (`review_id`, `user_id`).

3. `product_review_reports`
- Fields: `id`, `review_id`, `reporter_user_id` (nullable), `reason_code`, `note`, timestamps.
- Unique key recommendation: (`review_id`, `reporter_user_id`, `reason_code`).

4. `product_comment_threads`
- Fields: `id`, `product_id`, `status`, timestamps.

5. `product_comments`
- Fields: `id`, `thread_id`, `parent_comment_id` (nullable), `user_id`, `body`, `status`, `edited_at`, timestamps, soft delete.

6. `product_comment_reports`
- Fields: `id`, `comment_id`, `reporter_user_id` (nullable), `reason_code`, `note`, timestamps.

7. `moderation_cases`
- Fields: `id`, `entity_type`, `entity_id`, `status`, `priority`, `assigned_to`, `opened_by`, `opened_at`, `resolved_at`.

8. `moderation_actions`
- Fields: `id`, `case_id`, `action_type`, `actor_id`, `reason_code`, `note`, `reversible`, `reversed_at`, timestamps.

9. `ugc_policy_settings`
- Fields: `id`, `guest_reviews_enabled`, `guest_comments_enabled`, `min_body_len`, `max_body_len`, `auto_hide_report_threshold`, `banned_phrases_json`, `cooldown_seconds`, `updated_by`, timestamps.

10. `ugc_user_enforcements`
- Fields: `id`, `user_id`, `scope`, `action_type`, `starts_at`, `ends_at`, `created_by`, `reason_code`, timestamps.

## Endpoint/API Plan
1. Public review endpoints
- `GET /api/v1/products/{id}/reviews`
- Supports pagination, sort, star filters, and verified filter.
- `POST /api/v1/products/{id}/reviews`
- Creates review in `pending` or `approved` per policy.
- `PATCH /api/v1/reviews/{reviewId}`
- Author edit (window-limited) and status-aware validation.
- `POST /api/v1/reviews/{reviewId}/vote`
- Idempotent helpful vote mutation.
- `POST /api/v1/reviews/{reviewId}/report`
- Abuse reporting with reason codes.

2. Public comment endpoints
- `GET /api/v1/products/{id}/comments`
- Threaded comment retrieval with moderation-safe projection.
- `POST /api/v1/products/{id}/comments`
- Create top-level comments.
- `POST /api/v1/comments/{commentId}/replies`
- One-level replies.
- `POST /api/v1/comments/{commentId}/report`
- Abuse reporting.

3. Admin moderation endpoints
- `GET /api/v1/admin/moderation/cases`
- Queue list with filters and assignment.
- `POST /api/v1/admin/moderation/cases/{id}/assign`
- Assign moderator.
- `POST /api/v1/admin/moderation/cases/{id}/actions`
- Apply moderation action(s).
- `POST /api/v1/admin/moderation/enforcements`
- User-level posting restrictions.
- `GET/PUT /api/v1/admin/moderation/policy`
- Read/update UGC policy.

4. Contract compatibility notes
- Keep payloads additive in early phases so product pages without UGC support continue to render.
- Reserve explicit `status` enums to avoid ad hoc moderation states in frontend/backend.

## Execution Workflow in This Repo
1. Update `api/openapi.yaml` first for each phase that changes request/response shapes.
2. Run `make openapi-gen` and commit generated files:
- `internal/apicontract/openapi.gen.go`
- `frontend/src/lib/api/generated/openapi.ts`
3. Add/extend models in `models/` and wire migrations in `internal/migrations` (including backfills for rating aggregates if needed).
4. Implement service logic under `internal/services/reviews`, `internal/services/comments`, and `internal/services/moderation`.
5. Keep handlers in `handlers/` orchestration-only and reuse generated OpenAPI types.
6. Implement frontend UX in product and admin routes/components using generated client types.
7. Run `make openapi-check`.
8. Run backend tests with sandbox-safe cache:
- `GOCACHE=/tmp/go-build go test ./handlers/...`
- `GOCACHE=/tmp/go-build go test ./internal/services/...`
9. Run frontend checks/tests for touched areas:
- `cd frontend && bun run check`
- `cd frontend && bun run lint`

## Risk Register
- Review bombing and coordinated abuse can overwhelm moderation capacity.
- Mitigation: velocity limits, risk scoring, queue prioritization, temporary lockdown controls.
- False positives from automated moderation can suppress legitimate customer feedback.
- Mitigation: reversible actions, appeal path, sampled calibration reviews, threshold tuning.
- Defamation/privacy-sensitive content can create legal risk.
- Mitigation: clear policy taxonomy, staff escalation workflows, retention/deletion controls aligned with legal roadmap.
- High write volume can produce hot-spot contention on aggregate counters.
- Mitigation: async aggregate recompute jobs and bounded real-time counter updates.
- Poor UX can discourage legitimate reviewers.
- Mitigation: streamlined forms, transparent status messaging, and clear moderation outcomes.

## Immediate Next Slice
1. Define P0 OpenAPI schemas and endpoints for review list/create, report flows, and moderation case list/action.
2. Add initial migrations/models for `product_reviews`, `product_review_votes`, `product_review_reports`, `moderation_cases`, and `moderation_actions`.
3. Implement minimal review service + handler path with policy checks and verified-purchase derivation.
4. Add product page review panel with summary, list, and submit form (authenticated only for first slice).
5. Add backend tests for valid/invalid review submission, idempotent voting, and report deduping.
6. Add frontend validation tests for form errors and optimistic submit state transitions.
