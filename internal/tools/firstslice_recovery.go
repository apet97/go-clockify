package tools

import (
	"errors"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
)

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
	rawMessage := err.Error()
	message := rawMessage
	var apiErr *clockify.APIError
	if errors.As(err, &apiErr) {
		message = apiErr.ClientFacingMessage()
		switch apiErr.StatusCode {
		case 400:
			code = "invalid_request"
		case 402:
			code = "feature_unavailable"
		case 401, 403:
			code = "auth_or_permission"
			if looksLikeFeatureUnavailable(rawMessage) {
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
	} else if strings.Contains(strings.ToLower(message), "unsupported:") || strings.Contains(strings.ToLower(message), "does not expose") {
		code = "unsupported"
	}
	if strings.Contains(rawMessage, "requires a project on every time entry") {
		recovery = RecoveryHint{
			Hint: "This workspace requires a project on every time entry. List projects, then retry with project_id or project.",
			Tool: "clockify_projects_list",
		}
	} else if recovery.Hint == "" {
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
	for _, needle := range []string{"feature", "paid", "plan", "subscription", "upgrade", "not available", "not supported", "no static resource"} {
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
	case strings.Contains(action, "reports_"), strings.Contains(action, "_report"):
		return RecoveryHint{Hint: "Reports need an explicit date range. Pass date_range_start and date_range_end as YYYY-MM-DD (the money and summary reports also accept an optional summary_filter object). The weekly report needs start and end exactly 7 days apart.", Tool: "clockify_reports_summary"}
	case action == "clockify_invoices_mark_paid":
		return RecoveryHint{Hint: "Clockify records paid invoices through payments. Create a payment with invoice_id, amount, and date, then reload the invoice.", Tool: "clockify_invoices_payments_create"}
	case action == "clockify_invoices_send_guidance":
		return RecoveryHint{Hint: "Clockify does not expose invoice email sending in this API surface. Use the Clockify UI for email delivery, or inspect the invoice with the get tool.", Tool: "clockify_invoices_get"}
	case action == "clockify_webhooks_test_guidance":
		return RecoveryHint{Hint: "Clockify does not expose a webhook test-send endpoint. Trigger a real event or inspect delivery logs in the Clockify UI.", Tool: "clockify_webhooks_get"}
	case action == "clockify_invoices_items_update_guidance":
		return RecoveryHint{Hint: "Clockify has no update endpoint for invoice line items. Delete the line with clockify_invoices_items_delete, then re-add it with clockify_invoices_items_add.", Tool: "clockify_invoices_items_delete"}
	case strings.Contains(action, "invoice"):
		return RecoveryHint{Hint: "If invoicing is unavailable, report that and continue. Otherwise list clients or invoices, then retry with returned IDs.", Tool: "clockify_invoices_list"}
	case strings.Contains(action, "expense"):
		return RecoveryHint{Hint: "If expenses are unavailable, report that and continue. Otherwise list expense categories and retry with returned IDs.", Tool: "clockify_expenses_categories_list"}
	case strings.Contains(action, "time_off"):
		return RecoveryHint{Hint: "If time off is unavailable, report that and continue. Otherwise list policies and retry with a returned policy ID.", Tool: "clockify_time_off_policies_list"}
	case action == "clockify_schedule_work":
		// The schedule_work workflow tool fails on an invalid project/user
		// ID; listing existing assignments does not resolve those, so route
		// to the project list (then resolve the user) — not the domain tool.
		return RecoveryHint{Hint: "Scheduling writes can return 403 even for the workspace owner — Clockify may require a manager/admin role or a published schedule. If scheduling is unavailable, report that and continue. Otherwise verify the project and user IDs, then retry.", Tool: "clockify_projects_list"}
	case strings.Contains(action, "scheduling"):
		// The clockify_scheduling_* domain family. The substring is
		// "scheduling", not "schedule": the workflow tool above is the only
		// "schedule" action, and the domain tools never contain it — so a
		// "schedule" substring here would have matched nothing.
		return RecoveryHint{Hint: "Scheduling writes can return 403 even for the workspace owner — Clockify may require a manager/admin role or a published schedule. If scheduling is unavailable, report that and continue. Otherwise list users/projects and retry with returned IDs.", Tool: "clockify_scheduling_assignments_list"}
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
	case action == "clockify_audit_logs_search":
		return RecoveryHint{Hint: `Audit log search failed. Keep the start/end window within 31 days, verify author IDs (or pass "SYSTEM"), then retry. Audit log can also be plan-gated.`, Tool: "clockify_users_list"}
	case action == "clockify_entity_changes_list":
		return RecoveryHint{Hint: "Entity-changes list failed. Verify change_type, entity_types, and the start/end window, then retry.", Tool: "clockify_status"}
	default:
		return RecoveryHint{Hint: "Check the returned error, call clockify_status, then retry with IDs returned by previous calls.", Tool: "clockify_status"}
	}
}
