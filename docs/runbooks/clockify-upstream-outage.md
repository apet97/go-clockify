# Clockify upstream outage

## Why this runbook exists

`go-clockify` is a thin proxy in front of `api.clockify.me`. When
the upstream API degrades or fails, every tool call that depends on
it fails. The server itself stays up — the panic-recovery and
init-guard layers prevent it from crashing — but operators see a
flood of 5xx-class tool errors and need to decide whether to fail
fast, fall back to a degraded mode, or wait for the upstream to
recover.

## 1. Symptoms

- `clockify_mcp_tool_call_duration_seconds` p50 climbs to the
  upstream timeout (`CLOCKIFY_TOOL_TIMEOUT`, default 45s).
- `clockify_mcp_tool_calls_total{status="error"}` rises sharply.
- Structured logs show repeated `level=ERROR msg=clockify_request
  status_code=5XX` or `error="context deadline exceeded"`.
- `status.clockify.me` reports an incident.
- `/ready` returns `503` (the readiness probe checks upstream
  reachability and flips to not-ready when consecutive checks fail).

## 2. Where to look first

```sh
# Confirm upstream is the problem and not us
curl -sf https://status.clockify.me/api/v2/status.json | jq .

# Direct upstream check (replace with your sacrificial key)
curl -i -H "X-Api-Key: $CLOCKIFY_LIVE_API_KEY" \
  https://api.clockify.me/api/v1/workspaces

# Our error rate
curl -sf http://<host>:8080/metrics \
  | grep -E '^clockify_mcp_tool_calls_total\{.*status="error".*\}'

# Recent error logs
kubectl -n clockify-mcp logs deploy/clockify-mcp --since=15m \
  | grep 'level=ERROR msg=clockify_request'
```

## 3. Immediate mitigation

There is no way to make the upstream API work from inside this
project. The mitigation is about minimising blast radius and giving
operators a clean failure mode.

### Force read-only mode to stop accepting writes

```sh
kubectl -n clockify-mcp set env deploy/clockify-mcp \
  CLOCKIFY_POLICY=read_only
kubectl -n clockify-mcp rollout status deploy/clockify-mcp
```

`read_only` blocks every destructive tool at the policy layer. Read
calls still attempt the upstream, but operators can no longer write
data that may be lost when the upstream recovers (writes that landed
during a partial outage are the most common source of duplicate
entries).

### Lower the timeout to fail fast

If clients are hanging on 45-second timeouts, lower the upstream
timeout so they get a clean error and can retry against another
backend or surface the failure to the user faster:

```sh
kubectl -n clockify-mcp set env deploy/clockify-mcp \
  CLOCKIFY_TOOL_TIMEOUT=10s
```

Restore the default after the upstream recovers.

### Communicate

Open a `clockify-upstream-outage` issue with the start time and a
link to the `status.clockify.me` incident. Update it as the
upstream recovers. Resolve when `clockify_mcp_tool_calls_total`
error rate is back to baseline for >15 minutes.

## 4. Root-cause checklist

- [ ] **Genuine upstream outage.** `status.clockify.me` confirms the
  incident. Mitigation: wait. Document in postmortem.
- [ ] **Upstream rate limiting us specifically.** 5xx with
  `Retry-After` header, no entry on the public status page.
  Mitigation: lower `CLOCKIFY_RATE_LIMIT` (see
  `rate-limit-saturation.md` for the runbook). Cause: usually a
  retry storm in our client.
- [ ] **DNS resolution failure.** Logs show `dial tcp: lookup
  api.clockify.me: no such host`. Mitigation: check the cluster
  DNS resolver. Cause: rare; usually a CoreDNS misconfig.
- [ ] **TLS handshake failure.** Logs show `tls: handshake failure`
  or `x509: certificate signed by unknown authority`. Mitigation:
  check the system CA bundle in the pod. Cause: a base image
  rotation that dropped a Let's Encrypt root.
- [ ] **Egress network policy regression.** Logs show `dial tcp
  <ip>:443: i/o timeout`. Mitigation: check
  `deploy/k8s/base/networkpolicy.yaml` and any cluster-wide network
  policies. Cause: a security tightening that did not allowlist
  Clockify's CDN range.
- [ ] **Webhook URL validation rejecting a legitimate target.** Logs
  show `webhook URL rejected` rather than 5xx — this is a different
  symptom and lives in `auth-failures.md` instead.

## 5. Postmortem template

- **Trigger** — Upstream incident? Local config change? Network
  policy change?
- **Detection** — Did `/ready` flip to 503? How long between client
  reports and our discovery?
- **Mitigation** — Did `CLOCKIFY_POLICY=read_only` reduce write
  errors as expected? Did lowering `CLOCKIFY_TOOL_TIMEOUT` improve
  client UX?
- **Recovery** — When did upstream return to healthy? Did our error
  rate recover automatically or did we need to restart?
- **Permanent fix** — Anything to add to the readiness probe?
  Anything to harden in the upstream client (jitter, backoff,
  hedging)?
- **Prevention** — Should we add a circuit breaker to the upstream
  client so we fail fast without consuming the local rate-limit
  budget?

## See also

- `internal/clockify/` — upstream HTTP client (timeout, retry,
  backoff, pagination).
- `internal/metrics/metrics.go` — `clockify_mcp_tool_calls_total`,
  `clockify_mcp_tool_call_duration_seconds`,
  `clockify_mcp_panics_recovered_total`.
- `rate-limit-saturation.md` — when 5xx is actually 429 in disguise.
- `auth-failures.md` — when 5xx is actually 401 in disguise.
- `SECURITY.md` — panic-containment policy that keeps the server up
  during upstream chaos.
