# QA Agent 21 - reports-summary-endpoint

## Verdict
PASS WITH CONCERNS

## What I checked

1. **`clockify_summary_report` tool implementation** (`internal/tools/reports.go:288-331`) — verified the tool aggregates time entries by walking `GET /workspaces/{ws}/user/{uid}/time-entries` pages and builds project rollups locally
2. **`clockify_weekly_summary` tool** (`internal/tools/reports.go:333-382`) — verified the weekly variant adds day bucketing on top of the same aggregation pipeline
3. **`clockify_quick_report` tool** (`internal/tools/reports.go:384-439`) — verified it also uses the same time-entries aggregation with sample-entry retention
4. **Tool registry** (`internal/tools/registry.go:67-94`) — verified tool schemas, parameter definitions, read-only hints, idempotent hints
5. **Output schemas** (`internal/tools/output_schemas.go:34-35`, `docs/tool-catalog.json`) — verified structured output shape
6. **Clockify Reports Summary API** — probed live against both `api.clockify.me` and `reports.api.clockify.me` hosts
7. **`api-coverage.md`** — checked endpoint mappings and corrected inaccuracies
8. **`clockify-mcp doctor`** — verified the diagnostic command works with and without credentials
9. **Build, vet, and unit tests** — all pass clean
10. **E2E test** (`tests/e2e_live_tier1_readonly_test.go:110-136`) — verified live read-only coverage exists for summary_report and weekly_summary

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — API key and workspace credentials (redacted)
- `docs/official-api-notes.md` — domain documentation for shared-reports and scheduling
- `findings/shared-reports.md` — prior findings confirming reports host is `reports.api.clockify.me/v1`
- `probes/lib/common.sh` — shared probe library

## Commands run

```bash
# Build
go build ./...

# go vet
go vet ./internal/tools/

# Unit tests
go test ./internal/tools/ -run "TestSummaryReport|TestWeeklySummary|TestQuickReport|TestDetailedReport" -v -count=1

# Doctor command (no creds)
go run ./cmd/clockify-mcp doctor

# Doctor command (with creds)
CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED> go run ./cmd/clockify-mcp doctor

# Live API probes
curl -H "X-Api-Key: <REDACTED>" "https://api.clockify.me/api/v1/workspaces/<REDACTED>/reports/summary?start=...&end=..."
curl -H "X-Api-Key: <REDACTED>" "https://reports.api.clockify.me/v1/workspaces/<REDACTED>/reports/summary?start=...&end=..."
curl -X POST -H "X-Api-Key: <REDACTED>" -H "Content-Type: application/json" \
  -d '{"dateRangeStart":"...","dateRangeEnd":"...","summaryFilter":{"groups":["PROJECT"]}}' \
  "https://reports.api.clockify.me/v1/workspaces/<REDACTED>/reports/summary"

# Time-entries aggregation path (MCP server's actual implementation)
curl -H "X-Api-Key: <REDACTED>" \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/user/<REDACTED>/time-entries?start=...&end=...&page-size=200&hydrated=true"
```

## Live API probes run

