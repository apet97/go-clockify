# 0003 - Auth mode negotiation

## Status

Accepted — the four-mode contract has been stable since the OAuth 2.1
Resource Server completion (commit `9e6a6ff`, W1-06).

## Context

The four transports defined in ADR 0002 each trust a different thing:
`stdio` trusts its parent process, `http` is a single-tenant bearer
service, `streamable_http` is a multi-tenant OIDC service, and `grpc`
is a private-network RPC. We need an auth-mode taxonomy that maps
cleanly onto each transport without producing nonsensical
combinations (e.g. forward-auth on stdio, mTLS without TLS).

The existing operator UX is `MCP_AUTH_MODE`, set independently of
`MCP_TRANSPORT`. Validation must reject invalid combinations
fast-fail at startup so a misconfigured deployment cannot silently
serve traffic with the wrong trust model.

## Decision

Four named auth modes, configured via `MCP_AUTH_MODE` and validated
in `internal/config/config.go` `Load()` (search for
`cfg.AuthMode = strings.TrimSpace(os.Getenv("MCP_AUTH_MODE"))`):

| Mode            | Trusts                                                | Wire-level mechanism                                 |
|-----------------|-------------------------------------------------------|------------------------------------------------------|
| `static_bearer` | A ≥16-char shared secret in `MCP_BEARER_TOKEN`.       | `Authorization: Bearer <token>`, `crypto/subtle` compare. |
| `oidc`          | A JWT signed by `MCP_OIDC_ISSUER`'s JWKS.             | `Authorization: Bearer <jwt>`, RFC 9728 JWKS validation. |
| `forward_auth`  | An upstream proxy that has already authenticated.     | A configured forward-auth header (e.g. `X-Forwarded-User`). |
| `mtls`          | A TLS client certificate against a private CA.        | `tls.ConnectionState.VerifiedChains`.               |

Compatibility matrix (enforced in `internal/config/config.go`):

| Transport         | `static_bearer` | `oidc` | `forward_auth` | `mtls` | No auth |
|-------------------|:---------------:|:------:|:--------------:|:------:|:-------:|
| `stdio`           | reject          | reject | reject         | reject | **only valid choice** |
| `http`            | yes (default)   | yes    | yes            | yes    | reject  |
| `streamable_http` | yes             | yes (default) | yes     | yes    | reject  |
| `grpc`            | yes             | yes    | yes (W5-05b)   | yes (W5-05c) | reject |

Specific rules:

- `stdio` rejects every value of `MCP_AUTH_MODE`. The check returns
  an explicit error: "MCP_AUTH_MODE is only valid for HTTP transports"
  (search for that string in `config.go`).
- `static_bearer` requires `MCP_BEARER_TOKEN` to be set and at least
  16 characters long. Both checks live in `config.go` (search for
  `if len(cfg.BearerToken) < 16`).
- `streamable_http` defaults to `oidc` when `MCP_AUTH_MODE` is unset.
- `http` defaults to `static_bearer` when `MCP_AUTH_MODE` is unset.
- `grpc` accepts all four modes after W5-05a/b/c. mTLS and forward
  auth on gRPC piggy-back on TLS `ConnectionState` and gRPC metadata
  respectively.

The shared `internal/authn.Authenticator` interface implements all
four modes so that the HTTP transport, the streamable HTTP transport,
and the gRPC transport can use the same authentication logic via
their respective interceptors (see ADR 0008 for the gRPC interceptor
wiring).

## Consequences

### Positive

- Operators get a single env var (`MCP_AUTH_MODE`) per choice. The
  full matrix is documented in `docs/production-readiness.md` "Pick
  an auth mode".
- Invalid combinations are caught at startup via `config.Load`
  rather than at first request.
- A single `internal/authn.Authenticator` powers all transports, so
  a security fix lands once and protects every code path.
- Adding a new auth mode is mechanical: add a case to the switch in
  `config.go`, implement the `Authenticator` interface, decide which
  transports support it, and update this ADR's matrix.

### Negative

- The matrix has corners (which combinations are valid) that an
  operator must read before deploying. This is unavoidable given
  the variety of transports.
- `streamable_http`'s default of `oidc` means an operator who flips
  `MCP_TRANSPORT=streamable_http` without setting any other env vars
  will fail-fast at startup until they configure an OIDC issuer. We
  consider this the safer default than silently allowing
  `static_bearer` for a multi-tenant transport.

### Neutral

- The bearer-token minimum length (16 chars) is enforced for both
  `MCP_BEARER_TOKEN` and `MCP_METRICS_BEARER_TOKEN`. The threshold
  is a defence-in-depth heuristic, not a cryptographic guarantee —
  operators who care about strong tokens should generate them with
  at least 32 random bytes regardless.

## Alternatives considered

- **A single "auto" mode that introspects request headers** —
  rejected because ambiguous requests would then fall through to
  whichever check happens to match first, hiding misconfiguration.
- **Per-tenant auth mode via the control plane** — rejected for now
  because the operational complexity of managing per-tenant trust
  models exceeds the benefit. The control plane already scopes per-
  tenant config; adding per-tenant authn would couple two layers.
- **Drop `forward_auth` and rely only on the upstream proxy's
  network controls** — rejected because forward-auth is the only
  mode that lets operators delegate authentication entirely to a
  trusted upstream (Caddy, Envoy, Traefik) without re-validating in
  the MCP server.

## References

- Validation: `internal/config/config.go` `Load()` — the auth-mode
  switch and the per-transport compatibility checks (search for
  `cfg.AuthMode = strings.TrimSpace(os.Getenv("MCP_AUTH_MODE"))`).
- Token-length checks: `internal/config/config.go` (search for
  `if len(cfg.BearerToken) < 16`).
- Authenticator implementations: `internal/authn/`.
- gRPC mode wiring: `internal/transport/grpc/transport.go` (search
  for `if opts.Authenticator != nil {`).
- Related ADRs: 0002 (transport selection), 0008 (gRPC interceptor).
- Related docs: `docs/production-readiness.md` "Pick an auth mode",
  `docs/runbooks/auth-failures.md`.
- Spec: <https://datatracker.ietf.org/doc/html/rfc9728> (OAuth 2.0
  Protected Resource Metadata, used by the `oidc` mode).
