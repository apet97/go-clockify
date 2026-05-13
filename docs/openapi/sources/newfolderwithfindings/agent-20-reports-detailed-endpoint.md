# QA Agent 20 - reports-detailed-endpoint

## Verdict
PASS WITH CONCERNS

## What I checked

- `clockify_detailed_report` (Tier 1 read-only tool): handler logic, parameter validation, output schema, pagination, date-range parsing, project filtering, max_entries cap enforcement, include_entries default
- `clockify_get_shared_report` / `clockify_export_shared_report` / `clockify_create_shared_report` / `clockify_update_shared_report` / `clockify_delete_shared_report` / `clockify_list_shared_reports` (Tier 2 shared_reports group): API host routing, field name correctness, path construction, export type handling
- Unit test coverage for all report tools
- Docker build and containerized doctor command
- Live Clockify API probe for time-entries aggregation endpoint
- Auth failure behavior against the live API
- Output schema consistency between handler return types and registered schemas

## Live API probe lab files used

| File | Purpose |
|------|---------|
| `/tmp/clockify-livetest.env` | API key, workspace ID, live-confirm guard |
| `probes/lib/common.sh` | Shared curl wrapper, redaction, fixture save, cleanup registry |
| `probes/shared-reports.sh` | Read-only shared-reports host+path discovery |
| `fixtures/shared-reports/*.json` | Redacted upstream response shapes (list, single-get, export, create, update, delete) |
| `findings/shared-reports.md` | Prior lab finding — host/path/field-name corrections applied to go-clockify |
| `docs/official-api-notes.md` | Per-domain API reference notes compiled before probing |

Credentials were loaded from `/tmp/clockify-livetest.env` and never written to disk or printed. The workspace is confirmed as `CLOCKIFY_LIVE_WORKSPACE_CONFIRM == CLOCKIFY_WORKSPACE_ID`.

## Commands run

```sh
# Build
go build ./cmd/clockify-mcp/

# Unit tests (all pass)
go test ./internal/tools/ -run "TestDetailedReport|TestReports|TestAggregate|TestSharedReport|TestTier2.*Report" -v -count=1

# Doctor command (local)
go run ./cmd/clockify-mcp/ doctor

# Docker build
docker build -f deploy/Dockerfile -t clockify-mcp-test .

# Docker doctor
docker run --rm -e CLOCKIFY_API_KEY CLOCKIFY_WORKSPACE_ID clockify-mcp-test doctor

# Live API probes (redacted)
curl -s -H "X-Api-Key: ****REDACTED****" "https://api.clockify.me/api/v1/user"
curl -s -H "X-Api-Key: ****REDACTED****" "https://api.clockify.me/api/v1/workspaces/${WS}/user/${UID}/time-entries?..."
curl -s -H "X-Api-Key: ****REDACTED****" "https://reports.api.clockify.me/v1/workspaces/${WS}/shared-reports?..."
curl -s -H "X-Api-Key: ****REDACTED****" "https://reports.api.clockify.me/v1/shared-reports/${ID}?exportType=JSON_V1"
```

## Live API probes run

| # | Method | Host | Path | Status | Notes |
|---|--------|------|------|--------|-------|
| 1 | GET | api.clockify.me | `/api/v1/user` | 200 | User ID retrieved successfully |
| 2 | GET | api.clockify.me | `/api/v1/workspaces/{ws}/user/{uid}/time-entries?start=...&end=...&page-size=3&hydrated=true` | 200 | 3 hydrated entries returned with full user/project/tag data |
| 3 | GET | api.clockify.me | `/api/v1/workspaces/{ws}/user/{uid}/time-entries?start=...&end=...&page-size=1&hydrated=true` | 200 | Pagination edge case: page-size=1 works correctly |
| 4 | GET | api.clockify.me | `/api/v1/workspaces/{ws}/user/{uid}/time-entries?start=...&end=...&page-size=50&hydrated=true` | 200 | 7 entries in 7-day range |
| 5 | GET | api.clockify.me | `/api/v1/workspaces/{ws}/user/{uid}/time-entries?...&project={id}` | 200 | 3 entries filtered to Marketing Campaign Q3 |
| 6 | GET | api.clockify.me | `/api/v1/workspaces/{ws}/user/{uid}/time-entries?start=later...&end=earlier...` | 400 | Upstream rejects end-before-start with clear error |
| 7 | GET | api.clockify.me | `/api/v1/user` with invalid key | 401 | `{"message":"Api key does not exist","code":4003}` |
| 8 | GET | reports.api.clockify.me | `/v1/workspaces/{ws}/shared-reports?page=1&pageSize=5` | 200 | 5 reports returned, total=74 |
| 9 | GET | api.clockify.me | `/api/v1/workspaces/{ws}/shared-reports` | 404 | Confirms wrong host for shared reports |
| 10 | GET | reports.api.clockify.me | `/api/v1/workspaces/{ws}/shared-reports` | 404 | Confirms wrong prefix on reports host |

