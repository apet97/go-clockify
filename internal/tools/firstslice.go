package tools

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
	"github.com/apet97/go-clockify/internal/timeparse"
)

const demoDefaultRunID = "phase1"

type ToolResult struct {
	OK       bool              `json:"ok"`
	Action   string            `json:"action"`
	Entity   string            `json:"entity,omitempty"`
	IDs      map[string]string `json:"ids,omitempty"`
	Data     any               `json:"data,omitempty"`
	Meta     map[string]any    `json:"meta,omitempty"`
	Changed  ChangeSet         `json:"changed"`
	Warnings []Warning         `json:"warnings,omitempty"`
	Next     []NextAction      `json:"next,omitempty"`
}

type ToolError struct {
	OK       bool         `json:"ok"`
	Action   string       `json:"action"`
	Error    ErrorInfo    `json:"error"`
	Recovery RecoveryHint `json:"recovery"`
	Warnings []Warning    `json:"warnings,omitempty"`
}

type ChangeSet struct {
	Created []EntityRef `json:"created,omitempty"`
	Updated []EntityRef `json:"updated,omitempty"`
	Deleted []EntityRef `json:"deleted,omitempty"`
	Reused  []EntityRef `json:"reused,omitempty"`
}

type EntityRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type Warning struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

type NextAction struct {
	Tool   string         `json:"tool"`
	Args   map[string]any `json:"args,omitempty"`
	Reason string         `json:"reason,omitempty"`
}

type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type RecoveryHint struct {
	Hint string         `json:"hint"`
	Tool string         `json:"tool,omitempty"`
	Args map[string]any `json:"args,omitempty"`
}

type statusData struct {
	User                  clockify.User      `json:"user"`
	Workspace             clockify.Workspace `json:"workspace"`
	PinnedWorkspace       clockify.Workspace `json:"pinnedWorkspace"`
	ActiveWorkspaceID     string             `json:"activeWorkspaceId,omitempty"`
	DefaultWorkspaceID    string             `json:"defaultWorkspaceId,omitempty"`
	Timezone              string             `json:"timezone"`
	WeekStart             string             `json:"weekStart,omitempty"`
	CurrentTimer          any                `json:"currentTimer,omitempty"`
	WorkspaceFeatures     []string           `json:"workspaceFeatures,omitempty"`
	FeatureSubscription   string             `json:"featureSubscriptionType,omitempty"`
	FeatureStatus         map[string]string  `json:"featureStatus"`
	RecommendedFirstTools []string           `json:"recommendedFirstTools"`
}

func (s *Service) FirstSliceRegistry() []mcp.ToolDescriptor {
	descriptors := []mcp.ToolDescriptor{
		firstSliceDescriptor(0, toolRO("clockify_status", "Show current user, pinned workspace, timezone, features, and current timer.", objectSchema(nil)), s.ClockifyStatus),
		firstSliceDescriptor(10, toolRWIdem("clockify_demo_seed", "Create or reuse deterministic demo client/project/task/tag/time-entry objects.", objectSchema(map[string]any{
			"properties": map[string]any{
				"run_id": map[string]any{"type": "string", "description": "Stable run id. Default: phase1."},
				"prefix": map[string]any{"type": "string", "description": "Explicit object-name prefix. Default: DEMO-<run_id>."},
				"date":   map[string]any{"type": "string", "description": "YYYY-MM-DD date for the demo time entry. Default: 2026-01-02."},
				"upsert": map[string]any{"type": "boolean", "description": "Reuse existing prefixed objects. Default: true."},
			},
		})), s.ClockifyDemoSeed),
		firstSliceDescriptor(11, toolRWIdem("clockify_demo_cleanup", "Delete deterministic demo objects by prefix, continuing through partial failures.", objectSchema(map[string]any{
			"properties": map[string]any{
				"run_id": map[string]any{"type": "string", "description": "Stable run id. Default: phase1."},
				"prefix": map[string]any{"type": "string", "description": "Explicit object-name prefix. Default: DEMO-<run_id>."},
				"start":  map[string]any{"type": "string", "description": flexibleDatetimeDescription},
				"end":    map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			},
		})), s.ClockifyDemoCleanup),

		firstSliceDescriptor(20, toolRO("clockify_clients_list", "List clients in the pinned workspace.", paginationSchema(map[string]any{
			"properties": map[string]any{
				"name":        map[string]any{"type": "string"},
				"archived":    map[string]any{"type": "boolean"},
				"address":     map[string]any{"type": "string"},
				"note":        map[string]any{"type": "string"},
				"sort_column": map[string]any{"type": "string", "description": "Clockify sort-column query value."},
				"sort_order":  map[string]any{"type": "string", "description": "Clockify sort-order query value."},
			},
		})), s.ClientsList),
		firstSliceDescriptor(21, toolRW("clockify_clients_create", "Create a client in the pinned workspace.", objectSchema(map[string]any{
			"required": []string{"name"},
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
		})), s.ClientsCreate),
		firstSliceDescriptor(30, toolRO("clockify_projects_list", "List projects in the pinned workspace.", paginationSchema(map[string]any{
			"properties": map[string]any{
				"name":               map[string]any{"type": "string"},
				"strict_name_search": map[string]any{"type": "boolean"},
				"archived":           map[string]any{"type": "boolean"},
				"billable":           map[string]any{"type": "boolean"},
				"clients":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"contains_client":    map[string]any{"type": "boolean"},
				"client_status":      map[string]any{"type": "string", "enum": []string{"ACTIVE", "ARCHIVED", "ALL"}},
				"users":              map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"contains_user":      map[string]any{"type": "boolean"},
				"user_status":        map[string]any{"type": "string", "enum": []string{"PENDING", "ACTIVE", "DECLINED", "INACTIVE", "ALL"}},
				"is_template":        map[string]any{"type": "boolean"},
				"sort_column":        map[string]any{"type": "string", "enum": []string{"ID", "NAME", "CLIENT_NAME", "DURATION", "BUDGET", "PROGRESS"}},
				"sort_order":         map[string]any{"type": "string", "enum": []string{"ASCENDING", "DESCENDING"}},
				"hydrated":           map[string]any{"type": "boolean", "description": "Clockify hydrated query flag; current handler sends hydrated=true."},
				"access":             map[string]any{"type": "string", "enum": []string{"PUBLIC", "PRIVATE"}},
				"expense_limit":      map[string]any{"type": "integer"},
				"expense_date":       map[string]any{"type": "string", "description": "Expense date query value, typically YYYY-MM-DD."},
				"user_groups":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "User group IDs for the Clockify userGroups[] query."},
				"contains_group":     map[string]any{"type": "boolean"},
			},
		})), s.ProjectsList),
		firstSliceDescriptor(31, toolRW("clockify_projects_create", "Create a project in the pinned workspace.", objectSchema(map[string]any{
			"required": []string{"name"},
			"properties": map[string]any{
				"name":      map[string]any{"type": "string"},
				"client_id": map[string]any{"type": "string"},
				"client":    map[string]any{"type": "string", "description": "Client name or ID."},
				"color":     map[string]any{"type": "string", "description": "Hex color code."},
				"billable":  map[string]any{"type": "boolean"},
				"is_public": map[string]any{"type": "boolean"},
			},
		})), s.ProjectsCreate),
		firstSliceDescriptor(40, toolRO("clockify_tasks_list", "List tasks for a project.", paginationSchema(map[string]any{
			"properties": map[string]any{
				"project_id":         map[string]any{"type": "string"},
				"project":            map[string]any{"type": "string", "description": "Project name or ID."},
				"name":               map[string]any{"type": "string"},
				"strict_name_search": map[string]any{"type": "boolean"},
				"is_active":          map[string]any{"type": "boolean"},
				"sort_column":        map[string]any{"type": "string", "enum": []string{"ID", "NAME"}},
				"sort_order":         map[string]any{"type": "string", "enum": []string{"ASCENDING", "DESCENDING"}},
			},
		})), s.TasksList),
		firstSliceDescriptor(41, toolRW("clockify_tasks_create", "Create a task under a project.", objectSchema(map[string]any{
			"required": []string{"name"},
			"properties": map[string]any{
				"project_id": map[string]any{"type": "string"},
				"project":    map[string]any{"type": "string", "description": "Project name or ID."},
				"name":       map[string]any{"type": "string"},
				"billable":   map[string]any{"type": "boolean"},
			},
		})), s.TasksCreate),
		firstSliceDescriptor(50, toolRO("clockify_tags_list", "List tags in the pinned workspace.", paginationSchema(map[string]any{
			"properties": map[string]any{
				"name":        map[string]any{"type": "string"},
				"archived":    map[string]any{"type": "boolean"},
				"sort_column": map[string]any{"type": "string", "description": "Clockify sort-column query value."},
				"sort_order":  map[string]any{"type": "string", "description": "Clockify sort-order query value."},
			},
		})), s.TagsList),
		firstSliceDescriptor(51, toolRW("clockify_tags_create", "Create a tag in the pinned workspace.", objectSchema(map[string]any{
			"required": []string{"name"},
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
		})), s.TagsCreate),
		firstSliceDescriptor(60, toolRO("clockify_entries_list", "List current-user time entries in the pinned workspace.", paginationSchema(map[string]any{
			"properties": map[string]any{
				"start":      map[string]any{"type": "string", "description": flexibleDatetimeDescription},
				"end":        map[string]any{"type": "string", "description": flexibleDatetimeDescription},
				"project_id": map[string]any{"type": "string"},
				"project":    map[string]any{"type": "string", "description": "Project name or ID."},
			},
		})), s.EntriesList),
		firstSliceDescriptor(61, toolRW("clockify_entries_create", "Create a current-user time entry in the pinned workspace.", objectSchema(map[string]any{
			"required": []string{"start"},
			"properties": map[string]any{
				"start":       map[string]any{"type": "string", "description": flexibleDatetimeDescription},
				"end":         map[string]any{"type": "string", "description": flexibleDatetimeDescription},
				"description": map[string]any{"type": "string"},
				"project_id":  map[string]any{"type": "string"},
				"project":     map[string]any{"type": "string", "description": "Project name or ID."},
				"task_id":     map[string]any{"type": "string"},
				"task":        map[string]any{"type": "string", "description": "Task name. Requires project/project_id."},
				"tag_ids":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"tag":         map[string]any{"type": "string", "description": "Tag name or ID."},
				"billable":    map[string]any{"type": "boolean"},
			},
		})), s.EntriesCreate),
	}
	return normalizeDescriptors(descriptors)
}

