package tools

import (
	"maps"
	"strings"

	"github.com/apet97/go-clockify/internal/mcp"
)

// baseAnnotations returns the common annotation map every tool carries.
// openWorldHint is always true because every Clockify MCP tool touches the
// external Clockify API (not a closed local system), and title is derived
// from the tool name for display in MCP clients that render a tool picker.
// Callers overlay hint fields (readOnlyHint, destructiveHint, idempotentHint)
// on top of this base so each descriptor ends up with a complete annotation
// set instead of a sparse one that spec-strict clients misinterpret.
func baseAnnotations(name string) map[string]any {
	return map[string]any{
		"title":         titleFor(name),
		"openWorldHint": true,
	}
}

var toolTitleOverrides = map[string]string{
	"clockify_status":              "Status Overview",
	"clockify_tools_guide":         "Tools Guide",
	"clockify_create_work_package": "Create Work Package",
	"clockify_log_work":            "Log Finished Work",
	"clockify_start_work":          "Start Work Timer",
	"clockify_stop_work":           "Stop Work Timer",
	"clockify_switch_work":         "Switch Work Timer",
	"clockify_review_day":          "Review Day",
	"clockify_review_week":         "Review Week",
	"clockify_fix_entry":           "Fix Time Entry",
	"clockify_invoice_client_work": "Invoice Client Work",
	"clockify_record_expense":      "Record Expense",
	"clockify_request_time_off":    "Request Time Off",
	"clockify_schedule_work":       "Schedule Work",
	"clockify_setup_webhook":       "Set Up Webhook",
	"clockify_demo_seed":           "Seed Demo Data",
	"clockify_demo_cleanup":        "Clean Up Demo Data",
	"clockify_api_get":             "Raw Clockify API GET",
	"clockify_api_request":         "Raw Clockify API Request",
}

var titleCRUDVerbs = map[string]bool{
	"create": true,
	"update": true,
	"delete": true,
	"list":   true,
	"get":    true,
}

