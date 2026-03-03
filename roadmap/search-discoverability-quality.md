# Search and Discoverability Quality Roadmap

## Current Baseline
- Public product discovery currently uses `GET /api/v1/products` with only `q`, `min_price`, `max_price`, `sort`, `order`, `page`, and `limit` in `api/openapi.yaml`.
- Search logic in `internal/repositories/catalog/products.go` is `name ILIKE '%term%'` plus basic price filters and simple sort normalization.
- There is no faceting/aggregations API, no synonym dictionary, no typo tolerance, no query rewrite pipeline, and no ranking strategy beyond sortable fields.
- Search results are returned from the primary DB query path; there is no dedicated index pipeline, no denormalized search document, and no search freshness/lag visibility.
- Storefront search page (`frontend/src/routes/search/+page.svelte`) supports keyword, sort, and pagination only; no facet UI, suggestions, or “did you mean”.
- There is no admin surface for merchandising search results (pin, bury, boost, hide, campaign rules) or for tuning ranking.

## Goals
- Deliver real indexed search with category/brand/attribute/price/availability faceting and stable pagination.
- Add query understanding: typo tolerance, synonyms, normalization, and rewrite controls that improve recall without tanking precision.
- Add ranking controls: configurable scoring weights, business signals (availability, margin, conversion), and deterministic tie-break behavior.
- Add merchandising controls beyond basic filters: query rules for pin/bury/hide/boost, campaign windows, and rule precedence.
- Add query suggestions/autocomplete and empty-result recovery paths.
- Add observability and relevance quality loop: query analytics, click/order attribution, zero-result tracking, and offline evaluation datasets.

## Non-Goals
- Building a full AI conversational shopper in this roadmap.
- Personalized ranking per-user in initial rollout (phase later after baseline relevance quality is stable).
- Multi-lingual stemming/transliteration in first release.
- Replacing catalog ownership/source-of-truth; search index remains a projection of catalog/order analytics.

## Delivery Order
1. P0: Search foundation, contracts, and index projection pipeline.
2. P1: Faceted retrieval and query understanding.
3. P2: Ranking controls and relevance tuning framework.
4. P3: Merchandising rule engine and admin tooling.
5. P4: Discoverability UX, analytics feedback loop, and hardening.

## Cross-Roadmap Alignment
- Product catalog depth:
- Use variant-first catalog entities (`product_variant_id`, attributes, brands) as canonical inputs to search documents.
- Product categories:
- Category hierarchy from `roadmap/product-categories.md` is the canonical taxonomy for category facets and breadcrumbs.
- Discounts/promotions:
- Promotion/campaign metadata can be ranking signals and rule predicates, but pricing truth stays in totals pipeline.
- Merchant analytics/reporting:
- Query -> click -> add-to-cart -> order attribution should feed analytics facts and relevance dashboards.

## P0: Search Foundation and Index Projection
### Scope
- Define search service boundary and index document model.
- Add index lifecycle jobs (backfill + incremental updates).
- Add OpenAPI search endpoints separate from basic product listing.

### Deliverables
- New internal search module:
- `internal/search` for query parsing/rewrite, retrieval, ranking orchestration, and rule evaluation.
- Search backend abstraction:
- Provider interface (for example `SearchBackend`) to support engine-backed indexing/retrieval while keeping application logic in repo.
- Search document projection:
- Product + variant + category + brand + attribute + inventory fields flattened into a searchable document shape.
- Indexing jobs/workers:
- Full reindex command and incremental sync on publish/product change.
- Lag/freshness metadata endpoint for admin operations.
- API additions in `api/openapi.yaml`:
- `GET /api/v1/search/products` (results + facets + metadata).
- `GET /api/v1/search/suggestions` (autocomplete, popular queries, correction hints).
- `GET /api/v1/admin/search/freshness` (index lag status).

### Done Criteria
- Reindex can build an index from current catalog without manual data fixes.
- Incremental updates propagate product publish/unpublish changes to index within defined freshness target.
- Search and product listing paths are decoupled (search endpoint no longer piggybacks on list-products SQL flow).
- Generated OpenAPI artifacts compile and handler stubs are wired.

## P1: Facets and Query Understanding
### Scope
- Implement faceted filtering and aggregations.
- Add typo handling and synonyms with controlled rewrite behavior.
- Add structured query validation and deterministic fallback behavior.

