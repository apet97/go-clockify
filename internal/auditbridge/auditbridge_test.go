package auditbridge

import (
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/mcp"
)

// TestToControlPlaneEvent_PinsAllMetadata locks the mcp → controlplane
// field mapping. A drift here would silently lose tenant or transport
// from audit rows — invisible in production until an incident query
// went looking for the missing field. Every field on the produced
// controlplane.AuditEvent must come from the input or be deterministic
// from the supplied wall-clock.
func TestToControlPlaneEvent_PinsAllMetadata(t *testing.T) {
	now := time.Date(2026, 4, 25, 22, 0, 0, 12345, time.UTC)
	in := mcp.AuditEvent{
		Tool:        "clockify_add_entry",
		Action:      "tools/call",
		Outcome:     "success",
		Phase:       mcp.PhaseOutcome,
		Reason:      "ok",
		ResourceIDs: map[string]string{"entry_id": "e-7"},
		Metadata: map[string]string{
			"tenant_id":  "tenant-a",
			"subject":    "alice",
			"session_id": "sess-1",
			"transport":  "stdio",
			"extra":      "carried-through",
		},
	}

	got := ToControlPlaneEvent(in, now)

	if got.At != now {
		t.Errorf("At: got %v, want %v", got.At, now)
	}
	if got.TenantID != "tenant-a" {
		t.Errorf("TenantID: got %q, want tenant-a", got.TenantID)
	}
	if got.Subject != "alice" {
		t.Errorf("Subject: got %q, want alice", got.Subject)
	}
	if got.SessionID != "sess-1" {
		t.Errorf("SessionID: got %q, want sess-1", got.SessionID)
	}
	if got.Transport != "stdio" {
		t.Errorf("Transport: got %q, want stdio", got.Transport)
	}
	if got.Tool != in.Tool {
		t.Errorf("Tool: got %q, want %q", got.Tool, in.Tool)
	}
	if got.Action != in.Action {
		t.Errorf("Action: got %q, want %q", got.Action, in.Action)
	}
	if got.Outcome != in.Outcome {
		t.Errorf("Outcome: got %q, want %q", got.Outcome, in.Outcome)
	}
	if got.Phase != in.Phase {
		t.Errorf("Phase: got %q, want %q", got.Phase, in.Phase)
	}
	if got.Reason != in.Reason {
		t.Errorf("Reason: got %q, want %q", got.Reason, in.Reason)
	}
	if got.ResourceIDs["entry_id"] != "e-7" {
		t.Errorf("ResourceIDs: got %v, want entry_id=e-7", got.ResourceIDs)
	}
	// Metadata must pass through unfiltered so callers reading custom
	// keys (e.g. correlation IDs) are not silently dropped.
	if got.Metadata["extra"] != "carried-through" {
		t.Errorf("Metadata extra key dropped: got %v", got.Metadata)
	}

	// ID must contain phase + outcome; this is the exact regression
	// guard the runtime audit-ID synthesis exists for. A regression
	// that strips either segment would let the Postgres ON CONFLICT
	// clause collapse the two-phase pair into a single row.
	if !strings.Contains(got.ID, in.Phase) {
		t.Errorf("ID %q missing phase segment %q", got.ID, in.Phase)
	}
	if !strings.Contains(got.ID, in.Outcome) {
		t.Errorf("ID %q missing outcome segment %q", got.ID, in.Outcome)
	}
}

// TestToControlPlaneEvent_IntentAndOutcomeIDsDiffer is the direct
// regression for the bug the runtime ID synthesis was added to fix:
// two RecordAudit calls for the same tool/session at the same
// nanosecond — one PhaseIntent, one PhaseOutcome — must yield distinct
// IDs. Without that the Postgres store dedupes and silently halves
// the audit record count.
func TestToControlPlaneEvent_IntentAndOutcomeIDsDiffer(t *testing.T) {
	now := time.Unix(0, 1234567890).UTC()
	common := mcp.AuditEvent{
		Tool:     "clockify_delete_entry",
		Action:   "tools/call",
		Metadata: map[string]string{"session_id": "sess-x"},
	}
	intent := common
	intent.Phase = mcp.PhaseIntent
	intent.Outcome = "attempted"
	outcome := common
	outcome.Phase = mcp.PhaseOutcome
	outcome.Outcome = "success"

	intentID := ToControlPlaneEvent(intent, now).ID
	outcomeID := ToControlPlaneEvent(outcome, now).ID

	if intentID == outcomeID {
		t.Fatalf("intent and outcome share ID %q at the same nanosecond — Postgres ON CONFLICT would collapse them into one row", intentID)
	}
}

// TestToControlPlaneEvent_NilMetadata confirms missing metadata
// produces empty strings, not a panic. Real callers always populate
// the metadata map (mcp.emitAudit does), but a defensive contract
// here insulates the bridge from a future call site that forgets.
func TestToControlPlaneEvent_NilMetadata(t *testing.T) {
	now := time.Now().UTC()
	in := mcp.AuditEvent{Tool: "clockify_whoami", Phase: mcp.PhaseIntent}
	got := ToControlPlaneEvent(in, now)
	if got.TenantID != "" || got.Subject != "" || got.SessionID != "" || got.Transport != "" {
		t.Fatalf("expected empty metadata-derived fields, got %+v", got)
	}
}
