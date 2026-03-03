# Database Migrations Robustness Roadmap

## Current Baseline
- Migrations are defined in `internal/migrations/migrations.go` as ordered Go functions with `Version`, `Name`, and `Up`.
- Current baseline has one migration (`2026022601_initial_schema`) that relies heavily on `AutoMigrate`.
- Applied versions are tracked in `schema_migrations` with `version` and `applied_at`.
- Startup paths auto-apply pending migrations (`main.go`, `cmd/cli/commands/user.go`, `cmd/e2e-server/main.go`).
- `cmd/migrate` supports `up` and `check`, but there is no explicit rollback/down workflow.
- Several tests still use direct `AutoMigrate` in test setup instead of versioned migration paths.

## Goals
- Make schema evolution deterministic and safe across frequent future changes.
- Introduce an explicit migration workflow for safe but decisive schema cuts.
- Add strong operational controls (locking, observability, drift checks, safety gates).
- Standardize migration authoring patterns for schema and data backfills.
- Ensure migration behavior is validated in CI and local dev for both success and failure modes.

## Non-Goals
- Building a generic external migration framework with plugin ecosystems.
- Supporting arbitrary rollback of all historical data migrations.
- Rewriting existing domain models unrelated to migration safety.
- Removing all `AutoMigrate` usage from tests in one step.

## Delivery Order
1. P0: Migration Framework Hardening
2. P1: Authoring Standards and Safety Controls
3. P2: Operational Rollout Model (Expand/Contract without Long-Lived Compatibility Windows)
4. P3: CI/CD Enforcement and Drift Detection
5. P4: Test Harness Convergence and Legacy Cleanup

## P0: Migration Framework Hardening
### Scope
- Extend current migration primitive to support richer metadata and safer execution controls.
- Keep existing Go-based migration style (no immediate tool replacement).

### Deliverables
- Extend `internal/migrations` structures with:
  - `TransactionMode` (`required`, `none`) for DDL that cannot run in one transaction.
  - `PostChecks` callbacks for invariant validation.
  - `Tags` (for reporting and operational filtering).
- Add `migrate plan` command to print ordered pending steps without applying.
- Add `migrate status` command with:
  - latest known version in code,
  - latest applied version in DB,
  - pending count.
- Add deterministic ordering validation at startup (fail if duplicate/malformed versions exist).
- Add advisory lock for migration execution (Postgres lock key) to prevent concurrent runners.

### Done Criteria
- Concurrent `migrate up` runs do not apply the same version twice.
- `migrate plan` and `migrate status` output are stable and parseable in CI logs.
- Duplicate version IDs fail fast during test and startup.
- Existing initial migration still applies cleanly after framework changes.

## P1: Authoring Standards and Safety Controls
### Scope
- Define clear migration authoring conventions and guardrails for future changes.

### Deliverables
- Add `internal/migrations/README.md` with required migration pattern:
  - one change intent per version,
  - explicit forward-only `Up`,
  - required validation query/check,
  - idempotent data backfill behavior.
- Introduce helper package (`internal/migrations/ops`) for reusable operations:
  - `AddColumnIfNotExists`,
  - `CreateIndexConcurrently` (Postgres path),
  - batched backfill utility with checkpoint logging.
- Add `migrate lint` command to validate:
  - naming convention (`YYYYMMDDNN_slug`),
  - no direct model-wide `AutoMigrate` in new migration entries,
  - required description/name presence.
- Add migration-level structured logs: version, name, duration, rows touched, check results.

### Done Criteria
- New migration PRs have a documented checklist and fail CI if lint checks fail.
- At least one non-trivial new migration uses helper ops and post-checks.
- Operational logs show per-step duration and result for all applied steps.

## P2: Operational Rollout Model (Expand/Contract without Long-Lived Compatibility Windows)
### Scope
- Introduce a required rollout pattern for breaking schema or API-adjacent changes.

### Deliverables
- Add migration policy doc (`wiki/` if available, else `roadmap/`) with required phases:
  - expand (additive schema),
  - backfill/cutover,
  - contract (remove old columns/indexes after cutover).
- Avoid dual-write or parallel read paths unless unavoidable for operational safety; if used, time-box to one phase.
- Add explicit migration annotations for contract-phase blockers (prevent accidental destructive apply before readiness).
- Add `migrate guard` checks to assert readiness for contract steps (for example, old column read path disabled).

### Done Criteria
- One upcoming roadmap feature executes full expand/contract flow using this policy.
- Contract migration cannot run unless readiness checks pass.
- Breaking-change cutover notes are included in the relevant roadmap docs and PR templates.

## P3: CI/CD Enforcement and Drift Detection
### Scope
- Make migration safety enforceable in automation, not only by convention.

### Deliverables
- Add CI step to run:
  - `go test` for migration package,
  - `go run ./cmd/migrate check` against ephemeral Postgres instance,
  - migration replay test from empty DB to latest.
- Add schema drift check:
  - generate canonical DB schema snapshot after migrations,
  - compare with committed snapshot artifact in CI.
