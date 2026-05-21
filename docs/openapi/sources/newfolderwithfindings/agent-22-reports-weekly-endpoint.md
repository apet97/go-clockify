# QA Agent 22 - reports-weekly-endpoint

Status: COMPLETE
Started UTC: 2026-05-10T20:29:37Z
Completed UTC: 2026-05-11T00:00:00Z

Worktree: /Users/15x/Downloads/go-clockify-qa-swarm/worktrees/agent-22
Live API probe lab: /Users/15x/Downloads/WORKING/clockify-api-probe-lab

## Verdict
PASS

## What I checked

1. **Code correctness**: Reviewed `internal/tools/reports.go` (WeeklySummary, weekBounds, aggregateEntriesRangeForWorkspace, reportLimitsForArgs, totalsFromAgg, daySummariesFromAgg, projectSummariesFromAgg, reportSuggestedActions), `internal/tools/resources.go` (isoWeekStart, weeklyReportURIsForEntry, weeklyReportResourceURI), `internal/tools/common.go` (WeeklySummaryData, DaySummary, SummaryTotals, loadLocation, ResultEnvelope), and `internal/clockify/models.go` (DurationSeconds, IsRunning).

2. **MCP tool schema**: Verified the `clockify_weekly_summary` tool descriptor in `registry.go:77-85` — parameters `week_start` (optional RFC3339 or YYYY-MM-DD), `timezone` (optional IANA), `project` (optional name/id filter), `include_entries` (bool), `max_entries` (int, bounded by server cap). Output schema uses typed `WeeklySummaryData` struct via `envelopeSchemaFor`.

3. **Week boundary logic**: Inspected `weekBounds()` at `reports.go:633-651` and `isoWeekStart()` at `resources.go:81-95`. Both correctly implement ISO week Monday-anchored computation with Sunday-to-7 mapping. Timezone-aware date truncation via `time.Date()` + `AddDate()` handles DST transitions correctly.

4. **Unit tests**: Ran `go test ./internal/tools/` — all pass (7 weekly-report-related test suites: TestWeeklySummary, TestWeeklySummary_MultiPage, TestIsoWeekStart, TestWeeklyReportURIsForEntry_SingleWeek, _CrossWeek, _RunningTimer, _BadInputsDegrade, TestAddEntry_CrossWeekSpanEmitsBothWeeklyReports, TestWeeklyReportDeltaFromCachedState).

5. **Live e2e tests**: Ran `go test ./tests/ -tags=livee2e` with `CLOCKIFY_LIVE_FULL_SURFACE_ENABLED=true` against the probe workspace. All pass: weekly_summary (0.36s), summary_report (0.25s), quick_report (0.28s), detailed_report (0.15s).

6. **MCP doctor**: Ran `clockify-mcp doctor` with live credentials — configuration audit passes (Load OK, transport=stdio, workspace resolved, API key present).

7. **Docker build**: Built Docker image successfully from `deploy/Dockerfile` using pinned digests for golang:1.25-bookworm and distroless/static-debian12. Docker doctor command runs in container.

8. **Direct API probes**: Verified the underlying `/api/v1/workspaces/{ws}/user/{userId}/time-entries` endpoint returns correct entries with hydration (`?hydrated=true&page-size=200`), confirming the data pipeline the weekly report depends on is intact.

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — credentials (API key redacted, workspace ID: <REDACTED_ID>)
- `clockify-api-probe-lab/CLAUDE.md` — safety rules
- `clockify-api-probe-lab/probes/lib/common.sh` — shared probe library (probe_curl, probe_redact, etc.)
- `clockify-api-probe-lab/docs/official-api-notes.md` — per-domain API notes (shared-reports section documents host/path/pagination)

## Commands run

```bash
# Build
go build ./cmd/clockify-mcp

# Unit tests
go test ./internal/tools/ -count=1 -timeout 120s

# Live e2e tests
CLOCKIFY_LIVE_FULL_SURFACE_ENABLED=true go test ./tests/ \
  -run "TestLiveTier1ReadOnly/(summary_report|weekly_summary|quick_report|detailed_report)" \
  -v -count=1 -timeout 120s -tags=livee2e

# MCP doctor
CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED_ID> \
  go run ./cmd/clockify-mcp doctor

# Docker build
docker build -f deploy/Dockerfile -t clockify-mcp-test:qa .

# Docker doctor
docker run --rm \
  -e CLOCKIFY_API_KEY=<REDACTED> \
  -e CLOCKIFY_WORKSPACE_ID=<REDACTED_ID> \
  clockify-mcp-test:qa doctor

# Direct API probe (time entries endpoint)
curl -s --request GET \
  "https://api.clockify.me/api/v1/workspaces/${WS}/user/${UID}/time-entries?page-size=200&hydrated=true" \
  --header "X-Api-Key: <REDACTED>" | jq 'length'
```

