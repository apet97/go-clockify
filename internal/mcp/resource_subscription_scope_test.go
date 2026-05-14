package mcp

import (
	"context"
	"slices"
	"sync"
	"testing"
)

// scopedRecordingNotifier is the concurrency-safe variant of
// recordingNotifier used by the per-notifier subscription tests. We need
// a fresh type rather than reusing the existing recordingNotifier because
// two notifiers in the same test share the server's notifierHub goroutine
// surface and the existing recordingNotifier appends without a mutex.
type scopedRecordingNotifier struct {
	name string
	mu   sync.Mutex
	got  []scopedNotificationCall
}

type scopedNotificationCall struct {
	Method string
	Params any
}

func (s *scopedRecordingNotifier) Notify(method string, params any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.got = append(s.got, scopedNotificationCall{Method: method, Params: params})
	return nil
}

func (s *scopedRecordingNotifier) snapshot() []scopedNotificationCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]scopedNotificationCall, len(s.got))
	copy(out, s.got)
	return out
}

func (s *scopedRecordingNotifier) methods() []string {
	calls := s.snapshot()
	out := make([]string, 0, len(calls))
	for _, c := range calls {
		out = append(out, c.Method)
	}
	return out
}

func (s *scopedRecordingNotifier) updates(uri string) int {
	calls := s.snapshot()
	n := 0
	for _, c := range calls {
		if c.Method != "notifications/resources/updated" {
			continue
		}
		params, ok := c.Params.(map[string]any)
		if !ok {
			continue
		}
		if got, _ := params["uri"].(string); got == uri {
			n++
		}
	}
	return n
}

// newScopeTestServer mirrors newResourceTestServer but skips the initial
// SetNotifier so each test wires its own notifier set.
func newScopeTestServer(t *testing.T) *Server {
	t.Helper()
	server := NewServer("test", nil, nil, nil)
	server.ResourceProvider = &stubResourceProvider{}
	server.initialized.Store(true)
	return server
}

// subscribe drives the public dispatch path so the context wiring is
// covered, not just the package-internal handler signature.
func subscribe(t *testing.T, srv *Server, n Notifier, uri string) {
	t.Helper()
	ctx := WithNotifier(context.Background(), n)
	resp := srv.handle(ctx, Request{
		JSONRPC: "2.0", ID: 1, Method: "resources/subscribe",
		Params: map[string]any{"uri": uri},
	})
	if resp.Error != nil {
		t.Fatalf("subscribe(%q) failed: %+v", uri, resp.Error)
	}
}

func unsubscribe(t *testing.T, srv *Server, n Notifier, uri string) {
	t.Helper()
	ctx := WithNotifier(context.Background(), n)
	resp := srv.handle(ctx, Request{
		JSONRPC: "2.0", ID: 2, Method: "resources/unsubscribe",
		Params: map[string]any{"uri": uri},
	})
	if resp.Error != nil {
		t.Fatalf("unsubscribe(%q) failed: %+v", uri, resp.Error)
	}
}

// TestResourceSubscription_PerNotifier_Isolation pins notifier scoping: when
// two notifiers share one *mcp.Server and only one of them has called
// resources/subscribe, NotifyResourceUpdated must deliver only to the
// subscriber. Before the per-notifier scoping change this fanned out via
// notifierHub to every registered notifier; the test goes RED on the old
// behaviour ("nope: B observed leaked update").
func TestResourceSubscription_PerNotifier_Isolation(t *testing.T) {
	srv := newScopeTestServer(t)
	a := &scopedRecordingNotifier{name: "A"}
	b := &scopedRecordingNotifier{name: "B"}
	removeA := srv.AddNotifier(a)
	defer removeA()
	removeB := srv.AddNotifier(b)
	defer removeB()

	const uri = "clockify://workspace/ws/entry/iso"
	subscribe(t, srv, a, uri)

	srv.NotifyResourceUpdated(uri, ResourceUpdateDelta{})

	if got := a.updates(uri); got != 1 {
		t.Fatalf("subscriber A: want 1 resources/updated for %q, got %d (all calls: %+v)", uri, got, a.snapshot())
	}
	if got := b.updates(uri); got != 0 {
		t.Fatalf("non-subscriber B: want 0 resources/updated, got %d (leaked across streams; all calls: %+v)", got, b.snapshot())
	}
}

