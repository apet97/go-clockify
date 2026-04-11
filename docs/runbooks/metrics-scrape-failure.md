# Runbook — Prometheus scrape failure

**Alert**: none (absence-of-metrics problem — surfaced by Grafana "no data" panels, `up{job="clockify-mcp"} == 0`, or a missing `ServiceMonitor` target).
**Severity**: warning.

## Symptom

Dashboards that query `clockify_mcp_*` return "No data". `up{job="clockify-mcp"}` is 0 or absent. The server is almost certainly healthy — `/health` and `/ready` return 200 — but Prometheus cannot scrape `/metrics`.

## Triage

1. **Reach the endpoint directly from inside the cluster**:
   ```sh
   kubectl -n clockify-mcp exec -it deploy/clockify-mcp -- \
     wget -qO- http://127.0.0.1:9091/metrics | head -20
   ```
   Replace the port with `MCP_METRICS_BIND` (default `:9091`) or the pod port if metrics share the main bind.
2. **Check the service selectors**:
   ```sh
   kubectl -n clockify-mcp get svc clockify-mcp -o yaml
   kubectl -n clockify-mcp get endpoints clockify-mcp
   ```
   The `endpoints` list must contain at least one pod IP. Empty endpoints mean the service selector doesn't match the pods.
3. **Check the ServiceMonitor** (Prometheus Operator):
   ```sh
   kubectl -n clockify-mcp get servicemonitor clockify-mcp -o yaml
   ```
   The `spec.selector.matchLabels` must match the service labels.
4. **Check Prometheus targets**:
   - Open `prometheus.example.com/targets` (or equivalent).
   - Look for the `clockify-mcp` job. If the endpoint is listed but unhealthy, read the "Last Error" column — it distinguishes DNS, TLS, auth, and HTTP errors.

## Mitigation

- **Missing service**: apply `deploy/k8s/service.yaml`.
- **Missing ServiceMonitor**: apply `deploy/k8s/servicemonitor.yaml`.
- **Wrong port**: verify `MCP_METRICS_BIND` matches the Service port, and that the `ServiceMonitor.endpoints[].port` name matches the Service port name.
- **Auth 401**: `/metrics` is intentionally unauthenticated; if you're getting 401 it means a sidecar or middleware has wrapped the port. Remove the wrapper.
- **Network policy**: check `deploy/k8s/networkpolicy.yaml` permits scrape traffic from the Prometheus namespace.

## Escalation

- Every other clockify-mcp replica is also unscrapeable: suspect Prometheus, not the server.
- Metrics re-appeared but dashboards still show gaps: it's a display/TSDB issue, not a scrape issue; consult your Prometheus operator.
