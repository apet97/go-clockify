package tools

import (
	"slices"
	"strings"

	"github.com/apet97/go-clockify/internal/mcp"
)

// FullAccessRegistryChecked builds (once, memoized) and returns the full 156-tool
// registry in the canonical order (workflow, domain, raw fallback), validating
// it via ValidateRegistry. It returns an error rather than panicking when
// descriptor construction or validation fails.
func (s *Service) FullAccessRegistryChecked() ([]mcp.ToolDescriptor, error) {
	s.registry.once.Do(func() {
		s.registry.descriptors, s.registry.err = s.buildFullAccessRegistry()
		if s.registry.err == nil {
			s.registry.err = ValidateRegistry(s.registry.descriptors)
		}
	})
	if s.registry.err != nil {
		return nil, s.registry.err
	}
	return cloneToolDescriptors(s.registry.descriptors), nil
}

// RegistryForToolset returns the one-user startup registry for a narrower
// advertised surface. The "all" and empty paths are intentionally identical to
// FullAccessRegistryChecked so the canonical 156-tool product contract stays intact.
func (s *Service) RegistryForToolset(toolset string) []mcp.ToolDescriptor {
	toolset = strings.ToLower(strings.TrimSpace(toolset))
	full, err := s.FullAccessRegistryChecked()
	if err != nil {
		return nil
	}
	if toolset == "" || toolset == "all" {
		return full
	}
	out := make([]mcp.ToolDescriptor, 0, len(full))
	raw := make([]mcp.ToolDescriptor, 0, 2)
	for _, descriptor := range full {
		if descriptor.Tool.Name == "clockify_api_get" || descriptor.Tool.Name == "clockify_api_request" {
			raw = append(raw, descriptor)
		}
		if toolset == "default" {
			if descriptorInTier(descriptor, "default") {
				out = append(out, descriptor)
			}
			continue
		}
		if toolAllowedForToolset(toolset, descriptor.Tool.Name) {
			out = append(out, descriptor)
		}
	}
	if s != nil && s.EnableRawTools {
		out = append(out, raw...)
	}
	return out
}

func (s *Service) buildFullAccessRegistry() ([]mcp.ToolDescriptor, error) {
	out := make([]mcp.ToolDescriptor, 0, 160)
	out = append(out, s.workflowDescriptors()...)
	out = append(out, s.FirstSliceRegistry()...)
	out = append(out, s.nativeCoreDescriptors()...)
	highValue, err := s.nativeHighValueDescriptorsChecked()
	if err != nil {
		return nil, err
	}
	out = append(out, highValue...)
	out = append(out, s.nativeDomainExtras()...)
	out = append(out, s.timerAndReportDescriptors()...)
	out = append(out, s.rawAPIDescriptors()...)
	return normalizeDescriptors(out), nil
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

func descriptorInTier(descriptor mcp.ToolDescriptor, tier string) bool {
	return slices.Contains(descriptor.Tiers, tier)
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
