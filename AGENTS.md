# Repository Guidelines

## Project Structure & Module Organization
- Backend API entrypoint: `main.go`; CLI entrypoint: `cmd/cli/main.go`.
- HTTP handlers live in `handlers/`, middleware in `middleware/`, domain models in `models/`, configuration in `config/`.
- Media-specific internals are in `internal/media/`; helper scripts are in `scripts/`.
- Frontend (SvelteKit + TypeScript) is in `frontend/`:
  - routes: `frontend/src/routes/`
  - reusable UI/API code: `frontend/src/lib/`
- API contract source: `api/openapi.yaml` (single source of truth for generated API types).
- Documentation: `README.md` (setup), `API.md` (endpoint docs). Keep docs and OpenAPI in sync with behavior changes.

## Build, Test, and Development Commands
- Backend build: `make api` (builds `bin/ecommerce-api`)
- CLI build: `make cli` (builds `bin/ecommerce-cli`)
- Run backend locally: `make run`
- Run backend tests: `make test` (executes `go test ./...`)
- Generate backend + frontend API contract types: `make openapi-gen`
- Verify generated API contract files are current: `make openapi-check`
- Generate API docs from OpenAPI: `make openapi-docs`
- Start dev database: `scripts/run-dev-db-docker.sh` or `scripts/run-dev-db-podman.sh`
- Seed test data: `scripts/populate-test-database.sh`
- Frontend dev: `cd frontend && bun run dev`
- Frontend checks: `cd frontend && bun run check && bun run lint`
- Frontend API type generation only: `cd frontend && bun run gen:api`

## Formatting Commands
- Note: always format after editing a source code file
- Backend format: `gofmt -w .` (or target specific paths, e.g. `gofmt -w handlers models internal`).
- Optional backend import cleanup + formatting: `go fmt ./...`
- Frontend format: `cd frontend && bun run format`
- Format a specific frontend file: `cd frontend && bun x prettier -w <file path>`

## OpenAPI Contract Workflow
- Update `api/openapi.yaml` first whenever request/response shapes change.
- Regenerate contract artifacts with `make openapi-gen`.
- Commit generated files:
  - `internal/apicontract/openapi.gen.go`
  - `frontend/src/lib/api/generated/openapi.ts`
- Run `make openapi-check` before opening a PR to ensure generated files are up to date.
- Prefer generated types in backend/frontend code over hand-written duplicate API payload interfaces.

## Coding Style & Naming Conventions
- Go: follow `gofmt` formatting, idiomatic package names (short/lowercase), exported identifiers in `PascalCase`.
- Svelte/TS: use Prettier + ESLint defaults; component files in `PascalCase` (e.g., `NumberInput.svelte`), utility modules in lowercase (`api.ts`, `utils.ts`).
- Keep handlers thin where possible; shared logic should move to reusable helpers/services.

## Svelte Effect Safety
- Keep `$effect` blocks as pure state synchronization whenever possible: derive from reactive inputs and avoid side effects.
- Do not call async functions from `$effect` if they can write to state the same effect reads (directly or indirectly).
- Prefer triggering async work from explicit events (`onMount`, user actions, dedicated loader functions) instead of effect bodies.
- Avoid helper calls inside `$effect` when those helpers mutate state that contributes to the same effect dependency graph.
- If an effect must write state, ensure the writes target state that does not feed back into that same effect’s dependencies.

## Testing Guidelines
- Backend tests use Go’s `testing` package and live next to source (`*_test.go`), e.g., `handlers/orders_test.go`.
- Prioritize handler behavior, auth/middleware paths, and media edge cases.
- Run targeted tests during iteration (example: `go test ./handlers -run Orders`) and finish with `make test`.
- Frontend currently relies on type/lint checks (`check`, `lint`) rather than a dedicated test suite.

## Commit & Pull Request Guidelines
- Follow concise, imperative commit subjects (history examples: “Add admin UI”, “Fix API naming inconsistencies”).
- Keep commits scoped to one concern; avoid mixing backend/frontend refactors without reason.
- PRs should include:
  - what changed and why
  - affected endpoints/pages
  - docs updates (`API.md`, `README.md`) when contracts or setup changed
  - screenshots for UI changes (`frontend/`).
