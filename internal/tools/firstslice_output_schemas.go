package tools

import (
	"fmt"

	"github.com/apet97/go-clockify/internal/clockify"
)

func objectSchema(overrides map[string]any) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
	for k, v := range overrides {
		if v == nil {
			continue
		}
		if k == "required" {
			switch s := v.(type) {
			case []string:
				if len(s) == 0 {
					continue
				}
			case []any:
				if len(s) == 0 {
					continue
				}
			}
		}
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
			"ok":        map[string]any{"type": "boolean"},
			"supported": map[string]any{"type": "boolean"},
			"performed": map[string]any{"type": "boolean"},
			"action":    map[string]any{"type": "string", "const": action},
			"entity":    map[string]any{"type": "string"},
			"ids":       map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
			"data":      dataSchema,
			"meta":      map[string]any{"type": "object", "additionalProperties": true},
			"changed":   schemaFor[ChangeSet](),
			"warnings":  schemaFor[[]Warning](),
			"next":      schemaFor[[]NextAction](),
			"error":     schemaFor[ErrorInfo](),
			"recovery":  schemaFor[RecoveryHint](),
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
		return timerStopDataSchema()
	case "clockify_fix_entry":
		return schemaFor[FindAndUpdateEntryData]()
	case "clockify_switch_work":
		return objectDataSchema(map[string]any{
			"status":  map[string]any{"type": "string", "enum": []string{"ok", "partial_failure"}, "description": "ok = the previous timer stopped and the new one started; partial_failure = stopped but the new timer did not start."},
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
		return objectListDataSchema("projects", schemaFor[[]CompactProjectView]())
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
	case "clockify_entries_timer_start":
		return schemaFor[EntryView]()
	case "clockify_entries_timer_stop":
		return timerStopDataSchema()
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
		return schemaFor[[]CompactInvoiceView]()
	case "clockify_invoices_get", "clockify_invoices_create", "clockify_invoices_update", "clockify_invoices_mark_paid":
		return entityObjectDataSchema("id", "number", "status", "clientId")
	case "clockify_invoices_send_guidance":
		return guidanceDataSchema()
	case "clockify_invoices_delete":
		return entityObjectDataSchema("deleted", "invoiceId")
	case "clockify_invoices_items_list":
		return entityArrayDataSchema("id", "invoiceItemId", "description", "quantity", "unitPrice")
	case "clockify_invoices_items_add":
		return entityObjectDataSchema("id", "invoiceItemId", "description", "quantity", "unitPrice")
	case "clockify_invoices_items_update_guidance":
		return guidanceDataSchema()
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
		return schemaFor[[]CompactExpenseView]()
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
		return schemaFor[[]CompactTimeOffPolicyView]()
	case "clockify_time_off_policies_get", "clockify_time_off_policies_create", "clockify_time_off_policies_update":
		return entityObjectDataSchema("id", "policyId", "name", "timeUnit", "archived")
	case "clockify_time_off_balances":
		return entityObjectDataSchema("policyId", "userId", "balance", "used", "available")
	case "clockify_time_off_balances_update":
		return entityObjectDataSchema("policyId", "userIds", "value", "note")
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
	case "clockify_webhooks_test_guidance":
		return guidanceDataSchema()
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
		return schemaFor[[]CompactUserView]()
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
		return dataKeysSchema("amount", "approval", "category", "categoryId", "currency", "date", "entities", "has_file", "id", "invoicing", "notes", "projectId", "receipt", "status", "taskId", "tax", "userId")
	case "clockify_request_time_off":
		return dataKeysSchema("actions", "audit", "day_period", "duration", "id", "note", "period", "policy", "policyId", "requestId", "status", "suggestedActions", "user", "userId")
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
	case "clockify_audit_logs_search":
		return entityArrayDataSchema("action", "timestamp", "userId", "userEmail", "userName", "content", "previousContent", "workspaceId")
	case "clockify_entity_changes_list":
		return map[string]any{
			"type": "array",
			"items": objectDataSchema(map[string]any{
				"id":           map[string]any{"type": "string"},
				"documentCode": map[string]any{"type": "string", "description": "Entity type code, e.g. PROJECTS."},
				"auditMetadata": objectDataSchema(map[string]any{
					"createdAt": map[string]any{"type": "string", "description": "When the entity was created (ISO 8601)."},
					"updatedAt": map[string]any{"type": "string", "description": "When the entity was last updated (ISO 8601)."},
				}),
				"document":  map[string]any{"description": "The entity document; fields vary by entity type."},
				"deletedAt": map[string]any{"type": "string", "description": "Deletion timestamp; present on the deleted feed."},
			}),
		}
	case "clockify_invoices_info":
		return schemaFor[[]CompactInvoiceView]()
	case "clockify_scheduling_publish":
		return objectDataSchema(map[string]any{
			"published":   map[string]any{"type": "boolean", "description": "True when the publish call succeeded."},
			"start":       map[string]any{"type": "string", "description": "Resolved publish-window start (RFC3339 UTC)."},
			"end":         map[string]any{"type": "string", "description": "Resolved publish-window end (RFC3339 UTC)."},
			"notifyUsers": map[string]any{"type": "boolean", "description": "Whether affected users were notified."},
		})
	case "clockify_api_get", "clockify_api_request":
		// Raw API fallback tools return whatever the Clockify endpoint
		// produced, so there is no typed data schema to advertise.
		return nil
	default:
		// Fail loud: a registered tool with no data-schema case is a bug —
		// it would otherwise ship an undocumented data payload. The panic
		// fires at registry-build time, so every test that builds the
		// registry catches a forgotten case.
		panic(fmt.Sprintf("firstSliceDataOutputSchema: no data output schema for tool %q — add a case", action))
	}
}

func objectDataSchema(properties map[string]any) map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"properties":           properties,
	}
}

func guidanceDataSchema() map[string]any {
	return objectDataSchema(map[string]any{
		"supported": map[string]any{"type": "boolean"},
		"performed": map[string]any{"type": "boolean"},
		"hint":      map[string]any{"type": "string"},
	})
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
		"total":    map[string]any{"type": "integer"},
		"page":     map[string]any{"type": "integer"},
		"pageSize": map[string]any{"type": "integer"},
		"has_more": map[string]any{"type": "boolean"},
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
	fields := []string{"contentType", "filename", "bytes", "bodyEncoding", "base64Bytes", "truncated", "body", "path"}
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

func timerStopDataSchema() map[string]any {
	return dataKeysSchema(
		"id", "description", "projectId", "projectName", "taskId", "tagIds",
		"billable", "billable_state", "billable_present",
		"costRate", "customFieldValues", "custom_fields_normalized", "hourlyRate",
		"isLocked", "kioskId", "type", "userId", "workspaceId", "timeInterval",
		"financials", "approval", "invoicing", "entities", "audit",
		"stopped", "reason",
	)
}

func arraySchema(description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description,
		"items":       map[string]any{"type": "object"},
	}
}
