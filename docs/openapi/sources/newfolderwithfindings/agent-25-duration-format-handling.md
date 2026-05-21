# QA Agent 25 - duration-format-handling

## Verdict
PASS WITH CONCERNS

## What I checked

1. **`internal/timeparse/timeparse.go`** — `ParseDuration`, `parseISO8601Duration`, `ParseDatetime`, `parseTimeOfDay`, `FormatISO` functions.
2. **`internal/timeparse/timeparse_test.go`** — Unit tests and fuzz tests for the timeparse package.
3. **`internal/clockify/models.go`** — `TimeInterval`, `TimeEntry`, `DurationSeconds()` model definitions.
4. **`internal/tools/entries.go`** — `AddEntry`, `UpdateEntry`, `ListEntries` tool implementations.
5. **`internal/tools/workflows.go`** — `LogTime`, `StartTimerArgs` tool implementations.
6. **`internal/tools/common.go`** — `timeEntryPutPayload`, `parseRangeInLocation`, `isBareDateString`.
7. **`internal/tools/registry.go`** — Tool schemas for time entry tools.
8. **`internal/tools/reports.go`** — Usage of `DurationSeconds()` in report computation.
9. **Live Clockify API** — Verified actual duration format in API responses, created/updated/deleted test entries.

## Live API probe lab files used

| File | Purpose |
|------|---------|
| `/tmp/clockify-livetest.env` | API key ****REDACTED****, workspace ID ****REDACTED**** |
| `TIMEENTRYDOC.md` | Clockify API time entry schema reference |
| `probes/lib/common.sh` | Probe library helpers |
| `CLAUDE.md` | Probe lab safety rules |
| `README.md` | Probe lab setup instructions |

## Commands run

```bash
# Unit tests
go test ./internal/timeparse/... -v -count=1
go test ./internal/timeparse/... -run "Fuzz" -fuzztime=5s
go test ./internal/tools/... -run "TestAdd|TestUpdate|TestEntry" -v -count=1

# Build
go build ./...

# Live API: inspect duration format in existing entries
curl -s "https://api.clockify.me/api/v1/workspaces/$WS/user/$USER/time-entries?page=1&page-size=5" \
  -H "X-Api-Key: ****REDACTED****"

# Live API: create entry with start+end, observe computed duration
curl -s -X POST "https://api.clockify.me/api/v1/workspaces/$WS/time-entries" \
  -H "X-Api-Key: ****REDACTED****" \
  -d '{"start":"2026-05-10T14:00:00Z","end":"2026-05-10T14:30:00Z",...}'

# Live API: create entry with start+duration (no end)
curl -s -X POST "https://api.clockify.me/api/v1/workspaces/$WS/time-entries" \
  -H "X-Api-Key: ****REDACTED****" \
  -d '{"start":"2026-05-10T14:00:00Z","duration":"PT30M",...}'

# Live API: create entry with mismatched duration to test API behavior
curl -s -X POST "https://api.clockify.me/api/v1/workspaces/$WS/time-entries" \
  -H "X-Api-Key: ****REDACTED****" \
  -d '{"start":"2026-05-10T16:00:00Z","end":"2026-05-10T17:00:00Z","duration":"PT9H99M",...}'

# Live API: check running vs stopped entry duration format
curl -s "https://api.clockify.me/api/v1/workspaces/$WS/user/$USER/time-entries?page=1&page-size=5" \
  -H "X-Api-Key: ****REDACTED****" \
  | jq '.[] | {duration: .timeInterval.duration, running: (.timeInterval.end == null)}'

# Live API: sub-minute entry to check rounding
curl -s -X POST "..." -d '{"start":"2026-05-10T15:00:00Z","end":"2026-05-10T15:00:30Z",...}'
```

## Live API probes run

### Probe 1: Duration format in API responses
Clockify returns `timeInterval.duration` as an ISO 8601 PT-formatted string for finished entries:
```json
{"duration": "PT2H"}       // 2 hours
{"duration": "PT2H30M"}    // 2 hours 30 minutes
{"duration": "PT45M"}      // 45 minutes
{"duration": "PT2H46M"}    // 2 hours 46 minutes
```
For running timers (no `end`), `duration` is `null`.

