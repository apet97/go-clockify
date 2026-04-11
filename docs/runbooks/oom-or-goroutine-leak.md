# Runbook — OOM or goroutine leak

**Alert**: kubelet OOMKilled events on clockify-mcp pods, or `container_memory_working_set_bytes` growing linearly without release, or sustained goroutine count growth reported by `go_goroutines`.
**Severity**: critical if OOM-kills are happening, warning otherwise.

## Symptom

- A clockify-mcp pod is killed with `OOMKilled` in `kubectl describe pod`.
- Dashboards show `container_memory_working_set_bytes` climbing linearly over hours/days with no release.
- `go_goroutines` gauge climbs monotonically rather than oscillating around a steady value.

## Triage

1. **Confirm the growth pattern**:
   ```
   rate(container_memory_working_set_bytes{container="clockify-mcp"}[1h])
   ```
   A steady positive rate = leak; spikes that drop back down = normal burst behaviour.
2. **Goroutine count**:
   ```
   go_goroutines{job="clockify-mcp"}
   ```
   The stdio dispatch caps goroutines at `MCP_MAX_INFLIGHT_TOOL_CALLS` (default 64). Anything sustainably above that cap is a leak — almost certainly a `context.WithCancel` whose `cancel()` isn't being called, or a goroutine holding an unbuffered channel send forever.
3. **pprof snapshot** — the default binary does **not** link `net/http/pprof`; a CI symbol gate enforces this so production images can never accidentally expose the endpoints. Capturing a profile requires a debug rebuild with the `pprof` build tag, which mounts `/debug/pprof/*` on the same listener that serves `/mcp`:
   ```sh
   # Build with the pprof tag. This is the ONLY supported way to get pprof.
   go build -tags=pprof -o clockify-mcp ./cmd/clockify-mcp

   # Start in HTTP mode (legacy or streamable — both transports wire pprof).
   CLOCKIFY_API_KEY=... \
   MCP_TRANSPORT=http \
   MCP_HTTP_BIND=127.0.0.1:8080 \
   MCP_BEARER_TOKEN=... \
   ./clockify-mcp &

   # Capture — the endpoints are NOT bearer-authed; only run on loopback
   # or behind a firewall.
   go tool pprof http://127.0.0.1:8080/debug/pprof/goroutine
   curl -s 'http://127.0.0.1:8080/debug/pprof/goroutine?debug=1' > /tmp/goroutines.txt
   curl -s  http://127.0.0.1:8080/debug/pprof/heap         > /tmp/heap.pprof
   go tool pprof -top /tmp/heap.pprof
   ```

   Important security note: the pprof handlers bypass the `/mcp` bearer gate
   because they live at a sibling path. Debug builds must only run on
   trusted networks. Never push a `-tags=pprof` image to a registry used by
   production pulls; reuse the existing `clockify-mcp:<sha>` tag for prod.
4. **Correlate with traffic** — growth that scales with tool-call rate and doesn't plateau implies a per-call leak.

## Mitigation

- **Short-term**: rolling-restart the deployment to reset memory. This is a band-aid; the leak will return.
- **Pin the symptom**: if you can identify a tool or workflow that reliably produces the growth, deny it via `CLOCKIFY_DENY_TOOLS` while you investigate.
- **Bound growth**: lower `MCP_MAX_INFLIGHT_TOOL_CALLS` and `CLOCKIFY_MAX_CONCURRENT` to slow the leak rate until a fix ships.

## Root-cause candidates

Based on the repo's typical failure modes:
- An HTTP response body that isn't drained/closed — check recent edits to `internal/clockify/client.go`.
- A leaked ticker/goroutine in a tool handler that holds `context.Background()` or captures an infinite loop without the caller's ctx.
- A stuck upstream call with no client-side timeout — `Client.httpClient` has a 30s default, so this shouldn't happen, but verify in the diff.

## Escalation

- Repeated OOMKills within a single rollout: page on-call and roll back to the previous image.
- goroutine count keeps climbing after rollback: not a code regression — investigate shared state (control plane, JWKS cache).
