# Wave 2 backlog — active

**Opens:** 2026-04-11, immediately after `v0.6.0` was tagged.
**Supersedes:** [`docs/wave1-backlog.md`](wave1-backlog.md) (closed at `40ef8d3`).
This file is the active backlog for the 0.6.x → 0.7.0 development cycle. Items
land on `main` in priority order; each lands as an atomic commit with green CI.
Closed items migrate to the [Landed](#landed) section at the bottom with their
commit SHA, mirroring how Wave 1 was tracked.

## Tier 1 — contract enforcement

### W2-01 — Runtime JSON-schema validation at the enforcement boundary

**Context.** Phase H (`W1-10`) rewrote every Tier 1 + Tier 2 input schema to
carry `additionalProperties:false`, bounded pagination, RFC3339 format, and
hex-color patterns — but nothing on the wire actually validates against those
schemas. Clients that send extra keys or wrong types today silently reach the
handler. W2-01 turns the contract into real wire enforcement.

**Files.**
- `internal/jsonschema/validator.go` (new) — stdlib-only validator, ~400 LOC.
- `internal/jsonschema/validator_test.go` (new).
- `internal/enforcement/enforcement.go` — `Pipeline.BeforeCall` gains a
  validation step as its first gate.
- `internal/mcp/types.go` — introduce `InvalidParamsError` (typed, carries a
  JSON Pointer path) so the protocol core can translate it to JSON-RPC -32602.
- `internal/mcp/server.go` — `tools/call` dispatch `errors.As`-checks for
  `*mcp.InvalidParamsError` and sets `resp.Error = &RPCError{Code: -32602}`
  instead of wrapping into `result.isError:true`.
- `internal/enforcement/schema_validation_test.go` (new) — pipeline-level test.
- `internal/mcp/transport_streamable_http_test.go` — wire-level test asserting
  the -32602 code + JSON Pointer path on invalid args.
- `docs/adr/008-runtime-schema-validation.md` (new).
- `docs/troubleshooting.md` — add a row for the new -32602 failure mode.
- `CHANGELOG.md` — `[Unreleased] > Changed` entry with `**BREAKING:**` prefix.

**Scope (supported JSON-schema keywords).**
`type` (object/string/integer/number/boolean/array), `required`,
`additionalProperties: false`, `properties` (recursive), `items`,
`minimum`/`maximum`, `minLength`/`maxLength`, `pattern` (anchored via
`^...$`), `format: date` / `format: date-time`, `enum`.

**Out of scope.** `$ref`, `$defs`, `allOf`/`anyOf`/`oneOf`, `not`,
conditionals (`if`/`then`/`else`), `dependentSchemas`, `const`,
`exclusiveMinimum`/`exclusiveMaximum`, `multipleOf`, `propertyNames`,
`patternProperties`. None appear in Tier 1 or Tier 2 today.

**Size:** L. **Acceptance:**
- Every Tier 1 + Tier 2 tool's happy-path args still pass (property test).
- Representative invalid args per keyword get rejected with -32602 + pointer.
- `internal/enforcement` stays ≥ 85%, `internal/jsonschema` lands at ≥ 85%.
- Stdlib-only default-build OTel symbol gate still passes.

**Scheduled for TRACK C of the 2026-04-11 session.**

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

### W2-02 — `pprof` exposure behind `-tags=pprof`

**Context.** `docs/runbooks/oom-or-goroutine-leak.md` currently instructs
operators to rebuild with `net/http/pprof` manually. Turn that into a
build-tagged opt-in so production binaries never link `pprof` but
debug builds can mount `/debug/pprof/*` under the existing HTTP transport.

**Files.** `cmd/clockify-mcp/pprof_on.go` (tag-gated), `cmd/clockify-mcp/pprof_off.go`
(default stub). Add a "Build with `-tags=pprof`" CI step. Update the runbook.
**Size:** S.

### W2-03 — CodeQL Action v3 → v4

**Context.** `.github/workflows/*` emits deprecation warnings on every run.
Upgrade and re-test. **Size:** S.

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

(empty — items move here with commit SHA when they close)
