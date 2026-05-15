//go:build livee2e

package e2e_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveOneUserPaidFeatureHappyPaths(t *testing.T) {
	h := setupLiveMCPHarness(t, liveMCPOptions{})
	c := setupLiveHappyPathCampaign(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	work := createPaidFeatureWorkPackage(t, ctx, c)
	expense := createPaidFeatureExpense(t, ctx, c, work.ProjectID)

	t.Run("workflows", func(t *testing.T) {
		exerciseWorkflowHappyPaths(t, ctx, c, work, expense.CategoryID)
	})
	t.Run("core_domains_and_reports", func(t *testing.T) {
		exerciseCoreDomainAndReportHappyPaths(t, ctx, c, work)
	})
	t.Run("invoices", func(t *testing.T) {
		exerciseInvoiceHappyPath(t, ctx, c, work.ClientID, work.ProjectID, work.EntryID, expense.ID)
	})
	t.Run("expenses", func(t *testing.T) {
		exerciseExpenseHappyPath(t, ctx, c, expense)
	})
	t.Run("time_off", func(t *testing.T) {
		exerciseTimeOffHappyPath(t, ctx, c)
	})
	t.Run("scheduling", func(t *testing.T) {
		exerciseSchedulingHappyPath(t, ctx, c, work.ProjectID)
	})
	t.Run("webhooks", func(t *testing.T) {
		exerciseWebhookHappyPath(t, ctx, c)
	})
	t.Run("groups_holidays_and_governance", func(t *testing.T) {
		exerciseGroupsHolidaysAndGovernanceHappyPaths(t, ctx, c, work.ProjectID)
	})
}

func setupLiveHappyPathCampaign(t *testing.T, h *liveMCPHarness) *liveCampaignContext {
	t.Helper()
	if os.Getenv("CLOCKIFY_LIVE_HAPPY_PATH_CAMPAIGNS") != "1" {
		t.Skip("set CLOCKIFY_LIVE_HAPPY_PATH_CAMPAIGNS=1 to run paid-feature happy-path live campaigns")
	}
	for _, gate := range []string{
		"CLOCKIFY_LIVE_ADMIN_ENABLED",
		"CLOCKIFY_LIVE_BILLING_ENABLED",
		"CLOCKIFY_LIVE_SETTINGS_ENABLED",
	} {
		requireCategory(t, gate)
	}
	return setupLiveCampaign(t, h)
}

type paidFeatureWorkPackage struct {
	ClientID   string
	ProjectID  string
	TaskID     string
	EntryID    string
	EntryStart time.Time
	EntryEnd   time.Time
}

type paidFeatureExpense struct {
	ID         string
	CategoryID string
	Deleted    *bool
}

func liveFutureWeekdayDate(monthsAhead int) string {
	day := time.Now().UTC().AddDate(0, monthsAhead, 0)
	for day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		day = day.AddDate(0, 0, 1)
	}
	return day.Format("2006-01-02")
}

func liveShortName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().Unix()%1_000_000)
}

