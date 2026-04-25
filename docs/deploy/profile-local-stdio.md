# Deployment profile: local stdio

> Apply with `clockify-mcp --profile=local-stdio` or
> `MCP_PROFILE=local-stdio`. Example env file:
> [`deploy/examples/env.local-stdio.example`](../../deploy/examples/env.local-stdio.example).
> See also: [`internal/config/profile.go`](../../internal/config/profile.go)
> for the pinned defaults, [ADR-0015](../adr/0015-profile-centric-configuration.md)
> for the design rationale.

A single-user deployment where `clockify-mcp` runs as a subprocess
of one MCP client (Claude Code, Claude Desktop, Cursor, Codex). No
HTTP endpoints, no auth on the transport layer, no audit store —
the parent process owns the identity boundary.

Use this when: you're running as yourself against your personal
Clockify workspace. Do **not** use this when: multiple users share
the host, or when you need a durable audit trail.

## Canonical configuration

Minimum viable environment:

```env
CLOCKIFY_API_KEY=<your-personal-clockify-api-key>
MCP_TRANSPORT=stdio
CLOCKIFY_POLICY=safe_core
```

Defaults that apply automatically and don't need to be set:

| Variable | Default | Reason |
|----------|---------|--------|
| `MCP_AUTH_MODE` | n/a | stdio has no inbound HTTP; auth is delegated to the parent process |
| `MCP_CONTROL_PLANE_DSN` | n/a | no audit store; read-only-except-local-writes |
| `MCP_AUDIT_DURABILITY` | `best_effort` | nothing to persist; setting is inert |
| `MCP_METRICS_BIND` | unset | no metrics exposure; not useful for single-user CLI |

## Client wiring

### Claude Code

Add to `~/.claude/mcp.json`:

```json
{
  "mcpServers": {
    "clockify": {
      "command": "clockify-mcp",
      "env": {
        "CLOCKIFY_API_KEY": "pk_XXXXXXXXXXXXXXXXXXXXXX"
      }
    }
  }
}
```

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`
(macOS) or the equivalent path on Linux / Windows:

```json
{
  "mcpServers": {
    "clockify": {
      "command": "/usr/local/bin/clockify-mcp",
      "env": {
        "CLOCKIFY_API_KEY": "pk_XXXXXXXXXXXXXXXXXXXXXX"
      }
    }
  }
}
```

### Cursor

Add to `.cursor/mcp.json` in your workspace root:

```json
{
  "mcpServers": {
    "clockify": {
      "command": "clockify-mcp",
      "env": {
        "CLOCKIFY_API_KEY": "pk_XXXXXXXXXXXXXXXXXXXXXX"
      }
    }
  }
}
```

## Security model

- The binary inherits the parent client's process identity. Anyone
  who can run the client as you can run `clockify-mcp` as you —
  that's the same trust boundary as any local shell command.
- `CLOCKIFY_API_KEY` sits in the client's config file. Protect it
  the same way you'd protect an SSH private key: `chmod 600`, do
  not commit, rotate if leaked.
- No network listener is opened, so inbound auth modes
  (`static_bearer`, `oidc`, `forward_auth`, `mtls`) do not apply.

## What you give up vs. production profiles

| Capability | stdio | single-tenant HTTP | shared-service |
|------------|:-----:|:------------------:|:--------------:|
| Multi-user | no | no | yes |
| Durable audit ledger | no | optional | yes (`fail_closed`) |
| Metrics endpoint | no | recommended | required |
| HA / rolling upgrade | no | possible | yes |
| Per-tenant rate limits | no | single tenant | yes |

## Sanity check

After wiring, run in your client:

```text
clockify_whoami
```

The expected response is your name + workspace. If you see
`CLOCKIFY_API_KEY not set`, the client did not forward the env
var — check the client's config file for typos.

## Upgrade path

When you outgrow stdio (another user joins the team, or you need
an audit trail for compliance), move to
`profile-single-tenant-http.md` first, then to
`production-profile-shared-service.md`. See
`docs/upgrade-checklist.md` for the step-by-step migration.
