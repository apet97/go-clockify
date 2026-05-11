package policy

import (
	"fmt"
	"maps"
	"os"
	"sort"
	"strings"
)

type Mode string

const (
	ReadOnly         Mode = "read_only"
	TimeTrackingSafe Mode = "time_tracking_safe"
	SafeCore         Mode = "safe_core"
	Standard         Mode = "standard"
	Full             Mode = "full"
)

type Policy struct {
	Mode Mode
	// Ceiling is the maximum mode a control-plane tenant record may
	// select via TenantRecord.PolicyMode. Empty = implicit ceiling
	// (= process Mode itself; see EffectiveTenantMode). See ADR 0021.
	Ceiling        Mode
	DeniedTools    map[string]bool
	DeniedGroups   map[string]bool
	AllowedGroups  map[string]bool // nil = not set (all allowed per mode)
	Tier1ToolNames map[string]bool // populated after registry construction
	// TenantAllowGroupsIgnored is set on per-tenant clones when the
	// tenant carried AllowGroups under a group-blocking effective mode
	// (read_only / time_tracking_safe / safe_core) and the list was
	// silently dropped. Surfaced via Describe() for clockify_policy_info.
	TenantAllowGroupsIgnored bool
}

func (p *Policy) Clone() *Policy {
	if p == nil {
		return nil
	}
	return &Policy{
		Mode:           p.Mode,
		Ceiling:        p.Ceiling,
		DeniedTools:    cloneBoolMap(p.DeniedTools),
		DeniedGroups:   cloneBoolMap(p.DeniedGroups),
		AllowedGroups:  cloneBoolMap(p.AllowedGroups),
		Tier1ToolNames: cloneBoolMap(p.Tier1ToolNames),
		// TenantAllowGroupsIgnored is a per-clone marker; cleared on
		// Clone so a fresh tenant runtime starts from a clean slate.
	}
}

func FromEnv() (*Policy, error) {
	mode := Mode(strings.TrimSpace(strings.ToLower(os.Getenv("CLOCKIFY_POLICY"))))
	if mode == "" {
		mode = Standard
	}
	switch mode {
	case ReadOnly, TimeTrackingSafe, SafeCore, Standard, Full:
	default:
		return nil, fmt.Errorf("invalid CLOCKIFY_POLICY: %s", mode)
	}

	// MCP_TENANT_POLICY_CEILING constrains the maximum mode a
	// control-plane tenant record may select. Empty = no explicit
	// constraint (process mode acts as the implicit ceiling via
	// EffectiveTenantMode). See ADR 0021.
	var ceiling Mode
	if raw := strings.TrimSpace(strings.ToLower(os.Getenv("MCP_TENANT_POLICY_CEILING"))); raw != "" {
		c := Mode(raw)
		switch c {
		case ReadOnly, TimeTrackingSafe, SafeCore, Standard, Full:
			ceiling = c
		default:
			return nil, fmt.Errorf("invalid MCP_TENANT_POLICY_CEILING: %s", raw)
		}
	}

	denied := map[string]bool{}
	for item := range strings.SplitSeq(os.Getenv("CLOCKIFY_DENY_TOOLS"), ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			denied[item] = true
		}
	}

	deniedGroups := map[string]bool{}
	for item := range strings.SplitSeq(os.Getenv("CLOCKIFY_DENY_GROUPS"), ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			deniedGroups[item] = true
		}
	}

	var allowedGroups map[string]bool
	if raw := os.Getenv("CLOCKIFY_ALLOW_GROUPS"); raw != "" {
		allowedGroups = map[string]bool{}
		for item := range strings.SplitSeq(raw, ",") {
			item = strings.TrimSpace(item)
			if item != "" {
				allowedGroups[item] = true
			}
		}
	}

	return &Policy{
		Mode:          mode,
		Ceiling:       ceiling,
		DeniedTools:   denied,
		DeniedGroups:  deniedGroups,
		AllowedGroups: allowedGroups,
	}, nil
}

// SetTier1Tools stores the set of Tier-1 tool names for later reference.
func (p *Policy) SetTier1Tools(names map[string]bool) {
	p.Tier1ToolNames = names
}

func (p *Policy) IsAllowed(name string, readOnly bool) bool {
	if p == nil {
		return true
	}
	if p.DeniedTools[name] {
		return false
	}
	if isIntrospection(name) {
		return true
	}

	switch p.Mode {
	case ReadOnly:
		return readOnly
	case TimeTrackingSafe:
		if readOnly {
			return true
		}
		return isTimeTrackingSafeWrite(name)
	case SafeCore:
		if readOnly {
			return true
		}
		return isSafeCoreWrite(name)
	case Standard, Full:
		return true
	default:
		return false
	}
}

