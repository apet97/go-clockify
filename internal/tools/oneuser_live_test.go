package tools

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

func TestOneUserLiveWorkflow(t *testing.T) {
	if os.Getenv("CLOCKIFY_RUN_LIVE_E2E") != "1" {
		t.Skip("set CLOCKIFY_RUN_LIVE_E2E=1 with CLOCKIFY_API_KEY, CLOCKIFY_WORKSPACE_ID, and CLOCKIFY_LIVE_PREFIX to run live Clockify workflow tests")
	}
	apiKey := strings.TrimSpace(os.Getenv("CLOCKIFY_API_KEY"))
	workspaceID := strings.TrimSpace(os.Getenv("CLOCKIFY_WORKSPACE_ID"))
	prefix := strings.TrimSpace(os.Getenv("CLOCKIFY_LIVE_PREFIX"))
	if apiKey == "" || workspaceID == "" || prefix == "" {
		t.Fatal("CLOCKIFY_API_KEY, CLOCKIFY_WORKSPACE_ID, and CLOCKIFY_LIVE_PREFIX are required when CLOCKIFY_RUN_LIVE_E2E=1")
	}

	client := clockify.NewClient(apiKey, defaultLiveBaseURL(), 30*time.Second, 2)
	defer client.Close()
	svc := New(client, workspaceID)
	svc.DefaultTimezone = time.UTC
	server := mcp.NewServer("live", svc.FullAccessRegistry(), nil, nil)
	initializeServer(t, server)
	runID := cleanDemoRunID(prefix)
	if runID == "" {
		runID = "live"
	}
	date := "2026-01-02"

	status := callLiveToolDataOrRecovery(t, server, "clockify_status", nil)
	requireLiveID(t, status, "workspaceId")
	requireLiveID(t, status, "userId")

	seed := callLiveToolOKOrRecovery(t, server, "clockify_demo_seed", map[string]any{"run_id": runID, "prefix": prefix, "date": date})
	for _, key := range []string{"clientId", "projectId", "taskId", "tagId", "entryId"} {
		requireLiveID(t, seed, key)
	}

	logged := callLiveToolOKOrRecovery(t, server, "clockify_log_work", map[string]any{
		"start":       date + " 10:30",
		"end":         date + " 11:00",
		"description": prefix + " Live logged work",
		"project_id":  seed.IDs["projectId"],
		"task_id":     seed.IDs["taskId"],
		"tag_ids":     []any{seed.IDs["tagId"]},
		"billable":    true,
	})
	requireLiveID(t, logged, "entryId")

	startAt := time.Now().UTC().Add(-4 * time.Minute).Format(time.RFC3339)
	started := callLiveToolOKOrRecovery(t, server, "clockify_start_work", map[string]any{
		"start":       startAt,
		"description": prefix + " Live started work",
		"project_id":  seed.IDs["projectId"],
		"task_id":     seed.IDs["taskId"],
		"tag_ids":     []any{seed.IDs["tagId"]},
	})
	requireLiveID(t, started, "entryId")

	switched := callLiveToolOKOrRecovery(t, server, "clockify_switch_work", map[string]any{
		"start":       time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339),
		"description": prefix + " Live switched work",
		"project_id":  seed.IDs["projectId"],
		"task_id":     seed.IDs["taskId"],
		"tag_ids":     []any{seed.IDs["tagId"]},
	})
	requireLiveID(t, switched, "entryId")

	stopped := callLiveToolOKOrRecovery(t, server, "clockify_stop_work", nil)
	requireLiveID(t, stopped, "entryId")

	fixed := callLiveToolOKOrRecovery(t, server, "clockify_fix_entry", map[string]any{
		"entry_id":    logged.IDs["entryId"],
		"description": prefix + " Live fixed logged work",
	})
	requireLiveID(t, fixed, "entryId")

	day := callLiveToolDataOrRecovery(t, server, "clockify_review_day", map[string]any{"date": date, "include_entries": true})
	if day.Data == nil {
		t.Fatalf("live day review missing data: %+v", day)
	}

	week := callLiveToolDataOrRecovery(t, server, "clockify_review_week", map[string]any{"week_start": "2025-12-29", "include_entries": true})
	if week.Data == nil {
		t.Fatalf("live week review missing data: %+v", week)
	}

	cleanup := callLiveToolOKOrRecovery(t, server, "clockify_demo_cleanup", map[string]any{"run_id": runID, "prefix": prefix, "start": "2026-01-01 00:00", "end": time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)})
	requireLiveID(t, cleanup, "workspaceId")
}

