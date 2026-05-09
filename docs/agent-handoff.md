# Agent Handoff — Clockify MCP Launch Candidate Goal

You (Claude Code, Codex, or another autonomous coding agent) are
picking up the work to bring `github.com/apet97/go-clockify` from
"community-MCP ready / internal-support alpha" through the
launch-candidate evidence/review pass. Official-product promotion
still requires the external evidence and legal/product approval gates
named below.

This document is your entry point. Read it before doing anything,
then read the referenced docs, then do the smallest piece of useful
work and commit it.

> If you ignore the safety constraints in this document the
> maintainer will revert your work. They are not negotiable.

## Launch-state baseline

- **Current pushed main baseline:** `4fe957547f9e6aea749a85f87823d17a0ccc2928`
  (`fix(streamable): preserve session negotiation on touch`). This is
  the current `main` / `origin/main` tip before the uncommitted May 8
  remediation tree. It includes the post-PR #63 local/CI remediation
  wave plus later review-readiness fixes. It does not represent the
  dirty local remediation tree, did not run live Clockify probes from
  this remediation state, and does not tick any external
  launch-evidence box.
- **Latest manual live-campaign baseline:** `ff0047aa50cdcd4bb43037c72d66b218d51f13e8`
  (`test(livee2e): pin user invite validation`). This records the
  manual sacrificial-workspace campaign state after PR #62: the
  PR #59 catalog snapshot (then 121 generated tools) was live-probed
  through the MCP path, the documented invite-user route is pinned by
  a no-email validation probe, stale non-catalog user-invite risk
  overrides are removed, and risk overrides now fail if they target
  ghost descriptor names. The current catalog is 128 tools after two
  Tier 1 timesheet workflow helpers plus five local discovery,
  activation, and name-resolution helpers were added on top of that
  raw API coverage surface. The local helpers are unit tested; the
  timesheet workflow helpers also have live-test hooks.
  This is manual campaign coverage only and does not tick any
  external launch-evidence box.
- **Historical baselines, not current candidate SHAs:** post-PR #63
  remediation `0960bfa03db143778deb59f9b9522012116c9c9b`
  (`chore(review): harden MCP server readiness`), PR #60 docs
  stabilization `4e69c7a1db8011055187cf9892426ed48fc8e572`
  (`docs(handoff): update post-PR59 launch state`) and PR #51 merge
  tip `adce316d60644fe51365086aba186227c9ae3977`
  (`docs(launch): record bench comparison evidence`) are retained for
  audit continuity only. Do not cite either as the current
  launch-state baseline.
