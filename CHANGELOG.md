# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

_Nothing yet. Open the next version here._

## [1.0.1] - 2026-04-20

> **Scope note.** The eight days between v1.0.0 and v1.0.1 accumulated
> a large volume of backwards-compatible work — EnvSpec registry,
> Postgres control-plane backend, expanded auth matrix, audit
> retention reaper, transport parity matrix, async gRPC dispatch,
> SSE resume verification, and a full pre-ship gate (`make
> release-check`). No public API changed; tool names, resource URI
> templates, env-var surface, and protocol behaviour remain the v1
> baseline. The patch version reflects the absence of breaking
> changes, not the size of the delta.

### Added

- **Async gRPC Exchange dispatch.** `internal/transport/grpc/transport.go`
  now dispatches each inbound frame in its own goroutine and funnels
  all outbound frames through a single send-pump goroutine. A
  `notifications/cancelled` queued behind an in-flight `tools/call`
  now reaches the dispatcher immediately rather than waiting for the
  blocking handler to return. gRPC rows are re-enabled in the
  cancellation and `tools/list_changed` parity suites, giving those
  contracts full-transport coverage.
- **SSE `Last-Event-ID` resume parity test.** `tests/sse_resume_test.go`
  drives the streamable HTTP server through a drop-and-reconnect
  cycle, proving that `sessionEventHub`'s ring buffer replays the
  exact gap a client missed while disconnected.
- **Raw-send harness primitive + malformed-JSON parity.**
  `Transport.SendRaw` is now part of the `tests/harness` contract
  on stdio, legacy HTTP, streamable HTTP, and gRPC.
  `TestSizeLimit_MalformedJSONParity` sends a deliberately invalid
  frame and asserts every transport surfaces JSON-RPC parse error
  `-32700`. Closes the third boundary the size-limit suite had
  deferred alongside at-limit and over-limit.

- **Structured tool responses (A1).** Every successful `tools/call`
  now emits `structuredContent` alongside the existing text content
  block, validating against the tool's advertised `outputSchema`.
  Old clients that read `content[0].text` keep working unchanged.
- **Full auth matrix on legacy HTTP (A2).** `MCP_TRANSPORT=http` now
  plumbs `authn.Authenticator` (static_bearer / oidc / forward_auth).
  `mtls` on legacy HTTP is rejected at config load with a recovery
  hint (terminate TLS upstream and use `forward_auth`, or use gRPC).
- **SSE GET origin/CORS parity (A3).** `GET /mcp` now applies the
  same `AllowedOrigins` list and CORS headers as `POST /mcp`.
- **Configurable OIDC verify-cache TTL (A4).**
  `MCP_OIDC_VERIFY_CACHE_TTL` replaces the hardcoded 60s ceiling
  (clamped to `[1s, 5m]`). Startup logs a warning when raised above
  the default so the revocation tradeoff is visible.
- **Transport × auth matrix test (A5).** Every supported and
  unsupported combination is locked down at `config.Load()`.
- **Per-subject rate-limiter eviction (B3).** Idle subject entries
  are reaped on a background ticker
  (`CLOCKIFY_SUBJECT_IDLE_TTL`, `CLOCKIFY_SUBJECT_SWEEP_INTERVAL`).
  The subjects map no longer grows unbounded.
- **SSE observability counters (B4).**
  `clockify_mcp_sse_subscriber_drops_total{reason}`,
  `clockify_mcp_sse_replay_misses_total`, and
  `clockify_mcp_sessions_reaped_total{reason}` surface hub / reaper
  eviction reasons that were previously silent.
- **File-store audit cap (B5).** `MCP_CONTROL_PLANE_AUDIT_CAP`
  bounds the in-memory audit slice on the file-backed control
  plane; FIFO eviction keeps dev deployments from growing forever.
- **Fail-closed dev-backend guard (C1).** `streamable_http` refuses
  to start against a `memory`/`file://` control plane unless
  `MCP_ALLOW_DEV_BACKEND=1` acknowledges the single-process limits.
