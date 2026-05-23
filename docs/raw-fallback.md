# Raw API Fallback

Use raw fallback only after checking for a workflow or domain tool. The two raw
tools are deliberately last in the startup registry:

- `clockify_api_get` for raw `GET` requests.
- `clockify_api_request` for raw `POST`, `PUT`, `PATCH`, and `DELETE`.

Raw calls still run through the configured Clockify client. The API key and
workspace ID are never passed as tool arguments.

## Path Fence

Raw fallback is scoped to safe Clockify paths:

- `/user` is allowed for caller identity checks.
- `/workspaces/{workspaceId}/...` is allowed for the pinned
  workspace.
- Other workspace IDs, absolute URLs, scheme-relative URLs, path traversal,
  `/file/image`, and non-API escape paths are rejected.

Prefer relative paths such as:

```json
{
  "path": "/workspaces/{workspaceId}/clients",
  "query": {
    "page": "1",
    "page-size": "50"
  }
}
```

## Raw Enablement

Raw fallback is disabled and unadvertised by default. It becomes available when
`CLOCKIFY_TOOLSET=all`, or when explicitly enabled:

```sh
export CLOCKIFY_ENABLE_RAW_TOOLS=true
```

Raw `GET` has a separate read gate:

```sh
export CLOCKIFY_ENABLE_RAW_GET=true
```

Sensitive workspace reads such as invoices, users, audit logs, approvals, time
off, balances, webhooks, and workspace settings are rejected unless the selected
toolset is `admin` or `all`. Use the typed tools first.

## Raw Writes

Raw write methods require raw tools plus the write gate:

```sh
export CLOCKIFY_ENABLE_RAW_TOOLS=true
export CLOCKIFY_ENABLE_RAW_WRITES=true
```

By default, raw writes are additionally fenced to documented Clockify endpoints:

```sh
export CLOCKIFY_RAW_WRITE_DOCUMENTED_ONLY=true
```

Set `CLOCKIFY_RAW_WRITE_DOCUMENTED_ONLY=false` only for deliberate endpoint
probes. That relaxes the documented-route allowlist but does not relax the
workspace/path fence.

Documented global write routes outside the pinned workspace scope, such as
`POST /file/image` and `POST /workspaces`, are deliberately excluded from the
raw write allowlist. Add a typed tool if one of those operations becomes part of
the product.

Every raw `POST`, `PUT`, `PATCH`, and `DELETE` is high risk. Run the same call
with `dry_run:true` first; the server returns a preview with method, safe path,
query hash, body hash, documented-route status, and a short-lived
`confirm_token`. The live retry must omit `dry_run`, keep the same method/path/
query/body, and include that `confirm_token`. A changed payload, expired token,
or reused token returns `ok:false` before any upstream request.

## DELETE Bodies

Raw `DELETE` now preserves Clockify response bodies. If Clockify returns JSON,
the raw result includes that JSON. If the upstream response is empty, the MCP
returns a small deletion envelope:

```json
{
  "deleted": true
}
```

## Recovery

If a raw call fails because the endpoint is unsupported, undocumented, denied,
or outside the pinned workspace, prefer adding or using a typed tool. Raw
fallback is an escape hatch, not a second product surface.
