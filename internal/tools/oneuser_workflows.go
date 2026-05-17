package tools

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/resolve"
	"github.com/apet97/go-clockify/internal/timeparse"
)

func (s *Service) workflowDescriptors() []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		workflowDescriptor(0, "workspace", "workspace", []string{"Check identity, workspace, timezone, timer state, and next tools."}, nil,
			toolRO("clockify_status", "Show current user, pinned workspace, timezone, features, and current timer.", objectSchema(nil)), s.ClockifyStatus),
		workflowDescriptor(1, "guide", "tool", []string{"Choose the best workflow or domain tool for a task."}, []string{"clockify_api_get", "clockify_api_request"},
			toolRO("clockify_tools_guide", "Show grouped workflow and domain tools with common task guidance.", objectSchema(nil)), s.ClockifyToolsGuide),
		workflowDescriptor(2, "work_tracking", "work_package", []string{"Create or reuse client, project, task, and tag objects in one call."}, []string{"clockify_clients_create", "clockify_projects_create", "clockify_tasks_create", "clockify_tags_create"},
			toolRWIdem("clockify_create_work_package", "Create or reuse a client/project/task/tag work package from names or IDs.", workPackageSchema()), s.ClockifyCreateWorkPackage),
		workflowDescriptor(3, "work_tracking", "entry", []string{"Log finished work with project, task, and tag names or IDs."}, []string{"clockify_entries_create", "clockify_api_request"},
			toolRW("clockify_log_work", "Log a finished time entry using human-friendly names or returned IDs.", logWorkSchema()), s.ClockifyLogWork),
		workflowDescriptor(4, "work_tracking", "entry", []string{"Start a running timer with project, task, and tag names or IDs."}, []string{"clockify_entries_timer_start", "clockify_entries_create"},
			toolRW("clockify_start_work", "Start a running work timer using names or IDs.", startWorkSchema()), s.ClockifyStartWork),
		workflowDescriptor(5, "work_tracking", "entry", []string{"Stop the current running timer."}, []string{"clockify_entries_timer_stop"},
			toolRWIdem("clockify_stop_work", "Stop the current running work timer.", stopWorkSchema()), s.ClockifyStopWork),
		workflowDescriptor(6, "work_tracking", "entry", []string{"Stop current work and start another timer in one call."}, []string{"clockify_stop_work", "clockify_start_work", "clockify_entries_timer_switch"},
			toolRW("clockify_switch_work", "Switch the running timer to another work item using names or IDs.", switchWorkSchema()), s.ClockifySwitchWork),
		workflowDescriptor(7, "review", "entry", []string{"Review one day for totals, missing fields, gaps, overlaps, and next actions."}, []string{"clockify_reports_detailed", "clockify_entries_list"},
			toolRO("clockify_review_day", "Review one day of work and return totals, issues, and next actions.", reviewDaySchema()), s.ClockifyReviewDay),
		workflowDescriptor(8, "review", "entry", []string{"Review one week for totals, missing fields, gaps, overlaps, and next actions."}, []string{"clockify_reports_weekly", "clockify_entries_list"},
			toolRO("clockify_review_week", "Review one week of work and return totals, issues, and next actions.", reviewWeekSchema()), s.ClockifyReviewWeek),
		workflowDescriptor(9, "work_tracking", "entry", []string{"Find and fix one time entry without hand-chaining get/update calls."}, []string{"clockify_entries_get", "clockify_entries_update", oneUserToolFixEntry},
			toolRWIdem("clockify_fix_entry", "Find one entry by ID or strict filters, then update selected fields.", fixEntrySchema()), s.ClockifyFixEntry),
		workflowDescriptor(10, "billing", "invoice", []string{"Create an invoice shell for a client and optionally import time entries."}, []string{"clockify_invoices_create", "clockify_invoices_import_time"},
			toolRW("clockify_invoice_client_work", "Create an invoice for a client from a name or ID, degrading gracefully when invoicing is unavailable.", invoiceClientWorkSchema()), s.ClockifyInvoiceClientWork),
		workflowDescriptor(11, "expenses", "expense", []string{"Record an expense with category/project names or IDs."}, []string{"clockify_expenses_create"},
			toolRW("clockify_record_expense", "Record an expense with category, project, task, and user names or IDs.", recordExpenseSchema()), s.ClockifyRecordExpense),
		workflowDescriptor(12, "time_off", "time_off_request", []string{"Request time off with a policy name or ID."}, []string{"clockify_time_off_requests_create"},
			toolRW("clockify_request_time_off", "Create a time-off request with a policy name or ID.", requestTimeOffSchema()), s.ClockifyRequestTimeOff),
		workflowDescriptor(13, "scheduling", "assignment", []string{"Schedule a user on a project with names or IDs."}, []string{"clockify_scheduling_assignments_create"},
			toolRW("clockify_schedule_work", "Create a scheduling assignment with user/project names or IDs.", scheduleWorkSchema()), s.ClockifyScheduleWork),
		workflowDescriptor(14, "webhooks", "webhook", []string{"Create a webhook from a simple name, URL, and event."}, []string{"clockify_webhooks_create"},
			toolRW("clockify_setup_webhook", "Create a webhook subscription for this workspace.", setupWebhookSchema()), s.ClockifySetupWebhook),
		workflowDescriptor(15, "demo", "demo", []string{"Create a deterministic full fixture for smoke testing."}, []string{"clockify_clients_create", "clockify_projects_create", "clockify_tasks_create", "clockify_tags_create", "clockify_entries_create"},
			toolRWIdem("clockify_demo_seed", "Create or reuse deterministic demo client/project/task/tag/time-entry objects.", demoSeedSchema()), s.ClockifyDemoSeed),
		workflowDescriptor(16, "demo", "demo", []string{"Clean deterministic demo objects repeatedly."}, []string{"clockify_clients_delete", "clockify_projects_delete", "clockify_tasks_delete", "clockify_tags_delete", "clockify_entries_delete"},
			toolRWIdem("clockify_demo_cleanup", "Delete deterministic demo objects by prefix, continuing through partial failures.", demoCleanupSchema()), s.ClockifyDemoCleanup),
	}
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
			"Use workflow tools first.",
			"Use IDs returned by previous calls when available.",
			"Use domain tools for precise CRUD and raw API tools only when no workflow or domain tool fits.",
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

