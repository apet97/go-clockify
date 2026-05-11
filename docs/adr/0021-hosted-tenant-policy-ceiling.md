# 0021 - Hosted tenant policy ceiling

## Status

Accepted — 2026-05-11.

Implemented on the `hosted-tenant-policy-ceiling` branch:

- Policy package gains `Rank`, `IsAtMost`, `EffectiveTenantMode`,
  `isGroupBlockingMode`, `Policy.Ceiling`, `Policy.TenantAllowGroupsIgnored`.
- `internal/runtime/service.go::tenantRuntime` calls
  `EffectiveTenantMode` and switches tenant deny/allow merge semantics
  from *replace* to **union / intersect**.
- `MCP_TENANT_POLICY_CEILING` env var added to `internal/config/spec.go`.
- Hosted profiles (`shared-service`, `prod-postgres`) default the
  ceiling to `time_tracking_safe`. Other profiles leave it unset
  (process mode acts as the implicit ceiling).
- `clockify-mcp doctor` displays the effective ceiling and whether it
  is explicit (env / profile) or implicit (= process mode).
- `internal/controlplane/postgres/e2e_shared_service_test.go` tenant
  A seed updated from `Standard` to `time_tracking_safe` to remain
  consistent with the new hosted default.

## Context

`tenantRuntime` (`internal/runtime/service.go:284-360`) clones the
process policy and then **unconditionally overwrites** the policy
mode from the control-plane `TenantRecord.PolicyMode` field, with no
validation against the process / profile posture. In hosted
multi-tenant deployments — the `shared-service` and `prod-postgres`
profiles — a corrupted or overly broad tenant record can therefore
broaden the live posture from the profile-pinned `time_tracking_safe`
up to `safe_core`, `standard`, or `full`, silently re-opening
destructive writes, Tier-2 admin surface, and group elevation that
the hosted profile is explicitly designed to suppress.

The previous waves closed adjacent risks:

- `safe_core` no longer permits destructive deletes
  (`docs/policy/production-tool-scope.md`,
  `TestSafeCoreBlocksDestructiveDeletes`).
- Personal time-entry mutations fail closed when ownership cannot be
  proven (`internal/tools/users.go::requireCurrentUserEntry`).
- `RiskClass` plumbed through `ToolHints` (ADR 0018 prerequisite).
- ADR 0018 HMAC confirmation tokens implemented for high-risk
  classes.

But none of those gates protect against the control plane *itself*
being the broadening surface. A hosted profile that fail-closes on
every other input still trusts whatever `PolicyMode` the Postgres row
carries.

Tenant deny lists carry the inverse weakness. Today the merge
semantics are **replace**, not **union**: a tenant record with any
`DenyTools` entry erases every process-level deny. A reasonable
operator who configured a process-level `CLOCKIFY_DENY_TOOLS`
expecting it to be a floor for every tenant gets the opposite —
the floor disappears the moment any tenant adds a single deny.
The same shape applies to `DenyGroups` and to `AllowGroups`.

## Decision

Introduce a **tenant policy ceiling**: a per-process maximum policy
posture that tenant records may narrow but cannot exceed. Bundle the
*replace → narrow* fix for tenant deny / allow lists into the same
ADR, since they're the same narrowing-semantics decision applied at
three places in the same code path.

### Total ordering of policy modes

`policy.Rank(Mode) int` defines an explicit total ordering:

| Mode | Rank |
|---|---|
| `read_only` | 0 |
| `time_tracking_safe` | 1 |
| `safe_core` | 2 |
| `standard` | 3 |
| `full` | 4 |

Unknown modes return `-1` and fail closed at every comparison.

**Standard < Full despite current `IsAllowed` equivalence.**
`IsAllowed` (`internal/policy/policy.go:104-122`) treats `standard`
and `full` identically. Conflating them in `Rank` would couple the
ceiling contract to that accident: a future feature that distinguishes
the two — e.g. unlocking workspace-admin tools only under `full` —
would silently widen any deployment whose ceiling is pinned at
`standard`. Rank reflects *posture breadth*, not current `IsAllowed`
equivalence. A test pins this divergence
(`TestRank_FullStrictlyAboveStandard`).

### Implicit and explicit ceilings

`policy.EffectiveTenantMode(processMode, tenantMode, ceiling)` returns
the effective mode for a session. Invariants:

