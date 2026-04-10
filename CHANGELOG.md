# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.5.0] - 2026-04-10

Enterprise-grade production hardening across correctness, safety,
observability, supply chain, and operations.

### Fixed

- **Reports silently capped at 100 entries.** `entryRangeQuery` hardcoded `page-size: 100` and all four report handlers (`SummaryReport`, `WeeklySummary`, `QuickReport`, `DetailedReport`) fetched exactly one page, returning wrong totals at scale. Introduced `aggregateEntriesRange`, a streaming paginator that walks all pages for the requested range and updates totals incrementally. Fails closed with actionable guidance when `include_entries=true` and the range exceeds `CLOCKIFY_REPORT_MAX_ENTRIES` (default 10000). With `include_entries=false`, entries are never stored so memory stays bounded regardless of range size. Replaces free-form `meta.warning` strings with structured `meta.pagination = {page_size, pages_fetched, entries_total}` and `meta.limits = {max_entries}`. Reports gain an optional `max_entries` request parameter for per-call cap override.
- **Truncation violated homogeneous array schemas.** `reduceArrays` appended `{_truncated: true, _remaining: N}` sentinel objects into every truncated array, breaking consumers that expected uniform element types. Arrays now truncate to a prefix with no trailing sentinel; reduction metadata is threaded through a `TruncationReport` and emitted as `result._truncation.array_reductions` with `{path, original_len, new_len, removed}` records (capped at 50 entries).
- **Truncation silently no-op'd on real tool outputs.** `truncate.Truncate` used a type switch that only matched `map[string]any` and `[]any`, but every tool handler returns a typed `ResultEnvelope` struct, so `AfterCall` passed a struct to `Truncate` which fell through the `default` case unchanged. Token budget was silently unenforced for every real tool call. `Pipeline.AfterCall` now JSON-roundtrips the result to a generic value before calling `Truncate`, at the cost of one extra marshal/unmarshal per call (the server marshals again for stdout shortly afterward). Marshal failures fail open.
- **Stdio dispatch spawned unbounded goroutines.** The `Run` loop spawned a goroutine for every `tools/call` request before any limiter ran, creating amplification under bursty input. Added a dispatch-layer semaphore (`MCP_MAX_INFLIGHT_TOOL_CALLS`, default 64) acquired with a context-aware select **before** `go func` is called. Release happens in the goroutine's defer. Context cancellation during acquire exits `Run` cleanly, no deadlocks. Independent of `internal/ratelimit` — one cap gates goroutine creation, the other gates business work.

### Added

- **`internal/metrics` package** — stdlib-only Prometheus text exposition v0.0.4 (Counter, Histogram, Gauge, Registry) backed by `sync.Map` and `atomic.Uint64`. Zero external dependencies. ~570 LOC including a full test suite.
- **`GET /metrics` HTTP endpoint** exposing seven metrics:
  - `clockify_mcp_tool_calls_total{tool, outcome}` — outcome ∈ {success, tool_error, rate_limited, policy_denied, timeout, dry_run}
  - `clockify_mcp_tool_call_duration_seconds{tool}` histogram
  - `clockify_mcp_rate_limit_rejections_total{kind}` — kind ∈ {concurrency, window}
  - `clockify_mcp_http_requests_total{path, method, status}`
  - `clockify_mcp_ready_state` — reads the cached readiness probe, does not trigger upstream calls
  - `clockify_mcp_build_info{version}` — always 1
  - `clockify_mcp_inflight_tool_calls` — samples the dispatch semaphore depth
