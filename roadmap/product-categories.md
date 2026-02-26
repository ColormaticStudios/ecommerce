# Product Categories Roadmap

## Current Baseline
- `models.Product` currently has no category fields or category relationship.
- Public product listing (`GET /api/v1/products`) supports `q`, `min_price`, `max_price`, `sort`, `order`, `page`, `limit` only.
- Admin product editing uses draft-aware flows (`draft_data`, publish/unpublish) and generated OpenAPI handlers.
- Storefront product sections support `source: manual|newest|search` and query/sort controls, but no category source/filter.
- Database schema is managed through GORM `AutoMigrate` in `main.go`.

## Goals
- Introduce first-class product categories with stable slugs and admin management.
- Allow products to belong to multiple categories.
- Add category filtering to public/admin product discovery with a clean contract.
- Integrate categories into product draft editing and publish workflows.
- Enable storefront merchandising by category.
- Prefer long-term contract clarity over backward compatibility (breaking changes are acceptable pre-production).

## Non-Goals
- Deep taxonomy rules (facets/attributes/brand trees) in initial rollout.
- Search-engine-grade relevance ranking by category.
- Customer-personalized category ordering.

## Delivery Order
1. P0: Category data model + admin CRUD.
2. P1: Product-to-category assignment + API contract updates.
3. P2: Public/admin filtering + storefront category sections.
4. P3: Hardening, migration quality, and operational guardrails.

## Cross-Roadmap Alignment
- Catalog depth:
  - Product write contract aligns with `roadmap/product-catalog-depth.md` nested `ProductUpsertInput`, not legacy flat `ProductInput`.
  - Categories attach at product level and apply to all product variants.
- Discounts/promotions:
  - `roadmap/discounts-promotions.md` category-targeted campaigns reuse this `categories` + `product_categories` data.
- Guest checkout/providers:
  - Checkout and payment roadmaps consume product/category metadata read-side only; category writes remain admin-only.

## P0: Category Data Model and Admin CRUD
### Scope
- Add base category entity with naming, slug, lifecycle fields.
- Provide admin CRUD endpoints for categories.
- Include hierarchy support now to avoid early redesign.

### Deliverables
- New `models.Category` (`models.BaseModel`-based) with:
  - `name` (required)
  - `slug` (required, unique)
  - `description` (optional)
  - `is_active` (default `true`)
  - `sort_order` (default `0`)
  - `parent_id` (nullable)
  - `path` (materialized path, indexed)
  - `depth` (small int, indexed)
- Registration of `models.Category` in `main.go` `AutoMigrate`.
- OpenAPI/admin endpoints:
  - `GET /api/v1/admin/categories`
  - `POST /api/v1/admin/categories`
  - `PATCH /api/v1/admin/categories/{id}`
  - `DELETE /api/v1/admin/categories/{id}` (soft delete preferred).

### Done Criteria
- Admin can create/edit/disable categories with unique slug enforcement.
- Invalid category payloads (blank name/slug, duplicate slug) are rejected.
- Category list endpoint returns stable ordering (`sort_order`, then `id`).
- Hierarchy writes validate acyclic tree constraints and max depth policy.

## P1: Product Assignment and Draft-Aware Contract
### Scope
- Connect products and categories through many-to-many relation.
- Replace product contracts cleanly where needed (breaking changes allowed).
- Ensure draft editing and publish flows carry category assignments correctly.

### Deliverables
- `models.Product` relation: `Categories []Category` with join table (for example `product_categories`).
- OpenAPI schema updates (breaking allowed):
  - `Product` includes required `categories` (array).
  - `ProductUpsertInput` includes required `category_ids` (array; empty allowed only for uncategorized policy if enabled).
- Admin product write paths (`createProduct`, `updateProduct`, publish/discard draft) persist category assignments.
- Product response mappers in `handlers/products.go` include categories for public/admin payloads.

### Done Criteria
- Updating product draft category assignments does not leak to live product until publish.
- Publishing a draft updates product-category join rows atomically with other product changes.
- Contract consumers compile against regenerated types without compatibility shims.

## P2: Filtering and Storefront Merchandising
### Scope
- Add category filters to existing product listing surfaces.
- Add category-driven storefront section source for merchandising.
- Keep query semantics deterministic and simple.

