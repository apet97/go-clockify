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
- `/workspaces/{CLOCKIFY_WORKSPACE_ID}/...` is allowed for the pinned
  workspace.
- Other workspace IDs, absolute URLs, scheme-relative URLs, path traversal,
  `/file/image`, and non-API escape paths are rejected.

Prefer relative paths such as:

```json
{
  "path": "/workspaces/<workspace-id>/clients",
  "query": {
    "page": "1",
    "page-size": "50"
  }
}
```

## Raw Writes

Raw `GET` is always available. Raw write methods require:

```sh
export CLOCKIFY_ENABLE_RAW_WRITES=true
```

By default, raw writes are additionally fenced to documented Clockify endpoints:

```sh
export CLOCKIFY_RAW_WRITE_DOCUMENTED_ONLY=true
```

Set `CLOCKIFY_RAW_WRITE_DOCUMENTED_ONLY=false` only for deliberate endpoint
probes. That relaxes the documented-route allowlist but does not relax the
workspace/path fence.

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
