package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/testclockify"
)

func TestQualityGateResultEnvelopeContract(t *testing.T) {
	out := result("clockify_test_write", "test_entity", map[string]string{
		"workspaceId": "ws1",
		"empty":       "",
	}, map[string]any{"ok": true}, ChangeSet{
		Created: []EntityRef{{Type: "test_entity", ID: "created-1", Name: "Created"}},
		Reused:  []EntityRef{{Type: "test_entity", ID: "reused-1", Name: "Reused"}},
	}, []Warning{{Code: "note", Message: "warning"}}, []NextAction{{Tool: "clockify_status"}})

	if !out.OK || out.Action != "clockify_test_write" || out.Entity != "test_entity" {
		t.Fatalf("bad envelope identity: %+v", out)
	}
	if out.IDs["workspaceId"] != "ws1" {
		t.Fatalf("workspace id missing: %+v", out.IDs)
	}
	if _, ok := out.IDs["empty"]; ok {
		t.Fatalf("empty ID should be stripped: %+v", out.IDs)
	}
	if len(out.Changed.Created) != 1 || len(out.Changed.Reused) != 1 {
		t.Fatalf("changed contract missing created/reused: %+v", out.Changed)
	}
	if len(out.Warnings) != 1 || len(out.Next) != 1 {
		t.Fatalf("warnings/next missing: %+v", out)
	}
}

func TestQualityGateRecoveryErrorCodes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "404",
			err: &clockify.APIError{
				Method:     "GET",
				Path:       "/workspaces/ws/clients/missing",
				StatusCode: 404,
				Status:     "404 Not Found",
				Body:       `{"message":"not found"}`,
			},
			want: "not_found",
		},
		{
			name: "403",
			err: &clockify.APIError{
				Method:     "GET",
				Path:       "/workspaces/ws/clients",
				StatusCode: 403,
				Status:     "403 Forbidden",
				Body:       `{"message":"forbidden"}`,
			},
			want: "auth_or_permission",
		},
		{
			name: "paid feature",
			err: &clockify.APIError{
				Method:     "POST",
				Path:       "/workspaces/ws/invoices",
				StatusCode: 403,
				Status:     "403 Forbidden",
				Body:       `{"message":"feature is not supported on this plan"}`,
			},
			want: "feature_unavailable",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := recoverable("clockify_clients_list", tt.err, RecoveryHint{})
			if !env.OK && env.Error.Code != tt.want {
				t.Fatalf("code=%q want %q: %+v", env.Error.Code, tt.want, env)
			}
			if env.OK || env.Recovery.Hint == "" {
				t.Fatalf("bad recovery envelope: %+v", env)
			}
		})
	}
}

