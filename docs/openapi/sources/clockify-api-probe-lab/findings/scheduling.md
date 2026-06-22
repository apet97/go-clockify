# Finding: scheduling

## Endpoint(s) probed
| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /api/v1/workspaces/{ws}/scheduling/assignments?page=1&page-size=5 | 404 | fixtures/scheduling/api__workspaces_{ws}_scheduling_assignments_page-1_page-size-5.json |
| GET | api.clockify.me | /api/v1/workspaces/{ws}/scheduling/scheduling | 404 | fixtures/scheduling/api__workspaces_{ws}_scheduling_scheduling.json |
| GET | api.clockify.me | /api/v1/workspaces/{ws}/scheduling/capacity | 404 | fixtures/scheduling/api__workspaces_{ws}_scheduling_capacity.json |
| GET | api.clockify.me | /api/v2/workspaces/{ws}/scheduling/assignments | 404 | fixtures/scheduling/v2__workspaces_{ws}_scheduling_assignments_page-1_page-size-5.json |
| GET | scheduling.api.clockify.me | /api/v1/workspaces/{ws}/assignments | 404 | fixtures/scheduling/scheduling__workspaces_{ws}_assignments_page-1_page-size-5.json |
| GET | scheduling.api.clockify.me | /api/v1/workspaces/{ws}/scheduling/assignments | 404 | fixtures/scheduling/scheduling__workspaces_{ws}_scheduling_assignments_page-1_page-size-5.json |
| GET | pto.api.clockify.me | /api/v1/workspaces/{ws}/assignments | 404 | fixtures/scheduling/scheduling-pkce__workspaces_{ws}_assignments_page-1_page-size-5.json |
| GET | api.clockify.me | /api/v1/workspaces/{ws}/scheduling/assignments/all?start=...&end=... | **200** | fixtures/scheduling/assignments-all.json |
| GET | api.clockify.me | /api/v1/workspaces/{ws}/scheduling/assignments/users/{userId}/totals?start=...&end=... | **200** | fixtures/scheduling/user-totals.json |
| POST | api.clockify.me | /api/v1/workspaces/{ws}/scheduling/assignments/projects/totals | **200** | fixtures/scheduling/projects-totals.json |
| POST | api.clockify.me | /api/v1/workspaces/{workspaceId}/scheduling/assignments/user-filter/totals | 200 | live probe 2026-06-22 (Leftovers:0) |

**Root cause: the path is wrong, not the host.** The host is the standard `api.clockify.me/api/v1`. go-clockify hits `/scheduling/assignments` (no suffix), which 404s. The live path is `/scheduling/assignments/all` and requires `start` and `end` as mandatory query parameters.

## Request headers (no secrets)
- X-Api-Key: [REDACTED]
- Content-Type: not sent (all GET; no body)

## Request body (when applicable)
n/a — all probes were read-only GETs.

## Response shape

### GET /api/v1/workspaces/{ws}/scheduling/assignments/all — 200
Bare array — **no wrapper object**:
```json
[
  {
    "id": "<string>",
    "workspaceId": "<string>",
    "userId": "<string>",
    "userName": "<string>",
    "projectId": "<string>",
    "projectName": "<string>",
    "clientId": "<string>",
    "clientName": "<string>",
    "projectColor": "#009688",
    "period": {
      "start": "2025-03-02T00:00:00Z",
      "end": "2025-03-09T23:59:59.999Z"
    },
    "hoursPerDay": 8.0,
    "startTime": null,
    "note": null,
    "billable": true,
    "taskId": "<string or null>",
    "taskName": "<string or null>",
    "projectBillable": true,
    "projectArchived": true
  }
]
```
`period` is a nested object (`{start, end}`), not a flat date pair. `hoursPerDay` is a float. `startTime`, `note`, `taskId`, `taskName` are all nullable.

Required query params (confirmed live): `start` and `end` in `yyyy-MM-ddThh:mm:ssZ` format. Optional: `page`, `page-size` (hyphenated), `sort-column` (`PROJECT|USER|ID`), `sort-order` (`ASCENDING|DESCENDING`), `name`.

### GET /api/v1/workspaces/{ws}/scheduling/assignments/users/{userId}/totals — 200
Flat object:
```json
{
  "userId": "<string>",
  "workspaceId": "<string>",
  "userName": "<string>",
  "userImage": "<url string>",
  "userStatus": "ACTIVE",
  "capacityPerDay": 3600.0,
  "workingDays": ["MONDAY", "TUESDAY", "WEDNESDAY", "THURSDAY", "FRIDAY"],
  "totalHoursPerDay": [
    { "date": "2025-03-02T00:00:00Z", "totalHours": 8.0 }
  ]
}
```
`capacityPerDay` is in **seconds** (3600 = 1 hr/day; docs note 25200 = 7 hr/day). Required query params: `start`, `end`. Optional: `page`, `page-size`.

