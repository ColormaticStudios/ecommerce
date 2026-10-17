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

if ! command -v curl >/dev/null 2>&1; then
  echo "error: curl is required for forward-compat smoke" >&2
  exit 1
fi

export DATABASE_URL="${MIGRATIONS_CI_POSTGRES_DSN:-${DATABASE_URL:-}}"

previous_commit="$(git rev-parse --verify HEAD~1)"
worktree_dir="$(mktemp -d /tmp/ecommerce-forward-compat-XXXXXX)"
api_pid=""
cleanup() {
  if [[ -n "$api_pid" ]] && kill -0 "$api_pid" 2>/dev/null; then
    kill "$api_pid" 2>/dev/null || true
    wait "$api_pid" 2>/dev/null || true
  fi
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
GOCACHE=/tmp/go-build go build -o "$worktree_dir/current-api" ./main.go
smoke_port="${MIGRATIONS_SMOKE_PORT:-39001}"
PORT="$smoke_port" GIN_MODE=release TELEMETRY_ENABLED=false timeout 15s "$worktree_dir/current-api" >"$worktree_dir/current-api.log" 2>&1 &
api_pid=$!
ready=false
for _ in {1..100}; do
  if ! kill -0 "$api_pid" 2>/dev/null; then
    break
  fi
  if curl --silent --fail --max-time 1 "http://127.0.0.1:$smoke_port/api/v1/localization/locales" >/dev/null; then
    ready=true
    break
  fi
  sleep 0.1
done
set +e
wait "$api_pid"
status=$?
set -e
api_pid=""

if [[ "$status" -ne 0 && "$status" -ne 124 ]]; then
  echo "error: current API binary failed to boot after previous-commit migration path" >&2
  cat "$worktree_dir/current-api.log" >&2
  exit 1
fi

if [[ "$ready" != true ]]; then
  echo "error: forward-compat smoke did not observe HTTP readiness" >&2
  cat "$worktree_dir/current-api.log" >&2
  exit 1
fi

echo "[forward-compat] status=ok"
