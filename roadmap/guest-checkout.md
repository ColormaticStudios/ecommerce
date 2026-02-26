# Guest Checkout Roadmap

## Current Baseline
- Checkout is fully authenticated today:
  - Cart endpoints are only under `/api/v1/me/cart*`.
  - Checkout quote is only under `/api/v1/me/checkout/quote`.
  - Order create/pay/cancel are only under `/api/v1/me/orders*`.
- `handlers/generated_api_server.go` enforces auth+CSRF for all `/me/*` routes via `runProtected`.
- Cart ownership is hard-bound to `models.Cart.UserID` (`uniqueIndex`), and cart lookup is `getOrCreateCart(db, user.ID)`.
- Order ownership is hard-bound to `models.Order.UserID`; user order queries always filter by `user_id`.
- Frontend cart and checkout pages (`frontend/src/routes/cart/+page.svelte`, `frontend/src/routes/checkout/+page.svelte`) currently block unauthenticated users.
- Saved payment methods/addresses are user profile features and are only available for authenticated accounts.

## Goals
- Enable full guest checkout from cart to order placement without requiring login.
- Make guest checkout configurable so admins can disable it without code changes.
- Keep authenticated checkout first-class, while sharing the same core checkout pipeline as guests.
- Replace user-bound cart ownership with checkout-session ownership that works for both guests and signed-in users.
- Support guest contact capture (email required) and shipping data capture in checkout.
- Allow optional post-purchase account linking/claiming for guest orders.
- Prefer clean contract and data model redesign over backward compatibility (breaking changes are acceptable pre-production).

## Non-Goals
- Implementing accountless order self-service portal in the first phase (beyond basic confirmation lookup token support).
- Building fraud/risk scoring infrastructure in this roadmap.
- Supporting multi-cart per browser profile in initial rollout.

## Delivery Order
1. P0: Core session model + guest-capable cart domain.
2. P1: Checkout/order API redesign with guest support.
3. P2: Frontend migration to session checkout UX.
4. P3: Guest order claim/link + hardening.

## Cross-Roadmap Alignment
- Catalog depth:
  - This roadmap uses `product_variant_id` as the cart/order line identifier (aligned with `roadmap/product-catalog-depth.md` P2).
  - No long-term `product_id` purchase fallback is planned.
- Providers:
  - `submit-payment` is a short-term endpoint that evolves into explicit payment lifecycle endpoints in `roadmap/providers.md`.
- Discounts/promotions:
  - `/api/v1/checkout/quote` must include discount/promotion adjustments from `roadmap/discounts-promotions.md`.

## P0: Checkout Session Core
### Scope
- Introduce checkout session as the ownership unit for cart and checkout state.
- Remove direct `Cart.UserID` coupling.
- Keep user association optional on session for authenticated customers.
- Add a central `guest checkout enabled` setting and enforcement path.

### Deliverables
- New `models.CheckoutSession` (or `models.ShopperSession`) with:
  - `id` (DB PK)
  - `public_token` (opaque token used by client cookie)
  - `user_id` (nullable FK to `users.id`)
  - `guest_email` (nullable; required later at order submit for guests)
  - `status` (`ACTIVE|CONVERTED|EXPIRED`)
  - `expires_at`, `last_seen_at`
- `models.Cart` changed from `user_id` to `checkout_session_id` (unique active cart per session).
- `main.go` `AutoMigrate` updated with new model and altered models.
- Session resolver helper in `handlers/` or `internal/checkout/`:
  - Resolves checkout session by HttpOnly cookie token.
  - Creates a new session+cart when absent.
  - If authenticated user exists, links session to that user.
- Storefront/admin config integration:
  - Add `checkout.allow_guest_checkout` to storefront config JSON (`models.StorefrontSettings`).
  - Read this setting in checkout-session middleware/service to gate guest access.
- CSRF handling preserved for mutating endpoints; auth no longer required for checkout-session routes.

### Done Criteria
- Unauthenticated request can create and retrieve a cart tied to checkout session cookie.
- Authenticated request uses the same session flow and can link to `user_id`.
- No cart endpoint relies on `userID` Gin context for ownership checks.
- Session token rotation/invalid-token handling is covered by tests.
- When `checkout.allow_guest_checkout=false`, unauthenticated checkout-session access is rejected consistently (for example `403` with machine-readable code) while authenticated checkout still works.

