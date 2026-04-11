# HTTP Transport Guide

## When to Use HTTP

Use legacy `MCP_TRANSPORT=http` for:
- Backward-compatible single-tenant HTTP deployments
- Existing clients that only speak the documented POST JSON-RPC transport
- Compatibility migrations where shared session isolation is not required

Use `MCP_TRANSPORT=streamable_http` for:
- Multi-user/shared-service deployments
- Session-aware MCP over HTTP
- Per-session tool activation and notification delivery
- Enterprise deployments behind an ingress, gateway, or service mesh

Use stdio (default) for:
- Single-user desktop setups (Claude Desktop, Cursor)
- Local development

## Quick Start

Legacy HTTP:

```sh
CLOCKIFY_API_KEY=your-key \
MCP_TRANSPORT=http \
MCP_HTTP_BIND=0.0.0.0:8080 \
MCP_BEARER_TOKEN=your-secret-token \
clockify-mcp
```

Session-aware shared service:

```sh
MCP_TRANSPORT=streamable_http \
MCP_AUTH_MODE=oidc \
MCP_OIDC_ISSUER=https://issuer.example.com \
MCP_CONTROL_PLANE_DSN=file:///var/lib/clockify-mcp/control-plane.json \
MCP_METRICS_BIND=127.0.0.1:9091 \
clockify-mcp
```

## Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `MCP_TRANSPORT` | No | `stdio` | `stdio`, compatibility `http`, or session-aware `streamable_http` |
| `MCP_AUTH_MODE` | No | transport-dependent | `static_bearer`, `oidc`, `forward_auth`, `mtls` |
| `MCP_HTTP_BIND` | Yes (http) | `:8080` | Bind address |
| `MCP_BEARER_TOKEN` | Yes (`static_bearer`) | — | Bearer token for auth (`Authorization: Bearer <token>`) |
| `MCP_ALLOWED_ORIGINS` | No | — | Comma-separated allowed browser origins |
| `MCP_ALLOW_ANY_ORIGIN` | No | — | Set `1` to allow all origins |
| `MCP_STRICT_HOST_CHECK` | No | — | Set `1` to require `Host` match `localhost`, `127.0.0.1`, `::1`, or a host from `MCP_ALLOWED_ORIGINS` |
| `MCP_HTTP_MAX_BODY` | No | `2097152` | Positive max request body (bytes, default 2MB) |
| `MCP_CONTROL_PLANE_DSN` | Yes (`streamable_http`) | `memory` | Control-plane store for tenants, credentials, sessions, and audit events |
| `MCP_SESSION_TTL` | No | `30m` | Session expiry for `streamable_http` |
| `MCP_TENANT_CLAIM` | No | `tenant_id` | Tenant claim for OIDC |
| `MCP_SUBJECT_CLAIM` | No | `sub` | Subject claim for OIDC |
| `MCP_METRICS_BIND` | No | — | Dedicated metrics listener; recommended for `streamable_http` |
| `MCP_LOG_LEVEL` | No | `info` | `debug`, `info`, `warn`, `error` |

## Authentication

All requests to `/mcp` require a Bearer token:

```sh
curl -X POST http://localhost:8080/mcp \
  -H "Authorization: Bearer your-secret-token" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"curl","version":"0"}}}'
```

The token is compared using constant-time comparison (`crypto/subtle`) to prevent timing attacks.

Legacy HTTP mode is a compatibility JSON-RPC transport. It does not provide server-initiated streaming, session-backed notification delivery, or per-client initialization semantics. Clients should send `initialize` before the first `tools/call` after process start.

## Current Semantics

Current HTTP behavior is intentionally capability-reduced rather than pretending to implement Streamable HTTP:

- `initialize` state is process-global, not per-client or per-connection.
- Negotiated protocol version and recorded `clientInfo` are process-global.
- Tool activation and visibility changes are process-global.
- Server-initiated notifications are not delivered; attempted `tools/list_changed` notifications are dropped and counted.

This is acceptable for single-tenant compatibility deployments, but it is not a session-aware MCP transport and should not be treated as enterprise shared-service safe.

## Streamable HTTP Semantics

`MCP_TRANSPORT=streamable_http` creates an authenticated per-client session:

- `initialize` creates a session-bound MCP server instance and returns `X-MCP-Session-ID`.
- Subsequent `POST /mcp` requests must send `X-MCP-Session-ID`.
- `GET /mcp/events` streams server notifications for that same session.
- Negotiated protocol version, `clientInfo`, tool activation, notifier delivery, and audit correlation are all session-local.
- Sessions expire after `MCP_SESSION_TTL` unless refreshed by activity.