### Probe 1: `GET /workspaces/{ws}/reports/summary` on api.clockify.me
- **Result: 404** — "No static resource" (reports endpoints don't live on the primary API host)

### Probe 2: `GET /workspaces/{ws}/reports/summary` on reports.api.clockify.me
- **Result: 405** — Method Not Allowed (the endpoint requires POST)

### Probe 3: `POST /workspaces/{ws}/reports/summary` on reports.api.clockify.me (no body)
- **Result: 400** — "Please provide summary filter." (requires `summaryFilter` in body)

### Probe 4: `POST .../reports/summary` with `summaryFilter: {groups: ["PROJECT"]}`
- **Result: 200** — Returns structured report data with totals, groupOne, donutChart, groupTotals
- Date format must be full ISO 8601 (`2026-05-03T21:10:45.000Z`); date-only format (`2026-05-03`) returns 400

### Probe 5: Time-entries aggregation (MCP server's actual implementation)
- **Result: 200** — Returns 7 entries across 2 projects, total 18.73 hours
- One entry has `projectId` set but `projectName` empty in the hydrated response; the real reports API correctly resolves this name

### Probe 6: Doctor command
- **Without credentials:** Load() ERROR with `CLOCKIFY_API_KEY is required`, exit code 2 — clean error handling
- **With credentials:** Load() OK, transport=stdio, reports all effective config values correctly

## Findings

### Finding 1 (P1): `api-coverage.md` had incorrect endpoint mappings (FIXED)
**Location:** `docs/api-coverage.md:71,74`

**Issue:** The coverage matrix mapped both `clockify_summary_report` and `clockify_quick_report` to `GET /workspaces/{ws}/reports/summary`. In reality:
- The MCP server does NOT call the Clockify Reports Summary endpoint
- Both tools aggregate time entries via `GET /workspaces/{ws}/user/{uid}/time-entries` and roll up locally
- The actual Clockify Reports Summary API endpoint is `POST https://reports.api.clockify.me/v1/workspaces/{ws}/reports/summary`, not GET on the primary API host

**Fix applied:** Updated both entries to read "wrapper (aggregates `GET /workspaces/{ws}/user/{uid}/time-entries`)", matching the `clockify_weekly_summary` entry's accurate self-description.

### Finding 2 (P2): Time-entries wrapper vs real Reports Summary API — shape mismatch
**Location:** `internal/tools/reports.go:288-331` vs live API response

The MCP server's time-entries wrapper returns:
```json
{
  "range": {"start": "...", "end": "..."},
  "totals": {"entries": N, "runningEntries": N, "totalSeconds": N, "totalHours": N},
  "byProject": [{"projectId": "...", "projectName": "...", "entries": N, "totalSeconds": N, "totalHours": N}],
  "suggestedActions": [...]
}
```

The actual Clockify Reports Summary API returns:
```json
{
  "totals": [{"totalTime": N, "totalBillableTime": N, "entriesCount": N, "amounts": [...], "totalAmount": N}],
  "donutChart": [],
  "groupTotals": {"groupOneTotalCount": N},
  "groupOne": [{"_id": "...", "name": "...", "duration": N, "amount": N, "amounts": [...]}]
}
```

These are fundamentally different shapes. The MCP wrapper focuses on time duration per project; the real API includes billing amounts, currency data, and chart data.

**Assessment:** This is a design choice, not a bug. The MCP server describes itself as a "safe wrapper built over time-entry data" and the meta field includes `source: "time-entries-wrapper"`. The wrapper approach avoids the additional reports host complexity and billing/plan-gated features. However, users familiar with the Clockify Reports API may expect the real summary shape. API parity would require implementing the POST endpoint on the reports host.

### Finding 3 (P3): Hydrated time-entries may lack projectName
**Location:** `internal/tools/reports.go:153-163`

In the live probe, one time entry had `projectId=69e96841a50edf6295fdf46a` but `projectName` was empty in the hydrated response, causing the MCP server to label it "(no project)". The actual Reports Summary API correctly resolved this to "Marketing Campaign Q3".

The MCP server's fallback logic at `reports.go:161-163` handles this partially:
```go
} else if bucket.Name == "(no project)" && strings.TrimSpace(entry.ProjectName) != "" {
    bucket.Name = strings.TrimSpace(entry.ProjectName)
}
```
This only updates the name if a later entry in the same bucket has a non-empty `projectName`. If all entries in a bucket lack `projectName`, the bucket stays as "(no project)".

**Assessment:** Low impact for normal use — most hydrated entries do include projectName. The real reports API correctly resolves project names via server-side joins where the time-entries endpoint may omit them.

### Finding 4 (P3): `clockify_quick_report` enum validation
**Location:** `internal/tools/reports.go:386`

`days` is validated at runtime as `1 <= days <= 365`, and the input schema at `registry.go:87` correctly specifies both `minimum: 1` and `maximum: quickReportMaxDays`. No issue found — the schema and runtime validation are consistent.

### Finding 5 (P3): No `report_type` or `groups` parameter in summary tool
The MCP tools don't expose the `summaryFilter.groups` parameter that the real Reports API supports. The MCP always groups by project (for summary_report) and by day+project (for weekly_summary). The real API supports groups like `PROJECT`, `TIMEENTRY`, `USER`, `CLIENT`, `TAG`, `TASK`. Users wanting different grouping dimensions can't get them through the MCP tools.

**Assessment:** Feature gap, not a bug. The current project-only grouping covers the most common use case. Adding group-by support would require implementing the actual Reports API endpoint.

## Fixes made

1. **`docs/api-coverage.md`** — Corrected endpoint mapping for `clockify_summary_report` and `clockify_quick_report` from `GET /workspaces/{ws}/reports/summary` to `wrapper (aggregates GET /workspaces/{ws}/user/{uid}/time-entries)`, matching the accurate descriptions already present for `clockify_weekly_summary` and `clockify_timesheet_review`.

## Reproduction steps for each issue

### Finding 1 (already fixed)
1. Open `docs/api-coverage.md`
2. Search for `clockify_summary_report` — previously showed incorrect `GET /workspaces/{ws}/reports/summary` mapping

### Finding 2
1. Start the MCP server with valid credentials
2. Call `clockify_summary_report` with `start`/`end` for a range with entries
3. Compare the response shape against the actual API response from:
   ```
   POST https://reports.api.clockify.me/v1/workspaces/{ws}/reports/summary
   Body: {"dateRangeStart": "...", "dateRangeEnd": "...", "summaryFilter": {"groups": ["PROJECT"]}}
   ```

### Finding 3
1. Create a time entry linked to a project
2. Fetch it via `GET /workspaces/{ws}/user/{uid}/time-entries?hydrated=true`
3. If `projectName` is empty in the hydrated response, the summary report will show "(no project)" even though the project exists

## Cleanup performed

No test resources were created. All probes were read-only. No cleanup needed.

## Leftover test resources

None.

## Severity

| ID | Severity | Summary |
|----|----------|---------|
| 1 | P1 | Incorrect api-coverage.md endpoint mappings for summary/quick reports (FIXED) |
| 2 | P2 | Time-entries wrapper shape differs from real Reports Summary API shape |
| 3 | P3 | Hydrated time-entries may lack projectName, causing "(no project)" labels |
| 5 | P3 | No summaryFilter.groups parameter exposure — only project grouping available |

## Files changed

- `docs/api-coverage.md` — lines 71, 74: corrected endpoint descriptions

## Suggested next action

1. **Short-term (P2):** Consider adding a `clockify_reports_summary` Tier 2 tool that calls the actual `POST /reports/summary` endpoint on the reports host, complementing the existing time-entries wrapper approach. This would give users the billing/currency data and let them specify custom grouping dimensions.

2. **Short-term (P3):** Add a fallback project-name resolution step: when `projectName` is empty in a hydrated entry but `projectId` is set, resolve the name by looking up the project in the workspace. This mirrors what the real reports API does server-side.

3. **Documentation:** Note in the tool description or README that `clockify_summary_report` is a time-entries wrapper and differs from the Clockify Reports Summary API shape. A URL reference to the reports host endpoint could help advanced users.

## False positives / uncertainty

- **Finding 3 (projectName):** The incidence of entries with projectId but empty projectName in hydrated responses is unknown at scale. It may be specific to certain workspace states or entry creation methods. Further sampling across workspaces would clarify.
- **Finding 2 (shape mismatch):** The MCP server explicitly positions itself as a "safe wrapper" — users who need the real Reports API shape are not the target audience of this tool. This is more accurately a feature-gap note than a bug.

## Final recommendation

**PASS WITH CONCERNS** — The `clockify_summary_report` tool works correctly as designed: it aggregates time-entries and produces a clean project-based summary with totals, per-project rollups, and suggested follow-up actions. All tests pass, build and vet are clean, the doctor command works, and the tool schemas are well-defined.

The concerns are documentation accuracy (now fixed) and the architectural gap between the time-entries wrapper approach and the real Clockify Reports Summary API. If users expect the billing/currency/chart data from the actual reports endpoint, this tool will not satisfy them. The `source: "time-entries-wrapper"` metadata and the honest tool descriptions mitigate this, but the `api-coverage.md` document had been misleading until this fix.

For local/internal/community/self-hosted readiness, the tool is functional and safe. For full Clockify API parity, a Tier 2 tool wrapping the real `POST /reports/summary` endpoint would be the appropriate next step.
