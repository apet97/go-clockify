# Security Hardening Design — 2026-04-25

## Context

A comprehensive security audit of `github.com/apet97/go-clockify` (GOCLMCP) produced findings
ranging from Critical to Low. This document captures the agreed remediation design for all
findings, structured as two phases of work.

**Goal:** Phase 1 fixes correctness and safety bugs (ships quickly). Phase 2 hardens the server
for paid/public hosted-service deployment.

**mTLS decision:** Native TLS/mTLS will be implemented on the streamable HTTP transport (Option B),
consistent with how gRPC already handles it. The `forward_mtls` pattern (header trust) was
rejected to avoid introducing a new header-injection attack class.

---

## PR sequence

| PR | Phase | Findings | Contents |
|----|-------|----------|----------|
| 0 | 1 | H7, H2, M2, M1 | Hot-fix wave: panic leak, docs drift |
| 1 | 1 | H3, H4 | Native TLS/mTLS on streamable HTTP + gRPC cert validation |
| 2 | 2 | C1, H6 | OIDC strict mode + tenant claim enforcement |
| 3 | 2 | H1, M3 | Deployment defaults (Docker, Helm, Kustomize) |
| 4 | 2 | H5 | Audit pre-mutation intent |
| 5 | 2 | M7, M4, L1–L3 | Hosted hardening miscellany |

---

## Phase 1

### PR 0 — Hot-fix wave

**Scope:** Zero architectural change. All changes are in existing code paths or docs.

#### H7 — Panic value leak (`internal/mcp/server.go`)

The stdio panic-recovery path currently returns the raw panic value to the MCP client:

```go
// Before
"text": "tool panic: " + fmt.Sprintf("%v", rec),

// After
"text": "internal tool error; request logged",
```

The full panic value and stack continue to go to `slog.Error("panic_recovered", ...)`.
Nothing is lost from observability. The client receives a generic message that cannot
leak internal state, keys, or request data.

#### H2 — Protocol version in README

The README support matrix row for "MCP Protocol" advertises `2025-06-18`. The server's
`SupportedProtocolVersions[0]` is `2025-11-25`. Update the README row to `2025-11-25`.

#### M2 — Health endpoint names in production docs

Production docs reference `/healthz` and `/readyz`. The actual registered routes are
`/health` and `/ready` (both legacy and streamable HTTP transports). Update all references
in `docs/` to match the implementation.

#### M1 — Version matrix in SECURITY.md / SUPPORT.md

Align the supported-version table with CHANGELOG's current state: v1.1.x unreleased,
v1.0.x current stable.

#### Tests added in PR 0

- `TestPanicResponseDoesNotExposePanicValue` — registers an in-process tool that panics
  with a string containing a fake secret (`"sk-secret-12345"`), invokes it via the stdio
  dispatcher, asserts MCP response `content[0].text` == `"internal tool error; request
  logged"` and does not contain the fake secret string. Lives in `internal/mcp/`.
- `TestReadmeProtocolBadgeMatchesSupportedProtocolVersions` — parses the README support
  matrix row, extracts the version string, asserts it equals `mcp.SupportedProtocolVersions[0]`.
  Lives in `tests/doc_parity_test.go` alongside the existing config-doc parity gate.

---

### PR 1 — Transport hardening

**Scope:** Native TLS/mTLS on the streamable HTTP transport; fail-fast config validation
requiring cert material when `mtls` auth mode is selected on any network transport.
Legacy HTTP (`MCP_TRANSPORT=http`) is explicitly excluded — it does not get TLS in this PR.

#### New env vars (`internal/config/spec.go`)

| Var | Purpose |
|-----|---------|
| `MCP_HTTP_TLS_CERT` | Path to server TLS certificate PEM for the HTTP transport |
| `MCP_HTTP_TLS_KEY` | Path to server TLS private key PEM for the HTTP transport |

`MCP_MTLS_CA_CERT_PATH` already exists and is shared between gRPC and HTTP transports.
`go run ./cmd/gen-config-docs -mode=all` regenerates `help_generated.go` and the README
CONFIG-TABLE after this change.

#### TLS wiring in `ServeStreamableHTTP`

After `net.Listen`, before `srv.Serve`:

```go
ln, err := net.Listen("tcp", addr)
if err != nil { return err }

if cfg.HTTPTLSCert != "" {
    cert, err := tls.LoadX509KeyPair(cfg.HTTPTLSCert, cfg.HTTPTLSKey)
    if err != nil {
        return fmt.Errorf("load HTTP TLS cert: %w", err)
    }
    tlsCfg := &tls.Config{
        Certificates: []tls.Certificate{cert},
        MinVersion:   tls.VersionTLS12,
    }
    if cfg.MTLSCACertPath != "" {
        pool, err := loadCACertPool(cfg.MTLSCACertPath)
        if err != nil { return err }
        tlsCfg.ClientCAs = pool
        tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
    }
    ln = tls.NewListener(ln, tlsCfg)
}
return srv.Serve(ln)
```

`loadCACertPool` is a small shared helper (reads PEM, calls
`x509.NewCertPool().AppendCertsFromPEM`). It is extracted into `internal/tlsutil/` so
both the HTTP and gRPC transports can use it without import cycles.

