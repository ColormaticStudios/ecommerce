# User Data Encryption Roadmap (Saved Payment Details and Addresses)

## Current Baseline
- `models/saved_payment_methods` stores non-encrypted card metadata (`brand`, `last4`, `exp_month`, `exp_year`, `cardholder_name`).
- `models/saved_addresses` stores full address fields in plaintext (`line1`, `city`, `postal_code`, etc.).
- `POST /api/v1/me/payment-methods` currently accepts raw `card_number` in request payload, then persists derived metadata.
- There is no centralized envelope-encryption service, no key version tracking on encrypted records, and no cryptographic erasure workflow.
- The checkout/provider roadmap supports pluggable payment providers, but customer saved payment storage is not yet hardened against DB-only compromise.

## Goals
- Encrypt saved addresses and sensitive saved payment fields at the application layer before DB writes.
- Stay provider-agnostic: do not require any single external payment vault product.
- Introduce internal tokenization semantics so downstream code uses stable references, not raw PAN.
- Make key lifecycle first-class: key IDs on records, rotation, and re-encryption workflows.
- Prevent high-risk data retention: never persist CVV/CVC after authorization attempt.
- Keep queryability for operational use cases (default method/address, masked display, de-dup) without exposing plaintext.

## Non-Goals
- Replacing PCI scope obligations with encryption alone.
- Storing CVV/CVC for reuse (explicitly prohibited).
- Building a complete HSM product in this roadmap (we integrate with KMS/HSM interfaces).
- End-to-end provider payment lifecycle changes (covered by `roadmap/providers.md`).
- Full privacy workflow redesign (covered by `roadmap/legal-compliance-security.md`).

## Delivery Order
1. P0: Security baseline, data classification, and crypto contract.
2. P1: Encryption/key-management foundation in backend services.
3. P2: Address encryption rollout.
4. P3: Saved payment encryption + internal tokenization rollout.
5. P4: Key rotation, backfill, and cryptographic erasure.
6. P5: Operational hardening, monitoring, and incident readiness.

## Cross-Roadmap Alignment
- `roadmap/providers.md`
  - Saved payment records expose a provider-agnostic token/reference model used by provider adapters.
  - Provider credential encryption and customer payment-data encryption share the same key service primitives.
- `roadmap/legal-compliance-security.md`
  - Audit requirements, retention classes, and access controls for decrypt operations map into centralized security controls.
- `roadmap/guest-checkout.md`
  - Encryption scope for account-saved methods/addresses is separate from guest one-time checkout payload handling.

## P0: Security Baseline and Crypto Contract
### Scope
- Define exact field-level sensitivity classifications and allowed storage forms.
- Define encryption envelope and key-provider interface contracts.
- Define policy boundaries for what can be decrypted, by whom, and for which flows.

### Deliverables
- Data classification matrix for:
  - saved payment fields,
  - saved address fields,
  - order display snapshots (`orders.payment_method_display`, `orders.shipping_address_pretty`).
- ADR documenting:
  - envelope encryption approach,
  - AEAD algorithm choice and nonce strategy,
  - associated data (AAD) contract (`tenant/store`, `user_id`, `record_id`, `field`).
- Security policy updates:
  - CVV never stored,
  - decrypt allowed only inside dedicated service paths,
  - all decrypt operations auditable.

### Done Criteria
- Every sensitive field has exactly one approved storage format (`plaintext`, `masked`, `hashed index`, `ciphertext`).
- Crypto contract is accepted and referenced by implementation tasks.
- Prohibited fields and prohibited logs are explicitly listed and testable.

## P1: Encryption and Key-Management Foundation
### Scope
- Add reusable encryption service in `internal/`.
- Add key resolver supporting active key + previous keys for decrypt during rotation.
- Add model support for key versioning and ciphertext metadata.

### Deliverables
- New service package (example): `internal/security/crypto` with:
  - `Encrypt(plaintext, aad) -> ciphertext, key_id, alg, nonce`,
  - `Decrypt(ciphertext, aad, key_id) -> plaintext`.
