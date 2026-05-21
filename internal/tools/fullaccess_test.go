package tools

import (
	"context"
	"encoding/json"
	"fmt"
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
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
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
		category, _ := descriptor.Tool.Annotations["category"].(string)
		switch category {
		case "workflow", "domain", "raw":
		default:
			t.Fatalf("%s category=%q, want workflow/domain/raw", name, category)
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

func TestTimerAndReportDescriptorsDoNotShadowEarlierSources(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	earlierSources := map[string][]string{
		"workflow":          descriptorNames(svc.workflowDescriptors()),
		"first_slice":       descriptorNames(svc.FirstSliceRegistry()),
		"native_core":       descriptorNames(svc.nativeCoreDescriptors()),
		"native_high_value": descriptorNames(svc.nativeHighValueDescriptors()),
	}
	seen := map[string]string{}
	for source, names := range earlierSources {
		for _, name := range names {
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = source
		}
	}

	for _, descriptor := range svc.timerAndReportDescriptors() {
		if descriptor.Tool.Annotations["method"] == nil && descriptor.Tool.Annotations["path"] == nil {
			continue
		}
		if source, ok := seen[descriptor.Tool.Name]; ok {
			t.Fatalf("timer/report descriptor %s is shadowed by earlier source %s", descriptor.Tool.Name, source)
		}
		seen[descriptor.Tool.Name] = "timer_report"
	}
}

func TestRemainingHelperDescriptorBucketHasClearName(t *testing.T) {
	raw, err := os.ReadFile("oneuser_domains.go")
	if err != nil {
		t.Fatal(err)
	}
	staleName := "core" + "RouteDescriptors"
	if strings.Contains(string(raw), staleName) {
		t.Fatalf("timer/report helper descriptor bucket should not use the stale %s name", staleName)
	}
}

func TestFullAccessToolsListWorkflowToolsFirstAndAnnotated(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
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

	server := mcp.NewServer("test", reg)
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
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	first := svc.FullAccessRegistry()
	if len(first) != 156 {
		t.Fatalf("registry size=%d, want 156", len(first))
	}
	if len(svc.registry) != 156 {
		t.Fatalf("cached registry size=%d, want 156", len(svc.registry))
	}
	cachedFirstName := svc.registry[0].Tool.Name
	first[0].Tool.Name = "mutated-by-test"

	second := svc.FullAccessRegistry()
	if second[0].Tool.Name != cachedFirstName {
		t.Fatalf("cached registry was not defensively cloned: got %s want %s", second[0].Tool.Name, cachedFirstName)
	}
	if len(svc.registry) != 156 || svc.registry[0].Tool.Name != cachedFirstName {
		t.Fatalf("cached registry changed across calls: len=%d first=%s", len(svc.registry), svc.registry[0].Tool.Name)
	}
}

func TestCloneToolDescriptorsDeepCopy(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	first := svc.FullAccessRegistry()
	if len(first) == 0 {
		t.Fatal("empty registry")
	}

	first[0].Tool.InputSchema["x-mutated"] = true
	if props, ok := first[0].Tool.InputSchema["properties"].(map[string]any); ok {
		props["x-nested-mutated"] = true
	}
	first[0].Tool.Annotations["x-mutated"] = true

	second := svc.FullAccessRegistry()
	if _, ok := second[0].Tool.InputSchema["x-mutated"]; ok {
		t.Fatal("input schema sentinel leaked into cached registry")
	}
	if props, ok := second[0].Tool.InputSchema["properties"].(map[string]any); ok {
		if _, leaked := props["x-nested-mutated"]; leaked {
			t.Fatal("nested input schema sentinel leaked into cached registry")
		}
	}
	if _, ok := second[0].Tool.Annotations["x-mutated"]; ok {
		t.Fatal("annotation sentinel leaked into cached registry")
	}
}

func TestRegistryForToolsetFiltersOwnerSurfaces(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	tests := []struct {
		toolset   string
		want      []string
		wantOrder []string
		absent    []string
	}{
		{
			toolset: "default",
			want: []string{
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
				"clockify_entries_running",
				"clockify_entries_list",
				"clockify_entries_get",
				"clockify_entries_update",
				"clockify_entries_delete",
				"clockify_reports_summary",
			},
			wantOrder: []string{
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
				"clockify_entries_list",
				"clockify_entries_get",
				"clockify_entries_update",
				"clockify_entries_delete",
				"clockify_entries_running",
				"clockify_reports_summary",
			},
			absent: []string{
				"clockify_demo_seed",
				"clockify_clients_list",
				"clockify_entries_create",
				"clockify_entries_timer_start",
				"clockify_reports_detailed",
				"clockify_api_request",
			},
		},
		{
			toolset: "core",
			want: []string{
				"clockify_status",
				"clockify_tools_guide",
				"clockify_create_work_package",
				"clockify_log_work",
				"clockify_clients_list",
				"clockify_projects_create",
				"clockify_entries_timer_start",
				"clockify_reports_summary",
			},
			absent: []string{
				"clockify_demo_seed",
				"clockify_invoice_client_work",
				"clockify_invoices_create",
				"clockify_entries_mark_invoiced",
				"clockify_webhooks_create",
				"clockify_api_request",
			},
		},
		{
			toolset: "business",
			want: []string{
				"clockify_invoice_client_work",
				"clockify_record_expense",
				"clockify_invoices_create",
				"clockify_expenses_create",
				"clockify_entries_mark_invoiced",
			},
			absent: []string{
				"clockify_demo_seed",
				"clockify_request_time_off",
				"clockify_webhooks_create",
				"clockify_users_invite",
				"clockify_api_request",
			},
		},
		{
			toolset: "admin",
			want: []string{
				"clockify_request_time_off",
				"clockify_schedule_work",
				"clockify_setup_webhook",
				"clockify_custom_fields_create",
				"clockify_time_off_requests_create",
				"clockify_scheduling_assignments_create",
				"clockify_approvals_approve",
				"clockify_webhooks_create",
				"clockify_groups_delete",
				"clockify_holidays_create",
				"clockify_users_invite",
				"clockify_workspace_settings",
			},
			absent: []string{
				"clockify_demo_seed",
				"clockify_api_get",
				"clockify_api_request",
			},
		},
		{
			toolset: "all",
			want: []string{
				"clockify_demo_seed",
				"clockify_api_get",
				"clockify_api_request",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.toolset, func(t *testing.T) {
			reg := svc.RegistryForToolset(tt.toolset)
			assertNoDuplicateTools(t, reg)
			assertWorkflowPrefixForRegistry(t, reg)
			names := toolNameSet(reg)
			for _, want := range tt.want {
				if !names[want] {
					t.Fatalf("%s registry missing %s", tt.toolset, want)
				}
			}
			for _, absent := range tt.absent {
				if names[absent] {
					t.Fatalf("%s registry unexpectedly includes %s", tt.toolset, absent)
				}
			}
			if len(tt.wantOrder) > 0 {
				if got := descriptorNames(reg); !slicesEqual(got, tt.wantOrder) {
					t.Fatalf("%s registry order mismatch\n got=%v\nwant=%v", tt.toolset, got, tt.wantOrder)
				}
			}
			assertToolsetToolsListMatchesRegistry(t, reg)
		})
	}
}

func BenchmarkFullAccessRegistry(b *testing.B) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	b.ReportAllocs()
	for b.Loop() {
		if got := len(svc.FullAccessRegistry()); got != 156 {
			b.Fatalf("registry size=%d, want 156", got)
		}
	}
}

func BenchmarkRegistryForToolset(b *testing.B) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	b.ReportAllocs()
	for b.Loop() {
		if got := len(svc.RegistryForToolset("business")); got == 0 || got >= 156 {
			b.Fatalf("business registry size=%d, want filtered non-empty registry", got)
		}
	}
}

func descriptorNames(descriptors []mcp.ToolDescriptor) []string {
	names := make([]string, 0, len(descriptors))
	for _, descriptor := range descriptors {
		names = append(names, descriptor.Tool.Name)
	}
	return names
}

func assertNoDuplicateTools(t *testing.T, descriptors []mcp.ToolDescriptor) {
	t.Helper()
	seen := map[string]bool{}
	for _, descriptor := range descriptors {
		if seen[descriptor.Tool.Name] {
			t.Fatalf("duplicate tool %s", descriptor.Tool.Name)
		}
		seen[descriptor.Tool.Name] = true
	}
}

func assertWorkflowPrefixForRegistry(t *testing.T, descriptors []mcp.ToolDescriptor) {
	t.Helper()
	seenDomain := false
	for _, descriptor := range descriptors {
		category, _ := descriptor.Tool.Annotations["category"].(string)
		if category == "workflow" {
			if seenDomain {
				t.Fatalf("workflow tool %s appears after a non-workflow tool", descriptor.Tool.Name)
			}
			assertWorkflowAnnotations(t, descriptor.Tool)
			continue
		}
		seenDomain = true
	}
}

func toolNameSet(descriptors []mcp.ToolDescriptor) map[string]bool {
	out := make(map[string]bool, len(descriptors))
	for _, descriptor := range descriptors {
		out[descriptor.Tool.Name] = true
	}
	return out
}

func assertToolsetToolsListMatchesRegistry(t *testing.T, descriptors []mcp.ToolDescriptor) {
	t.Helper()
	server := mcp.NewServer("test", descriptors)
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
	got := make([]string, 0, len(resp.Result.Tools))
	for _, tool := range resp.Result.Tools {
		got = append(got, tool.Name)
	}
	wantDescriptors := cloneToolDescriptors(descriptors)
	sortDescriptorsForToolsList(wantDescriptors)
	want := descriptorNames(wantDescriptors)
	if !slicesEqual(got, want) {
		t.Fatalf("tools/list names diverged\n got=%v\nwant=%v", got, want)
	}
}

func slicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func BenchmarkOneUserToolsResourceData(b *testing.B) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	b.ReportAllocs()
	for b.Loop() {
		data := svc.toolsResourceData()
		if got, _ := data["count"].(int); got != 156 {
			b.Fatalf("tools resource count=%d, want 156", got)
		}
	}
}