// TestResourceSubscription_DifferentURIs_NoCrossTalk pins the inverse:
// A subscribed to X and B subscribed to Y must each receive only their
// URI's updates. Guards against a regression that would deliver any
// resource update to any subscribed notifier.
func TestResourceSubscription_DifferentURIs_NoCrossTalk(t *testing.T) {
	srv := newScopeTestServer(t)
	a := &scopedRecordingNotifier{name: "A"}
	b := &scopedRecordingNotifier{name: "B"}
	removeA := srv.AddNotifier(a)
	defer removeA()
	removeB := srv.AddNotifier(b)
	defer removeB()

	const uriX = "clockify://workspace/ws/entry/x"
	const uriY = "clockify://workspace/ws/entry/y"
	subscribe(t, srv, a, uriX)
	subscribe(t, srv, b, uriY)

	srv.NotifyResourceUpdated(uriX, ResourceUpdateDelta{})
	srv.NotifyResourceUpdated(uriY, ResourceUpdateDelta{})

	if got := a.updates(uriX); got != 1 {
		t.Fatalf("A want 1 update for X, got %d", got)
	}
	if got := a.updates(uriY); got != 0 {
		t.Fatalf("A want 0 updates for Y, got %d (cross-talk)", got)
	}
	if got := b.updates(uriY); got != 1 {
		t.Fatalf("B want 1 update for Y, got %d", got)
	}
	if got := b.updates(uriX); got != 0 {
		t.Fatalf("B want 0 updates for X, got %d (cross-talk)", got)
	}
}

// TestResourceSubscription_Unsubscribe_PerNotifier pins that
// resources/unsubscribe only affects the calling notifier; a sibling
// stream subscribed to the same URI keeps receiving updates.
func TestResourceSubscription_Unsubscribe_PerNotifier(t *testing.T) {
	srv := newScopeTestServer(t)
	a := &scopedRecordingNotifier{name: "A"}
	b := &scopedRecordingNotifier{name: "B"}
	removeA := srv.AddNotifier(a)
	defer removeA()
	removeB := srv.AddNotifier(b)
	defer removeB()

	const uri = "clockify://workspace/ws/entry/shared"
	subscribe(t, srv, a, uri)
	subscribe(t, srv, b, uri)

	unsubscribe(t, srv, a, uri)

	srv.NotifyResourceUpdated(uri, ResourceUpdateDelta{})

	if got := a.updates(uri); got != 0 {
		t.Fatalf("A unsubscribed but still got %d updates", got)
	}
	if got := b.updates(uri); got != 1 {
		t.Fatalf("B still subscribed, want 1 update, got %d", got)
	}
	if !srv.HasResourceSubscription(uri) {
		t.Fatalf("HasResourceSubscription must remain true while B is still subscribed")
	}
}

// TestResourceSubscription_NotifierRemoval_GCs pins that closing a stream
// (notifierHub.add's remove closure) drops every subscription the stream
// owned. Without this, HasResourceSubscription would lie about active
// subscribers after every stream disconnect and the tools layer's
// skip-fetch gate would keep paying for reads nobody wants.
func TestResourceSubscription_NotifierRemoval_GCs(t *testing.T) {
	srv := newScopeTestServer(t)
	a := &scopedRecordingNotifier{name: "A"}
	b := &scopedRecordingNotifier{name: "B"}
	removeA := srv.AddNotifier(a)
	removeB := srv.AddNotifier(b)
	defer removeB()

	const uri = "clockify://workspace/ws/entry/gc"
	subscribe(t, srv, a, uri)
	if !srv.HasResourceSubscription(uri) {
		t.Fatalf("subscription not registered for A")
	}

	removeA() // simulate stream close — A is gone.

	if srv.HasResourceSubscription(uri) {
		t.Fatalf("HasResourceSubscription must be false after the only subscriber's notifier was removed")
	}
	srv.NotifyResourceUpdated(uri, ResourceUpdateDelta{})
	if got := a.updates(uri); got != 0 {
		t.Fatalf("A removed but still observed %d updates", got)
	}
	if got := b.updates(uri); got != 0 {
		t.Fatalf("B never subscribed but observed %d updates", got)
	}
}

