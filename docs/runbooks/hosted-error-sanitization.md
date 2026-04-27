# Hosted-Mode Upstream Error Sanitisation

## Why this runbook exists

On the `shared-service` and `prod-postgres` profiles, tool-error
responses returned to MCP clients omit the upstream Clockify response
body. This is intentional: a 4xx body can carry per-tenant identifiers
that must not cross tenant boundaries on a shared deployment. But it
means an operator chasing "why did this tool fail?" cannot read the
full diagnostic from the MCP client log alone.

This runbook explains where the full diagnostic goes, how to flip the
toggle for debugging, and what client-visible response to expect.

## 1. Symptom

A user reports a tool failed with a generic message like:

```
clockify PUT /workspaces/abc.../invoices/inv1 failed: 403 Forbidden
```

…with no further detail. They want to know *why* — was it a missing
scope, a stale invoice, a billing-plan limitation?

## 2. Where the full diagnostic lives

The full upstream error (including the response body) is always
emitted to server-side slog at `WARN` level under the `tool_call` event:

```
2026-04-27T05:12:51Z WARN tool_call tool=clockify_mark_invoice_paid \
  error="clockify PUT /workspaces/abc.../invoices/inv1 failed: \
  403 Forbidden: insufficient permissions tenant=acme-internal" \
  duration_ms=42 req_id=42
```

Filter on `tool=<tool-name>` and `req_id=<id-from-client>` to find the
full body. The MCP client wire only ever sees the sanitised form; the
log retains everything.

## 3. Configuration

| Profile | Default | Override |
|---|---|---|
| `local-stdio` | verbose (full body on wire) | `CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=1` to opt in |
| `single-tenant-http` | verbose | same |
| `private-network-grpc` | verbose | same |
| `shared-service` | sanitised | `CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=0` to opt out |
| `prod-postgres` | sanitised | same |

The explicit env var always wins over the profile default.

## 4. Temporary debug procedure

When the slog record is not enough (e.g. you need to ask the user to
re-run with full context), prefer one of these:

1. **Operator side:** the WARN-level `msg=tool_call` record at
   `internal/mcp/tools.go:117` (BeforeCall failure path) and `:179`
   (handler failure path) already carries the full upstream body
   verbatim in the `error=` attribute regardless of `MCP_LOG_LEVEL`
   — sanitisation only affects the MCP wire response, never the
   server-side log. Filter on `tool=<name>` and `req_id=<id>` to
   correlate with the client's seen sanitised error. (Raising
   `MCP_LOG_LEVEL=debug` does not surface a separate enriched record
   for this code path; the WARN line is the canonical record.)
2. **Client side:** if you really need the upstream body on the wire
   for a single debug session, set
   `CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=0` on a non-production replica
   and reproduce. Do **not** flip it on the live shared-service
   deployment — a body containing `tenant=acme-internal` shipped to a
   different tenant's MCP client is exactly the cross-tenant leak the
   default exists to prevent.

## 5. Verification checklist

- [ ] Server slog captures the full upstream body (grep `tool=` and
      `req_id=`).
- [ ] MCP client wire shows only `clockify <verb> <path> failed: <status>`
      (no body suffix) on `shared-service` / `prod-postgres`.
- [ ] Local profiles still show the full body for fast diagnostics.
- [ ] `clockify_mcp_audit_events_total` ticked (the failed call is
      audited even though the body is hidden from the client).

## 6. Related

- `internal/clockify/errors.go` — `APIError.Error()` (verbose) +
  `Sanitized()` (sanitised) implementation.
- `internal/mcp/server.go` — `sanitizeClientError()` walker plus the
  `Server.SanitizeUpstreamErrors` field.
- `internal/runtime/service.go` — `buildServer()` propagates the config
  flag onto the Server struct uniformly across every transport.
- `docs/deploy/production-profile-shared-service.md` — strict-gate
  rationale table.
- Audit finding 9 (closed in the 2026-04-27 hardening wave).
