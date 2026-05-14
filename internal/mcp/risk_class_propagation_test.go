//go:build legacy_platform

package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

// recordingEnforcement captures the ToolHints passed to FilterTool and
// BeforeCall so the test can assert RiskClass is forwarded from the
// ToolDescriptor through both surfaces. It otherwise allows every call.
type recordingEnforcement struct {
	mu             sync.Mutex
	filterHints    map[string]ToolHints
	beforeCallHits map[string]ToolHints
}

func newRecordingEnforcement() *recordingEnforcement {
	return &recordingEnforcement{
		filterHints:    map[string]ToolHints{},
		beforeCallHits: map[string]ToolHints{},
	}
}

func (e *recordingEnforcement) FilterTool(name string, hints ToolHints) bool {
	e.mu.Lock()
	e.filterHints[name] = hints
	e.mu.Unlock()
	return true
}

func (e *recordingEnforcement) BeforeCall(_ context.Context, name string, _ map[string]any, hints ToolHints, _ map[string]any, _ func(string) (ToolHandler, bool)) (any, func(), error) {
	e.mu.Lock()
	e.beforeCallHits[name] = hints
	e.mu.Unlock()
	return nil, nil, nil
}

func (e *recordingEnforcement) AfterCall(result any) (any, error) {
	return result, nil
}

// TestCallToolPropagatesRiskClassIntoHints is the Q3 prerequisite pin
// from docs/adr/0018-risk-class-confirmation-tokens.md: every place
// the server constructs a ToolHints literal must populate RiskClass
// from the ToolDescriptor. Without this, a future risk-aware policy
// gate or confirmation-token enforcer would have to re-derive the
// class from the tool name string, which defeats the point of the
// taxonomy.
//
// The test sets up two descriptors — one tagged RiskRead, one tagged
// RiskDestructive — registers them on a Server with a recording
// Enforcement, then exercises both code paths:
//
//  1. tools/list goes through buildToolListLocked → FilterTool.
//  2. tools/call goes through callTool → BeforeCall (and onward to
//     AuditKeys / invokeHandler).
//
// Both paths must surface the descriptor's RiskClass verbatim.
func TestCallToolPropagatesRiskClassIntoHints(t *testing.T) {
	readTool := ToolDescriptor{
		Tool: Tool{
			Name:        "risk_read_probe",
			Description: "read probe",
			InputSchema: map[string]any{"type": "object"},
			Annotations: map[string]any{"readOnlyHint": true},
		},
		Handler:      func(ctx context.Context, _ map[string]any) (any, error) { return "ok", nil },
		ReadOnlyHint: true,
		RiskClass:    RiskRead,
	}
	destructiveTool := ToolDescriptor{
		Tool: Tool{
			Name:        "risk_destructive_probe",
			Description: "destructive probe",
			InputSchema: map[string]any{"type": "object"},
			Annotations: map[string]any{"readOnlyHint": false, "destructiveHint": true},
		},
		Handler:         func(ctx context.Context, _ map[string]any) (any, error) { return "ok", nil },
		ReadOnlyHint:    false,
		DestructiveHint: true,
		RiskClass:       RiskDestructive,
	}

	enf := newRecordingEnforcement()
	server := NewServer("test", []ToolDescriptor{readTool, destructiveTool}, enf, nil)
	server.initialized.Store(true) // skip init guard — we're testing the hint plumbing

	// Cover the tools/list path: buildToolListLocked → FilterTool.
	tools := server.listTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	enf.mu.Lock()
	gotReadFilter, hasRead := enf.filterHints["risk_read_probe"]
	gotDestructiveFilter, hasDestructive := enf.filterHints["risk_destructive_probe"]
	enf.mu.Unlock()
	if !hasRead || !hasDestructive {
		t.Fatalf("FilterTool was not called for both descriptors: read=%v destructive=%v",
			hasRead, hasDestructive)
	}
	if gotReadFilter.RiskClass != RiskRead {
		t.Errorf("FilterTool hints.RiskClass for read probe: got %b want %b (RiskRead)",
			gotReadFilter.RiskClass, RiskRead)
	}
	if gotDestructiveFilter.RiskClass != RiskDestructive {
		t.Errorf("FilterTool hints.RiskClass for destructive probe: got %b want %b (RiskDestructive)",
			gotDestructiveFilter.RiskClass, RiskDestructive)
	}

	// Cover the tools/call path: callTool → BeforeCall.
	for _, name := range []string{"risk_read_probe", "risk_destructive_probe"} {
		input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + name + `","arguments":{}}}`
		var out strings.Builder
		if err := server.Run(context.Background(), strings.NewReader(input), &out); err != nil {
			t.Fatalf("run failed for %s: %v", name, err)
		}
		var resp Response
		if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &resp); err != nil {
			t.Fatalf("unmarshal %s: %v", name, err)
		}
		if resp.Error != nil {
			t.Fatalf("tools/call %s returned JSON-RPC error: %+v", name, resp.Error)
		}
	}
	enf.mu.Lock()
	gotReadCall := enf.beforeCallHits["risk_read_probe"]
	gotDestructiveCall := enf.beforeCallHits["risk_destructive_probe"]
	enf.mu.Unlock()
	if gotReadCall.RiskClass != RiskRead {
		t.Errorf("BeforeCall hints.RiskClass for read probe: got %b want %b (RiskRead)",
			gotReadCall.RiskClass, RiskRead)
	}
	if gotDestructiveCall.RiskClass != RiskDestructive {
		t.Errorf("BeforeCall hints.RiskClass for destructive probe: got %b want %b (RiskDestructive)",
			gotDestructiveCall.RiskClass, RiskDestructive)
	}
	// Sanity: the read-tool BeforeCall hint must reflect ReadOnly=true
	// and Destructive=false — proves nothing else regressed in the
	// hint copy site.
	if !gotReadCall.ReadOnly || gotReadCall.Destructive {
		t.Errorf("BeforeCall hints for read probe: got ReadOnly=%v Destructive=%v want ReadOnly=true Destructive=false",
			gotReadCall.ReadOnly, gotReadCall.Destructive)
	}
	if gotDestructiveCall.ReadOnly || !gotDestructiveCall.Destructive {
		t.Errorf("BeforeCall hints for destructive probe: got ReadOnly=%v Destructive=%v want ReadOnly=false Destructive=true",
			gotDestructiveCall.ReadOnly, gotDestructiveCall.Destructive)
	}
}
