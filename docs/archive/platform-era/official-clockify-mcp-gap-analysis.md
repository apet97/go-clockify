# Clockify MCP Official-Launch Gap Analysis

> **Historical artifact. Not current one-user MCP product documentation.**
> Preserved for platform-era audit/history only. Start current one-user work from `README.md`, `docs/agent-cookbook.md`, `docs/tool-catalog.md`, and `docs/goals/oneuser-tool-coverage.md`.


A snapshot of where `clockify-mcp` sits on the path from
"community-grade MCP server" to a Clockify-supported product launch.
Written 2026-05-02; last updated 2026-05-03 after
PR #62's invite-user route probe and the current 128-tool catalog
cleanup.

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
- Five policy modes with a load-time guard against misuse.
- Signed releases (cosign mandatory, SLSA when GitHub artifact
  attestations are available), SBOMs, FIPS variant, reproducibility
  workflow.
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
  The local `make test-postgres` gate is also self-contained:
  under `-tags=postgres,integration`, these E2Es reuse the package
  Testcontainers DSN when `MCP_LIVE_CONTROL_PLANE_DSN` is unset,
  and the Makefile normalizes Unix Docker sockets for Colima /
  Docker Desktop.
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
  (`disabled, bearer, jwt`) is fixed in the same wave; the
  `forward_auth` boundary now rejects duplicated identity headers
  and principal values larger than 1024 bytes before sanitization,
  closing the earlier box-3 deferral. Closed Group 4 of the
  launch-candidate checklist (commits 0bcd30b + 8a627d6 + 222c206
  plus the 2026-05-02 forward-auth hardening pass).
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
- Read-side schema drift is now mechanically checked by
  `tests/e2e_live_schema_test.go::TestLiveReadSideSchemaDiff`,
  which compares raw Clockify JSON field sets against the
  `internal/clockify` structs. Scheduled run evidence is archived in
  `live-contract.yml` runs 25608259477 and 25607242862 on
  `feef83c641ced93d2ab6ba07ef766d61c82cc703`.

### Tier 3 — Clockify-supported product launch (⛔ not yet)

Audience: any external customer, any deployment Clockify itself
links to or supports through its support channels, any embedding
in a Clockify-branded product surface. Failure tolerance: low — a
schema drift, an unauthenticated tool call leaking another
tenant's data, or an unrecoverable session loss is a P0.

What is missing for tier 3 is intentionally narrow:

1. ~~**Scheduled live-contract evidence is still missing.**~~
   **Closed 2026-05-09** by consecutive scheduled runs 25608259477
   and 25607242862 on
   `feef83c641ced93d2ab6ba07ef766d61c82cc703`, with the mutating,
   MCP-path safety, audit, and `TestLiveReadSideSchemaDiff` tiers
   captured. The rolling `live-test-failure` issues remain closed.
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
4. ~~**Auth-model docs are scattered across multiple docs.**~~
   **Closed 2026-05-02** by `docs/auth-model.md` and the
   operator-doc cross-links; a reviewer can now answer the auth
   model questions from one page with test pins.
5. ~~**Product launch docs are not verified end-to-end.**~~
   **Closed 2026-05-02** by the launch-doc verification pass:
   client support now names tested transport/auth combinations and
   flags untested combos, the support matrix names Go/OS/FIPS/kernel
   posture, every deployment profile ends with an explicit
   verification section, and the docs parity gates are the recorded
   local proof.
6. ~~**Bench baseline check has not been re-run on the candidate
   shape.**~~ **Closed 2026-05-02 on main by PR #51.** The committed
   linux/amd64 baseline was refreshed from `Bench` workflow run
   25255062599 (`bench-current-25255062599`) after the cached
   tools/list, Tier 2 descriptor cache, and schema compaction
   wave. `make bench-baseline-check` validates the committed
   artifact shape, and follow-up `Bench` workflow run 25255216987
   passed the linux/amd64 regression comparison.

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
- **Product launch docs are operator-verifiable.** The publishable
  surface now has one place for client compatibility, one support
  matrix for Go / OS / FIPS / kernel posture, and a "How to verify
  this deployment" section at the end of every deployment-profile
  doc. Non-hosted profiles explicitly say that `doctor --strict` is
  a negative hosted-posture check, so operators do not mistake exit
  3 for a broken local or small-team install.
- **Security-review local preflight is clean on the current tree.**
  `govulncheck@v1.3.0` is now a tagged scanner release and is green
  after the repo's Go pin moved to 1.25.10; gitleaks, Semgrep
  (`p/default`, metrics off), and the local FIPS build-tag check are
  also green. The only Semgrep suppressions are scoped to
  streamable-HTTP SSE frame writes and are justified both in code and
  ADR 0017. The production `MCP_ALLOW_DEV_BACKEND=1` rejection now
  has a dedicated regression test. This is not candidate-tag evidence;
  the same walk-through must be repeated after `vX.Y.Z-rc.N` is cut.
