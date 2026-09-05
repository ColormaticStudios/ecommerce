# Platform Reliability and Operability Roadmap

## Implementation Status

- Overall status: Complete as of 2026-09-05.
- P0 through P4 are implemented and documented. The completion review also closed expired-lease visibility, deterministic media-failure classification, provider recovery-call telemetry, deployment credential isolation, migration-version stability, and shutdown-readiness symmetry findings.
- Environment-specific acceptance remains an operator responsibility rather than an open implementation phase:
  - validate dashboards and alert delivery against a non-development deployment;
  - run the first encrypted backup and isolated restore drill against the selected PostgreSQL and S3-compatible services;
  - integrate pre/post deployment checks into the selected systemd or Docker release workflow; and
  - conduct and retain evidence from the first quarterly non-production incident exercise.
- Normative subsystem and operating contracts now live in the wiki; this document is retained as the completed implementation record.

## Current Baseline

- P1 provides a shared DB-backed job runtime with scheduling, leased multi-worker claims, bounded retries, attempt history, and dead-letter handling.
- Media processing is the first durable runtime adopter; other legacy scheduled loops remain candidates for later migration.
- P2 provides repo-versioned Grafana dashboards, Prometheus rules/simulations, metrics, optional OTLP traces, and dependency-aware health endpoints.
- P3 provides a separately deployable encrypted logical-backup workload for systemd or Docker, external manifests/metrics, and guarded isolated restore drills.
- P4 provides incident governance, machine-readable MTTD/MTTR evidence, release-aware deployment gates, provider failure telemetry, and non-production failure-drill tooling.

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
- discounts/promotions, Ecommerce CMS, checkout-session lifecycle work:
  - Scheduled activation/cleanup work migrates from ad hoc in-process loops to shared scheduler/worker model.
- Implemented inventory baseline, `roadmap/order-fulfillment-ops.md`, `roadmap/returns-rma.md`:
  - Reconciliation and alerting jobs adopt shared retry policy, idempotency keys, and observability conventions.

## P0: Baseline Standards and Instrumentation Contract

Status: Complete. The normative contract is [Reliability and Operability](../../wiki/Reliability-and-Operability.md).

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
  - metrics path (the separate bind address is deferred to P2),
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

Status: Complete. The normative integration contract is [Background Jobs](../../wiki/Background-Jobs.md).

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

Status: Complete. Live non-development dashboard validation remains a deployment acceptance check. The deployment and integration contract is [Observability](../../wiki/Observability.md), with alert procedures in [Observability Runbooks](../../wiki/Observability-Runbooks.md).

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

Status: Complete. A real non-development backup and isolated restore drill remain deployment acceptance checks. The normative operations contract is [Backup and Restore](../../wiki/Backup-and-Restore.md).

### Scope

- Implement a separately deployed encrypted logical-backup workflow for PostgreSQL without coupling recovery to the API process.
- Support systemd and Docker scheduling while remaining provider-neutral for PostgreSQL and S3-compatible storage.
- Define guarded restore validation against an operator-provisioned isolated target.
- Track recovery objectives (RPO/RTO) against measured drill results.
- Keep product-media object protection provider-neutral and deployment-owned in this first slice; the operations workload does not assume or copy a particular media store.

### Deliverables

- Separate `ecommerce-ops` binary and operations image with:
  - `backup` and Docker `schedule` modes,
  - `restore-drill` mode,
  - manifest-backed `serve-metrics` mode.
- Deployment templates:
  - hardened systemd oneshot/timer and metrics units,
  - dedicated Docker image and Compose example.
- External backup metadata:
  - immutable per-run JSON manifests stored beside artifacts,
  - small latest-run/latest-success status objects for monitoring and restore selection,
  - no dependency on a `backup_runs` table inside the protected database.
- Integrity controls:
  - age X25519 encryption with public-recipient/private-identity separation,
  - SHA-256 verification of encrypted artifacts before restore,
  - source/target fingerprint guard and empty-target requirement,
  - provider-side versioning, retention, object lock, and replication guidance.
