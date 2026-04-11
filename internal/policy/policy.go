package policy

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

type Mode string

const (
	ReadOnly Mode = "read_only"
	SafeCore Mode = "safe_core"
	Standard Mode = "standard"
	Full     Mode = "full"
)

type Policy struct {
	Mode           Mode
	DeniedTools    map[string]bool
	DeniedGroups   map[string]bool
	AllowedGroups  map[string]bool // nil = not set (all allowed per mode)
	Tier1ToolNames map[string]bool // populated after registry construction
}

func (p *Policy) Clone() *Policy {
	if p == nil {
		return nil
	}
	return &Policy{
		Mode:           p.Mode,
		DeniedTools:    cloneBoolMap(p.DeniedTools),
		DeniedGroups:   cloneBoolMap(p.DeniedGroups),
		AllowedGroups:  cloneBoolMap(p.AllowedGroups),
		Tier1ToolNames: cloneBoolMap(p.Tier1ToolNames),
	}
}

func FromEnv() (*Policy, error) {
	mode := Mode(strings.TrimSpace(strings.ToLower(os.Getenv("CLOCKIFY_POLICY"))))
	if mode == "" {
		mode = Standard
	}
	switch mode {
	case ReadOnly, SafeCore, Standard, Full:
	default:
		return nil, fmt.Errorf("invalid CLOCKIFY_POLICY: %s", mode)
	}

	denied := map[string]bool{}
	for _, item := range strings.Split(os.Getenv("CLOCKIFY_DENY_TOOLS"), ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			denied[item] = true
		}
	}

	deniedGroups := map[string]bool{}
	for _, item := range strings.Split(os.Getenv("CLOCKIFY_DENY_GROUPS"), ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			deniedGroups[item] = true
		}
	}

	var allowedGroups map[string]bool
	if raw := os.Getenv("CLOCKIFY_ALLOW_GROUPS"); raw != "" {
		allowedGroups = map[string]bool{}
		for _, item := range strings.Split(raw, ",") {
			item = strings.TrimSpace(item)
			if item != "" {
				allowedGroups[item] = true
			}
		}
	}

	return &Policy{
		Mode:          mode,
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
	if p.Mode == ReadOnly || p.Mode == SafeCore {
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
	if p.Mode == SafeCore && !readOnly && !isSafeCoreWrite(name) {
		return fmt.Sprintf("policy is safe_core; '%s' is not in the safe write list", name)
	}
	return fmt.Sprintf("tool '%s' is blocked by policy mode '%s'", name, string(p.Mode))
}

// Describe returns a map describing the current policy configuration.
func (p *Policy) Describe() map[string]any {
	m := map[string]any{
		"mode":                string(p.Mode),
		"denied_tools":        sortedKeys(p.DeniedTools),
		"denied_groups":       sortedKeys(p.DeniedGroups),
		"allowed_groups":      nil,
		"introspection_tools": introspectionList(),
		"safe_core_writes":    safeCoreWriteList(),
	}
	if p.AllowedGroups != nil {
		m["allowed_groups"] = sortedKeys(p.AllowedGroups)
	}
	return m
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
		"clockify_current_user",
		"clockify_list_workspaces",
		"clockify_policy_info",
		"clockify_resolve_debug",
		"clockify_search_tools",
		"clockify_whoami",
	}
}

func safeCoreWriteList() []string {
	return []string{
		"clockify_add_entry",
		"clockify_create_client",
		"clockify_create_project",
		"clockify_create_tag",
		"clockify_create_task",
		"clockify_find_and_update_entry",
		"clockify_log_time",
		"clockify_start_timer",
		"clockify_stop_timer",
		"clockify_switch_project",
		"clockify_update_entry",
	}
}

func cloneBoolMap(in map[string]bool) map[string]bool {
	if in == nil {
		return nil
	}
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func isIntrospection(name string) bool {
	switch name {
	case "clockify_whoami", "clockify_current_user", "clockify_list_workspaces",
		"clockify_search_tools", "clockify_policy_info", "clockify_resolve_debug":
		return true
	}
	return false
}

func isSafeCoreWrite(name string) bool {
	switch name {
	case "clockify_start_timer", "clockify_stop_timer",
		"clockify_add_entry", "clockify_update_entry",
		"clockify_log_time", "clockify_switch_project",
		"clockify_find_and_update_entry",
		"clockify_create_project", "clockify_create_client",
		"clockify_create_tag", "clockify_create_task":
		return true
	}
	return false
}
