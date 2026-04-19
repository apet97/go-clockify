# Production readiness

This is the single page a platform-team reviewer should need to
answer "is this ready for production deployment in our environment?"
Every section below is a one-paragraph summary plus a link to the
canonical artifact. If you find yourself needing a second page that
this one does not link, file an issue — the goal is for this page to
be the only entry point.

## Threat model summary

`go-clockify` is a thin proxy in front of `api.clockify.me` exposed
over MCP. The threats it must defend against are: (1) API key
exposure or leakage, (2) command injection via tool inputs, (3) SSRF
through webhook URL parameters, (4) authentication bypass on the
HTTP transport, (5) path traversal in ID validation, (6) CORS
bypass, and (7) timing attacks on bearer-token comparison. The full
list lives in [`SECURITY.md`](../SECURITY.md) under "Scope" with the
mitigations under "Security Features". Architectural rationale for the
decisions that shape this threat model lives in [`docs/adr/`](adr/).

## Pick a transport

| Transport         | When to use                                                                              | Auth modes supported                                  |
|-------------------|------------------------------------------------------------------------------------------|-------------------------------------------------------|
| `stdio` (default) | Local CLI clients (Claude Code, Claude Desktop, Cursor, Codex). Parent process is trusted; no inbound auth. | n/a — parent-process trust                            |
| `http`            | Single-tenant HTTP service behind a reverse proxy that terminates TLS. **Legacy transport** — prefer `streamable_http` for new deployments. Set `MCP_HTTP_LEGACY_POLICY=deny` to gate against accidental use; `warn` (default) logs once at startup. | `static_bearer`, `oidc`, `forward_auth` — `mtls` is rejected at config load (no native TLS listener; terminate TLS upstream and use `forward_auth`) |
| `streamable_http` | Multi-tenant HTTP service serving spec-strict MCP clients (2025-03-26). Per-installation tokens, session resumption. | `static_bearer`, `oidc`, `forward_auth`, `mtls`       |
| `grpc`            | Bidirectional, low-latency clients on a private network. Build-tag opt-in (`-tags=grpc`); not in the default binary. | `static_bearer`, `oidc`, `forward_auth`, `mtls` (mTLS needs `MCP_GRPC_TLS_CERT`/`_KEY`) |

Set with `MCP_TRANSPORT={stdio,http,streamable_http,grpc}`. See the [Support Matrix](support-matrix.md) for transport and auth compatibility. Validation
lives in [`internal/config/config.go`](../internal/config/config.go).
Coverage for every supported / unsupported cell is locked down by
[`TestTransportAuthMatrix`](../internal/config/transport_auth_matrix_test.go).

## Production Profile
The blessed production profile for shared services is documented in [Production Profile: Shared Service](deploy/production-profile-shared-service.md).

## Pick an auth mode (HTTP / gRPC transports only)

| Mode            | When to use                                                                              | What it trusts                                         |
|-----------------|------------------------------------------------------------------------------------------|--------------------------------------------------------|
| `static_bearer` | A small, fixed set of clients. ≥16-char shared secret distributed out of band.           | The `MCP_BEARER_TOKEN` env var (compared with `crypto/subtle`). |
| `oidc`          | Clients can present a JWT signed by an OIDC provider you trust (Auth0, Okta, Keycloak).  | `MCP_OIDC_ISSUER`'s JWKS endpoint and configured audience. Cached verify TTL caps at `MCP_OIDC_VERIFY_CACHE_TTL` (default 60s, clamped to `[1s, 5m]`). Larger values amortise verify cost; revocation then takes up to that TTL to propagate. |
| `forward_auth`  | An upstream reverse proxy (Caddy, Envoy, Traefik) already authenticates and forwards the result via a header. | `MCP_FORWARD_SUBJECT_HEADER` and `MCP_FORWARD_TENANT_HEADER` set by a trusted proxy. |
| `mtls`          | Both ends present X.509 certs against a private CA. Highest assurance, highest setup cost. | The TLS client cert presented to the listener.        |

Set with `MCP_AUTH_MODE=…`. Auth-failure triage lives in
[`docs/runbooks/auth-failures.md`](runbooks/auth-failures.md).

## Pick a deployment