func TestQualityGateNameResolutionStrictAndTimeParsing(t *testing.T) {
	fake := testclockify.NewServer("65b382b606de527a7ee2b60e")
	defer fake.Close()
	svc := New(clockify.NewClient("test-key", fake.URL, time.Second, 0), fake.WorkspaceID)
	loc, err := time.LoadLocation("Europe/Belgrade")
	if err != nil {
		t.Fatal(err)
	}
	svc.DefaultTimezone = loc
	ctx := context.Background()

	pkgOut, err := svc.ClockifyCreateWorkPackage(ctx, map[string]any{
		"client":  "Resolve Client",
		"project": "Resolve Project",
		"task":    "Resolve Task",
		"tag":     "Resolve Tag",
	})
	pkg := mustToolResult(t, pkgOut, err)

	entryOut, err := svc.EntriesCreate(ctx, map[string]any{
		"start":       "2026-01-02 09:00",
		"end":         "2026-01-02 10:00",
		"description": "Resolve by name",
		"project":     "Resolve Project",
		"task":        "Resolve Task",
		"tag":         "Resolve Tag",
	})
	entryResult := mustToolResult(t, entryOut, err)
	if entryResult.IDs["projectId"] != pkg.IDs["projectId"] || entryResult.IDs["taskId"] != pkg.IDs["taskId"] || entryResult.IDs["tagId"] != pkg.IDs["tagId"] {
		t.Fatalf("name resolution returned wrong IDs: got %+v package %+v", entryResult.IDs, pkg.IDs)
	}
	entry, ok := entryResult.Data.(clockify.TimeEntry)
	if !ok {
		t.Fatalf("entry data type = %T", entryResult.Data)
	}
	if entry.TimeInterval.Start != "2026-01-02T08:00:00Z" || entry.TimeInterval.End != "2026-01-02T09:00:00Z" {
		t.Fatalf("timezone parsing did not use Europe/Belgrade: %+v", entry.TimeInterval)
	}

	if _, err := svc.ClientsCreate(ctx, map[string]any{"name": "Duplicate Client"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ClientsCreate(ctx, map[string]any{"name": "Duplicate Client"}); err != nil {
		t.Fatal(err)
	}
	_, err = svc.ClockifyCreateWorkPackage(ctx, map[string]any{
		"client":  "Duplicate Client",
		"project": "Should Not Resolve",
	})
	if err == nil || !strings.Contains(err.Error(), "multiple clients match") {
		t.Fatalf("expected strict duplicate-name failure, got %v", err)
	}
}

func TestQualityGateGoldenInitializeToolsPromptsResources(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	server := mcp.NewServer("test", svc.FullAccessRegistry(), nil, nil)
	server.StaticToolList = true
	server.ResourceProvider = svc

	responses := runOneUserProtocol(t, server, []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"prompts/list"}`,
		`{"jsonrpc":"2.0","id":4,"method":"resources/list"}`,
		`{"jsonrpc":"2.0","id":5,"method":"resources/templates/list"}`,
	})

	initialize := responses[1]
	assertGoldenJSON(t, "oneuser_initialize.golden.json", initialize)
	initializeText := mustJSONText(t, initialize)
	for _, forbidden := range []string{"confirmation", "activation", "activate", blockedTerm("ten", "ant"), blockedTerm("hos", "ted"), blockedTerm("Tier ", "2"), blockedTerm("policy ", "mode")} {
		if strings.Contains(strings.ToLower(initializeText), strings.ToLower(forbidden)) {
			t.Fatalf("initialize golden contains forbidden language %q: %s", forbidden, initializeText)
		}
	}

	toolsResult := responses[2]
	tools := decodeToolsFromResult(t, toolsResult)
	toolNames := make([]string, 0, len(tools))
	for _, tool := range tools {
		toolNames = append(toolNames, tool.Name)
		if tool.InputSchema == nil {
			t.Fatalf("exposed tool %s missing input schema", tool.Name)
		}
	}
	assertGoldenJSON(t, "oneuser_tools_list.golden.json", toolNames)
	toolGolden := mustJSONText(t, toolNames)
	for _, forbidden := range []string{
		blockedTerm("clockify_", "activate_group"),
		blockedTerm("clockify_", "activate_tool"),
		blockedTerm("clockify_", "deactivate_group"),
		blockedTerm("clockify_", "search_tools"),
		blockedTerm("clockify_", "list_tools"),
	} {
		if strings.Contains(toolGolden, forbidden) {
			t.Fatalf("tools/list golden contains old tool %s", forbidden)
		}
	}
	if len(tools) != 151 {
		t.Fatalf("tools/list returned %d tools, want 151", len(tools))
	}
	if tools[0].Name != "clockify_status" {
		t.Fatalf("first tool=%s, want clockify_status", tools[0].Name)
	}
	if tools[len(tools)-2].Name != "clockify_api_get" || tools[len(tools)-1].Name != "clockify_api_request" {
		t.Fatalf("raw API tools are not last: %s, %s", tools[len(tools)-2].Name, tools[len(tools)-1].Name)
	}
	seenDomain := false
	for _, tool := range tools[:len(tools)-2] {
		if tool.Annotations["category"] == "workflow" {
			if seenDomain {
				t.Fatalf("workflow tool %s appears after a domain tool", tool.Name)
			}
			continue
		}
		seenDomain = true
	}

	promptsResult := responses[3]
	promptText := mustJSONText(t, promptsResult)
	for _, forbidden := range []string{
		blockedTerm("confirmation ", "token"),
		blockedTerm("policy ", "mode"),
		blockedTerm("ten", "ant"),
		blockedTerm("hos", "ted"),
		blockedTerm("shared-", "service"),
		blockedTerm("tier ", "2"),
		blockedTerm("control ", "plane"),
		blockedTerm("forward ", "auth"),
	} {
		if strings.Contains(strings.ToLower(promptText), forbidden) {
			t.Fatalf("prompts/list contains forbidden language %q: %s", forbidden, promptText)
		}
	}
	promptNames := namesFromPromptResult(t, promptsResult)
	assertGoldenJSON(t, "oneuser_prompts_list.golden.json", promptNames)
	wantPrompts := []string{"demo-full-workspace-story", "setup-client-project-task", "log-week", "invoice-client", "cleanup-demo", "review-week", "create-expense", "request-time-off", "schedule-week", "setup-webhook"}
	if !slices.Equal(promptNames, wantPrompts) {
		t.Fatalf("prompts/list names=%v, want %v", promptNames, wantPrompts)
	}

	resourcesResult := responses[4]
	assertGoldenJSON(t, "oneuser_resources_list.golden.json", urisFromResourceResult(t, resourcesResult))
	if len(urisFromResourceResult(t, resourcesResult)) == 0 {
		t.Fatal("resources/list returned no concrete resources")
	}

	templateNames := uriTemplatesFromResourceTemplateResult(t, responses[5])
	if !slices.Contains(templateNames, "clockify://demo/{run_id}") {
		t.Fatalf("resources/templates/list missing demo template: %v", templateNames)
	}
}

func TestQualityGateFakeClockifyDomainWritesAndErrorEdges(t *testing.T) {
	fake := testclockify.NewServer("65b382b606de527a7ee2b60e")
	defer fake.Close()
	client := clockify.NewClient("test-key", fake.URL, time.Second, 0)
	svc := New(client, fake.WorkspaceID)
	svc.DefaultTimezone = time.UTC
	server := mcp.NewServer("test", svc.FullAccessRegistry(), nil, nil)
	initializeServer(t, server)

	clientResult := callToolOK(t, server, "clockify_clients_create", map[string]any{"name": "Quality Client"})
	requireID(t, clientResult, "clientId")
	projectResult := callToolOK(t, server, "clockify_projects_create", map[string]any{"name": "Quality Project", "client_id": clientResult.IDs["clientId"]})
	requireID(t, projectResult, "projectId")
	taskResult := callToolOK(t, server, "clockify_tasks_create", map[string]any{"name": "Quality Task", "project_id": projectResult.IDs["projectId"]})
	requireID(t, taskResult, "taskId")
	tagResult := callToolOK(t, server, "clockify_tags_create", map[string]any{"name": "Quality Tag"})
	requireID(t, tagResult, "tagId")
	entryResult := callToolOK(t, server, "clockify_entries_create", map[string]any{
		"start":       "2026-01-02 09:00",
		"end":         "2026-01-02 10:00",
		"description": "Quality entry",
		"project_id":  projectResult.IDs["projectId"],
		"task_id":     taskResult.IDs["taskId"],
		"tag_ids":     []any{tagResult.IDs["tagId"]},
	})
	requireID(t, entryResult, "entryId")
	seedResult := callToolOK(t, server, "clockify_demo_seed", map[string]any{"run_id": "quality"})
	for _, key := range []string{"clientId", "projectId", "taskId", "tagId", "entryId"} {
		requireID(t, seedResult, key)
	}
	cleanupResult := callToolOK(t, server, "clockify_demo_cleanup", map[string]any{"run_id": "quality"})
	requireID(t, cleanupResult, "workspaceId")

	notFound := callToolError(t, server, "clockify_clients_get", map[string]any{"client_id": "missing-client"})
	if notFound.Error.Code != "not_found" || notFound.Recovery.Hint == "" {
		t.Fatalf("bad 404 recovery envelope: %+v", notFound)
	}

	fake.SetError("GET", "/workspaces/"+fake.WorkspaceID+"/clients", 403, "forbidden")
	forbidden := callToolError(t, server, "clockify_clients_list", nil)
	if forbidden.Error.Code != "auth_or_permission" || forbidden.Recovery.Hint == "" {
		t.Fatalf("bad 403 recovery envelope: %+v", forbidden)
	}
}

func TestQualityGateFakeClockifyFeatureUnavailableAndPagination(t *testing.T) {
	fake := testclockify.NewServer("65b382b606de527a7ee2b60e")
	defer fake.Close()
	client := clockify.NewClient("test-key", fake.URL, time.Second, 0)
	svc := New(client, fake.WorkspaceID)
	server := mcp.NewServer("test", svc.FullAccessRegistry(), nil, nil)
	initializeServer(t, server)

	fake.SetError("POST", "/workspaces/"+fake.WorkspaceID+"/invoices", 403, "feature is not supported on this plan")
	feature := callToolError(t, server, "clockify_invoice_client_work", map[string]any{"client_id": "client-paid-feature"})
	if feature.Error.Code != "feature_unavailable" || feature.Recovery.Hint == "" || feature.Recovery.Tool == "" {
		t.Fatalf("bad feature_unavailable envelope: %+v", feature)
	}

	for i := range 5 {
		_, err := svc.ClientsCreate(context.Background(), map[string]any{"name": "Page Client " + string(rune('A'+i))})
		if err != nil {
			t.Fatal(err)
		}
	}
	pageOut, err := svc.ClientsList(context.Background(), map[string]any{"page": 2, "page_size": 2})
	page := mustToolResult(t, pageOut, err)
	data, ok := page.Data.(map[string]any)
	if !ok {
		t.Fatalf("page data type = %T", page.Data)
	}
	clients, ok := data["clients"].([]clockify.ClientEntity)
	if !ok {
		t.Fatalf("clients data type = %T", data["clients"])
	}
	if len(clients) != 2 || data["page"] != 2 || data["pageSize"] != 2 {
		t.Fatalf("pagination not applied: %+v", data)
	}
}

func TestOneUserDocsAvoidRemovedToolNames(t *testing.T) {
	for _, path := range []string{
		"../../README.md",
		"../../docs/agent-cookbook.md",
		"../../docs/tool-catalog.md",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.ToLower(string(raw))
		for _, forbidden := range []string{
			blockedTerm("clockify_", "activate_group"),
			blockedTerm("clockify_", "activate_tool"),
			blockedTerm("clockify_", "deactivate_group"),
			blockedTerm("clockify_", "search_tools"),
			blockedTerm("clockify_", "policy_info"),
			blockedTerm("clockify_", "list_tools"),
			blockedTerm("clockify_", "resolve_name"),
			blockedTerm("clockify_", "log_time"),
			blockedTerm("clockify_", "timesheet_review"),
			blockedTerm("clockify_", "timesheet_fill_gap"),
			blockedTerm("clockify_", "switch_project"),
			blockedTerm("clockify_", "timer_status"),
			"activation",
			blockedTerm("policy ", "mode"),
			blockedTerm("confirmation ", "token"),
			blockedTerm("ten", "ant"),
			blockedTerm("hos", "ted"),
			blockedTerm("shared-", "service"),
			blockedTerm("tier ", "2"),
			blockedTerm("control ", "plane"),
			blockedTerm("forward ", "auth"),
			blockedTerm("oi", "dc"),
			blockedTerm("mt", "ls"),
			blockedTerm("gr", "pc"),
			blockedTerm("stream", "able"),
		} {
			if strings.Contains(text, strings.ToLower(forbidden)) {
				t.Fatalf("%s contains removed one-user language/tool %q", path, forbidden)
			}
		}
	}
}

func TestOneUserOutputSchemasAreActionPinnedToolResultEnvelopes(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	for _, descriptor := range svc.FullAccessRegistry() {
		t.Run(descriptor.Tool.Name, func(t *testing.T) {
			schema := descriptor.Tool.OutputSchema
			if schema == nil {
				t.Fatalf("missing output schema")
			}
			props, ok := schema["properties"].(map[string]any)
			if !ok {
				t.Fatalf("output schema missing properties: %+v", schema)
			}
			action, ok := props["action"].(map[string]any)
			if !ok {
				t.Fatalf("output schema missing action property: %+v", props)
			}
			if action["const"] != descriptor.Tool.Name {
				t.Fatalf("action const=%v want %s", action["const"], descriptor.Tool.Name)
			}
			for _, field := range []string{"ok", "ids", "data", "changed", "warnings", "next", "error", "recovery"} {
				if _, ok := props[field]; !ok {
					t.Fatalf("output schema missing %s property: %+v", field, props)
				}
			}
		})
	}
}

func TestOneUserCoverageLedgerHasNoKnownImplementationGaps(t *testing.T) {
	raw, err := os.ReadFile("../../docs/goals/oneuser-tool-coverage.md")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(raw))
	for _, forbidden := range []string{
		"needs follow-up",
		"remaining known gap",
		"| generic |",
		"still need direct fake-server tests",
		"shared generic result envelope",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("coverage ledger still advertises gap %q", forbidden)
		}
	}
}

func TestOneUserStatusIsUsefulFirstCall(t *testing.T) {
	fake := testclockify.NewServer("65b382b606de527a7ee2b60e")
	defer fake.Close()
	client := clockify.NewClient("test-key", fake.URL, time.Second, 0)
	svc := New(client, fake.WorkspaceID)
	svc.DefaultTimezone = time.UTC

	statusOut, err := svc.ClockifyStatus(context.Background(), nil)
	status := mustToolResult(t, statusOut, err)
	requireID(t, status, "workspaceId")
	requireID(t, status, "userId")
	data, ok := status.Data.(statusData)
	if !ok {
		t.Fatalf("status data type=%T", status.Data)
	}
	if data.User.ID == "" || data.User.Name == "" || data.User.Email == "" {
		t.Fatalf("status missing user identity: %+v", data.User)
	}
	if data.PinnedWorkspace.ID != fake.WorkspaceID || data.PinnedWorkspace.Name == "" {
		t.Fatalf("status missing pinned workspace: %+v", data.PinnedWorkspace)
	}
	if data.ActiveWorkspaceID == "" || data.DefaultWorkspaceID == "" {
		t.Fatalf("status missing active/default workspace: %+v", data)
	}
	if data.Timezone != "UTC" || data.WeekStart == "" {
		t.Fatalf("status missing timezone/week start: %+v", data)
	}
	if len(data.WorkspaceFeatures) == 0 {
		t.Fatalf("status missing raw workspace features: %+v", data)
	}
	for _, feature := range []string{"invoices", "expenses", "customFields", "timeOff", "scheduling", "approvals", "webhooks", "reports", "groups", "holidays", "sharedReports"} {
		if data.FeatureStatus[feature] == "" {
			t.Fatalf("featureStatus missing %s: %+v", feature, data.FeatureStatus)
		}
	}
	if len(data.RecommendedFirstTools) == 0 || data.RecommendedFirstTools[0] != "clockify_tools_guide" {
		t.Fatalf("bad recommendedFirstTools: %+v", data.RecommendedFirstTools)
	}
	if len(status.Warnings) == 0 {
		t.Fatalf("status should warn about unavailable/unknown feature signals: %+v", status)
	}
}

func TestMCPStartupListingsMakeNoClockifyRequests(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, `{"message":"unexpected network call"}`, http.StatusTeapot)
	}))
	defer upstream.Close()

	svc := New(clockify.NewClient("test-key", upstream.URL, time.Second, 0), "65b382b606de527a7ee2b60e")
	server := mcp.NewServer("test", svc.FullAccessRegistry(), nil, nil)
	server.StaticToolList = true
	server.ResourceProvider = svc

	_ = runOneUserProtocol(t, server, []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":4,"method":"prompts/list"}`,
		`{"jsonrpc":"2.0","id":5,"method":"resources/list"}`,
		`{"jsonrpc":"2.0","id":6,"method":"resources/templates/list"}`,
	})
	if got := calls.Load(); got != 0 {
		t.Fatalf("startup/list methods made %d upstream calls before first tool/resource read", got)
	}
}