func firstSliceDescriptor(priority int, tool mcp.Tool, handler mcp.ToolHandler) mcp.ToolDescriptor {
	if tool.Annotations == nil {
		tool.Annotations = map[string]any{}
	}
	tool.Annotations["priority"] = priority
	if _, ok := tool.Annotations["handlerKind"]; !ok {
		tool.Annotations["handlerKind"] = "native handler"
	}
	tool.OutputSchema = firstSliceOutputSchema(tool.Name, firstSliceDataOutputSchema(tool.Name))
	return mcp.ToolDescriptor{Tool: tool, Handler: firstSliceHandler(tool.Name, handler)}
}

func firstSliceHandler(action string, handler mcp.ToolHandler) mcp.ToolHandler {
	return func(ctx context.Context, args map[string]any) (any, error) {
		result, err := handler(ctx, args)
		if err != nil {
			return recoverable(action, err, defaultRecovery(action, args)), nil
		}
		return result, nil
	}
}

func objectSchema(overrides map[string]any) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
	for k, v := range overrides {
		schema[k] = v
	}
	return schema
}

func firstSliceOutputSchema(action string, dataSchema map[string]any) map[string]any {
	if dataSchema == nil {
		dataSchema = map[string]any{"description": "Tool-specific payload for " + action}
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required": []string{
			"ok",
			"action",
		},
		"properties": map[string]any{
			"ok":       map[string]any{"type": "boolean"},
			"action":   map[string]any{"type": "string", "const": action},
			"entity":   map[string]any{"type": "string"},
			"ids":      map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
			"data":     dataSchema,
			"meta":     map[string]any{"type": "object", "additionalProperties": true},
			"changed":  schemaFor[ChangeSet](),
			"warnings": schemaFor[[]Warning](),
			"next":     schemaFor[[]NextAction](),
			"error":    schemaFor[ErrorInfo](),
			"recovery": schemaFor[RecoveryHint](),
		},
	}
}

