# go-clockify

A production-grade Go MCP server for Clockify — 124 tools, zero external dependencies.

## Features

- **124 MCP tools** — 33 Tier 1 (always available) + 91 Tier 2 (on-demand via 11 domain groups)
- **Dual transport** — stdio (default) and HTTP with bearer auth, CORS, health/ready endpoints
- **Safety stack** — policy modes, dry-run preview, rate limiting, duplicate detection, token truncation
- **Stdlib only** — zero external dependencies, single binary
- **Progressive disclosure** — bootstrap modes control tool visibility; `search_tools` activates domains on demand

## Quick Start

```bash
# Stdio mode (for Claude Desktop, Cursor, etc.)
CLOCKIFY_API_KEY=your-key go run ./cmd/clockify-mcp

# HTTP mode
CLOCKIFY_API_KEY=your-key MCP_TRANSPORT=http MCP_BEARER_TOKEN=secret go run ./cmd/clockify-mcp
```

### MCP Client Config (Claude Desktop)

```json
{
  "mcpServers": {
    "clockify": {
      "command": "go",
      "args": ["run", "./cmd/clockify-mcp"],
      "cwd": "/path/to/go-clockify",
      "env": {
        "CLOCKIFY_API_KEY": "your-key"
      }
    }
  }
}
```

## Tool Surface

### Tier 1 — Always Available (33 tools)

| Domain | Tools |
|---|---|
| **Context** | `whoami`, `search_tools`, `policy_info`, `resolve_debug` |
| **Timer** | `start_timer`, `stop_timer`, `timer_status` |
| **Entries** | `list_entries`, `get_entry`, `today_entries`, `add_entry`, `update_entry`, `delete_entry` |
| **Projects** | `list_projects`, `get_project`, `create_project` |
| **Clients** | `list_clients`, `create_client` |
| **Tags** | `list_tags`, `create_tag` |
| **Tasks** | `list_tasks`, `create_task` |
| **Users** | `current_user`, `list_users` |
| **Workspaces** | `list_workspaces`, `get_workspace` |
| **Reports** | `summary_report`, `detailed_report`, `weekly_summary`, `quick_report` |
| **Workflows** | `log_time`, `switch_project`, `find_and_update_entry` |

### Tier 2 — On-Demand (91 tools across 11 groups)

Activate via `search_tools` with `activate_group`:

| Group | Tools | Highlights |
|---|---|---|
| **invoices** | 12 | CRUD, send, mark paid, line items, reporting |
| **expenses** | 10 | CRUD, categories, reporting by category |
| **scheduling** | 10 | Assignments, schedules, capacity planning |
| **time_off** | 12 | Requests, policies, approve/deny, balances |
| **approvals** | 6 | Timesheet submission, approve, reject, withdraw |
| **shared_reports** | 6 | CRUD, export (CSV/JSON/PDF/Excel) |
| **user_admin** | 8 | Groups, roles, deactivation |
| **webhooks** | 7 | CRUD with HTTPS/private-IP validation, test delivery |
| **custom_fields** | 6 | CRUD, set values on entries/projects |
| **groups_holidays** | 8 | User groups (admin), public holidays |
| **project_admin** | 6 | Templates, estimates, memberships, batch archive |

## Safety

### Policy Modes (`CLOCKIFY_POLICY`)

| Mode | Read | Tier 1 Write | Destructive | Tier 2 |
|---|---|---|---|---|
| `read_only` | yes | no | no | no |
| `safe_core` | yes | 11 safe tools | no | no |
| `standard` | yes | yes | yes (dry-run) | on-demand |
| `full` | yes | yes | yes (dry-run) | all |

### Enforcement Pipeline

Every `tools/call` passes through: **policy → rate limit → dry-run intercept → handler → truncation → logging**

### Dry-Run

Destructive tools support `dry_run: true` with 3 interception strategies:
- **Confirm pattern** — removes confirm flag, calls handler for preview
- **GET preview** — calls the GET counterpart (e.g., `delete_entry` → `get_entry`)
- **Minimal fallback** — echoes parameters without API call

### Additional Safety

- **Rate limiting** — concurrent call semaphore + sliding window throughput limit
- **Duplicate detection** — 3-part match (description + project + start time) with warn/block modes
- **Overlap detection** — prevents overlapping time entries on same project
- **Token truncation** — progressive output truncation to stay within LLM context budgets
- **Name resolution** — ambiguity blocking, email detection for users, actionable error messages

## Environment Variables

### Core
| Variable | Required | Default |
|---|---|---|
| `CLOCKIFY_API_KEY` | yes | — |
| `CLOCKIFY_WORKSPACE_ID` | no | auto-resolve |
| `CLOCKIFY_BASE_URL` | no | `https://api.clockify.me/api/v1` |
| `CLOCKIFY_REPORTS_URL` | no | — |
| `CLOCKIFY_TIMEZONE` | no | system |

### Safety
| Variable | Default | Values |
|---|---|---|
| `CLOCKIFY_POLICY` | `standard` | `read_only`, `safe_core`, `standard`, `full` |
| `CLOCKIFY_DRY_RUN` | enabled | `off` to disable |
| `CLOCKIFY_DEDUPE_MODE` | `warn` | `warn`, `block`, `off` |
| `CLOCKIFY_BOOTSTRAP_MODE` | `full_tier1` | `full_tier1`, `minimal`, `custom` |
| `CLOCKIFY_MAX_CONCURRENT` | `10` | max simultaneous calls |
| `CLOCKIFY_RATE_LIMIT` | `120` | max calls per 60s |
| `CLOCKIFY_TOKEN_BUDGET` | `8000` | truncation threshold (0=off) |

### Transport
| Variable | Default |
|---|---|
| `MCP_TRANSPORT` | `stdio` |
| `MCP_HTTP_BIND` | `:8080` |
| `MCP_BEARER_TOKEN` | — (required for HTTP) |
| `MCP_LOG_FORMAT` | `text` (`json` for structured) |

## Development

```bash
go build ./...          # Build
go test ./...           # 276 tests across 13 packages
gofmt -w ./cmd ./internal  # Format
```

## Architecture

```
cmd/clockify-mcp/          Entrypoint — wires 8 subsystems
internal/
  config/                   Environment config + validation
  clockify/                 HTTP client (retry, pagination, typed errors)
  mcp/                      Server (stdio + HTTP), enforcement pipeline
  tools/                    33 Tier 1 + 91 Tier 2 handlers, registry
  policy/                   4 modes + group/tool deny/allow lists
  resolve/                  Name→ID resolution with ambiguity blocking
  dryrun/                   3-strategy dry-run interception
  bootstrap/                Tool visibility modes + searchable catalog
  ratelimit/                Semaphore + sliding window rate limiting
  truncate/                 Progressive token-aware output truncation
  dedupe/                   Duplicate + overlap detection
  timeparse/                Natural language time parsing
  helpers/                  Error mapping, pagination, write envelopes
```

## License

Private — not yet published.
