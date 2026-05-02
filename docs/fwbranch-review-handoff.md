# fwbranch Review Handoff

Prepared 2026-05-02 for human + Claude + Codex review before merge to
`main`.

## Branch state

- **Branch:** `fwbranch`
- **Review base:** `origin/main` at `2e59e2b` (docs(agents): add durable launch handoff for AI agents)
- **Validated tip before bench refresh:** `6e21080` (docs(handoff): refresh fwbranch review state)
- **Review span:** 18 commits ahead of `origin/main` before the benchmark
  baseline refresh. If this file is read from the final bench-refresh commit,
  the branch has one additional generated-baseline/docs commit.

## Commits

```
6e21080 docs(handoff): refresh fwbranch review state
8fb625e docs(handoff): update push status after fwbranch push
70dc04e docs(checklist): cross-reference api-coverage.md from Group 1
ac4fc30 docs(agents): document evidence gate in Local vs CI evidence section
c31f64f docs(live): promote make live-contract-local as preferred local path
c74b803 test(scripts): add Group 7 box pattern to evidence gate regression tests
8e1eb2f test(scripts): add workflow_run_id and missing-file test cases to evidence gate regression
354d28f docs(handoff): add Wave 1 quality fixes summary to review handoff
58d8800 docs(api): fix missing tool and stale classification counts in coverage matrix
8c092c7 docs(gap): cross-reference api-coverage.md from gap analysis
c7be8ba docs(api): add per-tool dry-run breakdown and policy-mode coverage table
5b7e84b docs(handoff): sync fwbranch review handoff to HEAD at 60350da
60350da docs(api): add MCP API coverage matrix and fwbranch review handoff
bed943b scripts(test): add regression test for launch-evidence-gate
1e6faa7 scripts(parity): add launch-evidence-gate to prevent premature box-ticking
82452bf docs(agents): reference live-contract-local and skip sentinel
237f42a make: add live-contract-local target with evidence warning banner
43f6788 test(live): add skip sentinel to prevent false-green local evidence
```

## Change themes

### 1. False-green prevention (core theme)

The core theme is preventing skipped local live tests from being mistaken
for passing evidence. Before these changes, `go test -tags=livee2e
./tests/...` without the required env vars (`CLOCKIFY_RUN_LIVE_E2E=1`,
`CLOCKIFY_API_KEY`, `CLOCKIFY_WORKSPACE_ID`) would silently skip every
test and report `ok`, visually indistinguishable from a real green run.

**What changed (false-green prevention):**
- `tests/e2e_live_skip_sentinel_test.go` — `TestLiveContractSkipSentinel`
  fails explicitly when every `livee2e` test skipped, turning a misleading
  `ok` into an explicit `FAIL`
- `Makefile` — `live-contract-local` target wraps the test run with
  evidence-warning banners reminding the operator that local green is not
  Group 1 evidence
- `scripts/check-launch-evidence-gate.sh` — parity script that checks
  launch-candidate-checklist.md boxes against available workflow evidence
- `scripts/test-check-launch-evidence-gate.sh` — regression test for the
  evidence gate script
- `AGENTS.md` and `docs/agent-handoff.md` — document the new targets,
  skip-sentinel, and evidence hierarchy

### 2. API coverage and reviewer readiness

Commit `60350da` added two new docs for reviewer handoff, and the follow-up
docs commits corrected counts, coverage gaps, and cross-links:

- `docs/api-coverage.md` — maps all 124 MCP tools to Clockify API endpoints,
  classifies each as read-only/mutating/destructive/billing/admin, lists
  current test coverage per tool, documents schema-drift/dry-run/policy gaps,
  and establishes the evidence hierarchy (scheduled workflow > manual dispatch
  > local with env vars > local without env vars as non-evidence). Follow-ups
  added the missing `clockify_weekly_summary`, corrected Tier 2 counts, and
  added per-tool dry-run plus per-mode policy coverage tables.
- `docs/fwbranch-review-handoff.md` — this file; provides reviewer prompts
  for Claude and Codex/OpenAI, a human merge checklist, and live evidence
  caveats

### 3. Files changed