| Mode               | What you get                                                                              | Where to start                                       |
|--------------------|-------------------------------------------------------------------------------------------|------------------------------------------------------|
| Kustomize          | Reference manifests with base + dev/prod overlays, NetworkPolicy, PDB, ServiceMonitor, PrometheusRule with burn-rate alerts. | [`deploy/k8s/`](../deploy/k8s/) and [`deploy/k8s/README.md`](../deploy/k8s/README.md) |
| Helm               | Same surface as Kustomize, packaged as a chart for clusters that prefer Helm.            | [`deploy/helm/`](../deploy/helm/) and `deploy/helm/README.md` |
| Container image    | Multi-arch, distroless, non-root, read-only root FS, dropped capabilities, Trivy-scanned, cosign-signed. | `ghcr.io/apet97/go-clockify:v1.0.0` (see [`docs/verification.md`](verification.md)) |
| Raw binary         | Single static binary, no runtime dependencies. Suitable for systemd or container-less hosts. | `go install github.com/apet97/go-clockify/cmd/clockify-mcp@latest` or download from the Releases page. |

Image tags are not pinned in the `prod` Kustomize overlay by design —
the pin happens at deploy time. See
[`docs/runbooks/image-digest-pinning.md`](runbooks/image-digest-pinning.md)
for the deploy-time pinning workflow (Kustomize edit, Argo CD, Flux).

## Pick a control-plane backend

The control plane holds tenants, credential references, sessions, and
the audit log. Pick via `MCP_CONTROL_PLANE_DSN`:

| Scheme               | Backend                                 | When to use                                                                                                                            |
|----------------------|-----------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------|
| `memory`             | In-process map, discarded at shutdown   | Unit tests, `go run`, single-user stdio. Never for `streamable_http`.                                                                  |
| `file://<path>`      | Single JSON file, whole-state rewrite   | Single-operator local demo. Blocked by the C1 guard for `streamable_http` unless `MCP_ALLOW_DEV_BACKEND=1` (single-process only).       |
| `postgres://<dsn>`   | pgx-backed, forward-only migrations     | Shared-service `streamable_http`, HA, any deployment where more than one clockify-mcp process talks to the same tenants or audit log. |

The Postgres backend is gated behind `-tags=postgres` because pgx is
not in the default binary's dependency graph (ADR 0001). To run
against Postgres:

```sh
go build -tags=postgres -o clockify-mcp ./cmd/clockify-mcp
export MCP_TRANSPORT=streamable_http
export MCP_CONTROL_PLANE_DSN='postgres://user:pass@db.example.com/clockify_mcp?sslmode=require'
./clockify-mcp
```

Migrations are embedded in the binary and applied under a
`pg_advisory_lock` at startup, so multiple clockify-mcp instances can
start against the same database concurrently. Schema evolution is
forward-only; the applier refuses to boot if the database reports a
schema version the binary does not know about (ADR 0011). The file
and memory stores are the dev / offline fallback and are not
recommended for any multi-process deployment.

## Upgrade path

Versioning, support window, deprecation flow, and the definition of
"breaking change" used by this project live in
[`docs/release-policy.md`](release-policy.md). Short version: only
the current minor (1.0.x today) is supported; when 1.1 ships, 1.0.x
gets security-only fixes for one minor cycle and then EOLs. There is
no LTS.

## Rollback

Roll back by re-applying a previous release tag through the same
deployment mode you used to deploy. Image tags are not in the
overlay, so a rollback is a `kubectl set image` (Kustomize edit) or
an Argo CD / Flux parameter change against an already-deployed
overlay — no in-tree YAML edit is required.

The deploy-time pinning policy and the Argo CD / Flux examples live
in [`docs/runbooks/image-digest-pinning.md`](runbooks/image-digest-pinning.md).

## Operational runbooks

Triage flows for operational classes:

- [`rate-limit-saturation.md`](runbooks/rate-limit-saturation.md) — local/upstream quota saturation.
- [`clockify-upstream-outage.md`](runbooks/clockify-upstream-outage.md) — upstream outage drill and response.
- [`postgres-restore-drill.md`](runbooks/postgres-restore-drill.md) — database restore procedures.
- [`auth-failures.md`](runbooks/auth-failures.md) — auth triage.
- [`image-digest-pinning.md`](runbooks/image-digest-pinning.md) — image pinning policy.

## Testing and Verification
- [Soak Testing and Profiling](testing/soak-and-profile.md)
- [Live Contract Testing](live-tests.md)
- [Verification Guide](verification.md)
- [Deploy-Readiness Checklist](release/deploy-readiness-checklist.md)

## Compliance posture

Each bullet is a pointer to the canonical artifact, not a claim. If
you need a checklist for a third-party assessor, this is the list.

- **Stdlib-only build** — zero external Go dependencies in the
  default binary. Verified in CI by
  [`scripts/check-build-tags.sh`](../scripts/check-build-tags.sh).
- **Reproducible builds** — `-trimpath` and a separate
  reproducibility verification job
  ([`.github/workflows/reproducibility.yml`](../.github/workflows/reproducibility.yml))
  rebuild from the tag and assert byte-for-byte parity with the
  release artifact.