- **`docs/observability.md`** — SLOs (99.9% availability, 99% tool success, p95<3s/p99<10s latency), SLIs as PromQL, five example alert rules, Prometheus scrape config, and the stable log event taxonomy.
- **`docs/verification.md`** — step-by-step verification of cosign bundles, GitHub build attestations, and SBOM inspection, plus a combined end-to-end recipe.
- **`deploy/k8s/`** — Kubernetes reference manifests: hardened Deployment (non-root, read-only root FS, dropped ALL caps, seccomp RuntimeDefault, runAsUser/Group 65532, resource requests/limits, liveness/readiness/startup probes), Service (ClusterIP on 8080), ConfigMap, Secret template, and README covering quickstart, security posture, observability, scaling, secret rotation, and troubleshooting.
- **`docs/runbooks/`** — three incident runbooks following a consistent template: `rate-limit-saturation.md`, `clockify-upstream-outage.md`, `auth-failures.md`.
- **`Server.InFlightToolCalls()`** and **`Server.IsReadyCached()`** — read-only accessors for observability wiring.
- **`ratelimit.Stats()`** — snapshot of semaphore depth and window counter state.
- **Config fields**: `CLOCKIFY_REPORT_MAX_ENTRIES` (default 10000) and `MCP_MAX_INFLIGHT_TOOL_CALLS` (default 64).
- **SLSA build provenance** via `actions/attest-build-provenance` in the release workflow, SHA-pinned, generating per-artifact attestations verifiable with `gh attestation verify`.

### Changed

- **Build reproducibility**: `-trimpath` added to `Makefile` and the release workflow's per-platform build step. Binaries no longer embed the builder's absolute paths.
- **Cosign signing** switched from `--output-signature`/`--output-certificate` pair to `--bundle <file>.sigstore.json`. The bundle is self-contained (signature, certificate, transparency log entry) and aligns with current sigstore ecosystem defaults.
- **Release workflow permissions** extended with `attestations: write` to support the build provenance step.
- **`Pipeline.AfterCall`** is now an outcome-aware stage, not just a truncation pass-through. Marshal/unmarshal failures fail open with a debug log rather than losing the tool response.
- **`addTruncationWarning` removed** in favor of `paginationMeta`. No code references the vague warning string anymore.

### Tests

Added property tests and targeted coverage for every fix:
- `TestSummaryReport_MultiPage`, `TestWeeklySummary_MultiPage`, `TestDetailedReport_CapExceeded_*`, `TestReports_PaginationMeta`, `TestAggregateEntriesRange_NeverLosesData` (table-driven across N ∈ {0, 1, 199, 200, 201, 400, 599, 600, 999, 1000}).
- `TestReduceArrays`, `TestReduceArrays_ReportPopulated`, `TestReduceArrays_Homogeneous`, `TestTruncate_PropertyArraysStayHomogeneous` (60-iteration property test), `TestAfterCall_TruncatesResultEnvelope`.
- `TestStdioDispatch_BoundedConcurrency`, `TestStdioDispatch_ContextCancelReleases`, `TestStdioDispatch_Unlimited`.
- `TestCounter_Format`, `TestHistogram_Format`, `TestGauge_Format`, `TestWriteTo_DeterministicOrder`, `TestCounter_Concurrent`, `TestLabelEscape`.

All 15 packages pass `go test -race -count=1 -timeout 180s ./...`.

## [0.4.1] - 2026-04-08

### Security

- **Go toolchain bumped to 1.25.9** (`go.mod`, CI, docs). Closes 17 stdlib advisories flagged by `govulncheck` across `crypto/x509`, `crypto/tls`, `net/http`, `net/url`, `encoding/asn1`, and `os` — `GO-2025-4007`..`GO-2025-4013`, `GO-2025-4155`, `GO-2025-4175`, and siblings. `govulncheck` now reports **"No vulnerabilities found."**

### Fixed

