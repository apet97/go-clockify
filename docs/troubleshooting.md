# Troubleshooting

Symptom → diagnosis → fix matrix for the most common failure modes.
For operational incidents (alert firing, pod disruption, metrics
outage), see the runbooks under [`docs/runbooks/`](runbooks/).

## Tool-call failures

| Symptom | Diagnosis | Fix |
|---|---|---|
| All `tools/call` responses return `{"error":{"code":-32002,"message":"server not initialized"}}` | The client skipped `initialize` before its first `tools/call`. | Send an `initialize` request first. See [`docs/safe-usage.md`](safe-usage.md). |
| Tool calls fail with `tool blocked by policy` | `CLOCKIFY_POLICY` denies this tool (read_only / safe_core) or an explicit `CLOCKIFY_DENY_TOOLS` entry. | Relax `CLOCKIFY_POLICY` or remove the deny entry. See [ADR 005](adr/005-policy-modes.md). |
| `rate limited: concurrency limit exceeded` | `CLOCKIFY_MAX_CONCURRENT` (default 10) or `CLOCKIFY_PER_TOKEN_CONCURRENCY` (default 5) ceiling hit. | Raise the cap, or reduce concurrent calls from the client. Check `clockify_mcp_rate_limit_rejections_total{kind="concurrency",scope}`. |
| `rate limited: rate limit exceeded` | `CLOCKIFY_RATE_LIMIT` (default 120/60s) or `CLOCKIFY_PER_TOKEN_RATE_LIMIT` (default 60/60s) window exhausted. | Wait for the window to roll, raise the cap, or split traffic across replicas. |
| `tool call timed out` after 45s | `CLOCKIFY_TOOL_TIMEOUT` (default 45s) expired. | Usually a slow Clockify response or a very wide report range. Narrow the range, or raise `CLOCKIFY_TOOL_TIMEOUT` (max 10m). |
| A report tool returns `entry cap of N exceeded` | `CLOCKIFY_REPORT_MAX_ENTRIES` (default 10000) — the range with `include_entries=true` would aggregate too many rows. | Narrow the range, set `include_entries=false`, or raise the cap. This fails closed on purpose — see the ADR set for the safety rationale. |
| `tools/call` returns `{"error":{"code":-32602,"message":"invalid params at /<field>","data":{"pointer":"/<field>"}}}` | JSON-schema validation rejected the arguments (W2-01). The pointer in `error.data.pointer` identifies the failing field in RFC 6901 form — e.g. `/start` for a missing required RFC3339 timestamp, or `/bogus` for an unknown top-level key. Every Tier 1 + Tier 2 tool now carries `additionalProperties: false`. | Inspect the tool's advertised `inputSchema` via `tools/list` and fix the field the pointer names. Drop unknown top-level keys — extras are the most common cause after Wave 2. See [ADR 008](adr/008-runtime-schema-validation.md). |

## Transport / auth

| Symptom | Diagnosis | Fix |
|---|---|---|
| `missing bearer token` / 401 on every request | `Authorization: Bearer <token>` header missing. | Send the header. Token is configured via `MCP_BEARER_TOKEN` (static) or negotiated via OIDC. |
| `invalid bearer token` with static auth | Token mismatch against `MCP_BEARER_TOKEN`. | Rotate the token via the k8s Secret and restart the Deployment. See `deploy/k8s/README.md`. |
| OIDC auth fails with `token audience does not list resource URI` | `MCP_RESOURCE_URI` configured but the inbound JWT's `aud` claim omits it. | Issue the token from an IdP that accepts an audience query parameter, or add the server's resource URI to the client's token request. |
| `stdio process exits silently at startup` | Configuration error — check **stderr**. Protocol responses go to stdout, everything else (including config-validation errors) goes to stderr via slog. | Redirect stderr to a file and re-run: `2>/tmp/clockify-mcp.log`. |
| Streamable HTTP client sends `Last-Event-ID: N` but receives no replayed events | Either the session has no backlog (nothing happened), or `N` is beyond the highest-stamped event id in the backlog, or the session expired. | Check the session's `expiresAt` in the control plane. Bump `MCP_SESSION_TTL` if legitimate clients drift off. |
| Mcp-Protocol-Version mismatch on 0.6+ | The client sent a supported version that doesn't match the one negotiated at `initialize` for this session. | Re-initialize the session, or stop sending `Mcp-Protocol-Version` for back-compat clients (absent header is accepted). |
| `per_token` rate limits hit while `global` is not | Intentional isolation — see [ADR 005](adr/005-policy-modes.md) and [W1-07](wave1-backlog.md). | Raise `CLOCKIFY_PER_TOKEN_RATE_LIMIT` for that tenant, or throttle the client. |

## Observability

| Symptom | Diagnosis | Fix |
|---|---|---|
| `no data` on dashboards but pods are healthy | Prometheus can't scrape `/metrics`. | Follow [`docs/runbooks/metrics-scrape-failure.md`](runbooks/metrics-scrape-failure.md). |
| `ClockifyMCPHighLatency` firing | p99 tool latency > 10s. | Follow [`docs/runbooks/high-latency.md`](runbooks/high-latency.md). |
| OTel traces expected but missing | Binary was built without `-tags=otel`, or `OTEL_EXPORTER_OTLP_ENDPOINT` is unset. | Rebuild with `-tags=otel` and set the env var. See [`docs/observability.md`](observability.md). |

## Getting more debug info

- Set `MCP_LOG_LEVEL=debug` to enable slog debug events (e.g., truncation decisions, dry-run strategy selection).
- Set `MCP_LOG_FORMAT=json` for structured JSON logs to stderr.
- Run with `-tags=otel` and `OTEL_EXPORTER_OTLP_ENDPOINT=...` to get per-call distributed traces.
- For reproducibility, capture the full `initialize` → `tools/call` transcript from stdin and stderr; the stdio protocol is deterministic apart from time-based tool arguments.
