# Operator Guide: Self-Hosted (Single-User/Small-Team)

This guide is for individual users or small teams operating `clockify-mcp-go` locally or in a private cloud environment.

## Architecture
- **Transport:** `stdio` (single-user, MCP client subprocess) or
  `streamable_http` (small-team HTTP, behind a TLS-terminating proxy).
  The legacy `http` transport is deprecated and rejected by
  `MCP_HTTP_LEGACY_POLICY=deny`; do not use for new setups.
- **Control Plane:** `file://` or `memory` (Safe for single-process)
- **Auth:** N/A for `stdio`; `static_bearer` for `streamable_http`.
- **Infrastructure:** Docker, Docker Compose, or local process

## Key Responsibilities

### 1. Data Persistence
- If using `file://`, ensure the specified file path is writable by the process.
- Back up the state file regularly.

### 2. Local Security
- Protect your `CLOCKIFY_API_KEY`. Do not commit it to version control.
- If using the `streamable_http` transport, generate a strong (≥16-char) `MCP_BEARER_TOKEN` and distribute it to your clients.

### 3. Client Configuration
- Configure your local MCP client (Claude Desktop, Cursor, etc.) to use the correct transport and credentials.
- See `docs/clients.md` for specific client notes.

### 4. Updates
- Manually check for new releases or subscribe to GitHub release notifications.
- Re-run `go install` or pull the latest Docker image to update.

## Recommended Configuration

Apply one of the two registered profiles that match this guide's
shape (see [`docs/deploy/`](../deploy/) for full profile notes):

- `clockify-mcp --profile=local-stdio` — single user, MCP client
  subprocess, no auth. Sets `MCP_TRANSPORT=stdio`,
  `CLOCKIFY_POLICY=safe_core`,
  `MCP_AUDIT_DURABILITY=best_effort`.
- `clockify-mcp --profile=single-tenant-http` — small-team HTTP,
  static-bearer auth, file-backed control plane. Sets
  `MCP_TRANSPORT=streamable_http`, `MCP_AUTH_MODE=static_bearer`,
  `MCP_HTTP_LEGACY_POLICY=deny`,
  `CLOCKIFY_POLICY=time_tracking_safe`.

The legacy `deploy/examples/env.self-hosted.example` preset
pre-dates the profile system and is preserved for operator muscle
memory; new setups should pick a profile name above and override
individual env vars (e.g. `CLOCKIFY_POLICY=standard` for individual
power users) rather than start from the legacy file. See
[`docs/deploy/profile-self-hosted.md`](../deploy/profile-self-hosted.md)
for the upgrade path.
