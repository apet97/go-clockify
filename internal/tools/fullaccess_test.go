package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/testclockify"
)

var workflowTools = []string{
	"clockify_status",
	"clockify_tools_guide",
	"clockify_create_work_package",
	"clockify_log_work",
	"clockify_start_work",
	"clockify_stop_work",
	"clockify_switch_work",
	"clockify_review_day",
	"clockify_review_week",
	"clockify_fix_entry",
	"clockify_invoice_client_work",
	"clockify_record_expense",
	"clockify_request_time_off",
	"clockify_schedule_work",
	"clockify_setup_webhook",
	"clockify_demo_seed",
	"clockify_demo_cleanup",
}

func TestFullAccessRegistryMigratesDomainsAtStartup(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	reg := svc.FullAccessRegistry()
	names := map[string]bool{}
	for _, descriptor := range reg {
		name := descriptor.Tool.Name
		if names[name] {
			t.Fatalf("duplicate full-access tool %q", name)
		}
		names[name] = true
		if descriptor.Tool.InputSchema == nil {
			t.Fatalf("%s missing input schema", name)
		}
		if descriptor.Tool.OutputSchema == nil {
			t.Fatalf("%s missing output schema", name)
		}
	}

	for _, forbidden := range []string{
		blockedTerm("clockify_", "activate_group"),
		blockedTerm("clockify_", "activate_tool"),
		blockedTerm("clockify_", "deactivate_group"),
		blockedTerm("clockify_", "search_tools"),
		blockedTerm("clockify_", "list_tools"),
		blockedTerm("clockify_", "policy_info"),
	} {
		if names[forbidden] {
			t.Fatalf("forbidden old product tool exposed: %s", forbidden)
		}
	}

	for _, want := range []string{
		"clockify_status",
		"clockify_tools_guide",
		"clockify_create_work_package",
		"clockify_log_work",
		"clockify_start_work",
		"clockify_stop_work",
		"clockify_switch_work",
		"clockify_review_day",
		"clockify_review_week",
		"clockify_fix_entry",
		"clockify_invoice_client_work",
		"clockify_record_expense",
		"clockify_request_time_off",
		"clockify_schedule_work",
		"clockify_setup_webhook",
		"clockify_demo_seed",
		"clockify_demo_cleanup",
		"clockify_reports_detailed",
		"clockify_reports_summary",
		"clockify_reports_weekly",
		"clockify_reports_attendance",
		"clockify_reports_money",
		"clockify_reports_expense",
		"clockify_reports_export",
		"clockify_invoices_list",
		"clockify_invoices_create",
		"clockify_invoices_import_time",
		"clockify_invoices_import_expenses",
		"clockify_invoices_payments_list",
		"clockify_expenses_create",
		"clockify_expenses_categories_list",
		"clockify_custom_fields_set_value",
		"clockify_time_off_requests_create",
		"clockify_time_off_balances",
		"clockify_scheduling_assignments_list",
		"clockify_scheduling_project_totals",
		"clockify_scheduling_user_totals",
		"clockify_scheduling_capacity",
		"clockify_approvals_submit",
		"clockify_approvals_resubmit",
		"clockify_webhooks_events",
		"clockify_groups_add_user",
		"clockify_holidays_list_for_user_period",
		"clockify_users_invite",
		"clockify_workspace_settings",
		"clockify_api_get",
		"clockify_api_request",
	} {
		if !names[want] {
			t.Fatalf("missing migrated full-access tool %s", want)
		}
	}
}

func TestFullAccessToolsListWorkflowToolsFirstAndAnnotated(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	reg := svc.FullAccessRegistry()
	for i, want := range workflowTools {
		if i >= len(reg) {
			t.Fatalf("registry too short, missing workflow %s", want)
		}
		got := reg[i].Tool.Name
		if got != want {
			t.Fatalf("registry[%d]=%s, want workflow %s", i, got, want)
		}
		assertWorkflowAnnotations(t, reg[i].Tool)
	}

	server := mcp.NewServer("test", reg, nil, nil)
	if _, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)); err != nil {
		t.Fatal(err)
	}
	raw, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Result struct {
			Tools []mcp.Tool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	for i, want := range workflowTools {
		if i >= len(resp.Result.Tools) {
			t.Fatalf("tools/list too short, missing workflow %s", want)
		}
		if got := resp.Result.Tools[i].Name; got != want {
			t.Fatalf("tools/list[%d]=%s, want workflow %s", i, got, want)
		}
		assertWorkflowAnnotations(t, resp.Result.Tools[i])
	}
}

