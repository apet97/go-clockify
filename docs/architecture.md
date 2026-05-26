# Architecture

Audience: a new contributor adding or modifying a Clockify MCP tool.

This doc explains the *call graph* between the layers that make up the
156-tool full-access registry. CONTRIBUTING.md covers project structure
in one paragraph; `docs/development.md` covers the per-tool checklist.
What was missing — and what this doc supplies — is the layer ordering,
the descriptor build sequence, and the result-envelope state.

For *why* this matters: a new contributor who wires a tool into the
wrong layer ends up with the wrong envelope shape, no drift-gate
coverage, or a tool that the toolset filter silently drops. The
distinctions below are load-bearing.

---

## 1. Runtime shape (60-second pitch)

- **Transport:** stdio. JSON-RPC over `os.Stdin` / `os.Stdout`.
- **Audience:** one local user, one pinned `CLOCKIFY_WORKSPACE_ID`.
- **Registration:** all 156 tools loaded at startup. No dynamic
  add/remove. `tools/list` is `StaticToolList`-cached.
- **God object:** `tools.Service` holds the HTTP client, workspace ID,
  rate-limit config, confirmation store, and ~200 receiver methods.
  Constructed once in `cmd/clockify-mcp/main.go:runWithContext`.
- **Server instructions (returned in `initialize`):**

  > This is a single-user full-access Clockify MCP for one pinned workspace.
  > All tools are loaded at startup.
  > Use workflow tools first.
  > Use IDs returned by previous calls.
  > If a feature is unavailable, report it and continue.

  Source: `internal/mcp/server.go` `ServerInstructions` (~line 61).

---

## 2. The five tool layers

Tools are organised in five concentric layers, narrowest semantics
first, widest (raw) last. Each layer has its own file in
`internal/tools/`.

| Layer | Priority | File | Purpose |
|---|---|---|---|
| 1. **Workflow** | 0–16 | `oneuser_workflows.go` | High-level composites: `clockify_log_work`, `clockify_review_week`, `clockify_create_work_package`. One tool call = one user intent. |
| 2. **First-slice domain CRUD** | 20–179 | `firstslice.go` | Typed thin wrappers over `Service` methods: list / get / create / update / delete for clients, projects, tasks, tags, entries. Hand-written input schemas. |
| 3. **Native core domain** | 22–149 | `oneuser_native_core.go` | The `get` / `update` / `delete` peers for the first-slice list/create pairs. Same envelope shape; split by file for readability. |
| 4. **Native high-value descriptors** | 200–1104 | `oneuser_native_descriptors.go` | Generated-from-spec domain tools (invoices, expenses, custom fields, time-off, scheduling, approvals, webhooks, groups, holidays, audit log, …). Built from `nativeDomainDescriptorMap()`. |
| 5. **Raw API fallback** | 9000–9001 | `oneuser_raw_api.go` | `clockify_api_get` + `clockify_api_request`. Last resort. Gated by `CLOCKIFY_ENABLE_RAW_TOOLS`. |

Plus two helper layers that ride along the same registration order:

| Helper | File | Purpose |
|---|---|---|
| Native domain extras | `audit_log.go` (`nativeDomainExtras`) | Tools that don't fit the high-value template (e.g. workspace audit log). |
| Timer & report descriptors | `oneuser_timer_reports.go` | Timer start/stop/switch/status + reports/summary/weekly/detailed. |

The **priority** number determines `tools/list` ordering shown to the
client. Workflow tools sort to the top; raw fallback to the bottom.
The catalog drift gate asserts this ordering doesn't shift silently.

---

## 3. The registration call graph

```
cmd/clockify-mcp/main.go : runWithContext()
  └─ service.FullAccessRegistryChecked()              ─┐
        └─ s.buildFullAccessRegistry()                 │  oneuser_domains.go
              ├─ s.workflowDescriptors()               │  ─ Layer 1
              ├─ s.FirstSliceRegistry()                │  ─ Layer 2
              ├─ s.nativeCoreDescriptors()             │  ─ Layer 3
              ├─ s.nativeHighValueDescriptorsChecked() │  ─ Layer 4
              │     ├─ s.nativeDomainDescriptorMap()   │      (builds map keyed by *old* tool name)
              │     └─ s.nativeHighValueDescriptorsFromSources(sources)
              │           └─ for each entry:
              │                 add(priority, NEW_NAME, OLD_NAME, entity, change, handler)
              ├─ s.nativeDomainExtras()                │  ─ helper (audit log etc.)
              ├─ s.timerAndReportDescriptors()         │  ─ helper (timer + reports)
              └─ s.rawAPIDescriptors()                 │  ─ Layer 5
        └─ normalizeDescriptors(out)                   │  (sort + dedupe + sanity)
        └─ ValidateRegistry(descriptors)               │  (every descriptor has a handler etc.)
                                                       │
  service.RegistryForToolset(cfg.Toolset)              │  toolset filter
        └─ FullAccessRegistryChecked() + filter        │  ─ cfg.Toolset ∈ {default, core, business, admin, all}
                                                       ─┘
  mcp.NewServer(version, fullRegistry)
  server.SetAdvertisedTools(advertisedRegistry)
  server.Run(ctx, stdin, stdout)
```

