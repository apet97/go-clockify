# GOCLMCP Production Plan

## Goal

Build a production-ready **Go MCP server for Clockify** that is safe, maintainable, testable, observable, and compatible with real MCP clients such as Claude Desktop, Cursor, and OpenClaw-style runtimes.

This plan assumes we want more than a demo:
- strong MCP compliance
- broad Clockify coverage
- safe write/destructive behavior
- stable packaging and release process
- good docs and onboarding

---

## Definition of Done

GOCLMCP is production-ready when it has all of the following:

### Product / Client Readiness
- Works with stdio MCP clients reliably
- Optionally supports HTTP transport for hosted/server deployments
- Returns valid MCP-compatible `initialize`, `tools/list`, and `tools/call` responses
- Includes stable tool schemas and clear tool descriptions
- Supports both discovery and safe execution patterns

### Clockify Coverage
- Covers the most important Clockify workflows:
  - whoami / context
  - workspaces
  - users
  - projects
  - clients
  - tags
  - tasks
  - entries
  - timer
  - reports
  - workflow helpers
- Has a path for advanced or admin-only domains later:
  - invoices
  - expenses
  - time off
  - approvals
  - scheduling
  - webhooks
  - custom fields
  - groups/admin

### Safety
- Policy modes for read-only / safe-core / standard / full
- Dry-run support for destructive tools
- Name-to-ID resolution with ambiguity blocking
- Validation of IDs and user input
- Secure handling of API keys and config
- Rate limiting and concurrency control

### Engineering Quality
- Clean package layout
- Unit tests + integration tests + golden/schema tests
- Structured logs
- Versioning and release pipeline
- Reproducible builds
- Good README + examples + client config docs

---

# Current State

The current repository is a scaffold and **not production-ready yet**.

It currently has:
- basic Go module and package layout
- small Clockify client
- minimal stdio JSON-RPC loop
- a few starter tools:
  - `clockify_whoami`
  - `clockify_list_workspaces`
  - `clockify_list_projects`
  - `clockify_start_timer`
  - `clockify_stop_timer`

Current gaps:
- not full MCP behavior/spec coverage
- no robust schema/annotation layer
- no policy modes
- no dry-run framework
- no resolution layer
- no tests yet
- no observability layer
- no release pipeline
- no packaging/install story

---

# Delivery Strategy

We should build this in **8 phases**.

## Phase 1 — Foundation & Protocol Correctness
**Objective:** Make the server structurally correct and stable as an MCP server.

### Deliverables
- Replace ad hoc JSON-RPC loop with a proper MCP-compatible server core
- Implement:
  - `initialize`
  - `notifications/initialized` handling
  - `tools/list`
  - `tools/call`
  - `ping`
- Standardize response and error handling
- Add version metadata and server capabilities
- Separate protocol transport from tool logic

### Architecture work
Create these packages:
- `internal/mcp/protocol`
- `internal/mcp/server`
- `internal/mcp/transport/stdio`
- later: `internal/mcp/transport/http`

### Acceptance criteria
- Claude/Desktop-style client can initialize and list tools cleanly
- malformed requests return proper JSON-RPC/MCP errors
- logs stay off stdout in stdio mode

---

## Phase 2 — Robust Clockify Core Layer
**Objective:** Make the Clockify client trustworthy and reusable.

### Deliverables
- Harden HTTP client:
  - retries with backoff for 429 / transient 5xx
  - timeout control
  - user agent
  - structured API errors
- Add typed request/response models for key entities
- Add pagination helpers
- Add shared helpers for:
  - GET / POST / PUT / PATCH / DELETE
  - query building
  - response decoding
- Add config validation:
  - API key required
  - HTTPS enforcement for custom base URL unless explicitly overridden
  - workspace auto-resolution when only one workspace exists

### Acceptance criteria
- all client errors become actionable tool errors
- retry behavior is deterministic and tested
- client package is usable independently of MCP layer

