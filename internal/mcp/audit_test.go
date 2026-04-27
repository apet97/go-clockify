package mcp

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

// failAuditor always returns an error from RecordAudit.
type failAuditor struct{ calls int }

func (a *failAuditor) RecordAudit(e AuditEvent) error {
	a.calls++
	return errors.New("simulated persist failure")
}

// successAuditor always succeeds.
type successAuditor struct{ calls int }

func (a *successAuditor) RecordAudit(e AuditEvent) error {
	a.calls++
	return nil
}

func newAuditServer(auditor Auditor, mode string) *Server {
	s := NewServer("test", []ToolDescriptor{
		{
			Tool:         Tool{Name: "write_tool"},
			Handler:      func(_ context.Context, _ map[string]any) (any, error) { return "ok", nil },
			ReadOnlyHint: false,
		},
		{
			Tool:         Tool{Name: "read_tool"},
			Handler:      func(_ context.Context, _ map[string]any) (any, error) { return "ok", nil },
			ReadOnlyHint: true,
		},
	}, nil, nil)
	s.Auditor = auditor
	s.AuditDurabilityMode = mode
	return s
}

// TestAuditDurability_BestEffort_AllowsSuccess verifies that a persist failure
// in best_effort mode does not cause the tool call to fail. With the two-phase
// audit model, best_effort emits BOTH the intent and the outcome record even
// when the auditor is broken — the failures are logged and the call proceeds.
func TestAuditDurability_BestEffort_AllowsSuccess(t *testing.T) {
	aud := &failAuditor{}
	s := newAuditServer(aud, "best_effort")
	s.initialized.Store(true)

	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write_tool","arguments":{}}}`
	var out strings.Builder
	if err := s.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(out.String(), `"isError":true`) {
		t.Fatalf("best_effort: expected success, got error response: %s", out.String())
	}
	// 2 attempts: intent (pre-handler) + outcome (post-handler).
	if aud.calls != 2 {
		t.Fatalf("best_effort: expected 2 audit attempts (intent + outcome), got %d", aud.calls)
	}
}

// TestAuditDurability_FailClosed_RejectsOnPersistError verifies that a persist
// failure in fail_closed mode causes the tool call to return an error BEFORE
// the handler runs. With the two-phase audit, fail_closed short-circuits on
// the intent persistence failure so the mutation never commits — this is the
// property the audit refactor was designed to give us.
func TestAuditDurability_FailClosed_RejectsOnPersistError(t *testing.T) {
	aud := &failAuditor{}
	s := newAuditServer(aud, "fail_closed")
	s.initialized.Store(true)

	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write_tool","arguments":{}}}`
	var out strings.Builder
	if err := s.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out.String(), `"isError":true`) {
		t.Fatalf("fail_closed: expected isError response when audit intent fails, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "audit intent persistence failed") {
		t.Fatalf("fail_closed: expected intent-audit failure message, got: %s", out.String())
	}
	// Only 1 attempt: intent failed → handler never ran → no outcome record.
	// This is the pre-mutation guarantee: no mutation if audit can't be
	// persisted.
	if aud.calls != 1 {
		t.Fatalf("fail_closed: expected handler-skip after intent failure (1 audit call), got %d", aud.calls)
	}
}

// TestAuditDurability_ReadOnly_Unaffected verifies that read-only tools never
// trigger the auditor and are unaffected by AuditDurabilityMode.
func TestAuditDurability_ReadOnly_Unaffected(t *testing.T) {
	aud := &failAuditor{}
	s := newAuditServer(aud, "fail_closed")
	s.initialized.Store(true)

	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_tool","arguments":{}}}`
	var out strings.Builder
	if err := s.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(out.String(), `"isError":true`) {
		t.Fatalf("read-only: should not be affected by audit failure, got: %s", out.String())
	}
	if aud.calls != 0 {
		t.Fatalf("read-only: auditor should not be called, got %d calls", aud.calls)
	}
}