- Empty `tenantMode` → inherit `processMode`.
- Unknown `tenantMode` → error (fail closed).
- Effective ceiling is `min(processMode, ceiling-if-set)`. Even when
  no explicit ceiling is configured, the process mode is itself an
  implicit ceiling: a hosted operator who forgets to set
  `MCP_TENANT_POLICY_CEILING` still cannot have tenants broaden past
  the process posture.
- `tenantMode > effective ceiling` → error (fail closed). Prefer
  explicit failure over silent clamp so the operator sees the
  misconfiguration immediately.

### `Policy.Ceiling` field

```go
type Policy struct {
    Mode    Mode
    Ceiling Mode // NEW
    ...
}
```

`Clone()` copies the field. `FromEnv()` reads
`MCP_TENANT_POLICY_CEILING` (lowercase / trimmed like
`CLOCKIFY_POLICY`), validates against the enum, and assigns. Empty
is allowed and means "implicit ceiling = process mode" via the
helper.

`Describe()` exposes:

- `ceiling`: the configured ceiling string (or `""`).
- `tenant_allow_groups_ignored`: per-tenant boolean indicating
  whether `tenant.AllowGroups` was silently dropped because the
  effective mode blocks groups (see below).

### Env var and profile defaults

`MCP_TENANT_POLICY_CEILING` is added to `internal/config/spec.go`
under the `Safety` group, with enum `{"", "read_only",
"time_tracking_safe", "safe_core", "standard", "full"}` and empty
default.

Profile defaults — **hosted profiles only**:

| Profile | Default `MCP_TENANT_POLICY_CEILING` |
|---|---|
| `shared-service` | `time_tracking_safe` |
| `prod-postgres` | `time_tracking_safe` |
| `single-tenant-http` | *(unset)* |
| `local-stdio` | *(unset)* |
| `private-network-grpc` | *(unset)* |

`single-tenant-http` is deliberately left unset. The profile
auto-registers one tenant with `PolicyMode = depsafePolicyMode()` —
i.e. whatever the operator put in `CLOCKIFY_POLICY`. Hard-defaulting
to `time_tracking_safe` would break any single-tenant operator who
legitimately chose `safe_core` or `standard`. The implicit
"process mode as ceiling" still applies and is sufficient.

`local-stdio` has no tenants. `private-network-grpc` callers are
trusted infrastructure already gated by mTLS — the operator owns
posture decisions; an explicit `MCP_TENANT_POLICY_CEILING` is the
escape hatch when they want one.

### `tenantRuntime` enforcement

`internal/runtime/service.go:327-348` is rewritten:

1. Compute `effectiveMode` via `EffectiveTenantMode`. Fail closed
   on broadening.
2. `pol.Mode = effectiveMode`.
3. Tenant `DenyTools` and `DenyGroups` **union** with the process
   lists (do not replace).
4. Tenant `AllowGroups`:
   - Under a group-blocking mode (`read_only`, `time_tracking_safe`,
     `safe_core`): silently dropped. `pol.TenantAllowGroupsIgnored`
     is set so `clockify_policy_info` surfaces the diagnostic.
   - Under `standard` / `full`: **intersect** with the process
     `AllowedGroups` when both are set; when the process did not
     set an allowlist, the tenant list defines the whitelist.

### Doctor surface

`cmd/clockify-mcp/doctor.go` adds a line to the policy section
showing the effective ceiling and whether it is explicit (env /
profile) or implicit (= process mode). This is the single best
place for operators to detect misconfiguration before it hits a
real tenant.

### E2E seed update

`internal/controlplane/postgres/e2e_shared_service_test.go` seeds
tenant A with `Standard` (line 473). Under the new
`shared-service` ceiling default (`time_tracking_safe`), tenant A
would be rejected at session creation. The seed is updated to
`time_tracking_safe` to remain consistent with the new hosted
default. The `Standard`-versus-group-blocking nuance previously
exercised by tenant A is fully covered by new unit tests using the
`tenantRuntimeStore` fixture
(`internal/runtime/service_test.go:197-220`).

## Alternatives considered

**Silent clamp** (cap tenant mode to ceiling, no error). Rejected
because it hides operator misconfiguration. The hosted operator who
mis-configured a tenant record needs to *see* the rejection at
session-create time, not discover months later that their
`standard`-row was being silently served as `time_tracking_safe`.

**Fail-closed on `AllowGroups` under a group-blocking mode.**
Rejected because legacy tenants today harmlessly carry `AllowGroups`
entries that the mode-level block (line 129) already nullifies.
Failing every session for a harmless misconfiguration is a
production-incident shape. Skip-and-mark is the safer migration.
Reconsider after a deprecation window when operators have had time
to remove the stale entries.

