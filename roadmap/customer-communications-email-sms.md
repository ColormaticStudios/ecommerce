# Customer Communications (Email + SMS) Roadmap

## Current Baseline
- There is no first-class outbound communications subsystem for transactional email/SMS.
- The API and admin UI do not expose communication delivery history or failure reporting.
- There is no background worker pipeline for outbound send retries or dead-letter handling.
- Existing backend domain services live in `internal/services/*` and handlers are under `handlers/`.

## Goals
- Add transactional email and SMS support with safe default behavior:
  - If provider configuration is absent, channel is treated as disabled.
  - Business flows continue without hard-failing when communications are disabled.
- Add production-grade send execution with deterministic retries and terminal failure states.
- Add admin reporting for customer communication history, delivery outcomes, and failed sends.
- Keep handlers thin and place orchestration/retry logic in reusable `internal/services` packages.

## Non-Goals
- Building a marketing automation suite/campaign builder.
- Implementing inbound email/SMS parsing in initial phases.
- Guaranteeing delivery/read receipts for every provider from day one.

## Delivery Order
1. P0: Communication domain and disabled-by-default runtime behavior.
2. P1: Outbox queue and retry-safe worker execution.
3. P2: Provider adapters (email + SMS) and failure classification.
4. P3: Admin dashboard and communication reporting APIs.
5. P4: Operational hardening, alerts, and replay controls.

## Cross-Roadmap Alignment
- Checkout baseline:
  - Use guest/customer contact data from canonical checkout/order records.
  - Do not create a parallel customer identity model for communications.
- Provider platform baseline:
  - Reuse the established reliability patterns: idempotency, async processing, retry/backoff, dead-letter visibility.
  - Keep communication provider interfaces separate from payment/shipping/tax interfaces, but follow the same adapter style.
- Order fulfillment (`roadmap/order-fulfillment-ops.md`) and returns (`roadmap/returns-rma.md`):
  - Emit communications from domain events (order placed, shipped, delivery exception, return approved) instead of direct handler calls.

## P0: Foundation and Safe Defaults
### Scope
- Define communication event + message model for transactional sends.
- Add runtime configuration gating so email/SMS are disabled unless explicitly configured.
- Ensure calling code can enqueue communication intents without needing to branch on provider state.

### Deliverables
- New config section in `config/` and env mappings in `.env.example`:
  - `COMM_EMAIL_PROVIDER`, provider credentials, sender defaults.
  - `COMM_SMS_PROVIDER`, provider credentials, sender defaults.
  - Optional global kill switch for emergency disable.
- New service package: `internal/services/communications` with channel capability checks.
- Domain events emitted by key flows (initially: order placed, payment failed, shipment update).
- Documented behavior contract:
  - If channel disabled: message record becomes `SKIPPED_DISABLED` (not `FAILED`).
  - Core business operation remains successful.

### Done Criteria
- Boot-time validation reports which channels are enabled/disabled with non-secret structured logs.
- Missing provider config does not cause startup failure.
- Communication event creation path is covered by tests for enabled and disabled channels.

## P1: Outbox + Retry Worker
### Scope
- Introduce DB-backed outbox for communication jobs.
- Add worker loop to claim pending rows and execute sends with retry/backoff.
- Guarantee idempotent retries and bounded failure behavior.

### Deliverables
- New tables/models:
  - `communication_messages` (one logical message).
  - `communication_deliveries` (per channel attempt lifecycle).
  - `communication_attempts` (immutable attempt log).
- Message states:
  - `PENDING`, `SENDING`, `SENT`, `FAILED_RETRYABLE`, `FAILED_TERMINAL`, `SKIPPED_DISABLED`.
- Retry policy (configurable):
  - Exponential backoff with jitter.
  - Max attempts per channel.
  - Next-attempt scheduling via `next_attempt_at`.
- Worker lifecycle wiring in server startup (single-process compatible, multi-process safe row claiming).

### Done Criteria
- Failed transient send is retried automatically until max attempts.
- Terminal failures stop retrying and are visible in DB and logs.
- Concurrent workers do not double-send the same attempt (row lock/claim token enforced).
- Tests cover retryable vs terminal classification and duplicate worker race safety.

## P2: Provider Adapters and Send Logic
### Scope
- Implement channel provider interfaces and first provider adapters.
- Support provider response normalization and external message ID storage.
- Add webhook/status ingest scaffold where providers support async status updates.

### Deliverables
- Interfaces in `internal/services/communications/providers.go`:
  - `EmailProvider.Send(...)`
  - `SMSProvider.Send(...)`
- Adapter packages:
  - `internal/services/communications/email/<provider>`
  - `internal/services/communications/sms/<provider>`
- Error taxonomy used by worker:
  - Retryable: timeout, 429/rate-limit, 5xx/transient transport.
  - Terminal: invalid recipient, invalid template payload, auth misconfiguration.
- Optional webhook endpoint family for delivery updates:
  - `POST /api/v1/webhooks/communications/{channel}/{provider}`

### Done Criteria
- Enabled channel sends real provider requests and persists provider message IDs.
- Known invalid recipient path marks terminal failure without retries.
- Rate-limited responses reschedule with retry/backoff.
- Provider adapter tests use fakes and cover success + classified failures.

## P3: Admin Dashboard and Reporting APIs
### Scope
- Add admin visibility for communication activity across customers/orders.
- Provide filters for failed sends, channel, template/event type, and date range.
- Add customer-level communication timeline in admin UX.

