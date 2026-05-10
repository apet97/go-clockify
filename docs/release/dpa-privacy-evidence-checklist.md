# DPA / Terms / Privacy Evidence Checklist

Status: **REQUEST PACKET — NO COUNSEL REVIEW PERFORMED**. This is the
operator-side checklist for packaging the data-processing addendum,
customer-terms, and privacy / data-handling artifacts that counsel
needs in order to record a written sign-off on the
`DPA / terms / privacy posture` gate in
[`../launch-readiness-review-may-8.md`](../launch-readiness-review-may-8.md).

This checklist does not perform the legal review, does not record
counsel's decision, and does not close the gate. The gate's evidence
artifact is "an executed DPA template (or counsel-signed written
confirmation that the existing CAKE.com DPA covers `clockify-mcp`
paid-hosted usage) plus a counsel-acknowledged data-flow review"
plus a launch-readiness ledger row referencing the executed document
and counsel identity. Local doc edits do not substitute.

This is not a community-MCP affiliation, endorsement, or partnership
claim. `clockify-mcp` is a community MCP server; the DPA / terms /
privacy posture remains under counsel review.

## Owner / reviewer role

Per the gate's "Owner role" line:

- **Reviewer:** CAKE.com / Clockify legal counsel responsible for
  customer terms and the data-processing addendum, plus a privacy /
  data-handling reviewer authorized to sign off on personal-data
  flows. Both seats may be filled by the same individual if their
  authority covers both surfaces; counsel records that explicitly.
  The repository maintainer cannot self-approve this gate.
- **Maintainer (request driver):** `@apet97`. Drives the request,
  packages the data-flow evidence, answers counsel's questions, and
  archives the response per
  [`../launch-readiness-review-may-8.md`](../launch-readiness-review-may-8.md).

Until counsel accepts the engagement, the row in
`docs/launch-readiness-review-may-8.md` § "DPA / terms / privacy
posture" stays open. Record the assigned counsel identity and the
engagement date in that row before any review iteration begins.

## Scope of the counsel review

Counsel is asked to sign off on three orthogonal surfaces:

1. **Customer terms / DPA.** Whether the existing CAKE.com DPA (or
   a `clockify-mcp`-specific addendum) covers paid-hosted usage of
   the `clockify-mcp` server, including subprocessor disclosure,
   data-residency posture, security-incident notification timing,
   and audit-rights wording.
2. **Privacy / personal-data flows.** Whether the personal data the
   server actually persists matches what the customer-facing privacy
   notice describes; whether retention windows are honored; whether
   credential-handling and incident-response posture are
   acceptable.
3. **Operator obligations.** Whether the operator-facing posture
   documented in this repository matches what counsel can defend
   under the executed terms (e.g., whether the audit-retention
   window the operator sets is consistent with the contractual
   commitment).

Counsel is **not** asked to review trademark / "official Clockify"
framing (separate gate, see
[`brand-legal-review.md`](brand-legal-review.md)), the external
security review (see
[`external-security-review-request.md`](external-security-review-request.md)),
or the paid-commercial RLS decision (separate ADR template under
`docs/adr/`).

## Evidence bundle the maintainer hands to counsel

The maintainer prepares the following bundle for each counsel
iteration. Counsel may request more; the bundle below is the
floor.

### 1. Personal-data flow inventory

Counsel needs a written inventory of the personal data the server
persists, where it lives, how long it lives, and which operator
controls each retention knob. The current shape is recorded across
several files; this checklist is the index counsel reads first.

