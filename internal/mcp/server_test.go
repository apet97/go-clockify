package mcp

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestScannerMaxMessageSizeBoundsIntConversion(t *testing.T) {
	tests := []struct {
		name       string
		configured int64
		want       int
	}{
		{name: "default", configured: 0, want: 4 * 1024 * 1024},
		{name: "positive", configured: 12345, want: 12345},
		{name: "too large", configured: math.MaxInt64, want: math.MaxInt},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scannerMaxMessageSize(tt.configured); got != tt.want {
				t.Fatalf("scannerMaxMessageSize(%d) = %d, want %d", tt.configured, got, tt.want)
			}
		})
	}
}

func TestInitializeAndToolsList(t *testing.T) {
	server := NewServer("test", []ToolDescriptor{{
		Tool:    Tool{Name: "demo_tool", Description: "demo"},
		Handler: func(context.Context, map[string]any) (any, error) { return map[string]any{"ok": true}, nil },
	}})

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	}, "\n")

	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, `"protocolVersion":"`+SupportedProtocolVersions[0]+`"`) {
		t.Fatalf("missing initialize response: %s", got)
	}
	if !strings.Contains(got, `"listChanged":true`) {
		t.Fatalf("expected stdio initialize to advertise tools.listChanged: %s", got)
	}
	if !strings.Contains(got, `"demo_tool"`) {
		t.Fatalf("missing tool list response: %s", got)
	}
}

func TestAdvertisedToolsCanLimitDispatchByName(t *testing.T) {
	visible := ToolDescriptor{
		Tool:    Tool{Name: "visible_tool", Description: "visible"},
		Handler: func(context.Context, map[string]any) (any, error) { return map[string]any{"ok": true}, nil },
	}
	hiddenCalled := false
	hidden := ToolDescriptor{
		Tool: Tool{Name: "hidden_tool", Description: "hidden", InputSchema: map[string]any{"type": "object"}},
		Handler: func(context.Context, map[string]any) (any, error) {
			hiddenCalled = true
			return map[string]any{"ok": true, "tool": "hidden_tool"}, nil
		},
	}
	server := NewServer("test", []ToolDescriptor{visible, hidden})
	server.SetAdvertisedTools([]ToolDescriptor{visible})
	server.EnforceAdvertisedTools = true

	raw, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	if err != nil {
		t.Fatalf("initialize dispatch: %v", err)
	}
	var initResp Response
	if err := json.Unmarshal(raw, &initResp); err != nil {
		t.Fatalf("unmarshal initialize response: %v", err)
	}
	if initResp.Error != nil {
		t.Fatalf("initialize returned error: %+v", initResp.Error)
	}

	raw, err = server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	if err != nil {
		t.Fatalf("tools/list dispatch: %v", err)
	}
	listPayload := string(raw)
	if !strings.Contains(listPayload, `"visible_tool"`) {
		t.Fatalf("tools/list missing advertised tool: %s", listPayload)
	}
	if strings.Contains(listPayload, `"hidden_tool"`) {
		t.Fatalf("tools/list exposed unadvertised tool: %s", listPayload)
	}

	raw, err = server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"hidden_tool","arguments":{}}}`))
	if err != nil {
		t.Fatalf("tools/call dispatch: %v", err)
	}
	callPayload := string(raw)
	if hiddenCalled {
		t.Fatal("tools/call invoked an unadvertised loaded tool despite enforcement")
	}
	if !strings.Contains(callPayload, `"error"`) || !strings.Contains(callPayload, `unknown tool: hidden_tool`) {
		t.Fatalf("tools/call should reject unadvertised loaded tool: %s", callPayload)
	}
}

func TestAdvertisedToolsDispatchByNameWhenEnforcementDisabled(t *testing.T) {
	visible := ToolDescriptor{
		Tool:    Tool{Name: "visible_tool", Description: "visible"},
		Handler: func(context.Context, map[string]any) (any, error) { return map[string]any{"ok": true}, nil },
	}
	hiddenCalled := false
	hidden := ToolDescriptor{
		Tool: Tool{Name: "hidden_tool", Description: "hidden", InputSchema: map[string]any{"type": "object"}},
		Handler: func(context.Context, map[string]any) (any, error) {
			hiddenCalled = true
			return map[string]any{"ok": true, "tool": "hidden_tool"}, nil
		},
	}
	server := NewServer("test", []ToolDescriptor{visible, hidden})
	server.SetAdvertisedTools([]ToolDescriptor{visible})

	if _, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)); err != nil {
		t.Fatalf("initialize dispatch: %v", err)
	}
	raw, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"hidden_tool","arguments":{}}}`))
	if err != nil {
		t.Fatalf("tools/call dispatch: %v", err)
	}
	callPayload := string(raw)
	if !hiddenCalled {
		t.Fatal("tools/call did not invoke the unadvertised loaded tool when enforcement was disabled")
	}
	if strings.Contains(callPayload, `"error"`) || !strings.Contains(callPayload, `"tool":"hidden_tool"`) {
		t.Fatalf("tools/call could not dispatch unadvertised loaded tool: %s", callPayload)
	}
}

