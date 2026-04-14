// Package testharness wires the real MCP dispatch path (auth → enforcement
// pipeline → tool handler → Clockify client) against a fake Clockify upstream
// so tests can assert policy, auth, and idempotency properties end-to-end
// without bypassing the layers that enforce them.
//
// Existing service-layer tests in internal/tools call `svc.Foo(ctx, args)`
// directly — that path is fine for happy-path coverage but skips the
// enforcement pipeline, which is precisely where policy and rate-limit
// regressions live. Use testharness.Invoke for tests whose premise is
// "this call SHOULD/SHOULD NOT reach Clockify" — the UpstreamHit field on
// the result is the canonical way to assert the call was rejected before
// an HTTP request was emitted.
package testharness

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/enforcement"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/tools"
)

// FakeClockify is a counted wrapper around httptest.Server. The harness reads
// the request count before and after each Invoke to decide whether the call
// reached Clockify (UpstreamHit) — this is how tests assert "policy rejected
// the call before any HTTP request was made."
type FakeClockify struct {
	server *httptest.Server
	count  atomic.Int64
}

// NewFakeClockify constructs a counted httptest.Server fronting the supplied
// handler. The server is automatically closed via t.Cleanup.
//
// Accepts testing.TB so benchmarks (writes_bench_test.go) can reuse the same
// fake-upstream wiring without duplicating the httptest plumbing. *testing.T
// continues to satisfy testing.TB transparently for existing callers.
func NewFakeClockify(t testing.TB, handler http.Handler) *FakeClockify {
	t.Helper()
	f := &FakeClockify{}
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.count.Add(1)
		if handler != nil {
			handler.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
	f.server = httptest.NewServer(wrapped)
	t.Cleanup(f.server.Close)
	return f
}

// URL returns the base URL of the fake upstream, suitable for passing to
// clockify.NewClient.
func (f *FakeClockify) URL() string { return f.server.URL }

// RequestCount returns the total number of HTTP requests received since
// construction. Tests usually don't call this directly — Invoke compares
// it against a pre-call snapshot to populate InvokeResult.UpstreamHit.
func (f *FakeClockify) RequestCount() int64 { return f.count.Load() }

// Reset zeroes the request counter. Useful when a test invokes the harness
// multiple times against the same fake and wants independent UpstreamHit
// assertions without tearing down the upstream between calls.
func (f *FakeClockify) Reset() { f.count.Store(0) }

// InvokeOpts configures a single tool-call dispatch through the full MCP
// pipeline. Only Tool and Upstream are required; everything else has a sane
// default that mirrors the Standard production profile.
type InvokeOpts struct {
	// Tool is the MCP tool name (e.g. "clockify_delete_entry"). Required.
	Tool string
	// Args is the tools/call arguments map. Nil means an empty args object.
	Args map[string]any
	// PolicyMode defaults to policy.Standard. Set to policy.ReadOnly or
	// policy.SafeCore to assert that write tools are rejected before the
	// handler runs.
	PolicyMode policy.Mode
	// DeniedTools is an optional per-call override for the policy deny list.
	// Lets a single test assert that an explicitly-denied tool is rejected
	// under an otherwise-permissive mode.
	DeniedTools []string
	// Principal is attached to the call context via authn.WithPrincipal.
	// Nil means no principal (rate-limit falls back to the global scope);
	// set this to simulate per-subject rate limiting or auth failures that
	// still flow through the dispatcher.
	Principal *authn.Principal
	// Upstream is the fake Clockify server the tool's HTTP client talks to.
	// Required — the harness panics if nil, because there's nothing sensible
	// to default here.
	Upstream *FakeClockify
	// ClockifyAPIKey is the bearer key the clockify client sends upstream.
	// Defaults to "test-api-key". The fake upstream decides whether to
	// reject it — tests that want to assert upstream auth errors provide a
	// handler that returns 401 for mismatched keys.
	ClockifyAPIKey string
	// WorkspaceID is written into tools.Service.WorkspaceID. Defaults to
	// "test-workspace".
	WorkspaceID string
	// RequestID is the JSON-RPC request id. Defaults to 1; set this when a
	// test needs to correlate multiple invocations in the same log stream.
	RequestID int
	// Client lets callers supply a pre-built clockify.Client whose HTTP
	// transport is reused across many Invoke calls. Default behaviour is
	// "construct a fresh client per call" (correct for tests where each
	// case must be independent). Benchmarks pass a shared client so they
	// don't burn an ephemeral port per iteration — without this the
	// loopback exhausts its port range after a few thousand calls.
	Client *clockify.Client
}

// InvokeResult captures everything a test wants to assert about one dispatch.
// The canonical assertion for "policy blocked the call before any HTTP
// request was made" is `!result.UpstreamHit && result.Outcome == OutcomePolicyDenied`.
type InvokeResult struct {
	// Result is the decoded tools/call result envelope. For a successful
	// call this is `{"content":[{"type":"text","text":"<json>"}]}`; tests
	// that need the tool-specific shape should decode the "text" field.
	// Nil when the call failed at the JSON-RPC protocol layer (-32xxx).
	Result map[string]any
	// ResultText is the JSON text returned inside Result.content[0].text
	// when the call succeeded. Tests that want to assert on the concrete
	// tool response shape typically json.Unmarshal this. Empty string when
	// the call errored.
	ResultText string
	// IsError is the MCP spec's tool-error flag on the result envelope.
	// True when the tool handler (or the enforcement pipeline) returned an
	// error without triggering a JSON-RPC -32xxx protocol error. Policy
	// denials, handler errors, and upstream 4xx/5xx all surface here.
	IsError bool
	// ErrorMessage is the human-readable error string extracted from either
	// the JSON-RPC error envelope or the isError:true content block. Empty
	// on success.
	ErrorMessage string
	// RPCError is the JSON-RPC protocol error (schema validation -32602,
	// uninitialized server -32002, etc.) or nil when the call went through.
	// Tool errors come back as IsError:true on the result envelope, NOT as
	// a JSON-RPC error.
	RPCError *mcp.RPCError
	// Outcome classifies the call result for assertion. See the Outcome*
	// constants below.
	Outcome Outcome
	// UpstreamHit reports whether any HTTP request reached the fake
	// Clockify server during this call. False when the enforcement pipeline
	// rejected the call (policy, rate limit, schema validation) before the
	// handler had a chance to run.
	UpstreamHit bool
	// Raw is the full JSON-RPC response bytes in case a test needs more
	// detail than the structured fields expose.
	Raw []byte
}

// Outcome classifies how a dispatch call ended. Kept as a string type (not
// an int enum) so test failure messages are self-explanatory.
type Outcome string

const (
	// OutcomeSuccess means the tool handler ran and returned without error.
	OutcomeSuccess Outcome = "success"
	// OutcomePolicyDenied means the enforcement pipeline rejected the call
	// because the policy mode forbids this tool. UpstreamHit is false.
	OutcomePolicyDenied Outcome = "policy_denied"
	// OutcomeInvalidParams means the request failed JSON schema validation
	// (JSON-RPC -32602). UpstreamHit is false.
	OutcomeInvalidParams Outcome = "invalid_params"
	// OutcomeToolError means the tool handler returned an error (upstream
	// 4xx/5xx, business rule violation, etc.). UpstreamHit is usually true
	// — the error originated from a completed HTTP exchange.
	OutcomeToolError Outcome = "tool_error"
	// OutcomeProtocolError means the JSON-RPC layer rejected the request
	// (unknown method, uninitialized server, etc.) before the dispatcher ran.
	OutcomeProtocolError Outcome = "protocol_error"
)

// Invoke dispatches a single tools/call through a freshly constructed server
// with real enforcement, a real tools.Service, and a Clockify client pointed
// at opts.Upstream. The server is initialized (initialize → initialized flag)
// before the call so tools/call passes the spec-compliance guard.
//
// Each Invoke gets a fresh server so independent calls can't leak state
// through the dispatcher. Tests that want shared state across calls should
// share the *FakeClockify upstream and assert on its RequestCount directly.
func Invoke(t testing.TB, opts InvokeOpts) InvokeResult {
	t.Helper()

	if opts.Tool == "" {
		t.Fatalf("testharness: Tool is required")
	}
	if opts.Upstream == nil {
		t.Fatalf("testharness: Upstream is required (use NewFakeClockify)")
	}
	if opts.ClockifyAPIKey == "" {
		opts.ClockifyAPIKey = "test-api-key"
	}
	if opts.WorkspaceID == "" {
		opts.WorkspaceID = "test-workspace"
	}
	if opts.PolicyMode == "" {
		opts.PolicyMode = policy.Standard
	}
	if opts.RequestID == 0 {
		opts.RequestID = 1
	}

	client := opts.Client
	if client == nil {
		client = clockify.NewClient(opts.ClockifyAPIKey, opts.Upstream.URL(), 5*time.Second, 0)
		defer client.Close()
	}

	svc := tools.New(client, opts.WorkspaceID)
	descriptors := svc.Registry()

	pol := &policy.Policy{
		Mode:        opts.PolicyMode,
		DeniedTools: map[string]bool{},
	}
	for _, name := range opts.DeniedTools {
		pol.DeniedTools[name] = true
	}

	pipeline := &enforcement.Pipeline{
		Policy: pol,
	}

	server := mcp.NewServer("testharness", descriptors, pipeline, nil)

	ctx := context.Background()
	if opts.Principal != nil {
		ctx = authn.WithPrincipal(ctx, opts.Principal)
	}

	// Flip the initialized flag by sending a real initialize request —
	// cheaper than exposing a test-only setter on Server and keeps the
	// harness exercising the real wire protocol.
	initMsg := mustMarshal(t, mcp.Request{
		JSONRPC: "2.0",
		ID:      opts.RequestID,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": mcp.SupportedProtocolVersions[0],
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "testharness", "version": "0"},
		},
	})
	if _, err := server.DispatchMessage(ctx, initMsg); err != nil {
		t.Fatalf("testharness: initialize failed: %v", err)
	}

	before := opts.Upstream.RequestCount()

	callMsg := mustMarshal(t, mcp.Request{
		JSONRPC: "2.0",
		ID:      opts.RequestID + 1,
		Method:  "tools/call",
		Params: mcp.ToolCallParams{
			Name:      opts.Tool,
			Arguments: opts.Args,
		},
	})

	raw, err := server.DispatchMessage(ctx, callMsg)
	if err != nil {
		t.Fatalf("testharness: dispatch error for %s: %v", opts.Tool, err)
	}

	after := opts.Upstream.RequestCount()

	return parseDispatchResult(t, raw, after > before)
}