### Probe 2: Sub-minute entry rounding
Created entry: start=15:00:00Z, end=15:00:30Z -> API stored as 15:00:00Z-15:01:00Z, duration=`PT1M`.
**Finding**: Clockify rounds entries UP to the next full minute. The minimum stored duration is PT1M. Seconds are never present in the returned duration string.

### Probe 3: Duration-only creation (no end)
Created entry with `start` + `duration: "PT30M"` (no `end`). API created a **running timer** (end=null, duration=null). The API ignores the `duration` field when `end` is absent.

### Probe 4: Mismatched duration
Created entry with `start=16:00` + `end=17:00` + `duration: "PT9H99M"`. API returned `duration: "PT1H"` (computed from start/end, ignoring the provided duration). **The Clockify API always computes `duration` server-side from start and end; any client-supplied duration is ignored.**

### Probe 5: Updating to add end time
Updated a running entry to add an end time. API then computed and returned `duration: "PT30M"` based on start/end.

## Findings

### Finding 1: `timeEntryPutPayload` correctly excludes duration (PASS)
`internal/tools/common.go:20-40` — The PUT payload builder sends `start`, `end`, `description`, `projectId`, `billable`, and optionally `taskId`, `tagIds`, `customFields`. It does NOT send `duration`. **This is correct**: Probe 4 proved the API computes duration server-side from start/end and ignores any client-supplied duration. Sending duration would be redundant at best and could confuse consumers reading the code.

### Finding 2: `parseISO8601Duration` accepts non-standard unit ordering (P3)
`internal/timeparse/timeparse.go:187-236` — The ISO 8601 duration parser iterates through the string character by character and accumulates H/M/S components in whatever order they appear. This means `PT30M1H` is accepted and returns 1h30m instead of being rejected. ISO 8601 section 4.4.4.2 specifies the designators must appear in order (P, Y, M, W, D, T, H, M, S). However:
- The Clockify API always produces standard order (`PT[n]H[n]M`), so **consumer-side use is safe**.
- If a user passes a non-standard duration to a tool that accepts duration input (none currently do), the parser accepts it silently.

### Finding 3: No tests for `DurationSeconds()` (P2 - Test gap)
`internal/clockify/models.go:131-144` — The `DurationSeconds()` method has no dedicated unit tests. It is used in `reports.go:143` for report computation. Untested scenarios include:
- Running timers (no end time — falls back to `time.Now().UTC()`)
- Entries with unparseable start times (returns 0)
- Entries where end is before start (returns 0)
- Entries spanning multiple days

### Finding 4: `DurationSeconds()` is non-deterministic for running timers (P3 - Design note)
`internal/clockify/models.go:136-139` — When a time entry has no end time (running timer), `DurationSeconds()` uses `time.Now().UTC()` as the end time. This means two calls to the same running entry at different times return different values. While this is semantically correct for "elapsed time so far", it makes report output non-deterministic. This is by design but worth documenting.

### Finding 5: No `duration` input parameter in AddEntry/LogTime tools (P3 - Feature gap)
`internal/tools/registry.go:132-146` — The `clockify_add_entry` and `clockify_log_time` tools accept `start` and `end` datetime parameters but do not accept a `duration` parameter. The Clockify API docs list `duration` as a field in `TimeIntervalDtoV1`. However, live probes show the API ignores client-supplied duration (Probe 4), so adding this parameter would have no effect on the API call. The gap is purely in the user-facing schema — a user might reasonably expect to specify "2 hours" instead of computing the end time.

### Finding 6: No `FormatDuration` reverse conversion exists (PASS — intentional)
The codebase has `ParseDuration` (string -> `time.Duration`) but not `FormatDuration` (`time.Duration` -> ISO 8601 string). This is correct: the server never needs to send duration strings to the API (Probe 4), and Go's native `time.Duration` is used internally.

