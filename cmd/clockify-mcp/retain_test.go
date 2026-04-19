package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/controlplane"
)

// fakeRetainStore satisfies controlplane.Store just enough for the
// reaper: every other method returns a zero value or nil so the test
// does not depend on the control-plane state shape. RetainAudit is
// scripted per call.
type fakeRetainStore struct {
	calls    []time.Duration
	deleted  int
	err      error
	lastMaxAge time.Duration
}

func (f *fakeRetainStore) Tenant(string) (controlplane.TenantRecord, bool) {
	return controlplane.TenantRecord{}, false
}
func (f *fakeRetainStore) PutTenant(controlplane.TenantRecord) error { return nil }
func (f *fakeRetainStore) CredentialRef(string) (controlplane.CredentialRef, bool) {
	return controlplane.CredentialRef{}, false
}
func (f *fakeRetainStore) PutCredentialRef(controlplane.CredentialRef) error { return nil }
func (f *fakeRetainStore) Session(string) (controlplane.SessionRecord, bool) {
	return controlplane.SessionRecord{}, false
}
func (f *fakeRetainStore) PutSession(controlplane.SessionRecord) error { return nil }
func (f *fakeRetainStore) DeleteSession(string) error                  { return nil }
func (f *fakeRetainStore) AppendAuditEvent(controlplane.AuditEvent) error {
	return nil
}
func (f *fakeRetainStore) RetainAudit(_ context.Context, maxAge time.Duration) (int, error) {
	f.calls = append(f.calls, maxAge)
	f.lastMaxAge = maxAge
	return f.deleted, f.err
}
func (f *fakeRetainStore) Close() error { return nil }

func TestRetainAuditOnce_DeletedPath(t *testing.T) {
	fake := &fakeRetainStore{deleted: 7}
	retainAuditOnce(context.Background(), fake, 24*time.Hour)
	if len(fake.calls) != 1 || fake.lastMaxAge != 24*time.Hour {
		t.Fatalf("expected one call with 24h, got %+v", fake.calls)
	}
}

func TestRetainAuditOnce_ErrorPath(t *testing.T) {
	fake := &fakeRetainStore{err: errors.New("boom")}
	retainAuditOnce(context.Background(), fake, 1*time.Hour)
	if len(fake.calls) != 1 {
		t.Fatalf("expected one call even on error, got %+v", fake.calls)
	}
}

// TestRetainAuditLoop_ExitsOnCancel: the loop must exit promptly when
// ctx is cancelled rather than blocking forever on the ticker.
func TestRetainAuditLoop_ExitsOnCancel(t *testing.T) {
	fake := &fakeRetainStore{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		retainAuditLoop(ctx, fake, 1*time.Hour, 50*time.Millisecond)
		close(done)
	}()
	// The loop runs one immediate pass then waits on the ticker.
	// Cancel before the ticker fires so the test is not timing-sensitive.
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("loop did not exit within 2s of context cancel")
	}
	if len(fake.calls) < 1 {
		t.Fatalf("expected at least one RetainAudit call from the immediate pass, got %d", len(fake.calls))
	}
}

// TestRetainAuditLoop_NoopWhenMaxAgeZero: maxAge <= 0 should skip the
// goroutine loop entirely so the retention knob can be disabled.
func TestRetainAuditLoop_NoopWhenMaxAgeZero(t *testing.T) {
	fake := &fakeRetainStore{}
	retainAuditLoop(context.Background(), fake, 0, time.Millisecond)
	if len(fake.calls) != 0 {
		t.Fatalf("expected zero calls when maxAge=0, got %d", len(fake.calls))
	}
}