func TestSetAdvertisedToolsInvalidatesSerializedToolsListCache(t *testing.T) {
	first := ToolDescriptor{
		Tool:    Tool{Name: "first_tool", Description: "first"},
		Handler: func(context.Context, map[string]any) (any, error) { return map[string]any{"ok": true}, nil },
	}
	second := ToolDescriptor{
		Tool:    Tool{Name: "second_tool", Description: "second"},
		Handler: func(context.Context, map[string]any) (any, error) { return map[string]any{"ok": true}, nil },
	}
	server := NewServer("test", []ToolDescriptor{first, second})
	server.SetAdvertisedTools([]ToolDescriptor{first})

	if _, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)); err != nil {
		t.Fatalf("initialize dispatch: %v", err)
	}

	raw, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	if err != nil {
		t.Fatalf("first tools/list dispatch: %v", err)
	}
	assertToolsListNames(t, raw, []string{"first_tool"})

	server.SetAdvertisedTools([]ToolDescriptor{second})
	raw, err = server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`))
	if err != nil {
		t.Fatalf("second tools/list dispatch: %v", err)
	}
	assertToolsListNames(t, raw, []string{"second_tool"})
}

func TestToolsListSupportsCursorPagination(t *testing.T) {
	descriptors := []ToolDescriptor{
		{Tool: Tool{Name: "a_tool", Description: "a"}, Handler: func(context.Context, map[string]any) (any, error) { return nil, nil }},
		{Tool: Tool{Name: "b_tool", Description: "b"}, Handler: func(context.Context, map[string]any) (any, error) { return nil, nil }},
		{Tool: Tool{Name: "c_tool", Description: "c"}, Handler: func(context.Context, map[string]any) (any, error) { return nil, nil }},
	}
	server := NewServer("test", descriptors)
	server.ToolsListPageSize = 2
	if _, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)); err != nil {
		t.Fatalf("initialize dispatch: %v", err)
	}

	raw, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`))
	if err != nil {
		t.Fatalf("first tools/list dispatch: %v", err)
	}
	assertToolsListNames(t, raw, []string{"a_tool", "b_tool"})
	if got := toolsListNextCursor(t, raw); got != "2" {
		t.Fatalf("nextCursor = %q, want 2: %s", got, raw)
	}

	raw, err = server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{"cursor":"2"}}`))
	if err != nil {
		t.Fatalf("second tools/list dispatch: %v", err)
	}
	assertToolsListNames(t, raw, []string{"c_tool"})
	if got := toolsListNextCursor(t, raw); got != "" {
		t.Fatalf("last page nextCursor = %q, want empty: %s", got, raw)
	}
}

func TestToolsListRejectsInvalidCursor(t *testing.T) {
	server := NewServer("test", []ToolDescriptor{{
		Tool:    Tool{Name: "demo_tool", Description: "demo"},
		Handler: func(context.Context, map[string]any) (any, error) { return nil, nil },
	}})
	if _, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)); err != nil {
		t.Fatalf("initialize dispatch: %v", err)
	}
	raw, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{"cursor":2}}`))
	if err != nil {
		t.Fatalf("tools/list dispatch: %v", err)
	}
	var resp Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal tools/list response: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != -32602 {
		t.Fatalf("invalid cursor response = %s, want -32602", raw)
	}
}

func TestInitializeRejectsMalformedParams(t *testing.T) {
	server := NewServer("test", nil)
	raw, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":{"not":"a string"}}}`))
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	var resp Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil {
		t.Fatalf("expected invalid params error, got response: %s", raw)
	}
	if resp.Error.Code != -32602 {
		t.Fatalf("initialize error code=%d want -32602: %+v", resp.Error.Code, resp.Error)
	}
	if server.initialized.Load() {
		t.Fatal("malformed initialize params should not mark the server initialized")
	}
}

