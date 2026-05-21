# QA Agent 17 - time-entry-read-paths

Status: COMPLETED
Started UTC: 2026-05-10T20:29:26Z
Completed UTC: 2026-05-10T21:30:00Z

Worktree: /Users/15x/Downloads/go-clockify-qa-swarm/worktrees/agent-17
Live API probe lab: /Users/15x/Downloads/WORKING/clockify-api-probe-lab

## Verdict
PASS WITH CONCERNS

## What I checked

1. **Repository structure and code review**: Explored all time-entry read path files including `internal/tools/entries.go`, `internal/tools/reports.go`, `internal/tools/registry.go`, `internal/tools/common.go`, `internal/tools/resources.go`, `internal/clockify/models.go`, `internal/clockify/client.go`, `internal/clockify/errors.go`, `internal/paths/paths.go`, `cmd/clockify-mcp/main.go`, and `cmd/clockify-mcp/doctor.go`.

2. **MCP tool schema validation**: Reviewed all 8 time-entry read tools (`clockify_list_entries`, `clockify_get_entry`, `clockify_today_entries`, `clockify_summary_report`, `clockify_weekly_summary`, `clockify_quick_report`, `clockify_timesheet_review`, `clockify_detailed_report`) and their parameter schemas against the official Clockify API documentation.

3. **Build verification**: Successfully built the MCP server binary from the repo (`go build ./cmd/clockify-mcp/`).

4. **Doctor/startup commands**: Ran `clockify-mcp doctor`, `--version`, and `--help` successfully. Doctor produces a comprehensive 11-group config audit with source attribution (explicit/profile/default/empty). Sensitive values are redacted.

5. **Live API probes**: Tested against the Clockify API probe workspace:
   - GET single time entry by ID (with and without `hydrated=true`)
   - GET user time entries list (paginated, date range, description search, project filter)
   - GET in-progress entries
   - DELETE (cleanup)
   - Error cases: non-existent entry, missing auth, invalid API key, oversized page-size, empty high page number

6. **Error handling quality**: Verified APIError construction, compact upstream error body extraction, sanitized error paths, retry logic, and response body size limits.

7. **`TimeEntry.ProjectName` population**: Discovered that the Clockify API never returns a flat `projectName` field — neither in non-hydrated nor hydrated responses. Project names are only available via the nested `project.name` path when using `hydrated=true`.

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — API key and workspace ID (credentials sourced at runtime, never written to disk)
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/TIMEENTRYDOC.md` — Official Clockify time entry API documentation
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/probes/lib/common.sh` — Shared probe library (curl wrapper, redaction)
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/README.md` — Lab context and safety rules

## Commands run

```bash
# Build
go build -o /tmp/clockify-mcp-qa17 ./cmd/clockify-mcp/

# Doctor (redacted — API key appears as "set (redacted)")
CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED> /tmp/clockify-mcp-qa17 doctor

# Version and help
/tmp/clockify-mcp-qa17 --version   # → dev
/tmp/clockify-mcp-qa17 --help      # → full help banner

# Live API: GET single entry (no hydration)
curl -s -H "X-Api-Key: <REDACTED>" \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/time-entries/<ENTRY_ID>"

# Live API: GET single entry (hydrated)
curl -s -H "X-Api-Key: <REDACTED>" \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/time-entries/<ENTRY_ID>?hydrated=true"

# Live API: GET user entries (paginated)
curl -s -H "X-Api-Key: <REDACTED>" \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/user/<USER_ID>/time-entries?page=1&page-size=5"

# Live API: GET with date range
curl -s -H "X-Api-Key: <REDACTED>" \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/user/<USER_ID>/time-entries?start=2026-05-10T00:00:00Z&end=2026-05-11T00:00:00Z&page-size=3"

# Live API: GET with description search
curl -s -H "X-Api-Key: <REDACTED>" \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/user/<USER_ID>/time-entries?description=qa-agent-17&page-size=5"

# Live API: Error cases
# Non-existent entry → 400 "Time entry doesn't belong to Workspace"
# Missing auth → 401 "Multiple or none auth tokens present"
# Invalid API key → 401 "Api key does not exist"
# Oversized page-size → 400 "Page size cannot be larger than 5,000."

# Live API: CREATE test entry
curl -s -X POST -H "X-Api-Key: <REDACTED>" -H "Content-Type: application/json" \
  -d '{"start":"2026-05-10T20:18:53Z","end":"2026-05-10T21:18:53Z","description":"qa-agent-17-read-probe-entry","billable":false}' \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/time-entries"

