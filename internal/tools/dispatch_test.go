package tools_test

// Dispatcher-level tests for the Tier 1 destructive surface. These assert
// properties that the service-layer tests in tools_test.go cannot reach:
//
//   1. Every ReadOnlyHint:false tool is rejected under read_only policy
//      BEFORE the handler runs (UpstreamHit == false).
//   2. SafeCore policy allows the whitelisted writes and blocks only the
//      tools outside the whitelist (currently: clockify_delete_entry).
//   3. Schema validation runs BEFORE the policy gate, so malformed calls
//      never consume a rate-limit slot — critical for abuse prevention.
//   4. Upstream 4xx/5xx errors surface as MCP tool errors with IsError:true
//      (not JSON-RPC protocol errors), and UpstreamHit is true.
//
// The table of destructive tools is derived from internal/tools/registry.go
// at test time via svc.Registry(); adding a new ReadOnlyHint:false tool
// without adding a validArgs entry here will fail TestPolicyCoverageIsComplete.

import (
	"net/http"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/testharness"
	"github.com/apet97/go-clockify/internal/tools"
)

// validDestructiveArgs provides the minimal set of arguments that satisfies
// each destructive tool's InputSchema. Used so the policy/enforcement gate
// runs instead of schema validation short-circuiting to -32602.
//
// Kept in lockstep with registry.go by TestPolicyCoverageIsComplete below.
var validDestructiveArgs = map[string]map[string]any{
	"clockify_start_timer":           {"description": "harness-test"},
	"clockify_stop_timer":            {},
	"clockify_log_time":              {"start": "2026-01-01T09:00:00Z", "end": "2026-01-01T10:00:00Z"},
	"clockify_add_entry":             {"start": "2026-01-01T09:00:00Z"},
	"clockify_update_entry":          {"entry_id": "e-123"},
	"clockify_delete_entry":          {"entry_id": "e-123"},
	"clockify_find_and_update_entry": {"entry_id": "e-123"},
	"clockify_create_project":        {"name": "p"},
	"clockify_create_client":         {"name": "c"},
	"clockify_create_tag":            {"name": "t"},
	"clockify_create_task":           {"project": "p", "name": "t"},
	"clockify_switch_project":        {"project": "p"},
	// clockify_search_tools is technically write-classified (the
	// activate_group / activate_tool branches mutate the server's
	// visible tool surface), but the introspection allowlist in
	// internal/policy/policy.go isIntrospection() exempts it from
	// the read_only / time_tracking_safe / safe_core gates. The
	// query-only path is always safe; the activation path is
	// separately gated by IsGroupAllowed (which DOES deny in
	// read_only). The empty args here exercise the query path.
	"clockify_search_tools": {},
}

// introspectionExempt returns the names of write-classified tools
// that the policy isIntrospection() allowlist permits even under
// read_only / time_tracking_safe / safe_core. Currently only
// clockify_search_tools (write-classified post-ChatGPT-audit, but
// query path remains read-safe).
func introspectionExempt() map[string]bool {
	return map[string]bool{
		"clockify_search_tools": true,
	}
}

// destructiveToolNames returns the sorted list of every ReadOnlyHint:false
// tool in the Tier 1 registry so test tables can't drift from reality.
func destructiveToolNames(t *testing.T) []string {
	t.Helper()
	svc := tools.New(&clockify.Client{}, "unused")
	names := []string{}
	for _, d := range svc.Registry() {
		if !d.ReadOnlyHint {
			names = append(names, d.Tool.Name)
		}
	}
	return names
}

func fakeClockifyEmptyJSON(t *testing.T) *testharness.FakeClockify {
	return testharness.NewFakeClockify(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"fake-id","name":"stub"}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
}

// TestPolicyCoverageIsComplete fails when a new ReadOnlyHint:false tool is
// added to registry.go without a matching entry in validDestructiveArgs.
// This is the guard against "new write tool silently bypasses enforcement
// tests because nobody remembered to add it to the table."
func TestPolicyCoverageIsComplete(t *testing.T) {
	names := destructiveToolNames(t)
	if len(names) == 0 {
		t.Fatalf("no destructive tools found in registry — table-builder regression")
	}
	for _, name := range names {
		if _, ok := validDestructiveArgs[name]; !ok {
			t.Errorf("%s has ReadOnlyHint:false but no validDestructiveArgs entry — "+
				"add a minimal valid args map so the policy gate can be tested", name)
		}
	}
}

