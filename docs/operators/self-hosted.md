# Operator Guide: Self-Hosted (Single-User/Small-Team)

This guide is for individual users or small teams operating `clockify-mcp-go` locally or in a private cloud environment.

## Architecture
- **Transport:** `stdio` or `http` (Standard)
- **Control Plane:** `file://` or `memory` (Safe for single-process)
- **Auth:** N/A for `stdio` or `static_bearer` for `http`
- **Infrastructure:** Docker, Docker Compose, or local process

## Key Responsibilities

### 1. Data Persistence
- If using `file://`, ensure the specified file path is writable by the process.
- Back up the state file regularly.

### 2. Local Security
- Protect your `CLOCKIFY_API_KEY`. Do not commit it to version control.
- If using the `http` transport, generate a strong `MCP_BEARER_TOKEN` and distribute it to your clients.

### 3. Client Configuration
- Configure your local MCP client (Claude Desktop, Cursor, etc.) to use the correct transport and credentials.
- See `docs/clients.md` for specific client notes.

### 4. Updates
- Manually check for new releases or subscribe to GitHub release notifications.
- Re-run `go install` or pull the latest Docker image to update.

## Recommended Configuration
Use the `deploy/examples/env.self-hosted.example` preset.
- **`CLOCKIFY_POLICY=standard`**: Suitable for individual power users.
- **`MCP_TRANSPORT=stdio`**: Recommended for the simplest setup with local clients.
