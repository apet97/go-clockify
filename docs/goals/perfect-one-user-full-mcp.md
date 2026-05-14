# Perfect One-User Full Clockify MCP

Branch: `refactor/perfect-one-user-full-mcp`

This document is the product and implementation spec for a clean branch of
`apet97/go-clockify`. It exists before implementation work begins so the branch
does not drift back into the hosted, multi-tenant, safety-gated product shape on
`main`.

## 1. Product Definition

This branch is a local, single-user, full-access Clockify MCP. It runs over
stdio, uses one Clockify API key, targets one pinned workspace, loads all
supported tools at startup, and lets the MCP do everything it can do from the
start.

The product should feel like a direct Clockify workbench for one trusted local
user:

- One local MCP process.
- One Clockify API key.
- One required `CLOCKIFY_WORKSPACE_ID`.
- One optional timezone override.
- All supported tools visible in `tools/list` from startup.
- Workflow tools first, domain tools second, raw API fallback tools last.
- Direct writes with clear IDs, change summaries, warnings, and recovery hints.
- Short initialize instructions that tell the agent how to act, not how to
  navigate safety machinery.

Initial required config:

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `CLOCKIFY_API_KEY` | yes | none | Clockify API key used for every request. |
| `CLOCKIFY_WORKSPACE_ID` | yes | none | The pinned workspace for every tool. |
| `CLOCKIFY_TIMEZONE` | no | Clockify/user/default local behavior | Time parsing and display timezone. |
| `CLOCKIFY_BASE_URL` | no | `https://api.clockify.me/api/v1` | Clockify API base URL. |
| `MCP_LOG_LEVEL` | no | implementation default | Local server logging verbosity. |

## 2. Non-Goals

This branch is not a hosted service and is not a launch-candidate hardening
branch.

Do not build or preserve these product concepts:

- Hosted service.
- Control plane.
- Tenants.
- OIDC.
- mTLS.
- Forward auth.
- gRPC initially.
- Streamable HTTP initially.
- Policy ladder.
- Confirmation-token gate.
- Tier 2 activation.
- Hidden tools.
- Safety-first prompts or initialize instructions.
- Huge environment-variable matrix.
- Kubernetes, deployment, release-hardening, SLSA, or production-operations docs.
- API coverage lab behavior as a user-facing product concept.

## 3. What To Preserve From Main

`main` has a lot of useful material. Mine it where it keeps the new code simple:

- Clockify HTTP client behavior, including request construction, pagination,
  error handling, response caps, report-host handling, multipart support, and
  redirect/path safety.
- Broad Clockify API endpoint knowledge and naming already captured in tools,
  tests, docs, and generated catalog artifacts.
- MCP protocol implementation pieces for stdio, initialize, tools, prompts,
  resources, cancellation, and output schemas.
- Output schema helper patterns where they reduce boilerplate.
- Prompt/resource support, but with product-specific text.
- Timezone and natural time parsing fixes.
- Name resolution patterns for clients, projects, tasks, tags, users, and
  workspace-scoped lookups.
- Fake-server and live-test patterns.
- Prior bug fixes around tag/task preservation, overlap checks, report
  hydration, dry-run envelope consistency, webhook token masking, compact
  upstream errors, and timezone-aware ranges.

Preserve behavior and lessons, not unnecessary architecture.

## 4. What To Remove From Main

Remove or quarantine anything that exists only for the old product shape:

- Streamable HTTP runtime.
- gRPC runtime.
- Multi-tenant control plane.
- Tenant-aware stores, principals, and runtime factories.
- Hosted/shared-service profiles.
- Auth modes and auth negotiation.
- Policy modes and policy-gated tool visibility.
- Confirmation-token and replay-protection enforcement.
- Tier 2 lazy activation and deactivation.
- Runtime tool hiding.
- Production audit durability modes.
- Hosted deployment posture and launch-candidate docs from the new product path.
- User-facing language about policy modes, tenants, activation, confirmation
  tokens, hosted service, or safety gates.

The new branch can look back to `main` for proven code, but it should not make
the user carry `main`'s product complexity.

## 5. Architecture

Keep the package structure simple:

```text
cmd/clockify-mcp
internal/config
internal/clockify
internal/mcp
internal/tools
internal/resolve
internal/testclockify
```

Responsibilities:

- `cmd/clockify-mcp`: process entrypoint, config load, Clockify client creation,
  tool registry creation, stdio server start.
- `internal/config`: tiny environment loader and validation for the five initial
  config variables.
- `internal/clockify`: HTTP client, endpoint methods, pagination, models,
  upstream error normalization, reports support, multipart support.
- `internal/mcp`: minimal MCP JSON-RPC/stdout/stdin protocol handling,
  initialize instructions, tool descriptors, prompts, resources, output schema
  helpers, and call dispatch.
- `internal/tools`: workflow tools, domain tools, raw API fallback tools, result
  envelope helpers, and tool metadata. Split by file inside the package before
  introducing more packages.
