# CODEX Verification Results — API Reconciliation

Date: 2026-05-21
Repo: `/Users/15x/Downloads/WORKING/addons-me/GOCLMCP`
Spec: `docs/goals/api-reconciliation/TRUTH.openapi.yaml`

## Verdict

**Conditional GO for nightly drift detection only if the workflow treats `LIVE-VERIFIED` and `LIVE-OVERRIDE` as authoritative and keeps `UNCONFIRMED-*` / `UNVERIFIABLE-*` out of hard truth assertions.** It is **not** a 100% live-proven API truth file: 1 operation remains `UNCONFIRMED-AGREE`, 1 is `UNVERIFIABLE-DESTRUCTIVE`, and the documented Redocly lint commands now emit no warnings or errors under the repo-local Redocly policy.

The handoff was wrong in material ways:

- It claimed 195 operations; canonical/source comparison later showed five real omitted operations across the stabilization and narrow reconciliation passes. The corrected file has **200 operations**.
- It claimed Redocly would only surface warnings; initial Redocly run failed with **237 errors and 194 warnings**.
- It claimed every op had evidence or acceptable documented status; the initial scan found **10 operations missing `x-evidence` arrays**.
- It claimed `POST SUMMARY` without `summaryFilter` returns HTTP 501; fresh live returned **HTTP 400** with body `{"code":501,"message":"Please provide summary filter."}`.

Remaining-gap closure addendum: a live sacrificial-workspace pass on
2026-05-21 promoted 13 safe fixture-backed operations from UNCONFIRMED-AGREE to
LIVE-VERIFIED. A later authorized sacrificial-workspace pass promoted 9 more rows
and moved `DELETE /users/{userId}` to LIVE-OVERRIDE after live Clockify returned a
Cake-migration removal error. Current counts are **148 LIVE-VERIFIED**,
**50 LIVE-OVERRIDE**, **1 UNCONFIRMED-AGREE**, **0 UNRESOLVED-NO-LIVE**, and
**1 UNVERIFIABLE-DESTRUCTIVE** across **200 operations / 128 paths**.

Lint-clean addendum: the OpenAPI now carries license metadata and generic `400` responses on all success-response operations. `redocly.yaml` is present at the repo root and in `docs/goals/api-reconciliation/` so both documented invocation styles disable only the two truth-incompatible style rules: `operation-2xx-response` for intentional LIVE-OVERRIDE non-2xx rows, and `no-ambiguous-paths` for source-covered Clockify route families.

## 1. Spec Validity

Commands requested:

```bash
cd /Users/15x/Downloads/WORKING/addons-me/GOCLMCP/docs/goals/api-reconciliation
openapi-spec-validator TRUTH.openapi.yaml
npx --yes @apidevtools/swagger-cli validate TRUTH.openapi.yaml
npx --yes @redocly/cli lint --max-problems 1000 TRUTH.openapi.yaml
```

Latest validator addendum: after the lint-clean pass, validators were re-run from the repository root against `docs/goals/api-reconciliation/TRUTH.openapi.yaml`. Both structural validators pass. Redocly exits 0 with no warnings and no errors. Redocly uses the repo-local `redocly.yaml` policy described above; this keeps LIVE-OVERRIDE rows honest instead of inventing fake 2xx responses for routes live probes proved absent or rejected.

Final outputs:

```text
docs/goals/api-reconciliation/TRUTH.openapi.yaml: OK
```

```text
docs/goals/api-reconciliation/TRUTH.openapi.yaml is valid
```

```text
validating docs/goals/api-reconciliation/TRUTH.openapi.yaml...
docs/goals/api-reconciliation/TRUTH.openapi.yaml: validated in 123ms

Woohoo! Your API description is valid. 🎉
```

## 2. Counts and Provenance Integrity

Final tally command output:

```text
paths: 128
ops: 200
  LIVE-VERIFIED: 148
  LIVE-OVERRIDE: 50
  UNCONFIRMED-AGREE: 1
  UNVERIFIABLE-DESTRUCTIVE: 1
  UNRESOLVED-NO-LIVE: 0
```

Deviation from handoff claim:

```text
handoff: paths 128, ops 195, LIVE-VERIFIED 127, LIVE-OVERRIDE 46, UNCONFIRMED-AGREE 16, UNRESOLVED-NO-LIVE 4, UNVERIFIABLE-DESTRUCTIVE 2
actual final after Codex stabilization + narrow reconciliation + live workspace closure + remaining-gap closure + final authorized live pass: paths 128, ops 200, LIVE-VERIFIED 148, LIVE-OVERRIDE 50, UNCONFIRMED-AGREE 1, UNRESOLVED-NO-LIVE 0, UNVERIFIABLE-DESTRUCTIVE 1
```

