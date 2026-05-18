package tools

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

type routeTool struct {
	Name        string
	Description string
	Method      string
	Path        string
	Entity      string
	Required    []string
	Properties  map[string]any
	Query       []string
	Body        []string
	Defaults    map[string]any
	Reports     bool
	ReadOnly    bool
	Idempotent  bool
	Destructive bool
	Change      string
	Priority    int
}

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

func cloneToolDescriptors(in []mcp.ToolDescriptor) []mcp.ToolDescriptor {
	out := make([]mcp.ToolDescriptor, len(in))
	copy(out, in)
	return out
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

func (s *Service) timerAndReportDescriptors() []mcp.ToolDescriptor {
	out := make([]mcp.ToolDescriptor, 0, 8)
	out = append(out,
		firstSliceDescriptor(66, toolRO("clockify_entries_running", "Return the current running timer, if any.", objectSchema(nil)), s.EntriesRunning),
		firstSliceDescriptor(67, toolRW("clockify_entries_timer_start", "Start a timer.", objectSchema(map[string]any{"properties": map[string]any{
			"project_id":  map[string]any{"type": "string"},
			"project":     map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
			"billable":    map[string]any{"type": "boolean", "description": "Override the project's billable default. Omit to inherit the project setting."},
		}})), s.EntriesTimerStart),
		firstSliceDescriptor(68, toolRWIdem("clockify_entries_timer_stop", "Stop the current timer.", objectSchema(map[string]any{"properties": map[string]any{
			"end": map[string]any{"type": "string", "description": flexibleDatetimeDescription},
		}})), s.EntriesTimerStop),
		firstSliceDescriptor(69, toolRO("clockify_entries_timer_status", "Show timer status.", objectSchema(nil)), s.EntriesTimerStatus),
		firstSliceDescriptor(70, toolRW("clockify_entries_timer_switch", "Switch the running timer to another project.", objectSchema(map[string]any{"required": []string{"project"}, "properties": map[string]any{
			"project":     map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
			"task_id":     map[string]any{"type": "string"},
			"billable":    map[string]any{"type": "boolean"},
		}})), s.EntriesTimerSwitch),
		firstSliceDescriptor(104, toolRO("clockify_reports_detailed", "Run the local detailed time report helper.", reportHelperSchema()), aliasHandler("clockify_reports_detailed", "report", "", s.DetailedReport)),
		firstSliceDescriptor(105, toolRO("clockify_reports_summary", "Run the local summary report helper.", reportHelperSchema()), aliasHandler("clockify_reports_summary", "report", "", s.SummaryReport)),
		firstSliceDescriptor(106, toolRO("clockify_reports_weekly", "Run the local weekly report helper. Range must be exactly 7 days; pass week_start (YYYY-MM-DD) alone to auto-derive the week end.", reportHelperSchema()), aliasHandler("clockify_reports_weekly", "report", "", s.WeeklySummary)),
	)
	return out
}

func (s *Service) nativeCoreDescriptors() []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		nativeDomainTool(22, toolRO("clockify_clients_get", "Get one client by name or ID.", objectSchema(map[string]any{"required": []string{"client"}, "properties": map[string]any{
			"client":    map[string]any{"type": "string", "description": "Client name or ID."},
			"client_id": map[string]any{"type": "string", "description": "Client ID."},
		}})), "client", "", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.GetClientWithArgs(ctx, aliasArg(args, "client_id", "client"))
		}),
		nativeDomainTool(23, toolRWIdem("clockify_clients_update", "Update a client by name or ID.", objectSchema(map[string]any{"required": []string{"client"}, "properties": map[string]any{
			"client":             map[string]any{"type": "string", "description": "Client name or ID."},
			"client_id":          map[string]any{"type": "string", "description": "Client ID."},
			"name":               map[string]any{"type": "string"},
			"email":              map[string]any{"type": "string"},
			"address":            map[string]any{"type": "string"},
			"note":               map[string]any{"type": "string"},
			"currency_id":        map[string]any{"type": "string"},
			"cc_emails":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"archived":           map[string]any{"type": "boolean"},
			"archive_projects":   map[string]any{"type": "boolean"},
			"mark_tasks_as_done": map[string]any{"type": "boolean"},
		}})), "client", "updated", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.UpdateClient(ctx, aliasArg(args, "client_id", "client"))
		}),
		nativeDomainTool(24, toolDestructive("clockify_clients_delete", "Permanently delete a client by name or ID. Destructive; supports dry_run preview.", objectSchema(map[string]any{"required": []string{"client"}, "properties": map[string]any{
			"client":    map[string]any{"type": "string", "description": "Client name or ID."},
			"client_id": map[string]any{"type": "string", "description": "Client ID."},
			"dry_run":   map[string]any{"type": "boolean"},
		}})), "client", "deleted", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.DeleteClient(ctx, aliasArg(args, "client_id", "client"))
		}),

		nativeDomainTool(32, toolRO("clockify_projects_get", "Get one project by name or ID.", objectSchema(map[string]any{"required": []string{"project"}, "properties": map[string]any{
			"project":                  map[string]any{"type": "string", "description": "Project name or ID."},
			"project_id":               map[string]any{"type": "string", "description": "Project ID."},
			"hydrated":                 map[string]any{"type": "boolean", "description": "Clockify hydrated query flag; defaults to false for compact list responses."},
			"custom_field_entity_type": map[string]any{"type": "string"},
			"expense_limit":            map[string]any{"type": "integer"},
			"expense_date":             map[string]any{"type": "string", "description": "Expense date query value, typically YYYY-MM-DD."},
		}})), "project", "", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.GetProject(ctx, aliasArg(args, "project_id", "project"))
		}),
		nativeDomainTool(33, toolRWIdem("clockify_projects_update", "Update a project by name or ID.", objectSchema(map[string]any{"required": []string{"project"}, "properties": map[string]any{
			"project":    map[string]any{"type": "string", "description": "Project name or ID."},
			"project_id": map[string]any{"type": "string", "description": "Project ID."},
			"name":       map[string]any{"type": "string"},
			"client":     map[string]any{"type": "string", "description": "Client name or ID."},
			"client_id":  map[string]any{"type": "string"},
			"color":      map[string]any{"type": "string"},
			"billable":   map[string]any{"type": "boolean"},
			"is_public":  map[string]any{"type": "boolean"},
			"archived":   map[string]any{"type": "boolean"},
			"note":       map[string]any{"type": "string"},
		}})), "project", "updated", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.UpdateProject(ctx, aliasArg(args, "project_id", "project"))
		}),
		nativeDomainTool(34, toolDestructive("clockify_projects_delete", "Permanently delete a project by name or ID. Destructive; supports dry_run preview.", objectSchema(map[string]any{"required": []string{"project"}, "properties": map[string]any{
			"project":    map[string]any{"type": "string", "description": "Project name or ID."},
			"project_id": map[string]any{"type": "string", "description": "Project ID."},
			"dry_run":    map[string]any{"type": "boolean"},
		}})), "project", "deleted", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.DeleteProject(ctx, aliasArg(args, "project_id", "project"))
		}),
		nativeDomainTool(35, toolRWIdem("clockify_projects_archive", "Archive a project by name or ID.", objectSchema(map[string]any{"required": []string{"project"}, "properties": map[string]any{
			"project":    map[string]any{"type": "string", "description": "Project name or ID."},
			"project_id": map[string]any{"type": "string", "description": "Project ID."},
		}})), "project", "updated", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			args = aliasArg(args, "project_id", "project")
			args["archived"] = true
			return s.UpdateProject(ctx, args)
		}),
		nativeDomainTool(41, toolRWIdem("clockify_projects_rates_update", "Set a project member hourly or cost rate. Admin and billing impact: rate changes flow through every future time entry on this project.", objectSchema(map[string]any{"required": []string{"project_id", "user_id", "rate_kind", "amount"}, "properties": map[string]any{
			"project_id": map[string]any{"type": "string"},
			"user_id":    map[string]any{"type": "string"},
			"rate_kind":  map[string]any{"type": "string", "enum": []string{"hourly", "cost"}},
			"amount":     map[string]any{"type": "integer", "minimum": 0, "description": "Rate amount in minor currency units (cents): 1000 = $10.00/hr."},
			"since":      map[string]any{"type": "string"},
			"dry_run":    map[string]any{"type": "boolean"},
		}})), "project_rate", "updated", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			endpoint, err := rateKindEndpoint(args)
			if err != nil {
				return ResultEnvelope{}, err
			}
			return s.UpdateProjectUserRate(ctx, args, endpoint)
		}),

		nativeDomainTool(42, toolRO("clockify_tasks_get", "Get one task by name or ID within a project.", objectSchema(map[string]any{"required": []string{"project", "task"}, "properties": map[string]any{
			"project":    map[string]any{"type": "string", "description": "Project name or ID."},
			"project_id": map[string]any{"type": "string", "description": "Project ID."},
			"task":       map[string]any{"type": "string", "description": "Task name or ID."},
			"task_id":    map[string]any{"type": "string", "description": "Task ID."},
		}})), "task", "", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.GetTask(ctx, aliasArgs(args, map[string]string{"project_id": "project", "task_id": "task"}))
		}),
		nativeDomainTool(43, toolRWIdem("clockify_tasks_update", "Update a task by name or ID within a project.", objectSchema(map[string]any{"required": []string{"project", "task"}, "properties": map[string]any{
			"project":           map[string]any{"type": "string", "description": "Project name or ID."},
			"project_id":        map[string]any{"type": "string", "description": "Project ID."},
			"task":              map[string]any{"type": "string", "description": "Task name or ID."},
			"task_id":           map[string]any{"type": "string", "description": "Task ID."},
			"name":              map[string]any{"type": "string"},
			"billable":          map[string]any{"type": "boolean"},
			"assignee_ids":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"status":            map[string]any{"type": "string", "enum": []string{"ACTIVE", "DONE"}},
			"contains_assignee": map[string]any{"type": "boolean"},
			"membership_status": map[string]any{"type": "string", "enum": []string{"PENDING", "ACTIVE", "DECLINED", "INACTIVE", "ALL"}},
		}})), "task", "updated", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.UpdateTask(ctx, aliasArgs(args, map[string]string{"project_id": "project", "task_id": "task"}))
		}),
		nativeDomainTool(44, toolDestructive("clockify_tasks_delete", "Permanently delete a task by name or ID within a project. A task that is not DONE is marked DONE first (Clockify requires it), then deleted. Destructive; supports dry_run preview.", objectSchema(map[string]any{"required": []string{"project", "task"}, "properties": map[string]any{
			"project":    map[string]any{"type": "string", "description": "Project name or ID."},
			"project_id": map[string]any{"type": "string", "description": "Project ID."},
			"task":       map[string]any{"type": "string", "description": "Task name or ID."},
			"task_id":    map[string]any{"type": "string", "description": "Task ID."},
			"dry_run":    map[string]any{"type": "boolean"},
		}})), "task", "deleted", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.DeleteTask(ctx, aliasArgs(args, map[string]string{"project_id": "project", "task_id": "task"}))
		}),
		nativeDomainTool(45, toolRWIdem("clockify_tasks_rates_update", "Set a task hourly or cost rate. Billing impact: changes the billable amount on every future time entry charged to this task.", objectSchema(map[string]any{"required": []string{"rate_kind", "amount", "project", "task"}, "properties": map[string]any{
			"project":    map[string]any{"type": "string", "description": "Project name or ID."},
			"project_id": map[string]any{"type": "string"},
			"task":       map[string]any{"type": "string", "description": "Task name or ID."},
			"task_id":    map[string]any{"type": "string"},
			"rate_kind":  map[string]any{"type": "string", "enum": []string{"hourly", "cost"}},
			"amount":     map[string]any{"type": "integer", "minimum": 0, "description": "Rate amount in minor currency units (cents): 1000 = $10.00/hr."},
			"since":      map[string]any{"type": "string"},
			"dry_run":    map[string]any{"type": "boolean"},
		}})), "task_rate", "updated", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			endpoint, err := rateKindEndpoint(args)
			if err != nil {
				return ResultEnvelope{}, err
			}
			return s.UpdateTaskRate(ctx, args, endpoint)
		}),

		nativeDomainTool(52, toolRO("clockify_tags_get", "Get one tag by name or ID.", objectSchema(map[string]any{"required": []string{"tag"}, "properties": map[string]any{
			"tag":    map[string]any{"type": "string", "description": "Tag name or ID."},
			"tag_id": map[string]any{"type": "string", "description": "Tag ID."},
		}})), "tag", "", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.GetTag(ctx, aliasArg(args, "tag_id", "tag"))
		}),
		nativeDomainTool(53, toolRWIdem("clockify_tags_update", "Update a tag by name or ID.", objectSchema(map[string]any{"required": []string{"tag"}, "properties": map[string]any{
			"tag":      map[string]any{"type": "string", "description": "Tag name or ID."},
			"tag_id":   map[string]any{"type": "string", "description": "Tag ID."},
			"name":     map[string]any{"type": "string"},
			"archived": map[string]any{"type": "boolean"},
		}})), "tag", "updated", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.UpdateTag(ctx, aliasArg(args, "tag_id", "tag"))
		}),
		nativeDomainTool(54, toolDestructive("clockify_tags_delete", "Permanently delete a tag by name or ID. Destructive; supports dry_run preview.", objectSchema(map[string]any{"required": []string{"tag"}, "properties": map[string]any{
			"tag":     map[string]any{"type": "string", "description": "Tag name or ID."},
			"tag_id":  map[string]any{"type": "string", "description": "Tag ID."},
			"dry_run": map[string]any{"type": "boolean"},
		}})), "tag", "deleted", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.DeleteTag(ctx, aliasArg(args, "tag_id", "tag"))
		}),

		nativeDomainTool(62, toolRO("clockify_entries_get", "Get one time entry by ID.", objectSchema(map[string]any{"required": []string{"entry_id"}, "properties": map[string]any{
			"entry_id": map[string]any{"type": "string"},
		}})), "entry", "", s.GetEntry),
		nativeDomainTool(63, toolRWIdem("clockify_entries_update", "Update a time entry by ID.", objectSchema(map[string]any{"required": []string{"entry_id"}, "properties": map[string]any{
			"entry_id":    map[string]any{"type": "string"},
			"start":       map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"end":         map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"description": map[string]any{"type": "string"},
			"project":     map[string]any{"type": "string", "description": "Project name or ID."},
			"project_id":  map[string]any{"type": "string"},
			"task":        map[string]any{"type": "string", "description": "Task name or ID."},
			"task_id":     map[string]any{"type": "string"},
			"tag":         map[string]any{"type": "string", "description": "Tag name or ID."},
			"tag_ids":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"billable":    map[string]any{"type": "boolean"},
		}})), "entry", "updated", s.UpdateEntry),
		nativeDomainTool(64, toolDestructive("clockify_entries_delete", "Permanently delete a time entry by ID. Destructive; supports dry_run preview.", objectSchema(map[string]any{"required": []string{"entry_id"}, "properties": map[string]any{
			"entry_id": map[string]any{"type": "string"},
			"dry_run":  map[string]any{"type": "boolean"},
		}})), "entry", "deleted", s.DeleteEntry),

		nativeDomainTool(1100, toolRO("clockify_users_list", "List users in the pinned workspace.", userListSchema()), "user", "", s.ListUsers),
		nativeDomainTool(1101, toolRO("clockify_users_profile", "Get the current Clockify user.", objectSchema(nil)), "user", "", func(ctx context.Context, _ map[string]any) (ResultEnvelope, error) {
			return s.CurrentUser(ctx)
		}),
		nativeDomainTool(1105, toolRO("clockify_workspace_settings", "Read pinned workspace settings.", objectSchema(nil)), "workspace", "", func(ctx context.Context, _ map[string]any) (ResultEnvelope, error) {
			return s.GetWorkspace(ctx)
		}),
	}
}