func TestOneUserLivePaidFeatureWorkflowRecovery(t *testing.T) {
	if os.Getenv("CLOCKIFY_RUN_LIVE_E2E") != "1" || os.Getenv("CLOCKIFY_LIVE_HIGH_RISK_WORKFLOWS") != "1" {
		t.Skip("set CLOCKIFY_RUN_LIVE_E2E=1 and CLOCKIFY_LIVE_HIGH_RISK_WORKFLOWS=1 to probe paid/high-risk workflow tools")
	}
	apiKey := strings.TrimSpace(os.Getenv("CLOCKIFY_API_KEY"))
	workspaceID := strings.TrimSpace(os.Getenv("CLOCKIFY_WORKSPACE_ID"))
	prefix := strings.TrimSpace(os.Getenv("CLOCKIFY_LIVE_PREFIX"))
	if apiKey == "" || workspaceID == "" || prefix == "" {
		t.Fatal("CLOCKIFY_API_KEY, CLOCKIFY_WORKSPACE_ID, and CLOCKIFY_LIVE_PREFIX are required when CLOCKIFY_RUN_LIVE_E2E=1")
	}

	client := clockify.NewClient(apiKey, defaultLiveBaseURL(), 30*time.Second, 2)
	defer client.Close()
	svc := New(client, workspaceID)
	svc.DefaultTimezone = time.UTC
	server := mcp.NewServer("live", svc.FullAccessRegistry(), nil, nil)
	initializeServer(t, server)
	runID := cleanDemoRunID(prefix)
	if runID == "" {
		runID = "live-paid"
	}
	seed := callLiveToolOKOrRecovery(t, server, "clockify_demo_seed", map[string]any{"run_id": runID, "prefix": prefix, "date": "2026-01-02"})
	status := callLiveToolDataOrRecovery(t, server, "clockify_status", nil)
	userID := requireLiveID(t, status, "userId")
	defer func() {
		callLiveToolOKOrRecovery(t, server, "clockify_demo_cleanup", map[string]any{"run_id": runID, "prefix": prefix, "start": "2026-01-01 00:00", "end": time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)})
	}()

	callLiveToolOKOrRecovery(t, server, "clockify_invoice_client_work", map[string]any{"client_id": seed.IDs["clientId"], "number": "MCP-LIVE-" + runID})
	callLiveToolOKOrRecovery(t, server, "clockify_record_expense", map[string]any{"amount": float64(10), "category": prefix, "project_id": seed.IDs["projectId"], "date": "2026-01-02T00:00:00Z"})
	callLiveToolOKOrRecovery(t, server, "clockify_request_time_off", map[string]any{"policy": prefix, "start": "2026-01-05", "end": "2026-01-06"})
	callLiveToolOKOrRecovery(t, server, "clockify_schedule_work", map[string]any{"user_id": userID, "project_id": seed.IDs["projectId"], "start": "2026-01-05T09:00:00Z", "end": "2026-01-09T17:00:00Z"})
	callLiveToolOKOrRecovery(t, server, "clockify_setup_webhook", map[string]any{"name": prefix + " Live webhook", "url": "https://example.com/clockify", "event": "NEW_TIME_ENTRY"})
}