### Deliverables
- Public API filter additions:
  - Extend `GET /api/v1/products` with canonical `category_slug` (single or repeated query values).
- Admin API filter additions:
  - Extend `GET /api/v1/admin/products` with `category_slug`, `category_id`, and `include_inactive_categories`.
- Optional discovery endpoint for storefront/search UI:
  - `GET /api/v1/categories` (active categories only, tree payload).
- Storefront section model enhancement:
  - Extend `StorefrontProductSection.source` with `category`.
  - Add `category_slug` field for section config.
- Storefront loaders (`frontend/src/routes/+page.server.ts`) support category source mode.

### Done Criteria
- Category-filtered product queries return only products linked to selected category.
- Combined filters (`q` + `category_slug` + price range) behave consistently with pagination/sorting.
- Storefront "category" sections render expected products and respect `limit`, `sort`, `order`.

## P3: Hardening and Quality
### Scope
- Address integrity, performance, and regression risks.
- Add migration/backfill and safety checks for rollout confidence.

### Deliverables
- Data integrity rules:
  - Prevent assigning inactive/deleted categories to published products.
  - Prevent deleting categories still referenced by published products (or auto-detach by explicit policy).
- Query/index tuning:
  - Indexes on `categories.slug`, `categories.is_active`, and join table composite keys.
- Test coverage:
  - Unit tests for validation and slug normalization.
  - Integration tests for admin CRUD, draft publish semantics, filter combinations.
  - Regression tests for existing listing flows without category params.

### Done Criteria
- Category endpoints and product filters pass valid/invalid test cases.
- Draft/publish category behavior is covered by automated tests.
- No regression in existing product listing behavior when no category filters are used.

## Data Model Changes
1. `categories`
- Columns: `id`, `name`, `slug`, `description`, `is_active`, `sort_order`, `parent_id`, `path`, `depth`, timestamps + soft delete.

2. `product_categories` (join table)
- Columns: `product_id`, `category_id`, timestamps (optional), unique composite key (`product_id`, `category_id`).

3. `products` (relation only)
- Add GORM many-to-many relation to `categories`; no direct FK required for multi-category support.

## Endpoint/API Plan
1. New admin endpoints
- `GET /api/v1/admin/categories`
- `POST /api/v1/admin/categories`
- `PATCH /api/v1/admin/categories/{id}`
- `DELETE /api/v1/admin/categories/{id}`

2. New public endpoint
- `GET /api/v1/categories`

3. Breaking changes to existing endpoints
- `GET /api/v1/products`: standardize category filter to `category_slug` (drop ambiguous `category` form).
- `GET /api/v1/admin/products`: support `category_slug`, `category_id`, `include_inactive_categories`.
- `Product` schema: make `categories` required and always populated.
- `ProductUpsertInput` schema: make `category_ids` explicit contract field.
- Remove compatibility fallbacks; update frontend/backend in one coordinated cut.

## Execution Workflow in This Repo
1. Update `api/openapi.yaml` first for category schemas/params/endpoints.
2. Run `make openapi-gen` to regenerate:
- `internal/apicontract/openapi.gen.go`
- `frontend/src/lib/api/generated/openapi.ts`
3. Implement model + handler/service updates (keep handlers thin, move reusable logic to `internal/` helpers/services).
4. Register new models in `main.go` `AutoMigrate`.
5. Run `make openapi-check`.
6. Run backend/frontend tests for touched paths.

## Risk Register
- Category hierarchy complexity can grow quickly; enforce max depth and cycle checks.
- Draft/live divergence can cause inconsistent category visibility if publish flow is not atomic.
- Join-table filtering can become slow without indexes as catalog grows.
- Slug changes can break links; define either immutable slug policy or explicit redirect alias table before launch.

## Immediate Next Slice
1. Add `Category` model + AutoMigrate registration and join table relation on `Product`.
2. Add admin category CRUD endpoints in OpenAPI + generated handlers.
3. Extend nested product upsert/read contracts with `category_ids`/`categories` and wire draft-aware persistence.
4. Add initial integration tests for category CRUD and product assignment publish behavior.