- **Closed locally:** Groups 2 (shared-service Postgres E2E,
  required-gated on `main`), 3 (ADR 0017 Path A — streamable-HTTP
  cross-instance session rehydration), 4 (auth-model docs +
  `forward_auth` cardinality/size guard), 5 (per-profile "How to
  verify this deployment" sections, client matrix, support matrix),
  false-green live-contract prevention, launch-evidence parity gate,
  benchmark baseline refresh (`bench-current-25255062599` +
  comparison run 25255216987), and PR #59's exhaustive manual
  sacrificial-workspace probe across the then-current generated
  catalog, PR #62's invite-user route validation probe, and later
  timesheet workflow helper unit/live-hook coverage. These manual
  probes are coverage evidence only and do not tick any external
  launch-evidence box.
- **Open launch-evidence gates (not a complete blocker list):**
  - **Scheduled live-contract cron greens** — two consecutive
    *scheduled* runs of `live-contract.yml` on the candidate SHA,
    with `TestLiveReadSideSchemaDiff`, mutating tests, and the
    audit-phase tier captured. The rolling `live-test-failure`
    issue is closed; two manual-dispatch runs are green; cron is
    calendar-bound.
  - **Candidate-tag security walk-through** — re-run
    `make verify-vuln`, `make verify-fips`, gitleaks, and Semgrep
    on the final candidate tag. Local preflight was green on
    2026-05-02, and Semgrep now has a recurring CI workflow artifact,
    but candidate-tag evidence is still required.
  - **Release/sigstore/SLSA evidence** — cut `vX.Y.Z-rc.N`, run
    `release-smoke.yml`, verify sigstore + SLSA artefact
    attestations, and archive the reference `doctor --strict`
    outputs.

If a local-shell run of the live-contract suite reports `ok`
suspiciously fast (≤ ~0.5s), the env-var gate
(`CLOCKIFY_RUN_LIVE_E2E=1` + `CLOCKIFY_API_KEY` +
`CLOCKIFY_WORKSPACE_ID`) was not visible to the test process and
it took the silent skip path — `live-contract.yml` is the
authoritative evidence path.

`TestLiveContractSkipSentinel` (under `-tags=livee2e`) now fails
explicitly when every live test skipped, so `go test -tags=livee2e
./tests/...` without env vars reports FAIL instead of a misleading
`ok`. Use `make live-contract-local` for pre-flight debugging — it
wraps the test run with evidence warnings.

## Read first (in this order)

1. [`../AGENTS.md`](../AGENTS.md) — standard agent-spec
   entrypoint at the repo root with the binding safety
   constraints and tight-loop commands. **Always tracked.** If a
   workstation `CLAUDE.md` also exists it is gitignored
   per-workstation context, not a source of binding rules.
2. [`launch-candidate-checklist.md`](launch-candidate-checklist.md)
   — the bound list of what must be true to declare launch
   candidate.
3. [`launch-readiness-review-may-8.md`](launch-readiness-review-may-8.md)
   — disposition ledger for the 2026-05-08 review folder. It names
   what this remediation pass closed, what was already closed, what
   remains code/CI backlog, and what requires external evidence or
   legal/product/maintainer approval. It also includes the current
   objective-to-artifact completion audit; do not mark the launch
   objective complete while any row in that audit is open.
4. [`official-clockify-mcp-gap-analysis.md`](official-clockify-mcp-gap-analysis.md)
   — the narrative of where the project is, what is strong, and
   what blocks tier 3 readiness.
5. [`adr/0017-streamable-http-session-rehydration.md`](adr/0017-streamable-http-session-rehydration.md)
   — Accepted; Path A landed. Read this before touching session
   state.
6. [`live-tests.md`](live-tests.md) — how the live-contract
   nightly works and how the sacrificial workspace is wired.
7. [`deploy/production-profile-shared-service.md`](deploy/production-profile-shared-service.md)
   — the deployment shape that the launch candidate is built
   around.
8. [`claude-code-continuation.md`](claude-code-continuation.md) —
   historical Claude Code continuation packet from the post-PR #51
   state. Do not run its prompt blocks as written.

## Current known blockers

The May 8 review remediation closed several local code/docs/test
findings, but the repo is not launch-ready until the following
evidence and approval gates are closed. Additional non-blocking
code/CI hardening backlog is tracked in
[`launch-readiness-review-may-8.md`](launch-readiness-review-may-8.md).

1. **Scheduled live-contract cron evidence on the candidate SHA.**
   The rolling `live-test-failure` issue is closed (auto-closed by
   manual run 25238997088). Two manual-dispatch runs are green
   (read-only 25238997088, full-tier 25239216412).
   `TestLiveReadSideSchemaDiff` is wired into the read-only step of
   `.github/workflows/live-contract.yml`. What is still open: two
   consecutive **scheduled** (cron) green runs of
   `live-contract.yml` on the candidate SHA, with schema-diff,
   mutating, and audit-phase evidence captured. Rechecked on
   2026-05-09: scheduled runs 25593042387 and 25538247771 are green on
   `4fe957547f9e6aea749a85f87823d17a0ccc2928` and their logs include
   `TestE2EMutating`, `TestLiveCreateUpdateDeleteEntryAuditPhases`,
   and `TestLiveReadSideSchemaDiff`, but that SHA is not this newer
   dirty remediation tree. Those runs prove the audit-phase DSN path
   for the pushed HEAD only. Use `/fix-live-contract` only if a future
   cron firing reds.
2. **Candidate-tag security walk-through.** Local launch-review
   preflight was green on 2026-05-09 after moving to Go 1.25.10 and
   tagged `govulncheck@v1.3.0`; the final candidate tag still needs
   `make verify-vuln`, `make verify-fips`, gitleaks, and Semgrep
   evidence. The Semgrep workflow artifact still needs its first
   pushed-run evidence and does not replace the candidate-tag scan.
   File findings or explicit "no findings" evidence in `SECURITY.md`.
3. **Release/sigstore/SLSA evidence.** The candidate tag still
   needs `release-smoke.yml`, sigstore/SLSA/SBOM verification, and
   archived `doctor --strict` outputs for the reference deployment.
4. **Externally visible repo-state cleanup.** The May 8 review
   ledger records maintainer-owned cleanup still outside local code:
   stale GitHub repository description wording, stale issue #28,
   private-repo branch-protection API limitation, and first pushed-run
   evidence on the final candidate SHA for the new CodeQL,
   dependency-review, and Semgrep workflow files. Rechecked on
   2026-05-09: the repo is still private,
   its description still has stale tool-count / policy-count wording,
   issue #28 is still open, and `gh run list` returns
   workflow-not-found 404s for `codeql.yml`, `dependency-review.yml`,
   and `semgrep.yml` because those workflow files are present only in
   this local remediation tree. `make launch-external-status` now
   directly checks `CLOCKIFY_LIVE_AUDIT_REQUIRED=true` and prints
   maintainer action hints for each open gate while staying read-only.
5. **Mutation cron evidence.** The local workflow timeout was raised
   for the slow `internal/tools` leg, but `make launch-external-status`
   recheck on 2026-05-09 shows latest scheduled mutation run
   25592823559 is `completed/cancelled` on pushed commit
   `4fe957547f9e6aea749a85f87823d17a0ccc2928`. `gh run view` shows the
   `internal/tools` matrix leg was cancelled while the other mutation
   legs succeeded. Wait for scheduled-run evidence on the final
   candidate SHA after the workflow change lands.
6. **Paid-hosted external review and legal/commercial gates.** A paid
   hosted launch still needs the final plan's non-code evidence:
   third-party or peer security review recorded in `SECURITY.md`
   against the candidate tag, DPA / customer terms, privacy and
   data-handling review by counsel, and the trademark / official
   Clockify framing decision. Local tests and repo docs cannot close
   these gates.

## Local commit-readiness checkpoint

The uncommitted May 8 remediation tree has been locally checked for
landing hygiene. This is not launch evidence, but it is the current
local handoff state before any commit/push:

- `GOTOOLCHAIN=go1.25.10 make release-check` passed after the latest
  verifier/doc edits, including coverage floors (total 79.6%),
  config/doc/catalog/gRPC release parity, repo hygiene, script tests,
  Go-version parity, build-tag wiring, HTTP and stdio smokes, strict
  doctor smoke, gRPC-tagged race E2E tests, Kustomize render, Helm
  lint, and Helm template validation.
- `make doc-parity`, `make config-doc-parity catalog-drift
  doc-parity`, `make shellcheck`, and `git diff --check` passed.
- Pinned workflow lint passed: installed
  `github.com/rhysd/actionlint/cmd/actionlint@914e7df21a07ef503a81201c76d2b11c789d3fca`
  into a temporary `GOBIN` with `GOTOOLCHAIN=go1.25.10`, then ran
  `<tmp>/actionlint -color=false .github/workflows/*.yml` with no
  findings.
- `make script-tests` passed, including the 69-case doc-parity suite,
  the 39-case launch-review ledger suite, the 20-case
  launch-external-status suite, public-content audit, Go-version
  parity, live-tool coverage, license-evidence, and RC-evidence helper
  suites.
- `make bench-baseline-check` passed against the committed
  `internal/benchdata/baseline.txt` (Linux/amd64, 10-sample minimum).
  Do not treat a macOS/arm64 local `verify-bench` comparison as release
  evidence; the CI bench workflow is the authoritative comparison for
  the committed baseline.
- `make rc-evidence-plan TAG=v1.2.1-rc.1` printed the expected
  Group 6/7 evidence plan, including the `release-smoke-doctor-output`
  artifact reminder, without dirtying the tree. This validates the
  planning entrypoint only; no RC tag was cut and no release evidence
  was collected.
- `make test-postgres` passed after starting the local Colima Docker
  runtime, exercising the Postgres control-plane package with
  Testcontainers and `INTEGRATION_REQUIRED=1`. The DSN-gated
  `make shared-service-e2e` still skips unless
  `MCP_LIVE_CONTROL_PLANE_DSN` points at a sacrificial Postgres.
- `make build-postgres build-grpc-postgres` passed. It left the
  expected ignored `clockify-mcp` local binary, which was removed with
  `make clean`; `git diff --check` remained clean.
- Local Group 6 preflight was refreshed after `release-check`:
  `GOTOOLCHAIN=go1.25.10 make verify-vuln verify-fips` passed,
  pinned `govulncheck@v1.3.0` found no vulnerabilities, FIPS-tagged
  tests passed, and Semgrep `p/default` scanned 1153 tracked files
  plus `.github/workflows/semgrep.yml` with 0 findings. This is not
  candidate-tag evidence.
- `bash scripts/check-live-tool-coverage.sh` reports
  `0 open, 0 unknown`: all 88 Tier-2 tools and all API-backed Tier-1
  tools are named in live E2E source, the four local-only Tier-1
  helpers are explicitly allowed, and no unknown `clockify_*` live-test
  references exist. This does not replace scheduled cron evidence.
- Generator idempotence was checked by rerunning
  `go run ./cmd/gen-config-docs -mode=all` and
  `make gen-tool-catalog`, then comparing working-tree checksums for
  `cmd/clockify-mcp/help_generated.go`, `README.md`, and
  `docs/tool-catalog.{json,md}`.
- Candidate Markdown link hygiene was checked over tracked plus
  unignored Markdown files: 99 files, no missing relative targets.
  Offline `lychee` reported 390 total, 352 OK, 0 errors,
  38 excluded.
- Staging-manifest audit covered 198 candidate paths and found no
  nested checkout, local state, review scratch, benchmark/coverage
  artifact, macOS artifact, sensitive-extension file, or
  uncategorized path. The only env-like candidate path is the
  intentional deletion of `.env.example`. Current bucket counts:
  `.github/ISSUE_TEMPLATE` 3, `.github/codeql` 1,
  `.github/workflows` 14, root policy/build docs 10 including
  `.env.example`, `cmd` 5, `deploy` 11, `docs` 46, `internal` 70,
  `scripts` 26, `tests` 6, `tools` 3, plus `go.mod` and `go.work`.
- Post-`release-check` artifact hygiene was rechecked:
  `git ls-files -o --exclude-standard` shows 42 intended untracked
  commit-candidate files; no unignored `coverage.out`, `.bench`,
  `.DS_Store`, local loop-artifact directory, env, secret, token, tmp,
  bin, dist, `.out`, or `.test` candidates were present. Ignored local
  artifacts are covered by `.gitignore`.
- `bash scripts/check-launch-external-status.sh --fail-open` exits
  nonzero with `11 open, 0 unknown`; `bash
  scripts/check-public-content-audit.sh --fail-open` exits 0 with
  `0 open, 0 unknown`.
- `bash scripts/collect-license-evidence.sh --fail-missing-license`
  reports `0 module(s) without local license candidates,
  0 unknown variant(s)`. This is raw evidence input only, not legal
  advice or license clearance.
- The actual review source bundle (`~/Downloads/review may 8`) still
  has the four expected files and 86 finding IDs; the ledger has the
  same 86 IDs and `bash scripts/check-launch-review-ledger.sh` passes.

For Claude Code work, start from this handoff and substitute the
actual candidate SHA you are evaluating (`git rev-parse HEAD` or the
release-candidate tag target). Do **not** use the historical PR #51
SHA in [`claude-code-continuation.md`](claude-code-continuation.md).

For the full audit framing run `/launch-candidate` from a Claude
Code session inside this repo. Keep workstation-private slash-command
files out of launch commits.

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
- `make test-postgres` is now self-contained for local launch
  verification: under `-tags=postgres,integration`, the shared-service
  E2Es reuse the package Testcontainers DSN, and the Makefile target
  normalizes Unix Docker sockets for Colima / Docker Desktop.
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
  `authn.go:36-41`, `forward_auth` header cardinality/size guard
  in `forwardAuthHeaderValue`).
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
| Live-contract local pre-flight | `make live-contract-local` (prints evidence warnings; **local green is not Group 1 evidence**) |
| Live-contract tests (read-only, raw) | `go test -tags=livee2e -run '^(TestE2EReadOnly\|TestE2EErrors\|TestLiveReadSideSchemaDiff)$' ./tests/...` with `CLOCKIFY_RUN_LIVE_E2E=1`, `CLOCKIFY_API_KEY`, `CLOCKIFY_WORKSPACE_ID` set against a sacrificial workspace |
| Live-contract tests (mutating, sacrificial only) | append `-run '^TestE2EMutating$\|^TestLiveDryRunDoesNotMutate$\|^TestLivePolicyTimeTrackingSafeBlocksProjectCreate$'` and only against the workspace named in `docs/live-tests.md` |
| External launch-status snapshot | `make launch-external-status` (read-only; reports GitHub/npm/local-branch-facing evidence gates, including the `CLOCKIFY_LIVE_AUDIT_REQUIRED=true` repo variable, with maintainer action hints and exits 0 unless called with `scripts/check-launch-external-status.sh --fail-open`) |
| Public repo content audit | `make public-content-audit` (read-only; reports redacted gitleaks metadata, tracked scratch/personal references, env-like files, internal/private task-marker hits, and commit-message review hits; summary separates candidate branch file content from public-history and local-artifact review; exits 0 unless called with `scripts/check-public-content-audit.sh --fail-open`) |
| Local ignored-artifact cleanup | `make clean-deep CONFIRM=1` (destructive; removes ignored build/scratch artifacts only, including `.local/`, `.serena/`, and duplicate `go-clockify/`; do not run without maintainer approval) |
| Doctor (config-strict) | `clockify-mcp doctor --profile=<profile> --strict` |
| Doctor (backends) | `clockify-mcp-postgres doctor --profile=prod-postgres --strict --check-backends` |
| Refresh generated docs | `go run ./cmd/gen-config-docs -mode=all && make gen-tool-catalog` |
| Vuln scan | `make verify-vuln` |
| FIPS verify | `make verify-fips` |
| Bench | `make verify-bench` then `make bench-baseline-check` |
| Mutation testing | `make mutation` |