// IsGroupAllowed reports whether tools in the given group are permitted.
func (p *Policy) IsGroupAllowed(group string) bool {
	if p == nil {
		return true
	}
	if p.Mode == ReadOnly || p.Mode == TimeTrackingSafe || p.Mode == SafeCore {
		return false
	}
	if p.DeniedGroups[group] {
		return false
	}
	if p.AllowedGroups != nil && !p.AllowedGroups[group] {
		return false
	}
	return true
}

// BlockReason returns a human-readable explanation for why a tool is blocked.
func (p *Policy) BlockReason(name string, readOnly bool) string {
	if p.DeniedTools[name] {
		return fmt.Sprintf("tool '%s' is explicitly denied", name)
	}
	if p.Mode == ReadOnly && !readOnly {
		return fmt.Sprintf("policy is read_only; '%s' is a write tool", name)
	}
	if p.Mode == TimeTrackingSafe && !readOnly && !isTimeTrackingSafeWrite(name) {
		return fmt.Sprintf("policy is time_tracking_safe; '%s' is not in the time-tracking write list", name)
	}
	if p.Mode == SafeCore && !readOnly && !isSafeCoreWrite(name) {
		return fmt.Sprintf("policy is safe_core; '%s' is not in the safe write list", name)
	}
	return fmt.Sprintf("tool '%s' is blocked by policy mode '%s'", name, string(p.Mode))
}

// Describe returns a map describing the current policy configuration.
func (p *Policy) Describe() map[string]any {
	m := map[string]any{
		"mode":                        string(p.Mode),
		"ceiling":                     string(p.Ceiling),
		"tenant_allow_groups_ignored": p.TenantAllowGroupsIgnored,
		"denied_tools":                sortedKeys(p.DeniedTools),
		"denied_groups":               sortedKeys(p.DeniedGroups),
		"allowed_groups":              nil,
		"introspection_tools":         introspectionList(),
		"safe_core_writes":            safeCoreWriteList(),
		"time_tracking_safe_writes":   timeTrackingSafeWriteList(),
	}
	if p.AllowedGroups != nil {
		m["allowed_groups"] = sortedKeys(p.AllowedGroups)
	}
	return m
}

// Rank reflects posture breadth, not current IsAllowed equivalence.
// The ceiling contract surface (see ADR 0021) lives on this total
// ordering; standard and full are deliberately split so future
// divergence between them cannot silently widen deployments whose
// ceiling is pinned at standard.
//
//	read_only         = 0  (narrowest)
//	time_tracking_safe = 1
//	safe_core         = 2
//	standard          = 3
//	full              = 4  (broadest)
//
// Unknown modes return -1 so every comparison against a real
// ceiling rejects them (fail closed).
func Rank(m Mode) int {
	switch m {
	case ReadOnly:
		return 0
	case TimeTrackingSafe:
		return 1
	case SafeCore:
		return 2
	case Standard:
		return 3
	case Full:
		return 4
	}
	return -1
}

// IsAtMost reports whether candidate is at most as broad as ceiling.
// An empty ceiling means "no explicit constraint" (the helper still
// rejects unknown candidates). EffectiveTenantMode layers the
// implicit "process mode as ceiling" semantics on top.
func IsAtMost(candidate, ceiling Mode) bool {
	cr := Rank(candidate)
	if cr < 0 {
		return false
	}
	if ceiling == "" {
		return true
	}
	return cr <= Rank(ceiling)
}

// EffectiveTenantMode returns the effective per-tenant policy mode
// given the process mode, the tenant's requested mode (may be empty
// to inherit), and an optional explicit ceiling from
// MCP_TENANT_POLICY_CEILING.
//
// Invariants (see ADR 0021):
//
//   - Empty tenantMode inherits processMode.
//   - Unknown tenantMode fails closed.
//   - Effective ceiling is min(processMode, ceiling-if-set). Process
//     mode is an implicit ceiling even when no explicit ceiling is
//     configured, so a hosted operator who forgets to set
//     MCP_TENANT_POLICY_CEILING still cannot have tenants broaden
//     past the process posture.
//   - tenantMode > effective ceiling fails closed with an explicit
//     error rather than silent clamp.
func EffectiveTenantMode(processMode, tenantMode, ceiling Mode) (Mode, error) {
	if tenantMode == "" {
		return processMode, nil
	}
	if Rank(tenantMode) < 0 {
		return "", fmt.Errorf("invalid tenant policyMode %q", string(tenantMode))
	}
	effectiveCeiling := processMode
	if ceiling != "" && Rank(ceiling) >= 0 && Rank(ceiling) < Rank(effectiveCeiling) {
		effectiveCeiling = ceiling
	}
	if Rank(tenantMode) > Rank(effectiveCeiling) {
		return "", fmt.Errorf("tenant policyMode %q exceeds ceiling %q", string(tenantMode), string(effectiveCeiling))
	}
	return tenantMode, nil
}