func (s *Service) ClockifyCreateWorkPackage(ctx context.Context, args map[string]any) (any, error) {
	upsert := true
	if v, ok := args["upsert"].(bool); ok {
		upsert = v
	}
	changed := ChangeSet{}
	warnings := []Warning{}
	ids := map[string]string{"workspaceId": s.WorkspaceID}
	data := map[string]any{}

	clientID := strings.TrimSpace(stringArg(args, "client_id"))
	clientName := strings.TrimSpace(stringArg(args, "client"))
	if clientID != "" {
		if err := resolve.ValidateID(clientID, "client_id"); err != nil {
			return nil, err
		}
		ids["clientId"] = clientID
	} else if clientName != "" {
		client, reused, err := s.ensureClientNamed(ctx, clientName, upsert)
		if err != nil {
			return nil, err
		}
		clientID = client.ID
		ids["clientId"] = client.ID
		data["client"] = client
		addChanged(&changed, reused, clientRef(client))
	}

	projectID := strings.TrimSpace(stringArg(args, "project_id"))
	projectName := strings.TrimSpace(stringArg(args, "project"))
	if projectID != "" {
		if err := resolve.ValidateID(projectID, "project_id"); err != nil {
			return nil, err
		}
		ids["projectId"] = projectID
	} else {
		if projectName == "" {
			return nil, fmt.Errorf("project or project_id is required")
		}
		project, reused, err := s.ensureProjectNamed(ctx, projectName, clientID, upsert, args)
		if err != nil {
			return nil, err
		}
		projectID = project.ID
		ids["projectId"] = project.ID
		if project.ClientID != "" {
			ids["clientId"] = project.ClientID
		}
		data["project"] = project
		addChanged(&changed, reused, projectRef(project))
	}

	taskID := strings.TrimSpace(stringArg(args, "task_id"))
	taskName := strings.TrimSpace(stringArg(args, "task"))
	if taskID != "" {
		if err := resolve.ValidateID(taskID, "task_id"); err != nil {
			return nil, err
		}
		ids["taskId"] = taskID
	} else if taskName != "" {
		task, reused, err := s.ensureTaskNamed(ctx, projectID, taskName, upsert, args)
		if err != nil {
			return nil, err
		}
		ids["taskId"] = task.ID
		data["task"] = task
		addChanged(&changed, reused, taskRef(task))
	}

	tagIDs := append([]string(nil), stringSliceArg(args, "tag_ids")...)
	for _, id := range tagIDs {
		if err := resolve.ValidateID(id, "tag_id"); err != nil {
			return nil, err
		}
	}
	tagNames := workflowStringList(args, "tags")
	if tag := strings.TrimSpace(stringArg(args, "tag")); tag != "" {
		tagNames = append(tagNames, tag)
	}
	tags := make([]clockify.Tag, 0, len(tagNames))
	for _, name := range tagNames {
		tag, reused, err := s.ensureTagNamed(ctx, name, upsert)
		if err != nil {
			return nil, err
		}
		tagIDs = append(tagIDs, tag.ID)
		tags = append(tags, tag)
		addChanged(&changed, reused, tagRef(tag))
	}
	if len(tagIDs) == 1 {
		ids["tagId"] = tagIDs[0]
	}
	if len(tagIDs) > 0 {
		data["tagIds"] = tagIDs
	}
	if len(tags) > 0 {
		data["tags"] = tags
	}

	return result("clockify_create_work_package", "work_package", ids, data, changed, warnings, []NextAction{
		{Tool: "clockify_log_work", Args: map[string]any{"project_id": projectID, "task_id": ids["taskId"], "tag_ids": tagIDs}, Reason: "Log finished work against this package."},
		{Tool: "clockify_start_work", Args: map[string]any{"project_id": projectID, "task_id": ids["taskId"], "tag_ids": tagIDs}, Reason: "Start a timer against this package."},
	}), nil
}

