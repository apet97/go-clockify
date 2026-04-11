# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Wave 1

- **W1-01 — Streamable HTTP completion** (`internal/mcp/transport_streamable_http.go`, `internal/mcp/transport_streamable_http_helpers_test.go`, `internal/mcp/transport_streamable_http_test.go`, `docs/http-transport.md`). `GET /mcp` now serves the SSE notification stream per MCP Streamable HTTP 2025-03-26 §3.3, alongside the existing `POST /mcp`. The legacy `GET /mcp/events` path is kept as a deprecated back-compat alias through 0.6 and will be removed in 0.7. Every emitted SSE event carries a monotonically-increasing `id:` line (new `sessionEvent.id` + `sessionEventHub.lastEventID`); reconnecting clients may send `Last-Event-ID: <n>` and the new `sessionEventHub.subscribeFrom` replays only backlog entries stamped strictly after the supplied id — events trimmed from the backlog when it exceeded its cap are unrecoverable per standard SSE best-effort semantics. Non-initialize requests carrying a present-but-mismatched or unsupported `Mcp-Protocol-Version` header are rejected with HTTP 400 + JSON-RPC `-32600` via a new `validateProtocolVersion` helper wired into `streamableRPCHandler`, counted under `clockify_mcp_protocol_errors_total{code="protocol_version_mismatch"}`. Absent header is still accepted for pre-2025-03-26 clients. Four new tests cover `Last-Event-ID` replay/future-skip, the unified `GET /mcp` SSE path (asserting `id:` lines on the wire), the `/mcp/events` back-compat alias, protocol-version mismatch/unsupported rejection, and protocol-version absent back-compat. `internal/mcp` coverage 65.5% → **69.7%**; global 65.1% → **65.8%**.
- **W1-02 — Cancellation map** (`internal/mcp/server.go`, `internal/metrics/metrics.go`). The protocol core tracks in-flight `tools/call` requests in `Server.inflight` keyed by JSON-RPC request id. `handle()` registers a cancellable child context per call and defers unregister + cancel; `notifications/cancelled` looks up the id and aborts the in-flight tool handler. New `clockify_mcp_cancellations_total{reason}` counter (`client_requested`, `timeout`, `context_cancelled`). New cancelled outcome label on `clockify_mcp_tool_calls_total`.
- **W1-09 — `outputSchema` for every tool** (`internal/mcp/types.go`, `internal/tools/schemagen.go`, `internal/tools/output_schemas.go`). `mcp.Tool` gains an `OutputSchema` field per the 2025-06-18 spec. New stdlib reflection-based schema generator (`schemaFor[T]`) walks Go structs and emits JSON Schema (Draft 2020-12 subset) covering string/bool/int/float/`time.Time`/struct/slice/map/pointer/interface, honouring `json:"...,omitempty"` tags. `envelopeSchemaFor[T](action)` wraps typed data in the `ResultEnvelope` shape with `action` bound as a JSON Schema `const` so MCP clients can dispatch on it. `envelopeOpaque(action)` is the open-shape variant for tools whose data is `map[string]any`. `Service.Registry()` decorates every Tier 1 tool via a `tier1OutputSchemas` lookup table; `Service.Tier2Handlers()` decorates every Tier 2 tool with `envelopeOpaque`. Golden test now enforces every Tier 1 descriptor has an `OutputSchema` with the action const matching the tool name.
- **W1-11 — `internal/tools` coverage push** from 38.9% → 52.0%. Four new comprehensive Tier 2 sweep tests (`tier2_invoices_test.go`, `tier2_expenses_test.go`, `tier2_groups_holidays_test.go`, `tier2_custom_fields_test.go`) drive every handler in those four groups (37 handlers total) through happy paths, validation errors, and dry-run/executed branches via `httptest`-mocked Clockify API responses.
- **W1-06 — OAuth 2.1 Resource Server completion** (`internal/authn/`, `internal/config/config.go`, `internal/mcp/transport_streamable_http.go`, `cmd/clockify-mcp/main.go`). The OIDC auth path is now MCP OAuth 2.1 spec-compliant:
  - **Pluggable JWKS HTTP client**: `jwksCache.client` defaults to `http.DefaultClient`. Tests inject `httptest`-backed clients via `Config.HTTPClient`.
  - **Resource indicator binding (RFC 8707)**: new `MCP_RESOURCE_URI` env var. When set, every OIDC token must list the URI in its `aud` claim. Independent of the legacy `OIDCAudience` match.
  - **WWW-Authenticate header (RFC 6750 §3)**: new `authn.WriteUnauthorized` helper emits `Bearer realm="clockify-mcp", error="invalid_token", error_description="..."` on every 401. Streamable HTTP transport's three auth-failure sites route through it instead of plain `writeJSONError`.
  - **`/.well-known/oauth-protected-resource` metadata document (RFC 9728)**: new `authn.ProtectedResourceHandler` returns the unauthenticated metadata endpoint advertising the resource URI, the authorization server (OIDC issuer), and the supported bearer methods. Mounted by `ServeStreamableHTTP` when `StreamableHTTPOptions.ProtectedResource` is non-nil; `main.go` wires it from `authn.Config` automatically when `MCP_RESOURCE_URI` is set.
  - **Integration test** (`internal/authn/oidc_integration_test.go`): end-to-end happy path with a real RSA-2048 key, freshly generated JWKS doc, signed JWT round-tripped through an `httptest` server. Covers tampered signature, missing resource URI in `aud`, wrong issuer, expired `exp`, `nbf` in the future, and the legacy audience-only path. Lifts `internal/authn` from 65.9% → **88.2%**.

