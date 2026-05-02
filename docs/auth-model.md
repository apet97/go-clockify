# Auth model

A reviewer-facing one-page summary of how `clockify-mcp`
authenticates inbound requests, derives a tenant identity, and
constructs the `Principal` object that every downstream handler
trusts. The promise: a Clockify engineer who has never read this
codebase should be able to answer the questions at the bottom of
this page in **under five minutes** without opening another file.

For decisional rationale (why we chose this shape over
alternatives) read [`adr/0003-auth-mode-negotiation.md`](adr/0003-auth-mode-negotiation.md),
[`adr/0008-grpc-auth-interceptor.md`](adr/0008-grpc-auth-interceptor.md),
and [`adr/0017-streamable-http-session-rehydration.md`](adr/0017-streamable-http-session-rehydration.md).
For operator triage during an incident read
[`runbooks/auth-failures.md`](runbooks/auth-failures.md).

## Three independent auth layers

`go-clockify` authenticates at three layers, and an "auth is
broken" symptom can come from any of them:

```
   ┌──────────────────────────┐  ┌──────────────────────┐  ┌──────────────────────────┐
   │  Inbound MCP auth        │  │  Upstream Clockify   │  │  gRPC stream re-auth     │
   │  (this document)         │  │  API key             │  │  (build tag `grpc`)      │
   ├──────────────────────────┤  ├──────────────────────┤  ├──────────────────────────┤
   │ MCP_AUTH_MODE            │  │ CLOCKIFY_API_KEY     │  │ MCP_GRPC_REAUTH_INTERVAL │
   │   static_bearer          │  │ (or per-installation │  │ Re-runs the inbound      │
   │   oidc                   │→→│  token in HTTP       │→→│ Authenticator on         │
   │   forward_auth           │  │  multi-tenant)       │  │ long-lived bidi          │
   │   mtls                   │  │ Sent as X-Api-Key to │  │ streams.                 │
   │ Produces a Principal     │  │ api.clockify.me      │  │                          │
   └──────────────────────────┘  └──────────────────────┘  └──────────────────────────┘
```

This document covers **only the leftmost layer** — the inbound
MCP authentication that turns a wire-format request into an
[`authn.Principal`](../internal/authn/authn.go) carried through
the request context.

## The four inbound auth modes

`stdio` is a transport, not an auth mode — it has no inbound auth
because the parent process is trusted (set
`MCP_AUTH_MODE` on `stdio` and config load fails closed; pinned
by [`internal/config/transport_auth_matrix_test.go`](../internal/config/transport_auth_matrix_test.go) lines 35-39).

The four modes that authenticate HTTP / gRPC requests are
defined as `Mode` constants in
[`internal/authn/authn.go`](../internal/authn/authn.go) lines 36-41:

| Mode | When to use | What it trusts |
|---|---|---|
| `static_bearer` | Fixed roster of clients distribute one shared secret out of band. | `MCP_BEARER_TOKEN` env var (≥ 16 chars), compared with `crypto/subtle`. |
| `oidc` | Clients present a JWT signed by an OIDC provider you trust (Auth0, Okta, Keycloak). **Default for `streamable_http`.** | `MCP_OIDC_ISSUER`'s JWKS endpoint and configured audience / `MCP_RESOURCE_URI`. |
| `forward_auth` | An upstream reverse proxy (Caddy, Envoy, Traefik) already terminates auth and forwards the result via headers. | `MCP_FORWARD_SUBJECT_HEADER` + `MCP_FORWARD_TENANT_HEADER` set by a trusted proxy (CIDR allow-list at `MCP_FORWARD_AUTH_TRUSTED_PROXIES`). |
| `mtls` | Both ends present X.509 certs against a private CA. **Required for the `private-network-grpc` profile.** | TLS client cert presented to the listener; tenant from URI SAN or Subject.Organization. |

Per-transport gating is asserted by
[`TestTransportAuthMatrix`](../internal/config/transport_auth_matrix_test.go) —
every cell in `{transport × auth_mode}` either loads cleanly or
fails with a config-level error naming the mismatch. The default
when `MCP_AUTH_MODE` is unset is `static_bearer` for legacy `http`
and `oidc` for `streamable_http` and `grpc`.

## Principal construction

The `Principal` struct lives at
[`internal/authn/authn.go`](../internal/authn/authn.go) lines
43-49 and carries five fields: `Subject`, `TenantID`, `AuthMode`,
`Claims`, `SessionID`. `SessionID` is set later by the
streamable-HTTP session manager — every authenticator only fills
the first four.