func TestOneUserToolsResourceDataIsCachedAndDefensivelyCloned(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
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
	if got, _ := second["count"].(int); got != 156 {
		t.Fatalf("cached tools resource count=%d, want 156", got)
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
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	server := mcp.NewServer("bench", svc.FullAccessRegistry())
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

// BenchmarkClockifyStatusDispatch measures the full clockify_status path
// through the MCP server against a fake Clockify upstream: the workflow
// handler's multi-endpoint fan-out, response decode, and envelope build.
// The fake server is local so HTTP cost is near-zero — what moves is the
// status handler itself, which makes this a useful regression sentinel.
func BenchmarkClockifyStatusDispatch(b *testing.B) {
	upstream := newOneUserCoverageUpstream()
	defer upstream.Close()
	svc := New(clockify.NewClient("test-key", upstream.URL, 5*time.Second, 0), "000000000000000000000001")
	svc.DefaultTimezone = time.UTC
	server := mcp.NewServer("bench", svc.FullAccessRegistry())
	server.MarkInitialized(mcp.SupportedProtocolVersions[0], "bench", "0")
	msg := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"clockify_status","arguments":{}}}`)
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		if _, err := server.DispatchMessage(ctx, msg); err != nil {
			b.Fatalf("clockify_status: %v", err)
		}
	}
}

// BenchmarkClockifyReviewDayDispatch covers a heavier workflow tool:
// review-day aggregates entries, detects gaps/overlaps, and assembles a
// structured report. It is the most allocation-sensitive workflow call.
func BenchmarkClockifyReviewDayDispatch(b *testing.B) {
	upstream := newOneUserCoverageUpstream()
	defer upstream.Close()
	svc := New(clockify.NewClient("test-key", upstream.URL, 5*time.Second, 0), "000000000000000000000001")
	svc.DefaultTimezone = time.UTC
	server := mcp.NewServer("bench", svc.FullAccessRegistry())
	server.MarkInitialized(mcp.SupportedProtocolVersions[0], "bench", "0")
	msg := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"clockify_review_day","arguments":{"date":"2026-01-02"}}}`)
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		if _, err := server.DispatchMessage(ctx, msg); err != nil {
			b.Fatalf("clockify_review_day: %v", err)
		}
	}
}