func userListSchema() map[string]any {
	return paginationSchema(map[string]any{"properties": map[string]any{
		"email":            map[string]any{"type": "string"},
		"project_id":       map[string]any{"type": "string"},
		"status":           map[string]any{"type": "string", "enum": []string{"PENDING", "ACTIVE", "DECLINED", "INACTIVE", "ALL"}},
		"account_statuses": map[string]any{"type": "string"},
		"name":             map[string]any{"type": "string"},
		"sort_column":      map[string]any{"type": "string", "enum": []string{"ID", "EMAIL", "NAME", "NAME_LOWERCASE", "ACCESS", "HOURLYRATE", "COSTRATE"}},
		"sort_order":       map[string]any{"type": "string", "enum": []string{"ASCENDING", "DESCENDING"}},
		"memberships":      map[string]any{"type": "string", "enum": []string{"ALL", "NONE", "WORKSPACE", "PROJECT", "USERGROUP"}},
		"include_roles":    map[string]any{"type": "boolean"},
	}})
}

func (s *Service) nativeHighValueDescriptors() []mcp.ToolDescriptor {
	sources := s.nativeDomainDescriptorMap()
	out := make([]mcp.ToolDescriptor, 0, 96)
	add := func(priority int, name, oldName, entity, change string, handler func(context.Context, map[string]any) (ResultEnvelope, error)) {
		old, ok := sources[oldName]
		if !ok {
			// A missing source key means a typo or partial merge dropped a
			// high-value tool. Surface it loudly instead of vanishing the
			// tool silently; the registry count test is the hard gate.
			fmt.Fprintf(os.Stderr, "WARNING: descriptor source %q missing for tool %q — skipping\n", oldName, name)
			return
		}
		out = append(out, nativeDirectDescriptor(priority, name, old, entity, change, handler))
	}
	add(200, "clockify_invoices_list", "clockify_list_invoices", "invoice", "", s.listInvoices)
	add(201, "clockify_invoices_get", "clockify_get_invoice", "invoice", "", s.getInvoice)
	add(202, "clockify_invoices_create", "clockify_create_invoice", "invoice", "created", s.createInvoice)
	add(203, "clockify_invoices_update", "clockify_update_invoice", "invoice", "updated", s.updateInvoice)
	add(204, "clockify_invoices_delete", "clockify_delete_invoice", "invoice", "deleted", s.deleteInvoice)
	add(205, "clockify_invoices_send", "clockify_send_invoice", "invoice", "updated", s.sendInvoice)
	add(206, "clockify_invoices_mark_paid", "clockify_mark_invoice_paid", "invoice", "updated", s.markInvoicePaid)
	add(207, "clockify_invoices_items_list", "clockify_list_invoice_items", "invoice_item", "", s.listInvoiceItems)
	add(208, "clockify_invoices_items_add", "clockify_add_invoice_item", "invoice_item", "created", s.addInvoiceItem)
	add(209, "clockify_invoices_items_update", "clockify_update_invoice_item", "invoice_item", "updated", s.updateInvoiceItem)
	add(216, "clockify_invoices_items_delete", "clockify_delete_invoice_item", "invoice_item", "deleted", s.deleteInvoiceItem)
	add(36, "clockify_projects_templates_list", "clockify_list_project_templates", "project_template", "", s.ListProjectTemplates)
	add(37, "clockify_projects_templates_create", "clockify_create_project_template", "project_template", "created", s.CreateProjectTemplate)
	add(38, "clockify_projects_estimates_update", "clockify_update_project_estimate", "project", "updated", s.UpdateProjectEstimate)
	add(40, "clockify_projects_memberships_update", "clockify_update_project_memberships", "membership", "updated", s.updateProjectMembershipsOneUser)
	add(300, "clockify_expenses_list", "clockify_list_expenses", "expense", "", s.listExpenses)
	add(301, "clockify_expenses_get", "clockify_get_expense", "expense", "", s.getExpense)
	add(302, "clockify_expenses_create", "clockify_create_expense", "expense", "created", s.createExpense)
	add(303, "clockify_expenses_update", "clockify_update_expense", "expense", "updated", s.updateExpense)
	add(304, "clockify_expenses_delete", "clockify_delete_expense", "expense", "deleted", s.deleteExpense)
	add(305, "clockify_expenses_categories_list", "clockify_list_expense_categories", "expense_category", "", s.listExpenseCategories)
	add(306, "clockify_expenses_categories_create", "clockify_create_expense_category", "expense_category", "created", s.createExpenseCategory)
	add(307, "clockify_expenses_categories_update", "clockify_update_expense_category", "expense_category", "updated", s.updateExpenseCategory)
	add(308, "clockify_expenses_categories_delete", "clockify_delete_expense_category", "expense_category", "deleted", s.deleteExpenseCategory)
	add(400, "clockify_custom_fields_list", "clockify_list_custom_fields", "custom_field", "", s.ListCustomFields)
	add(401, "clockify_custom_fields_get", "clockify_get_custom_field", "custom_field", "", s.GetCustomField)
	add(402, "clockify_custom_fields_create", "clockify_create_custom_field", "custom_field", "created", s.CreateCustomField)
	add(403, "clockify_custom_fields_update", "clockify_update_custom_field", "custom_field", "updated", s.UpdateCustomField)
	add(404, "clockify_custom_fields_delete", "clockify_delete_custom_field", "custom_field", "deleted", s.DeleteCustomField)
	add(405, "clockify_custom_fields_set_value", "clockify_set_custom_field_value", "custom_field_value", "updated", s.SetCustomFieldValue)
	add(500, "clockify_time_off_requests_list", "clockify_list_time_off_requests", "time_off_request", "", s.listTimeOffRequests)
	add(501, "clockify_time_off_requests_get", "clockify_get_time_off_request", "time_off_request", "", s.getTimeOffRequest)
	add(502, "clockify_time_off_requests_create", "clockify_create_time_off_request", "time_off_request", "created", s.createTimeOffRequest)
	add(503, "clockify_time_off_requests_update", "clockify_update_time_off_request", "time_off_request", "updated", s.updateTimeOffRequest)
	add(504, "clockify_time_off_requests_delete", "clockify_delete_time_off_request", "time_off_request", "deleted", s.deleteTimeOffRequest)
	add(505, "clockify_time_off_approve", "clockify_approve_time_off", "time_off_request", "updated", s.approveTimeOff)
	add(506, "clockify_time_off_deny", "clockify_deny_time_off", "time_off_request", "updated", s.denyTimeOff)
	add(507, "clockify_time_off_policies_list", "clockify_list_time_off_policies", "time_off_policy", "", s.listTimeOffPolicies)
	add(508, "clockify_time_off_policies_get", "clockify_get_time_off_policy", "time_off_policy", "", s.getTimeOffPolicy)
	add(509, "clockify_time_off_policies_create", "clockify_create_time_off_policy", "time_off_policy", "created", s.createTimeOffPolicy)
	add(510, "clockify_time_off_policies_update", "clockify_update_time_off_policy", "time_off_policy", "updated", s.updateTimeOffPolicy)
	add(511, "clockify_time_off_balances", "clockify_time_off_balance", "time_off_balance", "", s.timeOffBalance)
	add(512, "clockify_time_off_balances_update", "clockify_time_off_balance_update", "time_off_balance", "updated", s.updateTimeOffBalance)
	add(600, "clockify_scheduling_assignments_list", "clockify_list_assignments", "assignment", "", s.listAssignments)
	add(601, "clockify_scheduling_assignments_get", "clockify_get_assignment", "assignment", "", s.getAssignment)
	add(602, "clockify_scheduling_assignments_create", "clockify_create_assignment", "assignment", "created", s.createAssignment)
	add(603, "clockify_scheduling_assignments_update", "clockify_update_assignment", "assignment", "updated", s.updateAssignment)
	add(604, "clockify_scheduling_assignments_delete", "clockify_delete_assignment", "assignment", "deleted", s.deleteAssignment)
	add(605, "clockify_scheduling_project_totals", "clockify_get_project_schedule_totals", "scheduling", "", s.getProjectScheduleTotals)
	add(700, "clockify_approvals_list", "clockify_list_approval_requests", "approval", "", s.listApprovalRequests)
	add(701, "clockify_approvals_get", "clockify_get_approval_request", "approval", "", s.getApprovalRequest)
	add(702, "clockify_approvals_submit", "clockify_submit_for_approval", "approval", "created", s.submitForApproval)
	add(703, "clockify_approvals_approve", "clockify_approve_timesheet", "approval", "updated", s.approveTimesheet)
	add(704, "clockify_approvals_reject", "clockify_reject_timesheet", "approval", "updated", s.rejectTimesheet)
	add(705, "clockify_approvals_withdraw", "clockify_withdraw_approval", "approval", "updated", s.withdrawApproval)
	add(800, "clockify_webhooks_list", "clockify_list_webhooks", "webhook", "", s.ListWebhooks)
	add(801, "clockify_webhooks_get", "clockify_get_webhook", "webhook", "", s.GetWebhook)
	add(802, "clockify_webhooks_create", "clockify_create_webhook", "webhook", "created", s.CreateWebhook)
	add(803, "clockify_webhooks_update", "clockify_update_webhook", "webhook", "updated", s.UpdateWebhook)
	add(804, "clockify_webhooks_delete", "clockify_delete_webhook", "webhook", "deleted", s.DeleteWebhook)
	add(805, "clockify_webhooks_test", "clockify_test_webhook", "webhook", "updated", s.TestWebhook)
	add(806, "clockify_webhooks_events", "clockify_list_webhook_events", "webhook_event", "", s.ListWebhookEvents)
	add(900, "clockify_groups_list", "clockify_list_user_groups_admin", "group", "", s.ListUserGroupsAdmin)
	add(901, "clockify_groups_get", "clockify_get_user_group", "group", "", s.GetUserGroup)
	add(902, "clockify_groups_create", "clockify_create_user_group_admin", "group", "created", s.CreateUserGroupAdmin)
	add(903, "clockify_groups_update", "clockify_update_user_group_admin", "group", "updated", s.UpdateUserGroupAdmin)
	add(904, "clockify_groups_delete", "clockify_delete_user_group_admin", "group", "deleted", s.DeleteUserGroupAdmin)
	add(905, "clockify_groups_add_user", "clockify_add_user_to_group", "group_member", "created", s.AddUserToGroup)
	add(906, "clockify_groups_remove_user", "clockify_remove_user_from_group", "group_member", "deleted", s.RemoveUserFromGroup)
	add(1000, "clockify_holidays_list", "clockify_list_holidays", "holiday", "", func(ctx context.Context, _ map[string]any) (ResultEnvelope, error) {
		return s.ListHolidays(ctx)
	})
	add(1003, "clockify_holidays_list_for_user_period", "clockify_list_holidays_in_period", "holiday", "", s.ListHolidaysInPeriod)
	add(1004, "clockify_holidays_create", "clockify_create_holiday", "holiday", "created", s.CreateHoliday)
	add(1005, "clockify_holidays_delete", "clockify_delete_holiday", "holiday", "deleted", s.DeleteHoliday)
	add(1103, "clockify_users_deactivate", "clockify_deactivate_user", "user", "updated", s.DeactivateUser)
	add(1104, "clockify_users_role", "clockify_update_user_role", "user", "updated", s.UpdateUserRole)
	out = append(out, s.nativeRouteDescriptors()...)
	out = append(out,
		nativeDomainTool(65, toolRWIdem("clockify_entries_mark_invoiced", "Mark time entries as invoiced or not invoiced.", objectSchema(map[string]any{"required": []string{"time_entry_ids", "invoiced"}, "properties": map[string]any{
			"time_entry_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"invoiced":       map[string]any{"type": "boolean"},
		}})), "entry", "updated", s.EntriesMarkInvoiced),
		nativeDomainTool(1102, toolRW("clockify_users_invite", "Invite users by email. External side effect when send_email is true; supports dry_run.", objectSchema(map[string]any{"properties": map[string]any{
			"email":      map[string]any{"type": "string", "format": "email"},
			"emails":     map[string]any{"type": "array", "items": map[string]any{"type": "string", "format": "email"}},
			"dry_run":    map[string]any{"type": "boolean"},
			"send_email": map[string]any{"type": "boolean", "description": "Whether Clockify should send invitation email. Defaults to true."},
		}})), "user", "created", s.UsersInvite),
	)
	return out
}