func TestToolsResourceMatchesServerToolsList(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	server := mcp.NewServer("test", svc.FullAccessRegistry(), nil, nil)
	server.StaticToolList = true
	server.ResourceProvider = svc
	initializeServer(t, server)

	raw, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	var toolsResp struct {
		Result struct {
			Tools []mcp.Tool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &toolsResp); err != nil {
		t.Fatal(err)
	}
	want := make([]string, 0, len(toolsResp.Result.Tools))
	for _, tool := range toolsResp.Result.Tools {
		want = append(want, tool.Name)
	}

	contents, err := svc.ReadResource(context.Background(), "clockify://tools")
	if err != nil {
		t.Fatal(err)
	}
	var resource struct {
		Count int        `json:"count"`
		Tools []mcp.Tool `json:"tools"`
	}
	if err := json.Unmarshal([]byte(contents[0].Text), &resource); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(resource.Tools))
	for _, tool := range resource.Tools {
		got = append(got, tool.Name)
	}
	if resource.Count != len(want) || !slices.Equal(got, want) {
		t.Fatalf("clockify://tools diverged from tools/list\nresource count=%d got=%v\nwant=%v", resource.Count, got, want)
	}
}

func TestHighRiskWorkflowAndRawFallbackFakeServerCoverage(t *testing.T) {
	fake := testclockify.NewServer("65b382b606de527a7ee2b60e")
	defer fake.Close()
	client := clockify.NewClient("test-key", fake.URL, time.Second, 0)
	svc := New(client, fake.WorkspaceID)
	svc.DefaultTimezone = time.UTC
	server := mcp.NewServer("test", svc.FullAccessRegistry(), nil, nil)
	initializeServer(t, server)

	const (
		clientID   = "65b382b606de527a7ee2b60a"
		categoryID = "65b382b606de527a7ee2b60b"
		policyID   = "65b382b606de527a7ee2b60c"
		projectID  = "65b382b606de527a7ee2b60d"
		userID     = "65b382b606de527a7ee2b60e"
	)

	invoice := callToolOK(t, server, "clockify_invoice_client_work", map[string]any{"client_id": clientID, "number": "INV-FAKE-1"})
	requireID(t, invoice, "workspaceId")
	requireID(t, invoice, "clientId")
	requireID(t, invoice, "invoiceId")
	requireChanged(t, invoice, "created")
	requireNext(t, invoice)

	expense := callToolOK(t, server, "clockify_record_expense", map[string]any{
		"amount":      float64(125),
		"category_id": categoryID,
		"project_id":  projectID,
		"date":        "2026-01-02T00:00:00Z",
		"notes":       "Fake coverage",
	})
	requireID(t, expense, "workspaceId")
	requireID(t, expense, "expenseId")
	requireChanged(t, expense, "created")
	requireNext(t, expense)

	timeOff := callToolOK(t, server, "clockify_request_time_off", map[string]any{"policy_id": policyID, "start": "2026-01-05", "end": "2026-01-06", "note": "Fake coverage"})
	requireID(t, timeOff, "workspaceId")
	requireID(t, timeOff, "policyId")
	requireID(t, timeOff, "timeOffRequestId")
	requireChanged(t, timeOff, "created")
	requireNext(t, timeOff)

	scheduled := callToolOK(t, server, "clockify_schedule_work", map[string]any{
		"user_id":       userID,
		"project_id":    projectID,
		"start":         "2026-01-05T09:00:00Z",
		"end":           "2026-01-09T17:00:00Z",
		"hours_per_day": float64(6),
	})
	requireID(t, scheduled, "workspaceId")
	requireID(t, scheduled, "userId")
	requireID(t, scheduled, "projectId")
	requireID(t, scheduled, "assignmentId")
	requireChanged(t, scheduled, "created")
	requireNext(t, scheduled)

	webhook := callToolOK(t, server, "clockify_setup_webhook", map[string]any{"name": "Fake webhook", "url": "https://example.com/clockify", "event": "NEW_TIME_ENTRY"})
	requireID(t, webhook, "workspaceId")
	requireID(t, webhook, "webhookId")
	requireChanged(t, webhook, "created")
	requireNext(t, webhook)

	rawGet := callToolOK(t, server, "clockify_api_get", map[string]any{"path": "/workspaces/{workspaceId}"})
	requireID(t, rawGet, "workspaceId")

	rawWrite := callToolOK(t, server, "clockify_api_request", map[string]any{"method": "POST", "path": "/workspaces/{workspaceId}/clients", "body": map[string]any{"name": "Raw Client"}})
	requireID(t, rawWrite, "workspaceId")
	requireID(t, rawWrite, "rawApiId")
	requireChanged(t, rawWrite, "created")

	fake.SetError("POST", "/workspaces/"+fake.WorkspaceID+"/expenses", 403, "feature is not supported on this plan")
	failure := callToolError(t, server, "clockify_record_expense", map[string]any{"amount": float64(99), "category_id": categoryID, "date": "2026-01-02T00:00:00Z"})
	if failure.Error.Code != "feature_unavailable" || failure.Recovery.Hint == "" || failure.Recovery.Tool == "" {
		t.Fatalf("bad recoverable failure envelope: %+v", failure)
	}
}

