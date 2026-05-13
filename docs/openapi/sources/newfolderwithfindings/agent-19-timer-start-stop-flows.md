# QA Agent 19 - timer-start-stop-flows

## Verdict
FAIL

## What I checked

1. **MCP build** — `go build ./cmd/clockify-mcp/...` (PASS)
2. **Timer tool registration** — `clockify_start_timer`, `clockify_stop_timer`, `clockify_timer_status` all present in tools/list with correct schemas (PASS)
3. **Unit tests** — `TestTimerStatusRunning`, `TestTimerStatusNotRunning`, `TestTimerStatus_NoRunning`, `TestTimerStatus_Running` all pass after fix (PASS)
4. **Live API probes** — start, stop, status, idempotent stop, second-timer auto-stop, empty description (see details below, FAIL on P1)
5. **MCP server startup smoke** — server starts and responds to initialize + tools/list with live credentials (PASS)
6. **Doctor command** — `clockify-mcp doctor` outputs comprehensive config audit (PASS)
7. **Tool schema validation** — parameter names, types, required fields match API docs (PASS)
8. **StopTimer endpoint correctness** — PATCH path matches Clockify API docs (PASS)
9. **Switch project workflow** — handles 404 gracefully, tested in unit tests (PASS)
10. **Doc/impl consistency** — found 2 inconsistencies, both fixed (PASS after fix)

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — API key, workspace ID (redacted in report)
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/TIMEENTRYDOC.md` — Clockify Time Entry API reference
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/CLAUDE.md` — probe lab rules

## Commands run

```
go build ./cmd/clockify-mcp/...
go test ./internal/tools/ -run "TestTimerStatus|TestStopTimer|TestStartTimer|TimerStatus" -v -count=1
go test ./internal/tools/ -count=1

# MCP server startup smoke
CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED> \
  go run ./cmd/clockify-mcp --transport stdio < initialize+list

# Doctor command
go run ./cmd/clockify-mcp doctor
```

## Live API probes run

### Probe 1: Start timer (no project)
`POST /workspaces/{ws}/time-entries` with `{"start":"...","description":"qa-agent-19-timer-probe-1"}`
→ 201 Created. Entry created with `end:null`, `type:REGULAR`. Correct.

### Probe 2: Check timer running status via page-size=1
`GET /workspaces/{ws}/user/{uid}/time-entries?page-size=1`
→ Running timer found when it's the most recent. Correct (but fragile — see P1).

### Probe 3: Stop timer
`PATCH /workspaces/{ws}/user/{uid}/time-entries` with `{"end":"..."}`
→ 200 OK. Entry returned with `end` set and `duration` populated. Correct.

### Probe 4: Verify stopped
`GET /workspaces/{ws}/time-entries/{id}`
→ Entry shows `end` and `duration`. Correct.

### Probe 5: Idempotent stop (no running timer)
`PATCH /workspaces/{ws}/user/{uid}/time-entries` with `{"end":"..."}`
→ 404 Not Found. Code propagates error — functional but UX could be cleaner.

### Probe 6: Start timer with project
`POST /workspaces/{ws}/time-entries` with `projectId` set
→ Entry created with project assigned. `projectName` is null (API limitation). Correct.

### Probe 7: Start second timer (auto-stop first)
Starting a second timer while one is running auto-stops the first timer server-side.
→ The MCP doesn't warn about this. The first timer is silently ended. (P2)

### Probe 8: Empty description
`POST /workspaces/{ws}/time-entries` with `"description":""`
→ Accepted. Clockify allows empty descriptions. Correct.

### Probe 9: TimerStatus with page-size=1 misses running timer (P1 bug reproduction)
Running timer at `20:00:00Z`, finished entry at `21:00:00Z`.
`page-size=1` returned the finished entry (not the running one).
`in-progress=true` correctly returned the running timer.

### Probe 10: in-progress=true with no running timer
→ Returns `[]` (empty array). Clean.

### Probe 11: PATCH stop with no running timer returns 404
→ HTTP 404. Code propagates error via `clockify.APIError`.

## Findings

### P1 — TimerStatus uses page-size=1 instead of in-progress=true (FIXED)

**File:** `internal/tools/timer.go:90`
**Severity:** P1 — correctness bug that can cause incorrect state reporting

The `TimerStatus` function fetches the most recent time entry with `?page-size=1` and checks if it's running. But Clockify sorts entries by start time descending, so a recently FINISHED entry with a later start time can appear before a RUNNING timer with an earlier start time. This causes `TimerStatus` to report `running: false` when a timer IS actually running.

**Reproduction:**
1. Start a timer at `20:00:00Z` (description: "qa-agent-19-running")
2. Create a finished entry at `21:00:00Z` → `21:05:00Z` (description: "qa-agent-19-finished")
3. Call `TimerStatus` — it sees the `21:00` finished entry and reports `running: false`
4. But `in-progress=true` correctly shows the running timer at `20:00`

**Root cause:** `page-size=1` without a sort or filter parameter relies on undocumented API ordering behavior.

**Fix applied:** Changed `map[string]string{"page-size": "1"}` to `map[string]string{"in-progress": "true"}` and updated the matching unit test in `context_test.go`.

**Files changed:**
- `internal/tools/timer.go` — changed query parameter
- `internal/tools/context_test.go` — updated assertion from `page-size=1` to `in-progress=true`

### P2 — StopTimer does not check for running timer before PATCH

**File:** `internal/tools/timer.go:64-87`
**Severity:** P2 — UX issue, no data loss

