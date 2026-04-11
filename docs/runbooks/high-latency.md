# Runbook — ClockifyMCPHighLatency

**Alert**: `ClockifyMCPHighLatency`
**Severity**: warning
**Trigger**: `histogram_quantile(0.99, rate(clockify_mcp_tool_call_duration_seconds_bucket[10m])) > 10` for 10m.

## Symptom

p99 tool-call latency exceeds 10 seconds for a sustained 10-minute window. Clients experience slow tool responses and may hit the 45s `CLOCKIFY_TOOL_TIMEOUT`. Upstream Clockify may or may not be the cause.

## Triage

1. **Confirm the breach is real and current**:
   ```
   histogram_quantile(0.99,
     sum by (le) (rate(clockify_mcp_tool_call_duration_seconds_bucket[10m])))
   ```
2. **Identify the hot tool(s)**:
   ```
   topk(5, histogram_quantile(0.99,
     sum by (tool, le) (rate(clockify_mcp_tool_call_duration_seconds_bucket[10m]))))
   ```
3. **Upstream correlation** — check upstream Clockify latency:
   ```
   histogram_quantile(0.99,
     sum by (le) (rate(clockify_upstream_request_duration_seconds_bucket[10m])))
   ```
   If upstream p99 is also elevated, the server is propagating slowness from Clockify — proceed to `clockify-upstream-outage.md`.
4. **Concurrency saturation** — check `clockify_mcp_in_flight_tool_calls` and `clockify_mcp_rate_limit_rejections_total{kind="concurrency"}`. Saturation at the dispatch semaphore indicates the server is queuing work.

## Mitigation

- If upstream is healthy but a single tool is hot: the tool is likely walking too many pages. Check `CLOCKIFY_REPORT_MAX_ENTRIES`, and whether the affected tool is `clockify_summary_report` / `clockify_weekly_summary` / `clockify_detailed_report` with `include_entries=true` over a wide date range. Narrow the range or disable entry inclusion.
- If dispatch saturation: raise `MCP_MAX_INFLIGHT_TOOL_CALLS` temporarily and investigate whether a caller is holding open long-running tool calls beyond the 45s timeout.
- If the server just restarted or scaled, allow 2 minutes for the histogram to drain before escalating.

## Escalation

- Persistent p99 > 10s for 30m despite healthy upstream: page on-call.
- Any correlated `ClockifyMCPUpstreamUnavailable` fire: follow `clockify-upstream-outage.md`.