func TestToolsListReturnsDeepClonedSchemaMetadata(t *testing.T) {
	server := NewServer("test", []ToolDescriptor{{
		Tool: Tool{
			Name:        "demo_tool",
			Description: "demo",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
			OutputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ok": map[string]any{"type": "boolean"},
				},
			},
			Annotations: map[string]any{
				"riskClass": []any{"read"},
				"nested":    map[string]any{"safe": true},
			},
		},
		AdvertiseOutputSchema: true,
		Handler:               func(context.Context, map[string]any) (any, error) { return map[string]any{"ok": true}, nil },
	}})

	first := server.listTools()
	first[0].InputSchema["x-mutated"] = true
	firstProps := first[0].InputSchema["properties"].(map[string]any)
	firstProps["name"].(map[string]any)["type"] = "number"
	first[0].OutputSchema["x-mutated"] = true
	first[0].OutputSchema["properties"].(map[string]any)["ok"].(map[string]any)["type"] = "string"
	first[0].Annotations["riskClass"].([]any)[0] = "destructive"
	first[0].Annotations["nested"].(map[string]any)["safe"] = false

	second := server.listTools()
	if _, ok := second[0].InputSchema["x-mutated"]; ok {
		t.Fatal("top-level input schema mutation leaked into cached tools/list metadata")
	}
	secondProps := second[0].InputSchema["properties"].(map[string]any)
	if got := secondProps["name"].(map[string]any)["type"]; got != "string" {
		t.Fatalf("nested input schema mutation leaked: type=%v", got)
	}
	if _, ok := second[0].OutputSchema["x-mutated"]; ok {
		t.Fatal("top-level output schema mutation leaked into cached tools/list metadata")
	}
	if got := second[0].OutputSchema["properties"].(map[string]any)["ok"].(map[string]any)["type"]; got != "boolean" {
		t.Fatalf("nested output schema mutation leaked: type=%v", got)
	}
	if got := second[0].Annotations["riskClass"].([]any)[0]; got != "read" {
		t.Fatalf("annotation slice mutation leaked: %v", got)
	}
	if got := second[0].Annotations["nested"].(map[string]any)["safe"]; got != true {
		t.Fatalf("annotation map mutation leaked: %v", got)
	}
}

func assertToolsListNames(t *testing.T, raw []byte, want []string) {
	t.Helper()
	var resp struct {
		Result struct {
			Tools []Tool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal tools/list response: %v", err)
	}
	if len(resp.Result.Tools) != len(want) {
		t.Fatalf("tools/list returned %d tools, want %d: %s", len(resp.Result.Tools), len(want), raw)
	}
	for i, tool := range resp.Result.Tools {
		if tool.Name != want[i] {
			t.Fatalf("tools/list[%d]=%s, want %s: %s", i, tool.Name, want[i], raw)
		}
	}
}

func toolsListNextCursor(t *testing.T, raw []byte) string {
	t.Helper()
	var resp struct {
		Result struct {
			NextCursor string `json:"nextCursor"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal tools/list response: %v", err)
	}
	return resp.Result.NextCursor
}

func TestServerInstructionsPublicContract(t *testing.T) {
	for _, want := range []string{"single-user full-access", "All tools are loaded", "Use workflow tools first", "Use IDs returned by previous calls"} {
		if !strings.Contains(ServerInstructions, want) {
			t.Fatalf("ServerInstructions missing %q: %q", want, ServerInstructions)
		}
	}
	for _, forbidden := range []string{
		"clockify_" + "activate_group",
		"clockify_" + "activate_tool",
		"clockify_" + "deactivate_group",
		"clockify_" + "search_tools",
		"confirm" + "ation",
		"policy " + "mode",
		"Tier " + "2",
		"ten" + "ant",
		"hos" + "ted",
	} {
		if strings.Contains(strings.ToLower(ServerInstructions), strings.ToLower(forbidden)) {
			t.Fatalf("ServerInstructions contains forbidden language %q: %q", forbidden, ServerInstructions)
		}
	}
}

// FuzzJSONRPCParse feeds random byte sequences into the JSON-RPC Request
// decoder and ensures it never panics. Malformed requests should produce
// errors, not crashes.
func FuzzJSONRPCParse(f *testing.F) {
	seeds := [][]byte{
		[]byte(`{}`),
		[]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`),
		[]byte(`{"jsonrpc":"2.0","id":"abc","method":"tools/list"}`),
		[]byte(`{"jsonrpc":"2.0","id":null,"method":"ping"}`),
		[]byte(`{"jsonrpc":"1.0","id":1,"method":"bad"}`),
		[]byte(`{"jsonrpc":"2.0","id":{"nested":true},"method":"weird"}`),
		[]byte(`not json at all`),
		[]byte(``),
		[]byte(`{"method":"x","params":{"a":1,"b":[1,2,3]}}`),
		[]byte(`{"\u0000":"null byte key"}`),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, payload []byte) {
		var req Request
		_ = json.Unmarshal(payload, &req)
	})
}