// TestReadOnlyPolicy_BlocksAllDestructiveTools enforces the foundational
// property: every non-read tool is rejected by read_only mode BEFORE
// reaching Clockify. A regression in this test means the enforcement
// pipeline let a write through policy — the single most dangerous class
// of bug for this server.
//
// Tools in introspectionExempt() are skipped: they are write-classified
// (mutate server-visible state via Tier-2 activation) but the
// policy.isIntrospection() allowlist exempts them so the query path
// remains usable under read_only. The activation path is gated
// separately by Activator.IsGroupAllowed.
func TestReadOnlyPolicy_BlocksAllDestructiveTools(t *testing.T) {
	exempt := introspectionExempt()
	for _, name := range destructiveToolNames(t) {
		if exempt[name] {
			continue
		}
		t.Run(name, func(t *testing.T) {
			upstream := fakeClockifyEmptyJSON(t)
			result := testharness.Invoke(t, testharness.InvokeOpts{
				Tool:       name,
				Args:       validDestructiveArgs[name],
				PolicyMode: policy.ReadOnly,
				Upstream:   upstream,
			})
			if result.Outcome != testharness.OutcomePolicyDenied {
				t.Fatalf("outcome=%q want %q (err=%q raw=%s)",
					result.Outcome, testharness.OutcomePolicyDenied,
					result.ErrorMessage, string(result.Raw))
			}
			if result.UpstreamHit {
				t.Fatalf("%s reached upstream under read_only policy — enforcement regression", name)
			}
			if !strings.Contains(result.ErrorMessage, "read_only") {
				t.Fatalf("error message missing policy mode: %q", result.ErrorMessage)
			}
		})
	}
}

// TestSafeCorePolicy_BlocksOnlyDeleteEntry asserts the SafeCore whitelist
// at policy.go:184-198: every Tier 1 write tool is allowed EXCEPT
// clockify_delete_entry (the only one tagged toolDestructive). A change to
// the whitelist that adds or removes a tool will break this test, which
// is the intended behavior — safe-write policy is a security boundary and
// changes to it should be deliberate.
func TestSafeCorePolicy_BlocksOnlyDeleteEntry(t *testing.T) {
	for _, name := range destructiveToolNames(t) {
		t.Run(name, func(t *testing.T) {
			upstream := fakeClockifyEmptyJSON(t)
			result := testharness.Invoke(t, testharness.InvokeOpts{
				Tool:       name,
				Args:       validDestructiveArgs[name],
				PolicyMode: policy.SafeCore,
				Upstream:   upstream,
			})

			if name == "clockify_delete_entry" {
				if result.Outcome != testharness.OutcomePolicyDenied {
					t.Fatalf("delete_entry allowed under safe_core (outcome=%q) — whitelist regression",
						result.Outcome)
				}
				if result.UpstreamHit {
					t.Fatalf("delete_entry reached upstream under safe_core")
				}
				return
			}

			// Non-delete writes are on the whitelist: policy must NOT
			// reject them. Handler errors are acceptable here — the
			// test asserts the policy gate outcome, not the happy path.
			if result.Outcome == testharness.OutcomePolicyDenied {
				t.Fatalf("%s rejected under safe_core but is on the whitelist (err=%q)",
					name, result.ErrorMessage)
			}
		})
	}
}

// TestExplicitDenyList_BlocksEvenUnderStandard asserts that CLOCKIFY_DENY_TOOLS
// (modelled here as Policy.DeniedTools) blocks a tool regardless of the mode.
// The destructive surface is the most important case, but the property
// applies to all tools — we sample delete_entry and create_project.
func TestExplicitDenyList_BlocksEvenUnderStandard(t *testing.T) {
	samples := []string{"clockify_delete_entry", "clockify_create_project"}
	for _, name := range samples {
		t.Run(name, func(t *testing.T) {
			upstream := fakeClockifyEmptyJSON(t)
			result := testharness.Invoke(t, testharness.InvokeOpts{
				Tool:        name,
				Args:        validDestructiveArgs[name],
				PolicyMode:  policy.Standard,
				DeniedTools: []string{name},
				Upstream:    upstream,
			})
			if result.Outcome != testharness.OutcomePolicyDenied {
				t.Fatalf("outcome=%q want %q (err=%q)", result.Outcome,
					testharness.OutcomePolicyDenied, result.ErrorMessage)
			}
			if result.UpstreamHit {
				t.Fatalf("explicitly denied tool reached upstream")
			}
		})
	}
}

// TestSchemaValidationRunsBeforePolicy asserts the enforcement ordering:
// malformed calls must be rejected at the schema layer BEFORE the policy
// gate runs. The property matters because rate-limit-abuse defense relies
// on bad requests never consuming a rate-limit slot.
//
// Concrete scenario: call delete_entry under ReadOnly policy with no
// entry_id. If the policy gate runs first, outcome is policy_denied; if
// the schema gate runs first (correct), outcome is invalid_params. Either
// way UpstreamHit must be false.
func TestSchemaValidationRunsBeforePolicy(t *testing.T) {
	upstream := fakeClockifyEmptyJSON(t)
	result := testharness.Invoke(t, testharness.InvokeOpts{
		Tool:       "clockify_delete_entry",
		Args:       map[string]any{}, // missing required entry_id
		PolicyMode: policy.ReadOnly,
		Upstream:   upstream,
	})
	if result.Outcome != testharness.OutcomeInvalidParams {
		t.Fatalf("outcome=%q want %q — schema validation ordering regression",
			result.Outcome, testharness.OutcomeInvalidParams)
	}
	if result.RPCError == nil || result.RPCError.Code != -32602 {
		t.Fatalf("expected JSON-RPC -32602, got %+v", result.RPCError)
	}
	if result.UpstreamHit {
		t.Fatalf("schema-rejected call reached upstream")
	}
}