### GET /api/v1/workspaces/{ws}/scheduling/assignments (wrong path — no /all suffix) — 404
```json
{"message": "No static resource api/v1/workspaces/{ws}/scheduling/assignments.", "code": 3000}
```

## Cleanup behavior
Read-only probe — no entities created. `cleanup-registry/scheduling.tsv` not written. Nothing to clean up.

## Recommended go-clockify change

### Bug: missing `/all` suffix and missing required query params
- File: `internal/tools/tier2_scheduling.go`
- Function: `listSchedulingAssignments` (or equivalent function that constructs the GET URL for assignments)
- Change: Append `/all` to the path (`/workspaces/{ws}/scheduling/assignments/all`) and add `start` and `end` as required query parameters. Without both params the endpoint 404s regardless of the suffix.

### Secondary: user-totals path (confirm it's correct)
- File: `internal/tools/tier2_scheduling.go`
- Function: `getUserSchedulingTotals` (or equivalent)
- Change: Verify the path is `/scheduling/assignments/users/{userId}/totals` with `start` and `end` query params. This endpoint works correctly on the standard host — if it also 404s in go-clockify, the most likely cause is the same missing params rather than a host issue.

## Test that flips from pinned-error to success-path
- Test: the test for `listSchedulingAssignments` in `tests/tier2_scheduling_test.go`
- Action: Remove the `expectErr` annotation (the 404 from the missing `/all` suffix). Replace with an assertion that the result is a slice (`[]map[string]any`) and that when assignments exist, `result[0]["period"]` is a non-nil map with `"start"` and `"end"` string keys. The test fixture or live call must supply `start` and `end` params — if they are currently omitted from the test, add them.

## Open questions

1. **`page-size` vs `pageSize` param name.** The `assignments/all` endpoint uses `page-size` (hyphenated, consistent with the rest of `api.clockify.me`). The user-totals and project-totals endpoints use `pageSize` (camelCase, per SCHED.md). Verify which convention each scheduling endpoint actually accepts to avoid silent param drops.

2. **Project totals (`POST /scheduling/assignments/projects/totals`) — RESOLVED.** Live probe (2026-05-02) confirms: `POST /api/v1/workspaces/{ws}/scheduling/assignments/projects/totals` with body `{"start":"...","end":"...","pageSize":2}` returns **200, bare array** of `{workspaceId, projectId, projectName, projectColor, projectArchived, clientName, totalHours, assignments:[{date, hasAssignment}], milestones, projectBillable, taskId, taskName}`. See `fixtures/scheduling/projects-totals.json`. If go-clockify has a `listSchedulingProjectTotals` that hits the wrong path (`/scheduling/projects/totals` without `assignments/` segment), it will 404.

3. **Scheduling feature plan gating.** The workspace returned live data, so the SCHEDULING feature is enabled. If go-clockify's tests run against a workspace where scheduling is not enabled, they may get 403 rather than the 404 the campaign saw. The pinned error should be re-checked against a scheduling-enabled workspace before declaring the test flipped.

4. **`capacityPerDay` unit.** The field is documented as seconds (25200 = 7 hr/day) but the live value is 3600 (1 hr/day). This may be a workspace configuration artifact, not a unit bug. Confirm before exposing this field as hours in any go-clockify tool output.

## Live read-side promotions (2026-06-20)

Captured HTTP 200 live this session against the sandbox. The older rows
above carry `?start=...&end=...` query strings, which `normalize_path`
keeps verbatim, so they never matched the clean merged operation key and
left these ops at `probe-documented`. These clean-path rows (no query
string, canonical `{workspaceId}`/`{userId}`/`{projectId}`) match the
operation key and flip each op to `live-success`. Fixtures are
documentary + gitignored.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /workspaces/{workspaceId}/scheduling/assignments/all | 200 | fixtures/live-shape/scheduling-assignments-all.json |
| GET | api.clockify.me | /workspaces/{workspaceId}/scheduling/assignments/projects/totals/{projectId} | 200 | fixtures/live-shape/scheduling-project-totals.json |
| GET | api.clockify.me | /workspaces/{workspaceId}/scheduling/assignments/users/{userId}/totals | 200 | fixtures/live-shape/scheduling-user-totals.json |