- Restore drill runbook in `wiki/`:
  - isolated-target provisioning contract,
  - migration/schema and transactional read/write checks,
  - measured RPO/RTO and cleanup responsibilities.
- Versioned dashboard and tested alerts for backup failure/freshness, manifest collection, and restore-drill failure/freshness.

### Done Criteria

- A systemd or Docker deployment completes daily encrypted backups with alerting on failure/missed run.
- A monthly operator/CI workflow provisions a fresh isolated target, restores it, and records measured RTO/RPO.
- Restore validation confirms schema compatibility and core API read/write functionality.
- Drill outcomes and action items are tracked and closed before the next drill cycle.

## P4: Operability Hardening and Incident Maturity

Status: Complete. Live pre/post deployment integration and the first quarterly non-production incident exercise remain deployment acceptance checks. The normative contracts are [Deployment Safety](../../wiki/Deployment-Safety.md), [Incident Response](../../wiki/Incident-Response.md), and [Failure Drills](../../wiki/Failure-Drills.md).

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
  - fail-closed `ecommerce-ops deploy-check` pre-deploy health/readiness/canary gates,
  - bounded post-deploy polling with immutable release-ID verification,
  - provider-neutral rollback criteria and nonzero exit contract.
- Chaos/failure drills (non-production):
  - DB unavailable,
  - worker crash loops,
  - provider timeout storms.
- Reliability review checklist for roadmap PRs:
  - SLO impact,
  - alert/runbook updates,
  - backup/restore implications.
- Machine-readable incident/drill evidence with chronology, ownership, action validation, and quarterly MTTD/MTTR summaries.
- Bounded provider call/outcome/latency metrics, dashboard, failure-storm alert, and fire/recovery simulation.
- Docker Toxiproxy harness for controlled database and provider faults without production runtime fault switches.

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

4. Backup and restore manifests (external object storage)

- Immutable JSON records contain artifact references, checksums, outcomes, verification status, and measured RPO/RTO without making disaster recovery depend on the protected database.

## Endpoint/API Plan

1. Deferred internal/admin job reliability endpoints (not part of P0-P4; require a separate approved roadmap slice):

- `GET /api/v1/admin/ops/jobs`
- `GET /api/v1/admin/ops/jobs/{id}`
- `POST /api/v1/admin/ops/jobs/{id}/retry`

Backup and restore operations are intentionally not exposed through application HTTP endpoints. Operators invoke the separately deployed workload so recovery remains available when the API or its database is unavailable.

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

1. Reliability models and migrations live in `models/` and `internal/migrations`.
2. The shared job runtime lives in `internal/jobs` and its startup/shutdown lifecycle is wired in `main.go`.
3. `internal/media` is the first durable-job adopter.
4. `ecommerce-ops` builds and deploys independently with systemd or Docker; source, restore, and monitoring credentials remain separated.
5. Admin job inspection/replay endpoints and UI are deferred and must follow the OpenAPI-first workflow if separately approved.
6. Run formatters on touched code files:

- Backend: `gofmt -w <file>`
- Frontend: `cd frontend && bun x prettier -w <file>`

7. Run the implementation and artifact checks:

- `go test ./...`
- `make domain-performance localization-performance`
- `make observability-check`
- `make operability-check`

## Risk Register

- Worker runtime bugs can create duplicate side effects without strict idempotency enforcement.
- Poorly tuned alert thresholds can cause noise and pager fatigue.
- Backup artifacts without regular restore validation provide false confidence.
- Job/metrics tables can grow unbounded without retention/partition strategy.
- In-process scheduler behavior can conflict in multi-instance deployments if leader/lease rules are weak.

## Post-Completion Operational Follow-Ups

1. Deploy P3 against a selected non-development PostgreSQL and S3-compatible target, then record the first isolated restore drill's measured RPO/RTO.
2. Integrate `ecommerce-ops deploy-check` before and after traffic promotion in the chosen systemd or Docker release workflow.
3. Set immutable `RELEASE_ID` values and retain deployment gate reports with release evidence.
4. Run the first P4 non-production failure drill, validate its incident record, and assign/close resulting corrective actions.
