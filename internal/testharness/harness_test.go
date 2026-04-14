package testharness_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/testharness"
)

// A dispatcher-level proof test that exercises every field on InvokeResult
// against a single known-destructive tool. The real A3.1 test suite uses
// the same patterns, driven from a registry-enumerating table.

func fakeClockifyAllOK(t *testing.T) *testharness.FakeClockify {
	return testharness.NewFakeClockify(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Minimal stub: every GET returns an empty JSON object; DELETE
		// returns 204. Good enough to exercise the "reached upstream"
		// signal without baking real response shapes into the harness test.
		switch r.Method {
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		}
	}))
}

func TestInvoke_DeleteEntry_HappyPath(t *testing.T) {
	upstream := fakeClockifyAllOK(t)

	result := testharness.Invoke(t, testharness.InvokeOpts{
		Tool: "clockify_delete_entry",
		Args: map[string]any{
			"entry_id": "entry-123",
		},
		PolicyMode: policy.Standard,
		Upstream:   upstream,
	})

	if result.RPCError != nil {
		t.Fatalf("unexpected RPC error: %+v", result.RPCError)
	}
	if result.IsError {
		t.Fatalf("tool error: %s", result.ErrorMessage)
	}
	if result.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("outcome=%q want %q", result.Outcome, testharness.OutcomeSuccess)
	}
	if !result.UpstreamHit {
		t.Fatalf("expected upstream hit on happy path")
	}
}

func TestInvoke_DeleteEntry_PolicyBlocked_NoUpstreamHit(t *testing.T) {
	upstream := fakeClockifyAllOK(t)

	result := testharness.Invoke(t, testharness.InvokeOpts{
		Tool: "clockify_delete_entry",
		Args: map[string]any{
			"entry_id": "entry-123",
		},
		// ReadOnly mode must reject every non-read tool before any
		// HTTP request is issued.
		PolicyMode: policy.ReadOnly,
		Upstream:   upstream,
	})

	if result.Outcome != testharness.OutcomePolicyDenied {
		t.Fatalf("outcome=%q want %q (err=%q)", result.Outcome, testharness.OutcomePolicyDenied, result.ErrorMessage)
	}
	if !result.IsError {
		t.Fatalf("expected IsError=true for policy-blocked call")
	}
	if result.UpstreamHit {
		t.Fatalf("policy-blocked call reached upstream — enforcement layer regression")
	}
	if !strings.Contains(result.ErrorMessage, "blocked by policy") {
		t.Fatalf("error message missing policy marker: %q", result.ErrorMessage)
	}
}

func TestInvoke_DeleteEntry_SafeCoreBlocked_NoUpstreamHit(t *testing.T) {
	upstream := fakeClockifyAllOK(t)

	// clockify_delete_entry is not on the safe_core write whitelist, so
	// safe_core must reject it just like read_only.
	result := testharness.Invoke(t, testharness.InvokeOpts{
		Tool: "clockify_delete_entry",
		Args: map[string]any{
			"entry_id": "entry-123",
		},
		PolicyMode: policy.SafeCore,
		Upstream:   upstream,
	})

	if result.Outcome != testharness.OutcomePolicyDenied {
		t.Fatalf("outcome=%q want %q (err=%q)", result.Outcome, testharness.OutcomePolicyDenied, result.ErrorMessage)
	}
	if result.UpstreamHit {
		t.Fatalf("safe_core rejected call still reached upstream")
	}
}

func TestInvoke_DeleteEntry_ExplicitDeny_NoUpstreamHit(t *testing.T) {
	upstream := fakeClockifyAllOK(t)

	// Even under Standard policy, an explicit deny-list must block.
	result := testharness.Invoke(t, testharness.InvokeOpts{
		Tool: "clockify_delete_entry",
		Args: map[string]any{
			"entry_id": "entry-123",
		},
		PolicyMode:  policy.Standard,
		DeniedTools: []string{"clockify_delete_entry"},
		Upstream:    upstream,
	})

	if result.Outcome != testharness.OutcomePolicyDenied {
		t.Fatalf("outcome=%q want %q (err=%q)", result.Outcome, testharness.OutcomePolicyDenied, result.ErrorMessage)
	}
	if result.UpstreamHit {
		t.Fatalf("explicitly denied tool still reached upstream")
	}
}

