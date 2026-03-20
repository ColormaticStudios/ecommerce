# Migrations

This package uses ordered, forward-only Go migrations.

Applied migration rows persist a checksum derived from migration definitions. If
an already-applied migration is edited later, runners fail fast on checksum mismatch.

## Required Pattern

1. One change intent per migration version.
1. Forward-only `Up` function; do not add rollback logic here.
1. Include a clear human-readable `Name`.
1. Explicitly set `TransactionMode`:
   - `required` for default transactional migrations,
   - `none` for operations that cannot run in a transaction (for example `CREATE INDEX CONCURRENTLY`).
1. Tag each migration (`expand`, `backfill`, `contract`, etc.).
1. Contract-tagged migrations must include `ContractBlockers` (for example `allow_contract_migrations`).
   These blockers are enforced by `go run ./cmd/migrate guard` and any release/CI readiness checks that call it, not by ordinary `Run()` execution used in local bootstrap, snapshots, or tests.
1. Include a validation step (query or invariant check) in the migration body when possible.
1. Prefer `PostChecks` for migration-level invariant checks.
1. Data backfills must be idempotent so reruns are safe if a step is retried.
1. Prefer reusable helpers in `internal/migrations/ops` over raw SQL where practical.

## Naming Rules

- Version format: `YYYYMMDDNN_slug`
- `YYYYMMDD`: calendar date.
- `NN`: sequence for the day (two digits).
- `slug`: lowercase letters, numbers, and underscores.
- Versions must be unique and strictly increasing in `orderedMigrations`.

## Author Checklist

1. Add a new `Migration` entry to `internal/migrations/migrations.go`.
1. Confirm version/name lint requirements pass.
1. Add or update migration tests for happy path and failure path.
1. Run backend formatting and tests:
   - `gofmt -w internal/migrations/*.go`
   - `GOCACHE=/tmp/go-build go test ./internal/migrations/...`
   - `go run ./cmd/migrate lint`
   - `go run ./cmd/migrate guard` (before promoting or shipping pending contract migrations)
   - `go run ./cmd/migrate snapshot` and commit `internal/migrations/schema_snapshot.sql` when schema changes
   - `go run ./cmd/migrate drift-check`
   - Optional Postgres advisory-lock integration test:
     - `MIGRATIONS_TEST_POSTGRES_DSN=postgres://... GOCACHE=/tmp/go-build go test ./internal/migrations/... -run PostgresLock`

## Commands

- `go run ./cmd/migrate lint`: validate migration definitions and policy checks.
- `go run ./cmd/migrate guard`: evaluate readiness checks for pending contract-tagged migrations before release/CI rollout.
- `go run ./cmd/migrate snapshot [path]`: write canonical schema snapshot.
- `go run ./cmd/migrate drift-check [path]`: compare live schema to committed snapshot.
- `go run ./cmd/migrate new <slug>`: generate a migration stub under `internal/migrations/stubs/`.
- `MIGRATIONS_STEP_ALERT_THRESHOLD_MS` (default `30000`): emit `migration_step_alert` logs when a step exceeds threshold.