The full registry is **always** loaded; toolset filtering only changes
what is *advertised* in `tools/list`. This matters because the safety
classifier and the drift gates assume the full 156-tool set.

---

## 4. Toolset filter

`RegistryForToolset` (in `oneuser_domains.go`) returns a subset of the
156-tool full registry for narrower advertised surfaces:

| Toolset | Includes | Defined in |
|---|---|---|
| `default` | Descriptors tagged `Tiers: ["default"]` (workflow + entries + the common domain CRUD) | `defaultTier()` helper + per-descriptor opt-in |
| `core` | `coreToolsetTool(name)`: workflow + clients/projects/tasks/tags/entries/reports | `oneuser_domains.go:150` |
| `business` | `core` + invoices + expenses | `oneuser_domains.go:169` |
| `admin` | `business` + custom_fields + time_off + scheduling + approvals + webhooks + groups + holidays + users + workspace_settings | `oneuser_domains.go:184` |
| `all` (default if unset) | Full 156 | — |

Raw API tools (`clockify_api_get` / `clockify_api_request`) are appended
to **any** non-`all` toolset when `EnableRawTools=true`, but as a
trailing escape hatch — never inside the toolset's primary set.

**Drift contract:** changing the membership of any toolset requires
updating `docs/default-toolset.md`, `docs/tool-catalog.md`, and the
catalog drift fixture in one commit.

---

## 5. Result envelopes: current state (and what changes with T2.1)

There are **two** success-envelope types in `internal/tools/`. The
audit (`docs/audits/2026-05-26-claude-audit.md` § T2.1) calls for
convergence onto a single type. The current state:

### `ResultEnvelope` (the older, narrower type)

Defined in `internal/tools/common.go` (~line 180):

```go
type ResultEnvelope struct {
    OK     bool           `json:"ok"`
    Action string         `json:"action"`
    Data   any            `json:"data,omitempty"`
    Meta   map[string]any `json:"meta,omitempty"`
}
```

Used by ~824 return sites. Constructed via the `ok(action, data, meta)`
helper. Wire-locked by the per-tool `outputSchemas` in
`firstslice_output_schemas.go`.

### `ToolResult` (the richer, newer type)

Defined in `internal/tools/firstslice_types.go`:

```go
type ToolResult struct {
    OK       bool              `json:"ok"`
    Action   string            `json:"action"`
    Entity   string            `json:"entity,omitempty"`
    IDs      map[string]string `json:"ids,omitempty"`
    Data     any               `json:"data,omitempty"`
    Meta     map[string]any    `json:"meta,omitempty"`
    Changed  ChangeSet         `json:"changed"`
    Warnings []Warning         `json:"warnings,omitempty"`
    Next     []NextAction      `json:"next,omitempty"`
}
```

Strict superset of `ResultEnvelope` at the wire level: the extra
fields (`Entity`, `IDs`, `Changed`, `Warnings`, `Next`) all serialise
as zero-values when unset. `omitempty` on the optional fields keeps
existing clients pattern-matching `{ok, action, data, meta}` happy.

### The bridge today

`firstslice.go:firstSliceHandler` wraps handlers that return
`(ResultEnvelope, error)` and translates errors into `ToolError`. The
result side is *not* translated — a handler that returns
`ResultEnvelope` produces the narrow envelope on the wire; a handler
that returns `ToolResult` produces the rich one. The MCP client sees
two different shapes depending on which layer answered.

### Convergence direction (T2.1)

Replace `ResultEnvelope` with `ToolResult` as the **single** success
type. Steps (tracked separately):

1. Add `ChangeSet`, `Warnings`, `Next` as **optional** zero-value
   fields to the new unified type.
2. Migrate `ok(action, data, meta)` → `toolResult(action, data, meta)`
   that returns a `ToolResult` with the extra fields zero.
3. Sweep return sites mechanically: `ResultEnvelope` → `ToolResult`.
   `go vet` + test suite catches missed sites.
4. Delete `ResultEnvelope` from `common.go`.

Wire-compatibility for the migration:
- New handlers can set `IDs`, `Changed`, `Warnings`, `Next` freely.
- Old handlers continue to leave them empty → wire shape unchanged.
- The per-tool `outputSchemas` need to allow the new optional fields
  (additive, never required).

Until T2.1 lands, **new handlers should return `ToolResult` directly**
and use the `Changed` field to record what was created/updated/deleted.

---

## 6. How to add a new tool, end-to-end

1. **Pick the layer** (table in §2). New high-level user intent →
   layer 1 (workflow). Mirror of an existing Clockify endpoint → layer
   4 (native high-value) is the default.