- **Bootstrap + policy drift tests (D1).** Every name in
  `AlwaysVisible`, `MinimalSet`, `Tier1Catalog`, `introspection`,
  and `safeCoreWrites` must resolve to a registered tool.
- **`make verify-bench` Makefile target (D3).** Capture a baseline
  with `make bench BENCH_OUT=.bench/baseline.txt`, then
  `make verify-bench` diffs fresh profiles via `benchstat`.
- **Descriptor-runtime contract tests (D4).** `action` const in
  every outputSchema must match the tool name; Tier 2 descriptors
  must carry `readOnlyHint`/`destructiveHint`/`idempotentHint` in
  their Annotations map.
- **Protocol-version compat suite (E1).** Negotiation, capability
  shape, and dual-emit tools/call are now asserted across
  `2024-11-05`, `2025-03-26`, and `2025-06-18`.
- **ADR 0010 — metrics stack direction (E3, proposed).** Keep the
  homegrown metrics facade for v0.x; revisit with an OTel adapter
  on the ADR 0006 pattern at v1.0.
- **Postgres control-plane backend (B1).** pgx-backed
  `controlplane.Store` implementation lives in a dedicated
  `internal/controlplane/postgres` sub-module behind `-tags=postgres`
  so the default binary stays stdlib-only (ADR 0001). Selected by
  `MCP_CONTROL_PLANE_DSN=postgres://...`; migrations are embedded,
  run under a `pg_advisory_lock`, and version-tracked in a
  `schema_migrations` table. testcontainers-based integration tests
  cover round-trip, migration idempotence, and concurrent writes.
- **Control-plane schema compat guard (E2, ADR 0011).** The applier
  refuses to boot when the database reports a schema newer than the
  embedded migrations, protecting against silent rollback over a
  forward-only change. Integration test plants a bogus version and
  asserts the refuse-to-start error.
- **`RetainAudit(ctx, maxAge)` on Store + retention reaper (B2).**
  `MCP_CONTROL_PLANE_AUDIT_RETENTION` (default 720h, range 1h–8760h,
  0 disables) drives a 1h ticker that drops old audit events from
  both the file store and the Postgres store.
  `clockify_mcp_audit_events_retained_total{outcome="deleted|error"}`
  exposes the per-tick outcome.
- **`internal/runtime` scaffold (C2.1).** Dev-backend predicate,
  control-plane store construction (C1 fail-closed guard included),
  and the retention reaper moved out of `cmd/clockify-mcp` so the
  boot-time plumbing is unit-testable and reusable.
- **Transport dispatch extraction (C2.2).** The streamable_http,
  legacy http, grpc, and stdio arms now live in
  `internal/runtime/{streamable,legacy_http,grpc,grpc_stub,stdio}.go`
  behind `Runtime.Run(ctx)`. `cmd/clockify-mcp/main.go` is a
  ~120-line boot shim (logging, signals, OTel, metrics listener,
  BuildInfo gauge) that delegates the rest. gRPC stays behind
  `//go:build grpc` with a stub for the default binary so the
  ADR 0012 stdlib-only guarantee holds. `auth.go:buildAuthnConfig`
  deduplicates the three previously drifting `authn.Config`
  constructions (grpc had omitted `MTLSTenantHeader` and
  `OIDCVerifyCacheTTL`).

### Added (infrastructure)

- **ADR 0011 — control-plane schema versioning.** Forward-only
  embedded migrations + refuse-to-boot-on-future-schema, with
  `internal/controlplane/COMPAT.md` tracking every version and
  interface addition.
- **`-tags=postgres` CI gate.** `scripts/check-build-tags.sh`
  asserts zero pgx symbols / zero pgx rows in the default build
  and that `-tags=postgres` actually links pgx.
- **`Makefile` targets** `build-postgres` and `test-postgres` for
  the sub-module.