| Data class | Persisted in | Persistence trigger | Retention control | Reference |
|---|---|---|---|---|
| Authenticated principal claims (subject + tenant ID + claim subset) | `sessions` table (`internal/controlplane/postgres/`) at session creation; rebuilt on cross-pod rehydration after strict re-auth. | Streamable-HTTP session creation (`initialize` handshake). | `MCP_SESSION_TTL` (default 24h ceiling). | [`../auth-model.md`](../auth-model.md) § "Tenant resolution"; [`../adr/0017-streamable-http-session-rehydration.md`](../adr/0017-streamable-http-session-rehydration.md). |
| Tool-call audit records (intent + outcome phases, tool name, risk class, action-defining argument keys, principal subject, tenant ID, outcome status) | `audit_events` table. | Every non-read-only tool call when `MCP_AUDIT_DURABILITY` is `fail_closed` (prod default), `fail_closed_strict`, or `best_effort` (dev default; phase rows are still written when persistence succeeds). | `MCP_CONTROL_PLANE_AUDIT_RETENTION` (operator-set per compliance window). | [`../../SECURITY.md`](../../SECURITY.md) § "Security Features" → Audit durability + Audit fidelity; [`../runbooks/audit-durability.md`](../runbooks/audit-durability.md). |
| Clockify API key (per-principal) | Inbound HTTP headers / OIDC claims at request time; never written to the control-plane store; never logged (slog `RedactingHandler`). | Per request. | Lifetime of the request; cleared when the request handler returns. | [`../auth-model.md`](../auth-model.md) § "Failure modes"; [`../runbooks/credential-leak-response.md`](../runbooks/credential-leak-response.md). |
| OIDC verify cache entries (token decode result keyed by `kid` + signature, no full token text) | In-process cache, never persisted. | OIDC verify path. | Bounded by `oidcVerifyCacheTTLCeiling` (5 minutes; tunable down via `MCP_OIDC_VERIFY_CACHE_TTL`). | `internal/authn/oidc_verify_cache_test.go`; [`../runbooks/tenant-offboarding.md`](../runbooks/tenant-offboarding.md). |
| Webhook URLs (per-tenant operator-supplied) | Forwarded to upstream Clockify; not persisted in the control plane. | Tool-call argument when invoking webhook tools. | Lifetime of the upstream Clockify resource. | [`../../SECURITY.md`](../../SECURITY.md) § "Security Features" → Webhook URL validation. |
| HTTP request/response bodies | Not persisted. Slog logs are PII-redacted by `internal/logging.RedactingHandler`. | Per request. | Process log retention only. | [`../../SECURITY.md`](../../SECURITY.md) § "Security Features" → PII-redacting logs. |
| Upstream Clockify response bodies on error paths | Logged server-side to slog for operator debugging when `CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=1` (hosted-profile default); omitted from the tool-error envelope returned to the client. | Per upstream error response on `shared-service` / `prod-postgres` profiles. | Process log retention only. | [`../../SECURITY.md`](../../SECURITY.md) § "Security Features" → Hosted-profile error sanitisation. |

If counsel asks for a column that is not in this table, add the
column here rather than answering inline — that keeps every
counsel iteration working off the same inventory.

### 2. Counsel-acknowledgement checklist

For each of the seven personal-data classes above, counsel records
acknowledgement against the following acceptance criteria. The
acknowledgement is the artifact the launch-readiness ledger
references; an unchecked row keeps the gate open.

- [ ] **Subprocessor disclosure** is consistent with the upstream
      Clockify API, the ingress / load-balancer provider, and the
      Postgres-hosting subprocessor for the paid-hosted plane.
- [ ] **Data residency** wording in the DPA matches where the
      Postgres control plane and the upstream Clockify region
      actually run for the paid-hosted plane.
- [ ] **Retention windows.** The operator-controlled retention
      knobs (`MCP_SESSION_TTL`,
      `MCP_CONTROL_PLANE_AUDIT_RETENTION`,
      `MCP_OIDC_VERIFY_CACHE_TTL`) are bounded such that no
      personal data survives longer than the contractual
      commitment.
- [ ] **Credential handling.** The Clockify API key handling
      described in [`../auth-model.md`](../auth-model.md) and the
      credential-leak response posture in
      [`../runbooks/credential-leak-response.md`](../runbooks/credential-leak-response.md)
      are acceptable to counsel as the operator's incident-response
      posture under the executed terms.
- [ ] **OIDC token decode + verify-cache behavior.** Counsel
      acknowledges that token claims are decoded for
      authentication, that the verify cache stores the verification
      *result* (not the raw token), and that the cache TTL ceiling
      is bounded.
- [ ] **slog redaction posture.** `internal/logging.RedactingHandler`
      masks 20+ well-known secret-key patterns and obvious secret-
      shaped string values before encoding; counsel confirms this is
      acceptable as the default operational logging posture.
- [ ] **Audit retention.** `MCP_CONTROL_PLANE_AUDIT_RETENTION` is
      set to a value counsel approves for the contractual
      compliance window for the paid-hosted plane.
- [ ] **Cross-tenant isolation contract.** Counsel acknowledges
      the application-layer tenant scoping posture documented in
      [`../auth-model.md`](../auth-model.md) and the
      cross-tenant E2E pins; the
      `P1-8 paid-commercial RLS decision` gate is recorded
      separately and counsel notes whether DPA wording is contingent
      on database-enforced RLS landing first.