func (s *Service) ClockifyLogWork(ctx context.Context, args map[string]any) (any, error) {
	if strings.TrimSpace(stringArg(args, "end")) == "" {
		return nil, fmt.Errorf("end is required for clockify_log_work; use clockify_start_work for a running timer")
	}
	if err := s.rejectLogWorkOverlap(ctx, args); err != nil {
		return nil, err
	}
	entry, ids, err := s.createEntry(ctx, args)
	if err != nil {
		return nil, err
	}
	return result("clockify_log_work", "entry", ids, entry, ChangeSet{Created: []EntityRef{entryRef(entry)}}, nil, []NextAction{
		{Tool: "clockify_review_day", Args: reviewDayArgsFromEntry(entry), Reason: "Review the day after logging work."},
		{Tool: "clockify_fix_entry", Args: map[string]any{"entry_id": entry.ID}, Reason: "Adjust this entry if any details are wrong."},
	}), nil
}

// rejectLogWorkOverlap fails closed when a clockify_log_work entry overlaps an
// existing entry, unless allow_overlap is set. logWorkSchema advertises
// allow_overlap; this is where the handler honors it.
func (s *Service) rejectLogWorkOverlap(ctx context.Context, args map[string]any) error {
	if boolArg(args, "allow_overlap") {
		return nil
	}
	loc := s.location()
	startRaw := strings.TrimSpace(stringArg(args, "start"))
	start, err := timeparse.ParseDatetime(startRaw, loc)
	if err != nil {
		return fmt.Errorf("could not parse date %q for start — use YYYY-MM-DD or RFC3339", startRaw)
	}
	endRaw := strings.TrimSpace(stringArg(args, "end"))
	end, err := timeparse.ParseDatetime(endRaw, loc)
	if err != nil {
		return fmt.Errorf("could not parse date %q for end — use YYYY-MM-DD or RFC3339", endRaw)
	}
	overlaps, _, err := s.findEntryOverlaps(ctx, start, end)
	if err != nil {
		return err
	}
	if len(overlaps) > 0 {
		return fmt.Errorf("requested entry overlaps %d existing entr%s; pass allow_overlap=true only after manual review", len(overlaps), pluralY(len(overlaps)))
	}
	return nil
}

func (s *Service) ClockifyStartWork(ctx context.Context, args map[string]any) (any, error) {
	startArgs, startDefaulted, err := s.prepareStartWorkArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	entry, ids, err := s.createEntry(ctx, startArgs)
	if err != nil {
		return nil, err
	}
	meta := map[string]any{}
	if startDefaulted {
		meta["startWasDefaulted"] = true
		meta["resolvedStart"] = startArgs["start"]
	}
	return result("clockify_start_work", "entry", ids, entry, ChangeSet{Created: []EntityRef{entryRef(entry)}}, nil, []NextAction{
		{Tool: "clockify_stop_work", Reason: "Stop this timer when the work session is finished."},
		{Tool: "clockify_switch_work", Reason: "Switch to another project/task without manually stopping first."},
	}, meta), nil
}

func (s *Service) ClockifyStopWork(ctx context.Context, args map[string]any) (any, error) {
	out, err := s.StopTimer(ctx, args)
	if err != nil {
		return nil, err
	}
	resultOut := standardizeDomainResult("clockify_stop_work", "entry", "updated", out, args)
	if env, ok := out.(ResultEnvelope); ok {
		if entry, ok := env.Data.(clockify.TimeEntry); ok {
			resultOut.IDs = cleanIDs(map[string]string{
				"workspaceId": stringFromAny(env.Meta["workspaceId"]),
				"userId":      stringFromAny(env.Meta["userId"]),
				"entryId":     entry.ID,
				"projectId":   entry.ProjectID,
				"taskId":      entry.TaskID,
			})
			resultOut.Changed = ChangeSet{Updated: []EntityRef{entryRef(entry)}}
		}
		if entry, ok := env.Data.(EntryView); ok {
			resultOut.IDs = cleanIDs(map[string]string{
				"workspaceId": firstNonEmptyString(entry.WorkspaceID, stringFromAny(env.Meta["workspaceId"])),
				"userId":      firstNonEmptyString(entry.UserID, stringFromAny(env.Meta["userId"])),
				"entryId":     entry.ID,
				"projectId":   entry.ProjectID,
				"taskId":      entry.TaskID,
			})
			resultOut.Changed = ChangeSet{Updated: []EntityRef{{Type: "entry", ID: entry.ID, Name: entry.Description}}}
		}
	}
	resultOut.Next = []NextAction{{Tool: "clockify_review_day", Reason: "Review the day after stopping work."}}
	return resultOut, nil
}

