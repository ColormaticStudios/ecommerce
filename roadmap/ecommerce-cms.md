# Ecommerce CMS Roadmap

## Current Baseline
- Content management is currently limited to `StorefrontSettings` JSON (`models/storefront.go`) with draft/publish fields (`config_json`, `draft_config_json`).
- Public content read surface is `GET /api/v1/storefront`; admin writes are `GET/PUT /api/v1/admin/storefront`, `POST /api/v1/admin/storefront/publish`, and `DELETE /api/v1/admin/storefront/draft`.
- Homepage content is section-based (`hero`, `products`, `promo_cards`, `badges`) with fixed schema and validation in `handlers/storefront.go`.
- There is no first-class page model, URL routing table, navigation model, redirect manager, reusable blocks, or approvals workflow.
- SEO is limited to product/category plans; there is no CMS-wide SEO metadata model for landing pages, blog-like content, or campaign pages.
- Scheduling for content publication is not available; publication is immediate-only.
- Personalization, targeting, and experimentation do not exist in the storefront content layer.

## Goals
- Deliver a full ecommerce CMS that supports high-frequency merchandising and marketing without code deploys.
- Introduce first-class content entities (pages, sections/blocks, templates, navigation, redirects) with robust draft/version/publish workflows.
- Provide commerce-aware content primitives (product rails, category highlights, promotion banners, trust/social proof blocks).
- Support campaign scheduling, audience targeting, and A/B experiments with deterministic eligibility and auditability.
- Provide strong SEO controls (metadata, canonical/robots, sitemaps, redirects) for content and campaign pages.
- Support localization and market-aware content variants for multi-region ecommerce operations.
- Keep CMS architecture OpenAPI-first and aligned with existing generated handlers and frontend generated types.
- Maintain safe publication operations with rollback, preview, and operational observability.

## Non-Goals
- Replacing backend transactional commerce domains (catalog, checkout, payment lifecycle, discount engine).
- Building a full no-code email/SMS marketing automation suite in this roadmap.
- Building a complete DAM/PIM replacement in the first iteration (reuse current media pipeline; extend metadata only as needed).
- Solving marketplace multi-tenant content ownership across unrelated stores in the first rollout.

## Delivery Order
1. P0: CMS domain foundation and versioned content model.
2. P1: Pages, routing, reusable blocks, and storefront rendering integration.
3. P2: Navigation, layout regions, and global content assets.
4. P3: Commerce-aware CMS components and campaign landing workflows.
5. P4: Scheduling, targeting, and experimentation.
6. P5: SEO, redirects, and discovery surfaces.
7. P6: Localization, market variants, governance, and operational hardening.

## Cross-Roadmap Alignment
- Checkout baseline:
  - CMS CTAs and links for cart/checkout flows must target the session checkout UX and `/api/v1/checkout/*` routes, not authenticated-only `/api/v1/me/*` mutation assumptions.
  - Guest checkout disable state must be readable by CMS rendering logic so checkout-related blocks can degrade safely.
- Product catalog depth baseline:
  - CMS product blocks consume the implemented variant-first catalog contracts and `price_range`/variant-aware summaries.
  - Product pickers in CMS authoring should resolve to product IDs while storefront purchase actions remain variant-aware at runtime.
- Product categories (`roadmap/product-categories.md`):
  - Category landing blocks and navigation use canonical category slugs from `categories` and product-category joins.
- Discounts/promotions (`roadmap/discounts-promotions.md`):
  - Promotion banners and campaign pages consume active campaign metadata and scheduled windows from promotion domain, not duplicated CMS discount logic.
- Provider platform baseline:
  - CMS confirmation/help pages can reference payment/shipping statuses but must not bypass the implemented provider lifecycle APIs.
  - Checkout and payment totals pipeline remains catalog -> promotions -> checkout snapshot/provider execution; CMS is presentation/orchestration only.

## P0: CMS Foundation and Versioned Content Model
### Scope
- Introduce normalized CMS entities and versioning model.
- Define ownership and lifecycle states for content (`DRAFT`, `SCHEDULED`, `PUBLISHED`, `ARCHIVED`).
- Provide migration path from existing singleton storefront JSON.

### Deliverables
- New models/tables for:
  - content entries (page/layout/global/nav types),
  - content versions,
  - publication records,
  - content references to media and commerce entities.