// parseDispatchResult decodes the JSON-RPC response envelope from
// server.DispatchMessage and classifies the outcome. Shared by Invoke
// and BenchHarness.Call so both routes produce behaviourally identical
// InvokeResult values.
func parseDispatchResult(t testing.TB, raw []byte, upstreamHit bool) InvokeResult {
	t.Helper()
	var envelope mcp.Response
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("testharness: response unmarshal: %v (raw=%s)", err, string(raw))
	}

	result := InvokeResult{
		Raw:         raw,
		UpstreamHit: upstreamHit,
	}

	if envelope.Error != nil {
		result.RPCError = envelope.Error
		result.ErrorMessage = envelope.Error.Message
		result.Outcome = classifyRPCError(envelope.Error)
		return result
	}

	resultMap, ok := envelope.Result.(map[string]any)
	if !ok {
		t.Fatalf("testharness: unexpected result type %T (raw=%s)", envelope.Result, string(raw))
	}
	result.Result = resultMap

	if text, ok := extractResultText(resultMap); ok {
		result.ResultText = text
	}

	if isErr, _ := resultMap["isError"].(bool); isErr {
		result.IsError = true
		result.ErrorMessage = result.ResultText
		result.Outcome = classifyToolError(result.ErrorMessage)
		return result
	}

	result.Outcome = OutcomeSuccess
	return result
}

