package controlplane

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestOpenMemory verifies the in-memory DSN forms ("memory", "memory://",
// empty string) all return a usable Store with no on-disk persistence.
func TestOpenMemory(t *testing.T) {
	for _, dsn := range []string{"", "memory", "memory://"} {
		t.Run(dsn, func(t *testing.T) {
			s, err := Open(dsn)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			dev, ok := s.(*DevFileStore)
			if !ok {
				t.Fatalf("memory DSN %q did not yield *DevFileStore, got %T", dsn, s)
			}
			if dev.path != "" {
				t.Fatalf("expected memory store, got path %q", dev.path)
			}
			// Ensure the Store can take writes without erroring even though
			// there is no disk backing.
			if err := s.PutTenant(TenantRecord{ID: "t1"}); err != nil {
				t.Fatalf("PutTenant: %v", err)
			}
		})
	}
}

// TestOpenFileDSNRoundTrip exercises the file:// DSN form: write everything
// the store knows about, close it, re-open from the same path, and assert
// the records re-hydrate identically.
func TestOpenFileDSNRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "store.json")
	dsn := "file://" + path

	s, err := Open(dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	tenant := TenantRecord{
		ID:              "tenant-1",
		CredentialRefID: "cred-1",
		WorkspaceID:     "ws-1",
		BaseURL:         "https://api.example.com",
		PolicyMode:      "standard",
		DenyTools:       []string{"clockify_delete_entry"},
		Metadata:        map[string]string{"owner": "alice"},
	}
	if err := s.PutTenant(tenant); err != nil {
		t.Fatalf("PutTenant: %v", err)
	}

	cred := CredentialRef{
		ID:         "cred-1",
		Backend:    "env",
		Reference:  "API_KEY",
		Workspace:  "ws-1",
		BaseURL:    "https://api.example.com",
		ModifiedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := s.PutCredentialRef(cred); err != nil {
		t.Fatalf("PutCredentialRef: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	session := SessionRecord{
		ID:              "sess-1",
		TenantID:        "tenant-1",
		Subject:         "alice@example.com",
		Transport:       "streamable_http",
		ProtocolVersion: "2025-06-18",
		ClientName:      "claude-test",
		ClientVersion:   "0.1.0",
		CreatedAt:       now,
		ExpiresAt:       now.Add(30 * time.Minute),
		LastSeenAt:      now,
	}
	if err := s.PutSession(session); err != nil {
		t.Fatalf("PutSession: %v", err)
	}

	audit := AuditEvent{
		ID:        "audit-1",
		At:        now,
		TenantID:  "tenant-1",
		Subject:   "alice@example.com",
		SessionID: "sess-1",
		Tool:      "clockify_log_time",
		Action:    "tools/call",
		Outcome:   "success",
	}
	if err := s.AppendAuditEvent(audit); err != nil {
		t.Fatalf("AppendAuditEvent: %v", err)
	}

	// File should now exist on disk; re-open from the same DSN.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("store file not persisted: %v", err)
	}
	s2, err := Open(dsn)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}

	gotTenant, ok := s2.Tenant("tenant-1")
	if !ok {
		t.Fatal("tenant missing after reopen")
	}
	if gotTenant.WorkspaceID != "ws-1" || gotTenant.PolicyMode != "standard" {
		t.Fatalf("tenant fields wrong: %+v", gotTenant)
	}
	if len(gotTenant.DenyTools) != 1 || gotTenant.DenyTools[0] != "clockify_delete_entry" {
		t.Fatalf("DenyTools wrong: %+v", gotTenant.DenyTools)
	}

	gotCred, ok := s2.CredentialRef("cred-1")
	if !ok {
		t.Fatal("credential ref missing after reopen")
	}
	if gotCred.Backend != "env" || gotCred.Reference != "API_KEY" {
		t.Fatalf("credential fields wrong: %+v", gotCred)
	}

	gotSession, ok := s2.Session("sess-1")
	if !ok {
		t.Fatal("session missing after reopen")
	}
	if gotSession.TenantID != "tenant-1" || gotSession.Subject != "alice@example.com" {
		t.Fatalf("session fields wrong: %+v", gotSession)
	}
}