### Deliverables
- Facets:
- Category, brand, price-range buckets, stock availability, and top filterable attributes.
- Facet response contract includes selected filters, counts, and disabled states.
- Query normalization pipeline:
- Lowercasing, punctuation normalization, token cleanup, optional stop-word handling.
- Typo tolerance:
- Configurable edit-distance rules with keyword length thresholds and strict mode toggles.
- Synonym support:
- Unidirectional and bidirectional synonym sets managed by admin APIs.
- Empty-result fallback:
- “Did you mean” suggestion, relaxed query fallback (with explicit metadata flag).
- Admin APIs:
- CRUD endpoints for synonym sets and typo tolerance profiles.

### Done Criteria
- Facet counts remain consistent with returned result set for same filter context.
- Synonym changes are reflected in search behavior after index/rule refresh without deploy.
- Typo tolerance improves recall in test corpus without unacceptable precision regression.
- Search response clearly indicates when rewrites/corrections were applied.

## P2: Ranking Controls and Relevance Tuning
### Scope
- Introduce explicit ranking formula and tunable weights.
- Support blending textual relevance with business signals.
- Add offline relevance test harness.

### Deliverables
- Ranking model:
- Weighted components for textual score, exact phrase matches, field boosts (`name`, `brand`, attributes), recency, stock status, conversion proxies.
- Ranking profiles:
- Named configs (for example `default`, `new_arrivals`, `high_margin`) selectable per endpoint/context.
- Tie-break policy:
- Deterministic secondary sorting to eliminate unstable pagination.
- Relevance evaluation tooling:
- Gold query dataset + judged expected top-N results in repo fixtures.
- Automated scorer to compare NDCG/precision metrics before and after changes.
- Admin APIs:
- Read/update ranking profile weights with audit metadata.

### Done Criteria
- Ranking changes can be made via config/API without touching query code.
- Offline evaluation job runs in CI and fails on defined relevance regression threshold.
- Pagination remains stable for repeated requests with same query + filters.
- Result explain metadata is available in admin/debug mode.

## P3: Merchandising Rules and Campaign Controls
### Scope
- Add rule engine for query/category-based merchandising controls.
- Implement precedence/compatibility between ranking and manual rules.
- Ship admin workflows for rule management.

### Deliverables
- Rule types:
- Pin (fixed positions), bury (demote), hide (exclude), boost (score multiplier), and conditional include.
- Rule predicates:
- Query match (exact/prefix/contains), category context, campaign window, channel.
- Precedence model:
- Hard rules (hide/pin) applied before scoring boosts; explicit conflict resolution.
- Scheduling:
- Start/end timestamps and enable/disable flags.
- Admin APIs/UI:
- CRUD for merchandising rules, preview endpoint to simulate query + rule outcome.
- Audit trail:
- Who changed rules, when, and before/after payload snapshots.

### Done Criteria
- Merchandising rule application is deterministic and test-covered for conflicts.
- Admin can preview rule impact for a query without publishing globally.
- Expired rules automatically stop affecting results.
- Rule changes are visible in search metadata for troubleshooting.

## P4: Discoverability UX, Analytics Loop, and Hardening
### Scope
- Improve storefront discoverability components.
- Build analytics loop from behavior to ranking/rule improvements.
- Harden performance, reliability, and incident response.

### Deliverables
- Storefront UX:
- Facet panel, suggestions/autocomplete, spelling suggestions, popular/trending queries, zero-result recovery modules.
- Analytics capture:
- Query impression, click-through, add-to-cart, and order attribution events keyed by query/session.
- Search quality dashboards:
- Zero-result rate, CTR@k, conversion@k, facet usage, latency p95/p99, stale-index incidents.
- Operational controls:
- Reindex backpressure controls, circuit breakers, and fallback-to-basic-list behavior on search outage.
- Performance work:
- Index/schema tuning for high-cardinality facets and hot queries.
- Runbooks/docs:
- Incident handling for stale index, degraded latency, and bad-rule rollback.

### Done Criteria
- Search latency and availability meet defined SLOs under load test profile.
- Analytics dashboards expose enough signal to prioritize relevance iteration.
- Outage fallback keeps storefront functional (degraded but usable discovery).
- Runbooks are documented and tested in game-day simulation.

