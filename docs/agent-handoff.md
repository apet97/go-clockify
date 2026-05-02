# Agent Handoff — Clockify MCP Launch Candidate Goal

You (Claude Code, Codex, or another autonomous coding agent) are
picking up the work to bring `github.com/apet97/go-clockify` from
"community-MCP ready / internal-support alpha" to **official
Clockify launch candidate**.

This document is your entry point. Read it before doing anything,
then read the three referenced docs, then do the smallest piece
of useful work and commit it.

> If you ignore the safety constraints in this document the
> maintainer will revert your work. They are not negotiable.

## Read first (in this order)

1. The workstation-private `CLAUDE.md` at the repo root
   (gitignored — fetch from your local clone or the maintainer)
   — project conventions, "Strict agent rules", and the
   canonical "Launch Candidate Goal" statement.
2. [`launch-candidate-checklist.md`](launch-candidate-checklist.md)
   — the bound list of what must be true to declare launch
   candidate.
3. [`official-clockify-mcp-gap-analysis.md`](official-clockify-mcp-gap-analysis.md)
   — the narrative of where the project is, what is strong, and
   what blocks tier 3 readiness.
4. [`adr/0017-streamable-http-session-rehydration.md`](adr/0017-streamable-http-session-rehydration.md)
   — the open architectural decision; its resolution is on the
   critical path.
5. [`live-tests.md`](live-tests.md) — how the live-contract
   nightly works and how the sacrificial workspace is wired.
6. [`deploy/production-profile-shared-service.md`](deploy/production-profile-shared-service.md)
   — the deployment shape that the launch candidate is built
   around.

## Current known blockers

In priority order. Each blocker has a Claude-Code slash command
that scopes the investigation; the slash-command files live
under the workstation-private `.claude/commands/` directory
(gitignored). If you are running outside Claude Code, treat the
prose below as the checklist.

1. **Live contract failures** — `.github/workflows/live-contract.yml`
   has been intermittently red. The promotion gate starts with
   two consecutive green nightlies. Slash command:
   `/fix-live-contract`.
2. **No shared-service Postgres E2E** — pieces exist
   (`make test-postgres`, `TestLiveCreateUpdateDeleteEntryAuditPhases`)
   but no single test exercises the full shared-service stack
   end-to-end. Slash command: `/postgres-e2e`.
3. **ADR 0017 unresolved** — streamable-HTTP session rehydration
   is "Proposed + band-aid". Either ship the fix or document the
   single-replica limitation. Slash command:
   `/session-rehydration`.
