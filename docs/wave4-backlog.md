# Wave 4 backlog — closed

**Opened:** 2026-04-12, immediately after `v0.8.0` was tagged.
**Closed:** 2026-04-12 at `v0.9.0`. Every Wave 4 item listed here landed on
`main` in the v0.9.0 development session. Supersedes:
[`docs/wave3-backlog.md`](wave3-backlog.md) (closed at `v0.8.0`).
The file is kept as an append-only historical record; future work lives in
a subsequent wave backlog that will be opened when the v0.9.x → v1.0.0
cycle begins.

All Wave 4 items landed — see the [Landed](#landed) section below.

## Out of scope for Wave 4 (deferred to Wave 5)

- **Full Helm / Kustomize env-var parity.** Wave 4 T-2 landed `MCP_GRPC_BIND`
  + the gRPC service port only; 22 other env vars that `Config.Load` reads
  are still NOT reachable through either renderer (authn surface,
  control plane, metrics, CORS). Tracked in
  [`docs/audit-chart-vs-config.md`](audit-chart-vs-config.md).
- **RFC 6902 JSON Patch (`format=jsonpatch`) alternative delta format on
  `notifications/resources/updated`.** Wave 4 still ships only RFC 7396
  merge patch. Original plan deferred this from T-4e.
- **Delta-sync for remaining mutation paths.** T-4a/b wired user +
  weekly-report URIs; still out-of-scope:
  - `DeleteEntry` cross-week invalidation (requires a pre-delete GET).
  - `UpdateEntry` where the start moves across ISO weeks (requires
    stashing the pre-update week from the existing fetch-then-update step).
  - Tier 2 user-group mutations (no URI template for groups today).
  - `CreateProject` / `UpdateProject` / `DeleteProject` cache write-through
    (T-4d wired entries only — same pattern, deferred for scope).
- **gRPC forward_auth / mtls auth modes.** Wave 4 T-3 ships `static_bearer`
  and `oidc` via the synthetic `*http.Request` bridge. `forward_auth`
  requires `X-Forwarded-User` / `X-Forwarded-Tenant` headers that the
  synthetic request cannot carry from gRPC metadata without a design
  decision on which metadata keys to map. `mtls` needs real
  `r.TLS.VerifiedChains` which the synthetic bridge inherently can't
  expose — that path requires a dedicated gRPC credentials integration.
- **Per-message re-validation on long-lived gRPC streams.** The T-3
  interceptor fires once per stream open, so OIDC tokens that expire
  mid-stream retain the original principal. ADR 012 §follow-ups.
- **`clockify_mcp_grpc_auth_rejections_total` metric** for interceptor-
  level `codes.Unauthenticated` rejections. Currently metrics only cover
  post-auth enforcement rejections (policy denied, rate limited).
- **Native gRPC health protocol probes** so Kubernetes `readinessProbe`
  can use `grpc: { port: N }` instead of the W4-02 `tcpSocket` fallback.
- **Multi-stream notifier fan-out for the gRPC transport.** Wave 3
  follow-up still deferred — the most recently opened `Exchange` stream
  wins for server-wide broadcasts.
- **Config-parity CI check.** Long-term fix for the env-var drift problem
  surfaced by T-2's audit: diff the code-generated list of `Config.Load`
  env vars against the chart/kustomize surface and fail PRs that add a
  new env var without exposing or explicitly opting out.

## Landed

### W4-01 — Auto-fire reproducibility workflow on release
- **Commit:** `019083e` (`ci: dispatch reproducibility workflow from release.yml (W4-01)`)
- **Files:** `.github/workflows/reproducibility.yml`,
  `.github/workflows/release.yml`
- **Summary:** `.github/workflows/reproducibility.yml` had a
  `release: [published]` trigger but `gh api repos/.../runs?event=release`
  returned zero runs for v0.8.0 — the auto-trigger had never fired. Root
  cause: releases created via `goreleaser-action` use the default
  `GITHUB_TOKEN`, which GitHub suppresses as a downstream workflow
  trigger to prevent infinite recursion. The documented exception is
  `workflow_dispatch` itself. Fix: add a final `gh workflow run
  reproducibility.yml --ref <tag> -f tag=<tag>` step to `release.yml`
  and grant the release job `actions: write` so the dispatch call
  succeeds. Validation deferred to the first v0.9.0 release cut — it
  replaces the former manual dispatch step.
- **Evidence it was broken:** Two reproducibility runs in history (ids
  24292520289 and 24292876258) at 21:58Z and 22:19Z on 2026-04-11, both
  with `event=workflow_dispatch`. The v0.8.0 release event at 22:16Z
  delivered zero auto-triggered runs.

### W4-02 — Expose gRPC transport in Helm + Kustomize
- **Commit:** `30b104d` (`chore(deploy): expose gRPC transport in Helm + Kustomize (W4-02)`)
- **Files:** `deploy/helm/clockify-mcp/values.yaml`,
  `deploy/helm/clockify-mcp/templates/deployment.yaml`,
  `deploy/helm/clockify-mcp/templates/service.yaml`,
  `deploy/k8s/base/deployment.yaml`, `deploy/k8s/base/service.yaml`,
  `docs/architecture.md`, `docs/observability.md`,
  `docs/audit-chart-vs-config.md` (new)
- **Summary:** Helm chart `transport.mode` accepts `"grpc"` in addition
  to `"http"`/`"streamable_http"`. When set, the deployment flips
  `MCP_GRPC_BIND` (default `:9090`), exposes the gRPC container/service
  port instead of `:8080`, and switches liveness/readiness/startup probes
  from `httpGet /health` to `tcpSocket` on the gRPC port (proper gRPC
  health protocol probes are W5 backlog). Kustomize base keeps HTTP as
  the default and documents the gRPC knobs as commented blocks.
  `docs/audit-chart-vs-config.md` enumerates 22 other `Config.Load` env
  vars that are still not reachable through either renderer — W5 backlog.
- **Latent bugs closed:** `values.yaml:9` pinned `image.tag: "0.7.0"`
  which defeated the `| default .Chart.AppVersion` fallback; chart had
  been shipping a stale image since v0.7.1. Fixed to `image.tag: ""`.
  `deploy/k8s/base/deployment.yaml` pinned `v0.5.0`, three releases
  stale; bumped to `v0.8.0`.

### W4-02.1 — Fuzz budget bump + dead-code cleanup
- **Commits:** `cb4e100` (`ci: bump fuzz budget to 30s to absorb corpus
  growth`), `838c6c9` (`chore(tools): remove dead emitEntryAndWeekly
  helper (post-W4-04d)`)
- **Files:** `.github/workflows/ci.yml`, `internal/tools/resources.go`
- **Summary:** Two CI-hygiene fixes surfaced during the Wave 4 session.
  `FuzzJSONRPCParse` flake: the fuzzer was still finding new coverage
  paths at the 20-second budget mark and Go reported the expiring worker
  context as a FAIL. Bumped per-target budget to 30s (90s worst case
  job time, still well under the 8min ceiling). The dead-code commit
  removed `emitEntryAndWeekly` which became unreachable after T-4d
  replaced every call site with `emitEntryAndWeeklyWithState`;
  golangci-lint's unused check flagged it post-merge.

### W4-03 — Native gRPC auth interceptor
- **Commit:** `4c864e9` (`feat(transport): native auth interceptor for gRPC (W4-03)`)
- **Files:** `internal/transport/grpc/auth.go` (new),
  `internal/transport/grpc/auth_test.go` (new),
  `internal/transport/grpc/transport.go`, `internal/config/config.go`,
  `internal/config/config_test.go`, `cmd/clockify-mcp/grpc_on.go`,
  `cmd/clockify-mcp/grpc_off.go`, `cmd/clockify-mcp/main.go`,
  `docs/adr/012-grpc-transport.md`, `CLAUDE.md`
- **Summary:** New stream interceptor bridges `internal/authn` onto gRPC
  metadata via a synthetic `*http.Request` that carries only
  `Authorization: Bearer <token>`. Supports `static_bearer` (reads the
  Authorization header only) and `oidc` (reads Authorization + fetches
  JWKS via the real request context). `forward_auth` and `mtls` remain
  HTTP-only because they need data the synthetic request cannot carry.
  Wrapped stream returns `authn.WithPrincipal(ctx, &principal)` from
  `Context()`, so the existing enforcement pipeline buckets rate limits
  per `Principal.Subject` without changes. 6 interceptor unit tests
  (missing metadata, missing authorization, empty authorization, wrong
  token, happy path principal propagation, authenticator error) plus
  4 config-layer tests (gRPC + each auth mode). ADR 012 amended with a
  new "Auth bridge (W4-03)" section. `CLAUDE.md` env var table updated.
- **Invariant preserved:** `go tool nm /tmp/clockify-mcp-default | grep
  -c 'google.golang.org/grpc' == 0`. Verified locally and by the CI
  "Verify default build has zero gRPC symbols" gate.

### W4-04a — Wire user mutations to delta-sync
- **Commit:** `14d4095` (`feat(resources): wire user mutations to delta-sync (W4-04a)`)
- **Files:** `internal/tools/resources.go`,
  `internal/tools/tier2_user_admin.go`,
  `internal/tools/tier2_user_admin_emit_test.go` (new)
- **Summary:** New `userResourceURI` helper matching the existing
  `clockify://workspace/{ws}/user/{id}` template. Tier 2 `UpdateUserRole`
  and `DeactivateUser` now emit after the Clockify PUT succeeds.
  Group-management mutations (`CreateUserGroup`, `UpdateUserGroup`, etc.)
  not wired because groups have no URI template today — W5 backlog.
  3 new tests covering the URI builder, UpdateUserRole emit, and
  DeactivateUser emit.

### W4-04b — Weekly-report URI fan-out on entry mutations
- **Commit:** `1cab9dd` (`feat(resources): emit weekly-report URI on entry mutations (W4-04b)`)
- **Files:** `internal/tools/resources.go`, `internal/tools/entries.go`,
  `internal/tools/timer.go`, `internal/tools/resource_emit_test.go`,
  `internal/tools/weekly_report_emit_test.go` (new)
- **Summary:** New `weeklyReportResourceURI` + `isoWeekStart` +
  `weeklyReportURIsForEntry` helpers. Every entry-producing handler
  (AddEntry, UpdateEntry, StartTimer, StopTimer) now fans out the emit
  to both the concrete entry URI and the weekly-report URI(s) for the
  ISO week(s) the entry touches. Multi-week spans (e.g. Sun 23:00 →
  Mon 01:00) emit two weekly-report URIs. Wave 3 entry-emit tests
  updated to assert the 2-emit shape. 6 new tests covering ISO week
  math, single-week / cross-week / running-timer / bad-input shapes,
  and a full end-to-end cross-week AddEntry.

### W4-04c — Subscription-set short-circuit gate
- **Commit:** `e3f4a4b` (`feat(resources): subscription gate to skip unsubscribed re-reads (W4-04c)`)
- **Files:** `internal/mcp/resources.go`, `internal/tools/common.go`,
  `internal/tools/resources.go`, `cmd/clockify-mcp/runtime.go`,
  `internal/tools/subscription_gate_test.go` (new), plus a bundled
  gofmt fix to `internal/tools/weekly_report_emit_test.go`
- **Summary:** New public `Server.HasResourceSubscription(uri) bool`
  wrapping the private `resourceSubs.has()`. New `tools.Service.SubscriptionGate
  func(uri string) bool` field, wired at runtime to
  `Server.HasResourceSubscription`. `emitResourceUpdate` now
  short-circuits BEFORE the `ReadResource` round-trip when the gate
  reports no active subscription — unsubscribed mutations pay zero
  Clockify API cost for the delta-sync path. 3 new tests: counting
  httptest handler asserts zero GETs while unsubscribed, per-URI
  granularity test (gate returns true only for `/entry/` URIs), and a
  `HasResourceSubscription` round-trip via the public JSON-RPC
  `resources/subscribe` dispatch path.

### W4-04d — Cache write-through on mutation responses
- **Commit:** `c53f17b` (`feat(resources): cache write-through on mutation responses (W4-04d)`)
- **Files:** `internal/tools/resources.go`, `internal/tools/entries.go`,
  `internal/tools/timer.go`,
  `internal/tools/cache_write_through_test.go` (new)
- **Summary:** New `emitResourceUpdateWithState(uri, payload)` variant
  that bypasses `ReadResource` and marshals the caller's post-API
  response struct directly. New `emitEntryAndWeeklyWithState(ctx, ws,
  entry clockify.TimeEntry)` wrapper. Every entry-mutating handler now
  feeds the post-API `TimeEntry` through the write-through path, so
  subscribed entry mutations cost exactly 1 Clockify HTTP call
  (the mutation itself) instead of 2. Weekly-report URIs still route
  through the normal emit path because their aggregated data is not
  derivable from a single entry. 2 new tests: counting handler asserts
  zero GETs on subscribed `AddEntry`, and a sequential `AddEntry →
  UpdateEntry` scenario proving the cache was primed by the
  write-through (second mutation produces `format=merge` with a
  minimal `{billable: true}` patch instead of `format=none`).
- **Hot path impact:** Wave 3 subscribed `AddEntry` = 1 POST + 1 GET.
  Wave 4 T-4c + T-4d subscribed `AddEntry` = 1 POST + 0 GETs for the
  entry URI. Wave 4 unsubscribed `AddEntry` = 1 POST + 0 emits + 0
  GETs.
