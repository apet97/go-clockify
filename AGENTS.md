# AGENTS.md — Clockify MCP

> Standard agent-spec entrypoint for Codex, Claude Code, and any
> autonomous coding agent operating on this repository. Read this
> file first; it points at the binding constraints and the live
> launch-state docs.

## What this repo is

A production-grade [Model Context Protocol](https://modelcontextprotocol.io)
server for [Clockify](https://clockify.me), written in Go. Three
transports (stdio, streamable HTTP 2025-03-26, opt-in gRPC behind
`-tags=grpc`), four policy modes, signed releases. v1.2.0 is the
current stable line. The active workstream is the **official
Clockify launch candidate** promotion.

## Read first (in this order)

1. [`docs/agent-handoff.md`](docs/agent-handoff.md) — entry point
   for autonomous work. Names the latest pushed state, current
   blockers, files to inspect first, commands to run, the
   non-negotiable safety constraints, and a suggested
   implementation order.
2. [`docs/launch-candidate-checklist.md`](docs/launch-candidate-checklist.md)
   — bound checklist with per-group definitions of done.
3. [`docs/official-clockify-mcp-gap-analysis.md`](docs/official-clockify-mcp-gap-analysis.md)
   — narrative readiness ladder (community / internal alpha /
   official launch).
4. [`docs/adr/0017-streamable-http-session-rehydration.md`](docs/adr/0017-streamable-http-session-rehydration.md)
   — Accepted; Path A landed. Read this before touching session
   state.
5. [`docs/live-tests.md`](docs/live-tests.md) — how the
   live-contract nightly works and how the **sacrificial
   workspace** is wired. Read this before any live Clockify call.

If a contributor maintains a workstation-private `CLAUDE.md` at the
repo root, it is gitignored and machine-specific; treat it as
optional context, not as a source of binding rules. The binding
rules live in this file and the docs above.

## Latest pushed state

- **HEAD:** `f33b113 chore(launch): close groups 2/3/4, wire
  schema-diff, harden forward_auth` (pushed to `origin/main` on
  2026-05-02).
- **Closed launch-candidate groups:**
  - **Group 2 — Shared-service Postgres E2E.** Lives at
    `internal/controlplane/postgres/e2e_shared_service_test.go`
    (`make shared-service-e2e`); runs per-PR as the
    `Shared-service Postgres E2E` job in
    `.github/workflows/ci.yml`; **promoted to required-status
    check on `main`** (commit `50aa87f`) after three consecutive
    green runs.
  - **Group 3 — ADR 0017 Path A.** `streamSessionManager.get`
    rehydrates from the shared `controlplane.Store` on local
    miss, strict-validates the principal vs persisted
    Subject/TenantID, and reuses the principal-aware Factory.
    Pinned by `TestStreamableHTTPCrossInstanceRehydration`. The
    `sessionAffinity: ClientIP` band-aid stays as
    defence-in-depth.
  - **Group 4 — Auth-model docs.** [`docs/auth-model.md`](docs/auth-model.md)
    is the one-page reviewer summary; cross-linked from
    `docs/production-readiness.md` and
    `docs/runbooks/auth-failures.md`. `forward_auth` rejects
    duplicated identity headers and principal values >1024 bytes
    (pinned by
    `TestForwardAuth_RejectsDuplicatedAndOversizedHeaders`).
  - **Group 5 — Launch docs.** Per-profile "How to verify this
    deployment" sections; client matrix in
    [`docs/clients.md`](docs/clients.md) names tested
    transport/auth combos; [`docs/support-matrix.md`](docs/support-matrix.md)
    pins Go / OS / FIPS / kernel posture.
- **Read-side schema diff** (`tests/e2e_live_schema_test.go::TestLiveReadSideSchemaDiff`)
  is wired into the read-only step of
  `.github/workflows/live-contract.yml`. It needs scheduled-cron
  evidence on the candidate SHA before Group 1 fully closes.

## Remaining launch blockers

Listed in priority order; full detail in
[`docs/agent-handoff.md`](docs/agent-handoff.md).

1. **Group 1 — scheduled-cron evidence on the candidate SHA.**
   Two consecutive *scheduled* (cron) green runs of
   `live-contract.yml` are required, including the
   `TestLiveReadSideSchemaDiff` evidence. The rolling
   `live-test-failure` issue is currently closed; two manual-dispatch
   runs are green; cron is calendar-bound.
2. **Group 6 — security walk-through on the candidate tag.**
   Re-run `make verify-vuln`, `make verify-fips`, gitleaks, and
   semgrep on the final candidate tag and file any findings in
   `SECURITY.md`. The same suite was last green on 2026-05-02 on
   the launch-review tree.
3. **Group 7 — bench / release readiness on the candidate tag.**
   Cut `vX.Y.Z-rc.N`, watch `release-smoke.yml`, verify
   sigstore + SLSA artefact attestation, and archive the
   `doctor --strict` output alongside the release notes.

## Safety constraints (non-negotiable)

These constraints apply to any autonomous agent. They override
convenience.

1. **Do not declare launch-ready until live-contract +
   shared-service Postgres E2E + CI on `main` are simultaneously
   green** on the candidate SHA. Local `release-check` is
   necessary, not sufficient.
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
   record them in the `Verified:` line.
4. **Keep generated docs in lockstep with the source.** Any
   change to `internal/config/spec.go` or to a tool descriptor
   must re-run `go run ./cmd/gen-config-docs -mode=all` and
   `make gen-tool-catalog` in the same commit. CI gates
   `config-doc-parity` and `catalog-drift` will reject partial
   updates.
5. **Do not run destructive live Clockify calls outside the
   sacrificial workspace.** The only approved workspace is the
   one named in [`docs/live-tests.md`](docs/live-tests.md),
   reachable via `CLOCKIFY_LIVE_API_KEY` +
   `CLOCKIFY_LIVE_WORKSPACE_ID`. Do not point those secrets at
   any personal, teammate, or production workspace. When in
   doubt: read-only only.
6. **Do not skip git hooks.** No `--no-verify`, no
   `--no-gpg-sign`. If a hook fails, fix the underlying issue.
7. **Atomic commits, atomic pushes.** One logical change per
   commit; the body ends with `Why:` and `Verified:` lines. When
   landing a multi-commit wave, push only when the whole group
   is green locally.
8. **Do not modify generator-owned files by hand.** Listed in
   `CONTRIBUTING.md` and the workstation `CLAUDE.md` if present.
9. **Do not invent commands.** If a command is not in
   `Makefile`, `.github/workflows/`, or the docs, propose it as
   a Makefile target first.
10. **Never print, echo, commit, or log API keys or tokens** —
    even disposable ones for sacrificial workspaces. Reference
    them by env-var name only.

## Tight-loop commands

| Goal | Command |
|------|---------|
| Fast inner loop | `make check` (fmt + vet + test) |
| Pre-ship local gate | `make release-check` |
| Coverage | `make cover` |
| Single test | `go test -race -run TestName ./path/...` |
| With gRPC transport | append `-tags=grpc` to `go test` / `go build` |
| With Postgres backend | `make build-postgres` / `make test-postgres` |
| Shared-service Postgres E2E | `make shared-service-e2e` |
| Streamable-HTTP smoke | `make http-smoke` |
| Stdio smoke | `make stdio-smoke` |
| gRPC auth smoke | `make grpc-auth-smoke` |
| Strict deploy gate (config) | `clockify-mcp doctor --profile=<profile> --strict` |
| Strict deploy gate (backends) | `clockify-mcp-postgres doctor --profile=prod-postgres --strict --check-backends` |
| Live-contract tests (local pre-flight) | `make live-contract-local` with `CLOCKIFY_RUN_LIVE_E2E=1`, `CLOCKIFY_API_KEY`, `CLOCKIFY_WORKSPACE_ID` set against a sacrificial workspace. **Local green is NOT Group 1 launch-candidate evidence** — two consecutive scheduled-cron greens in `.github/workflows/live-contract.yml` on the candidate SHA are the authoritative bar. |
| Live-contract tests (read-only, raw) | `go test -tags=livee2e -run '^(TestE2EReadOnly\|TestE2EErrors\|TestLiveReadSideSchemaDiff)$' ./tests/...` — prefer `make live-contract-local` which wraps this with evidence warnings. |
| Refresh generated docs/help | `go run ./cmd/gen-config-docs -mode=all && make gen-tool-catalog` |
| Vuln scan | `make verify-vuln` |
| FIPS verify | `make verify-fips` |
| Bench baseline | `make verify-bench` then `make bench-baseline-check` |

## When you are uncertain

Stop and write down what you know in the commit message body.
The `Why:` line is the right place for "I am uncertain because
X" — hidden uncertainty is worse than documented uncertainty.

If the uncertainty is a security or default-weakening question:
**stop and ask the maintainer.** The cost of waiting is low; the
cost of guessing wrong is high.

## Local vs. CI evidence

`go test -tags=livee2e ./tests/...` without the required env vars
silently skips every live-contract test and reports `ok` in <0.5s.
The `TestLiveContractSkipSentinel` test (under the same build tag)
detects this and fails with an explicit message. If you see that
failure, set the env vars or stop claiming evidence.

Even with env vars set, a local green run is **not** Group 1
launch-candidate evidence. Only two consecutive scheduled (cron)
green runs of `.github/workflows/live-contract.yml` on the
candidate SHA count. Use `make live-contract-local` for pre-flight
debugging — it prints this warning automatically.

`make launch-checklist-parity` runs the evidence gate
(`scripts/check-launch-evidence-gate.sh`) which fails when a
launch-candidate-checklist box in Groups 1, 6, or 7 is checked
without an evidence URL, `workflow_run_id:`, or `_Closed_`
annotation. The gate's regression test
(`scripts/test-check-launch-evidence-gate.sh`) exercises the
fail-closed paths and runs as part of `make script-tests`.
