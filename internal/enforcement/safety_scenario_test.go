package enforcement

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/policy"
)

type mockAuditor struct {
	recorded bool
	err      error
}

func (m *mockAuditor) RecordAudit(event mcp.AuditEvent) error {
	m.recorded = true
	return m.err
}

func TestSafetyScenarios(t *testing.T) {
	tests := []struct {
		name            string
		toolName        string
		hints           mcp.ToolHints
		policy          *policy.Policy
		dryRunEnabled   bool
		args            map[string]any
		handler         mcp.ToolHandler
		expectError     string
		expectDryRun    bool
		expectAudited   bool
		auditDurability string
	}{
		{
			name:          "destructive_tool_with_dry_run_true_intercepted",
			toolName:      "clockify_delete_tag",
			hints:         mcp.ToolHints{Destructive: true, ReadOnly: false},
			policy:        standardPolicy(),
			dryRunEnabled: true,
			args:          map[string]any{"dry_run": true, "tag_id": "t1"},
			expectDryRun:  true,
		},
		{
			name:          "destructive_tool_blocked_by_readonly_policy",
			toolName:      "clockify_delete_tag",
			hints:         mcp.ToolHints{Destructive: true, ReadOnly: false},
			policy:        readOnlyPolicy(),
			dryRunEnabled: true,
			args:          map[string]any{"dry_run": true, "tag_id": "t1"},
			expectError:   "tool blocked by policy",
		},
		{
			name:     "ambiguous_name_resolution_fails_closed",
			toolName: "clockify_get_project",
			hints:    mcp.ToolHints{ReadOnly: true},
			policy:   standardPolicy(),
			args:     map[string]any{"project": "Ambiguous"},
			handler: func(ctx context.Context, args map[string]any) (any, error) {
				return nil, errors.New("multiple projects match 'Ambiguous' (2 found). Use the full project ID instead")
			},
			expectError: "multiple projects match 'Ambiguous'",
		},
		{
			name:     "audit_on_successful_non_readonly_call",
			toolName: "clockify_add_entry",
			hints:    mcp.ToolHints{ReadOnly: false},
			policy:   standardPolicy(),
			args:     map[string]any{"description": "work"},
			handler: func(ctx context.Context, args map[string]any) (any, error) {
				return map[string]any{"id": "e1"}, nil
			},
			expectAudited: true,
		},
		{
			name:     "audit_fail_closed_aborts_on_persist_error",
			toolName: "clockify_add_entry",
			hints:    mcp.ToolHints{ReadOnly: false},
			policy:   standardPolicy(),
			args:     map[string]any{"description": "work"},
			handler: func(ctx context.Context, args map[string]any) (any, error) {
				return map[string]any{"id": "e1"}, nil
			},
			auditDurability: "fail_closed",
			// Two-phase audit (2026-04-25 H5 refactor) short-circuits on
			// the intent record, so the error message now names the
			// intent phase explicitly.
			expectError: "audit intent persistence failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pipeline := &Pipeline{
				Policy: tc.policy,
				DryRun: dryrun.Config{Enabled: tc.dryRunEnabled},
			}

			auditor := &mockAuditor{}
			if tc.name == "audit_fail_closed_aborts_on_persist_error" {
				auditor.err = errors.New("db down")
			}

			server := mcp.NewServer("v1", nil, pipeline, nil)
			server.Auditor = auditor
			server.AuditDurabilityMode = tc.auditDurability

			// Add the tool to server
			h := tc.handler
			if h == nil {
				h = func(ctx context.Context, args map[string]any) (any, error) {
					return map[string]any{"ok": true}, nil
				}
			}

			server.ActivateGroup("test", []mcp.ToolDescriptor{
				{
					Tool:            mcp.Tool{Name: tc.toolName},
					Handler:         h,
					ReadOnlyHint:    tc.hints.ReadOnly,
					DestructiveHint: tc.hints.Destructive,
				},
			})

			// Call initialize so tools/call is allowed
			server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","method":"initialize","id":1,"params":{"protocolVersion":"2025-06-18"}}`))

			reqMsg, _ := json.Marshal(mcp.Request{
				JSONRPC: "2.0",
				ID:      2,
				Method:  "tools/call",
				Params: mcp.ToolCallParams{
					Name:      tc.toolName,
					Arguments: tc.args,
				},
			})

			respBytes, err := server.DispatchMessage(context.Background(), reqMsg)
			if err != nil {
				t.Fatalf("DispatchMessage failed: %v", err)
			}

			var resp mcp.Response
			if err := json.Unmarshal(respBytes, &resp); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if tc.expectError != "" {
				// Tool errors in MCP are returned in the result with isError: true,
				// OR as JSON-RPC errors if they are protocol/enforcement errors.
				var errStr string
				if resp.Error != nil {
					errStr = resp.Error.Message
				} else if m, ok := resp.Result.(map[string]any); ok {
					if isErr, _ := m["isError"].(bool); isErr {
						if content, ok := m["content"].([]any); ok && len(content) > 0 {
							if c0, ok := content[0].(map[string]any); ok {
								errStr, _ = c0["text"].(string)
							}
						}
					}
				}

				if errStr == "" {
					t.Fatal("expected error, got success")
				}
				if !strings.Contains(errStr, tc.expectError) {
					t.Fatalf("expected error containing %q, got %q", tc.expectError, errStr)
				}
				return
			}

			if resp.Error != nil {
				t.Fatalf("unexpected JSON-RPC error: %v", resp.Error.Message)
			}

			if tc.expectDryRun {
				resMap, ok := resp.Result.(map[string]any)
				if !ok {
					t.Fatalf("expected map result, got %T", resp.Result)
				}
				sc := resMap["structuredContent"]
				if sc == nil {
					t.Fatal("expected structuredContent in result")
				}
				m, ok := sc.(map[string]any)
				if !ok {
					t.Fatalf("expected map in structuredContent, got %T", sc)
				}
				if m["dry_run"] != true {
					t.Fatalf("expected dry_run=true in result, got %+v", m)
				}
			}

			if tc.expectAudited && !auditor.recorded {
				t.Fatal("expected audit to be recorded")
			}
		})
	}
}