---

## Phase 3 — Tool Model, Schemas, and Core Domains
**Objective:** Define the real tool surface and implement high-value domains first.

### Priority domains
1. meta/context
2. timer
3. entries
4. projects
5. clients
6. tags
7. tasks
8. users
9. workspaces
10. reports
11. workflows/search helpers

### Deliverables
For each tool:
- stable tool name
- JSON schema for input
- clear description
- read/write/destructive metadata
- predictable result structure

Status note: report and workflow helpers are now partially delivered in a pragmatic form by aggregating the current user's time-entry data instead of depending on a broader/less-certain reports API surface.

### Proposed Tier 1 tool set
- `clockify_whoami`
- `clockify_policy_info`
- `clockify_search_tools`
- `clockify_list_workspaces`
- `clockify_get_workspace`
- `clockify_current_user`
- `clockify_list_users`
- `clockify_list_projects`
- `clockify_get_project`
- `clockify_create_project`
- `clockify_list_clients`
- `clockify_create_client`
- `clockify_list_tags`
- `clockify_create_tag`
- `clockify_list_tasks`
- `clockify_create_task`
- `clockify_timer_status`
- `clockify_start_timer`
- `clockify_stop_timer`
- `clockify_list_entries`
- `clockify_get_entry`
- `clockify_today_entries`
- `clockify_add_entry`
- `clockify_update_entry`
- `clockify_delete_entry`
- `clockify_summary_report`
- `clockify_detailed_report`
- `clockify_log_time`
- `clockify_switch_project`
- `clockify_find_and_update_entry`
- `clockify_quick_report`
- `clockify_weekly_summary`
- `clockify_resolve_debug`

### Acceptance criteria
- Tier 1 is complete and tested
- tools have stable schemas
- results are formatted consistently

---

## Phase 4 — Safety Framework
**Objective:** Make agent use safe enough for real-world deployment.

### Deliverables

#### 4.1 Policy modes
Implement:
- `read_only`
- `safe_core`
- `standard`
- `full`

Behavior:
- block tools by mode
- optionally block whole domain groups
- optionally block specific tool names
- always allow introspection tools

#### 4.2 Dry-run framework
- support `dry_run: true` on destructive tools
- for some tools, return resource previews
- for others, return minimal no-op envelopes
- support confirm-pattern where appropriate

#### 4.3 Input validation
- validate raw IDs
- reject malformed path-ish IDs
- validate dates / times / report ranges
- reject ambiguous name resolution

#### 4.4 Name resolution layer
Implement resolvers for:
- project
- client
- tag
- user
- task
- workspace if needed

### Acceptance criteria
- destructive actions can be previewed
- ambiguous names fail closed
- policy enforcement works in both `tools/list` and `tools/call`

---

## Phase 5 — Discovery, Bootstrap, and Tool Scaling
**Objective:** Keep the server usable as the tool surface grows.

### Deliverables

#### 5.1 Bootstrap modes
- `full_tier1`
- `minimal`
- `custom`

#### 5.2 Discovery tool
Implement `clockify_search_tools` for:
- searching available tools/domains
- discovering deferred tools
- activating optional tool groups

#### 5.3 Tier 2 domains
Add on-demand domains:
- invoices
- expenses
- scheduling
- time off
- approvals
- shared reports
- user admin
- webhooks
- custom fields
- groups / holidays
- project admin

### Acceptance criteria
- server remains usable even with 100+ tools
- clients can discover advanced tools without overwhelming tool lists

---

## Phase 6 — Observability, Reliability, and Performance
**Objective:** Make it operable in production.

### Deliverables
- structured logging to stderr
- text/json log formats
- request correlation IDs where practical
- metrics hooks
- optional Prometheus endpoint for HTTP mode
- concurrency limiting
- rate limiting
- response truncation / token-budget awareness
- health / readiness endpoints for HTTP mode

