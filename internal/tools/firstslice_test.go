package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/testclockify"
)

var firstSliceTools = []string{
	"clockify_status",
	"clockify_clients_list",
	"clockify_clients_create",
	"clockify_projects_list",
	"clockify_projects_create",
	"clockify_tasks_list",
	"clockify_tasks_create",
	"clockify_tags_list",
	"clockify_tags_create",
	"clockify_entries_list",
	"clockify_entries_create",
	"clockify_demo_seed",
	"clockify_demo_cleanup",
}

func TestFirstSliceRegistryContainsOnlyPhase1Tools(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	reg := svc.FirstSliceRegistry()
	got := map[string]bool{}
	for _, d := range reg {
		got[d.Tool.Name] = true
	}
	for _, want := range firstSliceTools {
		if !got[want] {
			t.Fatalf("missing first-slice tool %q from registry", want)
		}
	}
	if len(got) != len(firstSliceTools) {
		t.Fatalf("registry count = %d, want %d: %#v", len(got), len(firstSliceTools), got)
	}
	for _, forbidden := range []string{
		blockedTerm("clockify_", "activate_group"),
		blockedTerm("clockify_", "activate_tool"),
		blockedTerm("clockify_", "deactivate_group"),
		blockedTerm("clockify_", "search_tools"),
		blockedTerm("clockify_", "list_tools"),
	} {
		if got[forbidden] {
			t.Fatalf("old product tool exposed: %s", forbidden)
		}
	}
}

func TestFirstSliceInitializeAndToolsList(t *testing.T) {
	fake := testclockify.NewServer("65b382b606de527a7ee2b60e")
	defer fake.Close()
	svc := New(clockify.NewClient("test-key", fake.URL, time.Second, 0), fake.WorkspaceID)
	server := mcp.NewServer("test", svc.FirstSliceRegistry(), nil, nil)

	raw, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{"single-user full-access", "All tools are loaded", "Use workflow tools first"} {
		if !strings.Contains(text, want) {
			t.Fatalf("initialize missing %q: %s", want, text)
		}
	}
	for _, forbidden := range []string{"confirmation", "policy", blockedTerm("Tier ", "2"), blockedTerm("ten", "ant"), blockedTerm("hos", "ted"), "activate"} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(forbidden)) {
			t.Fatalf("initialize contains forbidden language %q: %s", forbidden, text)
		}
	}

	raw, err = server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
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
	names := map[string]bool{}
	for _, tool := range resp.Result.Tools {
		names[tool.Name] = true
	}
	for _, want := range firstSliceTools {
		if !names[want] {
			t.Fatalf("tools/list missing %s; got %#v", want, names)
		}
	}
	if resp.Result.Tools[0].Name != "clockify_status" {
		t.Fatalf("first listed tool = %q, want clockify_status", resp.Result.Tools[0].Name)
	}
}

