package bootstrap

import (
	"fmt"
	"maps"
	"os"
	"strings"
)

// Mode controls which tools are visible to the LLM.
type Mode int

const (
	FullTier1 Mode = iota
	Minimal
	Custom
)

// Config holds the bootstrap configuration for tool visibility.
type Config struct {
	Mode        Mode
	CustomTools map[string]bool
	ActiveTools map[string]bool
	Tier1Names  map[string]bool // set post-registry via SetTier1Tools
}

func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}
	out := &Config{
		Mode:        c.Mode,
		CustomTools: cloneBoolMap(c.CustomTools),
		ActiveTools: cloneBoolMap(c.ActiveTools),
		Tier1Names:  cloneBoolMap(c.Tier1Names),
	}
	return out
}

// CatalogEntry describes a single tool for search/discovery.
type CatalogEntry struct {
	Name        string
	Description string
	Domain      string
	Keywords    []string
}

// AlwaysVisible lists introspection tools shown regardless of mode.
var AlwaysVisible = map[string]bool{
	"clockify_whoami":        true,
	"clockify_search_tools":  true,
	"clockify_policy_info":   true,
	"clockify_resolve_debug": true,
}

// MinimalSet lists the core tools exposed in Minimal mode.
var MinimalSet = map[string]bool{
	"clockify_whoami":        true,
	"clockify_search_tools":  true,
	"clockify_policy_info":   true,
	"clockify_resolve_debug": true,
	"clockify_start_timer":   true,
	"clockify_stop_timer":    true,
	"clockify_timer_status":  true,
	"clockify_list_entries":  true,
	"clockify_add_entry":     true,
	"clockify_list_projects": true,
	"clockify_log_time":      true,
}

// Tier1Catalog contains all 33 Tier 1 tools with searchable metadata.
var Tier1Catalog = []CatalogEntry{
	{Name: "clockify_whoami", Description: "Get current user and workspace context", Domain: "context", Keywords: []string{"identity", "user", "workspace", "session"}},
	{Name: "clockify_search_tools", Description: "Search and discover available tools", Domain: "context", Keywords: []string{"find", "discover", "activate", "tools"}},
	{Name: "clockify_policy_info", Description: "Display effective policy configuration", Domain: "context", Keywords: []string{"policy", "mode", "permissions", "safety"}},
	{Name: "clockify_resolve_debug", Description: "Debug name-to-ID resolution", Domain: "context", Keywords: []string{"resolve", "debug", "lookup", "name", "id"}},
	{Name: "clockify_list_workspaces", Description: "List available workspaces", Domain: "workspaces", Keywords: []string{"workspace", "list"}},
	{Name: "clockify_get_workspace", Description: "Get workspace details", Domain: "workspaces", Keywords: []string{"workspace", "details"}},
	{Name: "clockify_current_user", Description: "Get the current Clockify user", Domain: "users", Keywords: []string{"user", "profile", "me"}},
	{Name: "clockify_list_users", Description: "List workspace members", Domain: "users", Keywords: []string{"user", "members", "team"}},
	{Name: "clockify_start_timer", Description: "Start a new timer", Domain: "timer", Keywords: []string{"timer", "start", "track", "begin"}},
	{Name: "clockify_stop_timer", Description: "Stop the running timer", Domain: "timer", Keywords: []string{"timer", "stop", "end", "finish"}},
	{Name: "clockify_timer_status", Description: "Check if a timer is running", Domain: "timer", Keywords: []string{"timer", "status", "running", "active"}},
	{Name: "clockify_list_entries", Description: "List recent time entries", Domain: "entries", Keywords: []string{"entries", "time", "list", "history"}},
	{Name: "clockify_get_entry", Description: "Get a single time entry by ID", Domain: "entries", Keywords: []string{"entry", "get", "detail"}},
	{Name: "clockify_today_entries", Description: "List today's time entries", Domain: "entries", Keywords: []string{"today", "entries", "current"}},
	{Name: "clockify_add_entry", Description: "Create a new time entry", Domain: "entries", Keywords: []string{"entry", "create", "add", "log"}},
	{Name: "clockify_update_entry", Description: "Update an existing time entry", Domain: "entries", Keywords: []string{"entry", "update", "edit", "modify"}},
	{Name: "clockify_delete_entry", Description: "Delete a time entry", Domain: "entries", Keywords: []string{"entry", "delete", "remove"}},
	{Name: "clockify_list_projects", Description: "List workspace projects", Domain: "projects", Keywords: []string{"project", "list"}},
	{Name: "clockify_get_project", Description: "Get project details by name or ID", Domain: "projects", Keywords: []string{"project", "get", "detail"}},
	{Name: "clockify_create_project", Description: "Create a new project", Domain: "projects", Keywords: []string{"project", "create", "new"}},
	{Name: "clockify_list_clients", Description: "List workspace clients", Domain: "clients", Keywords: []string{"client", "list", "customer"}},
	{Name: "clockify_create_client", Description: "Create a new client", Domain: "clients", Keywords: []string{"client", "create", "new", "customer"}},
	{Name: "clockify_list_tags", Description: "List workspace tags", Domain: "tags", Keywords: []string{"tag", "label", "list"}},
	{Name: "clockify_create_tag", Description: "Create a new tag", Domain: "tags", Keywords: []string{"tag", "create", "new", "label"}},
	{Name: "clockify_list_tasks", Description: "List tasks for a project", Domain: "tasks", Keywords: []string{"task", "list", "todo"}},
	{Name: "clockify_create_task", Description: "Create a new task in a project", Domain: "tasks", Keywords: []string{"task", "create", "new", "todo"}},
	{Name: "clockify_summary_report", Description: "Summarize time entries by project for a date range", Domain: "reports", Keywords: []string{"report", "summary", "aggregate", "project"}},
	{Name: "clockify_detailed_report", Description: "Detailed time entry report with filtering", Domain: "reports", Keywords: []string{"report", "detailed", "entries", "export"}},
	{Name: "clockify_weekly_summary", Description: "Weekly summary grouped by day and project", Domain: "reports", Keywords: []string{"report", "weekly", "week", "summary"}},
	{Name: "clockify_quick_report", Description: "Quick high-signal summary for recent period", Domain: "reports", Keywords: []string{"report", "quick", "overview", "recent"}},
	{Name: "clockify_log_time", Description: "Create a finished time entry", Domain: "workflows", Keywords: []string{"log", "time", "entry", "create", "finished"}},
	{Name: "clockify_switch_project", Description: "Stop timer and start on different project", Domain: "workflows", Keywords: []string{"switch", "project", "timer", "change"}},
	{Name: "clockify_find_and_update_entry", Description: "Find and update a time entry by filters", Domain: "workflows", Keywords: []string{"find", "update", "search", "entry", "modify"}},
}