func TestOneUserLiveOptionalDomainContracts(t *testing.T) {
	if os.Getenv("CLOCKIFY_RUN_LIVE_E2E") != "1" || os.Getenv("CLOCKIFY_LIVE_OPTIONAL_DOMAINS") != "1" {
		t.Skip("set CLOCKIFY_RUN_LIVE_E2E=1 and CLOCKIFY_LIVE_OPTIONAL_DOMAINS=1 to probe optional-domain tools")
	}
	apiKey := strings.TrimSpace(os.Getenv("CLOCKIFY_API_KEY"))
	workspaceID := strings.TrimSpace(os.Getenv("CLOCKIFY_WORKSPACE_ID"))
	prefix := strings.TrimSpace(os.Getenv("CLOCKIFY_LIVE_PREFIX"))
	if apiKey == "" || workspaceID == "" || prefix == "" {
		t.Fatal("CLOCKIFY_API_KEY, CLOCKIFY_WORKSPACE_ID, and CLOCKIFY_LIVE_PREFIX are required when CLOCKIFY_RUN_LIVE_E2E=1")
	}

	client := clockify.NewClient(apiKey, defaultLiveBaseURL(), 30*time.Second, 2)
	defer client.Close()
	svc := New(client, workspaceID)
	svc.DefaultTimezone = time.UTC
	server := mcp.NewServer("live", svc.FullAccessRegistry(), nil, nil)
	initializeServer(t, server)

	runID := cleanDemoRunID(prefix)
	if runID == "" {
		runID = "live-optional"
	}
	nameSuffix := liveOptionalName(prefix, runID, "opt", 18)
	status := callLiveToolDataOrRecovery(t, server, "clockify_status", nil)
	userID := status.IDs["userId"]
	if userID == "" {
		t.Fatalf("clockify_status did not return userId: %+v", status)
	}
	workPackage := callLiveToolOKOrRecovery(t, server, "clockify_create_work_package", map[string]any{
		"client":  liveOptionalName(prefix, runID, "client", 80),
		"project": liveOptionalName(prefix, runID, "project", 80),
		"task":    liveOptionalName(prefix, runID, "task", 80),
		"tag":     liveOptionalName(prefix, runID, "tag", 80),
	})
	var entry liveToolEnvelope
	if workPackage.OK {
		entry = callLiveToolOKOrRecovery(t, server, "clockify_log_work", map[string]any{
			"start":       "2026-01-06T09:00:00Z",
			"end":         "2026-01-06T10:00:00Z",
			"description": liveOptionalName(prefix, runID, "invoiceable work", 120),
			"project_id":  requireLiveID(t, workPackage, "projectId"),
			"task_id":     workPackage.IDs["taskId"],
		})
	}

	for _, probe := range []struct {
		name string
		args map[string]any
	}{
		{"clockify_invoices_list", map[string]any{"page_size": float64(1)}},
		{"clockify_expenses_categories_list", nil},
		{"clockify_time_off_policies_list", map[string]any{"page_size": float64(1)}},
		{"clockify_scheduling_assignments_list", map[string]any{"start": "2026-01-05", "end": "2026-01-09"}},
		{"clockify_webhooks_events", nil},
		{"clockify_groups_list", map[string]any{"page_size": float64(1)}},
		{"clockify_holidays_list", nil},
	} {
		callLiveToolDataOrRecovery(t, server, probe.name, probe.args)
	}

	callLiveToolOKOrRecovery(t, server, "clockify_invoices_create", map[string]any{
		"client_id":   workPackage.IDs["clientId"],
		"number":      "MCP-LIVE-" + runID,
		"issued_date": "2026-01-06T00:00:00Z",
		"due_date":    "2026-01-31T00:00:00Z",
		"currency":    "USD",
	})
	callLiveToolOKOrRecovery(t, server, "clockify_invoices_send", map[string]any{"invoice_id": "65b382b606de527a7ee2b619"})
	callLiveToolOKOrRecovery(t, server, "clockify_expenses_create", map[string]any{
		"amount":      float64(5),
		"date":        "2026-01-06T00:00:00Z",
		"category_id": "65b382b606de527a7ee2b612",
		"project_id":  workPackage.IDs["projectId"],
		"notes":       liveOptionalName(prefix, runID, "expense", 120),
	})
	callLiveToolOKOrRecovery(t, server, "clockify_time_off_requests_create", map[string]any{
		"policy_id": "65b382b606de527a7ee2b61c",
		"start":     "2026-01-07",
		"end":       "2026-01-07",
		"note":      liveOptionalName(prefix, runID, "time off", 120),
	})
	callLiveToolOKOrRecovery(t, server, "clockify_scheduling_assignments_create", map[string]any{
		"user_id":       userID,
		"project_id":    workPackage.IDs["projectId"],
		"start":         "2026-01-12T09:00:00Z",
		"end":           "2026-01-16T17:00:00Z",
		"hours_per_day": float64(4),
	})
	callLiveToolOKOrRecovery(t, server, "clockify_users_invite", map[string]any{
		"email":      "mcp-live-" + runID + "@example.com",
		"send_email": false,
	})
	if entry.OK {
		callLiveToolOKOrRecovery(t, server, "clockify_entries_mark_invoiced", map[string]any{
			"time_entry_ids": []any{requireLiveID(t, entry, "entryId")},
			"invoiced":       true,
		})
	}

	group := callLiveToolOKOrRecovery(t, server, "clockify_groups_create", map[string]any{"name": liveOptionalName(prefix, runID, "group", 80)})
	if group.OK {
		requireLiveID(t, group, "groupId")
	}

	holiday := callLiveToolOKOrRecovery(t, server, "clockify_holidays_create", map[string]any{
		"name":       liveOptionalName(prefix, runID, "holiday", 80),
		"start_date": "2026-01-12",
		"end_date":   "2026-01-12",
		"user_ids":   []any{userID},
	})
	if holiday.OK {
		requireLiveID(t, holiday, "holidayId")
	}

	webhook := callLiveToolOKOrRecovery(t, server, "clockify_webhooks_create", map[string]any{
		"name":          liveOptionalName("mcp", nameSuffix, "webhook", 30),
		"url":           "https://example.com/clockify",
		"webhook_event": "NEW_TIME_ENTRY",
	})
	if webhook.OK {
		requireLiveID(t, webhook, "webhookId")
	}
}

