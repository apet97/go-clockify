package mcp

import (
	"context"
	"strings"
	"testing"
)

type fakeUpstreamError struct {
	verbose string
}

func (e *fakeUpstreamError) Error() string { return e.verbose }

// TestToolErrorsExposeVerboseMessage locks in the one-user behavior: tool
// errors flow through to the MCP client unchanged for local diagnostics.
func TestToolErrorsExposeVerboseMessage(t *testing.T) {
	wantBody := "insufficient permissions " + "ten" + "ant=acme-internal"
	server := NewServer("test", []ToolDescriptor{{
		Tool: Tool{Name: "leaky"},
		Handler: func(context.Context, map[string]any) (any, error) {
			return nil, &fakeUpstreamError{
				verbose: "clockify PUT /workspaces/ws1/invoices/inv1 failed: 403 Forbidden: " + wantBody,
			}
		},
	}}, nil, nil)

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"leaky","arguments":{}}}`,
	}, "\n")
	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, wantBody) {
		t.Fatalf("local-mode response should include verbose body, got: %s", got)
	}
}
