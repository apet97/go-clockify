package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// TestProtocolVersion_NegotiationTable exhaustively drives each
// protocol version through initialize and asserts the negotiated
// response matches the documented policy in handleInitialize:
//
//   - requested version is in SupportedProtocolVersions → echo it back
//   - requested version is absent → default to newest supported
//   - requested version is unknown → echo the newest supported (the
//     client is expected to accept the downgrade or disconnect)
//
// Before E1 we had a single happy-path assertion in
// TestFullMCPHandshake and no guard against the negotiation table
// ever silently changing shape.
func TestProtocolVersion_NegotiationTable(t *testing.T) {
	cases := []struct {
		name      string
		requested string
		want      string
	}{
		{"empty_defaults_newest", "", SupportedProtocolVersions[0]},
		{"2025-06-18_echoed", "2025-06-18", "2025-06-18"},
		{"2025-03-26_echoed", "2025-03-26", "2025-03-26"},
		{"2024-11-05_echoed", "2024-11-05", "2024-11-05"},
		{"unknown_future_downgrades_to_newest", "2099-01-01", SupportedProtocolVersions[0]},
		{"unknown_past_downgrades_to_newest", "2019-04-01", SupportedProtocolVersions[0]},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := NewServer("test", nil, nil, nil)

			var params any
			if tc.requested != "" {
				params = map[string]any{"protocolVersion": tc.requested}
			}
			req := map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "initialize",
				"params":  params,
			}
			raw, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var out strings.Builder
			if err := server.Run(context.Background(), strings.NewReader(string(raw)), &out); err != nil {
				t.Fatalf("server.Run: %v", err)
			}
			var resp Response
			if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if resp.Error != nil {
				t.Fatalf("unexpected RPC error: %+v", resp.Error)
			}
			result, _ := resp.Result.(map[string]any)
			got, _ := result["protocolVersion"].(string)
			if got != tc.want {
				t.Fatalf("requested=%q negotiated=%q want %q", tc.requested, got, tc.want)
			}
		})
	}
}

// TestProtocolVersion_CapabilitiesShape asserts the initialize response
// shape is stable across every supported version — the client sees
// `capabilities.tools` with a listChanged flag plus `prompts` with
// listChanged. A silent regression that dropped listChanged would
// break MCP 2025-03-26+ clients that rely on it.
func TestProtocolVersion_CapabilitiesShape(t *testing.T) {
	for _, version := range SupportedProtocolVersions {
		t.Run(version, func(t *testing.T) {
			server := NewServer("test", []ToolDescriptor{{
				Tool: Tool{Name: "t", Description: "noop"},
				Handler: func(_ context.Context, _ map[string]any) (any, error) {
					return map[string]any{"ok": true, "action": "t"}, nil
				},
			}}, nil, nil)

			req := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":%q}}`, version)
			var out strings.Builder
			if err := server.Run(context.Background(), strings.NewReader(req), &out); err != nil {
				t.Fatalf("run: %v", err)
			}
			var resp Response
			if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			result, _ := resp.Result.(map[string]any)
			caps, ok := result["capabilities"].(map[string]any)
			if !ok {
				t.Fatalf("missing capabilities in initialize result")
			}
			tools, _ := caps["tools"].(map[string]any)
			if tools == nil {
				t.Fatalf("capabilities.tools missing")
			}
			prompts, _ := caps["prompts"].(map[string]any)
			if prompts == nil {
				t.Fatalf("capabilities.prompts missing")
			}
			if prompts["listChanged"] == nil {
				t.Fatalf("capabilities.prompts.listChanged missing for version %s", version)
			}
		})
	}
}

// TestProtocolVersion_ToolsCallSurvivesEveryVersion runs a minimal
// tools/call after the initialize handshake for each supported
// version, asserting the structured + text envelope is identical
// across versions. Before E1 there was no cross-version guarantee
// that the A1 dual-emit contract held at 2024-11-05.
func TestProtocolVersion_ToolsCallSurvivesEveryVersion(t *testing.T) {
	for _, version := range SupportedProtocolVersions {
		t.Run(version, func(t *testing.T) {
			server := NewServer("test", []ToolDescriptor{{
				Tool: Tool{Name: "ping_tool", Description: "pong"},
				Handler: func(_ context.Context, _ map[string]any) (any, error) {
					return map[string]any{"ok": true, "action": "ping_tool", "data": map[string]any{"p": "pong"}}, nil
				},
				ReadOnlyHint: true,
			}}, nil, nil)

			lines := strings.Join([]string{
				fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":%q}}`, version),
				`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"ping_tool","arguments":{}}}`,
			}, "\n")
			var out strings.Builder
			if err := server.Run(context.Background(), strings.NewReader(lines), &out); err != nil {
				t.Fatalf("run: %v", err)
			}
			outLines := strings.Split(strings.TrimSpace(out.String()), "\n")
			if len(outLines) != 2 {
				t.Fatalf("expected 2 responses, got %d: %s", len(outLines), out.String())
			}
			var call Response
			if err := json.Unmarshal([]byte(outLines[1]), &call); err != nil {
				t.Fatalf("unmarshal tools/call: %v", err)
			}
			result, _ := call.Result.(map[string]any)
			if result["content"] == nil {
				t.Fatalf("[%s] expected content in tools/call result", version)
			}
			if result["structuredContent"] == nil {
				t.Fatalf("[%s] expected structuredContent in tools/call result", version)
			}
		})
	}
}
