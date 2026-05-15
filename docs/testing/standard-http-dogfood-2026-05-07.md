# Standard-Policy Single-Tenant HTTP Dogfood — 2026-05-07

> **Historical artifact. Not current one-user MCP product documentation.**
> Preserved for platform-era audit/history only. Start current one-user work from `README.md`, `docs/agent-cookbook.md`, `docs/tool-catalog.md`, and `docs/goals/oneuser-tool-coverage.md`.


This note records internal dogfood evidence only. It is not
launch-candidate evidence and does not close Groups 1, 6, or 7 in
`docs/launch-candidate-checklist.md`.

## Run

| Field | Value |
|---|---|
| Commit tested | `80662dd8221017a451934f5497428dca401773ef` |
| Transport | `streamable_http` |
| Auth | `static_bearer` |
| Policy | `standard` |
| Client | Claude Code / curl |
| Workspace | `65b382b606de527a7ee2b60e` |
| User | `<REDACTED>` |
| Evidence dir | `/tmp/clockify-mcp-standard-deep-1778163965` |

## Validated

- health endpoint
- initialize/session/initialized notification
- static bearer auth
- negative auth/session/protocol errors
- `clockify_policy_info` in `standard`
- cleanup of old leftovers
- full time-entry lifecycle create/read/update/find-update/delete
- workspace-wide dry-run previews
- all 11 Tier 2 groups activated/deactivated
- Tier 2 read/list/report surfaces sampled
- `resources/templates/list`, `resources/list`, and `resources/read`
- `prompts/list` and `prompts/get`
- final cleanup verified: 0 test entries remain

## Result

Functional dogfood result: PASS.

Activation/deactivation evidence JSON: P1 confirmed in the saved
evidence files. The exact verification command reported invalid JSON for
every `p7-activate-*.json` and `p7-deactivate-*.json` file because
quoted identifiers in activation/deactivation prose created fragile
nested escaping in `content[0].text`. The durable structured fields
already expose the activated/deactivated group and tool names, so the
fix removes quoted identifiers from the human-readable messages.

## Follow-Up State

- Activation/deactivation response escaping: fixed after this run.
- `time_tracking_safe` cleanup: documented in
  `docs/internal-test-posture.md`; safe mode may create time entries but
  still blocks `clockify_delete_entry`, so tests need temporary
  `standard` cleanup or UI cleanup.
- Clockify API key rotation: still required after exposing the test key.
  Rotation must be performed from an authenticated Clockify Profile
  Settings session; do not paste the key into docs, issues, or chat.

## Launch-Candidate Boundary

This evidence is useful internal confidence only. The launch candidate
still requires:

- two consecutive scheduled `live-contract.yml` cron greens on the
  candidate SHA;
- candidate-tag security walk-through evidence;
- release/sigstore/SLSA evidence on the candidate tag.