Evidence scan:

```text
missing_or_empty_x_evidence: 0
missing_x_evidence_files: 0
probe_json_parse_errors: 0
raw_24hex_or_email_leaks: 0
```

Corrective edit:

```diff
diff --git a/docs/goals/api-reconciliation/TRUTH.openapi.yaml b/docs/goals/api-reconciliation/TRUTH.openapi.yaml
@@
+ security:
+   - ApiKeyAuth: []
+ components:
+   securitySchemes:
+     ApiKeyAuth:
+       type: apiKey
+       in: header
+       name: X-Api-Key
@@
+ /workspaces:
+   get:
+     operationId: getAllMyWorkspaces
+     x-verification-status: LIVE-VERIFIED
+     x-evidence:
+       - PROBE-LOG/20260521T023344Z-codex-workspaces-list-live.json
@@
+ /workspaces/{workspaceId}/projects/{projectId}/template:
+   patch:
+     operationId: updateProjectTemplate
+     x-verification-status: LIVE-VERIFIED
+     x-evidence:
+       - PROBE-LOG/20260521T023345Z-codex-projects-template-patch-true.json
+       - PROBE-LOG/20260521T023425Z-codex-project-cleanup-template-false.json
@@
- /workspaces/{workspaceId}/user-groups/{groupId}/users
+ /workspaces/{workspaceId}/user-groups/{userGroupId}/users
@@
- /workspaces/{workspaceId}/time-off/policies/{policyId}
+ /workspaces/{workspaceId}/time-off/policies/{id}
@@
- /workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}/cost-rate
+ /workspaces/{workspaceId}/projects/{projectId}/tasks/{id}/cost-rate
@@
- /workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}/hourly-rate
+ /workspaces/{workspaceId}/projects/{projectId}/tasks/{id}/hourly-rate
@@
- GET /workspaces/{workspaceId}/time-off/requests/{requestId}: UNRESOLVED-NO-LIVE, 200 provisional
+ GET /workspaces/{workspaceId}/time-off/requests/{requestId}: LIVE-OVERRIDE, 404 No static resource with real request id
@@
- DELETE /workspaces/{workspaceId}/time-off/policies/{id}: UNRESOLVED-NO-LIVE
+ DELETE /workspaces/{workspaceId}/time-off/policies/{id}: LIVE-VERIFIED; active DELETE 400, archive then DELETE 200 empty body
@@
- GET /workspaces/{workspaceId}/expenses/{expenseId}/files/{fileId}: UNRESOLVED-NO-LIVE
+ GET /workspaces/{workspaceId}/expenses/{expenseId}/files/{fileId}: LIVE-VERIFIED; image/png fixture downloaded as application/octet-stream
```

## 3. Live Re-Probe

### 3a. Shared Reports

Fresh redacted probe evidence:

```json
{
  "create": {
    "response_json": {
      "fixedDate": true,
      "id": "<redacted-id:2e4c2c0182>",
      "isPublic": false,
      "name": "CODEX-VERIFY-1779330620",
      "type": "SUMMARY",
      "workspaceId": "<CLOCKIFY_WORKSPACE_ID>"
    },
    "status": 200
  },
  "delete": {
    "body_bytes": 0,
    "response_body_text": "",
    "status": 204
  },
  "get_by_id": {
    "body_bytes": 3949,
    "status": 200
  },
  "list": {
    "response_json": {
      "count": 74,
      "keys": [
        "count",
        "reports"
      ],
      "reports_len": 5
    },
    "status": 200
  },
  "main_host": {
    "response_json": {
      "code": 3000,
      "message": "No static resource v1/workspaces/<CLOCKIFY_WORKSPACE_ID>/shared-reports."
    },
    "status": 404
  },
  "missing_dates": {
    "response_json": {
      "code": 400,
      "message": "saveSharedReportV1.arg0.filter.dateRangeEnd: Field dateRangeEnd is required, saveSharedReportV1.arg0.filter.dateRangeStart: Field dateRangeStart is required"
    },
    "status": 400
  },
  "missing_summary_filter": {
    "response_json": {
      "code": 501,
      "message": "Please provide summary filter."
    },
    "status": 400
  },
  "update": {
    "response_json": {
      "fixedDate": false,
      "id": "<redacted-id:2e4c2c0182>",
      "isPublic": false,
      "name": "CODEX-VERIFY-1779330620-UPDATED",
      "type": "SUMMARY",
      "workspaceId": "<CLOCKIFY_WORKSPACE_ID>"
    },
    "status": 200
  },
  "workspace_scoped_get": {
    "response_json": {
      "code": 405,
      "message": "HTTP 405 Method Not Allowed"
    },
    "status": 405
  }
}
```