| Mode | Subject source | TenantID source | When can either be empty? |
|---|---|---|---|
| `static_bearer` | hardcoded `"static-bearer"` (a stand-in identifier — every caller bearing the same token has the same Subject by design) | `MCP_DEFAULT_TENANT_ID` (default `"default"`) | Neither is empty. |
| `oidc` | JWT `MCP_SUBJECT_CLAIM` (default `sub`) | JWT `MCP_TENANT_CLAIM` (default `tenant_id`); falls back to `MCP_DEFAULT_TENANT_ID` unless `MCP_REQUIRE_TENANT_CLAIM=1` | **Subject:** `oidc token missing subject claim` if absent. **TenantID:** `oidc token missing tenant claim` iff strict required. |
| `forward_auth` | `MCP_FORWARD_SUBJECT_HEADER` (default `X-Forwarded-User`), sanitized | `MCP_FORWARD_TENANT_HEADER` (default `X-Forwarded-Tenant`), sanitized; falls back to default | **Subject:** `missing <header>` if header absent or sanitization rejects. **TenantID:** empty header → fallback (never the rejection path). |
| `mtls` | client cert `Subject.CommonName`, fallback to full `Subject.String()` | per `MCP_MTLS_TENANT_SOURCE` (`cert` default → URI SAN `clockify-mcp://tenant/<id>` or `spiffe://*/tenant/<id>`, fallback `Subject.Organization[0]`); `header` and `header_or_cert` are migration variants | **Subject:** never empty (cert CN fallback). **TenantID:** `mtls client has no tenant identity (source=<src>)` iff `MCP_REQUIRE_MTLS_TENANT=1`. |

`Claims` is a small `map[string]string` carrying mode-specific
metadata (e.g. OIDC `issuer` + `audience`, mTLS `cert_subject` +
`tenant_source`). It is **not** the place to look for an arbitrary
JWT claim; only the curated set the authenticator promotes
survives. Tools that need extra claim data should add an explicit
`Config` knob and a sanitization step rather than reach into the
map.

## Tenant resolution

Tenant identity is a **field on Principal** — there is no
separate `tenant` package. The flow is:

1. The authenticator reads its mode-specific source (claim,
   header, SAN, default).
2. The authenticator returns a `Principal` with `TenantID`
   populated.
3. The MCP handler (or the gRPC interceptor) attaches that
   Principal to the request context via
   [`authn.WithPrincipal`](../internal/authn/context.go).
4. Every downstream tool reads
   [`authn.PrincipalFromContext`](../internal/authn/context.go)
   and operates within `principal.TenantID`'s scope (audit row
   keying, session pinning, control-plane reads).