func createPaidFeatureWorkPackage(t *testing.T, ctx context.Context, c *liveCampaignContext) paidFeatureWorkPackage {
	t.Helper()

	client := c.h.callOK(ctx, "clockify_clients_create", map[string]any{
		"name": c.LivePrefix("happy-client", 0),
	})
	clientID := requireToolEntityID(t, client, "clientId", "id")
	c.RegisterCleanup("client", clientID, func(ctx context.Context) error {
		return c.rawArchiveAndDeleteClient(ctx, clientID)
	})

	project := c.h.callOK(ctx, "clockify_projects_create", map[string]any{
		"name":      c.LivePrefix("happy-project", 0),
		"client_id": clientID,
		"billable":  true,
	})
	projectID := requireToolEntityID(t, project, "projectId", "id")
	c.RegisterCleanup("project", projectID, func(ctx context.Context) error {
		return c.rawArchiveAndDeleteProject(ctx, projectID)
	})

	task := c.h.callOK(ctx, "clockify_tasks_create", map[string]any{
		"project_id": projectID,
		"name":       c.LivePrefix("happy-task", 0),
		"billable":   true,
	})
	taskID := requireToolEntityID(t, task, "taskId", "id")
	c.RegisterCleanup("task", taskID, func(ctx context.Context) error {
		_, err := c.h.Service.DeleteTask(ctx, map[string]any{
			"project": projectID,
			"task":    taskID,
		})
		return err
	})

	start := time.Now().UTC().Add(-72 * time.Hour).Truncate(time.Second)
	end := start.Add(30 * time.Minute)
	entry := c.h.callOK(ctx, "clockify_entries_create", map[string]any{
		"start":         start.Format(time.RFC3339),
		"end":           end.Format(time.RFC3339),
		"description":   c.LivePrefix("happy-entry", 0),
		"project_id":    projectID,
		"task_id":       taskID,
		"billable":      true,
		"allow_overlap": true,
	})
	entryID := requireToolEntityID(t, entry, "entryId", "id")
	c.RegisterCleanup("entry", entryID, func(ctx context.Context) error {
		return c.rawDeletePath(ctx, "/time-entries/"+entryID)
	})

	return paidFeatureWorkPackage{
		ClientID:   clientID,
		ProjectID:  projectID,
		TaskID:     taskID,
		EntryID:    entryID,
		EntryStart: start,
		EntryEnd:   end,
	}
}

func createPaidFeatureExpense(t *testing.T, ctx context.Context, c *liveCampaignContext, projectID string) paidFeatureExpense {
	t.Helper()

	categoryID := firstListEntityID(t, c.h.callOK(ctx, "clockify_expenses_categories_list", nil))
	if categoryID == "" {
		category := c.h.callOK(ctx, "clockify_expenses_categories_create", map[string]any{
			"name": c.LivePrefix("happy-exp-cat", 0),
		})
		categoryID = requireToolEntityID(t, category, "categoryId", "id")
		c.RegisterCleanup("expense-category", categoryID, func(ctx context.Context) error {
			err := c.rawDeletePath(ctx, "/expenses/categories/"+categoryID)
			if err != nil && strings.Contains(err.Error(), "archived") {
				c.t.Logf("expense-category %s left orphaned: upstream requires UI archive before delete", categoryID)
				return nil
			}
			return err
		})
	}

	expense := c.h.callOK(ctx, "clockify_expenses_create", map[string]any{
		"amount":      1.25,
		"date":        time.Now().UTC().Truncate(time.Second).Format("2006-01-02T15:04:05Z"),
		"category_id": categoryID,
		"project_id":  projectID,
		"notes":       c.LivePrefix("happy-expense", 0),
	})
	expenseID := requireToolEntityID(t, expense, "expenseId", "id")
	expenseDeleted := false
	c.RegisterCleanup("expense", expenseID, func(ctx context.Context) error {
		if expenseDeleted {
			return nil
		}
		return c.rawDeletePath(ctx, "/expenses/"+expenseID)
	})
	return paidFeatureExpense{ID: expenseID, CategoryID: categoryID, Deleted: &expenseDeleted}
}

