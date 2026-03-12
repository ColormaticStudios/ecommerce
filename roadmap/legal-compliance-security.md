# Legal Compliance and Security Controls Roadmap

## Current Baseline
- The project has core auth/authz primitives, but no explicit permission catalog, role graph, or deny-by-default RBAC policy model.
- Auditability exists in isolated areas, but there is no platform-wide immutable admin action log with evidence-friendly export and retention semantics.
- Secrets usage exists for provider integrations and app config, but there is no centralized secret inventory, rotation schedule policy, or break-glass workflow.
- Privacy rights workflows (access/export, delete/erasure, correction, objections/limits) are not yet modeled as first-class request lifecycles.
- Tax and compliance exports are being introduced in adjacent work (`roadmap/providers.md`, `roadmap/merchant-analytics-reporting.md`) but evidence packaging and retention controls are not centralized.

## Goals
- Implement fine-grained RBAC that is explicit, testable, and safe under default-deny semantics.
- Provide full admin audit trails with tamper-evident storage and deterministic export for audits, disputes, and incident response.
- Define and enforce secrets lifecycle standards (ownership, rotation cadence, emergency rotation, evidence of rotation).
- Deliver privacy request workflows for data export and delete with identity verification, legal hold exceptions, and SLA tracking.
- Build compliance evidence trails for tax and operational controls with reproducible artifacts and retention policy enforcement.
- Align controls to common frameworks used by ecommerce teams (PCI DSS v4.x, CCPA/CPRA, GDPR controls, SOC 2-style evidence expectations).

## Non-Goals
- Legal interpretation or jurisdiction-specific legal advice (counsel still required before production policy commitments).
- Full GRC platform replacement or ISO/SOC automation tooling procurement in this roadmap.
- Fraud decisioning, KYC, or AML systems beyond audit integration touchpoints.
- End-user marketing consent preference center redesign (handled in a separate roadmap if needed).

## Delivery Order (Strict)
1. P0: Control baseline and policy model.
2. P1: Fine-grained RBAC and authorization enforcement.
3. P2: Immutable admin audit logs and evidence exports.
4. P3: Privacy rights workflows (export/delete/correction/objection).
5. P4: Secrets lifecycle governance and rotation evidence.
6. P5: Tax/compliance evidence vault and operational hardening.

## Cross-Roadmap Alignment
- Checkout baseline:
  - Privacy workflows must support both authenticated users and guest/session-bound orders (`checkout_session_id` ownership).
- Providers (`roadmap/providers.md`):
  - Provider credential handling, webhook events, and transaction events are evidence inputs for this roadmap.
  - Credential rotation metadata integrates with provider configuration lifecycle.
- Merchant analytics and reporting (`roadmap/merchant-analytics-reporting.md`):
  - Tax and finance exports generated there become signed/retained artifacts here.
- Order fulfillment and returns (`roadmap/order-fulfillment-ops.md`, `roadmap/returns-rma.md`):
  - Admin mutations in fulfillment/returns must emit standardized audit entries with actor and reason fields.
- Canonical write surfaces:
  - Customer mutation routes remain `/api/v1/checkout/*`.
  - Admin control routes remain `/api/v1/admin/*`.

## P0: Control Baseline and Policy Model
### Scope
- Define policy model for roles, permissions, scopes, and actor types.
- Define audit event schema and compliance evidence taxonomy.
- Define privacy request lifecycle states and SLA targets.
- Define secret classes and minimum rotation/ownership standards.

### Deliverables
- Architecture decision record in `wiki/` (or `roadmap/` temporary) describing:
  - permission naming convention (`resource:action[:scope]`),
  - actor model (`user`, `service_account`, `automation_worker`),
  - default-deny behavior and break-glass approval flow.
- Compliance controls matrix mapping product controls to:
  - PCI DSS v4.x control families,
  - CCPA/CPRA consumer rights workflow requirements,
  - GDPR rights handling and response-time controls.
- Threat model and abuse cases for:
  - unauthorized admin actions,
  - audit log tampering,
  - privacy workflow abuse,
  - stale secrets and credential leakage.

### Done Criteria
- A control matrix exists and is accepted by engineering + security stakeholders.
- Every proposed table/endpoint in later phases has named ownership and data retention class.
- Explicit go/no-go criteria are defined for enabling new admin surfaces.

## P1: Fine-Grained RBAC and Authorization Enforcement
### Scope
- Introduce permission catalog, role templates, role assignments, and scoped grants.
- Enforce authorization in handlers with centralized guard helpers (thin handlers, reusable service logic).
- Add bootstrap flow for initial super-admin with mandatory break-glass protections.

### Deliverables
- Schema + models for:
  - `permissions`,
  - `roles`,
  - `role_permissions`,
  - `principal_role_bindings` (with scope such as `global`, `merchant_id`, `resource_id`),
  - `authorization_decision_audit` (optional lightweight decision trace).