### Reliability goals
- no stdout pollution in stdio mode
- clean shutdown handling
- deterministic error messages
- no panics from invalid user/tool input

### Acceptance criteria
- can diagnose failures from logs alone
- load behavior under concurrency is predictable

---

## Phase 7 — Packaging, Distribution, and Security Hygiene
**Objective:** Make it easy and safe to install.

### Deliverables
- single binary release builds for:
  - darwin arm64/x64
  - linux x64/arm64
  - windows x64
- npm wrapper or installer script if desired
- Homebrew tap formula later if worth it
- sample client configs for:
  - Claude Desktop
  - Cursor
  - OpenClaw-style usage
- `.env.example`
- no real secrets in repo
- release automation via GitHub Actions
- semantic versioning

### Security hygiene
- no plaintext live keys in committed config
- explicit docs for local secret storage
- CI checks for accidental secret leakage patterns
- dependency audit in CI

### Acceptance criteria
- fresh user can install in <10 minutes
- release artifacts are reproducible and documented

---

## Phase 8 — QA, Compatibility, and Launch
**Objective:** Make sure it behaves well in the wild.

### Deliverables

#### Test layers
1. **Unit tests**
   - config parsing
   - time parsing
   - resolution
   - policy
   - dry-run
   - rate limiting

2. **Integration tests**
   - mock Clockify API via httptest server
   - MCP initialize / list / call flows
   - HTTP mode auth/CORS tests

3. **Golden tests**
   - tool list snapshot
   - schema snapshot
   - error message expectations

4. **Client compatibility tests**
   - Claude Desktop smoke test
   - Cursor smoke test
   - local stdio harness test

5. **Manual acceptance tests**
   - timer start/stop
   - project lookup by name
   - ambiguous name failure
   - dry-run delete preview
   - report generation

### Launch criteria
- all critical tool flows pass
- docs are complete
- release process works
- no known secret/config leaks

---

# Recommended Repo Structure

```text
GOCLMCP/
  cmd/
    clockify-mcp/
      main.go
  internal/
    app/
    config/
    clockify/
      client.go
      errors.go
      pagination.go
      models/
      endpoints/
    mcp/
      protocol/
      server/
      transport/
        stdio/
        http/
      schema/
    tools/
      meta/
      timer/
      entries/
      projects/
      clients/
      tags/
      tasks/
      users/
      workspaces/
      reports/
      workflows/
    policy/
    dryrun/
    resolve/
    bootstrap/
    ratelimit/
    truncate/
    observability/
  tests/
    integration/
    golden/
  docs/
    architecture.md
    tool-catalog.md
    deployment.md
  .github/
    workflows/
```

---

# Recommended Engineering Principles

## 1. Fail closed
If the server cannot safely decide:
- do not guess
- do not auto-pick ambiguous resources
- return an actionable error

## 2. Keep stdout pure
For stdio MCP:
- protocol responses only on stdout
- logs only on stderr

## 3. Prefer typed internal models
Avoid a codebase full of raw `map[string]any` except at the protocol boundary.

## 4. Separate concerns hard
Do not let:
- HTTP transport details
- MCP protocol details
- Clockify API details
- business tool logic

collapse into one package.

## 5. Metadata must drive safety consistently
If a tool is destructive:
- annotation
- policy behavior
- dry-run behavior
- docs
- tests

should all line up.

---

# Concrete Task Backlog

## Milestone A — Protocol Core
- [x] replace current ad hoc MCP loop with protocol package
- [x] formalize request/response types
- [x] add initialized notification handling
- [x] add standardized error mapping
- [x] ensure stderr-only logging

## Milestone B — Clockify Client Hardening
- [x] add API error type with status/body context
- [x] add retry/backoff transport wrapper
- [x] add DELETE / PUT helpers
- [x] add pagination helpers
- [x] add config validation and workspace auto-resolve