func exerciseWorkflowHappyPaths(t *testing.T, ctx context.Context, c *liveCampaignContext, work paidFeatureWorkPackage, categoryID string) {
	t.Helper()

	invoiceDeleted := false
	invoice := c.h.callOK(ctx, "clockify_invoice_client_work", map[string]any{
		"client_id":   work.ClientID,
		"number":      strings.ToUpper(c.LivePrefix("happy-workflow-invoice", 0)),
		"issued_date": time.Now().UTC().Format(time.RFC3339),
		"due_date":    time.Now().UTC().AddDate(0, 0, 14).Format(time.RFC3339),
		"currency":    "USD",
	})
	invoiceID := requireToolEntityID(t, invoice, "invoiceId", "id")
	c.RegisterCleanup("workflow-invoice", invoiceID, func(ctx context.Context) error {
		if invoiceDeleted {
			return nil
		}
		return c.rawDeletePath(ctx, "/invoices/"+invoiceID)
	})

	expenseDeleted := false
	expense := c.h.callOK(ctx, "clockify_record_expense", map[string]any{
		"amount":      1.75,
		"date":        time.Now().UTC().Truncate(time.Second).Format("2006-01-02T15:04:05Z"),
		"category_id": categoryID,
		"project_id":  work.ProjectID,
		"notes":       c.LivePrefix("happy-workflow-expense", 0),
	})
	expenseID := requireToolEntityID(t, expense, "expenseId", "id")
	c.RegisterCleanup("workflow-expense", expenseID, func(ctx context.Context) error {
		if expenseDeleted {
			return nil
		}
		return c.rawDeletePath(ctx, "/expenses/"+expenseID)
	})

	policyArchived := false
	policy := c.h.callOK(ctx, "clockify_time_off_policies_create", map[string]any{
		"name":              c.LivePrefix("happy-workflow-policy", 0),
		"time_unit":         "DAYS",
		"days_per_year":     3,
		"negative_balance":  true,
		"requires_approval": true,
	})
	policyID := requireToolEntityID(t, policy, "policyId", "id")
	c.RegisterCleanup("workflow-time-off-policy", policyID, func(ctx context.Context) error {
		if policyArchived {
			return nil
		}
		var ignored map[string]any
		return c.h.Service.Client.Patch(ctx, "/workspaces/"+c.WorkspaceID+"/time-off/policies/"+policyID, map[string]any{"status": "ARCHIVED"}, &ignored)
	})

	requestDeleted := false
	requestDate := liveFutureWeekdayDate(8)
	request := c.h.callOK(ctx, "clockify_request_time_off", map[string]any{
		"policy_id": policyID,
		"start":     requestDate,
		"end":       requestDate,
		"note":      c.LivePrefix("happy-workflow-time-off", 0),
	})
	requestID := requireToolEntityID(t, request, "timeOffRequestId", "requestId", "id")
	c.RegisterCleanup("workflow-time-off-request", requestID, func(ctx context.Context) error {
		if requestDeleted {
			return nil
		}
		return c.rawDeletePath(ctx, "/time-off/policies/"+policyID+"/requests/"+requestID)
	})

	assignmentDeleted := false
	scheduleStart := time.Now().UTC().AddDate(0, 2, 0).Truncate(time.Second)
	scheduleEnd := scheduleStart.AddDate(0, 0, 4)
	assignment := c.h.callOK(ctx, "clockify_schedule_work", map[string]any{
		"user_id":       c.OwnerUserID,
		"project_id":    work.ProjectID,
		"start":         scheduleStart.Format("2006-01-02T15:04:05Z"),
		"end":           scheduleEnd.Format("2006-01-02T15:04:05Z"),
		"hours_per_day": 1,
	})
	assignmentID := requireToolEntityID(t, assignment, "assignmentId", "id")
	c.RegisterCleanup("workflow-assignment", assignmentID, func(ctx context.Context) error {
		if assignmentDeleted {
			return nil
		}
		return c.rawDeletePath(ctx, "/scheduling/assignments/recurring/"+assignmentID)
	})

	webhookDeleted := false
	webhook := c.h.callOK(ctx, "clockify_setup_webhook", map[string]any{
		"name":          liveShortName("mcp-flow-hook"),
		"url":           "https://example.com/clockify",
		"webhook_event": "NEW_TIME_ENTRY",
	})
	webhookID := requireToolEntityID(t, webhook, "webhookId", "id")
	c.RegisterCleanup("workflow-webhook", webhookID, func(ctx context.Context) error {
		if webhookDeleted {
			return nil
		}
		return c.rawDeletePath(ctx, "/webhooks/"+webhookID)
	})

	c.h.callOK(ctx, "clockify_invoices_delete", map[string]any{"invoice_id": invoiceID})
	invoiceDeleted = true
	c.h.callOK(ctx, "clockify_expenses_delete", map[string]any{"expense_id": expenseID})
	expenseDeleted = true
	c.h.callOK(ctx, "clockify_time_off_requests_delete", map[string]any{"policy_id": policyID, "request_id": requestID})
	requestDeleted = true
	c.h.callOK(ctx, "clockify_scheduling_assignments_delete", map[string]any{"assignment_id": assignmentID})
	assignmentDeleted = true
	c.h.callOK(ctx, "clockify_webhooks_delete", map[string]any{"webhook_id": webhookID})
	webhookDeleted = true
	c.h.callOK(ctx, "clockify_time_off_archive", map[string]any{"policy_id": policyID, "archived": true})
	policyArchived = true
}

