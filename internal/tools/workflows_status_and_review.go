package tools

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *Service) ClockifyToolsGuide(_ context.Context, _ map[string]any) (any, error) {
	return result("clockify_tools_guide", "tool_guide", map[string]string{"workspaceId": s.WorkspaceID}, map[string]any{
		"workflows": []map[string]any{
			{"group": "orientation", "tools": []string{"clockify_status", "clockify_tools_guide"}, "useFor": []string{"first call", "tool choice", "current timer status"}},
			{"group": "work tracking", "tools": []string{"clockify_create_work_package", "clockify_log_work", "clockify_start_work", "clockify_stop_work", "clockify_switch_work", "clockify_fix_entry"}, "useFor": []string{"create reusable work objects", "log finished work", "run timers", "fix entries"}},
			{"group": "review", "tools": []string{"clockify_review_day", "clockify_review_week"}, "useFor": []string{"daily totals", "weekly totals", "gaps", "overlaps", "missing projects or descriptions"}},
			{"group": "business workflows", "tools": []string{"clockify_invoice_client_work", "clockify_record_expense", "clockify_request_time_off", "clockify_schedule_work", "clockify_setup_webhook"}, "useFor": []string{"billing", "expenses", "leave requests", "scheduling", "webhook setup"}},
			{"group": "demo", "tools": []string{"clockify_demo_seed", "clockify_demo_cleanup"}, "useFor": []string{"deterministic smoke fixtures", "repeatable cleanup"}},
		},
		"commonTasks": []map[string]any{
			{"task": "Start using the MCP", "tool": "clockify_status"},
			{"task": "Create a project/task/tag bundle", "tool": "clockify_create_work_package"},
			{"task": "Log time from plain names", "tool": "clockify_log_work"},
			{"task": "Review a week and decide what to fix", "tool": "clockify_review_week"},
			{"task": "Use a paid feature that may not be enabled", "tool": "workflow tool first; if unavailable, continue with returned recovery"},
		},
		"domainTools": workflowDomainGroups(),
		"rawFallback": []string{"clockify_api_get", "clockify_api_request"},
		"rulesOfThumb": []string{
			"Keep CLOCKIFY_TOOLSET=default for first contact; it advertises the everyday workflow and orientation tools.",
			"Use workflow tools first.",
			"Use IDs returned by previous calls when available.",
			"Use domain tools for precise CRUD only after a workflow result points there or the task clearly needs a specific resource.",
			"Use raw API tools last, only when no workflow or typed domain tool fits.",
			"CLOCKIFY_TOOLSET=all is for expert/debug sessions, not first-run onboarding.",
		},
	}, ChangeSet{}, nil, []NextAction{
		{Tool: "clockify_status", Reason: "Verify the pinned workspace and current timer before making changes."},
		{Tool: "clockify_create_work_package", Reason: "Set up reusable work objects for future logging."},
	}), nil
}

func workflowDomainGroups() []map[string]any {
	return []map[string]any{
		{"domain": "clients", "prefix": "clockify_clients_", "verbs": []string{"list", "get", "create", "update", "delete"}},
		{"domain": "projects", "prefix": "clockify_projects_", "verbs": []string{"list", "get", "create", "update", "delete", "archive", "templates", "estimates", "memberships", "rates"}},
		{"domain": "tasks", "prefix": "clockify_tasks_", "verbs": []string{"list", "get", "create", "update", "delete", "rates"}},
		{"domain": "tags", "prefix": "clockify_tags_", "verbs": []string{"list", "get", "create", "update", "delete"}},
		{"domain": "entries", "prefix": "clockify_entries_", "verbs": []string{"list", "get", "create", "update", "delete", "timer", "mark_invoiced"}},
		{"domain": "reports", "prefix": "clockify_reports_", "verbs": []string{"detailed", "summary", "weekly", "attendance", "money", "expense", "export"}},
		{"domain": "invoices", "prefix": "clockify_invoices_", "verbs": []string{"list", "get", "create", "update", "delete", "items", "import", "send", "mark_paid", "export", "payments"}},
		{"domain": "expenses", "prefix": "clockify_expenses_", "verbs": []string{"list", "get", "create", "update", "delete", "categories"}},
		{"domain": "admin", "prefixes": []string{"clockify_custom_fields_", "clockify_time_off_", "clockify_scheduling_", "clockify_approvals_", "clockify_webhooks_", "clockify_groups_", "clockify_holidays_", "clockify_users_", "clockify_workspace_"}},
	}
}