- Migration entries in `internal/migrations` for new CMS models/tables and any backfills.
- Service layer in `internal/services/cms/` for:
  - draft creation/update,
  - version snapshots,
  - publish/rollback orchestration,
  - consistency validation (broken references, duplicate slugs, invalid schemas).
- Breaking contract cut from storefront singleton payload to CMS content APIs.

### Done Criteria
- New CMS entries can be created and updated without mutating published storefront state.
- Publish operation creates immutable version snapshot and updates read model atomically.
- Rollback to a previous published version is supported in one operation.
- Legacy `/api/v1/storefront` output is removed once CMS content APIs ship.

## P1: Pages, Routing, Reusable Blocks, and Rendering
### Scope
- Add first-class page and path routing model for ecommerce content.
- Introduce reusable block library for composable page building.
- Integrate rendering into storefront loaders and draft preview.

### Deliverables
- CMS page model with:
  - `slug`, `path`, `title`, `status`, `template_key`, `seo_id`, `visibility`.
- Reusable block schemas:
  - hero, rich text, image/gallery, video embed, FAQ/accordion, CTA, promo banner, custom HTML (sanitized).
- `GET /api/v1/content/{path}` public resolver with draft preview support when preview cookie is active.
- Admin CRUD endpoints for pages and blocks under `/api/v1/admin/cms/pages*`.
- Frontend route integration:
  - catch-all page renderer route (for example `frontend/src/routes/[...path]/+page.server.ts`) with explicit precedence rules vs product/category/core routes.

### Done Criteria
- Admin can create and publish standalone pages (for example `/about`, `/shipping`, `/returns`) without code changes.
- Draft preview shows unpublished page content while public route serves last published version.
- Duplicate path collisions are blocked with deterministic error responses.
- Block schema validation rejects invalid payloads (missing required fields, invalid links/media references).

## P2: Navigation, Layout Regions, and Global Content
### Scope
- Replace hard-coded nav/footer assumptions with managed global content entities.
- Add support for menu trees, region slots, and reusable site-wide fragments.

### Deliverables
- Models and endpoints for:
  - navigation menus and nested menu items,
  - global regions (`header`, `footer`, `announcement_bar`, `trust_strip`, `sitewide_banner`),
  - reusable snippets/fragments for repeated blocks.
- Admin APIs under `/api/v1/admin/cms/navigation*` and `/api/v1/admin/cms/global*`.
- Frontend layout integration in `frontend/src/routes/+layout.server.ts` and shared components to hydrate managed menus/regions.
- Policy validation:
  - max menu depth,
  - external link allowlist policy,
  - broken internal-link detection.

### Done Criteria
- Non-developer admin can reorder menus and publish navigation changes safely.
- Header/footer/global banners are fully CMS-driven and draft-previewable.
- Deleted or unpublished target pages are surfaced in admin validation and blocked from publish unless policy allows.
- Existing storefront homepage sections can be represented as CMS-managed regions.

## P3: Commerce-Aware Components and Campaign Landing Workflows
### Scope
- Add ecommerce-specific blocks needed by merchants for merchandising and conversion.
- Provide campaign page tooling tied to catalog/categories/promotions.

### Deliverables
- Commerce block set:
  - product rail (manual/search/category source),
  - featured category tiles,
  - promotion highlights,
  - urgency/inventory messaging blocks,
  - testimonial/review summary blocks,
  - UGC/social embed blocks (allowlisted providers).
- Campaign page templates:
  - seasonal sale landing,
  - collection launch,
  - bundle spotlight,
  - new arrivals.
- Block targeting hooks to consume promotion and category APIs without duplicating business logic.
- Admin preview endpoint to evaluate commerce block output against a sample context.

### Done Criteria
- Merchants can build and publish campaign pages that pull live catalog/category/promotion data.
- Product/category blocks respect publish status and never surface unpublished catalog entities.
- Invalid references (deleted category/product/promotion) are blocked or auto-degraded per explicit policy.
- Rendered output is deterministic for a fixed content version and context snapshot.

## P4: Scheduling, Targeting, and Experimentation
### Scope
- Introduce timed publication, audience targeting, and controlled experiments.
- Provide deterministic eligibility and exposure logging.

### Deliverables
- Scheduling model for:
  - delayed publish (`publish_at`),
  - auto-expire (`unpublish_at`),
  - recurring campaign windows (optional after one-time schedules).
