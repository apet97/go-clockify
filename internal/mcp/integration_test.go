package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/apet97/go-clockify/internal/bootstrap"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/truncate"
)

// ---------------------------------------------------------------------------
// 1. Full MCP handshake: initialize -> tools/list -> tools/call -> ping
// ---------------------------------------------------------------------------

func TestFullMCPHandshake(t *testing.T) {
	server := NewServer("test", nil, []ToolDescriptor{
		{
			Tool: Tool{
				Name:        "test_tool",
				Description: "test",
				InputSchema: map[string]any{"type": "object"},
				Annotations: map[string]any{"readOnlyHint": true},
			},
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return map[string]any{"ok": true, "action": "test_tool"}, nil
			},
			ReadOnlyHint: true,
		},
	}, nil, truncate.Config{}, dryrun.Config{}, nil)

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"test_tool","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"ping"}`,
	}, "\n")

	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 responses, got %d: %s", len(lines), out.String())
	}

	responses := make(map[float64]Response)
	for _, line := range lines {
		var r Response
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("unmarshal line: %v", err)
		}
		if id, ok := r.ID.(float64); ok {
			responses[id] = r
		}
	}

	// Verify initialize
	initResp := responses[1]
	if initResp.Error != nil {
		t.Fatalf("initialize error: %v", initResp.Error)
	}
	resultMap, ok := initResp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", initResp.Result)
	}
	if resultMap["protocolVersion"] != "2025-06-18" {
		t.Fatalf("unexpected protocol version: %v", resultMap["protocolVersion"])
	}
	serverInfo, _ := resultMap["serverInfo"].(map[string]any)
	if serverInfo["version"] != "test" {
		t.Fatalf("unexpected server version: %v", serverInfo["version"])
	}

	// Verify tools/list
	listResp := responses[2]
	if listResp.Error != nil {
		t.Fatalf("tools/list error: %v", listResp.Error)
	}
	listResult, _ := listResp.Result.(map[string]any)
	toolsList, _ := listResult["tools"].([]any)
	if len(toolsList) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(toolsList))
	}

	// Verify tools/call
	callResp := responses[3]
	if callResp.Error != nil {
		t.Fatalf("tools/call error: %v", callResp.Error)
	}
	callResult, _ := callResp.Result.(map[string]any)
	content, _ := callResult["content"].([]any)
	if len(content) == 0 {
		t.Fatal("expected content in tools/call response")
	}

	// Verify ping
	pingResp := responses[4]
	if pingResp.Error != nil {
		t.Fatalf("ping error: %v", pingResp.Error)
	}
}

// ---------------------------------------------------------------------------
// 2. Unknown method returns -32601
// ---------------------------------------------------------------------------

func TestUnknownMethodReturnsError(t *testing.T) {
	server := NewServer("test", nil, nil, nil, truncate.Config{}, dryrun.Config{}, nil)
	input := `{"jsonrpc":"2.0","id":1,"method":"bogus/method","params":{}}`

	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	var resp Response
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Fatalf("expected error code -32601, got %d", resp.Error.Code)
	}
}

// ---------------------------------------------------------------------------
// 3. Unknown tool name returns error
// ---------------------------------------------------------------------------

func TestUnknownToolReturnsError(t *testing.T) {
	server := NewServer("test", nil, nil, nil, truncate.Config{}, dryrun.Config{}, nil)
	server.initialized.Store(true) // skip init guard — we're testing tool dispatch
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nonexistent","arguments":{}}}`

	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	var resp Response
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Tool errors now use isError per MCP spec
	resultMap, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	isErr, _ := resultMap["isError"].(bool)
	if !isErr {
		t.Fatal("expected isError=true for unknown tool")
	}
	content, _ := resultMap["content"].([]any)
	if len(content) == 0 {
		t.Fatal("expected content in error response")
	}
	textObj, _ := content[0].(map[string]any)
	text, _ := textObj["text"].(string)
	if !strings.Contains(text, "unknown tool") {
		t.Fatalf("expected 'unknown tool' in error text, got %q", text)
	}
}

