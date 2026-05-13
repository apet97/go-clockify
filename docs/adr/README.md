# Architecture Decision Records

This directory captures the load-bearing architectural decisions
behind `clockify-mcp` in [MADR 3.0][madr] format. ADRs answer "why"
for choices a future contributor would otherwise have to reconstruct
from `git log` and inline code comments.

[madr]: https://adr.github.io/madr/

## Index

| ADR | Title | File |
|-----|-------|------|
| 0001 | Stdlib-only default build | [0001-stdlib-only-default-build.md](0001-stdlib-only-default-build.md) |
| 0002 | Transport selection | [0002-transport-selection.md](0002-transport-selection.md) |
| 0003 | Auth mode negotiation | [0003-auth-mode-negotiation.md](0003-auth-mode-negotiation.md) |
| 0004 | Policy enforcement architecture | [0004-policy-enforcement-architecture.md](0004-policy-enforcement-architecture.md) |
| 0005 | Tool tier activation | [0005-tool-tier-activation.md](0005-tool-tier-activation.md) |
| 0006 | OpenTelemetry tracing via build tag | [0006-otel-build-tag.md](0006-otel-build-tag.md) |
| 0007 | FIPS 140-3 build via build tag | [0007-fips-build-tag.md](0007-fips-build-tag.md) |
| 0008 | gRPC auth via stream interceptor | [0008-grpc-auth-interceptor.md](0008-grpc-auth-interceptor.md) |
| 0009 | Resource delta-sync subscriptions | [0009-resource-delta-sync.md](0009-resource-delta-sync.md) |
| 0010 | Metrics stack direction | [0010-metrics-stack-direction.md](0010-metrics-stack-direction.md) |
| 0011 | Control-plane schema versioning | [0011-controlplane-schema-versioning.md](0011-controlplane-schema-versioning.md) |
| 0012 | Backward-compatibility policy | [0012-backward-compatibility-policy.md](0012-backward-compatibility-policy.md) |
| 0013 | Private-repo SLSA posture | [0013-private-repo-slsa-posture.md](0013-private-repo-slsa-posture.md) |
| 0014 | Production fail-closed defaults | [0014-prod-fail-closed-defaults.md](0014-prod-fail-closed-defaults.md) |
| 0015 | Profile-centric configuration model | [0015-profile-centric-configuration.md](0015-profile-centric-configuration.md) |
| 0016 | Single-maintainer governance reality | [0016-single-maintainer-governance.md](0016-single-maintainer-governance.md) |
| 0017 | Streamable-HTTP session rehydration | [0017-streamable-http-session-rehydration.md](0017-streamable-http-session-rehydration.md) |
| 0018 | Risk-class enforcement and confirmation tokens | [0018-risk-class-confirmation-tokens.md](0018-risk-class-confirmation-tokens.md) |
| 0019 | Postgres row-level security for the paid-hosted plane (proposed) | [0019-paid-commercial-rls-decision.md](0019-paid-commercial-rls-decision.md) |
| 0021 | Hosted tenant policy ceiling | [0021-hosted-tenant-policy-ceiling.md](0021-hosted-tenant-policy-ceiling.md) |
| 0022 | Batched audit_events INSERT for non-strict outcomes | [0022-batched-audit-outcome-inserts.md](0022-batched-audit-outcome-inserts.md) |
| 0023 | Cross-transport MaxInFlightToolCalls parity (proposed) | [0023-cross-transport-inflight-parity.md](0023-cross-transport-inflight-parity.md) |
| 0024 | Per-tenant aggregate rate-limit (proposed) | [0024-per-tenant-aggregate-rate-limit.md](0024-per-tenant-aggregate-rate-limit.md) |
| 0025 | Upstream-5xx circuit breaker against Clockify | [0025-upstream-clockify-circuit-breaker.md](0025-upstream-clockify-circuit-breaker.md) |

ADRs 0001–0018 are **Accepted**. ADR 0010 was promoted from Proposed
during the May 8 launch-readiness remediation because the v1.x line has
already shipped on its homegrown-metrics decision. ADR 0018 was
implemented on the `adr0018-confirmation-tokens` branch: the gate now
runs in `enforcement.Pipeline.BeforeCall` for every high-risk tool
call, with HMAC tokens issued during dry-run preview and the
implementation documented in the ADR's "Status" section. ADR 0019
remains **Proposed**: it is the template for the paid-commercial
Postgres row-level-security decision tracked in
[`../launch-readiness-review-may-8.md`](../launch-readiness-review-may-8.md)
§ "P1-8 paid-commercial RLS decision" — its decision-maker, GUC name,
and migration ordering land before it moves to Accepted.
0013 is active again as of 2026-05-07 because GitHub artifact attestations remain
unavailable for this user-owned private repository; its skip path
applies only to that platform gate.

New ADRs should follow the MADR 3.0 template (status / context /
decision / consequences / alternatives / references) used by the
existing files.

## Numbering translation

Several inline code comments still refer to ADRs by their old
informal numbers (e.g. `// See ADR 009`). Those comments were
written before this directory existed and were never normalised.
The mapping is:

| Old inline number | New ADR | Why renumbered |
|-------------------|---------|----------------|
| ADR 001 | [0001](0001-stdlib-only-default-build.md) | Same decision, zero-padded to 4 digits. |
| ADR 009 | [0006](0006-otel-build-tag.md) | The old numbering had gaps reserved for unwritten ADRs; this directory closes the gaps. |
| ADR 011 | [0007](0007-fips-build-tag.md) | Same decision, renumbered for canonical ordering. |
| ADR 012 | [0008](0008-grpc-auth-interceptor.md) | Same decision, renumbered for canonical ordering. |
| ADR 013 | [0009](0009-resource-delta-sync.md) | Same decision, renumbered for canonical ordering. |

The inline comments will keep working — every renumbered ADR carries
a "Previously referred to as ADR NNN" line under **References** so
`grep ADR` continues to lead a contributor to the right document.
Future code comments should reference the canonical 4-digit number
(e.g. `// See ADR 0009`).

## When to write a new ADR

Write an ADR when a decision is:

1. **Load-bearing** — a future change would require reverting the
   decision rather than just editing code.
2. **Non-obvious** — the rationale is not visible from the code
   alone; a reviewer would need to read commit messages or ask the
   maintainer to understand it.
3. **Cross-cutting** — the decision touches multiple packages,
   configuration surfaces, or operator-visible behaviour.

Bug fixes, refactors, and small feature additions do not need ADRs.
The bar is "would a contributor revisiting this in twelve months
want to know why?" — if yes, write an ADR.

## Process

1. Copy an existing ADR as a template.
2. Pick the next number in the sequence and a short slug.
3. Fill in **Context**, **Decision**, **Consequences**,
   **Alternatives considered**, and **References** sections.
4. Add the new entry to the index table above.
5. Open a PR with the `docs(adr):` commit prefix.
6. Reference the ADR from any code comment that needs to point at
   the rationale.

ADRs are factual artefacts of decisions that have already shipped or
are about to ship. They are not RFCs; they describe what is, not
what might be. If a decision is being debated, capture it in a local
planning queue until the decision is made, then write the ADR.