func (s *Service) nativeRouteDescriptors() []mcp.ToolDescriptor {
	out := []mcp.ToolDescriptor{
		nativeDomainTool(39, toolRO("clockify_projects_memberships_list", "List project memberships from the hydrated project record.", objectSchema(map[string]any{
			"required":   []string{"project_id"},
			"properties": map[string]any{"project_id": map[string]any{"type": "string", "description": "Project ID"}},
		})), "membership", "", s.ListProjectMemberships),
	}
	out = append(out,
		nativeDomainTool(100, toolRO("clockify_reports_attendance", "Run the attendance report. Raw report amounts are in minor units (cents); meta.totalAmount gives normalized major-unit totals per currency.", reportInputSchema()), "report", "", s.AttendanceReport),
		nativeDomainTool(101, toolRO("clockify_reports_money", "Run money summary for billing-rate breakdowns by user or project. Use clockify_reports_summary for simple time totals. Raw amounts are minor units; meta.totalAmount gives major-unit totals.", reportInputSchema()), "report", "", s.MoneyReport),
		nativeDomainTool(102, toolRO("clockify_reports_expense", "Run the detailed expense report. Raw report amounts are in minor units (cents); meta.totalAmount gives normalized major-unit totals per currency.", reportInputSchema()), "report", "", s.ExpenseReport),
		nativeDomainTool(103, toolRO("clockify_reports_export", "Export a detailed report. JSON returns contentType, filename, bytes, bodyEncoding, body base64 payload, base64Bytes, truncated:false. CSV/PDF/XLSX/ZIP use bodyEncoding:\"file\" and path. Amounts are minor units.", reportInputSchema()), "report", "", s.DetailedReport),
	)
	out = append(out, s.explicitInvoiceNativeDescriptors()...)
	out = append(out, nativeDomainTool(506, toolRWIdem("clockify_time_off_archive", "Archive or reactivate a time off policy.", objectSchema(map[string]any{
		"required": []string{"policy_id", "archived"},
		"properties": map[string]any{
			"policy_id": map[string]any{"type": "string", "description": "Time-off policy ID"},
			"archived":  map[string]any{"type": "boolean", "description": "Required. Pass true to archive the policy; pass false to reactivate it."},
		},
	})), "time_off", "updated", s.archiveTimeOffPolicy))
	out = append(out, s.explicitPostTimeOffNativeDescriptors()...)
	return out
}

