# Product Catalog Depth Roadmap

## Current Baseline
- `models.Product` is a flat model (`sku`, `name`, `description`, `price`, `stock`, `images`, `related`) with no first-class variants/options/attributes/brand/SEO fields.
- Cart and order flows are product-based (`models.CartItem.ProductID`, `models.OrderItem.ProductID`) and stock checks are done against `products.stock` in `handlers/cart.go` and `handlers/orders.go`.
- Product drafts are stored as JSON in `products.draft_data` via `handlers/product_drafts.go` and published by copying draft values back to product columns in `handlers/admin.go`.
- Product API contract (`api/openapi.yaml`) currently exposes a shallow `Product`/`ProductInput` schema and storefront/admin loaders consume that shape directly (`frontend/src/lib/models.ts`, `frontend/src/lib/admin/ProductEditor.svelte`).
- This project is pre-production, so breaking API/schema changes are acceptable.

## Goals
- Move to a variant-first catalog model suitable for real ecommerce complexity.
- Support reusable option definitions and per-product option value sets.
- Support normalized product attributes for filtering/faceting and merchandising.
- Introduce first-class brands with storefront/admin integration.
- Add structured SEO fields for products (and shared pattern support for category/brand later).
- Replace brittle draft JSON for deep catalog edits with a draft architecture that handles nested entities safely.
- Keep implementation aligned with OpenAPI-first and generated contract flow used in this repo.

## Non-Goals
- Multi-language localization in this first catalog-depth rollout.
- Marketplace/vendor multi-tenant catalog ownership.
- Search-engine implementation details (Algolia/Meilisearch/Elasticsearch) in first pass.

## Delivery Order
1. P0: Catalog data model redesign and product draft architecture replacement.
2. P1: Variant/option APIs and admin editor rewrite.
3. P2: Variant-aware cart/order/checkout migration (breaking).
4. P3: Attributes, brands, SEO, and storefront/search integration.
5. P4: Hardening, indexing, and operational quality.

## Cross-Roadmap Alignment
- Product categories:
  - `roadmap/product-categories.md` category model/join table is the canonical category source for product-level categorization.
- Guest checkout:
  - Checkout/cart/order mutation routes are `/api/v1/checkout/*` from `roadmap/guest-checkout.md`; this roadmap supplies the `product_variant_id` contract those routes consume.
- Discounts/promotions:
  - Discount evaluation applies to variant-backed line items while campaign targets remain product/category/brand entities.
- Providers:
  - Provider snapshots and payment lifecycle operate on totals produced from variant pricing plus discount adjustments.

## P0: Catalog Model Redesign and Draft Architecture
### Scope
- Introduce normalized catalog entities for products, options, variants, attributes, brands, and SEO metadata.
- Replace single `products.draft_data` JSON blob with explicit draft tables for complex nested editing.
- Keep existing product publishing semantics (draft edits isolated until publish) but make them work for nested entities.

### Deliverables
- New core tables/models:
  - `brands`
  - `product_options` (definition level: `name`, `position`, `display_type`)
  - `product_option_values` (per-option value set)
  - `product_variants` (SKU, price, stock, compare_at_price, weight/dimensions, published flag)
  - `product_variant_option_values` (join mapping variant -> option values)
  - `product_attributes` (definition: key/slug/type/filterable/sortable)
  - `product_attribute_values` (per-product values; typed storage)
  - `seo_metadata` (entity_type + entity_id keyed metadata)
- Draft architecture upgrade:
  - `product_drafts` (draft header/versioning)
  - `product_variant_drafts`, `product_option_drafts`, `product_attribute_value_drafts`, `product_related_drafts`.
- `main.go` `AutoMigrate` updated to register all new catalog models.
- Legacy `products.draft_data` marked for removal in final phase of rollout.

### Done Criteria
- Admin draft edits can create/update/delete variants/options/attribute values without mutating live catalog data.
- Publishing applies nested changes atomically in one transaction.
- Discard draft fully restores live state for nested entities.
- Draft model supports deterministic ordering (`position`) for options, values, and variants.

## P1: Variant and Option Contract + Admin Editing
### Scope
- Replace flat product contract with variant-first contract.
- Update admin product editing workflows to manage options and variant matrix.
- Support per-variant inventory/pricing while preserving product-level merchandising fields.

