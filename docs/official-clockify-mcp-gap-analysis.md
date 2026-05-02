# Official Clockify MCP — Gap Analysis

A snapshot of where `clockify-mcp` sits on the path from
"community-grade MCP server" to "officially-supported Clockify
product." Written 2026-05-02, intended to be updated as gaps close.

This document is **not** a roadmap and **not** a checklist. It is a
narrative reading of the current state. The bound checklist lives
in [`launch-candidate-checklist.md`](launch-candidate-checklist.md).

---

## Readiness ladder

We separate three distinct readiness postures because they have
different audiences, different failure tolerances, and different
gate criteria.

### Tier 1 — Community MCP ready (✅ achieved)

Audience: developers running the binary locally or in a small
self-hosted setup, comfortable reading Go source if something
breaks. Failure tolerance: high — a flake or schema drift causes
inconvenience, not an incident.

The repo cleared this bar at v1.0.0 and has stayed there:

- Stable v1 wire format, tool names, and env-var surface.
- Three transports (stdio, streamable HTTP, opt-in gRPC).
- Four policy modes with a load-time guard against misuse.
- Signed releases (cosign + SLSA), SBOMs, FIPS variant,
  reproducibility workflow.
- Cross-transport parity matrix (`tests/parity_test.go` and
  siblings) that fails compilation on every adapter when the
  harness interface widens.
- Generator-owned docs that reject doc/config drift in CI
  (`config-doc-parity`, `catalog-drift`).
- Doctor command with strict mode (`clockify-mcp doctor --strict`).
- Documented runbooks (`docs/runbooks/`) for the operational
  scenarios that have actually happened in deployment.

### Tier 2 — Internal support alpha (✅ achieved, ⚠ caveats)

Audience: an internal team running the MCP behind a known set of
clients, with operators who will read `docs/runbooks/` when paged
and who can cycle config or roll a pod. Failure tolerance: medium
— flakes are owned and chased, but a tenant-isolation breach or
silent audit drop is unacceptable.

What earned the tier:

- Live-contract nightly with mutating + audit-phase tiers, gated
  by a sacrificial workspace; rolling `live-test-failure` issue
  is the single signal.
- Postgres-backed control-plane store
  (`internal/controlplane/postgres/`, behind `-tags=postgres`)
  with Testcontainers-driven integration tests and a separate
  hosted-deploy binary (`clockify-mcp-postgres`) gated by ADR 0001.
- Audit invariants pinned by a live test
  (`TestLiveCreateUpdateDeleteEntryAuditPhases`) that asserts
  intent + outcome rows for every non-read tool call.
- Forward-auth, OIDC, JWKS, EC JWK auth modes; recent hardening
  wave closed eight findings (control-byte rejection, panic
  recovery, response-size limits, HSTS, tenant validation).
- `ENVIRONMENT=prod` flips the audit durability default to
  `fail_closed` and the legacy-HTTP policy to `deny`; locked by
  `internal/config/prod_defaults_test.go`.
- Deployment profile docs under `docs/deploy/` that map a use
  case to a config preset and a verification command.
- `sessionAffinity: ClientIP` band-aid on the Helm/k8s Service
  templates with a 24h timeout, addressing the most common
  multi-replica session-loss case.

Caveats that the tier carries today:

- ADR `0017-streamable-http-session-rehydration.md` is **Proposed**,
  not implemented. The band-aid covers the common case but not
  shared-NAT egress, pod eviction, rolling upgrade, or cross-AZ
  failover.
- The shared-service profile is documented but only fragmentally
  exercised end-to-end (Postgres store unit + integration via
  Testcontainers, plus the live audit-phase test). There is no
  test that boots the Postgres-tagged binary, drives multi-tenant
  traffic over the streamable-HTTP transport, and asserts tenant
  isolation through the full stack.
- Live-contract is fail-soft on missing secrets: a fresh fork
  reports green nightlies because the test steps gate on `if:`. A
  green nightly badge does not by itself prove the live tests
  ran. The maintainer reads the warning annotations.

### Tier 3 — Official Clockify product launch (⛔ not yet)