- **Lint CI job**: migrated `.golangci.yml` to golangci-lint v2 format (`default: none`, `gosimple` folded into `staticcheck`), bumped `golangci-lint-action` from v6.5.2 → v9.2.0 (SHA-pinned) and linter version from v1.62 → v2.5.0.
- **Lint findings** (12 issues surfaced by v2): `errcheck` on `resp.Body.Close`, `json.Encoder.Encode`, `w.Write`, and the inline RPC error responder in `internal/clockify/client.go` and `internal/mcp/transport_http.go`; `ineffassign` on a dead `explicitRetryAfter` reset; `staticcheck` QF1012 (prefer `fmt.Fprintf`), three QF1001 De Morgan's law flattenings, and SA4004 (dead UTF-8 scan loop in `internal/truncate/truncate_test.go`).
- **Coverage CI job**: scope tests to `./internal/...` so Go doesn't try to instrument the `cmd/clockify-mcp` main package, which previously tripped `go: no such tool "covdata"` on some toolchain installs. Replaced `bc`-based threshold comparison with `awk` for portability and added `set -euo pipefail`.
- **HTTP smoke CI job** (`ci.yml` + `release.yml`): the smoke test used `MCP_BEARER_TOKEN=smoke-test` (10 characters), but the server requires ≥16 since the 0.3.x hardening round, so the server refused to start. Bumped the dummy token to 26 characters and updated the `/ready` assertion to accept 503 (the upstream Clockify call must fail with a dummy API key; the smoke test only verifies the endpoint is reachable).
- **Vulncheck CI job**: install `golang.org/x/vuln/cmd/govulncheck@master` instead of `@latest`. The `v1.1.4` tagged release bundled `go/types` from go1.24 and refused to parse any go1.25 module.
- **Fuzz CI job**: added `timeout-minutes: 8` ceiling so a hung fuzz target can no longer pin the runner for GitHub's default 6-hour budget, and dropped `-fuzztime` from 30s → 20s.

## [0.4.0] - 2026-04-08

### Added

- **Supply-chain hardening**: All GitHub Actions in `ci.yml` and `release.yml` are now pinned to full 40-char commit SHAs with a version comment.
- **golangci-lint in CI**: New `.golangci.yml` (govet, staticcheck, errcheck, ineffassign, unused, gosimple) and a dedicated `lint` CI job.
- **govulncheck in CI**: New `vulncheck` job (soft-fail initially) scans the stdlib for known vulnerabilities on every push.
- **Fuzz testing**:
  - `FuzzParseDatetime` in `internal/timeparse/timeparse_test.go`
  - `FuzzValidateID` in `internal/resolve/resolve_test.go`
  - `FuzzJSONRPCParse` in `internal/mcp/server_test.go`
  - New `fuzz` CI job runs each target for 30s (continue-on-error).
- **List tool pagination**: `clockify_list_projects`, `clockify_list_clients`, `clockify_list_tags`, `clockify_list_tasks`, and `clockify_list_users` now accept `page` and `page_size` parameters (default 1/50, max 200), matching the existing `clockify_list_entries` contract.
- **IdempotentHint annotations**: All read-only Tier 1 and Tier 2 tools now carry `idempotentHint: true` via both the descriptor field and the MCP `Annotations` map. `clockify_stop_timer`, `clockify_update_entry`, and `clockify_find_and_update_entry` are also marked idempotent via the new `toolRWIdem` helper.
- **Dockerfile HEALTHCHECK**: `deploy/Dockerfile` now runs `/usr/local/bin/clockify-mcp --version` every 30s as a distroless-compatible liveness probe.
- **Test coverage investment**: New tests for `WeeklySummary`, `QuickReport`, `DetailedReport` (incl. project filtering), `AddEntry` dry-run, `FindAndUpdateEntry` happy path, `ListClients`/`ListTags`/`ListTasks`/`ListEntries`/`ListUsers` pagination. Total coverage crossed **50%** (up from ~45% at 0.3.0).
- **Coverage threshold raised** from 40% to **50%** in `.github/workflows/ci.yml`.
- **Opt-in live end-to-end testing**: `tests/e2e_live_test.go` is now gated behind the `livee2e` build tag and `CLOCKIFY_RUN_LIVE_E2E=1`, with cleanup for created resources.
- **Client Reliability**: Clockify API client now accurately listens to `Retry-After` HTTP headers on 429 errors.
- **Server Concurrency**: Asynchronous multiplexing inside `stdio` transport using goroutines for `tools/call` requests.
- **Generic Pagination**: Cleanly typed internal API pagination (`ListAll[T any]`) instead of vulnerable map casts.
- **Data Safety**: `server.initialized` is now safeguarded with `atomic.Bool` to prevent read/write lifecycle panics.