- `internal/resolve`: name-to-ID resolution helpers that call `internal/clockify`
  and return predictable ambiguity/not-found errors.
- `internal/testclockify`: stateful fake Clockify server, fixtures, and test
  helpers.

Dependency rule:

```text
cmd/clockify-mcp -> internal/config
cmd/clockify-mcp -> internal/clockify
cmd/clockify-mcp -> internal/mcp -> internal/tools -> internal/clockify
internal/tools -> internal/resolve -> internal/clockify
internal/testclockify is test-only
```

Do not invent a framework. Start with explicit structs and small functions.
Extract only when duplication becomes real.

Initialize instructions must be short and action-oriented:

```text
This is a single-user full-access Clockify MCP for one pinned workspace.
All tools are loaded at startup.
Use workflow tools first.
Use IDs returned by previous calls.
If a feature is unavailable, report it and continue.
```

The initialize instructions must not mention confirmation tokens, policy modes,
Tier 2 activation, tenants, hosted service, or safety gates.

## 6. Tool Model

All supported tools must be loaded at startup.

There must be:

- No activate/deactivate tools.
- No hidden tools.
- No runtime group activation.
- No "tool not found because it is not activated" path.

Tool ordering:

1. Workflow tools.
2. Domain tools.
3. Raw API fallback tools.

Primary workflow tools:

- `clockify_status`
- `clockify_demo_seed`
- `clockify_demo_cleanup`
- `clockify_create_work_package`
- `clockify_log_work`
- `clockify_start_work`
- `clockify_stop_work`
- `clockify_switch_work`
- `clockify_review_day`
- `clockify_review_week`
- `clockify_fix_entry`
- `clockify_invoice_client_work`
- `clockify_record_expense`
- `clockify_request_time_off`
- `clockify_schedule_work`
- `clockify_setup_webhook`

Domain tools should cover:

- Clients.
- Projects.
- Tasks.
- Tags.
- Entries.
- Reports.
- Invoices.
- Expenses.
- Custom fields.
- Time off.
- Scheduling.
- Approvals.
- Webhooks.
- Groups.
- Holidays.
- Users.
- Workspace.

Raw API fallback tools:

- `clockify_api_get`
- `clockify_api_request`

Raw API tools are advanced escape hatches. They should appear last and should not
be recommended when a workflow or domain tool exists.

### First Vertical Slice Tools

Implement only this first:

- `clockify_status`
- `clockify_demo_seed`
- `clockify_demo_cleanup`
- Clients list/create.
- Projects list/create.
- Tasks list/create.
- Tags list/create.
- Entries list/create.

All first-slice tools must be loaded at startup. Activation tools and
confirmation-token flow must not exist in the first-slice runtime.

## 7. Result Envelope Contract

Every tool should return a consistent success envelope:

```json
{
  "ok": true,
  "action": "clockify_projects_create",
  "entity": "project",
  "ids": {
    "workspaceId": "workspace_id",
    "projectId": "project_id"
  },
  "data": {},
  "changed": {
    "created": [],
    "updated": [],
    "deleted": [],
    "reused": []
  },
  "warnings": [],
  "next": []
}
```

Fields:

- `ok`: boolean success marker.
- `action`: stable action name, usually `domain_operation`.
- `entity`: primary entity type when applicable.
- `ids`: all important IDs, especially IDs created or needed by next calls.
- `data`: typed result payload.
- `changed`: created, updated, deleted, and reused entity references.
- `warnings`: non-fatal issues, skipped optional features, degraded feature
  behavior.
- `next`: suggested follow-up tool calls or next human-readable actions.

Every recoverable failure should return:

```json
{
  "ok": false,
  "action": "clockify_projects_create",
  "error": {
    "code": "name_conflict",
    "message": "A project with this name already exists."
  },
  "recovery": {
    "hint": "List projects and reuse the existing project ID, or retry with a different name.",
    "tool": "clockify_list_projects",
    "args": {
      "name": "Example"
    }
  }
}
```

Non-negotiables:

- Every write must return IDs.
- Every write must fill `changed.created`, `changed.updated`,
  `changed.deleted`, or `changed.reused` where applicable.
- Every recoverable error must include `recovery`.
- Raw API tools may return less typed `data`, but they must still use the same
  top-level envelope.

## 8. Implementation Phases

### Phase 0 - Spec And Branch Setup

- Create `refactor/perfect-one-user-full-mcp`.
- Add this spec before changing code.
- Confirm the branch starts from current `main`.

### Phase 1 - First Vertical Slice

Implement only:

- Tiny config.
- Stdio MCP server.
- New initialize instructions.
- Clean result envelope helpers.
- All first-slice tools loaded at startup.
- `clockify_status`.
- Clients list/create.
- Projects list/create.
- Tasks list/create.
- Tags list/create.
- Entries list/create.
- `clockify_demo_seed`.
- `clockify_demo_cleanup`.
- README for the new product.