2. **Implement the handler** as a method on `*Service`. Return
   `ToolResult` (preferred) or `ResultEnvelope` (pre-T2.1). Validate
   args locally (alias pairs, required fields) before calling the
   Clockify client.
3. **Register the descriptor** in the matching layer file (e.g.
   `oneuser_native_descriptors.go` for layer 4). Use
   `nativeDomainTool` / `firstSliceDescriptor` / `workflowDescriptor`
   helpers — they wire up `RiskClass`, output schema, and the recovery
   wrapper.
4. **Wire output schema** in `firstslice_output_schemas.go` (panics at
   startup if a registered tool has no entry, by design).
5. **Risk + safety:** if the tool mutates state, set
   `SafetyRequirementFunc` (raw API does this; high-value descriptors
   inherit from `nativeDomainTool`). Destructive actions get
   confirmation tokens automatically.
6. **Tests:** add a `*_test.go` against the fake Clockify
   (`internal/testclockify/`). For live evidence, gate with
   `CLOCKIFY_RUN_LIVE_E2E=1` in `tests/`.
7. **Regenerate + drift-check** generated surfaces:
   ```sh
   make gen-tool-catalog
   bash scripts/check-api-parity-matrix.sh --write
   make gen-coverage-dashboard
   make gen-raw-allowlist
   make sync-selfinspect-assets
   make catalog-drift api-parity-matrix-drift coverage-dashboard-drift raw-allowlist-drift selfinspect-drift
   ```
8. **Tool-count guard:** if the new tool changes the 156 count, update
   the catalog baseline + live-tests count + `internal/tools` size
   assertion in the same commit. Tool count is a product contract.

---

## 7. Drift gates that pin the registry shape

| Gate | What it pins | Why |
|---|---|---|
| `catalog-drift` | Sorted catalog vs `docs/tool-catalog.{json,md}` | Detects silent tool addition / rename / reorder. |
| `raw-allowlist-drift` | `internal/tools/raw_allowlist_gen.go` vs the raw API allowlist | Stops accidental widening of the raw fallback. |
| `selfinspect-drift` | The MCP server's self-inspection bundle vs `docs/selfinspect/` | Catches input/output schema drift end-to-end. |
| `api-parity-matrix-drift` | `docs/api-parity-matrix.md` vs the live coverage ledger | Aligns the MCP tool surface with the OpenAPI ops. |
| `coverage-dashboard-drift` | `docs/tool-coverage-dashboard.md` vs the runtime registry | Per-domain coverage snapshot. |
| `mod-tidy-drift` | `go.mod` / `go.sum` | Prevents stealth dependency additions. |
| `openapi-drift` | The canonical OpenAPI snapshot (regenerated via Ruby) | Holds the spec pipeline deterministic. |

All seven run in `ci.yml`. `make perfect` chains them with `fmt + vet
+ test`. Never push to `main` with any drift gate red.

---

## 8. The `Service` god object (one paragraph)

`tools.Service` carries the runtime context for every handler: the
Clockify HTTP client, workspace ID, default timezone, audit log path,
confirmation-token store, rate-limit thresholds, feature flags
(`EnableRawTools`, `EnableRawGet`, `EnableRawWrites`,
`RawWriteDocumentedOnly`), notifier, subscription gate, and resource
provider hooks. Every tool handler is a method on `*Service`. The
struct lives in `internal/tools/context.go`; it's constructed once at
startup and passed by pointer everywhere. Receiver naming is **always**
`s *Service` — no exceptions.

---

## 9. Glossary

- **Descriptor** — `mcp.ToolDescriptor`: a `Tool` (name, description,
  schemas, annotations) plus a `Handler`, plus `Tiers` for toolset
  filtering, plus optional `SafetyRequirementFunc` for risk-class
  routing.
- **Toolset** — one of `default`, `core`, `business`, `admin`, `all`.
  Selects which subset of the 156-tool full registry the server
  *advertises* in `tools/list`. The full registry is always loaded.
- **Risk class** — one of `read`, `write`, `billing_admin`,
  `destructive`. Each has its own rate limit and confirmation policy.
  See `internal/mcp/risk.go`.
- **Confirmation token** — short-lived token returned by a `dry_run`
  preview, required to call the destructive tool for real. Stored in
  `internal/safety/`.
- **Phantom path** — a Clockify route that the published OpenAPI lists
  but the live API rejects (404/405). Quarantined in
  `scripts/gen-clockify-openapi:PHANTOM_PATHS`. Never registered as a
  tool.

---

## See also

- `CONTRIBUTING.md` — project structure, common commands, design
  principles.
- `docs/development.md` — per-tool implementation checklist.
- `docs/agent-cookbook.md` — task-oriented examples for LLM agents.
- `docs/tool-catalog.md` — generated catalog of every registered tool.
- `docs/audits/2026-05-26-claude-audit.md` — the audit that produced
  this doc; § T2.1 + T2.3.
