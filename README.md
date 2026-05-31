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

Use the short path first:

- **Installing or configuring:** read this README.
- **Choosing tools as an agent:** start with `clockify_tools_guide`, then
  [docs/agent-cookbook.md](docs/agent-cookbook.md).
- **Checking every available tool:** use the generated
  [docs/tool-catalog.md](docs/tool-catalog.md).
- **Checking the everyday default surface:** use
  [docs/default-toolset.md](docs/default-toolset.md).
- **Checking release-readiness coverage buckets:** use
  [docs/tool-coverage-dashboard.md](docs/tool-coverage-dashboard.md).
- **Before risky writes:** read
  [docs/dangerous-tools.md](docs/dangerous-tools.md).
- **When a call returns `ok:false`:** read
  [docs/error-recovery.md](docs/error-recovery.md).

- Setup, build, doctor, Claude config, and first calls: this README
- Compact domain-language glossary: [CONTEXT.md](CONTEXT.md)
- Workflow examples: [docs/agent-cookbook.md](docs/agent-cookbook.md)
- Full generated tool list: [docs/tool-catalog.md](docs/tool-catalog.md)
- Default advertised tool list: [docs/default-toolset.md](docs/default-toolset.md)
- Coverage dashboard: [docs/tool-coverage-dashboard.md](docs/tool-coverage-dashboard.md)
- Permissions and plan requirements: [docs/permissions.md](docs/permissions.md)
- Destructive, billing, admin, and external-side-effect tools:
  [docs/dangerous-tools.md](docs/dangerous-tools.md)
- Raw API fallback: [docs/raw-fallback.md](docs/raw-fallback.md)
- Live tests and sacrificial workspace gates: [docs/live-tests.md](docs/live-tests.md)

## Non-goals for now

This project is intentionally local and single-user. Network-service
deployment, browser sign-in flows, non-stdio transports, remote administration,
per-person secret stores, database-backed account separation, and team-wide
policy management are future products, not part of the default stdio binary.

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
setup. For the first five calls, keep the default toolset and use workflows:

1. `clockify_status` - confirm the pinned workspace, user, feature plan, and
   current timer
2. `clockify_tools_guide` - choose the workflow before dropping to domain tools
3. `clockify_create_work_package` - create or reuse client/project/task/tag IDs
4. `clockify_log_work`, `clockify_start_work`, or `clockify_stop_work` - do the
   everyday time-tracking action
5. `clockify_review_day` or `clockify_review_week` - inspect totals, gaps, and
   suggested follow-up calls

Workflow tools should come before raw or low-level domain tools:

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

Tool surface selection:

1. Keep `CLOCKIFY_TOOLSET=default` for normal agent use; it advertises the 16
   everyday workflow/orientation tools.
2. Use `core`, `business`, or `admin` only when a client needs that narrower
   domain family.
3. Use `all` for expert debugging or catalog inspection, not first contact.
4. Use raw fallback only after `clockify_tools_guide` and the domain tools do
   not fit.

The default surface is listed in
[docs/default-toolset.md](docs/default-toolset.md). The complete generated
list is in [docs/tool-catalog.md](docs/tool-catalog.md), and
[docs/agent-cookbook.md](docs/agent-cookbook.md) shows worked examples.
Operator references:
[permissions](docs/permissions.md),
[dangerous tools](docs/dangerous-tools.md),
[raw fallback](docs/raw-fallback.md), and
[error recovery](docs/error-recovery.md). Contributors should also read
[development guide](docs/development.md) and
[architecture](docs/architecture.md).

### Raw API fallback

`clockify_api_get` and `clockify_api_request` reach Clockify endpoints that
have no dedicated tool, scoped to your pinned workspace. Raw fallback is hidden
and disabled by default unless you choose `CLOCKIFY_TOOLSET=all`; otherwise set
`CLOCKIFY_ENABLE_RAW_TOOLS=true` and, for raw reads,
`CLOCKIFY_ENABLE_RAW_GET=true`. Raw `POST`, `PUT`, `PATCH`, and `DELETE` also
require `CLOCKIFY_ENABLE_RAW_WRITES=true` and a dry-run `confirm_token`
before live execution. Prefer the domain tools — raw fallback is an explicit
escape hatch. Raw writes are limited to documented Clockify routes by default;
see [docs/raw-fallback.md](docs/raw-fallback.md) for
`CLOCKIFY_RAW_WRITE_DOCUMENTED_ONLY`.

## Configuration

`CLOCKIFY_API_KEY` and `CLOCKIFY_WORKSPACE_ID` are required. Everything else is
optional:

Most tools need a workspace **owner or admin** API key. A regular-member key can
still read data and manage its own time entries, but admin, billing, and
settings tools will return `feature_unavailable` or Clockify permission errors.