With a TLS listener in place, `r.TLS.VerifiedChains` is populated normally, so
`mtlsAuthenticator.Authenticate` works without any code changes.

#### Fail-fast config validation (`internal/config/`)

Added to `config.Load()`, checked before any transport starts:

```
streamable_http + mtls  →  require MCP_HTTP_TLS_CERT, MCP_HTTP_TLS_KEY, MCP_MTLS_CA_CERT_PATH
grpc + mtls             →  require MCP_GRPC_TLS_CERT, MCP_GRPC_TLS_KEY, MCP_MTLS_CA_CERT_PATH
```

Error messages name the missing variable explicitly. Both checks fire at startup, not at
first request. Setting `MCP_HTTP_TLS_CERT` with `MCP_TRANSPORT=http` (legacy) is a
config error — legacy HTTP does not support TLS.

#### Support matrix correction

`internal/config/transport_auth_matrix_test.go`: move `streamable_http + mtls` from the
unsupported cell to supported. Add comment: *"requires MCP_HTTP_TLS_CERT + MCP_HTTP_TLS_KEY
+ MCP_MTLS_CA_CERT_PATH; plain HTTP (no cert) + mtls is rejected by config.Load"*.

Helm `values.yaml` gains two new commented-out keys under `transport:`:

```yaml
httpTlsCert: ""   # MCP_HTTP_TLS_CERT (path to server cert PEM)
httpTlsKey: ""    # MCP_HTTP_TLS_KEY  (path to server key PEM)
```

#### Tests added in PR 1

| Test | What it checks |
|------|----------------|
| `TestStreamableMTLSRequiresCertKeyAndCA` | config.Load with `streamable_http + mtls` + missing cert/key/CA returns error naming the missing var |
| `TestGRPCMTLSRequiresCertKeyAndCA` | same for gRPC transport |
| `TestStreamableHTTPNativeTLS` | start with self-signed cert, TLS client connects, `/health` returns 200 |
| `TestStreamableHTTPNativeMTLS` | valid client cert → authenticated principal; no client cert → TLS handshake rejection |
| `TestLegacyHTTPRejectsTLSCert` | `MCP_HTTP_TLS_CERT` with `MCP_TRANSPORT=http` fails config.Load |

Integration tests use `crypto/tls` in-process with `httptest`-generated self-signed certs.
No external tooling required.

---

## Phase 2

### PR 2 — OIDC strict mode + tenant isolation (C1, H6)

#### C1 — Audience/resource enforcement

New env var: `MCP_OIDC_STRICT=1`.

When `MCP_OIDC_STRICT=1`, `config.Load()` rejects OIDC configuration where both
`MCP_OIDC_AUDIENCE` and `MCP_RESOURCE_URI` are empty:

```
oidc + no audience + no resource URI + MCP_OIDC_STRICT=1  →  config load error
```

`validateClaims` gains one additional check under strict mode:

```
exp == 0 (no expiry claim present) + MCP_OIDC_STRICT=1  →  token rejected
```

This closes the gap where a token issued without an `exp` claim passes the existing
`claims.Expires != 0 && now >= claims.Expires` guard. For the existing per-field audience
and resource URI checks, no change is needed — they already enforce both when set. The
fix is that "at least one must be set" becomes a startup-time error and "exp must be
present" becomes a runtime claim rejection in strict mode. The production-profile example
and hosted-service docs always set `MCP_OIDC_STRICT=1`.

#### H6 — Tenant claim enforcement

New env var: `MCP_REQUIRE_TENANT_CLAIM=1`.

When set, `oidcAuthenticator.Authenticate` returns an auth error if the tenant claim is
absent from the token — instead of falling back to `DefaultTenantID`. The default
(fallback allowed) is preserved for backward compatibility with self-hosted deployments.

The hosted-service profile sets both `MCP_OIDC_STRICT=1` and `MCP_REQUIRE_TENANT_CLAIM=1`.

#### Tests added in PR 2

- `TestOIDCStrictRejectsNoAudienceOrResource`
- `TestOIDCRequiresTenantClaimWhenFlagSet`
- `TestOIDCDefaultFallbackUnchangedWithoutFlag`
- `TestOIDCStrictPassesWithAudienceSet`
- `TestOIDCStrictPassesWithResourceURISet`

---

### PR 3 — Deployment defaults (H1, M3)

#### Dockerfile

- `ENV MCP_TRANSPORT=http` → `ENV MCP_TRANSPORT=streamable_http`
- Add `ENV MCP_STRICT_HOST_CHECK=1`
- Add inline comment explaining why streamable_http is the recommended network transport

#### Helm `values.yaml`

- `transport.mode: "http"` → `"streamable_http"`
- `transport.strictHostCheck: "0"` → `"1"`
- `config.CLOCKIFY_POLICY: "standard"` → `"safe_core"`

#### Kustomize

- Base deployment changes `MCP_TRANSPORT` to `streamable_http`
- Legacy HTTP overlay extracted to `deploy/kustomize/overlays/legacy-http/` with a
  comment: *"Not recommended for new deployments. Retained for backward compatibility."*

