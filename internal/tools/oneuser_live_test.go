package tools

import (
	"context"
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
	server := mcp.NewServer("live", svc.FullAccessRegistry())
	initializeServer(t, server)
	runID := cleanDemoRunID(prefix)
	if runID == "" {
		runID = "live"
	}
	date := liveFutureWeekdayDate(1)

	status := callLiveToolDataOrRecovery(t, server, "clockify_status", nil)
	requireLiveID(t, status, "workspaceId")
	requireLiveID(t, status, "userId")
	clearLiveRunningTimer(t, svc)
	requiredCustomFields := liveRequiredTimeEntryCustomFields(t, svc, prefix)

	seed := callLiveToolOKOrRecovery(t, server, "clockify_demo_seed", addLiveTimeEntryCustomFields(map[string]any{"run_id": runID, "prefix": prefix, "date": date}, requiredCustomFields))
	for _, key := range []string{"clientId", "projectId", "taskId", "tagId", "entryId"} {
		requireLiveID(t, seed, key)
	}

	logged := callLiveToolOKOrRecovery(t, server, "clockify_log_work", addLiveTimeEntryCustomFields(map[string]any{
		"start":       date + " 10:30",
		"end":         date + " 11:00",
		"description": prefix + " Live logged work",
		"project_id":  seed.IDs["projectId"],
		"task_id":     seed.IDs["taskId"],
		"tag_ids":     []any{seed.IDs["tagId"]},
		"billable":    true,
	}, requiredCustomFields))
	requireLiveID(t, logged, "entryId")

	startAt := time.Now().UTC().Add(-4 * time.Minute).Format(time.RFC3339)
	started := callLiveToolOKOrRecovery(t, server, "clockify_start_work", addLiveTimeEntryCustomFields(map[string]any{
		"start":       startAt,
		"description": prefix + " Live started work",
		"project_id":  seed.IDs["projectId"],
		"task_id":     seed.IDs["taskId"],
		"tag_ids":     []any{seed.IDs["tagId"]},
	}, requiredCustomFields))
	requireLiveID(t, started, "entryId")

	switched := callLiveToolOKOrRecovery(t, server, "clockify_switch_work", addLiveTimeEntryCustomFields(map[string]any{
		"start":       time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339),
		"description": prefix + " Live switched work",
		"project_id":  seed.IDs["projectId"],
		"task_id":     seed.IDs["taskId"],
		"tag_ids":     []any{seed.IDs["tagId"]},
	}, requiredCustomFields))
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

	cleanup := callLiveToolOKOrRecovery(t, server, "clockify_demo_cleanup", map[string]any{"run_id": runID, "prefix": prefix, "start": "2000-01-01 00:00", "end": "2100-01-01T00:00:00Z"})
	requireLiveID(t, cleanup, "workspaceId")
}

