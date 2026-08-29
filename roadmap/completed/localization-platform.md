# Localization and Translation Management Roadmap

## Terminology
- Always use the full word `localization` in code, paths, identifiers, documentation, and user-facing copy.
- Numeronym abbreviations for localization and internationalization are prohibited throughout every phase of this roadmap.

## Current Baseline
- P0 and the platform foundations of P1 through P4 are implemented: the platform owns locale configuration, fallback, translation lifecycle, request negotiation, bundle delivery, account preferences, market defaults, entity-localized fields, translation operations, usage context, rollout controls, and localized communication rendering.
- CMS consumes the shared locale registry and resolver instead of owning a runtime locale system.
- Core storefront, cart, checkout, account, and admin-shell strings use localization keys; remaining surfaces migrate incrementally in later phases.
- Problem-details responses expose stable `error_code`, optional `message_key`, and structured `message_params` for localized frontend rendering.
- `defaults/localization.en-US.json` is the extracted baseline source catalog and is protected against migration-seed drift by tests.
- Customer communications roadmap (`roadmap/customer-communications-email-sms.md`) introduces outbound delivery infrastructure, but not localized template management.
- Ecommerce CMS owns localized content variants while consuming the shared platform registry and fallback contract.

## Goals
- Localize all user-facing text across storefront, checkout, account, admin, transactional communications, and API-driven UI states.
- Provide strong admin UX for translation operations:
- source/target side-by-side editing,
- workflow states (draft/review/published),
- placeholder/plural validation,
- bulk operations and import/export,
- preview by locale before publish.
- Keep runtime deterministic with explicit fallback chains and zero silent translation failures.
- Keep implementation OpenAPI-first and compatible with generated backend/frontend contract artifacts.
- Provide auditability for translation mutations and publishing to align with security/compliance roadmap controls.

## Non-Goals
- Shipping production ML translation quality scoring in initial phases.
- Per-user personalized copy generation.
- Solving multilingual stemming/search relevance in first localization release (search roadmap handles this later).
- Translating raw user-generated content automatically by default.

## Delivery Order
1. P0: Localization domain foundation and locale resolution contract.
2. P1: Frontend/backend runtime integration and key extraction.
3. P2: Entity localization for CMS, storefront settings, and commerce metadata.
4. P3: Translation Admin UX and workflow controls.
5. P4: Communications, error surfaces, and operational rollout.
6. P5: Hardening, quality gates, and compatibility cleanup.

## Cross-Roadmap Alignment
- CMS (now baseline):
  - This roadmap defines the shared localization platform consumed by CMS.
  - CMS P6 should use this roadmap’s locale/fallback/workflow model rather than defining a second localization stack.
- Product catalog depth baseline:
  - Catalog’s canonical purchasable model is already variant-first; localization adds translated display fields only.
  - Do not reintroduce product-level purchase semantics while localizing product copy.
- Search (`roadmap/search-discoverability-quality.md`):
  - Initial localization rollout keeps search behavior deterministic with canonical-language indexing fallback.
  - Multilingual search relevance remains a later search phase and consumes localized content once stable.
- Customer communications (`roadmap/customer-communications-email-sms.md`):
  - Transactional email/SMS template rendering must read localized template variants from this localization domain.
- Legal/compliance/security (`roadmap/legal-compliance-security.md`):
  - Translation publish and approval actions emit auditable events and respect RBAC.
- Merchant analytics (`roadmap/merchant-analytics-reporting.md`):
  - Locale and market become reporting dimensions for funnel/conversion and communication outcomes.
- Checkout baseline:
  - Canonical customer mutation surface remains `/api/v1/checkout/*`; localization must not introduce alternate checkout APIs.

## P0: Localization Domain Foundation
Status: Complete.

### Scope
- Introduce locale registry, fallback rules, key namespace model, and publication semantics.
- Define request locale resolution contract for API and frontend loaders.
- Define a stable key format for UI copy, errors, and templates.

### Deliverables
- New models/tables for:
- locale registry (`en-US`, `fr-FR`, etc.) with enabled/default flags,
- translation keys with namespace and source text metadata,
- per-locale translation values with state (`draft|review|published`),
- translation release snapshots for deterministic runtime reads.
- Locale resolution policy:
- precedence `explicit user choice -> account preference -> market default -> global default`,
- explicit fallback chain (example: `fr-CA -> fr -> en-US`).
- OpenAPI contract additions for locale metadata and bundle retrieval.
- Backend service package `internal/services/localization/` for key lookup, fallback resolution, and release activation.

### Done Criteria
- Runtime can resolve a locale deterministically for anonymous and authenticated requests.
- Translation lookup returns either localized value or explicit fallback metadata (never silent empty string).
- Locale registry and key/value state transitions are covered by backend tests for valid and invalid paths.