func TestOneUserAdvertisedDomainToolsFakeServerSmoke(t *testing.T) {
	upstream := newOneUserCoverageUpstream()
	defer upstream.Close()
	svc := New(clockify.NewClient("test-key", upstream.URL, time.Second, 0), "65b382b606de527a7ee2b60e")
	svc.DefaultTimezone = time.UTC
	server := mcp.NewServer("test", svc.FullAccessRegistry(), nil, nil)
	initializeServer(t, server)

	for _, descriptor := range svc.FullAccessRegistry() {
		category, _ := descriptor.Tool.Annotations["category"].(string)
		if category == "workflow" || descriptor.Tool.Name == "clockify_api_get" || descriptor.Tool.Name == "clockify_api_request" {
			continue
		}
		t.Run(descriptor.Tool.Name, func(t *testing.T) {
			raw := callToolRaw(t, server, descriptor.Tool.Name, oneUserCoverageArgs(descriptor.Tool))
			result := decodeToolResult(t, descriptor.Tool.Name, raw)
			if result.IDs["workspaceId"] == "" {
				t.Fatalf("domain tool did not return workspaceId: %+v", result)
			}
			if !descriptor.ReadOnlyHint && !hasAnyChange(result.Changed) {
				t.Fatalf("write tool did not report a changed entity: %+v", result.Changed)
			}
		})
	}
}

