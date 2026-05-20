# Clockify MCP

![MCP Protocol](https://img.shields.io/badge/MCP-2025--11--25-blue)

A local, single-user Model Context Protocol (MCP) server for Clockify. It runs
on your machine over stdio, holds one Clockify API key, and exposes one
Clockify workspace — time entries, projects, invoices, reports, scheduling, and
more — as tools an AI client can call.

No account to create and no service to deploy: the binary runs as a subprocess
of your MCP client.

## Current docs

This README is the setup entry point. The complete tracked doc set lives
under `docs/`. Pre-one-user platform-era designs are preserved off-main,
outside the current setup path.

- Setup, build, doctor, Claude config, and first calls: this README
- Workflow examples: [docs/agent-cookbook.md](docs/agent-cookbook.md)
- Full generated tool list: [docs/tool-catalog.md](docs/tool-catalog.md)
- Permissions and plan requirements: [docs/permissions.md](docs/permissions.md)
- Destructive, billing, admin, and external-side-effect tools:
  [docs/dangerous-tools.md](docs/dangerous-tools.md)
- Raw API fallback: [docs/raw-fallback.md](docs/raw-fallback.md)
- Live tests and sacrificial workspace gates: [docs/live-tests.md](docs/live-tests.md)

## Start from zero

You need a Clockify account. The npm launcher (0A below) and prebuilt binary
(option A) need nothing else; building it yourself (B or C) needs a Go
toolchain ([go.dev/dl](https://go.dev/dl/)).

### 0A. Use the npm launcher

For MCP clients that can run `npx`, use
[`npm/clockify-mcp-launcher/README.md`](npm/clockify-mcp-launcher/README.md):

```bash
npx -y @apet97/clockify-mcp
```

The npm package bundles the Go server binary for supported platforms, so you do
not need to install Go, download a release asset, or set a binary path override.

### 1. Get the binary

All three options produce the same `clockify-mcp` binary — pick one.

**A. Download a prebuilt binary** — no toolchain needed. From the
[latest release](https://github.com/apet97/go-clockify/releases/latest),
download the asset for your platform (`clockify-mcp_<version>_<os>_<arch>`),
check it against the `SHA256SUMS` asset, and — on macOS/Linux — make it
executable with `chmod +x`.

**B. Install with Go:**

```bash
go install github.com/apet97/go-clockify/cmd/clockify-mcp@latest
```

This installs `clockify-mcp` into `$(go env GOPATH)/bin`.

**C. Build from source:**

```bash
git clone https://github.com/apet97/go-clockify.git
cd go-clockify
go build -o clockify-mcp ./cmd/clockify-mcp
```

### 2. Get your Clockify credentials

- **API key** — open your Clockify profile settings and generate a key in the
  API section.
- **Workspace ID** — the identifier that appears after `/workspaces/` in your
  Clockify workspace URL.

### 3. Check the setup

```bash
export CLOCKIFY_API_KEY="your-api-key"
export CLOCKIFY_WORKSPACE_ID="your-workspace-id"
./clockify-mcp doctor
```

`doctor` validates your configuration and prints every resolved setting.
`doctor --live` verifies auth and workspace access against Clockify. It
positively verifies owner status and makes a best-effort check for
workspace-admin status. A key that is neither passes with a warning that
lists the tool families that may be denied, and still reports OK. Admin
status is detected on a best-effort basis and is not guaranteed by doctor.

### 4. Connect it to your MCP client

Point your MCP client at the binary. For a Claude `.mcp.json`:

```json
{
  "mcpServers": {
    "clockify": {
      "command": "/absolute/path/to/clockify-mcp",
      "env": {
        "CLOCKIFY_API_KEY": "your-api-key",
        "CLOCKIFY_WORKSPACE_ID": "your-workspace-id"
      }
    }
  }
}
```

The client launches `clockify-mcp` as a stdio subprocess. That is the whole
setup. Start with `clockify_status`, then use workflow tools before raw or
low-level domain tools:

An npm launcher is also available as an alternative install path for clients
that prefer `npx`. It remains a thin stdio launcher around the same Go binary;
see [npm/clockify-mcp-launcher/README.md](npm/clockify-mcp-launcher/README.md).

- `clockify_status` - confirm the pinned workspace, user, feature plan, and
  optional feature visibility
- `clockify_start_work` / `clockify_stop_work` - day-to-day time tracking
- `clockify_review_day` - summarize and check a workday
- `clockify_create_work_package` - create a client/project/task/tag bundle
- `clockify_invoice_client_work`, `clockify_record_expense`,
  `clockify_request_time_off`, `clockify_schedule_work`,
  `clockify_setup_webhook` - business workflows that guide follow-up calls

For a direct Codex/CLI smoke run, keep secrets in your shell environment and
launch the server over stdio:

```bash
export CLOCKIFY_API_KEY="your-api-key"
export CLOCKIFY_WORKSPACE_ID="your-workspace-id"
/absolute/path/to/clockify-mcp
```

## Tools

`clockify-mcp` loads the full 156-tool startup registry in a fixed order:

1. **Workflow tools** — high-level actions like start and stop work, log time,
   review a day, or invoice a client. Reach for these first.
2. **Domain tools** — direct create / read / update / delete for clients,
   projects, tasks, time entries, reports, invoices, expenses, time off,
   scheduling, and more.
3. **Raw API fallback** — for the rare endpoint with no dedicated tool.

The complete generated list is in
[docs/tool-catalog.md](docs/tool-catalog.md), and
[docs/agent-cookbook.md](docs/agent-cookbook.md) shows worked examples.
Operator references:
[permissions](docs/permissions.md),
[dangerous tools](docs/dangerous-tools.md),
[raw fallback](docs/raw-fallback.md), and
[error recovery](docs/error-recovery.md).

### Raw API fallback

`clockify_api_get` and `clockify_api_request` reach Clockify endpoints that
have no dedicated tool, scoped to your pinned workspace. Raw `GET` always
works; raw `POST`, `PUT`, `PATCH`, and `DELETE` require
`CLOCKIFY_ENABLE_RAW_WRITES=true`. Prefer the domain tools — raw writes are an
explicit escape hatch. Raw writes are also limited to documented Clockify
routes by default; see [docs/raw-fallback.md](docs/raw-fallback.md) for
`CLOCKIFY_RAW_WRITE_DOCUMENTED_ONLY`.

## Configuration

`CLOCKIFY_API_KEY` and `CLOCKIFY_WORKSPACE_ID` are required. Everything else is
optional:

Most tools need a workspace **owner or admin** API key. A regular-member key can
still read data and manage its own time entries, but admin, billing, and
settings tools will return `feature_unavailable` or Clockify permission errors.

| Variable | Default | Purpose |
| --- | --- | --- |
| `CLOCKIFY_BASE_URL` | `https://api.clockify.me/api/v1` | Clockify API base URL |
| `CLOCKIFY_TIMEZONE` | system local | Timezone for date handling |
| `CLOCKIFY_TOOLSET` | `all` | Tool surface: `core`, `business`, `admin`, or `all` |
| `CLOCKIFY_TOOL_RATE_LIMIT_PER_MINUTE` | `0` | Optional tool-invocation rate cap per minute; `0` disables it |
| `CLOCKIFY_MAX_IN_FLIGHT_TOOL_CALLS` | `4` | Max concurrent `tools/call` handlers |
| `CLOCKIFY_MAX_MESSAGE_SIZE` | `4194304` | Max inbound JSON-RPC message bytes (`1`..`104857600`) |
| `CLOCKIFY_MAX_TOOL_RESULT_BYTES` | `50000` | Result-size cap before truncation (`1`..`104857600`) |
| `CLOCKIFY_TOOL_TIMEOUT` | `45s` | Per-tool timeout (`5s`..`10m`) |
| `CLOCKIFY_ENABLE_RAW_WRITES` | `false` | Allow raw `POST` / `PUT` / `PATCH` / `DELETE` |
| `CLOCKIFY_RAW_WRITE_DOCUMENTED_ONLY` | `true` | Limit raw writes to documented Clockify routes |
| `CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS` | none | Comma-separated allowlist of webhook callback domains |
| `CLOCKIFY_CIRCUIT_BREAKER` | `enabled` | Clockify circuit breaker: `enabled`/`auto`/`on` or `disabled`/`off` |
| `CLOCKIFY_CIRCUIT_BREAKER_FAILURE_THRESHOLD` | `5` | Consecutive upstream failures before the breaker opens |
| `CLOCKIFY_CIRCUIT_BREAKER_OPEN_DURATION` | `45s` | How long the breaker stays open before a half-open probe |
| `CLOCKIFY_CIRCUIT_BREAKER_HALF_OPEN_PROBES` | `1` | Probe requests allowed while the breaker is half-open |
| `MCP_LOG_LEVEL` | `warn` | Log level: `debug`, `info`, `warn`, `error`; the stdio runtime defaults to `warn` so a clean session leaves stderr quiet |

Run `clockify-mcp doctor` to see every resolved value.

## Compatibility

| Capability | Support |
| --- | --- |
| MCP Protocol | `2025-11-25` |
| Transport | stdio |
| Clockify scope | one pinned workspace |

## Tests

```bash
go test ./...     # full suite against an in-memory fake Clockify server
make check        # adds the race detector and repo hygiene checks
```

Live tests run against real Clockify, are opt-in, and must target a sacrificial
workspace — see [docs/live-tests.md](docs/live-tests.md).

## License

[MIT](LICENSE).
