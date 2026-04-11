# Wave 3 backlog — closed

**Opened:** 2026-04-11, immediately after `v0.7.1` was tagged.
**Closed:** 2026-04-12 at `v0.8.0`. Every Wave 3 item listed here landed on
`main` in the v0.8.0 development session. Supersedes:
[`docs/wave2-backlog.md`](wave2-backlog.md) (closed at `v0.7.0`).
The file is kept as an append-only historical record; future work lives in
a subsequent wave backlog that will be opened when the v0.9.0 cycle begins.

All Wave 3 items landed — see the [Landed](#landed) section below.

## Out of scope for Wave 3 (deferred to Wave 4)

- gRPC native authentication interceptor (`MCP_AUTH_MODE=static_bearer` and
  `MCP_AUTH_MODE=oidc` on the gRPC transport). Wave 3 ships gRPC with auth
  rejected at config validation; production deployments are expected to
  terminate authentication at the service mesh edge. ADR 012 follow-ups.
- Multi-stream notifier fan-out for the gRPC transport. Today the most
  recently opened `Exchange` stream wins for server-wide broadcasts; an
  N-stream fan-out hub is a follow-up.
- Delta-sync wiring for user, workspace, and weekly-report URI templates.
  Wave 3 wires entry and project mutations only — see ADR 013 §Wiring
  matrix and Follow-ups.
- Cache write-through optimisation for delta-sync. The emit helper currently
  re-reads the resource via `ResourceProvider.ReadResource` to obtain the
  fresh state; a future optimisation can pass the post-mutation API response
  directly into the cache when the response shape matches the resource view.
- `Server.HasResourceSubscription(uri)` short-circuit so the tools layer can
  skip the re-read entirely when nothing is subscribed. Trades a small
  surface increase for an API-call savings on the unsubscribed hot path.
- RFC 6902 JSON Patch as an alternative `format=jsonpatch` code on
  `notifications/resources/updated`. Wave 3 ships only RFC 7396 merge patch.

## Landed

### W3-01 — Pinned actions bumped to Node.js 24
- **Commit:** `9abea1d` (`ci: bump pinned actions to Node.js 24 (W3-01)`)
- **Files:** `.github/workflows/release.yml`, `.github/workflows/docker-image.yml`
- **Summary:** Audited every `uses:` line by fetching `action.yml` at the
  pinned SHA. Seven actions were on Node 20; bumped to the latest Node 24
  releases (`actions/setup-node` v6.3.0, `goreleaser/goreleaser-action`
  v7.0.0, `actions/attest-build-provenance` v4.1.0,
  `sigstore/cosign-installer` v4.1.1, `docker/setup-qemu-action` v4.0.0,
  `docker/setup-buildx-action` v4.0.0, `docker/login-action` v4.1.0,
  `docker/metadata-action` v6.0.0, `docker/build-push-action` v7.1.0).
  Done well ahead of GitHub's 2026-06-02 Node 20 deprecation cutoff.
- **Latent bug closed:** `sigstore/cosign-installer@f713795c...` was a SHA
  that no longer resolves in the cosign-installer repo — the next tagged
  release would have failed. The W3-01 bumps replaced it with v4.1.1.

### W3-02 — Reproducible-build verification CI job
- **Commit:** `cd90e40` (`ci: reproducible-build verification job (W3-02)`)
- **Files:** `.github/workflows/reproducibility.yml`, `docs/reproducibility.md`,
  `docs/verification.md`
- **Summary:** New workflow fires on `release: published` and on manual
  `workflow_dispatch`. Cross-compiles all 9 release assets (5 default +
  4 FIPS) on a single `ubuntu-22.04` runner using the same Go toolchain
  and exact ldflags goreleaser uses, then sha256-compares each rebuild
  against the published asset. Verified end-to-end against v0.7.1 — all
  9 platforms reproduce byte-for-byte. The non-obvious step:
  goreleaser dirties the working tree by creating `dist/`, so a clean
  checkout would embed `vcs.modified=false` and produce different bytes;
  the workflow induces the same dirty state via a placeholder file.
- **Verified:** `gh workflow run reproducibility.yml -f tag=v0.7.1` with
  9/9 matrix entries green.

### W3-03 — Delta-sync notifications/resources/updated via RFC 7396 Merge Patch
- **Commits:** `5fcdfaa` (W3-03a), `ffb2f8c` (W3-03b), `693d22f` (W3-03c),
  `20cf724` (W3-03d), `c0702f4` (W3-03e), `d1a699b` (W3-03f docs)
- **Files:** `internal/jsonmergepatch/`, `internal/tools/resource_cache.go`,
  `internal/tools/resources.go`, `internal/tools/common.go`,
  `internal/tools/resource_emit_test.go`, `internal/mcp/resources.go`,
  `internal/mcp/resources_test.go`, `internal/tools/entries.go`,
  `internal/tools/timer.go`, `internal/tools/projects.go`,
  `internal/tools/tier2_project_admin.go`, `cmd/clockify-mcp/runtime.go`,
  `docs/adr/013-resource-delta-sync.md`
- **Summary:** Hand-rolled stdlib-only RFC 7396 merge-patch differ at
  `internal/jsonmergepatch/`. Additive wire-format extension to
  `notifications/resources/updated` carries `format` + `patch` alongside
  the legacy `uri` field. Bounded LRU `resourceStateCache` (1024 entries)
  in `tools.Service` plus an `EmitResourceUpdate` hook wired from
  `runtime.go` to `Server.NotifyResourceUpdated`. Mutation wiring covers
  every Tier 1 + Tier 2 entry and project mutation: AddEntry,
  UpdateEntry, DeleteEntry (format=deleted), StartTimer, StopTimer,
  CreateProject, UpdateProjectEstimate, SetProjectMemberships,
  ArchiveProjects (per-project). Wave 1's dormant subscription set is
  finally reachable. ADR 013 documents the wire format, format-code
  table, mutation matrix, and Wave 4 follow-ups.
- **Tests:** RFC 7396 §3 vectors (14 cases), Diff↔Apply round-trip
  property test (11 permutations), wire-format assertions in
  `TestResourcesSubscribeAndNotify` (legacy + merge + none envelopes),
  three end-to-end mutation tests via `httptest`-based `newTestClient`
  harness (AddEntry, UpdateEntry, DeleteEntry).

### W3-04 — gRPC transport via isolated sub-module
- **Commit:** `614b775` (`feat(transport): gRPC transport via isolated sub-module (W3-04)`)
- **Files:** `internal/transport/grpc/`, `cmd/clockify-mcp/grpc_on.go`,
  `cmd/clockify-mcp/grpc_off.go`, `cmd/clockify-mcp/main.go`,
  `internal/config/config.go`, `internal/config/config_test.go`,
  `internal/mcp/server.go`, `.github/workflows/ci.yml`,
  `docs/adr/012-grpc-transport.md`, `CLAUDE.md`, `go.work`
- **Summary:** New optional gRPC transport linked only under `-tags=grpc`.
  Sub-module at `internal/transport/grpc/` with its own `go.mod` so the
  top-level module graph stays clean. Wire format is **raw JSON-RPC 2.0
  bytes** carried by a hand-wired `grpc.ServiceDesc` and a custom
  `encoding.Codec` — no protobuf code generation required, no `protoc`
  toolchain dependency. New `Server.DispatchMessage` exports the
  protocol-core dispatcher for non-stdio transports. Six new CI gates
  enforce the stdlib-only invariant: zero gRPC symbols in the default
  binary, zero gRPC rows in the top-level `go.mod`, sub-module
  build/vet/test, and a positive symbol-count check on the `-tags=grpc`
  binary. Tests cover initialize, ping, malformed-JSON parse-error,
  and graceful shutdown via `google.golang.org/grpc/test/bufconn`.
- **Auth note:** `MCP_AUTH_MODE` is currently rejected for gRPC; auth
  is expected to terminate at the service mesh edge. Native interceptor
  is a Wave 4 follow-up.

### W3-05 — OIDC tampered-signature flake (latent bug closed)
- **Commit:** `9e941b6` (`test(authn): fix 1/4096 OIDC tampered-signature flake`)
- **Files:** `internal/authn/oidc_integration_test.go`
- **Summary:** Surfaced during the W3-04 CI run as a phantom regression.
  `TestOIDCAuthenticator_JWKSIntegration` overwrote the last 2 base64url
  characters of an RSA-2048 signature with `"AA"` to simulate tampering;
  base64url `"AA"` decodes to 12 zero bits, so whenever the signature
  happened to end in 12 zero bits — probability ~1/4096 per run — the
  "tampering" was a no-op and the test failed with "expected tampered
  signature to fail". Replaced with a `decode → XOR middle byte with
  0xFF → re-encode` sequence that guarantees a flip on every run.
  Verified across 5 consecutive runs under default and `-tags=fips`.