## P1: Runtime Integration and Key Migration
Status: Partially complete. Runtime integration and core-route migration are complete; repository-wide storefront/admin copy migration remains open.

### Scope
- Integrate the localization runtime in frontend and backend.
- Replace hard-coded user-facing strings with key-based lookup across core pages/components.
- Standardize localized error handling contract.

### Deliverables
- Frontend localization module in `frontend/src/lib/localization/`:
- locale store,
- bundle loader/cache,
- formatting helpers (plural/select/interpolation).
- Backend middleware for locale negotiation from request headers/cookies/profile.
- Error contract update in `api/openapi.yaml`:
- stable machine-readable `error_code`,
- optional `message_key`,
- structured interpolation params.
- Migration tooling/scripts to extract baseline English source strings from:
- `frontend/src/routes/**/*.svelte`,
- `frontend/src/lib/components/**/*.svelte`,
- current storefront defaults in `defaults/storefront.json`.

### Done Criteria
- Core storefront/cart/checkout/account/admin shell strings are rendered through translation keys.
- API errors can be localized in frontend without parsing English strings.
- Missing keys render readable source copy in every environment and are tracked in logs/metrics without exposing internal key markers to users.

## P2: Entity Localization (CMS, Storefront, Catalog, Legal Copy)
Status: Complete for the currently supported CMS, storefront-settings, catalog, and SEO entity policies. Additional legal surfaces remain ordinary CMS content until dedicated contracts are introduced.

### Scope
- Localize content entities that are merchant-authored or customer-visible domain data.
- Provide compatibility path from single-language fields to localized variants.

### Deliverables
- Storefront and CMS content:
- add localized field support for titles, subtitles, CTA labels, footer copy, page metadata.
- Catalog and category metadata:
- localized display fields for `name`, `description`, and merchandising text.
- Legal/compliance content:
- localized policy and notice surfaces (privacy, returns, shipping copy).
- Backward compatibility adapters:
- existing single-language fields remain readable during migration,
- write path moves to localized payloads with default-locale backfill.

### Done Criteria
- Published storefront/CMS/catalog pages render locale-specific content with deterministic fallback.
- Admin can migrate existing default-language content to localized forms without data loss.
- Legacy single-language response fields are either removed (breaking cut) or clearly marked deprecated with removal phase.

## P3: Translation Admin UX and Workflow
Status: Complete. Translation keys support multiple structured usage records with route/component guidance and optional screenshots owned by the media subsystem.

### Scope
- Deliver translation operations UX for admins/editors with high throughput and quality controls.
- Implement role-gated approval and publish flow.

### Deliverables
- Admin translation workspace in `frontend/src/routes/admin`:
- side-by-side source/target editor with key context and screenshots/usage hints,
- filterable queue by namespace, locale, state, assignee, stale keys, missing keys,
- keyboard-first bulk editing and bulk state transitions,
- in-context preview links for storefront/CMS routes per locale.
- Workflow features:
- status transitions (`draft -> review -> published`),
- reviewer assignment and comments,
- diff view between published and draft text,
- glossary/term lock and placeholder validation.
- Import/export APIs and UI:
- CSV/XLIFF/JSON export by locale/namespace,
- validated re-import with conflict detection and dry-run report.
- RBAC integration:
- translator/editor/publisher roles aligned with legal/security roadmap.

### Done Criteria
- A non-developer admin can update and publish translations without direct DB/API tooling.
- Invalid placeholders/plural forms are blocked pre-publish with field-level errors.
- Publish actions create audit events with actor, locale, namespace, and change summary.

## P4: Communications, System Messages, and Rollout Controls
Status: Complete for localization rendering and rollout controls only. Provider delivery, retries, and proof that messages are sent remain unimplemented and owned by `roadmap/customer-communications-email-sms.md`.

### Scope
- Extend localization to outbound communications and operational/user status messaging.
- Roll out locale-aware behavior safely with feature flags and monitoring.

### Deliverables
- Communications localization integration:
- localized templates for order/payment/shipping/return events,
- fallback to default locale when recipient locale unavailable with explicit logging.
- Localize remaining high-surface strings:
- checkout validations,
- order lifecycle statuses shown to customers,
- admin-facing operational warnings and confirmations.
- Feature flags/config:
- per-locale enablement gates,
- staged rollout percentages by route/domain.
- Observability:
- missing-key rate,
- fallback-hit rate by locale,
- translation publish latency and rollback events.

### Done Criteria
- Transactional emails/SMS are sent in recipient locale when available.
- Rollout can enable/disable locales without deploy.
- Operational dashboards can identify missing translation hotspots and publish regressions.

## P5: Hardening and Compatibility Cleanup
Status: Partially complete. Release safety, ETags, cache invalidation, rollback, and local performance gates are implemented. Repository-wide hard-coded-copy enforcement and deployed regional p95 verification remain open.

