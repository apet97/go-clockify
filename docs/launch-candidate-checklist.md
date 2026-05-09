# Launch Candidate Checklist

The pre-flight gate to take `clockify-mcp` from "community MCP
ready / internal-support alpha" to a launch-candidate package for
official-product review. This is **additive** to the existing
[`deploy-readiness-checklist.md`](release/deploy-readiness-checklist.md)
and [`public-hosted-launch-checklist.md`](release/public-hosted-launch-checklist.md);
those checklists govern any deploy. This one governs the *promotion
review for the project itself*; it does not grant official-product
status or legal/product approval by itself.

A box gets ticked only when it is reproducibly green from a clean
checkout. "Worked once" is not green.

> **Strict agent rule.** Do not declare "launch candidate" until
> every group below is ticked **and** the group-level definition of
> done is satisfied. The binding constraints live in the
> tracked [`AGENTS.md`](../AGENTS.md) at the repo root. A local
> workstation `CLAUDE.md` may exist, but it is gitignored context,
> not the binding source of truth.

---

## 1. Live API contract

The nightly **Live contract** workflow
(`.github/workflows/live-contract.yml`) drives this section.

- [x] `CLOCKIFY_LIVE_API_KEY` and `CLOCKIFY_LIVE_WORKSPACE_ID`
      configured against the **sacrificial** workspace named in
      [`live-tests.md`](live-tests.md).
- [x] `CLOCKIFY_LIVE_WRITE_ENABLED=true` (repo variable) — mutating
      tests run, not just read-only.
- [ ] Latest scheduled run of `live-contract.yml` is green with
      both `TestE2EReadOnly` and `TestE2EMutating` passing.
      _Tracking 2026-05-09: latest scheduled runs 25593042387 and
      25538247771 are green on commit
      4fe957547f9e6aea749a85f87823d17a0ccc2928 and include the
      required read-only, mutating, audit-phase, and schema-diff log
      markers. The local May 8 remediation tree is newer and
      uncommitted, so this box remains open until the final candidate
      SHA has scheduled-run evidence._
- [ ] `TestLiveDryRunDoesNotMutate` and
      `TestLivePolicyTimeTrackingSafeBlocksProjectCreate` are
      passing on the same run (MCP-path enforcement contract).
      _Tracking 2026-05-02: both green on the manual full-tier
      run 25239216412 (after the dry-run envelope fix in
      commit 71c4f8a). Restating the proof on a scheduled run is
      gated on box 3._
- [ ] Two consecutive nightly runs green with no flakes; if there
      is a flake, the rolling `live-test-failure` GitHub issue is
      closed and the root cause is documented in `CHANGELOG.md`.
      _Tracking 2026-05-09: two consecutive scheduled greens exist on
      pushed commit 4fe957547f9e6aea749a85f87823d17a0ccc2928, but
      not on the final remediation SHA because this local tree is
      still dirty and uncommitted. The live-test-failure issues remain
      closed; awaiting two cron greens on the final candidate._
