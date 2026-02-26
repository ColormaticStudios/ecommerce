# Roadmap Document Guidelines

## Purpose
Roadmap docs define implementation direction, execution order, and completion criteria.
They are planning artifacts for engineers, not marketing documents.

## Scope of a Roadmap Doc
Each roadmap document should answer:
- What problem are we solving?
- What is in scope and out of scope?
- What is the phased delivery plan?
- What artifacts must ship (API, schema, code, tests, docs)?
- How do we know each phase is done?

## Required Structure
Use this section order unless there is a strong reason not to:
1. `Current Baseline`
2. `Goals`
3. `Non-Goals`
4. `Delivery Order` (explicit phase sequence)
5. Phase sections (for example `P0`, `P1`, `P2`) with:
- `Scope`
- `Deliverables`
- `Done Criteria`
6. `Data Model Changes` (if applicable)
7. `Endpoint/API Plan` (if applicable)
8. `Execution Workflow in This Repo`
9. `Risk Register`
10. `Immediate Next Slice`

## Writing Style
- Be concrete and testable.
- Prefer short bullets over long paragraphs.
- Use repo-specific paths, endpoint names, and table names.
- Avoid vague wording like "improve", "optimize", or "support better".
- Include migration/backward compatibility notes when replacing existing behavior.

## Architecture Alignment Checklist
Every roadmap must explicitly map changes to this codebase:
- API contract: identify `api/openapi.yaml` changes and affected generated handlers.
- Backend wiring: identify affected `handlers/`, `internal/`, and model registration in `main.go` `AutoMigrate`.
- Data model: use existing model conventions (`models.BaseModel`, `models.Money` where relevant).
- Frontend impact: call out existing fields consumed by `frontend/` and whether changes are additive or breaking.
- Runtime model: if background work is needed, state where worker lifecycle lives in this repo.

## Cross-Roadmap Compatibility Checklist
Every roadmap should explicitly list dependencies/assumptions against other roadmap docs:
- Canonical customer mutation surface (`/api/v1/checkout/*` vs `/api/v1/me/*`).
- Canonical purchasable identifier (`product_variant_id` vs `product_id`).
- Shared ownership/session model (`user_id` vs `checkout_session_id`).
- Shared totals pipeline (catalog pricing -> discounts/promotions -> provider snapshot/payment).
- Whether temporary compatibility wrappers exist and which roadmap phase removes them.

## Phase Quality Bar
Each phase must include:
- A clear boundary (what it includes and excludes).
- Observable outputs (code, schema, endpoints, jobs, docs).
- Measurable done criteria (pass/fail checks).
- Dependencies on previous phases.

## Repository-Specific Rules
- If request/response shapes change, update `api/openapi.yaml` first.
- Regenerate contract artifacts with `make openapi-gen`.
- Verify generated files are current with `make openapi-check`.
- Prefer generated API types over duplicated handwritten payload types.
- Keep handlers thin; place shared logic in reusable services/helpers.

## Testing Expectations in Roadmaps
Roadmaps should include test intent, including:
- Valid path behavior.
- Invalid input/rejection behavior.
- Retry/idempotency behavior for mutation flows.
- Concurrency/race safety where duplicate execution is possible.
- Regression coverage for compatibility wrappers.

## Definition of Done (Document Level)
A roadmap doc is ready when:
- It has all required sections.
- Each phase has concrete done criteria.
- API and schema impacts are explicit.
- Architecture mapping to existing repo components is explicit.
- Risks and operational concerns are listed.
- The immediate next slice is implementable without extra discovery.

## Template
Use this template for new roadmap docs:

```md
# <Topic> Roadmap

## Current Baseline
- ...

## Goals
- ...

## Non-Goals
- ...

## Delivery Order
1. P0: ...
2. P1: ...

## P0: <Phase Name>
### Scope
- ...

### Deliverables
- ...

### Done Criteria
- ...

## P1: <Phase Name>
### Scope
- ...

### Deliverables
- ...

### Done Criteria
- ...

## Data Model Changes
1. `<table_or_model>`
- ...

## Endpoint/API Plan
1. ...

## Execution Workflow in This Repo
1. Update `api/openapi.yaml` first (if API shapes change).
2. Run `make openapi-gen`.
3. Implement changes.
4. Run `make openapi-check`.
5. Run tests for touched areas.

## Risk Register
- ...

## Immediate Next Slice
1. ...
```