// ConfigFromEnv reads bootstrap configuration from environment variables.
//
// CLOCKIFY_BOOTSTRAP_MODE: "full_tier1" (default), "minimal", "custom" (case-insensitive)
// CLOCKIFY_BOOTSTRAP_TOOLS: comma-separated tool names (only used in custom mode)
func ConfigFromEnv() (Config, error) {
	modeStr := strings.ToLower(strings.TrimSpace(os.Getenv("CLOCKIFY_BOOTSTRAP_MODE")))
	if modeStr == "" {
		modeStr = "full_tier1"
	}

	var mode Mode
	switch modeStr {
	case "full_tier1":
		mode = FullTier1
	case "minimal":
		mode = Minimal
	case "custom":
		mode = Custom
	default:
		return Config{}, fmt.Errorf("invalid bootstrap mode: %q (expected full_tier1, minimal, or custom)", modeStr)
	}

	cfg := Config{Mode: mode}

	if mode == Custom {
		toolsStr := strings.TrimSpace(os.Getenv("CLOCKIFY_BOOTSTRAP_TOOLS"))
		if toolsStr == "" {
			return Config{}, fmt.Errorf("CLOCKIFY_BOOTSTRAP_TOOLS is required when mode is custom")
		}
		cfg.CustomTools = make(map[string]bool)
		for t := range strings.SplitSeq(toolsStr, ",") {
			name := strings.TrimSpace(t)
			if name != "" {
				cfg.CustomTools[name] = true
			}
		}
	}

	return cfg, nil
}

// SetTier1Tools stores the set of Tier 1 tool names for visibility checking.
func (c *Config) SetTier1Tools(names map[string]bool) {
	c.Tier1Names = names
}

// IsVisible reports whether a tool should be exposed to the LLM.
func (c *Config) IsVisible(name string) bool {
	if AlwaysVisible[name] {
		return true
	}
	if c.ActiveTools[name] {
		return true
	}
	switch c.Mode {
	case FullTier1:
		return c.Tier1Names[name]
	case Minimal:
		return MinimalSet[name]
	case Custom:
		return c.CustomTools[name]
	}
	return false
}

// ActivateTool marks a tool as visible regardless of bootstrap mode.
func (c *Config) ActivateTool(name string) {
	if c.ActiveTools == nil {
		c.ActiveTools = make(map[string]bool)
	}
	c.ActiveTools[name] = true
}

// ActivateTools marks multiple tools as visible regardless of bootstrap mode.
func (c *Config) ActivateTools(names []string) {
	for _, name := range names {
		c.ActivateTool(name)
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

// VisibleCount returns how many of the registered Tier 1 tool names pass IsVisible.
func (c *Config) VisibleCount() int {
	count := 0
	for name := range c.Tier1Names {
		if c.IsVisible(name) {
			count++
		}
	}
	return count
}

// SearchCatalog performs a case-insensitive substring search across all catalog
// entries, matching against Name, Description, Domain, and Keywords.
// An empty query returns all entries.
func SearchCatalog(query string) []CatalogEntry {
	if query == "" {
		result := make([]CatalogEntry, len(Tier1Catalog))
		copy(result, Tier1Catalog)
		return result
	}

	q := strings.ToLower(query)
	var results []CatalogEntry
	for _, entry := range Tier1Catalog {
		if strings.Contains(strings.ToLower(entry.Name), q) ||
			strings.Contains(strings.ToLower(entry.Description), q) ||
			strings.Contains(strings.ToLower(entry.Domain), q) {
			results = append(results, entry)
			continue
		}
		for _, kw := range entry.Keywords {
			if strings.Contains(strings.ToLower(kw), q) {
				results = append(results, entry)
				break
			}
		}
	}
	return results
}

// String returns the string representation of a Mode.
func (m Mode) String() string {
	switch m {
	case FullTier1:
		return "full_tier1"
	case Minimal:
		return "minimal"
	case Custom:
		return "custom"
	default:
		return fmt.Sprintf("unknown(%d)", int(m))
	}
}
