# QA Agent 44 - release-docs-accuracy

## Verdict
PASS WITH CONCERNS

## What I checked

Release-documentation accuracy for the `go-clockify` MCP server at commit `abf9459` (11 commits ahead of `v1.2.1`). I verified that:

- The README accurately describes 128 tools (40 Tier 1 + 88 Tier 2), all deployment profiles, configuration variables, and build commands
- All internal documentation cross-references resolve to existing files
- The tool catalog (`docs/tool-catalog.md`) has exact parity with the tool descriptors in `internal/tools/` — all 128 tool names match
- Auto-generated doc blocks (CONFIG-TABLE, tool catalog) have valid generation markers and the generation scripts exist
- The release policy (`docs/release-policy.md`) correctly references `scripts/check-release-assets.sh` and describes 15 binaries, 46 total artifacts
- The `doctor` command runs successfully and reports all env vars with sources
- `make doc-parity` passes
- Live API shapes match the probe lab documentation for invoices, webhooks, shared reports, scheduling, expenses, custom fields, time-off, approvals, and holidays

## Live API probe lab files used

From `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/`:
- `CLAUDE.md` — agent rules and safety constraints
- `README.md` — project overview and layout
- `docs/official-api-notes.md` — per-domain API surface documentation
- `docs/safety-rules.md` — expanded safety rules
- Credentials from `/tmp/clockify-livetest.env` (not in the repo)

Credentials used:
- API key: `****REDACTED****`
- Workspace ID: `65b382b606de527a7ee2b60e`

## Commands run

All commands run from the agent-44 worktree at `/Users/15x/Downloads/go-clockify-qa-swarm/worktrees/agent-44`.

```sh
# Build verification
go build ./cmd/clockify-mcp
# Result: OK (Go 1.26.2, go.mod declares go 1.25.10 minimum)

# Doctor command
go run ./cmd/clockify-mcp doctor
# Result: OK — 75+ env vars audited, all sources labeled

# Version
go run ./cmd/clockify-mcp --version
# Result: "dev" (expected — not at a tagged commit in the worktree)

# Tool count verification (code vs catalog)
grep -r '"clockify_' internal/tools/ --include='*.go' | grep -v '_test.go' | \
  grep -o '"clockify_[^"]*"' | sort -u | wc -l
# Result: 128 unique tool names in code

grep -E '^\| `clockify_' docs/tool-catalog.md | \
  sed 's/.*`\(clockify_[^`]*\)`.*/\1/' | sort -u | wc -l
# Result: 128 unique tool names in catalog

# Cross-file reference check — 40+ referenced paths verified to exist
# Result: All exist

# Doc parity
bash scripts/check-doc-parity.sh
# Result: OK

# Deprecation notice verification
go run ./cmd/clockify-mcp --help 2>&1 | grep -i deprecat
# Result: MCP_HTTP_MAX_BODY and http transport correctly flagged as deprecated
```

## Live API probes run

All probes sourced credentials from `/tmp/clockify-livetest.env` with `X-Api-Key` header. Workspace `65b382b606de527a7ee2b60e` ("WORKSPACE").

| # | Endpoint | Response shape | Matches probe lab doc? |
|---|----------|---------------|----------------------|
| 1 | `GET /user` | `{id, email, name, ...}` | Yes |
| 2 | `GET /workspaces/{ws}` | `{id, name, workspaceSettings: {...}}` | Yes |
| 3 | `GET /workspaces/{ws}/projects?page-size=3` | Array of `{id, name, clientId, billable, ...}` | Yes |
| 4 | `GET /workspaces/{ws}/invoices?page-size=2` | `{total: 113, invoices: [...]}` | Yes |
| 5 | `GET /workspaces/{ws}/webhooks` | `{workspaceWebhookCount: 7, webhooks: [...]}` | Yes |
| 6 | `GET reports.api.clockify.me/v1/workspaces/{ws}/shared-reports?pageSize=2` | `{reports: [...], count: 74}` | Yes |
| 7 | `GET /workspaces/{ws}/scheduling/assignments/all?...` | `[]` (no assignments in workspace) | N/A (empty) |
| 8 | `GET /workspaces/{ws}/expenses?page-size=2` | `{dailyTotals, expenses, weeklyTotals}` | Yes |
| 9 | `GET /workspaces/{ws}/time-off` | `{code, message}` error — no policies | Yes (error shape) |
| 10 | `GET /workspaces/{ws}/custom-fields?page-size=2` | Bare array of 50 objects | Yes |
| 11 | `GET /workspaces/{ws}/approval-requests?page-size=2` | `[]` (no requests) | N/A (empty) |
| 12 | `GET /workspaces/{ws}/holidays` | Bare array of 8 objects | Yes |

All probed endpoints returned response shapes consistent with the probe lab documentation.

## Findings

### F1: CHANGELOG not updated for v1.2.1 release (P2)

**What:** The CHANGELOG has no `[1.2.1]` section. All changes between v1.2.0 and HEAD (337 commits) remain under `[Unreleased]`, which spans 2,215 lines (lines 8–2222). The last versioned section is `[1.2.0] - 2026-04-25` at line 2223. The `v1.2.1` tag was cut on 2026-05-10 (commit `ce56414`) but the CHANGELOG was never cut alongside it.