When no timer is running, the PATCH stop endpoint returns HTTP 404. The MCP propagates this as a raw error to the user instead of returning a clean "no timer running" response. The `clockify_switch_project` workflow already handles this gracefully (lines 239-248 of workflows.go), but standalone `StopTimer` does not.

**Suggested fix:** Check for running timer via `in-progress=true` before calling PATCH. Return `ResultEnvelope{OK: true, Action: "clockify_stop_timer", Data: map[string]any{"stopped": false, "reason": "no_running_timer"}}` if none is running.

Note: The tool is correctly marked as `idempotentHint: true` since repeated calls don't cause destructive side effects (404 is clean).

### P2 — StartTimer does not warn about auto-stopping previous timer

**File:** `internal/tools/timer.go:14-62`
**Severity:** P2 — informational gap

When a timer is already running and `StartTimer` is called, the Clockify API auto-stops the previous timer server-side. The MCP code doesn't check for this or inform the user. The auto-stopped entry's end time is set by the Clockify server, not by the user.

**Suggested fix:** Before creating a new timer, check for running timer via `in-progress=true`. If one exists, either:
- Include a warning in the response that the previous timer was auto-stopped, OR
- Call StopTimer explicitly first and include the stopped entry in the response (like `switch_project` does)

### P2 — docs/api-coverage.md stop_timer endpoint typo (FIXED)

**File:** `docs/api-coverage.md:98`
**Severity:** P2 — documentation accuracy

Documented endpoint as `PATCH /workspaces/{ws}/user/{uid}/time-entries/{id}` with extraneous `/{id}`. The correct Clockify API endpoint (per TIMEENTRYDOC.md line 1196) and the MCP code implementation both use `PATCH /workspaces/{ws}/user/{uid}/time-entries` without `/{id}`.

**Fix applied:** Removed `/{id}` from documented endpoint.

### P3 — StartTimer missing billable parameter

**File:** `internal/tools/registry.go:110`
**Severity:** P3 — minor feature gap

`clockify_add_entry` supports `billable:boolean` but `clockify_start_timer` does not. This is a minor inconsistency. Could be added as an optional parameter.

## Fixes made

1. **`internal/tools/timer.go`** — Changed TimerStatus from `?page-size=1` to `?in-progress=true` (P1 fix)
2. **`internal/tools/context_test.go`** — Updated test assertion to match new query parameter (P1 fix)
3. **`docs/api-coverage.md`** — Removed incorrect `/{id}` from stop_timer endpoint path (P2 fix)

All 3 files committed. Full `go test ./internal/tools/` passes (7.570s).

## Reproduction steps for each issue

### P1: TimerStatus misses running timer
```
1. POST /workspaces/{ws}/time-entries  {"start":"2026-05-10T20:00:00Z","description":"running"}
2. POST /workspaces/{ws}/time-entries  {"start":"2026-05-10T21:00:00Z","end":"2026-05-10T21:05:00Z","description":"finished"}
3. GET /workspaces/{ws}/user/{uid}/time-entries?page-size=1
   → Returns entry from step 2 (finished), NOT the running timer from step 1
4. GET /workspaces/{ws}/user/{uid}/time-entries?in-progress=true
   → Correctly returns entry from step 1 (running)
```

### P2: StopTimer 404 when idle
```
1. Ensure no timer is running
2. Call clockify_stop_timer
3. Observe HTTP 404 error propagated to user
```

### P2: StartTimer silent auto-stop
```
1. Start timer A on project X
2. Start timer B on project Y
3. Observe: Timer A is silently stopped by the API, no notification from MCP
```

## Cleanup performed

- Deleted 7 qa-agent-19- prefixed time entries
- Deleted 2 zero-duration junk entries created by PATCH no-timer probes
- Verified no leftover test resources

## Leftover test resources

None.

## Files changed

| File | Change |
|------|--------|
| `internal/tools/timer.go` | P1 fix: `page-size=1` → `in-progress=true` |
| `internal/tools/context_test.go` | P1 test: updated assertion |
| `docs/api-coverage.md` | P2 fix: removed incorrect `/{id}` |

## Suggested next action

1. **Apply P2 fix for StopTimer** — Add running-timer check before PATCH, return clean no-op response when idle
2. **Apply P2 fix for StartTimer** — Warn/notify about auto-stopped previous timer
3. **Apply P3 enhancement** — Add `billable` parameter to `clockify_start_timer`
4. **Consider live-contract test** — Add a live E2E test that creates a running timer, creates a finished entry with later start, then calls `clockify_timer_status` and asserts `running: true`

## False positives / uncertainty

- The PATCH stop endpoint behavior when no timer is running was observed to return both 404 (tested directly) and in one earlier probe returned what appeared to be a newly created zero-duration entry. The inconsistency may depend on timing or API version. The 404 behavior was confirmed in the final probe (HTTP 404, null body). More thorough testing under controlled conditions would be prudent.
- TimerStatus `page-size=1` behavior: While the P1 bug was reproduced consistently with entries having specific timestamps, the API's default sort order may depend on other factors (e.g., presence of other entries, workspace settings). The `in-progress=true` fix eliminates all such uncertainty.

## Final recommendation

**FAIL** due to P1 correctness bug (TimerStatus can report `running: false` when a timer IS running).

The P1 fix is applied and tested. The remaining P2 issues are well-understood and have clear fix paths. The MCP server builds, starts, registers all timer tools, and handles the core start/stop/status flows correctly against the live Clockify API. After reviewing the P2 fixes and applying the P3 enhancement, this area would be production-ready.