func exerciseCoreDomainAndReportHappyPaths(t *testing.T, ctx context.Context, c *liveCampaignContext, work paidFeatureWorkPackage) {
	t.Helper()

	extractList(t, c.h.callOK(ctx, "clockify_clients_list", map[string]any{"page_size": 200}))
	extractList(t, c.h.callOK(ctx, "clockify_projects_list", map[string]any{"page_size": 200}))
	requireListContainsID(t, c.h.callOK(ctx, "clockify_tasks_list", map[string]any{
		"project_id": work.ProjectID,
	}), work.TaskID)

	c.h.callOK(ctx, "clockify_projects_rates_update", map[string]any{
		"project_id": work.ProjectID,
		"user_id":    c.OwnerUserID,
		"rate_kind":  "hourly",
		"amount":     123,
	})
	c.h.callOK(ctx, "clockify_tasks_rates_update", map[string]any{
		"project_id": work.ProjectID,
		"task_id":    work.TaskID,
		"rate_kind":  "hourly",
		"amount":     123,
	})

	requireToolEntityID(t, c.h.callOK(ctx, "clockify_workspace_settings", nil), "workspaceId", "id")
	c.h.callOK(ctx, "clockify_entries_mark_invoiced", map[string]any{
		"time_entry_ids": []any{work.EntryID},
		"invoiced":       true,
	})
	c.h.callOK(ctx, "clockify_entries_mark_invoiced", map[string]any{
		"time_entry_ids": []any{work.EntryID},
		"invoiced":       false,
	})

	c.h.callOK(ctx, "clockify_reports_attendance", paidFeatureReportArgs(work.EntryStart.Add(-time.Hour), work.EntryEnd.Add(time.Hour), map[string]any{
		"attendanceFilter": map[string]any{},
	}))
	c.h.callOK(ctx, "clockify_reports_money", paidFeatureReportArgs(work.EntryStart.Add(-time.Hour), work.EntryEnd.Add(time.Hour), map[string]any{
		"summaryFilter": map[string]any{"groups": []any{"PROJECT"}},
	}))
	exported := extractDataMap(t, c.h.callOK(ctx, "clockify_reports_export", paidFeatureReportArgs(work.EntryStart.Add(-time.Hour), work.EntryEnd.Add(time.Hour), map[string]any{
		"detailedFilter": map[string]any{},
	})))
	if len(exported) == 0 {
		t.Fatalf("clockify_reports_export returned empty data")
	}
}

func paidFeatureReportArgs(start, end time.Time, body map[string]any) map[string]any {
	if body == nil {
		body = map[string]any{}
	}
	body["dateRangeType"] = "ABSOLUTE"
	return map[string]any{
		"start":       start.Format("2006-01-02T15:04:05.000"),
		"end":         end.Format("2006-01-02T15:04:05.000"),
		"export_type": "JSON",
		"body":        body,
	}
}

