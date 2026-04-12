package testharness_test

import (
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