- [ ] Read-side schema diff: response shapes returned by the
      Clockify upstream match the structs in `internal/clockify/`
      with no fields silently dropped (manual diff once per
      candidate cut, recorded in the wave's commit messages).
      _Tracking 2026-05-02: `TestLiveReadSideSchemaDiff` now fetches
      raw read-side Clockify JSON and fails on top-level fields not
      represented in `internal/clockify/models.go`; the read-only
      `live-contract.yml` step runs it alongside `TestE2EReadOnly`.
      Awaiting scheduled-run evidence on the candidate SHA before
      this box can close._

**Definition of done.** Two clean nightly runs in a row with
mutating + audit tiers enabled, no open `live-test-failure` issue,
and no upstream schema field that the client silently discards.

See also: [`docs/api-coverage.md`](api-coverage.md) for the full
128-tool coverage matrix, per-tool dry-run/policy breakdown, and
evidence hierarchy.

---

## 2. Shared-service / Postgres E2E

Group 2 is closed. The shared-service profile now has a CI-driven
Postgres-backed streamable-HTTP E2E that exercises the production
tenant/runtime shape, tenant isolation, sessions, and audit
durability. The checklist below records the evidence that closed the
old gap; do not reopen it unless the E2E, required-status-check
promotion, or documented invariants regress.

- [x] `make test-postgres` runs green from a clean checkout
      (Testcontainers, `INTEGRATION_REQUIRED=1`).
      _Closed 2026-05-02: the target now normalizes Unix Docker
      contexts with `TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock`
      when unset, and the shared-service / rehydration E2Es reuse the
      package Testcontainers DSN under the `integration` tag. Verified
      locally with Colima (`ok github.com/apet97/go-clockify/internal/controlplane/postgres`)._
- [x] `MCP_LIVE_CONTROL_PLANE_DSN` configured against a sacrificial
      Postgres database; `CLOCKIFY_LIVE_AUDIT_REQUIRED=true` set as
      a repo variable.
      _Rechecked 2026-05-09: `gh variable list --repo
      apet97/go-clockify` reports `CLOCKIFY_LIVE_AUDIT_REQUIRED`
      exists; scheduled `live-contract.yml` run 25538247771 logged
      `AUDIT_REQUIRED: true` and a redacted
      `MCP_LIVE_CONTROL_PLANE_DSN`._
- [x] `TestLiveCreateUpdateDeleteEntryAuditPhases` is green on the
      latest `live-contract.yml` run (intent + outcome rows for
      every non-read tool call).
      _Rechecked 2026-05-09: scheduled run 25538247771 executed
      `go test -tags=postgres,livee2e -run
      '^TestLiveCreateUpdateDeleteEntryAuditPhases$' ./...` and ended
      `ok github.com/apet97/go-clockify/internal/controlplane/postgres`._
- [x] **New** shared-service E2E
      (`internal/controlplane/postgres/e2e_shared_service_test.go`,
      build tag `postgres`, runnable via `make shared-service-e2e`)
      that:
      - Boots `mcp.ServeStreamableHTTP` in-process with the same
        per-tenant runtime shape `clockify-mcp-postgres` uses
        (per-tenant Clockify client + per-tenant policy +
        Pipeline/Gate + Postgres-backed control-plane store).
        In-process boot avoids subprocess flakiness while exercising
        the same wiring as the binary; the contract is the
        integration, not the binary packaging.
      - Exercises the streamable-HTTP transport.
      - Drives a multi-tenant traffic pattern: tenant A (operator,
        `policy_mode = standard`) and tenant B (AI-facing,
        `policy_mode = time_tracking_safe`), one principal each
        via `forward_auth` headers, 5 calls total.
      - Asserts tenant isolation in `audit_events` and `sessions`
        rows: per-tenant row counts, per-tenant
        `(tool, phase, outcome)` tuples, cross-tenant negative
        (zero rows for `tenant_id = A AND session_id = B` and the
        mirror), per-tenant `sessions.tenant_id` matches the
        principal-supplied `X-Forwarded-Tenant`.
      - Tears down all data it wrote (prefix-scoped `DELETE` in
        both pre-emptive and `t.Cleanup` passes).
- [x] The new E2E is wired into a CI workflow
      (`shared-service-e2e` job in `.github/workflows/ci.yml`,
      modeled on `doctor-postgres`'s service-container shape) and
      runs per-PR, which exceeds the live-contract nightly cadence.
      _Promoted to required-status check on 2026-05-02 after three
      consecutive green runs on `main` (25240007056, 25240085916,
      25240163213) via `gh api POST repos/apet97/go-clockify/branches/
      main/protection/required_status_checks/contexts` with
      `["Shared-service Postgres E2E"]`; verified by re-reading
      `required_status_checks.contexts` and by
      `scripts/audit-branch-protection.sh`._

**Definition of done.** A CI-driven shared-service E2E exists,
runs per PR/push, and asserts both functional behaviour (tools
behave) and operational invariants (tenant isolation, audit
durability, no cross-tenant leakage).

---

## 3. Streamable HTTP / session behavior

Driven by ADR `0017-streamable-http-session-rehydration.md`
(Accepted). One of the two paths below must be taken.

- [x] **Path A — implement the rehydration fix.** The four design
      questions in ADR 0017 (Factory contract widening, Principal
      reconstruction, persistence depth, eviction-on-restore) have
      explicit decisions recorded in the ADR; ADR is moved to
      Accepted; an implementation lands behind a parity test that
      proves cross-pod failover survives without re-initialize.
      _Closed 2026-05-02 by commits eb5351c (failing-first test)
      + 8353934 (`streamSessionManager.get` store fallback +
      `Server.MarkInitialized`) + fcfd7f0 (ADR Accepted with
      Q1=A, Q2=Strict, Q3=Fresh, Q4=PreserveTTL)._
- [ ] ~~**OR Path B — document the single-replica limitation.**~~
      _Not taken; Path A landed instead._ The
      `sessionAffinity: ClientIP` band-aid stays as defence-in-
      depth + perf optimisation per ADR 0017's "Decision" section;
      correctness no longer depends on it.
- [x] In either path, `tests/sse_resume_test.go` and the
      streamable-HTTP parity tests stay green.
      _Verified post-Path-A: `go test -race ./internal/mcp/...
      ./tests/...` green; SSE resume test unchanged because the
      single-instance Last-Event-ID replay path is untouched._
- [x] If Path A is chosen, a multi-replica integration test
      (≥2 backends, traffic crossing replicas, no re-initialize
      observed) gates the merge.
      _`TestStreamableHTTPCrossInstanceRehydration` in
      `internal/controlplane/postgres/e2e_session_rehydration_test.go`
      pins the contract; runs in CI under the existing
      `Shared-service Postgres E2E` job (test pattern extended in
      the same wave's Make-target update)._

**Definition of done.** ADR 0017 is no longer in the "Proposed"
state; the production posture is unambiguous in the docs; CI
pinpoints the chosen path with a parity or integration test.

---

## 4. Auth and tenant model

The recent hardening wave (forward_auth control bytes, OIDC strict
mode, JWKS, EC JWK, tenant validation) closed eight findings and
did not introduce regressions in the parity matrix. The launch
gate is to make the model **legible** to a reviewer who has not
read every commit.

- [x] One-page auth-model summary at
      [`docs/auth-model.md`](auth-model.md): lists every supported
      auth mode (`static_bearer`, `oidc`, `forward_auth`, `mtls` —
      `stdio` is a transport with no inbound auth, not an auth
      mode), what principal each produces, what tenant it
      derives, and what the failure mode looks like, with every
      claim cross-cited to a test pin. Cross-linked from
      `docs/production-readiness.md` "Pick an auth mode" and
      from `docs/runbooks/auth-failures.md`. _Closed 2026-05-02
      (commits 0bcd30b + 8a627d6)._
- [x] Every auth mode is exercised by at least one entry in the
      transport-auth parity matrix. Coverage lives in two places:
      `internal/config/transport_auth_matrix_test.go::TestTransportAuthMatrix`
      pins the **{transport × auth_mode}** config-load surface
      (every cell either loads cleanly or fails with a named
      error), and `internal/mcp/transport_http_authmatrix_test.go`
      pins the HTTP-handler-level rejection for each mode. _Pre-
      existing; cross-cited in `docs/auth-model.md` "Test pins"._
- [x] `forward_auth` headers are rejected for control bytes,
      duplicated values, and oversized payloads; tests pin all
      three boundaries.
      _Closed 2026-05-02: control-byte boundary pinned by
      `internal/authn/auth_hardening_test.go::TestForwardAuth_RejectsControlBytesInHeaders`
      and duplicated/oversized boundary pinned by
      `TestForwardAuth_RejectsDuplicatedAndOversizedHeaders`
      (`forward_auth` accepts at most one value per configured
      principal header and caps each raw value at 1024 bytes), with
      the trusted-proxy CIDR gate pinned by
      `TestForwardAuth_RejectsUntrustedSource` /
      `TestForwardAuth_AcceptsTrustedCIDR` /
      `TestForwardAuth_EmptyAllowlistPreservesLegacyBehaviour`._
- [x] OIDC strict mode and kid-required mode are documented defaults
      for the shared-service profile; the JWKS rotation path is covered
      by bounded cache semantics and a kid-less-token regression test.
      _2026-05-02: `MCP_OIDC_STRICT=1` was pinned in
      `docs/deploy/production-profile-shared-service.md`; 2026-05-08
      adds `MCP_OIDC_REQUIRE_KID=1` and
      `internal/authn/oidc_integration_test.go::TestOIDCAuthenticator_RequireKID`.
      The JWKS rotation propagation window is bounded
      by `internal/authn/oidc_verify_cache_test.go::TestOIDCVerifyCache_CeilingTTL`
      (cache entries cannot survive past `oidcVerifyCacheTTLCeiling`,
      capped at 5m) and `TestOIDCVerifyCache_TTLClamping`. A literal
      mid-session key-swap test (issuer rotates kid → next request
      re-fetches JWKS) is not present today; the safety margin is
      the bounded TTL plus the JWKS-fetch error semantics in
      `internal/authn/jwks_document_test.go`. Documented in
      `docs/auth-model.md` "Edge cases" and the failure-mode
      table._
- [x] Tenant isolation invariants are documented in
      [`docs/auth-model.md`](auth-model.md) "Tenant resolution"
      and `docs/production-readiness.md` "Pick an auth mode" /
      "Session rehydration" (one tenant cannot read another
      tenant's audit rows or sessions). Pinned by:
      `internal/controlplane/postgres/e2e_shared_service_test.go::TestSharedServicePostgresE2E`
      (cross-tenant query for `tenant_id=A AND session_id=B`
      returns zero rows) and
      `internal/controlplane/postgres/e2e_session_rehydration_test.go::TestStreamableHTTPCrossInstanceRehydration`
      (cross-tenant replay across pods returns 403 + zero new
      audit rows). _Pre-existing; the auth-model.md doc commit
      (0bcd30b) and this checklist tick make the invariant
      legible without grepping the test files._

**Definition of done.** Anyone reading
[`docs/auth-model.md`](auth-model.md) can answer "what does auth
look like?" in under five minutes; every claim made there is
pinned by a test cited in the same doc.

---

## 5. Product launch docs

The publishable docs surface that anyone outside the maintainer
will read.

- [x] `README.md` — top-of-file claims (transport list, policy
      modes, tool count, supported deployments) match the live
      `docs/tool-catalog.md` count, the `internal/config/spec.go`
      surface, and the deployment profile docs. Run
      `make doc-parity` to verify.
      _Verified 2026-05-03: `docs/tool-catalog.json` has
      40 Tier 1 tools + 88 Tier 2 tools = 128 total, matching
      README. Tier 2 dropped from 91 → 88 over PR #55 (phantom
      `list_schedules` removed) and the matching cleanup branch
      removing the equivalent phantom `get_` and `create_` schedule
      tools (no `/scheduling/{id}` surface exists upstream); the
      current Tier 1 count includes two high-level timesheet workflow
      helpers.
      `make doc-parity`, `make config-doc-parity`,
      `make catalog-drift`, and `make launch-checklist-parity` all
      green after the regeneration._
- [x] `CHANGELOG.md` Unreleased section has a clear,
      user-facing summary of every behavioural change since
      v1.2.0; no "internal only" hand-waving for changes that
      affect operators.
      _Verified 2026-05-02: Unreleased has operator-facing entries
      for shared-service E2E, session rehydration, auth-model docs,
      branch-protection promotion, and this launch-doc verification
      pass._
- [x] `docs/clients.md` lists every supported MCP client we have
      tested against (Claude Desktop, Claude Code, Cursor,
      VS Code MCP, …) with the exact transport + auth combo each
      one uses. Untested combos are flagged.
      _Closed 2026-05-02: client matrix now names the exact
      stdio + env-auth shape for Claude Code, Claude Desktop,
      Cursor, Codex, and VS Code MCP; custom streamable HTTP and
      gRPC client rows separate server-transport support from
      operator-owned client semantics. Untested non-stdio desktop
      combos are explicitly flagged._
- [x] `docs/support-matrix.md` is current for the candidate tag:
      Go version pin, OS/arch matrix, FIPS posture, kernel
      requirements (if any).
      _Closed 2026-05-02: support matrix now records Go 1.25.10,
      default/Postgres/gRPC/FIPS artifact OS-arch coverage,
      container platform coverage, Windows limitations, FIPS
      posture, and the absence of project-specific Linux kernel
      requirements. Refreshed 2026-05-09 with the Go 1.25.10
      security-patch bump._
- [x] Every deployment profile doc under `docs/deploy/` ends with
      a "How to verify this deployment" section that names the
      `doctor --strict` invocation and the smoke-test workflow
      that backs it.
      _Closed 2026-05-02: `profile-local-stdio.md`,
      `profile-single-tenant-http.md`,
      `profile-private-network-grpc.md`,
      `profile-self-hosted.md`, and
      `production-profile-shared-service.md` all end with a
      verification section. Non-hosted profiles explicitly mark
      `doctor --strict` as a negative hosted-posture check and
      name the positive smoke target (`stdio-smoke`,
      `http-smoke`, `grpc-auth-smoke`, or `shared-service-e2e`)._

**Definition of done.** A new operator can pick a profile,
deploy, and verify success without reading source code.

---

## 6. Security and policy review

Runbook and automation:
[`docs/runbooks/release-candidate-evidence.md`](runbooks/release-candidate-evidence.md)
and `make rc-evidence-plan TAG=vX.Y.Z-rc.N`. Use them only after
Group 1 scheduled-cron evidence closes.

- [ ] `make verify-vuln` green for the candidate tag (govulncheck
      across the build-tag matrix).
      _Local preflight 2026-05-09 on the May 8 remediation tree:
      `govulncheck@v1.3.0` failed against Go 1.25.9 with
      GO-2026-4971 and GO-2026-4918, then passed after the repo pin
      moved to Go 1.25.10. Rechecked on 2026-05-09 with
      pinned `govulncheck@v1.3.0` and `GOTOOLCHAIN=go1.25.10`;
      no vulnerabilities found. A host-toolchain scan with Go 1.26.2
      reports standard-library issues GO-2026-4971 and GO-2026-4918
      fixed in Go 1.26.3, so the public support docs now state the
      exact Go 1.25.10 launch-candidate pin instead of a broad
      `1.25.10+` claim. Do not tick this box until the same pinned
      scan is re-run on the final candidate tag._
- [ ] `gitleaks` scan green (config in `.gitleaks.toml`).
      _Local preflight 2026-05-02: `make secret-scan` ran
      `gitleaks detect --no-git --source . --redact --config
      .gitleaks.toml`; no leaks found. Rechecked on 2026-05-09:
      the candidate branch-content gitleaks scan in
      `make public-content-audit` returned no findings, but
      `make secret-scan` on this dirty workstation failed on ignored
      local artifacts (`.local/`, `.serena/`, and the duplicate
      `go-clockify/` checkout). Do not tick this box until
      `make secret-scan` is re-run from a clean checkout of the final
      candidate tag._
- [ ] `semgrep` review green; any `// nosemgrep` directive has a
      justification comment within five lines and is referenced
      from the relevant ADR or runbook.
      _Local preflight 2026-05-02: `semgrep scan --config p/default
      --metrics=off --error --exclude .git --exclude .bench
      --exclude clockify-mcp .` scanned 1094 tracked files and
      returned 0 findings. The SSE `text/event-stream` suppressions
      in `internal/mcp/transport_streamable_http.go` have inline
      justification comments and are recorded in ADR 0017. 2026-05-08
      adds `.github/workflows/semgrep.yml` as a recurring CE scan using
      the same `p/default` rule pack. Rechecked on 2026-05-09 against
      the current tree; Semgrep scanned 1153 tracked files and returned
      0 findings. Do not tick this box until the scan is re-run on the
      final candidate tag._
- [ ] `make verify-fips` green when the FIPS-aware tooling is
      installed (auto-skips otherwise — record the run on a host
      that has it).
      _Local preflight 2026-05-02 on macOS arm64 with a FIPS-capable Go
      toolchain: `make verify-fips` built and tested `-tags=fips`
      plus the `-tags=fips,grpc` build combination. Rechecked on
      2026-05-09 with `GOTOOLCHAIN=go1.25.10 make verify-fips`;
      default FIPS tests and the `-tags=fips,grpc` build combination
      passed. Do not tick this box until the same gate is re-run on the
      final candidate tag._
- [x] No public AI-facing deployment can boot with a policy
      weaker than `time_tracking_safe`; the load-time guard
      remains in place.
      _Pinned by `internal/config/profile_test.go`:
      `TestProfile_SingleTenantHTTPDefaults`,
      `TestProfile_SharedServiceIsStrict`, and
      `TestProfile_ProdPostgresIsStrict` assert the AI-facing
      profile defaults; `cmd/clockify-mcp/main_test.go::
      TestDoctorStrictAllowBroadPolicyFlag` asserts hosted
      `doctor --strict` rejects broader explicit overrides unless
      the operator passes `--allow-broad-policy`._
- [x] `MCP_AUDIT_DURABILITY=fail_closed` is the effective default
      under `ENVIRONMENT=prod` (locked by tests in
      `internal/config/prod_defaults_test.go`).
      _Pinned by
      `internal/config/prod_defaults_test.go::
      TestProdDefaults_AuditDurability`; covered by `make check`._
- [x] `MCP_ALLOW_DEV_BACKEND=1` cannot survive a load-time check
      under any production-shaped profile; the dev-backend
      escape hatch is documented and its risks are spelled out.
      _Pinned by
      `internal/config/prod_defaults_test.go::
      TestProdDefaults_RejectsDevBackendEscapeHatch`, which rejects
      `ENVIRONMENT=prod` + `MCP_ALLOW_DEV_BACKEND=1` even with a
      Postgres DSN. The risk and escape-hatch scope are documented
      in ADR 0014, `docs/production-readiness.md`, and the
      deployment profile docs._

**Definition of done.** No HIGH/CRITICAL vulnerability findings;
no policy regression; no escape-hatch can be activated by
accident.

---

## 7. CI / release readiness

Runbook and automation:
[`docs/runbooks/release-candidate-evidence.md`](runbooks/release-candidate-evidence.md)
and `scripts/prepare-rc-evidence.sh vX.Y.Z-rc.N`. The script gathers
local logs and GitHub workflow metadata, but the boxes below still
require the specific candidate-tag evidence links before they can be
checked.

- [ ] `make release-check` green from a clean checkout on at
      least one Linux x64 and one macOS arm64 host.
- [ ] All required workflows on `main` green: `ci.yml`,
      `build-matrix.yml`, `live-contract.yml` (latest scheduled
      run), `release-smoke.yml` (latest tag), `link-check.yml`,
      `chaos.yml`, `mutation.yml`, `reproducibility.yml`,
      `bench.yml`. No skipped-but-required steps.
      _Tracking 2026-05-09: `make launch-external-status` reports the
      latest `mutation.yml` scheduled run 25592823559 is
      `completed/cancelled` on pushed commit
      4fe957547f9e6aea749a85f87823d17a0ccc2928. `gh run view` shows
      the `internal/tools` matrix leg was cancelled while the other
      mutation legs succeeded. The local timeout increase still needs
      pushed scheduled-run evidence on the final candidate SHA._
- [x] `make verify-bench` and `make bench-baseline-check` green;
      no regression > the documented threshold versus the
      baseline.
      _Closed 2026-05-02 by PR #51 on `main`: refreshed
      `internal/benchdata/baseline.txt` from `Bench` workflow run
      25255062599, validated locally with `make bench-baseline-check`,
      then passed linux/amd64 comparison in
      https://github.com/apet97/go-clockify/actions/runs/25255216987._
- [ ] Release artefacts: signed binaries (cosign, plus SLSA when
      GitHub artifact attestations are available), SBOMs,
      Docker images, FIPS variant. Verified by `release-smoke.yml`
      on the candidate tag for its sampled default/Postgres
      linux-x64 artifacts, plus manual `docs/verification.md`
      evidence for any required variant not sampled by
      `release-smoke.yml`.
- [ ] `clockify-mcp doctor --strict` and
      `clockify-mcp-postgres doctor --strict --check-backends`
      both exit 0 against the candidate's reference deployment.
      For `release-smoke.yml`, archive or link the
      `release-smoke-doctor-output` artifact containing
      `release-doctor-strict-ok.txt`,
      `release-doctor-strict-fail.txt`, and
      `release-doctor-postgres-ok.txt`.

**Definition of done.** A clean checkout of the candidate tag
produces a green `release-check`, every required workflow on
`main` is green, and the release artefacts verify under cosign plus
SLSA when GitHub artifact attestations are available. If GitHub still
returns the ADR-0013 private-repo feature gate on the candidate tag,
the skip evidence must be archived with the mandatory cosign binary
and image verification.

---

## Promotion

When every group above is green and the definition of done is
satisfied:

1. Cut the candidate tag (`vX.Y.Z-rc.N`).
2. Run `release-smoke.yml` against the tag; archive its output.
3. Update `docs/official-clockify-mcp-gap-analysis.md`: move
   "blockers" entries that have been closed into the "what is
   already strong" section.
4. Link the closed `docs/release/brand-legal-review.md` decision if
   any public copy will claim official-product status; otherwise use
   the approved rebrand/community framing in release notes and public
   metadata.
5. Open a tracking issue titled `Launch candidate vX.Y.Z-rc.N`
   that links to the green workflow runs and the archived
   `doctor --strict` output, including the
   `release-smoke-doctor-output` artifact.

Only at that point may any agent or human report **"launch
candidate ready"**.
