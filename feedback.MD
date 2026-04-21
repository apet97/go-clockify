# Clockify MCP Review

Date: 2026-04-22

## Findings

### 1. High: `docs/policy/production-tool-scope.md` still documents tools that do not exist

The production policy doc is advertising a public tool surface that does not match the server's actual registry.

- `docs/policy/production-tool-scope.md:10-18` names `clockify_get_current_timer`, `clockify_get_summary_report`, `clockify_get_detailed_report`, `clockify_update_project`, `clockify_add_user_to_workspace`, and `clockify_remove_user_from_workspace`.
- The Tier 1 registry in `internal/tools/registry.go:9-181` exposes `clockify_timer_status`, `clockify_summary_report`, and `clockify_detailed_report` instead.
- The checked-in catalog in `docs/tool-catalog.json` confirms the current public surface is 33 Tier 1 tools plus 91 Tier 2 tools, for 124 total, with Tier 2 activation via `clockify_search_tools` and `activate_group` / `activate_tool`.

This is more than naming drift. Operators use this document to decide what is safe to expose in production, so stale names here can lead to incorrect allowlists, confusing client guidance, and false expectations about what the server supports.

### 2. High: runbooks and upgrade docs still point operators at log events the implementation does not emit

Several operator-facing docs instruct maintainers to grep for stale log event names that do not appear in the current codebase.

- `docs/runbooks/auth-failures.md:27-30,40,101` references `auth_rejected` and `clockify_request`.
- `docs/runbooks/rate-limit-saturation.md:27,43` references `clockify_request` and `tool_call_rejected`.
- `docs/runbooks/clockify-upstream-outage.md:18,40` references `clockify_request`.
- `docs/upgrade-checklist.md:113-116` references `protocol_version_negotiated` and `clockify_request`.

The implementation currently emits:

- `http_request` in `internal/mcp/transport_http.go:367-419,459-466`
- `tool_call` in `internal/mcp/tools.go:92-155`
- `initialize` in `internal/mcp/server.go:737-742`

A repository-wide search under `internal/` and `cmd/` found no emitted `clockify_request`, `auth_rejected`, `tool_call_rejected`, or `protocol_version_negotiated` events. That makes the current incident guidance unreliable: the recommended grep patterns will not match the logs maintainers actually have during an outage or auth failure.

### 3. Medium: the doc-parity guard is too noisy to catch real public-surface drift reliably

The repository already has a parity guard, but its current behavior weakens it enough that the stale docs above can survive.

- `scripts/check-doc-parity.sh:107-128` treats any `clockify_*` token in docs as a tool reference.
- The same script downgrades unknown tool references to warnings instead of failures.
- Running `./scripts/check-doc-parity.sh` passed overall, but it warned on both real stale tool names and obvious false positives such as `clockify_request` and `clockify_admin`.

That combination creates warning noise without enforcing the public contract. In practice, maintainers have to manually separate genuine tool drift from unrelated strings that happen to share the `clockify_` prefix, which reduces the script's value as a release gate.

## Strengths

- The architecture is well layered. The MCP protocol layer, runtime, Clockify client, tool registry, and enforcement pipeline are separated cleanly enough to stay understandable despite the size of the project.
- The safety posture is strong. Policy modes, dry-run support, rate limiting, auditing, auth transport handling, and operator deployment guidance are all well above the usual bar for an MCP integration.
- The quality gates are healthy. The repo currently presents as a maintained, test-heavy production service rather than a thin API wrapper.

## Validation Notes

This review is based on static repository inspection and local non-mutating verification only. It does not claim live validation against a real Clockify workspace or API key.

Checks run locally:

- `gofmt -l .` -> passed with no output
- `go vet ./...` -> passed
- `go test ./...` -> passed
- `./scripts/check-doc-parity.sh` -> passed with warnings; the warnings mixed real stale tool references with false positives caused by broad `clockify_*` matching
- `./scripts/check-config-doc-parity.sh` -> passed
- `./scripts/check-config-parity.sh` -> passed

## Scope

- Audience: maintainers and operators, not end users
- Deliverable: review feedback only; no code or public API changes are proposed in this pass
- Review baseline: repository state as inspected on 2026-04-22
