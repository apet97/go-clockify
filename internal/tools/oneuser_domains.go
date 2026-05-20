package tools

import (
	"strings"

	"github.com/apet97/go-clockify/internal/mcp"
)

// FullAccessRegistry is the one-user product registry: every supported tool is
// visible from startup, with workflows first and raw API fallback last.
func (s *Service) FullAccessRegistry() []mcp.ToolDescriptor {
	s.registryOnce.Do(func() {
		s.registry = s.buildFullAccessRegistry()
	})
	return cloneToolDescriptors(s.registry)
}

// RegistryForToolset returns the one-user startup registry for a narrower
// owner-mode surface. The default "all" path is intentionally identical to
// FullAccessRegistry so the canonical 156-tool product contract stays intact.
func (s *Service) RegistryForToolset(toolset string) []mcp.ToolDescriptor {
	toolset = strings.ToLower(strings.TrimSpace(toolset))
	if toolset == "" || toolset == "all" {
		return s.FullAccessRegistry()
	}
	full := s.FullAccessRegistry()
	out := make([]mcp.ToolDescriptor, 0, len(full))
	for _, descriptor := range full {
		if toolAllowedForToolset(toolset, descriptor.Tool.Name) {
			out = append(out, descriptor)
		}
	}
	return out
}

func (s *Service) buildFullAccessRegistry() []mcp.ToolDescriptor {
	out := make([]mcp.ToolDescriptor, 0, 160)
	out = append(out, s.workflowDescriptors()...)
	out = append(out, s.FirstSliceRegistry()...)
	out = append(out, s.nativeCoreDescriptors()...)
	out = append(out, s.nativeHighValueDescriptors()...)
	out = append(out, s.nativeDomainExtras()...)
	out = append(out, s.timerAndReportDescriptors()...)
	out = append(out, s.rawAPIDescriptors()...)
	return normalizeDescriptors(dedupeToolDescriptors(out))
}

// cloneToolDescriptors returns an independent copy of the descriptor slice.
// Each descriptor's Tool.InputSchema, Tool.OutputSchema, and Tool.Annotations
// maps are deep-copied so a caller that mutates a returned schema or annotation
// cannot corrupt the cached registry.
func cloneToolDescriptors(in []mcp.ToolDescriptor) []mcp.ToolDescriptor {
	out := make([]mcp.ToolDescriptor, len(in))
	copy(out, in)
	for i := range out {
		out[i].Tool.InputSchema = deepCopyAnyMap(out[i].Tool.InputSchema)
		out[i].Tool.OutputSchema = deepCopyAnyMap(out[i].Tool.OutputSchema)
		out[i].Tool.Annotations = deepCopyAnyMap(out[i].Tool.Annotations)
		out[i].Tiers = append([]string(nil), out[i].Tiers...)
	}
	return out
}

// deepCopyAnyMap returns an independent copy of a JSON-shaped map: nested
// map[string]any, []any, and []string values are copied; scalars are passed
// through (immutable). Used so cloneToolDescriptors hands callers a descriptor
// whose schema/annotation maps cannot mutate the cached registry.
func deepCopyAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = deepCopyAnyValue(v)
	}
	return out
}

func deepCopyAnyValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return deepCopyAnyMap(t)
	case []any:
		if t == nil {
			return []any(nil)
		}
		cp := make([]any, len(t))
		for i := range t {
			cp[i] = deepCopyAnyValue(t[i])
		}
		return cp
	case []string:
		if t == nil {
			return []string(nil)
		}
		cp := make([]string, len(t))
		copy(cp, t)
		return cp
	default:
		return v
	}
}

func toolAllowedForToolset(toolset, name string) bool {
	switch toolset {
	case "core":
		return coreToolsetTool(name)
	case "business":
		return coreToolsetTool(name) || businessToolsetTool(name)
	case "admin":
		return coreToolsetTool(name) || businessToolsetTool(name) || adminToolsetTool(name)
	default:
		return true
	}
}

func coreToolsetTool(name string) bool {
	if coreWorkflowTools[name] {
		return true
	}
	for _, prefix := range []string{
		"clockify_clients_",
		"clockify_projects_",
		"clockify_tasks_",
		"clockify_tags_",
		"clockify_entries_",
		"clockify_reports_",
	} {
		if strings.HasPrefix(name, prefix) {
			return name != "clockify_entries_mark_invoiced"
		}
	}
	return false
}

func businessToolsetTool(name string) bool {
	if businessWorkflowTools[name] {
		return true
	}
	if name == "clockify_entries_mark_invoiced" {
		return true
	}
	for _, prefix := range []string{"clockify_invoices_", "clockify_expenses_"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func adminToolsetTool(name string) bool {
	if adminWorkflowTools[name] {
		return true
	}
	for _, prefix := range []string{
		"clockify_custom_fields_",
		"clockify_time_off_",
		"clockify_scheduling_",
		"clockify_approvals_",
		"clockify_webhooks_",
		"clockify_groups_",
		"clockify_holidays_",
		"clockify_users_",
	} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return name == "clockify_workspace_settings"
}

var coreWorkflowTools = map[string]bool{
	"clockify_status":              true,
	"clockify_tools_guide":         true,
	"clockify_create_work_package": true,
	"clockify_log_work":            true,
	"clockify_start_work":          true,
	"clockify_stop_work":           true,
	"clockify_switch_work":         true,
	"clockify_review_day":          true,
	"clockify_review_week":         true,
	"clockify_fix_entry":           true,
}

var businessWorkflowTools = map[string]bool{
	"clockify_invoice_client_work": true,
	"clockify_record_expense":      true,
}

var adminWorkflowTools = map[string]bool{
	"clockify_request_time_off": true,
	"clockify_schedule_work":    true,
	"clockify_setup_webhook":    true,
}

func dedupeToolDescriptors(in []mcp.ToolDescriptor) []mcp.ToolDescriptor {
	seen := map[string]bool{}
	out := make([]mcp.ToolDescriptor, 0, len(in))
	for _, descriptor := range in {
		name := descriptor.Tool.Name
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, descriptor)
	}
	return out
}
