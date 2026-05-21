package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

// TestRealRegistrySchemaRejectsMissingRequiredArgs exercises runtime JSON-schema
// validation at the MCP boundary against the real startup registry — not a
// synthetic probe tool. For every registered tool that declares required input
// fields, a tools/call with empty arguments must be rejected with JSON-RPC
// -32602 and a JSON Pointer to the offending field, before the handler runs.
// (MCP axiom 29: the MCP boundary must be tested directly.)
func TestRealRegistrySchemaRejectsMissingRequiredArgs(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	reg := svc.FullAccessRegistry()
	server := mcp.NewServer("test", reg)
	if _, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	tested := 0
	for _, descriptor := range reg {
		required := inputSchemaRequiredFields(descriptor.Tool.InputSchema)
		if len(required) == 0 {
			continue
		}
		tested++
		name := descriptor.Tool.Name

		call, err := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/call",
			"params":  map[string]any{"name": name, "arguments": map[string]any{}},
		})
		if err != nil {
			t.Fatalf("%s: marshal request: %v", name, err)
		}
		raw, err := server.DispatchMessage(context.Background(), call)
		if err != nil {
			t.Fatalf("%s: dispatch: %v", name, err)
		}
		var resp struct {
			Error *struct {
				Code int            `json:"code"`
				Data map[string]any `json:"data"`
			} `json:"error"`
			Result json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			t.Fatalf("%s: unmarshal response: %v", name, err)
		}
		if resp.Error == nil {
			t.Errorf("%s: tools/call with {} args returned no JSON-RPC error; want -32602 (required: %v)", name, required)
			continue
		}
		if resp.Error.Code != -32602 {
			t.Errorf("%s: error.code = %d, want -32602 InvalidParams", name, resp.Error.Code)
		}
		if ptr, _ := resp.Error.Data["pointer"].(string); ptr == "" {
			t.Errorf("%s: error.data.pointer is empty; want a JSON Pointer to the missing required field", name)
		}
		if len(resp.Result) != 0 && string(resp.Result) != "null" {
			t.Errorf("%s: expected no result on the -32602 path, got %s", name, resp.Result)
		}
	}
	if tested == 0 {
		t.Fatal("no registry tools with required fields were exercised — registry build or schema introspection regressed")
	}
}

func TestUsersInviteAcceptsDryRunAtSchemaGate(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	server := mcp.NewServer("test", svc.FullAccessRegistry())
	if _, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	call, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "clockify_users_invite",
			"arguments": map[string]any{
				"email":   "invitee" + "@example.invalid",
				"dry_run": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	raw, err := server.DispatchMessage(context.Background(), call)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	var resp struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("clockify_users_invite dry_run was rejected at schema gate: code=%d message=%q", resp.Error.Code, resp.Error.Message)
	}
	if len(resp.Result) == 0 || string(resp.Result) == "null" {
		t.Fatalf("expected dry-run result, got %s", resp.Result)
	}
}

func TestUsersRoleRejectsMalformedRoleGrantAtSchemaGate(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	server := mcp.NewServer("test", svc.FullAccessRegistry())
	if _, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	call, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "clockify_users_role",
			"arguments": map[string]any{
				"user_id":     "000000000000000000000001",
				"role":        "REGULAR",
				"dry_run":     true,
				"role_grants": []any{map[string]any{}},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	raw, err := server.DispatchMessage(context.Background(), call)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	var resp struct {
		Error *struct {
			Code int            `json:"code"`
			Data map[string]any `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("malformed role_grants item reached handler; want schema rejection")
	}
	if resp.Error.Code != -32602 {
		t.Fatalf("error.code = %d, want -32602", resp.Error.Code)
	}
	if ptr, _ := resp.Error.Data["pointer"].(string); !strings.Contains(ptr, "/role_grants/0") {
		t.Fatalf("error pointer = %q, want role_grants item pointer", ptr)
	}
}

// inputSchemaRequiredFields returns the declared required-field names of a
// tool input schema, tolerating both the []string descriptors are built with
// and the []any a JSON round-trip would yield.
func inputSchemaRequiredFields(schema map[string]any) []string {
	if schema == nil {
		return nil
	}
	switch req := schema["required"].(type) {
	case []string:
		return req
	case []any:
		out := make([]string, 0, len(req))
		for _, v := range req {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