func TestOneUserLiveRemainingCoverageProbes(t *testing.T) {
	apiKey, workspaceID, prefix := requireRemainingCoverageLiveEnv(t)

	client := clockify.NewClient(apiKey, defaultLiveBaseURL(), 30*time.Second, 2)
	defer client.Close()
	svc := New(client, workspaceID)
	svc.DefaultTimezone = time.UTC
	server := mcp.NewServer("live-remaining", svc.FullAccessRegistry(), nil, nil)
	initializeServer(t, server)

	runID := cleanDemoRunID(prefix)
	if runID == "" {
		runID = "live-remaining"
	}
	runID = cleanDemoRunID(runID + "-remaining")
	unique := func(kind string) string {
		return liveOptionalName("mcp-remaining", runID, kind, 80)
	}
	bogusID := "000000000000000000000001"

	status := requireLiveOK(t, server, "clockify_status", nil)
	userID := requireLiveID(t, status, "userId")

	callLiveToolDataOrRecovery(t, server, "clockify_clients_list", nil)
	clientResp := requireLiveOK(t, server, "clockify_clients_create", map[string]any{"name": unique("client")})
	clientID := requireLiveID(t, clientResp, "clientId")
	requireLiveOK(t, server, "clockify_clients_get", map[string]any{"client_id": clientID})
	requireLiveOK(t, server, "clockify_clients_update", map[string]any{"client_id": clientID, "note": "live coverage probe"})

	callLiveToolDataOrRecovery(t, server, "clockify_projects_list", nil)
	projectResp := requireLiveOK(t, server, "clockify_projects_create", map[string]any{
		"name":      unique("project"),
		"client_id": clientID,
		"billable":  true,
	})
	projectID := requireLiveID(t, projectResp, "projectId")
	secondProjectResp := requireLiveOK(t, server, "clockify_projects_create", map[string]any{
		"name":     unique("timer-project"),
		"billable": true,
	})
	secondProjectID := requireLiveID(t, secondProjectResp, "projectId")
	requireLiveOK(t, server, "clockify_projects_update", map[string]any{"project_id": projectID, "note": "live coverage probe"})
	callLiveToolDataOrRecovery(t, server, "clockify_projects_rates_update", map[string]any{
		"project_id": projectID,
		"user_id":    userID,
		"rate_kind":  "hourly",
		"amount":     1,
	})

	taskResp := requireLiveOK(t, server, "clockify_tasks_create", map[string]any{
		"project_id": projectID,
		"name":       unique("task"),
	})
	taskID := requireLiveID(t, taskResp, "taskId")
	requireLiveOK(t, server, "clockify_tasks_get", map[string]any{"project_id": projectID, "task_id": taskID})
	requireLiveOK(t, server, "clockify_tasks_update", map[string]any{"project_id": projectID, "task_id": taskID, "billable": true})
	callLiveToolDataOrRecovery(t, server, "clockify_tasks_rates_update", map[string]any{
		"project_id": projectID,
		"task_id":    taskID,
		"rate_kind":  "hourly",
		"amount":     1,
	})

	tagResp := requireLiveOK(t, server, "clockify_tags_create", map[string]any{"name": unique("tag")})
	tagID := requireLiveID(t, tagResp, "tagId")
	requireLiveOK(t, server, "clockify_tags_get", map[string]any{"tag_id": tagID})
	requireLiveOK(t, server, "clockify_tags_update", map[string]any{"tag_id": tagID, "name": unique("tag-renamed")})

	entryResp := requireLiveOK(t, server, "clockify_entries_create", map[string]any{
		"start":         "2026-02-03T09:00:00Z",
		"end":           "2026-02-03T10:00:00Z",
		"description":   unique("entry"),
		"project_id":    projectID,
		"task_id":       taskID,
		"tag_ids":       []any{tagID},
		"allow_overlap": true,
	})
	entryID := requireLiveID(t, entryResp, "entryId")
	requireLiveOK(t, server, "clockify_entries_get", map[string]any{"entry_id": entryID})
	requireLiveOK(t, server, "clockify_entries_update", map[string]any{"entry_id": entryID, "description": unique("entry-updated")})

	callLiveToolDataOrRecovery(t, server, "clockify_entries_running", nil)
	callLiveToolDataOrRecovery(t, server, "clockify_entries_timer_status", nil)
	callLiveToolDataOrRecovery(t, server, "clockify_entries_timer_stop", nil)
	timerStart := requireLiveOK(t, server, "clockify_entries_timer_start", map[string]any{
		"project_id":  projectID,
		"description": unique("timer-start"),
	})
	requireLiveID(t, timerStart, "entryId")
	timerSwitch := callLiveToolOKOrRecovery(t, server, "clockify_entries_timer_switch", map[string]any{
		"project":     unique("timer-project"),
		"description": unique("timer-switch"),
	})
	var timerEntryID string
	if timerSwitch.OK {
		timerEntryID = timerSwitch.IDs["entryId"]
	}
	timerStop := callLiveToolOKOrRecovery(t, server, "clockify_entries_timer_stop", nil)
	if timerStop.OK && timerStop.IDs["entryId"] != "" {
		timerEntryID = timerStop.IDs["entryId"]
	}

	invoiceID := bogusID
	invoice := callLiveToolOKOrRecovery(t, server, "clockify_invoices_create", map[string]any{
		"client_id":   clientID,
		"number":      "MCP-LIVE-" + runID,
		"issued_date": "2026-02-03T00:00:00Z",
		"due_date":    "2026-02-28T00:00:00Z",
		"currency":    "USD",
	})
	if invoice.OK && invoice.IDs["invoiceId"] != "" {
		invoiceID = invoice.IDs["invoiceId"]
	}
	invoiceItemArgs := map[string]any{
		"invoice_id":  invoiceID,
		"item_index":  "0",
		"item_type":   "SERVICE",
		"description": "Live coverage item",
		"quantity":    1,
		"unit_price":  1,
	}
	for _, probe := range []struct {
		name string
		args map[string]any
	}{
		{"clockify_invoices_get", map[string]any{"invoice_id": invoiceID}},
		{"clockify_invoices_items_list", map[string]any{"invoice_id": invoiceID}},
		{"clockify_invoices_update", map[string]any{"invoice_id": invoiceID, "note": "live coverage probe"}},
		{"clockify_invoices_mark_paid", map[string]any{"invoice_id": invoiceID}},
		{"clockify_invoices_items_add", invoiceItemArgs},
		{"clockify_invoices_items_update", invoiceItemArgs},
		{"clockify_invoices_items_delete", invoiceItemArgs},
		{"clockify_invoices_export", map[string]any{"invoice_id": invoiceID, "format": "PDF"}},
		{"clockify_invoices_import_time", map[string]any{"invoice_id": invoiceID, "time_entry_ids": []any{entryID}}},
		{"clockify_invoices_import_expenses", map[string]any{"invoice_id": invoiceID, "expense_ids": []any{bogusID}, "include_expenses": true}},
		{"clockify_invoices_payments_list", map[string]any{"invoice_id": invoiceID}},
		{"clockify_invoices_payments_create", map[string]any{"invoice_id": invoiceID, "amount": 1, "date": "2026-02-03T00:00:00Z", "note": "live coverage"}},
		{"clockify_invoices_payments_delete", map[string]any{"invoice_id": invoiceID, "payment_id": bogusID}},
		{"clockify_invoices_delete", map[string]any{"invoice_id": invoiceID}},
	} {
		callLiveToolDataOrRecovery(t, server, probe.name, probe.args)
	}

	callLiveToolDataOrRecovery(t, server, "clockify_expenses_categories_delete", map[string]any{"category_id": bogusID})

	policyID := bogusID
	policy := callLiveToolOKOrRecovery(t, server, "clockify_time_off_policies_create", map[string]any{
		"name":              unique("policy"),
		"time_unit":         "DAYS",
		"days_per_year":     1,
		"requires_approval": false,
	})
	if policy.OK && policy.IDs["policyId"] != "" {
		policyID = policy.IDs["policyId"]
	}
	for _, probe := range []struct {
		name string
		args map[string]any
	}{
		{"clockify_time_off_policies_get", map[string]any{"policy_id": policyID}},
		{"clockify_time_off_policies_update", map[string]any{"policy_id": policyID, "name": unique("policy-updated")}},
		{"clockify_time_off_balances", map[string]any{"policy_id": policyID, "user_id": userID}},
		{"clockify_time_off_requests_get", map[string]any{"request_id": bogusID}},
		{"clockify_time_off_requests_update", map[string]any{"policy_id": policyID, "request_id": bogusID, "note": "live coverage"}},
		{"clockify_time_off_requests_delete", map[string]any{"policy_id": policyID, "request_id": bogusID}},
		{"clockify_time_off_approve", map[string]any{"policy_id": policyID, "request_id": bogusID, "note": "live coverage"}},
		{"clockify_time_off_deny", map[string]any{"policy_id": policyID, "request_id": bogusID, "note": "live coverage"}},
		{"clockify_time_off_archive", map[string]any{"policy_id": policyID, "archived": true}},
	} {
		callLiveToolDataOrRecovery(t, server, probe.name, probe.args)
	}

	assignmentID := bogusID
	assignment := callLiveToolOKOrRecovery(t, server, "clockify_scheduling_assignments_create", map[string]any{
		"user_id":       userID,
		"project_id":    projectID,
		"start":         "2026-02-09T09:00:00Z",
		"end":           "2026-02-13T17:00:00Z",
		"hours_per_day": 1,
	})
	if assignment.OK && assignment.IDs["assignmentId"] != "" {
		assignmentID = assignment.IDs["assignmentId"]
	}
	callLiveToolDataOrRecovery(t, server, "clockify_scheduling_assignments_get", map[string]any{"assignment_id": assignmentID})
	callLiveToolDataOrRecovery(t, server, "clockify_scheduling_assignments_update", map[string]any{
		"assignment_id": assignmentID,
		"start":         "2026-02-10T09:00:00Z",
		"end":           "2026-02-14T17:00:00Z",
		"hours_per_day": 2,
	})
	callLiveToolDataOrRecovery(t, server, "clockify_scheduling_assignments_delete", map[string]any{"assignment_id": assignmentID})

	webhookID := bogusID
	webhook := callLiveToolOKOrRecovery(t, server, "clockify_webhooks_create", map[string]any{
		"name":          unique("webhook"),
		"url":           "https://example.com/clockify",
		"webhook_event": "NEW_TIME_ENTRY",
	})
	if webhook.OK && webhook.IDs["webhookId"] != "" {
		webhookID = webhook.IDs["webhookId"]
	}
	for _, probe := range []struct {
		name string
		args map[string]any
	}{
		{"clockify_webhooks_get", map[string]any{"webhook_id": webhookID}},
		{"clockify_webhooks_update", map[string]any{"webhook_id": webhookID, "name": unique("webhook-updated")}},
		{"clockify_webhooks_test", map[string]any{"webhook_id": webhookID}},
		{"clockify_webhooks_delete", map[string]any{"webhook_id": webhookID}},
	} {
		callLiveToolDataOrRecovery(t, server, probe.name, probe.args)
	}

	groupID := bogusID
	group := callLiveToolOKOrRecovery(t, server, "clockify_groups_create", map[string]any{"name": unique("group")})
	if group.OK && group.IDs["groupId"] != "" {
		groupID = group.IDs["groupId"]
	}
	callLiveToolDataOrRecovery(t, server, "clockify_groups_add_user", map[string]any{"group_id": groupID, "user_id": userID})
	callLiveToolDataOrRecovery(t, server, "clockify_groups_remove_user", map[string]any{"group_id": groupID, "user_id": userID})
	callLiveToolDataOrRecovery(t, server, "clockify_groups_delete", map[string]any{"group_id": groupID})

	holidayID := bogusID
	holidayDate := time.Now().UTC().AddDate(1, 1, 0).Format("2006-01-02")
	holiday := callLiveToolOKOrRecovery(t, server, "clockify_holidays_create", map[string]any{
		"name":       unique("holiday"),
		"start_date": holidayDate,
		"end_date":   holidayDate,
		"user_ids":   []any{userID},
	})
	if holiday.OK && holiday.IDs["holidayId"] != "" {
		holidayID = holiday.IDs["holidayId"]
	}
	callLiveToolDataOrRecovery(t, server, "clockify_holidays_get", map[string]any{"holiday_id": holidayID})
	callLiveToolDataOrRecovery(t, server, "clockify_holidays_update", map[string]any{
		"holiday_id":      holidayID,
		"name":            unique("holiday-updated"),
		"start_date":      holidayDate,
		"end_date":        holidayDate,
		"occurs_annually": false,
		"user_ids":        []any{userID},
		"user_group_ids":  []any{},
	})
	callLiveToolDataOrRecovery(t, server, "clockify_holidays_list_for_user_period", map[string]any{
		"user_id": userID,
		"start":   holidayDate,
		"end":     holidayDate,
	})
	callLiveToolDataOrRecovery(t, server, "clockify_holidays_delete", map[string]any{"holiday_id": holidayID})

	for _, probe := range []struct {
		name string
		args map[string]any
	}{
		{"clockify_users_deactivate", map[string]any{"user_id": bogusID}},
		{"clockify_users_role", map[string]any{"user_id": bogusID, "role": "WORKSPACE_ADMIN"}},
		{"clockify_workspace_settings", nil},
		{"clockify_projects_memberships_list", map[string]any{"project_id": projectID}},
		{"clockify_reports_attendance", reportProbeArgs()},
		{"clockify_reports_money", reportProbeArgs()},
		{"clockify_reports_export", map[string]any{"start": "2026-02-03T00:00:00.000", "end": "2026-02-04T00:00:00.000", "export_type": "JSON"}},
		{"clockify_approvals_get", map[string]any{"approval_id": bogusID}},
		{"clockify_approvals_submit", map[string]any{"period": "WEEKLY", "period_start": "2026-02-02T00:00:00.000Z"}},
		{"clockify_approvals_approve", map[string]any{"approval_id": bogusID, "note": "live coverage"}},
		{"clockify_approvals_reject", map[string]any{"approval_id": bogusID, "reason": "live coverage"}},
		{"clockify_approvals_withdraw", map[string]any{"approval_id": bogusID, "note": "live coverage"}},
		{"clockify_approvals_resubmit", map[string]any{"approval_id": bogusID, "entry_ids": []any{entryID}, "note": "live coverage"}},
	} {
		callLiveToolDataOrRecovery(t, server, probe.name, probe.args)
	}

	if timerEntryID != "" {
		callLiveToolDataOrRecovery(t, server, "clockify_entries_delete", map[string]any{"entry_id": timerEntryID})
	}
	requireLiveOK(t, server, "clockify_entries_delete", map[string]any{"entry_id": entryID})
	requireLiveOK(t, server, "clockify_tasks_delete", map[string]any{"project_id": projectID, "task_id": taskID})
	requireLiveOK(t, server, "clockify_tags_delete", map[string]any{"tag_id": tagID})
	requireLiveOK(t, server, "clockify_projects_delete", map[string]any{"project_id": secondProjectID})
	requireLiveOK(t, server, "clockify_projects_delete", map[string]any{"project_id": projectID})
	requireLiveOK(t, server, "clockify_clients_delete", map[string]any{"client_id": clientID})
}