- Targeting rules:
  - market/country,
  - device class,
  - authenticated vs guest,
  - referral/UTM conditions,
  - customer segment key hooks.
- Experiment framework:
  - experiment entity,
  - variants with traffic split,
  - sticky assignment key,
  - impression/conversion event hooks.
- Worker lifecycle wiring (in-process background worker) for schedule activation/deactivation and experiment cleanup.

### Done Criteria
- Scheduled publish/unpublish transitions execute idempotently and are visible in admin history.
- Targeting decisions are consistent for same request context and content version.
- Experiment allocations honor configured traffic splits within tolerance.
- Exposure logs include `content_version_id`, `experiment_id` (if any), and correlation ID.

## P5: SEO, Redirects, and Discovery Surfaces
### Scope
- Add CMS-grade SEO controls and URL lifecycle management.
- Provide sitemap and metadata feeds for search engines and channel integrations.

### Deliverables
- SEO metadata model for pages and global templates:
  - title/meta description,
  - canonical URL,
  - robots directives,
  - Open Graph/Twitter metadata,
  - optional JSON-LD blocks from allowlisted schema types.
- Redirect manager:
  - 301/302 mappings,
  - wildcard/path-prefix rules with priority,
  - loop detection.
- Generated feeds:
  - XML sitemap (content + products + categories where configured),
  - optional RSS/news feed for editorial content types.
- Validation tooling for:
  - missing SEO fields,
  - duplicate canonical targets,
  - broken redirect targets.

### Done Criteria
- Merchants can manage SEO metadata per page without code changes.
- Redirect rules are applied before CMS route resolution and are loop-safe.
- Sitemap generation includes only published, indexable URLs.
- Preview routes are marked non-indexable (`X-Robots-Tag: noindex`) consistently.

## P6: Localization, Market Variants, Governance, and Hardening
### Scope
- Add multi-locale and market-specific CMS variants.
- Implement editorial governance and operational safeguards.

### Deliverables
- Localization model:
  - locale-specific content variants per page/entry,
  - fallback chains,
  - locale-aware slugs and hreflang metadata.
- Market overrides:
  - per-market banners/legal copy/shipping info,
  - currency/region messaging blocks.
- Governance features:
  - roles/permissions for author, editor, publisher,
  - approval workflow,
  - change request comments,
  - audit trail for content mutations and publish actions.
- Hardening:
  - publish queue concurrency controls,
  - cache invalidation strategy + webhook hooks,
  - backup/export of CMS content payloads.

### Done Criteria
- Storefront serves locale/market-appropriate published content with deterministic fallback.
- Publish permissions enforce role boundaries and approval requirements.
- Every published change is traceable to actor, content version, and timestamp.
- Recovery path exists for accidental publish (rollback + cache purge + audit evidence).

## Data Model Changes
1. `cms_entries`
- Columns: `id`, `entry_type` (`page|layout|global|navigation|template`), `key`, `status`, `current_version_id`, `published_version_id`, timestamps + soft delete.

2. `cms_entry_versions`
- Columns: `id`, `entry_id`, `version_number`, `schema_version`, `payload_json`, `created_by`, `change_summary`, `created_at`.

3. `cms_publications`
- Columns: `id`, `entry_id`, `version_id`, `published_by`, `published_at`, `rollback_from_publication_id`, `notes`.

4. `cms_pages`
- Columns: `id`, `entry_id`, `path`, `slug`, `title`, `template_key`, `visibility`, `seo_metadata_id`, `is_homepage`.

5. `cms_navigation_menus`
- Columns: `id`, `entry_id`, `key`, `title`, `location`, timestamps.

6. `cms_navigation_items`
- Columns: `id`, `menu_id`, `parent_id`, `label`, `item_type` (`internal|external|category|product|page`), `target_ref`, `url`, `sort_order`, `is_enabled`.

7. `cms_redirect_rules`
- Columns: `id`, `source_pattern`, `match_type` (`exact|prefix|regex`), `target_url`, `redirect_type` (`301|302`), `priority`, `is_enabled`.

8. `cms_schedules`
- Columns: `id`, `entry_id`, `version_id`, `publish_at`, `unpublish_at`, `recurrence_rule`, `timezone`, `status`.

9. `cms_targeting_rules`
- Columns: `id`, `entry_id`, `version_id`, `rule_json`, `priority`, `is_enabled`.