func BenchmarkReportNameResolution(b *testing.B) {
	upstream := newOneUserCoverageUpstream()
	defer upstream.Close()
	svc := New(clockify.NewClient("test-key", upstream.URL, 5*time.Second, 0), "000000000000000000000001")
	svc.DefaultTimezone = time.UTC
	server := mcp.NewServer("bench", svc.FullAccessRegistry())
	server.MarkInitialized(mcp.SupportedProtocolVersions[0], "bench", "0")
	msg := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"clockify_reports_detailed","arguments":{"start":"2026-01-01","end":"2026-01-02","project":"Project One"}}}`)
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		if _, err := server.DispatchMessage(ctx, msg); err != nil {
			b.Fatalf("report name resolution: %v", err)
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
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
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
	fake := testclockify.NewServer("000000000000000000000001")
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
	if data.Totals.Entries != 1 || data.Totals.TotalSeconds <= 0 {
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
	const workspaceID = "000000000000000000000001"
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
	server := mcp.NewServer("test", svc.FullAccessRegistry())
	if _, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)); err != nil {
		t.Fatal(err)
	}
	raw, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"clockify_invoice_client_work","arguments":{"client_id":"000000000000000000000001","currency":"USD"}}}`))
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

func TestSchedulingCapacityDoesNotRequireUserIds(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "ws1")
	for _, descriptor := range svc.FullAccessRegistry() {
		if descriptor.Tool.Name != "clockify_scheduling_capacity" {
			continue
		}
		required := inputSchemaRequiredFields(descriptor.Tool.InputSchema)
		for _, field := range required {
			if field == "user_ids" {
				t.Fatalf("clockify_scheduling_capacity required=%v, user_ids must be optional", required)
			}
		}
		return
	}
	t.Fatal("clockify_scheduling_capacity not found")
}

