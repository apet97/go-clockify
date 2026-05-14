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