// TestSessionDelete ensures DeleteSession removes the record and persists.
func TestSessionDelete(t *testing.T) {
	s, err := Open("memory")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PutSession(SessionRecord{ID: "s1", TenantID: "t1"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Session("s1"); !ok {
		t.Fatal("expected session to exist")
	}
	if err := s.DeleteSession("s1"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Session("s1"); ok {
		t.Fatal("expected session to be deleted")
	}
}

// TestUnknownLookups ensures missing IDs return ok=false rather than panicking.
func TestUnknownLookups(t *testing.T) {
	s, err := Open("memory")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Tenant("missing"); ok {
		t.Fatal("expected missing tenant to return ok=false")
	}
	if _, ok := s.CredentialRef("missing"); ok {
		t.Fatal("expected missing credential to return ok=false")
	}
	if _, ok := s.Session("missing"); ok {
		t.Fatal("expected missing session to return ok=false")
	}
}

// TestDevFileStore_RetainAudit_DropsOldEvents seeds four events with
// varying ages and asserts that RetainAudit drops the ones older than
// the cutoff, reports the correct count, and persists the reduced set.
func TestDevFileStore_RetainAudit_DropsOldEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	now := time.Now().UTC()
	events := []AuditEvent{
		{ID: "old-1", At: now.Add(-48 * time.Hour)},
		{ID: "old-2", At: now.Add(-24*time.Hour - time.Minute)},
		{ID: "fresh-1", At: now.Add(-1 * time.Hour)},
		{ID: "fresh-2", At: now.Add(-10 * time.Second)},
	}
	for _, e := range events {
		if err := s.AppendAuditEvent(e); err != nil {
			t.Fatalf("append %s: %v", e.ID, err)
		}
	}

	dropped, err := s.RetainAudit(context.Background(), 24*time.Hour)
	if err != nil {
		t.Fatalf("RetainAudit: %v", err)
	}
	if dropped != 2 {
		t.Fatalf("expected 2 dropped, got %d", dropped)
	}

	dev := s.(*DevFileStore)
	if got := len(dev.state.AuditEvents); got != 2 {
		t.Fatalf("expected 2 retained, got %d", got)
	}
	for _, e := range dev.state.AuditEvents {
		if e.ID == "old-1" || e.ID == "old-2" {
			t.Fatalf("old event %q survived retention", e.ID)
		}
	}

	// Reopening from disk must see the pruned set — RetainAudit
	// persists through the file store's normal mutex+rewrite path.
	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	reopenedDev := reopened.(*DevFileStore)
	if got := len(reopenedDev.state.AuditEvents); got != 2 {
		t.Fatalf("expected 2 retained after reload, got %d", got)
	}
}

// TestDevFileStore_RetainAudit_ZeroMaxAge is a no-op: the reaper may
// pass zero when retention is disabled, and RetainAudit must not drop
// anything in that case.
func TestDevFileStore_RetainAudit_ZeroMaxAge(t *testing.T) {
	s, err := Open("memory")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		if err := s.AppendAuditEvent(AuditEvent{
			ID: "e", At: now.Add(-time.Duration(i) * 24 * time.Hour),
		}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	dropped, err := s.RetainAudit(context.Background(), 0)
	if err != nil {
		t.Fatalf("RetainAudit: %v", err)
	}
	if dropped != 0 {
		t.Fatalf("expected 0 dropped with maxAge=0, got %d", dropped)
	}
}

// TestResolvePathErrors covers the DSN parser's error branches.
func TestResolvePathErrors(t *testing.T) {
	cases := []struct {
		dsn     string
		wantErr bool
	}{
		{"", false},
		{"memory", false},
		{"memory://", false},
		{"file://", true},
		{"postgres://user:pass@host/db", true},
		{"/tmp/store.json", false},
	}
	for _, tc := range cases {
		t.Run(tc.dsn, func(t *testing.T) {
			_, err := resolvePath(tc.dsn)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
