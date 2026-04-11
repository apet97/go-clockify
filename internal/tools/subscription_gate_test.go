package tools

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/apet97/go-clockify/internal/mcp"
)

// TestSubscriptionGate_SkipsReadResourceWhenUnsubscribed is the
// W4-04c load-bearing assertion: when SubscriptionGate returns false
// for a URI, emitResourceUpdate must not call ReadResource (i.e. no
// GET against the Clockify API). The counting handler below records
// every GET against the time-entries endpoint so we can compare the
// before/after count around a mutation.
func TestSubscriptionGate_SkipsReadResourceWhenUnsubscribed(t *testing.T) {
	const entryID = "e-unsub"
	const wsID = "w1"

	var getCount atomic.Int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/time-entries/"+entryID) {
			getCount.Add(1)
			respondJSON(t, w, map[string]any{"id": entryID, "description": "re-read-should-not-happen"})
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/time-entries") {
			respondJSON(t, w, map[string]any{
				"id":          entryID,
				"description": "first",
				"timeInterval": map[string]any{
					"start":    "2026-04-11T10:00:00Z",
					"end":      "2026-04-11T11:00:00Z",
					"duration": "PT1H",
				},
			})
			return
		}
		http.NotFound(w, r)
	})
	defer cleanup()

	svc := New(client, wsID)
	emit := &recordingEmit{}
	svc.EmitResourceUpdate = emit.hook()
	// Gate reports "nobody is subscribed" for every URI.
	svc.SubscriptionGate = func(_ string) bool { return false }

	_, err := svc.AddEntry(context.Background(), map[string]any{
		"start":       "2026-04-11T10:00:00Z",
		"end":         "2026-04-11T11:00:00Z",
		"description": "first",
		"dry_run":     false,
	})
	if err != nil {
		t.Fatalf("AddEntry: %v", err)
	}

	if n := getCount.Load(); n != 0 {
		t.Fatalf("expected zero ReadResource GETs while unsubscribed, got %d", n)
	}
	if calls := emit.snapshot(); len(calls) != 0 {
		t.Fatalf("expected zero emits while unsubscribed, got %d: %+v", len(calls), calls)
	}
}

// TestSubscriptionGate_FiresWhenSubscribed is the mirror: when the
// gate returns true for a URI, the normal emit path still runs and
// publishes the notification. The per-URI granularity is exercised
// by returning true for /entry/ but false for /report/weekly/.
func TestSubscriptionGate_FiresWhenSubscribed(t *testing.T) {
	const entryID = "e-sub"
	const wsID = "w1"

	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/time-entries/"+entryID) {
			respondJSON(t, w, map[string]any{"id": entryID, "description": "subscribed"})
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/time-entries") {
			respondJSON(t, w, map[string]any{
				"id":          entryID,
				"description": "first",
				"timeInterval": map[string]any{
					"start":    "2026-04-11T10:00:00Z",
					"end":      "2026-04-11T11:00:00Z",
					"duration": "PT1H",
				},
			})
			return
		}
		http.NotFound(w, r)
	})
	defer cleanup()

	svc := New(client, wsID)
	emit := &recordingEmit{}
	svc.EmitResourceUpdate = emit.hook()
	svc.SubscriptionGate = func(uri string) bool {
		return strings.Contains(uri, "/entry/")
	}

	_, err := svc.AddEntry(context.Background(), map[string]any{
		"start":       "2026-04-11T10:00:00Z",
		"end":         "2026-04-11T11:00:00Z",
		"description": "first",
		"dry_run":     false,
	})
	if err != nil {
		t.Fatalf("AddEntry: %v", err)
	}

	calls := emit.snapshot()
	if len(calls) != 1 {
		t.Fatalf("expected exactly 1 emit (entry URI only), got %d: %+v", len(calls), calls)
	}
	if !strings.Contains(calls[0].URI, "/entry/"+entryID) {
		t.Fatalf("unexpected URI: %q", calls[0].URI)
	}
}

// TestServer_HasResourceSubscription covers the public wrapper
// exposed on internal/mcp.Server. Empty URIs return false, URIs with
// no active subscription return false, and URIs added via the
// public resources/subscribe JSON-RPC path return true.
func TestServer_HasResourceSubscription(t *testing.T) {
	srv := mcp.NewServer("test", nil, nil, nil)
	srv.ResourceProvider = stubResourceProvider{}

	if srv.HasResourceSubscription("") {
		t.Fatal("empty URI should return false")
	}
	const uri = "clockify://workspace/w1/entry/e1"
	if srv.HasResourceSubscription(uri) {
		t.Fatal("should be false before subscribe")
	}

	req := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`)
	if _, err := srv.DispatchMessage(context.Background(), req); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	sub := []byte(`{"jsonrpc":"2.0","id":2,"method":"resources/subscribe","params":{"uri":"` + uri + `"}}`)
	if _, err := srv.DispatchMessage(context.Background(), sub); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	if !srv.HasResourceSubscription(uri) {
		t.Fatal("should return true after subscribe")
	}
}

// stubResourceProvider satisfies mcp.ResourceProvider with empty
// lists so the server advertises the resources capability without
// needing a real tools.Service.
type stubResourceProvider struct{}

func (stubResourceProvider) ListResources(context.Context) ([]mcp.Resource, error) {
	return nil, nil
}

func (stubResourceProvider) ListResourceTemplates(context.Context) ([]mcp.ResourceTemplate, error) {
	return nil, nil
}

func (stubResourceProvider) ReadResource(context.Context, string) ([]mcp.ResourceContents, error) {
	return nil, nil
}