func firstSliceDataOutputSchema(action string) map[string]any {
	switch action {
	case "clockify_status":
		return schemaFor[statusData]()
	case "clockify_tools_guide":
		return objectDataSchema(map[string]any{
			"workflows":    arraySchema("Workflow groups and the tools that satisfy them."),
			"commonTasks":  arraySchema("Common user intents mapped to primary tools."),
			"domainTools":  arraySchema("Domain tool prefixes for lower-level CRUD."),
			"rawFallback":  stringArraySchema("Raw API fallback tools to use last."),
			"rulesOfThumb": stringArraySchema("Short tool-selection guidance."),
		})
	case "clockify_create_work_package":
		return objectDataSchema(map[string]any{
			"client":  map[string]any{"type": "object"},
			"project": map[string]any{"type": "object"},
			"task":    map[string]any{"type": "object"},
			"tags":    arraySchema("Created or reused tags."),
			"tagIds":  stringArraySchema("Tag IDs attached to the package."),
		})
	case "clockify_log_work", "clockify_start_work":
		return schemaFor[clockify.TimeEntry]()
	case "clockify_stop_work":
		return schemaFor[EntryView]()
	case "clockify_fix_entry":
		return schemaFor[FindAndUpdateEntryData]()
	case "clockify_switch_work":
		return objectDataSchema(map[string]any{
			"stopped": map[string]any{"type": "object", "description": "Result from stopping the previous timer, when one was running."},
			"started": map[string]any{"type": "object", "description": "Result from starting the new timer."},
			"error":   map[string]any{"type": "string", "description": "Retryable start error after a previous timer was stopped."},
		})
	case "clockify_review_day", "clockify_review_week":
		return schemaFor[TimesheetReviewData]()
	case "clockify_clients_list":
		return objectListDataSchema("clients", schemaFor[[]clockify.ClientEntity]())
	case "clockify_clients_create":
		return schemaFor[clockify.ClientEntity]()
	case "clockify_clients_get", "clockify_clients_update":
		return schemaFor[ClientView]()
	case "clockify_clients_delete":
		return entityObjectDataSchema("deleted", "clientId")
	case "clockify_projects_list":
		return objectListDataSchema("projects", schemaFor[[]clockify.Project]())
	case "clockify_projects_create":
		return schemaFor[clockify.Project]()
	case "clockify_projects_get", "clockify_projects_update", "clockify_projects_archive":
		return schemaFor[ProjectView]()
	case "clockify_projects_delete":
		return entityObjectDataSchema("deleted", "projectId")
	case "clockify_projects_rates_update":
		return entityObjectDataSchema("id", "projectId", "userId", "rateKind", "amount", "currency")
	case "clockify_projects_templates_list":
		return entityArrayDataSchema("id", "name", "isTemplate", "clientId")
	case "clockify_projects_templates_create":
		return entityObjectDataSchema("id", "name", "isTemplate", "clientId")
	case "clockify_projects_estimates_update":
		return entityObjectDataSchema("id", "projectId", "timeEstimate", "budgetEstimate", "estimate")
	case "clockify_projects_memberships_list":
		return entityObjectDataSchema("id", "userId", "membershipId", "hourlyRate", "costRate")
	case "clockify_projects_memberships_update":
		return entityObjectDataSchema("id", "projectId", "memberships", "userGroups")
	case "clockify_tasks_list":
		return objectListDataSchema("tasks", schemaFor[[]clockify.Task]())
	case "clockify_tasks_create":
		return schemaFor[clockify.Task]()
	case "clockify_tasks_get", "clockify_tasks_update":
		return schemaFor[TaskView]()
	case "clockify_tasks_delete":
		return entityObjectDataSchema("deleted", "taskId", "projectId")
	case "clockify_tasks_rates_update":
		return entityObjectDataSchema("id", "taskId", "projectId", "rateKind", "amount", "currency")
	case "clockify_tags_list":
		return objectListDataSchema("tags", schemaFor[[]clockify.Tag]())
	case "clockify_tags_get", "clockify_tags_create", "clockify_tags_update":
		return schemaFor[clockify.Tag]()
	case "clockify_tags_delete":
		return entityObjectDataSchema("deleted", "tagId")
	case "clockify_entries_list":
		return objectListDataSchema("entries", schemaFor[[]clockify.TimeEntry]())
	case "clockify_entries_get", "clockify_entries_update":
		return schemaFor[EntryView]()
	case "clockify_entries_create":
		return schemaFor[clockify.TimeEntry]()
	case "clockify_entries_delete":
		return entityObjectDataSchema("deleted", "entryId")
	case "clockify_entries_running":
		return objectDataSchema(map[string]any{
			"running":        map[string]any{"type": "boolean"},
			"entry":          nullableObjectSchema(),
			"userId":         map[string]any{"type": "string"},
			"elapsedSeconds": map[string]any{"type": "integer"},
		})
	case "clockify_entries_timer_start", "clockify_entries_timer_stop":
		return schemaFor[EntryView]()
	case "clockify_entries_timer_status":
		return objectDataSchema(map[string]any{
			"running": map[string]any{"type": "boolean"},
			"entry":   nullableObjectSchema(),
			"elapsed": map[string]any{"type": "string"},
		})
	case "clockify_entries_timer_switch":
		return objectDataSchema(map[string]any{
			"stopped": map[string]any{"type": "object"},
			"started": map[string]any{"type": "object"},
			"error":   map[string]any{"type": "string"},
		})
	case "clockify_reports_detailed", "clockify_reports_summary":
		return objectDataSchema(map[string]any{
			"range":   schemaFor[DateRange](),
			"totals":  openObjectOrArraySchema(),
			"entries": arraySchema("Report entries or raw report rows."),
			"data":    openObjectOrArraySchema(),
		})
	case "clockify_reports_weekly":
		return objectDataSchema(map[string]any{
			"byDay":   schemaFor[[]DaySummary](),
			"range":   schemaFor[DateRange](),
			"totals":  openObjectOrArraySchema(),
			"entries": arraySchema("Weekly report entries or raw report rows."),
			"data":    openObjectOrArraySchema(),
		})
	case "clockify_invoices_list":
		return entityArrayDataSchema("id", "number", "status", "clientId")
	case "clockify_invoices_get", "clockify_invoices_create", "clockify_invoices_update", "clockify_invoices_send", "clockify_invoices_mark_paid":
		return entityObjectDataSchema("id", "number", "status", "clientId")
	case "clockify_invoices_delete":
		return entityObjectDataSchema("deleted", "invoiceId")
	case "clockify_invoices_items_list":
		return entityArrayDataSchema("id", "invoiceItemId", "description", "quantity", "unitPrice")
	case "clockify_invoices_items_add", "clockify_invoices_items_update":
		return entityObjectDataSchema("id", "invoiceItemId", "description", "quantity", "unitPrice")
	case "clockify_invoices_items_delete":
		return entityObjectDataSchema("deleted", "invoiceId", "itemIndex", "itemId")
	case "clockify_invoices_export":
		return binaryExportDataSchema("content")
	case "clockify_invoices_import_time", "clockify_invoices_import_expenses":
		return entityObjectDataSchema("id", "invoiceId", "invoiceItemId", "description")
	case "clockify_invoices_payments_list":
		return entityArrayDataSchema("id", "paymentId", "amount", "date", "note")
	case "clockify_invoices_payments_create":
		return entityObjectDataSchema("id", "paymentId", "amount", "date", "note")
	case "clockify_invoices_payments_delete":
		return entityObjectDataSchema("deleted", "paymentId", "invoiceId")
	case "clockify_expenses_list":
		return entityArrayDataSchema("id", "amount", "date", "categoryId", "projectId", "userId")
	case "clockify_expenses_get", "clockify_expenses_create", "clockify_expenses_update":
		return entityObjectDataSchema("id", "amount", "date", "categoryId", "projectId", "userId")
	case "clockify_expenses_delete":
		return entityObjectDataSchema("deleted", "expenseId")
	case "clockify_expenses_categories_list":
		return entityArrayDataSchema("id", "categoryId", "name", "hasUnitPrice", "unit", "archived")
	case "clockify_expenses_categories_create", "clockify_expenses_categories_update":
		return entityObjectDataSchema("id", "categoryId", "name", "hasUnitPrice", "unit", "archived")
	case "clockify_expenses_categories_delete":
		return entityObjectDataSchema("deleted", "categoryId")
	case "clockify_custom_fields_list":
		return entityArrayDataSchema("id", "name", "type", "status", "entity_type")
	case "clockify_custom_fields_get", "clockify_custom_fields_create", "clockify_custom_fields_update":
		return entityObjectDataSchema("id", "name", "type", "status", "entity_type")
	case "clockify_custom_fields_delete":
		return entityObjectDataSchema("deleted", "fieldId")
	case "clockify_custom_fields_set_value":
		return entityObjectDataSchema("id", "customFieldId", "value", "projectId", "entryId", "source")
	case "clockify_time_off_requests_list":
		return entityArrayDataSchema("id", "requestId", "policyId", "status")
	case "clockify_time_off_requests_create", "clockify_time_off_requests_get", "clockify_time_off_requests_update":
		return entityObjectDataSchema("id", "requestId", "policyId", "status")
	case "clockify_time_off_requests_delete":
		return entityObjectDataSchema("deleted", "requestId", "policyId")
	case "clockify_time_off_approve", "clockify_time_off_deny":
		return entityObjectDataSchema("id", "requestId", "policyId", "status")
	case "clockify_time_off_policies_list":
		return entityArrayDataSchema("id", "policyId", "name", "timeUnit", "archived")
	case "clockify_time_off_policies_get", "clockify_time_off_policies_create", "clockify_time_off_policies_update":
		return entityObjectDataSchema("id", "policyId", "name", "timeUnit", "archived")
	case "clockify_time_off_balances":
		return entityObjectDataSchema("policyId", "userId", "balance", "used", "available")
	case "clockify_time_off_archive":
		return entityObjectDataSchema("id", "policyId", "archived")
	case "clockify_scheduling_assignments_list":
		return entityArrayDataSchema("id", "assignmentId", "projectId", "userId", "start", "end")
	case "clockify_scheduling_assignments_create", "clockify_scheduling_assignments_update":
		return entityArrayDataSchema("id", "assignmentId", "projectId", "userId", "start", "end")
	case "clockify_scheduling_assignments_get":
		return entityObjectDataSchema("id", "assignmentId", "projectId", "userId", "start", "end")
	case "clockify_scheduling_assignments_delete":
		return entityObjectDataSchema("deleted", "assignmentId")
	case "clockify_scheduling_project_totals":
		return entityArrayDataSchema("id", "projectId", "total", "totals")
	case "clockify_scheduling_capacity":
		return entityArrayDataSchema("id", "userId", "assignmentId", "total", "totals")
	case "clockify_scheduling_user_totals":
		return entityObjectDataSchema("id", "userId", "assignmentId", "total", "totals")
	case "clockify_reports_attendance", "clockify_reports_money", "clockify_reports_expense", "clockify_reports_export":
		return reportDataSchema(action)
	case "clockify_approvals_list":
		return entityArrayDataSchema("id", "approvalId", "status", "userId", "start", "end")
	case "clockify_approvals_get", "clockify_approvals_submit", "clockify_approvals_approve", "clockify_approvals_reject", "clockify_approvals_withdraw":
		return entityObjectDataSchema("id", "approvalId", "status", "userId", "start", "end")
	case "clockify_approvals_resubmit":
		return entityObjectDataSchema("id", "approvalId", "status")
	case "clockify_webhooks_list":
		return entityArrayDataSchema("id", "name", "url", "webhookEvent")
	case "clockify_webhooks_get", "clockify_webhooks_create", "clockify_webhooks_update":
		return entityObjectDataSchema("id", "name", "url", "webhookEvent")
	case "clockify_webhooks_delete":
		return entityObjectDataSchema("deleted", "webhookId")
	case "clockify_webhooks_test":
		return entityObjectDataSchema("id", "webhookId", "status")
	case "clockify_webhooks_events":
		return stringArraySchema("Supported Clockify webhook event names.")
	case "clockify_groups_list":
		return entityArrayDataSchema("id", "groupId", "name", "userIds")
	case "clockify_groups_create", "clockify_groups_get", "clockify_groups_update":
		return entityObjectDataSchema("id", "groupId", "name", "userIds")
	case "clockify_groups_delete":
		return entityObjectDataSchema("deleted", "groupId")
	case "clockify_groups_add_user", "clockify_groups_remove_user":
		return entityObjectDataSchema("groupId", "userId")
	case "clockify_holidays_list", "clockify_holidays_list_for_user_period":
		return entityArrayDataSchema("id", "holidayId", "name", "start", "end")
	case "clockify_holidays_create", "clockify_holidays_get", "clockify_holidays_update":
		return entityObjectDataSchema("id", "name", "start", "end")
	case "clockify_holidays_delete":
		return entityObjectDataSchema("deleted", "holidayId")
	case "clockify_users_invite":
		return entityObjectDataSchema("id", "email", "status")
	case "clockify_users_list":
		return schemaFor[[]UserView]()
	case "clockify_users_profile":
		return schemaFor[UserView]()
	case "clockify_users_deactivate", "clockify_users_role":
		return entityObjectDataSchema("id", "userId", "status", "role")
	case "clockify_workspace_settings":
		return schemaFor[WorkspaceView]()
	case "clockify_entries_mark_invoiced":
		return objectDataSchema(map[string]any{
			"updated":       map[string]any{"type": "boolean"},
			"timeEntryIds":  stringArraySchema("Time entry IDs updated."),
			"invoiced":      map[string]any{"type": "boolean"},
			"upstreamReply": map[string]any{"type": "object", "additionalProperties": true},
		})
	case "clockify_invoice_client_work":
		return dataKeysSchema("billing", "client", "clientId", "discount", "financials", "id", "import", "invoice", "number", "payment_summary", "raw", "status", "suggestedActions", "taxes")
	case "clockify_record_expense":
		return dataKeysSchema("amount", "approval", "category", "categoryId", "date", "entities", "expense", "id", "invoicing", "notes", "projectId", "raw", "receipt", "status", "taskId", "tax", "userId")
	case "clockify_request_time_off":
		return dataKeysSchema("actions", "audit", "day_period", "duration", "id", "note", "period", "policy", "policyId", "raw", "request", "status", "suggestedActions", "user", "userId")
	case "clockify_schedule_work":
		return objectDataSchema(map[string]any{"id": map[string]any{"type": "string"}, "assignment": map[string]any{"type": "object"}})
	case "clockify_setup_webhook":
		return dataKeysSchema("id", "name", "secret_token_present", "triggerSource", "triggerSourceType", "url", "webhook", "webhookEvent", "webhookEventStatus")
	case "clockify_demo_seed":
		return objectDataSchema(map[string]any{
			"runId":      map[string]any{"type": "string"},
			"prefix":     map[string]any{"type": "string"},
			"client":     map[string]any{"type": "object"},
			"project":    map[string]any{"type": "object"},
			"task":       map[string]any{"type": "object"},
			"tag":        map[string]any{"type": "object"},
			"entry":      map[string]any{"type": "object"},
			"usableWith": stringArraySchema("Workflow tools that can use the seeded IDs."),
		})
	case "clockify_demo_cleanup":
		return objectDataSchema(map[string]any{
			"runId":         map[string]any{"type": "string"},
			"prefix":        map[string]any{"type": "string"},
			"deletedCount":  map[string]any{"type": "integer"},
			"warningsCount": map[string]any{"type": "integer"},
		})
	default:
		return nil
	}
}