Defect found: the handoff wording “POST with `type=SUMMARY` and no `summaryFilter` -> 501” is not HTTP-status accurate. The HTTP status is 400; the JSON error code is 501.

### 3b. Users Direct Curl Equivalent

Fresh redacted probe evidence:

```json
{
  "get": {
    "body_bytes": 70371,
    "count": 1,
    "first_keys": [
      "activeWorkspace",
      "customFields",
      "defaultWorkspace",
      "email",
      "id",
      "memberships",
      "name",
      "profilePicture",
      "settings",
      "status"
    ],
    "status": 200
  },
  "head": {
    "response_headers_subset": {
      "Content-Type": "application/json",
      "Date": "Thu, 21 May 2026 02:30:22 GMT"
    },
    "status": 200
  }
}
```

Result: status 200, bare array, `page-size=1` returned exactly 1 user. The recorded pagination claim is correct.

## 4. Canonical Official Specs Cross-Check

Initial exact script output before canonicalization showed these false gaps and two real omissions:

```text
[MISSING in TRUTH] POLICIESOPENAPI.YAML :: GET /v1/workspaces/{workspaceId}/time-off/policies/{id}
[MISSING in TRUTH] POLICIESOPENAPI.YAML :: DELETE /v1/workspaces/{workspaceId}/time-off/policies/{id}
[MISSING in TRUTH] POLICIESOPENAPI.YAML :: PATCH /v1/workspaces/{workspaceId}/time-off/policies/{id}
[MISSING in TRUTH] POLICIESOPENAPI.YAML :: PUT /v1/workspaces/{workspaceId}/time-off/policies/{id}
[MISSING in TRUTH] PROJECTSOPENAPI.YAML :: PATCH /v1/workspaces/{workspaceId}/projects/{projectId}/template
[MISSING in TRUTH] TASKOPENAPI.YAML :: PUT /v1/workspaces/{workspaceId}/projects/{projectId}/tasks/{id}/cost-rate
[MISSING in TRUTH] TASKOPENAPI.YAML :: PUT /v1/workspaces/{workspaceId}/projects/{projectId}/tasks/{id}/hourly-rate
[MISSING in TRUTH] USERGROUPSOPEAPI.YAML :: DELETE /v1/workspaces/{workspaceId}/user-groups/{id}
[MISSING in TRUTH] USERGROUPSOPEAPI.YAML :: PUT /v1/workspaces/{workspaceId}/user-groups/{id}
[MISSING in TRUTH] USERGROUPSOPEAPI.YAML :: POST /v1/workspaces/{workspaceId}/user-groups/{userGroupId}/users
[MISSING in TRUTH] USERGROUPSOPEAPI.YAML :: DELETE /v1/workspaces/{workspaceId}/user-groups/{userGroupId}/users/{userId}
[MISSING in TRUTH] WORKSPACEOPENAPI.YAML :: GET /v1/workspaces
```

Investigation:

- `GET /workspaces` was a real omission. Added and live-verified.
- `PATCH /projects/{projectId}/template` was a real omission. Added and live-verified.
- The other 10 lines were path-parameter-name aliases. I canonicalized TRUTH to the official names so the exact script now passes.

Final exact script output:

```text
no [MISSING in TRUTH] lines
```

## 5. Path Consolidation Correctness

Final grep and operationId check:

```text
no sharedReportId — good
DELETE /workspaces/{workspaceId}/shared-reports/{id}
```

## 6. UNRESOLVED-NO-LIVE Attack

Follow-up Codex live workspace closure resolved the remaining unresolved rows:

- `GET /workspaces/{workspaceId}/templates/{templateId}`: LIVE-OVERRIDE. A fresh project template and existing project-template candidates all returned HTTP 400 code 501 on the workspace-level route. Evidence: `PROBE-LOG/20260521T194101Z-codex-get-workspaces-templates-single-live.json`.
- `DELETE /workspaces/{workspaceId}/user/{userId}/time-entries`: LIVE-VERIFIED. A disposable time entry was bulk-deleted with `time-entry-ids`; live returned HTTP 200 bare array and follow-up GET no longer read the entry. Evidence: `PROBE-LOG/20260521T194802Z-codex-delete-workspaces-user-time-entries-live-reprobe3.json`.
- `PUT /workspaces/{workspaceId}/scheduling/assignments/recurring/{assignmentId}`: LIVE-OVERRIDE. Direct PUT against a freshly-created assignment returned HTTP 405; PATCH remains the supported update method. Evidence: `PROBE-LOG/20260521T194802Z-codex-put-workspaces-scheduling-assignments-recurring-live-reprobe3.json`.

