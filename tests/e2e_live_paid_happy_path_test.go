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
	ClientID  string
	ProjectID string
	EntryID   string
}

type paidFeatureExpense struct {
	ID      string
	Deleted *bool
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

	start := time.Now().UTC().Add(-72 * time.Hour).Truncate(time.Second)
	entry := c.h.callOK(ctx, "clockify_entries_create", map[string]any{
		"start":         start.Format(time.RFC3339),
		"end":           start.Add(30 * time.Minute).Format(time.RFC3339),
		"description":   c.LivePrefix("happy-entry", 0),
		"project_id":    projectID,
		"billable":      true,
		"allow_overlap": true,
	})
	entryID := requireToolEntityID(t, entry, "entryId", "id")
	c.RegisterCleanup("entry", entryID, func(ctx context.Context) error {
		return c.rawDeletePath(ctx, "/time-entries/"+entryID)
	})

	return paidFeatureWorkPackage{
		ClientID:  clientID,
		ProjectID: projectID,
		EntryID:   entryID,
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
	return paidFeatureExpense{ID: expenseID, Deleted: &expenseDeleted}
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