### Deliverables
- Breaking API contract updates in `api/openapi.yaml`:
  - `Product` includes:
    - `brand`
    - `options[]`
    - `variants[]`
    - `attributes[]`
    - `seo`
    - `default_variant_id`
    - `price_range` (derived min/max variant prices)
  - `ProductInput` replaced with explicit nested payload (`ProductUpsertInput`) for base metadata + options + variants + attributes + SEO.
- Admin endpoints updated to support draft writes of nested entities:
  - Keep existing product endpoint paths where practical, but change payloads to canonical nested shape.
  - Add focused endpoints for large operations if needed (for example variant bulk upsert).
- Frontend admin editor rewrite (`frontend/src/lib/admin/ProductEditor.svelte`) to handle:
  - Option definitions and value lists.
  - Variant matrix generation/editing.
  - Per-variant SKU/price/stock.
  - Brand selection and SEO editing.

### Done Criteria
- Admin can create product with multiple options and generated variants in one draft flow.
- Invalid variant combinations and duplicate variant SKU values are rejected.
- Product API always returns normalized nested catalog payloads (no legacy flat fallback).
- Draft preview renders the same variant data that would be published.

## P2: Variant-Aware Cart, Order, and Checkout Migration
### Scope
- Migrate transactional flows from `product_id` semantics to `variant_id` semantics.
- Move stock checks and price snapshots to variant level.
- Remove ambiguous product-level purchasing behavior.

### Deliverables
- Breaking model changes:
  - `models.CartItem`: replace `ProductID` with `ProductVariantID`.
  - `models.OrderItem`: replace `ProductID` with `ProductVariantID` and persist variant SKU/title snapshot.
- API updates:
  - Cart add/update payloads accept `product_variant_id`.
  - Order create/checkout payloads accept variant identifiers.
  - Cart/order responses include variant summary and parent product summary.
- Handler updates:
  - `handlers/cart.go` and `handlers/orders.go` load/check variants for stock and pricing.
  - Media/cover behavior remains parent-product based unless variant media is later enabled.
- Data migration strategy:
  - Backfill existing cart/order items to default variant where possible.
  - Hard-fail on orphaned product references instead of silent fallback.

### Done Criteria
- Cart and order creation work only with variant IDs.
- Stock decrement/increment logic uses variant stock only.
- Order item price snapshots are taken from variant price at commit time.
- Existing product-level purchase paths are removed from OpenAPI and handlers.

## P3: Attributes, Brands, SEO, and Discovery Integration
### Scope
- Add rich merchandising/search data to product catalog APIs.
- Integrate brands and attribute filtering in admin/storefront discovery.
- Add SEO metadata fields and slug policy for canonical URLs.

### Deliverables
- Brand management:
  - Admin brand CRUD (`name`, `slug`, `description`, `logo_media_id`, `is_active`).
  - Product-brand assignment in product draft editor and API.
- Attribute system:
  - Typed attribute definitions (`text`, `number`, `boolean`, `enum`) and filterable flag.
  - Product attribute value assignment in nested product payload.
- SEO fields:
  - Product SEO (`title`, `description`, `canonical_path`, `og_image_media_id`, `noindex`).
  - Enforced max lengths and uniqueness for canonical paths/slugs.
- Listing/search contract extensions:
  - `GET /api/v1/products` and `GET /api/v1/admin/products` add `brand_slug`, `attribute[...]`, and `has_variant_stock` filters.
  - Include `price_range`, `brand`, and selected attribute summary in listing responses.
- Storefront integration:
  - Product cards can show brand and variant-from-price data.
  - Search page and homepage product section queries can filter by brand/attributes.

### Done Criteria
- Brand and attribute filters return deterministic, paginated results.
- SEO metadata round-trips through admin edit -> publish -> public product read.
- Canonical URL collisions are rejected at write time.
- Storefront/product search behavior remains stable when no new filters are provided.

## P4: Hardening and Operational Quality
### Scope
- Ensure data integrity, performance, and regression safety for deeper catalog model.
- Finalize removal of legacy flat/draft fields.

