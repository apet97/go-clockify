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

- **Latest code-bearing launch baseline:** `308c81560a75db037dfdaf306ac04afb48a5cff6`
  (`chore(launch): harden hosted candidate readiness`). This is the
  May 9 hosted/public hardening and dependency-security push. Later
  docs-only evidence commits may move `main`; always verify the
  current SHA with `git ls-remote origin refs/heads/main` before
  binding scheduled or manual workflow evidence to a candidate. Manual
  `live-contract.yml` dispatches on May 9 are useful candidate-now
  evidence, but they do not replace the scheduled evidence that closed
  Group 1.
- **Latest scheduled live-contract evidence SHA:**
  `feef83c641ced93d2ab6ba07ef766d61c82cc703`
  (`ci(live): add temporary launch evidence cron`). Scheduled
  `live-contract.yml` runs 25608259477 and 25607242862 are consecutive
  greens on this SHA and include `TestLiveReadSideSchemaDiff`,
  `TestE2EMutating`, the MCP-path safety contracts, and
  `TestLiveCreateUpdateDeleteEntryAuditPhases`. The temporary
  high-frequency cron was removed after these runs were archived.
- **Latest manual live-campaign baseline:** `ff0047aa50cdcd4bb43037c72d66b218d51f13e8`
  (`test(livee2e): pin user invite validation`). This records the
  manual sacrificial-workspace campaign state after PR #62: the
  PR #59 catalog snapshot (then 121 generated tools) was live-probed
  through the MCP path, the documented invite-user route is pinned by
  a no-email validation probe, stale non-catalog user-invite risk
  overrides are removed, and risk overrides now fail if they target
  ghost descriptor names. The current catalog is 155 tools after two
  Tier 1 timesheet workflow helpers, five local discovery/activation
  and name-resolution helpers, expanded client/project/task/admin and
  reports coverage, and the latest documented probe-lab route refresh
  were added on top of that raw API coverage surface. The local
  helpers are unit tested; API-backed additions have live-test hooks
  where the upstream workspace permits them.
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
- **Closed locally:** Group 1 (scheduled live-contract evidence),
  Groups 2 (shared-service Postgres E2E,
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
  - **Candidate-tag security walk-through** — closed for
    `v1.2.1-rc.3` (peeled commit
    `ce56414ae012c4a49d21ae0a319b178619c5966a`) via PR #84. The
    fresh-worktree transcript of `make check`, `make verify-vuln`,
    `make secret-scan`, the canonical
    `semgrep scan --config p/default ...` invocation, the
    `git grep -n -C 5 nosemgrep` enumeration, and `make verify-fips`
    is captured on host short name `192` on 2026-05-10 between
    01:55 UTC and 01:59 UTC; full output is in
    [`SECURITY.md`](../SECURITY.md) § "Candidate-tag security
    evidence" and the four candidate-tag-dependent Group 6 boxes
    (`make verify-vuln`, gitleaks, Semgrep, `make verify-fips`) in
    [`launch-candidate-checklist.md`](launch-candidate-checklist.md)
    are now ticked. Re-run the same suite on any future candidate tag.
  - **Release/sigstore/SLSA evidence** — partially closed
    2026-05-10 for `v1.2.1-rc.3` (peeled commit `ce56414`). All
    five tag-triggered + dispatched workflows on rc.3 completed
    `success` in a single attempt: Release run 25616879096, Docker
    Image run 25616879055, Deploy run 25616879075, Reproducibility
    run 25616925376 (all 9 matrix jobs match released bytes), and
    release-smoke run 25616925600. Manual cosign + SLSA + container
    image verification passed against the documented certificate
    identities (default linux-x64, postgres-linux-x64,
    fips-linux-x64, darwin-arm64). The
    `release-smoke-doctor-output` artifact is archived with all
    three required `doctor --strict` files. The Group 7 release
    artefact and doctor-strict checklist boxes are ticked. Two
    Group 7 boxes remain open: `make release-check` from clean
    checkouts on Linux x64 and macOS arm64 (this lane is
    single-host darwin-arm64; macOS arm64 release-check itself
    hit a transient TMPDIR script-tests race and reproduced
    clean on re-run), and "All required workflows on `main`
    green" (mutation.yml's next scheduled cron green on the
    final candidate SHA is still pending). See
    [`runbooks/release-candidate-evidence.md`](runbooks/release-candidate-evidence.md)
    "v1.2.1-rc.3 evidence record" and
    [`launch-readiness-review-may-8.md`](launch-readiness-review-may-8.md)
    § "Final integration audit — rc.3 cycle ledger" for the
    consolidated rc.3 cycle PR ledger (#74, #75, #76, #77, #80,
    #83, #84, #85), validator-quirk classifications, and
    next-step actions on the still-open gates. **`v1.2.5` is the
    current stable community/self-hosted AIII-backed API-refresh
    line**, released 2026-05-13 with the 192-operation generated
    OpenAPI artifact. The original community/self-hosted launch
    evidence remains anchored to
    `v1.2.1`, cut from rc.3's peeled commit `ce56414` and released
    2026-05-10 (see
    [`runbooks/release-candidate-evidence.md`](runbooks/release-candidate-evidence.md)
    § "v1.2.1 release evidence record (2026-05-10)" for the
    canonical evidence anchor — including the bounded
    release-smoke SAN exception and the
    `ghcr.io/apet97/go-clockify:1.2.1` container image identity).
    Paid-hosted / commercial / "official Clockify" follow-ups
    remain explicitly deferred per
    [`launch-readiness-review-may-8.md`](launch-readiness-review-may-8.md)
    § "Deferred paid-hosted/commercial follow-ups — not required
    for community/self-hosted v1.2.1" and must not be re-promoted
    into community/self-hosted blockers.

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

1. **Candidate-tag security walk-through.** Local launch-review
   preflight was green on 2026-05-09 after moving to Go 1.25.10 and
   tagged `govulncheck@v1.3.0`. Closed for `v1.2.1-rc.3` on
   2026-05-10 via PR #84: a fresh-worktree run on the rc.3 peeled
   commit `ce56414ae012c4a49d21ae0a319b178619c5966a` (host short
   name `192`) recorded `make check`, `make verify-vuln`,
   `make secret-scan`, `semgrep scan --config p/default
   --metrics=off --error --exclude .git --exclude .bench
   --exclude clockify-mcp .`,
   `git grep -n -C 5 nosemgrep -- ':!CHANGELOG.md'`, and
   `make verify-fips` with explicit "no findings" evidence in
   [`SECURITY.md`](../SECURITY.md) § "Candidate-tag security
   evidence". Future candidate tags must re-run the same suite. The
   Semgrep workflow artifact still needs its first pushed-run
   evidence and does not replace the candidate-tag scan.
2. **Release/sigstore/SLSA evidence.** Partially closed 2026-05-10
   for `v1.2.1-rc.3` (peeled commit
   `ce56414ae012c4a49d21ae0a319b178619c5966a`). All five
   tag-triggered + dispatched workflows on rc.3 completed
   `success` in a single attempt: Release ([25616879096](https://github.com/apet97/go-clockify/actions/runs/25616879096)),
   Docker Image ([25616879055](https://github.com/apet97/go-clockify/actions/runs/25616879055)),
   Deploy ([25616879075](https://github.com/apet97/go-clockify/actions/runs/25616879075)),
   Reproducibility ([25616925376](https://github.com/apet97/go-clockify/actions/runs/25616925376)),
   release-smoke ([25616925600](https://github.com/apet97/go-clockify/actions/runs/25616925600)).
   Manual `cosign verify-blob` + `gh attestation verify` succeeded
   on the default linux-x64, postgres-linux-x64, fips-linux-x64,
   and darwin-arm64 binaries; `cosign verify
   ghcr.io/apet97/go-clockify:1.2.1-rc.3` (manifest digest
   `sha256:374fbfb4bc18fd14a2fcd39fcae6c8da4054df3c162596ad476c15947b8a351f`)
   passed against the docker-image.yml certificate identity. The
   `release-smoke-doctor-output` artifact contains
   `release-doctor-strict-ok.txt`,
   `release-doctor-strict-fail.txt`, and
   `release-doctor-postgres-ok.txt`. The two Group 7 boxes that
   remain open are `make release-check` on Linux x64 + macOS
   arm64 hosts and "All required workflows on `main` green"
   (the next scheduled `mutation.yml` cron on the final candidate
   SHA is still pending). See
   [`runbooks/release-candidate-evidence.md`](runbooks/release-candidate-evidence.md)
   "v1.2.1-rc.3 evidence record" for the full evidence list.
3. **Externally visible repo-state cleanup.** Rechecked on 2026-05-09
   after `a07443b` landed: CodeQL run 25609129989, Dependency Review
   run 25609129978, and Semgrep run 25609129983 are green on the
   pushed candidate SHA. The maintainer-owned cleanup pass on the same
   day closed the description, issue #28, and branch-protection rows
   that were still open at the start of the day:
   the GitHub repository description was set to
   `128 tools, three transports (stdio / streamable HTTP / optional gRPC), five policy modes, cosign-signed releases.`;
   issue #28 was closed with a comment linking commit `50aa87f`, the
   `Shared-service Postgres E2E` required-status check, and
   `internal/controlplane/postgres/e2e_shared_service_test.go`; classic
   branch protection was re-applied via
   `gh api PUT repos/apet97/go-clockify/branches/main/protection` with
   the three D9 launch required-status checks
   (`Doctor strict smoke`, `Doctor Postgres backend`, and
   `Shared-service Postgres E2E`) plus the documented hygiene
   (linear history, conversation resolution, no force push, no
   deletion, dismiss stale reviews, strict up-to-date, 0 required
   approvals, code-owner reviews disabled, signed commits disabled,
   admin enforcement disabled). Restoring the historical 19-context
   required list documented in
   [`docs/branch-protection.md`](branch-protection.md) is a separate
   maintainer follow-up. Five fully-merged stale local branches
   (`codex/resolve-benchmark-worktree`, `stabilize/quality-perf`,
   `wave-a`, `wave-d`, `wave-e`) were deleted with `git branch -d`
   from this workstation; `docs/document-f3897b2-bypass`
   (`ahead=1`) and `fwbranch` (`ahead=20`) still hold non-main
   commits and remain in maintainer manual review.
   `make launch-external-status` directly checks
   `CLOCKIFY_LIVE_AUDIT_REQUIRED=true` and prints maintainer action
   hints for each open gate while staying read-only; after the
   2026-05-09 cleanup it reports `4 open, 0 unknown` from default
   HEAD evaluation, three of which are real launch gates (mutation
   cron evidence pending the next scheduled green on a final
   candidate SHA, six non-main local branches that are either active
   worktree branches or held for maintainer review, and the
   next-release npm expected-version proof) and one is a current-HEAD
   validator nuance — Group 1 evidence remains closed on canonical
   SHA `feef83c641ced93d2ab6ba07ef766d61c82cc703` via scheduled
   `live-contract.yml` runs 25608259477 and 25607242862.
4. **Mutation cron evidence.** The `internal/tools` matrix-leg
   timeout increase landed on `main` in `2e7b6bd` ("May 9 hardening")
   ahead of `308c815` and the later docs-only commits, so the
   workflow fix is on the default branch. `make launch-external-status`
   recheck on 2026-05-09 still shows latest scheduled mutation run
   25592823559 is `completed/cancelled` on pushed commit
   `4fe957547f9e6aea749a85f87823d17a0ccc2928` because that scheduled
   cron fired before `2e7b6bd` landed; `gh run view` confirms only the
   `internal/tools` matrix leg was cancelled while the other mutation
   legs succeeded. Wait for the next scheduled-run evidence on the
   final candidate SHA now that the workflow fix is on `main`.
5. **Paid-hosted external review and legal/commercial gates.** A paid
   hosted launch still needs the final plan's non-code evidence: the
   `Paid-hosted external security review`,
   `DPA / terms / privacy posture`,
   `Trademark / "official Clockify" language`,
   `Clockify URI scheme and gRPC service-name branding review`,
   `P1-8 paid-commercial RLS decision`, and
   `Cross-replica hosted HTTP quotas` external gates in
   `docs/launch-readiness-review-may-8.md`. Each gate now lists Owner,
   Evidence artifact, and Non-goal sub-bullets so a maintainer can
   route the request without inventing the format. Closure requires
   the named external reviewer's written decision archived per the
   listed evidence artifact; local tests, doc edits, and runbook
   pointers are inputs, not approvals.

## Current commit-readiness checkpoint

The May 9 hosted-readiness remediation tree landed on `main` as
`308c81560a75db037dfdaf306ac04afb48a5cff6`. These checks are landing
hygiene plus manual candidate verification, not a substitute for the
scheduled-cron, candidate-tag security, or release-evidence gates:

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
  references exist. The authoritative scheduled cron evidence is
  archived in live-contract runs 25608259477 and 25607242862.
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
  nonzero with `6 open, 0 unknown`; `bash
  scripts/check-public-content-audit.sh --fail-open` exits 0 with
  `0 open, 0 unknown`.
- Scheduled `live-contract.yml` runs 25608259477 and 25607242862 are
  consecutive greens on `feef83c641ced93d2ab6ba07ef766d61c82cc703`,
  including read-only, `TestLiveReadSideSchemaDiff`, mutating,
  MCP-path safety, and audit-phase steps. Manual dispatch
  25605467213 remains useful candidate-now evidence only.
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
3. **Keep post-Group-1 churn intentional.** Group 1 closed on
   scheduled runs 25608259477 and 25607242862; after the temporary
   cron-removal commit, avoid unrelated default-branch churn until
   Group 6/7 evidence and the remaining external repo-state gates have
   a clear owner. This is an operator coordination rule, not a local
   git setting changed by agents.
4. **Audit scheduled live-contract evidence.** Start with
   `make launch-external-status` for the read-only snapshot, then use
   `gh run list --workflow=live-contract.yml --branch=main --limit 10`
   and the rolling `live-test-failure` issue for detailed evidence.
   The archived Group 1 pair is 25608259477 and 25607242862 on
   `feef83c641ced93d2ab6ba07ef766d61c82cc703`; future agents should
   re-audit only if `live-contract.yml`, live tests, or candidate
   scope changes.
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
