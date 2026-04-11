# Wave 1 backlog

Wave 0 (the enterprise-readiness sprint) is shipped on `main` as of
`5c69f2c`. The full plan with 250 enumerated items lives at
`/Users/15x/.claude/plans/moonlit-doodling-meadow.md`. This document is
the curated, priority-ordered subset of the plan that should land in Wave
1 — items that move the repo from "enterprise-grade with caveats" to
"enterprise-grade with no asterisks".

Each item names the file paths that need to change and the rough size
("S/M/L"). Pick by capacity and dependency, not by document order.

## Landed (do not re-pick)

- ✅ **W1-02** Cancellation map — `9e6a6ff` series, `internal/mcp/server.go`, `internal/metrics/metrics.go`, `cancel_test.go`. Notifications/cancelled wired end-to-end with `clockify_mcp_cancellations_total{reason}`.
- ✅ **W1-09** `outputSchema` for every tool — Tier 1 + Tier 2 decorated via `tier1OutputSchemas` lookup + `applyOpaqueOutputSchemas`. Stdlib reflection-based generator at `internal/tools/schemagen.go`.
- ✅ **W1-11** `internal/tools` coverage push — 38.9% → 52.0% via four Tier 2 sweep tests (invoices, expenses, groups_holidays, custom_fields).
- ✅ **W1-06** OAuth 2.1 Resource Server completion — pluggable JWKS HTTP client, resource indicator binding, WWW-Authenticate header, `/.well-known/oauth-protected-resource` metadata document, integration test. `internal/authn` 65.9% → 88.2%.
- ✅ **W1-01** Streamable HTTP completion — `GET /mcp` now serves the SSE notification stream with per-event `id:` stamping; clients reconnecting with `Last-Event-ID` receive replay of backlog entries stamped strictly after the supplied id. Non-initialize requests with a present-but-mismatched `Mcp-Protocol-Version` header are rejected with HTTP 400 + JSON-RPC `-32600`, counted under `clockify_mcp_protocol_errors_total{code="protocol_version_mismatch"}`. `GET /mcp/events` kept as a back-compat alias through 0.6 (removed in 0.7). `internal/mcp` 65.5% → 69.7%.
- ✅ **W1-15 + W1-16 + W1-17** Documentation polish — new `docs/architecture.md` with a layer diagram plus five mermaid sequence diagrams (tool-call enforcement flow, dry-run interception, Tier 2 activation, graceful shutdown, streamable HTTP session lifecycle). Seven new ADRs under `docs/adr/`: `001-stdlib-only`, `002-metrics-exporter`, `003-enforcement-pipeline`, `004-dispatch-semaphore`, `005-policy-modes`, `006-multi-tenant-control-plane`, `007-streamable-http-rewrite` — each with Context / Decision / Consequences / Status shape. New `docs/troubleshooting.md` symptom → diagnosis → fix matrix covering tool-call failures, transport/auth failures, observability gotchas, and debug-info capture. New `docs/migration/0.5-to-0.6.md` walking through every 0.6 delta with explicit client action items (routing change, capability additions, metric label change, opt-in tracing, schema tightening, operator manifests) and back-compat windows (`/mcp/events` → 0.7). README linked to all four.
- ✅ **W1-13 + W1-14** Observability + manifests — `docs/observability.md` gains a multi-window multi-burn-rate alert pair (`ClockifyMCPFastBurn` @ 14.4×/1h, `ClockifyMCPSlowBurn` @ 6×/6h) for the 99.9% SLO, plus the previously-referenced-but-undefined `ClockifyMCPUpstreamUnavailable` critical, plus `ClockifyMCPHighLatency` warning. Four new runbooks land under `docs/runbooks/`: `high-latency.md`, `metrics-scrape-failure.md`, `shutdown-drain-timeout.md`, `oom-or-goroutine-leak.md` — each with a consistent Symptom / Triage / Mitigation / Escalation shape. `deploy/k8s/prometheus-rule.yaml` mirrors every alert from observability.md as a Prometheus Operator `PrometheusRule` CR split across `clockify-mcp.slo` and `clockify-mcp.errors` groups. `deploy/k8s/servicemonitor.yaml` provides the matching `ServiceMonitor` selecting on the existing `app.kubernetes.io/name: clockify-mcp` label with a 30s scrape interval and a defensive metric-relabel drop for accidental `.*_test_.*` series.
- ✅ **W1-10** Schema tightening sweep — instead of hand-editing 100+ inline schemas across `registry.go` and 11 `tier2_*.go` files, a new `tightenInputSchema` walker inside `normalizeDescriptors` recursively mutates every Tier 1 + Tier 2 tool's `InputSchema` in place: object schemas gain `additionalProperties: false` unless explicitly set; `page` gains `minimum: 1`; `page_size` gains `minimum: 1, maximum: 200`; any `color` property whose description mentions "Hex" gains the `^#[0-9a-fA-F]{6}$` pattern; any string property whose description mentions "RFC3339" gains `format: "date-time"`. Two property tests (`TestRegistrySchemasAllHaveAdditionalPropertiesFalse`, `TestTier2SchemasAllHaveAdditionalPropertiesFalse`) walk the full 33-tool Tier 1 registry + every Tier 2 group's 91 tools and assert the invariant for every nested object and array-item schema. Two precondition tests (`TestTier1RegistryNonEmpty`, `TestTier2CatalogPopulated`) guard against the property tests becoming vacuous. Schema contract change only — no runtime validator is wired today, so this is advertised not enforced (follow-up captured as decision point #4 in the Wave 1 plan). `internal/tools` 52.4% → **52.9%**.
- ✅ **W1-12** OpenTelemetry tracing behind `-tags=otel` — new `internal/tracing` package carries a tag-neutral `Tracer`/`Span` facade with an always-safe no-op implementation. A tag-gated `otel.go` (`//go:build otel`) installs an OTLP HTTP exporter + W3C trace-context propagator at init time when `OTEL_EXPORTER_OTLP_ENDPOINT` is set. Two span sites (`mcp.tools/call` in `Server.callTool`, `clockify.http` in `Client.doOnce`) attach `tool.name`/`outcome` and `upstream.endpoint`/`http.method`/`http.status_code` attributes. Outbound Clockify requests propagate `traceparent` via `tracing.Default.InjectHTTPHeaders`. A new CI step `Verify default build has zero OpenTelemetry symbols` uses `go tool nm` to enforce that `go build ./...` (no tags) produces a binary with **zero** `opentelemetry.io` symbols; a sibling `Test tracing package with -tags=otel` job exercises the OTLP branch compiles and runs. `internal/tracing` 100% coverage.
- ✅ **W1-03 + W1-07** Progress notifications + per-token rate limiting — `ToolCallParams` / `InitializeParams` gain a `_meta.progressToken` field that `handle()` threads through the call context via `WithProgressToken`. `tools.Service` now carries a `Notifier mcp.Notifier` field wired from `cmd/clockify-mcp/runtime.go` to the `Server` itself (which satisfies the interface via a forwarding `Notify`). `aggregateEntriesRange` emits one `notifications/progress` per fetched page with an indeterminate `total`. The `authn.Principal` landed in Phase C is now attached to the request context via the new `authn.WithPrincipal`/`PrincipalFromContext` helpers at every streamable HTTP auth site. `ratelimit.RateLimiter` gains a per-subject sub-layer configured by new env vars `CLOCKIFY_PER_TOKEN_RATE_LIMIT` (default `60`/window) and `CLOCKIFY_PER_TOKEN_CONCURRENCY` (default `5`), exposed via a new `AcquireForSubject(ctx, subject)` method that first passes through the global layer and then through a lazily-created per-subject `subjectLimiter`. `enforcement.Pipeline.BeforeCall` reads the principal from the request context and routes rejections through `AcquireForSubject`, tagging `clockify_mcp_rate_limit_rejections_total` with a new `scope` label (`global` / `per_token`). Tests cover per-subject isolation, global-cap enforcement, anonymous fallback, empty-subject passthrough, authn context round-trip, and the enforcement pipeline per-subject path. `internal/authn` 88.2% → **88.5%**; `internal/enforcement` 88.6% → **89.5%**; `internal/mcp` 71.5% → **71.4%**; `internal/ratelimit` 93.8% → **84.4%** (floor 70% holds); global 66.2% → **66.4%**.
- ✅ **W1-04 + W1-05** Resources + Prompts capabilities — new pluggable `mcp.ResourceProvider` interface implemented by `tools.Service`, surfacing 2 concrete resources (`clockify://workspace/{current}` + `.../user/current`) and 5 parametric URI templates (workspace / user / project / entry / weekly report). Server dispatches `resources/list`, `resources/read`, `resources/templates/list`, `resources/subscribe`, `resources/unsubscribe`. `NotifyResourceUpdated` is gated by an internal subscription set so only subscribed URIs fire `notifications/resources/updated`. Five built-in prompt templates (`log-week-from-calendar`, `weekly-review`, `find-unbilled-hours`, `find-duplicate-entries`, `generate-timesheet-report`) shipped via a new `promptRegistry` and dispatched through `prompts/list` + `prompts/get` with `{{name}}` argument substitution and required-argument validation. `initialize.result.capabilities` now advertises `resources` (when a provider is wired) and `prompts.listChanged`. `internal/mcp` 69.7% → 71.5%.

---

## Tier 1: protocol completeness

### W1-02 — Cancellation map  (M)

Maintain `map[requestID]context.CancelFunc` keyed off in-flight
`tools/call` requests. Wire `notifications/cancelled` (currently
unhandled in the `handle` switch) to look up the request id, call the
cancel func, and clean up on completion. Add a metric
`clockify_mcp_cancellations_total{reason}` and a test that sends a
cancellation while a slow tool handler is in flight.

**Files**: `internal/mcp/server.go`, new test `internal/mcp/cancel_test.go`.

---

## Tier 2: enterprise auth + multi-tenancy hardening

### W1-06 — OAuth 2.1 Resource Server (real impl)  (L)

The `internal/authn` OIDC path is implemented (signed JWTs verified
against a JWKS) but operates as a generic OIDC validator, not a full
MCP-spec OAuth 2.1 RS. Add:

- `/.well-known/oauth-protected-resource` metadata document
- Resource indicator support
- `WWW-Authenticate: Bearer realm="…", error="invalid_token", error_description="…"` on 401
- Token-binding to the resource URI per the MCP spec
- Acceptance test using a real signed JWT through a test JWKS HTTP fixture
  (this single test will lift `internal/authn` from 65.9% → ~85%)

**Files**: `internal/authn/authn.go`, new `internal/authn/oauth_resource.go`,
new test `internal/authn/oidc_integration_test.go`.

### W1-08 — mTLS deep test + CRL/OCSP  (S)

`mtlsAuthenticator` works against `*tls.ConnectionState.VerifiedChains`
but we don't actually verify chain freshness. Add a CRL / OCSP stapling
hook for clusters with short-lived certs.

---

## Tier 3: tool surface depth

### W1-09 — Tool schema sweep — `outputSchema` for every tool  (L)

The single biggest MCP-spec UX upgrade. Currently every tool returns a
single `text` content block with a JSON-encoded `ResultEnvelope`. With
`outputSchema`, clients can validate and surface typed fields directly.
Most domain handlers already have typed return structs in
`internal/tools/common.go` (`SummaryData`, `WeeklySummaryData`,
`QuickReportData`, `LogTimeData`, …) — wiring is mostly mechanical.

**Files**: `internal/tools/common.go` (helper for `outputSchemaFor[T]()`),
every tool registration site, golden test additions.

### W1-11 — `internal/tools` coverage push  (M)

Currently 38.9% — the biggest drag on global coverage. Add tests for the
top 5 most-used handlers (`clockify_log_time`, `clockify_list_entries`,
`clockify_summary_report`, `clockify_start_timer`,
`clockify_create_project`) using the existing `httptest`-backed
client mocks.

**Files**: new test files under `internal/tools/`.

---

## Out of scope for Wave 1

These were on the original 250-item plan but should explicitly wait for
Wave 2 because they're substantial standalone projects:

- Helm chart + Kustomize overlays (`deploy/helm/`, `deploy/k8s/{base,overlays/*}`)
- Mutation testing in nightly CI
- Load + chaos test harnesses under `tests/load/` and `tests/chaos/`
- FIPS build target
- Reproducible-build verification job
- `release-please` / `goreleaser` migration

## How to use this document in the next session

Open `/Users/15x/.claude/plans/moonlit-doodling-meadow.md` for the
authoritative item list. This file is the curated subset; the plan is
the source of truth. When you finish a Wave 1 item, move it from this
backlog into the `[Unreleased]` section of `CHANGELOG.md` and add a
short test summary if applicable.