// TestOneUserLiveTimerHappyPaths exercises the domain timer tools against the
// sacrificial workspace: it creates a prefixed project fixture, reads the idle
// running-timer state, starts and switches a timer, reads running/status,
// lists the project memberships, then stops and cleans up every created
// object. It is core-gated (CLOCKIFY_RUN_LIVE_E2E) because timer mutations
// touch only the caller's own time entries in the pinned workspace.
func TestOneUserLiveTimerHappyPaths(t *testing.T) {
	if os.Getenv("CLOCKIFY_RUN_LIVE_E2E") != "1" {
		t.Skip("set CLOCKIFY_RUN_LIVE_E2E=1 with CLOCKIFY_API_KEY, CLOCKIFY_WORKSPACE_ID, and CLOCKIFY_LIVE_PREFIX to run live Clockify timer tests")
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
	server := mcp.NewServer("live", svc.FullAccessRegistry())
	initializeServer(t, server)
	runID := cleanDemoRunID(prefix)
	if runID == "" {
		runID = "live"
	}

	clearLiveRunningTimer(t, svc)
	requiredCustomFields := liveRequiredTimeEntryCustomFields(t, svc, prefix)

	project := callLiveToolOKOrRecovery(t, server, "clockify_projects_create", map[string]any{
		"name":     liveOptionalName(prefix, runID, "timer fixture", 80),
		"billable": false,
	})
	projectID := requireLiveID(t, project, "projectId")
	t.Cleanup(func() {
		_, _ = svc.DeleteProject(context.Background(), map[string]any{"project_id": projectID})
	})

	// Idle read before any timer is started.
	idle := requireLiveOK(t, server, "clockify_entries_running", nil)
	if data, ok := idle.Data.(map[string]any); ok && data["running"] == true {
		t.Fatalf("expected no running timer after preflight clear: %+v", data)
	}

	memberships := requireLiveOK(t, server, "clockify_projects_memberships_list", map[string]any{"project_id": projectID})
	if memberships.Data == nil {
		t.Fatalf("clockify_projects_memberships_list returned no data: %+v", memberships)
	}

	started := callLiveToolOKOrRecovery(t, server, "clockify_entries_timer_start", addLiveTimeEntryCustomFields(map[string]any{
		"project_id":  projectID,
		"description": prefix + " live timer start probe",
	}, requiredCustomFields))
	startedEntry := requireLiveID(t, started, "entryId")
	t.Cleanup(func() {
		_, _ = svc.DeleteEntry(context.Background(), map[string]any{"entry_id": startedEntry})
	})

	status := requireLiveOK(t, server, "clockify_entries_timer_status", nil)
	if data, ok := status.Data.(map[string]any); !ok || data["running"] != true {
		t.Fatalf("clockify_entries_timer_status did not report a running timer: %+v", status.Data)
	}

	running := requireLiveOK(t, server, "clockify_entries_running", nil)
	if data, ok := running.Data.(map[string]any); !ok || data["running"] != true {
		t.Fatalf("clockify_entries_running did not report the started timer: %+v", running.Data)
	}

	switched := callLiveToolOKOrRecovery(t, server, "clockify_entries_timer_switch", addLiveTimeEntryCustomFields(map[string]any{
		"project":     projectID,
		"description": prefix + " live timer switch probe",
	}, requiredCustomFields))
	switchedEntry := requireLiveID(t, switched, "entryId")
	t.Cleanup(func() {
		_, _ = svc.DeleteEntry(context.Background(), map[string]any{"entry_id": switchedEntry})
	})

	stopped := callLiveToolOKOrRecovery(t, server, "clockify_entries_timer_stop", nil)
	requireLiveID(t, stopped, "entryId")
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
	server := mcp.NewServer("live", svc.FullAccessRegistry())
	initializeServer(t, server)
	runID := cleanDemoRunID(prefix)
	if runID == "" {
		runID = "live-paid"
	}
	requiredCustomFields := liveRequiredTimeEntryCustomFields(t, svc, prefix)
	seed := callLiveToolOKOrRecovery(t, server, "clockify_demo_seed", addLiveTimeEntryCustomFields(map[string]any{"run_id": runID, "prefix": prefix, "date": liveFutureWeekdayDate(1)}, requiredCustomFields))
	status := callLiveToolDataOrRecovery(t, server, "clockify_status", nil)
	userID := requireLiveID(t, status, "userId")
	defer func() {
		callLiveToolOKOrRecovery(t, server, "clockify_demo_cleanup", map[string]any{"run_id": runID, "prefix": prefix, "start": "2000-01-01 00:00", "end": "2100-01-01T00:00:00Z"})
	}()

	callLiveToolOKOrRecovery(t, server, "clockify_invoice_client_work", map[string]any{"client_id": seed.IDs["clientId"], "number": "MCP-LIVE-" + runID})
	callLiveToolOKOrRecovery(t, server, "clockify_record_expense", map[string]any{"amount": float64(10), "category": prefix, "project_id": seed.IDs["projectId"], "date": "2026-01-02T00:00:00Z"})
	callLiveToolOKOrRecovery(t, server, "clockify_request_time_off", map[string]any{"policy": prefix, "start": "2026-01-05", "end": "2026-01-06", "note": "live coverage probe"})
	callLiveToolOKOrRecovery(t, server, "clockify_schedule_work", map[string]any{"user_id": userID, "project_id": seed.IDs["projectId"], "start": "2026-01-05T09:00:00Z", "end": "2026-01-09T17:00:00Z", "hours_per_day": float64(1)})
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
	server := mcp.NewServer("live", svc.FullAccessRegistry())
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
	requiredCustomFields := liveRequiredTimeEntryCustomFields(t, svc, prefix)
	workPackage := callLiveToolOKOrRecovery(t, server, "clockify_create_work_package", map[string]any{
		"client":  liveOptionalName(prefix, runID, "client", 80),
		"project": liveOptionalName(prefix, runID, "project", 80),
		"task":    liveOptionalName(prefix, runID, "task", 80),
		"tag":     liveOptionalName(prefix, runID, "tag", 80),
	})
	var entry liveToolEnvelope
	if workPackage.OK {
		entry = callLiveToolOKOrRecovery(t, server, "clockify_log_work", addLiveTimeEntryCustomFields(map[string]any{
			"start":       "2026-01-06T09:00:00Z",
			"end":         "2026-01-06T10:00:00Z",
			"description": liveOptionalName(prefix, runID, "invoiceable work", 120),
			"project_id":  requireLiveID(t, workPackage, "projectId"),
			"task_id":     workPackage.IDs["taskId"],
		}, requiredCustomFields))
	}

	for _, probe := range []struct {
		name string
		args map[string]any
	}{
		{"clockify_invoices_list", map[string]any{"page_size": float64(1)}},
		{"clockify_invoices_info", map[string]any{"page_size": float64(1)}},
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
	callLiveToolDataOrRecovery(t, server, "clockify_scheduling_publish", map[string]any{
		"start":   "2026-01-12",
		"end":     "2026-01-16",
		"dry_run": true,
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

	// Full API coverage: the audit-log report and the experimental entity
	// changes feed. Both are read-only and both return a bare JSON array; a
	// live probe confirmed real success envelopes on the sacrificial
	// workspace, so a successful read is required here.
	auditStart := time.Now().AddDate(0, 0, -7).UTC().Format("2006-01-02")
	auditEnd := time.Now().UTC().Format("2006-01-02")
	requireLiveOK(t, server, "clockify_audit_logs_search", map[string]any{
		"actions":    []any{"CREATE_PROJECT", "CREATE_CLIENT", "UPDATE_PROJECT", "CREATE_TIME_PERSONAL_MANUAL"},
		"author_ids": []any{userID},
		"start":      auditStart,
		"end":        auditEnd,
	})
	for _, changeType := range []string{"created", "updated", "deleted"} {
		requireLiveOK(t, server, "clockify_entity_changes_list", map[string]any{
			"change_type":  changeType,
			"entity_types": []any{"TIME_ENTRY", "PROJECTS", "CLIENTS"},
			"start":        auditStart,
			"end":          auditEnd,
		})
	}
}

func TestOneUserLiveRemainingCoverageProbes(t *testing.T) {
	apiKey, workspaceID, prefix := requireRemainingCoverageLiveEnv(t)

	client := clockify.NewClient(apiKey, defaultLiveBaseURL(), 30*time.Second, 2)
	defer client.Close()
	svc := New(client, workspaceID)
	svc.DefaultTimezone = time.UTC
	registry := svc.FullAccessRegistry()
	server := mcp.NewServer("live-remaining", registry)
	initializeServer(t, server)
	descriptors := remainingLiveDescriptorMap(registry)
	args := func(name string, overrides ...any) map[string]any {
		return remainingLiveArgs(t, descriptors, name, overrides...)
	}

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
	requiredCustomFields := liveRequiredTimeEntryCustomFields(t, svc, prefix)

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

	entryResp := requireLiveOK(t, server, "clockify_entries_create", addLiveTimeEntryCustomFields(map[string]any{
		"start":       "2026-02-03T09:00:00Z",
		"end":         "2026-02-03T10:00:00Z",
		"description": unique("entry"),
		"project_id":  projectID,
		"task_id":     taskID,
		"tag_ids":     []any{tagID},
	}, requiredCustomFields))
	entryID := requireLiveID(t, entryResp, "entryId")
	requireLiveOK(t, server, "clockify_entries_get", map[string]any{"entry_id": entryID})
	requireLiveOK(t, server, "clockify_entries_update", map[string]any{"entry_id": entryID, "description": unique("entry-updated")})

	callLiveToolDataOrRecovery(t, server, "clockify_entries_running", nil)
	callLiveToolDataOrRecovery(t, server, "clockify_entries_timer_status", nil)
	callLiveToolDataOrRecovery(t, server, "clockify_entries_timer_stop", nil)
	clearLiveRunningTimer(t, svc)
	timerStart := requireLiveOK(t, server, "clockify_entries_timer_start", addLiveTimeEntryCustomFields(map[string]any{
		"project_id":  projectID,
		"description": unique("timer-start"),
	}, requiredCustomFields))
	requireLiveID(t, timerStart, "entryId")
	timerSwitch := callLiveToolOKOrRecovery(t, server, "clockify_entries_timer_switch", addLiveTimeEntryCustomFields(map[string]any{
		"project":     unique("timer-project"),
		"description": unique("timer-switch"),
	}, requiredCustomFields))
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
	runRemainingLiveProbeTable(t, server, []remainingLiveProbe{
		{"clockify_invoices_get", args("clockify_invoices_get", "invoice_id", invoiceID)},
		{"clockify_invoices_items_list", args("clockify_invoices_items_list", "invoice_id", invoiceID)},
		{"clockify_invoices_update", args("clockify_invoices_update", "invoice_id", invoiceID, "note", "live coverage probe")},
		{"clockify_invoices_mark_paid", args("clockify_invoices_mark_paid", "invoice_id", invoiceID)},
		{"clockify_invoices_items_add", args("clockify_invoices_items_add", "invoice_id", invoiceID, "description", "Live coverage item", "quantity", 1, "unit_price", 1)},
		{"clockify_invoices_items_update", args("clockify_invoices_items_update", "invoice_id", invoiceID, "item_index", "0", "description", "Live coverage item", "quantity", 1, "unit_price", 1)},
		{"clockify_invoices_items_delete", args("clockify_invoices_items_delete", "invoice_id", invoiceID, "item_index", "0")},
		{"clockify_invoices_export", args("clockify_invoices_export", "invoice_id", invoiceID, "format", "PDF")},
		{"clockify_invoices_import_time", args("clockify_invoices_import_time", "invoice_id", invoiceID, "time_entry_ids", []any{entryID})},
		{"clockify_invoices_import_expenses", args("clockify_invoices_import_expenses", "invoice_id", invoiceID, "expense_ids", []any{bogusID}, "include_expenses", true)},
		{"clockify_invoices_payments_list", args("clockify_invoices_payments_list", "invoice_id", invoiceID)},
		{"clockify_invoices_payments_create", args("clockify_invoices_payments_create", "invoice_id", invoiceID, "amount", 1, "date", "2026-02-03T00:00:00Z", "note", "live coverage")},
		{"clockify_invoices_payments_delete", args("clockify_invoices_payments_delete", "invoice_id", invoiceID, "payment_id", bogusID)},
		{"clockify_invoices_delete", args("clockify_invoices_delete", "invoice_id", invoiceID)},
	})

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
	runRemainingLiveProbeTable(t, server, []remainingLiveProbe{
		{"clockify_time_off_policies_get", args("clockify_time_off_policies_get", "policy_id", policyID)},
		{"clockify_time_off_policies_update", args("clockify_time_off_policies_update", "policy_id", policyID, "name", unique("policy-updated"))},
		{"clockify_time_off_balances", args("clockify_time_off_balances", "policy_id", policyID, "user_id", userID)},
		{"clockify_time_off_balances_update", args("clockify_time_off_balances_update",
			"policy_id", policyID,
			"user_ids", []any{userID},
			"value", 0,
			"note", "live coverage probe: sets the owner balance to 0 on the prefixed throwaway policy created by this run",
		)},
		{"clockify_time_off_requests_get", args("clockify_time_off_requests_get", "policy_id", policyID, "request_id", bogusID)},
		{"clockify_time_off_requests_update", args("clockify_time_off_requests_update", "policy_id", policyID, "request_id", bogusID, "note", "live coverage")},
		{"clockify_time_off_requests_delete", args("clockify_time_off_requests_delete", "policy_id", policyID, "request_id", bogusID)},
		{"clockify_time_off_approve", args("clockify_time_off_approve", "policy_id", policyID, "request_id", bogusID, "note", "live coverage")},
		{"clockify_time_off_deny", args("clockify_time_off_deny", "policy_id", policyID, "request_id", bogusID, "note", "live coverage")},
		{"clockify_time_off_archive", args("clockify_time_off_archive", "policy_id", policyID, "archived", true)},
	})

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
	// Webhook names are capped at 30 chars by the Clockify schema, so
	// this probe uses the 30-bounded name helper rather than unique(),
	// whose 80-char budget overflows the limit.
	webhook := callLiveToolOKOrRecovery(t, server, "clockify_webhooks_create", map[string]any{
		"name":          liveOptionalName("mcp", runID, "webhook", 30),
		"url":           "https://example.com/clockify",
		"webhook_event": "NEW_TIME_ENTRY",
	})
	if webhook.OK && webhook.IDs["webhookId"] != "" {
		webhookID = webhook.IDs["webhookId"]
	}
	runRemainingLiveProbeTable(t, server, []remainingLiveProbe{
		{"clockify_webhooks_get", args("clockify_webhooks_get", "webhook_id", webhookID)},
		{"clockify_webhooks_update", args("clockify_webhooks_update", "webhook_id", webhookID, "name", liveOptionalName("mcp", runID, "wh-upd", 30))},
		{"clockify_webhooks_test", args("clockify_webhooks_test", "webhook_id", webhookID)},
		{"clockify_webhooks_delete", args("clockify_webhooks_delete", "webhook_id", webhookID)},
	})

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
		"assigned_to": userID,
		"start":       holidayDate,
		"end":         holidayDate,
	})
	callLiveToolDataOrRecovery(t, server, "clockify_holidays_delete", map[string]any{"holiday_id": holidayID})

	runRemainingLiveProbeTable(t, server, []remainingLiveProbe{
		{"clockify_users_deactivate", args("clockify_users_deactivate", "user_id", bogusID)},
		{"clockify_users_role", args("clockify_users_role", "user_id", bogusID, "role", "WORKSPACE_ADMIN")},
		{"clockify_workspace_settings", nil},
		{"clockify_projects_memberships_list", args("clockify_projects_memberships_list", "project_id", projectID)},
		{"clockify_reports_attendance", reportProbeArgs()},
		{"clockify_reports_money", reportProbeArgs()},
		{"clockify_reports_export", args("clockify_reports_export", "start", "2026-02-03T00:00:00.000", "end", "2026-02-04T00:00:00.000", "export_type", "JSON")},
		{"clockify_approvals_get", args("clockify_approvals_get", "approval_id", bogusID)},
		{"clockify_approvals_submit", args("clockify_approvals_submit", "period", "WEEKLY", "period_start", "2026-02-02T00:00:00.000Z")},
		{"clockify_approvals_approve", args("clockify_approvals_approve", "approval_id", bogusID, "note", "live coverage")},
		{"clockify_approvals_reject", args("clockify_approvals_reject", "approval_id", bogusID, "reason", "live coverage")},
		{"clockify_approvals_withdraw", args("clockify_approvals_withdraw", "approval_id", bogusID, "note", "live coverage")},
		{"clockify_approvals_resubmit", args("clockify_approvals_resubmit", "approval_id", bogusID, "entry_ids", []any{entryID}, "note", "live coverage")},
	})

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

type remainingLiveProbe struct {
	name string
	args map[string]any
}

func runRemainingLiveProbeTable(t *testing.T, server *mcp.Server, probes []remainingLiveProbe) {
	t.Helper()
	for _, probe := range probes {
		callLiveToolDataOrRecovery(t, server, probe.name, probe.args)
	}
}

func remainingLiveDescriptorMap(registry []mcp.ToolDescriptor) map[string]mcp.ToolDescriptor {
	descriptors := make(map[string]mcp.ToolDescriptor, len(registry))
	for _, descriptor := range registry {
		descriptors[descriptor.Tool.Name] = descriptor
	}
	return descriptors
}

func remainingLiveArgs(t *testing.T, descriptors map[string]mcp.ToolDescriptor, name string, overrides ...any) map[string]any {
	t.Helper()
	if len(overrides)%2 != 0 {
		t.Fatalf("%s overrides must be key/value pairs", name)
	}
	descriptor, ok := descriptors[name]
	if !ok {
		t.Fatalf("missing descriptor for live probe %s", name)
	}
	props, _ := descriptor.Tool.InputSchema["properties"].(map[string]any)
	out := map[string]any{}
	for _, field := range inputSchemaRequiredFields(descriptor.Tool.InputSchema) {
		prop, _ := props[field].(map[string]any)
		out[field] = oneUserCoverageValue(field, prop)
	}
	for i := 0; i < len(overrides); i += 2 {
		key, ok := overrides[i].(string)
		if !ok || key == "" {
			t.Fatalf("%s override key at index %d = %#v, want non-empty string", name, i, overrides[i])
		}
		out[key] = overrides[i+1]
	}
	return out
}

func TestRemainingLiveArgsDerivesRequiredFieldsFromDescriptor(t *testing.T) {
	descriptors := remainingLiveDescriptorMap((&Service{}).FullAccessRegistry())
	got := remainingLiveArgs(t, descriptors, "clockify_invoices_items_add",
		"invoice_id", "invoice-live",
		"description", "Live coverage item",
		"quantity", 1,
		"unit_price", 1,
	)
	for _, field := range []string{"invoice_id", "item_type", "description", "quantity", "unit_price"} {
		if _, ok := got[field]; !ok {
			t.Fatalf("descriptor-derived args missing required field %q: %#v", field, got)
		}
	}
	if got["invoice_id"] != "invoice-live" {
		t.Fatalf("invoice_id override = %#v, want invoice-live", got["invoice_id"])
	}
	if got["description"] != "Live coverage item" {
		t.Fatalf("description override = %#v", got["description"])
	}
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

func clearLiveRunningTimer(t *testing.T, svc *Service) {
	t.Helper()
	out, err := svc.EntriesRunning(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("preflight running timer check: %v", err)
	}
	result, ok := out.(ToolResult)
	if !ok {
		t.Fatalf("preflight running timer check returned %T", out)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["running"] != true {
		return
	}
	entryID := liveRunningEntryID(data["entry"])
	if entryID == "" {
		t.Fatalf("preflight running timer check could not extract entry id: %#v", data["entry"])
	}
	if _, err := svc.DeleteEntry(context.Background(), map[string]any{"entry_id": entryID}); err != nil {
		t.Fatalf("preflight delete stale running timer: %v", err)
	}
}

func liveRequiredTimeEntryCustomFields(t *testing.T, svc *Service, prefix string) []any {
	t.Helper()
	path := "/workspaces/" + svc.WorkspaceID + "/custom-fields"
	var fields []map[string]any
	if err := svc.Client.Get(context.Background(), path, map[string]string{"page": "1", "page-size": "200"}, &fields); err != nil {
		t.Fatalf("list custom fields: %v", err)
	}
	out := make([]any, 0)
	for _, field := range fields {
		if !boolFromAny(field["required"]) || !liveCustomFieldAppliesToTimeEntry(field) {
			continue
		}
		status := strings.ToUpper(strings.TrimSpace(firstReportString(field, "status")))
		if status != "" && status != "VISIBLE" {
			continue
		}
		fieldID := firstReportString(field, "id", "_id", "customFieldId", "custom_field_id")
		if strings.TrimSpace(fieldID) == "" {
			t.Fatalf("required time-entry custom field is missing an id")
		}
		value, ok := liveCustomFieldTestValue(field, prefix)
		if !ok {
			t.Fatalf("required time-entry custom field %q has unsupported type/allowed values", firstReportString(field, "name"))
		}
		out = append(out, map[string]any{"field_id": fieldID, "value": value})
	}
	return out
}

func addLiveTimeEntryCustomFields(args map[string]any, customFields []any) map[string]any {
	if len(customFields) > 0 {
		args["custom_fields"] = customFields
	}
	return args
}

func liveCustomFieldAppliesToTimeEntry(field map[string]any) bool {
	scopes := customFieldEntityScopes(field)
	if len(scopes) == 0 {
		return true
	}
	for _, scope := range scopes {
		switch strings.ToUpper(strings.TrimSpace(scope)) {
		case "TIME_ENTRY", "TIMEENTRY", "TIME_ENTRIES", "TIMEENTRY_CUSTOM_FIELD_VALUE":
			return true
		}
	}
	return false
}

func liveCustomFieldTestValue(field map[string]any, prefix string) (any, bool) {
	switch strings.ToUpper(strings.TrimSpace(firstReportString(field, "type", "customFieldType", "fieldType"))) {
	case "TXT":
		return prefix + " custom field", true
	case "LINK":
		return "https://example.com/" + strings.ToLower(strings.ReplaceAll(prefix, " ", "-")), true
	case "NUMBER":
		return float64(1), true
	case "CHECKBOX":
		return true, true
	case "DROPDOWN_SINGLE":
		return liveFirstAllowedCustomFieldValue(field)
	case "DROPDOWN_MULTIPLE":
		value, ok := liveFirstAllowedCustomFieldValue(field)
		if !ok {
			return nil, false
		}
		return []any{value}, true
	default:
		return nil, false
	}
}

func liveFirstAllowedCustomFieldValue(field map[string]any) (string, bool) {
	items, _ := field["allowedValues"].([]any)
	for _, item := range items {
		switch v := item.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return v, true
			}
		case map[string]any:
			if value := firstReportString(v, "id", "_id", "value", "name"); value != "" {
				return value, true
			}
		}
	}
	return "", false
}

func liveFutureWeekdayDate(monthsAhead int) string {
	day := time.Now().UTC().AddDate(0, monthsAhead, 0)
	for day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		day = day.AddDate(0, 0, 1)
	}
	return day.Format("2006-01-02")
}

func liveRunningEntryID(entry any) string {
	switch value := entry.(type) {
	case clockify.TimeEntry:
		return value.ID
	case map[string]any:
		return stringFromAny(value["id"])
	}
	return ""
}

// reportProbeArgs are the minimal args for the report route tools. Those
// tools carry a deliberately small typed schema (start/end/date-range/
// export_type); pagination is a report-body field, so it travels through
// the `body` raw escape hatch rather than as flat top-level args.
func reportProbeArgs() map[string]any {
	return map[string]any{
		"start": "2026-02-03T00:00:00.000",
		"end":   "2026-02-04T00:00:00.000",
		"body": map[string]any{
			"page":     1,
			"pageSize": 10,
		},
	}
}

type liveToolEnvelope struct {
	OK       bool              `json:"ok"`
	IDs      map[string]string `json:"ids,omitempty"`
	Data     any               `json:"data,omitempty"`
	Error    ErrorInfo         `json:"error"`
	Recovery RecoveryHint      `json:"recovery"`
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
	if _, ok := args["dry_run"]; ok && name != "clockify_scheduling_publish" {
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