// ---------------------------------------------------------------------------
// 3b. tools/call before initialize returns error
// ---------------------------------------------------------------------------

func TestToolCallBeforeInitialize(t *testing.T) {
	server := NewServer("test", nil, nil, nil, truncate.Config{}, dryrun.Config{}, nil)
	// Do NOT set server.initialized — test the guard
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"any","arguments":{}}}`

	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	var resp Response
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error for tools/call before initialize")
	}
	if resp.Error.Code != -32002 {
		t.Fatalf("expected error code -32002, got %d", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "not initialized") {
		t.Fatalf("expected 'not initialized' in error, got %q", resp.Error.Message)
	}
}

// ---------------------------------------------------------------------------
// 4. Invalid JSON returns -32700
// ---------------------------------------------------------------------------

func TestInvalidJSONReturnsParseError(t *testing.T) {
	server := NewServer("test", nil, nil, nil, truncate.Config{}, dryrun.Config{}, nil)
	input := `{not valid json}`

	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	var resp Response
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if resp.Error.Code != -32700 {
		t.Fatalf("expected error code -32700, got %d", resp.Error.Code)
	}
}

// ---------------------------------------------------------------------------
// 5. Policy filtering — read_only mode hides write tools
// ---------------------------------------------------------------------------

func TestPolicyFilteringReadOnly(t *testing.T) {
	pol := &policy.Policy{Mode: policy.ReadOnly, DeniedTools: map[string]bool{}}

	readTool := ToolDescriptor{
		Tool: Tool{
			Name:        "read_tool",
			Description: "r",
			InputSchema: map[string]any{"type": "object"},
			Annotations: map[string]any{"readOnlyHint": true},
		},
		Handler:      func(ctx context.Context, args map[string]any) (any, error) { return nil, nil },
		ReadOnlyHint: true,
	}
	writeTool := ToolDescriptor{
		Tool: Tool{
			Name:        "write_tool",
			Description: "w",
			InputSchema: map[string]any{"type": "object"},
			Annotations: map[string]any{"readOnlyHint": false},
		},
		Handler:      func(ctx context.Context, args map[string]any) (any, error) { return nil, nil },
		ReadOnlyHint: false,
	}

	server := NewServer("test", pol, []ToolDescriptor{readTool, writeTool}, nil, truncate.Config{}, dryrun.Config{}, nil)
	tools := server.listTools()

	if len(tools) != 1 {
		t.Fatalf("expected 1 tool in read_only mode, got %d", len(tools))
	}
	if tools[0].Name != "read_tool" {
		t.Fatalf("expected read_tool, got %s", tools[0].Name)
	}
}

// ---------------------------------------------------------------------------
// 6. Policy filtering — denied tool is blocked
// ---------------------------------------------------------------------------

func TestPolicyDeniedToolIsBlocked(t *testing.T) {
	pol := &policy.Policy{
		Mode:        policy.Standard,
		DeniedTools: map[string]bool{"blocked_tool": true},
	}

	blockedTool := ToolDescriptor{
		Tool: Tool{
			Name:        "blocked_tool",
			Description: "blocked",
			InputSchema: map[string]any{"type": "object"},
			Annotations: map[string]any{"readOnlyHint": true},
		},
		Handler:      func(ctx context.Context, args map[string]any) (any, error) { return "data", nil },
		ReadOnlyHint: true,
	}
	allowedTool := ToolDescriptor{
		Tool: Tool{
			Name:        "allowed_tool",
			Description: "allowed",
			InputSchema: map[string]any{"type": "object"},
			Annotations: map[string]any{"readOnlyHint": true},
		},
		Handler:      func(ctx context.Context, args map[string]any) (any, error) { return "data", nil },
		ReadOnlyHint: true,
	}

	server := NewServer("test", pol, []ToolDescriptor{blockedTool, allowedTool}, nil, truncate.Config{}, dryrun.Config{}, nil)
	tools := server.listTools()

	if len(tools) != 1 {
		t.Fatalf("expected 1 tool (blocked tool hidden), got %d", len(tools))
	}
	if tools[0].Name != "allowed_tool" {
		t.Fatalf("expected allowed_tool, got %s", tools[0].Name)
	}
}

// ---------------------------------------------------------------------------
// 7. Policy blocks tool call execution
// ---------------------------------------------------------------------------

func TestPolicyBlocksToolCallExecution(t *testing.T) {
	pol := &policy.Policy{
		Mode:        policy.ReadOnly,
		DeniedTools: map[string]bool{},
	}
	writeTool := ToolDescriptor{
		Tool: Tool{
			Name:        "write_tool",
			Description: "w",
			InputSchema: map[string]any{"type": "object"},
			Annotations: map[string]any{"readOnlyHint": false},
		},
		Handler:      func(ctx context.Context, args map[string]any) (any, error) { return "should not run", nil },
		ReadOnlyHint: false,
	}

	server := NewServer("test", pol, []ToolDescriptor{writeTool}, nil, truncate.Config{}, dryrun.Config{}, nil)
	server.initialized.Store(true) // skip init guard — we're testing policy enforcement
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write_tool","arguments":{}}}`

	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	var resp Response
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Tool errors now use isError per MCP spec
	resultMap, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	isErr, _ := resultMap["isError"].(bool)
	if !isErr {
		t.Fatal("expected isError=true when calling blocked tool")
	}
	content, _ := resultMap["content"].([]any)
	if len(content) == 0 {
		t.Fatal("expected content in error response")
	}
	textObj, _ := content[0].(map[string]any)
	text, _ := textObj["text"].(string)
	if !strings.Contains(text, "blocked by policy") {
		t.Fatalf("expected 'blocked by policy' in error text, got %q", text)
	}
}

