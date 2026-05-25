package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/jsonschema"
	"github.com/apet97/go-clockify/internal/mcp"
)

// dispatchToolCall sends one tools/call request through the MCP server and
// returns the decoded `result` object.
func dispatchToolCall(t *testing.T, srv *mcp.Server, id int, tool string, args map[string]any) map[string]any {
	t.Helper()
	if args == nil {
		args = map[string]any{}
	}
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "tools/call",
		"params":  map[string]any{"name": tool, "arguments": args},
	}
	msg, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	respBytes, err := srv.DispatchMessage(context.Background(), msg)
	if err != nil {
		t.Fatalf("DispatchMessage(%s): %v", tool, err)
	}
	var resp struct {
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("unmarshal response for %s: %v", tool, err)
	}
	return resp.Result
}

// TestRateLimitedStructuredContentConformsToOutputSchema forces the per-process
// token bucket empty, then confirms the rate_limited envelope the runtime
// returns is exposed as `structuredContent` and validates against the output
// schema of a read tool, an ordinary write tool, and a destructive tool.
func TestRateLimitedStructuredContentConformsToOutputSchema(t *testing.T) {
	svc := New(clockify.NewClient("k", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	reg := mustRegistry(t, svc)

	schemas := make(map[string]map[string]any, len(reg))
	for _, d := range reg {
		schemas[d.Tool.Name] = d.Tool.OutputSchema
	}

	cases := []struct {
		id   int
		tool string
		kind string
	}{
		{2, "clockify_status", "read"},
		{3, "clockify_clients_create", "ordinary write"},
		{4, "clockify_clients_delete", "destructive"},
	}
	for _, tc := range cases {
		srv := mcp.NewServer("test", reg)
		srv.ConfigureToolLimits(1) // token bucket: capacity 1 per risk bucket
		srv.MarkInitialized("2025-06-18", "rate-limit-test", "1")

		// Drain the selected tool's risk bucket. The rate limiter is checked
		// before schema validation and before the handler, so even empty
		// arguments consume the bucket token.
		_ = dispatchToolCall(t, srv, tc.id*10, tc.tool, nil)
		result := dispatchToolCall(t, srv, tc.id*10+1, tc.tool, nil)
		structured, ok := result["structuredContent"].(map[string]any)
		if !ok {
			t.Fatalf("%s (%s): rate-limited result has no structuredContent object: %v", tc.tool, tc.kind, result)
		}
		errObj, _ := structured["error"].(map[string]any)
		if errObj == nil || errObj["code"] != "rate_limited" {
			t.Fatalf("%s (%s): expected a rate_limited envelope, got %v", tc.tool, tc.kind, structured)
		}
		schema := schemas[tc.tool]
		if schema == nil {
			t.Fatalf("%s: not found in registry or has no output schema", tc.tool)
		}
		if err := jsonschema.Validate(schema, structured); err != nil {
			t.Errorf("%s (%s): rate_limited structuredContent fails the tool's output schema: %v", tc.tool, tc.kind, err)
		}
	}
}
