# Launch Candidate Checklist

The pre-flight gate to take `clockify-mcp` from "community MCP
ready / internal-support alpha" to **official Clockify launch
candidate**. This is **additive** to the existing
[`deploy-readiness-checklist.md`](release/deploy-readiness-checklist.md)
and [`public-hosted-launch-checklist.md`](release/public-hosted-launch-checklist.md);
those checklists govern any deploy. This one governs the *promotion
of the project itself* to official-product status.

A box gets ticked only when it is reproducibly green from a clean
checkout. "Worked once" is not green.

> **Strict agent rule.** Do not declare "launch candidate" until
> every group below is ticked **and** the group-level definition of
> done is satisfied. See [`../CLAUDE.md`](../CLAUDE.md) "Strict agent
> rules" for the binding constraints.

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
- [ ] `TestLiveDryRunDoesNotMutate` and
      `TestLivePolicyTimeTrackingSafeBlocksProjectCreate` are
      passing on the same run (MCP-path enforcement contract).
- [ ] Two consecutive nightly runs green with no flakes; if there
      is a flake, the rolling `live-test-failure` GitHub issue is
      closed and the root cause is documented in `CHANGELOG.md`.
- [ ] Read-side schema diff: response shapes returned by the
      Clockify upstream match the structs in `internal/clockify/`
      with no fields silently dropped (manual diff once per
      candidate cut, recorded in the wave's commit messages).

**Definition of done.** Two clean nightly runs in a row with
mutating + audit tiers enabled, no open `live-test-failure` issue,
and no upstream schema field that the client silently discards.

---

## 2. Shared-service / Postgres E2E

This is the launch-blocking gap today. The shared-service profile
docs (`docs/deploy/production-profile-shared-service.md`) describe
the target deployment shape, but only fragments are covered by the
existing test surface (`make test-postgres` covers the Postgres
control-plane store unit + integration tests via Testcontainers,
and the live audit-phase test exercises the Postgres write path
end-to-end against an external DB). There is no end-to-end test
that boots a Postgres-tagged binary, drives traffic through the
streamable-HTTP transport with the shared-service profile, and
asserts the tenant + audit invariants.

- [ ] `make test-postgres` runs green from a clean checkout
      (Testcontainers, `INTEGRATION_REQUIRED=1`).
- [x] `MCP_LIVE_CONTROL_PLANE_DSN` configured against a sacrificial
      Postgres database; `CLOCKIFY_LIVE_AUDIT_REQUIRED=true` set as
      a repo variable.
- [x] `TestLiveCreateUpdateDeleteEntryAuditPhases` is green on the
      latest `live-contract.yml` run (intent + outcome rows for
      every non-read tool call).
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
      Required-check status is deferred until three consecutive
      green runs on `main`.

**Definition of done.** A CI-driven shared-service E2E exists,
runs nightly, and asserts both functional behaviour (tools
behave) and operational invariants (tenant isolation, audit
durability, no cross-tenant leakage).

---

## 3. Streamable HTTP / session behavior

Driven by ADR `0017-streamable-http-session-rehydration.md`
(Proposed). One of the two paths below must be taken.

- [ ] **Path A — implement the rehydration fix.** The four design
      questions in ADR 0017 (Factory contract widening, Principal
      reconstruction, persistence depth, eviction-on-restore) have
      explicit decisions recorded in the ADR; ADR is moved to
      Accepted; an implementation lands behind a parity test that
      proves cross-pod failover survives without re-initialize.
- [ ] **OR Path B — document the single-replica limitation.** A
      "Single-replica deployment" subsection is added to
      `docs/production-readiness.md` and to every applicable
      deployment profile doc (`docs/deploy/profile-single-tenant-http.md`,
      `docs/deploy/production-profile-shared-service.md`); the
      Helm chart's `replicaCount` default is set to `1` with a
      comment pointing at ADR 0017; the `sessionAffinity:
      ClientIP` band-aid is documented as the **partial**
      multi-replica posture with its limits (NAT egress, pod
      eviction, rolling upgrade, cross-AZ failover).
- [ ] In either path, `tests/sse_resume_test.go` and the
      streamable-HTTP parity tests stay green.
- [ ] If Path A is chosen, a multi-replica integration test
      (≥2 backends, traffic crossing replicas, no re-initialize
      observed) gates the merge.

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

- [ ] One-page auth-model summary in `docs/production-readiness.md`
      or a dedicated `docs/auth-model.md`: lists every supported
      auth mode (`disabled`, `bearer`, `jwt`, `oidc`,
      `forward_auth`), what principal it produces, what tenant
      it derives, and what the failure mode looks like.
- [ ] Every auth mode is exercised by at least one entry in the
      transport-auth parity matrix
      (`internal/mcp/transport_http_authmatrix_test.go`).
- [ ] `forward_auth` headers are rejected for control bytes,
      duplicated values, and oversized payloads; tests pin all
      three boundaries.
- [ ] OIDC strict mode is the documented default for the
      shared-service profile; the JWKS rotation path is covered by
      a test that exercises a key swap mid-session.
- [ ] Tenant isolation invariants are documented in
      `docs/production-readiness.md` (one tenant cannot read another
      tenant's audit rows, sessions, or credential refs) and pinned
      by tests in `internal/controlplane/`.

**Definition of done.** Anyone reading
`docs/production-readiness.md` can answer "what does auth look
like?" in under five minutes; every claim made there is pinned
by a test.

---

## 5. Product launch docs

The publishable docs surface that anyone outside the maintainer
will read.

- [ ] `README.md` — top-of-file claims (transport list, policy
      modes, tool count, supported deployments) match the live
      `docs/tool-catalog.md` count, the `internal/config/spec.go`
      surface, and the deployment profile docs. Run
      `make doc-parity` to verify.
- [ ] `CHANGELOG.md` Unreleased section has a clear,
      user-facing summary of every behavioural change since
      v1.2.0; no "internal only" hand-waving for changes that
      affect operators.
- [ ] `docs/clients.md` lists every supported MCP client we have
      tested against (Claude Desktop, Claude Code, Cursor,
      VS Code MCP, …) with the exact transport + auth combo each
      one uses. Untested combos are flagged.
- [ ] `docs/support-matrix.md` is current for the candidate tag:
      Go version pin, OS/arch matrix, FIPS posture, kernel
      requirements (if any).
- [ ] Every deployment profile doc under `docs/deploy/` ends with
      a "How to verify this deployment" section that names the
      `doctor --strict` invocation and the smoke-test workflow
      that backs it.

**Definition of done.** A new operator can pick a profile,
deploy, and verify success without reading source code.

---

## 6. Security and policy review

- [ ] `make verify-vuln` green for the candidate tag (govulncheck
      across the build-tag matrix).
- [ ] `gitleaks` scan green (config in `.gitleaks.toml`).
- [ ] `semgrep` review green; any `// nosemgrep` directive has a
      justification comment within five lines and is referenced
      from the relevant ADR or runbook.
- [ ] `make verify-fips` green when the FIPS-aware tooling is
      installed (auto-skips otherwise — record the run on a host
      that has it).
- [ ] No public AI-facing deployment can boot with a policy
      weaker than `time_tracking_safe`; the load-time guard
      remains in place.
- [ ] `MCP_AUDIT_DURABILITY=fail_closed` is the effective default
      under `ENVIRONMENT=prod` (locked by tests in
      `internal/config/prod_defaults_test.go`).
- [ ] `MCP_ALLOW_DEV_BACKEND=1` cannot survive a load-time check
      under any production-shaped profile; the dev-backend
      escape hatch is documented and its risks are spelled out.

**Definition of done.** No HIGH/CRITICAL vulnerability findings;
no policy regression; no escape-hatch can be activated by
accident.

---

## 7. CI / release readiness

- [ ] `make release-check` green from a clean checkout on at
      least one Linux x64 and one macOS arm64 host.
- [ ] All required workflows on `main` green: `ci.yml`,
      `build-matrix.yml`, `live-contract.yml` (latest scheduled
      run), `release-smoke.yml` (latest tag), `link-check.yml`,
      `chaos.yml`, `mutation.yml`, `reproducibility.yml`,
      `bench.yml`. No skipped-but-required steps.
- [ ] `make verify-bench` and `make bench-baseline-check` green;
      no regression > the documented threshold versus the
      baseline.
- [ ] Release artefacts: signed binaries (cosign + SLSA), SBOMs,
      Docker images, FIPS variant. Verified by
      `release-smoke.yml` on the candidate tag.
- [ ] `clockify-mcp doctor --strict` and
      `clockify-mcp-postgres doctor --strict --check-backends`
      both exit 0 against the candidate's reference deployment.

**Definition of done.** A clean checkout of the candidate tag
produces a green `release-check`, every required workflow on
`main` is green, and the release artefacts verify under cosign
+ SLSA.

---

## Promotion

When every group above is green and the definition of done is
satisfied:

1. Cut the candidate tag (`vX.Y.Z-rc.N`).
2. Run `release-smoke.yml` against the tag; archive its output.
3. Update `docs/official-clockify-mcp-gap-analysis.md`: move
   "blockers" entries that have been closed into the "what is
   already strong" section.
4. Open a tracking issue titled `Launch candidate vX.Y.Z-rc.N`
   that links to the green workflow runs and the archived
   `doctor --strict` output.

Only at that point may any agent or human report **"launch
candidate ready"**.