// ---------------------------------------------------------------------------
// 8. Safe-core mode allows safe writes, blocks others
// ---------------------------------------------------------------------------

func TestSafeCorePolicy(t *testing.T) {
	pol := &policy.Policy{Mode: policy.SafeCore, DeniedTools: map[string]bool{}}

	// clockify_start_timer is in safe_core write list
	safeTool := ToolDescriptor{
		Tool: Tool{
			Name:        "clockify_start_timer",
			Description: "start",
			InputSchema: map[string]any{"type": "object"},
			Annotations: map[string]any{"readOnlyHint": false},
		},
		Handler:      func(ctx context.Context, args map[string]any) (any, error) { return nil, nil },
		ReadOnlyHint: false,
	}
	// A random write tool NOT in safe_core list
	unsafeTool := ToolDescriptor{
		Tool: Tool{
			Name:        "clockify_dangerous_write",
			Description: "danger",
			InputSchema: map[string]any{"type": "object"},
			Annotations: map[string]any{"readOnlyHint": false},
		},
		Handler:      func(ctx context.Context, args map[string]any) (any, error) { return nil, nil },
		ReadOnlyHint: false,
	}
	readTool := ToolDescriptor{
		Tool: Tool{
			Name:        "read_thing",
			Description: "read",
			InputSchema: map[string]any{"type": "object"},
			Annotations: map[string]any{"readOnlyHint": true},
		},
		Handler:      func(ctx context.Context, args map[string]any) (any, error) { return nil, nil },
		ReadOnlyHint: true,
	}

	server := NewServer("test", pol, []ToolDescriptor{safeTool, unsafeTool, readTool}, nil, truncate.Config{}, dryrun.Config{}, nil)
	tools := server.listTools()

	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}

	if !names["clockify_start_timer"] {
		t.Fatal("safe_core should allow clockify_start_timer")
	}
	if !names["read_thing"] {
		t.Fatal("safe_core should allow read tools")
	}
	if names["clockify_dangerous_write"] {
		t.Fatal("safe_core should block non-safe write tools")
	}
}

// ---------------------------------------------------------------------------
// 9. Bootstrap filtering — minimal mode
// ---------------------------------------------------------------------------

