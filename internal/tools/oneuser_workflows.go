package tools

import (
	"maps"

	"github.com/apet97/go-clockify/internal/mcp"
)

func (s *Service) workflowDescriptors() []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		defaultTier(advertiseOutputSchema(workflowDescriptor(0, "workspace", "workspace", []string{"Check identity, workspace, timezone, timer state, and next tools."}, nil,
			toolRO("clockify_status", "Show current user, pinned workspace, timezone, features, and current timer.", objectSchema(nil)), s.ClockifyStatus))),
		defaultTier(workflowDescriptor(1, "guide", "tool", []string{"Choose the best workflow or domain tool for a task."}, []string{"clockify_api_get", "clockify_api_request"},
			toolRO("clockify_tools_guide", "Show grouped workflow and domain tools with common task guidance.", objectSchema(nil)), s.ClockifyToolsGuide)),
		defaultTier(advertiseOutputSchema(workflowDescriptor(2, "work_tracking", "work_package", []string{"Create or reuse client, project, task, and tag objects in one call."}, []string{"clockify_clients_create", "clockify_projects_create", "clockify_tasks_create", "clockify_tags_create"},
			toolRWIdem("clockify_create_work_package", "Create or reuse a client/project/task/tag work package from names or IDs.", workPackageSchema()), s.ClockifyCreateWorkPackage))),
		defaultTier(advertiseOutputSchema(workflowDescriptor(3, "work_tracking", "entry", []string{"Log finished work with project, task, and tag names or IDs."}, []string{"clockify_entries_create", "clockify_api_request"},
			toolRW("clockify_log_work", "Log a finished time entry using human-friendly names or returned IDs.", logWorkSchema()), s.ClockifyLogWork))),
		defaultTier(advertiseOutputSchema(workflowDescriptor(4, "work_tracking", "entry", []string{"Start a running timer with project, task, and tag names or IDs."}, []string{"clockify_entries_timer_start", "clockify_entries_create"},
			toolRW("clockify_start_work", "Start a running work timer using names or IDs.", startWorkSchema()), s.ClockifyStartWork))),
		defaultTier(advertiseOutputSchema(workflowDescriptor(5, "work_tracking", "entry", []string{"Stop the current running timer."}, []string{"clockify_entries_timer_stop"},
			toolRWIdem("clockify_stop_work", "Stop the current running work timer.", stopWorkSchema()), s.ClockifyStopWork))),
		defaultTier(advertiseOutputSchema(workflowDescriptor(6, "work_tracking", "entry", []string{"Stop current work and start another timer in one call."}, []string{"clockify_stop_work", "clockify_start_work", "clockify_entries_timer_switch"},
			toolRW("clockify_switch_work", "Switch the running timer to another work item using names or IDs.", switchWorkSchema()), s.ClockifySwitchWork))),
		defaultTier(advertiseOutputSchema(workflowDescriptor(7, "review", "entry", []string{"Review one day for totals, missing fields, gaps, overlaps, and next actions."}, []string{"clockify_reports_detailed", "clockify_entries_list"},
			toolRO("clockify_review_day", "Review one day of work and return totals, issues, and next actions.", reviewDaySchema()), s.ClockifyReviewDay))),
		defaultTier(advertiseOutputSchema(workflowDescriptor(8, "review", "entry", []string{"Review one week for totals, missing fields, gaps, overlaps, and next actions."}, []string{"clockify_reports_weekly", "clockify_entries_list"},
			toolRO("clockify_review_week", "Review one week of work and return totals, issues, and next actions.", reviewWeekSchema()), s.ClockifyReviewWeek))),
		defaultTier(advertiseOutputSchema(workflowDescriptor(9, "work_tracking", "entry", []string{"Find and fix one time entry without hand-chaining get/update calls."}, []string{"clockify_entries_get", "clockify_entries_update", oneUserToolFixEntry},
			toolRWIdem("clockify_fix_entry", "Find one entry by ID or strict filters, then update selected fields.", fixEntrySchema()), s.ClockifyFixEntry))),
		advertiseOutputSchema(workflowDescriptor(10, "billing", "invoice", []string{"Create an invoice shell for a client and optionally import time entries."}, []string{"clockify_invoices_create", "clockify_invoices_import_time"},
			toolRW("clockify_invoice_client_work", "Billing workflow: create an invoice for a client from a name or ID, degrading gracefully when invoicing is unavailable. Supports dry_run preview.", invoiceClientWorkSchema()), s.ClockifyInvoiceClientWork)),
		advertiseOutputSchema(workflowDescriptor(11, "expenses", "expense", []string{"Record an expense with category/project names or IDs."}, []string{"clockify_expenses_create"},
			toolRW("clockify_record_expense", "Billing workflow: record an expense with category, project, task, and user names or IDs. Supports dry_run preview.", recordExpenseSchema()), s.ClockifyRecordExpense)),
		advertiseOutputSchema(workflowDescriptor(12, "time_off", "time_off_request", []string{"Request time off with a policy name or ID."}, []string{"clockify_time_off_requests_create"},
			toolRW("clockify_request_time_off", "Admin workflow: create a time-off request with a policy name or ID; this can enter approval workflows and affect PTO balances. Supports dry_run preview.", requestTimeOffSchema()), s.ClockifyRequestTimeOff)),
		advertiseOutputSchema(workflowDescriptor(13, "scheduling", "assignment", []string{"Schedule a user on a project with names or IDs."}, []string{"clockify_scheduling_assignments_create"},
			toolRW("clockify_schedule_work", "Admin scheduling workflow: create an assignment with user/project names or IDs. Supports dry_run preview.", scheduleWorkSchema()), s.ClockifyScheduleWork)),
		advertiseOutputSchema(workflowDescriptor(14, "webhooks", "webhook", []string{"Create a webhook from a simple name, URL, and event."}, []string{"clockify_webhooks_create"},
			toolRW("clockify_setup_webhook", "External-side-effect workflow: create a webhook subscription for this workspace and future outbound deliveries. Supports dry_run preview.", setupWebhookSchema()), s.ClockifySetupWebhook)),
		workflowDescriptor(15, "demo", "demo", []string{"Create a deterministic full fixture for smoke testing."}, []string{"clockify_clients_create", "clockify_projects_create", "clockify_tasks_create", "clockify_tags_create", "clockify_entries_create"},
			toolRWIdem("clockify_demo_seed", "Create or reuse deterministic demo client/project/task/tag/time-entry objects.", demoSeedSchema()), s.ClockifyDemoSeed),
		workflowDescriptor(16, "demo", "demo", []string{"Clean deterministic demo objects repeatedly."}, []string{"clockify_clients_delete", "clockify_projects_delete", "clockify_tasks_delete", "clockify_tags_delete", "clockify_entries_delete"},
			toolRWIdem("clockify_demo_cleanup", "Delete deterministic demo objects by prefix, continuing through partial failures.", demoCleanupSchema()), s.ClockifyDemoCleanup),
	}
}