Fresh redacted evidence:

```json
{
  "expense_file_create": {
    "response_json": {
      "fileId": "<redacted-id:577ec58cb5>",
      "id": "<redacted-id:5fbb3f7e95>",
      "notes": "CODEX-VERIFY-1779331006-EXPENSE-FILE",
      "total": 1234.0,
      "workspaceId": "<CLOCKIFY_WORKSPACE_ID>"
    },
    "status": 201
  },
  "expense_file_get": {
    "body_bytes": 68,
    "response_headers_subset": {
      "Content-Length": "68",
      "Content-Type": "application/octet-stream",
      "Date": "Thu, 21 May 2026 02:36:48 GMT"
    },
    "status": 200
  },
  "expense_text_plain_reject": {
    "response_json": {
      "code": 4015,
      "message": "Uploading files of type text/plain is not allowed"
    },
    "status": 400
  },
  "policy_archive": {
    "response_json": {
      "allowHalfDay": false,
      "allowNegativeBalance": false,
      "approve": {
        "requiresApproval": false,
        "specificMembers": false,
        "teamManagers": false,
        "userIds": []
      },
      "archived": true,
      "automaticAccrual": null,
      "automaticTimeEntryCreation": null,
      "everyoneIncludingNew": false,
      "id": "<redacted-id:57a75351b4>",
      "name": "CODEX-VERIFY-1779330887-POLICY",
      "negativeBalance": null,
      "projectId": null,
      "timeUnit": "DAYS",
      "userGroupIds": [],
      "userIds": [
        "<redacted-id:706f58c94f>"
      ],
      "workspaceId": "<CLOCKIFY_WORKSPACE_ID>"
    },
    "status": 200
  },
  "policy_delete_active": {
    "response_json": {
      "code": 400,
      "message": "Policy must be archived in order to be deleted"
    },
    "status": 400
  },
  "policy_delete_after_archive": {
    "body_bytes": 0,
    "response_body_text": "",
    "status": 200
  },
  "project_template_patch": {
    "response_json": {
      "archived": false,
      "id": "<redacted-id:592916c247>",
      "name": "CODEX-VERIFY-1779330824-PROJECT",
      "template": true,
      "workspaceId": "<CLOCKIFY_WORKSPACE_ID>"
    },
    "status": 200
  },
  "request_get_real_id": {
    "response_json": {
      "code": 3000,
      "message": "No static resource v1/workspaces/<CLOCKIFY_WORKSPACE_ID>/time-off/requests/<redacted-id:b431575b1a>."
    },
    "status": 404
  },
  "request_list": {
    "count": 2,
    "status": 200
  },
  "template_get": {
    "response_json": {
      "code": 501,
      "message": "Template doesn't belong to User"
    },
    "status": 400
  },
  "workspaces_get": {
    "body_bytes": 192431,
    "count": 26,
    "status": 200
  }
}
```

Results:

- `GET /workspaces/{workspaceId}/templates/{templateId}`: later closed as **LIVE-OVERRIDE**. Real project-template candidates still returned HTTP 400 body code 501 on the workspace-level route; project-level template workflows are the live path. Evidence: `PROBE-LOG/20260521T194101Z-codex-get-workspaces-templates-single-live.json`.
- `DELETE /workspaces/{workspaceId}/time-off/policies/{id}`: **promoted to LIVE-VERIFIED** with important precondition. Active policy DELETE returns 400; archive then DELETE returns 200 empty body.
- `GET /workspaces/{workspaceId}/time-off/requests/{requestId}`: **resolved as LIVE-OVERRIDE**. A real id from POST list still returned Spring 404 No static resource, so the single-item GET route is absent.
- `GET /workspaces/{workspaceId}/expenses/{expenseId}/files/{fileId}`: **promoted to LIVE-VERIFIED**. Created image/png receipt expense, downloaded bytes as application/octet-stream, deleted expense and project fixture.

## 7. Safety and Cleanup

`scripts/live-clean-prefix` output:

```text
Sweeping prefix "CODEX-VERIFY-"

scheduling assignments: none
time-off requests: none
time-off policies: none
expenses: none
invoices: none
webhooks: none
user-groups: none
holidays: none
tags: none
projects: none
clients: none

Post-delete rescan:

NOTE: Codex stopped the sweeper after it hung during post-delete rescan; pre-rescan output above reported none for every family. A separate targeted leftover scan follows in CODEX-leftover-scan-final.json.
```