```
 AGENTS.md                                  |  25 +-
 Makefile                                   |  55 +++-
 docs/agent-handoff.md                      |   9 +-
 docs/api-coverage.md                       | 390 +++++++++++++++++++++++++++++
 docs/fwbranch-review-handoff.md            | 219 ++++++++++++++++
 docs/launch-candidate-checklist.md         |   4 +
 docs/live-tests.md                         |  16 ++
 docs/official-clockify-mcp-gap-analysis.md |  10 +
 scripts/check-launch-evidence-gate.sh      | 134 ++++++++++
 scripts/test-check-launch-evidence-gate.sh | 125 +++++++++
 tests/e2e_live_mcp_test.go                 |   2 +
 tests/e2e_live_schema_test.go              |   1 +
 tests/e2e_live_skip_sentinel_test.go       |  37 +++
 tests/e2e_live_test.go                     |   2 +
 14 files changed, 1054 insertions(+), 8 deletions(-)
```

## Checks run (2026-05-02)

| Check | Result |
|-------|--------|
| `make check` | OK — requires local port binding for `httptest` packages |
| `make doc-parity` | OK — includes launch checklist parity and evidence gate |
| `make config-doc-parity` | OK |
| `make catalog-drift` | OK |
| `make bench-baseline-check` | OK — after replacing `internal/benchdata/baseline.txt` with `bench-current-25255062599` |
| `Bench` workflow run 25255062599 | OK — bootstrap run on `fwbranch`, artifact `bench-current-25255062599` downloaded and validated |
| `Bench` workflow run 25255216987 | OK — normal comparison run on `da39381`; “Compare against committed baseline” passed |
| `make release-check` | OK — local macOS arm64 pre-ship gate; `golangci-lint` and `actionlint` skipped locally because not installed, CI enforces them |

## Checks not run

- `make verify-bench` to completion on this host — the benchmark capture
  ran, `bench-baseline-check` passed, then the target failed closed because
  this host emits `darwin/arm64` samples and the committed release baseline
  is `linux/amd64`. Use the CI `Bench` workflow for release-grade regression
  comparison evidence.
- `make live-contract-local` — requires sacrificial workspace credentials
  not available in this session
- `make verify-vuln` / `make verify-fips` — security scan tools not
  invoked in this pass
- `make shared-service-e2e` — requires Postgres DSN not available locally
- CI (`ci.yml`) — triggered on push to origin/fwbranch; check Actions tab for run status

## Live evidence caveats

1. **Local live-test `ok` is non-evidence.** Always confirm the env vars
   were visible — a sub-1s `ok` means skip path.
2. **Only scheduled cron green counts for Group 1.** Manual dispatches
   are design validation; scheduled runs are the launch gate.
3. **`TestLiveContractSkipSentinel`** now prevents the most dangerous
   false-positive class (all-skipped = `ok`), but does not prevent
   partial-skip false confidence (some tests running, others skipping
   silently).
4. **`make launch-evidence-gate`** is wired as a parity check and will
   flag any checklist box that references live-contract evidence without
   corresponding scheduled workflow green.

## API coverage caveats

- `docs/api-coverage.md` maps all 124 tools to Clockify endpoints,
  classification, and test coverage. Counts cross-verified against
  `docs/tool-catalog.json` on 2026-05-02.
- Only 9 of 124 tools (7%) have live-contract test coverage (all Tier 1)
- 0 of 91 Tier 2 tools have live coverage
- Schema-drift detection covers read-side (GET) endpoints only
- Dry-run: 6/14 destructive tools have `dry_run` wired; 1/14 live-tested
- Policy modes: 2/5 live-tested (standard, time_tracking_safe)
- Classification counts: 55 read-only, 55 mutating, 14 destructive,
  8 billing, 7 admin (counts verified against tool-catalog.json;
  previously stale at 58/46/14/6/4)

## Wave 1 quality fix (2026-05-02)

After the initial handoff (bed943b), four documentation issues were found
and fixed on fwbranch before review:

1. Handoff doc was stale at bed943b, missing the api-coverage commit
2. `clockify_weekly_summary` (Tier 1, read-only) was missing from the
   coverage matrix
3. Tier 2 classification counts were stale (read-only 39→35, mutating
   34→43, billing 6→8, admin 4→7)
4. Dry-run and policy-mode coverage lacked per-tool/per-mode breakdowns

## Files requiring careful review

