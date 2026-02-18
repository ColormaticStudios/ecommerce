# Repository Guidelines

## Project Structure & Module Organization
- Backend API entrypoint: `main.go`; CLI entrypoint: `cmd/cli/main.go`.
- HTTP handlers live in `handlers/`, middleware in `middleware/`, domain models in `models/`, configuration in `config/`.
- Media-specific internals are in `internal/media/`; helper scripts are in `scripts/`.
- Frontend (SvelteKit + TypeScript) is in `frontend/`:
  - routes: `frontend/src/routes/`
  - reusable UI/API code: `frontend/src/lib/`
- Documentation: `README.md` (setup), `API.md` (contract). Keep both in sync with behavior changes.

## Build, Test, and Development Commands
- Backend build: `make api` (builds `bin/ecommerce-api`)
- CLI build: `make cli` (builds `bin/ecommerce-cli`)
- Run backend locally: `make run`
- Run backend tests: `make test` (executes `go test ./...`)
- Start dev database: `scripts/run-dev-db-docker.sh` or `scripts/run-dev-db-podman.sh`
- Seed test data: `scripts/populate-test-database.sh`
- Frontend dev: `cd frontend && bun run dev`
- Frontend checks: `cd frontend && bun run check && bun run lint`
- Frontend format: `cd frontend && bun run format`

## Coding Style & Naming Conventions
- Go: follow `gofmt` formatting, idiomatic package names (short/lowercase), exported identifiers in `PascalCase`.
- Svelte/TS: use Prettier + ESLint defaults; component files in `PascalCase` (e.g., `NumberInput.svelte`), utility modules in lowercase (`api.ts`, `utils.ts`).
- Keep handlers thin where possible; shared logic should move to reusable helpers/services.

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