## Data Model Changes
1. `search_documents` (if local projection table is used for pipeline staging/debug)
- Fields: `entity_id`, `entity_type`, `payload_json`, `version`, `updated_at`, `indexed_at`.

2. `search_synonym_sets`
- Fields: `id`, `name`, `direction` (`bi`, `uni`), `terms_json`, `is_active`, `updated_by`, timestamps.

3. `search_ranking_profiles`
- Fields: `id`, `name`, `weights_json`, `is_default`, `updated_by`, timestamps.

4. `search_merchandising_rules`
- Fields: `id`, `name`, `rule_type`, `predicate_json`, `action_json`, `priority`, `starts_at`, `ends_at`, `is_active`, `updated_by`, timestamps.

5. `search_query_events`
- Fields: `id`, `query`, `normalized_query`, `filters_json`, `result_count`, `latency_ms`, `session_id`, `user_id`, `created_at`.

6. `search_click_events`
- Fields: `id`, `query_event_id`, `product_variant_id`, `position`, `clicked_at`.

## Endpoint/API Plan
1. Public search and suggestions
- `GET /api/v1/search/products`
- Query: `q`, `page`, `limit`, `sort_profile`, `filters[...]`, `facets[]`.
- Response: `items`, `facets`, `applied_rewrites`, `did_you_mean`, `pagination`, `debug` (admin only).
- `GET /api/v1/search/suggestions`
- Query: `q`, `limit`, `context`.
- Response: `suggestions`, `corrections`, `popular`.

2. Admin relevance controls
- `GET/POST/PATCH /api/v1/admin/search/synonyms`
- `GET/POST/PATCH /api/v1/admin/search/ranking-profiles`
- `GET/POST/PATCH /api/v1/admin/search/merchandising-rules`
- `POST /api/v1/admin/search/preview` (dry-run rules/ranking for a query)
- `POST /api/v1/admin/search/reindex` and `GET /api/v1/admin/search/freshness`

3. Contract notes
- Keep `GET /api/v1/products` as non-search catalog listing only (no `q`-search semantics).
- Remove `q`-search behavior from list-products in the same release that `/api/v1/search/products` is adopted.

## Execution Workflow in This Repo
1. Update `api/openapi.yaml` first for each phase that changes request/response shapes.
2. Run `make openapi-gen` and commit generated files:
- `internal/apicontract/openapi.gen.go`
- `frontend/src/lib/api/generated/openapi.ts`
3. Add migrations in `internal/migrations` for search config/event tables.
4. Implement backend services under `internal/search` and keep `handlers/` orchestration-only.
5. Integrate storefront/admin frontend changes using generated API types.
6. Run `make openapi-check`.
7. Run backend tests with sandbox-safe cache:
- `GOCACHE=/tmp/go-build go test ./...`
8. Run frontend checks/tests for touched areas:
- `cd frontend && bun run check && bun run lint`

## Risk Register
- Relevance regressions from aggressive synonym/typo settings can hurt precision.
- Mitigation: profile-based rollout, query-level allow/deny lists, offline regression gating.
- Merchandising overreach can hide genuinely relevant products and reduce conversion.
- Mitigation: rule precedence/audit trail, rule expiry defaults, preview tooling.
- Index staleness can create mismatch between PDP and search availability/pricing.
- Mitigation: freshness SLA, lag alerts, reindex repair tools, degraded fallback behavior.
- High-cardinality facets can increase latency and memory.
- Mitigation: facet caps, caching hot aggregations, schema tuning.
- Event telemetry noise/duplication can mislead tuning decisions.
- Mitigation: idempotency keys and dedupe in query/click ingestion.

## Immediate Next Slice
1. Add P0 OpenAPI endpoints for `GET /api/v1/search/products`, `GET /api/v1/search/suggestions`, and `GET /api/v1/admin/search/freshness`.
2. Scaffold `internal/search` with:
- Query normalizer.
- Backend interface.
- Result/facet response mappers.
3. Add migration for `search_synonym_sets`, `search_ranking_profiles`, and `search_merchandising_rules` tables.
4. Implement an initial index projection job from published products to a search document format.
5. Ship a full storefront switch from `/api/v1/products?q=` to `/api/v1/search/products?q=` in one contract cut.