func (s *Service) ClockifyReviewDay(ctx context.Context, args map[string]any) (any, error) {
	reviewArgs := copyArgs(args)
	if strings.TrimSpace(stringArg(reviewArgs, "date")) == "" && strings.TrimSpace(stringArg(reviewArgs, "start")) == "" {
		reviewArgs["date"] = time.Now().In(s.location()).Format("2006-01-02")
	}
	return s.reviewWorkflow(ctx, "clockify_review_day", reviewArgs)
}

func (s *Service) ClockifyReviewWeek(ctx context.Context, args map[string]any) (any, error) {
	reviewArgs := copyArgs(args)
	if strings.TrimSpace(stringArg(reviewArgs, "week_start")) == "" && strings.TrimSpace(stringArg(reviewArgs, "start")) == "" {
		reviewArgs["week_start"] = time.Now().In(s.location()).Format("2006-01-02")
	}
	return s.reviewWorkflow(ctx, "clockify_review_week", reviewArgs)
}

func (s *Service) reviewWorkflow(ctx context.Context, action string, args map[string]any) (any, error) {
	out, err := s.TimesheetReview(ctx, args)
	if err != nil {
		return nil, err
	}
	standard := standardizeDomainResult(action, "entry_review", "", out, args)
	standard.Action = action
	standard.IDs = cleanIDs(map[string]string{
		"workspaceId": stringFromAny(out.Meta["workspaceId"]),
		"userId":      stringFromAny(out.Meta["userId"]),
	})
	if truncated, _ := out.Meta["truncated"].(bool); truncated {
		total, _ := out.Meta["issuesTotal"].(int)
		shown := total
		if r, ok := out.Meta["issuesReturned"].(int); ok {
			shown = r
		}
		if total > shown {
			standard.Warnings = append(standard.Warnings, Warning{
				Code:    "issues_truncated",
				Message: fmt.Sprintf("Only %d of %d issues are shown; raise max_rows or narrow the date range to see the rest.", shown, total),
			})
		}
	}
	standard.Next = nextFromReviewData(out.Data)
	if len(standard.Next) == 0 {
		standard.Next = []NextAction{{Tool: "clockify_log_work", Reason: "Log confirmed missing work if the review found gaps."}}
	}
	return standard, nil
}

func nextFromReviewData(data any) []NextAction {
	review, ok := data.(TimesheetReviewData)
	if !ok {
		return nil
	}
	out := make([]NextAction, 0, len(review.SuggestedActions)+2)
	for _, suggestion := range review.SuggestedActions {
		out = append(out, NextAction{
			Tool:   workflowPreferredTool(suggestion.Tool),
			Args:   suggestion.Arguments,
			Reason: suggestion.Reason,
		})
	}
	if review.Totals.RunningEntries > 0 {
		out = append(out, NextAction{Tool: "clockify_stop_work", Reason: "Stop the running timer if the session is complete."})
	}
	if len(out) == 0 {
		out = append(out, NextAction{Tool: "clockify_log_work", Reason: "Log any missing work discovered during review."})
	}
	return out
}

func workflowPreferredTool(tool string) string {
	switch tool {
	case "clockify_entries_timer_stop":
		return "clockify_stop_work"
	case "clockify_fix_entry", "clockify_entries_update":
		return "clockify_fix_entry"
	case "clockify_log_work", "clockify_entries_create":
		return "clockify_log_work"
	default:
		return tool
	}
}