func TestBootstrapMinimalFiltering(t *testing.T) {
	bc := &bootstrap.Config{Mode: bootstrap.Minimal}
	tier1 := map[string]bool{
		"clockify_whoami":          true,
		"clockify_start_timer":     true,
		"clockify_list_projects":   true,
		"clockify_detailed_report": true,
	}
	bc.SetTier1Tools(tier1)

	tools := []ToolDescriptor{
		{Tool: Tool{Name: "clockify_whoami", Description: "who", InputSchema: map[string]any{"type": "object"}, Annotations: map[string]any{"readOnlyHint": true}}, ReadOnlyHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) { return nil, nil }},
		{Tool: Tool{Name: "clockify_start_timer", Description: "start", InputSchema: map[string]any{"type": "object"}, Annotations: map[string]any{"readOnlyHint": false}}, ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) { return nil, nil }},
		{Tool: Tool{Name: "clockify_list_projects", Description: "list", InputSchema: map[string]any{"type": "object"}, Annotations: map[string]any{"readOnlyHint": true}}, ReadOnlyHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) { return nil, nil }},
		{Tool: Tool{Name: "clockify_detailed_report", Description: "report", InputSchema: map[string]any{"type": "object"}, Annotations: map[string]any{"readOnlyHint": true}}, ReadOnlyHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) { return nil, nil }},
	}

	server := NewServer("test", nil, tools, nil, truncate.Config{}, dryrun.Config{}, bc)
	visible := server.listTools()

	visibleNames := map[string]bool{}
	for _, tool := range visible {
		visibleNames[tool.Name] = true
	}

	// whoami is in AlwaysVisible and MinimalSet — always visible
	if !visibleNames["clockify_whoami"] {
		t.Fatal("whoami should be visible in minimal mode")
	}
	// start_timer is in MinimalSet
	if !visibleNames["clockify_start_timer"] {
		t.Fatal("start_timer should be visible in minimal mode")
	}
	// list_projects is in MinimalSet
	if !visibleNames["clockify_list_projects"] {
		t.Fatal("list_projects should be visible in minimal mode")
	}
	// detailed_report is NOT in MinimalSet
	if visibleNames["clockify_detailed_report"] {
		t.Fatal("detailed_report should NOT be visible in minimal mode")
	}
}

// ---------------------------------------------------------------------------
// 10. Bootstrap filtering — full_tier1 mode shows all registered tier1 tools
// ---------------------------------------------------------------------------

func TestBootstrapFullTier1Filtering(t *testing.T) {
	bc := &bootstrap.Config{Mode: bootstrap.FullTier1}
	tier1 := map[string]bool{
		"clockify_whoami":          true,
		"clockify_detailed_report": true,
		"clockify_list_projects":   true,
	}
	bc.SetTier1Tools(tier1)

	tools := []ToolDescriptor{
		{Tool: Tool{Name: "clockify_whoami", Description: "who", InputSchema: map[string]any{"type": "object"}, Annotations: map[string]any{"readOnlyHint": true}}, ReadOnlyHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) { return nil, nil }},
		{Tool: Tool{Name: "clockify_detailed_report", Description: "report", InputSchema: map[string]any{"type": "object"}, Annotations: map[string]any{"readOnlyHint": true}}, ReadOnlyHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) { return nil, nil }},
		{Tool: Tool{Name: "clockify_list_projects", Description: "list", InputSchema: map[string]any{"type": "object"}, Annotations: map[string]any{"readOnlyHint": true}}, ReadOnlyHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) { return nil, nil }},
	}

	server := NewServer("test", nil, tools, nil, truncate.Config{}, dryrun.Config{}, bc)
	visible := server.listTools()

	if len(visible) != 3 {
		t.Fatalf("expected 3 tools in full_tier1 mode, got %d", len(visible))
	}
}

// ---------------------------------------------------------------------------
// 11. Bootstrap filtering — custom mode
// ---------------------------------------------------------------------------

