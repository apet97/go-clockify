# @apet97/clockify-mcp

```bash
npx -y @apet97/clockify-mcp
```

That is the install path. The package includes the Go server binary for supported platforms, so MCP clients only need `npx -y @apet97/clockify-mcp` plus your Clockify environment variables. You do not need to install Go, build the repo, download a GitHub Release asset, or set `CLOCKIFY_MCP_BINARY` when using the published npm package.

All MCP tools and Clockify logic live in the Go binary. This npm package is only the launcher: it picks the right bundled binary, forwards stdio, and exits with the server's exit code.

The source checkout keeps `package.json` at version `0.0.0` on purpose. The
tagged release workflow rewrites it to the tag version before `npm pack` and
verifies that the packed tarball contains every vendored platform binary.

## MCP Client Config

Use `npx` as the command and pass your Clockify credentials through the client environment:

## Claude Code

```bash
claude mcp add \
  -e CLOCKIFY_API_KEY=your-api-key \
  -e CLOCKIFY_WORKSPACE_ID=your-workspace-id \
  clockify -- npx -y @apet97/clockify-mcp
```

## Codex CLI

```bash
codex mcp add \
  --env CLOCKIFY_API_KEY=your-api-key \
  --env CLOCKIFY_WORKSPACE_ID=your-workspace-id \
  clockify -- npx -y @apet97/clockify-mcp
```

## Claude Desktop

```json
{
  "mcpServers": {
    "clockify": {
      "command": "npx",
      "args": ["-y", "@apet97/clockify-mcp"],
      "env": {
        "CLOCKIFY_API_KEY": "your-api-key",
        "CLOCKIFY_WORKSPACE_ID": "your-workspace-id"
      }
    }
  }
}
```

## Cursor

```json
{
  "mcpServers": {
    "clockify": {
      "command": "npx",
      "args": ["-y", "@apet97/clockify-mcp"],
      "env": {
        "CLOCKIFY_API_KEY": "your-api-key",
        "CLOCKIFY_WORKSPACE_ID": "your-workspace-id"
      }
    }
  }
}
```

## Local Source Checkouts

The published npm package bundles platform binaries under `vendor/`. A source checkout does not commit `vendor/`, so local package development can use `CLOCKIFY_MCP_BINARY` as an override:

```bash
go build -o ~/.local/bin/clockify-mcp ./cmd/clockify-mcp
export CLOCKIFY_MCP_BINARY=~/.local/bin/clockify-mcp
```

Binary resolution order is:

1. `CLOCKIFY_MCP_BINARY` environment override.
2. Bundled `vendor/<platform-binary>` inside the npm package.