- Add forward-compat smoke check:
  - boot current app binary against DB migrated by previous commit + pending migrations.
- Add dialect parity checks:
  - run migration-sensitive integration/E2E tests against ephemeral Postgres as the required CI path,
  - keep SQLite checks limited to non-blocking API smoke coverage.
- Add migration timeout thresholds and alerting hooks in deployment logs.

### Done Criteria
- CI fails on migration drift, replay failure, or pending migration mismatch.
- Deployment pipeline can block release when migration checks fail.
- Teams get clear failure reason (which version failed and why).

## P4: Test Harness Convergence and Legacy Cleanup
### Scope
- Align test setup and local tooling with production-like migration paths.

### Deliverables
- Replace ad hoc test `AutoMigrate` usage in critical integration tests with versioned migration bootstrap helpers.
- Define policy for SQLite vs Postgres migration behavior in E2E:
  - either standardize on Postgres-backed E2E for migration-sensitive flows,
  - or explicitly mark SQLite harness as API-behavior-only (not migration parity).
- Extend `cmd/e2e-server` to support driver selection (`E2E_DB_DRIVER=postgres|sqlite`) and prefer Postgres in CI.
- Split initial large migration into focused follow-up migration steps where practical for future maintainability.
- Add command to generate migration stub file to reduce authoring mistakes.

### Done Criteria
- Critical handler/service integration suites bootstrap from versioned migrations.
- Migration-sensitive checks run on Postgres in CI.
- New migration authoring is standardized via generated stubs and linting.

## Data Model Changes
1. `schema_migrations`
- Add columns:
  - `name` (migration human-readable name),
  - `duration_ms`,
  - `checksum` (optional source hash for tamper detection),
  - `execution_meta` JSON (optional).

2. `migration_locks` (optional, if DB lock table used instead of advisory lock)
- Track lock owner, started_at, heartbeat_at for long-running backfills.

3. Schema snapshot artifact
- Committed SQL snapshot file under `internal/migrations/schema_snapshot.sql` for drift checks.

## Endpoint/API Plan
1. CLI/API execution surfaces
- Extend `cmd/migrate` and `cmd/cli migrate` with: `plan`, `status`, `lint`, `guard`.
- Keep current `up` and `check` command names unchanged for operator continuity.

2. No public HTTP API changes required in this roadmap.

## Execution Workflow in This Repo
1. Add migration framework changes in `internal/migrations` and wire new commands in:
   - `cmd/migrate/main.go`
   - `cmd/cli/commands/migrate.go`
2. Add migration docs/checklists in:
   - `internal/migrations/README.md`
   - `roadmap/` or `wiki/` policy docs
3. Add tests:
   - unit tests for ordering/validation/locking in `internal/migrations/*_test.go`
   - integration tests for replay and concurrent runners
4. Ensure command-level checks are represented in `Makefile` targets.
5. Run backend validation:
   - `GOCACHE=/tmp/go-build go test ./internal/migrations/...`
   - `GOCACHE=/tmp/go-build go test ./cmd/migrate/... ./cmd/cli/commands/...`

## Risk Register
- Transactional DDL differences:
  - Some Postgres operations (for example concurrent index creation) require non-transactional execution and explicit ordering.
- Startup auto-migration risk:
  - Auto-apply in app startup can increase blast radius during deploys unless gated by environment and migration policy.
- DB dialect mismatch risk:
  - E2E currently runs SQLite while production uses Postgres; migration behavior can diverge.
- Large backfill lock contention:
  - Unbatched updates can lock hot tables and degrade request latency.
- Contract-phase regressions:
  - Removing legacy columns too early can break still-deployed older binaries.

## Immediate Next Slice
1. Implement P0 primitives only:
- migration duplicate-version validation,
- advisory lock,
- `migrate status` command.

2. Add initial test coverage:
- concurrent runner lock test,
- malformed/duplicate version validation test,
- status output test.

3. Add `internal/migrations/README.md` with migration authoring checklist and naming rules.

## Dialect Parity Remediation Plan
1. Testing policy
- Migration correctness is validated on Postgres only.
- SQLite is allowed for fast API behavior smoke tests but is non-authoritative for schema/migration validation.

2. E2E harness changes
- Update `cmd/e2e-server/main.go` to accept:
  - `E2E_DB_DRIVER=postgres|sqlite`,
  - `E2E_DB_URL` for Postgres DSN,
  - existing SQLite path fallback for local smoke runs.
- Ensure both drivers execute `migrations.Run(db)` through the same code path.

3. CI gating
- Add required CI job:
  - start ephemeral Postgres,
  - run `go run ./cmd/migrate check`,
  - run migration replay test (empty -> latest),
  - run migration-sensitive integration/E2E suite.
- Optionally keep a separate non-blocking SQLite smoke job for quick feedback.

4. Developer workflow
- Add `make` targets for Postgres-backed E2E and migration replay checks.
- Document env setup and expected commands in `README.md` and `AGENTS.md`.
