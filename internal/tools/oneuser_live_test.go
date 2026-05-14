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
	ctx := context.Background()
	runID := cleanDemoRunID(prefix)
	if runID == "" {
		runID = "live"
	}
	date := "2026-01-02"

	statusOut, err := svc.ClockifyStatus(ctx, nil)
	status := mustToolResult(t, statusOut, err)
	requireID(t, status, "workspaceId")
	requireID(t, status, "userId")

	seedOut, err := svc.ClockifyDemoSeed(ctx, map[string]any{"run_id": runID, "prefix": prefix, "date": date})
	seed := mustToolResult(t, seedOut, err)
	for _, key := range []string{"clientId", "projectId", "taskId", "tagId", "entryId"} {
		requireID(t, seed, key)
	}

	logOut, err := svc.ClockifyLogWork(ctx, map[string]any{
		"start":       date + " 10:30",
		"end":         date + " 11:00",
		"description": prefix + " Live logged work",
		"project_id":  seed.IDs["projectId"],
		"task_id":     seed.IDs["taskId"],
		"tag_ids":     []any{seed.IDs["tagId"]},
		"billable":    true,
	})
	logged := mustToolResult(t, logOut, err)
	requireID(t, logged, "entryId")

	startAt := time.Now().UTC().Add(-4 * time.Minute).Format(time.RFC3339)
	startOut, err := svc.ClockifyStartWork(ctx, map[string]any{
		"start":       startAt,
		"description": prefix + " Live started work",
		"project_id":  seed.IDs["projectId"],
		"task_id":     seed.IDs["taskId"],
		"tag_ids":     []any{seed.IDs["tagId"]},
	})
	started := mustToolResult(t, startOut, err)
	requireID(t, started, "entryId")

	switchOut, err := svc.ClockifySwitchWork(ctx, map[string]any{
		"start":       time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339),
		"description": prefix + " Live switched work",
		"project_id":  seed.IDs["projectId"],
		"task_id":     seed.IDs["taskId"],
		"tag_ids":     []any{seed.IDs["tagId"]},
	})
	switched := mustToolResult(t, switchOut, err)
	requireID(t, switched, "entryId")

	stopOut, err := svc.ClockifyStopWork(ctx, nil)
	stopped := mustToolResult(t, stopOut, err)
	requireID(t, stopped, "entryId")

	fixOut, err := svc.ClockifyFixEntry(ctx, map[string]any{
		"entry_id":    logged.IDs["entryId"],
		"description": prefix + " Live fixed logged work",
	})
	fixed := mustToolResult(t, fixOut, err)
	requireID(t, fixed, "entryId")

	dayOut, err := svc.ClockifyReviewDay(ctx, map[string]any{"date": date, "include_entries": true})
	day := mustToolResult(t, dayOut, err)
	if day.Data == nil {
		t.Fatalf("live day review missing data: %+v", day)
	}

	weekOut, err := svc.ClockifyReviewWeek(ctx, map[string]any{"week_start": "2025-12-29", "include_entries": true})
	week := mustToolResult(t, weekOut, err)
	if week.Data == nil {
		t.Fatalf("live week review missing data: %+v", week)
	}

	cleanupOut, err := svc.ClockifyDemoCleanup(ctx, map[string]any{"run_id": runID, "prefix": prefix, "start": "2026-01-01 00:00", "end": time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)})
	cleanup := mustToolResult(t, cleanupOut, err)
	requireID(t, cleanup, "workspaceId")
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
	ctx := context.Background()
	runID := cleanDemoRunID(prefix)
	if runID == "" {
		runID = "live-paid"
	}
	seedOut, err := svc.ClockifyDemoSeed(ctx, map[string]any{"run_id": runID, "prefix": prefix, "date": "2026-01-02"})
	seed := mustToolResult(t, seedOut, err)
	user, err := svc.getCurrentUser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = svc.ClockifyDemoCleanup(context.Background(), map[string]any{"run_id": runID, "prefix": prefix, "start": "2026-01-01 00:00", "end": time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)})
	}()

	callLiveToolOKOrRecovery(t, server, "clockify_invoice_client_work", map[string]any{"client_id": seed.IDs["clientId"], "number": "MCP-LIVE-" + runID})
	callLiveToolOKOrRecovery(t, server, "clockify_record_expense", map[string]any{"amount": float64(10), "category": prefix, "project_id": seed.IDs["projectId"], "date": "2026-01-02T00:00:00Z"})
	callLiveToolOKOrRecovery(t, server, "clockify_request_time_off", map[string]any{"policy": prefix, "start": "2026-01-05", "end": "2026-01-06"})
	callLiveToolOKOrRecovery(t, server, "clockify_schedule_work", map[string]any{"user_id": user.ID, "project_id": seed.IDs["projectId"], "start": "2026-01-05T09:00:00Z", "end": "2026-01-09T17:00:00Z"})
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