- [ ] **Tenant offboarding.** The OIDC verify-cache TTL ceiling +
      rolling-restart drain documented in
      [`../runbooks/tenant-offboarding.md`](../runbooks/tenant-offboarding.md)
      meets the contractual sub-token-lifetime revocation
      requirement, OR counsel records that revocation enforcement
      is delegated to the IdP / proxy and the DPA reflects that
      delegation.
- [ ] **Postgres restore posture.** The restore drill in
      [`../runbooks/postgres-restore.md`](../runbooks/postgres-restore.md)
      meets the recovery-point and recovery-time wording in the
      DPA / customer terms.
- [ ] **Incident notification timing.** The
      [`../../SECURITY.md`](../../SECURITY.md) "Response Timeline"
      (acknowledgment within 48 hours; initial assessment within 1
      week) is consistent with contractual incident-notification
      commitments.
- [ ] **Privacy-notice consistency.** The customer-facing privacy
      notice describes the personal-data classes above without
      omissions and without listing classes that are not actually
      persisted.

### 3. Operator obligations the counsel iteration confirms

Counsel records the following operator-facing obligations as
either "consistent with executed terms" or "requires operator
remediation before launch":

- [ ] Hosted deployments default to `MCP_PROFILE=prod-postgres`
      with `MCP_OIDC_STRICT=1`, `MCP_OIDC_REQUIRE_KID=1`,
      `MCP_REQUIRE_TENANT_CLAIM=1`, `MCP_DISABLE_INLINE_SECRETS=1`,
      `MCP_AUDIT_DURABILITY=fail_closed`, and
      `CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=1` per the public
      hosted-launch checklist.
- [ ] `MCP_EXPOSE_AUTH_ERRORS=0` is the contractual default;
      operator-driven break-glass overrides require startup-log
      evidence and are documented in incident-response artifacts.
- [ ] Backup and restore drill cadence (the public hosted-launch
      checklist requires the drill to have run within the last 90
      days) is consistent with contractual recovery commitments.
- [ ] The community-MCP framing is preserved across customer-
      facing surfaces; no surface claims official-product status,
      endorsement, or partnership without the trademark gate
      closed (separate gate; see
      [`brand-legal-review.md`](brand-legal-review.md)).

### 4. Documents referenced by the executed DPA / terms entry

For the gate to close, the launch-readiness ledger row must
reference an executed document and counsel identity. Counsel
records the document reference here for the maintainer to copy
into the ledger:

- **Executed DPA template / amendment:** `<ticket or contract reference>`
  (redacted of private contact details before archival).
- **Counsel identity:** `<full name, role, CAKE.com / Clockify team>`.
- **ISO 8601 sign-off date:** `<YYYY-MM-DD>`.
- **Privacy / data-handling reviewer (if separate from counsel):**
  `<full name, role>`.
- **Privacy reviewer ISO 8601 sign-off date:** `<YYYY-MM-DD>`.
- **Scope statement:** which of the 12 acknowledgement-checklist
  rows above counsel attests to. A row counsel does not attest to
  must be called out as either "out of scope" or "operator
  remediation required".

## Closure rule

This gate closes only when:

1. The executed document (DPA template, amendment, or counsel-
   signed confirmation) is archived per the maintainer's
   contract-archive process.
2. The counsel-acknowledgement checklist above is filled in,
   archived, and referenced from the launch-readiness ledger row
   for "DPA / terms / privacy posture".
3. The launch-readiness ledger row quotes counsel identity, ISO
   8601 date, and document reference (redacted of private contact
   details).

[`../auth-model.md`](../auth-model.md),
[`../runbooks/credential-leak-response.md`](../runbooks/credential-leak-response.md),
[`../../SECURITY.md`](../../SECURITY.md), the `RedactingHandler`
slog tests, and any local doc-parity guard about privacy wording
do **not** close the gate. They are inputs counsel must inspect;
counsel's written sign-off is the only closure artifact.

## Non-goal of this checklist

This document is a request packet. It does not:

- Perform the legal review.
- Record counsel's decision.
- Approve a release.
- Grant official-product status, endorsement, partnership, or any
  legal claim.
- Replace the trademark gate, the external security review gate,
  the RLS decision gate, or the cross-replica quota gate.

Update this checklist only when the personal-data inventory
changes (new persisted class, removed class, changed retention
control, changed acknowledgement criterion). Each counsel
iteration's identity, date, and document reference live in the
launch-readiness ledger, not here.