func objectDataSchema(properties map[string]any) map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"properties":           properties,
	}
}

func nullableObjectSchema() map[string]any {
	return map[string]any{
		"type":                 []string{"object", "null"},
		"additionalProperties": true,
	}
}

func openObjectOrArraySchema() map[string]any {
	return map[string]any{
		"type":                 []string{"object", "array"},
		"additionalProperties": true,
		"items":                map[string]any{},
	}
}

func objectListDataSchema(key string, itemsSchema map[string]any) map[string]any {
	return objectDataSchema(map[string]any{
		key:        itemsSchema,
		"count":    map[string]any{"type": "integer"},
		"page":     map[string]any{"type": "integer"},
		"pageSize": map[string]any{"type": "integer"},
	})
}

func entityObjectDataSchema(fields ...string) map[string]any {
	props := make(map[string]any, len(fields))
	for _, field := range fields {
		props[field] = map[string]any{}
	}
	return objectDataSchema(props)
}

func entityArrayDataSchema(fields ...string) map[string]any {
	return map[string]any{
		"type":  "array",
		"items": entityObjectDataSchema(fields...),
	}
}

func binaryExportDataSchema(extraFields ...string) map[string]any {
	fields := []string{"contentType", "filename", "bytes", "bodyEncoding", "base64Bytes", "truncated", "body"}
	fields = append(fields, extraFields...)
	return entityObjectDataSchema(fields...)
}

func reportDataSchema(action string) map[string]any {
	if action == "clockify_reports_export" {
		return binaryExportDataSchema("id", "workspaceId", "totals", "timeentries", "entries", "data")
	}
	return entityObjectDataSchema("id", "workspaceId", "totals", "timeentries", "entries", "data")
}

func dataKeysSchema(keys ...string) map[string]any {
	props := make(map[string]any, len(keys))
	for _, key := range keys {
		props[key] = map[string]any{}
	}
	return objectDataSchema(props)
}

func arraySchema(description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description,
		"items":       map[string]any{"type": "object"},
	}
}

func result(action, entity string, ids map[string]string, data any, changed ChangeSet, warnings []Warning, next []NextAction, meta ...map[string]any) ToolResult {
	var resultMeta map[string]any
	if len(meta) > 0 && len(meta[0]) > 0 {
		resultMeta = meta[0]
	}
	return ToolResult{
		OK:       true,
		Action:   action,
		Entity:   entity,
		IDs:      cleanIDs(ids),
		Data:     sanitizeResultValue(data),
		Meta:     sanitizeResultMeta(resultMeta),
		Changed:  changed,
		Warnings: warnings,
		Next:     next,
	}
}

func cleanIDs(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if strings.TrimSpace(v) != "" {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func recoverable(action string, err error, recovery RecoveryHint) ToolError {
	code := "error"
	message := err.Error()
	var apiErr *clockify.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case 400:
			code = "invalid_request"
		case 402:
			code = "feature_unavailable"
		case 401, 403:
			code = "auth_or_permission"
			if looksLikeFeatureUnavailable(message) {
				code = "feature_unavailable"
			}
		case 404:
			code = "not_found"
		case 409:
			code = "conflict"
		case 429:
			code = "rate_limited"
		default:
			code = "clockify_upstream_error"
		}
	} else if strings.Contains(strings.ToLower(message), "not found") {
		code = "not_found"
	}
	if recovery.Hint == "" {
		recovery = defaultRecovery(action, nil)
	}
	return ToolError{
		OK:       false,
		Action:   action,
		Error:    ErrorInfo{Code: code, Message: message},
		Recovery: recovery,
	}
}

func looksLikeFeatureUnavailable(message string) bool {
	message = strings.ToLower(message)
	for _, needle := range []string{"feature", "paid", "plan", "subscription", "upgrade", "not available", "not supported"} {
		if strings.Contains(message, needle) {
			return true
		}
	}
	return false
}

func defaultRecovery(action string, args map[string]any) RecoveryHint {
	switch {
	case strings.Contains(action, "tools_guide"):
		return RecoveryHint{Hint: "Call clockify_status, then choose a workflow tool from tools/list.", Tool: "clockify_status"}
	case action == "clockify_api_get":
		return RecoveryHint{Hint: "Use workflow or domain tools first. Call clockify_tools_guide to find the nearest typed tool before retrying raw GET.", Tool: "clockify_tools_guide"}
	case action == "clockify_api_request":
		return RecoveryHint{Hint: "Use workflow or domain tools first. Call clockify_tools_guide to find the nearest typed tool before enabling raw writes or retrying raw fallback.", Tool: "clockify_tools_guide"}
	case strings.Contains(action, "create_work_package"):
		return RecoveryHint{Hint: "List clients, projects, tasks, or tags, then retry with returned IDs or exact names.", Tool: "clockify_tools_guide"}
	case strings.Contains(action, "log_work"), strings.Contains(action, "start_work"), strings.Contains(action, "stop_work"), strings.Contains(action, "switch_work"), strings.Contains(action, "fix_entry"), strings.Contains(action, "review_day"), strings.Contains(action, "review_week"):
		return RecoveryHint{Hint: "Check the entry, project, task, tag, and time fields; use returned IDs or exact names.", Tool: "clockify_review_day"}
	case strings.Contains(action, "invoice"):
		return RecoveryHint{Hint: "If invoicing is unavailable, report that and continue. Otherwise list clients or invoices, then retry with returned IDs.", Tool: "clockify_invoices_list"}
	case strings.Contains(action, "expense"):
		return RecoveryHint{Hint: "If expenses are unavailable, report that and continue. Otherwise list expense categories and retry with returned IDs.", Tool: "clockify_expenses_categories_list"}
	case strings.Contains(action, "time_off"):
		return RecoveryHint{Hint: "If time off is unavailable, report that and continue. Otherwise list policies and retry with a returned policy ID.", Tool: "clockify_time_off_policies_list"}
	case strings.Contains(action, "schedule"):
		return RecoveryHint{Hint: "If scheduling is unavailable, report that and continue. Otherwise list users/projects and retry with returned IDs.", Tool: "clockify_scheduling_assignments_list"}
	case strings.Contains(action, "webhook"):
		return RecoveryHint{Hint: "If webhooks are unavailable, report that and continue. Otherwise verify the HTTPS callback URL and event, then retry.", Tool: "clockify_webhooks_events"}
	case strings.Contains(action, "projects"):
		return RecoveryHint{Hint: "Check the project/client fields, list projects or clients, then retry with returned IDs.", Tool: "clockify_projects_list"}
	case strings.Contains(action, "clients"):
		return RecoveryHint{Hint: "List clients and reuse an existing client ID, or retry with a different name.", Tool: "clockify_clients_list"}
	case strings.Contains(action, "tasks"):
		return RecoveryHint{Hint: "List projects and tasks, then retry with returned project/task IDs.", Tool: "clockify_tasks_list", Args: map[string]any{"project_id": stringArg(args, "project_id")}}
	case strings.Contains(action, "tags"):
		return RecoveryHint{Hint: "List tags and reuse an existing tag ID, or retry with a different name.", Tool: "clockify_tags_list"}
	case strings.Contains(action, "entries"):
		return RecoveryHint{Hint: "Check start/end times and project/task/tag IDs, list entries or projects, then retry.", Tool: "clockify_entries_list"}
	default:
		return RecoveryHint{Hint: "Check the returned error, call clockify_status, then retry with IDs returned by previous calls.", Tool: "clockify_status"}
	}
}

