package clockify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// BenchmarkClient_Get measures the per-request overhead of the
// upstream HTTP client against a localhost httptest.Server. The
// server returns a tiny JSON object so the measurement is dominated
// by the client's marshal/unmarshal + connection pool path, not by
// the upstream's response generation.
//
// This is the regression detector for the request hot path. A change
// to retry, backoff, query-param construction, or response decoding
// shows up here. The number is not meaningful for capacity planning
// — see docs/performance.md for that.
//
// Run: go test -bench=BenchmarkClient -benchtime=10x ./internal/clockify
func BenchmarkClient_Get(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"1","name":"bench"}`)
	}))
	defer srv.Close()

	client := NewClient("bench-key", srv.URL, 5*time.Second, 0)
	defer client.Close()

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var out map[string]any
		if err := client.Get(ctx, "/workspaces/x/projects", nil, &out); err != nil {
			b.Fatalf("Get: %v", err)
		}
	}
}

// BenchmarkClient_Post measures the same overhead with a request
// body so the marshal path is exercised. The httptest.Server reads
// and discards the body to mimic an upstream that accepts and
// acknowledges every write.
func BenchmarkClient_Post(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"1"}`)
	}))
	defer srv.Close()

	client := NewClient("bench-key", srv.URL, 5*time.Second, 0)
	defer client.Close()

	body := map[string]any{
		"description": "bench entry",
		"start":       "2026-04-13T00:00:00Z",
		"end":         "2026-04-13T01:00:00Z",
		"projectId":   "5e2c8f9b8c1f4a7d6e9b3c1a",
	}

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var out map[string]any
		if err := client.Post(ctx, "/workspaces/x/time-entries", body, &out); err != nil {
			b.Fatalf("Post: %v", err)
		}
	}
}
