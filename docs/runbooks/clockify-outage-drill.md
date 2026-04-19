# Runbook: Clockify Outage Drill

This runbook describes the procedure for simulating an upstream Clockify API outage to test system resilience and error handling.

## Objective
Simulate various failure scenarios in the Clockify API and verify that `clockify-mcp` handles them gracefully without crashing or leaking sensitive data.

## Simulation Methods

### 1. DNS Failure
Block `api.clockify.me` in the environment's `/etc/hosts`:
```bash
echo "127.0.0.1 api.clockify.me" >> /etc/hosts
```

### 2. Network Latency/Packet Loss
Use `tc` or a proxy to introduce 10s latency to all outbound requests to Clockify.

### 3. API Error Responses (5xx)
Configure a local proxy to return `503 Service Unavailable` for all requests to `api.clockify.me`.

## Drill Scenarios

### Scenario A: Full API Outage
- [ ] Attempt a `clockify_whoami` tool call.
- [ ] Verify that the application returns a clear error message (e.g., `Upstream API Unavailable`).
- [ ] Confirm no panics are logged.
- [ ] Check that `clockify_mcp_upstream_errors_total` metric increments.

### Scenario B: Partial Outage (Read-Only)
- [ ] Simulate 503 for `POST`/`PUT`/`DELETE` and 200 for `GET`.
- [ ] Verify that read-only tools still function.
- [ ] Confirm that write operations fail with informative error messages.

### Scenario C: Rate Limiting
- [ ] Simulate `429 Too Many Requests` from Clockify.
- [ ] Verify that `clockify-mcp` honors any `Retry-After` headers if implemented.
- [ ] Check if the application logs the rate-limiting event at the correct severity.

## Recovery Steps
- [ ] Remove DNS blocks and proxy rules.
- [ ] Verify that the application recovers automatically within its configured timeout/retry window.
- [ ] Check for any stuck goroutines or leaked resources in the `/debug/pprof` endpoints.

## Post-Drill Evaluation
Record the time taken to detect and mitigate the simulated outage. Update any alert thresholds or timeout configurations based on the findings.