func (s *Service) ClockifyStatus(ctx context.Context, _ map[string]any) (any, error) {
	user, err := s.getCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	workspace, err := s.getWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	warnings := []Warning{}
	currentTimer, err := s.currentTimer(ctx)
	if err != nil {
		warnings = append(warnings, Warning{Code: "timer_unavailable", Message: err.Error()})
	}
	featureStatus, featureWarnings := oneUserFeatureStatus(workspace.Features)
	warnings = append(warnings, featureWarnings...)
	return result("clockify_status", "workspace", map[string]string{
		"workspaceId": wsID,
		"userId":      user.ID,
	}, statusData{
		User:                  user,
		Workspace:             workspace,
		PinnedWorkspace:       workspace,
		ActiveWorkspaceID:     user.ActiveWorkspace,
		DefaultWorkspaceID:    user.DefaultWorkspace,
		Timezone:              s.timezoneName(),
		WeekStart:             statusWeekStart(workspace, user),
		CurrentTimer:          currentTimer,
		WorkspaceFeatures:     append([]string(nil), workspace.Features...),
		FeatureSubscription:   workspace.FeatureSubscriptionType,
		FeatureStatus:         featureStatus,
		RecommendedFirstTools: []string{"clockify_tools_guide", "clockify_create_work_package", "clockify_log_work", "clockify_start_work", "clockify_review_day"},
	}, ChangeSet{}, warnings, []NextAction{
		{Tool: "clockify_tools_guide", Reason: "Pick the best workflow tool before falling back to domain tools."},
		{Tool: "clockify_create_work_package", Reason: "Create or reuse a client/project/task/tag package for work tracking."},
		{Tool: "clockify_log_work", Reason: "Log finished work with human-friendly project, task, and tag names."},
	}), nil
}

func statusWeekStart(workspace clockify.Workspace, user clockify.User) string {
	if weekStart, ok := workspaceSettingAny(workspace.WorkspaceSettings, "weekStart", "week_start").(string); ok && strings.TrimSpace(weekStart) != "" {
		return strings.ToUpper(strings.TrimSpace(weekStart))
	}
	if settings, ok := user.Settings.(map[string]any); ok {
		if weekStart := firstReportString(settings, "weekStart", "week_start"); weekStart != "" {
			return strings.ToUpper(weekStart)
		}
	}
	return ""
}

func oneUserFeatureStatus(features []string) (map[string]string, []Warning) {
	type signal struct {
		key     string
		needles []string
	}
	signals := []signal{
		{key: "invoices", needles: []string{"INVOICE", "INVOIC"}},
		{key: "expenses", needles: []string{"EXPENSE"}},
		{key: "customFields", needles: []string{"CUSTOM_FIELD", "CUSTOMFIELD"}},
		{key: "timeOff", needles: []string{"TIME_OFF", "TIMEOFF", "TIME OFF"}},
		{key: "scheduling", needles: []string{"SCHEDUL"}},
		{key: "approvals", needles: []string{"APPROVAL"}},
		{key: "webhooks", needles: []string{"WEBHOOK"}},
		{key: "reports", needles: []string{"REPORT"}},
		{key: "groups", needles: []string{"GROUP"}},
		{key: "holidays", needles: []string{"HOLIDAY"}},
		{key: "sharedReports", needles: []string{"SHARED_REPORT", "SHAREDREPORT"}},
	}
	normalized := make([]string, 0, len(features))
	for _, feature := range features {
		feature = strings.ToUpper(strings.TrimSpace(feature))
		if feature != "" {
			normalized = append(normalized, feature)
		}
	}
	out := make(map[string]string, len(signals))
	if len(normalized) == 0 {
		for _, sig := range signals {
			out[sig.key] = "unknown"
		}
		return out, []Warning{{Code: "feature_signals_unknown", Message: "Clockify did not return workspace feature signals; paid-feature tools may return recovery guidance when called."}}
	}
	notAdvertised := []string{}
	for _, sig := range signals {
		status := "not_advertised"
		if sig.key == "groups" || sig.key == "holidays" {
			status = "unknown"
		}
		for _, feature := range normalized {
			for _, needle := range sig.needles {
				if strings.Contains(feature, needle) {
					status = "available"
					break
				}
			}
			if status == "available" {
				break
			}
		}
		out[sig.key] = status
		if status == "not_advertised" {
			notAdvertised = append(notAdvertised, sig.key)
		}
	}
	if len(notAdvertised) == 0 {
		return out, nil
	}
	return out, []Warning{{Code: "feature_signals_not_advertised", Message: "Some optional Clockify feature signals were not advertised by the workspace: " + strings.Join(notAdvertised, ", ")}}
}

func (s *Service) ClientsList(ctx context.Context, args map[string]any) (any, error) {
	items, page, pageSize, err := s.listClients(ctx, args)
	if err != nil {
		return nil, err
	}
	return result("clockify_clients_list", "client", map[string]string{"workspaceId": s.WorkspaceID}, map[string]any{
		"clients":  items,
		"count":    len(items),
		"page":     page,
		"pageSize": pageSize,
	}, ChangeSet{}, nil, nil), nil
}

func (s *Service) ClientsCreate(ctx context.Context, args map[string]any) (any, error) {
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	client, err := s.createClient(ctx, name)
	if err != nil {
		return nil, err
	}
	return result("clockify_clients_create", "client", map[string]string{
		"workspaceId": s.WorkspaceID,
		"clientId":    client.ID,
	}, client, ChangeSet{Created: []EntityRef{clientRef(client)}}, nil, []NextAction{{
		Tool:   "clockify_projects_create",
		Args:   map[string]any{"client_id": client.ID},
		Reason: "Create a project for this client.",
	}}), nil
}

func (s *Service) ProjectsList(ctx context.Context, args map[string]any) (any, error) {
	items, page, pageSize, err := s.listProjects(ctx, args)
	if err != nil {
		return nil, err
	}
	return result("clockify_projects_list", "project", map[string]string{"workspaceId": s.WorkspaceID}, map[string]any{
		"projects": items,
		"count":    len(items),
		"page":     page,
		"pageSize": pageSize,
	}, ChangeSet{}, nil, nil), nil
}

func (s *Service) ProjectsCreate(ctx context.Context, args map[string]any) (any, error) {
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	project, clientID, err := s.createProject(ctx, args, name)
	if err != nil {
		return nil, err
	}
	return result("clockify_projects_create", "project", map[string]string{
		"workspaceId": s.WorkspaceID,
		"projectId":   project.ID,
		"clientId":    clientID,
	}, project, ChangeSet{Created: []EntityRef{projectRef(project)}}, nil, []NextAction{{
		Tool:   "clockify_tasks_create",
		Args:   map[string]any{"project_id": project.ID},
		Reason: "Add a task to this project.",
	}}), nil
}

func (s *Service) TasksList(ctx context.Context, args map[string]any) (any, error) {
	projectID, err := s.projectIDFromArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	items, page, pageSize, err := s.listTasks(ctx, projectID, args)
	if err != nil {
		return nil, err
	}
	return result("clockify_tasks_list", "task", map[string]string{"workspaceId": s.WorkspaceID, "projectId": projectID}, map[string]any{
		"tasks":    items,
		"count":    len(items),
		"page":     page,
		"pageSize": pageSize,
	}, ChangeSet{}, nil, nil), nil
}

func (s *Service) TasksCreate(ctx context.Context, args map[string]any) (any, error) {
	projectID, err := s.projectIDFromArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	task, err := s.createTask(ctx, projectID, name, args)
	if err != nil {
		return nil, err
	}
	return result("clockify_tasks_create", "task", map[string]string{
		"workspaceId": s.WorkspaceID,
		"projectId":   projectID,
		"taskId":      task.ID,
	}, task, ChangeSet{Created: []EntityRef{taskRef(task)}}, nil, nil), nil
}

func (s *Service) TagsList(ctx context.Context, args map[string]any) (any, error) {
	items, page, pageSize, err := s.listTags(ctx, args)
	if err != nil {
		return nil, err
	}
	return result("clockify_tags_list", "tag", map[string]string{"workspaceId": s.WorkspaceID}, map[string]any{
		"tags":     items,
		"count":    len(items),
		"page":     page,
		"pageSize": pageSize,
	}, ChangeSet{}, nil, nil), nil
}

