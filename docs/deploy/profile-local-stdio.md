# Deployment profile: local stdio

> **Historical artifact. Not current one-user MCP product documentation.**
> Preserved for platform-era audit/history only. Start current one-user work from `README.md`, `docs/agent-cookbook.md`, `docs/tool-catalog.md`, and `docs/goals/oneuser-tool-coverage.md`.


> Apply with `clockify-mcp --profile=local-stdio` or
> `MCP_PROFILE=local-stdio`. Example env file:
> [`deploy/examples/env.local-stdio.example`](../../deploy/examples/env.local-stdio.example).
> See also: [`internal/config/profile.go`](../../internal/config/profile.go)
> for the pinned defaults, [ADR-0015](../adr/0015-profile-centric-configuration.md)
> for the design rationale, and
> [`internal-test-posture.md`](../internal-test-posture.md) for
> owner-key testing guardrails.

A single-user deployment where `clockify-mcp` runs as a subprocess
of one MCP client (Claude Code, Claude Desktop, Cursor, Codex). No
HTTP endpoints, no auth on the transport layer, no audit store —
the parent process owns the identity boundary.

Use this when: you're running as yourself against your personal
Clockify workspace. Do **not** use this when: multiple users share
the host, or when you need a durable audit trail.

## Canonical configuration

Recommended personal tester environment:

```env
CLOCKIFY_API_KEY=<your-personal-clockify-api-key>
CLOCKIFY_WORKSPACE_ID=<your-clockify-workspace-id>
MCP_TRANSPORT=stdio
CLOCKIFY_POLICY=time_tracking_safe
```

`local-stdio` defaults to `time_tracking_safe`, matching the recommended
AI-facing posture for personal testing. Keep that default for exploratory
prompts and untrusted local clients. Set `CLOCKIFY_POLICY=safe_core`
explicitly only for trusted local automation that should create projects,
clients, tags, or tasks. Set `CLOCKIFY_WORKSPACE_ID` even if
auto-detection works today; `auto` is convenient only while the API key
can see exactly one workspace.

Defaults that apply automatically and don't need to be set:

| Variable | Default | Reason |
|----------|---------|--------|
| `MCP_AUTH_MODE` | n/a | stdio has no inbound HTTP; auth is delegated to the parent process |
| `MCP_CONTROL_PLANE_DSN` | n/a | no audit store; read-only-except-local-writes |
| `MCP_AUDIT_DURABILITY` | `best_effort` | nothing to persist; setting is inert |
| `MCP_METRICS_BIND` | unset | no metrics exposure; not useful for single-user CLI |

## Client wiring

### Claude Code

Add to the Claude Code MCP config file:

```json
{
  "mcpServers": {
    "clockify": {
      "command": "clockify-mcp",
      "env": {
        "MCP_PROFILE": "local-stdio",
        "CLOCKIFY_API_KEY": "pk_XXXXXXXXXXXXXXXXXXXXXX",
        "CLOCKIFY_WORKSPACE_ID": "workspace-id",
        "CLOCKIFY_POLICY": "time_tracking_safe"
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
        "MCP_PROFILE": "local-stdio",
        "CLOCKIFY_API_KEY": "pk_XXXXXXXXXXXXXXXXXXXXXX",
        "CLOCKIFY_WORKSPACE_ID": "workspace-id",
        "CLOCKIFY_POLICY": "time_tracking_safe"
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
        "MCP_PROFILE": "local-stdio",
        "CLOCKIFY_API_KEY": "pk_XXXXXXXXXXXXXXXXXXXXXX",
        "CLOCKIFY_WORKSPACE_ID": "workspace-id",
        "CLOCKIFY_POLICY": "time_tracking_safe"
      }
    }
  }
}
```

`MCP_PROFILE=local-stdio` is still required when you set an explicit
policy override. If you omit `CLOCKIFY_POLICY`, the profile default is
`time_tracking_safe`; without the profile the server falls back to
`CLOCKIFY_POLICY=standard`, which exposes more destructive tools than the
local profile intends.

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

For large workspaces, start with read-only or narrow calls:
`clockify_whoami`, `clockify_policy_info`, small `clockify_list_*`
pages, and short-range reports with `include_entries=false`. Use
`page` and `page_size` for list tools; keep `CLOCKIFY_REPORT_MAX_ENTRIES`
at the default unless you intentionally want to materialize a larger raw
entry set. Local personal smoke tests are useful preflight only; they do
not close the launch live-contract evidence tracked in
[`docs/api-coverage.md`](../api-coverage.md).

## Upgrade path

When you outgrow stdio (another user joins the team, or you need
an audit trail for compliance), move to
`profile-single-tenant-http.md` first, then to
`production-profile-shared-service.md`. See
`docs/upgrade-checklist.md` for the step-by-step migration.

## How to verify this deployment

Run the profile audit first:

```bash
CLOCKIFY_API_KEY=pk_xxx \
  CLOCKIFY_WORKSPACE_ID=workspace-id \
  CLOCKIFY_POLICY=time_tracking_safe \
  clockify-mcp doctor --profile=local-stdio
```

Expected result: `Load() result: OK`, `transport=stdio`, no inbound
auth mode, and `CLOCKIFY_POLICY=time_tracking_safe` from explicit env.
The doctor command does not contact Clockify; `pk_xxx` can be a dummy
value for this local config check. Omit the explicit policy override
when you want to verify the profile default of `time_tracking_safe`.

`doctor --strict` is a hosted-service posture gate, not the success
criterion for local stdio. Run it only as a negative check when you
want to prove this profile is not accidentally being treated as a
hosted deployment:

```bash
CLOCKIFY_API_KEY=pk_xxx \
  clockify-mcp doctor --profile=local-stdio --strict
```

Expected result: exit 3 with hosted-strict findings such as missing
Postgres control-plane DSN and `fail_closed` audit durability. That
is correct for this profile.

The CI-backed smoke for this deployment shape is:

```bash
make stdio-smoke
```

That target builds the binary, sends newline-delimited MCP
`initialize` and `tools/list` requests over stdio, and verifies both
JSON-RPC responses. It runs in the `stdio-smoke` path of `make
verify-core` and the corresponding PR CI job.
