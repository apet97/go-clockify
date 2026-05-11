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
	//
	// FromEnv only parses and validates the enum here. The
	// "process mode broader than ceiling" cross-check lives in
	// ValidateForTransport so transports that do not consume
	// control-plane tenant records (stdio, grpc, legacy http) are
	// not penalised for inheriting the env var from a misconfigured
	// shell / container env.
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
//
// Ceiling fields (ADR 0021):
//
//   - configured_ceiling: literal MCP_TENANT_POLICY_CEILING value, or
//     "" if unset.
//   - effective_ceiling: the actual per-tenant cap the runtime
//     enforces — `min(processMode, configured_ceiling)` per
//     EffectiveCeiling. When configured_ceiling is empty, the
//     process mode acts as the implicit ceiling so this equals the
//     process mode. When configured_ceiling is broader than the
//     process mode (a legitimate "I want headroom in this profile
//     but the process is narrower today" shape), the process mode
//     is what gets reported because EffectiveTenantMode caps tenants
//     at min(processMode, ceiling) — see PR #99 review final
//     diagnostic fix.
//   - ceiling_source: "explicit" when configured_ceiling is set;
//     "implicit_process_mode" when the process mode is acting as
//     the ceiling.
//
// The deprecated single "ceiling" key is removed — it conflated
// configured-vs-effective and silently hid the implicit-ceiling
// case (PR #99 review).
func (p *Policy) Describe() map[string]any {
	configured := string(p.Ceiling)
	effective := string(EffectiveCeiling(p.Mode, p.Ceiling))
	source := "explicit"
	if p.Ceiling == "" {
		source = "implicit_process_mode"
	}
	m := map[string]any{
		"mode":                        string(p.Mode),
		"configured_ceiling":          configured,
		"effective_ceiling":           effective,
		"ceiling_source":              source,
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
//   - processMode must be a known mode (read_only / time_tracking_safe /
//     safe_core / standard / full). Unknown processMode fails closed.
//   - When ceiling is set, ceiling must be a known mode AND processMode
//     must satisfy processMode <= ceiling. A processMode broader than
//     the explicit ceiling is a configuration error and fails closed
//     before the tenant override is considered. This catches the
//     hosted-profile misconfiguration where an operator overrode
//     CLOCKIFY_POLICY=standard while the profile pinned
//     MCP_TENANT_POLICY_CEILING=time_tracking_safe.
//   - Empty tenantMode inherits processMode (subject to the
//     processMode<=ceiling invariant above).
//   - Unknown tenantMode fails closed.
//   - After the processMode<=ceiling invariant, the effective ceiling
//     for the tenant override is processMode itself (it is the tighter
//     of the two). tenantMode > processMode fails closed with an
//     explicit error rather than silent clamp.
func EffectiveTenantMode(processMode, tenantMode, ceiling Mode) (Mode, error) {
	if Rank(processMode) < 0 {
		return "", fmt.Errorf("invalid process mode %q", string(processMode))
	}
	if ceiling != "" {
		if Rank(ceiling) < 0 {
			return "", fmt.Errorf("invalid ceiling %q", string(ceiling))
		}
		if Rank(processMode) > Rank(ceiling) {
			return "", fmt.Errorf("process mode %q exceeds ceiling %q", string(processMode), string(ceiling))
		}
	}
	if tenantMode == "" {
		return processMode, nil
	}
	if Rank(tenantMode) < 0 {
		return "", fmt.Errorf("invalid tenant policyMode %q", string(tenantMode))
	}
	if Rank(tenantMode) > Rank(processMode) {
		return "", fmt.Errorf("tenant policyMode %q exceeds ceiling %q", string(tenantMode), string(processMode))
	}
	return tenantMode, nil
}

// EffectiveCeiling returns the actual per-tenant ceiling the runtime
// enforces, which is the narrower of the process mode and the
// configured ceiling (when both are known). EffectiveTenantMode caps
// tenant overrides at this value, so it is what operators should see
// in clockify_policy_info's effective_ceiling field.
//
// Behaviour:
//
//   - Unknown mode → "" (defensive; signals "no usable cap").
//   - Empty ceiling → mode (implicit ceiling = process mode).
//   - Unknown ceiling → "" (defensive).
//   - Otherwise → min(mode, ceiling) by Rank.
//
// PR #99 review final diagnostic fix: previously Describe reported
// the configured ceiling verbatim, which hid the case
// CLOCKIFY_POLICY=time_tracking_safe + MCP_TENANT_POLICY_CEILING=
// standard (live cap = time_tracking_safe, not standard).
func EffectiveCeiling(mode, ceiling Mode) Mode {
	if Rank(mode) < 0 {
		return ""
	}
	if ceiling == "" {
		return mode
	}
	if Rank(ceiling) < 0 {
		return ""
	}
	if Rank(mode) < Rank(ceiling) {
		return mode
	}
	return ceiling
}

// ValidateForTransport returns an error when the policy's process
// mode exceeds the explicit ceiling under a transport that consumes
// control-plane tenant records (currently only streamable_http).
//
// The MCP_TENANT_POLICY_CEILING env var is documented as
// streamable-HTTP only (internal/config/spec.go AppliesTo). Transports
// that do not consume control-plane TenantRecord overrides — stdio,
// legacy http, grpc — would treat the ceiling as a no-op at runtime,
// so failing startup for "policy > ceiling" on those transports would
// reject a configuration that has no actual effect. ValidateForTransport
// scopes the cross-check to where it is load-bearing.
//
// EffectiveTenantMode / tenantpolicy.Derive keep their independent
// defense-in-depth checks for actual per-session derivation. See ADR
// 0021.
func ValidateForTransport(p *Policy, transport string) error {
	if p == nil || p.Ceiling == "" || transport != "streamable_http" {
		return nil
	}
	if Rank(p.Mode) > Rank(p.Ceiling) {
		return fmt.Errorf("CLOCKIFY_POLICY %q exceeds MCP_TENANT_POLICY_CEILING %q; lower one to match", string(p.Mode), string(p.Ceiling))
	}
	return nil
}

// IsGroupBlockingMode reports whether the given mode nullifies
// AllowedGroups (i.e. IsGroupAllowed returns false before consulting
// the allowlist). tenantRuntime uses this to decide whether to honour
// or silently drop tenant AllowGroups. Unknown / empty modes fail
// closed and are treated as blocking.
func IsGroupBlockingMode(m Mode) bool {
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
