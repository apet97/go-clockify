# Safe Usage Guide

This guide covers how to safely deploy the Clockify MCP server for LLM agent use.

## Policy Modes

Control tool access based on your trust level. Set via `CLOCKIFY_POLICY`:

| Mode | Read | Write | Delete | Tier 2 | Use Case |
|------|------|-------|--------|--------|----------|
| `read_only` | ✅ | ❌ | ❌ | ❌ | Untrusted agents — observe only |
| `safe_core` | ✅ | allowlist | ❌ | ❌ | Day-to-day time tracking |
| `standard` | ✅ | ✅ | ✅ | on demand | **Default** — balanced |
| `full` | ✅ | ✅ | ✅ | ✅ | Admin and automation |

### safe_core Allowlist

`safe_core` mode allows writes only for these tools:
- `clockify_start_timer`
- `clockify_stop_timer`
- `clockify_add_entry`
- `clockify_update_entry`
- `clockify_log_time`
- `clockify_switch_project`
- `clockify_find_and_update_entry`
- `clockify_create_project`
- `clockify_create_client`
- `clockify_create_tag`
- `clockify_create_task`

### Fine-Grained Overrides

| Variable | Description |
|----------|-------------|
| `CLOCKIFY_DENY_TOOLS` | Comma-separated tool names to block |
| `CLOCKIFY_DENY_GROUPS` | Comma-separated Tier 2 domain groups to block |
| `CLOCKIFY_ALLOW_GROUPS` | Comma-separated allowed groups (overrides mode default) |

### Introspection Tools

These tools are **always available** regardless of policy mode:
- `clockify_whoami`
- `clockify_current_user`
- `clockify_list_workspaces`
- `clockify_search_tools`
- `clockify_policy_info`
- `clockify_resolve_debug`

## Dry-Run

Destructive tools support a `dry_run: true` parameter that previews the operation without making changes.

### Strategies

| Strategy | When | Behavior | Side Effects? |
|----------|------|----------|---------------|
| **Confirm pattern** | `send_invoice`, `approve_timesheet`, etc. | Removes confirm flag and **executes the handler** — the result is wrapped in a dry-run envelope but the operation is performed | **Yes** — the tool runs against the Clockify API |
| **Preview (GET)** | `delete_entry`, `delete_invoice`, etc. | Calls the GET counterpart to show what would be deleted | No — read-only |
| **Minimal fallback** | All other destructive tools | Echoes parameters back, no API call | No |

> **Warning**: The confirm pattern strategy executes the real handler. It exists for tools like `send_invoice` and `approve_timesheet` where removing the confirm flag effectively makes the operation safe (the API requires explicit confirmation). If you need a guaranteed no-side-effect preview, only tools using the **Preview** or **Minimal fallback** strategies provide that.

### Example

```json
{"name": "clockify_delete_entry", "arguments": {"entry_id": "abc123", "dry_run": true}}
```

Response:
```json
{
  "dry_run": true,
  "tool": "clockify_delete_entry",
  "preview": {"id": "abc123", "description": "Meeting", "duration": "PT1H"},
  "note": "This is a dry-run preview. No changes were made."
}
```

### Configuration

Dry-run is enabled by default. To disable:
```sh
CLOCKIFY_DRY_RUN=off
```

## Duplicate Detection

The server checks for duplicate entries before creating new ones.

### How It Works

A duplicate is detected when a proposed entry matches an existing one on all three of:
1. Description (case-sensitive)
2. Project ID (or both empty)
3. Start time (to the minute)

### Configuration

| Variable | Default | Options |
|----------|---------|---------|
| `CLOCKIFY_DEDUPE_MODE` | `warn` | `warn` — include warning in response |
| | | `block` — reject the duplicate entry |
| | | `off` — disable detection |
| `CLOCKIFY_DEDUPE_LOOKBACK` | `25` | Number of recent entries to check |

## Overlap Detection

The server detects overlapping time ranges on the same project.

```sh
CLOCKIFY_OVERLAP_CHECK=true   # default, warns on overlap
CLOCKIFY_OVERLAP_CHECK=off    # disable
```

## Bootstrap vs Policy

**Important distinction:** Bootstrap mode (`CLOCKIFY_BOOTSTRAP_MODE`) controls tool *discovery* — which tools appear in `tools/list`. Policy mode (`CLOCKIFY_POLICY`) controls tool *execution* — which tools can be called.

A tool hidden by bootstrap (e.g., `minimal` mode hides most Tier 1 tools) can still be called if the client knows the tool name. Bootstrap only affects the `tools/list` response.

**To restrict tool access, use policy modes.** Bootstrap is a UX feature for reducing tool clutter, not a security boundary.

> **Note:** `standard` and `full` policy modes are currently functionally identical — both allow all tool operations. `full` is reserved for potential future capabilities (e.g., bypassing dry-run defaults). Use `standard` unless you have a specific reason to use `full`.

## Rate Limiting