## P1: Guest-Capable Checkout and Order APIs (Breaking)
### Scope
- Replace `/me` checkout/cart/order contract with explicit checkout-session endpoints.
- Allow order creation for both guest and authenticated sessions.
- Keep saved payment/address features as authenticated-only optional accelerators.

### Deliverables
- OpenAPI breaking redesign:
  - New routes under `/api/v1/checkout/*`:
    - `GET /api/v1/checkout/cart`
    - `POST /api/v1/checkout/cart/items`
    - `PATCH /api/v1/checkout/cart/items/{itemId}`
    - `DELETE /api/v1/checkout/cart/items/{itemId}`
    - `GET /api/v1/checkout/plugins`
    - `POST /api/v1/checkout/quote`
    - `POST /api/v1/checkout/orders`
    - `POST /api/v1/checkout/orders/{id}/submit-payment`
  - Deprecate/remove `/api/v1/me/cart*`, `/api/v1/me/checkout/*`, `/api/v1/me/orders` create/pay usage in frontend.
  - Add explicit guest-checkout-disabled error response schema/code for checkout-session endpoints.
- `Order` schema update:
  - `user_id` becomes nullable.
  - Add `checkout_session_id` (required).
  - Add guest-facing contact fields (at minimum `guest_email`).
  - Add optional `confirmation_token` for guest order lookup.
- Handlers refactor:
  - Cart and quote use checkout session resolver, not authenticated user lookup.
  - Cart item mutations accept `product_variant_id` as canonical purchasable reference.
  - Order creation snapshots checkout session context.
  - Payment submission supports:
    - guest-provided payment/address data
    - authenticated saved method/address IDs when session has `user_id`
- Admin order query (`handlers/orders.go` admin list) updated to support null-user orders cleanly.

### Done Criteria
- Guest can complete: add cart item -> quote -> create order -> submit payment.
- Authenticated customer can complete same flow and still use saved profile payment/address IDs.
- Order creation and payment are blocked when guest email or required checkout data is missing.
- Existing plugin quote path works identically for guest and authenticated flows.
- Guest checkout-disable switch immediately blocks new guest checkout operations without breaking authenticated checkout.

## P2: Frontend Migration to Session Checkout
### Scope
- Remove login gate from cart and checkout pages.
- Switch frontend API client from `/me/*` checkout routes to `/checkout/*`.
- Keep account pages (`/orders`, saved addresses/payment methods) authenticated.

### Deliverables
- `frontend/src/lib/api.ts`:
  - Replace cart/checkout/order-create/order-pay route methods with new checkout-session routes.
  - Keep `credentials: include` behavior so checkout session cookie is sent.
- `frontend/src/routes/cart/+page.svelte`:
  - Remove unauthenticated blocking text and render cart for guests.
- `frontend/src/routes/checkout/+page.server.ts` and `+page.svelte`:
  - Load checkout cart/plugins for any visitor.
  - Show guest contact capture fields when user is not authenticated.
  - Keep saved methods/addresses section conditional on authenticated session.
  - Render clear CTA to login/register when guest checkout is disabled and visitor is unauthenticated.
- Generated frontend contract types regenerated from OpenAPI.

### Done Criteria
- Logged-out user can browse, build cart, and reach order submission successfully.
- Logged-in user experience remains functional and can still use saved profile checkout data.
- No frontend checkout page depends on `serverIsAuthenticated` to load cart/quote.
- When guest checkout is disabled, logged-out users are redirected/prompted to authenticate before checkout completion.

## P3: Guest Order Linking and Hardening
### Scope
- Support claiming guest orders after account creation/login.
- Add lifecycle/cleanup for stale checkout sessions.
- Add reliability and abuse protections around guest checkout.

### Deliverables
- Guest order claim flow:
  - Endpoint to claim guest orders by confirmation token + email.
  - On claim, set `orders.user_id` and mark claim metadata.
- Session lifecycle:
  - Expire and archive old checkout sessions via background cleanup job.
  - Define worker location (in-process goroutine in `main.go` until dedicated worker runtime exists).
- Abuse controls:
  - Rate limit order submission per session/IP.
  - Idempotency key for order creation/payment submission endpoints.
- Observability:
  - Metrics/log fields for guest vs authenticated checkout conversion and failures.

### Done Criteria
- Guest order can be linked to a newly created user account without duplicating order records.
- Expired sessions cannot mutate cart/order state.
- Duplicate payment submission requests are safely deduplicated.

