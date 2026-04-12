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
| `GET /metrics` | Prometheus text exposition format v0.0.4. | None on the legacy shared listener; configurable on `MCP_METRICS_BIND` |
| `POST /mcp` | JSON-RPC MCP endpoint. | Bearer |

**Note on `/metrics` auth:** legacy `MCP_TRANSPORT=http` keeps `/metrics`
on the main listener for compatibility. `MCP_TRANSPORT=streamable_http`
does not expose public metrics by default; use `MCP_METRICS_BIND` and
optionally `MCP_METRICS_AUTH_MODE=static_bearer` to isolate scrapes onto
a dedicated listener.

**Note on `MCP_TRANSPORT=grpc`:** the gRPC transport (ADR 012) binds
a single listener on `MCP_GRPC_BIND` (default `:9090`) and serves
no HTTP endpoints at all — `/health`, `/ready`, and `/metrics` are
not available. To scrape metrics while running gRPC, set
`MCP_METRICS_BIND=:9091` (Wave 5 backlog — not yet plumbed through
the Helm chart, see `docs/audit-chart-vs-config.md`) or run a
sidecar that exports to Prometheus. Kubernetes readiness probes
should fall back to `tcpSocket` on the gRPC port until the server
implements the gRPC health protocol.

## Metrics reference

### Server RED metrics

| Name | Type | Labels | Description |
|---|---|---|---|
| `clockify_mcp_tool_calls_total` | counter | `tool`, `outcome` | Dispatches of `tools/call` by tool name and outcome. Outcome is one of `success`, `tool_error`, `rate_limited`, `policy_denied`, `timeout`, `cancelled`, `dry_run`. |
| `clockify_mcp_tool_call_duration_seconds` | histogram | `tool` | Dispatch duration in seconds. Buckets tuned to the 3s/10s SLO: `{0.05, 0.1, 0.25, 0.5, 1, 2, 3, 5, 10, 20, 45}`. |
| `clockify_mcp_http_requests_total` | counter | `path`, `method`, `status` | HTTP requests hitting the transport. `path` is normalized to `{/mcp, /health, /ready, /metrics, /other}`. |
| `clockify_mcp_http_request_duration_seconds` | histogram | `path`, `method`, `status` | HTTP request duration. Buckets tuned for fast JSON-RPC: `{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}`. |
| `clockify_mcp_rate_limit_rejections_total` | counter | `kind`, `scope` | Rate limiter rejections. `kind` is `concurrency` or `window`. `scope` is `global` (the process-wide `CLOCKIFY_RATE_LIMIT`/`CLOCKIFY_MAX_CONCURRENT` layer) or `per_token` (the per-`Principal.Subject` sub-layer gated by `CLOCKIFY_PER_TOKEN_RATE_LIMIT`/`CLOCKIFY_PER_TOKEN_CONCURRENCY`). |
| `clockify_mcp_cancellations_total` | counter | `reason` | `tools/call` cancellations. `reason` is one of `client_requested` (peer sent `notifications/cancelled`), `timeout` (per-tool deadline expired), `context_cancelled` (parent context aborted, e.g. server shutdown). |
| `clockify_mcp_protocol_errors_total` | counter | `code` | JSON-RPC protocol-level error responses by error code. `code` is the numeric JSON-RPC error code as a string, or a short reason for notification drops. |
| `clockify_mcp_panics_recovered_total` | counter | `site` | Panics recovered from tool handlers or HTTP handlers. `site` is one of `stdio_tool_dispatch`, `http`. |
| `clockify_mcp_grpc_auth_rejections_total` | counter | `reason` | gRPC auth interceptor rejections (requires `-tags=grpc` build). `reason` is one of `missing_metadata`, `missing_authorization`, `empty_authorization`, `auth_failed`. Only incremented on the gRPC transport; the counter exists but stays at zero on HTTP/stdio. |
| `clockify_mcp_inflight_tool_calls` | gauge | — | Current depth of the stdio dispatch semaphore. |
| `clockify_mcp_ready_state` | gauge | — | 1 when the cached readiness probe is passing, 0 otherwise. |
| `clockify_mcp_build_info` | gauge | `version`, `commit`, `build_date`, `go_version` | Build metadata. Value is always 1. |

### Upstream Clockify client metrics

| Name | Type | Labels | Description |
|---|---|---|---|
| `clockify_upstream_requests_total` | counter | `endpoint`, `method`, `status` | Outbound Clockify API requests. `endpoint` is the path template with `:id` placeholders; `status` is bucketed to `{2xx, 3xx, 4xx, 5xx, error}`. |
| `clockify_upstream_request_duration_seconds` | histogram | `endpoint`, `method` | Outbound Clockify API latency. Buckets: `{0.05, 0.1, 0.25, 0.5, 1, 2, 3, 5, 10, 20, 45}`. |
| `clockify_upstream_retries_total` | counter | `endpoint`, `reason` | Retry attempts by reason: `rate_limited`, `bad_gateway`, `service_unavailable`, `gateway_timeout`, `error`. |

