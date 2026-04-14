package mcp

import (
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/controlplane"
)

// newReapTestManager returns a streamSessionManager wired against an
// in-memory control-plane store with the supplied grace/TTL pair. Used
// by the reap tests to drive reapOnce on a controlled clock without
// spinning up ServeStreamableHTTP.
func newReapTestManager(t *testing.T, ttl, grace time.Duration) *streamSessionManager {
	t.Helper()
	store, err := controlplane.Open("memory")
	if err != nil {
		t.Fatalf("control plane: %v", err)
	}
	return &streamSessionManager{
		ttl:                      ttl,
		idleGraceAfterDisconnect: grace,
		store:                    store,
		items:                    map[string]*streamSession{},
	}
}

// insertFakeSession registers a minimal session in the manager suitable
// for reap assertions. events is a live hub so SubscriberCount observes
// real subscribe/close state. runtime is populated enough that destroy()
// can drive the close path without panicking; Close is a no-op.
func insertFakeSession(t *testing.T, m *streamSessionManager, id string, lastSeen, expires time.Time) *streamSession {
	t.Helper()
	sess := &streamSession{
		id:         id,
		principal:  authn.Principal{Subject: "reap-test"},
		runtime:    &StreamableSessionRuntime{Close: func() {}},
		events:     newSessionEventHub(8, 4),
		createdAt:  lastSeen,
		lastSeenAt: lastSeen,
		expiresAt:  expires,
	}
	m.mu.Lock()
	m.items[id] = sess
	m.mu.Unlock()
	return sess
}

// TestReapOnce_EvictsExpired confirms the original TTL rule still
// evicts sessions whose expiresAt is in the past. Regression guard for
// the reapOnce extraction — the new orphan path must not break the
// baseline TTL eviction the weekly bench depends on.
func TestReapOnce_EvictsExpired(t *testing.T) {
	m := newReapTestManager(t, 30*time.Minute, 5*time.Minute)
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	// lastSeen recent, expiresAt in the past → TTL evict path
	insertFakeSession(t, m, "expired-1", now.Add(-time.Minute), now.Add(-time.Second))

	m.reapOnce(now)

	m.mu.Lock()
	remaining := len(m.items)
	m.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("expected 0 sessions after TTL reap, got %d", remaining)
	}
}

// TestReapOnce_EvictsOrphanedSession confirms the new rule: a session
// whose SSE subscribers have all dropped AND whose lastSeenAt is older
// than idleGraceAfterDisconnect is evicted early, before TTL.
//
// This is the load-bearing test for G2a — proving dead sessions no
// longer sit for the full SessionTTL when the client drops TCP without
// closing the session cleanly.
func TestReapOnce_EvictsOrphanedSession(t *testing.T) {
	grace := 5 * time.Minute
	m := newReapTestManager(t, 30*time.Minute, grace)
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	// lastSeenAt = 6 minutes ago (past grace), expiresAt still in future
	// (TTL rule alone would keep this session for another 24 minutes).
	sess := insertFakeSession(t, m, "orphan-1",
		now.Add(-6*time.Minute),
		now.Add(24*time.Minute),
	)
	// Zero subscribers — the default state for a hub that had a client
	// drop their SSE GET. Do not subscribe here; SubscriberCount() == 0.
	if got := sess.events.SubscriberCount(); got != 0 {
		t.Fatalf("precondition: expected 0 subscribers, got %d", got)
	}

	m.reapOnce(now)

	m.mu.Lock()
	remaining := len(m.items)
	m.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("expected 0 sessions after orphan reap, got %d", remaining)
	}
}

// TestReapOnce_KeepsOrphanedInsideGrace confirms a session with zero
// subscribers but lastSeenAt inside the grace window is NOT evicted.
// This is the mitigation for the legitimate-slow-reconnect risk called
// out in the plan: a client that drops TCP and reconnects within the
// grace (SSE retry backoff) must survive the sweep.
func TestReapOnce_KeepsOrphanedInsideGrace(t *testing.T) {
	grace := 5 * time.Minute
	m := newReapTestManager(t, 30*time.Minute, grace)
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	// lastSeenAt 1 minute ago — client may still be in retry backoff.
	insertFakeSession(t, m, "reconnecting-1",
		now.Add(-1*time.Minute),
		now.Add(29*time.Minute),
	)

	m.reapOnce(now)

	m.mu.Lock()
	remaining := len(m.items)
	m.mu.Unlock()
	if remaining != 1 {
		t.Fatalf("expected session to survive inside grace window, got %d remaining", remaining)
	}
}

// TestReapOnce_KeepsSessionWithActiveSubscriber confirms a session is
// not evicted while an SSE client is actively subscribed, even if its
// lastSeenAt is old. The subscription itself is the liveness signal.
func TestReapOnce_KeepsSessionWithActiveSubscriber(t *testing.T) {
	grace := 5 * time.Minute
	m := newReapTestManager(t, 30*time.Minute, grace)
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	sess := insertFakeSession(t, m, "active-1",
		now.Add(-10*time.Minute), // well past grace
		now.Add(20*time.Minute),
	)
	_, cancel := sess.events.subscribe()
	t.Cleanup(cancel)

	m.reapOnce(now)

	m.mu.Lock()
	remaining := len(m.items)
	m.mu.Unlock()
	if remaining != 1 {
		t.Fatalf("expected active-subscriber session to survive, got %d remaining", remaining)
	}
}

// TestReapOnce_GraceDisabledWhenZero confirms that setting grace to 0
// disables the orphan rule — a session with zero subscribers and
// lastSeenAt long in the past is kept until TTL. Guards the default
// fallback in ServeStreamableHTTP so tests constructing a manager with
// grace=0 do not see surprise evictions.
func TestReapOnce_GraceDisabledWhenZero(t *testing.T) {
	m := newReapTestManager(t, 30*time.Minute, 0)
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	insertFakeSession(t, m, "no-grace-1",
		now.Add(-1*time.Hour),
		now.Add(29*time.Minute),
	)

	m.reapOnce(now)

	m.mu.Lock()
	remaining := len(m.items)
	m.mu.Unlock()
	if remaining != 1 {
		t.Fatalf("expected session to survive with grace=0, got %d remaining", remaining)
	}
}