func TestFullAccessRegistryIsCachedAndDefensivelyCloned(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	first := svc.FullAccessRegistry()
	if len(first) != 151 {
		t.Fatalf("registry size=%d, want 151", len(first))
	}
	if len(svc.registry) != 151 {
		t.Fatalf("cached registry size=%d, want 151", len(svc.registry))
	}
	cachedFirstName := svc.registry[0].Tool.Name
	first[0].Tool.Name = "mutated-by-test"

	second := svc.FullAccessRegistry()
	if second[0].Tool.Name != cachedFirstName {
		t.Fatalf("cached registry was not defensively cloned: got %s want %s", second[0].Tool.Name, cachedFirstName)
	}
	if len(svc.registry) != 151 || svc.registry[0].Tool.Name != cachedFirstName {
		t.Fatalf("cached registry changed across calls: len=%d first=%s", len(svc.registry), svc.registry[0].Tool.Name)
	}
}

func BenchmarkFullAccessRegistry(b *testing.B) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	b.ReportAllocs()
	for b.Loop() {
		if got := len(svc.FullAccessRegistry()); got != 151 {
			b.Fatalf("registry size=%d, want 151", got)
		}
	}
}

func BenchmarkOneUserToolsResourceData(b *testing.B) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	b.ReportAllocs()
	for b.Loop() {
		data := svc.toolsResourceData()
		if got, _ := data["count"].(int); got != 151 {
			b.Fatalf("tools resource count=%d, want 151", got)
		}
	}
}

func TestOneUserToolsResourceDataIsCachedAndDefensivelyCloned(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	first := svc.toolsResourceData()
	if svc.toolsResourceCache == nil {
		t.Fatal("tools resource cache was not populated")
	}
	first["count"] = 0
	tools, ok := first["tools"].([]mcp.Tool)
	if !ok || len(tools) == 0 {
		t.Fatalf("tools resource missing tool slice: %+v", first["tools"])
	}
	tools[0].Name = "mutated-by-test"

	second := svc.toolsResourceData()
	if got, _ := second["count"].(int); got != 151 {
		t.Fatalf("cached tools resource count=%d, want 151", got)
	}
	secondTools, ok := second["tools"].([]mcp.Tool)
	if !ok || len(secondTools) == 0 {
		t.Fatalf("cached tools resource missing tool slice: %+v", second["tools"])
	}
	if secondTools[0].Name != "clockify_status" {
		t.Fatalf("cached tools resource leaked caller mutation: first=%s", secondTools[0].Name)
	}
}

func BenchmarkOneUserToolsListRealRegistry(b *testing.B) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	server := mcp.NewServer("bench", svc.FullAccessRegistry(), nil, nil)
	server.MarkInitialized(mcp.SupportedProtocolVersions[0], "bench", "0")
	msg := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		if _, err := server.DispatchMessage(ctx, msg); err != nil {
			b.Fatalf("tools/list: %v", err)
		}
	}
}

func assertWorkflowAnnotations(t *testing.T, tool mcp.Tool) {
	t.Helper()
	ann := tool.Annotations
	if ann["category"] != "workflow" {
		t.Fatalf("%s category=%v, want workflow", tool.Name, ann["category"])
	}
	if ann["preferred"] != true {
		t.Fatalf("%s preferred=%v, want true", tool.Name, ann["preferred"])
	}
	for _, key := range []string{"priority", "bestFor", "preferOver", "domain", "entity"} {
		if _, ok := ann[key]; !ok {
			t.Fatalf("%s missing annotation %s: %+v", tool.Name, key, ann)
		}
	}
}