### Go runtime + process metrics

| Name | Type | Labels | Description |
|---|---|---|---|
| `go_goroutines` | gauge | — | Number of goroutines currently running. |
| `go_gomaxprocs` | gauge | — | Current `GOMAXPROCS` setting. |
| `go_memstats_heap_alloc_bytes` | gauge | — | Heap bytes allocated and still in use. |
| `go_memstats_heap_inuse_bytes` | gauge | — | Heap bytes currently in use (objects + unused). |
| `go_memstats_heap_released_bytes` | gauge | — | Heap bytes released to the OS. |
| `go_memstats_stack_inuse_bytes` | gauge | — | Stack bytes in use. |
| `go_memstats_sys_bytes` | gauge | — | Total bytes obtained from the OS. |
| `go_gc_runs_total` | gauge | — | Total number of completed GC cycles. |
| `go_info` | gauge | `version` | Go runtime version. Value is always 1. |
| `process_start_time_seconds` | gauge | — | Process start time as unix epoch seconds. |
| `process_resident_memory_bytes` | gauge | — | Resident memory approximation via `runtime/metrics`. |
| `process_open_fds` | gauge | — | Open file descriptor count (cached 5s, best-effort via `/dev/fd`). |

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
| `window` | The `CLOCKIFY_RATE_LIMIT` fixed 60s window was exhausted. |

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

      # Multi-window multi-burn-rate alerts for the 99.9% SLO on
      # (tool_error|timeout) as a fraction of non-policy-denied calls.
      # Fast burn: budget exhausted in 1h at 14.4x the allowed error rate.
      # Slow burn: budget exhausted in 6h at 6x the allowed error rate.
      - alert: ClockifyMCPFastBurn
        expr: |
          (
            sum(rate(clockify_mcp_tool_calls_total{outcome=~"tool_error|timeout"}[5m]))
            /
            sum(rate(clockify_mcp_tool_calls_total{outcome!="policy_denied"}[5m]))
          ) > (14.4 * 0.001)
          and
          (
            sum(rate(clockify_mcp_tool_calls_total{outcome=~"tool_error|timeout"}[1h]))
            /
            sum(rate(clockify_mcp_tool_calls_total{outcome!="policy_denied"}[1h]))
          ) > (14.4 * 0.001)
        for: 2m
        labels:
          severity: critical
          slo: clockify-mcp-99_9
        annotations:
          summary: clockify-mcp fast-burn — 1h error budget exhaustion trajectory
          description: Error rate is burning through the 99.9% SLO budget at 14.4x allowed rate.
          runbook: docs/runbooks/clockify-upstream-outage.md

      - alert: ClockifyMCPSlowBurn
        expr: |
          (
            sum(rate(clockify_mcp_tool_calls_total{outcome=~"tool_error|timeout"}[30m]))
            /
            sum(rate(clockify_mcp_tool_calls_total{outcome!="policy_denied"}[30m]))
          ) > (6 * 0.001)
          and
          (
            sum(rate(clockify_mcp_tool_calls_total{outcome=~"tool_error|timeout"}[6h]))
            /
            sum(rate(clockify_mcp_tool_calls_total{outcome!="policy_denied"}[6h]))
          ) > (6 * 0.001)
        for: 15m
        labels:
          severity: warning
          slo: clockify-mcp-99_9
        annotations:
          summary: clockify-mcp slow-burn — 6h error budget exhaustion trajectory
          description: Error rate is burning through the 99.9% SLO budget at 6x allowed rate.
          runbook: docs/runbooks/clockify-upstream-outage.md

      # ClockifyMCPUpstreamUnavailable — referenced by
      # docs/runbooks/clockify-upstream-outage.md but previously undefined.
      - alert: ClockifyMCPUpstreamUnavailable
        expr: |
          sum(rate(clockify_upstream_requests_total{status=~"5xx|error"}[5m])) > 0.5
          and
          sum(rate(clockify_upstream_requests_total{status="2xx"}[5m])) == 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: clockify-mcp cannot reach upstream Clockify API
          description: Every upstream Clockify request in the last 5m errored or returned 5xx.
          runbook: docs/runbooks/clockify-upstream-outage.md

      - alert: ClockifyMCPHighLatency
        expr: |
          histogram_quantile(0.99,
            sum by (le) (rate(clockify_mcp_tool_call_duration_seconds_bucket[10m]))
          ) > 10
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: clockify-mcp p99 tool latency >10s for 10m
          runbook: docs/runbooks/high-latency.md
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
| `initialize` | info | `internal/mcp/server.go` | `protocol_version`, `requested_version`, `client_name`, `client_version` |
| `tool_call` | info | `internal/mcp/server.go` | `tool`, `req_id`, `duration_ms`, `outcome` |
| `tool_error` | warn | `internal/mcp/server.go` | `tool`, `req_id`, `error` |
| `async_response_failed` | warn | `internal/mcp/server.go` | `error` |
| `panic_recovered` | error | `internal/mcp/server.go`, `internal/mcp/transport_http.go` | `site`, `tool` / `path` / `method`, `panic`, `stack` |
| `notification_dropped` | warn | `internal/mcp/server.go` | `method`, `reason` |
| `http_request` | info | `internal/mcp/transport_http.go` | `method`, `path`, `rpc_method`, `status`, `req_id`, `duration_ms` |
| `http_auth_failure` | warn | `internal/mcp/transport_http.go` | `remote`, `reason` |
| `http_body_too_large` | warn | `internal/mcp/transport_http.go` | `limit`, `size` |
| `clockify_api_error` | warn | `internal/clockify/client.go` | `status`, `method`, `path`, `retries` |
| `rate_limited` | debug | `internal/ratelimit/ratelimit.go` | `kind`, `reason` |
| `dry_run_intercepted` | info | `internal/dryrun/dryrun.go` | `tool`, `strategy` |
| `response_truncated` | debug | `internal/enforcement/enforcement.go` | `budget` |
| `truncate_marshal_failed` | debug | `internal/enforcement/enforcement.go` | `error` |