func exerciseInvoiceHappyPath(t *testing.T, ctx context.Context, c *liveCampaignContext, clientID, projectID, entryID, expenseID string) {
	t.Helper()

	invoiceDeleted := false
	invoice := c.h.callOK(ctx, "clockify_invoices_create", map[string]any{
		"client_id":   clientID,
		"number":      strings.ToUpper(c.LivePrefix("happy-invoice", 0)),
		"issued_date": time.Now().UTC().Format(time.RFC3339),
		"due_date":    time.Now().UTC().AddDate(0, 0, 14).Format(time.RFC3339),
		"currency":    "USD",
		"note":        c.LivePrefix("happy-invoice-note", 0),
	})
	invoiceID := requireToolEntityID(t, invoice, "invoiceId", "id")
	c.RegisterCleanup("invoice", invoiceID, func(ctx context.Context) error {
		if invoiceDeleted {
			return nil
		}
		return c.rawDeletePath(ctx, "/invoices/"+invoiceID)
	})

	requireToolEntityID(t, c.h.callOK(ctx, "clockify_invoices_get", map[string]any{
		"invoice_id": invoiceID,
	}), "invoiceId", "id")

	c.h.callOK(ctx, "clockify_invoices_update", map[string]any{
		"invoice_id": invoiceID,
		"note":       c.LivePrefix("happy-invoice-updated", 0),
	})

	c.h.callOK(ctx, "clockify_invoices_items_add", map[string]any{
		"invoice_id":  invoiceID,
		"item_type":   "NEW DEFAULT",
		"description": c.LivePrefix("happy-item", 0),
		"quantity":    1,
		"unit_price":  1,
	})
	if items := extractList(t, c.h.callOK(ctx, "clockify_invoices_items_list", map[string]any{
		"invoice_id": invoiceID,
	}), "items"); len(items) == 0 {
		t.Fatalf("clockify_invoices_items_list returned no items after add")
	}
	c.h.callOK(ctx, "clockify_invoices_items_delete", map[string]any{
		"invoice_id": invoiceID,
		"item_index": "1",
	})

	importBody := map[string]any{
		"from":          time.Now().UTC().Add(-96 * time.Hour).Format(time.RFC3339),
		"to":            time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
		"projectFilter": map[string]any{"ids": []any{projectID}, "contains": "CONTAINS", "status": "ACTIVE"},
	}
	c.h.callOK(ctx, "clockify_invoices_import_time", map[string]any{
		"invoice_id":            invoiceID,
		"time_entry_ids":        []any{entryID},
		"time_entry_group_type": "DETAILED",
		"body":                  importBody,
	})
	c.h.callOK(ctx, "clockify_invoices_import_expenses", map[string]any{
		"invoice_id":       invoiceID,
		"expense_ids":      []any{expenseID},
		"include_expenses": true,
		"body":             importBody,
	})

	payment := c.h.callOK(ctx, "clockify_invoices_payments_create", map[string]any{
		"invoice_id": invoiceID,
		"amount":     1,
		"date":       time.Now().UTC().Format(time.RFC3339),
		"note":       c.LivePrefix("happy-payment", 0),
	})
	paymentID := requireToolEntityID(t, payment, "paymentId", "id")
	if payments := extractList(t, c.h.callOK(ctx, "clockify_invoices_payments_list", map[string]any{
		"invoice_id": invoiceID,
	}), "payments", "items"); len(payments) == 0 {
		t.Fatalf("clockify_invoices_payments_list returned no payments after create")
	}
	c.h.callOK(ctx, "clockify_invoices_payments_delete", map[string]any{
		"invoice_id": invoiceID,
		"payment_id": paymentID,
	})

	exported := extractDataMap(t, c.h.callOK(ctx, "clockify_invoices_export", map[string]any{
		"invoice_id": invoiceID,
		"format":     "PDF",
	}))
	if firstNonEmptyString(exported, "content", "body") == "" {
		t.Fatalf("clockify_invoices_export returned no content/body: %#v", exported)
	}

	c.h.callOK(ctx, "clockify_invoices_delete", map[string]any{
		"invoice_id": invoiceID,
	})
	invoiceDeleted = true
}

func exerciseExpenseHappyPath(t *testing.T, ctx context.Context, c *liveCampaignContext, expense paidFeatureExpense) {
	t.Helper()

	requireToolEntityID(t, c.h.callOK(ctx, "clockify_expenses_get", map[string]any{
		"expense_id": expense.ID,
	}), "expenseId", "id")
	c.h.callOK(ctx, "clockify_expenses_update", map[string]any{
		"expense_id":    expense.ID,
		"change_fields": []any{"NOTES", "AMOUNT"},
		"notes":         c.LivePrefix("happy-expense-updated", 0),
		"amount":        2.5,
	})
	c.h.callOK(ctx, "clockify_expenses_delete", map[string]any{
		"expense_id": expense.ID,
	})
	*expense.Deleted = true
}

