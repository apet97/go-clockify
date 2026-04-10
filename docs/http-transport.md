# HTTP Transport Guide

## When to Use HTTP

Use HTTP transport for:
- Multi-user deployments
- Centralized hosting
- MCP clients that can speak the documented legacy POST JSON-RPC transport over HTTP
- Server-side deployments behind a reverse proxy

Use stdio (default) for:
- Single-user desktop setups (Claude Desktop, Cursor)
- Local development

## Quick Start

```sh
CLOCKIFY_API_KEY=your-key \
MCP_TRANSPORT=http \
MCP_HTTP_BIND=0.0.0.0:8080 \
MCP_BEARER_TOKEN=your-secret-token \
clockify-mcp
```

## Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `MCP_TRANSPORT` | No | `stdio` | Set to `http` for the legacy POST JSON-RPC transport |
| `MCP_HTTP_BIND` | Yes (http) | `:8080` | Bind address |
| `MCP_BEARER_TOKEN` | Yes (http) | — | Bearer token for auth (`Authorization: Bearer <token>`) |
| `MCP_ALLOWED_ORIGINS` | No | — | Comma-separated allowed browser origins |
| `MCP_ALLOW_ANY_ORIGIN` | No | — | Set `1` to allow all origins |
| `MCP_STRICT_HOST_CHECK` | No | — | Set `1` to require `Host` match `localhost`, `127.0.0.1`, `::1`, or a host from `MCP_ALLOWED_ORIGINS` |
| `MCP_HTTP_MAX_BODY` | No | `2097152` | Positive max request body (bytes, default 2MB) |
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

HTTP mode is a legacy stateless JSON-RPC transport. It does not provide server-initiated streaming, session-backed notification delivery, or per-client initialization semantics. Clients should send `initialize` before the first `tools/call` after process start.

## Current Semantics

Current HTTP behavior is intentionally capability-reduced rather than pretending to implement Streamable HTTP:

- `initialize` state is process-global, not per-client or per-connection.
- Negotiated protocol version and recorded `clientInfo` are process-global.
- Tool activation and visibility changes are process-global.
- Server-initiated notifications are not delivered; attempted `tools/list_changed` notifications are dropped and counted.

This is safe for a shared legacy JSON-RPC endpoint, but it is not a session-aware MCP transport.

## Endpoints

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/mcp` | POST | Bearer | MCP JSON-RPC endpoint |
| `/mcp` | OPTIONS | None | CORS preflight |
| `/health` | GET | None | Health check (always 200) |
| `/ready` | GET | None | Readiness check for the HTTP server and optional upstream Clockify probe; independent of MCP `initialize` state |

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
- **HTTP state is shared across callers.** A later HTTP `initialize` call replaces the process-global negotiated client metadata, and activated tool visibility is shared across all HTTP callers using that server process.

## Migration Path to Streamable HTTP

A correct Streamable HTTP implementation is intentionally out of scope for `MCP_TRANSPORT=http` today. Before introducing it, the MCP core needs session-scoped state for:

- `initialized`
- negotiated protocol version and `clientInfo`
- notifier / server-initiated delivery
- visible and activated tool state
- session lifecycle tests for spec-correct stream transport behavior

When that work lands, it should ship behind an explicit new transport mode instead of silently changing the meaning of `MCP_TRANSPORT=http`.

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