// titleFor returns a stable, human-readable display title for a tool name.
// Curated overrides win; otherwise a deterministic transform applies.
func titleFor(name string) string {
	if t, ok := toolTitleOverrides[name]; ok {
		return t
	}
	trimmed := strings.TrimPrefix(name, "clockify_")
	parts := strings.Split(trimmed, "_")
	if len(parts) > 1 && titleCRUDVerbs[parts[len(parts)-1]] {
		verb := parts[len(parts)-1]
		parts = append([]string{verb}, parts[:len(parts)-1]...)
	}
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

// newTool is the shared core of the four risk-class builders below.
// It overlays the supplied hint values on the baseAnnotations(name)
// scaffold and returns the assembled mcp.Tool. Centralising the
// overlay keeps the readOnly/destructive/idempotent hint matrix
// consistent — when a fifth risk class is added it becomes a one-
// line addition here, not another near-duplicate top-level helper.
func newTool(name, desc string, schema map[string]any, readOnly, destructive, idempotent bool) mcp.Tool {
	ann := baseAnnotations(name)
	ann["readOnlyHint"] = readOnly
	ann["destructiveHint"] = destructive
	ann["idempotentHint"] = idempotent
	return mcp.Tool{Name: name, Description: desc, InputSchema: schema, Annotations: ann}
}

func toolRO(name, desc string, schema map[string]any) mcp.Tool {
	return newTool(name, desc, schema, true, false, true)
}

// toolRW marks a write tool as non-destructive and non-idempotent.
// Absent this explicit declaration, MCP spec-strict clients assume
// destructive for write tools and may require extra confirmation
// for every call.
func toolRW(name, desc string, schema map[string]any) mcp.Tool {
	return newTool(name, desc, schema, false, false, false)
}

// toolRWIdem marks a write tool as idempotent. Use for PUT/PATCH-style updates
// and tools whose handlers produce the same end state on repeated calls
// (e.g. clockify_entries_timer_stop when no timer is running becomes a no-op).
func toolRWIdem(name, desc string, schema map[string]any) mcp.Tool {
	return newTool(name, desc, schema, false, false, true)
}

func toolDestructive(name, desc string, schema map[string]any) mcp.Tool {
	return newTool(name, desc, schema, false, true, false)
}

func defaultTier(d mcp.ToolDescriptor) mcp.ToolDescriptor {
	d.Tiers = append(d.Tiers, "default", "core")
	return d
}

func normalizeDescriptors(in []mcp.ToolDescriptor) []mcp.ToolDescriptor {
	for i := range in {
		if in[i].Tool.Annotations == nil {
			in[i].Tool.Annotations = map[string]any{}
		}
		if _, ok := in[i].Tool.Annotations["category"]; !ok {
			in[i].Tool.Annotations["category"] = defaultToolCategory(in[i].Tool.Name)
		}
		if in[i].Tool.Title == "" {
			in[i].Tool.Title = titleFor(in[i].Tool.Name)
		}
		applyMCPContractSafetyHints(&in[i])
		if value, ok := in[i].Tool.Annotations["readOnlyHint"].(bool); ok {
			in[i].ReadOnlyHint = value
		}
		if value, ok := in[i].Tool.Annotations["destructiveHint"].(bool); ok {
			in[i].DestructiveHint = value
		}
		if value, ok := in[i].Tool.Annotations["idempotentHint"].(bool); ok {
			in[i].IdempotentHint = value
		}
		if in[i].Tool.InputSchema != nil {
			tightenInputSchema(in[i].Tool.InputSchema)
		}
		applyRiskMetadata(&in[i])
		applyAgentToolMetadata(&in[i])
	}
	return in
}

var mcpContractDestructiveHintOverrides = map[string]bool{
	"clockify_invoices_send":      true,
	"clockify_projects_archive":   true,
	"clockify_scheduling_publish": true,
	"clockify_time_off_archive":   true,
	"clockify_users_invite":       true,
}

func applyMCPContractSafetyHints(d *mcp.ToolDescriptor) {
	if mcpContractDestructiveHintOverrides[d.Tool.Name] {
		d.Tool.Annotations["destructiveHint"] = true
	}
}

func defaultToolCategory(name string) string {
	switch name {
	case "clockify_api_get", "clockify_api_request":
		return "raw"
	default:
		return "domain"
	}
}

var agentToolMetadata = map[string]map[string]any{
	oneUserToolLogWork: {
		"bestToolFor": []string{"log finished past work", "create a complete time entry"},
		"preferOver":  []string{oneUserToolEntriesCreate},
	},
	oneUserToolReviewDay: {
		"bestToolFor": []string{"find gaps and overlaps", "plan timesheet cleanup"},
	},
	oneUserToolEntriesCreateFromGap: {
		"bestToolFor": []string{"fill a reviewed timesheet gap"},
		"preferAfter": []string{oneUserToolReviewDay},
	},
	"clockify_set_project_memberships": {
		"compatibilityShim": true,
		"primaryTool":       "clockify_update_project_memberships",
	},
}

func applyAgentToolMetadata(d *mcp.ToolDescriptor) {
	meta, ok := agentToolMetadata[d.Tool.Name]
	if !ok {
		return
	}
	if d.Tool.Annotations == nil {
		d.Tool.Annotations = map[string]any{}
	}
	for key, value := range meta {
		if _, exists := d.Tool.Annotations[key]; !exists {
			d.Tool.Annotations[key] = value
		}
	}
}

// applyRiskMetadata populates RiskClass and AuditKeys for a descriptor.
// The default risk class is derived from the three MCP boolean hints; per-tool
// overrides in riskOverrides win because billing/admin/permission_change
// distinctions cannot be expressed as booleans.
func applyRiskMetadata(d *mcp.ToolDescriptor) {
	if d.RiskClass == 0 {
		switch {
		case d.DestructiveHint:
			d.RiskClass = mcp.RiskDestructive
		case d.ReadOnlyHint:
			d.RiskClass = mcp.RiskRead
		default:
			d.RiskClass = mcp.RiskWrite
		}
	}
	if override, ok := riskOverrides[d.Tool.Name]; ok {
		if override.class != 0 {
			d.RiskClass = override.class
		}
		if len(override.auditKeys) > 0 && len(d.AuditKeys) == 0 {
			d.AuditKeys = append([]string(nil), override.auditKeys...)
		}
	}
	if d.Tool.Annotations == nil {
		d.Tool.Annotations = map[string]any{}
	}
	if names := riskClassAnnotationNames(d.RiskClass); len(names) > 0 {
		d.Tool.Annotations["riskClass"] = names
	}
	d.Tool.Annotations["dryRun"] = schemaHasDryRun(d.Tool.InputSchema)
}

func riskClassAnnotationNames(rc mcp.RiskClass) []string {
	if rc == 0 {
		return nil
	}
	type entry struct {
		bit  mcp.RiskClass
		name string
	}
	all := []entry{
		{mcp.RiskRead, "read"},
		{mcp.RiskWrite, "write"},
		{mcp.RiskSensitiveRead, "sensitive_read"},
		{mcp.RiskBilling, "billing"},
		{mcp.RiskAdmin, "admin"},
		{mcp.RiskPermissionChange, "permission_change"},
		{mcp.RiskExternalSideEffect, "external_side_effect"},
		{mcp.RiskDestructive, "destructive"},
	}
	out := make([]string, 0, 2)
	for _, e := range all {
		if rc.Has(e.bit) {
			out = append(out, e.name)
		}
	}
	return out
}

func schemaHasDryRun(schema map[string]any) bool {
	if schema == nil {
		return false
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return false
	}
	_, ok = props["dry_run"]
	return ok
}

func dryrunPreviewPayload(tool string, payload map[string]any) map[string]any {
	return map[string]any{
		"dry_run": true,
		"tool":    tool,
		"payload": maps.Clone(payload),
		"note":    "No changes were made.",
	}
}
