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
	for _, name := range []string{"clockify_list_tools", "clockify_activate_group", "clockify_activate_tool", "clockify_deactivate_group"} {
		if !strings.Contains(ServerInstructions, name) {
			t.Fatalf("ServerInstructions should reference %s: %q", name, ServerInstructions)
		}
	}
	if !strings.Contains(ServerInstructions, "clockify_search_tools") || !strings.Contains(ServerInstructions, "deprecated compatibility shim") {
		t.Fatalf("ServerInstructions should keep the deprecated search_tools compatibility note: %q", ServerInstructions)
	}
	if !strings.Contains(ServerInstructions, "dry_run:true") || !strings.Contains(ServerInstructions, "dry_run:false") {
		t.Fatalf("ServerInstructions should describe dry_run:true preview and dry_run:false execution: %q", ServerInstructions)
	}
	if strings.Contains(ServerInstructions, "dry-run interceptor by default") {
		t.Fatalf("ServerInstructions contains stale dry-run-default wording: %q", ServerInstructions)
	}
	// ADR 0018 confirmation-token gate. Agentic clients reading
	// instructions as system prompt need both the "high-risk" keyword
	// (so they recognise the gated class) and the confirmation_token
	// argument name (so they can echo the dry-run-issued token back).
	// Case-insensitive on "high-risk" so a sentence-leading "High-risk"
	// (the natural English rendering) still passes the contract test.
	lowered := strings.ToLower(ServerInstructions)
	for _, marker := range []string{"high-risk", "confirmation_token"} {
		if !strings.Contains(lowered, marker) {
			t.Fatalf("ServerInstructions missing %q (ADR 0018 confirmation-token guidance): %q", marker, ServerInstructions)
		}
	}
	// Policy-mode count must match the policy.Mode constants in
	// internal/policy. Stale "four policy modes" wording omits
	// time_tracking_safe — the recommended AI-facing default — and
	// misleads agentic clients reading instructions as system prompt.
	if !strings.Contains(ServerInstructions, "five policy modes") {
		t.Fatalf("ServerInstructions should advertise all five policy modes: %q", ServerInstructions)
	}
	for _, mode := range []string{"read_only", "time_tracking_safe", "safe_core", "standard", "full"} {
		if !strings.Contains(ServerInstructions, mode) {
			t.Fatalf("ServerInstructions missing policy mode %q: %q", mode, ServerInstructions)
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