- Config additions in `config/` for:
  - key provider mode (`dev-static`, `kms`),
  - active key identifier,
  - decrypt key ring.
- DB schema primitives:
  - reusable encrypted blob columns (or dedicated JSONB envelope fields),
  - `encryption_key_id`, `encryption_alg`, `encrypted_at` metadata.
- Audit hooks for decrypt operations and decrypt failures.

### Done Criteria
- Round-trip encryption/decryption tests pass with AAD mismatch rejection.
- Code paths fail closed when key config is missing or invalid.
- Sensitive values are never logged in plaintext in success or error paths.

## P2: Address Encryption Rollout
### Scope
- Encrypt saved address PII fields at write time.
- Preserve UX and operational behavior: defaults, list/read/delete, and masked display.
- Add safe lookup/index strategy for address dedup and default management.

### Deliverables
- `models/saved_addresses` migration to store encrypted fields for:
  - `full_name`, `line1`, `line2`, `city`, `state`, `postal_code`, `phone`.
- Optional blind index columns for limited exact-match operations (for example normalized postal code hash), if required.
- Handler/service refactor in `handlers/account_data.go` + `internal/services/...` so handlers stay thin and crypto logic is centralized.
- Backfill job to encrypt existing plaintext addresses and clear plaintext columns in one cut.

### Done Criteria
- DB snapshots show encrypted bytes for protected address fields.
- API responses remain functionally equivalent for authorized users.
- Unauthorized or malformed decrypt attempts return safe errors without plaintext leakage.

## P3: Saved Payment Encryption and Internal Tokenization
### Scope
- Introduce internal token/reference for saved payment methods.
- Encrypt sensitive payment fields at rest where stored.
- Keep non-sensitive display fields (`brand`, `last4`, `exp_month`, `exp_year`) accessible for UI.

### Deliverables
- Payment data model changes:
  - retain display metadata in `saved_payment_methods`,
  - move encrypted sensitive fields (for example PAN ciphertext/cardholder if required) to dedicated secret storage table (`saved_payment_method_secrets`) with strict access path.
- Internal token format:
  - stable opaque token bound to user/store scope,
  - versioned token metadata for forward compatibility.
- API contract updates in `api/openapi.yaml` (breaking-change allowed):
  - remove any implication that full card details are retrievable after save,
  - ensure create/update routes enforce CVV non-persistence behavior.
- Service logic ensuring provider adapters receive decrypted material only at authorized execution points.

### Done Criteria
- PAN is never stored in plaintext DB columns.
- CVV is never written to DB or logs.
- Saved payment retrieval APIs return masked/non-sensitive fields only.
- Attempted direct access to secret table from non-authorized services is blocked by design and tests.

## P4: Key Rotation, Re-Encryption, and Crypto Erasure
### Scope
- Add key rotation workflow with live decrypt-on-old / encrypt-on-new behavior.
- Add offline backfill/re-encryption job for full migration to current key.
- Add cryptographic erasure path for user data deletion scenarios.

### Deliverables
- Rotation job that:
  - reads ciphertext with old keys,
  - re-encrypts with active key,
  - updates key metadata atomically.
- Admin/internal endpoint or CLI command for controlled rotation execution.
- Erasure strategy:
  - delete encrypted blobs,
  - optionally invalidate/destroy specific DEKs if per-record/per-batch keys are used.
- Metrics and logs:
  - rotation progress,
  - failed decrypt counts,
  - stale-key record counts.

### Done Criteria
- Rotation can run safely without downtime and is idempotent.
- Post-rotation report shows zero records on retired keys (or explicit allowlist exceptions).
- Deletion flow verifiably removes encrypted payload material.

## P5: Operational Hardening and Incident Readiness
### Scope
- Enforce access-control, observability, and runbooks around encrypted user data.
- Validate behavior under incident scenarios (DB leak, key leak, partial compromise).

### Deliverables
- RBAC policy integration for decrypt-capable operations (admin and service actor boundaries).
- Alerting and dashboards for:
  - unusual decrypt volume,
  - repeated decrypt failures,
  - key misconfiguration events.