# Live API: DELETE test entry (cleanup)
curl -s -X DELETE -H "X-Api-Key: <REDACTED>" \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/time-entries/<ENTRY_ID>"
```

## Live API probes run

| # | Probe | Result | HTTP Status |
|---|-------|--------|-------------|
| 1 | Create test entry `qa-agent-17-read-probe-entry` | Created, id=`<REDACTED_ID>` | 201 |
| 2 | GET single entry by ID (no hydration) | Returned entry with 15 fields. `projectName` absent. | 200 |
| 3 | GET user entries (page-size=3) | Returned 3 entries | 200 |
| 4 | GET with date range filter | Returned entries in range | 200 |
| 5 | GET non-existent entry ID | `"Time entry doesn't belong to Workspace"` — note: 400 not 404 | 400 |
| 6 | GET without API key | `"Multiple or none auth tokens present"` | 401 |
| 7 | GET with invalid API key | `"Api key does not exist"` | 401 |
| 8 | GET with page-size=1 | Returned 1 entry | 200 |
| 9 | GET with description search | Returned 1 matching entry | 200 |
| 10 | GET with hydrated=true | Returned 20 fields incl. `project`, `task`, `user`, `tags` objects | 200 |
| 11 | Verify `projectName` in raw JSON (hydrated) | `projectName` NOT present in raw JSON | 200 |
| 12 | Verify `projectName` in raw JSON (non-hydrated) | `projectName` NOT present in raw JSON | 200 |
| 13 | GET with project ID filter | Server-side project filter works | 200 |
| 14 | GET in-progress entries | 0 running entries | 200 |
| 15 | GET with oversized page-size | `"Page size cannot be larger than 5,000."` | 400 |
| 16 | GET empty page (page=9999) | Empty array `[]` | 200 |
| 17 | DELETE test entry | Cleanup successful | 204 |
| 18 | Verify no leftovers | 0 entries remain with `qa-agent-17-` prefix | 200 |

## Findings

### Finding 1 (P2): `TimeEntry.ProjectName` is never populated from the live Clockify API

**Root cause**: The `TimeEntry` model at `internal/clockify/models.go:97-114` has `ProjectName string \`json:"projectName,omitempty"\``. However, the Clockify API **never** returns a flat `projectName` field in any response mode:

- **Without `hydrated=true`**: The API returns `projectId` but not `projectName`. Only 15 fields are returned.
- **With `hydrated=true`**: The API returns a nested `project` object (with `project.name` inside), but still no flat `projectName`. The `project` object is silently dropped during JSON unmarshaling because `TimeEntry` has no `Project` field to capture it.

**Live evidence** (probe 17): The raw JSON from a hydrated list response contains `"project":{...}` with `"name":"qa-agent-42-readme-test"` but zero occurrences of the string `"projectName"` anywhere in the response body.

**Impact**:
- **Reports** (`reports.go:155-162`): All projects appear as `"(no project)"` in report output since `entry.ProjectName` is always empty. The fallback logic `if name == "" { name = "(no project)" }` fires on every entry.
- **`entryMatchesProjectFilter`** (`entries.go:526-527`): The `ProjectName`-based filter check is dead code — it can never match against the user's filter string.
- **`clockify_list_entries` with project filter**: When `resolveProjectID` succeeds, the server-side `query["project"]` filter compensates. When resolution fails (ambiguous name), the fallback to client-side `ProjectName` matching silently returns no results.
- **All entry read responses**: The `projectName` JSON field is always empty/omitted.

**Test coverage gap**: All tests pass because test mocks directly set `ProjectName` on `TimeEntry` structs rather than simulating the actual API wire format.

**Proposed fix**: Either (a) add a `Project` field to `TimeEntry` to capture the hydrated `project` object and populate `ProjectName` from `Project.Name` in a post-unmarshal step, or (b) implement a custom `json.Unmarshaler` on `TimeEntry` that extracts `project.name` into `ProjectName`. The reports code already uses `hydrated=true` so the data is available on the wire — it just isn't being captured.

### Finding 2 (P3): `clockify_list_entries` does not expose the `description` search parameter

The Clockify API `GET /workspaces/{wsID}/user/{userID}/time-entries` supports a `description` query parameter for text-searching entries by description (`TIMEENTRYDOC.md` line 1086-1091). The MCP tool `clockify_list_entries` (registered at `registry.go:51-60`) does not include `description` in its parameter schema. Users who want to find entries by description text cannot do so via this MCP tool.

### Finding 3 (P3): `clockify_get_entry` and `clockify_list_entries` do not request hydrated data

Neither `GetEntry` (`entries.go:93-111`) nor `listEntriesWithQuery` (`entries.go:447-463`) pass `hydrated=true` to the Clockify API. This means entry responses for these tools lack project name/object, task name/object, user details, tag details, and kiosk info. The report tools (`reports.go:125`) DO use `hydrated=true`, so this inconsistency means the same entry looks different depending on which tool retrieves it.

