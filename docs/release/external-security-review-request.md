# External Security Review Request Packet

> **Historical artifact. Not current one-user MCP product documentation.**
> Preserved for platform-era audit/history only. Start current one-user work from `README.md`, `docs/agent-cookbook.md`, `docs/tool-catalog.md`, and `docs/goals/oneuser-tool-coverage.md`.


Status: **REQUEST PACKET — NO REVIEW PERFORMED**. This document is the
operator-side request packet that the maintainer hands to a third-party
or peer security reviewer. It does not perform a review, does not record
findings, and does not close the
`Paid-hosted external security review` gate in
[`../launch-readiness-review-may-8.md`](../launch-readiness-review-may-8.md).
That gate closes only when the reviewer's written attestation lands per
[`../../SECURITY.md`](../../SECURITY.md).

This is not a community-MCP affiliation, endorsement, or partnership
claim. `clockify-mcp` is a community MCP server; the paid-hosted
launch posture remains under review and no official-product status
is asserted by this packet.

## Why this packet exists

The May 8 launch-readiness ledger pins the external review gate at
[`../launch-readiness-review-may-8.md`](../launch-readiness-review-may-8.md)
§ "Paid-hosted external security review". The gate's evidence
artifact is "a new section in `SECURITY.md` (or a linked review
report under `docs/security/`) carrying reviewer identity, ISO 8601
review date, the candidate tag reviewed, scope statement, findings
list, and the reviewer's release-status recommendation." Local
verifiers cannot satisfy that evidence shape; the maintainer can
only package the request and hand it to the reviewer.

This packet keeps the request shape stable across iterations so the
reviewer always sees the same scope, same prior-evidence pointers,
and same expected deliverable shape regardless of which candidate
tag they review.

## Owner / reviewer role

Per the gate's "Owner role" line:

- **Reviewer (one of):** a third-party security firm under engagement,
  a peer reviewer outside the `@apet97` self-merge boundary, or a
  designated CAKE.com security reviewer with authority to record
  findings and a release-status recommendation. The repository
  maintainer cannot serve as the reviewer.
- **Maintainer (request driver):** `@apet97`. The maintainer prepares
  the packet, ships the candidate-tag evidence bundle, answers
  reviewer questions, and archives the response.

Until a reviewer accepts the engagement, the row in
`docs/launch-readiness-review-may-8.md` § "Paid-hosted external
security review" stays open. Record the assigned reviewer's identity
and the engagement date in that row before any review iteration
begins.

## Scope statement (what the reviewer is asked to cover)

The reviewer is asked to read the candidate tag end-to-end against
the live attack surface and to record a written release-status
recommendation. The scope mirrors `SECURITY.md` § "Scope":

1. **Authentication.** All four supported auth modes — `static_bearer`,
   `oidc` (with `MCP_OIDC_STRICT=1` and `MCP_OIDC_REQUIRE_KID=1`),
   `forward_auth` (with `MCP_FORWARD_AUTH_TRUSTED_PROXIES`
   CIDR allow-list), and `mtls`. The reviewer is asked to assess
   bypass risk, token-binding correctness, JWKS rotation behavior,
   audience/resource-URI binding (RFC 8707), and the OIDC verify
   cache TTL ceiling. Code entry points: `internal/authn/`.
2. **Tenant isolation.** Application-layer tenant scoping in the
   shared-service profile; cross-tenant rejection in the streamable-
   HTTP session manager (ADR 0017 Path A); audit and session row
   keying. Code entry points: `internal/mcp/transport_streamable_http.go`,
   `internal/controlplane/postgres/`. Reviewer reads the
   cross-tenant E2E pins listed under "Prior internal review
   pointers" below; reviewer is asked to assess whether application-
   layer scoping plus session-rehydration strict re-auth is
   sufficient for the paid-hosted threat model or whether
   database-enforced RLS is required (the latter is tracked in the
   `P1-8 paid-commercial RLS decision` gate and the ADR template
   under `docs/adr/`).
3. **Audit durability.** `MCP_AUDIT_DURABILITY=fail_closed` default
   under `ENVIRONMENT=prod`; the `intent`/`outcome` phase recorder;
   `clockify_mcp_audit_failures_total{reason="persist_error",phase="outcome"}`
   alert wiring. Reviewer is asked to assess whether non-read-only
   tool calls can mutate state without a corresponding intent or
   outcome record under any failure mode.