| File | Why |
|------|-----|
| `tests/e2e_live_skip_sentinel_test.go` | New test — verifies sentinel fails when all skip. Review that it doesn't false-positive under legitimate partial-run scenarios. |
| `scripts/check-launch-evidence-gate.sh` | New parity script — parses launch-candidate-checklist.md and maps checkboxes to workflow evidence. Review the checkbox→evidence mapping logic. |
| `scripts/test-check-launch-evidence-gate.sh` | Regression test for the evidence gate — review that test fixtures accurately model real checklist state. |
| `Makefile` (live-contract-local target) | New target — review the evidence warning text and env-var passthrough. |
| `AGENTS.md` and `docs/agent-handoff.md` | Updated with evidence hierarchy — review for consistency with the launch checklist and optional workstation `CLAUDE.md`. |
| `docs/api-coverage.md` | Full 124-tool coverage matrix with endpoint mappings, classifications, dry-run/policy breakdown, and gaps. Classification counts cross-verified against tool-catalog.json. |
| `docs/launch-candidate-checklist.md` | Group 1 now cross-references `docs/api-coverage.md`; verify no external-evidence boxes were ticked prematurely. |

## Claude review prompt

```
Review the 17 commits on fwbranch (43f6788 through 8fb625e) for:

1. False-green prevention correctness: does TestLiveContractSkipSentinel
   correctly detect the all-skipped case without false-positiving when
   some live tests run and others legitimately skip?

2. Script safety: do check-launch-evidence-gate.sh and its regression
   test correctly parse the checklist and map boxes to evidence? Are
   there edge cases where a box could be ticked without evidence?

3. Makefile target correctness: does live-contract-local correctly
   pass through env vars? Does the evidence banner render correctly
   on both success and failure paths?

4. Doc consistency: do AGENTS.md, agent-handoff.md, live-tests.md,
   api-coverage.md, and the launch checklist agree on the evidence
   hierarchy? Are there contradictions with any local CLAUDE.md?

5. API coverage accuracy: does docs/api-coverage.md correctly classify
   every MCP tool by read-only/mutating/destructive? Are the endpoint
   mappings accurate against internal/clockify/ and internal/paths/?

6. Security: do any of these changes weaken security defaults, relax
   policy enforcement, or introduce new auth bypass paths?
```

## Codex/OpenAI review prompt

```
Review fwbranch (17 commits ahead of origin/main) for the go-clockify
MCP server. Focus on:

1. The skip sentinel test (tests/e2e_live_skip_sentinel_test.go):
   is the all-skipped detection logic correct? Does it handle the
   case where build tags exclude the file?

2. The evidence gate script (scripts/check-launch-evidence-gate.sh):
   is the bash safe? Any quoting issues, command injection vectors,
   or undefined variable risks?

3. The regression test for the evidence gate: does test-check-launch-
   evidence-gate.sh exercise the gate script's error paths?

4. The API coverage matrix (docs/api-coverage.md): are all 124 tools
   accounted for? Are the endpoint mappings verifiable against the
   source? Are dry-run/policy gaps honestly documented?

5. Makefile changes: are the new targets idempotent? Do they depend
   on tools that might not be installed?

Files changed: 14 files, +1054/-8 lines.
Diff stat: git diff --stat origin/main..fwbranch
```

## Human merge checklist

- [ ] `make release-check` green locally
- [ ] Reviewed `tests/e2e_live_skip_sentinel_test.go` for correctness
- [ ] Reviewed `scripts/check-launch-evidence-gate.sh` for bash safety
- [ ] Reviewed `docs/api-coverage.md` for tool classification accuracy
- [ ] Verified `TestLiveContractSkipSentinel` fails when all live tests skip
      and passes when at least one runs (drift-check)
- [ ] Confirmed `make launch-checklist-parity` passes (gate wired)
- [ ] Confirmed `make doc-parity` passes
- [ ] Confirmed `make config-doc-parity` passes
- [ ] Confirmed `make catalog-drift` passes
- [ ] Confirmed `make check` passes on fwbranch HEAD
- [ ] No secrets, .env files, .claude/, or machine-specific paths in diff
- [ ] Push to origin/fwbranch
- [ ] CI green on fwbranch PR
- [ ] Squash-merge or rebase-merge to main (prefer rebase for atomic commits)