- **API coverage matrix.** [`docs/api-coverage.md`](api-coverage.md)
  maps all 128 MCP tools to their Clockify API endpoints or workflow
  composition paths, classifies
  each tool by read-only/mutating/destructive/billing/admin risk, and
  lists the current unit/integration/live-test coverage per tool. PR
  #59 extended the manual sacrificial-workspace suite so every
  then-current catalog tool name was live-probed through the MCP path.
  The two later timesheet workflow helpers are covered by unit tests
  and live-test hooks, not by launch evidence. The matrix separates
  full success paths from explicit upstream constraints such as
  unsupported 405 routes, permission/plan gates, and workspace-state
  caps. Remaining gaps are also explicit: mutating request-schema drift
  is not automated, the exhaustive probes are not cron launch evidence,
  and some destructive tools can only assert a minimal dry-run envelope
  or upstream 4xx because Clockify exposes no safe preview route. The
  evidence hierarchy (scheduled workflow > manual dispatch > local
  with env vars > local without env vars as non-evidence) is documented
  there. PR #62 also pins Clockify's documented invite-user route
  even though no dedicated catalog tool is exposed: the probe uses
  `send-email=false` plus an empty email and asserts validation,
  permission, plan, or unsupported-method refusal without creating a
  pending user or sending mail. The risk override table now has a
  descriptor-backed guard so stale planned tool names cannot linger.
  The two agent-facing timesheet workflow helpers added after PR #62
  turn that raw coverage into end-user actions: review a timesheet and
  safely fill one validated gap.
- **Benchmark baseline is current for the candidate shape.** The
  committed `internal/benchdata/baseline.txt` was refreshed from the
  `Bench` workflow bootstrap artifact `bench-current-25255062599`
  on 2026-05-02 after the cached tools/list, Tier 2 descriptor cache,
  and schema compaction wave. `make bench-baseline-check` validates
  that the baseline remains linux/amd64, covers every workflow
  package, and has the configured 10-sample floor. Follow-up
  `Bench` workflow run 25255216987 passed the linux/amd64 regression
  comparison against the refreshed baseline.

---

## Blockers for Clockify-supported product launch

In priority order — closing the lower-numbered ones first
unblocks the next.

> **Scope note (2026-05-10).** The blockers in this section gate the
> hypothetical **Clockify-supported / paid-hosted / "official Clockify"**
> launch path only. The community/self-hosted track is closed at
> `v1.2.1` (released 2026-05-10 from rc.3 peeled commit `ce56414`; see
> [`docs/runbooks/release-candidate-evidence.md`](runbooks/release-candidate-evidence.md)
> § "v1.2.1 release evidence record (2026-05-10)" for the canonical
> evidence anchor). The Tier-3 / "Clockify-supported product launch"
> blockers itemised below are the same five paid-hosted / commercial
> follow-ups deferred per
> [`docs/launch-readiness-review-may-8.md`](launch-readiness-review-may-8.md)
> § "Deferred paid-hosted/commercial follow-ups — not required for
> community/self-hosted v1.2.1": paid-hosted external security review,
> DPA / terms / privacy posture, trademark / "official Clockify"
> language plus `clockify://` URI / gRPC service-name branding,
> P1-8 paid-commercial RLS decision, and cross-replica hosted HTTP
> quotas. They do **not** block the community/self-hosted release.

