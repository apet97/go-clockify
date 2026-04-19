# Client Compatibility and Behavior Notes

This document provides guidance on how various Model Context Protocol (MCP) clients interact with `clockify-mcp-go`.

## Supported Client Matrix

| Client | Connection Mode | Stability | Notes |
|--------|-----------------|-----------|-------|
| Claude Code | `stdio` | Tier 1 | Full support for all tools and resources. |
| Claude Desktop | `stdio` | Tier 1 | Best-in-class support for tool rendering. |
| Cursor | `stdio` | Tier 1 | Supports via `.cursor/mcp.json`. |
| Codex | `stdio` | Tier 1 | Lightweight CLI. |

## Expected Client Behavior

### Tool Discovery
Most clients fetch the list of available tools at startup using `tools/list`.
- **Stdio Clients:** Automatically receive `notifications/tools/list_changed` when new tools are activated via `clockify_search_tools`.
- **HTTP Clients:** Must manually re-fetch the tool list or handle session-based tool visibility updates.

### Safety and Destructive Operations
`clockify-mcp-go` provides safety hints in tool definitions (`destructiveHint: true`).
- Clients like Claude Desktop may display a confirmation dialog before executing a destructive tool (e.g., `clockify_delete_entry`).
- The `CLOCKIFY_DRY_RUN` environment variable (default: `enabled`) adds a server-side safety layer by defaulting to a preview if the `dry_run` parameter is omitted.

### Resource Templates
The server exposes `clockify://` URI templates.
- Clients should use `resources/templates/list` to discover these.
- When a user asks for "my current timer," the client should resolve the template to a concrete URI and fetch it via `resources/read`.

## Compatibility Expectations

### Breaking Changes
We follow Semantic Versioning (SemVer). Breaking changes to the tool schema (renaming parameters, removing tools) will only occur in major releases (v2.x, etc.).
- See `docs/release-policy.md` for our full deprecation policy.

### Backwards Compatibility
The server supports multiple versions of the MCP protocol (today: `2025-06-18`, `2025-03-26`, and `2024-11-05`). It will negotiate the highest mutually supported version during the `initialize` handshake.

## Troubleshooting Client Issues

1.  **"Tools not found":** Some clients require a restart to see new tools if they don't support `notifications/tools/list_changed`. Try `clockify_search_tools` followed by a client reload.
2.  **Authentication Errors:** Ensure environment variables are passed correctly to the sub-process. Claude Desktop requires them in its `config.json` under the `env` key.
3.  **Logs:** Stdio clients often hide `stderr`. Check the client's internal log file (e.g., `~/Library/Logs/Claude/mcp.log` for Claude Desktop) to see redacted `clockify-mcp` logs.