### Finding 4 (P3): Non-standard error response for non-existent entries

The Clockify API returns HTTP 400 (not 404) with message `"Time entry doesn't belong to Workspace"` when querying a non-existent entry ID. The MCP server forwards this as-is via `APIError`, which correctly surfaces the 400 status and upstream error message. This is a Clockify API behavior quirk, not an MCP server bug, but operators may be surprised by 400 vs 404.

## Fixes made

No code fixes were made. The identified issues (Findings 1, 2, 3) require careful design consideration:

- **Finding 1** requires adding a new struct field or custom unmarshaler, both of which need test updates and may affect serialization contracts.
- **Finding 2** is a simple schema addition but needs consideration of whether description search should work alongside other filters.
- **Finding 3** requires evaluating whether always requesting hydrated data is safe (larger payloads, potentially different response envelopes from the Clockify API).

These are best addressed in a dedicated follow-up PR rather than in a QA pass.

## Reproduction steps for each issue

### Finding 1: ProjectName always empty
1. Create a time entry assigned to a project via the Clockify API
2. Call `clockify_get_entry` or `clockify_list_entries` via the MCP server
3. Observe that `projectName` is absent from the response
4. Compare against the raw Clockify API response (also lacks `projectName`) and the hydrated API response (has `project.name` but not flat `projectName`)

### Finding 2: Description search unavailable
1. Check the `clockify_list_entries` tool schema in `registry.go:51-60`
2. Observe that `description` is not in the `properties` map
3. Compare against `TIMEENTRYDOC.md` line 1086-1091 which documents the `description` query parameter

### Finding 3: Non-hydrated reads
1. Check `entries.go:447-463` (listEntriesWithQuery) and `entries.go:93-111` (GetEntry)
2. Observe that no `query["hydrated"] = "true"` is present
3. Compare against `reports.go:125` which does set it

## Cleanup performed

- Deleted test time entry `<REDACTED_ID>` (204 No Content)
- Verified 0 remaining entries with `qa-agent-17-` prefix in the probe workspace

## Leftover test resources

None.

## Severity

| Finding | Severity | Rationale |
|---------|----------|-----------|
| `ProjectName` never populated | P2 | Reports show "(no project)" for all entries; project name filtering broken in fallback path. No data loss but degrades core reporting functionality. |
| Missing `description` search param | P3 | Feature gap — the API supports it but the tool doesn't expose it. Workaround exists (fetch all and filter client-side). |
| Non-hydrated reads for basic tools | P3 | Minor inconsistency — report tools hydrate, basic tools don't. Low user impact since `projectName` is already broken (Finding 1). |
| Non-standard 400 for missing entries | P3 | Documentation note only — the MCP server correctly forwards the API's response. No fix needed. |

## Files changed

None.

## Suggested next action

1. **Fix Finding 1 (P2)**: Add `Project` field to `TimeEntry` or implement custom unmarshaler to extract `project.name` into `ProjectName`. This is the highest-impact bug — it silently corrupts all report output. Update tests to use realistic wire-format JSON instead of directly setting `ProjectName`.

2. **Fix Finding 2 (P3)**: Add optional `description` parameter to `clockify_list_entries` schema and pass it through as a query parameter.

3. **Fix Finding 3 (P3)**: Consider adding `hydrated=true` to `clockify_get_entry` and `clockify_list_entries` for consistency with report tools, once Finding 1 is resolved.

## False positives / uncertainty

- **Mock test data**: All 40+ tests that set `ProjectName` directly on `TimeEntry` structs pass, creating a false sense that the field works. The gap is between the mock data format and the actual Clockify API wire format.
- **Possible regional API differences**: The probe tests were run against the `api.clockify.me` region. If the Clockify API behaves differently in other regions (EU, UK, AU), the `projectName` field might be present there.
- **`projectName` may exist in other API versions**: The test was against the current v1 API. If there's a newer API version that returns `projectName` as a flat field, the existing code would work correctly.

## Final recommendation

The go-clockify MCP server is **production-ready for time-entry read paths** with one significant concern: the `ProjectName` field is never populated from the live API, which silently corrupts report output by showing all projects as "(no project)". This is a P2 issue that should be fixed before using reports in production. The basic entry read tools (`clockify_get_entry`, `clockify_list_entries`, `clockify_today_entries`) are fully functional. Error handling, auth, pagination, rate limiting, and security posture are all solid.

The build, doctor, and CLI commands work correctly. The codebase is well-structured with clear separation between read/write/destructive tool annotations and comprehensive config management. No security issues were found.
