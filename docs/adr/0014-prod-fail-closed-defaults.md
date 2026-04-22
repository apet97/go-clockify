# 0014 - Production fail-closed defaults

## Status

Accepted — implemented across two commits on
`wave-h/streamable-fail-closed`:

1. `fix(config): streamable_http fails closed at load-time on dev DSN`
2. `fix(config): prod-default flips for legacy HTTP and audit durability`

## Context

`go-clockify` supports three deployment shapes along the safety
axis:

- **Local developer** — stdio, memory control-plane, best-effort
  audit. Works out of the box; nothing to configure.
- **Self-hosted single tenant** — stdio or single-process HTTP,
  file-backed control-plane, best-effort audit. One binary, one
  persistent file.
- **Shared-service production** — `streamable_http` (or gRPC),
  Postgres-backed control-plane, audit durability fail-closed,
  legacy-HTTP denied. Multiple replicas behind a load balancer.

The config surface has grown to 53 env vars. Without explicit
safety defaults on the third shape, three silent hazards exist:

1. **streamable_http silently against a dev DSN.**
   `MCP_CONTROL_PLANE_DSN` defaulted to `"memory"`. A replica set
   running `MCP_TRANSPORT=streamable_http` came up without any DSN
   configured and appeared to serve traffic — but each replica
   held its own in-process store, so session state, audit events,
   and rate-limit counters diverged invisibly. The transport/auth
   matrix test even documented this as a known gap
   ("tracked by Wave C").

2. **Legacy HTTP quietly accepted in prod.**
   `MCP_HTTP_LEGACY_POLICY` defaulted to `"warn"` everywhere. A
   production cluster configured with `MCP_TRANSPORT=http` emitted
   a deprecation log and kept running. The legacy transport drops
   server-initiated notifications (`tools/list_changed`) and
   forces clients to re-poll; it is kept for backward
   compatibility, not for new deployments.

3. **Audit durability silently best-effort in prod.**
   `MCP_AUDIT_DURABILITY` defaulted to `"best_effort"` everywhere.
   In prod, an audit-persist failure (Postgres blip, disk full)
   became a structured log line and nothing else — the tool call
   that triggered it succeeded as if the audit were recorded.
   Shared-service operators expected `fail_closed` but had to
   remember to set it.

Existing `ENVIRONMENT=prod` enforcement covered only one of the
three: it required `MCP_CONTROL_PLANE_DSN=postgres://…` and
prohibited `MCP_ALLOW_DEV_BACKEND=1`. The other two defaults did
not change based on environment.

## Decision

The `streamable_http` fail-closed guard becomes unconditional at
`config.Load()`: dev DSN + `streamable_http` + no
`MCP_ALLOW_DEV_BACKEND=1` fails, regardless of `ENVIRONMENT`.
Operators who genuinely want the single-process path must say so.

The other two defaults become **environment-aware**:

| Var | Dev default | Prod default |
|-----|-------------|--------------|
| `MCP_AUDIT_DURABILITY` | `best_effort` | `fail_closed` |
| `MCP_HTTP_LEGACY_POLICY` | `warn` | `deny` |

Explicit values always win. An operator who wants
`best_effort` in prod can set `MCP_AUDIT_DURABILITY=best_effort`
and Load() honours it. An operator who needs the legacy HTTP
path in prod sets `MCP_HTTP_LEGACY_POLICY=allow` and Load()
honours that too. The defaults only flip when the operator has
not expressed an intent.

Rationale for the asymmetry between `streamable_http` guard
(unconditional) and the two `ENVIRONMENT=prod` flips
(conditional): a dev DSN on the multi-process transport is
*always* wrong — no environment in which it is correct. By
contrast, a file-backed audit store with `best_effort` is a
perfectly sensible default for a laptop; it is only *prod* that
demands fail-closed, so we gate that flip on `ENVIRONMENT`.

## Consequences

**Positive.**
- Three silent hazards turn into load-time errors or environment-
  aware defaults. Nothing about the default dev UX changes;
  everything about the default prod UX tightens.
- The "Wave C" follow-up placeholder in
  `internal/config/transport_auth_matrix_test.go` is closed. The
  silent-memory-fallback is now an actively-asserted failure.
- Error messages name the escape hatches. An operator who hits
  the guard sees `set MCP_ALLOW_DEV_BACKEND=1 ... or point
  MCP_CONTROL_PLANE_DSN at a production backend (postgres://...)`
  and knows which remediation applies.

**Negative.**
- Three existing config tests that relied on the old silent
  fallback (`TestLoadStreamableHTTPAllowsEmptyAPIKey`,
  `TestLoadStreamableHTTPWithoutStaticAPIKey`,
  `TestLoadStreamableHTTPRequiresOIDCIssuer`) had to add
  `MCP_ALLOW_DEV_BACKEND=1` to keep testing the dev path. A
  one-line change per test.
- Any external operator using `streamable_http` + memory today
  (should be no one — the repo documentation never advertised
  this path) will see a Load()-time error on upgrade. The error
  message is actionable.
- Prod operators with unset `MCP_AUDIT_DURABILITY` or
  `MCP_HTTP_LEGACY_POLICY` will see a default change. The
  stricter default is the one they should have been running; if
  they really wanted the looser one, they can set it explicitly.

## Implementation

`internal/config/config.go`:
- New fail-closed guard after the streamable_http + OIDC check.
  Calls `IsDevControlPlaneDSN` from the sibling `dsn.go` file.
  Removes the unreachable `ControlPlaneDSN == ""` check.
- `AuditDurabilityMode`: unset + `ENVIRONMENT=prod` → `fail_closed`.
- `HTTPLegacyPolicy`: unset + `ENVIRONMENT=prod` → `deny`.

`internal/runtime/store.go`:
- `BuildStore` now calls `config.IsDevControlPlaneDSN` (single
  source of truth) and keeps its own guard as
  defence-in-depth.

`internal/runtime/runtime.go` / `runtime_test.go`:
- The duplicate `IsDevControlPlaneDSN` and its test are removed;
  `internal/config/dsn_test.go` absorbs the cases.

`internal/config/transport_auth_matrix_test.go`:
- The "Wave C" follow-up placeholder is replaced with three
  explicit assertions (dev DSN without flag → fails; dev DSN
  with flag → ok; postgres DSN → ok).

`internal/config/prod_defaults_test.go` (new):
- Locks the two environment-aware defaults (`AuditDurability`,
  `HTTPLegacyPolicy`) across dev / prod-unset / prod-explicit.
- Spot-checks the error messages name both escape hatches.

## References

- `internal/config/config.go` — Load() ordering.
- `internal/config/dsn.go` — shared predicate.
- `internal/runtime/store.go` — defence-in-depth guard.
- `docs/deploy/production-profile-shared-service.md` — operator
  guidance aligned to the new defaults.
- ADR [0012-backward-compatibility-policy.md](0012-backward-compatibility-policy.md)
  — env-var renames are breaking; these are default changes,
  not renames.
