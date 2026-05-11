// Package tenantpolicy derives a per-tenant *policy.Policy from a
// process policy and a control-plane TenantRecord under the ADR 0021
// hosted tenant policy ceiling contract.
//
// The package is the single canonical place to apply tenant policy
// overrides so the production runtime (internal/runtime) and the
// shared-service Postgres E2E test harness
// (internal/controlplane/postgres) both exercise the same code path.
// Without this extraction the E2E factory closure had to mirror the
// production logic by hand, and a previous regression let the closure
// silently bypass the ceiling — see ADR 0021 §"Scope note" and PR #99
// review.
package tenantpolicy

import (
	"github.com/apet97/go-clockify/internal/controlplane"
	"github.com/apet97/go-clockify/internal/policy"
)

// Derive returns a per-tenant clone of processPolicy with the
// TenantRecord's overrides applied under the strict-narrowing
// contract documented in ADR 0021:
//
//   - Mode is set via policy.EffectiveTenantMode so the tenant cannot
//     broaden past min(processMode, processPolicy.Ceiling). Process
//     mode broader than an explicit ceiling fails closed before the
//     tenant override is considered.
//   - DenyTools and DenyGroups UNION with the process lists. A tenant
//     record can only add denies; it cannot erase a process-level deny.
//   - AllowGroups under a group-blocking effective mode is silently
//     dropped and TenantAllowGroupsIgnored is set so policy_info
//     surfaces the diagnostic. Under a non-blocking mode, the tenant
//     list INTERSECTS with the process AllowedGroups when both are
//     set, or defines the whitelist when the process did not set one.
func Derive(processPolicy *policy.Policy, tenant controlplane.TenantRecord) (*policy.Policy, error) {
	pol := processPolicy.Clone()

	effectiveMode, err := policy.EffectiveTenantMode(
		pol.Mode,
		policy.Mode(tenant.PolicyMode),
		pol.Ceiling,
	)
	if err != nil {
		return nil, err
	}
	pol.Mode = effectiveMode

	for _, item := range tenant.DenyTools {
		if item == "" {
			continue
		}
		if pol.DeniedTools == nil {
			pol.DeniedTools = map[string]bool{}
		}
		pol.DeniedTools[item] = true
	}
	for _, item := range tenant.DenyGroups {
		if item == "" {
			continue
		}
		if pol.DeniedGroups == nil {
			pol.DeniedGroups = map[string]bool{}
		}
		pol.DeniedGroups[item] = true
	}

	if len(tenant.AllowGroups) > 0 {
		if policy.IsGroupBlockingMode(effectiveMode) {
			pol.TenantAllowGroupsIgnored = true
		} else {
			tenantAllow := map[string]bool{}
			for _, item := range tenant.AllowGroups {
				if item != "" {
					tenantAllow[item] = true
				}
			}
			if pol.AllowedGroups == nil {
				pol.AllowedGroups = tenantAllow
			} else {
				intersected := map[string]bool{}
				for k := range pol.AllowedGroups {
					if tenantAllow[k] {
						intersected[k] = true
					}
				}
				pol.AllowedGroups = intersected
			}
		}
	}

	return pol, nil
}
