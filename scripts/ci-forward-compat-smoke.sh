#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${MIGRATIONS_CI_POSTGRES_DSN:-}" && -z "${DATABASE_URL:-}" ]]; then
  echo "error: set MIGRATIONS_CI_POSTGRES_DSN or DATABASE_URL" >&2
  exit 1
fi

if ! command -v timeout >/dev/null 2>&1; then
  echo "error: timeout command is required for forward-compat smoke" >&2
  exit 1
fi

export DATABASE_URL="${MIGRATIONS_CI_POSTGRES_DSN:-${DATABASE_URL:-}}"

previous_commit="$(git rev-parse --verify HEAD~1)"
worktree_dir="$(mktemp -d /tmp/ecommerce-forward-compat-XXXXXX)"
cleanup() {
  git worktree remove --force "$worktree_dir/prev" >/dev/null 2>&1 || true
  rm -rf "$worktree_dir"
}
trap cleanup EXIT

echo "[forward-compat] preparing previous commit worktree ${previous_commit}"
git worktree add --detach "$worktree_dir/prev" "$previous_commit" >/dev/null

echo "[forward-compat] migrating database with previous commit"
(
  cd "$worktree_dir/prev"
  GOCACHE=/tmp/go-build go run ./cmd/migrate up
)

echo "[forward-compat] applying current commit pending migrations"
GOCACHE=/tmp/go-build go run ./cmd/migrate up

echo "[forward-compat] booting current API binary against upgraded database"
set +e
PORT=39001 GIN_MODE=release GOCACHE=/tmp/go-build timeout 15s go run ./main.go >"$worktree_dir/current-api.log" 2>&1
status=$?
set -e

if [[ "$status" -ne 0 && "$status" -ne 124 ]]; then
  echo "error: current API binary failed to boot after previous-commit migration path" >&2
  cat "$worktree_dir/current-api.log" >&2
  exit 1
fi

if ! rg -q "Server starting on port|Database migration completed" "$worktree_dir/current-api.log"; then
  echo "error: forward-compat smoke did not observe API startup logs" >&2
  cat "$worktree_dir/current-api.log" >&2
  exit 1
fi

echo "[forward-compat] status=ok"