- **Signed releases** — every binary carries a cosign keyless
  signature (sigstore bundle) and a SLSA build provenance
  attestation. Verification flow:
  [`docs/verification.md`](verification.md). Continuous re-verification:
  [`.github/workflows/release-smoke.yml`](../.github/workflows/release-smoke.yml).
- **SBOM** — every binary and image carries a SPDX SBOM.
- **Container image** — distroless, non-root, read-only root FS,
  dropped capabilities, Trivy-scanned (HIGH/CRITICAL blocking),
  multi-arch (linux/amd64, linux/arm64). See
  [`.github/workflows/docker-image.yml`](../.github/workflows/docker-image.yml).
- **FIPS-mode build** — opt-in via build tag. CI exercises the FIPS
  toolchain when it is available; soft-fails locally when the host
  toolchain lacks FIPS support. See
  [`scripts/check-build-tags.sh`](../scripts/check-build-tags.sh).
- **TLS termination** — the HTTP transport does NOT terminate TLS by
  design. Production deployments must front it with a TLS-terminating
  proxy (Caddy, nginx, Envoy, Traefik). Reference Caddyfile:
  [`deploy/Caddyfile`](../deploy/Caddyfile).
- **Panic containment** — both the stdio dispatch goroutine and the
  HTTP handlers recover panics, emit a structured `panic_recovered`
  log event with the stack, increment
  `clockify_mcp_panics_recovered_total{site}`, and return a tool-error
  envelope instead of crashing the process. See
  [`SECURITY.md`](../SECURITY.md) "Security Features".
- **Audit durability** — every non-read-only tool call emits an
  `AuditEvent`. Persistence failures are logged at `ERROR` and
  increment `clockify_mcp_audit_failures_total{reason="persist_error"}`.
  Set `MCP_AUDIT_DURABILITY=fail_closed` to abort tool calls on audit
  failure (default `best_effort` succeeds and alerts via metrics/logs).
  See [`SECURITY.md`](../SECURITY.md) "Inline /metrics security" and
  "Audit durability".
- **Inline /metrics access control** — `/metrics` on the main HTTP
  listener is off by default (`MCP_HTTP_INLINE_METRICS_ENABLED`). When
  enabled, `inherit_main_bearer` (default) reuses the primary bearer
  token; `static_bearer` accepts a separate token; `none` requires
  explicit opt-in and logs a startup warning. Use `MCP_METRICS_BIND`
  for the recommended side-channel listener in shared-service
  deployments.
- **PII-redacting logs** — the default slog handler is wrapped in
  `internal/logging.RedactingHandler`, which masks 20+ well-known
  secret-key patterns before encoding. See
  [`internal/logging/redact.go`](../internal/logging/redact.go).
- **Response limits** — 10MB on Clockify API responses, 2MB default
  on HTTP request bodies. Tunable via `MCP_HTTP_MAX_BODY` and
  `CLOCKIFY_REPORT_MAX_ENTRIES`.
- **Tool annotations** — every tool carries `readOnlyHint`,
  `destructiveHint`, `idempotentHint`, `openWorldHint`, and `title`
  so spec-strict MCP clients can render correct safety hints. See
  [`internal/tools/`](../internal/tools/) handlers.
- **Live contract testing** — nightly read-only + opt-in mutating
  tests against a sacrificial Clockify workspace surface upstream
  drift as a `live-test-failure` GitHub issue. See
  [`docs/live-tests.md`](live-tests.md).
- **Coverage policy** — every PR must hold the floor for every
  gated package. Ratchet rule documented in
  [`docs/coverage-policy.md`](coverage-policy.md).

## Governance

Single-maintainer project today (`@apet97`). Self-merge is permitted
because there is no alternative reviewer; the audit trail comes from
signed commits, public CI logs, and SLSA build provenance on every
release. See [`GOVERNANCE.md`](../GOVERNANCE.md) for the honest
write-up and [`docs/branch-protection.md`](branch-protection.md) for
the snapshot of the actual GitHub branch-protection rules on `main`.

## Performance envelope

Reference numbers (M1 reference machine), recommended operating
envelope by workload size, and the hot-path microbenchmarks for
regression detection live in [`docs/performance.md`](performance.md).
The numbers are a snapshot, not an SLO.

## Where to file something

- **Bug or feature request** — GitHub Issues
  ([`apet97/go-clockify/issues`](https://github.com/apet97/go-clockify/issues)).
- **Security vulnerability** — GitHub Security Advisory, private
  channel. Process and response timeline in
  [`SECURITY.md`](../SECURITY.md).
- **Production-readiness gap in this page** — open an issue and
  link the page section. The point of this document is to be the
  single-entry answer; if it failed, that's a bug.