func advertiseOutputSchema(d mcp.ToolDescriptor) mcp.ToolDescriptor {
	d.AdvertiseOutputSchema = true
	return d
}

func workflowDescriptor(priority int, domain, entity string, bestFor, preferOver []string, tool mcp.Tool, handler mcp.ToolHandler) mcp.ToolDescriptor {
	d := firstSliceDescriptor(priority, tool, handler)
	ann := d.Tool.Annotations
	if ann == nil {
		ann = map[string]any{}
		d.Tool.Annotations = ann
	}
	ann["category"] = "workflow"
	ann["preferred"] = true
	ann["priority"] = priority
	ann["handlerKind"] = "native handler"
	ann["bestFor"] = bestFor
	ann["preferOver"] = preferOver
	ann["domain"] = domain
	ann["entity"] = entity
	return d
}

func workPackageSchema() map[string]any {
	return objectSchema(map[string]any{"properties": map[string]any{
		"client":     map[string]any{"type": "string", "description": "Client name or ID."},
		"client_id":  map[string]any{"type": "string"},
		"project":    map[string]any{"type": "string", "description": "Project name or ID."},
		"project_id": map[string]any{"type": "string"},
		"task":       map[string]any{"type": "string", "description": "Task name or ID."},
		"task_id":    map[string]any{"type": "string"},
		"tag":        map[string]any{"type": "string", "description": "Tag name or ID."},
		"tags":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"tag_ids":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"color":      map[string]any{"type": "string"},
		"billable":   map[string]any{"type": "boolean"},
		"is_public":  map[string]any{"type": "boolean"},
		"upsert":     map[string]any{"type": "boolean", "description": "Reuse exact existing objects before creating. Default: true."},
	}})
}

