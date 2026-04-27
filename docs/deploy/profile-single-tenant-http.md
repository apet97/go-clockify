# Deployment profile: single-tenant HTTP

> Apply with `clockify-mcp --profile=single-tenant-http` or
> `MCP_PROFILE=single-tenant-http`. Example env file:
> [`deploy/examples/env.single-tenant-http.example`](../../deploy/examples/env.single-tenant-http.example).
> See also: [`internal/config/profile.go`](../../internal/config/profile.go)
> for the pinned defaults, [ADR-0015](../adr/0015-profile-centric-configuration.md)
> for the design rationale.

A deployment where `clockify-mcp` runs as a long-lived HTTP
service behind a TLS-terminating reverse proxy, servicing one
workspace for one team. Identity is a shared bearer token; audit
events are persisted to the local filesystem.

Use this when: a small team needs to share a single Clockify
workspace via a web-based MCP client or a custom dashboard, but
doesn't yet need multi-tenant isolation. Do **not** use this
when: you need per-user audit records, tenant-scoped rate
limits, or HA rollout semantics (go to
`production-profile-shared-service.md` for those).

## Canonical configuration

```env
# Transport: streamable_http is recommended over legacy http for
# server-initiated notifications. streamable_http is the only
# transport that can emit tools/list_changed after Tier 2 group
# activation — legacy http drops all notifications silently.
MCP_TRANSPORT=streamable_http
MCP_HTTP_BIND=127.0.0.1:8080

# Auth: shared bearer token, ≥16 random characters.
MCP_AUTH_MODE=static_bearer
MCP_BEARER_TOKEN=<openssl rand -hex 32>

# Audit: file-backed with FIFO cap eviction. 30-day retention
# is a sensible default for a small-team deployment.
MCP_CONTROL_PLANE_DSN=file:///var/lib/clockify-mcp/audit.db
MCP_CONTROL_PLANE_AUDIT_CAP=100000
MCP_CONTROL_PLANE_AUDIT_RETENTION=720h
MCP_AUDIT_DURABILITY=best_effort

# Observability: metrics on the main listener with inherited
# bearer auth so your reverse proxy's allowlist is the only
# exposure gate.
MCP_HTTP_INLINE_METRICS_ENABLED=1
MCP_HTTP_INLINE_METRICS_AUTH_MODE=inherit_main_bearer

# Safety
CLOCKIFY_POLICY=time_tracking_safe
MCP_STRICT_HOST_CHECK=1
MCP_ALLOWED_ORIGINS=https://your-client.example.com

# Clockify credentials
CLOCKIFY_API_KEY=<your-team-clockify-api-key>
```

## Reverse-proxy snippets

### Caddy

```caddy
mcp.example.com {
  # Restrict origins upstream so MCP_ALLOWED_ORIGINS is a defense
  # in depth, not the only gate.
  @allowed header Origin "https://your-client.example.com"
  handle_path /mcp* {
    reverse_proxy 127.0.0.1:8080 {
      header_up Host {host}
      header_up X-Forwarded-For {remote_host}
      header_up X-Forwarded-Proto https
    }
  }
  # /metrics is deliberately omitted from the external host. Scrape
  # it from inside the private network on :8080/metrics if you
  # want Prometheus data out of this deployment.
}
```

### nginx

```nginx
server {
  listen 443 ssl http2;
  server_name mcp.example.com;

  ssl_certificate     /etc/letsencrypt/live/mcp.example.com/fullchain.pem;
  ssl_certificate_key /etc/letsencrypt/live/mcp.example.com/privkey.pem;

  location /mcp {
    proxy_pass              http://127.0.0.1:8080;
    proxy_http_version      1.1;
    proxy_set_header        Host              $host;
    proxy_set_header        X-Forwarded-For   $remote_addr;
    proxy_set_header        X-Forwarded-Proto https;
    # SSE streams: disable buffering
    proxy_buffering         off;
    proxy_read_timeout      300s;
  }

  # /metrics intentionally not exposed externally.
}
```

## Security model

- Every client presents the same shared bearer. Rotating the
  token invalidates every client simultaneously — plan rotations
  around a maintenance window.
- `MCP_STRICT_HOST_CHECK=1` ensures DNS-rebinding attempts
  from a browser context are rejected before the auth check.
- The audit file lives on local disk; protect it with
  `chmod 600`, back it up, and run the retention reaper
  (automatic on startup).
- `MCP_ALLOWED_ORIGINS` should be pinned to the exact client
  origin. When `MCP_STRICT_HOST_CHECK=1`, the public host
  preserved by your reverse proxy must also be derivable from this
  allowlist; otherwise legitimate requests are rejected before auth.
- `time_tracking_safe` policy is the recommended AI-facing default:
  the agent can start/stop timers and create or update time entries,
  but cannot create workspace objects such as projects, clients,
  tags, or tasks.
- `safe_core` is broader and should be an explicit trusted-assistant
  choice when project/client/tag/task creation is part of the workflow.
  `standard` and `full` are trusted operator/admin modes, not public
  AI defaults.

## Operational checklist

- [ ] TLS certificate automation (Let's Encrypt via Caddy or
      certbot) is set up; no self-signed certs in prod.
- [ ] Proxy rejects `/metrics` at the edge; Prometheus scrapes
      from inside the private network only.
- [ ] `MCP_BEARER_TOKEN` is stored in a secret manager, not the
      systemd unit or the repo.
- [ ] File backend volume has capacity + monitoring against
      `MCP_CONTROL_PLANE_AUDIT_CAP`.
- [ ] Audit-durability runbook is linked in your paging runbook
      (see `docs/runbooks/audit-durability.md`).
- [ ] Clockify API key is scoped to this workspace only.

## Upgrade path

When a second team joins, or you need per-tenant rate limits,
move to `production-profile-shared-service.md`. The migration
swaps:
- `MCP_AUTH_MODE=static_bearer` → `oidc`
- `MCP_CONTROL_PLANE_DSN=file://` → `postgres://`
- `MCP_AUDIT_DURABILITY=best_effort` → `fail_closed`

See `docs/upgrade-checklist.md` for the full pre-flight /
rollout / post-flight flow.