4. ~~**Auth-model docs scattered**~~ **Closed 2026-05-02** by
   commits 0bcd30b (new `docs/auth-model.md`: 4-mode table,
   Principal mapping, tenant resolution, failure modes, test
   pins, 5-question reviewer self-quiz) + 8a627d6 (operator-doc
   cross-links from `docs/production-readiness.md` "Pick an
   auth mode" and `docs/runbooks/auth-failures.md`) +
   222c206 (Group 4 checklist terminology fix + boxes ticked).
5. **Launch docs not verified end-to-end** — `README.md`,
   `docs/clients.md`, `docs/support-matrix.md`, and the
   `docs/deploy/profile-*.md` set need a parity pass.
6. **Bench baseline pre-dates the perf wave** — `.bench/` baseline
   needs refreshing after the cached tools/list, tier-2 cache,
   and schema compaction commits.
7. **Security review walk-through** —
   `make verify-vuln`, `make verify-fips`, `gitleaks`, `semgrep`
   all green on the candidate tag with findings filed in
   `SECURITY.md`.

For the full audit framing run `/launch-candidate` from a Claude
Code session inside this repo (the slash command is gitignored
under `.claude/commands/`).

## Likely files to inspect first

Group them by blocker so you do not hop around:

**Live contract / live tests**
- `tests/e2e_live_test.go`
- `tests/e2e_live_mcp_test.go`
- `.github/workflows/live-contract.yml`
- `docs/live-tests.md`
- The most recent `live-test-failure` issue on the GitHub repo
  (`gh issue list --label live-test-failure --state all -L 5`).

**Shared-service Postgres E2E** (Group 2 closed 2026-05-02; promoted
to required-status check on `main` 2026-05-02)
- `internal/controlplane/postgres/e2e_shared_service_test.go` —
  the test that closed Group 2; runs as the
  `Shared-service Postgres E2E` CI job on every PR.
- `internal/controlplane/postgres/`
- `internal/runtime/service.go`, `internal/runtime/store.go`
- `tests/harness/streamable.go`
- `docs/deploy/production-profile-shared-service.md`
- `docs/branch-protection.md` — snapshot of the required-check
  list including `Shared-service Postgres E2E`.
- `Makefile` targets `test-postgres`, `build-postgres`,
  `shared-service-e2e`, `release-check`.

**Session rehydration**
- `internal/mcp/transport_streamable_http.go`
  (`streamSessionManager.get`, `create`, `touch`)
- `internal/controlplane/store.go` (the `SessionRecord` shape)
- `internal/authn/` (Principal construction)
- `tests/sse_resume_test.go`
- `deploy/helm-chart/templates/service.yaml` (the `sessionAffinity:
  ClientIP` band-aid).

**Auth model** (Group 4 closed 2026-05-02)
- [`docs/auth-model.md`](auth-model.md) — one-page reviewer
  summary; start here.
- `internal/authn/` — implementation (mode constants at
  `authn.go:36-41`).
- `internal/config/transport_auth_matrix_test.go::TestTransportAuthMatrix` —
  `{transport × auth_mode}` config-load surface.
- `internal/mcp/transport_http_authmatrix_test.go` — HTTP
  handler-level rejection per mode.
- `docs/production-readiness.md` "Pick an auth mode" — operator
  picker; cross-links into `auth-model.md`.

**Generated docs (parity-gated)**
- `internal/config/spec.go` — single source of truth.
- `cmd/clockify-mcp/help_generated.go` — output of the generator.
- `README.md` — `<!-- CONFIG-TABLE BEGIN -->` block.
- `docs/tool-catalog.json` and `docs/tool-catalog.md`.

## Commands to run

Discovered from `Makefile`, `.github/workflows/`, and the existing
docs. Do not invent new commands; if you need one that does not
exist, propose it as a Makefile target before using it.

| Why | Command |
|---|---|
| Quick sanity (fmt + vet + test) | `make check` |
| Pre-ship local gate | `make release-check` |
| Coverage | `make cover` |
| Single test | `go test -race -run TestName ./path/...` |
| Streamable-HTTP smoke | `make http-smoke` |
| Stdio smoke | `make stdio-smoke` |
| gRPC build / parity | `make build-grpc`, `make grpc-release-parity`, `make grpc-auth-smoke` |
| Postgres build | `make build-postgres` |
| Postgres integration tests | `make test-postgres` (requires Docker; uses Testcontainers + `INTEGRATION_REQUIRED=1`) |
| Live-contract tests (read-only) | `CLOCKIFY_LIVE_API_KEY=... CLOCKIFY_LIVE_WORKSPACE_ID=... go test -tags=livee2e -run '^TestE2EReadOnly$\|^TestE2EErrors$' ./tests/` |
| Live-contract tests (mutating, sacrificial only) | append `-run '^TestE2EMutating$\|^TestLiveDryRunDoesNotMutate$\|^TestLivePolicyTimeTrackingSafeBlocksProjectCreate$'` and only against the workspace named in `docs/live-tests.md` |
| Doctor (config-strict) | `clockify-mcp doctor --profile=<profile> --strict` |
| Doctor (backends) | `clockify-mcp-postgres doctor --profile=prod-postgres --strict --check-backends` |
| Refresh generated docs | `go run ./cmd/gen-config-docs -mode=all && make gen-tool-catalog` |
| Vuln scan | `make verify-vuln` |
| FIPS verify | `make verify-fips` |
| Bench | `make verify-bench` then `make bench-baseline-check` |
| Mutation testing | `make mutation` |

## Non-negotiable safety constraints

These are restated from `CLAUDE.md` "Strict agent rules". If a
constraint conflicts with a task, the task is wrong, not the
constraint.

1. **Do not declare launch-ready until live-contract + shared-service
   Postgres E2E + CI on `main` are simultaneously green.** Local
   `release-check` is necessary, not sufficient.
2. **Do not weaken security or profile defaults to make tests
   pass.** No relaxing `time_tracking_safe`. No flipping
   `MCP_AUDIT_DURABILITY` away from `fail_closed` under
   `ENVIRONMENT=prod`. No granting `MCP_ALLOW_DEV_BACKEND=1` in
   production-shaped fixtures. If a default needs to change,
   write an ADR first.
3. **Tests before broad refactors.** First commit of any
   transport / enforcement / authn refactor is a failing test
   that expresses the new contract. Drift checks (flip the
   assertion, confirm red, restore) on non-trivial test commits;
   record in the `Verified:` line.
4. **Keep the generated docs in lockstep with the source.** Any
   change to `internal/config/spec.go` or to a tool descriptor
   must re-run `gen-config-docs` and `make gen-tool-catalog` in
   the same commit. CI gates `config-doc-parity` and
   `catalog-drift` will reject partial updates.
5. **Do not run destructive live Clockify calls outside the
   sacrificial workspace.** The only approved workspace is the
   one named in `docs/live-tests.md`, reachable via
   `CLOCKIFY_LIVE_API_KEY` + `CLOCKIFY_LIVE_WORKSPACE_ID`. Do not
   point those secrets at any personal, teammate, or production
   workspace. When in doubt: read-only only.
6. **Do not skip git hooks.** No `--no-verify`, no
   `--no-gpg-sign`. If a hook fails, fix the underlying issue.
7. **Atomic commits, atomic pushes.** One logical change per
   commit; the body ends with `Why:` and `Verified:` lines. When
   landing a multi-commit wave, push only when the whole group is
   green locally.
8. **Do not modify generator-owned files by hand.** Listed in
   `CLAUDE.md` "Generator-owned files".
9. **Do not invent commands.** If a command is not in `Makefile`,
   `.github/workflows/`, or the docs, propose it as a Makefile
   target first.
10. **`CLAUDE.md` and this file are gitignored.** Do not try to
    commit them; they are per-workstation context.

## Suggested implementation order

The order minimises rework: every step makes the next step
verifiable.

1. **Audit** (`/launch-candidate`). Confirm the gap analysis is
   still accurate. If the doc lags reality, fix the doc first;
   accurate docs are themselves a launch-candidate gate.
2. **Stabilise the live-contract loop** (`/fix-live-contract`).
   Until the nightly is reliably green, every other gate is
   relative to a moving baseline. Investigate, classify each
   failure (real / flake / upstream), fix or quarantine, and aim
   for two consecutive green nightly runs.
3. **Build the shared-service Postgres E2E** (`/postgres-e2e`).
   This is the largest single net-new test. Use the existing
   `make test-postgres` Testcontainers infrastructure for the
   Postgres bring-up; reuse `tests/harness/streamable.go` for the
   transport. Assert tenant isolation and audit invariants.
4. **Resolve ADR 0017** (`/session-rehydration`). Pick path A
   (implement) or path B (document). If path A, the multi-replica
   integration test from step 3's harness is the natural pin. If
   path B, update `docs/production-readiness.md`, the relevant
   profile docs, and the Helm chart's `replicaCount` default.
5. **Consolidate auth-model docs.** One page that maps every
   Clockify auth requirement to an MCP config knob. Cross-link
   from `README.md`.
6. **Verify launch docs.** `make doc-parity`, then a manual
   review pass over `docs/clients.md`, `docs/support-matrix.md`,
   `docs/deploy/profile-*.md`, and the `README.md` claims.
7. **Refresh bench baseline.** Run `make verify-bench` on the
   candidate shape, commit the new `.bench/` baseline file with
   a `Why:` line that names the perf wave it reflects.
8. **Security review walk-through.** `make verify-vuln`,
   `make verify-fips`, `gitleaks`, `semgrep`. File any findings
   in `SECURITY.md`.
9. **Cut a release candidate** (`vX.Y.Z-rc.N`), watch
   `release-smoke.yml`, archive the green run.
10. **Open the launch-candidate tracking issue.** Link to the
    green workflow runs and the archived `doctor --strict`
    output. Only at this point does any agent or human declare
    "launch candidate ready."

## When you are uncertain

Stop and write down what you know in the commit message body.
The `Why:` line is the place for "I am uncertain because X" —
hidden uncertainty is worse than a documented one.

If the uncertainty is a security or default-weakening question:
**stop and ask the maintainer.** The cost of waiting is low; the
cost of guessing wrong is high.