#### Tests added in PR 3

- `TestHelmDefaultTransportIsStreamable` — render Helm chart with default values, assert
  no `MCP_TRANSPORT=http` in the rendered Deployment env
- `TestDockerfileTransportNotLegacy` — parse Dockerfile, assert `ENV MCP_TRANSPORT` ≠ `http`
- `TestKustomizeBaseTransportNotLegacy` — render base kustomize, assert no `http` transport
- `TestHelmDefaultPolicyIsSafeCore` — assert `CLOCKIFY_POLICY` ≠ `standard` in defaults

---

### PR 4 — Audit pre-mutation intent (H5)

#### The semantic change

`callTool` currently: execute handler → record audit (for successful non-read-only calls).
With `fail_closed`, the mutation already happened if audit persistence fails.

New flow:

```
1. recordAuditIntent(tool, params)
   └─ fail_closed + intent failure  →  return error, skip handler
   └─ best_effort + intent failure  →  log warning, continue
2. result, err = handler(ctx, params)
3. recordAuditOutcome(tool, params, result, err)
```

`recordAuditIntent` writes a minimal "attempted" record with tool name, params, timestamp,
and principal. `recordAuditOutcome` appends result status (succeeded/failed) and serialised
error message if any.

#### Interface change

The audit store interface gains a phase field or separate `Intent`/`Outcome` methods.
In-memory and file-based backends implement the new interface. Postgres backend (if any)
gets a migration adding a `phase` column.

Tools that make no state-mutating Clockify API calls (GET-only paths) bypass the intent
record — there is no mutation to guard.

#### Backward compatibility

`best_effort` mode behaviour is unchanged — intent failures are logged but do not block
the handler. `fail_closed` gains the pre-mutation guarantee. No env var changes; the
existing `MCP_AUDIT_DURABILITY` values continue to work.

#### Tests added in PR 4

- `TestFailClosedAuditPreventsHandlerOnIntentFailure`
- `TestBestEffortAuditDoesNotBlockHandler`
- `TestAuditIntentAndOutcomeEmittedOnSuccess`
- `TestAuditIntentEmittedOutcomeMarksFailedOnHandlerError`
- `TestNonMutatingToolsSkipAuditIntent`

---

### PR 5 — Hosted hardening miscellany (M7, M4, L1–L3)

#### M7 — Live contract tests required on main repo

In `.github/workflows/live.yml`, change the missing-secrets behaviour based on repository:

```yaml
- name: Skip or fail on missing secrets
  run: |
    if [[ "${{ github.repository }}" == "apet97/go-clockify" ]]; then
      echo "::error::Live secrets required on main repo" && exit 1
    else
      echo "::warning::Skipping live tests (fork or secrets not configured)"
    fi
  if: env.CLOCKIFY_API_KEY == ''
```

Forks continue to skip gracefully. The main repo fails the workflow if secrets are absent.

#### M4 — `time_tracking_safe` policy mode

New policy mode covering only: start timer, stop timer, log time entry, update own time
entry, read time entries. Does not allow project/client/tag/task creation or deletion.

The hosted-service profile example uses `time_tracking_safe` as the default policy for
untrusted agents. `safe_core` retains its current breadth. Policy docs table is updated.

#### L1 — Docker healthcheck

Add a Kubernetes `httpGet` probe example to `deploy/` that hits `/health`. The existing
`--version` exec probe in the Dockerfile stays as a startup/liveness check. The README
k8s section documents both patterns and when to use each.

#### L2 — Branch protection docs

Update governance docs to describe the target state before paid launch: 1 required
approval, CODEOWNERS review enabled, signed commits or squash-merge with provenance.
No code change — tracking note added to `docs/adr/` as a decision record.

#### L3 — Inline vault backend opt-out

New env var: `MCP_DISABLE_INLINE_SECRETS=1`.

When set, `config.Load()` rejects any credential ref with `backend=inline`. The
hosted-service profile example sets it. Self-hosted users are unaffected unless they
opt in. Key rotation workflow documented in `docs/runbooks/`.

#### Tests added in PR 5

- `TestTimeTackingSafePolicyAllowsStartStop`
- `TestTimeTrackingSafePolicyBlocksProjectCreate`
- `TestInlineSecretsRejectedWhenFlagSet`
- `TestInlineSecretsAllowedWithoutFlag`

---

## What blocks paid/public hosted service (summary)

After all six PRs:

| Blocker | Resolved by |
|---------|-------------|
| OIDC accepts issuer-only tokens | PR 2 (`MCP_OIDC_STRICT=1`) |
| Tenant claim fallback into default tenant | PR 2 (`MCP_REQUIRE_TENANT_CLAIM=1`) |
| mTLS claims inconsistent with transport | PR 1 (native TLS on streamable HTTP) |
| Deployment defaults use legacy HTTP + standard policy | PR 3 |
| Audit fail-closed does not prevent mutation | PR 4 |
| Inline secrets in control-plane records | PR 5 (`MCP_DISABLE_INLINE_SECRETS=1`) |
| Docs drift (protocol version, health endpoints, version matrix) | PR 0 |
