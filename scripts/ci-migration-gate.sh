#!/usr/bin/env bash
set -euo pipefail

# Required by cmd/migrate and migration integration tests.
if [[ -z "${MIGRATIONS_CI_POSTGRES_DSN:-}" && -z "${DATABASE_URL:-}" ]]; then
  echo "error: set MIGRATIONS_CI_POSTGRES_DSN or DATABASE_URL" >&2
  exit 1
fi

export DATABASE_URL="${MIGRATIONS_CI_POSTGRES_DSN:-${DATABASE_URL:-}}"
export MIGRATIONS_TEST_POSTGRES_DSN="${MIGRATIONS_TEST_POSTGRES_DSN:-$DATABASE_URL}"

forward_schema="migration_gate_forward_$(date +%s%N)"
cleanup_forward_schema() {
  GOCACHE=/tmp/go-build go run ./scripts/postgres-schema-dsn.go drop "$DATABASE_URL" "$forward_schema" >/dev/null 2>&1 || true
}
trap cleanup_forward_schema EXIT

echo "[migration-gate] preparing isolated schema for forward-compat smoke"
forward_database_url="$(GOCACHE=/tmp/go-build go run ./scripts/postgres-schema-dsn.go create "$DATABASE_URL" "$forward_schema")"

echo "[migration-gate] running migration-focused tests"
GOCACHE=/tmp/go-build go test ./internal/migrations/... ./cmd/migrate/... ./cmd/cli/commands/...

echo "[migration-gate] applying migrations and checking latest version"
GOCACHE=/tmp/go-build go run ./cmd/migrate up
GOCACHE=/tmp/go-build go run ./cmd/migrate check

echo "[migration-gate] validating snapshot drift"
GOCACHE=/tmp/go-build go run ./cmd/migrate drift-check

echo "[migration-gate] running postgres replay test (empty -> latest)"
GOCACHE=/tmp/go-build go test ./internal/migrations/... -run TestRunReplayFromEmptyToLatestPostgres -count=1

echo "[migration-gate] running forward-compat smoke check"
MIGRATIONS_CI_POSTGRES_DSN="$forward_database_url" DATABASE_URL="$forward_database_url" ./scripts/ci-forward-compat-smoke.sh