### Changed

- **Truncation observability**: `enforcement.Pipeline.AfterCall` now logs a `response_truncated` debug event when progressive token-budget truncation is applied. The previous code silently discarded the `wasTruncated` signal.
- **List handler signatures**: `Service.ListProjects`, `ListClients`, `ListTags`, `ListUsers`, and `ListTasks` now take `args map[string]any` (matching `ListEntries`) to support pagination. This is an internal-only change; MCP tool schemas gained new optional properties.
- **`SECURITY.md`**: added explicit "TLS / HTTP Transport" section documenting the reverse-proxy requirement and the scope of `CLOCKIFY_INSECURE=1`.
- **`docs/safe-usage.md`**: added "HTTP Transport Security" section covering TLS requirements and `CLOCKIFY_INSECURE=1` clarification.

### Fixed

- **Timer Management**: Fixed `clockify_stop_timer` using `http.MethodPost` instead of the required `http.MethodPatch`, ensuring active timers end properly via standards-compliant requests.
- **Tier 2 activation**: `clockify_search_tools` now activates Tier 2 groups and hidden tools through the actual MCP request path and emits `tools/list_changed`.
- **Release packaging**: npm base-package publishing now rewrites `optionalDependencies` to the release version before publish.
- **Validation hardening**: additional path-building handlers now validate external IDs, and webhook URL validation now rejects reserved IP literals and embedded credentials.
- **Runtime controls**: `CLOCKIFY_MAX_CONCURRENT=0` and `CLOCKIFY_RATE_LIMIT=0` now disable only their intended limiter layers.

## [0.3.0] - 2026-04-06

### Added

- **Module path**: Changed to `github.com/apet97/go-clockify` — `go install` now works
- **`--help` flag**: Prints all 25+ environment variables with descriptions
- **`MCP_LOG_LEVEL`**: Configurable log level (`debug`, `info`, `warn`, `error`)
- **Init guard**: `tools/call` before `initialize` returns error code `-32002`
- **`isError` responses**: Tool errors now return `result.isError: true` per MCP spec (not JSON-RPC `error`)
- **Request ID correlation**: Monotonic counter for log entry correlation across tool calls
- **HTTP server timeouts**: `ReadHeaderTimeout` (10s), `ReadTimeout` (30s), `WriteTimeout` (60s), `IdleTimeout` (120s)
- **Security headers**: `X-Content-Type-Options: nosniff` and `Cache-Control: no-store` on all HTTP responses
- **JSON error responses**: HTTP error bodies are now JSON instead of `text/plain`
- **Structured access logging**: HTTP requests logged with method, path, rpc_method, status, req_id, duration_ms
- **`Patch()` method**: Added to Clockify client for Tier 2 tools requiring PATCH
- **Response body limit**: 10MB limit on Clockify API responses to prevent OOM
- **`TestToolCallBeforeInitialize`**: New integration test for the initialization guard

### Changed

- Version bumped to `0.3.0`
- Stdio loop now uses goroutine + `select` on `ctx.Done()` for context-aware shutdown
- HTTP shutdown consolidated in main.go (removed duplicate `signal.NotifyContext` from transport)
- Graceful HTTP shutdown with 10s drain timeout
- Encoder writes protected by `sync.Mutex` for thread-safe notifications
- Replaced deprecated `math/rand.Intn` with `math/rand/v2.IntN` for backoff jitter
- Startup log now includes transport, workspace, and bootstrap mode
- Client User-Agent updated to `clockify-mcp-go/0.3.0`
- Integration tests updated for `isError` response format

### Fixed

- **Rate limiter race condition**: Window reset (`windowStart.Store` + `windowCount.Store(0)`) was not atomic — two goroutines could both see an expired window, both reset, and lose a count. Fixed with `sync.Mutex` + double-checked locking.
- **Stdio shutdown hang**: Server could block indefinitely on `scanner.Scan()` when SIGTERM arrived with idle stdin. Now exits cleanly via context cancellation.
- **Notification errors silently dropped**: `encoder.Encode` errors for `tools/list_changed` are now logged.