func requireRemainingCoverageLiveEnv(t *testing.T) (string, string, string) {
	t.Helper()
	if os.Getenv("CLOCKIFY_RUN_LIVE_E2E") != "1" ||
		os.Getenv("CLOCKIFY_LIVE_OPTIONAL_DOMAINS") != "1" ||
		os.Getenv("CLOCKIFY_LIVE_HIGH_RISK_WORKFLOWS") != "1" {
		t.Skip("set CLOCKIFY_RUN_LIVE_E2E=1, CLOCKIFY_LIVE_OPTIONAL_DOMAINS=1, and CLOCKIFY_LIVE_HIGH_RISK_WORKFLOWS=1 to run remaining live coverage probes")
	}
	for _, gate := range []string{"CLOCKIFY_LIVE_ADMIN_ENABLED", "CLOCKIFY_LIVE_BILLING_ENABLED", "CLOCKIFY_LIVE_SETTINGS_ENABLED"} {
		if os.Getenv(gate) != "true" {
			t.Skipf("set %s=true to run remaining live coverage probes", gate)
		}
	}
	apiKey := strings.TrimSpace(os.Getenv("CLOCKIFY_API_KEY"))
	workspaceID := strings.TrimSpace(os.Getenv("CLOCKIFY_WORKSPACE_ID"))
	prefix := strings.TrimSpace(os.Getenv("CLOCKIFY_LIVE_PREFIX"))
	if apiKey == "" || workspaceID == "" || prefix == "" {
		t.Fatal("CLOCKIFY_API_KEY, CLOCKIFY_WORKSPACE_ID, and CLOCKIFY_LIVE_PREFIX are required")
	}
	if confirm := strings.TrimSpace(os.Getenv("CLOCKIFY_LIVE_WORKSPACE_CONFIRM")); confirm != workspaceID {
		t.Fatalf("CLOCKIFY_LIVE_WORKSPACE_CONFIRM=%q does not match CLOCKIFY_WORKSPACE_ID", confirm)
	}
	return apiKey, workspaceID, prefix
}

