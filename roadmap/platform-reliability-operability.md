# Platform Reliability and Operability Roadmap

## Current Baseline
- Background processing is mostly ad hoc and in-process today:
  - `internal/media/service.go` uses an in-memory channel queue.
  - `internal/media/processor.go` runs a goroutine worker loop.
- No shared job runtime exists for retries, scheduling, dead-letter handling, or multi-worker coordination.
- No repo-level standard dashboards/alerts exist for API latency/error rates, DB health, queue lag, or job failures.
- No documented backup + restore drill workflow exists in `scripts/`, `Makefile`, or `wiki/`.
- Reliability expectations (SLOs, paging thresholds, runbooks) are not defined as a platform standard.

## Goals
- Establish a shared background job infrastructure for async workflows across domains.
- Ship baseline observability: structured telemetry, dashboards, and actionable alerts.
- Implement repeatable backup and restore workflows with scheduled restore drills.
- Define reliability operations standards: SLOs, incident response runbooks, and on-call signal quality.
- Keep handlers thin and place reliability primitives in reusable `internal/` services.

## Non-Goals
- Building full workflow orchestration (DAG engine, cross-service saga platform).
- Introducing multi-region active-active architecture in this roadmap.
- Replacing all existing feature-specific roadmaps; this roadmap provides shared platform foundations they depend on.

## Delivery Order
1. P0: Reliability baseline and standards.
2. P1: Shared background job infrastructure.
3. P2: Observability dashboards and alerting.
4. P3: Backups, restore automation, and drills.
5. P4: Operability hardening and incident maturity.

## Cross-Roadmap Alignment
- `roadmap/customer-communications-email-sms.md`:
  - Reuse platform job runtime for outbox delivery retries, dead-letter handling, and queue lag metrics.
- Implemented provider platform:
  - Reconciliation and webhook retry jobs run on shared worker primitives.
- `roadmap/discounts-promotions.md`, `roadmap/ecommerce-cms.md`, checkout-session lifecycle work:
  - Scheduled activation/cleanup work migrates from ad hoc in-process loops to shared scheduler/worker model.
- Implemented inventory baseline, `roadmap/order-fulfillment-ops.md`, `roadmap/returns-rma.md`:
  - Reconciliation and alerting jobs adopt shared retry policy, idempotency keys, and observability conventions.

## P0: Baseline Standards and Instrumentation Contract
### Scope
- Define platform reliability standards and minimum instrumentation requirements.
- Introduce request/job correlation IDs and consistent structured logging fields.
- Define initial SLO set for API availability/latency and background job freshness.

### Deliverables
- New reliability standards doc in `wiki/` covering:
  - logging fields,
  - correlation ID propagation,
  - metric naming conventions,
  - alert severity policy.
- Configuration additions in `config/` + `.env.example` for:
  - telemetry enablement,
  - metrics bind address/path,
  - alert routing metadata.
- Middleware updates in `middleware/` for request ID propagation and log context injection.
- Shared helpers in `internal/` for:
  - correlation extraction/injection,
  - standardized error classification (`retryable`, `terminal`, `degraded`).

### Done Criteria
- Every API request log line includes request ID, route, status code, latency, and actor context when available.
- SLO definitions are documented with concrete targets and alert thresholds.
- A reliability standards checklist exists and is referenced by roadmap docs that add background jobs.

## P1: Shared Background Job Runtime
### Scope
- Introduce durable DB-backed job queue/runtime with retry and scheduling support.
- Migrate the media pipeline from in-memory queue to the shared runtime.
- Provide APIs/helpers for enqueueing idempotent jobs from domain services.

### Deliverables
- New package `internal/jobs` with:
  - job registry/handlers,
  - row-claiming worker loop,
  - retry with exponential backoff + jitter,
  - dead-letter transition.
- New tables/models:
  - `job_queue`,
  - `job_attempts`,
  - `job_dead_letters`.
- Migration updates in `internal/migrations/migrations.go` with required indexes:
  - `(status, run_at)`,
  - `(job_type, status)`,
  - `(idempotency_key)` unique where applicable.
- Runtime wiring in `main.go` for worker lifecycle:
  - start/stop with context cancellation,
  - configurable worker concurrency,
  - leader-safe periodic scheduler behavior.
- First migration target:
  - Media processing (`internal/media/*`) enqueues durable jobs instead of channel-only queue.

### Done Criteria
- Worker restarts do not lose enqueued jobs.
- Retryable failures are retried automatically and terminal failures move to dead-letter state.
- Concurrent workers do not process the same claimed job simultaneously.
- Media processing continues to function after migrating to durable queue path.
- Tests cover idempotent enqueue, retry exhaustion, and race safety.

## P2: Observability Dashboards and Alerts
### Scope
- Emit baseline metrics/traces/logs for API and jobs.
- Define standard dashboards and alert rules for reliability posture.
- Add runbook-linked alerts to reduce noisy/non-actionable pages.

### Deliverables
- Metrics exposure endpoint and instrumentation in backend (request rates, errors, latency percentiles, DB latency, job queue depth, job lag, dead-letter count).
- Optional tracing integration scaffold (OTel-compatible) behind config flags.
- Dashboard specs/templates (e.g., Grafana JSON or documented panels) for:
  - API health,
  - DB health,
  - background jobs.
- Alert rules (versioned in repo) for:
  - API 5xx error budget burn,
  - p95 latency breaches,
  - queue lag > threshold,
  - dead-letter growth,
  - backup failure/missed backup.