### Security

- HTTP error responses now use `Content-Type: application/json` (prevents content-type sniffing)
- Added `X-Content-Type-Options: nosniff` to all HTTP responses
- Added `Cache-Control: no-store` to MCP HTTP responses

## [0.2.0] - 2026-04-06

### Added

- GitHub Actions CI pipeline (format, vet, build, test with race detector, HTTP smoke test)
- GitHub Actions release workflow (5-platform binaries, cosign signing, SBOM, npm publish)
- Docker deployment (distroless multi-stage build, docker-compose, Caddy TLS)
- npm binary distribution (`@anycli/clockify-mcp-go`)
- `--version` flag support
- Signal handling (SIGINT/SIGTERM) for graceful shutdown
- CORS security fix: cross-origin requests rejected by default
- `MCP_ALLOW_ANY_ORIGIN` environment variable for explicit CORS opt-in
- Documentation: tool catalog, safe usage guide, HTTP transport guide, tool annotations
- Example configs: Claude Desktop, Cursor, Docker Compose environment
- Community files: SECURITY.md, CONTRIBUTING.md, LICENSE, issue templates, PR template
- `.env.example` with all configuration options documented
- Dependabot configuration for Go modules

### Changed

- Client User-Agent now uses build version instead of hardcoded `0.1.0`
- Error response body read limited to 64KB (prevents OOM on malicious responses)
- HTTP 502/503/504 now retried with backoff (matching 429/5xx behavior)

### Fixed

- CORS allowing all origins by default when `MCP_ALLOWED_ORIGINS` not set (security fix)

## [0.1.0] - 2026-04-06

### Added

- 124 tools across 11 domain groups (33 Tier 1 + 91 Tier 2)
- Tiered tool loading: core tools at startup, domain groups on demand via `clockify_search_tools`
- Policy modes: `read_only`, `safe_core`, `standard`, `full`
- Dry-run support for destructive tools (3 strategies: confirm, preview, minimal)
- Duplicate entry detection (warn/block/off) + time overlap checking
- Name-to-ID resolution with ambiguity blocking for projects, clients, tags, users, tasks
- Bootstrap modes: `full_tier1`, `minimal`, `custom`
- Token-aware output truncation (progressive: strip nulls, empties, truncate strings, reduce arrays)
- MCP-layer rate limiting (concurrent + per-window) and concurrency control
- Natural language date/time parsing (`now`, `today 14:30`, `yesterday`, ISO 8601)
- ISO 8601 duration parsing (`PT1H30M`)
- Structured audit logging for write operations (`slog` to stderr)
- Stdio and HTTP transports with bearer auth and CORS
- Health (`/health`) and readiness (`/ready`) endpoints for HTTP mode
- Graceful shutdown for HTTP transport (SIGINT/SIGTERM)
- Config validation (API key required, HTTPS enforcement, workspace auto-resolve)
- `tools/list_changed` notifications on Tier 2 group activation
- 5 workflow shortcuts: log time, switch project, weekly summary, quick report, find and update
- ResultEnvelope consistency across all tool handlers (`{ok, action, data, meta}`)
- 265 tests across 13 packages (unit, integration, golden, HTTP transport)

### Security

- API keys via environment variables only
- Constant-time bearer token comparison (`crypto/subtle`)
- ID validation rejects path traversal characters
- Non-HTTPS base URLs blocked unless loopback or `CLOCKIFY_INSECURE=1`
- Zero external dependencies (stdlib only)

[Unreleased]: https://github.com/apet97/go-clockify/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/apet97/go-clockify/compare/v0.4.1...v0.5.0
[0.4.1]: https://github.com/apet97/go-clockify/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/apet97/go-clockify/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/apet97/go-clockify/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/apet97/go-clockify/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/apet97/go-clockify/releases/tag/v0.1.0