### Scope
- Remove temporary compatibility wrappers and enforce localization quality bar in CI.
- Finalize breaking API/schema cleanup.

### Deliverables
- Cleanup:
- remove deprecated single-language fields and legacy translation adapters,
- require localized payload contracts for newly introduced user-facing fields.
- CI gates:
- fail on new hard-coded user-facing strings in covered frontend/backend paths,
- fail on missing required locale translations for release-critical namespaces.
- Performance and caching:
- locale bundle versioning/ETag support,
- cache invalidation on translation publish.
- Recovery tooling:
- one-click rollback to prior translation release snapshot.

### Done Criteria
- No release-critical user-facing surface depends on hard-coded strings.
- Localization regressions are blocked pre-merge by automated checks.
- Runtime bundle fetch and lookup latency stay within agreed SLOs.

## Data Model Changes
1. `locales`
- Fields: `id`, `code` (BCP-47), `name`, `is_enabled`, `is_default`, `fallback_locale_id`, timestamps.

2. `translation_keys`
- Fields: `id`, `namespace`, `key`, `source_text`, `description`, `owner_domain`, `is_deprecated`, timestamps.
- Constraint: unique (`namespace`, `key`).

3. `translation_values`
- Fields: `id`, `translation_key_id`, `locale_id`, `value`, `state`, `version`, `updated_by`, `reviewed_by`, timestamps.
- Constraint: unique (`translation_key_id`, `locale_id`, `version`).

4. `translation_releases`
- Fields: `id`, `name`, `status`, `published_at`, `published_by`, `notes`, `snapshot_hash`.

5. `translation_release_entries`
- Fields: `id`, `release_id`, `translation_value_id`.

6. `translation_comments` (optional but recommended for review UX)
- Fields: `id`, `translation_value_id`, `author_id`, `comment`, `resolved_at`, timestamps.

## Endpoint/API Plan
1. Public/runtime
- `GET /api/v1/localization/locales`
- `GET /api/v1/localization/bundles/{locale}`
- `GET /api/v1/localization/bundles/{locale}/meta`

2. Admin translation management
- `GET /api/v1/admin/localization/keys`
- `POST /api/v1/admin/localization/keys`
- `GET /api/v1/admin/localization/keys/{id}/values`
- `PUT /api/v1/admin/localization/keys/{id}/values/{locale}`
- `POST /api/v1/admin/localization/values/{id}/submit-review`
- `POST /api/v1/admin/localization/values/{id}/publish`
- `POST /api/v1/admin/localization/import`
- `POST /api/v1/admin/localization/export`

3. Error contract and localized domain payloads
- Extend error schemas with `error_code`, `message_key`, `message_params`.
- Update user-facing payload schemas (storefront/CMS/catalog/communications templates) to support localized fields or key references.

## Execution Workflow in This Repo
1. Update `api/openapi.yaml` first for localization endpoints, error schema updates, and localized entity payload changes.
2. Run `make openapi-gen` and commit generated files:
- `internal/apicontract/openapi.gen.go`
- `frontend/src/lib/api/generated/openapi.ts`
3. Add/modify models in `models/` and register migrations in `internal/migrations` (including default-locale backfills from existing fields).
4. Implement localization services in `internal/services/localization/` and keep strict endpoints in `internal/httpapi/` thin.
5. Integrate the frontend localization runtime in `frontend/src/lib/localization/` and migrate route/component strings incrementally by namespace.
6. Add admin translation UI flows under `frontend/src/routes/admin`.
7. Run `make openapi-check`.
8. Run backend tests with sandbox cache: `GOCACHE=/tmp/go-build go test ./...`.
9. Run frontend checks/tests: `cd frontend && bun run check && bun run lint && bun run test:e2e` (or targeted Playwright suites for touched flows).

## Risk Register
- Key drift risk between frontend/backend if namespaces are inconsistent.
- Fallback misuse can mask missing translations and ship partial-language UX.
- Placeholder/plural errors can break critical checkout or communication copy at runtime.
- Translation publish concurrency can cause stale caches or mixed-version bundles.
- Large bundle sizes can hurt TTFB and route transitions for multi-locale storefronts.
- Roadmap overlap with CMS P6 can create duplicate implementations if ownership is not explicitly reassigned.

## Immediate Next Slice
1. Expand source extraction and hard-coded-copy enforcement beyond the current checkout-focused file policy when the repository-wide UI migration is approved.
2. Implement the separate communications outbox/provider roadmap and consume `RenderCommunication` before delivery.
3. Validate the documented regional bundle-fetch p95 in a deployed environment; local CI continues to enforce deterministic backend, endpoint, and frontend lookup budgets.
