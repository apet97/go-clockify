# Runbook: Clockify upstream outage

Triggered when the upstream Clockify API is degraded or down and the
MCP server is returning elevated rates of timeouts, 5xx errors, or
readiness failures.

## Symptoms

- Spike in `clockify_mcp_tool_calls_total{outcome=~"tool_error|timeout"}`.
- Logs contain `clockify_api_error` with HTTP 5xx status codes or
  `context deadline exceeded` messages.
- `GET /ready` starts returning 503 (the readiness check probes
  upstream connectivity).
- Clients report broad tool-call failures across unrelated tools.
- Prometheus alert `ClockifyMCPUpstreamUnavailable` firing.

Example PromQL for error rate by outcome:

```promql
sum by (outcome) (rate(clockify_mcp_tool_calls_total{outcome!="success"}[5m]))
```

## Immediate mitigation

1. Confirm the outage is upstream, not local. Check the Clockify status
   page:

   <https://status.clockify.me/>

2. If the outage is confirmed, reduce write attempts so operators and
   clients are not fighting against broken upstream state. Switch to
   read-only policy:

   ```bash
   kubectl -n clockify-mcp set env deployment/clockify-mcp \
     CLOCKIFY_POLICY=read_only
   ```

3. Announce the incident in your ops/incident channel and pause any
   automated workflows that depend on Clockify writes (reporting jobs,
   scheduled timer starts, etc.).

4. Do not aggressively restart pods. The server already retries 5xx
   and honors `Retry-After`. Restart loops mask the real problem and
   reset the retry backoff state.

## Root cause investigation

- Verify the upstream status page and recent status posts. Clockify
  tends to publish incident postmortems on the same domain.

- Inspect the specific error shape returned by upstream:

  ```bash
  kubectl -n clockify-mcp logs -l app.kubernetes.io/name=clockify-mcp \
    --tail=200 | grep -E 'clockify_api_error|deadline exceeded'
  ```

  Distinguish between:
  - Gateway timeouts (upstream slow, likely transient)
  - 5xx from the API itself (upstream broken)
  - DNS resolution failures (your cluster networking, not upstream)
  - TLS handshake errors (upstream or MITM)

- Run a manual probe from inside the cluster to isolate MCP server
  behavior from upstream behavior:

  ```bash
  kubectl -n clockify-mcp run curl --rm -it --restart=Never \
    --image=curlimages/curl:8.7.1 -- \
    curl -v https://api.clockify.me/api/v1/user
  ```

- Check whether `/ready` recovers when `/health` stays stable — that
  confirms the upstream probe is the discriminator.

## Recovery

- When upstream recovers, unset the temporary policy override:

  ```bash
  kubectl -n clockify-mcp set env deployment/clockify-mcp CLOCKIFY_POLICY-
  kubectl -n clockify-mcp rollout restart deployment/clockify-mcp
  ```

- Confirm readiness and success rate:

  ```bash
  kubectl -n clockify-mcp port-forward svc/clockify-mcp 8080:8080 &
  curl -fsS http://127.0.0.1:8080/ready
  ```

  ```promql
  sum(rate(clockify_mcp_tool_calls_total{outcome="success"}[5m])) /
  sum(rate(clockify_mcp_tool_calls_total[5m]))
  ```

- Post-incident, review whether retry/backoff settings
  (`internal/clockify/client.go`) need tuning for the outage pattern
  you saw, and whether SLO budgets need to be widened.

## Related

- `docs/runbooks/rate-limit-saturation.md` — for local saturation
  cases that can look similar in the aggregate metrics.
- `docs/runbooks/auth-failures.md` — for 401 spikes, which are not
  an upstream outage.
- `CLAUDE.md` — Clockify client design, retry behavior.
