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
// in best_effort mode does not cause the tool call to fail.
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
	if aud.calls != 1 {
		t.Fatalf("expected 1 audit attempt, got %d", aud.calls)
	}
}

// TestAuditDurability_FailClosed_RejectsOnPersistError verifies that a persist
// failure in fail_closed mode causes the tool call to return an error.
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
		t.Fatalf("fail_closed: expected isError response when audit fails, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "audit persistence failed") {
		t.Fatalf("fail_closed: expected audit failure message, got: %s", out.String())
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

// TestAuditDurability_SuccessAuditor_AlwaysPasses verifies that a succeeding
// auditor does not interfere with tool call results in either durability mode.
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
			if aud.calls != 1 {
				t.Fatalf("%s: expected 1 audit call, got %d", mode, aud.calls)
			}
		})
	}
}
