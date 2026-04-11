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

## Tier 3 — deployment polish

### W2-05 — Helm chart under `deploy/helm/`

Same values surface as the raw manifests in `deploy/k8s/`. **Size:** M.

### W2-06 — Kustomize overlays

Split `deploy/k8s/` into `base/` + `overlays/{dev,staging,prod}`. **Size:** M.

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

### W2-07 — Release pipeline migration to goreleaser (closes W2-13)

**Landed:** 2026-04-11 (Track B of the v0.7.0 development session). The
hand-rolled `build-binaries` + `create-release` matrix in
`.github/workflows/release.yml` is replaced by a single goreleaser-driven
job configured through a new `.goreleaser.yaml` at the repo root. Same
binary filenames, same SBOM/signature/checksum filenames, same ldflags,
same build matrix. Since goreleaser free does not ship an npm publisher,
a companion `scripts/publish-npm.sh` consumes goreleaser's `dist/` output
and publishes six packages under `@apet97/` — closing W2-13 in the same
commit.

**Critical files shipped:**
- `.goreleaser.yaml` (new) — builds 5 platforms
  (darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, windows/amd64)
  with the pre-W2-07 ldflags (`-s -w -X main.version={{.Tag}} -X
  main.commit -X main.buildDate`) and `-trimpath`. Archive format is
  `binary` (no tarball wrapper); naming template produces
  `clockify-mcp-{os}-{arch}(.exe)` matching `docs/verification.md`
  expectations. `checksum.name_template: SHA256SUMS.txt`,
  `sboms` runs syft per archive into `{archive}.spdx.json`, `signs`
  runs `cosign sign-blob --bundle={artifact}.sigstore.json` keyless
  OIDC. `release:` uploads to the matching GH Release tag.
- `.github/workflows/release.yml` — rewritten to a single `release` job
  on `ubuntu-22.04` with `contents: write`, `id-token: write`,
  `attestations: write`, `packages: write`. Steps: install go/node/cosign/syft
  → `goreleaser release --clean` → stage binaries into `staging/` with
  release asset names → `actions/attest-build-provenance` (so SLSA
  subject names match what operators download) → invoke
  `scripts/publish-npm.sh` if `NPM_TOKEN` is set. Graceful no-op when
  `NPM_TOKEN` is absent so releases still ship without npm.
- `scripts/publish-npm.sh` (new, executable) — bash driver that
  iterates a 5-row platform table, stages each platform package from
  `dist/clockify-mcp_{os}_{arch}_*/clockify-mcp[.exe]`, substitutes
  placeholders in `npm/package.json.tmpl`, runs `npm publish --access
  public`, then copies the base package from `npm/clockify-mcp-go/`,
  substitutes `VERSION`, and publishes it last. Validates locally via
  `NPM_CONFIG_DRY_RUN=true`.
- `npm/package.json.tmpl` — scope changed from `@anycli` to `@apet97`,
  `repository.url` normalised to `git+https://...` so npm doesn't
  warn on publish.
- `npm/clockify-mcp-go/package.json` (new) — the base dispatcher
  package, depends on the 5 platform siblings via `optionalDependencies`
  so npm only installs the one matching the user's os/cpu. `bin` maps
  `clockify-mcp` to `bin/clockify-mcp.js`.
- `npm/clockify-mcp-go/bin/clockify-mcp.js` (new, executable) — Node
  dispatcher that resolves the correct platform package at runtime via
  `require.resolve` and exec's the Go binary with `spawnSync`. Fails
  loudly with actionable messages when no prebuilt matches the host
  or when optionalDependencies failed to install.
- `docs/adr/010-goreleaser-migration.md` (new) — ADR documenting the
  invariants preserved, the decisions deliberately left out of scope
  (docker image build, Homebrew tap, release-please), and why SLSA
  attestation needs the staging copy.

**Acceptance (local snapshot).**
- `goreleaser check` passes.
- `goreleaser release --snapshot --skip=publish --skip=sign --skip=sbom
  --clean` produces 5 binaries in ~20 seconds. Archive names in
  `dist/artifacts.json` match `clockify-mcp-{darwin-arm64,darwin-x64,
  linux-x64,linux-arm64,windows-x64.exe}`. `dist/SHA256SUMS.txt`
  references those exact filenames.
- `NPM_CONFIG_DRY_RUN=true ./scripts/publish-npm.sh v0.7.0` publishes
  6 packages via npm dry-run against the local dist/ with no errors.
- Released binary `--version` prints `v0.7.0` (tag preserved via
  `{{.Tag}}` in ldflags).

**Verification gate (post-tag).** `gh release view v0.7.0 --json assets`
must list every pre-W2-07 asset name, `gh attestation verify
clockify-mcp-linux-x64 --repo apet97/go-clockify` must succeed, and
`npm view @apet97/clockify-mcp-go versions` must include `0.7.0`.

### W2-04 — Tracing as a Go sub-module