func exerciseTimeOffHappyPath(t *testing.T, ctx context.Context, c *liveCampaignContext) {
	t.Helper()

	policyArchived := false
	policy := c.h.callOK(ctx, "clockify_time_off_policies_create", map[string]any{
		"name":              c.LivePrefix("happy-policy", 0),
		"time_unit":         "DAYS",
		"days_per_year":     5,
		"negative_balance":  true,
		"requires_approval": true,
	})
	policyID := requireToolEntityID(t, policy, "policyId", "id")
	c.RegisterCleanup("time-off-policy", policyID, func(ctx context.Context) error {
		if policyArchived {
			return nil
		}
		var ignored map[string]any
		return c.h.Service.Client.Patch(ctx, "/workspaces/"+c.WorkspaceID+"/time-off/policies/"+policyID, map[string]any{"status": "ARCHIVED"}, &ignored)
	})

	requireToolEntityID(t, c.h.callOK(ctx, "clockify_time_off_policies_get", map[string]any{
		"policy_id": policyID,
	}), "policyId", "id")
	c.h.callOK(ctx, "clockify_time_off_policies_update", map[string]any{
		"policy_id": policyID,
		"name":      c.LivePrefix("happy-policy-updated", 0),
	})
	start := liveFutureWeekdayDate(6)
	updateRequestFinalized := false
	request := c.h.callOK(ctx, "clockify_time_off_requests_create", map[string]any{
		"policy_id": policyID,
		"start":     start,
		"end":       start,
		"note":      c.LivePrefix("happy-time-off", 0),
	})
	requestID := requireToolEntityID(t, request, "requestId", "id")
	c.RegisterCleanup("time-off-update-request", requestID, func(ctx context.Context) error {
		if updateRequestFinalized {
			return nil
		}
		return c.rawDeletePath(ctx, "/time-off/policies/"+policyID+"/requests/"+requestID)
	})

	requireToolEntityID(t, c.h.callOK(ctx, "clockify_time_off_requests_get", map[string]any{
		"policy_id":  policyID,
		"request_id": requestID,
	}), "requestId", "id")

	deleteStart := liveFutureWeekdayDate(7)
	deleteRequestDeleted := false
	deleteRequest := c.h.callOK(ctx, "clockify_time_off_requests_create", map[string]any{
		"policy_id": policyID,
		"start":     deleteStart,
		"end":       deleteStart,
		"note":      c.LivePrefix("happy-time-off-delete", 0),
	})
	deleteRequestID := requireToolEntityID(t, deleteRequest, "requestId", "id")
	c.RegisterCleanup("time-off-delete-request", deleteRequestID, func(ctx context.Context) error {
		if deleteRequestDeleted {
			return nil
		}
		return c.rawDeletePath(ctx, "/time-off/policies/"+policyID+"/requests/"+deleteRequestID)
	})
	c.h.callOK(ctx, "clockify_time_off_requests_delete", map[string]any{
		"policy_id":  policyID,
		"request_id": deleteRequestID,
	})
	deleteRequestDeleted = true

	approveStart := liveFutureWeekdayDate(9)
	approveRequestFinalized := false
	approveRequest := c.h.callOK(ctx, "clockify_time_off_requests_create", map[string]any{
		"policy_id": policyID,
		"start":     approveStart,
		"end":       approveStart,
		"note":      c.LivePrefix("happy-time-off-approve", 0),
	})
	approveRequestID := requireToolEntityID(t, approveRequest, "requestId", "id")
	c.RegisterCleanup("time-off-approve-request", approveRequestID, func(ctx context.Context) error {
		if approveRequestFinalized {
			return nil
		}
		return c.rawDeletePath(ctx, "/time-off/policies/"+policyID+"/requests/"+approveRequestID)
	})
	requireToolEntityID(t, c.h.callOK(ctx, "clockify_time_off_approve", map[string]any{
		"policy_id":  policyID,
		"request_id": approveRequestID,
		"note":       c.LivePrefix("happy-time-off-approved-via-tool", 0),
	}), "requestId", "id")
	approveRequestFinalized = true

	denyStart := liveFutureWeekdayDate(10)
	denyRequestFinalized := false
	denyRequest := c.h.callOK(ctx, "clockify_time_off_requests_create", map[string]any{
		"policy_id": policyID,
		"start":     denyStart,
		"end":       denyStart,
		"note":      c.LivePrefix("happy-time-off-deny", 0),
	})
	denyRequestID := requireToolEntityID(t, denyRequest, "requestId", "id")
	c.RegisterCleanup("time-off-deny-request", denyRequestID, func(ctx context.Context) error {
		if denyRequestFinalized {
			return nil
		}
		return c.rawDeletePath(ctx, "/time-off/policies/"+policyID+"/requests/"+denyRequestID)
	})
	requireToolEntityID(t, c.h.callOK(ctx, "clockify_time_off_deny", map[string]any{
		"policy_id":  policyID,
		"request_id": denyRequestID,
		"note":       c.LivePrefix("happy-time-off-denied-via-tool", 0),
	}), "requestId", "id")
	denyRequestFinalized = true

	c.h.callOK(ctx, "clockify_time_off_requests_update", map[string]any{
		"policy_id":  policyID,
		"request_id": requestID,
		"status":     "APPROVED",
		"note":       c.LivePrefix("happy-time-off-approved", 0),
	})
	updateRequestFinalized = true

	c.h.callOK(ctx, "clockify_time_off_archive", map[string]any{
		"policy_id": policyID,
		"archived":  true,
	})
	policyArchived = true
}