- Runbooks in `wiki/` (or `roadmap/` fallback):
  - DB leak response,
  - key compromise response,
  - emergency key rotation.
- Security test suite additions:
  - ciphertext-only DB assertions,
  - negative authorization/decrypt tests,
  - log redaction regression tests.

### Done Criteria
- Security incident drill for DB-only compromise demonstrates unreadable sensitive fields without keys.
- Key compromise playbook tested with measurable recovery timeline.
- Monitoring detects and alerts on simulated abuse patterns.

## Data Model Changes
1. `saved_addresses`
- Replace sensitive plaintext columns with encrypted equivalents or encrypted envelope columns.
- Add encryption metadata columns (`encryption_key_id`, `encrypted_at`, `encryption_alg`).

2. `saved_payment_methods`
- Keep non-sensitive display metadata only (`brand`, `last4`, `exp_month`, `exp_year`, nickname/default flags).
- Add token/reference fields and encryption metadata references.

3. `saved_payment_method_secrets` (new)
- Stores encrypted sensitive payload (`pan_ciphertext`, optional encrypted cardholder name if needed).
- Strict FK to `saved_payment_methods`, with minimal query surface.

4. Optional blind-index columns/tables
- Deterministic keyed hashes for narrowly approved lookup needs (for example exact postal-code match), never reversible.

5. `decrypt_audit_events` (new or integrated with central audit tables)
- Actor, purpose, resource, timestamp, result for each decrypt attempt.

## Endpoint/API Plan
1. Existing customer routes to harden
- `GET /api/v1/me/payment-methods`
- `POST /api/v1/me/payment-methods`
- `DELETE /api/v1/me/payment-methods/{id}`
- `PATCH /api/v1/me/payment-methods/{id}/default`
- `GET /api/v1/me/addresses`
- `POST /api/v1/me/addresses`
- `DELETE /api/v1/me/addresses/{id}`
- `PATCH /api/v1/me/addresses/{id}/default`

2. Contract updates
- Clarify response payloads are masked/non-sensitive.
- Clarify CVV is transient input only and not persisted.
- If needed, add explicit provider-agnostic payment reference fields consumed by checkout APIs.

3. Internal/admin controls (if introduced)
- Key rotation trigger/status endpoint or CLI-equivalent.
- Decrypt audit inspection endpoint under admin security namespace.

## Execution Workflow in This Repo
1. Update `api/openapi.yaml` first for any request/response contract changes.
2. Run `make openapi-gen` and commit generated artifacts:
- `internal/apicontract/openapi.gen.go`
- `frontend/src/lib/api/generated/openapi.ts`
3. Add migrations in `internal/migrations/` and update models in `models/`.
4. Implement encryption/key logic in `internal/` services; keep `handlers/` thin.
5. Refactor payment/address handlers to consume service layer abstractions.
6. Run formatter on touched files (`gofmt -w <file>`).
7. Run backend tests for touched packages (`GOCACHE=/tmp/go-build go test ./...`).
8. Run `make openapi-check` before merge.
9. Update docs/runbooks in `wiki/` (or `roadmap/` if wiki repo is unavailable).

## Risk Register
- Storing PAN internally, even encrypted, keeps the system in higher PCI scope and increases audit burden.
- Weak or reused AAD design can allow ciphertext substitution attacks across records.
- Deterministic encryption/blind indexes can leak equality patterns if overused.
- Backfill errors can corrupt decryptability without robust migration verification and rollback plans.
- Logging or tracing middleware can accidentally capture sensitive request fields unless explicitly scrubbed.
- Key compromise invalidates DB-at-rest protections; response speed depends on rotation readiness.

## Immediate Next Slice
1. Ship P0 ADR and data-classification matrix with explicit prohibited fields/logging rules.
2. Implement P1 crypto service + key metadata schema and unit tests.
3. Encrypt `saved_addresses` first (P2) with migration/backfill and handler-service refactor.
4. Implement `saved_payment_method_secrets` and migrate saved payment creation flow to encrypted storage (P3).
5. Add decrypt audit events and a minimal key-rotation CLI path (P4 starter).
