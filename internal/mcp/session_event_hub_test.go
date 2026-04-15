package mcp

import (
	"sync"
	"testing"
)

// testingAllocsPerRun wraps testing.AllocsPerRun so the call site reads
// with a clear unit. Kept as a thin shim so a future envelope tightening
// is grep-discoverable.
func testingAllocsPerRun(runs int, f func()) float64 {
	return testing.AllocsPerRun(runs, f)
}

// TestSessionEventHub_NotifyZeroAllocSteadyState asserts the ring-buffer
// append path is zero-alloc once the ring has filled. The pre-ring
// implementation called append([]sessionEvent(nil), ...) on every
// overflow and deep-copied the tail; that cost is what motivated G2b.
//
// 10 warm-up notifies fill the ring (backlogCap=8), then AllocsPerRun
// measures 100 further notifies. Expected: 0 allocs per call. We
// intentionally use a no-subscriber hub so the measurement isolates
// the ring append from channel-send costs.
func TestSessionEventHub_NotifyZeroAllocSteadyState(t *testing.T) {
	hub := newSessionEventHub(8, 4)
	// Warm-up: fill the ring past capacity so every subsequent notify
	// exercises the overflow branch.
	for i := 0; i < 16; i++ {
		_ = hub.Notify("warmup", nil)
	}

	allocs := testingAllocsPerRun(100, func() {
		_ = hub.Notify("steady", nil)
	})
	if allocs != 0 {
		t.Fatalf("expected zero allocs in steady-state Notify, got %v", allocs)
	}
}

// TestSessionEventHub_ReplayPreservesOrderAfterOverflow asserts that a
// subscriber joining after the ring has wrapped still receives the
// retained window in publication order. Guards the (backlogStart+i)%cap
// walk in subscribeFrom — an off-by-one there would reorder events or
// skip the boundary entry.
func TestSessionEventHub_ReplayPreservesOrderAfterOverflow(t *testing.T) {
	const cap = 4
	hub := newSessionEventHub(cap, 16)
	// Publish 10 events into a ring of 4 — overflow 6 times.
	for i := 0; i < 10; i++ {
		_ = hub.Notify("evt", map[string]any{"seq": i})
	}

	ch, cancel := hub.subscribe()
	defer cancel()

	// Drain exactly `cap` events non-blocking — anything more would
	// indicate a replay mis-count.
	events := make([]sessionEvent, 0, cap)
	for i := 0; i < cap; i++ {
		select {
		case e := <-ch:
			events = append(events, e)
		default:
			t.Fatalf("expected %d events in replay, got %d", cap, len(events))
		}
	}
	// Drain any trailing event to guarantee no extras beyond the window.
	select {
	case extra := <-ch:
		t.Fatalf("unexpected extra event in replay: %+v", extra)
	default:
	}

	// The retained window after 10 writes into a cap-4 ring is ids 7..10.
	// (Event IDs are 1-indexed because lastEventID starts at 0 and
	// increments before assignment.)
	wantIDs := []uint64{7, 8, 9, 10}
	for i, want := range wantIDs {
		if events[i].id != want {
			t.Errorf("replay[%d]: got id=%d want=%d", i, events[i].id, want)
		}
	}
}

// TestSessionEventHub_ReplayRespectsLastEventID asserts Last-Event-ID
// resumption skips already-delivered events inside the retained window.
func TestSessionEventHub_ReplayRespectsLastEventID(t *testing.T) {
	hub := newSessionEventHub(8, 16)
	for i := 0; i < 5; i++ {
		_ = hub.Notify("evt", nil)
	}

	ch, cancel := hub.subscribeFrom(3) // resume after event id 3
	defer cancel()

	var ids []uint64
	for i := 0; i < 2; i++ {
		select {
		case e := <-ch:
			ids = append(ids, e.id)
		default:
			t.Fatalf("expected 2 events after resume, got %d", len(ids))
		}
	}
	select {
	case extra := <-ch:
		t.Fatalf("unexpected extra event after resume: %+v", extra)
	default:
	}
	if ids[0] != 4 || ids[1] != 5 {
		t.Errorf("resume ids = %v, want [4 5]", ids)
	}
}

// TestSessionEventHub_NotifyConcurrentWithSubscribe exercises the
// concurrent subscribe/notify path under the race detector. It does not
// assert delivery correctness (the ring buffer tests above cover order
// and overflow); the goal is to get -race to observe overlapping
// readers/writers on h.mu so a future data race regression fires here.
//
// Shape: bounded publishers emit a fixed number of events then exit,
// bounded subscribers range over their channel until close. After all
// publishers have drained, hub.close() releases any remaining
// subscribers. wg.Wait() then joins everyone. This avoids the prior
// busy-spin publisher shape that starved subscribers on low-GOMAXPROCS
// CI runners and timed out at 120s with all goroutines blocked on
// h.mu.
func TestSessionEventHub_NotifyConcurrentWithSubscribe(t *testing.T) {
	hub := newSessionEventHub(32, 16)

	const (
		publishers         = 4
		eventsPerPublisher = 50
		subscribers        = 20
	)

	var subsWG, pubsWG, ready sync.WaitGroup

	// `ready` gates publishers on every subscriber having successfully
	// called hub.subscribe(). Without this gate the scheduler can delay
	// a subscriber past pubsWG.Wait() → hub.close(); a late subscriber
	// would then register against a hub that has already been closed,
	// its channel would never be closed again, and the test would hang
	// in subsWG.Wait(). Observed as a 120s timeout in CI under the
	// "Verify build-tag wiring" step (no -race, different scheduler
	// timing than the race build).
	ready.Add(subscribers)

	for i := 0; i < subscribers; i++ {
		subsWG.Add(1)
		go func() {
			defer subsWG.Done()
			ch, cancel := hub.subscribe()
			defer cancel()
			ready.Done()
			// Drain until the hub closes our channel — either Notify's
			// non-blocking fan-out dropped us for being slow or
			// hub.close() fired below.
			for range ch {
			}
		}()
	}

	// Wait for every subscriber to be registered before publishing so
	// hub.close() later sees the full subscriber set.
	ready.Wait()

	for p := 0; p < publishers; p++ {
		pubsWG.Add(1)
		go func() {
			defer pubsWG.Done()
			for i := 0; i < eventsPerPublisher; i++ {
				_ = hub.Notify("concurrent", nil)
			}
		}()
	}

	// Wait for publishers to finish emitting, then close all subscriber
	// channels so the drain loops exit. subsWG then joins cleanly.
	pubsWG.Wait()
	hub.close()
	subsWG.Wait()
}