// BenchHarness is an amortised variant of Invoke for benchmarks. It
// builds the full MCP stack (tools.Service, Registry, enforcement
// Pipeline, mcp.Server, initialized state) ONCE, then the Call method
// dispatches a single tools/call through the already-wired server.
//
// Why this helper exists: memory profiling of the tier-1 write bench
// (internal/tools/writes_bench_test.go) showed ~82% of per-iteration
// allocations came from Service.Registry() — the bench was measuring
// "cold server boot + one call" instead of "one call against a warm
// server." Amortising the setup makes the measurement reflect real
// per-dispatch cost the way production clients actually pay it.
//
// Production use of the full pipeline is NOT affected: the MCP server
// already calls Registry exactly once at startup. This helper only
// exists to stop benchmarks from paying that cost every iteration.
//
// Tests that assert policy / enforcement / auth properties must
// continue to use Invoke — each test case needs an isolated pipeline
// so that state (rate-limit counters, initialized flag) from a prior
// assertion does not leak into the next one.
type BenchHarness struct {
	tb        testing.TB
	server    *mcp.Server
	upstream  *FakeClockify
	requestID int
}

// NewBenchHarness wires the full MCP stack once and returns a handle
// whose Call method reuses it across iterations. See the BenchHarness
// doc for the rationale.
//
// opts.Tool is ignored at construction time (BenchHarness.Call takes
// the tool name per dispatch); everything else mirrors Invoke's
// defaults exactly so the measured path is identical.
func NewBenchHarness(tb testing.TB, opts InvokeOpts) *BenchHarness {
	tb.Helper()

	if opts.Upstream == nil {
		tb.Fatalf("testharness: Upstream is required (use NewFakeClockify)")
	}
	if opts.ClockifyAPIKey == "" {
		opts.ClockifyAPIKey = "test-api-key"
	}
	if opts.WorkspaceID == "" {
		opts.WorkspaceID = "test-workspace"
	}
	if opts.PolicyMode == "" {
		opts.PolicyMode = policy.Standard
	}
	if opts.RequestID == 0 {
		opts.RequestID = 1
	}

	client := opts.Client
	if client == nil {
		client = clockify.NewClient(opts.ClockifyAPIKey, opts.Upstream.URL(), 5*time.Second, 0)
		tb.Cleanup(client.Close)
	}

	svc := tools.New(client, opts.WorkspaceID)
	descriptors := svc.Registry()

	pol := &policy.Policy{
		Mode:        opts.PolicyMode,
		DeniedTools: map[string]bool{},
	}
	for _, name := range opts.DeniedTools {
		pol.DeniedTools[name] = true
	}

	pipeline := &enforcement.Pipeline{
		Policy: pol,
	}

	server := mcp.NewServer("testharness-bench", descriptors, pipeline, nil)

	// Flip the initialized flag once. Subsequent Call invocations reuse
	// this state — just like a real MCP client that initializes once per
	// session and then issues many tools/call messages.
	initMsg := mustMarshal(tb, mcp.Request{
		JSONRPC: "2.0",
		ID:      opts.RequestID,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": mcp.SupportedProtocolVersions[0],
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "testharness-bench", "version": "0"},
		},
	})
	if _, err := server.DispatchMessage(context.Background(), initMsg); err != nil {
		tb.Fatalf("testharness: initialize failed: %v", err)
	}

	return &BenchHarness{
		tb:        tb,
		server:    server,
		upstream:  opts.Upstream,
		requestID: opts.RequestID + 1,
	}
}

