package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestResolveCacheCachesPositiveProjectName(t *testing.T) {
	var projectCalls atomic.Int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workspaces/ws1/projects":
			projectCalls.Add(1)
			q := r.URL.Query()
			if q.Get("name") != "Alpha" || q.Get("strict-name-search") != "true" {
				t.Fatalf("unexpected project query: %s", r.URL.RawQuery)
			}
			respondJSON(t, w, []map[string]any{{"id": "p1", "name": "Alpha"}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	for i := range 2 {
		id, err := svc.resolveProjectID(context.Background(), "ws1", "Alpha")
		if err != nil {
			t.Fatalf("resolve project #%d: %v", i+1, err)
		}
		if id != "p1" {
			t.Fatalf("resolve project #%d = %q, want p1", i+1, id)
		}
	}
	if got := projectCalls.Load(); got != 1 {
		t.Fatalf("project resolver upstream calls = %d, want 1", got)
	}

	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			id, err := svc.resolveProjectID(context.Background(), "ws1", "alpha")
			if err != nil {
				t.Errorf("resolve cached project: %v", err)
			}
			if id != "p1" {
				t.Errorf("resolve cached project = %q, want p1", id)
			}
		})
	}
	wg.Wait()
	if got := projectCalls.Load(); got != 1 {
		t.Fatalf("project resolver upstream calls after cached parallel hits = %d, want 1", got)
	}
}

func TestResolveCacheSkipsUserEmail(t *testing.T) {
	var userCalls atomic.Int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workspaces/ws1/users":
			userCalls.Add(1)
			q := r.URL.Query()
			if q.Get("email") != "owner@example.com" {
				t.Fatalf("unexpected user query: %s", r.URL.RawQuery)
			}
			respondJSON(t, w, []map[string]any{{"id": "u1", "name": "Owner", "email": "owner@example.com"}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	for i := range 2 {
		id, err := svc.resolveUserID(context.Background(), "ws1", "owner@example.com")
		if err != nil {
			t.Fatalf("resolve user #%d: %v", i+1, err)
		}
		if id != "u1" {
			t.Fatalf("resolve user #%d = %q, want u1", i+1, id)
		}
	}
	if got := userCalls.Load(); got != 2 {
		t.Fatalf("email user resolver upstream calls = %d, want 2", got)
	}
}

func TestResolveCacheDoesNotCacheErrors(t *testing.T) {
	var projectCalls atomic.Int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/ws1/projects" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		call := projectCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			if err := json.NewEncoder(w).Encode([]map[string]any{}); err != nil {
				t.Fatalf("encode empty response: %v", err)
			}
			return
		}
		if err := json.NewEncoder(w).Encode([]map[string]any{{"id": "p1", "name": "Alpha"}}); err != nil {
			t.Fatalf("encode project response: %v", err)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.resolveProjectID(context.Background(), "ws1", "Alpha"); err == nil {
		t.Fatal("expected first resolve to fail")
	}
	id, err := svc.resolveProjectID(context.Background(), "ws1", "Alpha")
	if err != nil {
		t.Fatalf("second resolve should not use cached error: %v", err)
	}
	if id != "p1" {
		t.Fatalf("second resolve = %q, want p1", id)
	}
	if got := projectCalls.Load(); got != 2 {
		t.Fatalf("project resolver upstream calls = %d, want 2", got)
	}
}

func TestResolveCacheTaskScopeIncludesProjectID(t *testing.T) {
	var taskCalls atomic.Int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workspaces/ws1/projects/p1/tasks":
			taskCalls.Add(1)
			respondJSON(t, w, []map[string]any{{"id": "t1", "name": "Build"}})
		case "/workspaces/ws1/projects/p2/tasks":
			taskCalls.Add(1)
			respondJSON(t, w, []map[string]any{{"id": "t2", "name": "Build"}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	for _, tt := range []struct {
		projectID string
		want      string
	}{
		{"p1", "t1"},
		{"p2", "t2"},
		{"p1", "t1"},
	} {
		id, err := svc.resolveTaskID(context.Background(), "ws1", tt.projectID, "Build")
		if err != nil {
			t.Fatalf("resolve task in %s: %v", tt.projectID, err)
		}
		if id != tt.want {
			t.Fatalf("resolve task in %s = %q, want %s", tt.projectID, id, tt.want)
		}
	}
	if got := taskCalls.Load(); got != 2 {
		t.Fatalf("task resolver upstream calls = %d, want 2", got)
	}
}

func TestResolveCacheSkipsClockifyIDs(t *testing.T) {
	client := clockify.NewClient("test-key", "http://127.0.0.1:1", 0, 0)
	svc := New(client, "ws1")
	id, err := svc.resolveProjectID(context.Background(), "ws1", "5e1b2c3d4e5f6a7b8c9d0e1f")
	if err != nil {
		t.Fatalf("resolve ID: %v", err)
	}
	if id != "5e1b2c3d4e5f6a7b8c9d0e1f" {
		t.Fatalf("resolve ID = %q", id)
	}
	if len(svc.resolver.entries) != 0 {
		t.Fatalf("ID path should not populate resolve cache: %+v", svc.resolver.entries)
	}
}

func TestResolveCacheReclaimsExpiredEntries(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", 0, 0), "ws1")
	svc.resolver.entries = map[resolveKey]resolveEntry{}
	expired := time.Now().Add(-time.Minute)
	for i := range 1_100 {
		svc.resolver.entries[resolveKey{kind: "project", scope: "ws1", ref: fmt.Sprintf("old-%d", i)}] = resolveEntry{
			id:        fmt.Sprintf("p-%d", i),
			expiresAt: expired,
		}
	}
	svc.resolveCacheSet(resolveKey{kind: "project", scope: "ws1", ref: "fresh"}, "fresh-id", time.Now().Add(time.Minute))
	if got := len(svc.resolver.entries); got != 1 {
		t.Fatalf("resolve cache retained expired entries: len=%d want 1", got)
	}
	if _, ok := svc.resolver.entries[resolveKey{kind: "project", scope: "ws1", ref: "fresh"}]; !ok {
		t.Fatalf("fresh cache entry missing after sweep: %+v", svc.resolver.entries)
	}
}

func BenchmarkResolveCacheProjectIDHit(b *testing.B) {
	var projectCalls atomic.Int32
	client, cleanup := newTestClient(b, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/ws1/projects" {
			b.Fatalf("unexpected path: %s", r.URL.Path)
		}
		projectCalls.Add(1)
		respondJSON(b, w, []map[string]any{{"id": "p1", "name": "Alpha"}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	if id, err := svc.resolveProjectID(context.Background(), "ws1", "Alpha"); err != nil || id != "p1" {
		b.Fatalf("prime resolve id=%q err=%v", id, err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		id, err := svc.resolveProjectID(context.Background(), "ws1", "Alpha")
		if err != nil {
			b.Fatalf("resolve cached project: %v", err)
		}
		if id != "p1" {
			b.Fatalf("resolve cached project = %q, want p1", id)
		}
	}
	b.StopTimer()
	if got := projectCalls.Load(); got != 1 {
		b.Fatalf("project resolver upstream calls = %d, want 1", got)
	}
}