func (s *Service) ClockifySwitchWork(ctx context.Context, args map[string]any) (any, error) {
	warnings := []Warning{}
	startArgs, startDefaulted, err := s.prepareStartWorkArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	// meta echoes a defaulted start so the model never has to guess whether
	// the resolved start time came from the caller or from time.Now().
	meta := map[string]any{}
	if startDefaulted {
		meta["startWasDefaulted"] = true
		meta["resolvedStart"] = startArgs["start"]
	}
	stopped, stopErr := s.ClockifyStopWork(ctx, map[string]any{})
	if stopErr != nil {
		if !isNoRunningTimer(stopErr) {
			return nil, fmt.Errorf("stop current work: %w", stopErr)
		}
		warnings = append(warnings, Warning{Code: "no_running_timer", Message: "No running timer was found; started the new work item."})
	}
	started, err := s.ClockifyStartWork(ctx, startArgs)
	if err != nil {
		if stopped != nil {
			warnings = append(warnings, Warning{Code: "partial_failure", Message: "Stopped the previous timer but could not start the new one."})
		}
		return result("clockify_switch_work", "entry", map[string]string{"workspaceId": s.WorkspaceID}, map[string]any{
			"status":  "partial_failure",
			"stopped": stopped,
			"error":   err.Error(),
		}, ChangeSet{Updated: refsFromToolResult(stopped)}, warnings, []NextAction{
			{Tool: "clockify_start_work", Args: startArgs, Reason: "Retry starting the target timer after fixing the error."},
		}, meta), nil
	}
	startResult, _ := started.(ToolResult)
	ids := map[string]string{"workspaceId": s.WorkspaceID}
	for k, v := range startResult.IDs {
		ids[k] = v
	}
	return result("clockify_switch_work", "entry", ids, map[string]any{
		"status":  "ok",
		"stopped": stopped,
		"started": started,
	}, ChangeSet{Created: startResult.Changed.Created, Updated: refsFromToolResult(stopped)}, warnings, []NextAction{
		{Tool: "clockify_stop_work", Reason: "Stop the newly started timer when finished."},
	}, meta), nil
}

// prepareStartWorkArgs resolves caller args into the shape createEntry
// expects. The returned bool reports whether `start` was absent and
// defaulted to now, so the caller can echo the resolved value (Axiom 22).
func (s *Service) prepareStartWorkArgs(ctx context.Context, args map[string]any) (map[string]any, bool, error) {
	startArgs := copyArgs(args)
	startDefaulted := false
	if strings.TrimSpace(stringArg(startArgs, "start")) == "" {
		startArgs["start"] = time.Now().UTC().Format(time.RFC3339)
		startDefaulted = true
	} else if raw := stringArg(startArgs, "start"); raw != "" {
		if _, err := timeparse.ParseDatetime(raw, s.location()); err != nil {
			return nil, false, fmt.Errorf("could not parse date %q for start — use YYYY-MM-DD or RFC3339", raw)
		}
	}
	delete(startArgs, "end")

	projectID := strings.TrimSpace(stringArg(startArgs, "project_id"))
	if projectID == "" {
		if project := strings.TrimSpace(stringArg(startArgs, "project")); project != "" {
			resolved, err := s.resolveProjectID(ctx, s.WorkspaceID, project)
			if err != nil {
				return nil, false, err
			}
			projectID = resolved
			startArgs["project_id"] = resolved
			delete(startArgs, "project")
		}
	} else if err := resolve.ValidateID(projectID, "project_id"); err != nil {
		return nil, false, err
	}

	if taskID := strings.TrimSpace(stringArg(startArgs, "task_id")); taskID != "" {
		if err := resolve.ValidateID(taskID, "task_id"); err != nil {
			return nil, false, err
		}
	} else if task := strings.TrimSpace(stringArg(startArgs, "task")); task != "" {
		if projectID == "" {
			return nil, false, fmt.Errorf("project_id or project is required when resolving task by name")
		}
		resolved, err := s.resolveTaskID(ctx, s.WorkspaceID, projectID, task)
		if err != nil {
			return nil, false, err
		}
		startArgs["task_id"] = resolved
		delete(startArgs, "task")
	}

	tagIDs, err := s.tagIDsFromArgs(ctx, startArgs)
	if err != nil {
		return nil, false, err
	}
	if len(tagIDs) > 0 {
		startArgs["tag_ids"] = tagIDs
		delete(startArgs, "tag")
	}

	return startArgs, startDefaulted, nil
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
	standard.Next = nextFromReviewData(out.Data)
	if len(standard.Next) == 0 {
		standard.Next = []NextAction{{Tool: "clockify_log_work", Reason: "Log confirmed missing work if the review found gaps."}}
	}
	return standard, nil
}

