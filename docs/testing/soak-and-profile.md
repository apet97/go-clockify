# Soak Testing and Profiling Workflow

This document describes how to perform long-running soak tests and use `pprof` to identify performance bottlenecks and memory leaks in `clockify-mcp-go`.

## Soak Testing

Soak testing involves running the application under expected load for an extended period (e.g., 24–72 hours).

### Preparation
1.  **Deployment:** Deploy a dedicated instance in a staging environment.
2.  **Monitoring:** Ensure Prometheus and Grafana are configured to track memory and CPU usage.
3.  **Load Generation:** Use the `tests/load/` suite or a script to simulate continuous tool calls.

### Key Metrics to Monitor
-   `process_resident_memory_bytes`: Look for a steady upward trend (potential leak).
-   `go_goroutines`: Ensure the count stabilizes and doesn't grow unbounded.
-   `clockify_mcp_tool_call_duration_seconds`: Monitor for latency degradation over time.

## Profiling with `pprof`

`clockify-mcp-go` optionally exposes the standard Go `net/http/pprof` endpoints.

### Enabling `pprof`
The pprof handlers ship behind the `pprof` build tag (see
`cmd/clockify-mcp/pprof_on.go`). The default release binary
**does not** link `net/http/pprof` — verified in CI by the
nm-gate in `.github/workflows/ci.yml`. To enable:

```bash
# Rebuild with the pprof build tag.
go build -tags=pprof ./cmd/clockify-mcp
```

A `pprof_enabled` WARN log fires on startup ("build tag pprof
mounted /debug/pprof/* — do not run in production") so an
operator can confirm the tag took.

### Collecting Profiles
The pprof handlers mount on whichever HTTP transport is active
(stdio binaries are unaffected — pprof requires HTTP). The
endpoints live on the same port as `MCP_HTTP_BIND` (default
`:8080`), NOT on a separate debug port. They are deliberately
unauthenticated — the build-tag gate is the only access
control, so debug builds must only run on trusted networks
(loopback or firewalled). Examples assume `MCP_HTTP_BIND=:8080`:

#### 1. Heap Profile (Memory)
```bash
go tool pprof http://localhost:8080/debug/pprof/heap
```
Use `top` and `list` in the interactive shell to find the largest memory consumers.

#### 2. CPU Profile
```bash
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30
```
This collects a 30-second CPU profile. Use `web` to view a call graph (requires Graphviz).

#### 3. Goroutine Stack Dump
```bash
curl -s http://localhost:8080/debug/pprof/goroutine?debug=1 > goroutines.txt
```
Analyze for any stuck or leaked goroutines.

## Analysis Workflow
1.  **Baseline:** Collect profiles immediately after startup.
2.  **During Load:** Collect profiles at various intervals during the soak test.
3.  **Comparison:** Use `pprof -base` to compare profiles and see exactly where memory grew.

```bash
go tool pprof -base baseline.heap current.heap
```

## Remediation
-   If memory grows in `internal/mcp/`, check for unclosed session contexts.
-   If goroutines grow, check for unhandled cancellation in `internal/clockify/` client calls.
-   If CPU is high in `internal/logging/`, review the redaction logic overhead.
