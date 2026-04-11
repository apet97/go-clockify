package mcp

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestSessionEventHubBacklogReplay verifies new subscribers receive any
// already-buffered backlog events on subscribe (Last-Event-ID resumability
// substitute), then receive subsequent live events.
func TestSessionEventHubBacklogReplay(t *testing.T) {
	hub := newSessionEventHub(8, 4)
	if err := hub.Notify("first", map[string]any{"k": 1}); err != nil {
		t.Fatal(err)
	}
	if err := hub.Notify("second", map[string]any{"k": 2}); err != nil {
		t.Fatal(err)
	}

	ch, cancel := hub.subscribe()
	defer cancel()

	// Two backlog events should arrive immediately.
	got1 := <-ch
	got2 := <-ch
	if got1.method != "first" || got2.method != "second" {
		t.Fatalf("backlog order wrong: %q %q", got1.method, got2.method)
	}

	// A live event after subscribe should also arrive.
	if err := hub.Notify("third", nil); err != nil {
		t.Fatal(err)
	}
	got3 := <-ch
	if got3.method != "third" {
		t.Fatalf("live event method: %q", got3.method)
	}
}

// TestSessionEventHubBacklogTrim ensures the backlog cap drops oldest
// entries once exceeded, preserving the most-recent N for new subscribers.
func TestSessionEventHubBacklogTrim(t *testing.T) {
	hub := newSessionEventHub(2, 4) // backlog cap of 2
	for i, m := range []string{"a", "b", "c", "d"} {
		if err := hub.Notify(m, i); err != nil {
			t.Fatal(err)
		}
	}
	ch, cancel := hub.subscribe()
	defer cancel()

	// Should receive only the last 2 backlog entries: c, d.
	got1 := <-ch
	got2 := <-ch
	if got1.method != "c" || got2.method != "d" {
		t.Fatalf("trimmed backlog wrong: %q %q", got1.method, got2.method)
	}
}

// TestSessionEventHubSlowSubscriberDropped verifies that a subscriber whose
// channel is full gets evicted (dropped + closed) so a slow client cannot
// stall publishing for everyone.
func TestSessionEventHubSlowSubscriberDropped(t *testing.T) {
	hub := newSessionEventHub(0, 1) // bufferCap=1, no backlog
	ch, cancel := hub.subscribe()
	defer cancel()

	// Publish twice without reading. The first fits in the channel, the
	// second forces the subscriber to be dropped.
	if err := hub.Notify("first", nil); err != nil {
		t.Fatal(err)
	}
	if err := hub.Notify("second", nil); err != nil {
		t.Fatal(err)
	}

	// Drain the first event then expect the channel to be closed.
	first := <-ch
	if first.method != "first" {
		t.Fatalf("first event: %q", first.method)
	}
	// After eviction, channel reads should return zero-value with ok=false.
	deadline := time.After(time.Second)
	for {
		select {
		case _, open := <-ch:
			if !open {
				return
			}
		case <-deadline:
			t.Fatal("expected subscriber channel to be closed after drop")
		}
	}
}

// TestSessionEventHubClose closes the hub and verifies all subscribers see
// their channels close.
func TestSessionEventHubClose(t *testing.T) {
	hub := newSessionEventHub(4, 4)
	chA, cancelA := hub.subscribe()
	defer cancelA()
	chB, cancelB := hub.subscribe()
	defer cancelB()
	hub.close()
	for _, ch := range []<-chan sessionEvent{chA, chB} {
		if _, open := <-ch; open {
			t.Fatal("expected channel to be closed after hub.close")
		}
	}
}