// TestToolsListChanged_RemainsBroadcast guards against a regression that
// would route every server-initiated notification through the per-URI
// fan-out path. tools/list_changed is intentionally a broadcast — every
// connected client must learn that the visible tool set changed,
// regardless of resource subscription state.
func TestToolsListChanged_RemainsBroadcast(t *testing.T) {
	srv := newScopeTestServer(t)
	a := &scopedRecordingNotifier{name: "A"}
	b := &scopedRecordingNotifier{name: "B"}
	removeA := srv.AddNotifier(a)
	defer removeA()
	removeB := srv.AddNotifier(b)
	defer removeB()

	if err := srv.Notify("notifications/tools/list_changed", nil); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	for _, n := range []*scopedRecordingNotifier{a, b} {
		if !slices.Contains(n.methods(), "notifications/tools/list_changed") {
			t.Fatalf("%s did not receive tools/list_changed broadcast: %+v", n.name, n.snapshot())
		}
	}
}

// TestResourceSubscription_BroadcastAndPerNotifier_NoDoubleDelivery
// guards the mixed mode where one subscribe call landed via the
// broadcastSubscriber sentinel (no Notifier in ctx) and another via
// the per-notifier path on the same URI. The broadcast branch is the
// strict superset, so NotifyResourceUpdated must take it alone — a
// future refactor that calls both s.Notify and the per-target loop
// would double-deliver to the real notifier.
func TestResourceSubscription_BroadcastAndPerNotifier_NoDoubleDelivery(t *testing.T) {
	srv := newScopeTestServer(t)
	a := &scopedRecordingNotifier{name: "A"}
	removeA := srv.AddNotifier(a)
	defer removeA()

	const uri = "clockify://workspace/ws/entry/mixed"

	// Per-notifier subscribe via WithNotifier(ctx, A).
	subscribe(t, srv, a, uri)
	// Legacy subscribe via bare ctx — lands on broadcastSubscriber.
	resp := srv.handle(context.Background(), Request{
		JSONRPC: "2.0", ID: 8, Method: "resources/subscribe",
		Params: map[string]any{"uri": uri},
	})
	if resp.Error != nil {
		t.Fatalf("legacy subscribe: %+v", resp.Error)
	}

	srv.NotifyResourceUpdated(uri, ResourceUpdateDelta{})

	if got := a.updates(uri); got != 1 {
		t.Fatalf("mixed mode must deliver exactly once to A; got %d (calls=%+v)", got, a.snapshot())
	}
}

// TestResourceSubscription_BroadcastSentinel pins backwards compatibility
// for handlers invoked without a Notifier in ctx (e.g. direct unit tests
// against handle). The legacy behaviour was server-wide subscription
// fan-out via Server.Notify; the broadcastSubscriber sentinel preserves
// it so older tests do not break.
func TestResourceSubscription_BroadcastSentinel(t *testing.T) {
	srv := newScopeTestServer(t)
	n := &scopedRecordingNotifier{name: "single"}
	removeN := srv.AddNotifier(n)
	defer removeN()

	const uri = "clockify://workspace/ws/entry/legacy"
	resp := srv.handle(context.Background(), Request{
		JSONRPC: "2.0", ID: 7, Method: "resources/subscribe",
		Params: map[string]any{"uri": uri},
	})
	if resp.Error != nil {
		t.Fatalf("subscribe (no notifier ctx): %+v", resp.Error)
	}

	srv.NotifyResourceUpdated(uri, ResourceUpdateDelta{})
	if got := n.updates(uri); got != 1 {
		t.Fatalf("broadcast-sentinel path: want 1 update, got %d", got)
	}
}
