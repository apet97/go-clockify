# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

GOCLMCP is a production-grade Go MCP (Model Context Protocol) server for Clockify. It exposes 124 Clockify tools (33 Tier 1 tools registered at startup + 91 Tier 2 tools activated on demand) and advertises MCP `resources` (2 concrete + 5 parametric URI templates) and `prompts` (5 built-in templates) capabilities alongside `tools`, over stdio and three HTTP transports (streamable HTTP 2025-03-26, legacy POST-only JSON-RPC, and a back-compat SSE alias). Intended for use with Claude Desktop, Cursor, and similar MCP clients.

## Build / Test / Run

```bash
# Build
go build ./...

# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run a single package's tests
go test ./internal/tools/...
go test ./internal/mcp/...

# Format
gofmt -w ./cmd ./internal ./tests

# Run opt-in live Clockify E2E tests
CLOCKIFY_RUN_LIVE_E2E=1 CLOCKIFY_API_KEY=xxx go test -tags livee2e ./tests

# Run server — stdio mode (default)
CLOCKIFY_API_KEY=xxx go run ./cmd/clockify-mcp

# Run server — HTTP mode
CLOCKIFY_API_KEY=xxx MCP_TRANSPORT=http MCP_BEARER_TOKEN=secret go run ./cmd/clockify-mcp

# Show help and all env vars
go run ./cmd/clockify-mcp --help

# Makefile shortcuts
make check       # fmt + vet + test (CI equivalent)
make cover       # tests with coverage report
```

Go 1.25.9, stdlib only — zero external dependencies. Module path: `github.com/apet97/go-clockify`.

## Environment Variables

### Core
| Variable | Required | Default | Purpose |
|---|---|---|---|
| `CLOCKIFY_API_KEY` | Yes | — | Clockify API key |
| `CLOCKIFY_WORKSPACE_ID` | No | auto-resolve | Workspace ID (auto if only one) |
| `CLOCKIFY_BASE_URL` | No | `https://api.clockify.me/api/v1` | API base URL |
| `CLOCKIFY_TIMEZONE` | No | system | IANA timezone for time parsing (used as default when no per-request timezone is provided) |
| `CLOCKIFY_INSECURE` | No | `0` | Set `1` for non-loopback HTTP |

### Safety & Control
| Variable | Default | Purpose |
|---|---|---|
| `CLOCKIFY_POLICY` | `standard` | `read_only`, `safe_core`, `standard`, `full` |
| `CLOCKIFY_DENY_TOOLS` | — | Comma-separated tool names to block |
| `CLOCKIFY_DENY_GROUPS` | — | Comma-separated Tier 2 groups to block |
| `CLOCKIFY_ALLOW_GROUPS` | — | Whitelist of allowed Tier 2 groups |
| `CLOCKIFY_DRY_RUN` | enabled | Set `off` to disable dry-run |
| `CLOCKIFY_DEDUPE_MODE` | `warn` | `warn`, `block`, `off` |
| `CLOCKIFY_DEDUPE_LOOKBACK` | `25` | Recent entries to check for duplicates |
| `CLOCKIFY_OVERLAP_CHECK` | `true` | Detect overlapping time entries |
| `CLOCKIFY_BOOTSTRAP_MODE` | `full_tier1` | `full_tier1`, `minimal`, `custom` |
| `CLOCKIFY_BOOTSTRAP_TOOLS` | — | Comma-separated tools for custom mode |
| `CLOCKIFY_MAX_CONCURRENT` | `10` | Max concurrent tool calls (business layer, `0` disables) |
| `CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT` | `100ms` | Max time to wait for a concurrency slot before rejecting (`1ms`–`30s`) |
| `CLOCKIFY_RATE_LIMIT` | `120` | Max calls per 60s window (`0` disables this layer) |
| `CLOCKIFY_PER_TOKEN_RATE_LIMIT` | `60` | Max calls per 60s window per authenticated `Principal.Subject` (`0` disables the per-token layer) |
| `CLOCKIFY_PER_TOKEN_CONCURRENCY` | `5` | Max concurrent in-flight calls per `Principal.Subject` (`0` disables) |
| `CLOCKIFY_TOKEN_BUDGET` | `8000` | Token truncation budget (0=off) |
| `CLOCKIFY_TOOL_TIMEOUT` | `45s` | Per-tool-call timeout (5s–10m, Go duration format) |
| `MCP_MAX_INFLIGHT_TOOL_CALLS` | `64` | Stdio dispatch-layer goroutine cap (`0` disables) |
| `CLOCKIFY_REPORT_MAX_ENTRIES` | `10000` | Hard cap on entries aggregated by report tools (`0` disables) |