func TestBootstrapCustomFiltering(t *testing.T) {
	bc := &bootstrap.Config{
		Mode:        bootstrap.Custom,
		CustomTools: map[string]bool{"clockify_whoami": true, "clockify_list_projects": true},
	}
	tier1 := map[string]bool{
		"clockify_whoami":          true,
		"clockify_list_projects":   true,
		"clockify_detailed_report": true,
	}
	bc.SetTier1Tools(tier1)

	tools := []ToolDescriptor{
		{Tool: Tool{Name: "clockify_whoami", Description: "who", InputSchema: map[string]any{"type": "object"}, Annotations: map[string]any{"readOnlyHint": true}}, ReadOnlyHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) { return nil, nil }},
		{Tool: Tool{Name: "clockify_list_projects", Description: "list", InputSchema: map[string]any{"type": "object"}, Annotations: map[string]any{"readOnlyHint": true}}, ReadOnlyHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) { return nil, nil }},
		{Tool: Tool{Name: "clockify_detailed_report", Description: "report", InputSchema: map[string]any{"type": "object"}, Annotations: map[string]any{"readOnlyHint": true}}, ReadOnlyHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) { return nil, nil }},
	}

	server := NewServer("test", nil, tools, nil, truncate.Config{}, dryrun.Config{}, bc)
	visible := server.listTools()

	visibleNames := map[string]bool{}
	for _, tool := range visible {
		visibleNames[tool.Name] = true
	}

	if !visibleNames["clockify_whoami"] {
		t.Fatal("whoami should be visible in custom mode (explicitly listed)")
	}
	if !visibleNames["clockify_list_projects"] {
		t.Fatal("list_projects should be visible in custom mode (explicitly listed)")
	}
	if visibleNames["clockify_detailed_report"] {
		t.Fatal("detailed_report should NOT be visible in custom mode (not in custom set)")
	}
}

// ---------------------------------------------------------------------------
// 12. Combined policy + bootstrap filtering
// ---------------------------------------------------------------------------

func TestCombinedPolicyAndBootstrap(t *testing.T) {
	pol := &policy.Policy{
		Mode:        policy.ReadOnly,
		DeniedTools: map[string]bool{},
	}
	bc := &bootstrap.Config{Mode: bootstrap.FullTier1}
	tier1 := map[string]bool{
		"clockify_whoami":      true,
		"clockify_start_timer": true,
	}
	bc.SetTier1Tools(tier1)

	tools := []ToolDescriptor{
		{Tool: Tool{Name: "clockify_whoami", Description: "who", InputSchema: map[string]any{"type": "object"}, Annotations: map[string]any{"readOnlyHint": true}}, ReadOnlyHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) { return nil, nil }},
		{Tool: Tool{Name: "clockify_start_timer", Description: "start", InputSchema: map[string]any{"type": "object"}, Annotations: map[string]any{"readOnlyHint": false}}, ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) { return nil, nil }},
	}

	server := NewServer("test", pol, tools, nil, truncate.Config{}, dryrun.Config{}, bc)
	visible := server.listTools()

	// Bootstrap allows both, but policy is read_only so start_timer (write) is hidden
	// However, clockify_start_timer is NOT an introspection tool, so read_only blocks it
	visibleNames := map[string]bool{}
	for _, tool := range visible {
		visibleNames[tool.Name] = true
	}

	if !visibleNames["clockify_whoami"] {
		t.Fatal("whoami should be visible (read-only tool)")
	}
	if visibleNames["clockify_start_timer"] {
		t.Fatal("start_timer should be hidden by read_only policy")
	}
}

// ---------------------------------------------------------------------------
// 13. Notifications/initialized is a no-op (no response)
// ---------------------------------------------------------------------------

func TestNotificationsInitializedNoResponse(t *testing.T) {
	server := NewServer("test", nil, nil, nil, truncate.Config{}, dryrun.Config{}, nil)
	input := `{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`

	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	result := strings.TrimSpace(out.String())
	if result != "" {
		t.Fatalf("expected no output for notifications/initialized, got %q", result)
	}
}

