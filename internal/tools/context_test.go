package tools

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestTimerStatusRunning(t *testing.T) {
	now := time.Now().UTC().Add(-2*time.Hour - 14*time.Minute)
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			if r.URL.Query().Get("page-size") != "1" {
				t.Fatalf("expected page-size=1, got %s", r.URL.Query().Get("page-size"))
			}
			respondJSON(t, w, []clockify.TimeEntry{
				{ID: "e1", Description: "Working", TimeInterval: clockify.TimeInterval{Start: now.Format(time.RFC3339)}},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.TimerStatus(context.Background())
	if err != nil {
		t.Fatalf("timer status failed: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["running"] != true {
		t.Fatalf("expected running=true, got %v", data["running"])
	}
	if data["entry"] == nil {
		t.Fatal("expected entry to be non-nil")
	}
	elapsed, _ := data["elapsed"].(string)
	if elapsed == "" {
		t.Fatal("expected non-empty elapsed string")
	}
}

func TestTimerStatusNotRunning(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			respondJSON(t, w, []clockify.TimeEntry{
				{ID: "e1", Description: "Done", TimeInterval: clockify.TimeInterval{Start: "2026-04-01T09:00:00Z", End: "2026-04-01T11:00:00Z"}},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.TimerStatus(context.Background())
	if err != nil {
		t.Fatalf("timer status failed: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["running"] != false {
		t.Fatalf("expected running=false, got %v", data["running"])
	}
	if data["entry"] != nil {
		t.Fatalf("expected entry to be nil, got %v", data["entry"])
	}
}

func TestSwitchProject(t *testing.T) {
	callCount := map[string]int{}
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		callCount[key]++
		switch {
		case r.URL.Path == "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case r.Method == http.MethodPatch && r.URL.Path == "/workspaces/ws1/user/u1/time-entries":
			// Stop timer
			respondJSON(t, w, clockify.TimeEntry{ID: "stopped1", Description: "Old task", TimeInterval: clockify.TimeInterval{Start: "2026-04-01T09:00:00Z", End: "2026-04-01T11:00:00Z"}})
		case r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/projects":
			respondJSON(t, w, []map[string]any{
				{"id": "p2", "name": "New Project"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/workspaces/ws1/time-entries":
			// Start timer
			respondJSON(t, w, clockify.TimeEntry{ID: "started1", Description: "Switched", ProjectID: "p2", TimeInterval: clockify.TimeInterval{Start: "2026-04-01T11:00:00Z"}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.SwitchProject(context.Background(), map[string]any{
		"project":     "New Project",
		"description": "Switched",
	})
	if err != nil {
		t.Fatalf("switch project failed: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["stopped"] == nil {
		t.Fatal("expected stopped to be non-nil")
	}
	if data["stop_outcome"] != "stopped" {
		t.Fatalf("expected stop_outcome=stopped, got %v", data["stop_outcome"])
	}
	if data["started"] == nil {
		t.Fatal("expected started to be non-nil")
	}
}

func TestSwitchProjectNoRunningTimerOutcome(t *testing.T) {
	callCount := map[string]int{}
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		callCount[key]++
		switch {
		case r.URL.Path == "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case r.Method == http.MethodPatch && r.URL.Path == "/workspaces/ws1/user/u1/time-entries":
			http.Error(w, `{"message":"no running timer"}`, http.StatusNotFound)
		case r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/projects":
			respondJSON(t, w, []map[string]any{
				{"id": "p2", "name": "New Project"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/workspaces/ws1/time-entries":
			respondJSON(t, w, clockify.TimeEntry{ID: "started1", Description: "Switched", ProjectID: "p2", TimeInterval: clockify.TimeInterval{Start: "2026-04-01T11:00:00Z"}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.SwitchProject(context.Background(), map[string]any{
		"project":     "New Project",
		"description": "Switched",
	})
	if err != nil {
		t.Fatalf("switch project failed: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["stop_outcome"] != "no_running_timer" {
		t.Fatalf("expected stop_outcome=no_running_timer, got %v", data["stop_outcome"])
	}
	if data["stopped"] != nil {
		t.Fatalf("expected stopped=nil when no timer was running, got %v", data["stopped"])
	}
	if data["started"] == nil {
		t.Fatal("expected started to be non-nil")
	}
	if got := callCount["POST /workspaces/ws1/time-entries"]; got != 1 {
		t.Fatalf("expected timer start after no-running stop, got %d start calls", got)
	}
}

func TestResolveDebugExactMatch(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workspaces/ws1/projects":
			respondJSON(t, w, []map[string]any{
				{"id": "p1", "name": "Alpha"},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.ResolveDebug(context.Background(), map[string]any{
		"entity_type": "project",
		"name_or_id":  "Alpha",
	})
	if err != nil {
		t.Fatalf("resolve debug failed: %v", err)
	}
	if result.Action != "clockify_resolve_debug" {
		t.Fatalf("expected resolve_debug action, got %q", result.Action)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["status"] != "exact_match" {
		t.Fatalf("expected exact_match, got %v", data["status"])
	}
	if data["resolved_id"] != "p1" {
		t.Fatalf("expected resolved_id=p1, got %v", data["resolved_id"])
	}
	if data["error"] != "" {
		t.Fatalf("expected empty error, got %v", data["error"])
	}
}

func TestResolveNameAliasExactMatch(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workspaces/ws1/projects":
			respondJSON(t, w, []map[string]any{
				{"id": "p1", "name": "Alpha"},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.ResolveName(context.Background(), map[string]any{
		"entity_type": "project",
		"name_or_id":  "Alpha",
	})
	if err != nil {
		t.Fatalf("resolve name failed: %v", err)
	}
	if result.Action != "clockify_resolve_name" {
		t.Fatalf("expected resolve_name action, got %q", result.Action)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["status"] != "exact_match" {
		t.Fatalf("expected exact_match, got %v", data["status"])
	}
	if data["resolved_id"] != "p1" {
		t.Fatalf("expected resolved_id=p1, got %v", data["resolved_id"])
	}
}

func TestResolveNameMultipleMatchesReturnsCandidates(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workspaces/ws1/projects":
			if got := r.URL.Query().Get("strict-name-search"); got != "true" {
				t.Fatalf("expected strict-name-search=true, got %q", got)
			}
			respondJSON(t, w, []map[string]any{
				{"id": "p1", "name": "Alpha"},
				{"id": "p2", "name": "Alpha"},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.ResolveName(context.Background(), map[string]any{
		"entity_type": "project",
		"name_or_id":  "Alpha",
	})
	if err != nil {
		t.Fatalf("resolve name failed: %v", err)
	}
	if result.Action != "clockify_resolve_name" {
		t.Fatalf("expected resolve_name action, got %q", result.Action)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["status"] != "multiple_matches" {
		t.Fatalf("expected multiple_matches, got %v", data["status"])
	}
	candidates, ok := data["candidates"].([]map[string]any)
	if !ok {
		t.Fatalf("unexpected candidates type: %T", data["candidates"])
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d: %+v", len(candidates), candidates)
	}
	if candidates[0]["id"] != "p1" || candidates[1]["id"] != "p2" {
		t.Fatalf("unexpected candidate IDs: %+v", candidates)
	}
}

func TestSearchToolsByQuery(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	result, err := svc.SearchTools(context.Background(), map[string]any{
		"query": "timer",
	})
	if err != nil {
		t.Fatalf("search tools failed: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	count, _ := data["count"].(int)
	if count == 0 {
		t.Fatal("expected at least one result for 'timer' query")
	}
	byDomain, ok := data["by_domain"].(map[string][]map[string]any)
	if !ok {
		t.Fatalf("unexpected by_domain type: %T", data["by_domain"])
	}
	if len(byDomain["timer"]) == 0 {
		t.Fatal("expected timer-domain results")
	}
}

func TestSearchToolsIncludesTier2Groups(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	result, err := svc.SearchTools(context.Background(), map[string]any{
		"query": "invoice",
	})
	if err != nil {
		t.Fatalf("search tools failed: %v", err)
	}
	data := result.Data.(map[string]any)
	allResults, ok := data["all_results"].([]map[string]any)
	if !ok {
		t.Fatalf("unexpected all_results type: %T", data["all_results"])
	}
	foundGroup := false
	for _, entry := range allResults {
		if entry["type"] == "group" && entry["name"] == "invoices" {
			foundGroup = true
			break
		}
	}
	if !foundGroup {
		t.Fatal("expected invoices tier2 group in search results")
	}
	if result.Meta == nil || result.Meta["deprecated"] != true || result.Meta["replacement"] != "clockify_list_tools" {
		t.Fatalf("search_tools shim should report deprecation metadata, got %+v", result.Meta)
	}
}

func TestListToolsReportsGroupActivationStatus(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	svc.GroupActivation = func(group string) (bool, string) {
		if group == "invoices" {
			return false, "blocked by time_tracking_safe"
		}
		return true, ""
	}
	result, err := svc.ListTools(context.Background(), map[string]any{"query": "invoice"})
	if err != nil {
		t.Fatalf("list tools failed: %v", err)
	}
	data := result.Data.(map[string]any)
	allResults := data["all_results"].([]map[string]any)
	for _, entry := range allResults {
		if entry["type"] == "group" && entry["name"] == "invoices" {
			if entry["activatable"] != false || entry["block_reason"] != "blocked by time_tracking_safe" {
				t.Fatalf("unexpected activation status: %+v", entry)
			}
			return
		}
	}
	t.Fatal("expected invoices group in list results")
}

func TestSearchToolsActivateGroup(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	svc.ActivateGroup = func(_ context.Context, name string) (ActivationResult, error) {
		if name != "invoices" {
			return ActivationResult{}, fmt.Errorf("unexpected group %q", name)
		}
		return ActivationResult{Kind: "group", Name: name, Group: name, ToolCount: 12, TotalVisibleTools: 47}, nil
	}

	result, err := svc.SearchTools(context.Background(), map[string]any{
		"activate_group": "invoices",
	})
	if err != nil {
		t.Fatalf("activate group failed: %v", err)
	}
	data := result.Data.(map[string]any)
	if data["activated"] != "invoices" {
		t.Fatalf("expected invoices activation, got %v", data["activated"])
	}
	if data["tool_count"] != 12 {
		t.Fatalf("expected tool_count=12, got %v", data["tool_count"])
	}
	if data["total_visible_tools"] != 47 {
		t.Fatalf("expected total_visible_tools=47, got %v", data["total_visible_tools"])
	}
	if msg, _ := data["activation_message"].(string); !strings.Contains(msg, "12 tools in group") || !strings.Contains(msg, "47 total visible tools") {
		t.Fatalf("activation_message should distinguish group and total counts, got %q", msg)
	}
	if result.Meta == nil || result.Meta["replacement"] != "clockify_activate_group" {
		t.Fatalf("search_tools activation shim should point at activate_group, got %+v", result.Meta)
	}
}

func TestActivateGroupTool(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	svc.ActivateGroup = func(_ context.Context, name string) (ActivationResult, error) {
		return ActivationResult{Kind: "group", Name: name, Group: name, ToolCount: 1, ActivatedTools: []string{"clockify_list_invoices"}}, nil
	}
	result, err := svc.ActivateToolGroup(context.Background(), map[string]any{"name": "invoices"})
	if err != nil {
		t.Fatalf("activate group failed: %v", err)
	}
	if result.Action != "clockify_activate_group" {
		t.Fatalf("Action=%q, want clockify_activate_group", result.Action)
	}
	data := result.Data.(map[string]any)
	assertStringSlice(t, data["activated_tools"], []string{"clockify_list_invoices"})
}

func TestSearchToolsActivateTool(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	svc.ActivateTool = func(_ context.Context, name string) (ActivationResult, error) {
		if name != "clockify_send_invoice" {
			return ActivationResult{}, fmt.Errorf("unexpected tool %q", name)
		}
		return ActivationResult{Kind: "tool", Name: name, Group: "invoices", ToolCount: 12}, nil
	}

	result, err := svc.SearchTools(context.Background(), map[string]any{
		"activate_tool": "clockify_send_invoice",
	})
	if err != nil {
		t.Fatalf("activate tool failed: %v", err)
	}
	data := result.Data.(map[string]any)
	if data["activated"] != "clockify_send_invoice" {
		t.Fatalf("expected send_invoice activation, got %v", data["activated"])
	}
	if data["group"] != "invoices" {
		t.Fatalf("expected group=invoices, got %v", data["group"])
	}
}

func TestDeactivateGroupTool(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	svc.DeactivateGroup = func(_ context.Context, name string) (DeactivationResult, error) {
		return DeactivationResult{
			Kind:              "group",
			Name:              name,
			Group:             name,
			ToolCount:         2,
			DeactivatedTools:  []string{"clockify_list_invoices", "clockify_get_invoice"},
			TotalVisibleTools: 35,
		}, nil
	}
	result, err := svc.DeactivateToolGroup(context.Background(), map[string]any{"name": "invoices"})
	if err != nil {
		t.Fatalf("deactivate group failed: %v", err)
	}
	if result.Action != "clockify_deactivate_group" {
		t.Fatalf("Action=%q, want clockify_deactivate_group", result.Action)
	}
	data := result.Data.(map[string]any)
	assertStringSlice(t, data["deactivated_tools"], []string{"clockify_list_invoices", "clockify_get_invoice"})
	if data["total_visible_tools"] != 35 {
		t.Fatalf("total_visible_tools=%v, want 35", data["total_visible_tools"])
	}
}

// TestSearchToolsActivateToolEnumeratesGroup locks in the audit-finding-1
// fix: when an LLM activates a single Tier-2 tool name, the response must
// surface the entire containing group so the LLM sees what other
// capabilities it just gained. The structured activated_tools field is the
// contract; activation_message stays concise to avoid duplicating tokens.
func TestSearchToolsActivateToolEnumeratesGroup(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	siblings := []string{
		"clockify_send_invoice",
		"clockify_mark_invoice_paid",
		"clockify_delete_invoice",
	}
	svc.ActivateTool = func(_ context.Context, name string) (ActivationResult, error) {
		return ActivationResult{
			Kind:              "tool",
			Name:              name,
			Group:             "invoices",
			ToolCount:         len(siblings),
			ActivatedTools:    siblings,
			TotalVisibleTools: 48,
		}, nil
	}

	result, err := svc.SearchTools(context.Background(), map[string]any{
		"activate_tool": "clockify_send_invoice",
	})
	if err != nil {
		t.Fatalf("activate tool failed: %v", err)
	}
	data := result.Data.(map[string]any)
	tools, ok := data["activated_tools"].([]string)
	if !ok {
		t.Fatalf("activated_tools missing or wrong type: %T %v", data["activated_tools"], data["activated_tools"])
	}
	if len(tools) != len(siblings) {
		t.Fatalf("expected %d activated tools, got %d", len(siblings), len(tools))
	}
	msg, _ := data["activation_message"].(string)
	for _, sibling := range siblings {
		if sibling == "clockify_send_invoice" {
			continue
		}
		if strings.Contains(msg, sibling) {
			t.Errorf("activation_message %q should not duplicate activated_tools entry %q", msg, sibling)
		}
	}
	if !strings.Contains(msg, `group "invoices"`) {
		t.Errorf("activation_message %q should identify the activated group", msg)
	}
	if data["total_visible_tools"] != 48 || !strings.Contains(msg, "48 total visible tools") {
		t.Errorf("activation response should include session total, got data=%+v message=%q", data, msg)
	}
}

// TestSearchToolsActivateGroupEnumerates covers the activate_group
// branch — same structured enumeration contract as the tool-name branch above.
func TestSearchToolsActivateGroupEnumerates(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	members := []string{"clockify_create_webhook", "clockify_test_webhook"}
	svc.ActivateGroup = func(_ context.Context, name string) (ActivationResult, error) {
		return ActivationResult{
			Kind:              "group",
			Name:              name,
			Group:             name,
			ToolCount:         len(members),
			ActivatedTools:    members,
			TotalVisibleTools: 39,
		}, nil
	}

	result, err := svc.SearchTools(context.Background(), map[string]any{
		"activate_group": "webhooks",
	})
	if err != nil {
		t.Fatalf("activate group failed: %v", err)
	}
	data := result.Data.(map[string]any)
	tools, ok := data["activated_tools"].([]string)
	if !ok || len(tools) != len(members) {
		t.Fatalf("expected %d activated_tools, got %T %v", len(members), data["activated_tools"], data["activated_tools"])
	}
	msg, _ := data["activation_message"].(string)
	for _, member := range members {
		if strings.Contains(msg, member) {
			t.Errorf("activation_message %q should not duplicate activated_tools entry %q", msg, member)
		}
	}
	if data["total_visible_tools"] != 39 || !strings.Contains(msg, "39 total visible tools") {
		t.Errorf("activation response should include session total, got data=%+v message=%q", data, msg)
	}
}

func TestSearchToolsActivationPayloadReportsOnlyVisibleToolsAsActivated(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	svc.ActivateGroup = func(_ context.Context, name string) (ActivationResult, error) {
		return ActivationResult{
			Kind:                            "group",
			Name:                            name,
			Group:                           name,
			ToolCount:                       3,
			ActivatedTools:                  []string{"clockify_visible_tool", "clockify_hidden_tool", "clockify_blocked_tool"},
			VisibleActivatedTools:           []string{"clockify_visible_tool"},
			ActivatedToolsHiddenByBootstrap: []string{"clockify_hidden_tool"},
			ActivatedToolsBlockedByPolicy:   []string{"clockify_blocked_tool"},
			TotalVisibleTools:               12,
		}, nil
	}

	result, err := svc.SearchTools(context.Background(), map[string]any{
		"activate_group": "test",
	})
	if err != nil {
		t.Fatalf("activate group failed: %v", err)
	}
	data := result.Data.(map[string]any)
	assertStringSlice(t, data["activated_tools"], []string{"clockify_visible_tool"})
	assertStringSlice(t, data["activated_tools_hidden_by_bootstrap"], []string{"clockify_hidden_tool"})
	assertStringSlice(t, data["activated_tools_blocked_by_policy"], []string{"clockify_blocked_tool"})
	if data["tool_count"] != 3 {
		t.Fatalf("tool_count should remain the group-local count, got %v", data["tool_count"])
	}
}

func TestPolicyInfo(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")

	// Without PolicyDescribe set, should return "not available" message.
	result, err := svc.PolicyInfo(context.Background())
	if err != nil {
		t.Fatalf("policy info failed: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["message"] != "policy info not available" {
		t.Fatalf("expected 'policy info not available', got %v", data["message"])
	}

	// With PolicyDescribe set, should return the callback result.
	svc.PolicyDescribe = func() map[string]any {
		return map[string]any{
			"mode":         "standard",
			"denied_tools": []string{},
		}
	}
	result2, err := svc.PolicyInfo(context.Background())
	if err != nil {
		t.Fatalf("policy info with callback failed: %v", err)
	}
	data2, ok := result2.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result2.Data)
	}
	if data2["mode"] != "standard" {
		t.Fatalf("expected mode=standard, got %v", data2["mode"])
	}
}

func assertStringSlice(t *testing.T, raw any, want []string) {
	t.Helper()
	got, ok := raw.([]string)
	if !ok {
		t.Fatalf("expected []string, got %T %v", raw, raw)
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d values, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("value[%d] = %q, want %q; full=%v", i, got[i], want[i], got)
		}
	}
}