## Findings

### F1 (P2): Output schema type mismatch — `clockify_detailed_report`

**Location**: `internal/tools/output_schemas.go:38`

```go
"clockify_detailed_report": envelopeSchemaFor[SummaryData]("clockify_detailed_report"),
```

**What's wrong**: The handler at `internal/tools/reports.go:483-499` returns `map[string]any`, not `SummaryData`. The schema is generated from the `SummaryData` struct but the Go runtime type passed to `ok()` is `map[string]any`.

**Impact**: No functional impact on MCP clients — the JSON serialization is structurally identical (same field names, `entries` is `omitempty` on both paths). The generated JSON schema correctly marks `entries` as non-required. The issue is code-level type hygiene.

**Suggested fix**: Either:
- (Preferred) Change `envelopeSchemaFor[SummaryData]` to `envelopeOpaque("clockify_detailed_report")` to match the actual return type, OR
- Refactor `DetailedReport` to return `SummaryData` directly (same JSON output, cleaner Go type).

### F2 (P3): DetailedReport returns `map[string]any` while SummaryReport returns `SummaryData`

**Location**: `internal/tools/reports.go:288-500`

**What's wrong**: `SummaryReport` (line 315) and `WeeklySummary` (line 364) both construct typed structs (`SummaryData`, `WeeklySummaryData`). `DetailedReport` (line 483) constructs an identical `map[string]any` instead. The code paths produce identical JSON but the Go-level consistency is poor.

**Impact**: No functional impact. Makes the code harder to reason about and causes the schema type mismatch in F1.

**Suggested fix**: Refactor `DetailedReport` to return a `SummaryData` struct, matching the pattern used by `SummaryReport`.

### F3 (P2): dockerized transport defaults differ from local stdio

**Location**: `cmd/clockify-mcp/` configuration loading

**What's wrong**: Running `docker run clockify-mcp-test doctor` shows `Load() result: ERROR MCP_OIDC_ISSUER is required`. The container default transport is `streamable_http`, which requires OIDC auth configuration. The local `go run` default is `stdio`, which does not.

**Impact**: Users following a Docker-first path may hit this confusing error. The error message is accurate but the default transport selection differs between Docker and local development.

**Suggested fix**: Document in README/Docker section that `MCP_TRANSPORT=stdio` must be set for single-user Docker deployments, or switch the Docker default to stdio.

### Verified Pass:

All the following were validated and passed:

| Check | Status | Evidence |
|-------|--------|----------|
| Build succeeds | PASS | `go build ./cmd/clockify-mcp/` — no errors |
| Unit tests | PASS | 25+ tests, all pass (detailed report, shared reports, pagination, caps, project filters) |
| Docker build | PASS | Multi-stage Docker build completes without errors |
| Tool schema: `start`/`end` required | PASS | Correctly marked required in tool descriptor (registry.go:214) |
| Tool schema: `include_entries` defaults to true | PASS | Handler at reports.go:452-457 defaults to true |
| Tool schema: `max_entries` param | PASS | Has minimum:0, description documents server cap |
| Tool schema: `timezone` optional | PASS | Falls back to CLOCKIFY_TIMEZONE or local/server |
| Tool schema: `project` optional | PASS | Resolves name to ID via resolveProjectID |
| Date range edge case: end before start | PASS | `parseRangeInLocation` rejects with "end must be after start" |
| Date range edge case: bare date (YYYY-MM-DD) | PASS | `isBareDateString` auto-adds 1 day to make end > start |
| Pagination: page-size clamped to [50,200] | PASS | aggregateEntriesRange enforces bounds |
| Pagination: 1000-page safety stop | PASS | Returns actionable error, not silent truncation |
| Cap enforcement: include_entries=true exceeds max_entries | PASS | Fails closed with "entry cap of N exceeded" |
| Cap enforcement: include_entries=false bypasses cap | PASS | Totals computed correctly, memory bounded |
| Pagination meta: structured pagination+limits | PASS | No legacy `warning` string in meta |
| Shared reports: host routing | PASS | `ReportsBaseURL()` swaps to `reports.api.clockify.me/v1` |
| Shared reports: field names | PASS | `createSharedReport` sends `type`/`filter` not `reportType`/`filters` |
| Shared reports: list query params | PASS | `pageSize` (camelCase) not `page-size` |
| Shared reports: single-get path | PASS | Bare-id `/shared-reports/{id}` (no workspace segment) |
| Shared reports: export path | PASS | Bare-id with `?exportType=` query param |
| Shared reports: JSON_V1 export stays decoded | PASS | Test confirms JSON export returns decoded object |
| Shared reports: binary export returns envelope | PASS | Base64-encoded body with content-type and filename |
| Shared reports: delete uses workspace-prefixed path | PASS | Dry-run preview fetches via bare-id GET first |
| Shared reports: type enum completeness | PASS | 19 upstream values in `sharedReportTypes` variable |
| Shared reports: filter schema completeness | PASS | Required fields exportType/dateRangeStart/dateRangeEnd documented |
| Auth failure | PASS | 401 with clear error message |
| Suggested actions | PASS | Both clockify_list_entries + clockify_log_time suggestions included |
| By-project rollup sorting | PASS | Sorted by TotalSeconds descending, then by name |