The streamable-HTTP session manager pins
`principal.TenantID` into the persisted `SessionRecord` at
session creation time (`internal/mcp/transport_streamable_http.go`
`streamSessionManager.create`). On a cross-pod rehydration
(ADR 0017 Path A), the rebuilt session **strict-compares** the
freshly-authenticated Principal's `Subject` and `TenantID`
against the persisted record; mismatch returns 403 "session
principal mismatch" — see
[`production-readiness.md` § Session rehydration](production-readiness.md#session-rehydration-streamable-http-multi-replica)
and [`clients.md` § Session Rehydration Boundaries](clients.md#session-rehydration-boundaries).

## Failure modes

Categorised by [`authn.FailureCategory`](../internal/authn/category.go),
returned over the wire by
[`authn.WriteUnauthorized`](../internal/authn/oauth_resource.go)
(HTTP) or the gRPC interceptor at
[`internal/transport/grpc/auth.go`](../internal/transport/grpc/auth.go).

| Symptom | HTTP status | gRPC status | Error string fragment | Category |
|---|---|---|---|---|
| Missing `Authorization` header (any mode that requires bearer) | 401 | `Unauthenticated` | `missing bearer token`, `missing authorization` | `missing_credentials` |
| Bearer comparison failure (`static_bearer`) | 401 | `Unauthenticated` | `invalid bearer token` | `invalid_token` |
| OIDC token expired | 401 | `Unauthenticated` | `token expired` | `expired_token` |
| OIDC issuer mismatch | 401 | `Unauthenticated` | `unexpected issuer` | `token_verification` |
| OIDC audience / resource mismatch | 401 | `Unauthenticated` | `unexpected audience`, `token aud does not contain resource URI` | `audience_mismatch` |
| OIDC strict mode rejects token without `exp` | 401 | `Unauthenticated` | `token missing exp claim (strict mode)` | `token_verification` |
| OIDC tenant claim missing under `MCP_REQUIRE_TENANT_CLAIM=1` | 401 | `Unauthenticated` | `oidc token missing tenant claim` | `tenant_claim` |
| `forward_auth` source not in CIDR allow-list | 401 | `Unauthenticated` | `forward_auth: source X.X.X.X not in MCP_FORWARD_AUTH_TRUSTED_PROXIES allow-list` | `invalid_token` |
| `forward_auth` header carries control byte / non-printable Unicode | 401 | `Unauthenticated` | `forward_auth: <subject\|tenant> contains disallowed byte 0x<hex>` | `invalid_token` |
| `forward_auth` subject header empty/missing | 401 | `Unauthenticated` | `missing <header>` | `missing_credentials` |
| `mtls` client did not present a verified cert | 401 | `Unauthenticated` | `verified mTLS client certificate required`, `missing client certificate` | `client_certificate` |
| `mtls` tenant required but unresolvable | 401 | `Unauthenticated` | `mtls client has no tenant identity (source=<src>)` | `tenant_claim` |
| Cross-pod replay with mismatched Subject/TenantID (streamable-HTTP only) | 403 | n/a | `session principal mismatch` | n/a (sentinel `errSessionPrincipalMismatch`) |

HTTP responses also set `WWW-Authenticate: Bearer realm="clockify-mcp", error=<errCode>, error_description=<sanitized>`
per RFC 9728. The structured log line is
`msg=http_auth_failed status=<code> reason=auth_failed
auth_failed=<error> auth_failure_category=<bucket>` (see
[`internal/mcp/transport_auth_errors.go`](../internal/mcp/transport_auth_errors.go)
lines 20-36).

## Test pins (every claim above is backed by code)

| Claim in this doc | Test |
|---|---|
| Mode constants and per-mode happy path | [`internal/authn/authn_test.go`](../internal/authn/authn_test.go) — `TestStaticBearerAuthenticate`, `TestForwardAuthAuthenticate`, `TestMTLSAuthenticate` (+ 7 mTLS variants) |
| Per-transport mode gating (the `{transport × auth_mode}` matrix) | [`internal/config/transport_auth_matrix_test.go`](../internal/config/transport_auth_matrix_test.go) — `TestTransportAuthMatrix` |
| HTTP-level auth surface (handler integration) | [`internal/mcp/transport_http_authmatrix_test.go`](../internal/mcp/transport_http_authmatrix_test.go), [`internal/mcp/transport_auth_errors_test.go`](../internal/mcp/transport_auth_errors_test.go) |
| gRPC interceptor behaviour, including stream re-auth | [`internal/transport/grpc/auth_test.go`](../internal/transport/grpc/auth_test.go) |
| `forward_auth` control-byte / non-printable rejection | [`internal/authn/auth_hardening_test.go`](../internal/authn/auth_hardening_test.go) — `TestForwardAuth_RejectsControlBytesInHeaders` |
| `forward_auth` trusted-proxy CIDR gate | [`internal/authn/auth_hardening_test.go`](../internal/authn/auth_hardening_test.go) — `TestForwardAuth_RejectsUntrustedSource`, `TestForwardAuth_AcceptsTrustedCIDR`, `TestForwardAuth_EmptyAllowlistPreservesLegacyBehaviour` |
| OIDC strict mode rejects HTTP issuer / JWKS | [`internal/authn/auth_hardening_test.go`](../internal/authn/auth_hardening_test.go) — `TestNewOIDCAuth_StrictRejectsHTTPIssuer`, `TestNewOIDCAuth_StrictRejectsHTTPJWKS` |
| OIDC verify-cache TTL clamp | [`internal/authn/oidc_verify_cache_test.go`](../internal/authn/oidc_verify_cache_test.go) |
| JWT alg-confusion (HS256 / `none`) rejection | [`internal/authn/jwt_alg_confusion_test.go`](../internal/authn/jwt_alg_confusion_test.go) |
| JWKS document parsing (RSA modulus floor, EC curve validation) | [`internal/authn/jwks_document_test.go`](../internal/authn/jwks_document_test.go) |
| Failure-category classification | [`internal/authn/category_test.go`](../internal/authn/category_test.go) |
| Principal context plumbing (`WithPrincipal` / `PrincipalFromContext`) | [`internal/authn/context_test.go`](../internal/authn/context_test.go) |
| mTLS tenant from URI SAN / Subject.Organization fallback | [`internal/authn/authn_test.go`](../internal/authn/authn_test.go) — `TestMTLSTenantFromCertificateURI`, `TestMTLSTenantFromCertificateOrganizationFallback` |
| Cross-pod strict re-auth (the streamable-HTTP rehydration boundary) | [`internal/controlplane/postgres/e2e_session_rehydration_test.go`](../internal/controlplane/postgres/e2e_session_rehydration_test.go) — `TestStreamableHTTPCrossInstanceRehydration` (cross-tenant 403 case) |

## Edge cases worth knowing

- **`forward_auth` empty allow-list** preserves the legacy
  "trust every source" posture for self-hosted single-tenant
  deployments. Hosted profiles plus `clockify-mcp doctor --strict`
  refuse to start with `forward_auth` + empty allow-list. Pin:
  `TestForwardAuth_EmptyAllowlistPreservesLegacyBehaviour`.
- **`forward_auth` sanitization** rejects `utf8.RuneError`,
  control bytes, and any rune that fails `unicode.IsPrint()`.
  ASCII space passes (legitimate in display names / org names).
  Applied to **both** subject and tenant headers.
- **mTLS `header_or_cert` mode** is a migration-window hybrid
  (header takes precedence, cert is the fallback). Only safe
  behind a proxy that strips client-supplied tenant headers,
  because a client-controlled header can otherwise impersonate
  any tenant. Default is `cert` for that reason.
- **OIDC strict mode** (`MCP_OIDC_STRICT=1`) makes two
  config-load assertions in one knob: (a) refuses to boot when
  `MCP_AUTH_MODE=oidc` without either `MCP_OIDC_AUDIENCE` or
  `MCP_RESOURCE_URI` set; (b) rejects tokens missing the `exp`
  claim at runtime.
- **`MCP_EXPOSE_AUTH_ERRORS`** is a dev-only knob — when set, the
  HTTP error envelope's `error_description` carries the raw
  authenticator error string. In hosted profiles the field is
  scrubbed to a generic `authentication failed` message; the raw
  reason still appears in the structured server log under
  `auth_failed=`.
- **gRPC stream re-auth** (`MCP_GRPC_REAUTH_INTERVAL`, default
  `0` = disabled) re-runs the same Authenticator on a long-lived
  bidirectional stream at the configured interval. Failures
  surface as `clockify_mcp_grpc_auth_rejections_total{reason="reauth_expired"}`
  and a `msg=grpc_reauth_failed` log line.
- **Per-subject rate limiting** (`CLOCKIFY_PER_TOKEN_RATE_LIMIT`,
  default 60 calls / 60s) keys off `principal.Subject` whenever a
  Principal is in context; the limit is enforced before the tool
  call dispatches and surfaces as
  `clockify_mcp_per_subject_rate_limited_total`.

## 5-question reviewer self-quiz

If you can answer these in five minutes using only this page,
the doc earned its keep.

1. **Which auth mode is the default for the `streamable_http`
   transport, and what does it trust?**
2. **A request to a `forward_auth`-mode listener arrives with
   `X-Forwarded-User: alice\nbob` (a literal newline). What
   happens, and which test pins that behaviour?**
3. **Where does `mtls` mode look for the tenant identifier when
   `MCP_MTLS_TENANT_SOURCE` is left at its default? What is the
   first fallback if that source is absent?**
4. **An OIDC token is replayed across a pod boundary in a
   streamable-HTTP deployment, but the token's `tenant_id` claim
   is different from the originally-authenticated session's
   tenant. What HTTP status is returned, and which file holds
   the test that pins this case?**
5. **Which env var, set to `1`, causes the server to refuse to
   boot when `MCP_AUTH_MODE=oidc` is selected without an
   audience or resource URI configured?**

Answers (compressed, scroll-resistant):

1. `oidc`; trusts `MCP_OIDC_ISSUER`'s JWKS + configured
   audience / `MCP_RESOURCE_URI`.
2. The request is rejected with HTTP 401 and the error
   `forward_auth: subject contains disallowed byte 0x0a`. Pinned
   by `TestForwardAuth_RejectsControlBytesInHeaders` in
   `internal/authn/auth_hardening_test.go`.
3. URI SAN (`clockify-mcp://tenant/<id>` or
   `spiffe://*/tenant/<id>`); fallback is the first
   `Subject.Organization` value on the cert.
4. HTTP 403 `session principal mismatch`; pinned by
   `TestStreamableHTTPCrossInstanceRehydration` in
   `internal/controlplane/postgres/e2e_session_rehydration_test.go`
   (the cross-tenant negative case).
5. `MCP_OIDC_STRICT=1`.

## See also

- [`adr/0003-auth-mode-negotiation.md`](adr/0003-auth-mode-negotiation.md) — decision record for the four-mode shape.
- [`adr/0008-grpc-auth-interceptor.md`](adr/0008-grpc-auth-interceptor.md) — gRPC stream re-auth and forward-auth-via-metadata.
- [`adr/0017-streamable-http-session-rehydration.md`](adr/0017-streamable-http-session-rehydration.md) — Q2 strict re-auth on the cross-pod rehydration boundary.
- [`production-readiness.md`](production-readiness.md) — operator-facing "Pick an auth mode" picker; cross-links back here.
- [`runbooks/auth-failures.md`](runbooks/auth-failures.md) — incident triage across the three layers.
- [`support-matrix.md`](support-matrix.md) — `{transport × auth_mode}` compatibility cells (mirrors the test).
- [`SECURITY.md`](../SECURITY.md) — threat model and the broader hardening posture.