// TestAuditDurability_LogsCanonicalOutcomeField verifies that a persist
// failure emits the audit_outcome=not_durable log field that operators
// alert on per docs/runbooks/audit-durability.md. The runbook's filter
// recipe relies on this exact field/value pair.
func TestAuditDurability_LogsCanonicalOutcomeField(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})))
	defer slog.SetDefault(prev)

	for _, mode := range []string{"best_effort", "fail_closed"} {
		t.Run(mode, func(t *testing.T) {
			buf.Reset()
			aud := &failAuditor{}
			s := newAuditServer(aud, mode)
			s.initialized.Store(true)

			input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write_tool","arguments":{}}}`
			var out strings.Builder
			if err := s.Run(context.Background(), strings.NewReader(input), &out); err != nil {
				t.Fatalf("Run: %v", err)
			}
			logged := buf.String()
			if !strings.Contains(logged, `"audit_outcome":"not_durable"`) {
				t.Fatalf("%s: expected audit_outcome=not_durable in log, got: %s", mode, logged)
			}
			if !strings.Contains(logged, `"durability_mode":"`+mode+`"`) {
				t.Fatalf("%s: expected durability_mode=%s in log, got: %s", mode, mode, logged)
			}
		})
	}
}

// TestAuditDurability_LogsTenantAttribution locks in the iter156 + iter157
// invariant that audit_persist_failed (best_effort + fail_closed outcome
// path) and tool_call_blocked_by_audit (fail_closed intent rejection)
// emit the four tenant-attribution fields that the audit-durability runbook
// §5 "Identify affected tenants" recovery step depends on. Reverting the
// "tenant_id"/"subject"/"session_id"/"transport" args on either slog call
// must fail this test loudly so a future contributor doesn't quietly
// regress incident-response capability.
func TestAuditDurability_LogsTenantAttribution(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prev)

	for _, mode := range []string{"best_effort", "fail_closed"} {
		t.Run(mode, func(t *testing.T) {
			buf.Reset()
			aud := &failAuditor{}
			s := newAuditServer(aud, mode)
			s.initialized.Store(true)
			s.AuditTenantID = "tenant-acme"
			s.AuditSubject = "user-42"
			s.AuditSessionID = "sess-xyz"
			s.AuditTransport = "streamable_http"

			input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write_tool","arguments":{}}}`
			var out strings.Builder
			if err := s.Run(context.Background(), strings.NewReader(input), &out); err != nil {
				t.Fatalf("Run: %v", err)
			}
			logged := buf.String()
			for _, want := range []string{
				`"tenant_id":"tenant-acme"`,
				`"subject":"user-42"`,
				`"session_id":"sess-xyz"`,
				`"transport":"streamable_http"`,
			} {
				if !strings.Contains(logged, want) {
					t.Errorf("%s: missing %s in audit slog stream\n--- captured slog ---\n%s", mode, want, logged)
				}
			}
		})
	}
}

// recordingAuditor captures each event (phase + outcome) for assertions.
type recordingAuditor struct {
	events []AuditEvent
}

func (r *recordingAuditor) RecordAudit(e AuditEvent) error {
	r.events = append(r.events, e)
	return nil
}

// TestAuditPhase_FailClosedPreventsHandlerOnIntentFailure is the core
// promise of the 2026-04-25 audit refactor (finding H5): when the audit
// pipeline is broken AND the operator has selected fail_closed, the
// handler must never be invoked for a non-read-only call.
func TestAuditPhase_FailClosedPreventsHandlerOnIntentFailure(t *testing.T) {
	var handlerInvocations int
	s := NewServer("test", []ToolDescriptor{{
		Tool: Tool{Name: "write_tool"},
		Handler: func(_ context.Context, _ map[string]any) (any, error) {
			handlerInvocations++
			return "ok", nil
		},
		ReadOnlyHint: false,
	}}, nil, nil)
	s.Auditor = &failAuditor{}
	s.AuditDurabilityMode = "fail_closed"
	s.initialized.Store(true)

	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write_tool","arguments":{}}}`
	var out strings.Builder
	if err := s.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if handlerInvocations != 0 {
		t.Fatalf("fail_closed + failed intent MUST NOT invoke handler; got %d invocations", handlerInvocations)
	}
	if !strings.Contains(out.String(), `"isError":true`) {
		t.Fatalf("expected isError response, got: %s", out.String())
	}
}