| Variable | Default | Purpose |
| --- | --- | --- |
| `CLOCKIFY_BASE_URL` | `https://api.clockify.me/api/v1` | Clockify API base URL. Overrides must point at a documented Clockify host (`api.clockify.me`, `reports.api.clockify.me`, `auditlog-api.api.clockify.me`) or a loopback target (test fixtures / local proxies); any other HTTPS host requires `CLOCKIFY_ALLOW_CUSTOM_BASE_URL=true` |
| `CLOCKIFY_ALLOW_CUSTOM_BASE_URL` | `false` | `true` allows an arbitrary HTTPS `CLOCKIFY_BASE_URL` host off the Clockify allowlist (logs a warning). Use only for a trusted proxy; non-HTTPS non-loopback hosts are still rejected |
| `CLOCKIFY_TIMEZONE` | system local | Timezone for date handling |
| `CLOCKIFY_TOOLSET` | `default` | Tool surface advertised and authorized on the wire: `default` (16 everyday tools), `core`, `business`, `admin`, or `all` (156). The full registry of 156 tools is loaded for self-inspection, but default/core/business/admin reject unadvertised tool calls. |
| `CLOCKIFY_TOOL_RATE_LIMIT_PER_MINUTE` | `120` | Tool-invocation rate cap per minute. Risk buckets narrow this to 30/min writes, 10/min billing/admin, and 5/min destructive tools; explicit `0` disables rate limiting. |
| `CLOCKIFY_MAX_IN_FLIGHT_TOOL_CALLS` | `4` | Max concurrent `tools/call` handlers |
| `CLOCKIFY_MAX_MESSAGE_SIZE` | `4194304` | Max inbound JSON-RPC message bytes (`1`..`104857600`) |
| `CLOCKIFY_MAX_TOOL_RESULT_BYTES` | `50000` | Result-size cap before truncation (`1`..`104857600`); the upstream response cap is at least 10 MiB and rises with this value for large exports |
| `CLOCKIFY_TOOL_TIMEOUT` | `45s` | Per-tool timeout (`5s`..`10m`); the Clockify HTTP client uses the same deadline |
| `CLOCKIFY_ENABLE_RAW_TOOLS` | `false` | Advertise and enable raw fallback outside `CLOCKIFY_TOOLSET=all` |
| `CLOCKIFY_ENABLE_RAW_GET` | `false` | Allow raw `GET` outside `CLOCKIFY_TOOLSET=all`; sensitive workspace reads still require `admin` or `all` |
| `CLOCKIFY_ENABLE_RAW_WRITES` | `false` | Allow raw `POST` / `PUT` / `PATCH` / `DELETE` when raw tools are enabled |
| `CLOCKIFY_RAW_WRITE_DOCUMENTED_ONLY` | `true` | Limit raw writes to documented Clockify routes |
| `CLOCKIFY_AUDIT_LOG` | off | Optional local JSONL audit path for redacted tool-call records. This is a local, optional operator-debugging aid, not a tamper-evident or compliance-grade audit trail. |
| `CLOCKIFY_AUDIT_LOG_MODE` | `off` | Audit mode: `off`, `side_effects_only`, or `all` |
| `CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS` | none | Comma-separated allowlist of webhook callback domains |
| `CLOCKIFY_CIRCUIT_BREAKER` | `enabled` | Clockify circuit breaker: `enabled`/`auto`/`on` or `disabled`/`off` |
| `CLOCKIFY_CIRCUIT_BREAKER_FAILURE_THRESHOLD` | `5` | Consecutive upstream failures before the breaker opens |
| `CLOCKIFY_CIRCUIT_BREAKER_OPEN_DURATION` | `45s` | How long the breaker stays open before a half-open probe |
| `CLOCKIFY_CIRCUIT_BREAKER_HALF_OPEN_PROBES` | `1` | Probe requests allowed while the breaker is half-open |
| `MCP_LOG_LEVEL` | `warn` | Log level: `debug`, `info`, `warn`, `error`; the stdio runtime defaults to `warn` so a clean session leaves stderr quiet |

Run `clockify-mcp doctor` to see every resolved value.

### Restoring the previous default

Earlier versions defaulted to `CLOCKIFY_TOOLSET=all`. To keep that behavior
after upgrading, set:

```bash
export CLOCKIFY_TOOLSET=all
```

The full 156-tool surface is advertised and callable in `all`; default, core,
business, and admin now authorize only their advertised subset.

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
The `nightly-live` GitHub Actions workflow re-runs the live suite against the
sacrificial workspace; see
[docs/live-tests.md](docs/live-tests.md#nightly-drift-detection).

## License

[MIT](LICENSE).