4. **Transport surfaces.** `streamable_http` (default hosted),
   `grpc` (build-tag opt-in), and `stdio`. Reviewer is asked to
   assess the HTTP baseline header set
   (`Strict-Transport-Security` when TLS or a trusted HTTPS proxy is
   active, `Cross-Origin-Opener-Policy: same-origin`,
   `Cross-Origin-Embedder-Policy: require-corp`,
   `Cross-Origin-Resource-Policy: same-origin`,
   plus `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`,
   `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`,
   `Permissions-Policy: ()`, `X-Content-Type-Options: nosniff`,
   `Cache-Control: no-store`), CORS posture
   (`MCP_ALLOWED_ORIGINS`, `MCP_ALLOW_ANY_ORIGIN`), DNS rebinding
   protection (`MCP_STRICT_HOST_CHECK`), webhook URL validation,
   and panic containment.
5. **Supply chain.** Cosign keyless signing chain on every binary
   and the multi-arch container image; SPDX SBOM per binary; SLSA
   build provenance when GitHub artifact attestations are available
   (per ADR 0013, the user-owned private repository skip path
   applies — reviewer reads the ADR for context); `govulncheck`
   under the pinned `tools/govulncheck` module against the candidate
   Go pin; `gitleaks` and Semgrep `p/default` evidence. Reviewer is
   asked to assess whether the chain is sufficient for the paid-
   hosted threat model.
6. **Hosted-profile error sanitisation.** `CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=1`
   default on `shared-service` and `prod-postgres`; tool-error
   responses must not leak per-tenant identifiers from upstream
   Clockify response bodies on cross-tenant traffic. Reviewer is
   asked to confirm the sanitisation contract holds across the
   tool-error envelope path.
7. **Cross-replica admission.** Process-local
   `MCP_HTTP_RATELIMIT_PER_IP`, `MCP_HTTP_RATELIMIT_PER_PRINCIPAL`,
   and `MCP_HTTP_RATELIMIT_GET_PER_SESSION` limits plus the external
   gateway/load-balancer evidence under the `Cross-replica hosted
   HTTP quotas` gate. Reviewer is asked to read the cross-replica
   quota proof checklist in
   [`../runbooks/release-candidate-evidence.md`](../runbooks/release-candidate-evidence.md)
   alongside this scope so per-process-only enforcement does not get
   credited against cross-replica claims.

The reviewer is **not** asked to review legal posture (DPA / terms /
privacy), trademark / "official Clockify" framing, or the
`clockify://` URI scheme and gRPC service-name branding. Those are
separate gates with their own owners and evidence artifacts under
[`brand-legal-review.md`](brand-legal-review.md) and
[`dpa-privacy-evidence-checklist.md`](dpa-privacy-evidence-checklist.md).

## In-scope branches and tags

The reviewer should pin the engagement to a single candidate tag.
Reviewing `main` directly is acceptable for an interim read but does
not produce launch-evidence — only a candidate-tag review does.

| What | Pointer |
|---|---|
| Default branch | `main` |
| Most recent candidate tag | the latest `vX.Y.Z-rc.N` tag listed in `git tag --list 'v*-rc.*' --sort=-version:refname \| head -n1` |
| Annotated tag SHA + peeled commit | recorded in [`../../SECURITY.md`](../../SECURITY.md) § "Candidate-tag security evidence" for the candidate the reviewer accepts |
| Out-of-scope branches | feature branches under `opus/*`, `wave-*/*`, `phase-*/*`; the launch-evidence gate only accepts review against a tag that has reached `main` |

For the candidate the reviewer accepts, capture the annotated tag
SHA and peeled commit in the review report so future audits can
verify the review applied to the same tree the released binaries
were built from.

## Threat model summary

The threat model the reviewer is asked to assume:

1. **Adversarial MCP client.** A hostile MCP client can speak the
   protocol over `streamable_http`, `grpc`, or `stdio` (whichever
   the deployment exposes). The client may craft tool arguments,
   replay session IDs across pods, attempt resource-template
   injection on `clockify://...` URIs, and try to coerce the server
   into emitting cross-tenant data via tool-error responses.
2. **Adversarial Clockify upstream response.** The upstream
   Clockify API is treated as untrusted from the response shape
   perspective — a 4xx body could carry per-tenant identifiers.
   `CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=1` is the hosted-profile
   contract that prevents leakage; the reviewer is asked to assess
   whether the contract holds end-to-end.