## Endpoints

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/mcp` | POST | Bearer | MCP JSON-RPC endpoint |
| `/mcp` | OPTIONS | None | CORS preflight |
| `/health` | GET | None | Health check (always 200) |
| `/ready` | GET | None | Readiness check for the HTTP server and optional upstream Clockify probe; independent of MCP `initialize` state |
| `/mcp/events` | GET | Transport auth + session | Server-initiated notifications for `streamable_http` |

## Security Headers

HTTP responses include:

| Header | Value | Purpose |
|--------|-------|---------|
| `X-Content-Type-Options` | `nosniff` | Prevent content-type sniffing |
| `Cache-Control` | `no-store` | Prevent caching of MCP responses |
| `Content-Type` | `application/json` | JSON endpoints and JSON error responses |

## Server Timeouts

The HTTP server enforces timeouts to prevent resource exhaustion:

| Timeout | Value | Purpose |
|---------|-------|---------|
| `ReadHeaderTimeout` | 10s | Max time to read request headers |
| `ReadTimeout` | 30s | Max time to read entire request |
| `WriteTimeout` | 60s | Max time to write response |
| `IdleTimeout` | 120s | Max time to keep idle connections |

## CORS

By default, cross-origin requests are **rejected** when `MCP_ALLOWED_ORIGINS` is not set.

To allow specific origins:
```sh
MCP_ALLOWED_ORIGINS=https://your-app.example.com,https://another.example.com
```

To allow all origins (not recommended for production):
```sh
MCP_ALLOW_ANY_ORIGIN=1
```

To enable strict Host-header validation against DNS rebinding attacks:
```sh
MCP_STRICT_HOST_CHECK=1
```

When strict host checking is enabled, non-loopback hosts must also appear in `MCP_ALLOWED_ORIGINS`. `0.0.0.0` is a bind address, not an allowed Host header.

Preflight `OPTIONS` requests do not require the bearer token.

## Known Limitations

- **Tool list change notifications are not supported in HTTP mode.** `initialize` over HTTP intentionally omits `capabilities.tools.listChanged`, because the legacy POST transport cannot deliver `notifications/tools/list_changed`. When Tier 2 tool groups are activated via `clockify_search_tools`, HTTP clients should re-fetch `tools/list` after activating groups.
- **Legacy HTTP state is shared across callers.** A later HTTP `initialize` call replaces the process-global negotiated client metadata, and activated tool visibility is shared across all HTTP callers using that server process.
- **`streamable_http` session state is replica-local.** Shared-service deployments must use sticky/session-affine routing so `X-MCP-Session-ID` stays on the owning replica.

## Migration Path to Streamable HTTP

Use `MCP_TRANSPORT=streamable_http` for all new shared-service deployments. Keep `MCP_TRANSPORT=http` only for compatibility with existing clients that cannot yet adopt session-aware HTTP.

## Structured Access Logging

Every HTTP request is logged with structured fields:

```
level=INFO msg=http_request method=POST path=/mcp rpc_method=ping status=200 req_id=1 duration_ms=0
```

Fields: `method`, `path`, `rpc_method`, `status`, `req_id` (monotonic), `duration_ms`.

Unauthorized and CORS-blocked requests are logged at `WARN` level.

## TLS with Caddy

For production, use a reverse proxy for TLS termination:

```sh
cd deploy
CLOCKIFY_API_KEY=your-key MCP_BEARER_TOKEN=your-secret docker compose up
```

Edit `deploy/Caddyfile` to set your domain. Caddy automatically provisions TLS certificates via Let's Encrypt.

## Docker

### Build

```sh
docker build -f deploy/Dockerfile -t clockify-mcp .
```

### Run

```sh
docker run -p 8080:8080 \
  -e CLOCKIFY_API_KEY=your-key \
  -e MCP_BEARER_TOKEN=your-secret-token \
  clockify-mcp
```

### Docker Compose

```sh
cd deploy
cp ../examples/docker-compose.env .env
# Edit .env with your values
docker compose up
```

## Graceful Shutdown

The HTTP server handles SIGINT and SIGTERM for graceful shutdown:

1. Signal received → context cancelled
2. New connections refused
3. In-flight requests allowed to finish (10s drain timeout)
4. Server exits cleanly

```
level=INFO msg=http_shutdown reason="context cancelled"
```

## Load Testing

The server includes built-in rate limiting:
- `CLOCKIFY_MAX_CONCURRENT=10` — max simultaneous tool calls (`0` disables this layer)
- `CLOCKIFY_RATE_LIMIT=120` — max calls per fixed 60s window (`0` disables this layer)

These protect both the MCP server and the upstream Clockify API.
