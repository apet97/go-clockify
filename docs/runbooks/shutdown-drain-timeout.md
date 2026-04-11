# Runbook — Shutdown drain timeout

**Alert**: none (event-driven — shows up in pod termination logs as a forced `SIGKILL` after the grace period, or as `shutdown_drain_timeout` slog events).
**Severity**: warning.

## Symptom

A clockify-mcp pod received `SIGTERM` but was killed by `SIGKILL` at the end of `terminationGracePeriodSeconds` instead of exiting cleanly. Some in-flight `tools/call` requests were dropped mid-flight.

## Triage

1. **Confirm the timeout in the pod events**:
   ```sh
   kubectl -n clockify-mcp describe pod <pod-name> | tail -40
   ```
   Look for "Killing container with a grace period of Ns" followed by "Container killed".
2. **Check what was in-flight** — examine the last minute of logs from the terminated pod:
   ```sh
   kubectl -n clockify-mcp logs <pod-name> --previous | tail -100
   ```
   The server logs `http_shutdown` when it starts draining and an `audit` event for every tool call. Compare the `requestSeq` counter immediately before shutdown to the last completed call.
3. **Measure current in-flight at shutdown time**:
   ```
   clockify_mcp_in_flight_tool_calls{pod="<pod-name>"}
   ```
   Spiked right before the pod terminated = caller was queuing work faster than the server could finish it during the drain window.

## Mitigation

- **Raise the grace period**: the default `terminationGracePeriodSeconds` in `deploy/k8s/deployment.yaml` is 30s; if tool calls often take 10–30s, bump it to 60–90s.
- **Lower `MCP_MAX_INFLIGHT_TOOL_CALLS`** during rollouts: fewer in-flight calls means faster drain.
- **Lower `CLOCKIFY_TOOL_TIMEOUT`** (default 45s) if you're OK with rougher per-tool behaviour. The server waits up to this long for any given tool before aborting it during shutdown.
- **Caller-side**: if a sync caller always has N outstanding tool calls, consider using `notifications/cancelled` to abort in-flight work before sending `SIGTERM` to the replica. The server's cancellation map (W1-02) drops the in-flight handler cleanly on a cancel notification.

## Escalation

- Drains still timing out after doubling the grace period: likely a stuck tool handler (upstream Clockify not responding). Check `ClockifyMCPUpstreamUnavailable`.
- Data loss or audit gaps from drops: file a compliance ticket. Shutdown drops are audited with `outcome=cancelled` so the audit log tells you which calls were aborted.