## Live API probes run

| # | Method | Endpoint | Status | Notes |
|---|--------|----------|--------|-------|
| 1 | GET | `/api/v1/user` | 200 | Current user resolved: id=64621fae..., name=Firstname Lastname |
| 2 | GET | `/api/v1/workspaces/{ws}/user/{uid}/time-entries?page-size=3&hydrated=true` | 200 | Returns correct entry shape with timeInterval, projectId, projectName |
| 3 | GET | `/api/v1/workspaces/{ws}/user/{uid}/time-entries?page-size=200&hydrated=true` | 200 | First page returns 200 entries (workspace has existing data) |
| 4 | MCP tool | `clockify_weekly_summary` (default) | PASS | Returns range, totals, byDay, byProject, suggestedActions |
| 5 | MCP tool | `clockify_summary_report` (start/end) | PASS | Returns range, totals, byProject |
| 6 | MCP tool | `clockify_quick_report` (days=7) | PASS | Returns totals, runningEntries, topProject, entriesSample |
| 7 | MCP tool | `clockify_detailed_report` (start/end) | PASS | Returns range, totals, byProject, entries |

## Findings

### Finding 1: Docker default transport requires OIDC configuration (P3)

**Description**: The Docker image (`deploy/Dockerfile:97`) sets `ENV MCP_TRANSPORT=streamable_http` as default. This transport requires OIDC authentication configuration (`MCP_OIDC_ISSUER`, etc.). Running the container with only `CLOCKIFY_API_KEY` and `CLOCKIFY_WORKSPACE_ID` fails with:

```
Load() result: ERROR MCP_OIDC_ISSUER is required when MCP_TRANSPORT=streamable_http and MCP_AUTH_MODE=oidc
```

**Impact**: Users following a "docker run" quickstart without reading the full transport/auth docs will get a startup error. Not specific to weekly reports — affects all tools.

**Mitigation**: Set `MCP_TRANSPORT=stdio` for local/self-hosted use, or `MCP_AUTH_MODE=bearer` with a `MCP_BEARER_TOKEN` for simple HTTP deployments. The `doctor` command correctly diagnoses this.

**Recommendation**: Document the minimum Docker env vars for local use in the README or add a Quick Start section showing `MCP_TRANSPORT=stdio`.

### Finding 2: No issues found in weekly report logic (PASS)

**Description**: The `clockify_weekly_summary` MCP tool is a safe wrapper built over the current user's time-entry data (`GET /api/v1/workspaces/{ws}/user/{userId}/time-entries`). It does NOT call the Clockify shared-reports API (`reports.api.clockify.me`). This is an architectural choice documented in the tool description: "This is a safe wrapper built over time-entry data rather than a separate reports API."

**Verified correct**:
- ISO week boundary calculation (Monday 00:00 to next Monday 00:00 in specified timezone)
- Sunday-to-Monday rollover mapping (weekday 0 to 7)
- DST-safe date arithmetic via `time.Date` + `AddDate` calendar days
- Streaming pagination (page-size=200, safety stop at 1000 pages)
- Memory bounds (IncludeEntries=false keeps memory bounded; EntriesCount tracks totals without retaining raw entries)
- Running entry detection (End="" -> IsRunning=true, DurationSeconds uses time.Now() for live elapsed time)
- Multi-page aggregation (TestWeeklySummary_MultiPage confirms correct cross-page rollups)
- Cross-week entry invalidation (resources emit both week URIs when entry spans two ISO weeks)
- Bad input degradation (weeklyReportURIsForEntry returns nil without panicking on malformed timestamps)
- Project filter resolution (name or ID passed through reportProjectFilterID to resolveProjectID)
- Suggested actions generation (drill-into-list-entries + log-time suggestions, zero-entry variant)
- Output schema typing (WeeklySummaryData struct with range, totals, byDay, byProject, suggestedActions, entries, unassignedKey)

