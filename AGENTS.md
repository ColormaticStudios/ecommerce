# Repository Guidelines

This file is primarily to warn about common mistakes made and confusion points found  in this project. If you encounter something that surprises you, alert the user so the `AGENTS.md` can be updated. This is to prevent future agents from encountering the same issue.

Docs should be in `wiki/`, but it's a separate Git repo so may not be there. If not, ask the user. Read for information.

Read `README.md` and `frontend/README.md` for information.

Run `ls scripts/` to know what scripts are available.

Always run tests for code you modify, read `frontend/package.json` for frontend tests. Backend tests are standard Go tests, read `Makefile` for information.

Create new tests where needed. Use best practices, test valid and invalid data.

Sandbox-only note for agents: when you run `go test` directly in this Codex sandbox, prefix it with `GOCACHE=/tmp/go-build` because the sandbox cannot write to `~/.cache/go-build`.
Do not change `Makefile`, scripts, or other repo automation to add sandbox workarounds; those should stay focused on normal developer workflows.

Always run the formatter on code you modify:
- Backend code: `gofmt -w <file>`
- Frontend code: `cd frontend && bun x prettier -w <file>`
Format all:
- Backend code: `gofmt -w .`
- Frontend code: `cd frontend && bun run format`

Keep docs and tests up-to-date with new changes.

For the frontend, always use the proper component (ex. NumberInput instead of TextInput with type="number").

Whenever you add a new meaningful UI state to a frontend route or major view, add or update the matching Storybook route story so that state stays visible in the state catalog.

Keep handlers thin where possible; shared logic should move to reusable helpers/services.

This project is still very early, we will take a **breaking-change first** policy to get the project in the right shape. The project is not being used in production so there is no risk.

If a task takes more work to complete than it should (like a simple change touching many files), report this to the user. Bad patterns throughout the codebase should be caught and not repeated. Abide by an "if you see something, say something" policy.

## Repo notes:
SQLite in-memory test DBs can leak state across tests if you use `file::memory:?cache=shared`.
For isolated tests, use a per-test DSN (for example `file:<test-name>?mode=memory&cache=shared`) so each test gets its own database namespace.

Migration replay tests must use frozen legacy schema structs for historical setup/assertions, not current `models.*` types. Current models can gain columns that do not exist in earlier migration states and will break replay tests with schema drift errors.

Contract migration blockers are enforced by `go run ./cmd/migrate guard` and workflows that explicitly call guard, not by ordinary `migrations.Run()` / `make migrate`. Keep docs and code aligned on that distinction; local/dev DB bootstrap, snapshots, and test setup must not require `MIGRATIONS_ALLOW_CONTRACT=true`.

Checkout snapshot validation can run both before and after later checkout steps update `orders.total`. If provider flows validate a snapshot against an order, compare against the pre-authorization subtotal and the finalized snapshot total as appropriate; checking only one side can incorrectly reject valid post-snapshot shipping/tax flows.

On SQLite, `tx.Migrator().DropColumn("table_name", "column")` can panic when called with a raw table-name string during migrations. Prefer explicit SQL `ALTER TABLE ... DROP COLUMN ...` or a model-backed path instead of the string-table `DropColumn` helper.

`handlers/validation_integration_test.go` defines a shared `newTestDB` helper used by many handler tests across the package. Treat its signature as package-level API: changing it can break a large number of tests outside that file.

Several checkout/admin handlers use helper `respond(...)` closures inside `db.Transaction(...)` callbacks. If a transaction branch serializes an error response, the outer handler still needs an explicit guard before writing the normal success response; otherwise you can double-write a 200 after the error branch.

When a transaction callback needs to populate an outer-scope `snapshot` (or similar state used after the transaction), do not use short declaration like `snapshot, handled, err := ...` inside the callback. That shadows the outer variable, leaves the outer snapshot zero-valued, and later provider calls can fail with blank-provider errors such as `unknown payment provider: `.

GORM can silently persist `bool` fields with schema defaults instead of explicit `false` on `Create`, unless the insert explicitly selects zero-value fields. If a row must persist `false` (for example `is_published` on variant draft/live rows), prefer `tx.Select("*").Create(&row)` or another path that explicitly includes zero values.

Tailwind v4 in this frontend rejects `@apply` of project-defined custom classes during formatting/build tooling, so shared CSS tokens need to be expanded rather than composed from other local classes.

The search route keys search results by `product.sku`, so Storybook factories need unique `sku` overrides whenever multiple `makeProduct()` results are shown together, or the rendered list can behave incorrectly.

The E2E server uses one shared DB per Playwright run, so helper assertions must be scoped to test-owned data or they will race under multiple workers.

don’t run `bun run check` in parallel with Playwright (the E2E test) in this frontend, because `svelte-kit sync` can reload the dev client mid-suite and create false E2E failures.

## OpenAPI Contract Workflow
- Update `api/openapi.yaml` first whenever request/response shapes change.
- Regenerate contract artifacts with `make openapi-gen`.
- Commit generated files:
  - `internal/apicontract/openapi.gen.go`
  - `frontend/src/lib/api/generated/openapi.ts`
- Run `make openapi-check` before finishing to ensure generated files are up to date.
- `make openapi-check` is a clean-tree guard against `HEAD`, not just a regeneration smoke test. If those generated files are intentionally uncommitted in your working tree, it will still fail after a fresh `make openapi-gen`.
- Prefer generated types in backend/frontend code over hand-written duplicate API payload interfaces.

## Svelte Effect Safety
- Keep `$effect` blocks as pure state synchronization whenever possible: derive from reactive inputs and avoid side effects.
- Do not call async functions from `$effect` if they can write to state the same effect reads (directly or indirectly).
- Prefer triggering async work from explicit events (`onMount`, user actions, dedicated loader functions) instead of effect bodies.
- Avoid helper calls inside `$effect` when those helpers mutate state that contributes to the same effect dependency graph.
- If an effect must write state, ensure the writes target state that does not feed back into that same effect’s dependencies.
- In load/hydration `$effect` blocks, do not call helper functions that read/write local `$state`; use `untrack` or move logic to explicit events.
