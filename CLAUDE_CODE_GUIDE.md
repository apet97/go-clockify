# CLAUDE_CODE_GUIDE.md

## Mission: Maintain and Extend a Production-Grade Go MCP Server

`GOCLMCP` is now a hardened, production-ready Go MCP server for Clockify. As of v0.3.0, the "hardening" phase is complete. Your mission is to maintain this stability while adding requested high-value features or expanding tool coverage.

---

### v0.3.0 Hardening Recap
The following production features are already implemented—**do not regress these**:
- **MCP Spec Compliance**: Tool errors return `isError: true` in the result (not JSON-RPC errors).
- **Initialization Guard**: All `tools/call` requests are rejected before the `initialize` handshake.
- **Graceful Shutdown**: SIGTERM/SIGINT triggers a 10s drain period for in-flight requests.
- **Race Safety**: Target resources like server `initialized` properties and the rate limiter window are protected by `atomic` boundaries and mutexes.
- **Async Multiplexing**: The `stdio` JSON-RPC transport uses concurrent goroutines synced via `sync.WaitGroup` for tool execution, eliminating IO stall scenarios.
- **Reliable Client Core**: Core HTTP limits depend on strict `Retry-After` header extraction, and paginations are typed securely via `ListAll[T any]`.
- **HTTP Security**: Security headers, JSON error bodies, and configurable timeouts are in place.
- **Observability**: Monotonic request IDs and configurable `MCP_LOG_LEVEL`.
- **Zero Dependencies**: Maintain the "stdlib only" constraint.

---

### Core Architecture

- **`internal/mcp/`**: Server core and JSON-RPC dispatch.
- **`internal/clockify/`**: Hardened HTTP client (retries, body limits, typed errors).
- **`internal/tools/`**: All tool handlers (Tier 1 and Tier 2).
- **Safety Pipeline**: Every call passes through Policy -> Rate Limit -> Dry Run -> Handler -> Truncation.

---

### Rules of the Road

1. **Stdout Purity**: **NEVER** print to stdout except via the MCP encoder. All logs, audit events, and debug info must go to stderr via `slog`.
2. **Fail Closed**: If a resource (project, user, tag) is ambiguous by name, return a tool error with `isError: true`. Do not guess.
3. **Safety First**: Any destructive tool (Delete, Archive) **MUST** support `dry_run: true` and have appropriate policy annotations.
4. **Race Detection**: Always run tests with the race detector: `go test -race ./...`.
5. **No Placeholders**: Use real logic. If an API is missing, implement a pragmatic workaround (like the current report helpers) or reject the task.

---

### Testing Standards

- **Maintain broad test coverage across unit, integration, golden, HTTP transport, and opt-in live E2E tests.**
- **Integration Tests**: `internal/mcp/integration_test.go` verifies the full protocol handshake.
- **Golden Tests**: `internal/tools/golden_test.go` verifies tool schemas and Tier 2 catalogs.
- **Race Safety**: All shared state must be protected by atomics or mutexes.

---

### Next Possible Tasks

1. **Tier 2 Domain Expansion**: Continue adding more domain groups from the Clockify API.
2. **Prometheus Metrics**: Add a `/metrics` endpoint to the HTTP transport.
3. **Interactive Fixes**: Add more "workflow" tools that combine multi-step Clockify actions.
4. **Client Compatibility**: Test and fix quirks for specific MCP clients (Cursor, OpenCLAW, etc).

---

### Useful Commands

```bash
# Build
go build ./...

# Test with race detector
go test -race -count=1 ./...

# Check formatting
gofmt -l .

# Run with log level
MCP_LOG_LEVEL=debug CLOCKIFY_API_KEY=xxx go run ./cmd/clockify-mcp
```

### v1.0 Target Checklist
- [x] Stable Tier 1 set (33 tools)
- [x] Optional Tier 2 activation (91 tools)
- [x] Hardened HTTP transport
- [x] Multi-platform CI/CD release
- [x] Complete documentation set
- [ ] Prometheus metrics (Pending)
- [ ] Homebrew tap (Pending)
