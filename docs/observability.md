# Observability

clockify-mcp exposes Prometheus-scrapable metrics on the HTTP transport,
structured logs on stderr, and health/ready endpoints for
liveness/readiness probes. This document describes the metrics surface,
SLOs/SLIs, suggested alerts, and the stable log event taxonomy.

## Endpoints

| Endpoint | Purpose | Auth |
|---|---|---|
| `GET /health` | Liveness probe. Returns 200 always while the process is alive. | None |
| `GET /ready` | Readiness probe. Optionally runs an upstream health check (cached for a few seconds). Returns 200 or 503. | None |
| `GET /metrics` | Prometheus text exposition format v0.0.4. | None (see note below) |
| `POST /mcp` | JSON-RPC MCP endpoint. | Bearer |

**Note on `/metrics` auth:** the metrics endpoint is exposed without
bearer auth so that Prometheus scrapers can pull without a shared
secret. Counters do not leak sensitive data (tool names are stable,
label values are low-cardinality). If you need to gate metrics, do so
at the network layer (NetworkPolicy, ingress filter, or a sidecar).

## Metrics reference

| Name | Type | Labels | Description |
|---|---|---|---|
| `clockify_mcp_tool_calls_total` | counter | `tool`, `outcome` | Dispatches of `tools/call` by tool name and outcome. |
| `clockify_mcp_tool_call_duration_seconds` | histogram | `tool` | Dispatch duration in seconds. Default Prometheus buckets. |
| `clockify_mcp_rate_limit_rejections_total` | counter | `kind` | Rate limiter rejections. Kind is `concurrency` or `window`. |
| `clockify_mcp_http_requests_total` | counter | `path`, `method`, `status` | HTTP requests hitting the transport. |
| `clockify_mcp_ready_state` | gauge | — | 1 when the cached readiness probe is passing, 0 otherwise. |
| `clockify_mcp_build_info` | gauge | `version` | Build metadata. Value is always 1. |
| `clockify_mcp_inflight_tool_calls` | gauge | — | Current depth of the stdio dispatch semaphore. |

### Outcome values for `tool_calls_total`

Every tools/call dispatch records exactly one outcome:

| Outcome | Meaning |
|---|---|
| `success` | Handler returned without error and `isError=false`. |
| `tool_error` | Handler returned an error or `isError=true`. |
| `rate_limited` | `BeforeCall` rejected due to rate or concurrency limit. |
| `policy_denied` | Policy mode blocked the call. |
| `timeout` | Handler exceeded `CLOCKIFY_TOOL_TIMEOUT`. |
| `dry_run` | Dry-run intercepted a destructive call and returned a preview. |

### Rate limit kinds

| Kind | Source |
|---|---|
| `concurrency` | The `CLOCKIFY_MAX_CONCURRENT` semaphore was full. |
| `window` | The `CLOCKIFY_RATE_LIMIT` per-minute window was exhausted. |

The dispatch-layer goroutine cap (`MCP_MAX_INFLIGHT_TOOL_CALLS`) does
not emit rejections — it backpressures via the stdin scanner channel
instead. Observe saturation via `clockify_mcp_inflight_tool_calls`.

## SLOs

These are suggested service-level objectives for a typical enterprise
deployment. Tune them to match your traffic and reliability targets.

### Availability SLO

> 99.9% of HTTP requests to `/mcp` return a non-5xx status, measured
> over a rolling 30-day window.

Why 2xx/4xx both count as "available": 4xx indicates the server
correctly rejected a malformed or unauthorized request. 5xx indicates
the server itself failed.

### Tool success SLO

> 99% of `tools/call` dispatches return `isError=false`, excluding
> policy denies, measured over a rolling 30-day window.

Policy denies are excluded because they're a correct configuration
response, not a reliability failure.

### Latency SLO

> p95 `tools/call` dispatch duration < 3s; p99 < 10s.

The per-call timeout is `CLOCKIFY_TOOL_TIMEOUT` (default 45s) which
sets the hard ceiling; the SLO tightens this to what clients should
actually experience.

## SLIs (PromQL)

### Availability

```promql
sum(rate(clockify_mcp_http_requests_total{path="/mcp",status!~"5.."}[5m]))
/
sum(rate(clockify_mcp_http_requests_total{path="/mcp"}[5m]))
```

### Tool success rate

```promql
sum(rate(clockify_mcp_tool_calls_total{outcome="success"}[5m]))
/
sum(rate(clockify_mcp_tool_calls_total{outcome!="policy_denied"}[5m]))
```

### p95 tool latency

```promql
histogram_quantile(0.95,
  sum by (le) (rate(clockify_mcp_tool_call_duration_seconds_bucket[5m]))
)
```

### Rate limit rejection rate

