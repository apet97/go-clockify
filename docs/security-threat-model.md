# Security Threat Model

## Scope

This document covers the HTTP transports, tenant/session handling, Clockify credential resolution, metrics exposure, audit logging, and tool activation behavior.

## Primary Trust Boundaries

- Client to MCP transport: bearer, OIDC, forward-auth, or mTLS authenticated edge.
- MCP service to control plane: tenant, credential-ref, session, and audit-event storage.
- MCP service to credential source: `env`, `file`, or future external secret manager.
- MCP service to Clockify API: outbound authenticated API traffic.

## Key Threats and Current Mitigations

### Session Confusion / Cross-Client State Bleed

- Threat: one HTTP client changes `initialize`, client metadata, or activated tool visibility for another client.
- Legacy `MCP_TRANSPORT=http`: known limitation; state is process-global and not shared-service safe.
- `MCP_TRANSPORT=streamable_http`: each session owns a dedicated MCP server instance, notifier, negotiated metadata, and activated tools.

### Session Hijacking

- Threat: an attacker reuses another session ID.
- Mitigation: `streamable_http` requires both `X-MCP-Session-ID` and a successful auth check on each request; the authenticated subject and tenant must match the stored session principal.

### Cross-Tenant Credential Bleed

- Threat: one tenant gains another tenant's Clockify API key, workspace cache, or audit trail.
- Mitigation: each `streamable_http` session resolves one tenant record and one credential ref, builds a dedicated Clockify client/service, and keeps user/workspace caches session-local.

### Metrics Leakage

- Threat: operational data is exposed on the public listener.
- Legacy `http`: `/metrics` remains on the shared listener for compatibility.
- `streamable_http`: metrics are not exposed on the public listener by default; use `MCP_METRICS_BIND` and optionally `MCP_METRICS_AUTH_MODE=static_bearer`.

### DNS Rebinding / Browser-Origin Abuse

- Threat: a hostile origin or Host header reaches the loopback or internal listener.
- Mitigation: CORS rejects unknown origins by default; optional `MCP_STRICT_HOST_CHECK=1` enforces Host allowlisting.

### Tool Activation Abuse

- Threat: one client activates Tier 2 tools for another client.
- Mitigation: activation is session-local in `streamable_http`. Legacy `http` remains process-global and should be treated as compatibility-only.

### Log / Audit Secret Exposure

- Threat: secrets leak through structured logs or audit events.
- Mitigation: the default slog handler is wrapped in recursive redaction. Audit events record tenant/session/tool metadata and resource IDs, not raw credentials.

## Residual Risks

- `streamable_http` sessions are replica-local today. Shared-service deployments must use sticky/session-affine routing.
- The control-plane store currently supports `memory` and file-backed persistence. Operators who need stronger durability or HA should front it with a managed store in future revisions.
- Vault backends are currently `inline`, `env`, and `file`. External secret-manager integrations are still pending.