// ---------------------------------------------------------------------------
// 14. ActivateGroup adds tools and sends notification
// ---------------------------------------------------------------------------

func TestActivateGroupAddsTools(t *testing.T) {
	server := NewServer("test", nil, nil, nil, truncate.Config{}, dryrun.Config{}, nil)

	newTools := []ToolDescriptor{
		{
			Tool: Tool{
				Name:        "new_tool_a",
				Description: "tool A",
				InputSchema: map[string]any{"type": "object"},
				Annotations: map[string]any{"readOnlyHint": true},
			},
			Handler:      func(ctx context.Context, args map[string]any) (any, error) { return nil, nil },
			ReadOnlyHint: true,
		},
		{
			Tool: Tool{
				Name:        "new_tool_b",
				Description: "tool B",
				InputSchema: map[string]any{"type": "object"},
				Annotations: map[string]any{"readOnlyHint": false},
			},
			Handler:      func(ctx context.Context, args map[string]any) (any, error) { return nil, nil },
			ReadOnlyHint: false,
		},
	}

	if err := server.ActivateGroup("test_group", newTools); err != nil {
		t.Fatalf("activate group failed: %v", err)
	}

	tools := server.listTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools after activation, got %d", len(tools))
	}
}

// ---------------------------------------------------------------------------
// 15. ActivateGroup blocked by read_only policy
// ---------------------------------------------------------------------------

