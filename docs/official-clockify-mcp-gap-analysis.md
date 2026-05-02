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
- Shared-service end-to-end coverage:
  `internal/controlplane/postgres/e2e_shared_service_test.go`
  (`make shared-service-e2e`, also wired as the
  `Shared-service Postgres E2E` job in
  `.github/workflows/ci.yml`) boots `mcp.ServeStreamableHTTP`
  in-process against the Postgres-backed control plane with two
  distinct `forward_auth` principals (one operator persona on
  `policy_mode=standard`, one AI-facing persona on
  `policy_mode=time_tracking_safe`) and asserts tenant
  isolation in `audit_events` + `sessions`, the cross-tenant
  negative (zero rows for `tenant_id=A AND session_id=B`), and
  per-tenant policy-mode enforcement. Closed Group 2 of the
  launch-candidate checklist (commits 42502cf + 79f0769;
  first CI green: ci.yml run 25240007056 on 2026-05-02).
  **Promoted to required-status check on `main` on 2026-05-02**
  after three consecutive green runs (25240007056, 25240085916,
  25240163213); the snapshot in
  [`docs/branch-protection.md`](branch-protection.md) is the
  audit trail.
- **Auth-model docs consolidation shipped (Group 4).** New
  one-page reviewer-facing summary at
  [`docs/auth-model.md`](auth-model.md) covers all four inbound
  auth modes (`static_bearer`, `oidc`, `forward_auth`, `mtls`),
  the Principal-construction contract, tenant resolution rules,
  failure modes with HTTP/gRPC status mapping, an end-to-end
  test-pin table, the three-layer auth diagram (inbound · upstream
  · gRPC re-auth), and a five-question reviewer self-quiz with
  answers. Cross-linked from `docs/production-readiness.md` "Pick
  an auth mode" and from `docs/runbooks/auth-failures.md`. The
  Group 4 checklist's mode-naming bug
  (`disabled, bearer, jwt`) is fixed in the same wave; box 3
  (forward_auth duplicated/oversized header pins) is honestly
  downgraded with a sub-bullet citing net/http's
  `Server.MaxHeaderBytes` cap as the server-side bound for the
  oversized-header risk. Closed Group 4 of the launch-candidate
  checklist (commits 0bcd30b + 8a627d6 + 222c206 on 2026-05-02).
- **Streamable-HTTP cross-pod session rehydration shipped
  (ADR 0017 Path A).** `streamSessionManager.get` consults the
  shared `controlplane.Store` on a local miss, strict-validates
  the freshly-authenticated principal against the persisted
  Subject/TenantID, and rebuilds the per-tenant runtime via the
  existing principal-aware Factory. The persisted CreatedAt /
  ExpiresAt / LastSeenAt are preserved (no fresh TTL); the
  rebuilt `mcp.Server` is pre-marked initialized with the
  persisted ProtocolVersion + ClientName + ClientVersion via the
  new `Server.MarkInitialized` setter. Pinned by
  `TestStreamableHTTPCrossInstanceRehydration` in
  `internal/controlplane/postgres/e2e_session_rehydration_test.go`,
  which boots two listeners against the same Postgres store and
  asserts the cross-instance happy path, cross-tenant 403, and
  expired-session 404 + row removal. Runs in the existing
  `Shared-service Postgres E2E` CI job (test pattern extended
  in the same wave). The `sessionAffinity: ClientIP` band-aid
  stays as defence-in-depth + perf optimisation — correctness
  no longer depends on it. Closed Group 3 of the launch-candidate
  checklist (commits eb5351c + 8353934 + fcfd7f0 + 5e566e8 on
  2026-05-02).

Caveats that the tier carries today:

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
2. ~~**Shared-service Postgres E2E does not exist as a single
   green-or-red test.**~~ **Closed 2026-05-02** by commits
   42502cf + 79f0769. The
   `Shared-service Postgres E2E` job in `.github/workflows/ci.yml`
   went green on its first run (ci.yml run 25240007056) and
   gates per-PR.
3. ~~**ADR 0017 is unresolved.**~~ **Closed 2026-05-02** by
   commits eb5351c (failing-first cross-instance E2E) + 8353934
   (`streamSessionManager.get` store fallback +
   `Server.MarkInitialized` setter) + fcfd7f0 (ADR moved to
   Accepted with Q1=A, Q2=Strict, Q3=Fresh, Q4=PreserveTTL) +
   5e566e8 (clients.md + production-readiness.md document the
   rehydration boundaries). The shipped fix is Path A
   (implement); Path B (single-replica documentation) is not
   taken. Pinned by `TestStreamableHTTPCrossInstanceRehydration`
   under the `Shared-service Postgres E2E` CI job.
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

2. ~~**Shared-service Postgres E2E.**~~ **Closed 2026-05-02**
   (commits 42502cf + 79f0769). See Tier 2 "What earned the
   tier" for the test name, Make target, and CI job name.

3. ~~**ADR 0017 resolution.**~~ **Closed 2026-05-02** (commits
   eb5351c + 8353934 + fcfd7f0 + 5e566e8). See Tier 2 "What
   earned the tier" for the test name, the Make-target update,
   and the CI job that gates the cross-instance rehydration
   contract per-PR.

4. ~~**Auth-model docs consolidation.**~~ **Closed 2026-05-02**
   (commits 0bcd30b + 8a627d6 + 222c206). See Tier 2 "What
   earned the tier" for the new `docs/auth-model.md` anchor and
   the operator-doc cross-links.

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
| 2. ~~Shared-service Postgres E2E~~ | _closed 2026-05-02_ — `internal/controlplane/postgres/e2e_shared_service_test.go`, `make shared-service-e2e`, `Shared-service Postgres E2E` job in `ci.yml` | Done. |
| 3. ~~ADR 0017~~ | _closed 2026-05-02_ — `internal/controlplane/postgres/e2e_session_rehydration_test.go`, `streamSessionManager.get` + `Server.MarkInitialized` in `internal/mcp/`, ADR doc moved to Accepted | Done (Path A). |
| 4. ~~Auth-model docs~~ | _closed 2026-05-02_ — `docs/auth-model.md` (new), `docs/production-readiness.md` "Pick an auth mode" + `docs/runbooks/auth-failures.md` cross-links | Done. |
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