func exerciseSchedulingHappyPath(t *testing.T, ctx context.Context, c *liveCampaignContext, projectID string) {
	t.Helper()

	start := time.Now().UTC().AddDate(0, 1, 0).Truncate(time.Second)
	end := start.AddDate(0, 0, 4)
	assignmentDeleted := false
	assignment := c.h.callOK(ctx, "clockify_scheduling_assignments_create", map[string]any{
		"user_id":       c.OwnerUserID,
		"project_id":    projectID,
		"start":         start.Format("2006-01-02T15:04:05Z"),
		"end":           end.Format("2006-01-02T15:04:05Z"),
		"hours_per_day": 1,
	})
	assignmentID := requireToolEntityID(t, assignment, "assignmentId", "id")
	c.RegisterCleanup("assignment", assignmentID, func(ctx context.Context) error {
		if assignmentDeleted {
			return nil
		}
		return c.rawDeletePath(ctx, "/scheduling/assignments/recurring/"+assignmentID)
	})

	rangeArgs := map[string]any{
		"assignment_id": assignmentID,
		"start":         start.AddDate(0, 0, -1).Format("2006-01-02T15:04:05Z"),
		"end":           end.AddDate(0, 0, 1).Format("2006-01-02T15:04:05Z"),
	}
	requireToolEntityID(t, c.h.callOK(ctx, "clockify_scheduling_assignments_get", rangeArgs), "assignmentId", "id")
	c.h.callOK(ctx, "clockify_scheduling_assignments_update", map[string]any{
		"assignment_id": assignmentID,
		"start":         start.AddDate(0, 0, 1).Format("2006-01-02T15:04:05Z"),
		"end":           end.AddDate(0, 0, 1).Format("2006-01-02T15:04:05Z"),
		"hours_per_day": 2,
	})
	c.h.callOK(ctx, "clockify_scheduling_assignments_delete", map[string]any{
		"assignment_id": assignmentID,
	})
	assignmentDeleted = true
}

func exerciseWebhookHappyPath(t *testing.T, ctx context.Context, c *liveCampaignContext) {
	t.Helper()

	webhookDeleted := false
	webhook := c.h.callOK(ctx, "clockify_webhooks_create", map[string]any{
		"name":          liveShortName("mcp-hook"),
		"url":           "https://example.com/clockify",
		"webhook_event": "NEW_TIME_ENTRY",
	})
	webhookID := requireToolEntityID(t, webhook, "webhookId", "id")
	c.RegisterCleanup("webhook", webhookID, func(ctx context.Context) error {
		if webhookDeleted {
			return nil
		}
		return c.rawDeletePath(ctx, "/webhooks/"+webhookID)
	})

	requireToolEntityID(t, c.h.callOK(ctx, "clockify_webhooks_get", map[string]any{
		"webhook_id": webhookID,
	}), "webhookId", "id")
	c.h.callOK(ctx, "clockify_webhooks_update", map[string]any{
		"webhook_id": webhookID,
		"name":       liveShortName("mcp-hook-upd"),
	})
	c.h.callOK(ctx, "clockify_webhooks_delete", map[string]any{
		"webhook_id": webhookID,
	})
	webhookDeleted = true
}