func (s *Service) explicitInvoiceNativeDescriptors() []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		nativeDomainTool(210, toolRO("clockify_invoices_export", "Export invoice PDF only; Clockify invoice export has no CSV/XLSX (use clockify_reports_export). Returns contentType, filename, bytes, bodyEncoding, body/base64 payload, base64Bytes, truncated:false, or path.", objectSchema(map[string]any{
			"required": []string{"invoice_id"},
			"properties": map[string]any{
				"invoice_id":  map[string]any{"type": "string", "description": "Invoice ID"},
				"format":      map[string]any{"type": "string", "enum": []string{"PDF"}, "description": "Invoice export format. Clockify only produces PDF here; use clockify_reports_export for CSV/XLSX data exports."},
				"user_locale": map[string]any{"type": "string", "description": "Locale for the exported document, e.g. en"},
			},
		})), "invoice_export", "", s.exportInvoiceOneUser),
		nativeDomainTool(211, toolRW("clockify_invoices_import_time", "Import a client's billable time entries onto an invoice. Clockify imports every billable time entry in the from..to date range; narrow it with project_ids.", objectSchema(map[string]any{
			"required": []string{"invoice_id", "from", "to"},
			"properties": map[string]any{
				"invoice_id":                  map[string]any{"type": "string", "description": "Invoice ID."},
				"from":                        map[string]any{"type": "string", "description": "Start of the billing period (YYYY-MM-DD or RFC3339). All billable time on or after this is imported."},
				"to":                          map[string]any{"type": "string", "description": "End of the billing period (YYYY-MM-DD or RFC3339)."},
				"project_ids":                 map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional. Limit the import to these project IDs; omit to import all of the client's billable time in the range."},
				"time_entry_group_type":       map[string]any{"type": "string", "enum": []string{"SINGLE_ITEM", "GROUPED", "DETAILED"}, "description": "How imported entries are grouped on the invoice: SINGLE_ITEM, GROUPED, or DETAILED (default). GROUPED also requires time_entry_primary_group_by."},
				"time_entry_primary_group_by": map[string]any{"type": "string", "enum": []string{"USER", "PROJECT", "DATE"}, "description": "Primary grouping dimension. Required when time_entry_group_type is GROUPED."},
			},
		})), "invoice_item", "updated", s.importInvoiceTimeOneUser),
		nativeDomainTool(212, toolRW("clockify_invoices_import_expenses", "Import a client's billable time and expenses onto an invoice for a date range. Clockify's import endpoint always imports time; this tool also imports billable expenses.", objectSchema(map[string]any{
			"required": []string{"invoice_id", "from", "to"},
			"properties": map[string]any{
				"invoice_id":                  map[string]any{"type": "string", "description": "Invoice ID."},
				"from":                        map[string]any{"type": "string", "description": "Start of the billing period (YYYY-MM-DD or RFC3339)."},
				"to":                          map[string]any{"type": "string", "description": "End of the billing period (YYYY-MM-DD or RFC3339)."},
				"project_ids":                 map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional. Limit the import to these project IDs; omit to import all of the client's billable items in the range."},
				"time_entry_group_type":       map[string]any{"type": "string", "enum": []string{"SINGLE_ITEM", "GROUPED", "DETAILED"}, "description": "How imported entries are grouped on the invoice: SINGLE_ITEM, GROUPED, or DETAILED (default). GROUPED also requires time_entry_primary_group_by."},
				"time_entry_primary_group_by": map[string]any{"type": "string", "enum": []string{"USER", "PROJECT", "DATE"}, "description": "Primary grouping dimension. Required when time_entry_group_type is GROUPED."},
			},
		})), "invoice_item", "updated", s.importInvoiceExpensesOneUser),
		nativeDomainTool(213, toolRO("clockify_invoices_payments_list", "List invoice payments.", objectSchema(map[string]any{
			"required": []string{"invoice_id"},
			"properties": map[string]any{
				"invoice_id": map[string]any{"type": "string", "description": "Invoice ID"},
				"page":       map[string]any{"type": "integer"},
				"page_size":  map[string]any{"type": "integer"},
			},
		})), "payment", "", s.listInvoicePayments),
		nativeDomainTool(214, toolRW("clockify_invoices_payments_create", "Create an invoice payment. amount defaults to minor units (cents), matching the live AddInvoicePaymentRequest body; pass amount_unit:\"major\" to enter the value in major currency units instead.", objectSchema(map[string]any{
			"required": []string{"invoice_id", "amount", "date"},
			"properties": map[string]any{
				"invoice_id":  map[string]any{"type": "string", "description": "Invoice ID"},
				"amount":      map[string]any{"type": "number", "description": "Payment amount"},
				"amount_unit": invoiceMinorUnitSchema("amount"),
				"date":        map[string]any{"type": "string", "description": "Payment date"},
				"note":        map[string]any{"type": "string", "description": "Optional payment note"},
			},
		})), "payment", "created", s.createInvoicePaymentOneUser),
		nativeDomainTool(215, toolDestructive("clockify_invoices_payments_delete", "Permanently delete an invoice payment. Billing impact; destructive; supports dry_run preview.", objectSchema(map[string]any{
			"required": []string{"invoice_id", "payment_id"},
			"properties": map[string]any{
				"invoice_id": map[string]any{"type": "string"},
				"payment_id": map[string]any{"type": "string"},
				"dry_run":    map[string]any{"type": "boolean", "description": "If true, preview the deletion without executing it."},
			},
		})), "payment", "deleted", s.deleteInvoicePaymentOneUser),
	}
}

