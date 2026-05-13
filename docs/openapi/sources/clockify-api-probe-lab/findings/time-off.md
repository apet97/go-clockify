# Finding: time-off

## Endpoint(s) probed
| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /api/v1/workspaces/{ws}/time-off/policies | 200 | fixtures/time-off/policies-list.json |
| GET | api.clockify.me | /api/v1/workspaces/{ws}/time-off/requests | 405 | fixtures/time-off/requests-get-broken.json |
| POST | api.clockify.me | /api/v1/workspaces/{ws}/time-off/requests | 200 | fixtures/time-off/requests-post-search.json |
| GET | api.clockify.me | /api/v1/workspaces/{ws}/time-off/policies/{policyId}/requests | 405 | fixtures/time-off/requests-per-policy.json |

## Request headers (no secrets)
- X-Api-Key: [REDACTED]
- Content-Type: application/json (POST only; omitted on GETs)

## Request body (when applicable)
POST /time-off/requests search body sent:
```json
{"page": 1, "pageSize": 5, "start": "2026-01-01T00:00:00Z", "end": "2027-01-01T00:00:00Z"}
```
Full documented body (from TO.md): `{end, page, pageSize, start, statuses[], userGroups[], users[]}`. All fields optional except that without `start`/`end` the date filter is open. Sending `{}` is also valid per the docs.

## Response shape

### GET /time-off/policies — 200
Bare array of policy objects:
```json
[
  {
    "id": "<string>",
    "workspaceId": "<string>",
    "name": "<string>",
    "allowNegativeBalance": true,
    "negativeBalance": null,
    "allowHalfDay": true,
    "archived": false,
    "timeUnit": "DAYS",
    "userIds": ["<string>"],
    "userGroupIds": [],
    "everyoneIncludingNew": true,
    "approve": {
      "requiresApproval": true,
      "teamManagers": false,
      "specificMembers": false,
      "userIds": []
    },
    "automaticAccrual": null,
    "projectId": null,
    "automaticTimeEntryCreation": {
      "enabled": true,
      "defaultEntities": { "projectId": null, "taskId": null }
    }
  }
]
```

### GET /time-off/requests — 405
```json
{"message": "Request method 'GET' is not supported", "code": 3000}
```

### POST /time-off/requests — 200
Wrapped object:
```json
{
  "count": 1,
  "requests": [
    {
      "id": "<string>",
      "workspaceId": "<string>",
      "policyId": "<string>",
      "policyName": "<string>",
      "timeUnit": "DAYS",
      "userId": "<string>",
      "userName": "<string>",
      "userEmail": "[REDACTED-EMAIL]",
      "userTimeZone": "<string>",
      "timeOffPeriod": {
        "period": {
          "start": "2026-04-21T22:00:00Z",
          "end": "2026-04-24T21:59:00Z"
        },
        "halfDay": false,
        "halfDayPeriod": "NOT_DEFINED",
        "halfDayHours": null
      },
      "note": "<string>",
      "status": {
        "statusType": "PENDING",
        "changedByUserId": null,
        "changedByUserName": "<string>",
        "changedForUserName": "<string>",
        "changedAt": null,
        "note": null
      },
      "balanceDiff": 3.0,
      "createdAt": "2026-04-22T14:45:45.679797762Z",
      "balance": -3.5,
      "requesterUserId": "<string>",
      "requesterUserName": "<string>"
    }
  ]
}
```
`timeOffPeriod.period.start/end` are ISO 8601 with nanosecond precision (`Z`-suffixed). `balance` and `balanceDiff` are floats. `status.statusType` observed as `PENDING`; enum is `PENDING|APPROVED|REJECTED|ALL`. `userEmail` is present in the response and was redacted from the fixture — `probe_redact` does not strip it automatically; a pattern should be added.

### GET /time-off/policies/{policyId}/requests — 405
```json
{"message": "Request method 'GET' is not supported", "code": 3000}
```
This path is POST-only for creating a request, not listing. There is no per-policy list GET; the workspace-level `POST /time-off/requests` with a `statuses`/`users` filter is the only list mechanism.

## Cleanup behavior
Read-only probe — no entities were created. `cleanup-registry/time-off.tsv` was not written. Nothing to clean up.

## Recommended go-clockify change
- File: `internal/tools/tier2_time_off.go`
- Function: `listTimeOffRequests`
- Change: Switch the HTTP verb from GET to POST, set `Content-Type: application/json`, and send a JSON body of at minimum `{}` (empty object, which returns all requests up to `pageSize`). A well-formed call sends `{"page":1,"pageSize":50}` and optionally `start`/`end` date filters. The function must also deserialize the response into `struct{ Count int \`json:"count"\`; Requests []map[string]any \`json:"requests"\` }` and return `.Requests` — the response is wrapped, not a bare array.

## Test that flips from pinned-error to success-path
- Test: the test for `listTimeOffRequests` in `tests/tier2_time_off_test.go`
- Action: Remove the `expectErr` annotation (the 405 from the wrong verb). Replace with an assertion that the result is a slice and, when requests exist, `result[0]["policyId"]` and `result[0]["status"]` are non-nil. The test HTTP client or fixture must use POST, not GET, for this path.

## Open questions

1. **`userEmail` in response not redacted automatically.** The `probe_redact` function in `probes/lib/common.sh` does not strip `userEmail` JSON field values. The fixture was redacted manually. A regex pattern for `"userEmail":"..."` should be added to `probe_redact` alongside the existing `authToken` gap noted in the webhooks finding.

2. **Empty-body POST validity — RESOLVED.** Live probe (2026-05-02): `POST /time-off/requests` with body `{}` returns `200 {count: 2, requests: [...]}`. All body fields are optional. go-clockify can send `{}` or omit body fields freely.

3. **Per-policy request list.** There is no `GET /time-off/policies/{policyId}/requests` list endpoint — it's 405. If go-clockify has a `listTimeOffRequestsByPolicy` function that hits this path, it needs the same GET→POST fix, but pointing at the workspace-level `POST /time-off/requests` with a policy filter — **however** TO.md does not show a `policyId` filter field in the POST body. This may mean per-policy filtering is not directly supported via the list endpoint; the caller would need to filter client-side on the `policyId` field of returned requests.

4. **create request body shape.** TO.md shows the create body as `{note, timeOffPeriod: {halfDayPeriod, isHalfDay, period: {days, end, start}, timeOffHalfDayPeriod}}` at `POST /time-off/policies/{policyId}/requests`. The mutating probe was not run (safety rule: avoid firing email to non-test recipients). If go-clockify's `createTimeOffRequest` also 404s or errors, probe it in a follow-up with a far-future date that won't trigger approvals.