- Service layer under `internal/` for:
  - policy evaluation,
  - scope resolution,
  - cached permission expansion with invalidation.
- Middleware updates in `middleware/`:
  - standardized permission checks per route,
  - request context actor metadata normalization.
- Admin API endpoints:
  - role CRUD,
  - permission catalog read,
  - role assignment/revocation with justification.

### Done Criteria
- High-risk admin routes are all gated by explicit permissions.
- Default-deny applies to routes without mapped permissions.
- Unit and integration tests cover valid and invalid grants, missing scopes, and privilege escalation attempts.
- RBAC changes emit audit events (phase P2 contract compatible).

## P2: Immutable Admin Audit Logs and Evidence Exports
### Scope
- Implement append-only admin audit event store.
- Capture who, what, when, where, why, and before/after summaries for mutable operations.
- Provide query and export APIs suitable for internal audits and incident response.

### Deliverables
- Schema + models:
  - `admin_audit_events` (immutable),
  - `admin_audit_event_hash_chain` (optional tamper-evidence),
  - `admin_audit_export_jobs`.
- Event envelope fields:
  - `event_id`, `occurred_at`, `actor_type`, `actor_id`, `actor_ip`, `user_agent`,
  - `action`, `resource_type`, `resource_id`,
  - `request_id`/`correlation_id`,
  - `reason_code`, `change_summary_redacted`,
  - `legal_hold_tags`, `retention_class`.
- Admin API:
  - paginated read with filter by actor/action/resource/date,
  - async export (CSV/JSONL) with signed checksum manifest.
- Worker jobs for export generation and retention processing.

### Done Criteria
- All admin mutating endpoints emit exactly one canonical audit event per successful mutation.
- Failed authorization attempts are logged with safe redaction.
- Exported audit files are reproducible and checksum-verified.
- Retention purge only affects records past retention policy and not under legal hold.

## P3: Privacy Rights Workflows (Export/Delete/Correction/Objection)
### Scope
- Create privacy request pipeline for consumer rights handling.
- Support authenticated and guest request channels with identity verification.
- Implement data export package and deletion workflow with exception handling.

### Deliverables
- Schema + models:
  - `privacy_requests`,
  - `privacy_request_events`,
  - `privacy_request_artifacts`,
  - `privacy_request_verifications`,
  - `data_retention_exceptions`.
- Request types:
  - `access_export`,
  - `delete_erasure`,
  - `correct_data`,
  - `limit_or_object_processing`.
- API endpoints:
  - `POST /api/v1/privacy/requests`,
  - `GET /api/v1/privacy/requests/{id}`,
  - admin review/decision endpoints under `/api/v1/admin/privacy/*`.
- Export format:
  - machine-readable archive with manifest and generated timestamp.
- Deletion workflow:
  - soft-delete stage,
  - dependency-aware purge jobs,
  - legal hold and statutory retention exception path.

### Done Criteria
- Requests move through explicit states (`submitted`, `verified`, `in_review`, `approved`, `rejected`, `fulfilled`, `closed`).
- Identity verification is required before data release or delete execution.
- SLA timers and breach alerts exist for overdue requests.
- Tests cover valid requests, invalid/unauthenticated requests, replay/idempotency, and legal-hold exceptions.

## P4: Secrets Lifecycle Governance and Rotation Evidence
### Scope
- Standardize secrets inventory, ownership metadata, and rotation workflows.
- Implement rotation evidence collection and emergency rotation runbooks.
- Enforce non-logging/non-exposure rules for secrets and token material.

### Deliverables
- Schema + models:
  - `secret_inventory`,
  - `secret_versions`,
  - `secret_rotation_events`,
  - `secret_access_audit`.
- Policy controls:
  - each secret has owner, environment, purpose, rotation SLA, and last rotation timestamp,
  - dual-control approval for high-impact rotations (payment/tax provider creds),
  - emergency rotate and revoke flow.
- Integrations:
  - provider credential tables from `roadmap/providers.md`,
  - CI/CD secret references and deploy-time validation hooks.
- Operational assets:
  - rotation runbook,
  - incident runbook for leaked credentials,
  - monthly compliance report job.

### Done Criteria
- All production secrets are inventoried and linked to owner + rotation policy.
- Rotation events are auditable with actor + timestamp + result.
- Secret values are not persisted in plaintext logs, audit payloads, or API responses.
- Quarterly game-day validates emergency rotation for at least one critical integration.

## P5: Tax/Compliance Evidence Vault and Hardening
### Scope
- Build centralized evidence store for tax reports, audit exports, privacy fulfillment artifacts, and control attestations.
- Add retention and legal-hold policies with immutable metadata.
- Provide auditor-ready evidence retrieval by control, period, and entity.

