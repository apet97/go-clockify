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
| 0010 | Metrics stack direction (proposed) | [0010-metrics-stack-direction.md](0010-metrics-stack-direction.md) |

ADRs 0001–0009 are **Accepted**. 0010 is **Proposed** — it captures
the design surface around whether to keep the homegrown metrics
facade or move to OpenTelemetry metrics; no code change yet.

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
what might be. If a decision is being debated, capture it in
`.planning/` until the decision is made, then write the ADR.
