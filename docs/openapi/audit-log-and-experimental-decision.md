# Audit Log And Experimental API Decision

Original decision date: 2026-05-15. Superseded 2026-05-16, when the maintainer
opted into full Clockify API coverage.

The one-user MCP now loads 156 tools at startup. Two of them close the last
documented OpenAPI coverage gaps: the Audit Log report and the experimental
Entity Changes feed.

## Audit Log — now covered

`clockify_audit_logs_search` wraps `POST /v1/workspaces/{workspaceId}/audit-log`
(operationId `getAuditLogs`). That route lives on the dedicated audit-log host
`https://auditlog-api.api.clockify.me/v1`, reached through `Client.PostAuditLog`
/ `Client.AuditLogBaseURL()`. The tool is a read-only search: it posts an
`AuditLogGetRequestV1` filter (actions, authors, date range, page).

A live probe against the sacrificial workspace showed the endpoint returns a
**bare JSON array** of audit-log entries — not the `PageableV1ListAuditLogDtoV1`
`{response:[...]}` envelope the OpenAPI document describes. As with entity
changes below, the tool's output schema follows the live-verified bare-array
shape, and the handler surfaces a clean `ok:false` recovery envelope if the
upstream ever switches to the documented envelope.

## Entity Changes — now covered, experimental

`clockify_entity_changes_list` covers
`GET /v1/workspaces/{workspaceId}/entities/{created,updated,deleted}` on the
primary host, selecting the feed with a `change_type` enum.

These routes are tagged `Entity changes (Experimental)` upstream, and the
tool's description says so. The OpenAPI document originally described the
response as a generic string (created/updated) or a `PageableCollection...`
envelope (deleted); live probing against the sacrificial workspace proved all
three endpoints return a **bare JSON array** of change documents. The tool's
output schema follows that live-verified shape, not the stale documented one —
schema-as-contract means the true schema. If Clockify ever switches to the
documented envelope, the handler surfaces a clean `ok:false` recovery envelope
rather than silently mis-decoding.

## Raw Fallback Boundary

`clockify_api_get` and `clockify_api_request` remain pinned-workspace escape
hatches for the primary API host only. They are intentionally not a blanket
multi-host router; audit-log and entity-changes traffic now has dedicated typed
tools instead.