// Call dispatches one tools/call through the already-initialized MCP
// server. The returned InvokeResult is shape-identical to the one from
// Invoke. Call is safe to invoke from the benchmark's b.Loop() body.
func (h *BenchHarness) Call(ctx context.Context, tool string, args map[string]any) InvokeResult {
	h.tb.Helper()
	h.requestID++

	callMsg := mustMarshal(h.tb, mcp.Request{
		JSONRPC: "2.0",
		ID:      h.requestID,
		Method:  "tools/call",
		Params: mcp.ToolCallParams{
			Name:      tool,
			Arguments: args,
		},
	})

	before := h.upstream.RequestCount()
	raw, err := h.server.DispatchMessage(ctx, callMsg)
	if err != nil {
		h.tb.Fatalf("testharness: dispatch error for %s: %v", tool, err)
	}
	after := h.upstream.RequestCount()

	return parseDispatchResult(h.tb, raw, after > before)
}

// classifyRPCError maps a JSON-RPC error code to our Outcome enum. -32602 is
// the MCP spec code for schema-validation failure; everything else is lumped
// into OutcomeProtocolError because the dispatcher itself rejected the call.
func classifyRPCError(err *mcp.RPCError) Outcome {
	switch err.Code {
	case -32602:
		return OutcomeInvalidParams
	default:
		return OutcomeProtocolError
	}
}

// classifyToolError maps a tool-error message (from result.isError:true) to
// our Outcome enum. The enforcement pipeline's policy-denied error always
// starts with "tool blocked by policy:" (see enforcement/enforcement.go).
func classifyToolError(msg string) Outcome {
	if len(msg) >= len("tool blocked by policy") && msg[:len("tool blocked by policy")] == "tool blocked by policy" {
		return OutcomePolicyDenied
	}
	return OutcomeToolError
}

// extractResultText pulls the JSON text out of the MCP content envelope.
// The server wraps every successful tool return in
// {"content":[{"type":"text","text":"<json>"}]}; this helper unwraps it so
// tests can json.Unmarshal the inner payload directly.
func extractResultText(result map[string]any) (string, bool) {
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		return "", false
	}
	first, ok := content[0].(map[string]any)
	if !ok {
		return "", false
	}
	text, ok := first["text"].(string)
	if !ok {
		return "", false
	}
	return text, true
}

func mustMarshal(t testing.TB, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("testharness: marshal: %v", err)
	}
	return b
}