## Non-negotiable safety constraints

These are restated from `AGENTS.md` and the launch checklist. If
a constraint conflicts with a task, the task is wrong, not the
constraint.

1. **Do not declare launch-ready until live-contract + shared-service
   Postgres E2E + CI on `main` are simultaneously green and
   candidate-tag security plus release/sigstore/SLSA evidence exists.**
   Local `release-check` is necessary, not sufficient.
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
   `CONTRIBUTING.md` and `AGENTS.md`.
9. **Do not invent commands.** If a command is not in `Makefile`,
   `.github/workflows/`, or the docs, propose it as a Makefile
   target first.
10. **Only workstation-private context is gitignored.** `CLAUDE.md`
    and local Claude Code slash-command files may exist locally; do
    not commit them.
    `AGENTS.md` and this handoff are tracked.

## Suggested continuation order

The local high-impact implementation queue is empty. Continue with
local code only if a new review finding appears or a deferred
low-risk item becomes evidence-backed; otherwise focus on the
external evidence gates below.

1. **Land the remediation tree only after explicit approval.** Before
   staging, rerun `git status --short --branch`, `make doc-parity`,
   `git diff --check`, and any narrow tests touched by last-minute
   edits. Do not use `git add .` from a parent workspace. Commit from
   this repo root only, preserve generated-file lockstep, and end the
   commit body with `Why:` and `Verified:` lines. Do not push until the
   whole staged group is locally green.
