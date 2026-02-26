# Repository Guidelines

This file is primarily to warn about common mistakes made and confusion points found  in this project. If you encounter something that surprises you, alert the user so the `AGENTS.md` can be updated. This is to prevent future agents from encountering the same issue.

Docs should be in `wiki/`, but it's a separate Git repo so may not be there. If not, ask the user. Read for information.

Read `README.md` and `frontend/README.md` for information.

Run `ls scripts/` to know what scripts are available.

Always run tests for code you modify, read `frontend/package.json` for frontend tests. Backend tests are standard Go tests, read `Makefile` for information.

Create new tests where needed. Use best practices, test valid and invalid data.

For your sandbox environment, you will need to prefix Go test with `GOCACHE=/tmp/go-build` because you do not have write access to `~/.cache/go-build`.

Always run the formatter on code you modify:
- Backend code: `gofmt -w <file>`
- Frontend code: `cd frontend && bun x prettier -w <file>`
Format all:
- Backend code: `gofmt -w .`
- Frontend code: `cd frontend && bun run format`

Keep docs and tests up-to-date with new changes.

For the frontend, always use the proper component (ex. NumberInput instead of TextInput with type="number").

Keep handlers thin where possible; shared logic should move to reusable helpers/services.

This project is still very early, don't hesitate to clean things up or make breaking changes to the API, schema, UI, etc. We just need to get the project in the right shape.

## OpenAPI Contract Workflow
- Update `api/openapi.yaml` first whenever request/response shapes change.
- Regenerate contract artifacts with `make openapi-gen`.
- Commit generated files:
  - `internal/apicontract/openapi.gen.go`
  - `frontend/src/lib/api/generated/openapi.ts`
- Run `make openapi-check` before finishing to ensure generated files are up to date.
- Prefer generated types in backend/frontend code over hand-written duplicate API payload interfaces.

## Svelte Effect Safety
- Keep `$effect` blocks as pure state synchronization whenever possible: derive from reactive inputs and avoid side effects.
- Do not call async functions from `$effect` if they can write to state the same effect reads (directly or indirectly).
- Prefer triggering async work from explicit events (`onMount`, user actions, dedicated loader functions) instead of effect bodies.
- Avoid helper calls inside `$effect` when those helpers mutate state that contributes to the same effect dependency graph.
- If an effect must write state, ensure the writes target state that does not feed back into that same effect’s dependencies.
- In load/hydration `$effect` blocks, do not call helper functions that read/write local `$state`; use `untrack` or move logic to explicit events.