func (s *Service) TagsCreate(ctx context.Context, args map[string]any) (any, error) {
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	tag, err := s.createTag(ctx, name)
	if err != nil {
		return nil, err
	}
	return result("clockify_tags_create", "tag", map[string]string{
		"workspaceId": s.WorkspaceID,
		"tagId":       tag.ID,
	}, tag, ChangeSet{Created: []EntityRef{tagRef(tag)}}, nil, nil), nil
}

func (s *Service) EntriesList(ctx context.Context, args map[string]any) (any, error) {
	entries, userID, page, pageSize, err := s.listCurrentUserEntries(ctx, args)
	if err != nil {
		return nil, err
	}
	return result("clockify_entries_list", "entry", map[string]string{"workspaceId": s.WorkspaceID, "userId": userID}, map[string]any{
		"entries":  entries,
		"count":    len(entries),
		"page":     page,
		"pageSize": pageSize,
	}, ChangeSet{}, nil, nil), nil
}

func (s *Service) EntriesCreate(ctx context.Context, args map[string]any) (any, error) {
	entry, ids, err := s.createEntry(ctx, args)
	if err != nil {
		return nil, err
	}
	return result("clockify_entries_create", "entry", ids, entry, ChangeSet{Created: []EntityRef{entryRef(entry)}}, nil, nil), nil
}

func (s *Service) ClockifyDemoSeed(ctx context.Context, args map[string]any) (any, error) {
	runID := demoRunID(args)
	prefix := demoPrefix(args)
	reviewDate := strings.TrimSpace(stringArg(args, "date"))
	if reviewDate == "" {
		reviewDate = "2026-01-02"
	}
	upsert := true
	if v, ok := args["upsert"].(bool); ok {
		upsert = v
	}

	changed := ChangeSet{}
	warnings := []Warning{}
	client, reused, err := s.ensureDemoClient(ctx, prefix, upsert)
	if err != nil {
		return nil, err
	}
	addChanged(&changed, reused, clientRef(client))

	project, reused, err := s.ensureDemoProject(ctx, prefix, client.ID, upsert)
	if err != nil {
		return nil, err
	}
	addChanged(&changed, reused, projectRef(project))

	task, reused, err := s.ensureDemoTask(ctx, prefix, project.ID, upsert)
	if err != nil {
		return nil, err
	}
	addChanged(&changed, reused, taskRef(task))

	tag, reused, err := s.ensureDemoTag(ctx, prefix, upsert)
	if err != nil {
		return nil, err
	}
	addChanged(&changed, reused, tagRef(tag))

	entry, reused, err := s.ensureDemoEntry(ctx, args, prefix, project.ID, task.ID, tag.ID, upsert)
	if err != nil {
		return nil, err
	}
	addChanged(&changed, reused, entryRef(entry))

	out := result("clockify_demo_seed", "demo", map[string]string{
		"workspaceId": s.WorkspaceID,
		"clientId":    client.ID,
		"projectId":   project.ID,
		"taskId":      task.ID,
		"tagId":       tag.ID,
		"entryId":     entry.ID,
	}, map[string]any{
		"runId":      runID,
		"prefix":     prefix,
		"client":     client,
		"project":    project,
		"task":       task,
		"tag":        tag,
		"entry":      entry,
		"usableWith": []string{"clockify_log_work", "clockify_start_work", "clockify_review_day", "clockify_demo_cleanup"},
	}, changed, warnings, []NextAction{
		{
			Tool:   "clockify_review_day",
			Args:   map[string]any{"date": reviewDate},
			Reason: "Review the seeded demo entry and verify report totals.",
		},
		{
			Tool:   "clockify_demo_cleanup",
			Args:   map[string]any{"prefix": prefix},
			Reason: "Clean up the deterministic demo objects when finished.",
		},
	})
	s.updateDemoResource(runID, prefix, "seeded", out)
	return out, nil
}

func (s *Service) ClockifyDemoCleanup(ctx context.Context, args map[string]any) (any, error) {
	runID := demoRunID(args)
	prefix := demoPrefix(args)
	changed := ChangeSet{}
	warnings := []Warning{}

	entries, _, _, _, err := s.listCurrentUserEntries(ctx, cleanupEntryRangeArgs(args))
	if err != nil {
		warnings = append(warnings, Warning{Code: "entries_list_failed", Message: err.Error()})
	} else {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Description, prefix) {
				s.deleteDemo(ctx, "entry", entry.ID, entry.Description, &changed, &warnings)
			}
		}
	}

	projects, _, _, err := s.listProjects(ctx, map[string]any{"page_size": float64(200)})
	if err != nil {
		warnings = append(warnings, Warning{Code: "projects_list_failed", Message: err.Error()})
	} else {
		for _, project := range projects {
			if strings.HasPrefix(project.Name, prefix) {
				tasks, _, _, taskErr := s.listTasks(ctx, project.ID, map[string]any{"page_size": float64(200)})
				if taskErr != nil {
					warnings = append(warnings, Warning{Code: "tasks_list_failed", Message: taskErr.Error()})
				} else {
					for _, task := range tasks {
						if strings.HasPrefix(task.Name, prefix) {
							s.deleteDemoTask(ctx, project.ID, task, &changed, &warnings)
						}
					}
				}
			}
		}
	}

	tags, _, _, err := s.listTags(ctx, map[string]any{"page_size": float64(200)})
	if err != nil {
		warnings = append(warnings, Warning{Code: "tags_list_failed", Message: err.Error()})
	} else {
		for _, tag := range tags {
			if strings.HasPrefix(tag.Name, prefix) {
				s.deleteDemo(ctx, "tag", tag.ID, tag.Name, &changed, &warnings)
			}
		}
	}

	for _, project := range projects {
		if strings.HasPrefix(project.Name, prefix) {
			s.deleteDemo(ctx, "project", project.ID, project.Name, &changed, &warnings)
		}
	}

	clients, _, _, err := s.listClients(ctx, map[string]any{"page_size": float64(200)})
	if err != nil {
		warnings = append(warnings, Warning{Code: "clients_list_failed", Message: err.Error()})
	} else {
		for _, client := range clients {
			if strings.HasPrefix(client.Name, prefix) {
				s.deleteDemo(ctx, "client", client.ID, client.Name, &changed, &warnings)
			}
		}
	}

	out := result("clockify_demo_cleanup", "demo", map[string]string{"workspaceId": s.WorkspaceID}, map[string]any{
		"runId":         runID,
		"prefix":        prefix,
		"deletedCount":  len(changed.Deleted),
		"warningsCount": len(warnings),
	}, changed, warnings, []NextAction{{
		Tool:   "clockify_demo_seed",
		Args:   map[string]any{"prefix": prefix},
		Reason: "Recreate the deterministic demo fixture if another smoke pass is needed.",
	}})
	s.updateDemoResource(runID, prefix, "cleaned", out)
	return out, nil
}

func (s *Service) getWorkspace(ctx context.Context, wsID string) (clockify.Workspace, error) {
	path, err := paths.Workspace(wsID)
	if err != nil {
		return clockify.Workspace{}, err
	}
	var workspace clockify.Workspace
	if err := s.Client.Get(ctx, path, nil, &workspace); err != nil {
		return clockify.Workspace{}, err
	}
	return workspace, nil
}

func (s *Service) currentTimer(ctx context.Context) (any, error) {
	entries, userID, _, _, err := s.listCurrentUserEntries(ctx, map[string]any{"page_size": float64(1)})
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 || !entries[0].IsRunning() {
		return map[string]any{"running": false, "entry": nil, "userId": userID}, nil
	}
	entry := entries[0]
	start, err := entry.StartTime()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"running":        true,
		"entry":          entry,
		"userId":         userID,
		"elapsedSeconds": int64(time.Since(start).Seconds()),
	}, nil
}

func (s *Service) listClients(ctx context.Context, args map[string]any) ([]clockify.ClientEntity, int, int, error) {
	page, pageSize := paginationFromArgs(args)
	path, err := paths.Workspace(s.WorkspaceID, "clients")
	if err != nil {
		return nil, 0, 0, err
	}
	var out []clockify.ClientEntity
	err = s.Client.Get(ctx, path, clientListQuery(args, page, pageSize), &out)
	return out, page, pageSize, err
}

func (s *Service) createClient(ctx context.Context, name string) (clockify.ClientEntity, error) {
	path, err := paths.Workspace(s.WorkspaceID, "clients")
	if err != nil {
		return clockify.ClientEntity{}, err
	}
	var client clockify.ClientEntity
	err = s.Client.Post(ctx, path, map[string]any{"name": name}, &client)
	return client, err
}

func (s *Service) listProjects(ctx context.Context, args map[string]any) ([]clockify.Project, int, int, error) {
	page, pageSize := paginationFromArgs(args)
	path, err := paths.Workspace(s.WorkspaceID, "projects")
	if err != nil {
		return nil, 0, 0, err
	}
	var out []clockify.Project
	values, err := projectListQueryValues(args, page, pageSize)
	if err != nil {
		return nil, 0, 0, err
	}
	err = s.Client.GetValues(ctx, path, values, &out)
	return out, page, pageSize, err
}

