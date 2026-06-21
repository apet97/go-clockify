# Finding: webhooks

## Endpoint(s) probed
| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /workspaces/{ws}/webhooks | 200 | fixtures/webhooks/list-page1.json |
| GET | api.clockify.me | /workspaces/{ws}/webhooks/{id} | 200 | fixtures/webhooks/single-get.json |
| GET | api.clockify.me | /workspaces/{ws}/webhooks/events | 400 | fixtures/webhooks/events-ws.json |
| GET | api.clockify.me | /workspaces/{ws}/webhooks/{id}/events | 404 | fixtures/webhooks/events-perwh.json |

## Request headers (no secrets)
- X-Api-Key: [REDACTED]
- Content-Type: not sent (all GET; no body)

## Request body (when applicable)
n/a — all probes were read-only GETs.

## Response shape

### GET /workspaces/{ws}/webhooks — 200
```json
{
  "workspaceWebhookCount": 10,
  "webhooks": [
    {
      "id": "<string>",
      "userId": "<string>",
      "workspaceId": "<string>",
      "name": "<string>",
      "url": "<string>",
      "authToken": "<string — webhook shared secret for HMAC verification>",
      "webhookEvent": "<string — single event type enum, e.g. NEW_TIMER_STARTED>",
      "triggerSourceType": "<string — e.g. WORKSPACE_ID>",
      "triggerSource": ["<string>"],
      "enabled": true,
      "deliveryEnabled": true,
      "planEnabled": true
    }
  ]
}
```
The top-level wrapper object is confirmed. `webhookEvent` is a **singular string**, not an array — each webhook subscribes to one event type.

### GET /workspaces/{ws}/webhooks/{id} — 200
Flat webhook object; identical shape to one element of the `webhooks` array above. No extra hydration.

### GET /workspaces/{ws}/webhooks/events — 400
```json
{"message": "Webhook doesn't belong to Workspace", "code": 501}
```
The router treats the literal path segment `events` as a webhook `{id}`. There is no workspace-level events-listing endpoint.

### GET /workspaces/{ws}/webhooks/{id}/events — 404
```json
{"message": "No static resource v1/workspaces/{ws}/webhooks/{id}/events.", "code": 3000}
```
The per-webhook events sub-resource does not exist.

## Cleanup behavior
Read-only probe — no entities were created. `cleanup-registry/webhooks.tsv` was not written. Nothing to clean up.

## Bug 1 — list-shape mismatch

### Recommended go-clockify change
- File: `internal/tools/tier2_webhooks.go`
- Function: `listWebhooks`
- Change: Deserialize the response into a wrapper struct
  `struct{ WorkspaceWebhookCount int \`json:"workspaceWebhookCount"\`; Webhooks []map[string]any \`json:"webhooks"\` }`
  and return `.Webhooks`. The current code deserializes directly into `[]map[string]any`, which yields an empty/error result because the top-level value is an object, not an array.

### Test that flips from pinned-error to success-path (Bug 1)
- Test: the test exercising `listWebhooks` in `tests/tier2_webhooks_test.go` (exact name to confirm against the file)
- Action: Remove the `expectErr` annotation (or equivalent inverted assertion). Replace with an assertion that `len(result) >= 1` and that `result[0]["id"]` is a non-empty string and `result[0]["webhookEvent"]` is a non-empty string.

## Bug 2 — listWebhookEvents hits a non-existent endpoint

### Recommended go-clockify change
- File: `internal/tools/tier2_webhooks.go`
- Function: `listWebhookEvents`
- Change: **Remove the live HTTP call entirely.** Neither the workspace-level path (`/workspaces/{ws}/webhooks/events` → 400) nor the per-webhook path (`/workspaces/{ws}/webhooks/{id}/events` → 404) returns a valid response. Clockify does not expose a dynamic events-listing endpoint. Replace the function body with a return of the static enum list sourced from the Clockify documentation (the full list is in `WHDOC.md` in this repo under the `webhookEvent` enum). If the tool is intended to show a user which events they can subscribe to, a hardcoded slice is the correct implementation.

### Test that flips from pinned-error to success-path (Bug 2)
- Test: the test exercising `listWebhookEvents` in `tests/tier2_webhooks_test.go`
- Action: Remove the `expectErr` annotation. Replace with an assertion that the returned list is non-empty and contains at least `"NEW_TIME_ENTRY"` and `"TIMER_STOPPED"` (sanity-check two members of the static list).

## Open questions

1. **`authToken` in list/single-get responses.** The `authToken` field is a per-webhook shared secret (used by the receiving server for HMAC verification). The probe redactor (`probe_redact` in `probes/lib/common.sh`) did not strip it — only the Clockify API key and `Authorization`/`X-Api-Key` header lines are stripped. A pattern for `"authToken":"..."` should be added to `probe_redact` before any future webhook probes that return this field. The fixtures for this probe have been manually redacted in place.

2. **Create body `events` vs `webhookEvent`.** The mutating probe script sends `"events": ["PROJECT_CREATED"]` (plural, array) at create time, but the GET response has `"webhookEvent": "NEW_TIMER_STARTED"` (singular string). It's unclear whether the POST body uses `events` (array) or `webhookEvent` (single string). This was not probed because the read-only fixtures were sufficient for both known bugs. If a go-clockify `createWebhook` bug surfaces later, add a mutating probe for it.

3. **`PROBE_LAST_STATUS` subshell propagation bug.** All four status files showed `unknown` on the first run because `probe_curl` sets `PROBE_LAST_STATUS` in a subshell (`$(...)` or pipe) and the value never reaches the parent. Status codes were retrieved manually and written to the `.status.txt` files. This is a library bug worth fixing — using a temp file to communicate the status code out of the subshell would fix it. Does not affect probe correctness.

4. **`triggerSourceType` / `triggerSource` semantics.** The live data shows `triggerSourceType: "WORKSPACE_ID"` with `triggerSource: [<ws-id>]` for all 10 webhooks. It is unclear whether other `triggerSourceType` values (`PROJECT_ID`, `TAG_ID`, `TASK_ID`, `ASSIGNMENT_ID`, `EXPENSE_ID`) are functionally supported or only documented. Not blocking the known bug fixes.

## Live write-side promotion (2026-06-21)

Captured live this session against the sandbox: a clean create -> update ->
delete cycle on a `sdk-live-probe-wh`-prefixed webhook (POST 201, PUT 200,
DELETE 200), deleted at teardown — **Leftovers:0** (the workspace `webhooks`
list shows zero `sdk-live-probe-wh` residue afterward). This also resolves
open question #2: the create body uses the **singular** `webhookEvent` (not
`events`) — `POST` with `{name, triggerSource:[], triggerSourceType:"WORKSPACE_ID",
url, webhookEvent:"NEW_PROJECT"}` returned 201 (an empty `triggerSource` is
accepted for a workspace-scoped event). Clean canonical paths so each op flips
to `live-success`. Fixtures are documentary.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| POST | api.clockify.me | /workspaces/{workspaceId}/webhooks | 201 | live-probe 2026-06-21 (documentary) |
| PUT | api.clockify.me | /workspaces/{workspaceId}/webhooks/{webhookId} | 200 | live-probe 2026-06-21 (documentary) |
| DELETE | api.clockify.me | /workspaces/{workspaceId}/webhooks/{webhookId} | 200 | live-probe 2026-06-21 (documentary) |