3. **Pod-scope compromise.** A single pod may go rogue (privilege
   escalation in a sidecar, supply-chain regression, or a binary
   that was tampered with post-build). The reviewer is asked to
   assess what cross-tenant data a compromised pod can read or
   write, which paths involve `controlplane.Store` round trips, and
   how audit durability behaves under adversarial conditions.
4. **Cross-replica session theft.** Stolen session IDs replayed
   against a different replica must not bypass the strict
   re-authentication contract recorded in ADR 0017 (Path A).
   Reviewer is asked to confirm the strict re-auth path on
   `streamSessionManager.get`'s store fallback.
5. **Out of scope.** Physical attacks on the hosting plane; insider
   threats with database admin credentials; denial-of-service
   absent the in-process admission limits and the external gateway
   quotas under the cross-replica gate. The reviewer notes these
   as out-of-scope but flags any in-scope finding that materially
   widens these out-of-scope risks.

## Prior internal review pointers (evidence the reviewer should read)

The reviewer should treat this list as required reading before
recording findings; it is what local verifiers, internal reviews,
and the May 8 launch-readiness ledger have already produced. None of
these closes the external review gate; they are inputs the reviewer
must inspect.

| Source | What it covers |
|---|---|
| [`../../SECURITY.md`](../../SECURITY.md) | Reporting channel, response timeline, scope, security features, TLS / HTTP transport contract, release artifact verification, and the candidate-tag security evidence transcript for the most recent rc tag. |
| [`../launch-readiness-review-may-8.md`](../launch-readiness-review-may-8.md) | May 8 review-disposition ledger; the rc.3 cycle final integration audit; the open-gate rows that pin owners, evidence, and non-goals for every paid-hosted gate. |
| [`../launch-candidate-checklist.md`](../launch-candidate-checklist.md) | Group 6 candidate-tag security evidence and Group 7 release/sigstore/SLSA evidence rows; doctor strict + check-backends contract. |
| [`../runbooks/release-candidate-evidence.md`](../runbooks/release-candidate-evidence.md) | The candidate-tag evidence runbook (`make rc-evidence-plan TAG=…`, `scripts/prepare-rc-evidence.sh`); the cross-replica hosted HTTP quota proof checklist subsection. |
| [`../auth-model.md`](../auth-model.md) | The one-page auth-model summary, every supported auth mode's principal/tenant derivation, failure modes, tenant resolution, session rehydration boundaries, and the application-layer tenant scoping posture. |
| [`../adr/0017-streamable-http-session-rehydration.md`](../adr/0017-streamable-http-session-rehydration.md) | Session rehydration design (Q1 Factory contract, Q2 strict re-auth, Q3 fresh-not-persisted-claims, Q4 PreserveTTL). |
| [`../adr/0014-prod-fail-closed-defaults.md`](../adr/0014-prod-fail-closed-defaults.md) | `streamable_http` dev-DSN guard, `MCP_HTTP_LEGACY_POLICY` and `MCP_AUDIT_DURABILITY` prod defaults. |
| [`../adr/0013-private-repo-slsa-posture.md`](../adr/0013-private-repo-slsa-posture.md) | SLSA private-repo skip rationale; the cosign binary/image chain remains the mandatory cryptographic gate. |
| [`../adr/0018-risk-class-confirmation-tokens.md`](../adr/0018-risk-class-confirmation-tokens.md) (Proposed) | Risk-class enforcement and confirmation-token follow-ups; deferred until the four design questions resolve. |
| [`../runbooks/credential-leak-response.md`](../runbooks/credential-leak-response.md) | Credential-leak posture — what the maintainer rotates, when, and the evidence trail. |
| [`../runbooks/audit-durability.md`](../runbooks/audit-durability.md) | Audit durability incident response. |
| [`../runbooks/tenant-offboarding.md`](../runbooks/tenant-offboarding.md) | OIDC verify-cache TTL ceiling and tenant credential revocation. |
| Group 6 evidence transcript | Per-rc-tag transcript in [`../../SECURITY.md`](../../SECURITY.md) § "Candidate-tag security evidence": `make check`, `make verify-vuln`, `make secret-scan`, Semgrep `p/default`, `make verify-fips`, and the `nosemgrep` enumeration. |

