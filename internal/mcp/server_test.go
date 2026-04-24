package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestInitializeAndToolsList(t *testing.T) {
	server := NewServer("test", []ToolDescriptor{{
		Tool:    Tool{Name: "demo_tool", Description: "demo"},
		Handler: func(context.Context, map[string]any) (any, error) { return map[string]any{"ok": true}, nil },
	}}, nil, nil)

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

func TestServerInstructionsPublicContract(t *testing.T) {
	if !strings.Contains(ServerInstructions, "clockify_search_tools") {
		t.Fatalf("ServerInstructions should reference clockify_search_tools: %q", ServerInstructions)
	}
	if !strings.Contains(ServerInstructions, "activate_group") || !strings.Contains(ServerInstructions, "activate_tool") {
		t.Fatalf("ServerInstructions should mention activate_group and activate_tool: %q", ServerInstructions)
	}
	if strings.Contains(ServerInstructions, "clockify_activate_group") || strings.Contains(ServerInstructions, "clockify_activate_tool") {
		t.Fatalf("ServerInstructions contains stale activation tool names: %q", ServerInstructions)
	}
	if !strings.Contains(ServerInstructions, "dry_run:true") || !strings.Contains(ServerInstructions, "dry_run:false") {
		t.Fatalf("ServerInstructions should describe dry_run:true preview and dry_run:false execution: %q", ServerInstructions)
	}
	if strings.Contains(ServerInstructions, "dry-run interceptor by default") {
		t.Fatalf("ServerInstructions contains stale dry-run-default wording: %q", ServerInstructions)
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