func (s *Service) createProject(ctx context.Context, args map[string]any, name string) (clockify.Project, string, error) {
	payload := map[string]any{"name": name}
	clientID := strings.TrimSpace(stringArg(args, "client_id"))
	if clientID == "" {
		clientRef := strings.TrimSpace(stringArg(args, "client"))
		if clientRef != "" {
			var err error
			clientID, err = s.resolveClientID(ctx, s.WorkspaceID, clientRef)
			if err != nil {
				return clockify.Project{}, "", err
			}
		}
	}
	if clientID != "" {
		if err := resolve.ValidateID(clientID, "client_id"); err != nil {
			return clockify.Project{}, "", err
		}
		payload["clientId"] = clientID
	}
	if color := strings.TrimSpace(stringArg(args, "color")); color != "" {
		payload["color"] = color
	}
	if billable, ok := args["billable"].(bool); ok {
		payload["billable"] = billable
	}
	if isPublic, ok := args["is_public"].(bool); ok {
		payload["isPublic"] = isPublic
	}
	path, err := paths.Workspace(s.WorkspaceID, "projects")
	if err != nil {
		return clockify.Project{}, "", err
	}
	var project clockify.Project
	err = s.Client.Post(ctx, path, payload, &project)
	return project, clientID, err
}

func (s *Service) listTasks(ctx context.Context, projectID string, args map[string]any) ([]clockify.Task, int, int, error) {
	if err := resolve.ValidateID(projectID, "project_id"); err != nil {
		return nil, 0, 0, err
	}
	page, pageSize := paginationFromArgs(args)
	path, err := paths.Workspace(s.WorkspaceID, "projects", projectID, "tasks")
	if err != nil {
		return nil, 0, 0, err
	}
	var out []clockify.Task
	err = s.Client.Get(ctx, path, taskListQuery(args, page, pageSize), &out)
	return out, page, pageSize, err
}

func (s *Service) createTask(ctx context.Context, projectID, name string, args map[string]any) (clockify.Task, error) {
	if err := resolve.ValidateID(projectID, "project_id"); err != nil {
		return clockify.Task{}, err
	}
	payload := map[string]any{"name": name}
	if billable, ok := args["billable"].(bool); ok {
		payload["billable"] = billable
	}
	path, err := paths.Workspace(s.WorkspaceID, "projects", projectID, "tasks")
	if err != nil {
		return clockify.Task{}, err
	}
	var task clockify.Task
	err = s.Client.Post(ctx, path, payload, &task)
	return task, err
}

func (s *Service) listTags(ctx context.Context, args map[string]any) ([]clockify.Tag, int, int, error) {
	page, pageSize := paginationFromArgs(args)
	path, err := paths.Workspace(s.WorkspaceID, "tags")
	if err != nil {
		return nil, 0, 0, err
	}
	var out []clockify.Tag
	err = s.Client.Get(ctx, path, tagListQuery(args, page, pageSize), &out)
	return out, page, pageSize, err
}

func (s *Service) createTag(ctx context.Context, name string) (clockify.Tag, error) {
	path, err := paths.Workspace(s.WorkspaceID, "tags")
	if err != nil {
		return clockify.Tag{}, err
	}
	var tag clockify.Tag
	err = s.Client.Post(ctx, path, map[string]any{"name": name}, &tag)
	return tag, err
}

func (s *Service) listCurrentUserEntries(ctx context.Context, args map[string]any) ([]clockify.TimeEntry, string, int, int, error) {
	user, err := s.getCurrentUser(ctx)
	if err != nil {
		return nil, "", 0, 0, err
	}
	page, pageSize := paginationFromArgs(args)
	query := pageQuery(page, pageSize)
	loc := s.location()
	if start := strings.TrimSpace(stringArg(args, "start")); start != "" {
		t, err := timeparse.ParseDatetime(start, loc)
		if err != nil {
			return nil, "", 0, 0, fmt.Errorf("invalid start: %w", err)
		}
		query["start"] = timeparse.FormatISO(t)
	}
	if end := strings.TrimSpace(stringArg(args, "end")); end != "" {
		t, err := timeparse.ParseDatetime(end, loc)
		if err != nil {
			return nil, "", 0, 0, fmt.Errorf("invalid end: %w", err)
		}
		query["end"] = timeparse.FormatISO(t)
	}
	projectID := strings.TrimSpace(stringArg(args, "project_id"))
	if projectID == "" {
		projectRef := strings.TrimSpace(stringArg(args, "project"))
		if projectRef != "" {
			projectID, err = s.resolveProjectID(ctx, s.WorkspaceID, projectRef)
			if err != nil {
				return nil, "", 0, 0, err
			}
		}
	}
	if projectID != "" {
		if err := resolve.ValidateID(projectID, "project_id"); err != nil {
			return nil, "", 0, 0, err
		}
		query["project"] = projectID
	}
	if err := resolve.ValidateID(user.ID, "user_id"); err != nil {
		return nil, "", 0, 0, err
	}
	path, err := paths.Workspace(s.WorkspaceID, "user", user.ID, "time-entries")
	if err != nil {
		return nil, "", 0, 0, err
	}
	var entries []clockify.TimeEntry
	err = s.Client.Get(ctx, path, query, &entries)
	return entries, user.ID, page, pageSize, err
}

func (s *Service) createEntry(ctx context.Context, args map[string]any) (clockify.TimeEntry, map[string]string, error) {
	startRaw := strings.TrimSpace(stringArg(args, "start"))
	if startRaw == "" {
		return clockify.TimeEntry{}, nil, fmt.Errorf("start is required")
	}
	loc := s.location()
	startTime, err := timeparse.ParseDatetime(startRaw, loc)
	if err != nil {
		return clockify.TimeEntry{}, nil, fmt.Errorf("invalid start: %w", err)
	}
	payload := map[string]any{"start": timeparse.FormatISO(startTime)}
	if endRaw := strings.TrimSpace(stringArg(args, "end")); endRaw != "" {
		endTime, err := timeparse.ParseDatetime(endRaw, loc)
		if err != nil {
			return clockify.TimeEntry{}, nil, fmt.Errorf("invalid end: %w", err)
		}
		if !endTime.After(startTime) {
			return clockify.TimeEntry{}, nil, fmt.Errorf("end must be after start")
		}
		payload["end"] = timeparse.FormatISO(endTime)
	}
	if desc := strings.TrimSpace(stringArg(args, "description")); desc != "" {
		payload["description"] = desc
	}
	projectID := strings.TrimSpace(stringArg(args, "project_id"))
	if projectID == "" {
		projectRef := strings.TrimSpace(stringArg(args, "project"))
		if projectRef != "" {
			projectID, err = s.resolveProjectID(ctx, s.WorkspaceID, projectRef)
			if err != nil {
				return clockify.TimeEntry{}, nil, err
			}
		}
	}
	if projectID != "" {
		if err := resolve.ValidateID(projectID, "project_id"); err != nil {
			return clockify.TimeEntry{}, nil, err
		}
		payload["projectId"] = projectID
	}
	taskID := strings.TrimSpace(stringArg(args, "task_id"))
	if taskID == "" {
		taskRef := strings.TrimSpace(stringArg(args, "task"))
		if taskRef != "" {
			if projectID == "" {
				return clockify.TimeEntry{}, nil, fmt.Errorf("project_id or project is required when resolving task by name")
			}
			taskID, err = s.resolveTaskID(ctx, s.WorkspaceID, projectID, taskRef)
			if err != nil {
				return clockify.TimeEntry{}, nil, err
			}
		}
	}
	if taskID != "" {
		if err := resolve.ValidateID(taskID, "task_id"); err != nil {
			return clockify.TimeEntry{}, nil, err
		}
		payload["taskId"] = taskID
	}
	tagIDs, err := s.tagIDsFromArgs(ctx, args)
	if err != nil {
		return clockify.TimeEntry{}, nil, err
	}
	if len(tagIDs) > 0 {
		payload["tagIds"] = tagIDs
	}
	if billable, ok := args["billable"].(bool); ok {
		payload["billable"] = billable
	}
	path, err := paths.Workspace(s.WorkspaceID, "time-entries")
	if err != nil {
		return clockify.TimeEntry{}, nil, err
	}
	var entry clockify.TimeEntry
	if err := s.Client.Post(ctx, path, payload, &entry); err != nil {
		return clockify.TimeEntry{}, nil, err
	}
	ids := map[string]string{
		"workspaceId": s.WorkspaceID,
		"entryId":     entry.ID,
		"projectId":   projectID,
		"taskId":      taskID,
	}
	if len(tagIDs) == 1 {
		ids["tagId"] = tagIDs[0]
	}
	return entry, ids, nil
}