// TestSessionEventHubLastEventIDReplay verifies subscribeFrom filters the
// backlog to events with id strictly greater than lastEventID.
func TestSessionEventHubLastEventIDReplay(t *testing.T) {
	hub := newSessionEventHub(16, 16)
	for _, m := range []string{"a", "b", "c", "d", "e"} {
		if err := hub.Notify(m, nil); err != nil {
			t.Fatal(err)
		}
	}

	ch, cancel := hub.subscribeFrom(2)
	defer cancel()

	// Events c(3), d(4), e(5) — strictly > 2.
	for _, want := range []string{"c", "d", "e"} {
		select {
		case got := <-ch:
			if got.method != want {
				t.Fatalf("got %q want %q", got.method, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for %q", want)
		}
	}
}

// TestSessionEventHubLastEventIDFutureSkip verifies that a lastEventID beyond
// the highest-stamped event yields no replay (but the subscriber still
// receives any subsequent live events).
func TestSessionEventHubLastEventIDFutureSkip(t *testing.T) {
	hub := newSessionEventHub(8, 4)
	if err := hub.Notify("one", nil); err != nil {
		t.Fatal(err)
	}
	ch, cancel := hub.subscribeFrom(999)
	defer cancel()

	// No backlog replay expected.
	select {
	case ev, ok := <-ch:
		if ok {
			t.Fatalf("unexpected replayed event: %+v", ev)
		}
	case <-time.After(50 * time.Millisecond):
		// ok — no events
	}

	// A new live event should still come through.
	if err := hub.Notify("two", nil); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-ch:
		if got.method != "two" {
			t.Fatalf("got %q want two", got.method)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for live event")
	}
}

// TestSessionEventHubCancelSubscriber removes one subscriber via its cancel
// function and verifies the others continue to receive events.
func TestSessionEventHubCancelSubscriber(t *testing.T) {
	hub := newSessionEventHub(0, 4)
	chKeep, cancelKeep := hub.subscribe()
	defer cancelKeep()
	_, cancelDrop := hub.subscribe()
	cancelDrop() // drop second subscriber

	if err := hub.Notify("event", nil); err != nil {
		t.Fatal(err)
	}
	got := <-chKeep
	if got.method != "event" {
		t.Fatalf("got %q want event", got.method)
	}

	// Calling cancel twice should be safe.
	cancelDrop()
}

// TestApplyHTTPBaselineHeaders verifies every defense-in-depth header is set.
func TestApplyHTTPBaselineHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	applyHTTPBaselineHeaders(rec)
	expected := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"Cache-Control":             "no-store",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		"Content-Security-Policy":   "default-src 'none'; frame-ancestors 'none'",
		"X-Frame-Options":           "DENY",
		"Referrer-Policy":           "no-referrer",
		"Permissions-Policy":        "()",
	}
	for k, want := range expected {
		if got := rec.Header().Get(k); got != want {
			t.Fatalf("%s: got %q want %q", k, got, want)
		}
	}
}

// TestAddSessionToInitializeResult covers both branches: a map result gets a
// sessionId field added without mutating the input; a non-map result is
// passed through unchanged.
func TestAddSessionToInitializeResult(t *testing.T) {
	t.Run("non_map_passthrough", func(t *testing.T) {
		got := addSessionToInitializeResult("not-a-map", "sess-1")
		if got != "not-a-map" {
			t.Fatalf("non-map should pass through, got %v", got)
		}
	})
	t.Run("adds_session_id", func(t *testing.T) {
		input := map[string]any{"a": 1, "b": 2}
		got := addSessionToInitializeResult(input, "sess-42")
		m, ok := got.(map[string]any)
		if !ok {
			t.Fatalf("expected map result, got %T", got)
		}
		if m["sessionId"] != "sess-42" {
			t.Fatalf("sessionId: got %v", m["sessionId"])
		}
		if m["a"] != 1 || m["b"] != 2 {
			t.Fatalf("existing fields lost: %+v", m)
		}
		// Original input must not be mutated.
		if _, found := input["sessionId"]; found {
			t.Fatal("original map mutated")
		}
	})
}

// TestRandomID asserts the helper produces a 32-char hex string and that
// successive calls produce distinct values (cryptographic randomness, not
// deterministic across runs).
func TestRandomID(t *testing.T) {
	id1 := randomID()
	id2 := randomID()
	if len(id1) != 32 || len(id2) != 32 {
		t.Fatalf("expected 32-char IDs, got %q %q", id1, id2)
	}
	if id1 == id2 {
		t.Fatal("two random IDs collided")
	}
	for _, c := range id1 {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Fatalf("unexpected char %q in randomID", c)
		}
	}
}

// TestStringsTrimSpace covers the tiny wrapper used for symbol naming.
func TestStringsTrimSpace(t *testing.T) {
	if got := stringsTrimSpace("  hello  "); got != "hello" {
		t.Fatalf("got %q", got)
	}
}