func (s *Service) ClockifyFixEntry(ctx context.Context, args map[string]any) (any, error) {
	fixArgs := copyArgs(args)
	if v := strings.TrimSpace(stringArg(fixArgs, "description")); v != "" && strings.TrimSpace(stringArg(fixArgs, "new_description")) == "" {
		fixArgs["new_description"] = v
	}
	out, err := s.FindAndUpdateEntry(ctx, fixArgs)
	if err != nil {
		return nil, err
	}
	standard := standardizeDomainResult("clockify_fix_entry", "entry", "updated", out, fixArgs)
	if env, ok := out.(ResultEnvelope); ok {
		if data, ok := env.Data.(FindAndUpdateEntryData); ok {
			standard.IDs = cleanIDs(map[string]string{"workspaceId": stringFromAny(env.Meta["workspaceId"]), "entryId": data.Entry.ID, "projectId": data.Entry.ProjectID})
			ref := EntityRef{Type: "entry", ID: data.Entry.ID, Name: data.Entry.Description}
			if len(data.UpdatedFields) == 0 {
				standard.Changed = ChangeSet{Reused: []EntityRef{ref}}
			} else {
				standard.Changed = ChangeSet{Updated: []EntityRef{ref}}
			}
		}
	}
	standard.Next = []NextAction{{Tool: "clockify_review_day", Reason: "Review the affected day after fixing the entry."}}
	return standard, nil
}

func (s *Service) ClockifyInvoiceClientWork(ctx context.Context, args map[string]any) (any, error) {
	invoiceArgs := copyArgs(args)
	clientID := strings.TrimSpace(stringArg(invoiceArgs, "client_id"))
	if clientID == "" {
		clientRef := strings.TrimSpace(stringArg(invoiceArgs, "client"))
		if clientRef == "" {
			return nil, fmt.Errorf("client or client_id is required")
		}
		resolved, err := s.resolveClientID(ctx, s.WorkspaceID, clientRef)
		if err != nil {
			return nil, err
		}
		clientID = resolved
		invoiceArgs["client_id"] = clientID
	}
	if strings.TrimSpace(stringArg(invoiceArgs, "currency")) == "" {
		return nil, fmt.Errorf("currency is required — pass a currency code such as USD or EUR")
	}
	if strings.TrimSpace(stringArg(invoiceArgs, "number")) == "" {
		invoiceArgs["number"] = fmt.Sprintf("MCP-%s-%s", time.Now().UTC().Format("20060102"), shortID(clientID))
	}
	now := time.Now().UTC()
	loc := s.location()
	if raw := strings.TrimSpace(stringArg(invoiceArgs, "issued_date")); raw == "" {
		invoiceArgs["issued_date"] = now.Format(time.RFC3339)
	} else if t, err := timeparse.ParseDatetime(raw, loc); err == nil {
		invoiceArgs["issued_date"] = t.UTC().Format(time.RFC3339)
	} else {
		return nil, fmt.Errorf("could not parse issued_date %q — use YYYY-MM-DD or RFC3339", raw)
	}
	if raw := strings.TrimSpace(stringArg(invoiceArgs, "due_date")); raw == "" {
		invoiceArgs["due_date"] = now.AddDate(0, 0, 14).Format(time.RFC3339)
	} else if t, err := timeparse.ParseDatetime(raw, loc); err == nil {
		invoiceArgs["due_date"] = t.UTC().Format(time.RFC3339)
	} else {
		return nil, fmt.Errorf("could not parse due_date %q — use YYYY-MM-DD or RFC3339", raw)
	}
	out, err := s.createInvoice(ctx, invoiceArgs)
	if err != nil {
		return nil, err
	}
	standard := standardizeDomainResult("clockify_invoice_client_work", "invoice", "created", out, invoiceArgs)
	if standard.IDs == nil {
		standard.IDs = map[string]string{}
	}
	standard.IDs["clientId"] = clientID
	invoiceID := standard.IDs["invoiceId"]
	importFrom := strings.TrimSpace(stringArg(invoiceArgs, "from"))
	importTo := strings.TrimSpace(stringArg(invoiceArgs, "to"))
	switch {
	case invoiceID != "" && importFrom != "" && importTo != "":
		importArgs := map[string]any{
			"invoice_id":            invoiceID,
			"from":                  importFrom,
			"to":                    importTo,
			"time_entry_group_type": firstNonEmpty([]string{stringArg(invoiceArgs, "time_entry_group_type"), "DETAILED"}),
		}
		if projectIDs := stringSliceArg(invoiceArgs, "project_ids"); len(projectIDs) > 0 {
			importArgs["project_ids"] = projectIDs
		}
		imported, importErr := s.importInvoiceTimeOneUser(ctx, importArgs)
		if importErr != nil {
			return recoverable("clockify_invoice_client_work", importErr, RecoveryHint{
				Hint: fmt.Sprintf("Invoice %s was created, but importing billable time failed, so the invoice total is still 0. Retry with clockify_invoices_import_time, or add line items manually with clockify_invoices_items_add.", invoiceID),
				Tool: "clockify_invoices_import_time",
				Args: importArgs,
			}), nil
		}
		standard.Data = map[string]any{
			"invoice": standard.Data,
			"import":  standardizeDomainResult("clockify_invoices_import_time", "invoice_item", "updated", imported, importArgs),
		}
	case invoiceID != "":
		standard.Warnings = append(standard.Warnings, Warning{
			Code:    "invoice_has_no_items",
			Message: "Invoice created with no line items and a 0 total. Pass from and to (the billing period) to import billable time, or use clockify_invoices_import_time / clockify_invoices_items_add to add items.",
		})
	}
	standard.Next = []NextAction{
		{Tool: "clockify_invoices_items_add", Args: map[string]any{"invoice_id": standard.IDs["invoiceId"]}, Reason: "Add manual invoice items if needed."},
		{Tool: "clockify_invoices_send", Args: map[string]any{"invoice_id": standard.IDs["invoiceId"]}, Reason: "Send the invoice when it is ready."},
	}
	return standard, nil
}