// TestAuditPhase_BestEffortRunsHandlerDespiteIntentFailure confirms the
// symmetric back-compat path: best_effort keeps the existing behaviour
// where audit persistence failures are logged but the tool call still
// runs.
func TestAuditPhase_BestEffortRunsHandlerDespiteIntentFailure(t *testing.T) {
	var handlerInvocations int
	s := NewServer("test", []ToolDescriptor{{
		Tool: Tool{Name: "write_tool"},
		Handler: func(_ context.Context, _ map[string]any) (any, error) {
			handlerInvocations++
			return "ok", nil
		},
		ReadOnlyHint: false,
	}}, nil, nil)
	s.Auditor = &failAuditor{}
	s.AuditDurabilityMode = "best_effort"
	s.initialized.Store(true)

	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write_tool","arguments":{}}}`
	var out strings.Builder
	if err := s.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if handlerInvocations != 1 {
		t.Fatalf("best_effort must still invoke handler despite audit failure; got %d invocations", handlerInvocations)
	}
	if strings.Contains(out.String(), `"isError":true`) {
		t.Fatalf("best_effort must return success, got: %s", out.String())
	}
}

// TestAudit_AuditKeysCaptureActionDefiningArgs verifies the end-to-end
// flow added in audit finding 8: a ToolDescriptor.AuditKeys list is
// forwarded via ToolHints into the audit recorder, and resourceIDs()
// captures those non-_id arg values in the AuditEvent. Without the
// flow, an audit event for a permission change would carry only the
// user_id and lose the new role.
func TestAudit_AuditKeysCaptureActionDefiningArgs(t *testing.T) {
	rec := &recordingAuditor{}
	s := NewServer("test", []ToolDescriptor{{
		Tool: Tool{Name: "update_role", InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"user_id": map[string]any{"type": "string"},
				"role":    map[string]any{"type": "string"},
			},
		}},
		Handler:      func(_ context.Context, _ map[string]any) (any, error) { return "ok", nil },
		ReadOnlyHint: false,
		AuditKeys:    []string{"role"},
	}}, nil, nil)
	s.Auditor = rec
	s.AuditDurabilityMode = "best_effort"
	s.initialized.Store(true)

	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"update_role","arguments":{"user_id":"u1","role":"ADMIN"}}}`
	var out strings.Builder
	if err := s.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(rec.events) < 1 {
		t.Fatalf("expected at least one audit event")
	}
	intent := rec.events[0]
	if intent.ResourceIDs["user_id"] != "u1" {
		t.Errorf("user_id missing or wrong: %+v", intent.ResourceIDs)
	}
	if intent.ResourceIDs["role"] != "ADMIN" {
		t.Errorf("role not captured by AuditKeys plumbing: %+v", intent.ResourceIDs)
	}
}

// TestAuditPhase_IntentThenOutcomeOnSuccess locks in the record order
// and phase tagging for a successful non-read-only call.
func TestAuditPhase_IntentThenOutcomeOnSuccess(t *testing.T) {
	rec := &recordingAuditor{}
	s := NewServer("test", []ToolDescriptor{{
		Tool:         Tool{Name: "write_tool"},
		Handler:      func(_ context.Context, _ map[string]any) (any, error) { return "ok", nil },
		ReadOnlyHint: false,
	}}, nil, nil)
	s.Auditor = rec
	s.AuditDurabilityMode = "best_effort"
	s.initialized.Store(true)

	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write_tool","arguments":{}}}`
	var out strings.Builder
	if err := s.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(rec.events) != 2 {
		t.Fatalf("expected 2 audit events (intent + outcome), got %d: %+v", len(rec.events), rec.events)
	}
	if rec.events[0].Phase != PhaseIntent {
		t.Errorf("event[0].Phase = %q, want %q", rec.events[0].Phase, PhaseIntent)
	}
	if rec.events[0].Outcome != "attempted" {
		t.Errorf("event[0].Outcome = %q, want \"attempted\"", rec.events[0].Outcome)
	}
	if rec.events[1].Phase != PhaseOutcome {
		t.Errorf("event[1].Phase = %q, want %q", rec.events[1].Phase, PhaseOutcome)
	}
	if rec.events[1].Outcome != "success" {
		t.Errorf("event[1].Outcome = %q, want \"success\"", rec.events[1].Outcome)
	}
}