### Finding 7: Clockify API rounds seconds up to minutes (DOCUMENTED)
The Clockify API rounds all time entry durations to whole minutes. A 30-second entry becomes 1 minute. The returned duration format never includes seconds (`PT1M`, never `PT30S`). The `parseISO8601Duration` function does handle seconds if present, but the API never produces them.

## Reproduction steps for each issue

### Issue F3: Missing tests for DurationSeconds()
1. Open `internal/clockify/models.go:131`
2. Observe that `DurationSeconds()` has no corresponding test in `internal/clockify/client_test.go` or any other test file
3. Expected: Tests covering running timers, unparseable start, and end-before-start edge cases

### Issue F2: Non-standard ISO 8601 ordering accepted
```go
// internal/timeparse/timeparse_test.go
// Add this test case to TestParseISO8601Duration:
{"PT30M1H", 1*time.Hour + 30*time.Minute, false}, // accepted, but non-standard order
```
The function should either:
- Accept any order (document as intentional relaxation), OR
- Require H->M->S order and reject non-conforming input

## Cleanup performed

Deleted all test entries created during this run:
- `<REDACTED_ID>` — qa-agent-25-duration-probe-1 (HTTP 400, already deleted earlier)
- `<REDACTED_ID>` — qa-agent-25-duration-probe-2 (HTTP 204)
- `<REDACTED_ID>` — qa-agent-25-duration-probe-3-seconds (HTTP 204)
- `<REDACTED_ID>` — qa-agent-25-duration-probe-4-hms (HTTP 204)
- `<REDACTED_ID>` — qa-agent-25-duration-only-probe (HTTP 204)
- `<REDACTED_ID>` — qa-agent-25-duration-mismatch-probe (HTTP 204)

## Leftover test resources

None. All `qa-agent-25-*` prefixed test entries were deleted.

## Severity

| Finding | Severity | Rationale |
|---------|----------|-----------|
| F1: PUT payload excludes duration | PASS | Correct behavior confirmed by live probes |
| F2: Non-standard ISO ordering accepted | P3 | Only affects user-supplied input; API always produces standard order |
| F3: No tests for DurationSeconds() | P2 | Used in report computation; no test coverage |
| F4: DurationSeconds() non-deterministic | P3 | By design; documented behavior |
| F5: No duration input parameter | P3 | API ignores client-supplied duration; schema-only gap |
| F6: No FormatDuration | PASS | Not needed; API computes duration server-side |
| F7: Clockify API second rounding | DOCUMENTED | API behavior; not an MCP issue |

## Files changed

None. No code changes were required.

## Suggested next action

1. **Add unit tests for `DurationSeconds()`** (P2) — Cover running timers, unparseable start times, end-before-start, and multi-day entries. This is the highest-value follow-up.
2. **Decide on ISO 8601 ordering strictness** (P3) — Either document the relaxed ordering as intentional or add ordering validation to `parseISO8601Duration`.
3. **Consider adding `duration` as an optional input parameter** to `clockify_add_entry` and `clockify_log_time` (P3) — Although the API ignores it, providing it in the schema would improve the user-facing API and could be implemented as client-side convenience: if `duration` is provided without `end`, compute `end = start + duration` locally. This is a feature enhancement, not a bug fix.

## False positives / uncertainty

- **ISO 8601 fractional durations** (e.g., `PT1.5H`): Not tested because Clockify never produces them. The parser would reject them (`.` triggers "unexpected character" error). No action needed unless fractional durations appear in the API.
- **Negative durations**: Clockify should never produce negative durations (end before start). The parser and `DurationSeconds()` both handle this gracefully.
- **Overflow**: `strconv.Atoi` caps at `int` size. For practical Clockify durations (max ~24h per entry), this is not a concern.

## Final recommendation

**PASS WITH CONCERNS** — The duration format handling in the MCP server is functionally correct and well-aligned with the Clockify API. The ISO 8601 parser correctly handles all formats the API produces. The PUT payload excludes duration appropriately since the API computes it server-side. The primary concerns are the test gap for `DurationSeconds()` (P2) and the non-standard ISO ordering flexibility (P3), neither of which affects correctness. No production-blocking issues found.