### Transport
| Variable | Default | Purpose |
|---|---|---|
| `MCP_TRANSPORT` | `stdio` | `stdio` or `http` |
| `MCP_HTTP_BIND` | `:8080` | HTTP listen address |
| `MCP_BEARER_TOKEN` | — | Required for HTTP mode (`Authorization: Bearer <token>`) |
| `MCP_ALLOWED_ORIGINS` | — | Comma-separated CORS origins |
| `MCP_ALLOW_ANY_ORIGIN` | — | Set `1` to allow all origins |
| `MCP_STRICT_HOST_CHECK` | — | Set `1` to require Host match `localhost`, `127.0.0.1`, `::1`, or `MCP_ALLOWED_ORIGINS` |
| `MCP_HTTP_MAX_BODY` | `2097152` | Positive max request body (bytes) |
| `MCP_LOG_FORMAT` | `text` | `text` or `json` (stderr) |
| `MCP_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `MCP_RESOURCE_URI` | — | Canonical resource URI for OAuth 2.1 RS mode (RFC 8707). When set, every OIDC token must list this URI in its `aud` claim, and `/.well-known/oauth-protected-resource` is mounted on the streamable HTTP transport. |

## Architecture

```
cmd/clockify-mcp/main.go           Entrypoint — wires subsystems, transport selection
cmd/clockify-mcp/runtime.go        Service + Server construction, ResourceProvider + Notifier wiring
internal/
  config/         Config from env vars, URL validation
  authn/          Auth modes (static/oidc/forward/mTLS), OAuth 2.1 RS metadata, Principal + request-ctx helpers
  enforcement/    Concrete Enforcement + Activator implementations; Principal-aware rate limit routing
  clockify/       HTTP client (Retry-After backoff, generic pagination, typed errors), entity models
  controlplane/   Multi-tenant session + credential store (memory/file-backed)
  mcp/
    server.go                        Stdio JSON-RPC server with enforcement pipeline (async tools/call dispatch)
                                     Dispatch-layer goroutine semaphore (MaxInFlightToolCalls) for stdio
                                     Forwarding Notify() so tools layer can publish through the active transport
    types.go                         MCP protocol types; InitializeParams / ToolCallParams carry _meta.progressToken
    resources.go + prompts.go        resources/* and prompts/* handlers + capability advertisement
    transport_http.go                Legacy POST-only HTTP transport (compat, no sessions)
    transport_streamable_http.go     2025-03-26 Streamable HTTP: session manager, per-session hub with Last-Event-ID,
                                     POST + GET multiplexing on /mcp, Mcp-Protocol-Version validation
  tools/
    common.go                        Service struct (with lazy user/workspace cache), ResultEnvelope, EmitProgress,
                                     schema tightening walker (tightenInputSchema)
    registry.go                      Tier 1 tool registration (33 tools)
    reports.go                       Streaming paginator (aggregateEntriesRange) with per-page progress notifications
    resources.go                     ResourceProvider implementation surfacing clockify:// URIs
    {domain}.go                      Domain handlers: users, workspaces, projects, clients, tags, tasks,
                                     entries, timer, reports, workflows, context
    tier2_catalog.go                 Tier 2 group catalog and activation
    tier2_{domain}.go                11 domain files: invoices, expenses, scheduling, time_off,
                                     approvals, shared_reports, user_admin, webhooks,
                                     custom_fields, groups_holidays, project_admin
  policy/         Policy modes (read_only/safe_core/standard/full), group control
  resolve/        Name-to-ID resolution with email detection, ambiguity blocking
  dryrun/         3-strategy dry-run: confirm pattern, GET preview, minimal fallback
  bootstrap/      Tool visibility modes (FullTier1/Minimal/Custom), searchable catalog
  ratelimit/      Three-layer control: global semaphore + global window + per-Principal.Subject sub-layer
  tracing/        Tag-neutral Tracer/Span facade; noop default; //go:build otel installs OTLP exporter
  truncate/       Schema-stable progressive token-aware output truncation (TruncationReport)
  metrics/        Stdlib-only Prometheus exposition (Counter/Histogram/Gauge + Registry)
  dedupe/         Duplicate entry detection + time overlap checking
  timeparse/      Natural language time parsing ("now", "today 14:30", "2h30m", ISO 8601)
  helpers/        Error message mapping, paginated results, write envelopes
deploy/k8s/       Kubernetes reference manifests (hardened Deployment + ConfigMap + Secret + PrometheusRule + ServiceMonitor)
docs/
  architecture.md      Layer diagram + mermaid sequence flows for every core path
  adr/                 Seven architecture decision records (001..007)
  observability.md     Prometheus metrics reference, SLOs/SLIs, alert rules, tracing, log taxonomy
  troubleshooting.md   Symptom -> diagnosis -> fix matrix
  migration/0.5-to-0.6.md  Wave 1 delta + rollout order
  verification.md      Release artifact verification (cosign bundles, SLSA provenance, SBOM)
  runbooks/            Incident runbooks (7 total): rate-limit saturation, upstream outage,
                       auth failures, high latency, metrics scrape failure, shutdown drain
                       timeout, oom or goroutine leak
```

### Layered Architecture

The server is structured in four clean layers:

1. **Protocol core** (`mcp/`) — pure JSON-RPC/MCP engine with zero domain imports. Delegates all filtering, gating, and post-processing to four pluggable interfaces defined in `mcp/types.go` + `mcp/resources.go`: `Enforcement`, `Activator`, `Notifier`, `ResourceProvider`. Owns a dispatch-layer goroutine semaphore (`MaxInFlightToolCalls`) that bounds stdio `tools/call` concurrency before any goroutine is spawned, so bursty input backpressures via the scanner channel instead of amplifying into unbounded goroutines. Dispatches `initialize`, `ping`, `tools/*`, `resources/*`, `prompts/*`, `notifications/cancelled`, `notifications/initialized`.
2. **Clockify client** (`clockify/`) — stdlib HTTP client with explicit connection pooling, retry/backoff with `Retry-After` compliance, generic `ListAll[T]` pagination, typed errors, `Close()` for clean shutdown, and (under `-tags=otel`) a `clockify.http` span per outbound request with W3C `traceparent` propagation.
3. **Tool surface** (`tools/`) — 33 Tier 1 tools in a single declarative registry, 91 Tier 2 tools across 11 lazy-loaded groups. Report tools use `aggregateEntriesRange`, a streaming paginator that walks all pages for a date range and aggregates totals incrementally so memory stays bounded regardless of range size; fails closed on `CLOCKIFY_REPORT_MAX_ENTRIES` when `include_entries=true`. Also implements `mcp.ResourceProvider` to surface `clockify://` URIs as MCP resources (workspace, user, project, entry, weekly report). Every Tier 1 + Tier 2 input schema is tightened at registration time by `tightenInputSchema` inside `normalizeDescriptors` (`additionalProperties: false`, pagination bounds, `format: date-time` on RFC3339 fields, color hex pattern).
4. **Safety layer** (`enforcement/`) — composes policy, rate limiting, dry-run, truncation, and bootstrap into the `Enforcement` and `Activator` interfaces consumed by the protocol core. Reads `authn.Principal` from the request context so rate limiting can bucket per-`Principal.Subject` via `ratelimit.AcquireForSubject`. `AfterCall` JSON-roundtrips tool results before truncation so the schema-stable walker in `truncate/` actually processes typed `ResultEnvelope` structs (the type-switch-only walker would otherwise no-op on them).

### Server Enforcement Pipeline

Every `tools/call` is gated by the `Enforcement` interface (`enforcement.Pipeline`):
1. **Dispatch semaphore** → acquire `toolCallSem` slot (stdio only) before the goroutine is spawned
2. **Init guard** → reject with `-32002` if `initialize` has not been called (protocol core)
3. **Progress-token extraction** → if `tools/call.params._meta.progressToken` is set, attach it to the call context via `mcp.WithProgressToken` so downstream handlers can emit `notifications/progress` (protocol core)
4. **OTel span start** → under `-tags=otel`, wrap the call in an `mcp.tools/call` span carrying `tool.name` and the eventual `outcome` attribute (protocol core via `tracing.Default`)
5. **`BeforeCall`** → policy check, rate limit acquire via `AcquireForSubject(ctx, principal.Subject)`, dry-run intercept (enforcement layer). Metrics recorded for rejections as `clockify_mcp_rate_limit_rejections_total{kind, scope}` where `scope` is `global` or `per_token`.
6. **Handler dispatch** → call the tool handler with the per-call context timeout (protocol core)
7. **`AfterCall`** → JSON-roundtrip + truncation post-processing (enforcement layer)
8. **Metrics** → `clockify_mcp_tool_calls_total{tool,outcome}` + `clockify_mcp_tool_call_duration_seconds{tool}` histogram. Outcomes: `success`, `tool_error`, `rate_limited`, `policy_denied`, `timeout`, `dry_run`, `cancelled`.
9. **Logging + span.End** → `slog` to stderr with tool name, duration, and request ID (protocol core)

Tool errors return as `result.isError: true` per the MCP spec (not JSON-RPC `error`). Protocol errors (unknown method, invalid JSON, init guard) use JSON-RPC `error`.

### MCP Capability Surface

Beyond `tools`, the server advertises and implements two additional capabilities:

- **`resources`** — `ResourceProvider` interface on `mcp.Server`, implemented by `tools.Service`. Two concrete resources pin the current workspace + current user; five parametric URI templates expose workspace / user / project / entry / weekly report by id. `resources/subscribe` registers a URI in an internal set; `Server.NotifyResourceUpdated(uri)` checks the set and publishes `notifications/resources/updated` through the active notifier only for subscribed URIs. Advertised only when `Server.ResourceProvider` is non-nil.
- **`prompts`** — `promptRegistry` with five built-in templates (`log-week-from-calendar`, `weekly-review`, `find-unbilled-hours`, `find-duplicate-entries`, `generate-timesheet-report`). `prompts/get` performs `{{name}}` substitution and returns `-32602` on missing required arguments. Advertised as `{"listChanged": true}`.

### Tool Tiers

**Tier 1 (33 tools):** Always registered. Visibility controlled by bootstrap mode. Includes CRUD for entries/projects/clients/tags/tasks, timer control, reports, workflows, and context/discovery tools.

**Tier 2 (91 tools, 11 groups):** Registered lazily via `clockify_search_tools` activation. Groups: invoices (12), expenses (10), scheduling (10), time_off (12), approvals (6), shared_reports (6), user_admin (8), webhooks (7), custom_fields (6), groups_holidays (8), project_admin (6).

### Key Design Decisions

See `docs/adr/001..007` for the full rationale behind each. Short version:

- **Stdlib-only default build.** The default `go build` links zero third-party runtime code. Uses `log/slog` for structured logging, `net/http` for HTTP transport, `crypto/subtle` for constant-time auth, `math/rand/v2` for jitter, `sync.Map` + `atomic.Uint64` + CAS loops for the Prometheus exporter. OpenTelemetry is opt-in behind `-tags=otel`; a CI step runs `go tool nm` on the default binary and fails the build if any `opentelemetry` symbol is linked. `go.mod` does carry OTel rows — the binary does not.
- **Layered separation.** Protocol core (`mcp/`) has zero domain imports. All enforcement logic lives in `enforcement/`. The two are connected via `Enforcement`, `Activator`, `Notifier`, and `ResourceProvider` interfaces.
- **Three-layer rate limiting.** Dispatch-layer semaphore (`MCP_MAX_INFLIGHT_TOOL_CALLS`, default 64) bounds goroutine creation in the stdio loop. Business-layer global semaphore + window limiter (`CLOCKIFY_MAX_CONCURRENT`, `CLOCKIFY_RATE_LIMIT`) gate process-wide work. Per-subject sub-layer (`CLOCKIFY_PER_TOKEN_RATE_LIMIT`, `CLOCKIFY_PER_TOKEN_CONCURRENCY`) buckets by `Principal.Subject` so a noisy tenant cannot monopolise the global budget. All three can reject without stranding resources in the others. Rejections are labelled `{kind, scope}` in metrics.
- **Stdout purity.** Protocol responses only on stdout. All logs go to stderr via slog. `/metrics` on HTTP transport is unauthenticated by design; counters carry no sensitive data and network-layer controls should gate scraping.
- **ResultEnvelope.** Every tool returns `{ok, action, data, meta}` via the `ok()` helper in `common.go`. Truncation is schema-stable: arrays stay homogeneous; truncation metadata lives in a side `_truncation` key with `array_reductions` path/original_len/new_len records.
- **Streaming report aggregation.** Report tools walk all pages of a date range via `aggregateEntriesRange` and update totals incrementally. `CLOCKIFY_REPORT_MAX_ENTRIES` (default 10000) fails closed with guidance when `include_entries=true` and the range exceeds the cap.
- **Fail closed.** Ambiguous name resolution errors. Multiple matches are rejected. Destructive tools require policy + dry-run. Report tools fail closed rather than silently truncating totals.
- **Fail fast.** Config validation at startup: HTTPS enforcement on BaseURL, transport validation, timezone validation, bearer token required for HTTP.
- **Lazy caching.** `Service` caches current user and workspace ID with `sync.Mutex` to avoid redundant API calls.
- **Flat package layout.** All Tier 1 and Tier 2 tools live in `package tools` with domain-named files. No sub-packages needed.
- **Context-aware shutdown.** Stdio loop exits cleanly on SIGTERM via goroutine + `select` on `ctx.Done()`. HTTP server uses `ReadHeaderTimeout`/`ReadTimeout`/`WriteTimeout`/`IdleTimeout`.

### Tool Registration

Tier 1: `internal/tools/registry.go` via `Service.Registry()` returning `[]mcp.ToolDescriptor`. Use `toolRO()`, `toolRW()`, `toolDestructive()`.

Tier 2: Each `tier2_*.go` file self-registers via `init()` calling `registerTier2Group()`. Activated at runtime via `clockify_search_tools` -> `Service.ActivateGroup` / `Service.ActivateTool` -> `server.ActivateGroup()` / `server.ActivateTier1Tool()`.

### Clockify Client

`internal/clockify/client.go` — stdlib HTTP client with explicit connection pooling (`http.Transport`), `X-Api-Key` auth, `Retry-After` compliance for 429/502/503/504, exponential backoff with jitter, generic `ListAll[T any]` pagination, typed `APIError`, and `Close()` for clean shutdown. Methods: `Get`, `Post`, `Put`, `Patch`, `Delete`. Response body limited to 10MB.

### Name Resolution

`internal/resolve/` — 24-char hex passthrough, `strict-name-search=true` API queries, case-insensitive matching, email detection for users, actionable error messages with list-tool suggestions.

## Testing

The repo uses unit, integration, golden, HTTP transport, and opt-in live E2E tests. Patterns:

```go
// Mock Clockify API via httptest
client, cleanup := newTestClient(t, handler)
defer cleanup()
svc := New(client, "ws1")
```

Key test files:
- `internal/tools/golden_test.go` — golden tool list (33 Tier 1 names), Tier 2 catalog (11 groups, 91 tools), schema validation, annotation consistency
- `internal/tools/schema_tighten_test.go` — property tests asserting every Tier 1 + Tier 2 object schema carries `additionalProperties: false`, plus the walker's individual rules (bounded pagination, RFC3339 format, color hex pattern)
- `internal/tools/resources_test.go` — real `httptest`-mocked Clockify dispatch for the `ResourceProvider` implementation on every URI template
- `internal/tools/reports_progress_test.go` — fake notifier + three-page walk asserting `notifications/progress` emission count, progress counter, absent `total`, token propagation
- `internal/mcp/integration_test.go` — full MCP handshake, policy filtering, bootstrap modes, truncation, dry-run pipeline, init guard, isError response format
- `internal/mcp/resources_test.go` — stub `ResourceProvider`, capability advertisement on/off, list/read/subscribe/notify/unsubscribe lifecycle
- `internal/mcp/prompts_test.go` — list order, argument substitution, missing-required rejection, unknown-prompt rejection
- `internal/mcp/progress_token_test.go` — `_meta.progressToken` extraction onto call context
- `internal/mcp/server_concurrency_test.go` — bounded dispatch concurrency, context cancel releases, unlimited mode
- `internal/mcp/cancel_test.go` — cancellation map + `notifications/cancelled` wiring
- `internal/mcp/transport_streamable_http_test.go` — Streamable HTTP 2025-03-26: unified `GET /mcp` SSE path, `/mcp/events` back-compat alias, protocol-version mismatch rejection, `Last-Event-ID` replay / future-skip via `subscribeFrom`
- `internal/mcp/transport_http_test.go` — legacy HTTP auth, CORS, health/ready, security headers
- `internal/tools/reports_test.go` — multi-page aggregation, cap fail-closed, pagination meta, data-integrity property test
- `internal/tools/*_test.go` — per-domain handler tests
- `internal/ratelimit/per_token_test.go` — per-subject isolation, global cap respect, anon fallback, disabled sub-layer
- `internal/enforcement/per_token_test.go` — full `BeforeCall` pipeline with a real `authn.Principal` in context
- `internal/authn/context_test.go` — `WithPrincipal` / `PrincipalFromContext` round-trip
- `internal/metrics/metrics_test.go` — Prometheus exposition format, label escaping, concurrency safety (now covering the `scope` label)
- `internal/truncate/truncate_test.go` — schema stability (no array sentinels), homogeneity property test
- `internal/enforcement/enforcement_test.go` — BeforeCall gating, AfterCall truncation on typed ResultEnvelope
- `internal/tracing/tracing_test.go` — noop tracer identity, `SetDefault` fallback to noop, `InjectHTTPHeaders` no-op

## Constraints

- **Stdlib-only default build** — `go build ./...` (no tags) must produce a binary with zero `opentelemetry` symbols. Enforced by the `Verify default build has zero OpenTelemetry symbols` CI step. OTel code only lives inside `//go:build otel` files.
- **No stdout pollution** — protocol responses only on stdout. All logs go to stderr via slog.
- **No fuzzy/guessed destructive updates** — fail closed on ambiguity.
- **Destructive tools must have policy + dry-run + tests.**
- **Typed models for stable entities**; `map[string]any` acceptable for Tier 2.
- **Tool errors** use `isError: true` (MCP spec); protocol errors use JSON-RPC `error`.
- **Arrays must stay homogeneous after truncation** — metadata lives in a side `_truncation` key, never as a sentinel element.
- **Report totals must be accurate at any range size** — reports stream-aggregate across all pages.
- **Stdio dispatch must never spawn unbounded goroutines** — always acquire `toolCallSem` before `go func`.
- **Every object input schema carries `additionalProperties: false`** — enforced by property tests walking the full Tier 1 + Tier 2 surface. The `tightenInputSchema` walker runs in `normalizeDescriptors` so any newly-added tool inherits the constraint automatically.
- **`Principal` must flow via request context** — every HTTP transport auth site calls `r.WithContext(authn.WithPrincipal(...))` before dispatch so `enforcement.Pipeline.BeforeCall` can bucket rate limits per subject.

## Deployment and operations

- [deploy/k8s/](deploy/k8s/) — hardened Kubernetes reference manifests (Deployment, Service, ConfigMap, Secret, NetworkPolicy, PDB, ServiceAccount, ServiceMonitor, PrometheusRule)
- [docs/architecture.md](docs/architecture.md) — layer diagram + mermaid sequence flows
- [docs/adr/](docs/adr/) — seven architecture decision records
- [docs/observability.md](docs/observability.md) — Prometheus metrics, SLOs, burn-rate alert rules, log taxonomy, OTel tracing
- [docs/troubleshooting.md](docs/troubleshooting.md) — symptom -> diagnosis -> fix matrix
- [docs/migration/0.5-to-0.6.md](docs/migration/0.5-to-0.6.md) — Wave 1 delta in rollout order
- [docs/verification.md](docs/verification.md) — release artifact verification (cosign + SLSA + SBOM)
- [docs/runbooks/](docs/runbooks/) — seven incident runbooks: rate-limit saturation, upstream outage, auth failures, high latency, metrics scrape failure, shutdown drain timeout, oom or goroutine leak
