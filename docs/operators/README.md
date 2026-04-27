# Operator Guides

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

The fifth registered profile, `private-network-grpc`, has no
operator guide here — its concerns (gRPC + mTLS behind a private
perimeter) live in
[`docs/deploy/profile-private-network-grpc.md`](../deploy/profile-private-network-grpc.md).