func (s *Service) explicitPostTimeOffNativeDescriptors() []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		nativeDomainTool(606, toolRO("clockify_scheduling_user_totals", "Get scheduled assignment totals for one user.", objectSchema(map[string]any{
			"required": []string{"user_id"},
			"properties": map[string]any{
				"user_id": map[string]any{"type": "string", "description": "User ID"},
				"start":   map[string]any{"type": "string", "description": flexibleDatetimeDescription},
				"end":     map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			},
		})), "scheduling", "", s.schedulingUserTotalsOneUser),
		nativeDomainTool(607, toolRO("clockify_scheduling_capacity", "Get workspace capacity totals. Defaults to every workspace user; pass user_ids to scope to specific users.", objectSchema(map[string]any{
			"properties": map[string]any{
				"start":    map[string]any{"type": "string", "description": flexibleDatetimeDescription},
				"end":      map[string]any{"type": "string", "description": flexibleDatetimeDescription},
				"user_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional. User IDs to scope the capacity totals; omit to include all workspace users."},
			},
		})), "scheduling", "", s.schedulingCapacityOneUser),
		nativeDomainTool(704, toolRW("clockify_approvals_resubmit", "Resubmit rejected or withdrawn entries and expenses and update approval state.", objectSchema(map[string]any{
			"required": []string{"approval_id", "entry_ids", "period", "period_start"},
			"properties": map[string]any{
				"approval_id":  map[string]any{"type": "string", "description": "Approval request ID"},
				"entry_ids":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Time entry IDs to resubmit"},
				"expense_ids":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Expense IDs to resubmit"},
				"period":       map[string]any{"type": "string", "enum": []string{"WEEKLY", "SEMI_MONTHLY", "MONTHLY"}, "description": "Approval period; must match workspace approval settings"},
				"period_start": map[string]any{"type": "string", "description": "Period start in RFC3339/millisecond form, e.g. 2026-05-01T00:00:00.000Z"},
				"note":         map[string]any{"type": "string", "description": "Optional resubmission note"},
			},
		})), "approval", "updated", s.resubmitApprovalOneUser),
		nativeDomainTool(1001, toolRO("clockify_holidays_get", "Get one holiday.", objectSchema(map[string]any{
			"required":   []string{"holiday_id"},
			"properties": map[string]any{"holiday_id": map[string]any{"type": "string"}},
		})), "holiday", "", s.GetHoliday),
		nativeDomainTool(1002, toolRWIdem("clockify_holidays_update", "Update a holiday.", objectSchema(map[string]any{
			"required": []string{"holiday_id"},
			"properties": map[string]any{
				"holiday_id":      map[string]any{"type": "string", "description": "Holiday ID"},
				"name":            map[string]any{"type": "string", "description": "Holiday name"},
				"start_date":      map[string]any{"type": "string", "description": "Holiday start date (YYYY-MM-DD)"},
				"end_date":        map[string]any{"type": "string", "description": "Holiday end date (YYYY-MM-DD)"},
				"occurs_annually": map[string]any{"type": "boolean", "description": "Whether the holiday repeats every year"},
				"user_ids":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "User IDs the holiday applies to"},
				"user_group_ids":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "User group IDs the holiday applies to"},
			},
		})), "holiday", "updated", s.UpdateHoliday),
	}
}

//nolint:unused // Retained for route-based native descriptors; the registry is currently fully native.
func (s *Service) nativeRouteDescriptor(spec routeTool) mcp.ToolDescriptor {
	descriptor := s.routeDescriptor(spec)
	descriptor.Tool.Annotations["handlerKind"] = "native handler"
	return descriptor
}

func (s *Service) updateProjectMembershipsOneUser(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	if _, ok := args["memberships"]; ok {
		return s.UpdateProjectMemberships(ctx, args)
	}
	if _, ok := args["user_ids"]; ok {
		return s.SetProjectMemberships(ctx, args)
	}
	return s.UpdateProjectMemberships(ctx, args)
}

func nativeDirectDescriptor(priority int, name string, old mcp.ToolDescriptor, entity, change string, handler func(context.Context, map[string]any) (ResultEnvelope, error)) mcp.ToolDescriptor {
	tool := old.Tool
	tool.Name = name
	adjustOneUserNativeSchema(name, tool.InputSchema)
	if tool.Annotations == nil {
		tool.Annotations = map[string]any{}
	}
	delete(tool.Annotations, "wraps")
	tool.Annotations["handlerKind"] = "native handler"
	return firstSliceDescriptor(priority, tool, aliasHandler(name, entity, change, handler))
}

func adjustOneUserNativeSchema(name string, schema map[string]any) {
	if schema == nil {
		return
	}
	switch name {
	case "clockify_groups_update":
		schema["required"] = []string{"group_id", "name"}
	case "clockify_projects_memberships_update":
		props, _ := schema["properties"].(map[string]any)
		if props == nil {
			props = map[string]any{}
			schema["properties"] = props
		}
		props["user_ids"] = stringArraySchema("User IDs to set as members")
		props["hourly_rate"] = map[string]any{"type": "number", "description": "Hourly rate for all members when using user_ids compatibility input"}
		schema["required"] = []string{"project_id"}
		// The "memberships OR user_ids" rule is enforced by the routed
		// handler (Update/SetProjectMemberships). It is intentionally not a
		// top-level anyOf: Anthropic tool input schemas must be a plain
		// object, and a sibling anyOf breaks subagent launches.
	}
}

func (s *Service) nativeDomainDescriptorMap() map[string]mcp.ToolDescriptor {
	out := map[string]mcp.ToolDescriptor{}
	add := func(descriptors []mcp.ToolDescriptor) {
		descriptors = normalizeDescriptors(descriptors)
		for _, descriptor := range descriptors {
			out[descriptor.Tool.Name] = descriptor
		}
	}
	add([]mcp.ToolDescriptor{
		{Tool: toolRO("clockify_users_list", "List users in the pinned workspace.", userListSchema()), Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ListUsers(ctx, args)
		}},
		{Tool: toolRO("clockify_users_profile", "Get the current Clockify user.", map[string]any{"type": "object"}), Handler: func(ctx context.Context, _ map[string]any) (any, error) {
			return s.CurrentUser(ctx)
		}},
	})
	add(invoiceHandlers(s))
	add(expenseHandlers(s))
	add(customFieldHandlers(s))
	add(timeOffHandlers(s))
	add(schedulingHandlers(s))
	add(approvalHandlers(s))
	add(webhookHandlers(s))
	add(groupsHolidaysHandlers(s))
	add(projectAdminHandlers(s))
	add(userAdminHandlers(s))
	return out
}

func nativeDomainTool(priority int, tool mcp.Tool, entity, change string, handler func(context.Context, map[string]any) (ResultEnvelope, error)) mcp.ToolDescriptor {
	if tool.Annotations == nil {
		tool.Annotations = map[string]any{}
	}
	tool.Annotations["handlerKind"] = "native handler"
	return firstSliceDescriptor(priority, tool, aliasHandler(tool.Name, entity, change, handler))
}

func rateKindEndpoint(args map[string]any) (string, error) {
	switch strings.ToLower(strings.TrimSpace(stringArg(args, "rate_kind"))) {
	case "hourly":
		return "hourly-rate", nil
	case "cost":
		return "cost-rate", nil
	default:
		return "", fmt.Errorf("rate_kind must be hourly or cost")
	}
}

func aliasArg(args map[string]any, from, to string) map[string]any {
	return aliasArgs(args, map[string]string{from: to})
}

func aliasArgs(args map[string]any, aliases map[string]string) map[string]any {
	out := make(map[string]any, len(args)+len(aliases))
	for key, value := range args {
		out[key] = value
	}
	for from, to := range aliases {
		if _, ok := out[to]; ok {
			continue
		}
		if value, ok := out[from]; ok {
			out[to] = value
		}
	}
	return out
}

func aliasHandler(action, entity, change string, handler func(context.Context, map[string]any) (ResultEnvelope, error)) mcp.ToolHandler {
	return func(ctx context.Context, args map[string]any) (any, error) {
		out, err := handler(ctx, args)
		if err != nil {
			return nil, err
		}
		return standardizeDomainResult(action, entity, change, out, args), nil
	}
}

//nolint:unused // Retained for route-based native descriptors; the registry is currently fully native.
func (s *Service) routeDescriptor(spec routeTool) mcp.ToolDescriptor {
	schema := objectSchema(map[string]any{
		"required":   spec.Required,
		"properties": spec.Properties,
	})
	var tool mcp.Tool
	switch {
	case spec.Destructive:
		tool = toolDestructive(spec.Name, spec.Description, schema)
	case spec.ReadOnly:
		tool = toolRO(spec.Name, spec.Description, schema)
	case spec.Idempotent:
		tool = toolRWIdem(spec.Name, spec.Description, schema)
	default:
		tool = toolRW(spec.Name, spec.Description, schema)
	}
	if tool.Annotations == nil {
		tool.Annotations = map[string]any{}
	}
	tool.Annotations["handlerKind"] = "route descriptor"
	tool.Annotations["method"] = spec.Method
	tool.Annotations["path"] = "/workspaces/{workspaceId}/" + spec.Path
	return firstSliceDescriptor(spec.Priority, tool, func(ctx context.Context, args map[string]any) (any, error) {
		return s.callRouteTool(ctx, spec, args)
	})
}