```promql
sum(rate(clockify_mcp_rate_limit_rejections_total[5m]))
/
sum(rate(clockify_mcp_tool_calls_total[5m]))
```

### Dispatch saturation

```promql
clockify_mcp_inflight_tool_calls
```

Correlate with `MCP_MAX_INFLIGHT_TOOL_CALLS` (default 64). Sustained
saturation near the ceiling indicates the dispatch cap is too tight
for the current load.

## Example alerting rules

```yaml
groups:
  - name: clockify-mcp
    rules:
      - alert: ClockifyMCPHighToolErrorRate
        expr: |
          sum(rate(clockify_mcp_tool_calls_total{outcome=~"tool_error|timeout"}[5m]))
          /
          sum(rate(clockify_mcp_tool_calls_total{outcome!="policy_denied"}[5m]))
          > 0.01
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: clockify-mcp tool error rate >1% for 10m
          runbook: docs/runbooks/clockify-upstream-outage.md

      - alert: ClockifyMCPRateLimitSaturation
        expr: |
          sum(rate(clockify_mcp_rate_limit_rejections_total[5m]))
          /
          sum(rate(clockify_mcp_tool_calls_total[5m]))
          > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: clockify-mcp rate-limit rejection ratio >5% for 5m
          runbook: docs/runbooks/rate-limit-saturation.md

      - alert: ClockifyMCPLatencyBreach
        expr: |
          histogram_quantile(0.95,
            sum by (le) (rate(clockify_mcp_tool_call_duration_seconds_bucket[5m]))
          ) > 5
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: clockify-mcp p95 tool latency >5s for 10m

      - alert: ClockifyMCPNotReady
        expr: clockify_mcp_ready_state == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: clockify-mcp readiness probe failing
          runbook: docs/runbooks/clockify-upstream-outage.md

      - alert: ClockifyMCPAuthFailures
        expr: |
          sum(rate(clockify_mcp_http_requests_total{status="401"}[5m])) > 0.5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: clockify-mcp 401 rate elevated
          runbook: docs/runbooks/auth-failures.md
```

## Prometheus scrape config

```yaml
scrape_configs:
  - job_name: clockify-mcp
    scrape_interval: 15s
    metrics_path: /metrics
    static_configs:
      - targets:
          - clockify-mcp.clockify-mcp.svc.cluster.local:8080
```

For annotation-based discovery in Kubernetes, add these annotations to
the `deploy/k8s/service.yaml` or the Pod template:

```yaml
prometheus.io/scrape: "true"
prometheus.io/port: "8080"
prometheus.io/path: "/metrics"
```

## Log event taxonomy

All logs go to stderr via `log/slog`. Format is controlled by
`MCP_LOG_FORMAT` (text or json) and `MCP_LOG_LEVEL`
(debug/info/warn/error).

Stable events emitted by the server:

| Event | Level | Source | Stable keys |
|---|---|---|---|
| `initialize` | info | `internal/mcp/server.go` | `client`, `version` |
| `tool_call` | info | `internal/mcp/server.go` | `tool`, `req_id`, `duration_ms`, `outcome` |
| `tool_error` | warn | `internal/mcp/server.go` | `tool`, `req_id`, `error` |
| `async_response_failed` | warn | `internal/mcp/server.go` | `error` |
| `http_request` | info | `internal/mcp/transport_http.go` | `method`, `path`, `rpc_method`, `status`, `req_id`, `duration_ms` |
| `http_auth_failure` | warn | `internal/mcp/transport_http.go` | `remote`, `reason` |
| `http_body_too_large` | warn | `internal/mcp/transport_http.go` | `limit`, `size` |
| `clockify_api_error` | warn | `internal/clockify/client.go` | `status`, `method`, `path`, `retries` |
| `rate_limited` | debug | `internal/ratelimit/ratelimit.go` | `kind`, `reason` |
| `dry_run_intercepted` | info | `internal/dryrun/dryrun.go` | `tool`, `strategy` |
| `response_truncated` | debug | `internal/enforcement/enforcement.go` | `budget` |
| `truncate_marshal_failed` | debug | `internal/enforcement/enforcement.go` | `error` |

Secret values (bearer tokens, API keys) are never logged. Request IDs
(`req_id`) are monotonic per-process counters suitable for log
correlation.

## Structured log JSON example

```json
{
  "time": "2026-04-10T12:00:00Z",
  "level": "INFO",
  "msg": "http_request",
  "method": "POST",
  "path": "/mcp",
  "rpc_method": "tools/call",
  "status": 200,
  "req_id": 42,
  "duration_ms": 123
}
```

## Related

- [docs/verification.md](verification.md) — release artifact verification
- [docs/runbooks/](runbooks/) — incident response procedures
- [deploy/k8s/](../deploy/k8s/) — Kubernetes reference manifests