func logWorkSchema() map[string]any {
	return objectSchema(map[string]any{"required": []string{"start", "end"}, "properties": map[string]any{
		"start":         map[string]any{"type": "string", "description": flexibleDatetimeDescription},
		"end":           map[string]any{"type": "string", "description": flexibleDatetimeDescription},
		"description":   map[string]any{"type": "string", "description": "What you are working on — free text shown on the time entry. Optional but recommended."},
		"project":       map[string]any{"type": "string"},
		"project_id":    map[string]any{"type": "string"},
		"task":          map[string]any{"type": "string"},
		"task_id":       map[string]any{"type": "string"},
		"tag":           map[string]any{"type": "string"},
		"tag_ids":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"custom_fields": entryCustomFieldsInputSchema(),
		"billable":      map[string]any{"type": "boolean"},
		"allow_overlap": map[string]any{"type": "boolean", "description": "When false (default) the entry is rejected if it overlaps an existing entry; set true to override after confirming the overlap is intentional."},
	}})
}

func startWorkSchema() map[string]any {
	schema := logWorkSchema()
	delete(schema, "required")
	props, _ := schema["properties"].(map[string]any)
	props["start"] = map[string]any{"type": "string", "description": flexibleDatetimeDescription + ". Default: now."}
	delete(props, "end")
	delete(props, "allow_overlap")
	return schema
}

func stopWorkSchema() map[string]any {
	return objectSchema(map[string]any{"properties": map[string]any{
		"end": map[string]any{"type": "string", "description": flexibleDatetimeDescription + ". Default: now."},
	}})
}

func switchWorkSchema() map[string]any {
	schema := startWorkSchema()
	schema["required"] = []string{"project"}
	return schema
}

func reviewDaySchema() map[string]any {
	return objectSchema(map[string]any{"properties": map[string]any{
		"date":            map[string]any{"type": "string", "description": "YYYY-MM-DD. Default: today."},
		"start":           map[string]any{"type": "string", "description": flexibleDatetimeDescription},
		"end":             map[string]any{"type": "string", "description": flexibleDatetimeDescription},
		"timezone":        map[string]any{"type": "string"},
		"workday_start":   map[string]any{"type": "string", "description": "HH:MM. Default: 09:00."},
		"workday_end":     map[string]any{"type": "string", "description": "HH:MM. Default: 17:00."},
		"min_gap_minutes": map[string]any{"type": "integer"},
		"include_entries": map[string]any{"type": "boolean"},
		"max_rows":        map[string]any{"type": "integer", "minimum": 0, "description": "Maximum detail rows for byProject, issues, and included entries. Default: 15; pass 0 for uncapped."},
	}})
}

func reviewWeekSchema() map[string]any {
	schema := reviewDaySchema()
	props, _ := schema["properties"].(map[string]any)
	props["week_start"] = map[string]any{"type": "string", "description": "Any date in the week to review. Default: current week."}
	delete(props, "date")
	return schema
}

