# Wave 2 backlog — active

**Opens:** 2026-04-11, immediately after `v0.6.0` was tagged.
**Supersedes:** [`docs/wave1-backlog.md`](wave1-backlog.md) (closed at `40ef8d3`).
This file is the active backlog for the 0.6.x → 0.7.0 development cycle. Items
land on `main` in priority order; each lands as an atomic commit with green CI.
Closed items migrate to the [Landed](#landed) section at the bottom with their
commit SHA, mirroring how Wave 1 was tracked.

## Tier 1 — contract enforcement

(empty — W2-01 landed; see the [Landed](#landed) section.)

## Tier 2 — observability & release-infra depth

### W2-12 — Release infrastructure gaps exposed by the v0.6.0 cut

**Context.** The `v0.6.0` release workflow and the tag-triggered Docker Image
workflow each partially failed on 2026-04-11 for infra/secret reasons (not
code). The core release — GitHub Release page with 16 assets, cosign keyless
signatures, SLSA build provenance attestations, `ghcr.io/apet97/go-clockify:v0.6.0`
— all landed cleanly. The gaps are narrowly scoped:

1. **npm publish — `ENEEDAUTH`**. The `Publish npm packages` job in
   `.github/workflows/release.yml` built every platform tarball
   (`@anycli/clockify-mcp-go-{darwin-arm64,darwin-x64,linux-x64,linux-arm64,windows-x64}@0.6.0`
   plus the base package) and reached `npm publish --access public` with
   `NODE_AUTH_TOKEN` empty, logging `npm error code ENEEDAUTH`. Root cause:
   the repo-level secret that populates `NODE_AUTH_TOKEN` (`NPM_TOKEN` or
   equivalent) is either missing or not wired to the step. Result: 0.6.0 is
   not on npm; the latest npm version remains 0.5.x until this is fixed.

2. **Docker-tag SBOM attach — `Resource not accessible by integration`**. The
   tag-triggered `Docker Image` workflow built + pushed
   `ghcr.io/apet97/go-clockify@sha256:0fc2e3857c2660a2497ea4943dcae2d30694de846f98cb11e31f9f3a5bb85ee9`,
   generated a syft SPDX image SBOM, uploaded it as a workflow artifact, then
   failed at the `Attaching SBOMs to release: 'v0.6.0'` step with
   `##[error]Resource not accessible by integration`. Root cause: the
   workflow's `GITHUB_TOKEN` lacks `contents: write` (or similar) on the
   tag-push event, so it cannot add an asset to the already-created Release.
   The image itself is fine; only the SBOM attachment is missing from the
   v0.6.0 Release page.

**Files.**
- `.github/workflows/release.yml` — verify `NODE_AUTH_TOKEN` wiring
  (`env.NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}` on the `publish` job, or
  `actions/setup-node@v4` with `registry-url: https://registry.npmjs.org`
  which reads `NPM_TOKEN` automatically).
- `.github/workflows/docker-image.yml` — add explicit top-level `permissions:`
  block granting `contents: write` (for release asset upload) and
  `id-token: write` (already required for cosign keyless); confirm the SBOM-
  attach step still runs when the Release was created by a sibling workflow.
- New repo-level secret: `NPM_TOKEN` (classic automation token with publish
  rights to `@anycli/clockify-mcp-go*`).
- `docs/verification.md` — add a note that operators should verify the cosign
  bundle + SLSA attestation directly; npm tarball verification is Wave 2+.
- `CHANGELOG.md [Unreleased] > Fixed` — note that 0.6.0 had partial
  distribution and 0.6.1 will be a re-cut with the full release surface.

**Acceptance.**
- A follow-up `v0.6.1` tag produces a fully-green Release workflow (all jobs).
- All five `@anycli/clockify-mcp-go-<platform>@0.6.1` packages + the base
  `@anycli/clockify-mcp-go@0.6.1` are live on npmjs.org.
- The tag-triggered Docker workflow successfully attaches the image SBOM to
  the v0.6.1 Release.
- Runbook `docs/runbooks/release-incident.md` (new, short) captures the
  diagnosis path for future breakages.

**Size:** S–M. **Blocks:** the next clean release (likely 0.6.1 or 0.7.0).

### W2-04 — Tracing as a Go submodule

**Context.** `go.mod` currently carries OTel rows even though the default
build links zero OTel symbols. Moving `internal/tracing/otel.go` into a
sub-module would remove those rows entirely — Decision Point #3 from the
Wave 1 plan, deferred at the time for scope. Needs an ADR addendum.
**Size:** M–L.

## Tier 3 — deployment polish

### W2-05 — Helm chart under `deploy/helm/`

Same values surface as the raw manifests in `deploy/k8s/`. **Size:** M.

### W2-06 — Kustomize overlays

Split `deploy/k8s/` into `base/` + `overlays/{dev,staging,prod}`. **Size:** M.

### W2-07 — Goreleaser / release-please migration

Replace the hand-rolled `release.yml`. Would also close W2-12 cleanly because
release-please owns its own npm publish wiring. **Size:** M.

## Tier 4 — verification depth

### W2-08 — Chaos harness

`tests/chaos/` with kill-replica, drop-packet, and 429-storm scenarios via
toxiproxy. **Size:** L.

### W2-09 — Load harness

`tests/load/` with a per-token tenant mix that reliably fires
`ClockifyMCPFastBurn` in staging. **Size:** M.

### W2-10 — Mutation testing in nightly CI

**Size:** L.

### W2-11 — FIPS build target behind `-tags=fips`

**Size:** M. Only if an enterprise contact asks for it.

## Out of scope for Wave 2 (deferred to Wave 3)

- Reproducible-build verification job
- gRPC transport
- Delta-sync resources on top of the subscription set from Phase E (W1-04)

## Landed

### W2-03 — CodeQL Action v3 → v4

**Landed:** 2026-04-11 (Track C of the v0.6.1 release session). The single
CodeQL action invocation in the repo — `github/codeql-action/upload-sarif`
in `.github/workflows/docker-image.yml:203` — was pinned at v3, which
GitHub began surfacing as a deprecation warning on every run.

**Change.** One SHA pin bump:
`5c8a8a642e79153f5d047b10ec1cba1d1cc65699 # v3` →
`c10b8064de6f491fea524254123dbe5e09572f13 # v4.35.1`.

**v4 breaking changes relevant to this repo.** None that affect
`upload-sarif` specifically. The only v3→v4 delta documented in the
action changelog (4.30.7, 2025-10-06) is "the CodeQL Action now runs
on Node.js v24" — a runtime bump that GitHub-hosted `ubuntu-22.04`
runners already support. All `upload-sarif` inputs are unchanged;
`sarif_file: trivy-results.sarif` still validates against the v4
schema without edits.

**Verification.** Docker Image workflow run on the Track C commit
uploads Trivy SARIF successfully via the v4 pin; the run's security
tab continues to show the scan results exactly as before the upgrade.

### W2-02 — `pprof` exposure behind `-tags=pprof`

**Landed:** 2026-04-11 (Track B of the v0.6.1 release session). Previously
`docs/runbooks/oom-or-goroutine-leak.md` instructed operators to rebuild
with `net/http/pprof` manually; that rebuild path is now a first-class
build tag that mounts `/debug/pprof/*` on whichever HTTP transport the
server is running (`http` or `streamable_http`).

**Design.** A neutral `ExtraHandler{Pattern, Handler}` type plus a
`mountExtras` helper in `internal/mcp/transport_extra.go` (stdlib-only,
zero pprof references). Both transports grew a slice field
(`Server.ExtraHTTPHandlers` for legacy HTTP, `StreamableHTTPOptions.ExtraHandlers`
for streamable) that `mountExtras` walks before `ListenAndServe`.
`cmd/clockify-mcp/` owns the sole `net/http/pprof` import behind
`//go:build pprof` in `pprof_on.go`; the default build sees only
`pprof_off.go` which returns `nil`, so `mountExtras` is a no-op.

**Critical files shipped:**
- `internal/mcp/transport_extra.go` (new) — `ExtraHandler` type +
  `mountExtras` helper, both stdlib-only.
- `internal/mcp/server.go` — new `ExtraHTTPHandlers []ExtraHandler`
  field on `Server`.
- `internal/mcp/transport_http.go` — `ServeHTTP` calls
  `mountExtras(mux, s.ExtraHTTPHandlers)` after core handlers.
- `internal/mcp/transport_streamable_http.go` — new
  `ExtraHandlers []ExtraHandler` field on `StreamableHTTPOptions`;
  `ServeStreamableHTTP` calls `mountExtras(mux, opts.ExtraHandlers)`.
- `internal/mcp/transport_extra_pprof_test.go` (new, `//go:build pprof`)
  — mountExtras + pprof end-to-end through `httptest.NewServer`;
  `goroutine` and `cmdline` profiles reachable; baseline handler still
  reachable alongside pprof; compile-time field-wiring guard.
- `cmd/clockify-mcp/pprof_on.go` (new, `//go:build pprof`) — side-imports
  `net/http/pprof`, returns a one-element `[]ExtraHandler` pointing at
  `http.DefaultServeMux` with the `/debug/pprof/` pattern. Emits a
  startup warning so operators never miss that a debug build is running.
- `cmd/clockify-mcp/pprof_off.go` (new, `//go:build !pprof`) — stub
  returning `nil`.
- `cmd/clockify-mcp/main.go` — two call sites: `ExtraHandlers: pprofExtras()`
  in the `ServeStreamableHTTP` branch; `server.ExtraHTTPHandlers = pprofExtras()`
  in the legacy HTTP branch, set between `ReadyChecker` wiring and
  `server.ServeHTTP(...)`.
- `.github/workflows/ci.yml` — extended the `build` job: added the
  negative nm-gate (`net/http/pprof` count must equal 0 in default
  build), the `-tags=pprof` build + positive nm-gate (must be > 0),
  a `-tags=pprof` test run of `./internal/mcp/...`, and a combined
  `-tags=pprof,otel` build step.
- `docs/runbooks/oom-or-goroutine-leak.md` — replaced the manual-rebuild
  paragraph with the `-tags=pprof` recipe and a security note that
  pprof endpoints bypass the `/mcp` bearer gate.

**Verification:** default build has 0 `net/http/pprof` symbols, `-tags=pprof`
build has 45. Both CI nm-gates cover the regression surface.

### W2-01 — Runtime JSON-schema validation at the enforcement boundary

**Landed:** 2026-04-11 (Track C of the v0.6.0 release session, commit SHA
recorded at push time). Wire enforcement of every Tier 1 + Tier 2 tool's
`InputSchema` via a new stdlib-only validator at `internal/jsonschema`,
threaded into `enforcement.Pipeline.BeforeCall` as the first gate.
Validation failures surface as JSON-RPC `-32602 invalid params` with
an RFC 6901 JSON Pointer in `error.data.pointer`.

**Critical files shipped:**
- `internal/jsonschema/validator.go` + `validator_test.go` — new package,
  ~450 LOC, 86.4% coverage.
- `internal/mcp/types.go` — `InvalidParamsError` typed error +
  `RPCError.Data` field; `Enforcement.BeforeCall` signature gained
  `schema map[string]any`.
- `internal/mcp/server.go` — `tools/call` dispatch `errors.As` branch
  sets `resp.Error = &RPCError{Code: -32602, Data: {pointer}}`;
  `callTool` passes `d.Tool.InputSchema` to BeforeCall; new `outcome`
  label `invalid_params` on `clockify_mcp_tool_calls_total`.
- `internal/enforcement/enforcement.go` — `Pipeline.BeforeCall` validation
  first step, wrapping `jsonschema.ValidationError` into
  `*mcp.InvalidParamsError`.
- `internal/mcp/schema_validation_dispatch_test.go` (new) — three
  dispatch-layer tests asserting -32602 + pointer translation, wire
  JSON shape, and non-schema-error pass-through.
- `internal/enforcement/schema_validation_test.go` (new) — six pipeline-
  level tests (unknown key, wrong type, missing required, invalid
  date-time, happy path, nil-schema passthrough).
- `internal/tools/schema_validator_property_test.go` (new) —
  `TestRegistrySchemasAcceptHappyPathArgs` synthesises happy-path args
  for every Tier 1 + Tier 2 descriptor; walker/validator drift guard.
- `docs/adr/008-runtime-schema-validation.md` (new).
- `docs/troubleshooting.md` — new `-32602 invalid params` row.
- `CHANGELOG.md [Unreleased]` — `Added` + `Changed` (with BREAKING note).
- `.github/workflows/ci.yml` — new per-package floor
  `check_pkg internal/jsonschema 85`.

**Coverage delta:** `internal/jsonschema` new at **86.4%**;
`internal/enforcement` 89.5% → **89.0%** (floor 85%);
`internal/mcp` 71.5% → **71.8%**; global 66.7% → **67.4%**. OTel
symbol gate still clean.
