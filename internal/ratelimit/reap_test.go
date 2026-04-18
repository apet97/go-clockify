package ratelimit

import (
	"context"
	"testing"
	"time"
)

// TestReapIdleSubjects asserts that per-subject limiters idle past the
// TTL are evicted while active ones (last acquire inside the window)
// are preserved. Before B3 the subjects map grew monotonically for the
// life of the process; long-lived multi-tenant deployments would leak
// one entry per unique subject forever.
func TestReapIdleSubjects(t *testing.T) {
	rl := New(0, 0, 60_000)
	rl.SetPerTokenLimits(4, 10)

	// Prime two subjects at t0.
	rl.subjectLimiterFor("alice")
	rl.subjectLimiterFor("bob")
	if got := rl.SubjectCount(); got != 2 {
		t.Fatalf("expected 2 subjects, got %d", got)
	}

	// Advance "bob" forward so only "alice" is idle past the cutoff.
	// We can't fake time.Now() inside subjectLimiterFor, so we touch
	// bob via a fresh acquire which re-stamps lastSeenMillis.
	rl.subjectLimiterFor("bob")

	// Reap at t0+1h with maxIdle=30m: alice was last seen ~0s ago, bob
	// was just re-touched, so neither should evict. Establishes the
	// negative path.
	if evicted := rl.ReapIdleSubjects(time.Now(), 30*time.Minute); evicted != 0 {
		t.Fatalf("expected 0 evictions inside TTL, got %d", evicted)
	}

	// Simulate alice being idle for 2h: rewind her lastSeen directly.
	rl.subjectsMu.Lock()
	rl.subjects["alice"].lastSeenMillis.Store(time.Now().Add(-2 * time.Hour).UnixMilli())
	rl.subjectsMu.Unlock()

	if evicted := rl.ReapIdleSubjects(time.Now(), 30*time.Minute); evicted != 1 {
		t.Fatalf("expected 1 eviction, got %d", evicted)
	}
	if got := rl.SubjectCount(); got != 1 {
		t.Fatalf("expected 1 surviving subject, got %d", got)
	}
}

// TestReapIdleSubjects_PreservesActiveAcquirers asserts that a subject
// currently holding a concurrency slot is never evicted, even if its
// lastSeen is past the cutoff. Dropping it would strand the outstanding
// release handle.
func TestReapIdleSubjects_PreservesActiveAcquirers(t *testing.T) {
	rl := New(0, 0, 60_000)
	rl.SetPerTokenLimits(2, 10)

	release, _, err := rl.AcquireForSubject(context.Background(), "charlie")
	if err != nil {
		t.Fatalf("AcquireForSubject: %v", err)
	}
	defer release()

	// Backdate lastSeen so the cutoff would normally select charlie.
	rl.subjectsMu.Lock()
	rl.subjects["charlie"].lastSeenMillis.Store(time.Now().Add(-1 * time.Hour).UnixMilli())
	rl.subjectsMu.Unlock()

	if evicted := rl.ReapIdleSubjects(time.Now(), 10*time.Minute); evicted != 0 {
		t.Fatalf("expected 0 evictions while slot held, got %d", evicted)
	}
	if got := rl.SubjectCount(); got != 1 {
		t.Fatalf("expected 1 subject to survive, got %d", got)
	}
}

// TestStartSubjectReaper exercises the background goroutine path by
// running a tight interval, then letting ctx cancel so the goroutine
// exits cleanly. Confirms the reaper loop wires ReapIdleSubjects into
// a ticker and respects context cancellation.
func TestStartSubjectReaper(t *testing.T) {
	rl := New(0, 0, 60_000)
	rl.SetPerTokenLimits(0, 10)

	rl.subjectLimiterFor("dave")
	// Backdate so the first tick will reap.
	rl.subjectsMu.Lock()
	rl.subjects["dave"].lastSeenMillis.Store(time.Now().Add(-1 * time.Hour).UnixMilli())
	rl.subjectsMu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rl.StartSubjectReaper(ctx, 10*time.Millisecond, 30*time.Minute)

	deadline := time.After(1 * time.Second)
	for {
		if rl.SubjectCount() == 0 {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("subject not reaped within 1s, count=%d", rl.SubjectCount())
		case <-time.After(15 * time.Millisecond):
		}
	}
}

// TestStartSubjectReaper_NoopOnZeroInterval documents the contract that
// a zero interval disables reaping rather than spinning the goroutine
// hot.
func TestStartSubjectReaper_NoopOnZeroInterval(t *testing.T) {
	rl := New(0, 0, 60_000)
	rl.SetPerTokenLimits(0, 10)
	rl.subjectLimiterFor("eve")
	rl.subjectsMu.Lock()
	rl.subjects["eve"].lastSeenMillis.Store(time.Now().Add(-1 * time.Hour).UnixMilli())
	rl.subjectsMu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rl.StartSubjectReaper(ctx, 0, 30*time.Minute) // interval=0 -> no-op

	time.Sleep(50 * time.Millisecond)
	if got := rl.SubjectCount(); got != 1 {
		t.Fatalf("expected reaper to be a no-op, subject was evicted anyway; count=%d", got)
	}
}
