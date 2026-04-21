# Authentication failures

## Why this runbook exists

`go-clockify` runs three independent authentication layers, and an
incident often presents as "auth is broken" before triage can tell
which one is at fault:

- **Inbound** — the HTTP transport authenticates clients via
  `MCP_AUTH_MODE` (`static_bearer`, `oidc`, `forward_auth`, `mtls`).
  Stdio mode has no inbound auth — the parent process is trusted.
- **Upstream** — the Clockify API client authenticates with the
  `CLOCKIFY_API_KEY` (or the per-installation token in HTTP-mode
  multi-tenant deployments).
- **gRPC** — the gRPC transport (build tag `grpc`) has its own
  bearer-token / OIDC handshake, surfaced via
  `clockify_mcp_grpc_auth_rejections_total`.

This runbook covers all three. The first job is to identify which
layer is failing.

## 1. Symptoms

- HTTP transport: clients see `401 Unauthorized` or `403 Forbidden`
  on every tool call.
- Structured logs show one of:
  - `level=WARN msg=http_request status=401 reason=auth_failed`
  - `level=WARN msg=http_request status=403 reason=cors_rejected`
  - `level=WARN msg=tool_call error="clockify ... 401 Unauthorized ..."`
- gRPC transport: `clockify_mcp_grpc_auth_rejections_total{reason="auth_failed|missing_authorization|empty_authorization|reauth_expired"}` rises.
- Upstream Clockify auth failure: `msg=tool_call` errors consistently
  include `401 Unauthorized` from `api.clockify.me` across multiple
  tools.

## 2. Where to look first

```sh
# Inbound HTTP rejections
kubectl -n clockify-mcp logs deploy/clockify-mcp --since=15m \
  | grep 'msg=http_request' \
  | grep -E 'status=401|status=403|reason=(auth_failed|cors_rejected)'

# Upstream Clockify auth failures
kubectl -n clockify-mcp logs deploy/clockify-mcp --since=15m \
  | grep 'msg=tool_call' \
  | grep '401 Unauthorized'

# Inbound rejection counters
curl -sf http://<host>:8080/metrics \
  | grep -E '^clockify_mcp_(http_requests_total|grpc_auth_rejections_total)'

# Confirm the inbound bearer token is set
kubectl -n clockify-mcp get deploy/clockify-mcp -o yaml \
  | grep -A1 MCP_BEARER_TOKEN  # value will be REDACTED in env

# Confirm the upstream API key is still valid
curl -sf -H "X-Api-Key: $CLOCKIFY_API_KEY" \
  https://api.clockify.me/api/v1/workspaces | jq '.[].name'
```

## 3. Immediate mitigation

Mitigation depends entirely on which layer is rejecting. Triage
first, then act.

### Inbound: `MCP_BEARER_TOKEN` rotation needed

If the bearer token has been distributed via a leaked channel, or
a client has been retired and you want to invalidate their access:

```sh
# Generate a new token (≥16 random chars; the server enforces this)
NEW_TOKEN=$(openssl rand -hex 32)
kubectl -n clockify-mcp create secret generic clockify-mcp-bearer \
  --from-literal=token="$NEW_TOKEN" \
  --dry-run=client -o yaml \
  | kubectl apply -f -
kubectl -n clockify-mcp rollout restart deploy/clockify-mcp
kubectl -n clockify-mcp rollout status deploy/clockify-mcp
```

Distribute the new token to authorized clients out of band. Because
the deployment uses `maxUnavailable: 0`, the service stays available
through the restart. See `deploy/k8s/README.md` for the full
rotation flow.

### Inbound: `MCP_AUTH_MODE=oidc` issuer outage

If the OIDC issuer (`MCP_OIDC_ISSUER`) is down, every inbound
request fails closed. Two options:

1. Wait for the issuer to recover. The server has no fallback by
   design.
2. Temporarily switch to `static_bearer` if you have a backup
   channel for distributing a token quickly:

```sh
kubectl -n clockify-mcp set env deploy/clockify-mcp \
  MCP_AUTH_MODE=static_bearer \
  MCP_BEARER_TOKEN=<new-token>
```

Restore `oidc` after the issuer recovers.