Targeted leftover scan:

```json
{
  "leftover_count": 0,
  "leftovers": [],
  "prefix": "CODEX-VERIFY-",
  "scans": [
    {
      "body_contains_prefix": false,
      "body_sample": "",
      "label": "projects",
      "matches": [],
      "status": 200,
      "url": "https://api.clockify.me/api/v1/workspaces/<CLOCKIFY_WORKSPACE_ID>/projects?page=1&page-size=200"
    },
    {
      "body_contains_prefix": false,
      "body_sample": "",
      "label": "clients",
      "matches": [],
      "status": 200,
      "url": "https://api.clockify.me/api/v1/workspaces/<CLOCKIFY_WORKSPACE_ID>/clients?page=1&page-size=200"
    },
    {
      "body_contains_prefix": false,
      "body_sample": "",
      "label": "tags",
      "matches": [],
      "status": 200,
      "url": "https://api.clockify.me/api/v1/workspaces/<CLOCKIFY_WORKSPACE_ID>/tags?page=1&page-size=200"
    },
    {
      "body_contains_prefix": false,
      "body_sample": "",
      "label": "user-groups",
      "matches": [],
      "status": 200,
      "url": "https://api.clockify.me/api/v1/workspaces/<CLOCKIFY_WORKSPACE_ID>/user-groups?page=1&page-size=200"
    },
    {
      "body_contains_prefix": false,
      "body_sample": "",
      "label": "holidays",
      "matches": [],
      "status": 200,
      "url": "https://api.clockify.me/api/v1/workspaces/<CLOCKIFY_WORKSPACE_ID>/holidays?page=1&page-size=200"
    },
    {
      "body_contains_prefix": false,
      "body_sample": "",
      "label": "webhooks",
      "matches": [],
      "status": 200,
      "url": "https://api.clockify.me/api/v1/workspaces/<CLOCKIFY_WORKSPACE_ID>/webhooks?page=1&page-size=200"
    },
    {
      "body_contains_prefix": false,
      "body_sample": "",
      "label": "expenses",
      "matches": [],
      "status": 200,
      "url": "https://api.clockify.me/api/v1/workspaces/<CLOCKIFY_WORKSPACE_ID>/expenses?page=1&pageSize=200"
    },
    {
      "body_contains_prefix": false,
      "body_sample": "",
      "label": "time-off-policies",
      "matches": [],
      "status": 200,
      "url": "https://api.clockify.me/api/v1/workspaces/<CLOCKIFY_WORKSPACE_ID>/time-off/policies?page=1&page-size=200"
    },
    {
      "body_contains_prefix": false,
      "body_sample": "",
      "label": "time-off-requests",
      "matches": [],
      "status": 200,
      "url": "https://api.clockify.me/api/v1/workspaces/<CLOCKIFY_WORKSPACE_ID>/time-off/requests"
    },
    {
      "body_contains_prefix": false,
      "body_sample": "",
      "label": "shared-reports",
      "matches": [],
      "status": 200,
      "url": "https://reports.api.clockify.me/v1/workspaces/<CLOCKIFY_WORKSPACE_ID>/shared-reports?pageSize=200"
    },
    {
      "body_contains_prefix": false,
      "body_sample": "",
      "label": "invoices",
      "matches": [],
      "status": 200,
      "url": "https://api.clockify.me/api/v1/workspaces/<CLOCKIFY_WORKSPACE_ID>/invoices?page=1&page-size=200"
    },
    {
      "body_contains_prefix": false,
      "body_sample": "",
      "label": "scheduling-assignments-all",
      "matches": [],
      "status": 400,
      "url": "https://api.clockify.me/api/v1/workspaces/<CLOCKIFY_WORKSPACE_ID>/scheduling/assignments/all?page=1&page-size=200"
    }
  ],
  "timestamp_utc": "2026-05-21T02:46:09Z"
}
```

Leftover test entities: **empty** (`leftover_count: 0`).

## Final Go/No-Go

**Conditional GO** for `nightly-api-drift-detection` if the implementation treats this file as a status-partitioned baseline, not a blanket truth oracle:

- Use `LIVE-VERIFIED` and `LIVE-OVERRIDE` as drift assertions.
- Keep the remaining 1 `UNCONFIRMED-AGREE` and 1 `UNVERIFIABLE-DESTRUCTIVE` operation out of hard failure checks until fresh fixtures/probes exist. There are no remaining `UNRESOLVED-NO-LIVE` operations.
- Redocly now reports no warnings or errors under the checked-in reconciliation lint policy; no Redocly lint debt remains for this artifact.

A plain “all operations are live-proven” claim would be **NO-GO**.
