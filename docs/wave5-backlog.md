# Wave 5 backlog

**Release:** v1.0.0 (2026-04-12)
**Previous:** v0.9.0 (Wave 4)

## Landed

| Track | Commit | Description |
|-------|--------|-------------|
| W5-02a | `7498570` | Native gRPC health protocol (grpc.health.v1.Health/Check) |
| W5-02b | `a8ff86f` | clockify_mcp_grpc_auth_rejections_total counter |
| W5-02c | `c266466` | Multi-stream notifier fan-out (AddNotifier + hub) |
| W5-03 | `4e19004` | Full Helm/Kustomize env-var parity (22 gaps closed) + CI gate |
| W5-04 | `57ee6da` | Delta-sync: delete weekly fan-out, cross-week moves, project/user write-through |
| W5-04d | `2ed30aa` | RFC 6902 JSON Patch alternative delta format |
| W5-05a | `241572e` | Per-interval gRPC auth re-validation |
| W5-05b | `86bdbe1` | forward_auth on gRPC via metadata passthrough |
| W5-05c | `d53186c` | mTLS on gRPC via credentials.TLSInfo |

## Deferred to Wave 6

- Tier 2 group mutations (user groups) not yet wired to delta-sync (no URI template).
- gRPC `Watch` RPC for health protocol (only `Check` implemented; Watch not needed by K8s).
- OIDC token refresh on gRPC (current reauth re-validates but doesn't refresh expired tokens).
- Comprehensive bufconn mTLS test with ephemeral cert generation (implementation landed without a dedicated mTLS test; the auth interceptor test covers the general path).