`make license-evidence` raw inventory output is also available; per
[`brand-legal-review.md`](brand-legal-review.md) it is evidence for
counsel to review, not legal clearance, and the reviewer should
treat it as supply-chain inventory only.

## Evidence bundle the maintainer hands over

For each engagement iteration, the maintainer ships:

1. The candidate-tag SHA (annotated tag SHA + peeled commit), in
   writing, so the reviewer can verify their checkout matches the
   reviewed tree.
2. A copy of (or link to) every file under "Prior internal review
   pointers" pinned to the candidate tag.
3. The Group 6 security walk-through transcript for the candidate
   tag from [`../../SECURITY.md`](../../SECURITY.md) §
   "Candidate-tag security evidence", and the `release-smoke.yml`
   `release-smoke-doctor-output` artifact for the same tag.
4. `make license-evidence` raw inventory output run against the
   candidate-tag tree, marked clearly as "not legal advice and not
   license clearance" per
   [`brand-legal-review.md`](brand-legal-review.md).
5. The cross-replica hosted HTTP quota proof checklist state at the
   time of the engagement, even if that gate is still open — the
   reviewer needs to know which cross-replica claims are evidence-
   backed and which are still pending.
6. A reviewer-friendly threat-model brief copied from the
   "Threat model summary" section above, plus the contact channel
   for follow-up questions.

## Contact channel

Use one of the following, depending on confidentiality requirements:

- **GitHub Security Advisory** (preferred when the reviewer has a
  GitHub account):
  <https://github.com/apet97/go-clockify/security/advisories/new>.
  End-to-end encrypted with the maintainer; provides an audit
  trail.
- **Direct email to the maintainer.** Use the contact route the
  reviewer was given when the engagement was scoped — do not cite a
  public address in this packet.
- **Out-of-band signed channel** (third-party security firm under
  NDA). The signed engagement letter pins the channel; that letter
  is the artifact the maintainer references in the launch-readiness
  ledger.

The reviewer's questions should not go through public issues or
public PR comments until the engagement closes; coordinate
disclosure timing with the maintainer.

## Expected deliverable shape

For the gate to close, the reviewer's written attestation must be
archivable into [`../../SECURITY.md`](../../SECURITY.md) (or a
linked review report under `docs/security/`) and must carry **all
of** the following per the May 8 ledger's evidence-artifact line:

- **Reviewer identity** (full name, role, firm or peer affiliation),
  redacted of any private contact details before archival.
- **ISO 8601 review date.**
- **Candidate tag reviewed** (annotated tag SHA + peeled commit).
- **Scope statement** restating which of the seven scope items in
  this packet were covered, and which were explicitly out of
  scope for this engagement.
- **Findings list** (or an explicit "no findings" statement) with
  severity classification per the reviewer's house rubric, mapped
  back to the relevant `SECURITY.md` § "Scope" surface where
  possible. Every "high" or "critical" finding must include a
  reproduction sketch the maintainer can verify.
- **Release-status recommendation** in one of:
  `approve-for-paid-hosted-launch`,
  `approve-with-conditions` (conditions enumerated and tracked as
  follow-ups), or
  `do-not-approve` (with the specific blockers).
- **Pointer to the engagement record** (signed engagement letter,
  ticket, contract reference), redacted of any private contact
  details.

A `SECURITY.md` edit alone does **not** close the gate; the entry
must be paired with the launch-readiness ledger row update naming
the reviewer identity, ISO 8601 date, and recommendation. Until
both edits land, the gate stays open.

## Closure rule

This gate closes only when the maintainer archives the reviewer's
written attestation per the shape above and the launch-readiness
ledger's "Open gates" row for "Paid-hosted external security
review" is updated to quote the reviewer identity and ISO 8601
date. Local tests, candidate-tag Group 6 / Group 7 evidence, and
this request packet itself do **not** close the gate.

## Non-goal of this packet

This document is a request packet. It does not:

- Perform the review.
- Record findings.
- Approve a release.
- Grant official-product status, endorsement, partnership, or any
  legal claim.
- Replace the DPA / terms / privacy gate, the trademark gate, the
  RLS decision gate, the cross-replica quota gate, or the existing
  Group 6 / Group 7 launch-evidence gates.

Update this packet only when the request shape itself changes
(scope, threat model, evidence bundle, or contact channel). Each
review iteration's reviewer identity, date, and findings live in
[`../../SECURITY.md`](../../SECURITY.md) and the launch-readiness
ledger, not here.