The remaining blockers are not local test failures. They are still
launch blockers for the paid-hosted / commercial / "official
Clockify" track: Group 6 candidate-tag security walk-through evidence,
Group 7 release/sigstore/SLSA evidence, pushed workflow first-run
evidence where still missing, repository-state cleanup,
public-readiness disposition, hosted/platform evidence, and
legal/product approval for any Clockify-supported product launch claim.
Local checks are useful but not sufficient for a Clockify-supported
product launch claim. Group 1 scheduled live-contract cron greens are
now archived on `feef83c641ced93d2ab6ba07ef766d61c82cc703`; the PR #59
through PR #62 manual live-probe work remains coverage evidence only.
Group 6/7 candidate-tag evidence + repository description + issue #28
all closed during the rc.3 cycle (PRs #84, #85, #77, #88) and the
v1.2.1 final release on 2026-05-10; the items still outstanding
are the five paid-hosted/commercial gates listed in the scope note
above.

1. ~~**Scheduled live-contract cron evidence.**~~ **Closed 2026-05-09.**
   *Where:* `.github/workflows/live-contract.yml` and the rolling
   `live-test-failure` issue.
   *Evidence:* scheduled runs 25608259477 and 25607242862 are
   consecutive greens on
   `feef83c641ced93d2ab6ba07ef766d61c82cc703`; their logs include
   the read-only/schema-diff, mutating, MCP-path safety, and
   audit-phase tiers. The temporary launch-evidence cron was removed
   after these runs were archived.

2. ~~**Shared-service Postgres E2E.**~~ **Closed 2026-05-02**
   (commits 42502cf + 79f0769 plus the local `make test-postgres`
   self-containment pass). See Tier 2 "What earned the tier" for
   the test name, Make targets, and CI job name.

3. ~~**ADR 0017 resolution.**~~ **Closed 2026-05-02** (commits
   eb5351c + 8353934 + fcfd7f0 + 5e566e8). See Tier 2 "What
   earned the tier" for the test name, the Make-target update,
   and the CI job that gates the cross-instance rehydration
   contract per-PR.

4. ~~**Auth-model docs consolidation.**~~ **Closed 2026-05-02**
   (commits 0bcd30b + 8a627d6 + 222c206). See Tier 2 "What
   earned the tier" for the new `docs/auth-model.md` anchor and
   the operator-doc cross-links.

5. ~~**Product launch docs verification.**~~ **Closed 2026-05-02.**
   `README.md` claims are covered by `make doc-parity`,
   `docs/clients.md` now names exact tested transport/auth
   combinations plus untested combos, `docs/support-matrix.md`
   names Go/OS/FIPS/kernel posture, and every deployment profile
   ends with a verification section.

6. ~~**Bench baseline refresh.**~~ **Closed 2026-05-02 on main by
   PR #51.** `internal/benchdata/baseline.txt` now comes from
   `Bench` workflow run 25255062599 (`bench-current-25255062599`)
   and `make bench-baseline-check` is green locally; follow-up
   `Bench` workflow run 25255216987 passed the linux/amd64 regression
   comparison. The default `make verify-bench` comparison is
   intentionally platform-guarded on macOS/arm64 workstations.

7. **Candidate-tag security walk-through.** `make verify-vuln`
   (with tagged `govulncheck@v1.3.0` on Go 1.25.10), gitleaks,
   Semgrep, `make verify-fips`, and `make check` were green on the
   May 8 remediation tree. The blocker closes only when the same
   walk-through is repeated on the final candidate tag and any
   findings are filed in `SECURITY.md`.

8. **Release/sigstore/SLSA evidence.** Cut `vX.Y.Z-rc.N`, run
   `release-smoke.yml`, verify sigstore bundles, SLSA attestations,
   SBOMs, Docker image signature, and archive reference
   `doctor --strict` outputs.

---

## What "fixing" each blocker looks like

This section is intentionally short — it points at where the
work happens, not how. The active agent handoff lives in
[`agent-handoff.md`](agent-handoff.md); the PR #51
[`claude-code-continuation.md`](claude-code-continuation.md) packet is
historical and must not be used as a current prompt source. The
historical `/postgres-e2e` and `/session-rehydration` local commands
are retained only for regressions in those closed areas.

| Blocker | First file an agent should open | Smallest verifiable green |
|---|---|---|
| 1. Live contract | `.github/workflows/live-contract.yml`, the rolling `live-test-failure` issue, `tests/e2e_live_test.go`, `tests/e2e_live_mcp_test.go`, `tests/e2e_live_schema_test.go` | One green nightly run with mutating tier and read-side schema diff on. |
| 2. ~~Shared-service Postgres E2E~~ | _closed 2026-05-02_ — `internal/controlplane/postgres/e2e_shared_service_test.go`, `make shared-service-e2e`, `make test-postgres`, `Shared-service Postgres E2E` job in `ci.yml` | Done. |
| 3. ~~ADR 0017~~ | _closed 2026-05-02_ — `internal/controlplane/postgres/e2e_session_rehydration_test.go`, `streamSessionManager.get` + `Server.MarkInitialized` in `internal/mcp/`, ADR doc moved to Accepted | Done (Path A). |
| 4. ~~Auth-model docs~~ | _closed 2026-05-02_ — `docs/auth-model.md` (new), `docs/production-readiness.md` "Pick an auth mode" + `docs/runbooks/auth-failures.md` cross-links | Done. |
| 5. ~~Launch docs~~ | _closed 2026-05-02_ — `README.md`, `docs/clients.md`, `docs/support-matrix.md`, `docs/deploy/profile-*.md` | Done; `make doc-parity` plus manual review of client/profile/support docs. |
| 6. ~~Bench baseline~~ | _closed 2026-05-02 on main by PR #51_ — `internal/benchdata/baseline.txt`, `bench.yml` workflow runs 25255062599 + 25255216987, `make bench-baseline-check` | Done. |
| 7. Candidate-tag security | `SECURITY.md`, `make verify-vuln`, gitleaks, Semgrep, `make verify-fips` | Same suite green on `vX.Y.Z-rc.N`, with findings or "no findings" recorded. |
| 8. Release/sigstore/SLSA | `docs/release-policy.md`, `docs/verification.md`, `.github/workflows/release-smoke.yml` | Candidate-tag `release-smoke.yml` green; sigstore/SLSA/SBOM and reference-doctor evidence archived. |

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