**PII redaction.** Every slog handler is wrapped in
`internal/logging.RedactingHandler`, which recursively scrubs 20+
well-known secret-key patterns (`authorization`, `api_key`, `bearer`,
`token`, `cookie`, `client_secret`, `refresh_token`, …) from both
top-level attrs and nested map/group values before they reach the
text/JSON encoder. Hot-path code still avoids logging secrets
explicitly; the redactor is defence-in-depth for accidental
header-map or error-body logs.

Request IDs (`req_id`) are monotonic per-process counters suitable for
log correlation.

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

## Distributed tracing (optional, `-tags=otel`)

The server supports OpenTelemetry tracing behind a build tag. Default
`go build` produces a stdlib-only binary with **zero** `opentelemetry.io`
symbols — enforced by the `verify-no-otel-default` CI step which runs
`go tool nm` and fails the build on any leaked symbols. To ship a
tracing-enabled binary, rebuild with `-tags=otel`:

```sh
go build -tags=otel ./cmd/clockify-mcp
OTEL_EXPORTER_OTLP_ENDPOINT=https://otlp.example.internal:4318 clockify-mcp
```

When the `otel` tag is compiled in AND `OTEL_EXPORTER_OTLP_ENDPOINT` is
set at runtime, `cmd/clockify-mcp/otel_on.go` delegates to
`Install(ctx)` in the `github.com/apet97/go-clockify/internal/tracing/otel`
sub-module, which constructs an OTLP HTTP exporter, wires a
`TracerProvider` with a default service name of `clockify-mcp`,
registers the W3C trace-context propagator, and replaces
`tracing.Default` via `SetDefault`. The sub-module is a separate Go
module (`internal/tracing/otel/go.mod`) so the top-level `go.mod`
carries zero `go.opentelemetry.io` rows — see ADR 009. If the
exporter fails to construct (bad endpoint, network error) the
installer logs through `slog.Warn("otel_install_failed")` and the
process continues with the no-op tracer rather than crashing.

Spans are emitted from two sites:

| Span | Attributes | Emitted by |
|------|------------|------------|
| `mcp.tools/call` | `tool.name`, `outcome` (`success`, `tool_error`, `rate_limited`, `policy_denied`, `timeout`, `dry_run`, `cancelled`) | `internal/mcp/server.callTool` |
| `clockify.http` | `upstream.endpoint` (template), `http.method`, `http.status_code` | `internal/clockify/client.doOnce` |

The outbound Clockify request carries a W3C `traceparent` header
injected via `tracing.Default.InjectHTTPHeaders`, so downstream services
that participate in the trace can stitch a complete request timeline.

Env vars honoured by the otel build (beyond `OTEL_EXPORTER_OTLP_ENDPOINT`)
are the standard ones supported by the `otlptracehttp` exporter —
`OTEL_EXPORTER_OTLP_HEADERS`, `OTEL_EXPORTER_OTLP_COMPRESSION`,
`OTEL_EXPORTER_OTLP_TIMEOUT`, etc. — see the upstream OpenTelemetry
documentation. The default build ignores all of them.

## Related

- [docs/verification.md](verification.md) — release artifact verification
- [docs/runbooks/](runbooks/) — incident response procedures
- [deploy/k8s/](../deploy/k8s/) — Kubernetes reference manifests
