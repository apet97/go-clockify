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
- `clockify-mcp doctor` surfaces the configured value of
  `MCP_TENANT_POLICY_CEILING` and its source (explicit / profile /
  default) through the standard env-var table that is built from
  `config.AllSpecs()`. The effective per-tenant ceiling (after
  `min(process, ceiling)` resolution) is exposed via the
  `clockify_policy_info` tool's `effective_ceiling` and
  `ceiling_source` fields rather than through a dedicated doctor
  display line.
- `internal/controlplane/postgres/e2e_shared_service_test.go` tenant
  A seed updated from `Standard` to `time_tracking_safe` to remain
  consistent with the new hosted default. The factory closure now
  derives its per-tenant policy via the shared
  `internal/tenantpolicy.Derive` helper so the E2E exercises the
  ceiling/union/intersect contract instead of bypassing it.

## Context

`tenantRuntime` (`internal/runtime/service.go`) clones the
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
`policy.go::IsAllowed` treats `standard` and `full` identically. Conflating them in `Rank` would couple the
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

`internal/runtime/service.go::tenantRuntime` is rewritten to route
through the shared `internal/tenantpolicy.Derive` helper:

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

### Operator visibility

The configured `MCP_TENANT_POLICY_CEILING` value (and whether it
came from an explicit env override, a profile default, or is unset)
appears in `clockify-mcp doctor`'s standard env-var table, which
is built from `config.AllSpecs()` and reports source attribution
for every spec'd variable. The per-tenant *effective* ceiling
(after `min(processMode, ceiling)` and the implicit-process-mode
fallback) is exposed through the `clockify_policy_info` tool as
the `configured_ceiling`, `effective_ceiling`, and
`ceiling_source` fields. Operators triaging a session-create
rejection ("tenant X: tenant policyMode 'Y' exceeds ceiling 'Z'")
can verify the live policy view by calling that tool — no
dedicated doctor-display line is needed.

### E2E seed update

`internal/controlplane/postgres/e2e_shared_service_test.go`
historically seeded tenant A with `Standard`. Under the new
`shared-service` ceiling default (`time_tracking_safe`), tenant A
would be rejected at session creation by the same
`tenantpolicy.Derive` path that guards production. Both seeds in
the shared-service E2E and the session-rehydration E2E
(`e2e_session_rehydration_test.go`) are now pinned to
`time_tracking_safe` so they remain consistent with the hosted
default. The `Standard`-versus-group-blocking nuance previously
exercised at the E2E layer is fully covered by the unit tests in
`internal/tenantpolicy` and the runtime-level cases that use the
`tenantRuntimeStore` fixture in `internal/runtime/service_test.go`.

## Alternatives considered

**Silent clamp** (cap tenant mode to ceiling, no error). Rejected
because it hides operator misconfiguration. The hosted operator who
mis-configured a tenant record needs to *see* the rejection at
session-create time, not discover months later that their
`standard`-row was being silently served as `time_tracking_safe`.

**Fail-closed on `AllowGroups` under a group-blocking mode.**
Rejected because legacy tenants today harmlessly carry `AllowGroups`
entries that `policy.go::IsGroupAllowed` already nullifies under
group-blocking modes.
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
- `clockify_policy_info` exposes the `configured_ceiling`,
  `effective_ceiling`, `ceiling_source`, and
  `tenant_allow_groups_ignored` diagnostics, so operators can
  verify the live posture without reading audit logs.
- `clockify-mcp doctor`'s standard env-var table reports the
  configured `MCP_TENANT_POLICY_CEILING` and its source (explicit
  / profile / default).

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

- `internal/policy/policy.go` — `Mode` constants and `Policy`
  struct, `IsAllowed`, `IsGroupAllowed`. New helpers (`Rank`,
  `IsAtMost`, `EffectiveTenantMode`, `IsGroupBlockingMode`,
  `ValidateForTransport`) and fields (`Policy.Ceiling`,
  `Policy.TenantAllowGroupsIgnored`) land here.
  `ValidateForTransport` is the streamable-HTTP-only startup gate
  for `processMode <= ceiling`; called from `runtime.New` and
  `cmd/clockify-mcp/doctor.go` so the binary refuses to boot and
  doctor reports a Load error when the pair is misaligned under
  streamable_http. Other transports skip the gate.
- `internal/tenantpolicy/tenantpolicy.go` — `Derive` is the
  shared per-tenant policy derivation entry point. Used by both
  `internal/runtime/service.go::tenantRuntime` and
  `internal/controlplane/postgres/e2e_shared_service_test.go::sharedSvcFactory`
  so the production runtime and the shared-service E2E exercise
  the same code path.
- `internal/runtime/service.go::tenantRuntime` — clones the
  process policy, runs `tenantpolicy.Derive`, builds the
  per-session server.
- `internal/controlplane/store.go` — `TenantRecord` shape.
  Unchanged by this ADR; the ceiling lives on the process policy,
  not the tenant record.
- `internal/config/profile.go` — profile definitions. Hosted
  profiles (`shared-service`, `prod-postgres`) gain the
  `MCP_TENANT_POLICY_CEILING=time_tracking_safe` default here.
- `internal/config/spec.go` — `EnvSpec` for
  `MCP_TENANT_POLICY_CEILING`.
- `internal/controlplane/postgres/e2e_shared_service_test.go` —
  shared-service E2E factory `sharedSvcFactory`; tenant seeds
  pinned to `time_tracking_safe` to match the new ceiling default.
- `internal/runtime/service_test.go::tenantRuntimeStore` —
  fixture reused by the runtime-level ceiling tests.
- ADR 0004 — Policy enforcement architecture. The ceiling slots in
  alongside the existing policy-mode gate.
- ADR 0015 — Profile-centric configuration. Hosted profiles are the
  natural home for the ceiling default.
- ADR 0018 — Risk-class enforcement and confirmation tokens. The
  ceiling is the upstream gate; confirmation tokens are the
  downstream gate for the high-risk classes that the ceiling
  allows through.
