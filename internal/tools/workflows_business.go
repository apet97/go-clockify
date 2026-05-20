package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/resolve"
	"github.com/apet97/go-clockify/internal/timeparse"
)

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

	rawTagIDs, _, err := strictStringSliceArg(args, "tag_ids")
	if err != nil {
		return nil, err
	}
	tagIDs := append([]string(nil), rawTagIDs...)
	for _, id := range tagIDs {
		if err := resolve.ValidateID(id, "tag_id"); err != nil {
			return nil, err
		}
	}
	tagNames, err := workflowStringList(args, "tags")
	if err != nil {
		return nil, err
	}
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
		projectIDs, _, err := strictStringSliceArg(invoiceArgs, "project_ids")
		if err != nil {
			return nil, err
		}
		if len(projectIDs) > 0 {
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
	reqArgs["__allow_empty_note"] = true
	out, err := s.createTimeOffRequest(ctx, reqArgs)
	if err != nil {
		return nil, err
	}
	standard := standardizeDomainResult("clockify_request_time_off", "time_off_request", "created", out, reqArgs)
	if standard.Meta == nil {
		standard.Meta = map[string]any{}
	}
	standard.Meta["timezone_note"] = "Clockify stores time-off boundaries in UTC; the displayed start/end times may differ from the input dates when the workspace timezone is not UTC."
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
	if strings.TrimSpace(stringArg(scheduleArgs, "user_id")) == "" {
		return nil, fmt.Errorf("schedule_work needs a user: pass user_id (an ID) or user (a name, email, or ID)")
	}
	if projectID == "" {
		return nil, fmt.Errorf("schedule_work needs a project: pass project_id (an ID) or project (a name or ID)")
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
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return "", err
	}
	categories, err := s.fetchAllExpenseCategories(ctx, wsID)
	if err != nil {
		return "", err
	}
	return uniqueIDByName(categories, category, "expense category", "category_id")
}

func (s *Service) resolveTimeOffPolicyID(ctx context.Context, policy string) (string, error) {
	if err := resolve.ValidateID(policy, "policy_id"); err == nil && len(policy) == 24 {
		return policy, nil
	}
	page := 1
	var allPolicies []map[string]any
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return "", err
	}
	for {
		args := map[string]any{"page_size": float64(200), "page": float64(page)}
		policies, _, _, err := s.fetchTimeOffPolicies(ctx, wsID, args)
		if err != nil {
			return "", err
		}
		allPolicies = append(allPolicies, policies...)
		if !boolFromAny(addPaginationMeta(map[string]any{"count": len(policies)}, args, page, 200)["has_more"]) {
			break
		}
		page++
	}
	return uniqueIDByName(allPolicies, policy, "time off policy", "policy_id")
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

func workflowStringList(args map[string]any, key string) ([]string, error) {
	values, _, err := strictStringSliceArg(args, key)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out, nil
}

func shortID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 8 {
		return id
	}
	return id[len(id)-8:]
}