**Equal rank for `standard` and `full`.** Rejected — see the rank
table rationale above. The cost of differentiating them is zero
(`IsAllowed` semantics are unchanged), and the upside is preserving
the ceiling contract through any future divergence of the two modes.

**Defer the deny/allow narrowing change to a separate ADR.**
Rejected because deny-replace and policy-broaden are the same
*"tenant can elevate"* shape. Splitting them into two ADRs landed
weeks apart leaves a hosted gap (one of the two surfaces broadens
while the other is fixed). Bundling here is the simpler safety
story and atomic-PR review remains tractable at this size.

**Default `single-tenant-http` to `time_tracking_safe`.** Rejected.
The profile already pins `CLOCKIFY_POLICY=time_tracking_safe` as
its own default; an operator who overrides that to `safe_core` or
`standard` is making a deliberate choice and should not be blocked
by a separate ceiling default applying to the auto-registered
tenant. The implicit ceiling (process mode) is the correct floor
here.

## Consequences

**Positive.**

- Hosted multi-tenant deployments gain a structural ceiling.
  A corrupted Postgres tenant row can no longer broaden the live
  posture beyond the profile-pinned default.
- Process-level denies are now load-bearing: tenant records can add
  to them but cannot erase them.
- Process-level allowlists are now load-bearing in the same way:
  tenant `AllowGroups` can narrow further but cannot widen.
- `clockify_policy_info` exposes the effective ceiling and the
  tenant-allow-groups-ignored diagnostic, so operators can verify
  the live posture without reading audit logs.
- `clockify-mcp doctor` surfaces the ceiling in its policy section.

**Negative — accept these.**

- Existing hosted operators who today rely on broadening policy via
  control-plane tenant records will see session-creation errors
  after upgrading. This is by design and is the entire point of the
  change. Document in the changelog and the runbook.
- Operators who relied on tenant `DenyTools` *replacing* a deliberately
  empty process deny list (i.e., "the tenant deny list is the floor")
  do not see a behaviour change — the process list was already empty.
  Operators who relied on tenant `DenyTools` *replacing* a non-empty
  process deny list (i.e., "this tenant gets only these denies")
  will see the union take effect. This shape is rare and is the
  intended fail-safe direction.
- The shared-service E2E seed for tenant A moves from `Standard` to
  `time_tracking_safe`. Tenant A's specific group-blocking expectation
  is preserved by the new unit tests; the E2E continues to pin the
  hosted multi-tenant contract.

## Migration

Hosted operators who today carry tenant records with
`PolicyMode > time_tracking_safe` have two options:

1. Lower the tenant `PolicyMode` to `time_tracking_safe` (or
   `read_only`). The recommended path.
2. Override `MCP_TENANT_POLICY_CEILING` explicitly to `safe_core`,
   `standard`, or `full` for the process. Documented in
   `docs/policy/production-tool-scope.md`. Operators who do this
   are making a deliberate choice to expose more surface than the
   hosted default.

## References

- `internal/policy/policy.go` — `Mode` constants (lines 13-19),
  `Policy` struct (21-27), `IsAllowed` (104-122),
  `IsGroupAllowed` (125-139). New helpers and field land here.
- `internal/runtime/service.go:284-360` — `tenantRuntime`. The
  rewrite target.
- `internal/controlplane/store.go:15-26` — `TenantRecord` shape.
  Unchanged by this ADR; the ceiling lives on the process policy,
  not the tenant record.
- `internal/config/profile.go:40-149` — profile definitions.
  Hosted profiles gain the new default here.
- `internal/config/spec.go` — `EnvSpec` for
  `MCP_TENANT_POLICY_CEILING`.
- `internal/controlplane/postgres/e2e_shared_service_test.go:473` —
  tenant A seed; updated to `time_tracking_safe`.
- `internal/runtime/service_test.go:197-220` — `tenantRuntimeStore`
  fixture; reused by the new unit tests.
- ADR 0004 — Policy enforcement architecture. The ceiling slots in
  alongside the existing policy-mode gate.
- ADR 0015 — Profile-centric configuration. Hosted profiles are the
  natural home for the ceiling default.
- ADR 0018 — Risk-class enforcement and confirmation tokens. The
  ceiling is the upstream gate; confirmation tokens are the
  downstream gate for the high-risk classes that the ceiling
  allows through.