## Data Model Changes
1. `checkout_sessions`
- `id`, `public_token` (unique), `user_id` (nullable), `guest_email` (nullable), `status`, `expires_at`, `last_seen_at`, timestamps, soft delete.

2. `carts`
- Replace `user_id` with `checkout_session_id` (unique active cart).

3. `orders`
- Make `user_id` nullable.
- Add `checkout_session_id` (required FK).
- Add `guest_email` (nullable for authenticated orders, required for guest orders).
- Add `confirmation_token` (nullable, unique when guest).

4. `order_items`
- No ownership change; remains linked to `orders`.
- Purchase reference aligns to variant model (`product_variant_id`) from `roadmap/product-catalog-depth.md`.

5. `storefront_settings` (`config_json` / `draft_config_json`)
- Add `checkout.allow_guest_checkout` boolean (default `true`).
- Publish flow controls when guest checkout setting becomes live.

## Endpoint/API Plan
1. Replace authenticated checkout routes with session routes
- Remove/retire:
  - `/api/v1/me/cart*`
  - `/api/v1/me/checkout/plugins`
  - `/api/v1/me/checkout/quote`
  - `/api/v1/me/orders` `POST`
  - `/api/v1/me/orders/{id}/pay`
- Add:
  - `/api/v1/checkout/cart`
  - `/api/v1/checkout/cart/items`
  - `/api/v1/checkout/cart/items/{itemId}`
  - `/api/v1/checkout/plugins`
  - `/api/v1/checkout/quote`
  - `/api/v1/checkout/orders`
  - `/api/v1/checkout/orders/{id}/submit-payment`
- Planned provider-lifecycle replacement (follow-up cut):
  - `/api/v1/checkout/orders/{id}/payments/authorize`
  - `/api/v1/admin/orders/{id}/payments/{intentId}/capture`
  - `/api/v1/admin/orders/{id}/payments/{intentId}/void`
  - `/api/v1/admin/orders/{id}/payments/{intentId}/refund`

2. Keep authenticated order history/account endpoints
- Keep `/api/v1/me/orders` `GET`, `/api/v1/me/orders/{id}`, `/api/v1/me/orders/{id}/cancel` as account features.
- Ensure they exclude unclaimed guest orders unless linked to user.

3. Optional guest confirmation endpoint
- Add `GET /api/v1/checkout/orders/{id}/confirmation?token=...` for short-term post-purchase status view.

4. Guest checkout config surface
- Reuse admin storefront settings endpoint contract to include `checkout.allow_guest_checkout`.
- Ensure checkout endpoints return a stable error code when guest mode is disabled.

## Execution Workflow in This Repo
1. Update `api/openapi.yaml` first for checkout-session endpoints and schema changes.
2. Run `make openapi-gen` and commit generated files:
- `internal/apicontract/openapi.gen.go`
- `frontend/src/lib/api/generated/openapi.ts`
3. Implement model changes and register new models/fields in `main.go` `AutoMigrate`.
4. Refactor handlers with thin route handlers + reusable checkout session services/helpers in `internal/`.
5. Update `handlers/generated_api_server.go` route methods to stop requiring auth for `/checkout/*` and keep CSRF for mutating methods.
6. Run `make openapi-check`.
7. Run backend tests (with sandbox cache): `GOCACHE=/tmp/go-build go test ./...`
8. Run frontend checks: `cd frontend && bun run check && bun run lint`.

## Risk Register
- Session hijacking risk if checkout session token handling is weak (must be HttpOnly, secure in prod, rotated when appropriate).
- Guest email typos can reduce supportability; require explicit email confirmation in checkout UX.
- Breaking route migration can leave stale frontend calls; complete OpenAPI-first cutover in one branch.
- Mixed guest/auth sessions may create duplicate carts without clear merge policy.
- Order query behavior can regress if admin and user views do not handle nullable `user_id` consistently.
- Misconfigured toggle defaults could unintentionally block checkout at launch; require explicit default and integration test coverage.

## Immediate Next Slice
1. Define `checkout_sessions` and cart ownership model in `models/` and `main.go` `AutoMigrate`.
2. Draft OpenAPI replacement for `/api/v1/checkout/cart*`, `/checkout/quote`, `/checkout/orders`, and updated `Order` schema with nullable `user_id`.
3. Implement checkout session resolver helper and migrate cart handlers to use it.
4. Add integration tests for guest cart lifecycle and guest order creation validation (including missing guest email invalid path).