func TestToolsGuideExplainsWorkflowAndDomainTools(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	out, err := svc.ClockifyToolsGuide(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result := out.(ToolResult)
	if !result.OK {
		t.Fatalf("guide not ok: %+v", result)
	}
	data := result.Data.(map[string]any)
	for _, key := range []string{"workflows", "commonTasks", "domainTools", "rawFallback"} {
		if data[key] == nil {
			t.Fatalf("guide missing %s: %+v", key, data)
		}
	}
	text := mustJSONText(t, data)
	for _, want := range []string{"clockify_log_work", "clockify_review_week", "clockify_invoice_client_work", "clockify_api_request"} {
		if !strings.Contains(text, want) {
			t.Fatalf("guide missing %s: %s", want, text)
		}
	}
}

func TestWorkflowPackageLogReviewAndRepeatableDemoCleanup(t *testing.T) {
	fake := testclockify.NewServer("65b382b606de527a7ee2b60e")
	defer fake.Close()
	svc := New(clockify.NewClient("test-key", fake.URL, time.Second, 0), fake.WorkspaceID)
	svc.DefaultTimezone = time.UTC

	pkgOut, err := svc.ClockifyCreateWorkPackage(context.Background(), map[string]any{
		"client":  "Workflow Client",
		"project": "Workflow Project",
		"task":    "Workflow Task",
		"tag":     "Workflow Tag",
	})
	pkg := mustToolResult(t, pkgOut, err)
	for _, key := range []string{"clientId", "projectId", "taskId", "tagId"} {
		requireID(t, pkg, key)
	}
	if got := len(pkg.Changed.Created); got != 4 {
		t.Fatalf("work package created %d objects, want 4: %+v", got, pkg.Changed)
	}

	pkgOut, err = svc.ClockifyCreateWorkPackage(context.Background(), map[string]any{
		"client":  "Workflow Client",
		"project": "Workflow Project",
		"task":    "Workflow Task",
		"tag":     "Workflow Tag",
	})
	pkgAgain := mustToolResult(t, pkgOut, err)
	if got := len(pkgAgain.Changed.Reused); got != 4 {
		t.Fatalf("work package reused %d objects, want 4: %+v", got, pkgAgain.Changed)
	}

	logOut, err := svc.ClockifyLogWork(context.Background(), map[string]any{
		"start":       "2026-01-02 09:00",
		"end":         "2026-01-02 10:30",
		"description": "Workflow logged work",
		"project":     "Workflow Project",
		"task":        "Workflow Task",
		"tag":         "Workflow Tag",
		"billable":    true,
	})
	logged := mustToolResult(t, logOut, err)
	requireID(t, logged, "entryId")
	requireID(t, logged, "projectId")
	requireID(t, logged, "taskId")
	requireID(t, logged, "tagId")
	if len(logged.Next) == 0 {
		t.Fatalf("log workflow missing next actions: %+v", logged)
	}

	fixOut, err := svc.ClockifyFixEntry(context.Background(), map[string]any{
		"entry_id":    logged.IDs["entryId"],
		"description": "Workflow logged work, corrected",
	})
	fixed := mustToolResult(t, fixOut, err)
	requireID(t, fixed, "entryId")
	if len(fixed.Changed.Updated) == 0 {
		t.Fatalf("fix workflow did not report update: %+v", fixed.Changed)
	}

	reviewDayOut, err := svc.ClockifyReviewDay(context.Background(), map[string]any{
		"date":            "2026-01-02",
		"include_entries": true,
	})
	reviewDay := mustToolResult(t, reviewDayOut, err)
	if len(reviewDay.Next) == 0 {
		t.Fatalf("day review missing next actions: %+v", reviewDay)
	}

	startOut, err := svc.ClockifyStartWork(context.Background(), map[string]any{
		"description": "Workflow running work",
		"project_id":  pkg.IDs["projectId"],
		"task_id":     pkg.IDs["taskId"],
		"tag_ids":     []any{pkg.IDs["tagId"]},
	})
	started := mustToolResult(t, startOut, err)
	requireID(t, started, "entryId")
	switchOut, err := svc.ClockifySwitchWork(context.Background(), map[string]any{
		"description": "Workflow switched work",
		"project_id":  pkg.IDs["projectId"],
		"task_id":     pkg.IDs["taskId"],
	})
	switched := mustToolResult(t, switchOut, err)
	requireID(t, switched, "entryId")
	if len(switched.Next) == 0 {
		t.Fatalf("switch workflow missing next actions: %+v", switched)
	}
	stopOut, err := svc.ClockifyStopWork(context.Background(), nil)
	stopped := mustToolResult(t, stopOut, err)
	requireID(t, stopped, "entryId")

	reviewOut, err := svc.ClockifyReviewWeek(context.Background(), map[string]any{
		"week_start":      "2026-01-02",
		"include_entries": true,
	})
	review := mustToolResult(t, reviewOut, err)
	data, ok := review.Data.(TimesheetReviewData)
	if !ok {
		t.Fatalf("review data type = %T, want TimesheetReviewData", review.Data)
	}
	if data.Totals.Entries < 2 || data.Totals.TotalSeconds <= 0 {
		t.Fatalf("review totals not useful: %+v", data.Totals)
	}
	if len(review.Next) == 0 {
		t.Fatalf("review missing next actions: %+v", review)
	}

	seedOut, err := svc.ClockifyDemoSeed(context.Background(), map[string]any{"run_id": "workflow"})
	seed := mustToolResult(t, seedOut, err)
	for _, key := range []string{"clientId", "projectId", "taskId", "tagId", "entryId"} {
		requireID(t, seed, key)
	}
	cleanupOut, err := svc.ClockifyDemoCleanup(context.Background(), map[string]any{"run_id": "workflow"})
	cleanup := mustToolResult(t, cleanupOut, err)
	if len(cleanup.Changed.Deleted) == 0 {
		t.Fatalf("first cleanup deleted nothing: %+v", cleanup)
	}
	cleanupOut, err = svc.ClockifyDemoCleanup(context.Background(), map[string]any{"run_id": "workflow"})
	cleanupAgain := mustToolResult(t, cleanupOut, err)
	if len(cleanupAgain.Changed.Deleted) != 0 {
		t.Fatalf("second cleanup should be repeatable no-op, deleted: %+v", cleanupAgain.Changed.Deleted)
	}
}

func TestInvoiceClientWorkFeatureUnavailableEnvelope(t *testing.T) {
	const workspaceID = "65b382b606de527a7ee2b60e"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && r.URL.Path == "/workspaces/"+workspaceID+"/invoices" {
			http.Error(w, `{"message":"feature is not supported on the current plan"}`, http.StatusForbidden)
			return
		}
		http.NotFound(w, r)
	}))
	defer upstream.Close()

	svc := New(clockify.NewClient("test-key", upstream.URL, time.Second, 0), workspaceID)
	server := mcp.NewServer("test", svc.FullAccessRegistry(), nil, nil)
	if _, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)); err != nil {
		t.Fatal(err)
	}
	raw, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"clockify_invoice_client_work","arguments":{"client_id":"65b382b606de527a7ee2b60e"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Result struct {
			StructuredContent ToolError `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Result.StructuredContent.OK {
		t.Fatalf("expected ok=false feature envelope: %s", raw)
	}
	if got := resp.Result.StructuredContent.Error.Code; got != "feature_unavailable" {
		t.Fatalf("error code=%s, want feature_unavailable: %s", got, raw)
	}
	if resp.Result.StructuredContent.Recovery.Hint == "" {
		t.Fatalf("missing recovery hint: %+v", resp.Result.StructuredContent)
	}
}

func mustJSONText(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestReadmeAvoidsOldPlatformLanguage(t *testing.T) {
	raw, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	readme := strings.ToLower(string(raw))
	for _, forbidden := range []string{
		blockedTerm("hos", "ted"),
		blockedTerm("ten", "ant"),
		blockedTerm("confirmation ", "token"),
		blockedTerm("tier ", "2"),
	} {
		if strings.Contains(readme, forbidden) {
			t.Fatalf("README contains old product language %q", forbidden)
		}
	}
}

func TestPaidFeatureErrorEnvelopeCode(t *testing.T) {
	envelope := recoverable("clockify_time_off_requests_list", &clockify.APIError{
		Method:     "GET",
		Path:       "/workspaces/ws/time-off/requests",
		StatusCode: 403,
		Status:     "403 Forbidden",
		Body:       "feature is not supported on the current plan",
	}, RecoveryHint{})
	if envelope.Error.Code != "feature_unavailable" {
		t.Fatalf("code=%q, want feature_unavailable: %+v", envelope.Error.Code, envelope)
	}
	if envelope.Recovery.Hint == "" {
		t.Fatalf("missing recovery: %+v", envelope)
	}
}