//nolint:unused // Retained for route-based native descriptors; the registry is currently fully native.
func (s *Service) callRouteTool(ctx context.Context, spec routeTool, args map[string]any) (any, error) {
	for _, key := range spec.Required {
		if _, ok := args[key]; !ok {
			return nil, fmt.Errorf("%s is required", key)
		}
		if value, ok := args[key].(string); ok && strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("%s is required", key)
		}
	}
	path, ids, err := routePath(s.WorkspaceID, spec, args)
	if err != nil {
		return nil, err
	}
	query := routeQuery(spec, args)
	body := routeBody(spec, args)
	var data any
	switch strings.ToUpper(spec.Method) {
	case "GET":
		if spec.Reports {
			err = s.Client.GetReports(ctx, path, query, &data)
		} else {
			err = s.Client.Get(ctx, path, query, &data)
		}
	case "POST":
		if spec.Reports {
			err = s.Client.PostReports(ctx, path, body, &data)
		} else {
			err = s.Client.Post(ctx, path, body, &data)
		}
	case "PUT":
		if spec.Reports {
			err = s.Client.PutReports(ctx, path, body, &data)
		} else {
			err = s.Client.Put(ctx, path, body, &data)
		}
	case "PATCH":
		err = s.Client.Patch(ctx, path, body, &data)
	case "DELETE":
		if spec.Reports {
			err = s.Client.DeleteReports(ctx, path)
		} else {
			err = s.Client.Delete(ctx, path)
		}
		data = map[string]any{"deleted": true}
	default:
		return nil, fmt.Errorf("unsupported method %s", spec.Method)
	}
	if err != nil {
		return nil, err
	}
	for key, value := range idsFromData(data, spec.Entity) {
		ids[key] = value
	}
	changed := changedFor(spec.Change, spec.Entity, data, ids)
	return result(spec.Name, spec.Entity, ids, data, changed, nil, nil), nil
}

func (s *Service) EntriesMarkInvoiced(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	entryIDs, found, err := strictStringSliceArg(args, "time_entry_ids")
	if err != nil {
		return ResultEnvelope{}, err
	}
	if !found || len(entryIDs) == 0 {
		return ResultEnvelope{}, fmt.Errorf("time_entry_ids is required")
	}
	invoiced, found := optionalBoolArg(args, "invoiced")
	if !found {
		return ResultEnvelope{}, fmt.Errorf("invoiced is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "time-entries", "invoiced")
	if err != nil {
		return ResultEnvelope{}, err
	}
	body := map[string]any{"timeEntryIds": entryIDs, "invoiced": invoiced}
	var upstream map[string]any
	if err := s.Client.Patch(ctx, path, body, &upstream); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_entries_mark_invoiced", map[string]any{
		"updated":       true,
		"timeEntryIds":  entryIDs,
		"invoiced":      invoiced,
		"upstreamReply": upstream,
	}, map[string]any{"workspaceId": wsID, "entryId": entryIDs[0]}), nil
}

func (s *Service) UsersInvite(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	emails, found, err := strictStringSliceArg(args, "emails")
	if err != nil {
		return ResultEnvelope{}, err
	}
	if !found {
		if email := strings.TrimSpace(stringArg(args, "email")); email != "" {
			emails = []string{email}
		}
	}
	if len(emails) == 0 {
		return ResultEnvelope{}, fmt.Errorf("email or emails is required")
	}
	baseArgs := map[string]any{}
	if sendEmail, ok := optionalBoolArg(args, "send_email"); ok {
		baseArgs["send_email"] = sendEmail
	}
	if len(emails) == 1 {
		baseArgs["email"] = emails[0]
		out, err := s.InviteUser(ctx, baseArgs)
		if err != nil {
			return ResultEnvelope{}, err
		}
		out.Action = "clockify_users_invite"
		return out, nil
	}
	var wsID string
	invitations := make([]any, 0, len(emails))
	for _, email := range emails {
		nextArgs := make(map[string]any, len(baseArgs)+1)
		for key, value := range baseArgs {
			nextArgs[key] = value
		}
		nextArgs["email"] = email
		out, err := s.InviteUser(ctx, nextArgs)
		if err != nil {
			return ResultEnvelope{}, err
		}
		if id, _ := out.Meta["workspaceId"].(string); id != "" {
			wsID = id
		}
		invitations = append(invitations, out.Data)
	}
	return ok("clockify_users_invite", map[string]any{
		"invitations": invitations,
		"count":       len(invitations),
	}, map[string]any{"workspaceId": wsID}), nil
}

//nolint:unused // Retained for route-based native descriptors; the registry is currently fully native.
func routePath(workspaceID string, spec routeTool, args map[string]any) (string, map[string]string, error) {
	ids := map[string]string{"workspaceId": workspaceID}
	segments := strings.Split(strings.Trim(spec.Path, "/"), "/")
	for i, segment := range segments {
		resolved, segmentIDs, err := resolveRouteSegment(segment, args)
		if err != nil {
			return "", nil, err
		}
		segments[i] = resolved
		for key, value := range segmentIDs {
			ids[key] = value
		}
	}
	if spec.Reports {
		return "/workspaces/" + workspaceID + "/" + strings.Join(segments, "/"), ids, nil
	}
	path, err := paths.Workspace(workspaceID, segments...)
	return path, ids, err
}

//nolint:unused // Retained for route-based native descriptors; the registry is currently fully native.
func resolveRouteSegment(segment string, args map[string]any) (string, map[string]string, error) {
	ids := map[string]string{}
	for {
		start := strings.IndexByte(segment, '{')
		if start < 0 {
			return segment, ids, nil
		}
		end := strings.IndexByte(segment[start:], '}')
		if end < 0 {
			return "", nil, fmt.Errorf("unclosed route parameter in %q", segment)
		}
		end += start
		key := segment[start+1 : end]
		value := strings.TrimSpace(stringArg(args, key))
		if value == "" {
			return "", nil, fmt.Errorf("%s is required", key)
		}
		if strings.HasSuffix(key, "_id") || strings.HasSuffix(key, "Id") || key == "id" {
			if err := resolve.ValidateID(value, key); err != nil {
				return "", nil, err
			}
		}
		if key == "rate_kind" {
			value = strings.ToLower(value)
			if value != "hourly" && value != "cost" {
				return "", nil, fmt.Errorf("rate_kind must be hourly or cost")
			}
		}
		segment = segment[:start] + value + segment[end+1:]
		ids[idKey(key)] = value
	}
}

func routeQuery(spec routeTool, args map[string]any) map[string]string {
	query := map[string]string{}
	for _, key := range spec.Query {
		if value, ok := spec.Defaults[key]; ok {
			query[wireQueryName(key)] = fmt.Sprint(value)
		}
		if raw, ok := args[key]; ok {
			query[wireQueryName(key)] = scalarToString(raw)
		}
	}
	if len(query) == 0 {
		return nil
	}
	return query
}

func routeBody(spec routeTool, args map[string]any) map[string]any {
	body := map[string]any{}
	if raw, ok := args["body"].(map[string]any); ok {
		for key, value := range raw {
			body[key] = value
		}
	}
	for key, value := range spec.Defaults {
		body[bodyName(key)] = value
	}
	for _, key := range spec.Body {
		if key == "body" {
			continue
		}
		if value, ok := args[key]; ok {
			wireKey := routeBodyName(spec, key)
			if spec.Reports && (key == "start" || key == "end") {
				if _, exists := body[wireKey]; exists {
					continue
				}
			}
			body[wireKey] = value
		}
	}
	if len(body) == 0 {
		return nil
	}
	return body
}

func routeBodyName(spec routeTool, key string) string {
	if spec.Reports {
		switch key {
		case "start":
			return "dateRangeStart"
		case "end":
			return "dateRangeEnd"
		}
	}
	return bodyName(key)
}

