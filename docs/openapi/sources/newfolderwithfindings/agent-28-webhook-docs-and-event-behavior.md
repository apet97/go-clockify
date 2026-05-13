# QA Agent 28 - webhook-docs-and-event-behavior

## Verdict
PASS WITH CONCERNS

## What I checked

1. **Repo webhook source code** — `internal/tools/tier2_webhooks.go` (7 tools, 779 lines), `tier2_webhooks_test.go` (24 test cases, 533 lines), plus config, runtime, dry-run, risk classification, admin tests, and all supporting infrastructure.

2. **Webhook API documentation** — Live `WEBHOOKDOC.md` (318,510 bytes) in the probe lab, covering all webhook endpoints (`GET /webhooks`, `GET /webhooks/{id}`, `POST /webhooks`, `PUT /webhooks/{id}`, `DELETE /webhooks/{id}`, `POST /webhooks/{id}/test`), response schemas, event type enums, trigger source types, entity types, feature flags, and payload types.

3. **Live API probes** — All read-only and mutating webhook endpoints tested against workspace `65b382b606de527a7ee2b60e`. Verified list shape, CRUD lifecycle, error conditions, and event type enumeration against live API responses.

4. **MCP tool schemas** — Validated parameter names (`webhook_id`, `webhook_event`, `trigger_source_type`, `trigger_source`, `auth_token`, `dry_run`), required fields, enum values (trigger source types), and tool descriptions against live API behavior.

5. **Unit test coverage** — 24 webhook tests, all passing. Covers list unwrap, auth token masking, create/update validation, dry-run, legacy `events` array fallback, DNS validation, allowed domains, SSH credentials rejection, and missing-field error paths.

6. **SSRF protection** — URL validation enforces HTTPS, rejects embedded credentials, rejects private/loopback/link-local/reserved IPs both literal and DNS-resolved, supports operator allowlist escape hatch. TOCTOU risk from DNS rebinding is documented.

7. **Event enum synchronization** — Static `webhookEventEnum` (51 entries) compared byte-for-byte against live API's accepted event types (from error response enumeration). Zero differences. Matches `WEBHOOKDOC.md`.

## Live API probe lab files used

| File | Purpose |
|------|---------|
| `/tmp/clockify-livetest.env` | API key, workspace ID, confirm token |
| `WEBHOOKDOC.md` | Complete webhook API reference (all endpoints, schemas, enums) |
| `probes/webhooks.sh` | Read-only webhook probe script |
| `probes/lib/common.sh` | Shared probe library (curl wrapper, redaction, fixtures) |
| `fixtures/webhooks/list-page1.json` | Captured list response (redacted) |
| `fixtures/webhooks/single-get.json` | Captured single-get response (redacted) |
| `fixtures/webhooks/events-ws.json` | Captured workspace-level events probe (failure) |
| `fixtures/webhooks/events-perwh.json` | Captured per-webhook events probe (failure) |
| `findings/webhooks.md` | Prior findings (list-shape and event-endpoint bugs -- both confirmed fixed) |

## Commands run

```bash
# Build
go build ./...

# Unit tests (24 tests, all webhook-related)
go test ./internal/tools/ -run "Webhook" -v -count=1

# Live probes (read-only)
cd clockify-api-probe-lab && PROBE_MUTATE=0 bash probes/webhooks.sh

# Live probes (mutating CRUD - direct curl)
curl -s -X POST -H "X-Api-Key: <REDACTED>" \
  -d '{"name":"qa-agent-28-xxx","url":"https://example.com/xxx",...}' \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/webhooks"

# Edge case probes (invalid event, missing name, HTTP URL, invalid trigger type)
curl -s -X POST -H "X-Api-Key: <REDACTED>" \
  -d '{"webhookEvent":"INVALID_EVENT_NAME",...}' \
  ".../webhooks"
```

## Live API probes run

### Read-only probes

| Probe | Endpoint | Status | Result |
|-------|----------|--------|--------|
| list-page1 | `GET /workspaces/{ws}/webhooks` | 200 | `{workspaceWebhookCount:7, webhooks:[...]}` -- wrapped shape confirmed |
| single-get | `GET /workspaces/{ws}/webhooks/{id}` | 200 | Flat webhook object with all fields including `authToken` |
| events-ws | `GET /workspaces/{ws}/webhooks/events` | 400 | `{"message":"Webhook doesn't belong to Workspace","code":501}` -- confirmed non-existent |
| events-perwh | `GET /workspaces/{ws}/webhooks/{id}/events` | 404 | `{"code":3000}` -- confirmed non-existent |

