# Stability Policy

## Compatibility Tiers

### Stable

- Tier 1 and Tier 2 tool names
- Tool annotations: `readOnlyHint`, `destructiveHint`, `idempotentHint`
- Dry-run semantics for destructive tools
- Policy-mode names: `read_only`, `safe_core`, `standard`, `full`
- `stdio` transport behavior
- Legacy `http` transport remaining compatibility-only
- `streamable_http` session requirements: `initialize` creates a session and later calls must send `X-MCP-Session-ID`

### Compatibility-Only

- `MCP_TRANSPORT=http`
- Public `/metrics` on the legacy HTTP listener

These behaviors remain supported, but new shared-service work should target `streamable_http`.

### Experimental

- `forward_auth` and `mtls` tenant derivation details
- Control-plane DSN backends beyond `memory` and file-backed JSON
- Vault backends beyond `inline`, `env`, and `file`

## Deprecation Rules

- Stable tool names and schemas should not change incompatibly without a documented release note and migration note.
- Compatibility-only features may be deprecated, but only after an explicit README/docs notice and a replacement path exists.
- Experimental features may change faster, but docs must call them out as such.