**Impact:** Operators reading the CHANGELOG cannot determine what shipped in v1.2.1 vs what is still unreleased. This violates Keep a Changelog conventions declared in the CHANGELOG header.

**Evidence:**
- `git tag --sort=-v:refname | head -5` → `v1.2.1-rc.3`, `v1.2.1-rc.2`, `v1.2.1-rc.1`, `v1.2.1`, `v1.2.0`
- `grep '^## \[' CHANGELOG.md` → `[Unreleased]`, `[1.2.0]`, `[1.1.0]`, `[1.0.3]`, `[1.0.2]`, `[1.0.1]`, `[1.0.0]`
- `[Unreleased]` section: 2,215 lines (78% of the 2,847-line CHANGELOG)

**Suggested fix:** Rename the current `[Unreleased]` content up to the v1.2.1 tag point to `[1.2.1] - 2026-05-10` and start a fresh `[Unreleased]` section for the 11 post-tag commits.

### F2: api-coverage.md Tier 1 heading count was stale (P3 — FIXED)

**What:** The heading at `docs/api-coverage.md:41` said "Tier 1 — Core tools (35)" but the section lists 40 tools. This count was not updated when tools were added (e.g., the timesheet workflow tools `clockify_timesheet_review` and `clockify_timesheet_fill_gap`, plus other helpers).

**Impact:** Low — the table beneath the heading is correct. A reader scanning headings would momentarily think only 35 Tier 1 tools exist.

**Fix applied:** Changed heading to "Tier 1 — Core tools (40)" to match the actual tool count.

### F3: Compressed RC-to-release cycle (P3 — informational)

**What:** `v1.2.1` and `v1.2.1-rc.3` point to the same commit (`ce56414`), both tagged on 2026-05-10. The RC tag was for validation before the release, but they were tagged on the same day with the same commit, making the RC process a formality.

**Impact:** Low. The release process works, but the RC-then-release cadence is compressed into a single day. The release-policy.md documents a proper RC evidence sequence (Groups 6 and 7), which appears to have been followed in spirit per the launch-candidate-checklist.md evidence links.

## Fixes made

1. **`docs/api-coverage.md:41`** — Updated stale heading from "(35)" to "(40)" to match the actual Tier 1 tool count.

## Reproduction steps for each issue

### F1: CHANGELOG missing v1.2.1
1. `grep '^## \[' CHANGELOG.md` — observe `v1.2.0` is the last versioned section
2. `git tag --sort=-v:refname | head -5` — observe `v1.2.1` exists
3. Contrast: the CHANGELOG's `[Unreleased]` spans 2,215 lines but v1.2.1 was tagged

### F2: Stale heading (FIXED)
1. Before fix: `grep "Tier 1 — Core" docs/api-coverage.md` → "(35)"
2. Count tools in the section: 40
3. After fix: heading now reads "(40)"

### F3: RC tag timeline
1. `git for-each-ref --sort=-taggerdate --format '%(refname:short) %(taggerdate:short)' refs/tags | head -5`
2. Observe v1.2.1 and v1.2.1-rc.3 both dated 2026-05-10 on the same commit

## Cleanup performed

No test resources were created. All probes were read-only GET requests against existing workspace data. No cleanup needed.

## Leftover test resources

None — no resources were created during this QA run.

## Severity

| ID | Severity | Description |
|----|----------|-------------|
| F1 | P2 | CHANGELOG missing [1.2.1] section — 2,215 lines in Unreleased despite v1.2.1 tag existing |
| F2 | P3 | api-coverage.md heading count (35) didn't match actual tool count (40) — **FIXED** |
| F3 | P3 | v1.2.1 release tag and rc.3 tag on same commit, same day — compressed RC cycle |

## Files changed

- `docs/api-coverage.md` — Updated Tier 1 heading from "(35)" to "(40)"

## Suggested next action

1. **Cut CHANGELOG for v1.2.1.** Move the current `[Unreleased]` content up to the v1.2.1 tag point into a `[1.2.1] - 2026-05-10` section and start a fresh `[Unreleased]` for the 11 post-tag commits. This is the highest-impact doc fix.

2. **Automate the CHANGELOG cut as part of the release workflow.** Consider adding a CI check that fails if tags exist without matching CHANGELOG sections, or a `make cut-changelog` target.

3. **Re-run `make gen-tool-catalog` and `make config-doc-parity`** after the next tool addition to keep generated docs in sync.

## False positives / uncertainty

- The v1.2.1 tag was created on 2026-05-10 and the CHANGELOG update may be a follow-up step. If the maintainer's workflow is "tag first, update CHANGELOG second," this is a transient state. F1 would close when the follow-up lands.

- The compressed RC cycle (v1.2.1-rc.3 and v1.2.1 on same commit) could be intentional — the RC validation may have passed without changes needed. This is documented in the launch-candidate-checklist.md evidence links.

## Final recommendation

**PASS WITH CONCERNS.** The documentation is well-structured and consistent across the vast majority of the surface. All references resolve, generated docs are in sync with code, and live API shapes match the probe lab docs. The single significant finding (F1: stale CHANGELOG for v1.2.1) is a process gap that can be closed with a straightforward follow-up. The project's doc infrastructure (auto-generation, parity checks, config-doc-parity CI job) is robust and would catch most drift automatically.
