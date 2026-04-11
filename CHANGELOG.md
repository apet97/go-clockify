# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Changed

### Fixed

### Security

### Deprecated

## [0.7.0] - 2026-04-11

Wave 2 complete. v0.7.0 closes every Wave 2 backlog item (W2-04 through W2-11 plus W2-13, which is subsumed by W2-07). The release ships on a new goreleaser-driven pipeline with parallel FIPS 140-3 binaries, npm distribution under the `@apet97` scope, Kustomize overlays and Helm chart for operators, load and chaos harnesses, and nightly mutation testing via gremlins. Every Wave 1 invariant is preserved — stdlib-only default build (now also with zero OTel rows in `go.mod`), symbol-level nm gates for default/pprof/otel/fips binaries, protocol-core layering, and the `docs/verification.md` cosign + SLSA + SBOM matrix.

### Added

- **W2-11 — FIPS 140-3 build target** (`cmd/clockify-mcp/fips_on.go`, `cmd/clockify-mcp/fips_off.go`, `cmd/clockify-mcp/main.go`, `.goreleaser.yaml`, `.github/workflows/ci.yml`, `docs/adr/011-fips-build-target.md`, `docs/verification.md`). Every tagged release now ships parallel FIPS binaries under the `-tags=fips` build tag, built with `GOFIPS140=latest` so the Go 1.25 native FIPS 140-3 cryptographic module is embedded at compile time. **No cgo, no BoringSSL, no toolchain fork** — the stdlib-only invariant from ADR 001 is preserved for the FIPS variant. A companion startup assertion in `cmd/clockify-mcp/fips_on.go` calls `crypto/fips140.Enabled()` as the very first statement of `main()` and fails the process fatally if the FIPS module is not active; on success it logs `fips140_enabled version=latest enforced=<bool>` via `slog`. Archives are named `clockify-mcp-fips-{os}-{arch}` and land on the GH Release alongside the default binaries with identical cosign signature + SLSA attestation + SBOM coverage. The FIPS matrix is Linux + macOS only (darwin/arm64, darwin/amd64, linux/amd64, linux/arm64) — Windows FIPS available on request. Three new CI gates (build, verify, test) catch any regression toward a non-approved primitive. See [ADR 011](docs/adr/011-fips-build-target.md) and [`docs/verification.md`](docs/verification.md#5-fips-140-3-build-variant-optional).
- **W2-08 — Chaos harness at `tests/chaos/`** (`tests/chaos/main.go`, `tests/chaos/README.md`, `.github/workflows/chaos.yml`). New in-process chaos driver exercising the stdlib-only Clockify HTTP client under five failure-injection scenarios: `429-storm` (Retry-After compliance), `503-burst` (jittered exponential backoff), `mid-body-reset` (connection hijacked and closed mid-body; reader cleanup), `tls-handshake-fail` (self-signed cert; no infinite retry), `dns-fail` (unresolvable `.invalid` hostname; fail-fast). Each scenario asserts error type, attempt count, and elapsed-time bounds. Exits non-zero on regression. Workflow runs on `workflow_dispatch` only. See [`tests/chaos/README.md`](tests/chaos/README.md).
- **W2-09 — Load harness at `tests/load/`** (`tests/load/main.go`, `tests/load/README.md`, `.github/workflows/load.yml`, `internal/ratelimit/ratelimit.go`). New in-process load driver that exercises `ratelimit.RateLimiter.AcquireForSubject` — the same entry point `enforcement.Pipeline.BeforeCall` uses in production — under four scenarios: `steady`, `burst`, `tenant-mix`, and `per-token-saturation`. The `per-token-saturation` scenario is the W2-09 acceptance gate: the noisy tenant is expected to exhaust its per-token budget while quiet tenants keep flowing at 100% success. Harness encodes the assertion explicitly and `log.Fatal`s on isolation regression. A new public method `RateLimiter.SetPerTokenLimits(maxConcurrent, maxPerWindow)` lets programmatic consumers configure the per-subject sub-layer without mutating env vars. `.github/workflows/load.yml` triggers on `workflow_dispatch` only — never on the PR critical path. See [`tests/load/README.md`](tests/load/README.md).
- **W2-10 — Mutation testing nightly via gremlins.dev** (`.github/workflows/mutation.yml`, `docs/testing/mutation-floors.md`, `Makefile`). New scheduled workflow runs [gremlins](https://gremlins.dev/) against six critical packages daily at 02:00 UTC plus on `workflow_dispatch`. Per-package efficacy floors (40% for jsonschema/enforcement/ratelimit/truncate, 35% for mcp, 30% for tools) are enforced via `gremlins unleash --threshold-efficacy`. Floors are ratcheted up over time, never lowered to paper over regressions. **Tool substitution note:** the plan called for `go-mutesting`, but that tool depends on 2019-era `golang.org/x/tools` and panics inside `go/types` when parsing Go 1.25 source; gremlins uses modern `x/tools` and loads the repo cleanly. See [`docs/testing/mutation-floors.md`](docs/testing/mutation-floors.md). A new `make mutation PKG=<path>` Makefile target runs the same tool locally for triage.
- **Deploy render CI gate** (`.github/workflows/ci.yml`). New `deploy-render` job runs on every push: `kubectl kustomize deploy/k8s/base | kubeconform`, the same against each of the three overlays (`dev`, `staging`, `prod`), `helm lint deploy/helm/clockify-mcp`, and `helm template | kubeconform` twice (defaults + `metrics.serviceMonitor.enabled=true metrics.prometheusRule.enabled=true`). Catches any future regression in the deploy manifests before it lands on main.
- **W2-13 — npm distribution restored under `@apet97` scope** (landed as part of W2-07 below). Six packages ship on every release: `@apet97/clockify-mcp-go` (dispatcher) plus `@apet97/clockify-mcp-go-{darwin-arm64,darwin-x64,linux-x64,linux-arm64,windows-x64}`. Users install the dispatcher via `npm install -g @apet97/clockify-mcp-go`; `optionalDependencies` auto-installs the right platform sibling; the `bin/clockify-mcp.js` shim resolves and exec's the native Go binary. Requires `NPM_TOKEN` repo secret scoped to `@apet97` with automation-token rights before the v0.7.0 tag is pushed.

### Changed

- **W2-07 — Release pipeline migrated to goreleaser** (`.goreleaser.yaml`, `.github/workflows/release.yml`, `scripts/publish-npm.sh`, `npm/package.json.tmpl`, `npm/clockify-mcp-go/package.json`, `npm/clockify-mcp-go/bin/clockify-mcp.js`, `docs/adr/010-goreleaser-migration.md`). The hand-rolled `build-binaries` + `create-release` matrix in `release.yml` is replaced by a single goreleaser-driven job. `.goreleaser.yaml` owns the 5-platform build matrix, archive naming, SBOM generation via syft, cosign keyless signing, `SHA256SUMS.txt`, and the GH Release upload — all with filenames byte-identical to the pre-W2-07 outputs so `docs/verification.md` operator commands work unchanged. SLSA build provenance attestation is handled by a post-goreleaser shell step that copies binaries into `staging/` with release asset names before invoking `actions/attest-build-provenance`, because attestation subject names index by filename. Since goreleaser free does not ship an npm publisher, `scripts/publish-npm.sh` runs after goreleaser and publishes six `@apet97/*` packages from `dist/`. The workflow gracefully no-ops the npm step when `NPM_TOKEN` is absent. Release.yml drops from ~205 lines to ~80. Subsumes W2-13. Deliberately out of scope: container image builds (still handled by `docker-image.yml`), Homebrew tap, release-please automation. See [ADR 010](docs/adr/010-goreleaser-migration.md).
- **W2-04 — Tracing OTel wiring moved into a dedicated Go sub-module** (`internal/tracing/otel/go.mod`, `internal/tracing/otel/go.sum`, `internal/tracing/otel/otel.go`, `cmd/clockify-mcp/otel_on.go`, `cmd/clockify-mcp/otel_off.go`, `cmd/clockify-mcp/main.go`, `go.mod`, `go.work`, `.github/workflows/ci.yml`, `docs/adr/009-tracing-submodule.md`, `docs/adr/001-stdlib-only.md`, `docs/observability.md`). The OpenTelemetry-backed tracer has been moved out of the main module and into a dedicated Go sub-module at `internal/tracing/otel/`. The top-level `go.mod` drops from 28 lines to 7 and carries **zero** `go.opentelemetry.io` rows — closing the Wave 1 deferred trade-off documented in ADR 001. The `init()` auto-register hook is replaced by an exported `Install(ctx) (shutdown, error)` delegated from a new `cmd/clockify-mcp/otel_{on,off}.go` build-tag pair, which mirrors the `pprof_{on,off}.go` template established in W2-02. A new CI gate in the `build` job (`Verify go.mod has zero OpenTelemetry rows`) catches any `go mod tidy` regression. A `go.work` file at the repo root makes the sub-module resolvable for parent-tree `-tags=otel` builds. Default binary symbol count is unchanged (0 OTel symbols); `-tags=otel` binary symbol count is unchanged (2077 OTel symbols). **Developer note:** running `go mod tidy` on the main module will re-add the OTel transitive deps as `// indirect` rows because Go 1.17+ lazy-loading requires the main module to list transitively reachable modules — follow with `git restore go.mod` to undo. See [ADR 009](docs/adr/009-tracing-submodule.md).

### Fixed

### Security

### Deprecated

## [0.6.1] - 2026-04-11

Re-cut of 0.6.0 that closes the three release-infra gaps exposed by the 0.6.0 tag run. The binaries, Docker image, cosign signatures, SLSA attestations, and SBOMs for 0.6.1 carry exactly the same protocol semantics as 0.6.0 plus the four Wave 2 items below (W2-01, W2-02, W2-03, W2-12). 0.6.0's git tag remains in history; 0.6.1 is the supported release for new installs.

### Added

- **W2-01 — Runtime JSON-schema validation at the enforcement boundary** (`internal/jsonschema/validator.go`, `internal/jsonschema/validator_test.go`, `internal/mcp/types.go`, `internal/mcp/server.go`, `internal/mcp/schema_validation_dispatch_test.go`, `internal/enforcement/enforcement.go`, `internal/enforcement/schema_validation_test.go`, `internal/tools/schema_validator_property_test.go`, `docs/adr/008-runtime-schema-validation.md`, `.github/workflows/ci.yml`). New stdlib-only JSON-schema validator package at `internal/jsonschema` wired into `enforcement.Pipeline.BeforeCall` as the first gate — before policy, rate-limiting, and dry-run. Every incoming `tools/call` is now validated against the tool's advertised `InputSchema`; malformed calls are rejected at the enforcement boundary. Supported keyword subset: `type`, `required`, `additionalProperties: false`, `properties` (recursive), `items`, `minimum`/`maximum`, `minLength`/`maxLength`, `pattern` (anchored), `format: date`/`date-time`, `enum`. Deliberately out of scope: `$ref`, `$defs`, `allOf`/`anyOf`/`oneOf`, `not`, conditionals, `const`, `exclusiveMinimum`/`exclusiveMaximum`, `multipleOf`, `propertyNames`, `patternProperties`. A new property test `TestRegistrySchemasAcceptHappyPathArgs` walks every Tier 1 + Tier 2 descriptor and synthesises a happy-path argument map, catching walker/validator drift. Coverage: `internal/jsonschema` new at **86.4%** (floor 85%), `internal/enforcement` 89.5% → **89.0%** (floor still 85%), global 66.7% → **67.4%**. See [ADR 008](docs/adr/008-runtime-schema-validation.md).
- **W2-02 — `pprof` endpoints behind `-tags=pprof`** (`internal/mcp/transport_extra.go`, `internal/mcp/server.go`, `internal/mcp/transport_http.go`, `internal/mcp/transport_streamable_http.go`, `internal/mcp/transport_extra_pprof_test.go`, `cmd/clockify-mcp/pprof_on.go`, `cmd/clockify-mcp/pprof_off.go`, `cmd/clockify-mcp/main.go`, `.github/workflows/ci.yml`, `docs/runbooks/oom-or-goroutine-leak.md`). The `net/http/pprof` side-imports are now a first-class build tag that mounts `/debug/pprof/*` on whichever HTTP transport the server is running (legacy `http` or `streamable_http`). Default builds are byte-identical to 0.6.0 — a new CI symbol gate enforces that `go tool nm` on the default binary shows **zero** `net/http/pprof` symbols, and a sibling positive gate enforces that the `-tags=pprof` binary shows at least one. Design: a neutral `ExtraHandler{Pattern, Handler}` type plus a `mountExtras` helper in `internal/mcp/transport_extra.go` (stdlib-only); both transports grew an opt-in slice field (`Server.ExtraHTTPHandlers` for legacy HTTP, `StreamableHTTPOptions.ExtraHandlers` for streamable) that's walked before `ListenAndServe`. `cmd/clockify-mcp/` owns the sole `net/http/pprof` import behind `//go:build pprof`; the default stub returns `nil`. pprof endpoints bypass the `/mcp` bearer gate because they live at a sibling path — debug builds must only run on loopback or behind a firewall. Documented in `docs/runbooks/oom-or-goroutine-leak.md`, which now carries the exact `-tags=pprof` recipe replacing the prior "rebuild manually" note.

### Changed

- **BREAKING (W2-01)**: `tools/call` dispatch now rejects invalid arguments with JSON-RPC `-32602 invalid params` and a JSON Pointer to the offending field in `error.data.pointer` (RFC 6901). Clients that previously relied on silent extra-key acceptance, loose-type coercion, or lax RFC3339 parsing will observe rejections where the handler used to run. Mitigation: read `tools/list` responses and align payloads with each tool's advertised `inputSchema`. The first offending field surfaces in `error.data.pointer`, e.g. `/start` or `/billable`.
- **Metric**: `clockify_mcp_tool_calls_total` gains a new `outcome` label value `invalid_params` distinct from `tool_error`, `rate_limited`, `policy_denied`, `timeout`, `dry_run`, and `cancelled`. Dashboards that `sum by (outcome)` pick up the new dimension automatically.
- **Interface change**: `mcp.Enforcement.BeforeCall` gained a `schema map[string]any` parameter between `hints` and `lookupHandler`. The single production implementation (`enforcement.Pipeline`) and every test stub were updated. Nil schema means "skip validation" — preserving the pre-W2-01 contract for legacy tests.

### Fixed

- **W2-12 — Release infrastructure gaps from the 0.6.0 cut** (`.github/workflows/docker-image.yml`, `.github/workflows/release.yml`, `docs/runbooks/release-incident.md`, `docs/wave2-backlog.md`). The docker-image workflow's top-level `permissions.contents` was bumped from `read` to `write` so `anchore/sbom-action`'s Release-asset upload step can attach the image SBOM to a tag Release. On 0.6.0 this step failed with `Resource not accessible by integration` and no SBOM reached the GH Release asset list; the 0.6.1 Docker-tag run completes this step cleanly. A new runbook at `docs/runbooks/release-incident.md` documents the two canonical partial-release failure modes (`ENEEDAUTH`, `Resource not accessible by integration`), diagnosis commands, and the rerun-vs-re-cut decision tree so on-call has a playbook for the next breakage.
- **W2-12 (continued) — Broken `publish-npm` job removed**. Investigating the 0.6.0 `ENEEDAUTH` surfaced three independent problems in the `publish-npm` job that made it impossible to run as written: `NPM_TOKEN` was never set, the hard-coded `@anycli` scope is not controlled by this project (`npm view @anycli/clockify-mcp-go` returns 404 — the package has never existed on npm), and the `Publish base package` step referenced `npm/clockify-mcp-go/` which has no directory on disk. The job was deleted rather than papered over, with a comment at the deletion site listing the checklist for a rebuild. `npm/package.json.tmpl` is kept as a starting point. The entire npm distribution surface is re-filed as W2-13 in `docs/wave2-backlog.md`. **Note to npm consumers:** 0.6.1 does not ship npm tarballs. Use the GH Release binaries, `go install github.com/apet97/go-clockify/cmd/clockify-mcp@v0.6.1`, or `ghcr.io/apet97/go-clockify:0.6.1` (note: no `v` prefix — `docker/metadata-action` uses `{{version}}` which strips it).

### Security

- **W2-03 — CodeQL Action v3 → v4.35.1** (`.github/workflows/docker-image.yml`). The sole `github/codeql-action/upload-sarif` pin in the repo was on v3, which GitHub began surfacing as a deprecation warning on every Docker-image workflow run. Bumped to `c10b8064de6f491fea524254123dbe5e09572f13 # v4.35.1`. The only v3→v4 delta from the action changelog is a Node.js v24 runtime requirement which GitHub-hosted `ubuntu-22.04` runners already satisfy; `upload-sarif` inputs are unchanged and `sarif_file: trivy-results.sarif` still validates.

### Deprecated

- **npm tarballs for `@anycli/clockify-mcp-go*`** (never shipped; see W2-13 in the backlog). The broken npm distribution wiring was deleted from `release.yml` in the 0.6.1 re-cut. A replacement will ship when W2-13 lands or when W2-07 (goreleaser migration) subsumes it.

## [0.6.0] - 2026-04-11

### Wave 1

- **W1-15 + W1-16 + W1-17 — Documentation polish** (`docs/architecture.md`, `docs/adr/001-stdlib-only.md` .. `docs/adr/007-streamable-http-rewrite.md`, `docs/troubleshooting.md`, `docs/migration/0.5-to-0.6.md`, `README.md`). Closes the Wave 1 documentation tier:
  - `docs/architecture.md` — layer diagram + five mermaid sequence diagrams (tool-call enforcement flow, dry-run interception strategies, Tier 2 activation + list-changed notification, graceful shutdown drain, streamable HTTP session lifecycle including Last-Event-ID replay).
  - `docs/adr/` — seven ADRs with consistent Context / Decision / Consequences / Status shape: `001-stdlib-only` (zero-runtime-deps principle), `002-metrics-exporter` (re-implemented Prometheus text format on `sync/atomic`), `003-enforcement-pipeline` (single gating interface + AfterCall JSON roundtrip), `004-dispatch-semaphore` (the goroutine-cap rationale for `MCP_MAX_INFLIGHT_TOOL_CALLS`), `005-policy-modes` (read_only / safe_core / standard / full mapping to hint flags), `006-multi-tenant-control-plane` (control-plane + per-session runtime factory), `007-streamable-http-rewrite` (2025-03-26 spec adoption + back-compat strategy).
  - `docs/troubleshooting.md` — symptom → diagnosis → fix matrix covering tool-call failures (init guard, policy denial, rate limit, timeout, report cap), transport/auth failures (missing bearer, OIDC audience mismatch, protocol-version mismatch, silent stdio exit), and observability gotchas (scrape failure, high latency, missing OTel traces).
  - `docs/migration/0.5-to-0.6.md` — client-facing delta walking through every 0.6 change in rollout order: Streamable HTTP routing + `/mcp/events` back-compat window, new Resources + Prompts capabilities, progress notifications + per-token rate limiting (with the breaking `scope` label change and a `sum without(scope)` backfill snippet), opt-in OTel build, schema tightening (contract-only, no runtime enforcement yet), operator manifests, runbooks.
  - README updated to link every new page under the Documentation section, and the Wave 1 backlog summary rewritten to reflect the fully-landed state.
- **W1-13 + W1-14 — Observability + manifests** (`docs/observability.md`, `docs/runbooks/high-latency.md`, `docs/runbooks/metrics-scrape-failure.md`, `docs/runbooks/shutdown-drain-timeout.md`, `docs/runbooks/oom-or-goroutine-leak.md`, `deploy/k8s/prometheus-rule.yaml`, `deploy/k8s/servicemonitor.yaml`, `deploy/k8s/README.md`).
  - **Alerting**: `docs/observability.md` gains a multi-window multi-burn-rate alert pair (`ClockifyMCPFastBurn` @ 14.4× / 1h, `ClockifyMCPSlowBurn` @ 6× / 6h) for the 99.9% SLO on `(tool_error|timeout)` as a fraction of non-policy-denied calls, plus the previously-referenced-but-undefined `ClockifyMCPUpstreamUnavailable` critical alert that the `clockify-upstream-outage.md` runbook already pointed at, plus `ClockifyMCPHighLatency` (p99 > 10s for 10m).
  - **Runbooks**: four new runbooks with a consistent Symptom / Triage / Mitigation / Escalation shape — `high-latency.md` (correlates per-tool latency to upstream), `metrics-scrape-failure.md` (ServiceMonitor + Prometheus targets walkthrough), `shutdown-drain-timeout.md` (grace period + in-flight drain semantics), `oom-or-goroutine-leak.md` (pprof procedure, noting the default binary doesn't currently expose pprof — Wave 2 follow-up).
  - **Kubernetes operator manifests**: new `deploy/k8s/prometheus-rule.yaml` mirroring every alert as a `PrometheusRule` CR split across `clockify-mcp.slo` and `clockify-mcp.errors` groups, and `deploy/k8s/servicemonitor.yaml` providing the matching `ServiceMonitor` selecting on the existing `app.kubernetes.io/name: clockify-mcp` label with a 30s scrape interval and a defensive metric-relabel drop for accidental `.*_test_.*` series.
  - `deploy/k8s/README.md` now lists every manifest file and its role.
- **W1-10 — Schema tightening sweep** (`internal/tools/common.go`, `internal/tools/schema_tighten_test.go`). Instead of editing ~100 inline schemas across `registry.go` and the 11 `tier2_*.go` files, a new `tightenInputSchema` walker inside `normalizeDescriptors` recursively mutates every Tier 1 + Tier 2 tool's `InputSchema` in place at registration time:
  - Every `type: "object"` schema (top-level, nested, and array-items) gains `additionalProperties: false` unless the author explicitly set one. Explicit values are preserved — a tool that needs an open shape can still set `additionalProperties: true`.
  - `page` integer properties gain `minimum: 1`.
  - `page_size` integer properties gain `minimum: 1, maximum: 200`.
  - Any `color` string property whose description mentions "Hex" gains `pattern: "^#[0-9a-fA-F]{6}$"`.
  - Any string property whose description mentions "RFC3339" gains `format: "date-time"`.
  - Walks nested `properties` objects and `items` arrays recursively.
  - Two property tests enforce the invariant: `TestRegistrySchemasAllHaveAdditionalPropertiesFalse` walks the full 33-tool Tier 1 registry and `TestTier2SchemasAllHaveAdditionalPropertiesFalse` walks all 11 Tier 2 groups (91 tools), asserting every nested object has `additionalProperties: false`. Two precondition tests (`TestTier1RegistryNonEmpty`, `TestTier2CatalogPopulated`) guard against the property tests becoming vacuous.
  - Contract change only — no runtime JSON-schema validator is wired today (decision point #4 in the Wave 1 plan), so the tightening is advertised to clients but not enforced at dispatch. Follow-up work to enforce validation at the enforcement layer is captured for Wave 2.
  - Coverage: `internal/tools` 52.4% → **52.9%**.
- **W1-12 — OpenTelemetry tracing behind `-tags=otel`** (`internal/tracing/tracing.go`, `internal/tracing/otel.go`, `internal/tracing/tracing_test.go`, `internal/mcp/server.go`, `internal/clockify/client.go`, `.github/workflows/ci.yml`, `docs/observability.md`, `go.mod`, `go.sum`). A new `internal/tracing` package carries a tiny, tag-neutral `Tracer` / `Span` facade with an always-safe no-op implementation. `internal/tracing/otel.go` is behind `//go:build otel` and is the only file in the codebase that imports `go.opentelemetry.io/...`: when compiled in, an `init()` checks `OTEL_EXPORTER_OTLP_ENDPOINT` and — if set — constructs an OTLP HTTP exporter with a default `service.name=clockify-mcp` resource, wires a `sdktrace.TracerProvider`, registers the W3C `propagation.TraceContext` propagator, and replaces `tracing.Default` via `SetDefault`. Failing to construct the exporter falls back silently to the no-op. Two span sites are instrumented: `Server.callTool` opens an `mcp.tools/call` span that carries `tool.name` and the resolved `outcome`, and `Client.doOnce` opens a `clockify.http` span carrying `upstream.endpoint`, `http.method`, and `http.status_code` while also injecting the W3C `traceparent` header into the outbound request. CI now gates the "stdlib-only default" promise: a new `Verify default build has zero OpenTelemetry symbols` step runs `go tool nm` on the default-built binary and fails the job on any `opentelemetry` match, plus a sibling `Build with -tags=otel` + `Test tracing package with -tags=otel` pair exercises the OTLP path. Measured gate: default build = 0 symbols, otel build = ~2k symbols. The tracing package is 100% covered by the no-op test suite. `go.mod` gains OTel rows (`go.opentelemetry.io/otel@v1.43.0` and friends) but the default binary links none of them — this is the accepted trade-off from the plan's decision point #3; the alternative (a separate sub-module) was deferred.
- **W1-03 + W1-07 — Progress notifications + per-token rate limiting** (`internal/mcp/types.go`, `internal/mcp/server.go`, `internal/mcp/progress_token_test.go`, `internal/tools/common.go`, `internal/tools/reports.go`, `internal/tools/reports_progress_test.go`, `internal/authn/context.go`, `internal/authn/context_test.go`, `internal/mcp/transport_streamable_http.go`, `internal/ratelimit/ratelimit.go`, `internal/ratelimit/per_token_test.go`, `internal/ratelimit/testing.go`, `internal/enforcement/enforcement.go`, `internal/enforcement/per_token_test.go`, `internal/metrics/metrics.go`, `internal/metrics/metrics_test.go`, `cmd/clockify-mcp/runtime.go`, `README.md`, `CLAUDE.md`, `docs/observability.md`).
  - **Progress notifications (W1-03)**: `ToolCallParams` and `InitializeParams` gain a `_meta.progressToken` field (typed via a shared `RequestMeta` struct). The `tools/call` dispatcher in `Server.handle` threads the token through the call context via a new `WithProgressToken`/`ProgressTokenFromContext` helper pair. `tools.Service` now carries a `Notifier mcp.Notifier` field which is wired from `cmd/clockify-mcp/runtime.go` to the `*Server` itself — the server gained a public `Notify(method, params)` method that forwards through the currently installed `s.notifier`, so tool handlers emitting notifications will automatically reach whichever transport sink is active (stdio encoder or per-session streamable event hub). A new `Service.EmitProgress(ctx, progress, total, message)` helper publishes `notifications/progress` only when both a token and a notifier are present; it's invoked once per fetched page from `aggregateEntriesRange` with `total=-1` (indeterminate) and a running message like `"fetched N entries"`.
  - **Per-token rate limiting (W1-07)**: the `authn.Principal` landed in Phase C is now attached to the request context at every streamable HTTP auth site via the new `authn.WithPrincipal`/`PrincipalFromContext` helpers. `ratelimit.RateLimiter` gains a lazy per-subject sub-layer: every subject gets its own `subjectLimiter` struct with its own window counter + concurrency semaphore, configured by two new env vars — `CLOCKIFY_PER_TOKEN_RATE_LIMIT` (default `60` calls / 60s window) and `CLOCKIFY_PER_TOKEN_CONCURRENCY` (default `5`). A new method `AcquireForSubject(ctx, subject) (release, scope, err)` runs the existing global acquire path first, then — when the subject is non-empty and the per-token layer is configured — also checks the per-subject sub-limiter, releasing the global slot on sub-layer failure so the global budget is never stranded. `enforcement.Pipeline.BeforeCall` now reads the principal from the request context and routes through `AcquireForSubject`, tagging every rejection on `clockify_mcp_rate_limit_rejections_total` with a new `scope` label (`global` / `per_token`) so operators can tell a noisy-tenant event apart from a global saturation.
  - **Metric label change**: `clockify_mcp_rate_limit_rejections_total` gains a `scope` label; dashboards that `sum(rate(...))` keep working, but per-kind queries must add `scope`. Backfill rule: `sum without(scope) (rate(clockify_mcp_rate_limit_rejections_total[5m]))`.
  - **Tests**: `internal/ratelimit/per_token_test.go` covers per-subject isolation, global-cap enforcement even with per-token permissive, empty-subject passthrough, disabled per-token layer. `internal/enforcement/per_token_test.go` exercises the full pipeline through `Pipeline.BeforeCall` with a real principal context. `internal/authn/context_test.go` covers the round-trip helper. `internal/mcp/progress_token_test.go` asserts that `handle(tools/call)` extracts `_meta.progressToken` and puts it on the call context. `internal/tools/reports_progress_test.go` stubs the notifier and drives a three-page `aggregateEntriesRange` walk asserting exactly three notifications (with correct token, progress counter, and absent `total`), plus the no-notifier and no-token short-circuits. `internal/ratelimit/testing.go` adds a test-only `SetPerTokenLimitsForTest` hook so downstream packages can drive the per-token fields without exporting them.
  - Coverage: `internal/authn` 88.2% → **88.5%**, `internal/enforcement` 88.6% → **89.5%**, `internal/mcp` 71.5% → **71.4%**, `internal/tools` 52.2% → **52.4%**, `internal/ratelimit` 93.8% → **84.4%** (floor 70% still easily clears); global 66.2% → **66.4%**.
- **W1-04 + W1-05 — Resources and Prompts capabilities** (`internal/mcp/resources.go`, `internal/mcp/resources_test.go`, `internal/mcp/prompts.go`, `internal/mcp/prompts_test.go`, `internal/tools/resources.go`, `internal/tools/resources_test.go`, `internal/mcp/server.go`, `cmd/clockify-mcp/runtime.go`). The server now advertises and implements both the `resources` and `prompts` MCP capabilities alongside the existing `tools` capability.
  - **Resources**: new pluggable `mcp.ResourceProvider` interface (List/ListTemplates/Read) implemented by `tools.Service`. Two concrete resources are surfaced for the active workspace (`clockify://workspace/{current}` and `.../user/current`) plus five parametric URI templates: `workspace/{workspaceId}`, `workspace/{workspaceId}/user/{userId}`, `.../project/{projectId}`, `.../entry/{entryId}`, `.../report/weekly/{weekStart}`. `resources/read` dispatches the URI through the real Clockify client and JSON-encodes the result into a `ResourceContents` entry. `resources/subscribe` + `resources/unsubscribe` maintain a `resourceSubscriptions` set; `Server.NotifyResourceUpdated` publishes `notifications/resources/updated` only for subscribed URIs, silently dropping if the notifier is nil. `initialize.result.capabilities.resources = {"subscribe": true, "listChanged": true}` is advertised whenever `Server.ResourceProvider` is non-nil; `cmd/clockify-mcp/runtime.go` wires `tools.Service` as the default provider.
  - **Prompts**: new `promptRegistry` with five built-in templates — `log-week-from-calendar`, `weekly-review`, `find-unbilled-hours`, `find-duplicate-entries`, `generate-timesheet-report` — each carrying a typed `PromptArgument` list with `required` flags. `prompts/list` returns metadata, `prompts/get` applies `{{name}}` substitution into the canned `PromptMessage` sequence and returns `-32602` when a required argument is missing. `initialize.result.capabilities.prompts = {"listChanged": true}` is now always advertised.
  - Tests: mcp-side `resources_test.go` (stub provider, capability advertisement on/off, list/read/subscribe/notify/unsubscribe lifecycle, nil-provider rejection), `prompts_test.go` (list order, substitution, missing-argument rejection, unknown-prompt rejection, capability advertisement), and tools-side `resources_test.go` (real `httptest`-mocked Clockify fetches for workspace/user/project/entry plus malformed URI handling).
  - Coverage: `internal/mcp` 69.7% → **71.5%**; `internal/tools` 52.0% → **52.2%**; global 65.8% → **66.2%**.
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