// isGroupBlockingMode reports whether the given mode nullifies
// AllowedGroups (i.e. IsGroupAllowed returns false before consulting
// the allowlist). tenantRuntime uses this to decide whether to honour
// or silently drop tenant AllowGroups. Unknown / empty modes fail
// closed and are treated as blocking.
func isGroupBlockingMode(m Mode) bool {
	switch m {
	case Standard, Full:
		return false
	case ReadOnly, TimeTrackingSafe, SafeCore:
		return true
	}
	return true
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func introspectionList() []string {
	return []string{
		"clockify_activate_group",
		"clockify_activate_tool",
		"clockify_current_user",
		"clockify_deactivate_group",
		"clockify_list_tools",
		"clockify_list_workspaces",
		"clockify_policy_info",
		"clockify_resolve_name",
		"clockify_resolve_debug",
		"clockify_search_tools",
		"clockify_whoami",
	}
}

// safeCoreWriteList enumerates the writes that `safe_core` permits.
// It deliberately excludes every delete_* tool: docs/policy/
// production-tool-scope.md states safe_core "still blocks all delete
// operations and Tier 2 admin surface", and the policy/test pin in
// TestSafeCoreBlocksDestructiveDeletes enforces that contract.
// Adding a destructive tool here without also flipping
// docs/policy/production-tool-scope.md and the contract pins is a
// regression of the documented safe_core posture.
func safeCoreWriteList() []string {
	return []string{
		"clockify_add_entry",
		"clockify_create_client",
		"clockify_create_project",
		"clockify_create_tag",
		"clockify_create_task",
		"clockify_find_and_update_entry",
		"clockify_update_client",
		"clockify_update_project",
		"clockify_update_tag",
		"clockify_update_task",
		"clockify_log_time",
		"clockify_start_timer",
		"clockify_stop_timer",
		"clockify_switch_project",
		"clockify_timesheet_fill_gap",
		"clockify_update_entry",
	}
}

// timeTrackingSafeWriteList is the narrow allowlist for the
// time_tracking_safe policy: own-entry mutations and timer control
// only. Crucially excludes project/client/tag/task creation — those
// are workspace-wide effects and belong in safe_core, not a
// time-tracking-agent default.
func timeTrackingSafeWriteList() []string {
	return []string{
		"clockify_add_entry",
		"clockify_find_and_update_entry",
		"clockify_log_time",
		"clockify_start_timer",
		"clockify_stop_timer",
		"clockify_switch_project",
		"clockify_timesheet_fill_gap",
		"clockify_update_entry",
	}
}

func cloneBoolMap(in map[string]bool) map[string]bool {
	if in == nil {
		return nil
	}
	out := make(map[string]bool, len(in))
	maps.Copy(out, in)
	return out
}

func isIntrospection(name string) bool {
	switch name {
	case "clockify_whoami", "clockify_current_user", "clockify_list_workspaces",
		"clockify_list_tools", "clockify_search_tools", "clockify_activate_group",
		"clockify_activate_tool", "clockify_deactivate_group",
		"clockify_policy_info", "clockify_resolve_name", "clockify_resolve_debug":
		return true
	}
	return false
}

// isSafeCoreWrite mirrors safeCoreWriteList; keep the two in lockstep.
// Delete tools are intentionally absent — see the comment on
// safeCoreWriteList.
func isSafeCoreWrite(name string) bool {
	switch name {
	case "clockify_start_timer", "clockify_stop_timer",
		"clockify_add_entry", "clockify_update_entry",
		"clockify_log_time", "clockify_switch_project",
		"clockify_find_and_update_entry", "clockify_timesheet_fill_gap",
		"clockify_create_project", "clockify_create_client",
		"clockify_create_tag", "clockify_create_task",
		"clockify_update_client", "clockify_update_project",
		"clockify_update_tag", "clockify_update_task":
		return true
	}
	return false
}

// isTimeTrackingSafeWrite is a strict subset of isSafeCoreWrite that
// omits workspace-wide create_* tools. time_tracking_safe is the
// recommended default for untrusted AI agents that should log time
// but not reshape the workspace.
func isTimeTrackingSafeWrite(name string) bool {
	switch name {
	case "clockify_start_timer", "clockify_stop_timer",
		"clockify_add_entry", "clockify_update_entry",
		"clockify_log_time", "clockify_switch_project",
		"clockify_find_and_update_entry", "clockify_timesheet_fill_gap":
		return true
	}
	return false
}