func requireLiveOK(t *testing.T, server *mcp.Server, name string, args map[string]any) liveToolEnvelope {
	t.Helper()
	envelope := callLiveToolDataOrRecovery(t, server, name, args)
	if !envelope.OK {
		t.Fatalf("%s returned recovery where success is required: code=%s hint=%s", name, envelope.Error.Code, envelope.Recovery.Hint)
	}
	return envelope
}

func reportProbeArgs() map[string]any {
	return map[string]any{
		"start":     "2026-02-03T00:00:00.000",
		"end":       "2026-02-04T00:00:00.000",
		"page":      1,
		"page_size": 10,
	}
}

type liveToolEnvelope struct {
	OK       bool              `json:"ok"`
	IDs      map[string]string `json:"ids,omitempty"`
	Data     any               `json:"data,omitempty"`
	Error    ErrorInfo         `json:"error,omitempty"`
	Recovery RecoveryHint      `json:"recovery,omitempty"`
}

func callLiveToolOKOrRecovery(t *testing.T, server *mcp.Server, name string, args map[string]any) liveToolEnvelope {
	t.Helper()
	envelope := callLiveToolDataOrRecovery(t, server, name, args)
	if envelope.OK && len(envelope.IDs) == 0 {
		t.Fatalf("%s succeeded without returning IDs", name)
	}
	return envelope
}