func runOneUserProtocol(t *testing.T, server *mcp.Server, lines []string) map[float64]map[string]any {
	t.Helper()
	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(strings.Join(lines, "\n")+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	responses := map[float64]map[string]any{}
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var resp struct {
			ID     float64        `json:"id"`
			Result map[string]any `json:"result"`
			Error  any            `json:"error,omitempty"`
		}
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("decode response line %q: %v", line, err)
		}
		if resp.Error != nil {
			t.Fatalf("response %s returned error", line)
		}
		responses[resp.ID] = resp.Result
	}
	return responses
}

func decodeToolsFromResult(t *testing.T, result map[string]any) []mcp.Tool {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Tools []mcp.Tool `json:"tools"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded.Tools
}

func namesFromPromptResult(t *testing.T, result map[string]any) []string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Prompts []mcp.Prompt `json:"prompts"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(decoded.Prompts))
	for _, prompt := range decoded.Prompts {
		out = append(out, prompt.Name)
	}
	return out
}

func urisFromResourceResult(t *testing.T, result map[string]any) []string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Resources []mcp.Resource `json:"resources"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(decoded.Resources))
	for _, resource := range decoded.Resources {
		out = append(out, resource.URI)
	}
	return out
}

func uriTemplatesFromResourceTemplateResult(t *testing.T, result map[string]any) []string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		ResourceTemplates []mcp.ResourceTemplate `json:"resourceTemplates"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(decoded.ResourceTemplates))
	for _, tmpl := range decoded.ResourceTemplates {
		out = append(out, tmpl.URITemplate)
	}
	return out
}