func requirePresentArgs(args map[string]any, keys ...string) error {
	for _, key := range keys {
		value, ok := args[key]
		if !ok {
			return fmt.Errorf("%s is required", key)
		}
		if str, ok := value.(string); ok && strings.TrimSpace(str) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	return nil
}

func requiredIDArg(args map[string]any, key string) (string, error) {
	value := strings.TrimSpace(stringArg(args, key))
	if value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	if err := resolve.ValidateID(value, key); err != nil {
		return "", err
	}
	return value, nil
}

func nativeBodyFromArgs(args map[string]any, keys ...string) map[string]any {
	body := map[string]any{}
	if raw, ok := args["body"].(map[string]any); ok {
		for key, value := range raw {
			body[key] = value
		}
	}
	for _, key := range keys {
		if key == "body" {
			continue
		}
		if value, ok := args[key]; ok {
			body[bodyName(key)] = value
		}
	}
	if len(body) == 0 {
		return nil
	}
	return body
}

func standardizeDomainResult(action, entity, change string, out any, args map[string]any) ToolResult {
	if current, ok := out.(ToolResult); ok {
		current.Action = action
		current.Data = sanitizeResultValue(current.Data)
		current.Meta = sanitizeResultMeta(current.Meta)
		if dryRunResult(args, current.Data) {
			current.Changed = ChangeSet{}
		}
		return current
	}
	ids := map[string]string{}
	data := out
	var meta map[string]any
	var warnings []Warning
	if env, ok := out.(ResultEnvelope); ok {
		data = env.Data
		if len(env.Meta) > 0 {
			meta = env.Meta
		}
		for key, value := range env.Meta {
			if str, ok := value.(string); ok && str != "" {
				ids[key] = str
			}
		}
	}
	for key, value := range args {
		if strings.HasSuffix(key, "_id") {
			if str, ok := value.(string); ok && str != "" {
				ids[idKey(key)] = str
			}
		}
	}
	for key, value := range idsFromData(data, entity) {
		ids[key] = value
	}
	changed := ChangeSet{}
	if !dryRunResult(args, data) {
		changed = changedFor(change, entity, data, ids)
	}
	return result(action, entity, ids, data, changed, warnings, nil, meta)
}

func dryRunResult(args map[string]any, data any) bool {
	if boolArg(args, "dry_run") {
		return true
	}
	if m, ok := data.(map[string]any); ok {
		if dry, _ := m["dry_run"].(bool); dry {
			return true
		}
	}
	return false
}

func changedFor(change, entity string, data any, ids map[string]string) ChangeSet {
	if change == "" {
		return ChangeSet{}
	}
	ref := EntityRef{Type: entity, ID: firstEntityID(entity, ids), Name: entityName(data)}
	switch change {
	case "created":
		return ChangeSet{Created: []EntityRef{ref}}
	case "updated":
		return ChangeSet{Updated: []EntityRef{ref}}
	case "deleted":
		return ChangeSet{Deleted: []EntityRef{ref}}
	case "reused":
		return ChangeSet{Reused: []EntityRef{ref}}
	default:
		return ChangeSet{}
	}
}

func idsFromData(data any, entity string) map[string]string {
	out := map[string]string{}
	switch m := data.(type) {
	case clockify.TimeEntry:
		addEntryIDs(out, m.ID, m.WorkspaceID, m.UserID, m.ProjectID, m.TaskID)
	case clockify.User:
		if m.ID != "" {
			out["userId"] = m.ID
		}
	case UserView:
		if m.ID != "" {
			out["userId"] = m.ID
		}
		if m.ActiveWorkspace != "" {
			out["workspaceId"] = m.ActiveWorkspace
		}
	case WorkspaceView:
		if m.ID != "" {
			out["workspaceId"] = m.ID
		}
	case EntryView:
		addEntryIDs(out, m.ID, m.WorkspaceID, m.UserID, m.ProjectID, m.TaskID)
	case map[string]any:
		addEntityIDFromMap(out, entity, m)
	case InvoiceView:
		addEntityIDFromMap(out, entity, map[string]any(m))
	case InvoiceItemView:
		addEntityIDFromMap(out, entity, map[string]any(m))
	case InvoicePaymentView:
		addEntityIDFromMap(out, entity, map[string]any(m))
	case InvoiceSettingsView:
		addEntityIDFromMap(out, entity, map[string]any(m))
	case ExpenseView:
		addEntityIDFromMap(out, entity, map[string]any(m))
	case TimeOffPolicyView:
		addEntityIDFromMap(out, entity, map[string]any(m))
	case TimeOffRequestView:
		addEntityIDFromMap(out, entity, map[string]any(m))
	case TimeOffBalanceView:
		addEntityIDFromMap(out, entity, map[string]any(m))
	case AssignmentView:
		addEntityIDFromMap(out, entity, map[string]any(m))
	case ApprovalView:
		if m.ID != "" {
			out["approvalId"] = m.ID
		}
	case []ApprovalView:
		for _, item := range m {
			if item.ID != "" {
				out["approvalId"] = item.ID
				break
			}
		}
	case WebhookLogView:
		addEntityIDFromMap(out, entity, map[string]any(m))
	case []map[string]any:
		for _, item := range m {
			if id := oneUserStringFromMap(item, "id", "_id"); id != "" {
				out[idKey(entity+"_id")] = id
				break
			}
		}
	case []AssignmentView:
		for _, item := range m {
			if id := oneUserStringFromMap(map[string]any(item), "id", "_id"); id != "" {
				out[idKey(entity+"_id")] = id
				break
			}
		}
	case []any:
		for _, item := range m {
			if asMap, ok := item.(map[string]any); ok {
				if id := oneUserStringFromMap(asMap, "id", "_id"); id != "" {
					out[idKey(entity+"_id")] = id
					break
				}
			}
		}
	}
	return out
}

func addEntityIDFromMap(out map[string]string, entity string, m map[string]any) {
	for _, key := range []string{
		"workspaceId",
		"userId",
		"clientId",
		"projectId",
		"taskId",
		"tagId",
		"entryId",
		"invoiceId",
		"invoiceItemId",
		"paymentId",
		"expenseId",
		"categoryId",
		"customFieldId",
		"policyId",
		"requestId",
		"assignmentId",
		"approvalId",
		"webhookId",
		"groupId",
		"holidayId",
	} {
		if value := oneUserStringFromMap(m, key, snakeIDKey(key)); value != "" {
			out[key] = value
		}
	}
	if id := oneUserStringFromMap(m, "id", "_id"); id != "" {
		out[idKey(entity+"_id")] = id
	}
}

func snakeIDKey(key string) string {
	var b strings.Builder
	for i, r := range key {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

func addEntryIDs(out map[string]string, entryID, workspaceID, userID, projectID, taskID string) {
	if entryID != "" {
		out["entryId"] = entryID
	}
	if workspaceID != "" {
		out["workspaceId"] = workspaceID
	}
	if userID != "" {
		out["userId"] = userID
	}
	if projectID != "" {
		out["projectId"] = projectID
	}
	if taskID != "" {
		out["taskId"] = taskID
	}
}

func firstEntityID(entity string, ids map[string]string) string {
	preferred := idKey(entity + "_id")
	if ids[preferred] != "" {
		return ids[preferred]
	}
	keys := make([]string, 0, len(ids))
	for key := range ids {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if strings.HasSuffix(strings.ToLower(key), "id") && key != "workspaceId" {
			return ids[key]
		}
	}
	return ids["workspaceId"]
}

func entityName(data any) string {
	switch v := data.(type) {
	case clockify.TimeEntry:
		return v.Description
	case EntryView:
		return v.Description
	case map[string]any:
		return oneUserStringFromMap(v, "name", "description", "number")
	case InvoiceView:
		return oneUserStringFromMap(map[string]any(v), "name", "description", "number")
	case InvoiceItemView:
		return oneUserStringFromMap(map[string]any(v), "name", "description", "number")
	case InvoicePaymentView:
		return oneUserStringFromMap(map[string]any(v), "name", "description", "number")
	case ExpenseView:
		return oneUserStringFromMap(map[string]any(v), "name", "description", "number")
	case TimeOffPolicyView:
		return oneUserStringFromMap(map[string]any(v), "name", "description", "number")
	case TimeOffRequestView:
		return oneUserStringFromMap(map[string]any(v), "name", "description", "number")
	case AssignmentView:
		return oneUserStringFromMap(map[string]any(v), "name", "description", "number")
	}
	return ""
}

func oneUserStringFromMap(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := m[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func idKey(name string) string {
	name = strings.TrimSuffix(name, "_id")
	name = strings.TrimSuffix(name, "Id")
	parts := strings.Split(name, "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "") + "Id"
}

func bodyName(name string) string {
	switch name {
	case "client_id":
		return "clientId"
	case "project_id":
		return "projectId"
	case "task_id":
		return "taskId"
	case "tag_ids":
		return "tagIds"
	case "time_entry_ids":
		return "timeEntryIds"
	case "expense_ids":
		return "expenseIds"
	case "user_ids":
		return "userIds"
	case "user_group_ids":
		return "userGroupIds"
	case "entry_ids":
		return "entryIds"
	case "approval_id":
		return "approvalRequestId"
	case "time_entry_group_type":
		return "timeEntryGroupType"
	case "is_public":
		return "isPublic"
	case "send_email":
		return "sendEmail"
	default:
		parts := strings.Split(name, "_")
		for i := 1; i < len(parts); i++ {
			if parts[i] != "" {
				parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
			}
		}
		return strings.Join(parts, "")
	}
}

func wireQueryName(name string) string {
	switch name {
	case "page_size":
		return "page-size"
	case "is_template":
		return "is-template"
	case "send_email":
		return "send-email"
	case "user_ids":
		return "user-ids"
	default:
		return strings.ReplaceAll(name, "_", "-")
	}
}

func scalarToString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, scalarToString(item))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprint(value)
	}
}

// rawBodyField is the fenced raw-passthrough escape hatch. A few tools
// (invoice imports, report export) accept Clockify request fields this MCP
// does not model individually; they carry this field. Tools that fully
// model their input do not — the raw API tools are the general escape
// hatch. Each caller gets a fresh map so tightenInputSchema cannot mutate
// a shared instance.
func rawBodyField() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": true, "description": "Optional raw Clockify request body fields merged with the typed fields above."}
}

func reportHelperSchema() map[string]any {
	return objectSchema(map[string]any{"properties": map[string]any{
		"start":           map[string]any{"type": "string", "description": flexibleDatetimeDescription},
		"end":             map[string]any{"type": "string", "description": flexibleDatetimeDescription},
		"project":         map[string]any{"type": "string"},
		"include_entries": map[string]any{"type": "boolean"},
		"max_entries":     map[string]any{"type": "integer", "minimum": 0, "description": "Advisory only. The Clockify reports API returns a fixed page size; narrow start/end to get fewer rows. meta.truncated reports when the row cap was hit."},
		"max_rollups":     map[string]any{"type": "integer", "minimum": 0, "description": "Summary/weekly reports: keep only the N largest project rollups by tracked time (default 15). Aggregates still cover every rollup. Pass 0 for the full, unbounded list."},
		"week_start":      map[string]any{"type": "string", "description": "YYYY-MM-DD or RFC3339 week start."},
		"timezone":        map[string]any{"type": "string"},
	}})
}

func rt(priority int, method, name, desc, path, entity string, required []string, props map[string]any, query, body []string, readOnly, idempotent, destructive bool, change string) routeTool {
	if props == nil {
		props = map[string]any{}
	}
	for _, key := range required {
		if _, ok := props[key]; !ok {
			props[key] = map[string]any{"type": "string"}
		}
	}
	return routeTool{
		Name:        name,
		Description: desc,
		Method:      method,
		Path:        path,
		Entity:      entity,
		Required:    required,
		Properties:  props,
		Query:       query,
		Body:        body,
		ReadOnly:    readOnly,
		Idempotent:  idempotent,
		Destructive: destructive,
		Change:      change,
		Priority:    priority,
	}
}

func reportRT(priority int, name, desc, reportPath string) routeTool {
	props := map[string]any{
		"date_range_start": map[string]any{"type": "string", "description": "Report range start"},
		"date_range_end":   map[string]any{"type": "string", "description": "Report range end"},
		"start":            map[string]any{"type": "string", "description": flexibleDatetimeDescription},
		"end":              map[string]any{"type": "string", "description": flexibleDatetimeDescription},
		"export_type":      map[string]any{"type": "string", "enum": []string{"JSON", "JSON_V1", "PDF", "CSV", "XLSX", "ZIP"}, "description": "Report export type"},
		"body":             rawBodyField(),
	}
	return rt(priority, "POST", name, desc, reportPath, "report", nil, props, nil, []string{"date_range_start", "date_range_end", "start", "end", "export_type"}, true, false, false, "").withReports()
}

func (r routeTool) withReports() routeTool {
	r.Reports = true
	return r
}

func (s *Service) EntriesRunning(ctx context.Context, _ map[string]any) (any, error) {
	timer, err := s.currentTimer(ctx)
	if err != nil {
		return nil, err
	}
	return result("clockify_entries_running", "entry", map[string]string{"workspaceId": s.WorkspaceID}, timer, ChangeSet{}, nil, nil), nil
}

func (s *Service) EntriesTimerStart(ctx context.Context, args map[string]any) (any, error) {
	out, err := s.StartTimerArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	return standardizeDomainResult("clockify_entries_timer_start", "entry", "created", out, args), nil
}

func (s *Service) EntriesTimerStop(ctx context.Context, args map[string]any) (any, error) {
	out, err := s.StopTimer(ctx, args)
	if err != nil {
		return nil, err
	}
	if env, ok := out.(ResultEnvelope); ok {
		if data, ok := env.Data.(map[string]any); ok && !boolFromAny(data["stopped"]) && strings.TrimSpace(reportValueString(data["reason"])) == "no timer running" {
			ids := map[string]string{}
			for key, value := range env.Meta {
				if str, ok := value.(string); ok && str != "" {
					ids[key] = str
				}
			}
			return result("clockify_entries_timer_stop", "entry", ids, data, ChangeSet{}, nil, nil, env.Meta), nil
		}
	}
	return standardizeDomainResult("clockify_entries_timer_stop", "entry", "updated", out, args), nil
}

func (s *Service) EntriesTimerStatus(ctx context.Context, _ map[string]any) (any, error) {
	out, err := s.TimerStatus(ctx)
	if err != nil {
		return nil, err
	}
	return standardizeDomainResult("clockify_entries_timer_status", "entry", "", out, nil), nil
}

func (s *Service) EntriesTimerSwitch(ctx context.Context, args map[string]any) (any, error) {
	out, err := s.SwitchProject(ctx, args)
	if err != nil {
		return nil, err
	}
	return standardizeDomainResult("clockify_entries_timer_switch", "entry", "updated", out, args), nil
}

func (s *Service) rawAPIDescriptors() []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		firstSliceDescriptor(9000, toolRO("clockify_api_get", "Raw GET fallback within the pinned workspace or Clockify API path.", objectSchema(map[string]any{"required": []string{"path"}, "properties": map[string]any{
			"path":  map[string]any{"type": "string"},
			"query": map[string]any{"type": "object", "additionalProperties": true},
		}})), s.RawAPIGet),
		firstSliceDescriptor(9001, toolRW("clockify_api_request", "Raw method fallback within the pinned workspace or Clockify API path.", objectSchema(map[string]any{"required": []string{"method", "path"}, "properties": map[string]any{
			"method": map[string]any{"type": "string", "enum": []string{"GET", "POST", "PUT", "PATCH", "DELETE"}},
			"path":   map[string]any{"type": "string"},
			"query":  map[string]any{"type": "object", "additionalProperties": true},
			"body":   map[string]any{"type": "object", "additionalProperties": true},
		}})), s.RawAPIRequest),
	}
}