func TestFullAccessInputSchemasAreValidAndTyped(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "ws1")
	for _, descriptor := range svc.FullAccessRegistry() {
		if err := assertSchemaRequiredAndPropertyTypes(descriptor.Tool.InputSchema, descriptor.Tool.Name, "$inputSchema"); err != nil {
			t.Error(err)
		}
	}
}

func assertSchemaRequiredAndPropertyTypes(schema any, tool, path string) error {
	m, ok := schema.(map[string]any)
	if !ok {
		return nil
	}
	if req, exists := m["required"]; exists {
		if req == nil {
			return newSchemaError(tool, path, "required is null")
		}
		switch items := req.(type) {
		case []string:
			// ok
		case []any:
			for i, item := range items {
				if _, ok := item.(string); !ok {
					return newSchemaError(tool, path, fmt.Sprintf("required[%d] is not a string", i))
				}
			}
		default:
			return newSchemaError(tool, path, fmt.Sprintf("required has unsupported type %T", req))
		}
	}
	if props, ok := m["properties"].(map[string]any); ok {
		for name, raw := range props {
			pm, ok := raw.(map[string]any)
			if !ok {
				return newSchemaError(tool, path+"."+name, "property schema is not an object")
			}
			if _, hasType := pm["type"]; !hasType {
				if _, hasAnyOf := pm["anyOf"]; !hasAnyOf {
					if _, hasConst := pm["const"]; !hasConst {
						return newSchemaError(tool, path+"."+name, "property is missing type/anyOf/const")
					}
				}
			}
			if err := assertSchemaRequiredAndPropertyTypes(pm, tool, path+"."+name); err != nil {
				return err
			}
		}
	}
	if items, ok := m["items"].(map[string]any); ok {
		if err := assertSchemaRequiredAndPropertyTypes(items, tool, path+"[*]"); err != nil {
			return err
		}
	}
	if options, ok := m["anyOf"].([]any); ok {
		for i, option := range options {
			if err := assertSchemaRequiredAndPropertyTypes(option, tool, fmt.Sprintf("%s.anyOf[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}
