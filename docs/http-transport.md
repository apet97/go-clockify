# HTTP Transport Guide

## When to Use HTTP

Use HTTP transport for:
- Multi-user deployments
- Centralized hosting
- MCP clients that support Streamable HTTP transport
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
| `MCP_TRANSPORT` | No | `stdio` | Set to `http` |
| `MCP_HTTP_BIND` | Yes (http) | `:8080` | Bind address |
| `MCP_BEARER_TOKEN` | Yes (http) | — | Bearer token for auth |
| `MCP_ALLOWED_ORIGINS` | No | — | Comma-separated allowed browser origins |
| `MCP_ALLOW_ANY_ORIGIN` | No | — | Set `1` to allow all origins |
| `MCP_HTTP_MAX_BODY` | No | `2097152` | Max request body (bytes, default 2MB) |
| `MCP_LOG_LEVEL` | No | `info` | `debug`, `info`, `warn`, `error` |

## Authentication

All requests to `/mcp` require a Bearer token:

```sh
curl -X POST http://localhost:8080/mcp \
  -H "Authorization: Bearer your-secret-token" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

The token is compared using constant-time comparison (`crypto/subtle`) to prevent timing attacks.

## Endpoints

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/mcp` | POST | Bearer | MCP JSON-RPC endpoint |
| `/mcp` | OPTIONS | None | CORS preflight |
| `/health` | GET | None | Health check (always 200) |
| `/ready` | GET | None | Readiness check (503 until initialized) |

## Security Headers

All HTTP responses include:

| Header | Value | Purpose |
|--------|-------|---------|
| `X-Content-Type-Options` | `nosniff` | Prevent content-type sniffing |
| `Cache-Control` | `no-store` | Prevent caching of MCP responses |
| `Content-Type` | `application/json` | All responses (including errors) are JSON |

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

Preflight `OPTIONS` requests do not require the bearer token.

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
- `CLOCKIFY_MAX_CONCURRENT=10` — max simultaneous tool calls
- `CLOCKIFY_RATE_LIMIT=120` — max calls per 60s window

These protect both the MCP server and the upstream Clockify API.
