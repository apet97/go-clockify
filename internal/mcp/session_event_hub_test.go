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
// concurrent subscribe/notify path under race detector. The race is
// non-deterministic but repeated runs under `go test -race` make
// data-race regressions surface quickly.
func TestSessionEventHub_NotifyConcurrentWithSubscribe(t *testing.T) {
	hub := newSessionEventHub(32, 16)

	var wg sync.WaitGroup
	// Publisher goroutine: notify continuously for the test duration.
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = hub.Notify("concurrent", nil)
			}
		}
	}()

	// Subscriber goroutines: subscribe, drain a few events, unsubscribe.
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, cancel := hub.subscribe()
			drained := 0
			for drained < 5 {
				select {
				case _, ok := <-ch:
					if !ok {
						return
					}
					drained++
				case <-stop:
					cancel()
					return
				}
			}
			cancel()
		}()
	}

	// Let the workers run briefly, then stop.
	close(stop)
	wg.Wait()
}