### Upstream: `CLOCKIFY_API_KEY` expiration

If `msg=tool_call` errors consistently include `401 Unauthorized`
from Clockify across multiple tools, the upstream API key has been
rotated, revoked, or expired. Mitigation:

```sh
# Generate a new key in the Clockify dashboard
# (https://app.clockify.me/user/preferences#advanced)
kubectl -n clockify-mcp create secret generic clockify-api-key \
  --from-literal=api-key=<new-key> \
  --dry-run=client -o yaml \
  | kubectl apply -f -
kubectl -n clockify-mcp rollout restart deploy/clockify-mcp
```

### gRPC: certificate-based mTLS rejection

If `MCP_AUTH_MODE=mtls` is in use on the gRPC transport, check the
client certificate's expiration and the trust bundle in the server.
Initial rejections increment
`clockify_mcp_grpc_auth_rejections_total{reason="auth_failed"}`.
Only periodic re-authentication failures emit a structured log
record (`msg=grpc_reauth_failed`).

## 4. Root-cause checklist

- [ ] **Token leak.** Was the failing client's token shared in a
  channel that exposed it? If yes, rotate immediately and run a
  retroactive check of ingress / reverse-proxy logs for unfamiliar
  client identity. `clockify_mcp_http_requests_total` confirms the
  rejection volume, but it does not identify the caller.
- [ ] **Upstream key expiration.** Clockify API keys do not expire
  by default but can be revoked from the dashboard. Confirm with
  the workspace admin.
- [ ] **OIDC issuer outage or key rotation.** Hit
  `<MCP_OIDC_ISSUER>/.well-known/openid-configuration` and
  `<MCP_OIDC_ISSUER>/.well-known/jwks.json` to confirm both are
  reachable and contain the expected key IDs.
- [ ] **Forward-auth header drift.** `MCP_AUTH_MODE=forward_auth`
  trusts a header set by an upstream proxy. If the proxy was
  reconfigured to send a different header, every request fails
  closed. Check the proxy config and both
  `MCP_FORWARD_SUBJECT_HEADER` + `MCP_FORWARD_TENANT_HEADER`.
- [ ] **CORS mistakenly diagnosed as auth.** A browser client that
  is blocked by CORS can present as "401" in the client UI even
  though the server returned 403 + CORS-rejection. Check
  `MCP_ALLOWED_ORIGINS` and the structured log for
  `msg=http_request reason=cors_rejected` rather than an auth
  failure.
- [ ] **Init-handshake gating.** A client that sends `tools/call`
  before `initialize` gets `-32002 server not initialized`, which
  some clients render as "auth failure". Confirm via the client
  transcript or raw JSON-RPC response; there is no dedicated
  `init_required` server log event today.
- [ ] **Webhook URL validation rejection.** A `webhooks_create`
  call against an HTTP-only or private-IP target is rejected at
  validation time. Logs show `webhook URL rejected`. This is not
  an auth failure but is sometimes mistaken for one — direct the
  client to use HTTPS and a public hostname.

## 5. Postmortem template

- **Layer** — Inbound (HTTP), inbound (gRPC), or upstream Clockify?
- **Trigger** — Token leak, expiration, OIDC outage, key rotation,
  proxy reconfig, client misconfiguration?
- **Detection** — Which counter or log message surfaced the issue?
- **Mitigation** — Rotate token, rotate API key, switch auth mode,
  fix CORS, fix proxy?
- **Communication** — Did affected clients get notified out-of-band
  before they hit the rejection?
- **Permanent fix** — Anything to harden in the auth path? A new
  metric to add? A health check that would have surfaced this
  earlier?

## See also

- `internal/authn/` — all four inbound auth modes and their tests.
- `internal/clockify/` — upstream API key handling and the
  `X-Api-Key` header.
- `internal/metrics/metrics.go` — `clockify_mcp_grpc_auth_rejections_total`
  and the inbound `clockify_mcp_http_requests_total{status="401|403"}`
  series.
- `deploy/k8s/README.md` — bearer token rotation flow and Secret
  template.
- `SECURITY.md` — auth threat model and hardening details.
- `clockify-upstream-outage.md` — when 401 is actually a 5xx in
  disguise.