Audience: any external customer, any deployment Clockify itself
links to or supports through its support channels, any embedding
in a Clockify-branded product surface. Failure tolerance: low — a
schema drift, an unauthenticated tool call leaking another
tenant's data, or an unrecoverable session loss is a P0.

What is missing for tier 3 is intentionally narrow:

1. **Live contract is intermittently red and the rolling issue is
   open.** Every promotion to launch candidate must start from
   two consecutive green nightly runs with the mutating + audit
   tiers enabled. Today the loop is short of that bar.
2. **Shared-service Postgres E2E does not exist as a single
   green-or-red test.** The pieces are there but no CI gate fails
   when the integration of those pieces breaks. This is the
   biggest single gap on the launch-candidate checklist.
3. **ADR 0017 is unresolved.** Either we ship the rehydration fix
   and gate it with a multi-replica integration test, or we
   formally document the single-replica posture and pin
   `replicaCount: 1` in the chart's default. Today neither has
   happened, so the operational story is "it usually works."
4. **Auth-model docs are scattered across multiple docs.** A
   reviewer cannot answer "what does auth look like?" in five
   minutes without reading ADRs and grepping the codebase.
5. **Product launch docs are not verified end-to-end.** Some
   profile docs do not name a verification command; some
   reference older flag names; the README's tool-count claim
   floats. A `make doc-parity` pass plus a docs review will catch
   these.
6. **Bench baseline check has not been re-run on the candidate
   shape.** Recent perf wave (cached tools/list, tier-2
   descriptor cache, schema compaction) needs the baseline
   refreshed and the regression threshold reaffirmed before any
   tag claims launch quality.

---

## What is already strong

- **Cross-transport parity discipline.** The harness `Transport`
  interface in `tests/harness/harness.go` is the single throat for
  every transport adapter. Adding a method there fails compilation
  on stdio, legacy HTTP, streamable HTTP, and gRPC simultaneously.
  This is unusually rigorous for an MCP server.
- **Generator-owned docs.** `cmd/gen-config-docs` plus
  `make gen-tool-catalog` mean every config knob and every tool
  descriptor lands in three places (help text, README table, tool
  catalog) atomically or not at all. CI rejects partial updates.
- **Two-binary discipline.** ADR 0001 keeps `pgx` out of the
  default `go.mod`. The default binary is stdlib-only and
  deliberately fails the strict-backend doctor check; the
  Postgres binary is the *only* artefact that satisfies the
  hosted-deploy gate. This makes the supply chain story crisp.
- **Audit pipeline is end-to-end testable against live Clockify.**
  `TestLiveCreateUpdateDeleteEntryAuditPhases` exercises real
  Postgres + real Clockify and asserts both intent and outcome
  rows. Most MCP servers do not have anything close to this.
- **Policy enforcement is gate-first, not handler-first.** A
  policy regression that lets a write through under
  `time_tracking_safe` would be caught by
  `TestLivePolicyTimeTrackingSafeBlocksProjectCreate` before the
  Clockify upstream ever sees the request.
- **Production defaults are environment-aware.** `ENVIRONMENT=prod`
  flips legacy-HTTP policy to `deny` and audit durability to
  `fail_closed` automatically; explicit values still win.
- **Release artefacts are reviewable.** Signed binaries, FIPS
  variant, SBOM, SLSA attestations, and a `release-smoke.yml`
  workflow that exercises every artefact.

---

## Blockers for official Clockify product launch

In priority order — closing the lower-numbered ones first
unblocks the next.

1. **Live contract failures (current).**
   *Where:* `.github/workflows/live-contract.yml` and the rolling
   `live-test-failure` issue.
   *Why blocking:* the launch-candidate definition starts with two
   consecutive green nightlies. Until the rolling issue is
   triaged and the failure mode is either fixed or quarantined
   with a known-cause note, the candidate clock has not started.

