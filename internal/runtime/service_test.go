package runtime

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/controlplane"
	"github.com/apet97/go-clockify/internal/mcp"
)

// recordingStore captures every AppendAuditEvent call so the runtime
// auditor's ID synthesis can be inspected. Other Store methods return
// zero values — the runtime auditor only ever calls AppendAuditEvent.
type recordingStore struct {
	mu     sync.Mutex
	events []controlplane.AuditEvent
}

func (s *recordingStore) AppendAuditEvent(e controlplane.AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	return nil
}

func (s *recordingStore) snapshot() []controlplane.AuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]controlplane.AuditEvent, len(s.events))
	copy(out, s.events)
	return out
}

// Stubs — never called by controlPlaneAuditor.RecordAudit, but the
// controlplane.Store interface requires them.
func (s *recordingStore) Tenant(string) (controlplane.TenantRecord, bool) {
	return controlplane.TenantRecord{}, false
}
func (s *recordingStore) PutTenant(controlplane.TenantRecord) error               { return nil }
func (s *recordingStore) CredentialRef(string) (controlplane.CredentialRef, bool) { return controlplane.CredentialRef{}, false }
func (s *recordingStore) PutCredentialRef(controlplane.CredentialRef) error       { return nil }
func (s *recordingStore) Session(string) (controlplane.SessionRecord, bool)       { return controlplane.SessionRecord{}, false }
func (s *recordingStore) PutSession(controlplane.SessionRecord) error             { return nil }
func (s *recordingStore) DeleteSession(string) error                              { return nil }
func (s *recordingStore) RetainAudit(context.Context, time.Duration) (int, error) { return 0, nil }
func (s *recordingStore) Close() error                                            { return nil }

// TestControlPlaneAuditorAuditIDIncludesPhaseAndOutcome locks that the
// runtime auditor synthesises an external ID containing both the phase
// ("intent") and outcome ("attempted") segments so the Postgres
// ON CONFLICT (external_id) DO NOTHING dedupe cannot collapse the
// intent/outcome pair into a single row. It also pins the event-level
// metadata (tenant/subject/session/transport, phase, At) the runtime
// must preserve when bridging mcp.AuditEvent → controlplane.AuditEvent.
func TestControlPlaneAuditorAuditIDIncludesPhaseAndOutcome(t *testing.T) {
	store := &recordingStore{}
	auditor := controlPlaneAuditor{store: store}

	begin := time.Now()
	if err := auditor.RecordAudit(mcp.AuditEvent{
		Tool:    "clockify_log_time",
		Action:  "create_entry",
		Outcome: "attempted",
		Phase:   mcp.PhaseIntent,
		Metadata: map[string]string{
			"tenant_id":  "acme",
			"subject":    "alice@example.com",
			"session_id": "sess-123",
			"transport":  "streamable_http",
		},
	}); err != nil {
		t.Fatalf("RecordAudit returned error: %v", err)
	}

	got := store.snapshot()
	if len(got) != 1 {
		t.Fatalf("expected 1 event recorded, got %d", len(got))
	}
	ev := got[0]

	if !strings.Contains(ev.ID, "intent") {
		t.Errorf("ID %q does not contain phase segment %q", ev.ID, "intent")
	}
	if !strings.Contains(ev.ID, "attempted") {
		t.Errorf("ID %q does not contain outcome segment %q", ev.ID, "attempted")
	}
	if !strings.Contains(ev.ID, "sess-123") {
		t.Errorf("ID %q does not contain session segment", ev.ID)
	}
	if !strings.Contains(ev.ID, "clockify_log_time") {
		t.Errorf("ID %q does not contain tool segment", ev.ID)
	}

	if ev.At.IsZero() {
		t.Error("event At is zero; runtime auditor must stamp the event time")
	}
	if ev.At.Before(begin.Add(-time.Second)) || ev.At.After(time.Now().Add(time.Second)) {
		t.Errorf("event At %v is outside the call window [%v..now]", ev.At, begin)
	}
	if ev.Phase != mcp.PhaseIntent {
		t.Errorf("Phase = %q, want %q", ev.Phase, mcp.PhaseIntent)
	}
	if ev.TenantID != "acme" {
		t.Errorf("TenantID = %q, want %q", ev.TenantID, "acme")
	}
	if ev.Subject != "alice@example.com" {
		t.Errorf("Subject = %q, want %q", ev.Subject, "alice@example.com")
	}
	if ev.SessionID != "sess-123" {
		t.Errorf("SessionID = %q, want %q", ev.SessionID, "sess-123")
	}
	if ev.Transport != "streamable_http" {
		t.Errorf("Transport = %q, want %q", ev.Transport, "streamable_http")
	}
	if ev.Tool != "clockify_log_time" {
		t.Errorf("Tool = %q, want %q", ev.Tool, "clockify_log_time")
	}
	if ev.Action != "create_entry" {
		t.Errorf("Action = %q, want %q", ev.Action, "create_entry")
	}
	if ev.Outcome != "attempted" {
		t.Errorf("Outcome = %q, want %q", ev.Outcome, "attempted")
	}
}

// TestControlPlaneAuditorIntentAndOutcomeIDsDiffer is the regression
// test for the runtime side of the dedupe-collision bug. Two RecordAudit
// calls for the same tool/session in the same nanosecond — one
// PhaseIntent, one PhaseOutcome — must yield distinct external IDs.
// Without phase+outcome in the synthesised ID, the Postgres
// ON CONFLICT (external_id) DO NOTHING insert would collapse the pair
// into a single row and the audit trail would lose the outcome record.
func TestControlPlaneAuditorIntentAndOutcomeIDsDiffer(t *testing.T) {
	store := &recordingStore{}
	auditor := controlPlaneAuditor{store: store}

	meta := map[string]string{
		"tenant_id":  "acme",
		"subject":    "alice",
		"session_id": "sess-xyz",
		"transport":  "streamable_http",
	}

	if err := auditor.RecordAudit(mcp.AuditEvent{
		Tool:     "clockify_add_entry",
		Action:   "create_entry",
		Outcome:  "attempted",
		Phase:    mcp.PhaseIntent,
		Metadata: meta,
	}); err != nil {
		t.Fatalf("intent RecordAudit: %v", err)
	}
	if err := auditor.RecordAudit(mcp.AuditEvent{
		Tool:     "clockify_add_entry",
		Action:   "create_entry",
		Outcome:  "succeeded",
		Phase:    mcp.PhaseOutcome,
		Metadata: meta,
	}); err != nil {
		t.Fatalf("outcome RecordAudit: %v", err)
	}

	got := store.snapshot()
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	if got[0].ID == got[1].ID {
		t.Fatalf("intent and outcome share ID %q — Postgres ON CONFLICT would collapse them into one row", got[0].ID)
	}
	if got[0].Phase != mcp.PhaseIntent || got[1].Phase != mcp.PhaseOutcome {
		t.Fatalf("phase order corrupted: got %q then %q", got[0].Phase, got[1].Phase)
	}
}

// TestControlPlaneAuditorNilStoreNoOps locks the documented "store-less"
// behaviour — a controlPlaneAuditor with a nil store must silently
// succeed rather than panic, so the runtime can wire it unconditionally.
func TestControlPlaneAuditorNilStoreNoOps(t *testing.T) {
	auditor := controlPlaneAuditor{store: nil}
	if err := auditor.RecordAudit(mcp.AuditEvent{Tool: "x", Phase: mcp.PhaseOutcome}); err != nil {
		t.Fatalf("nil-store RecordAudit returned error: %v", err)
	}
}
