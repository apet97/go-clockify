package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	})

	initialize := responses[1]
	assertGoldenJSON(t, "oneuser_initialize.golden.json", initialize)
	initializeText := mustJSONText(t, initialize)
	for _, forbidden := range []string{"confirmation", "activation", "activate", "tenant", "hosted", "Tier 2", "policy mode"} {
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
	for _, forbidden := range []string{"clockify_activate_group", "clockify_activate_tool", "clockify_deactivate_group", "clockify_search_tools", "clockify_list_tools"} {
		if strings.Contains(toolGolden, forbidden) {
			t.Fatalf("tools/list golden contains old tool %s", forbidden)
		}
	}

	promptsResult := responses[3]
	assertGoldenJSON(t, "oneuser_prompts_list.golden.json", namesFromPromptResult(t, promptsResult))

	resourcesResult := responses[4]
	assertGoldenJSON(t, "oneuser_resources_list.golden.json", urisFromResourceResult(t, resourcesResult))
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
	if feature.Error.Code != "feature_unavailable" || feature.Recovery.Hint == "" {
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

func dispatchOneUserResult(t *testing.T, server *mcp.Server, method string, params any) map[string]any {
	t.Helper()
	req := map[string]any{"jsonrpc": "2.0", "id": 1, "method": method}
	if params != nil {
		req["params"] = params
	}
	rawReq, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := server.DispatchMessage(context.Background(), rawReq)
	if err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Result map[string]any `json:"result"`
		Error  any            `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil {
		t.Fatalf("%s returned error: %s", method, raw)
	}
	return resp.Result
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