## Milestone C — Tier 1 Tools
- [x] implement meta/context tools
- [x] implement timer tools
- [x] implement entry tools
- [x] implement project/client/tag/task/user/workspace tools
- [x] implement report tools (currently via safe time-entry aggregation helpers)
- [x] implement workflow helper tools (`clockify_log_time`, `clockify_quick_report`, `clockify_find_and_update_entry` foundation)

## Milestone D — Safety
- [x] implement policy modes
- [x] implement dry-run interception
- [x] implement resolvers
- [x] implement ID validation
- [x] add ambiguity-safe behavior

## Milestone E — Tool Scaling
- [ ] implement bootstrap modes
- [ ] implement `clockify_search_tools`
- [ ] add Tier 2 groups
- [ ] add list_changed behavior if supported in design

## Milestone F — Reliability / Ops
- [ ] structured logs
- [ ] rate limiting
- [ ] truncation
- [ ] metrics hooks
- [ ] HTTP transport
- [ ] health/ready endpoints

## Milestone G — Release
- [ ] GitHub Actions test/build/release
- [ ] multi-platform binaries
- [ ] npm wrapper optional
- [ ] docs and examples
- [ ] secret hygiene checks

---

# Resource Plan — “Use all resources”

To do this properly, use all of these resource classes:

## Local codebase references
- current `GOCLMCP` scaffold
- Rust `clmcp` repo as reference implementation / feature map

## Upstream product/API references
- Clockify API docs
- MCP protocol docs/spec examples
- real client behavior from Claude Desktop / Cursor expectations

## Validation resources
- local mock servers
- real Clockify account testing with safe read-only flows first
- golden tests / schema snapshots

## Automation resources
- CI workflows
- linters (`go test`, `go vet`, maybe `staticcheck`)
- release automation

---

# Critical Risks

## Risk 1 — Overbuilding too early
If we chase 100+ tools before stabilizing protocol + safety, the codebase will get messy.

**Mitigation:**
Build strong core first, then scale tool count.

## Risk 2 — MCP compatibility drift
A server can look fine locally and still behave badly in real clients.

**Mitigation:**
Test against actual MCP clients early, not only unit tests.

## Risk 3 — Secret leakage
Sample configs often accidentally carry real keys.

**Mitigation:**
No live secrets in repo. Add checks and examples only.

## Risk 4 — Unsafe destructive tooling
Deletion/admin tools without dry-run and policy constraints are a footgun.

**Mitigation:**
No destructive/admin tool shipped without policy + dry-run + tests.

---

# Recommended Execution Order

If I were building this myself, I would do it in this exact order:

1. Harden MCP protocol core
2. Harden Clockify client
3. Implement Tier 1 read-only tools
4. Implement Tier 1 write tools
5. Add policy + dry-run + resolution
6. Add tests + golden snapshots
7. Add bootstrap/discovery
8. Add HTTP mode + metrics
9. Add Tier 2 domains
10. Add release automation and packaging

That order gives the best balance of correctness, safety, and momentum.

---

# Immediate Next Actions

## Next action set (recommended)
1. Refactor current code into cleaner packages
2. Implement a proper MCP response model and tool registry
3. Add typed Clockify error handling
4. Build out read-only Tier 1 tools first
5. Add first test suite before expanding too far

## Short-term target
Reach a **v0.2** that has:
- correct MCP stdio behavior
- robust read-only tools
- stable schemas
- test coverage

## Medium-term target
Reach a **v0.5** that has:
- write tools
- safety framework
- dry-run
- policy modes
- resolver layer

## v1.0 target
- stable Tier 1 set
- optional Tier 2 activation
- HTTP transport
- CI/release pipeline
- documented install paths
- real client compatibility proven

---

# Final Recommendation

Do **not** try to jump straight from the current scaffold to a giant feature dump.

The right move is:
- build a clean protocol core,
- stabilize the Clockify client,
- add a disciplined Tier 1 tool surface,
- then layer in safety, discovery, and packaging.

That’s how this becomes a production MCP instead of a fragile demo.