**Landed:** 2026-04-11 (Track A of the v0.7.0 development session).
Closes the ADR 001 W1 deferred trade-off. `internal/tracing/otel.go`
was moved into a dedicated Go sub-module at `internal/tracing/otel/`
with its own `go.mod`; the top-level `go.mod` now carries zero
`go.opentelemetry.io` rows (down from 9). The tag-gated installer
pair `cmd/clockify-mcp/otel_{on,off}.go` mirrors the `pprof_{on,off}.go`
template established by W2-02.

**Critical files shipped:**
- `internal/tracing/otel/go.mod` + `go.sum` (new sub-module) — module
  path `github.com/apet97/go-clockify/internal/tracing/otel`, replaces
  `github.com/apet97/go-clockify => ../../..` for the `Tracer`/`Span`
  interface.
- `internal/tracing/otel/otel.go` (moved from `internal/tracing/otel.go`)
  — package `otel`, exported `Install(ctx) (shutdown, error)` replaces
  the previous `init()` auto-register hook.
- `cmd/clockify-mcp/otel_on.go` (new, `//go:build otel`) — reads
  `OTEL_EXPORTER_OTLP_ENDPOINT` as a gate, delegates to the sub-module's
  `Install`, logs through `slog` on failure, returns a safe no-op on
  unset endpoint or failed exporter.
- `cmd/clockify-mcp/otel_off.go` (new, `//go:build !otel`) — default-build
  stub returning a no-op shutdown.
- `cmd/clockify-mcp/main.go` — `run()` calls `installOTel(ctx)` right
  after `signal.NotifyContext` and `defer`s the shutdown.
- `go.mod` (top-level) — dropped from 28 lines to 7; now carries a
  single `replace` directive pointing at the sub-module plus the
  corresponding `require`. Zero `go.opentelemetry.io` rows.
- `go.work` (new, repo root) — lists the main module and the sub-module
  so `go build -tags=otel ./...` from the parent resolves the sub-module
  locally.
- `.github/workflows/ci.yml` — two new `build`-job steps:
  `Verify go.mod has zero OpenTelemetry rows` (grep gate) and
  `Build tracing sub-module` (cd into sub-module and run `go build` +
  `go vet`). Existing `-tags=otel` build and nm gate remain unchanged.
- `docs/adr/009-tracing-submodule.md` (new) — ADR covering context,
  decision, consequences, and the `go mod tidy` caveat (developers
  must `git restore go.mod` after tidy re-adds OTel indirect rows).
- `docs/adr/001-stdlib-only.md` — extended the "opt-in OpenTelemetry"
  paragraph to point at ADR 009 as the closure of the W1-deferred
  trade-off.
- `docs/observability.md` — updated the `-tags=otel` section to
  describe the sub-module + `Install` path instead of the previous
  `init()` hook.

**Acceptance.**
- Default binary: 0 `opentelemetry` symbols (unchanged).
- `-tags=otel` binary: 2077 `opentelemetry` symbols (unchanged).
- Top-level `go.mod`: 0 `go.opentelemetry.io` rows (down from 9).
- Sub-module builds and vets cleanly from its own directory.
- Full `go test -race ./...` suite green.

### W2-12 — Release infrastructure gaps from the v0.6.0 cut

**Landed:** 2026-04-11 across Track A.1 + the npm deletion of the v0.6.1
release session. The v0.6.0 cut surfaced two pipeline gaps that blocked a
clean release: (1) the docker-image workflow's SBOM attach-to-release
step failed with `Resource not accessible by integration` because the
workflow permissions granted only `contents: read`, and (2) the
`publish-npm` job hit `ENEEDAUTH` because `NPM_TOKEN` was never set.

Investigating (2) during the v0.6.1 cut surfaced additional latent
problems: the `@anycli` scope is not controlled by this project, and the
`Publish base package` step referenced `npm/clockify-mcp-go/` which has
never existed in the repo. The entire hand-rolled npm pipeline was
therefore deleted rather than papered over, and the full npm surface
is now tracked as W2-13 (see above — not yet landed).

**Changes shipped:**
- `.github/workflows/docker-image.yml` — bumped top-level
  `permissions.contents` from `read` to `write` so the
  `anchore/sbom-action` upload-to-release step can add the image SBOM
  as a GH Release asset. Fix is scoped to the docker-image workflow;
  unrelated jobs keep their default permissions.
- `docs/runbooks/release-incident.md` (new) — runbook with the two
  canonical partial-release failure modes (`ENEEDAUTH`, `Resource not
  accessible by integration`), diagnosis commands, and the rerun-vs-re-cut
  decision tree.
- `.github/workflows/release.yml` — the `publish-npm` job was deleted in
  full. A comment at the deletion site documents why and lists the
  checklist of everything that must be present when the job is
  rebuilt (W2-13). The `create-release` job keeps all its existing
  behaviour; the GH Release continues to carry binaries, signatures,
  attestations, and SBOMs.
- `CHANGELOG.md [0.6.1]` — `Fixed` entry noting that v0.6.0 shipped
  with partial distribution (missing SBOM release asset + no npm
  tarballs) and v0.6.1 is the re-cut with the docker SBOM fix.

**Acceptance.** The v0.6.1 tag re-runs both the release and docker-tag
workflows end-to-end with zero failing jobs. The image SBOM now attaches
to the v0.6.1 GH Release as an asset.

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