2. **Refresh external status on the landed SHA.** After the commit is
   pushed, run `make launch-external-status` and then
   `bash scripts/check-launch-external-status.sh --fail-open`. The
   dirty-tree gate should close only when `HEAD` represents the
   remediation tree; workflow first-run, mutation, live-contract,
   repository metadata, issue, branch-protection, npm, release, and
   legal/product gates still need their own evidence.
3. **Freeze `main` while Group 1 is pending.** After the remediation
   tree lands, avoid unrelated default-branch churn until the two
   scheduled `live-contract.yml` runs for that final SHA either close
   Group 1 or produce a focused fix. This is an operator coordination
   rule, not a local git setting changed by agents.
4. **Audit scheduled live-contract evidence.** Start with
   `make launch-external-status` for the read-only snapshot, then use
   `gh run list --workflow=live-contract.yml --branch=main --limit 10`
   and the rolling `live-test-failure` issue for detailed evidence. If
   two consecutive scheduled runs are green on the candidate SHA and
   the run logs show schema-diff, mutating, and audit-phase tests
   executed, update the launch checklist with the exact run URLs.
5. **Perform candidate-tag security walk-through.** Start with
   `docs/runbooks/release-candidate-evidence.md` and rehearse with
   `make rc-evidence-plan TAG=vX.Y.Z-rc.N`. On the final candidate
   tag, re-run `make verify-vuln`, `make verify-fips`,
   `make secret-scan`, and Semgrep. Record findings or "no
   findings" in `SECURITY.md` and link the evidence from the
   checklist.
6. **Cut the release candidate and verify artefacts.** After live
   contract and security evidence exist, cut `vX.Y.Z-rc.N`, run
   `scripts/prepare-rc-evidence.sh vX.Y.Z-rc.N` from a clean tag
   checkout on the required hosts, watch `release-smoke.yml`, verify
   sigstore/SLSA/SBOM evidence, and archive the
   `release-smoke-doctor-output` reference `doctor --strict` outputs.
7. **Open the launch-candidate tracking issue.** Link every green
   workflow run and archived output. Only after all links exist may
   any agent or human report "launch candidate ready."

## When you are uncertain

Stop and write down what you know in the commit message body.
The `Why:` line is the place for "I am uncertain because X" —
hidden uncertainty is worse than a documented one.

If the uncertainty is a security or default-weakening question:
**stop and ask the maintainer.** The cost of waiting is low; the
cost of guessing wrong is high.