func (s *Service) ClockifyRecordExpense(ctx context.Context, args map[string]any) (any, error) {
	expenseArgs := copyArgs(args)
	if strings.TrimSpace(stringArg(expenseArgs, "date")) == "" {
		expenseArgs["date"] = time.Now().UTC().Format(time.RFC3339)
	}
	if strings.TrimSpace(stringArg(expenseArgs, "category_id")) == "" {
		category := strings.TrimSpace(stringArg(expenseArgs, "category"))
		if category == "" {
			return nil, fmt.Errorf("category or category_id is required")
		}
		categoryID, err := s.resolveExpenseCategoryID(ctx, category)
		if err != nil {
			return nil, err
		}
		expenseArgs["category_id"] = categoryID
	}
	if strings.TrimSpace(stringArg(expenseArgs, "project_id")) == "" {
		if project := strings.TrimSpace(stringArg(expenseArgs, "project")); project != "" {
			projectID, err := s.resolveProjectID(ctx, s.WorkspaceID, project)
			if err != nil {
				return nil, err
			}
			expenseArgs["project_id"] = projectID
		}
	}
	out, err := s.createExpense(ctx, expenseArgs)
	if err != nil {
		return nil, err
	}
	standard := standardizeDomainResult("clockify_record_expense", "expense", "created", out, expenseArgs)
	standard.Next = []NextAction{{Tool: "clockify_expenses_list", Reason: "Verify the recorded expense in the expense list."}}
	return standard, nil
}

func (s *Service) ClockifyRequestTimeOff(ctx context.Context, args map[string]any) (any, error) {
	reqArgs := copyArgs(args)
	if strings.TrimSpace(stringArg(reqArgs, "policy_id")) == "" {
		policy := strings.TrimSpace(stringArg(reqArgs, "policy"))
		if policy == "" {
			return nil, fmt.Errorf("policy or policy_id is required")
		}
		policyID, err := s.resolveTimeOffPolicyID(ctx, policy)
		if err != nil {
			return nil, err
		}
		reqArgs["policy_id"] = policyID
	}
	out, err := s.createTimeOffRequest(ctx, reqArgs)
	if err != nil {
		return nil, err
	}
	standard := standardizeDomainResult("clockify_request_time_off", "time_off_request", "created", out, reqArgs)
	standard.Next = []NextAction{{Tool: "clockify_time_off_requests_list", Reason: "Check the request status after submitting."}}
	return standard, nil
}

func (s *Service) ClockifyScheduleWork(ctx context.Context, args map[string]any) (any, error) {
	scheduleArgs := copyArgs(args)
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	if userID := strings.TrimSpace(stringArg(scheduleArgs, "user_id")); userID != "" {
		resolved, err := s.resolveUserID(ctx, wsID, userID)
		if err != nil {
			return nil, err
		}
		scheduleArgs["user_id"] = resolved
	} else if user := strings.TrimSpace(stringArg(scheduleArgs, "user")); user != "" {
		resolved, err := s.resolveUserID(ctx, wsID, user)
		if err != nil {
			return nil, err
		}
		scheduleArgs["user_id"] = resolved
		delete(scheduleArgs, "user")
	}

	projectID := strings.TrimSpace(stringArg(scheduleArgs, "project_id"))
	if projectID != "" {
		resolved, err := s.resolveProjectID(ctx, wsID, projectID)
		if err != nil {
			return nil, err
		}
		projectID = resolved
		scheduleArgs["project_id"] = resolved
	} else if project := strings.TrimSpace(stringArg(scheduleArgs, "project")); project != "" {
		resolved, err := s.resolveProjectID(ctx, wsID, project)
		if err != nil {
			return nil, err
		}
		projectID = resolved
		scheduleArgs["project_id"] = resolved
		delete(scheduleArgs, "project")
	}
	if task := strings.TrimSpace(stringArg(scheduleArgs, "task")); task != "" && strings.TrimSpace(stringArg(scheduleArgs, "task_id")) == "" {
		if projectID == "" {
			return nil, fmt.Errorf("project_id or project is required when resolving task by name")
		}
		resolved, err := s.resolveTaskID(ctx, wsID, projectID, task)
		if err != nil {
			return nil, err
		}
		scheduleArgs["task_id"] = resolved
		delete(scheduleArgs, "task")
	}
	out, err := s.createAssignment(ctx, scheduleArgs)
	if err != nil {
		return nil, err
	}
	standard := standardizeDomainResult("clockify_schedule_work", "assignment", "created", out, scheduleArgs)
	standard.Data = map[string]any{"assignment": singleAssignmentData(standard.Data)}
	standard.Next = []NextAction{{Tool: "clockify_scheduling_assignments_list", Args: map[string]any{"start": stringArg(scheduleArgs, "start"), "end": stringArg(scheduleArgs, "end")}, Reason: "Verify the scheduled assignment."}}
	return standard, nil
}

