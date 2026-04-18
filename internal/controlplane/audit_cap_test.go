package controlplane

import (
	"path/filepath"
	"testing"
	"time"
)

// TestAppendAuditEvent_CapEvictsOldest drives the FIFO eviction path:
// with auditCap=3, appending four events retains ids 2,3,4 and drops
// id 1. Before B5 the file-backed store grew without bound.
func TestAppendAuditEvent_CapEvictsOldest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := Open(path, WithAuditCap(3))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for i := 1; i <= 4; i++ {
		if err := store.AppendAuditEvent(AuditEvent{ID: id(i), At: time.Unix(int64(i), 0), Tool: "t"}); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	if got := len(store.state.AuditEvents); got != 3 {
		t.Fatalf("expected 3 retained events after cap, got %d", got)
	}
	first := store.state.AuditEvents[0].ID
	last := store.state.AuditEvents[2].ID
	if first != id(2) || last != id(4) {
		t.Fatalf("expected [2,3,4] retained, got first=%q last=%q", first, last)
	}
}

// TestAppendAuditEvent_NoCapUnbounded confirms back-compat: absent the
// Option, appending stays unbounded as before so existing deployments
// see no behaviour change.
func TestAppendAuditEvent_NoCapUnbounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for i := 0; i < 50; i++ {
		if err := store.AppendAuditEvent(AuditEvent{ID: id(i), At: time.Unix(int64(i), 0)}); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	if got := len(store.state.AuditEvents); got != 50 {
		t.Fatalf("expected 50 retained (no cap), got %d", got)
	}
}

// TestAppendAuditEvent_CapPersistsAcrossLoad confirms the dropped
// entries stay dropped on reload — the eviction is persisted, not
// just in-memory.
func TestAppendAuditEvent_CapPersistsAcrossLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := Open(path, WithAuditCap(2))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for i := 1; i <= 5; i++ {
		if err := store.AppendAuditEvent(AuditEvent{ID: id(i), At: time.Unix(int64(i), 0)}); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}

	reopened, err := Open(path, WithAuditCap(2))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := len(reopened.state.AuditEvents); got != 2 {
		t.Fatalf("expected 2 retained after reload, got %d", got)
	}
	if reopened.state.AuditEvents[0].ID != id(4) || reopened.state.AuditEvents[1].ID != id(5) {
		t.Fatalf("expected ids [4,5], got [%q,%q]",
			reopened.state.AuditEvents[0].ID, reopened.state.AuditEvents[1].ID)
	}
}

func id(i int) string {
	return "evt-" + time.Unix(int64(i), 0).Format(time.RFC3339)
}
