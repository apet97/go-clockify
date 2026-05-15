# Rate-limit operations

> **Historical artifact. Not current one-user MCP product documentation.**
> Preserved for platform-era audit/history only. Start current one-user work from `README.md`, `docs/agent-cookbook.md`, `docs/tool-catalog.md`, and `docs/goals/oneuser-tool-coverage.md`.


This is the launch-checklist entry point for rate-limit incidents. For
step-by-step saturation triage, use
[`rate-limit-saturation.md`](rate-limit-saturation.md).

## What Is Enforced In Process

`clockify-mcp` has two layers of process-local protection:

- Tool-call throughput and concurrency:
  `CLOCKIFY_RATE_LIMIT`, `CLOCKIFY_PER_TOKEN_RATE_LIMIT`,
  `CLOCKIFY_MAX_CONCURRENT`, `CLOCKIFY_PER_TOKEN_CONCURRENCY`, and
  `MCP_MAX_INFLIGHT_TOOL_CALLS`.
- HTTP admission before JSON-RPC dispatch:
  `MCP_HTTP_RATELIMIT_PER_IP`,
  `MCP_HTTP_RATELIMIT_PER_PRINCIPAL`, and
  `MCP_HTTP_RATELIMIT_GET_PER_SESSION`.

Hosted manifests set non-zero HTTP admission defaults, but these
limits are still per process. A multi-replica hosted service needs a
gateway, ingress, or load balancer to enforce global source and
principal quotas across pods.

## Metrics Operators Must Watch

- `clockify_mcp_rate_limit_rejections_total`
- `clockify_mcp_http_admission_rejections_total{path,reason}`
- `clockify_mcp_inflight_tool_calls`
- `clockify_mcp_tool_call_duration_seconds`
- `clockify_upstream_retries_total{reason="rate_limited"}`
- `clockify_upstream_requests_total{status="4xx"}`

If `clockify_mcp_http_admission_rejections_total{reason="ip"}` or
`{reason="principal"}` rises on more than one pod, check the external
gateway quota before raising the per-process MCP limits.

The starter Prometheus rules alert on both
`ClockifyMCPRateLimitSaturation` and
`ClockifyMCPHTTPAdmissionRejections`; the Grafana dashboard's rate-limit
panel includes the same HTTP admission series grouped by `path` and
`reason`.

## Launch Evidence

For a paid hosted launch, archive:

1. The selected per-process env values.
2. The external gateway or load-balancer quota policy.
3. A load run from `.github/workflows/load.yml` or the equivalent k6
   harness with p50, p95, and p99 latency compared to
   [`docs/performance.md`](../performance.md).
4. A metrics snapshot showing both local rate-limit counters and HTTP
   admission counters are visible to on-call dashboards.

## See Also

- [`rate-limit-saturation.md`](rate-limit-saturation.md) - active
  incident triage and mitigation.
- [`clockify-upstream-outage.md`](clockify-upstream-outage.md) -
  distinguishing Clockify upstream 429s from local saturation.
- [`../performance.md`](../performance.md) - published operating
  envelope.
