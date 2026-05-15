# Operator Guides

> **Historical artifact. Not current one-user MCP product documentation.**
> Preserved for platform-era audit/history only. Start current one-user work from `README.md`, `docs/agent-cookbook.md`, `docs/tool-catalog.md`, and `docs/goals/oneuser-tool-coverage.md`.


Choose the guide that matches your deployment model. Each guide names
the registered profile(s) it covers — apply with `clockify-mcp
--profile=<name>` or `MCP_PROFILE=<name>`.

- [Managed Shared Service](shared-service.md) — Multi-tenant
  `streamable_http` service for large organisations. Covers the
  `shared-service` and `prod-postgres` profiles.
- [Self-Hosted / Private](self-hosted.md) — Single-tenant or small
  private instances; `stdio` (local subprocess) or `streamable_http`
  (small-team HTTP). Covers the `local-stdio` and
  `single-tenant-http` profiles. The legacy `http` transport is
  deprecated and rejected by the `single-tenant-http` profile via
  `MCP_HTTP_LEGACY_POLICY=deny`.
- [MCP HTTP Error Codes](error-codes.md) — JSON-RPC error codes used
  by HTTP transport admission failures before JSON-RPC dispatch.
- [Legacy HTTP EOL](../runbooks/legacy-http-eol.md) — deprecated
  POST-only HTTP migration, `Deprecation` / successor `Link` response
  headers, and future `Sunset` handling.

The fifth registered profile, `private-network-grpc`, has no
operator guide here — its concerns (gRPC + mTLS behind a private
perimeter) live in
[`docs/deploy/profile-private-network-grpc.md`](../deploy/profile-private-network-grpc.md).