### Changed

- **Legacy HTTP `ServeHTTP` signature** now takes an
  `authn.Authenticator`. Callers that passed only a bearer token
  construct one via `authn.New(authn.Config{Mode: ModeStaticBearer, …})`.
- **`controlplane.Open` accepts options.** Add
  `controlplane.WithAuditCap(n)` to cap the file-backed audit
  slice; back-compat: zero args keeps the historical unbounded
  behaviour.
- **`controlplane.Store` is now an interface** (B1.0). The
  file-backed implementation is renamed to `DevFileStore`; external
  backends (Postgres today) plug in via `RegisterOpener`. Callers
  that typed `*controlplane.Store` switch to the interface type;
  in-package tests type-assert to `*DevFileStore` when they need
  unexported state. A `Close()` method releases backend-owned
  resources (pool, handles); the file store returns nil.

### Docs

- **Runbook rename.** `docs/runbooks/w2-12-digest-pinning.md` is now
  `docs/runbooks/image-digest-pinning.md`. Content unchanged; the
  internal wave label is dropped from the filename and title so the
  runbook reads as a durable operator doc rather than a ticket
  reference. The three callers in `docs/production-readiness.md`,
  `docs/verification.md`, and `docs/verify-release.md` follow.
- Auth × transport matrix in `docs/production-readiness.md` and
  `README.md` now matches the code. mTLS-on-legacy-http is
  documented as rejected; OIDC TTL + dev-backend knobs are
  listed in the main env-var table.
- `docs/production-readiness.md` gains a "Pick a control-plane
  backend" section. `MCP_CONTROL_PLANE_DSN`,
  `MCP_CONTROL_PLANE_AUDIT_CAP`, and
  `MCP_CONTROL_PLANE_AUDIT_RETENTION` are documented in the
  README env-var table.

## [1.0.0] - 2026-04-12

Initial stable release.

> **Stability commitment.** The current API surface — tool names, resource URI templates, configuration env vars, delta-sync wire format, and JSON-RPC protocol behaviour — is now the v1 baseline. No breaking changes will be made without a major version bump. Tier 2 tool groups, the RFC 6902 JSON Patch delta format, and the gRPC transport are considered stable at this release.

### Highlights

- **124 tools** — 33 Tier 1 registered at startup, 91 Tier 2 activated on demand across 11 domain groups.
- **MCP capabilities**: `tools`, `resources` (2 concrete + 6 parametric URI templates), and `prompts` (5 built-in templates).
- **Transports**: stdio (default), streamable HTTP 2025-03-26, legacy POST-only HTTP, and opt-in gRPC (`-tags=grpc`).
- **Auth modes**: `static_bearer`, `oidc`, `forward_auth`, `mtls` — routed via a shared `authn.Authenticator` interface with per-stream validation on gRPC.
- **Four policy modes** (`read_only`, `safe_core`, `standard`, `full`) plus three-strategy dry-run for every destructive tool.
- **Three-layer rate limiting**: stdio dispatch semaphore, per-process concurrency + window limiter, and per-`Principal.Subject` sub-layer.
- **Stdlib-only default build** — the default binary links no OpenTelemetry, gRPC, or protobuf symbols. Verified in CI via `go tool nm`.
- **Opt-in observability**: OpenTelemetry tracing behind `-tags=otel`, Prometheus metrics always on, PII-scrubbed structured logs.
- **Signed releases** — cosign keyless signatures, SPDX SBOMs, and SLSA build provenance on every binary and container image.
- **Reference Kubernetes manifests** — Deployment (non-root distroless, read-only root FS), NetworkPolicy (default-deny), PodDisruptionBudget, ServiceMonitor, and PrometheusRule with multi-window burn-rate alerts for a 99.9% SLO. Helm chart and Kustomize overlays included.

[Unreleased]: https://github.com/apet97/go-clockify/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/apet97/go-clockify/releases/tag/v1.0.0
