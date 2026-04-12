package mcp

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

type hubTestNotifier struct {
	mu       sync.Mutex
	calls    []string
	forceErr error
}

func (r *hubTestNotifier) Notify(method string, _ any) error {
	r.mu.Lock()
	r.calls = append(r.calls, method)
	r.mu.Unlock()
	return r.forceErr
}

func (r *hubTestNotifier) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func TestNotifierHub_FanOut(t *testing.T) {
	var hub notifierHub
	a := &hubTestNotifier{}
	b := &hubTestNotifier{}
	removeA := hub.add(a)
	_ = hub.add(b)

	if err := hub.notify("test/event", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.count() != 1 || b.count() != 1 {
		t.Fatalf("expected both notifiers to be called once, got a=%d b=%d", a.count(), b.count())
	}

	removeA()
	_ = hub.notify("test/event2", nil)
	if a.count() != 1 {
		t.Fatalf("removed notifier should not receive, got a=%d", a.count())
	}
	if b.count() != 2 {
		t.Fatalf("remaining notifier should receive, got b=%d", b.count())
	}
}

func TestNotifierHub_RemoveStopsDelivery(t *testing.T) {
	var hub notifierHub
	n := &hubTestNotifier{}
	remove := hub.add(n)
	remove()

	_ = hub.notify("test/event", nil)
	if n.count() != 0 {
		t.Fatal("removed notifier should receive no events")
	}
}

func TestNotifierHub_ErrorDoesNotBlockOthers(t *testing.T) {
	var hub notifierHub
	failing := &hubTestNotifier{forceErr: errors.New("broken")}
	healthy := &hubTestNotifier{}
	_ = hub.add(failing)
	_ = hub.add(healthy)

	err := hub.notify("test/event", nil)
	if err == nil {
		t.Fatal("expected error from failing notifier")
	}
	if healthy.count() != 1 {
		t.Fatalf("healthy notifier should still receive, got %d", healthy.count())
	}
}

func TestNotifierHub_EmptyIsNoop(t *testing.T) {
	var hub notifierHub
	if err := hub.notify("test/event", nil); err != nil {
		t.Fatalf("empty hub should return nil, got %v", err)
	}
}

func TestNotifierHub_ConcurrentAddRemove(t *testing.T) {
	var hub notifierHub
	var wg sync.WaitGroup
	var sent atomic.Int64

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n := &hubTestNotifier{}
			remove := hub.add(n)
			_ = hub.notify("concurrent", nil)
			sent.Add(1)
			remove()
		}()
	}
	wg.Wait()
	if sent.Load() != 20 {
		t.Fatalf("expected 20 sends, got %d", sent.Load())
	}
}

func TestNotifierHub_DoubleRemoveIsSafe(t *testing.T) {
	var hub notifierHub
	n := &hubTestNotifier{}
	remove := hub.add(n)
	remove()
	remove() // should not panic
	if hub.len() != 0 {
		t.Fatalf("expected empty hub after double remove, got %d", hub.len())
	}
}