### Deliverables
- OpenAPI additions for admin reporting endpoints:
  - `GET /api/v1/admin/communications/messages`
  - `GET /api/v1/admin/communications/messages/{id}`
  - `GET /api/v1/admin/customers/{id}/communications`
  - `GET /api/v1/admin/communications/metrics`
- Frontend admin tab in `frontend/src/routes/admin/+page.svelte` (or extracted component) for:
  - Delivery table (status, channel, recipient, attempts, last error, timestamps).
  - Failure-focused view (retryable vs terminal, aging failures).
  - KPI cards (send volume, failure rate, retry success rate, disabled-skip count).
- Drill-down panel showing attempt history and provider error snapshots.

### Done Criteria
- Admin can filter and export failed send records for triage.
- Customer profile/admin view shows chronological communication history.
- Dashboard correctly distinguishes disabled-skip from send failure.
- API + frontend integration tests validate pagination/filter contracts.

## P4: Hardening and Runbook Support
### Scope
- Add replay/resend controls with guardrails.
- Add alerting and SLO-oriented monitoring for send failures and queue lag.
- Add data-retention and PII-redaction rules for payload/error storage.

### Deliverables
- Admin mutation endpoints:
  - `POST /api/v1/admin/communications/messages/{id}/retry`
  - `POST /api/v1/admin/communications/messages/{id}/cancel`
- Metrics/alerts:
  - queue depth, oldest pending age, retry exhaustion count, provider error rate.
- Runbook docs in `wiki/` for:
  - provider outage response,
  - credential rotation,
  - stuck queue remediation.
- Scheduled cleanup/retention task for old attempt payloads.

### Done Criteria
- Manual retry creates a new controlled attempt with audit trail.
- Alerts trigger under simulated outage and clear after recovery.
- Retention jobs remove/redact expired sensitive fields without breaking reporting.

## Data Model Changes
1. `communication_messages`
- Fields: `id`, `event_type`, `subject_type`, `subject_id`, `template_key`, `payload_json`, `created_by_source`, timestamps.
- Purpose: canonical logical message generated from domain events.

2. `communication_deliveries`
- Fields: `id`, `message_id`, `channel` (`EMAIL|SMS`), `recipient`, `status`, `provider`, `provider_message_id`, `attempt_count`, `max_attempts`, `next_attempt_at`, `last_error_code`, `last_error_message`, `sent_at`, `failed_at`, timestamps.
- Indexes: `(status, next_attempt_at)`, `(channel, status)`, `(recipient)`.

3. `communication_attempts`
- Fields: `id`, `delivery_id`, `attempt_number`, `started_at`, `finished_at`, `outcome`, `provider_status_code`, `provider_error_code`, `error_message_redacted`, `latency_ms`, timestamps.
- Constraint: unique `(delivery_id, attempt_number)`.

4. Optional `communication_templates`
- Fields for template metadata and channel compatibility if template management is internalized later.

## Endpoint/API Plan
1. Admin read/reporting (P3)
- `GET /api/v1/admin/communications/messages`
- `GET /api/v1/admin/communications/messages/{id}`
- `GET /api/v1/admin/customers/{id}/communications`
- `GET /api/v1/admin/communications/metrics`

2. Admin controls (P4)
- `POST /api/v1/admin/communications/messages/{id}/retry`
- `POST /api/v1/admin/communications/messages/{id}/cancel`

3. Provider status ingestion (optional P2+)
- `POST /api/v1/webhooks/communications/{channel}/{provider}`

4. API contract workflow
- Update `api/openapi.yaml` first.
- Run `make openapi-gen`.
- Commit generated:
  - `internal/apicontract/openapi.gen.go`
  - `frontend/src/lib/api/generated/openapi.ts`
- Verify with `make openapi-check`.

## Execution Workflow in This Repo
1. Add configuration and models under `config/` and `models/`; register schema changes in `internal/migrations/migrations.go`.
2. Implement communication domain service in `internal/services/communications` with:
- event intake,
- outbox persistence,
- retry scheduler,
- provider dispatch.
3. Keep transport/API handlers thin by delegating to service layer from `handlers/`.
4. Add admin API handlers and wire generated server routes.
5. Add admin UI reporting tab/components in `frontend/src/routes/admin` and `frontend/src/lib/admin`.
6. Run formatters on touched files:
- Backend: `gofmt -w <file>`
- Frontend: `cd frontend && bun x prettier -w <file>`
7. Run tests for touched areas:
- `GOCACHE=/tmp/go-build go test ./internal/services/...`
- `GOCACHE=/tmp/go-build go test ./handlers`
- `cd frontend && bun run check && bun run lint`

## Risk Register
- Misclassifying provider errors can either spam retries or suppress valid retries.
- If retry backoff is not jittered, provider outages can cause synchronized retry storms.
- Storing raw provider payloads may leak PII/secrets unless redacted.
- Dashboard queries can become expensive without status/time indexes and pagination limits.
- Event triggering directly in handlers instead of domain services can cause drift and missed sends.

## Immediate Next Slice
1. Ship P0 config gating + `communication_messages`/`communication_deliveries` schema skeleton.
2. Implement a minimal worker that processes `PENDING` deliveries with one fake provider adapter.
3. Add retry/backoff policy and attempt logs, then cover with service tests for transient vs terminal failures.
4. Add first admin read endpoint (`GET /api/v1/admin/communications/messages`) and a basic admin table view.
