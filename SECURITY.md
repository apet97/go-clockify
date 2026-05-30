# Security Policy

## Supported Versions

`clockify-mcp` ships a single supported release line. Security fixes land on
the latest release.

| Version | Status    |
| ------- | --------- |
| 0.4.x   | Supported |

## Reporting a Vulnerability

**Do not open a public issue for security vulnerabilities.**

Report privately through GitHub's Security Advisory workflow:
<https://github.com/apet97/go-clockify/security/advisories/new>

Please include:

- A description of the vulnerability
- Steps to reproduce
- The affected version
- The potential impact

## Response

- **Acknowledgment:** within 48 hours
- **Initial assessment:** within 1 week
- **Fix:** severity-dependent — critical as soon as possible, high within
  1-2 weeks, medium in the next release

## Scope

`clockify-mcp` is a local, single-user, stdio MCP server. It runs as a
subprocess of a trusted MCP client, holds one `CLOCKIFY_API_KEY`, and talks to
one Clockify workspace. It opens no network listener and has no multi-user
surface.

For setup and usage, see [README.md](README.md),
[docs/agent-cookbook.md](docs/agent-cookbook.md),
[docs/tool-catalog.md](docs/tool-catalog.md), and
[docs/goals/oneuser-tool-coverage.md](docs/goals/oneuser-tool-coverage.md).

In scope:

- API key exposure or leakage — in logs, error messages, or tool output
- Command or path injection through tool inputs
- Server-side request forgery (SSRF) through webhook URL parameters
- Path traversal in workspace or entity ID handling

Out of scope: anything that requires a second user, an inbound network
listener, or a modified build — the server has none of those.

## Security Properties

- **Credentials from the environment only.** `CLOCKIFY_API_KEY` is read from
  the environment, never from a config file, and `doctor` reports it only as
  `set (redacted)`.
- **Redacted logs.** The logging handler masks well-known secret keys and
  secret-shaped values before anything is written.
- **Stdout stays clean.** Only JSON-RPC frames go to stdout; every log line
  goes to stderr, so diagnostics never mix into the protocol stream.
- **ID validation at startup.** `CLOCKIFY_WORKSPACE_ID` is validated when
  configuration loads — path-traversal-shaped values (`/ ? # % ..`, control
  bytes) fail fast instead of flowing into request URLs.
- **Webhook URL validation.** Webhook tools reject non-HTTPS URLs, embedded
  credentials, and hosts that resolve to localhost, private, reserved, or
  link-local addresses. `CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS` is the explicit
  allowlist escape hatch.
- **Raw API writes are off by default.** `clockify_api_get` and
  `clockify_api_request` stay scoped to the pinned workspace; raw `POST`,
  `PUT`, `PATCH`, and `DELETE` require `CLOCKIFY_ENABLE_RAW_WRITES=true`.
- **Panic containment.** The stdio dispatch loop recovers panics and returns a
  tool-error envelope instead of crashing the process.
- **Bounded results.** Tool results are capped (`CLOCKIFY_MAX_TOOL_RESULT_BYTES`,
  default 50000); oversized exports spill to a temp file rather than inline.
- **Minimal dependencies.** The binary is effectively standard-library only,
  which keeps the supply-chain surface small.