2. **Shared-service Postgres E2E.**
   *Where:* missing entirely as a CI-gated test. Pieces exist in
   `internal/controlplane/postgres/` and
   `tests/e2e_live_mcp_test.go::TestLiveCreateUpdateDeleteEntryAuditPhases`.
   *Why blocking:* the shared-service profile is the headline
   deployment shape for an official product. Without an E2E
   test, every regression in the wiring (config → store →
   transport → audit) is shipped silently.

3. **ADR 0017 resolution.**
   *Where:* `docs/adr/0017-streamable-http-session-rehydration.md`.
   *Why blocking:* either the rehydration fix lands and is proven
   by a multi-replica E2E, or the single-replica limitation is
   documented and pinned. The current "Proposed + band-aid"
   posture is not officially defensible.

4. **Auth-model docs consolidation.**
   *Where:* `docs/production-readiness.md` plus a possible new
   `docs/auth-model.md`.
   *Why blocking:* an external reviewer cannot map a Clockify auth
   requirement to an MCP config in under five minutes. A
   one-page summary closes that.

5. **Product launch docs verification.**
   *Where:* `README.md` claims, `docs/clients.md`,
   `docs/support-matrix.md`, every `docs/deploy/profile-*.md`.
   *Why blocking:* docs that diverge from behaviour erode trust
   the moment a customer notices.

6. **Bench baseline refresh.**
   *Where:* `make bench-baseline-check` against the candidate
   shape post-perf wave.
   *Why blocking:* a launch claim that includes "low overhead"
   needs a baseline that reflects the current code. Today the
   baseline is from before the perf wave.

7. **Security review walk-through.**
   *Where:* `make verify-vuln`, `gitleaks`, `semgrep`,
   `make verify-fips` — each green on the candidate tag with the
   findings filed in `SECURITY.md` if any.
   *Why blocking:* this is mostly mechanical but cannot be
   skipped.

---

## What "fixing" each blocker looks like

This section is intentionally short — it points at where the
work happens, not how. The agent slash commands
(`/fix-live-contract`, `/postgres-e2e`, `/session-rehydration`,
`/launch-candidate`) drive the actual sequencing.

| Blocker | First file an agent should open | Smallest verifiable green |
|---|---|---|
| 1. Live contract | `.github/workflows/live-contract.yml`, the rolling `live-test-failure` issue, `tests/e2e_live_test.go`, `tests/e2e_live_mcp_test.go` | One green nightly run with mutating tier on. |
| 2. Shared-service Postgres E2E | `tests/e2e_live_mcp_test.go`, `internal/controlplane/postgres/`, `docs/deploy/production-profile-shared-service.md` | A new test in `tests/` that boots `clockify-mcp-postgres` + Postgres and exercises ≥2 tenants. |
| 3. ADR 0017 | `docs/adr/0017-streamable-http-session-rehydration.md`, `internal/mcp/transport_streamable_http.go`, Helm chart `replicaCount` | Either a multi-replica E2E or a doc PR that pins `replicaCount: 1` and documents the limit. |
| 4. Auth-model docs | `docs/production-readiness.md`, `internal/authn/`, `internal/mcp/transport_http_authmatrix_test.go` | A one-page auth-model summary linked from `README.md`. |
| 5. Launch docs | `README.md`, `docs/clients.md`, `docs/support-matrix.md`, `docs/deploy/profile-*.md` | `make doc-parity` green and a manual review pass. |
| 6. Bench baseline | `.bench/`, `bench.yml` workflow, `Makefile` `verify-bench` target | Updated baseline file committed; `make bench-baseline-check` green. |
| 7. Security review | `make verify-vuln`, `make verify-fips`, `.gitleaks.toml`, `SECURITY.md` | All four green on the candidate tag, findings filed. |

---

## Update protocol

When a blocker is closed:

1. Move it from the "Blockers" section to the matching tier's "What
   earned the tier" list.
2. Tick the relevant boxes in
   [`launch-candidate-checklist.md`](launch-candidate-checklist.md).
3. Reference the merge commit in `CHANGELOG.md` Unreleased.
4. If the close changes the readiness tier, update the tier
   header from `(✅ achieved, ⚠ caveats)` to `(✅ achieved)` and
   move the caveat into "What earned the tier."