## Fixes made

No code fixes were applied. The identified issues (F1, F2, F3) are minor code-quality/documentation concerns without functional impact. They are documented above with suggested fixes for the maintainer to evaluate.

## Reproduction steps for each issue

### F1/F2: Output schema type mismatch
1. Open `internal/tools/output_schemas.go:38`
2. Observe `envelopeSchemaFor[SummaryData]("clockify_detailed_report")`
3. Open `internal/tools/reports.go:483` — handler builds `map[string]any`
4. The schema claims `SummaryData` type; the handler returns `map[string]any`

### F3: Docker transport default mismatch
1. `docker build -f deploy/Dockerfile -t clockify-mcp-test .`
2. `docker run --rm -e CLOCKIFY_API_KEY -e CLOCKIFY_WORKSPACE_ID clockify-mcp-test doctor`
3. Observe `Load() result: ERROR MCP_OIDC_ISSUER is required`
4. The container defaults to `streamable_http` which needs OIDC auth

## Cleanup performed

No test resources were created. All probes were read-only (GET requests only). No prefixed resources to clean up.

## Leftover test resources

None.

## Severity

| ID | Severity | Description |
|----|----------|-------------|
| F1 | P2 | Output schema type mismatch (SummaryData vs map[string]any) |
| F2 | P3 | Inconsistent return type between DetailedReport and SummaryReport |
| F3 | P2 | Docker transport default differs from local stdio default |

## Files changed

None.

## Suggested next action

1. **Fix F1** (low effort, high clarity): Change `output_schemas.go:38` to use `envelopeOpaque("clockify_detailed_report")` so the schema declaration matches the handler's actual return type. This is a one-line change.
2. **Fix F2** (medium effort): Refactor `DetailedReport` in `reports.go` to return `SummaryData` instead of `map[string]any`, matching the pattern in `SummaryReport`. Then revert F1's opaque schema back to `envelopeSchemaFor[SummaryData]`.
3. **Fix F3** (low effort): Document in README or Dockerfile that `MCP_TRANSPORT=stdio` is needed for single-user Docker deployments, or change the Docker entrypoint default.
4. Consider adding a live e2e test that creates a shared report with type=DETAILED on the probe workspace and verifies the round-trip (create, get, export, delete). The existing probe lab has fixtures confirming the API shape works.

## False positives / uncertainty

- **F1/F2**: The JSON schema output is actually correct (same field names, `entries` uses `omitempty`). An MCP client would see no difference. This is purely a code-quality and maintainability issue.
- **Shared reports host**: Confirmed correct at `reports.api.clockify.me/v1`. If Clockify ever migrates reports to a different host, `ReportsBaseURL()` will need updating, but the current logic is sound.
- **Docker transport**: The `streamable_http` default may be intentional for production deployment. The `stdio` variant is for local/dev use. This is a documentation issue, not a code bug.

## Final recommendation

**PASS WITH CONCERNS** — the `reports-detailed-endpoint` area is functionally correct and safe for production use. All unit tests pass, the Docker build succeeds, and live API probes confirm the handler logic matches upstream Clockify API behavior.

The three findings (F1-F3) are cosmetic/documentation issues that do not affect correctness, security, or data integrity. F1 is the only code-level concern (a type mismatch in the schema registry), and it has zero impact on JSON output or client behavior.

The shared-reports Tier 2 handlers have already been corrected for the host/path/field-name issues discovered in the prior probe lab campaign. The `ReportsBaseURL()` client method, the `sharedReportTypes` enum, and the body field names (`type`/`filter`) are all correct per the live lab fixtures.