### Deliverables
- Constraints and indexes:
  - Unique indexes for variant SKU and product+option/value uniqueness.
  - Composite indexes for common listing filters (`brand_id`, `is_published`, `price`/price-range helpers).
- Concurrency safeguards:
  - Publish operations use row locking/version checks on draft vs live rows.
  - Variant stock update race coverage in order payment path.
- Contract cleanup:
  - Remove legacy flat `ProductInput` and legacy draft blob usage.
  - Remove compatibility code paths in frontend parsers.
- Documentation and test expansion:
  - API docs refreshed through generated `API.md`.
  - Roadmaps cross-linked (`roadmap/product-categories.md`, `roadmap/discounts-promotions.md`).

### Done Criteria
- No remaining writes depend on `products.draft_data`.
- Variant and attribute queries meet agreed latency targets under expected catalog size.
- Integration tests cover all breaking paths (admin edit, publish/discard, cart/order checkout).
- OpenAPI-generated backend/frontend artifacts are in sync and `make openapi-check` passes.

## Data Model Changes
1. `products` (breaking reshape)
- Keep core merchandising fields (`name`, `description`, publish flags, timestamps).
- Remove product-level purchasable fields as source of truth (`price`, `stock`) after variant migration completion.
- Add `brand_id`, `default_variant_id`, optional `subtitle` and merchandising metadata fields.

2. `product_variants`
- Fields: `id`, `product_id`, `sku`, `title`, `price`, `compare_at_price`, `stock`, `position`, `is_published`, timestamps.

3. `product_options` + `product_option_values`
- Option definitions and values with ordering and uniqueness constraints per product.

4. `product_variant_option_values`
- Many-to-many mapping table to represent variant combinations.

5. `brands`
- Fields: `id`, `name`, `slug`, `description`, `logo_media_id`, `is_active`, timestamps.

6. `product_attributes` + `product_attribute_values`
- Typed catalog attributes and per-product values.

7. `seo_metadata`
- Generic SEO table keyed by (`entity_type`, `entity_id`) for product first, reusable for categories/brands later.

8. Draft tables (breaking replacement of JSON blob draft)
- `product_drafts` and nested draft child tables mirroring live catalog entities.

## Endpoint/API Plan
1. Breaking product contract updates
- Replace flat `ProductInput` with nested `ProductUpsertInput`.
- Update `Product` response shape to include `variants/options/attributes/brand/seo` and derived `price_range`.

2. Cart/order API breaking updates
- Replace `product_id` request fields with `product_variant_id`.
- Update response schemas to include variant identity and selected option values.

3. Brand and attribute APIs
- Add admin CRUD for brands and attribute definitions.
- Extend list/search endpoints with brand and attribute filters.

4. SEO API behavior
- SEO fields are managed as part of product upsert/read payloads (single-source contract for frontend).

## Execution Workflow in This Repo
1. Update `api/openapi.yaml` first for each phase boundary.
2. Run `make openapi-gen` to regenerate:
- `internal/apicontract/openapi.gen.go`
- `frontend/src/lib/api/generated/openapi.ts`
3. Implement backend model/handler/service changes, keeping handlers thin.
4. Register model changes in `main.go` `AutoMigrate` while migrations are still GORM-managed.
5. Update frontend models/parsers/admin UI for new generated types.
6. Run `make openapi-check`.
7. Run backend tests with `GOCACHE=/tmp/go-build` and frontend checks/tests for touched areas.

## Risk Register
- Draft/live divergence risk grows with nested entities unless publish/discard operations are strictly transactional.
- Variant migration touches cart/order/checkout core paths and can cause regressions without strong integration coverage.
- Attribute flexibility can create slow queries without disciplined indexing and filter constraints.
- Canonical URL/slug collisions across product/category/brand entities can break storefront links if not centrally validated.

## Immediate Next Slice
1. Land P0 schema skeleton in models + `AutoMigrate` (no behavior switch yet).
2. Define and commit the new nested product OpenAPI schemas (`Product`, `ProductUpsertInput`, variant/option/brand/attribute/seo components).
3. Implement draft table persistence helpers and publish/discard transaction scaffolding.
4. Add integration tests proving nested draft isolation and atomic publish before touching cart/order flows.