Do not migrate remaining domains during this phase.

### Phase 2 - Result And Schema Hardening

- Make first-slice output schemas match the envelope.
- Normalize Clockify errors into envelope errors.
- Add recovery hints for common validation, auth, not-found, conflict, feature,
  and upstream failures.
- Ensure every write response includes IDs and changed references.

### Phase 3 - Core Workflow Expansion

- `clockify_create_work_package`
- `clockify_log_work`
- `clockify_start_work`
- `clockify_stop_work`
- `clockify_switch_work`
- `clockify_review_day`
- `clockify_review_week`
- `clockify_fix_entry`

### Phase 4 - Remaining Domain Migration

Migrate domain tools in this order:

1. Reports.
2. Invoices.
3. Expenses.
4. Custom fields.
5. Time off.
6. Scheduling.
7. Approvals.
8. Webhooks.
9. Groups.
10. Holidays.
11. Users.
12. Workspace.

Each migrated domain must be visible at startup and must use the envelope.

### Phase 5 - Remaining Workflow Tools

- `clockify_invoice_client_work`
- `clockify_record_expense`
- `clockify_request_time_off`
- `clockify_schedule_work`
- `clockify_setup_webhook`

### Phase 6 - Raw API Fallback

- Add `clockify_api_get`.
- Add `clockify_api_request`.
- Keep them last in the tool list.
- Ensure path handling cannot escape the configured Clockify base URL.

### Phase 7 - Prompts, Resources, And Docs

- Add action-oriented prompts.
- Add concise resources for status, workspace, user, tools, workflows, demo
  state, recent entries, and recent projects.
- Keep README focused on the local one-user full-access product.
- Do not add hosted, Kubernetes, or release-hardening docs.

## 9. Testing Plan

Fast local gate:

```bash
go test ./...
```

First-slice tests:

- Config loader requires `CLOCKIFY_API_KEY` and `CLOCKIFY_WORKSPACE_ID`.
- Config defaults `CLOCKIFY_BASE_URL` to `https://api.clockify.me/api/v1`.
- MCP initialize response contains the new one-user full-access instructions.
- Initialize response does not contain forbidden runtime concepts.
- `tools/list` shows every first-slice tool at startup.
- `tools/list` contains no activation/deactivation tools.
- `clockify_status` works as the first tool call.
- Clients list/create use the envelope and return IDs on create.
- Projects list/create use the envelope and return IDs on create.
- Tasks list/create use the envelope and return IDs on create.
- Tags list/create use the envelope and return IDs on create.
- Entries list/create use the envelope and return IDs on create.
- `clockify_demo_seed` creates or reuses deterministic objects with a prefix.
- `clockify_demo_cleanup` cleans deterministic objects by prefix and continues
  through partial failures.
- Every recoverable fake-server failure returns `ok=false` plus `recovery`.

Fake Clockify server coverage:

- Stateful clients, projects, tasks, tags, and entries.
- Current user and pinned workspace status.
- Not-found, duplicate/conflict, unauthorized, validation, and feature-unavailable
  responses.
- Demo seed and cleanup idempotency.

Live tests:

- Gated behind explicit env vars.
- Use only the pinned workspace from config.
- Prefix all created demo objects.
- Always attempt cleanup.
- Never print API keys or tokens.

Regression checks:

- No activation tools in user-facing tool list.
- No confirmation-token language in initialize instructions, prompts, README, or
  first-slice user-facing docs.
- No tenant, hosted service, policy-mode, streamable HTTP, or gRPC concepts in
  first-slice user-facing runtime.

## 10. Acceptance Criteria

First slice is accepted when:

- `go test ./...` passes.
- MCP initialize response contains the new one-user full-access instructions.
- `tools/list` shows all first-slice tools at startup.
- No activation tools are present.
- No confirmation-token language is present in the first-slice runtime,
  initialize instructions, prompts, or README.
- `clockify_status` works as the first call.
- `clockify_demo_seed` creates deterministic demo objects with a prefix.
- `clockify_demo_cleanup` cleans those demo objects.
- Every write returns IDs.
- Every write returns `changed.created`, `changed.updated`,
  `changed.deleted`, or `changed.reused` where applicable.
- Every recoverable error returns `recovery`.
- README describes the new local, single-user, full-access product.
- The first slice does not implement remaining domains beyond clients,
  projects, tasks, tags, and entries.

The full branch is accepted when:

- All supported tools are loaded at startup.
- Workflow tools are listed before domain tools, and raw API tools are listed
  last.
- All migrated domain and workflow tools use the result envelope.
- Optional Clockify paid-feature failures report a warning or recovery path and
  do not derail unrelated workflow steps.
- There is no user-facing activation flow, confirmation-token gate, tenant
  system, hosted-service posture, policy ladder, or safety-first instruction set.
- The codebase remains simple enough to understand from the package list above.