10. `cms_experiments` and `cms_experiment_variants`
- Columns for experiment metadata, allocation, activation windows, and variant payload references.

11. `seo_metadata`
- Extend existing planned SEO model to support CMS entity types (`page`, `global_fragment`, `campaign`) and locale variants.

## Endpoint/API Plan
1. Public content APIs
- `GET /api/v1/content/{path}`: resolve published page by path.
- `GET /api/v1/content/navigation/{location}`: fetch published menu tree for header/footer contexts.
- `GET /api/v1/content/sitemap.xml`: generated sitemap (XML response).

2. Admin CMS APIs
- `GET /api/v1/admin/cms/pages`
- `POST /api/v1/admin/cms/pages`
- `GET /api/v1/admin/cms/pages/{id}`
- `PATCH /api/v1/admin/cms/pages/{id}`
- `POST /api/v1/admin/cms/pages/{id}/publish`
- `POST /api/v1/admin/cms/pages/{id}/rollback`
- `DELETE /api/v1/admin/cms/pages/{id}/draft`
- Equivalent CRUD/publish endpoints for navigation, global regions, redirects, schedules, and experiments.

3. Preview and validation APIs
- `POST /api/v1/admin/cms/preview/resolve`: resolve draft payload with optional request context.
- `POST /api/v1/admin/cms/validate`: run schema/reference/SEO/link checks before publish.

4. Legacy endpoint removal
- Remove `GET /api/v1/storefront` and legacy admin storefront endpoints in the same contract cut as CMS route adoption.
- Update frontend loaders to consume CMS APIs directly (no long-lived adapter layer).

## Execution Workflow in This Repo
1. Update `api/openapi.yaml` first for CMS endpoints and schemas.
2. Run `make openapi-gen` and commit generated files:
- `internal/apicontract/openapi.gen.go`
- `frontend/src/lib/api/generated/openapi.ts`
3. Add models under `models/` and wire schema/data changes via `internal/migrations`.
4. Implement thin handlers in `handlers/` and move CMS business logic into `internal/services/cms/`.
5. Wire public content resolution into storefront/frontend loaders (`frontend/src/routes/+layout.server.ts` and dynamic page route).
6. Add/extend worker startup wiring for scheduled publish jobs and experiment maintenance.
7. Run `make openapi-check`.
8. Run backend tests for touched areas with sandbox cache:
- `GOCACHE=/tmp/go-build go test ./handlers/...`
- `GOCACHE=/tmp/go-build go test ./internal/services/...`
9. Run frontend checks/tests for touched areas:
- `cd frontend && bun run check`
- `cd frontend && bun run lint`
- `cd frontend && bun run test:e2e` (for routing/publish/preview flows).
10. Keep docs updated in `wiki/` when available; if wiki repo is absent locally, capture notes in PR description and ask maintainer to mirror.

## Risk Register
- Publish race conditions can expose mixed-version content; mitigate with transactional publication and cache invalidation ordering.
- Route precedence conflicts (CMS page paths vs product/category/system routes) can cause regressions; require explicit resolver precedence tests.
- Rich text/custom HTML can introduce XSS vectors; enforce sanitization and CSP-compatible rendering.
- Targeting/experiments can create nondeterministic behavior; require deterministic rule ordering and sticky assignment strategy.
- Localization rollout can fragment SEO/canonical mappings; require hreflang and canonical validation at publish time.
- Redirect misconfiguration can cause loops or checkout detours; enforce static analysis before enabling rules.
- Admin UX complexity can reduce adoption; phase editor ergonomics and provide validation-first authoring.

## Immediate Next Slice
1. Define P0/P1 OpenAPI schemas for:
- `CmsEntry`, `CmsEntryVersion`, `CmsPage`, and publish/rollback payloads.
- `GET /api/v1/content/{path}` and initial `/api/v1/admin/cms/pages*` endpoints.
2. Implement minimal CMS page model (`cms_entries`, `cms_entry_versions`, `cms_pages`) and migration wiring.
3. Build `internal/services/cms/page_service.go` with draft create/update/publish/rollback.
4. Remove legacy storefront endpoint wiring from handlers/frontend once CMS homepage route is active.
5. Add handler/service tests for:
- valid page create/publish,
- duplicate path rejection,
- preview vs published resolution,
- rollback correctness.
6. Add frontend read path for one CMS-managed static page route while keeping existing homepage route unchanged.
