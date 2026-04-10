# Runbook: rate-limit saturation

Triggered when the MCP server's local rate limiter (concurrency or
window) is rejecting calls at a non-trivial rate, or when upstream
Clockify rate limiting is cascading into user-visible errors.

## Symptoms

- Spike in `clockify_mcp_rate_limit_rejections_total{kind=~"concurrency|window"}`.
- Tool responses with the `rate limited:` error prefix.
- Prometheus alert `ClockifyMCPRateLimitSaturation` firing.
- Clients reporting slow or failed MCP `tools/call` responses.
- Logs on stderr containing `rate_limited` or `window full`.

Example PromQL for the rejection rate:

```promql
sum by (kind) (rate(clockify_mcp_rate_limit_rejections_total[5m]))
```

## Immediate mitigation

1. Scale the deployment horizontally to add local concurrency headroom.
   Remember that replicas share the same upstream rate-limit ceiling, so
   this only helps when the bottleneck is local concurrency, not upstream.

   ```bash
   kubectl -n clockify-mcp scale deployment clockify-mcp --replicas=4
   ```

2. Temporarily raise the per-minute window limit. The default is 120
   calls per 60s; 240 is a safe ceiling for most Clockify plans but
   confirm against your upstream quota first.

   ```bash
   kubectl -n clockify-mcp set env deployment/clockify-mcp \
     CLOCKIFY_RATE_LIMIT=240
   ```

3. If the issue is local concurrency contention rather than upstream
   quota, increase the semaphore wait modestly before raising the hard
   concurrency cap:

   ```bash
   kubectl -n clockify-mcp set env deployment/clockify-mcp \
     CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT=250ms
   ```

4. If abuse is suspected (runaway client, tight loop without backoff),
   tighten the policy to block writes while you investigate:

   ```bash
   kubectl -n clockify-mcp set env deployment/clockify-mcp \
     CLOCKIFY_POLICY=safe_core
   ```

## Root cause investigation

- Per-tool call rate:

  ```promql
  topk(10, sum by (tool) (rate(clockify_mcp_tool_calls_total[5m])))
  ```

- HTTP access logs — look for concentrated callers via request ID or
  remote IP:

  ```bash
  kubectl -n clockify-mcp logs -l app.kubernetes.io/name=clockify-mcp \
    --tail=500 | grep -E '"method":"tools/call"'
  ```

- Check for retry storms. A client without exponential backoff will
  hammer the window limit until the server returns success. Look for
  the same tool repeating dozens of times per second from one caller.

- Confirm whether the saturation is local (our rate limiter) or upstream
  (Clockify returning 429). The `kind` label on
  `clockify_mcp_rate_limit_rejections_total` distinguishes local
  concurrency vs. window rejections. Upstream 429s surface as tool
  errors whose message mentions `429`.

## Recovery

- Confirm the rejection rate drops below 1% of call volume:

  ```promql
  sum(rate(clockify_mcp_rate_limit_rejections_total[5m])) /
  sum(rate(clockify_mcp_tool_calls_total[5m]))
  ```

- Revert any temporary env overrides that were intended as short-term
  mitigations:

  ```bash
  kubectl -n clockify-mcp set env deployment/clockify-mcp \
    CLOCKIFY_RATE_LIMIT- CLOCKIFY_POLICY-
  kubectl -n clockify-mcp rollout restart deployment/clockify-mcp
  ```

- If traffic has grown organically, consider making the new limit
  permanent by updating `deploy/k8s/configmap.yaml` and committing
  the change.

## Related

- `docs/runbooks/clockify-upstream-outage.md` — when upstream 5xx is
  the root cause, not saturation.
- `CLAUDE.md` — `CLOCKIFY_RATE_LIMIT`, `CLOCKIFY_MAX_CONCURRENT`,
  `CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT`, `CLOCKIFY_POLICY` env var reference.
