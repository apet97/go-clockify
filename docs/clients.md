# Client Setup

`clockify-mcp` is a local stdio MCP server. Clients launch the binary as a
subprocess and pass the Clockify environment variables to that process.

## Required Environment

```sh
export CLOCKIFY_API_KEY="<clockify-api-key>"
export CLOCKIFY_WORKSPACE_ID="<clockify-workspace-id>"
```

Optional:

```sh
export CLOCKIFY_TIMEZONE="Europe/Belgrade"
export CLOCKIFY_BASE_URL="https://api.clockify.me/api/v1"
```

Do not put real keys in checked-in config files. Prefer the client-specific
secret mechanism or a local, ignored environment file.

## Tool Discovery

Clients should call `tools/list` after `initialize`. The list is static for a
process lifetime:

- 151 tools total.
- 17 workflow tools first.
- 132 domain tools second.
- 2 raw API fallback tools last.

The server does not require any activation step. Agents should start with
`clockify_status` and `clockify_tools_guide`, then use returned IDs in later
calls.

## Claude Desktop Example

```json
{
  "mcpServers": {
    "clockify": {
      "command": "/absolute/path/to/clockify-mcp",
      "args": [],
      "env": {
        "CLOCKIFY_API_KEY": "<redacted>",
        "CLOCKIFY_WORKSPACE_ID": "<workspace-id>",
        "CLOCKIFY_TIMEZONE": "Europe/Belgrade"
      }
    }
  }
}
```

## Codex Example

Use the same command/env shape in the Codex MCP configuration. Keep the key in
your local environment or secret store, not in the repository.

## Client Expectations

- The server reads and writes only the pinned workspace.
- Write-style workflow tools return IDs in `ids` and entity references in
  `changed`.
- Recoverable upstream failures return `ok:false` with `error` and `recovery`
  inside `structuredContent`.
- Raw fallback tools are last-resort tools for Clockify API paths that are not
  yet covered by a workflow or domain tool.