func exerciseGroupsHolidaysAndGovernanceHappyPaths(t *testing.T, ctx context.Context, c *liveCampaignContext, projectID string) {
	t.Helper()

	groupDeleted := false
	group := c.h.callOK(ctx, "clockify_groups_create", map[string]any{
		"name": c.LivePrefix("happy-group", 0),
	})
	groupID := requireToolEntityID(t, group, "groupId", "id")
	c.RegisterCleanup("happy-group", groupID, func(ctx context.Context) error {
		if groupDeleted {
			return nil
		}
		return c.rawDeletePath(ctx, "/user-groups/"+groupID)
	})

	requireToolEntityID(t, c.h.callOK(ctx, "clockify_groups_add_user", map[string]any{
		"group_id": groupID,
		"user_id":  c.OwnerUserID,
	}), "groupId", "id")
	requireToolEntityID(t, c.h.callOK(ctx, "clockify_groups_remove_user", map[string]any{
		"group_id": groupID,
		"user_id":  c.OwnerUserID,
	}), "groupId", "id")

	holidayDeleted := false
	holidayDay := time.Now().UTC().AddDate(1, 2, 0)
	holidayDate := holidayDay.Format("2006-01-02")
	holiday := c.h.callOK(ctx, "clockify_holidays_create", map[string]any{
		"name":       c.LivePrefix("happy-holiday", 0),
		"start_date": holidayDate,
		"end_date":   holidayDate,
		"user_ids":   []any{c.OwnerUserID},
	})
	holidayID := requireToolEntityID(t, holiday, "holidayId", "id")
	c.RegisterCleanup("happy-holiday", holidayID, func(ctx context.Context) error {
		if holidayDeleted {
			return nil
		}
		return c.rawDeletePath(ctx, "/holidays/"+holidayID)
	})

	requireListContainsID(t, c.h.callOK(ctx, "clockify_holidays_list_for_user_period", map[string]any{
		"user_id": c.OwnerUserID,
		"start":   holidayDay.Format("2006-01-02T00:00:00.000Z"),
		"end":     holidayDay.AddDate(0, 0, 1).Format("2006-01-02T00:00:00.000Z"),
	}), holidayID)
	c.h.callOK(ctx, "clockify_holidays_delete", map[string]any{"holiday_id": holidayID})
	holidayDeleted = true

	c.h.callOK(ctx, "clockify_groups_delete", map[string]any{"group_id": groupID})
	groupDeleted = true
}

func requireListContainsID(t *testing.T, envelope map[string]any, wantID string, fields ...string) {
	t.Helper()
	for _, raw := range extractList(t, envelope, fields...) {
		item, _ := raw.(map[string]any)
		if item == nil {
			continue
		}
		if id := firstNonEmptyString(item, "id", "_id", "clientId", "projectId", "taskId", "holidayId"); id == wantID {
			return
		}
	}
	t.Fatalf("list result did not include id %s: %#v", wantID, structuredContentMap(t, envelope))
}

func requireToolEntityID(t *testing.T, envelope map[string]any, keys ...string) string {
	t.Helper()
	sc := structuredContentMap(t, envelope)
	if ids, _ := sc["ids"].(map[string]any); ids != nil {
		if id := firstNonEmptyString(ids, keys...); id != "" {
			return id
		}
	}
	if data, _ := sc["data"].(map[string]any); data != nil {
		if id := firstNonEmptyString(data, keys...); id != "" {
			return id
		}
	}
	t.Fatalf("result missing entity id keys %v: %#v", keys, sc)
	return ""
}

func firstListEntityID(t *testing.T, envelope map[string]any) string {
	t.Helper()
	for _, raw := range extractList(t, envelope) {
		if item, _ := raw.(map[string]any); item != nil {
			if id := firstNonEmptyString(item, "id", "_id", "categoryId"); id != "" {
				return id
			}
		}
	}
	return ""
}

func firstNonEmptyString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, _ := values[key].(string); strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
