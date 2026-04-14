package tools_test

// Helper for dispatching Tier 2 tool calls through the full MCP pipeline in
// integration tests. testharness.Invoke only registers Tier 1 descriptors via
// svc.Registry(); Tier 2 tools are gated behind clockify_search_tools
// activation and would surface as "unknown tool" errors if invoked through
// the bare harness. dispatchTier2 mirrors testharness.Invoke's construction
// (real clockify.Client → real Service → real enforcement.Pipeline → real
// mcp.Server) but pre-activates the requested Tier 2 group's descriptors so
// the dispatcher can route the call. The intent of the harness rule — never
// bypass dispatch by calling svc.Foo directly — is preserved: every call
// here still flows through DispatchMessage, schema validation, the
// enforcement pipeline, and the registered handler closure.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/enforcement"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/testharness"
	"github.com/apet97/go-clockify/internal/tools"
)

// tier2InvokeOpts is a strict subset of testharness.InvokeOpts plus the
// Tier 2 group name that must be activated before the call.
type tier2InvokeOpts struct {
	Group       string
	Tool        string
	Args        map[string]any
	PolicyMode  policy.Mode
	Upstream    *testharness.FakeClockify
	WorkspaceID string
}

// dispatchTier2 builds a one-shot dispatch path that includes both the Tier 1
// registry and the activated Tier 2 group, then issues an initialize +
// tools/call sequence. The return shape matches testharness.InvokeResult so
// assertions in the per-group test files read identically to dispatch_test.go.
func dispatchTier2(t *testing.T, opts tier2InvokeOpts) testharness.InvokeResult {
	t.Helper()
	if opts.Tool == "" {
		t.Fatalf("dispatchTier2: Tool is required")
	}
	if opts.Group == "" {
		t.Fatalf("dispatchTier2: Group is required")
	}
	if opts.Upstream == nil {
		t.Fatalf("dispatchTier2: Upstream is required")
	}
	if opts.WorkspaceID == "" {
		opts.WorkspaceID = "test-workspace"
	}
	if opts.PolicyMode == "" {
		opts.PolicyMode = policy.Standard
	}

	client := clockify.NewClient("test-api-key", opts.Upstream.URL(), 5*time.Second, 0)
	defer client.Close()

	svc := tools.New(client, opts.WorkspaceID)

	tier2Descs, ok := svc.Tier2Handlers(opts.Group)
	if !ok {
		t.Fatalf("dispatchTier2: unknown group %q", opts.Group)
	}

	descriptors := append(svc.Registry(), tier2Descs...)

	pipeline := &enforcement.Pipeline{
		Policy: &policy.Policy{Mode: opts.PolicyMode, DeniedTools: map[string]bool{}},
	}

	server := mcp.NewServer("tier2-dispatch-helper", descriptors, pipeline, nil)

	ctx := context.Background()

	initMsg := mustJSON(t, mcp.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": mcp.SupportedProtocolVersions[0],
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "tier2-helper", "version": "0"},
		},
	})
	if _, err := server.DispatchMessage(ctx, initMsg); err != nil {
		t.Fatalf("dispatchTier2: initialize failed: %v", err)
	}

	before := opts.Upstream.RequestCount()

	callMsg := mustJSON(t, mcp.Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: mcp.ToolCallParams{
			Name:      opts.Tool,
			Arguments: opts.Args,
		},
	})

	raw, err := server.DispatchMessage(ctx, callMsg)
	if err != nil {
		t.Fatalf("dispatchTier2: dispatch error for %s: %v", opts.Tool, err)
	}

	after := opts.Upstream.RequestCount()

	var envelope mcp.Response
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("dispatchTier2: response unmarshal: %v (raw=%s)", err, string(raw))
	}

	result := testharness.InvokeResult{
		Raw:         raw,
		UpstreamHit: after > before,
	}

	if envelope.Error != nil {
		result.RPCError = envelope.Error
		result.ErrorMessage = envelope.Error.Message
		switch envelope.Error.Code {
		case -32602:
			result.Outcome = testharness.OutcomeInvalidParams
		default:
			result.Outcome = testharness.OutcomeProtocolError
		}
		return result
	}

	resultMap, ok := envelope.Result.(map[string]any)
	if !ok {
		t.Fatalf("dispatchTier2: unexpected result type %T", envelope.Result)
	}
	result.Result = resultMap

	if content, ok := resultMap["content"].([]any); ok && len(content) > 0 {
		if first, ok := content[0].(map[string]any); ok {
			if text, ok := first["text"].(string); ok {
				result.ResultText = text
			}
		}
	}

	if isErr, _ := resultMap["isError"].(bool); isErr {
		result.IsError = true
		result.ErrorMessage = result.ResultText
		if len(result.ErrorMessage) >= len("tool blocked by policy") &&
			result.ErrorMessage[:len("tool blocked by policy")] == "tool blocked by policy" {
			result.Outcome = testharness.OutcomePolicyDenied
		} else {
			result.Outcome = testharness.OutcomeToolError
		}
		return result
	}

	result.Outcome = testharness.OutcomeSuccess
	return result
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("dispatchTier2: marshal: %v", err)
	}
	return b
}