func assertGoldenJSON(t *testing.T, name string, got any) {
	t.Helper()
	raw, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	path := filepath.Join("testdata", name)
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	if string(raw) != string(want) {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, raw, want)
	}
}

func initializeServer(t *testing.T, server *mcp.Server) {
	t.Helper()
	if _, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`)); err != nil {
		t.Fatal(err)
	}
}

func callToolOK(t *testing.T, server *mcp.Server, name string, args map[string]any) ToolResult {
	t.Helper()
	raw := callToolRaw(t, server, name, args)
	return decodeToolResult(t, name, raw)
}

func decodeToolResult(t *testing.T, name string, raw []byte) ToolResult {
	t.Helper()
	var resp struct {
		Result struct {
			StructuredContent ToolResult `json:"structuredContent"`
		} `json:"result"`
		Error any `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil || !resp.Result.StructuredContent.OK {
		t.Fatalf("tool %s did not return ok result: %s", name, raw)
	}
	if len(resp.Result.StructuredContent.IDs) == 0 {
		t.Fatalf("write tool %s returned no IDs: %+v", name, resp.Result.StructuredContent)
	}
	return resp.Result.StructuredContent
}

func callToolError(t *testing.T, server *mcp.Server, name string, args map[string]any) ToolError {
	t.Helper()
	raw := callToolRaw(t, server, name, args)
	var resp struct {
		Result struct {
			StructuredContent ToolError `json:"structuredContent"`
		} `json:"result"`
		Error any `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil || resp.Result.StructuredContent.OK {
		t.Fatalf("tool %s did not return error envelope: %s", name, raw)
	}
	return resp.Result.StructuredContent
}

func callToolRaw(t *testing.T, server *mcp.Server, name string, args map[string]any) []byte {
	t.Helper()
	if args == nil {
		args = map[string]any{}
	}
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": args,
		},
	}
	rawReq, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := server.DispatchMessage(context.Background(), rawReq)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func requireChanged(t *testing.T, result ToolResult, kind string) {
	t.Helper()
	var refs []EntityRef
	switch kind {
	case "created":
		refs = result.Changed.Created
	case "updated":
		refs = result.Changed.Updated
	case "deleted":
		refs = result.Changed.Deleted
	case "reused":
		refs = result.Changed.Reused
	default:
		t.Fatalf("unknown change kind %q", kind)
	}
	if len(refs) == 0 || refs[0].ID == "" {
		t.Fatalf("missing changed.%s with ID: %+v", kind, result.Changed)
	}
}

func requireNext(t *testing.T, result ToolResult) {
	t.Helper()
	if len(result.Next) == 0 || result.Next[0].Tool == "" {
		t.Fatalf("missing useful next actions: %+v", result.Next)
	}
}

func hasAnyChange(changed ChangeSet) bool {
	return len(changed.Created)+len(changed.Updated)+len(changed.Deleted)+len(changed.Reused) > 0
}

func oneUserCoverageArgs(tool mcp.Tool) map[string]any {
	props, _ := tool.InputSchema["properties"].(map[string]any)
	args := make(map[string]any, len(props))
	for name, schema := range props {
		prop, _ := schema.(map[string]any)
		args[name] = oneUserCoverageValue(name, prop)
	}
	switch tool.Name {
	case "clockify_reports_weekly":
		delete(args, "start")
		delete(args, "end")
		args["week_start"] = "2026-01-05"
	case "clockify_entries_mark_invoiced":
		args["time_entry_ids"] = []any{oneUserCoverageID("entry_id")}
		args["invoiced"] = true
	case "clockify_users_invite":
		args["emails"] = []any{"coverage@example.com"}
	}
	return args
}

func oneUserCoverageValue(name string, schema map[string]any) any {
	if enum, ok := schema["enum"].([]string); ok && len(enum) > 0 {
		return enum[0]
	}
	if enum, ok := schema["enum"].([]any); ok && len(enum) > 0 {
		return enum[0]
	}
	switch name {
	case "body":
		return map[string]any{"name": "Coverage object", "id": "65b382b606de527a7ee2b61f"}
	case "url":
		return "https://example.com/clockify"
	case "event":
		return "NEW_TIME_ENTRY"
	case "events":
		return []any{"NEW_TIME_ENTRY"}
	case "rate_kind":
		return "hourly"
	case "color":
		return "#0088cc"
	case "currency":
		return "USD"
	case "email":
		return "coverage@example.com"
	case "emails":
		return []any{"coverage@example.com"}
	case "memberships":
		return []any{map[string]any{"userId": oneUserCoverageID("user_id"), "role": "MANAGER"}}
	case "client", "project", "task", "tag", "user", "entry", "invoice", "expense", "policy", "request", "assignment", "webhook", "group", "holiday":
		return oneUserCoverageID(name + "_id")
	case "date":
		return "2026-01-02"
	case "start", "start_time":
		return "2026-01-02T09:00:00Z"
	case "end", "end_time":
		return "2026-01-02T10:00:00Z"
	case "issued_date":
		return "2026-01-02T00:00:00Z"
	case "due_date":
		return "2026-01-31T00:00:00Z"
	case "week_start":
		return "2026-01-05"
	case "timezone":
		return "UTC"
	case "dry_run":
		return false
	case "change_fields":
		return []any{"DATE", "PROJECT", "CATEGORY", "NOTES", "AMOUNT", "BILLABLE"}
	}
	if strings.HasSuffix(name, "_ids") || strings.HasSuffix(name, "Ids") {
		return []any{oneUserCoverageID(name)}
	}
	if strings.HasSuffix(name, "_id") || strings.HasSuffix(name, "Id") || name == "id" {
		return oneUserCoverageID(name)
	}
	switch schema["type"] {
	case "integer":
		return 1
	case "number":
		return float64(1)
	case "boolean":
		return false
	case "array":
		return []any{"Coverage"}
	case "object":
		return map[string]any{"name": "Coverage object"}
	default:
		return "Coverage " + name
	}
}

func oneUserCoverageID(name string) string {
	ids := map[string]string{
		"approval_id":     "65b382b606de527a7ee2b610",
		"assignment_id":   "65b382b606de527a7ee2b611",
		"category_id":     "65b382b606de527a7ee2b612",
		"client_id":       "65b382b606de527a7ee2b613",
		"custom_field_id": "65b382b606de527a7ee2b614",
		"entry_id":        "65b382b606de527a7ee2b615",
		"expense_id":      "65b382b606de527a7ee2b616",
		"group_id":        "65b382b606de527a7ee2b617",
		"holiday_id":      "65b382b606de527a7ee2b618",
		"invoice_id":      "65b382b606de527a7ee2b619",
		"invoice_item_id": "65b382b606de527a7ee2b61a",
		"payment_id":      "65b382b606de527a7ee2b61b",
		"policy_id":       "65b382b606de527a7ee2b61c",
		"project_id":      "65b382b606de527a7ee2b61d",
		"request_id":      "65b382b606de527a7ee2b61e",
		"task_id":         "65b382b606de527a7ee2b620",
		"tag_id":          "65b382b606de527a7ee2b621",
		"user_id":         "65b382b606de527a7ee2b622",
		"webhook_id":      "65b382b606de527a7ee2b623",
		"workspace_id":    "65b382b606de527a7ee2b60e",
	}
	key := strings.ToLower(strings.TrimSuffix(name, "s"))
	if id, ok := ids[key]; ok {
		return id
	}
	return "65b382b606de527a7ee2b61f"
}

func newOneUserCoverageUpstream() *httptest.Server {
	const workspaceID = "65b382b606de527a7ee2b60e"
	const userID = "65b382b606de527a7ee2b624"
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/export") || strings.Contains(r.URL.RawQuery, "export") {
			fmt.Fprint(w, `{"content":"ZmFrZQ==","contentType":"application/pdf","headers":{}}`)
			return
		}
		_ = json.NewEncoder(w).Encode(oneUserCoveragePayload(workspaceID, userID, r.Method, r.URL.Path))
	}))
}

func oneUserCoveragePayload(workspaceID, userID, method, path string) any {
	user := map[string]any{
		"id":                 userID,
		"name":               "Coverage User",
		"email":              "coverage@example.com",
		"activeWorkspace":    workspaceID,
		"defaultWorkspace":   workspaceID,
		"activeWorkspaceId":  workspaceID,
		"defaultWorkspaceId": workspaceID,
	}
	workspace := map[string]any{
		"id":                      workspaceID,
		"name":                    "Coverage Workspace",
		"workspaceSettings":       map[string]any{"weekStart": "MONDAY", "features": []any{"invoicing", "expenses"}},
		"features":                []any{"invoicing", "expenses", "timeOff", "scheduling"},
		"subscriptionType":        "PRO",
		"featureSubscription":     "PRO",
		"featureSubscriptionType": "PRO",
	}
	entity := oneUserCoverageEntity(workspaceID, userID, path)
	switch {
	case path == "/user":
		return user
	case path == "/workspaces":
		return []any{workspace}
	case method == http.MethodGet && strings.HasSuffix(path, "/users"):
		return []any{user}
	case method == http.MethodGet && strings.Contains(path, "/webhooks"):
		return map[string]any{"workspaceWebhookCount": 1, "webhooks": []any{entity}}
	case method == http.MethodGet && strings.HasSuffix(path, "/invoices"):
		return map[string]any{"total": 1, "invoices": []any{entity}}
	case method == http.MethodGet && strings.HasSuffix(path, "/expenses"):
		return map[string]any{"expenses": map[string]any{"expenses": []any{entity}, "count": 1}}
	case method == http.MethodGet && strings.Contains(path, "/custom-fields"):
		return []any{entity}
	case strings.Contains(path, "/time-off/balance"):
		return map[string]any{"count": 1, "balances": []any{entity}}
	case method == http.MethodPost && strings.HasSuffix(path, "/time-off/requests"):
		return map[string]any{"count": 1, "requests": []any{entity}}
	case method == http.MethodGet && strings.HasSuffix(path, "/time-off/policies"):
		return []any{entity}
	case method == http.MethodGet && strings.Contains(path, "/time-off/policies/"):
		return entity
	case method == http.MethodGet && (strings.Contains(path, "/groups") || strings.Contains(path, "/user-groups")):
		return []any{entity}
	case method == http.MethodGet && oneUserCoverageListPath(path):
		return []any{entity}
	case strings.Contains(path, "/scheduling/"):
		return []any{entity}
	case method == http.MethodGet && (strings.Contains(path, "/approval") || strings.Contains(path, "/holidays/")):
		return []any{entity}
	case strings.Contains(path, "/reports/") || strings.Contains(path, "/report"):
		return map[string]any{"totals": []any{entity}, "timeentries": []any{entity}, "entries": []any{entity}, "data": []any{entity}, "id": entity["id"], "workspaceId": workspaceID}
	default:
		return entity
	}
}

func oneUserCoverageListPath(path string) bool {
	for _, suffix := range []string{"/clients", "/projects", "/tasks", "/tags", "/time-entries", "/groups", "/holidays", "/assignments", "/approvals", "/custom-fields"} {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

func oneUserCoverageEntity(workspaceID, userID, path string) map[string]any {
	id := oneUserCoverageEntityID(path)
	status := "ACTIVE"
	if strings.Contains(path, "/custom-fields") {
		status = "VISIBLE"
	}
	return map[string]any{
		"id":            id,
		"_id":           id,
		"name":          "Coverage object",
		"workspaceId":   workspaceID,
		"userId":        userID,
		"clientId":      "65b382b606de527a7ee2b613",
		"projectId":     "65b382b606de527a7ee2b61d",
		"taskId":        "65b382b606de527a7ee2b620",
		"tagId":         "65b382b606de527a7ee2b621",
		"invoiceId":     "65b382b606de527a7ee2b619",
		"expenseId":     "65b382b606de527a7ee2b616",
		"policyId":      "65b382b606de527a7ee2b61c",
		"requestId":     "65b382b606de527a7ee2b61e",
		"assignmentId":  "65b382b606de527a7ee2b611",
		"webhookId":     "65b382b606de527a7ee2b623",
		"status":        status,
		"number":        "INV-COVERAGE",
		"url":           "https://example.com/clockify",
		"events":        []any{"NEW_TIME_ENTRY"},
		"description":   "Coverage entry",
		"amount":        1,
		"date":          "2026-01-02",
		"start":         "2026-01-02T09:00:00Z",
		"end":           "2026-01-02T10:00:00Z",
		"timeInterval":  map[string]any{"start": "2026-01-02T09:00:00Z", "end": "2026-01-02T10:00:00Z"},
		"billable":      true,
		"hourlyRate":    map[string]any{"amount": 100, "currency": "USD"},
		"costRate":      map[string]any{"amount": 50, "currency": "USD"},
		"pathUnderTest": path,
	}
}

func oneUserCoverageEntityID(path string) string {
	switch {
	case strings.Contains(path, "approval"):
		return oneUserCoverageID("approval_id")
	case strings.Contains(path, "assignments") || strings.Contains(path, "scheduling"):
		return oneUserCoverageID("assignment_id")
	case strings.Contains(path, "clients"):
		return oneUserCoverageID("client_id")
	case strings.Contains(path, "projects"):
		return oneUserCoverageID("project_id")
	case strings.Contains(path, "tasks"):
		return oneUserCoverageID("task_id")
	case strings.Contains(path, "tags"):
		return oneUserCoverageID("tag_id")
	case strings.Contains(path, "time-entries"):
		return oneUserCoverageID("entry_id")
	case strings.Contains(path, "invoices"):
		return oneUserCoverageID("invoice_id")
	case strings.Contains(path, "expenses"):
		return oneUserCoverageID("expense_id")
	case strings.Contains(path, "time-off"):
		return oneUserCoverageID("request_id")
	case strings.Contains(path, "webhooks"):
		return oneUserCoverageID("webhook_id")
	case strings.Contains(path, "groups") || strings.Contains(path, "user-groups"):
		return oneUserCoverageID("group_id")
	case strings.Contains(path, "holidays"):
		return oneUserCoverageID("holiday_id")
	default:
		return "65b382b606de527a7ee2b61f"
	}
}

func blockedTerm(parts ...string) string {
	return strings.Join(parts, "")
}
