# Finding: holidays

## Endpoint(s) probed
| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /api/v1/workspaces/{ws}/holidays | 200 | fixtures/holidays/list.json |
| GET | api.clockify.me | /api/v1/workspaces/{ws}/holidays/in-period | 200 | fixtures/holidays/in-period.json |
| POST | api.clockify.me | /api/v1/workspaces/{ws}/holidays | 200 | fixtures/holidays/create.json |
| DELETE | api.clockify.me | /api/v1/workspaces/{ws}/holidays/{id} | 200 | fixtures/holidays/delete.json |

## Request headers (no secrets)
- X-Api-Key: [REDACTED]
- Content-Type: not required for GETs

## Request body (when applicable)
GET only — no request body. Query params for in-period:
```
assigned-to=<userId>   (required: filter by user assignment)
start=2026-01-01T00:00:00Z  (required: ISO 8601 with Z suffix)
end=2027-01-01T00:00:00Z    (required: ISO 8601 with Z suffix)
```
Create body shape (POST, **probed live** — required fields confirmed):
```json
{
  "name": "<string, 2–100 chars>",            // required
  "datePeriod": {                              // required
    "startDate": "2027-09-15",               // required, yyyy-MM-dd
    "endDate": "2027-09-15"                  // required, yyyy-MM-dd
  },
  "users": { "contains": "CONTAINS", "ids": ["<userId>"], "status": "ACTIVE" } // at least one user or userGroup required
}
```
**Live probe result (2026-05-02):** Sending only `name` + `datePeriod` returns `400 {message:"At least one user or user group must be assigned", code:501}`. Adding `users.ids` with self → `200` with response object. See `fixtures/holidays/create.json`.

## Response shape

Both GET `/holidays` and GET `/holidays/in-period` return a **bare array** of identical objects:
```json
[
  {
    "id": "<string>",
    "workspaceId": "<string>",
    "name": "<string>",
    "userIds": ["<string>"],
    "userGroupIds": ["<string>"],
    "datePeriod": {
      "startDate": "2026-12-25",
      "endDate": "2026-12-25"
    },
    "occursAnnually": false,
    "everyoneIncludingNew": true,
    "automaticTimeEntryCreation": false,
    "projectId": null,
    "taskId": null
  }
]
```
`datePeriod.startDate` and `datePeriod.endDate` are `yyyy-MM-dd` strings (not ISO 8601 datetime). Both GET endpoints return a bare array — no wrapper object.

## Cleanup behavior
Live create + immediate delete probed (2026-05-02). Created `mcp-probe-1777753190-448ba8e4-hol`, then deleted it directly. **DELETE returns 200 immediately — no archive step required.** Note: the DELETE response shape differs from GET/POST: it includes `color`, `users`, `userGroups` keys but lacks `projectId`, `taskId`. See `fixtures/holidays/delete.json`.

## Recommended go-clockify change
- File: `internal/tools/tier2_groups_holidays.go`
- Function: `createHoliday`
- Change: Replace the flat `{"name": ..., "date": ..., "recurring": ...}` body with `{"name": ..., "datePeriod": {"startDate": ..., "endDate": ...}, "occursAnnually": ...}` — the field name is `datePeriod` (nested object with `startDate`/`endDate` in `yyyy-MM-dd` format), `occursAnnually` (not `recurring`), and the flat `date` field does not exist at all on this endpoint.

## Test that flips from pinned-error to success-path
- Test: the test for `createHoliday` in `tests/tier2_groups_holidays_test.go`
- Action: Remove the `expectErr` annotation (the 400/422 from the flat-date body). Replace with an assertion that the result map has a non-nil `"id"` field and that `result["datePeriod"]` is a non-nil map. The test fixture or HTTP mock must POST `{"name": ..., "datePeriod": {"startDate": "...", "endDate": "..."}}` to succeed.

## Open questions

1. **`occursAnnually` vs `recurring`.** The existing go-clockify handler sends `"recurring"`. The upstream field name is `occursAnnually` (confirmed in both GET response and live probe). The rename is needed alongside the `datePeriod` restructure.

2. **`everyoneIncludingNew` default behaviour.** All live holidays on the workspace have `everyoneIncludingNew: true`. If the go-clockify handler omits this field on create, verify whether the upstream defaults to `true` or `false` — it matters for test assertions.

3. **`GET /holidays/in-period` in go-clockify.** The probe found this endpoint exists and works (200, same shape as the flat list). If go-clockify exposes a `listHolidaysInPeriod` function that currently hits `GET /holidays` without the `/in-period` suffix or without the required `assigned-to`/`start`/`end` params, it will receive either a wrong set of holidays or a 400. Confirm whether go-clockify has this function and check it routes to the correct path with the required query params.

4. **Resolved: user/group assignment required.** `POST /holidays` with only `name` + `datePeriod` returns `400 code:501 "At least one user or user group must be assigned"`. The go-clockify `createHoliday` body must include either `users.ids` or `userGroups.ids` — omitting both is a validation error, not a server default.