func TestActivateGroupBlockedByPolicy(t *testing.T) {
	pol := &policy.Policy{Mode: policy.ReadOnly, DeniedTools: map[string]bool{}}
	server := NewServer("test", pol, nil, nil, truncate.Config{}, dryrun.Config{}, nil)

	err := server.ActivateGroup("some_group", []ToolDescriptor{})
	if err == nil {
		t.Fatal("expected error when activating group in read_only mode")
	}
	if !strings.Contains(err.Error(), "blocked by policy") {
		t.Fatalf("expected 'blocked by policy' error, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// 16. Empty lines in input are skipped
// ---------------------------------------------------------------------------

func TestEmptyLinesSkipped(t *testing.T) {
	server := NewServer("test", nil, nil, nil, truncate.Config{}, dryrun.Config{}, nil)
	input := "\n\n" + `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n\n"

	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 response, got %d: %s", len(lines), out.String())
	}

	var resp Response
	if err := json.Unmarshal([]byte(lines[0]), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("ping error: %v", resp.Error)
	}
}

// ---------------------------------------------------------------------------
// 17. tools/list returns sorted tools
// ---------------------------------------------------------------------------

func TestToolsListReturnsSorted(t *testing.T) {
	tools := []ToolDescriptor{
		{Tool: Tool{Name: "z_tool", Description: "z", InputSchema: map[string]any{"type": "object"}, Annotations: map[string]any{"readOnlyHint": true}}, ReadOnlyHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) { return nil, nil }},
		{Tool: Tool{Name: "a_tool", Description: "a", InputSchema: map[string]any{"type": "object"}, Annotations: map[string]any{"readOnlyHint": true}}, ReadOnlyHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) { return nil, nil }},
		{Tool: Tool{Name: "m_tool", Description: "m", InputSchema: map[string]any{"type": "object"}, Annotations: map[string]any{"readOnlyHint": true}}, ReadOnlyHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) { return nil, nil }},
	}

	server := NewServer("test", nil, tools, nil, truncate.Config{}, dryrun.Config{}, nil)
	listed := server.listTools()

	if len(listed) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(listed))
	}
	if listed[0].Name != "a_tool" || listed[1].Name != "m_tool" || listed[2].Name != "z_tool" {
		t.Fatalf("tools not sorted: %s, %s, %s", listed[0].Name, listed[1].Name, listed[2].Name)
	}
}

// ---------------------------------------------------------------------------
// 18. Truncation works on tool results
// ---------------------------------------------------------------------------

func TestTruncationApplied(t *testing.T) {
	// Create a tool that returns a large payload
	bigTool := ToolDescriptor{
		Tool: Tool{
			Name:        "big_tool",
			Description: "returns big data",
			InputSchema: map[string]any{"type": "object"},
			Annotations: map[string]any{"readOnlyHint": true},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			// Create a large map that exceeds token budget
			data := map[string]any{}
			for i := 0; i < 500; i++ {
				data["key_"+strings.Repeat("x", 50)+string(rune('a'+i%26))] = strings.Repeat("value", 100)
			}
			return data, nil
		},
		ReadOnlyHint: true,
	}

	tc := truncate.Config{TokenBudget: 100, Enabled: true}
	server := NewServer("test", nil, []ToolDescriptor{bigTool}, nil, tc, dryrun.Config{}, nil)
	server.initialized.Store(true) // skip init guard — we're testing truncation

	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"big_tool","arguments":{}}}`
	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	var resp Response
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	// The result should contain the truncated data with _truncation metadata
	callResult, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	content, ok := callResult["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatal("expected content in response")
	}
	textObj, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map in content, got %T", content[0])
	}
	text, _ := textObj["text"].(string)
	if !strings.Contains(text, "_truncation") {
		t.Fatal("expected _truncation metadata in truncated response")
	}
}

// ---------------------------------------------------------------------------
// 19. Dry-run on non-destructive tool returns error via server pipeline
// ---------------------------------------------------------------------------

func TestDryRunOnNonDestructiveTool(t *testing.T) {
	readTool := ToolDescriptor{
		Tool: Tool{
			Name:        "safe_read",
			Description: "safe read tool",
			InputSchema: map[string]any{"type": "object"},
			Annotations: map[string]any{"readOnlyHint": true},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]any{"data": "ok"}, nil
		},
		ReadOnlyHint:    true,
		DestructiveHint: false,
	}

	server := NewServer("test", nil, []ToolDescriptor{readTool}, nil, truncate.Config{}, dryrun.Config{Enabled: true}, nil)
	server.initialized.Store(true) // skip init guard — we're testing dry-run
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"safe_read","arguments":{"dry_run":true}}}`

	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	var resp Response
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Tool errors now use isError per MCP spec
	resultMap, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	isErr, _ := resultMap["isError"].(bool)
	if !isErr {
		t.Fatal("expected isError=true for dry_run on non-destructive tool")
	}
	content, _ := resultMap["content"].([]any)
	if len(content) == 0 {
		t.Fatal("expected content in error response")
	}
	textObj, _ := content[0].(map[string]any)
	text, _ := textObj["text"].(string)
	if !strings.Contains(text, "not supported for non-destructive") {
		t.Fatalf("expected 'not supported for non-destructive' in error text, got %q", text)
	}
}

// ---------------------------------------------------------------------------
// 20. Multiple requests in sequence all get responses
// ---------------------------------------------------------------------------

func TestMultipleSequentialRequests(t *testing.T) {
	var counter atomic.Int32
	tool := ToolDescriptor{
		Tool: Tool{
			Name:        "counter_tool",
			Description: "increments counter",
			InputSchema: map[string]any{"type": "object"},
			Annotations: map[string]any{"readOnlyHint": true},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			val := counter.Add(1)
			return map[string]any{"count": val}, nil
		},
		ReadOnlyHint: true,
	}

	server := NewServer("test", nil, []ToolDescriptor{tool}, nil, truncate.Config{}, dryrun.Config{}, nil)
	server.initialized.Store(true) // skip init guard — we're testing sequential dispatch

	var requests []string
	for i := 1; i <= 5; i++ {
		requests = append(requests, `{"jsonrpc":"2.0","id":`+strings.Repeat("", 0)+string(rune('0'+i))+`,"method":"tools/call","params":{"name":"counter_tool","arguments":{}}}`)
	}
	input := strings.Join(requests, "\n")

	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 responses, got %d", len(lines))
	}
	if counter.Load() != 5 {
		t.Fatalf("expected counter=5, got %d", counter.Load())
	}
}