func TestInvoke_InvalidParams_SchemaFailure(t *testing.T) {
	upstream := fakeClockifyAllOK(t)

	// entry_id is a required property per registry.go:110. Missing it
	// should fail JSON-schema validation at enforcement.BeforeCall and
	// surface as a JSON-RPC -32602 error.
	result := testharness.Invoke(t, testharness.InvokeOpts{
		Tool:       "clockify_delete_entry",
		Args:       map[string]any{},
		PolicyMode: policy.Standard,
		Upstream:   upstream,
	})

	if result.Outcome != testharness.OutcomeInvalidParams {
		t.Fatalf("outcome=%q want %q (raw=%s)", result.Outcome, testharness.OutcomeInvalidParams, string(result.Raw))
	}
	if result.RPCError == nil {
		t.Fatalf("expected JSON-RPC error envelope")
	}
	if result.RPCError.Code != -32602 {
		t.Fatalf("RPC error code=%d want -32602", result.RPCError.Code)
	}
	if result.UpstreamHit {
		t.Fatalf("schema-rejected call reached upstream")
	}
}

// TestBenchHarness_ReusesInitializedServer asserts the amortised path:
// NewBenchHarness builds svc + registry + pipeline + mcp.Server and runs
// initialize ONCE; each Call then dispatches against the already-warm
// server. Two sequential successful calls prove the reuse is safe (if
// Call accidentally re-sent initialize, the second one would fail with
// the server's "already initialized" path).
func TestBenchHarness_ReusesInitializedServer(t *testing.T) {
	upstream := fakeClockifyAllOK(t)

	harness := testharness.NewBenchHarness(t, testharness.InvokeOpts{
		PolicyMode: policy.Standard,
		Upstream:   upstream,
	})
	ctx := context.Background()

	first := harness.Call(ctx, "clockify_delete_entry", map[string]any{"entry_id": "entry-1"})
	if first.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("first Call outcome=%q err=%q raw=%s",
			first.Outcome, first.ErrorMessage, string(first.Raw))
	}
	if !first.UpstreamHit {
		t.Fatalf("first Call did not reach upstream")
	}

	second := harness.Call(ctx, "clockify_delete_entry", map[string]any{"entry_id": "entry-2"})
	if second.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("second Call outcome=%q err=%q raw=%s",
			second.Outcome, second.ErrorMessage, string(second.Raw))
	}
	if !second.UpstreamHit {
		t.Fatalf("second Call did not reach upstream")
	}
}

// TestBenchHarness_PolicyBlockStillEnforced asserts that running tools
// through BenchHarness still traverses the enforcement pipeline — a
// ReadOnly-mode bench of a destructive tool must be rejected before the
// upstream is hit. This prevents a future refactor from accidentally
// routing benchmark traffic around the policy gate.
func TestBenchHarness_PolicyBlockStillEnforced(t *testing.T) {
	upstream := fakeClockifyAllOK(t)

	harness := testharness.NewBenchHarness(t, testharness.InvokeOpts{
		PolicyMode: policy.ReadOnly,
		Upstream:   upstream,
	})
	ctx := context.Background()

	result := harness.Call(ctx, "clockify_delete_entry", map[string]any{"entry_id": "entry-1"})
	if result.Outcome != testharness.OutcomePolicyDenied {
		t.Fatalf("outcome=%q want %q raw=%s",
			result.Outcome, testharness.OutcomePolicyDenied, string(result.Raw))
	}
	if result.UpstreamHit {
		t.Fatalf("policy-denied call reached upstream")
	}
}