**Wave 1 coverage delta**: global 57.2% → **65.1%**, `internal/tools` 38.9% → **52.0%**, `internal/authn` 65.9% → **88.2%**, `internal/mcp` 63.2% → **65.5%**. All per-package floors hold.

### Added

- **MCP protocol version negotiation** — `initialize` now parses `InitializeParams`, negotiates against `SupportedProtocolVersions` (2025-06-18, 2025-03-26, 2024-11-05), echoes back the negotiated version, and records `clientInfo.name`/`clientInfo.version` for log correlation. `serverInfo` carries a human-readable `title`. A new `instructions` field explains Tier 1/Tier 2 discovery, the dry-run idiom, and the four policy modes so agentic clients can self-orient.
- **Transport-aware `tools.listChanged` capability advertisement.** `initialize.result.capabilities.tools.listChanged` is now only advertised on transports that can actually deliver `notifications/tools/list_changed` (stdio today). Legacy HTTP intentionally omits the capability.
- **Pluggable `Notifier` interface** decouples server→client notification delivery from the stdio JSON encoder. `encoderNotifier` is installed by `Run()`; the legacy HTTP POST transport installs `droppingNotifier`, which logs every suppressed notification and increments `clockify_mcp_protocol_errors_total{code="notification_dropped"}` — previously activations on HTTP silently vanished into a nil encoder.
- **Panic recovery** in the stdio dispatch goroutine (`server.Run`) and the HTTP middleware (`observeHTTPH`). Panics produce a structured `panic_recovered` slog event with the recovered value + `debug.Stack()`, increment `clockify_mcp_panics_recovered_total{site}`, and return a tool-error envelope to the client instead of crashing the loop.
- **PII-redacting slog handler** (`internal/logging/redact.go`) wraps every log handler at startup. Recursively scrubs 20 well-known secret key patterns (`authorization`, `api_key`, `bearer`, `token`, `cookie`, `client_secret`, `refresh_token`, …) from both top-level attrs and nested maps/groups. Defence-in-depth layer: hot-path code still avoids logging secrets explicitly, but an accidental header-map log no longer leaks credentials.
- **Full HTTP security header suite** on every `/mcp` response: `Strict-Transport-Security: max-age=31536000; includeSubDomains`, `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, `Permissions-Policy: ()`, in addition to the pre-existing `X-Content-Type-Options: nosniff` and `Cache-Control: no-store`.
- **Validated transport knobs** for `MCP_STRICT_HOST_CHECK` and `CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT`, both parsed in `internal/config` instead of being read ad hoc at the edge.
- **Typed rate-limit errors** in `internal/ratelimit`, allowing `errors.Is` classification in enforcement and protocol paths instead of message-substring matching.
- **Dedicated redaction tests** covering top-level attrs, grouped attrs, nested maps, nested slices, reflect-backed maps/slices, case-insensitive substring matches, and non-sensitive passthrough.
- **Release metadata smoke verification** in the release workflow, asserting `/metrics` exposes the expected `clockify_mcp_build_info{version,commit,build_date,...}` labels.
- **Docker metadata parity**: the image workflow now passes `BUILD_DATE`, aligning OCI labels with embedded binary metadata.
- **HTTP request duration histogram** `clockify_mcp_http_request_duration_seconds{path,method,status}` with buckets tuned for fast JSON-RPC (0.005→10s). `HTTPRequestsTotal.path` is normalized to `{/mcp,/health,/ready,/metrics,/other}` so probe traffic can never blow label cardinality. All mux routes flow through a single `observeHTTPH` middleware that records metrics + panic recovery uniformly.
- **Upstream Clockify client metrics** (`clockify/metrics.go` + instrumentation in `doOnce` / `doJSON`):
  - `clockify_upstream_requests_total{endpoint,method,status}` with status bucketed to `{2xx,3xx,4xx,5xx,error}`
  - `clockify_upstream_request_duration_seconds{endpoint,method}` histogram tuned 0.05→45s
  - `clockify_upstream_retries_total{endpoint,reason}` with reasons `rate_limited|bad_gateway|service_unavailable|gateway_timeout|error`
  - `normalizeEndpoint` collapses 24/32/36-char hex segments to `:id`, bounding the endpoint label to the ~40 distinct Clockify URL templates regardless of traffic volume
- **Go runtime + process metrics** (`internal/metrics/runtime.go`) exposed via `runtime/metrics.Read` (lock-free, no stop-the-world): `go_goroutines`, `go_gomaxprocs`, `go_memstats_heap_{alloc,inuse,released}_bytes`, `go_memstats_sys_bytes`, `go_memstats_stack_inuse_bytes`, `go_gc_runs_total`, `go_info{version}`, `process_start_time_seconds`, `process_resident_memory_bytes`, `process_open_fds` (cached 5s, O(1) between refreshes).
- **`clockify_mcp_build_info`** gauge labels extended to `{version,commit,build_date,go_version}`. `commit` and `buildDate` are set via `-ldflags` (`-X main.commit=... -X main.buildDate=...`) and default to `"unknown"` for local `go build` / `go run`.
- **`clockify_mcp_protocol_errors_total{code}`** counter fires on every JSON-RPC error response (stdio + HTTP paths) keyed by JSON-RPC error code.
- **SLO-aligned histogram buckets** — new `ToolCallBuckets` (0.05→45s with fine resolution at the 3s SLO boundary), `HTTPDurationBuckets` (fast JSON-RPC), and `UpstreamDurationBuckets` (Clockify API).
- **Tool surface annotations**: every one of the 124 tools now carries `openWorldHint: true` (all tools touch the external Clockify API), a derived human-readable `title`, and **explicit** `destructiveHint` / `idempotentHint` bools — previously `toolRW` omitted these fields, causing spec-strict clients to default-assume destructive for all write tools.
- **Enterprise k8s manifests** — `deploy/k8s/networkpolicy.yaml` (default-deny ingress except labelled allowed pods, default-deny egress except DNS + HTTPS), `deploy/k8s/pdb.yaml` (`minAvailable: 1`), `deploy/k8s/serviceaccount.yaml` (dedicated SA with `automountServiceAccountToken: false`).
- **Multi-arch Docker image pipeline** (`.github/workflows/docker-image.yml`): multi-arch buildx (linux/amd64, linux/arm64) via SHA-pinned `docker/build-push-action`, Trivy vulnerability scan fail-on-HIGH-CRITICAL with SARIF upload to CodeQL, cosign keyless OIDC image signing, SPDX SBOM generation + `cosign attest` attachment, `attest-build-provenance` with image digest subject pushed to the registry. Tags generated from `docker/metadata-action` (sha, branch, PR, semver, `latest` on tag).
- **Hardened `deploy/Dockerfile`**: multi-arch build args (`TARGETOS`/`TARGETARCH`), `-trimpath`, three build ldflags (`VERSION`/`COMMIT`/`BUILD_DATE`), full OCI image labels (`title`/`description`/`source`/`licenses`/`version`/`revision`/`created`), `USER 65532:65532` numeric, `STOPSIGNAL SIGTERM`, distroless `:nonroot` base.

### Changed

- **`deploy/k8s/deployment.yaml`** pinned from `ghcr.io/apet97/go-clockify:latest` → `:v0.5.0`, added `terminationGracePeriodSeconds: 30` and `serviceAccountName: clockify-mcp`.
- **Default log format** wrapped in the redacting handler at startup (affects both text and JSON modes).
- **stdio + HTTP transport** share one dispatch-layer goroutine semaphore via `observeHTTPH` instrumentation so concurrency caps are uniform across transports.
- **`Server.callTool`** records `clockify_mcp_protocol_errors_total` on every JSON-RPC error response.
- **Legacy HTTP transport is now truthful about its semantics.** It no longer auto-initializes the server, `/ready` no longer depends on `initialize`, and docs describe it as stateless POST JSON-RPC without server-push notifications.
- **Strict host checking tightened**: `0.0.0.0` is no longer accepted as a Host header, and strict mode now requires non-loopback hosts to be explicitly allowlisted in `MCP_ALLOWED_ORIGINS`.
- **Release binaries now inject all three build metadata fields**: `main.version`, `main.commit`, and `main.buildDate`.
- **CI gates tightened**: `govulncheck` and fuzzing are now blocking, and coverage enforcement moved from one soft global threshold to a global floor plus critical-package floors.

### Removed

- **Repo hygiene pass** — deleted stale planning docs from repo root: `HARDENING_PLAN.md`, `IMPLEMENTATION_PLAN.md`, `IMPLEMENTATION_SUMMARY.md`, `PRODUCTION_PLAN.md`, `PRODUCTION_READINESS_PLAN.md`, `PRODUCTION_REVIEW.md`, `CLAUDE_CODE_GUIDE.md`. Deleted the legacy `RUST MCP/` submodule reference. Retired the `.gitignore` and `.gitmodules` files — the repo now contains only curated content, nothing that needs to be masked.

### Tests

- **`internal/vault/vault_test.go`** — every backend (inline, env, file), every error branch, JSON-payload variants, missing-api_key, fallback workspace/baseURL propagation. **0% → 95.2%**.
- **`internal/controlplane/store_test.go`** — memory + file DSN forms, full PutTenant/PutCredentialRef/PutSession/AppendAuditEvent round-trip with on-disk reload, DeleteSession, missing-id lookups, `resolvePath` parser branches. **0% → 84.1%**.
- **`internal/authn/authn_test.go`** — `New` defaults across every mode (`static_bearer`, `forward_auth`, `mtls`, `oidc`); `staticBearerAuthenticator` constant-time happy + missing/invalid token; `forwardAuthAuthenticator` header propagation; `mtlsAuthenticator` with fabricated `*tls.ConnectionState` (no real handshake); `bearerToken` parser; `decodeJWT` happy + 5 error branches; `validateClaims` issuer/audience/exp/nbf branches; `claimAudience.UnmarshalJSON` for both shapes; `claimString`; `jwkPublicKey` round-trip for RSA + EC + unsupported kty + decode errors; `curveFor`; `hashForAlg` for every supported alg + the unsupported error; `verifyJWT` RSA round-trip with a generated 2048-bit key including tamper-detection. **0% → 65.9%**.
- **`internal/enforcement/clone_test.go`** — `Pipeline.Clone` and `Gate.Clone` nil + deep-copy paths (Policy/Bootstrap must not alias parent); `Gate.OnActivate` marks bootstrap-tracked tools visible; `Gate.IsGroupAllowed` nil-policy default. **80.0% → 88.6%**.
- **`internal/mcp/server_helpers_test.go`** — `toolNameFromRequest` happy + 4 edge cases; `resourceIDs` nil/empty/full coverage; `InFlightToolCalls` nil-sem + active-sem; `IsReadyCached` round-trip; `ActivateTier1Tool` unknown-tool error + happy path with stub notifier; `droppingNotifier.Notify`; `encoderNotifier.Notify` nil-encoder no-op + buffer round-trip; `notifyToolsChanged` drop-with-no-notifier path.
- **`internal/mcp/transport_streamable_http_helpers_test.go`** — `sessionEventHub` backlog replay + cap trimming + slow-subscriber drop + close + cancel-with-double-cancel; `applyHTTPBaselineHeaders`; `addSessionToInitializeResult` non-map passthrough + map merge without input mutation; `randomID`; `stringsTrimSpace`.
- **`internal/mcp/transport_http_helpers_test.go`** — `statusRecorder` WriteHeader + Write-defaults-to-200; `handleMetrics` exposition headers + body prefix; `observeHTTPH` happy path + panic recovery for string/error/struct panic types; `fmtAny` every branch.
- **CI critical-package coverage floors enforced**: `internal/mcp 62%`, `internal/config 78%`, `internal/enforcement 85%`, `internal/ratelimit 70%`, `internal/logging 85%` — all passing alongside the global 55% gate.

| Package | Coverage |
|---|---|
| `internal/logging` | 97.2% |
| `internal/vault` | 95.2% |
| `internal/ratelimit` | 93.8% |
| `internal/truncate` | 92.3% |
| `internal/timeparse` | 90.4% |
| `internal/enforcement` | 88.6% |
| `internal/helpers` | 87.5% |
| `internal/controlplane` | 84.1% |
| `internal/metrics` | 83.3% |
| `internal/dryrun` | 82.9% |
| `internal/resolve` | 80.3% |
| `internal/config` | 78.1% |
| `internal/policy` | 77.2% |
| `internal/bootstrap` | 74.3% |
| `internal/clockify` | 71.9% |
| `internal/authn` | 65.9% |
| `internal/dedupe` | 64.1% |
| `internal/mcp` | 63.2% |
| `internal/tools` | 38.9% |
| **Total** | **57.2%** |

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