// TestAuditPhase_OutcomeMarksFailedOnHandlerError verifies that when
// the intent succeeded but the handler errored, the outcome record
// captures the failure status + reason so the two records pair up.
func TestAuditPhase_OutcomeMarksFailedOnHandlerError(t *testing.T) {
	rec := &recordingAuditor{}
	s := NewServer("test", []ToolDescriptor{{
		Tool:         Tool{Name: "write_tool"},
		Handler:      func(_ context.Context, _ map[string]any) (any, error) { return nil, errors.New("boom") },
		ReadOnlyHint: false,
	}}, nil, nil)
	s.Auditor = rec
	s.AuditDurabilityMode = "best_effort"
	s.initialized.Store(true)

	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write_tool","arguments":{}}}`
	var out strings.Builder
	if err := s.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(rec.events) != 2 {
		t.Fatalf("expected 2 events (intent + outcome), got %d", len(rec.events))
	}
	if rec.events[0].Phase != PhaseIntent {
		t.Errorf("intent phase missing: %+v", rec.events[0])
	}
	if rec.events[1].Phase != PhaseOutcome {
		t.Errorf("outcome phase missing: %+v", rec.events[1])
	}
	if rec.events[1].Outcome != "tool_error" {
		t.Errorf("outcome = %q, want \"tool_error\"", rec.events[1].Outcome)
	}
	if !strings.Contains(rec.events[1].Reason, "boom") {
		t.Errorf("outcome reason missing handler error: %q", rec.events[1].Reason)
	}
}

// TestAuditPhase_ReadOnlyToolsSkipBothPhases confirms read-only calls
// never emit intent or outcome records — nothing to audit.
func TestAuditPhase_ReadOnlyToolsSkipBothPhases(t *testing.T) {
	rec := &recordingAuditor{}
	s := NewServer("test", []ToolDescriptor{{
		Tool:         Tool{Name: "read_tool"},
		Handler:      func(_ context.Context, _ map[string]any) (any, error) { return "ok", nil },
		ReadOnlyHint: true,
	}}, nil, nil)
	s.Auditor = rec
	s.AuditDurabilityMode = "fail_closed"
	s.initialized.Store(true)

	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_tool","arguments":{}}}`
	var out strings.Builder
	if err := s.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(rec.events) != 0 {
		t.Fatalf("read-only tool emitted %d audit events, expected 0: %+v", len(rec.events), rec.events)
	}
}

// TestAuditDurability_SuccessAuditor_AlwaysPasses verifies that a succeeding
// auditor does not interfere with tool call results in either durability mode.
// Both modes now emit an intent AND outcome record on a successful mutation,
// so the auditor should see 2 calls regardless of mode.
func TestAuditDurability_SuccessAuditor_AlwaysPasses(t *testing.T) {
	for _, mode := range []string{"best_effort", "fail_closed"} {
		t.Run(mode, func(t *testing.T) {
			aud := &successAuditor{}
			s := newAuditServer(aud, mode)
			s.initialized.Store(true)

			input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write_tool","arguments":{}}}`
			var out strings.Builder
			if err := s.Run(context.Background(), strings.NewReader(input), &out); err != nil {
				t.Fatalf("Run: %v", err)
			}
			if strings.Contains(out.String(), `"isError":true`) {
				t.Fatalf("%s: expected success, got error: %s", mode, out.String())
			}
			if aud.calls != 2 {
				t.Fatalf("%s: expected 2 audit calls (intent + outcome), got %d", mode, aud.calls)
			}
		})
	}
}