// TestHappyPath_CreateClient exercises the full pipeline end-to-end:
// MCP dispatch → enforcement (Standard policy) → Service.CreateClient →
// clockify.Client.Post → fake upstream → JSON unmarshal → ResultEnvelope →
// AfterCall (no-op truncation) → tools/call response envelope. This is
// the canonical "the whole stack is wired correctly" smoke test.
func TestHappyPath_CreateClient(t *testing.T) {
	upstream := testharness.NewFakeClockify(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/clients") {
			t.Errorf("unexpected upstream request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"c-new-1","name":"Acme","workspaceId":"test-workspace"}`))
	}))

	result := testharness.Invoke(t, testharness.InvokeOpts{
		Tool:       "clockify_create_client",
		Args:       map[string]any{"name": "Acme"},
		PolicyMode: policy.Standard,
		Upstream:   upstream,
	})

	if result.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("outcome=%q want %q (err=%q raw=%s)",
			result.Outcome, testharness.OutcomeSuccess, result.ErrorMessage, string(result.Raw))
	}
	if !result.UpstreamHit {
		t.Fatalf("create_client did not reach upstream")
	}
	if !strings.Contains(result.ResultText, "c-new-1") {
		t.Fatalf("result text missing created id: %q", result.ResultText)
	}
}

// TestHappyPath_CreateTag exercises the create_tag handler through the real
// pipeline. The fake returns a minimal Tag payload; the service layer's
// ok() envelope wraps it into the MCP content block.
func TestHappyPath_CreateTag(t *testing.T) {
	upstream := testharness.NewFakeClockify(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/tags") {
			t.Errorf("unexpected upstream request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"tag-1","name":"urgent","workspaceId":"test-workspace"}`))
	}))

	result := testharness.Invoke(t, testharness.InvokeOpts{
		Tool:       "clockify_create_tag",
		Args:       map[string]any{"name": "urgent"},
		PolicyMode: policy.Standard,
		Upstream:   upstream,
	})
	if result.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("outcome=%q want %q (err=%q)", result.Outcome, testharness.OutcomeSuccess, result.ErrorMessage)
	}
	if !strings.Contains(result.ResultText, "tag-1") {
		t.Fatalf("result text missing created id: %q", result.ResultText)
	}
}

// TestHappyPath_DeleteEntry exercises the one toolDestructive tool through
// the real pipeline. delete_entry pre-fetches the target via GET (for
// audit logging and to surface 404 as a handler error rather than a
// silent no-op) before issuing the DELETE, so the fake must respond to
// both methods on the same path.
func TestHappyPath_DeleteEntry(t *testing.T) {
	upstream := testharness.NewFakeClockify(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/time-entries/e-123") {
			t.Errorf("unexpected upstream path: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"e-123","description":"lunch","workspaceId":"test-workspace"}`))
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		}
	}))

	result := testharness.Invoke(t, testharness.InvokeOpts{
		Tool:       "clockify_delete_entry",
		Args:       map[string]any{"entry_id": "e-123"},
		PolicyMode: policy.Standard,
		Upstream:   upstream,
	})
	if result.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("outcome=%q want %q (err=%q raw=%s)",
			result.Outcome, testharness.OutcomeSuccess, result.ErrorMessage, string(result.Raw))
	}
	if !result.UpstreamHit {
		t.Fatalf("delete_entry did not reach upstream")
	}
}

// TestUpstream5xxSurfacesAsToolError asserts that Clockify 500 responses
// are wrapped in the MCP tool-error envelope (result.isError:true, not
// JSON-RPC error). This is the contract for MCP clients: "tool errors" and
// "protocol errors" live in different response fields and clients handle
// them differently.
func TestUpstream5xxSurfacesAsToolError(t *testing.T) {
	upstream := testharness.NewFakeClockify(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"simulated server error"}`, http.StatusInternalServerError)
	}))

	result := testharness.Invoke(t, testharness.InvokeOpts{
		Tool:       "clockify_delete_entry",
		Args:       map[string]any{"entry_id": "e-123"},
		PolicyMode: policy.Standard,
		Upstream:   upstream,
	})
	if result.Outcome != testharness.OutcomeToolError {
		t.Fatalf("outcome=%q want %q (err=%q)",
			result.Outcome, testharness.OutcomeToolError, result.ErrorMessage)
	}
	if !result.IsError {
		t.Fatalf("expected IsError=true for upstream 5xx")
	}
	if !result.UpstreamHit {
		t.Fatalf("upstream was hit but RequestCount delta was zero — counter bug")
	}
	if result.RPCError != nil {
		t.Fatalf("upstream 5xx leaked as JSON-RPC error %+v — should be isError:true content", result.RPCError)
	}
}