### Deliverables
- Schema + models:
  - `compliance_artifacts`,
  - `compliance_artifact_versions`,
  - `compliance_control_links`,
  - `legal_holds`,
  - `compliance_attestations`.
- Evidence ingestion:
  - tax liability exports and sales journals,
  - admin audit exports,
  - privacy request fulfillment manifests,
  - secret rotation summaries.
- Admin API:
  - search by control framework, period, merchant/entity, and artifact type,
  - download with integrity hash verification.
- Monitoring:
  - missing-artifact alerts by period,
  - failed retention policy jobs,
  - evidence integrity drift checks.

### Done Criteria
- Required monthly/quarterly evidence artifacts are generated and retrievable.
- Evidence metadata is immutable after finalization (new version required for corrections).
- Tax totals in retained artifacts reconcile with source order/payment/tax snapshot data.
- Compliance export can be produced for a selected audit period without manual DB querying.

## Data Model Changes (Priority Order)
1. `permissions`, `roles`, `role_permissions`, `principal_role_bindings`
- Fine-grained RBAC graph and scoped assignments.

2. `admin_audit_events`, `admin_audit_export_jobs`, `admin_audit_event_hash_chain`
- Immutable admin action trail and export evidence.

3. `privacy_requests`, `privacy_request_events`, `privacy_request_artifacts`, `privacy_request_verifications`
- Privacy rights workflow state machine and artifacts.

4. `secret_inventory`, `secret_versions`, `secret_rotation_events`, `secret_access_audit`
- Secret governance and rotation evidence.

5. `compliance_artifacts`, `compliance_control_links`, `compliance_attestations`, `legal_holds`
- Cross-domain evidence vault with retention/legal hold controls.

## Endpoint/API Plan
1. RBAC admin endpoints
- `GET /api/v1/admin/permissions`
- `GET /api/v1/admin/roles`
- `POST /api/v1/admin/roles`
- `PATCH /api/v1/admin/roles/{id}`
- `POST /api/v1/admin/roles/{id}/assignments`
- `DELETE /api/v1/admin/roles/{id}/assignments/{assignmentId}`

2. Audit log endpoints
- `GET /api/v1/admin/audit/events`
- `POST /api/v1/admin/audit/exports`
- `GET /api/v1/admin/audit/exports/{id}`

3. Privacy endpoints
- `POST /api/v1/privacy/requests`
- `GET /api/v1/privacy/requests/{id}`
- `POST /api/v1/admin/privacy/requests/{id}/verify`
- `POST /api/v1/admin/privacy/requests/{id}/approve`
- `POST /api/v1/admin/privacy/requests/{id}/reject`
- `POST /api/v1/admin/privacy/requests/{id}/fulfill`

4. Compliance evidence endpoints
- `GET /api/v1/admin/compliance/artifacts`
- `POST /api/v1/admin/compliance/artifacts/{id}/finalize`
- `GET /api/v1/admin/compliance/controls/{controlId}/evidence`

5. Secret governance endpoints (admin internal)
- `GET /api/v1/admin/security/secrets`
- `POST /api/v1/admin/security/secrets/{id}/rotate`
- `GET /api/v1/admin/security/secrets/{id}/rotation-events`

## Execution Workflow in This Repo
1. Update `api/openapi.yaml` first for every endpoint or schema change.
2. Run `make openapi-gen` and commit generated artifacts:
- `internal/apicontract/openapi.gen.go`
- `frontend/src/lib/api/generated/openapi.ts`
3. Implement/adjust models and migrations under `models/` and `internal/migrations/`.
4. Keep handlers thin in `handlers/`; put policy, audit, privacy, and evidence logic in `internal/` services.
5. Add backend tests (`GOCACHE=/tmp/go-build go test ./...`) for touched packages.
6. Add frontend checks for touched UI workflows (`cd frontend && bun run check && bun run lint`).
7. Run `make openapi-check` before merge.
8. Update docs/runbooks in `wiki/` (or `roadmap/` fallback when wiki repo is unavailable).

## Risk Register
- Overly broad role grants can create hidden privilege escalation paths.
- Audit payloads can leak PII/secrets if redaction policy is incomplete.
- Privacy deletion may conflict with statutory retention/tax records if exception modeling is weak.
- Secret rotation without staged rollout can cause provider downtime.
- Evidence generation that is not deterministic can fail audits even when controls exist.
- High cardinality audit/event tables can impact query performance if partitioning/indexing is delayed.

## Immediate Next Slice (Suggested)
1. Ship P0 control matrix and ADR for permission model + audit schema.
2. Implement P1 minimal RBAC on highest-risk admin endpoints (orders, payments, tax exports, provider credentials).
3. Add P2 canonical `admin_audit_events` writes for those same endpoints.
4. Launch P3 MVP for privacy `access_export` and `delete_erasure` requests.
5. Add P4 secret inventory table and first rotation report job.