func (s *Service) projectIDFromArgs(ctx context.Context, args map[string]any) (string, error) {
	projectID := strings.TrimSpace(stringArg(args, "project_id"))
	if projectID != "" {
		if err := resolve.ValidateID(projectID, "project_id"); err != nil {
			return "", err
		}
		return projectID, nil
	}
	projectRef := strings.TrimSpace(stringArg(args, "project"))
	if projectRef == "" {
		return "", fmt.Errorf("project_id or project is required")
	}
	return s.resolveProjectID(ctx, s.WorkspaceID, projectRef)
}

func (s *Service) tagIDsFromArgs(ctx context.Context, args map[string]any) ([]string, error) {
	out := stringSliceArg(args, "tag_ids")
	for _, id := range out {
		if err := resolve.ValidateID(id, "tag_id"); err != nil {
			return nil, err
		}
	}
	if tag := strings.TrimSpace(stringArg(args, "tag")); tag != "" {
		tagID, err := s.resolveTagID(ctx, s.WorkspaceID, tag)
		if err != nil {
			return nil, err
		}
		out = append(out, tagID)
	}
	return out, nil
}

func (s *Service) ensureDemoClient(ctx context.Context, prefix string, upsert bool) (clockify.ClientEntity, bool, error) {
	name := prefix + " Client"
	if upsert {
		clients, _, _, err := s.listClients(ctx, map[string]any{"page_size": float64(200)})
		if err != nil {
			return clockify.ClientEntity{}, false, err
		}
		for _, c := range clients {
			if c.Name == name {
				return c, true, nil
			}
		}
	}
	client, err := s.createClient(ctx, name)
	return client, false, err
}

func (s *Service) ensureDemoProject(ctx context.Context, prefix, clientID string, upsert bool) (clockify.Project, bool, error) {
	name := prefix + " Project"
	if upsert {
		projects, _, _, err := s.listProjects(ctx, map[string]any{"page_size": float64(200)})
		if err != nil {
			return clockify.Project{}, false, err
		}
		for _, p := range projects {
			if p.Name == name {
				return p, true, nil
			}
		}
	}
	project, _, err := s.createProject(ctx, map[string]any{"client_id": clientID, "billable": true}, name)
	return project, false, err
}

func (s *Service) ensureDemoTask(ctx context.Context, prefix, projectID string, upsert bool) (clockify.Task, bool, error) {
	name := prefix + " Task"
	if upsert {
		tasks, _, _, err := s.listTasks(ctx, projectID, map[string]any{"page_size": float64(200)})
		if err != nil {
			return clockify.Task{}, false, err
		}
		for _, t := range tasks {
			if t.Name == name {
				return t, true, nil
			}
		}
	}
	task, err := s.createTask(ctx, projectID, name, map[string]any{"billable": true})
	return task, false, err
}

func (s *Service) ensureDemoTag(ctx context.Context, prefix string, upsert bool) (clockify.Tag, bool, error) {
	name := prefix + " Tag"
	if upsert {
		tags, _, _, err := s.listTags(ctx, map[string]any{"page_size": float64(200)})
		if err != nil {
			return clockify.Tag{}, false, err
		}
		for _, t := range tags {
			if t.Name == name {
				return t, true, nil
			}
		}
	}
	tag, err := s.createTag(ctx, name)
	return tag, false, err
}

func (s *Service) ensureDemoEntry(ctx context.Context, args map[string]any, prefix, projectID, taskID, tagID string, upsert bool) (clockify.TimeEntry, bool, error) {
	date := strings.TrimSpace(stringArg(args, "date"))
	if date == "" {
		date = "2026-01-02"
	}
	description := prefix + " Time Entry"
	entryArgs := map[string]any{
		"start":       date + " 09:00",
		"end":         date + " 10:00",
		"description": description,
		"project_id":  projectID,
		"task_id":     taskID,
		"tag_ids":     []any{tagID},
		"billable":    true,
	}
	if upsert {
		entries, _, _, _, err := s.listCurrentUserEntries(ctx, map[string]any{
			"start": date + " 00:00",
			"end":   date + " 23:59",
		})
		if err != nil {
			return clockify.TimeEntry{}, false, err
		}
		for _, e := range entries {
			if e.Description == description {
				return e, true, nil
			}
		}
	}
	entry, _, err := s.createEntry(ctx, entryArgs)
	return entry, false, err
}

func addChanged(changed *ChangeSet, reused bool, ref EntityRef) {
	if reused {
		changed.Reused = append(changed.Reused, ref)
		return
	}
	changed.Created = append(changed.Created, ref)
}

func (s *Service) deleteDemoTask(ctx context.Context, projectID string, task clockify.Task, changed *ChangeSet, warnings *[]Warning) {
	if err := resolve.ValidateID(projectID, "project_id"); err != nil {
		*warnings = append(*warnings, Warning{Code: "delete_failed", Message: fmt.Sprintf("task %s: %v", task.ID, err)})
		return
	}
	if err := resolve.ValidateID(task.ID, "task_id"); err != nil {
		*warnings = append(*warnings, Warning{Code: "delete_failed", Message: fmt.Sprintf("task %s: %v", task.ID, err)})
		return
	}
	path, err := paths.Workspace(s.WorkspaceID, "projects", projectID, "tasks", task.ID)
	if err == nil {
		err = s.Client.Delete(ctx, path)
	}
	if err != nil {
		*warnings = append(*warnings, Warning{Code: "delete_failed", Message: fmt.Sprintf("task %s: %v", task.ID, err)})
		return
	}
	changed.Deleted = append(changed.Deleted, taskRef(task))
}

func (s *Service) deleteDemo(ctx context.Context, entity, id, name string, changed *ChangeSet, warnings *[]Warning) {
	if err := resolve.ValidateID(id, entity+"_id"); err != nil {
		*warnings = append(*warnings, Warning{Code: "delete_failed", Message: fmt.Sprintf("%s %s: %v", entity, id, err)})
		return
	}
	var path string
	var err error
	switch entity {
	case "entry":
		path, err = paths.Workspace(s.WorkspaceID, "time-entries", id)
	case "tag":
		path, err = paths.Workspace(s.WorkspaceID, "tags", id)
	case "project":
		path, err = paths.Workspace(s.WorkspaceID, "projects", id)
	case "client":
		path, err = paths.Workspace(s.WorkspaceID, "clients", id)
	default:
		err = fmt.Errorf("unknown cleanup entity %q", entity)
	}
	if err == nil {
		err = s.Client.Delete(ctx, path)
	}
	if err != nil {
		*warnings = append(*warnings, Warning{Code: "delete_failed", Message: fmt.Sprintf("%s %s: %v", entity, id, err)})
		return
	}
	changed.Deleted = append(changed.Deleted, EntityRef{Type: entity, ID: id, Name: name})
}

func cleanupEntryRangeArgs(args map[string]any) map[string]any {
	out := map[string]any{"page_size": float64(200)}
	if start := strings.TrimSpace(stringArg(args, "start")); start != "" {
		out["start"] = start
	} else {
		out["start"] = "2026-01-01 00:00"
	}
	if end := strings.TrimSpace(stringArg(args, "end")); end != "" {
		out["end"] = end
	} else {
		out["end"] = time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	}
	return out
}

func pageQuery(page, pageSize int) map[string]string {
	return map[string]string{
		"page":      strconv.Itoa(page),
		"page-size": strconv.Itoa(pageSize),
	}
}

func demoPrefix(args map[string]any) string {
	if prefix := strings.TrimSpace(stringArg(args, "prefix")); prefix != "" {
		return prefix
	}
	runID := strings.TrimSpace(stringArg(args, "run_id"))
	if runID == "" {
		runID = demoDefaultRunID
	}
	return "DEMO-" + runID
}

func (s *Service) location() *time.Location {
	if s.DefaultTimezone != nil {
		return s.DefaultTimezone
	}
	return time.UTC
}

func (s *Service) timezoneName() string {
	if s.DefaultTimezone != nil {
		return s.DefaultTimezone.String()
	}
	return "UTC"
}

func clientRef(c clockify.ClientEntity) EntityRef {
	return EntityRef{Type: "client", ID: c.ID, Name: c.Name}
}

func projectRef(p clockify.Project) EntityRef {
	return EntityRef{Type: "project", ID: p.ID, Name: p.Name}
}

func taskRef(t clockify.Task) EntityRef {
	return EntityRef{Type: "task", ID: t.ID, Name: t.Name}
}

func tagRef(t clockify.Tag) EntityRef {
	return EntityRef{Type: "tag", ID: t.ID, Name: t.Name}
}

func entryRef(e clockify.TimeEntry) EntityRef {
	return EntityRef{Type: "entry", ID: e.ID, Name: e.Description}
}