func TestFirstSliceStatusDemoSeedAndCleanup(t *testing.T) {
	fake := testclockify.NewServer("65b382b606de527a7ee2b60e")
	defer fake.Close()
	svc := New(clockify.NewClient("test-key", fake.URL, time.Second, 0), fake.WorkspaceID)
	svc.DefaultTimezone = time.UTC

	status, err := svc.ClockifyStatus(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	statusResult := status.(ToolResult)
	if !statusResult.OK || statusResult.IDs["workspaceId"] != fake.WorkspaceID || statusResult.IDs["userId"] == "" {
		t.Fatalf("bad status result: %+v", statusResult)
	}

	seed, err := svc.ClockifyDemoSeed(context.Background(), map[string]any{"run_id": "test"})
	if err != nil {
		t.Fatal(err)
	}
	seedResult := seed.(ToolResult)
	for _, key := range []string{"clientId", "projectId", "taskId", "tagId", "entryId"} {
		if seedResult.IDs[key] == "" {
			t.Fatalf("seed missing id %s: %+v", key, seedResult)
		}
	}
	if len(seedResult.Changed.Created) != 5 {
		t.Fatalf("seed created = %d, want 5: %+v", len(seedResult.Changed.Created), seedResult.Changed)
	}

	seedAgain, err := svc.ClockifyDemoSeed(context.Background(), map[string]any{"run_id": "test"})
	if err != nil {
		t.Fatal(err)
	}
	reused := seedAgain.(ToolResult).Changed.Reused
	if len(reused) != 5 {
		t.Fatalf("seed idempotency reused = %d, want 5: %+v", len(reused), reused)
	}

	cleanup, err := svc.ClockifyDemoCleanup(context.Background(), map[string]any{"run_id": "test"})
	if err != nil {
		t.Fatal(err)
	}
	cleanupResult := cleanup.(ToolResult)
	if len(cleanupResult.Changed.Deleted) != 5 {
		t.Fatalf("cleanup deleted = %d, want 5: %+v", len(cleanupResult.Changed.Deleted), cleanupResult)
	}
}

func TestFirstSliceWritesReturnIDsAndChanges(t *testing.T) {
	fake := testclockify.NewServer("65b382b606de527a7ee2b60e")
	defer fake.Close()
	svc := New(clockify.NewClient("test-key", fake.URL, time.Second, 0), fake.WorkspaceID)
	svc.DefaultTimezone = time.UTC

	clientOut, err := svc.ClientsCreate(context.Background(), map[string]any{"name": "Phase 1 Client"})
	client := mustToolResult(t, clientOut, err)
	requireID(t, client, "clientId")
	requireCreated(t, client, "client")

	projectOut, err := svc.ProjectsCreate(context.Background(), map[string]any{
		"name":      "Phase 1 Project",
		"client_id": client.IDs["clientId"],
	})
	project := mustToolResult(t, projectOut, err)
	requireID(t, project, "projectId")
	requireCreated(t, project, "project")

	taskOut, err := svc.TasksCreate(context.Background(), map[string]any{
		"name":       "Phase 1 Task",
		"project_id": project.IDs["projectId"],
	})
	task := mustToolResult(t, taskOut, err)
	requireID(t, task, "taskId")
	requireCreated(t, task, "task")

	tagOut, err := svc.TagsCreate(context.Background(), map[string]any{"name": "Phase 1 Tag"})
	tag := mustToolResult(t, tagOut, err)
	requireID(t, tag, "tagId")
	requireCreated(t, tag, "tag")

	entryOut, err := svc.EntriesCreate(context.Background(), map[string]any{
		"start":       "2026-01-03 09:00",
		"end":         "2026-01-03 10:00",
		"description": "Phase 1 entry",
		"project_id":  project.IDs["projectId"],
		"task_id":     task.IDs["taskId"],
		"tag_ids":     []any{tag.IDs["tagId"]},
	})
	entry := mustToolResult(t, entryOut, err)
	requireID(t, entry, "entryId")
	requireCreated(t, entry, "entry")
}

func TestFirstSliceRecoverableErrorEnvelope(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	server := mcp.NewServer("test", svc.FirstSliceRegistry(), nil, nil)
	if _, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)); err != nil {
		t.Fatal(err)
	}
	raw, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"clockify_clients_create","arguments":{}}}`))
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
		t.Fatalf("expected ok=false envelope: %s", raw)
	}
	if resp.Result.StructuredContent.Recovery.Hint == "" {
		t.Fatalf("missing recovery hint: %+v", resp.Result.StructuredContent)
	}
}

func mustToolResult(t *testing.T, out any, err error) ToolResult {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	result, ok := out.(ToolResult)
	if !ok {
		t.Fatalf("result type = %T, want ToolResult", out)
	}
	if !result.OK {
		t.Fatalf("result not ok: %+v", result)
	}
	return result
}

func requireID(t *testing.T, result ToolResult, key string) {
	t.Helper()
	if result.IDs[key] == "" {
		t.Fatalf("%s missing from IDs: %+v", key, result.IDs)
	}
}

func requireCreated(t *testing.T, result ToolResult, entity string) {
	t.Helper()
	for _, ref := range result.Changed.Created {
		if ref.Type == entity && ref.ID != "" {
			return
		}
	}
	t.Fatalf("created %s missing from change set: %+v", entity, result.Changed)
}