func singleAssignmentData(data any) any {
	switch items := data.(type) {
	case []map[string]any:
		if len(items) == 1 {
			return items[0]
		}
	case []any:
		if len(items) == 1 {
			return items[0]
		}
	}
	return data
}

func (s *Service) ClockifySetupWebhook(ctx context.Context, args map[string]any) (any, error) {
	webhookArgs := copyArgs(args)
	if strings.TrimSpace(stringArg(webhookArgs, "webhook_event")) == "" {
		if event := strings.TrimSpace(stringArg(webhookArgs, "event")); event != "" {
			webhookArgs["webhook_event"] = event
		}
	}
	out, err := s.CreateWebhook(ctx, webhookArgs)
	if err != nil {
		return nil, err
	}
	standard := standardizeDomainResult("clockify_setup_webhook", "webhook", "created", out, webhookArgs)
	standard.Next = []NextAction{{Tool: "clockify_webhooks_test", Args: map[string]any{"webhook_id": standard.IDs["webhookId"]}, Reason: "Send a test event after setup if desired."}}
	return standard, nil
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
		"required": []string{"currency"},
		"properties": map[string]any{
			"client":                map[string]any{"type": "string", "description": "Client name or ID (this or client_id is required)."},
			"client_id":             map[string]any{"type": "string", "description": "Client ID (this or client is required)."},
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
		"category":    map[string]any{"type": "string"},
		"category_id": map[string]any{"type": "string"},
		"project":     map[string]any{"type": "string"},
		"project_id":  map[string]any{"type": "string"},
		"task_id":     map[string]any{"type": "string"},
		"user_id":     map[string]any{"type": "string"},
		"notes":       map[string]any{"type": "string"},
		"billable":    map[string]any{"type": "boolean"},
	}})
}

func requestTimeOffSchema() map[string]any {
	return objectSchema(map[string]any{"required": []string{"start", "end", "note"}, "properties": map[string]any{
		"policy":    map[string]any{"type": "string"},
		"policy_id": map[string]any{"type": "string"},
		"start":     map[string]any{"type": "string"},
		"end":       map[string]any{"type": "string"},
		"note":      map[string]any{"type": "string"},
		"half_day":  map[string]any{"type": "boolean"},
	}})
}

func scheduleWorkSchema() map[string]any {
	return objectSchema(map[string]any{"required": []string{"start", "end", "hours_per_day"}, "properties": map[string]any{
		"user":                     map[string]any{"type": "string", "description": "User name, email, or ID."},
		"user_id":                  map[string]any{"type": "string"},
		"project":                  map[string]any{"type": "string", "description": "Project name or ID."},
		"project_id":               map[string]any{"type": "string"},
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
	}})
}

func setupWebhookSchema() map[string]any {
	return objectSchema(map[string]any{"required": []string{"name", "url"}, "properties": map[string]any{
		"name":                map[string]any{"type": "string"},
		"url":                 map[string]any{"type": "string"},
		"event":               map[string]any{"type": "string", "description": "Alias for webhook_event."},
		"webhook_event":       map[string]any{"type": "string"},
		"trigger_source_type": map[string]any{"type": "string"},
		"trigger_source":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"dry_run":             map[string]any{"type": "boolean"},
	}})
}