### Mutating probes (all on qa-agent-28-* resources)

| Probe | Endpoint | Status | Result |
|-------|----------|--------|--------|
| create | `POST /workspaces/{ws}/webhooks` | 201 | Created successfully, returns full webhook with `authToken` |
| get | `GET /workspaces/{ws}/webhooks/{id}` | 200 | Retrieved created webhook correctly |
| update | `PUT /workspaces/{ws}/webhooks/{id}` | 200 | Updated name and event successfully |
| delete | `DELETE /workspaces/{ws}/webhooks/{id}` | 200 | Deleted successfully |
| verify-deleted | `GET /workspaces/{ws}/webhooks/{id}` | 400 | "Webhook doesn't belong to Workspace" -- deletion confirmed |

### Edge case probes

| Probe | Endpoint | Status | Result |
|-------|----------|--------|--------|
| invalid-event | `POST .../webhooks` (INVALID_EVENT_NAME) | 400 | Proper error listing all 51 valid events |
| missing-trigger | `POST .../webhooks` (without triggerSourceType) | 400 | "triggerSourceType can't be empty" -- API requires it |
| http-url | `POST .../webhooks` (http:// URL) | 201 | **Upstream ACCEPTS HTTP** -- MCP's HTTPS enforcement adds security |
| invalid-trigger | `POST .../webhooks` (invalid triggerSourceType) | 400 | Proper error listing 7 valid values |
| partial-put | `PUT .../webhooks/{id}` (all fields required) | 400 | API validates all fields -- MCP's fetch-merge-PUT pattern is correct |

## Findings

### F1 [PASS] Event enum is perfectly synchronized
The code's static `webhookEventEnum` (51 entries) byte-for-byte matches the live API's accepted event types and `WEBHOOKDOC.md`. The `clockify_list_webhook_events` tool correctly returns the static enum since Clockify has no dynamic events-listing endpoint. Test `TestListWebhookEvents` validates count >= 50 and presence of well-known members.

### F2 [PASS] List response shape correctly handled
The `ListWebhooks` function (line 399-438) correctly deserializes into a wrapper struct `{WorkspaceWebhookCount, Webhooks}` and returns `Webhooks`. This fixes the previously reported Bug 1 from `findings/webhooks.md`.

### F3 [PASS] CRUD lifecycle works end-to-end
Create (201) -> Get (200) -> Update (200) -> Delete (200) -> Verify Delete (400) -- all confirmed working against live API with proper authToken masking.

### F4 [PASS] SSRF protection is comprehensive
URL validation enforces HTTPS, rejects embedded credentials, blocks private/loopback/link-local/reserved IPs (both literal and DNS-resolved), supports operator allowlist (`CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS`), and documents residual DNS rebinding TOCTOU risk. Test coverage includes 19 DNS validation test cases.

### F5 [PASS] All 24 webhook unit tests pass
Including: list unwrap, auth token masking, create/update validation, dry-run avoidance, legacy events array fallback, DNS validation, allowed domains, credentials rejection, missing-field errors.

### F6 [PASS] Required fields defaulting is correct
`triggerSourceType` defaults to `WORKSPACE_ID` and `triggerSource` defaults to `[workspaceId]` when omitted. API requires these fields -- confirmed by live probe showing 400 when omitted.

### F7 [CONCERN] Upstream Clockify accepts HTTP URLs
The live API creates webhooks with `http://` URLs (status 201). The MCP server correctly rejects HTTP URLs in `validateWebhookURL` (HTTPS-only enforcement). This is a security-positive divergence -- the MCP is stricter than upstream. No action needed.

### F8 [CONCERN] authToken masking could leak in error paths
`maskWebhookAuthToken` is called on success-path responses (List, Get, Create, Update). If an upstream error response coincidentally contains an authToken string, it would pass through unmasked. This is low-risk (error responses from Clockify don't contain authTokens), but the masking is best-effort rather than guaranteed.

### F9 [CONCERN] No MCP startup/doctor command found
The repository has `cmd/clockify-mcp` as the main entry point and a Makefile with build targets, but there's no built-in `doctor` or `config` command that reports webhook configuration state. This is a feature gap, not a bug. The `--help` flag shows webhook DNS validation config options.

## Fixes made

### Fix: Name length validation (P2)
**File**: `internal/tools/tier2_webhooks.go`
**Lines**: 501-503 (CreateWebhook), 592-594 (UpdateWebhook)

Added pre-flight validation that webhook name length is 2-30 characters, matching the Clockify API spec (`WEBHOOKDOC.md` line 355: `string [ 2 .. 30 ] characters`). Before this fix, an overly long/short name would produce a cryptic upstream error ("length must be between 2 and 30") instead of a clear MCP-side error.

```go
// CreateWebhook (after empty check)
if len(name) < 2 || len(name) > 30 {
    return ResultEnvelope{}, fmt.Errorf("name must be between 2 and 30 characters (per Clockify API spec)")
}

// UpdateWebhook (inside name change block)
if len(name) < 2 || len(name) > 30 {
    return ResultEnvelope{}, fmt.Errorf("name must be between 2 and 30 characters (per Clockify API spec)")
}
```

Build and all 24 webhook tests pass after this change.

## Reproduction steps for each issue

### F7 (Upstream accepts HTTP)
```bash
curl -s -X POST \
  -H "X-Api-Key: <REDACTED>" -H "Content-Type: application/json" \
  -d '{"name":"test","url":"http://example.com/test","webhookEvent":"NEW_TIME_ENTRY","triggerSourceType":"WORKSPACE_ID","triggerSource":["<REDACTED>"]}' \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/webhooks"
# Returns 201 -- upstream accepts HTTP. MCP rejects this correctly.
```

### F8 (authToken in error paths -- theoretical)
No clean reproduction. Clockify error responses don't carry authTokens. The masking is applied only to success-path data. An upstream error that coincidentally contained a string matching an authToken format would not be masked.

## Cleanup performed

All `qa-agent-28-*` webhook resources created during testing were deleted successfully:

| Resource ID | Name | Status |
|-------------|------|--------|
| `6a00f343284e03fc79325ad6` | qa-agent-28-d8c426-test-wh | Deleted |
| `6a00f455385b9fac085a1197` | qa-agent-28-nonssl (HTTP URL test) | Deleted |
| `6a00f5ffd9647159dc103141` | qa-agent-28-46b42c-puttest | Deleted |

## Leftover test resources

None. All test resources were deleted during the probe session.

## Severity

| Issue | Severity | Rationale |
|-------|----------|-----------|
| Upstream accepts HTTP URLs | **P3** | Security-positive divergence; MCP is stricter. No user impact. |
| authToken not masked in error paths | **P3** | Theoretical; Clockify error responses don't contain authTokens. |
| No doctor/config startup command | **P3** | Feature gap; webhook config is documented in `--help` and runbook. |
| Name length validation missing (FIXED) | **P2** | Pre-flight validation gap; caused cryptic upstream errors. Fixed. |

## Files changed

- `internal/tools/tier2_webhooks.go` -- Added name length validation (2-30 chars) to CreateWebhook and UpdateWebhook
- `go.work.sum` -- Auto-generated dependency checksum updates (no functional change)

## Suggested next action

1. **Add test coverage for name length validation** -- Add test cases like `TestCreateWebhookRejectsNameTooShort` and `TestCreateWebhookRejectsNameTooLong` to `tier2_webhooks_test.go`.

2. **Consider adding a `clockify_doctor` tool** that reports webhook configuration state (DNS validation enabled/disabled, allowed domains, webhook count, etc.) for operator troubleshooting.

3. **Review `authToken` field exposure** -- Audit all code paths that return upstream error responses through the MCP layer to ensure `authToken`-like fields are never leaked in error messages.

## False positives / uncertainty

1. **`findings/webhooks.md` Bug 1 and Bug 2** are both already fixed in the current code. The list response shape is correctly unwrapped, and `listWebhookEvents` uses a static enum. These findings are stale and should be archived.

2. **DNS rebinding TOCTOU** is documented as a known residual risk in `docs/runbooks/webhook-dns-validation.md` section 5. Not a bug -- properly scoped.

3. **Auth token redaction in probe fixtures** -- The fixture files in `clockify-api-probe-lab/fixtures/webhooks/` contain authToken values from pre-existing webhooks (not created by QA probes). These belong to the probe workspace owner. The probe lab's `probe_redact` function strips the API key but does not strip authToken fields -- this is a probe lab hygiene issue, not a go-clockify issue.

## Final recommendation

The webhook subsystem is in good shape. All 7 tools are properly implemented with correct API paths, parameter names, required field validation, auth token masking, SSRF protection, and dry-run support. The event enum is perfectly synchronized with the live API. The one fix applied (name length validation) closes a small pre-flight validation gap. No blocking issues found.