func (s *Service) RawAPIGet(ctx context.Context, args map[string]any) (any, error) {
	return s.rawAPI(ctx, "GET", args)
}

func (s *Service) RawAPIRequest(ctx context.Context, args map[string]any) (any, error) {
	method := strings.ToUpper(strings.TrimSpace(stringArg(args, "method")))
	if method == "" {
		return nil, fmt.Errorf("method is required")
	}
	return s.rawAPI(ctx, method, args)
}

func (s *Service) rawAPI(ctx context.Context, method string, args map[string]any) (any, error) {
	if method != "GET" && (s == nil || !s.EnableRawWrites) {
		return nil, fmt.Errorf("raw API writes are disabled; use workflow or domain tools first, or set CLOCKIFY_ENABLE_RAW_WRITES=true to allow %s", method)
	}
	path, err := safeRawPath(s.WorkspaceID, stringArg(args, "path"))
	if err != nil {
		return nil, err
	}
	query := rawQuery(args["query"])
	body, _ := args["body"].(map[string]any)
	var data any
	switch method {
	case "GET":
		err = s.Client.Get(ctx, path, query, &data)
	case "POST":
		err = s.Client.Post(ctx, path, body, &data)
	case "PUT":
		err = s.Client.Put(ctx, path, body, &data)
	case "PATCH":
		err = s.Client.Patch(ctx, path, body, &data)
	case "DELETE":
		err = s.Client.DeleteWithQuery(ctx, path, query)
		data = map[string]any{"deleted": true}
	default:
		return nil, fmt.Errorf("unsupported method %s", method)
	}
	if err != nil {
		return nil, err
	}
	action := "clockify_api_request"
	if method == "GET" {
		action = "clockify_api_get"
	}
	ids := map[string]string{"workspaceId": s.WorkspaceID}
	for key, value := range idsFromData(data, "raw_api") {
		ids[key] = value
	}
	return result(action, "raw_api", ids, data, changedFor(rawChange(method), "raw_api", data, ids), nil, nil), nil
}

func safeRawPath(workspaceID, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("path is required")
	}
	raw = strings.ReplaceAll(raw, "{workspaceId}", workspaceID)
	raw = strings.TrimPrefix(raw, "/api/v1")
	raw = strings.TrimPrefix(raw, "/v1")
	if strings.Contains(raw, "://") || strings.Contains(raw, "\\") || strings.Contains(raw, "..") || strings.ContainsAny(raw, "?#") {
		return "", fmt.Errorf("raw API path must be a relative Clockify API path")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.IsAbs() || u.Host != "" {
		return "", fmt.Errorf("raw API path must not include a host")
	}
	path, err := url.PathUnescape(u.EscapedPath())
	if err != nil {
		return "", fmt.Errorf("raw API path must be a valid escaped path: %w", err)
	}
	if strings.Contains(path, "\\") {
		return "", fmt.Errorf("raw API path must be a relative Clockify API path")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	for i, segment := range strings.Split(path, "/") {
		if i > 0 && segment == "" {
			return "", fmt.Errorf("raw API path must not contain duplicated slashes")
		}
		if segment == "." || segment == ".." {
			return "", fmt.Errorf("raw API path must not contain dot segments")
		}
	}
	if !strings.HasPrefix(path, "/workspaces/"+workspaceID+"/") && path != "/workspaces/"+workspaceID && path != "/user" {
		return "", fmt.Errorf("raw API path must stay within /user or the pinned workspace API")
	}
	return path, nil
}

func rawQuery(raw any) map[string]string {
	m, ok := raw.(map[string]any)
	if !ok || len(m) == 0 {
		return nil
	}
	query := make(map[string]string, len(m))
	for key, value := range m {
		query[key] = scalarToString(value)
	}
	return query
}

func rawChange(method string) string {
	switch method {
	case "POST":
		return "created"
	case "PUT", "PATCH":
		return "updated"
	case "DELETE":
		return "deleted"
	default:
		return ""
	}
}

func reportInputSchema() map[string]any {
	return objectSchema(map[string]any{
		"properties": map[string]any{
			"date_range_start":  map[string]any{"type": "string", "description": "Report range start. " + flexibleDatetimeDescription},
			"date_range_end":    map[string]any{"type": "string", "description": "Report range end. " + flexibleDatetimeDescription},
			"start":             map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"end":               map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"export_type":       map[string]any{"type": "string", "enum": []string{"JSON", "JSON_V1", "PDF", "CSV", "XLSX", "ZIP"}, "description": "Report export type"},
			"max_rollups":       map[string]any{"type": "integer", "minimum": 0, "description": "Keep only the N largest project rollups by tracked time (default 15). Aggregates still cover every rollup. Pass 0 for the full, unbounded list."},
			"summary_filter":    map[string]any{"type": "object", "additionalProperties": true},
			"detailed_filter":   map[string]any{"type": "object", "additionalProperties": true},
			"attendance_filter": map[string]any{"type": "object", "additionalProperties": true},
		},
	})
}