Two-layer protection: a concurrency semaphore plus a race-safe fixed-window throughput limiter.

| Control | Variable | Default | Description |
|---------|----------|---------|-------------|
| Concurrency | `CLOCKIFY_MAX_CONCURRENT` | `10` | Max simultaneous tool calls |
| Acquire timeout | `CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT` | `100ms` | Max time to wait for a concurrency slot before rejecting |
| Throughput | `CLOCKIFY_RATE_LIMIT` | `120` | Max calls per fixed 60s window |

Set either to `0` to disable that layer.

The throughput limiter intentionally stays fixed-window. A token bucket would smooth bursts, but it would also change operator-visible semantics by carrying burst credit across windows. A sliding window would be fairer at the boundary, but adds state and complexity without much benefit for this per-process guardrail. Because the server already has a concurrency cap and explicitly honors Clockify `Retry-After`, the deterministic fixed window remains the lowest-risk choice.

Tradeoff: callers can still burst at a window boundary. That is accepted for the current deployment model and is preferable to changing the documented behavior without a clear correctness gain.

Additionally, the server strictly honors Clockify's built-in `Retry-After` headers if the API returns a `429 Too Many Requests` or `503 Service Unavailable`, preventing potential API key bans under high load.

## Token Budget

Large responses are automatically truncated to fit within a token budget:

```sh
CLOCKIFY_TOKEN_BUDGET=8000    # default
CLOCKIFY_TOKEN_BUDGET=0       # disable truncation
```

Progressive truncation stages:
1. Strip null values
2. Strip empty collections
3. Truncate long strings (200 chars)
4. Halve arrays (up to 8 iterations)

Truncation metadata is added when applied.

## Name Resolution

The server resolves human-readable names to Clockify IDs:
- Projects, clients, tags, tasks: exact name match via `strict-name-search=true`
- Users: match by name or email (case-insensitive)
- 24-char hex strings: passed through as IDs

**Fail closed**: Multiple matches are rejected with an actionable error suggesting the list tool.

## Audit Logging

All write-capable tool calls emit structured audit events to stderr:

```
level=INFO msg=audit tool=clockify_add_entry destructive=false req_id=42
```

Set `MCP_LOG_LEVEL=debug` for verbose logging including API request/response details.

## Error Behavior

**Tool errors** return as `result.isError: true` per the MCP spec:
```json
{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"unknown tool: foo"}],"isError":true}}
```

**Protocol errors** (invalid JSON, unknown method, uninitialized server) return as JSON-RPC errors:
```json
{"jsonrpc":"2.0","id":1,"error":{"code":-32002,"message":"server not initialized: send initialize first"}}
```

The server requires an `initialize` handshake before accepting `tools/call` requests.

`clockify_search_tools` remains read-only with respect to Clockify data, but it can activate additional MCP tool descriptors at runtime.

## Recommended Production Setup

```sh
# Minimal safe config for untrusted agents
export CLOCKIFY_API_KEY=your-key
export CLOCKIFY_POLICY=read_only
export MCP_LOG_LEVEL=warn

# Balanced config for trusted agents
export CLOCKIFY_API_KEY=your-key
export CLOCKIFY_POLICY=safe_core
export CLOCKIFY_DEDUPE_MODE=block
export CLOCKIFY_DRY_RUN=enabled

# Full access with legacy HTTP transport
export CLOCKIFY_API_KEY=your-key
export CLOCKIFY_POLICY=standard
export MCP_TRANSPORT=http
export MCP_BEARER_TOKEN=your-secret
export MCP_ALLOWED_ORIGINS=https://your-app.example.com
export MCP_LOG_LEVEL=info
```

## HTTP Transport Security

### No Built-in TLS — Use a Reverse Proxy

The HTTP transport listens in plain HTTP. **Production deployments MUST front
the server with a TLS-terminating reverse proxy** (Caddy, nginx, Envoy,
Traefik, or a cloud load balancer). The bearer token in `Authorization:` and
all request/response bodies travel in the clear otherwise.

See `deploy/Caddyfile.example` for a reference Caddy config with automatic
Let's Encrypt certificates.

### `CLOCKIFY_INSECURE=1` — Scope Clarification

Setting `CLOCKIFY_INSECURE=1` only bypasses **base-URL scheme validation** so
you can point `CLOCKIFY_BASE_URL` at a non-`https://` endpoint on a non-loopback
host (e.g. a test fixture or a local proxy).

It does **NOT**:

- Disable TLS certificate verification in the Go HTTP client. Connecting to an
  `https://` endpoint with a self-signed certificate will still fail with a
  TLS error. For self-signed endpoints, install the CA into the system trust
  store or route through a reverse proxy that terminates and re-originates TLS.
- Relax any other security check (bearer auth, CORS, body size limits, ID
  validation, webhook URL validation).

Only use `CLOCKIFY_INSECURE=1` in development or when you explicitly trust the
link between the server and the Clockify-compatible endpoint.
