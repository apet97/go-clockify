# 0012 - Backward-compatibility policy

## Status

Accepted — this ADR declares the back-compat surface; the
release gate (`docs/release-policy.md`) enforces what the gate
can check automatically.

## Context

Consumers of `clockify-mcp` make four kinds of long-lived
commitments against the server:

1. **MCP protocol version.** A client negotiates an MCP
   protocol version on `initialize`. Dropping a protocol
   version the client speaks causes an immediate disconnect,
   not a graceful downgrade.
2. **Tool names.** Client automations reference tool names
   verbatim (`clockify_start_timer`, `clockify_add_entry`).
   Renaming a tool silently breaks every pinned reference.
3. **Policy modes.** Operators configure `CLOCKIFY_POLICY`
   (`read_only`, `safe_core`, `standard`, `full`) and expect
   the allowlist semantics to remain stable. Narrowing what
   `safe_core` permits, or broadening `read_only`, breaks the
   contract operators made when they chose a mode.
4. **Config env vars.** Deployment tooling (Helm, Kustomize,
   systemd units) encodes env-var names. Removing or renaming
   a var without a transition period is an operator-facing
   break.

Without an explicit policy, every minor-version bump is a
coin flip for downstream consumers. This ADR writes down the
rules that already govern day-to-day changes and makes them
enforceable by reviewers.

Two alternatives were considered:

- **SemVer alone, no per-surface policy.** Leaves "what
  constitutes a breaking change" up to the reviewer. Rejected
  — the four surfaces above have different natural change
  cadences and warrant different deprecation windows.
- **Formal deprecation registry with compile-time guards.**
  Every deprecated symbol goes into a package-level list and
  CI fails if the list shrinks without a major-version bump.
  Interesting but premature — we've had zero documented
  deprecations across Wave 1 + 2. Write the policy first, add
  enforcement if the volume of deprecations justifies it.

## Decision

### MCP protocol versions

- **Committed window:** the last three published protocol
  versions as listed in `internal/mcp/server.go:SupportedProtocolVersions`
  (today: `2025-06-18`, `2025-03-26`, `2024-11-05`).
- **Dropping a version is a major-version bump** of
  `clockify-mcp`. The release notes must list the dropped
  version and name the earliest client version that still
  works.
- **Adding a version is a minor bump.** New protocol
  negotiation becomes available to clients that speak it;
  older clients continue to work unchanged.

### Tool names

- **Stable from first release.** `clockify_start_timer` cannot
  become `clockify_start_timing` without a major-version bump.
- **Deprecation flow for a rename or removal:**
  1. Minor release N: both old and new name are registered;
     the old name's `Description` starts with `DEPRECATED:
     use <new-name>`; a runbook row is added to
     `docs/release-policy.md` listing the sunset version.
  2. Minor release N+1 or later: the old name is removed at
     the next **major** bump, not silently in a minor.
- **Destructive hint changes** (a tool flipping from
  `ReadOnlyHint: true` to `false`, or gaining
  `DestructiveHint`) are major-version changes because they
  change how clients classify risk.

### Policy modes

- **Mode names are stable.** `safe_core` cannot be renamed to
  `guarded` without a major bump.
- **Allowlists are additive only in minor releases.** A tool
  newly added to `safe_core` is a minor bump (more permissive,
  existing call sites unaffected). Removing a tool from
  `safe_core` is a **major** bump — it breaks existing
  operators' expectations.
- **`read_only` is write-hermetic in perpetuity.** No
  combination of settings can cause `read_only` to permit a
  mutating call. Operators treat this as a compliance
  guarantee; weakening it has no backward-compatible path.
- **`full` is the unrestricted escape hatch.** Its semantics
  never narrow — anything the server can do is callable in
  `full`.

### Config env vars

- **Removal is a major-version bump.**
- **Rename flow (minor bump):**
  1. Minor release N: both names are honoured. Startup logs
     `config_env_rename` at WARN when the old name is
     observed, pointing at the new one. The old name is added
     to `deploy/.config-parity-opt-out.txt` so the parity
     check stops requiring it in deployment manifests.
  2. Minor release N+K (K ≥ 1): the old name is removed only
     at the next major bump.
- **Default changes are called out in the release notes.** A
  default flip (e.g. `MCP_AUDIT_DURABILITY=best_effort` →
  `fail_closed`) is not a breaking change in the strict
  sense, but it alters semantics and must be documented.

### What is *not* covered

- **Output payload shape.** Tool responses use a stable
  envelope (`structuredContent + content`), but the inner
  fields track Clockify's API responses and can change when
  the upstream changes. Downstream consumers should treat
  unknown fields as forward-compatible.
- **Internal packages (`internal/**`).** No public commitment
  — these can change freely at any release.
- **Metrics names.** Tracked separately in
  `internal/metrics/metrics.go`; adds are free, removes are
  minor bumps with release-note entries, rename is rare.

## Consequences

- Release reviewers have a named checklist to work against for
  every PR: did this change a tool name? did it narrow a
  policy mode? did it drop a protocol version? If any answer
  is yes without a major-version bump, the PR goes back for
  either revert or deprecation-window splitting.
- Downstream consumers can pin to a minor version confident
  that tool names, policy modes, and config keys survive.
- The deprecation flow is spelled out so a reviewer can reach
  for it without negotiating the shape on every case.
- The `docs/upgrade-checklist.md` pre-flight inherits this
  ADR's commitments: env-var diff, policy mode diff, and
  protocol-version drift are the three checks operators run.

## See also

- `docs/release-policy.md` — the semver cadence and the EOL
  window for the current-minor security-fix channel.
- `docs/support-matrix.md` — which combinations are
  Recommended / Tolerated / Unsupported today.
- `deploy/.config-parity-opt-out.txt` — the list of env vars
  deliberately excluded from the Helm/Kustomize parity check
  (includes deprecated names in their transition window).
- `internal/mcp/server.go` — `SupportedProtocolVersions`.
- `internal/policy/policy.go` — the canonical allowlist per
  policy mode.
- ADR 0004 (policy enforcement architecture) — the structural
  rationale for the four policy modes whose names this ADR
  pins.