### Finding 3: Weekly report resource URI invalidation is correct (PASS)

**Description**: When a time entry is created/updated/deleted, `weeklyReportURIsForEntry()` at `resources.go:106` emits resource update notifications for the affected weekly report URIs. Verified:
- Single-week entries emit exactly one URI: `clockify://workspace/{ws}/report/weekly/{YYYY-MM-DD}`
- Cross-week entries (e.g., Sunday 23:00 to Monday 01:00) emit two URIs
- Running timers (empty end) emit only the start week URI
- Bad inputs (empty workspace, empty start, malformed timestamp) return nil (safety net — primary entry URI path is not blocked)

### Finding 4: All schema coverage is complete (PASS)

**Description**: The `clockify_weekly_summary` tool has:
- Input schema defined in registry.go with all parameters
- Output schema via `envelopeSchemaFor[WeeklySummaryData]` in output_schemas.go
- Schema property validation test in schema_validator_property_test.go
- Schema tightening test in schema_tighten_test.go (week_start recognized as flexible date type)
- Golden test inclusion in golden_test.go
- Tool catalog entry in docs/tool-catalog.md

## Fixes made

No fixes required. All tests pass, no bugs found in the weekly summary endpoint.

## Reproduction steps for each issue

### Finding 1 (Docker transport)
```bash
docker build -f deploy/Dockerfile -t clockify-mcp:local .
docker run --rm \
  -e CLOCKIFY_API_KEY=<REDACTED> \
  -e CLOCKIFY_WORKSPACE_ID=<REDACTED_ID> \
  clockify-mcp:local doctor
# Expected: ERROR about OIDC issuer requirement
# Fix: add -e MCP_TRANSPORT=stdio
```

## Cleanup performed

No test resources created. All probes were read-only. No prefixed entities (`qa-agent-22-*` or `mcp-probe-*`) were created during this run.

## Leftover test resources

None.

## Severity

| Finding | Severity | Rationale |
|---------|----------|-----------|
| Docker default transport requires OIDC | P3 | Documentation/UX issue. Functional with correct env vars. Not specific to reports-weekly. |
| Weekly report logic | Pass | No defects found. All unit and live e2e tests pass. |

## Files changed

No files modified. All validation was read-only.

## Suggested next action

1. **P3**: Add a Docker quickstart section to README.md showing the minimum env vars for local use: `docker run -e CLOCKIFY_API_KEY=... -e CLOCKIFY_WORKSPACE_ID=... -e MCP_TRANSPORT=stdio ...`. Alternatively, consider changing the Dockerfile default transport to `stdio` for the default image and documenting `streamable_http` as the production override.

2. **Optional**: Consider adding a live e2e subtest for `clockify_weekly_summary` with explicit `week_start` and `timezone` parameters (currently the test only calls with no args). The existing test validates the default case; parameterized coverage would increase confidence.

3. **Optional**: Verify `weekly_summary` with a `project` filter parameter in the live e2e suite. The filter path is exercised in unit tests but not in the live suite.

## False positives / uncertainty

- **Docker transport default**: This is arguably correct behavior for a production image. The Dockerfile explicitly documents the rationale in comments at line 93-96: "Default to the spec-strict streamable HTTP transport." Whether to treat this as a P3 issue depends on the expected quickstart experience. If the Docker image is targeted at production operators who will read the full docs, this is not a concern.
- **No parameterized live e2e test**: The existing live test for weekly_summary passes with zero arguments, which validates the default path. Explicit parameter tests (week_start, timezone, project) exist in unit tests but not in the live e2e suite. This is a low-risk gap.

## Final recommendation

**PASS** — The `clockify_weekly_summary` MCP tool is ready for local/internal/community/self-hosted use.

The tool correctly wraps time-entry data to produce ISO-week-aligned summaries grouped by day and project. All edge cases (cross-week entries, running timers, pagination safety stops, DST transitions, empty data, bad inputs) are handled correctly. Unit tests and live e2e tests confirm end-to-end correctness against the Clockify API.

The only production-readiness gap is the Docker image defaulting to `streamable_http` transport, which requires OIDC configuration. This is a P3 documentation issue, not a code defect, and is resolved by setting `MCP_TRANSPORT=stdio` for local use.
