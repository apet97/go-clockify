# Internal Test Posture

Use this page when running `clockify-mcp` locally with an owner-level
Clockify API key. It describes what the local `stdio` profile protects
by default, what changes when `CLOCKIFY_POLICY` is widened, and which
guards should stay enabled while testing against a real workspace.

## Baseline

Recommended local owner-key test environment:

```env
MCP_PROFILE=local-stdio
CLOCKIFY_API_KEY=<owner-api-key>
CLOCKIFY_WORKSPACE_ID=<test-workspace-id>
CLOCKIFY_POLICY=time_tracking_safe
```

`CLOCKIFY_WORKSPACE_ID` should be pinned even when auto-detection works.
Owner keys commonly see many workspaces, and `auto` is only predictable
when the key can see exactly one.

Use a sacrificial or explicitly approved workspace for destructive or
broad write testing. Do not point an exploratory agent at a personal,
teammate, customer, or production workspace.

## Policy Levels

| Policy | Local testing use | Write surface |
|---|---|---|
| `read_only` | Dashboard checks, catalog inspection, report smoke. | No writes. |
| `time_tracking_safe` | Default for AI-facing time-tracking tests. | Time-entry and timer writes only; no workspace-shaping creates. |
| `safe_core` | Trusted local assistant that may create project structure. | Time-entry writes plus project, client, tag, and task creation; no deletes or Tier 2. |
| `standard` | Trusted operator debugging only. | Tier 1 writes and deletes; Tier 2 groups remain opt-in. |
| `full` | Short, explicit admin automation only. | Broadest surface, including Tier 2 admin/billing groups once activated. |

Prefer `safe_core` over `standard` or `full` when the test only needs to
create projects, clients, tags, or tasks. Use `standard` and `full` only
when the specific delete, billing, admin, or Tier 2 behaviour is the
thing being tested.

### Cleanup Under `time_tracking_safe`

`time_tracking_safe` allows normal time-entry creation and updates but
intentionally blocks destructive cleanup tools such as
`clockify_delete_entry`. For safe-mode dogfood tests that create real
entries, plan one of these cleanup paths before the run starts:

- temporarily switch the same sacrificial workspace to `standard` only
  for cleanup, then switch back to `time_tracking_safe`;
- delete the temporary entries in the Clockify UI;
- use dry-run-only tests when cleanup cannot be guaranteed.

Do not relax the default policy just to make cleanup convenient. The
safe-mode deny is the expected policy behaviour; the cleanup path is a
test-harness responsibility.

## Guards That Should Stay On

Keep these enabled unless the test explicitly needs to prove the escape
hatch:

| Guard | Recommended value | Why |
|---|---|---|
| `CLOCKIFY_DRY_RUN` | `enabled` | Lets callers preview supported writes with `dry_run:true`. |
| `CLOCKIFY_OVERLAP_CHECK` | `true` | Blocks overlapping finished entries unless `allow_overlap:true` is explicit. |
| `CLOCKIFY_DEDUPE_MODE` | `warn` for exploratory testing, `block` for cautious mutation testing | Warn mode surfaces likely duplicate entries without blocking; block mode is safer when an agent is allowed to write repeatedly. |
| `CLOCKIFY_REPORT_MAX_ENTRIES` | bounded, default `10000` | Prevents a broad report from materializing unbounded entry sets. |
| `CLOCKIFY_RATE_LIMIT` and per-token limits | leave defaults unless load-testing | Prevents accidental tight loops from turning into high-volume API traffic. |

When raising `CLOCKIFY_POLICY` above `time_tracking_safe`, consider also
setting:

```env
CLOCKIFY_DEDUPE_MODE=block
```

This is especially important for repeated prompt/repair loops where an
agent might retry a write after seeing partial output.

## Verification

Before handing a local owner-key setup to an agent, run:

```bash
clockify-mcp doctor --profile=local-stdio --strict
```

Expected outcome:

- exit code `0`;
- `MCP_PROFILE=local-stdio`;
- `MCP_TRANSPORT=stdio`;
- `CLOCKIFY_WORKSPACE_ID` is set to the intended test workspace;
- `CLOCKIFY_POLICY` is no broader than the test requires.

For a read/write smoke, use only a sacrificial workspace and prefer a
dry-run first. Local smoke results do not count as launch-candidate live
evidence; the launch gate still requires the scheduled live-contract
workflow evidence named in `docs/launch-candidate-checklist.md`.