func demoSeedSchema() map[string]any {
	return objectSchema(map[string]any{"properties": map[string]any{
		"run_id": map[string]any{"type": "string", "description": "Stable run id. Default: phase1."},
		"prefix": map[string]any{"type": "string", "description": "Explicit object-name prefix. Default: DEMO-<run_id>."},
		"date":   map[string]any{"type": "string", "description": "YYYY-MM-DD date for the demo time entry. Default: 2026-01-02."},
		"upsert": map[string]any{"type": "boolean", "description": "Reuse existing prefixed objects. Default: true."},
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

func (s *Service) ensureClientNamed(ctx context.Context, name string, upsert bool) (clockify.ClientEntity, bool, error) {
	if strings.TrimSpace(name) == "" {
		return clockify.ClientEntity{}, false, fmt.Errorf("client name is required")
	}
	if upsert {
		clients, err := s.listAllClients(ctx, nil)
		if err != nil {
			return clockify.ClientEntity{}, false, err
		}
		matches := make([]clockify.ClientEntity, 0, 1)
		for _, client := range clients {
			if strings.EqualFold(client.Name, name) {
				matches = append(matches, client)
			}
		}
		if len(matches) > 1 {
			return clockify.ClientEntity{}, false, fmt.Errorf("multiple clients match %q; use client_id", name)
		}
		if len(matches) == 1 {
			return matches[0], true, nil
		}
	}
	client, err := s.createClient(ctx, name)
	return client, false, err
}

func (s *Service) ensureProjectNamed(ctx context.Context, name, clientID string, upsert bool, args map[string]any) (clockify.Project, bool, error) {
	if strings.TrimSpace(name) == "" {
		return clockify.Project{}, false, fmt.Errorf("project name is required")
	}
	if upsert {
		projects, err := s.listAllProjects(ctx, map[string]any{"hydrated": false})
		if err != nil {
			return clockify.Project{}, false, err
		}
		matches := make([]clockify.Project, 0, 1)
		for _, project := range projects {
			if strings.EqualFold(project.Name, name) {
				if clientID == "" || project.ClientID == "" || project.ClientID == clientID {
					matches = append(matches, project)
				}
			}
		}
		if len(matches) > 1 {
			return clockify.Project{}, false, fmt.Errorf("multiple projects match %q; use project_id", name)
		}
		if len(matches) == 1 {
			return matches[0], true, nil
		}
	}
	createArgs := copyArgs(args)
	if clientID != "" {
		createArgs["client_id"] = clientID
	}
	project, _, err := s.createProject(ctx, createArgs, name)
	return project, false, err
}

func (s *Service) ensureTaskNamed(ctx context.Context, projectID, name string, upsert bool, args map[string]any) (clockify.Task, bool, error) {
	if strings.TrimSpace(name) == "" {
		return clockify.Task{}, false, fmt.Errorf("task name is required")
	}
	if upsert {
		tasks, err := s.listAllTasks(ctx, projectID, nil)
		if err != nil {
			return clockify.Task{}, false, err
		}
		matches := make([]clockify.Task, 0, 1)
		for _, task := range tasks {
			if strings.EqualFold(task.Name, name) {
				matches = append(matches, task)
			}
		}
		if len(matches) > 1 {
			return clockify.Task{}, false, fmt.Errorf("multiple tasks match %q in project %s; use task_id", name, projectID)
		}
		if len(matches) == 1 {
			return matches[0], true, nil
		}
	}
	task, err := s.createTask(ctx, projectID, name, args)
	return task, false, err
}

func (s *Service) ensureTagNamed(ctx context.Context, name string, upsert bool) (clockify.Tag, bool, error) {
	if strings.TrimSpace(name) == "" {
		return clockify.Tag{}, false, fmt.Errorf("tag name is required")
	}
	if upsert {
		tags, err := s.listAllTags(ctx, nil)
		if err != nil {
			return clockify.Tag{}, false, err
		}
		matches := make([]clockify.Tag, 0, 1)
		for _, tag := range tags {
			if strings.EqualFold(tag.Name, name) {
				matches = append(matches, tag)
			}
		}
		if len(matches) > 1 {
			return clockify.Tag{}, false, fmt.Errorf("multiple tags match %q; use tag_ids", name)
		}
		if len(matches) == 1 {
			return matches[0], true, nil
		}
	}
	tag, err := s.createTag(ctx, name)
	return tag, false, err
}

func (s *Service) resolveExpenseCategoryID(ctx context.Context, category string) (string, error) {
	if err := resolve.ValidateID(category, "category_id"); err == nil && len(category) == 24 {
		return category, nil
	}
	out, err := s.listExpenseCategories(ctx, nil)
	if err != nil {
		return "", err
	}
	env := out
	categories, _ := env.Data.([]map[string]any)
	return uniqueIDByName(categories, category, "expense category", "category_id")
}

func (s *Service) resolveTimeOffPolicyID(ctx context.Context, policy string) (string, error) {
	if err := resolve.ValidateID(policy, "policy_id"); err == nil && len(policy) == 24 {
		return policy, nil
	}
	out, err := s.listTimeOffPolicies(ctx, map[string]any{"page_size": float64(200)})
	if err != nil {
		return "", err
	}
	policies, _ := out.Data.([]map[string]any)
	return uniqueIDByName(policies, policy, "time off policy", "policy_id")
}

func uniqueIDByName(items []map[string]any, name, label, idField string) (string, error) {
	matches := make([]string, 0, 1)
	for _, item := range items {
		if strings.EqualFold(stringFromMap(item, "name"), name) {
			if id := oneUserStringFromMap(item, "id", "_id"); id != "" {
				matches = append(matches, id)
			}
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("%s %q not found; use %s", label, name, idField)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple %ss match %q; use %s", label, name, idField)
	}
	return matches[0], nil
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

func reviewDayArgsFromEntry(entry clockify.TimeEntry) map[string]any {
	start, err := entry.StartTime()
	if err != nil {
		return nil
	}
	return map[string]any{"date": start.Format("2006-01-02")}
}

func copyArgs(args map[string]any) map[string]any {
	out := make(map[string]any, len(args))
	for key, value := range args {
		out[key] = value
	}
	return out
}

func workflowStringList(args map[string]any, key string) []string {
	values := stringSliceArg(args, key)
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func refsFromToolResult(value any) []EntityRef {
	if result, ok := value.(ToolResult); ok {
		return append([]EntityRef(nil), result.Changed.Updated...)
	}
	return nil
}

func isNoRunningTimer(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *clockify.APIError
	if strings.Contains(strings.ToLower(err.Error()), "no running") {
		return true
	}
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 400 || apiErr.StatusCode == 404
	}
	return false
}

func shortID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 8 {
		return id
	}
	return id[len(id)-8:]
}
