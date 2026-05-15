# Legacy HTTP EOL

> **Historical artifact. Not current one-user MCP product documentation.**
> Preserved for platform-era audit/history only. Start current one-user work from `README.md`, `docs/agent-cookbook.md`, `docs/tool-catalog.md`, and `docs/goals/oneuser-tool-coverage.md`.


This runbook covers the deprecated POST-only `MCP_TRANSPORT=http`
transport. New HTTP deployments should use `streamable_http` at
`/mcp`.

## Current signal

Every legacy HTTP response carries machine-readable migration hints:

- `Deprecation: true`
- `Link: </mcp>; rel="successor-version"; type="application/json"`

The server does not emit a `Sunset` header yet. Do not add a
placeholder date. A `Sunset` header needs a concrete major-version
removal date from release owners, then matching release notes under the
deprecation policy.

## Operator action

1. Move clients to `MCP_TRANSPORT=streamable_http` and the `/mcp`
   endpoint.
2. Set `MCP_HTTP_LEGACY_POLICY=deny` in hosted and production-like
   profiles so accidental legacy binds fail at startup.
3. Watch startup logs for `legacy_http_transport`; any occurrence means
   a process is still intentionally or accidentally using the legacy
   transport.
4. For external clients, tell them to follow the successor `Link`
   header and to treat `Deprecation: true` as a migration warning.

## Verification

```sh
export MCP_BEARER_TOKEN=dev-bearer-token-123

MCP_TRANSPORT=http MCP_HTTP_LEGACY_POLICY=warn \
  MCP_AUTH_MODE=static_bearer \
  CLOCKIFY_API_KEY=dummy CLOCKIFY_WORKSPACE_ID=dummy \
  clockify-mcp

curl -i http://127.0.0.1:8080/mcp \
  --oauth2-bearer "$MCP_BEARER_TOKEN" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize"}'
```

Expected response headers include `Deprecation: true` and a
`successor-version` link to `/mcp`. If a concrete major-version removal
date is approved later, update this runbook, `docs/release-policy.md`,
and the legacy HTTP response-header test in the same change.
