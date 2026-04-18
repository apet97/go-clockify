package mcp

import (
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/metrics"
)

// TestSSEMetrics_SessionReap_Labels asserts reapOnce emits the ttl
// label on clockify_mcp_sessions_reaped_total — the branch that fires
// when expiresAt is in the past. Operators must see every reap, so a
// regression that bypassed Inc() (e.g., early-return) would show up
// here as a zero counter delta.
func TestSSEMetrics_SessionReap_Labels(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	opts.SessionTTL = 50 * time.Millisecond
	mgr.ttl = opts.SessionTTL
	mgr.idleGraceAfterDisconnect = 30 * time.Millisecond

	base := metrics.StreamableSessionsReapedTotal.Get("ttl")

	// Create two sessions, then backdate so both hit the TTL branch.
	for i := 0; i < 2; i++ {
		sessID := initializeStreamSession(t, streamableRPCHandler(opts, mgr),
			`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
		if sessID == "" {
			t.Fatalf("init %d failed", i)
		}
	}
	mgr.mu.Lock()
	for _, s := range mgr.items {
		s.expiresAt = time.Now().Add(-1 * time.Second)
		s.lastSeenAt = time.Now().Add(-1 * time.Second)
	}
	mgr.mu.Unlock()
	mgr.reapOnce(time.Now())

	if got := metrics.StreamableSessionsReapedTotal.Get("ttl") - base; got < 2 {
		t.Fatalf("expected >=2 ttl reaps, got delta %d", got)
	}
}

// TestSSEMetrics_SubscriberDrop asserts the slow_subscriber label
// increments when the hub evicts a subscriber with a full channel.
func TestSSEMetrics_SubscriberDrop(t *testing.T) {
	base := metrics.SSESubscriberDropsTotal.Get("slow_subscriber")
	hub := newSessionEventHub(4, 1) // 1-slot buffer so the second publish blocks.
	ch, cancel := hub.subscribe()
	defer cancel()

	// First publish fits in the 1-slot buffer.
	if err := hub.Notify("a", nil); err != nil {
		t.Fatal(err)
	}
	// Second publish — channel still full because the test never reads
	// from `ch`. The hub should evict us and bump the counter.
	if err := hub.Notify("b", nil); err != nil {
		t.Fatal(err)
	}
	// The channel should be closed now; drain to avoid a goroutine leak.
	for range ch {
	}

	if got := metrics.SSESubscriberDropsTotal.Get("slow_subscriber") - base; got < 1 {
		t.Fatalf("expected slow_subscriber drop increment, got %d", got)
	}
}

// TestSSEMetrics_ReplayMiss asserts that a resume using a Last-Event-ID
// older than the retained backlog ring increments the miss counter.
func TestSSEMetrics_ReplayMiss(t *testing.T) {
	base := metrics.SSEReplayMissesTotal.Get()
	hub := newSessionEventHub(2, 4) // 2-event backlog ring.

	// Publish 5 events: ring retains only the last 2 (ids 4 and 5).
	for i := 0; i < 5; i++ {
		if err := hub.Notify("e", nil); err != nil {
			t.Fatal(err)
		}
	}

	// Client resumes from id=1: oldest retained is 4, so events 2-3
	// were trimmed. Expect a miss.
	_, cancel := hub.subscribeFrom(1)
	defer cancel()
	if got := metrics.SSEReplayMissesTotal.Get() - base; got < 1 {
		t.Fatalf("expected replay miss increment, got %d", got)
	}

	// Client resumes from id=3: oldest retained is 4, so id=3 itself
	// was the last trimmed, no events between lastEventID+1=4 and
	// oldest=4 are missing — should NOT increment.
	snap := metrics.SSEReplayMissesTotal.Get()
	_, cancel2 := hub.subscribeFrom(3)
	defer cancel2()
	if got := metrics.SSEReplayMissesTotal.Get() - snap; got != 0 {
		t.Fatalf("expected no miss when lastEventID is adjacent to oldest retained, got %d", got)
	}
}
