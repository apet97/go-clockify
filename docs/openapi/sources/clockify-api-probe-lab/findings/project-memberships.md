# Finding: project-memberships

## Endpoint(s) probed
| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /api/v1/workspaces/{ws}/projects | 200 | fixtures/project-memberships/projects-page1.json |
| PUT | api.clockify.me | /api/v1/workspaces/{ws}/projects/{id}/memberships | 405 | fixtures/project-memberships/memberships-PUT-405.json |
| PATCH | api.clockify.me | /api/v1/workspaces/{ws}/projects/{id}/memberships (self only) | 200 | fixtures/project-memberships/patch-response-full-project.json |
| POST | api.clockify.me | /api/v1/workspaces/{ws}/projects/{id}/memberships (add other) | 200 | fixtures/project-memberships/post-add-member.json |
| PATCH | api.clockify.me | /api/v1/workspaces/{ws}/projects/{id}/memberships (replace test) | 200 | fixtures/project-memberships/patch-replace-semantics.json |

## Request headers (no secrets)
- X-Api-Key: [REDACTED]
- Content-Type: application/json (on mutating calls)

## Request body (when applicable)
PUT body sent (to confirm 405, no state change):
```json
{"memberships": []}
```

PATCH body shape (from PROJ.md, not probed live to avoid modifying real project memberships):
```json
{
  "memberships": [
    {
      "userId": "<string>",       // required
      "hourlyRate": {             // optional
        "amount": 0,
        "since": "2020-01-01T00:00:00Z"
      },
      "costRate": {               // optional
        "amount": 0,
        "since": "2020-01-01T00:00:00Z"
      }
    }
  ],
  "userGroups": {                 // optional
    "contains": "CONTAINS",
    "ids": [],
    "status": "ALL"
  }
}
```

POST /memberships body (assign/remove — separate operation, not the campaign bug):
```json
{
  "remove": false,
  "userGroups": {"contains": "CONTAINS", "ids": [], "status": "ALL"},
  "userIds": ["<string>"]
}
```

## Response shape

### GET /workspaces/{ws}/projects (embedded memberships shape)
Each project object contains a `memberships` array:
```json
[
  {
    "id": "<string>",
    "name": "<string>",
    "workspaceId": "<string>",
    "clientId": "<string>",
    "clientName": "<string>",
    "billable": false,
    "public": false,
    "template": false,
    "archived": false,
    "color": "<string>",
    "duration": "<string>",
    "hourlyRate": null,
    "costRate": null,
    "estimate": null,
    "budgetEstimate": null,
    "timeEstimate": null,
    "estimateReset": null,
    "note": "<string>",
    "memberships": [
      {
        "userId": "<string>",
        "targetId": "<string>",
        "membershipType": "PROJECT",
        "membershipStatus": "ACTIVE",
        "hourlyRate": null,
        "costRate": null
      }
    ]
  }
]
```

### PUT /memberships — 405
```json
{"message": "Request method 'PUT' is not supported", "code": 3000}
```

### PATCH /memberships — 200 (live probe 2026-05-02)
Returns the **full project object** (same shape as `GET /projects/{id}`), not a bare membership array. Top-level keys confirmed live:
`id, name, hourlyRate, clientId, workspaceId, billable, memberships, color, estimate, archived, duration, clientName, note, costRate, timeEstimate, budgetEstimate, estimateReset, template, public`

The `memberships` array within this response contains the updated members. See `fixtures/project-memberships/patch-response-full-project.json`.

**REPLACE semantics confirmed (2026-05-02):** PATCH with `{"memberships":[userId_A]}` on a 2-member project `[userId_A, userId_B]` results in exactly 1 member `[userId_A]`. The other user is removed. This is REPLACE, not UPSERT. See `fixtures/project-memberships/patch-replace-semantics.json`.

## Cleanup behavior
Live probe (2026-05-02):
- Created test project `mcp-probe-1777752757-a959eb-proj2` (ID `69f65ae0307374b46320f3b2`). DELETE returned 400 initially. Fixed by archiving first (`PUT /projects/{id}` with `{"archived":true}`) then DELETE → 200. Project cleaned up.
- PATCH probe temporarily removed a second member from the test project. Both PATCH (replace) and POST (add back) were run and the final project state was restored to the original member before deletion.
- No orphaned entities remain.

## Recommended go-clockify change

### Bug 1 — wrong verb
- File: `internal/tools/tier2_project_admin.go`
- Function: `setProjectMemberships`
- Change: Change the HTTP verb from `PUT` to `PATCH`. The path is correct; only the verb is wrong. `PUT` returns 405; `PATCH` returns 200.

### Bug 2 — wrong response deserialization
- File: `internal/tools/tier2_project_admin.go`
- Function: `setProjectMemberships`
- Change: The PATCH response is the full project object, not a bare membership array. Deserialise into a `map[string]any` and return `result["memberships"]` — the array at that key is the updated membership list. Any code that tries to cast the response to `[]map[string]any` directly will fail.

### Semantic note (not a bug, but must be documented)
PATCH /memberships is **REPLACE semantics**: the sent array completely replaces the current membership list. callers must send the full desired member list, not just new additions. To add a single user without removing others, use `POST /memberships` with `{"userIds":[newUser], "remove":false}`.

## Test that flips from pinned-error to success-path
- Test: the test for `setProjectMemberships` in `tests/tier2_project_admin_test.go`
- Action: Remove the `expectErr` annotation (the 405 from PUT). Replace with an assertion that the result is non-nil and, if a project object is returned, `result["id"]` and `result["memberships"]` are non-nil. The test HTTP mock must change the verb from PUT to PATCH for the `/memberships` path.

## Open questions

1. **`POST /memberships` vs `PATCH /memberships` — both confirmed live.** Two operations coexist:
   - `PATCH /memberships` — REPLACE semantics, full desired list in body
   - `POST /memberships` — additive/remove with `{"userIds": [...], "remove": false/true}`
   go-clockify should expose both but document the REPLACE behavior of PATCH prominently.

2. **`userGroups` field in PATCH body.** Optional `userGroups: {contains, ids, status}` field exists per docs but was not probed with group data. The current bug fix (PUT→PATCH + response extraction) does not require this field.

3. **Project DELETE requires archive-first.** `DELETE /projects/{id}` returned 400 on a non-archived project. Archiving via `PUT /projects/{id}` with `{"archived":true}` then DELETE → 200. go-clockify's `deleteProject` function (if it exists) must archive before deleting or document this constraint.
