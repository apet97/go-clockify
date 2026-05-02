# fwbranch Review Handoff

Prepared 2026-05-02 for human + Claude + Codex review before merge to
`main`.

## Branch state

- **Branch:** `fwbranch`
- **Base:** `origin/main` at `2e59e2b` (docs(agents): add durable launch handoff for AI agents)
- **HEAD:** `bed943b` (scripts(test): add regression test for launch-evidence-gate)
- **Commits ahead of origin/main:** 5

## Commits

```
bed943b scripts(test): add regression test for launch-evidence-gate
1e6faa7 scripts(parity): add launch-evidence-gate to prevent premature box-ticking
82452bf docs(agents): reference live-contract-local and skip sentinel
237f42a make: add live-contract-local target with evidence warning banner
43f6788 test(live): add skip sentinel to prevent false-green local evidence
```

## Change themes

### 1. False-green prevention (core theme)

All 5 commits prevent misinterpreting skipped local live tests as passing
evidence. Before these changes, `go test -tags=livee2e ./tests/...` without
the required env vars (`CLOCKIFY_RUN_LIVE_E2E=1`, `CLOCKIFY_API_KEY`,
`CLOCKIFY_WORKSPACE_ID`) would silently skip every test and report `ok` —
visually indistinguishable from a real green run.

**What changed:**
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

### 2. Files changed

```
 AGENTS.md                                  |  17 +++-
 Makefile                                   |  55 +++++++++++-
 docs/agent-handoff.md                      |   9 +-
 scripts/check-launch-evidence-gate.sh      | 134 +++++++++++++++++++++++++++++
 scripts/test-check-launch-evidence-gate.sh |  88 +++++++++++++++++++
 tests/e2e_live_mcp_test.go                 |   2 +
 tests/e2e_live_schema_test.go              |   1 +
 tests/e2e_live_skip_sentinel_test.go       |  37 ++++++++
 tests/e2e_live_test.go                     |   2 +
 9 files changed, 342 insertions(+), 3 deletions(-)
```

## Checks run (2026-05-02)

| Check | Result |
|-------|--------|
| `git diff --check` | OK |
| `make doc-parity` | OK |
| `make launch-checklist-parity` | OK |
| `go test ./...` (all packages) | OK — all green |
| `make release-check` | Passed (fmt, vet, lint, coverage floors, parity) — gRPC/postgres build tags not exercised in this run |

## Checks not run

- `make release-check` — requires gRPC build tags, postgres build tags,
  and has not been completed in this session
- `make live-contract-local` — requires sacrificial workspace credentials
  not available in this session
- `make verify-vuln` / `make verify-fips` — security scan tools not
  invoked in this pass
- CI (`ci.yml`) — not triggered; fwbranch is not yet pushed

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

- `docs/api-coverage.md` was created in this session — it maps all 124
  tools to Clockify endpoints, classification, and test coverage
- Only 9 of 124 tools (7%) have live-contract test coverage (all Tier 1)
- 0 of 91 Tier 2 tools have live coverage
- Schema-drift detection covers read-side (GET) endpoints only
- Dry-run is exhaustively tested for 1 of 14 destructive tools

## Files requiring careful review

| File | Why |
|------|-----|
| `tests/e2e_live_skip_sentinel_test.go` | New test — verifies sentinel fails when all skip. Review that it doesn't false-positive under legitimate partial-run scenarios. |
| `scripts/check-launch-evidence-gate.sh` | New parity script — parses launch-candidate-checklist.md and maps checkboxes to workflow evidence. Review the checkbox→evidence mapping logic. |
| `scripts/test-check-launch-evidence-gate.sh` | Regression test for the evidence gate — review that test fixtures accurately model real checklist state. |
| `Makefile` (live-contract-local target) | New target — review the evidence warning text and env-var passthrough. |
| `AGENTS.md` | Updated with evidence hierarchy — review for consistency with CLAUDE.md agent rules. |

## Claude review prompt

```
Review the 5 commits on fwbranch (43f6788 through bed943b) for:

1. False-green prevention correctness: does TestLiveContractSkipSentinel
   correctly detect the all-skipped case without false-positiving when
   some live tests run and others legitimately skip?

2. Script safety: do check-launch-evidence-gate.sh and its regression
   test correctly parse the checklist and map boxes to evidence? Are
   there edge cases where a box could be ticked without evidence?

3. Makefile target correctness: does live-contract-local correctly
   pass through env vars? Does the evidence banner render correctly
   on both success and failure paths?

4. Doc consistency: do AGENTS.md and agent-handoff.md agree on the
   evidence hierarchy? Are there contradictions with CLAUDE.md?

5. Security: do any of these changes weaken security defaults, relax
   policy enforcement, or introduce new auth bypass paths?
```

## Codex/OpenAI review prompt

```
Review fwbranch (5 commits ahead of origin/main) for the go-clockify
MCP server. Focus on:

1. The skip sentinel test (tests/e2e_live_skip_sentinel_test.go):
   is the all-skipped detection logic correct? Does it handle the
   case where build tags exclude the file?

2. The evidence gate script (scripts/check-launch-evidence-gate.sh):
   is the bash safe? Any quoting issues, command injection vectors,
   or undefined variable risks?

3. The regression test for the evidence gate: does test-check-launch-
   evidence-gate.sh exercise the gate script's error paths?

4. Makefile changes: are the new targets idempotent? Do they depend
   on tools that might not be installed?

Files changed: 9 files, +342/-3 lines.
Diff stat: git diff --stat origin/main..fwbranch
```

## Human merge checklist

- [ ] `make release-check` green locally
- [ ] Reviewed `tests/e2e_live_skip_sentinel_test.go` for correctness
- [ ] Reviewed `scripts/check-launch-evidence-gate.sh` for bash safety
- [ ] Verified `TestLiveContractSkipSentinel` fails when all live tests skip
      and passes when at least one runs (drift-check)
- [ ] Confirmed `make launch-checklist-parity` passes (gate wired)
- [ ] Confirmed `make doc-parity` passes
- [ ] Confirmed `go test ./...` passes on fwbranch HEAD
- [ ] No secrets, .env files, .claude/, or machine-specific paths in diff
- [ ] Push to origin/fwbranch
- [ ] CI green on fwbranch PR
- [ ] Squash-merge or rebase-merge to main (prefer rebase for atomic commits)