func callLiveToolDataOrRecovery(t *testing.T, server *mcp.Server, name string, args map[string]any) liveToolEnvelope {
	t.Helper()
	assertNoDryRunArgs(t, name, args)
	raw := callToolRaw(t, server, name, args)
	var resp struct {
		Result struct {
			StructuredContent liveToolEnvelope `json:"structuredContent"`
		} `json:"result"`
		Error any `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil {
		t.Fatalf("%s returned RPC error: %s", name, raw)
	}
	if resp.Result.StructuredContent.OK {
		return resp.Result.StructuredContent
	}
	if resp.Result.StructuredContent.Error.Code == "" || resp.Result.StructuredContent.Recovery.Hint == "" {
		t.Fatalf("%s did not return useful recovery: %s", name, raw)
	}
	return resp.Result.StructuredContent
}

func requireLiveID(t *testing.T, envelope liveToolEnvelope, key string) string {
	t.Helper()
	if envelope.IDs[key] == "" {
		t.Fatalf("live tool result missing %s: %+v", key, envelope.IDs)
	}
	return envelope.IDs[key]
}

func assertNoDryRunArgs(t *testing.T, name string, args map[string]any) {
	t.Helper()
	if args == nil {
		return
	}
	if _, ok := args["dry_run"]; ok {
		t.Fatalf("%s live probe must not pass dry_run", name)
	}
}

func liveOptionalName(prefix, runID, suffix string, maxLen int) string {
	name := strings.TrimSpace(prefix + " " + suffix + " " + runID)
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	if maxLen > 0 && len(name) > maxLen {
		name = strings.TrimSpace(name[:maxLen])
	}
	if len(name) < 2 {
		return "MCP " + suffix
	}
	return name
}

func defaultLiveBaseURL() string {
	if baseURL := strings.TrimSpace(os.Getenv("CLOCKIFY_BASE_URL")); baseURL != "" {
		return baseURL
	}
	return "https://api.clockify.me/api/v1"
}