func fixEntrySchema() map[string]any {
	return objectSchema(map[string]any{"properties": map[string]any{
		"entry_id":             map[string]any{"type": "string"},
		"description_contains": map[string]any{"type": "string"},
		"exact_description":    map[string]any{"type": "string"},
		"start_after":          map[string]any{"type": "string", "description": flexibleDatetimeDescription},
		"start_before":         map[string]any{"type": "string", "description": flexibleDatetimeDescription},
		"description":          map[string]any{"type": "string", "description": "New description."},
		"new_description":      map[string]any{"type": "string"},
		"project":              map[string]any{"type": "string"},
		"project_id":           map[string]any{"type": "string"},
		"start":                map[string]any{"type": "string", "description": flexibleDatetimeDescription},
		"end":                  map[string]any{"type": "string", "description": flexibleDatetimeDescription},
		"billable":             map[string]any{"type": "boolean"},
	}})
}

func invoiceClientWorkSchema() map[string]any {
	return objectSchema(map[string]any{
		"required": []string{"currency", "client"},
		"properties": map[string]any{
			"client":                map[string]any{"type": "string", "description": "Client name or ID. Required."},
			"client_id":             map[string]any{"type": "string", "description": "Optional ID-only alias for client; prefer client, which already accepts an ID."},
			"number":                map[string]any{"type": "string"},
			"issued_date":           map[string]any{"type": "string", "description": "Invoice issued date (YYYY-MM-DD or RFC3339). Defaults to today."},
			"due_date":              map[string]any{"type": "string", "description": "Invoice due date (YYYY-MM-DD or RFC3339). Defaults to 14 days out."},
			"currency":              map[string]any{"type": "string", "description": "Currency code, e.g. USD or EUR (required)."},
			"note":                  map[string]any{"type": "string"},
			"from":                  map[string]any{"type": "string", "description": "Start of the billing period to import (YYYY-MM-DD or RFC3339). Pass both from and to to import billable time onto the invoice."},
			"to":                    map[string]any{"type": "string", "description": "End of the billing period to import (YYYY-MM-DD or RFC3339)."},
			"project_ids":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional. Limit the imported time to these project IDs."},
			"time_entry_group_type": map[string]any{"type": "string"},
			"dry_run":               map[string]any{"type": "boolean"},
		},
	})
}

func recordExpenseSchema() map[string]any {
	return objectSchema(map[string]any{"required": []string{"amount"}, "properties": map[string]any{
		"amount":      map[string]any{"type": "number"},
		"date":        map[string]any{"type": "string", "description": "Default: now."},
		"category":    map[string]any{"type": "string", "description": "Expense category name. Provide either category or category_id (one is required)."},
		"category_id": map[string]any{"type": "string", "description": "Expense category ID. Provide either category or category_id (one is required)."},
		"project":     map[string]any{"type": "string"},
		"project_id":  map[string]any{"type": "string"},
		"task_id":     map[string]any{"type": "string"},
		"user_id":     map[string]any{"type": "string"},
		"notes":       map[string]any{"type": "string"},
		"billable":    map[string]any{"type": "boolean"},
		"dry_run":     map[string]any{"type": "boolean", "description": "Preview the resolved expense payload without creating the expense."},
	}})
}

func requestTimeOffSchema() map[string]any {
	return objectSchema(map[string]any{"required": []string{"start", "end"}, "properties": map[string]any{
		"policy":    map[string]any{"type": "string", "description": "Time-off policy name. Provide either policy or policy_id (one is required)."},
		"policy_id": map[string]any{"type": "string", "description": "Time-off policy ID. Provide either policy or policy_id (one is required)."},
		"start":     map[string]any{"type": "string"},
		"end":       map[string]any{"type": "string"},
		"note":      map[string]any{"type": "string"},
		"half_day":  map[string]any{"type": "boolean"},
		"dry_run":   map[string]any{"type": "boolean", "description": "Preview the resolved time-off request payload without creating it."},
	}})
}

