# Finding: custom-fields

## Endpoint(s) probed
| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /api/v1/workspaces/{ws}/custom-fields | 200 | fixtures/custom-fields/list.json / list-sample.json |
| POST | api.clockify.me | /api/v1/workspaces/{ws}/custom-fields (TXT, cap hit) | 400 | fixtures/custom-fields/create-cap-error.json |
| POST | api.clockify.me | /api/v1/workspaces/{ws}/custom-fields (TEXT wrong type) | 400 | fixtures/custom-fields/create-text-wrong.json |

## Request headers (no secrets)
- X-Api-Key: [REDACTED]
- Content-Type: not required for GETs

## Request body (when applicable)
GET only — no request body. Query params used:
```
page-size=200
```
Create body shape (POST, not probed live — workspace was at 50-field capacity):
```json
{
  "name": "<string>",           // required
  "type": "<enum>",             // required; see enum below
  "entityType": "TIMEENTRY",    // optional; enum: TIMEENTRY | USER
  "allowedValues": ["a", "b"],  // required when type=DROPDOWN_SINGLE or DROPDOWN_MULTIPLE
  "description": "<string>",    // optional
  "placeholder": "<string>",    // optional
  "status": "VISIBLE",          // optional; enum: INACTIVE | VISIBLE | INVISIBLE
  "required": false,            // optional
  "onlyAdminCanEdit": false,    // optional
  "workspaceDefaultValue": null // optional; type depends on field type
}
```

## Response shape

`GET /workspaces/{ws}/custom-fields` returns a **bare array** — one representative object per `type` (6 types confirmed live):

```json
[
  {
    "id": "<string>",
    "workspaceId": "<string>",
    "name": "<string>",
    "type": "TXT",
    "entityType": "TIMEENTRY",
    "status": "INVISIBLE",
    "allowedValues": [],
    "required": false,
    "onlyAdminCanEdit": true,
    "placeholder": "<string>",
    "description": "<string>",
    "projectDefaultValues": [],
    "workspaceDefaultValue": null
  },
  {
    "id": "<string>",
    "workspaceId": "<string>",
    "name": "<string>",
    "type": "DROPDOWN_SINGLE",
    "entityType": "TIMEENTRY",
    "status": "INACTIVE",
    "allowedValues": ["<string>", "<string>"],
    "required": false,
    "onlyAdminCanEdit": false,
    "placeholder": null,
    "description": null,
    "projectDefaultValues": [],
    "workspaceDefaultValue": null
  }
]
```

**Confirmed live `type` enum (all 6 values observed in workspace):**
| Enum value | Observed live | Notes |
|---|---|---|
| `TXT` | ✓ | Plain text — NOT `TEXT` |
| `NUMBER` | ✓ | Numeric value |
| `DROPDOWN_SINGLE` | ✓ | Single-select — NOT `DROPDOWN` |
| `DROPDOWN_MULTIPLE` | ✓ | Multi-select — NOT `DROPDOWN` |
| `CHECKBOX` | ✓ | Boolean |
| `LINK` | ✓ | URL |

`TEXT` and `DROPDOWN` do not exist in the upstream — they are wrong enum values.

## Cleanup behavior
Attempted to create a TXT field but workspace was at the 50-field cap — `400 code:4033 "You reached the limit of 50 custom fields"`. No entities created, nothing to clean up. Also confirmed: `POST` with `type:"TEXT"` returns `400 code:3002` with message listing valid enum values explicitly. Both error fixtures are saved.

## Recommended go-clockify change
- File: `internal/tools/tier2_custom_fields.go`
- Function: `createCustomField` (and any other function that documents or accepts `type`)
- Change: Update the descriptor docstring and any const/enum definition from the incorrect set `{TEXT, DROPDOWN, …}` to the correct upstream enum `{TXT, NUMBER, DROPDOWN_SINGLE, DROPDOWN_MULTIPLE, CHECKBOX, LINK}` — these are case-sensitive string literals; the upstream rejects `TEXT` and `DROPDOWN` with an explicit accepted-values error.

## Test that flips from pinned-error to success-path
- Test: the test for `createCustomField` in `tests/tier2_custom_fields_test.go`
- Action: Remove the `expectErr` annotation (the rejection from sending `type: "TEXT"`). Replace with an assertion that the result map has non-nil `"id"` and `"type"`. The test HTTP mock body must pass `"type": "TXT"` (not `"TEXT"`) to succeed. If the test exercises `DROPDOWN`, change it to either `DROPDOWN_SINGLE` or `DROPDOWN_MULTIPLE` with an `allowedValues` array.

## Open questions

1. **`allowedValues` required for DROPDOWN types?** CFS.md does not explicitly mark `allowedValues` as required when `type=DROPDOWN_SINGLE` or `DROPDOWN_MULTIPLE`, but all observed instances have non-empty `allowedValues` for these types. Could not probe live (workspace at 50-field cap). All live DROPDOWN fields have non-empty `allowedValues` — treat as implicitly required in go-clockify handler validation.

2. **Project-level custom-field endpoints.** CFS.md documents `GET /projects/{id}/custom-fields`, `DELETE /projects/{id}/custom-fields/{cfId}`, and `PATCH /projects/{id}/custom-fields/{cfId}`. If go-clockify has functions for these, they use different paths and verbs than the workspace-level endpoints. These were not probed — the campaign bug was scoped to the workspace descriptor docstring only.

3. **`entityType` filter on GET.** The `entity-type` query param accepts multiple values (`TIMEENTRY`, `USER`). If go-clockify's list function always queries all types, this may return more fields than expected on workspaces that mix `USER` and `TIMEENTRY` custom fields.

4. **50-field workspace cap.** The probe workspace has reached the 50-field cap. Any future test that tries to create a custom field in this workspace will fail until one is deleted first. The `/cleanup-domain custom-fields` command will not help (no fields were created by this probe). The maintainer should free space manually or use a fresh workspace for create-path integration tests.

## Live read-side promotions (2026-06-20)

Captured HTTP 200 live this session against the sandbox; clean canonical
path (no query string) so the generator's `normalize_path` matches the
merged operation key and `status_bucket` flips the op to `live-success`.
The existing 200 row above is the workspace-level `/custom-fields` list (a
different operation); this is the per-project custom-fields list. Fixtures
are documentary + gitignored.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /workspaces/{workspaceId}/projects/{projectId}/custom-fields | 200 | fixtures/live-shape/project-custom-fields.json |