- Alert-to-runbook mapping in `wiki/` with clear remediation steps.

### Done Criteria
- Dashboards render from live metrics in a non-dev environment.
- Each paging alert has a linked runbook and an explicit owner.
- Alert test simulation shows fire and recovery behavior for at least:
  - API outage,
  - worker outage,
  - job backlog growth.

## P3: Backup and Restore Drills
### Scope
- Implement automated backup workflow for PostgreSQL and critical object storage assets.
- Define restore validation process and schedule recurring restore drills.
- Track recovery objectives (RPO/RTO) against measured drill results.

### Deliverables
- New scripts in `scripts/`:
  - `backup-db.sh`,
  - `restore-db.sh`,
  - `verify-backup.sh`.
- `Makefile` targets for backup/restore workflows:
  - `backup`,
  - `restore-check`,
  - `restore-drill`.
- Backup metadata store/table:
  - `backup_runs` with start/end timestamps, artifact URI, checksum, outcome.
- Integrity controls:
  - encrypted backup artifacts,
  - checksums + verification before retention acceptance.
- Restore drill runbook in `wiki/`:
  - step-by-step restore,
  - validation queries,
  - post-restore smoke checks (API + critical flows).

### Done Criteria
- Daily automated backups succeed with alerting on failure/missed run.
- Monthly restore drill restores to isolated environment and records measured RTO/RPO.
- Restore validation confirms schema compatibility and core API read/write functionality.
- Drill outcomes and action items are tracked and closed before the next drill cycle.

## P4: Operability Hardening and Incident Maturity
### Scope
- Improve incident response quality and reduce MTTR.
- Add reliability governance around deployments, dependencies, and failure injection.
- Ensure platform changes remain operable by default.

### Deliverables
- Incident process docs in `wiki/`:
  - severity matrix,
  - communication templates,
  - postmortem template with corrective action tracking.
- Deployment guardrails:
  - pre-deploy health gates,
  - post-deploy canary checks,
  - rollback criteria.
- Chaos/failure drills (non-production):
  - DB unavailable,
  - worker crash loops,
  - provider timeout storms.
- Reliability review checklist for roadmap PRs:
  - SLO impact,
  - alert/runbook updates,
  - backup/restore implications.

### Done Criteria
- On-call playbook is exercised in at least one simulated incident per quarter.
- Postmortems for Sev1/Sev2 incidents include actionable follow-ups with owners/dates.
- Deployment guardrails block releases when health checks fail.
- Mean time to detect and mean time to recover are measured and trending downward.

## Data Model Changes
1. `job_queue`
- Durable async work items with type, payload, status, schedule time, and idempotency key.

2. `job_attempts`
- Immutable per-attempt execution log with timestamps, error classification, and latency.

3. `job_dead_letters`
- Terminally failed jobs with failure reason and replay metadata.

4. `backup_runs`
- Backup and restore drill execution records, artifact references, checksum status, and measured RPO/RTO.

## Endpoint/API Plan
1. Internal/admin reliability endpoints (new, admin-auth only):
- `GET /api/v1/admin/ops/jobs`
- `GET /api/v1/admin/ops/jobs/{id}`
- `POST /api/v1/admin/ops/jobs/{id}/retry`
- `GET /api/v1/admin/ops/backups`
- `POST /api/v1/admin/ops/backups/restore-drill`

2. Telemetry/readiness endpoints:
- `GET /healthz`
- `GET /readyz`
- `GET /metrics`

3. API contract workflow:
- Update `api/openapi.yaml` first for new admin ops endpoints.
- Run `make openapi-gen`.
- Commit generated artifacts:
  - `internal/apicontract/openapi.gen.go`
  - `frontend/src/lib/api/generated/openapi.ts`
- Verify with `make openapi-check`.

## Execution Workflow in This Repo
1. Add models in `models/` and migrations in `internal/migrations`.
2. Implement shared runtime in `internal/jobs` and wire startup/shutdown in `main.go`.
3. Migrate first adopter (`internal/media`) to durable jobs.
4. Add thin admin ops handlers in `handlers/` and delegate orchestration to services in `internal/services/`.
5. Add admin operability UI surfaces in `frontend/src/routes/admin` and `frontend/src/lib/admin`.
6. Add scripts + `Makefile` targets for backup/restore workflows.
7. Run formatters on touched code files:
- Backend: `gofmt -w <file>`
- Frontend: `cd frontend && bun x prettier -w <file>`
8. Run tests for touched areas:
- `GOCACHE=/tmp/go-build go test ./internal/...`
- `GOCACHE=/tmp/go-build go test ./handlers`
- `cd frontend && bun run check && bun run lint`

## Risk Register
- Worker runtime bugs can create duplicate side effects without strict idempotency enforcement.
- Poorly tuned alert thresholds can cause noise and pager fatigue.
- Backup artifacts without regular restore validation provide false confidence.
- Job/metrics tables can grow unbounded without retention/partition strategy.
- In-process scheduler behavior can conflict in multi-instance deployments if leader/lease rules are weak.

## Immediate Next Slice
1. Implement P0 reliability standards doc and request ID/logging baseline in backend middleware.
2. Add `job_queue` + `job_attempts` schema and minimal `internal/jobs` claim/execute loop.
3. Migrate media processing to durable job enqueue/dequeue path.
4. Add first dashboard + alerts for API error rate, p95 latency, queue depth, and dead-letter count.
5. Add `scripts/backup-db.sh` + `scripts/restore-db.sh` and run first restore drill in a disposable environment.