func scheduleWorkSchema() map[string]any {
	return objectSchema(map[string]any{"required": []string{"start", "end", "hours_per_day", "user", "project"}, "properties": map[string]any{
		"user":                     map[string]any{"type": "string", "description": "User name, email, or ID. Required."},
		"user_id":                  map[string]any{"type": "string", "description": "Optional ID-only alias for user; prefer user, which already accepts an ID."},
		"project":                  map[string]any{"type": "string", "description": "Project name or ID. Required."},
		"project_id":               map[string]any{"type": "string", "description": "Optional ID-only alias for project; prefer project, which already accepts an ID."},
		"start":                    map[string]any{"type": "string", "description": flexibleDatetimeDescription},
		"end":                      map[string]any{"type": "string", "description": flexibleDatetimeDescription},
		"hours_per_day":            map[string]any{"type": "number", "minimum": 0.5, "maximum": 24, "description": "Work hours per day (0.5-24)."},
		"billable":                 map[string]any{"type": "boolean"},
		"include_non_working_days": map[string]any{"type": "boolean"},
		"start_time":               map[string]any{"type": "string"},
		"task_id":                  map[string]any{"type": "string"},
		"note":                     map[string]any{"type": "string"},
		"repeat":                   map[string]any{"type": "boolean"},
		"weeks":                    map[string]any{"type": "integer", "minimum": 1, "maximum": 99, "description": "Repeat interval in weeks when repeat is true. Default: 1."},
		"dry_run":                  map[string]any{"type": "boolean", "description": "Preview the resolved scheduling assignment payload without creating it."},
	}})
}

func setupWebhookSchema() map[string]any {
	return objectSchema(map[string]any{"required": []string{"name", "url"}, "properties": map[string]any{
		"name":                map[string]any{"type": "string"},
		"url":                 map[string]any{"type": "string", "format": "uri", "description": "HTTPS webhook destination URL. Private, loopback, localhost, and credential-bearing URLs are rejected before upstream calls."},
		"event":               map[string]any{"type": "string", "enum": append([]string(nil), webhookEventEnum...), "description": "Optional alias for webhook_event."},
		"webhook_event":       map[string]any{"type": "string", "enum": append([]string(nil), webhookEventEnum...), "description": "Webhook event type, e.g. NEW_TIME_ENTRY. Required."},
		"trigger_source_type": map[string]any{"type": "string"},
		"trigger_source":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"dry_run":             map[string]any{"type": "boolean"},
	}})
}

func demoSeedSchema() map[string]any {
	return objectSchema(map[string]any{"properties": map[string]any{
		"run_id":        map[string]any{"type": "string", "description": "Stable run id. Default: phase1."},
		"prefix":        map[string]any{"type": "string", "description": "Explicit object-name prefix. Default: DEMO-<run_id>."},
		"date":          map[string]any{"type": "string", "description": "YYYY-MM-DD date for the demo time entry. Default: 2026-01-02."},
		"upsert":        map[string]any{"type": "boolean", "description": "Reuse existing prefixed objects. Default: true."},
		"custom_fields": entryCustomFieldsInputSchema(),
	}})
}

func demoCleanupSchema() map[string]any {
	return objectSchema(map[string]any{"properties": map[string]any{
		"run_id": map[string]any{"type": "string", "description": "Stable run id. Default: phase1."},
		"prefix": map[string]any{"type": "string", "description": "Explicit object-name prefix. Default: DEMO-<run_id>."},
		"start":  map[string]any{"type": "string", "description": flexibleDatetimeDescription},
		"end":    map[string]any{"type": "string", "description": flexibleDatetimeDescription},
	}})
}

func copyArgs(args map[string]any) map[string]any {
	out := make(map[string]any, len(args))
	maps.Copy(out, args)
	return out
}
