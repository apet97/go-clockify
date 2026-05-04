package resolve

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

// BenchmarkValidateID exercises the path-injection guard that runs
// before every tool call that takes an ID argument. A regression here
// hits every dispatched call against the workspace and is the kind of
// thing that's easy to overlook because it does not fail any
// correctness test.
//
// The corpus mixes typical Clockify object-id shapes (24-char hex)
// with the rejection cases the function exists to catch. Both paths
// must stay fast.
//
// Run: go test -bench=BenchmarkValidateID -benchtime=10x ./internal/resolve
func BenchmarkValidateID(b *testing.B) {
	corpus := []string{
		"5e2c8f9b8c1f4a7d6e9b3c1a", // typical 24-char hex
		"5b1e2c0bb079873471b6f6e8",
		"deadbeefcafebabe12345678",
		"workspace-1",
		"u_abc123",
		"../../../etc/passwd",    // rejection: path traversal
		"foo?bar=1",              // rejection: contains ?
		"foo#frag",               // rejection: contains #
		"foo/bar",                // rejection: contains /
		strings.Repeat("a", 200), // rejection: too long
	}
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		_ = ValidateID(corpus[i%len(corpus)], "id")
		i++
	}
}

// BenchmarkResolveProjectIDCold500Names exercises the resolver path that has
// historically regressed in owner workspaces: strict-name-search still returns
// enough candidates to require pagination, and the exact match is late.
func BenchmarkResolveProjectIDCold500Names(b *testing.B) {
	const (
		workspaceID = "ws-bench"
		targetName  = "Target Project"
	)
	projects := make([]map[string]any, 500)
	for i := range projects {
		projects[i] = map[string]any{
			"id":   fmt.Sprintf("p-%03d", i),
			"name": fmt.Sprintf("Project %03d", i),
		}
	}
	projects[len(projects)-1] = map[string]any{"id": "p-target", "name": targetName}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/"+workspaceID+"/projects" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("strict-name-search"); got != "true" {
			b.Fatalf("strict-name-search=%q, want true", got)
		}
		if got := r.URL.Query().Get("name"); got != targetName {
			b.Fatalf("name=%q, want %q", got, targetName)
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page-size"))
		if page < 1 {
			page = 1
		}
		if pageSize <= 0 {
			pageSize = 200
		}
		start := (page - 1) * pageSize
		if start >= len(projects) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
			return
		}
		end := start + pageSize
		if end > len(projects) {
			end = len(projects)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(projects[start:end]); err != nil {
			b.Fatalf("encode projects: %v", err)
		}
	}))
	defer ts.Close()

	client := clockify.NewClient("bench-key", ts.URL, 5*time.Second, 0)
	defer client.Close()

	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		id, err := ResolveProjectID(ctx, client, workspaceID, targetName)
		if err != nil {
			b.Fatalf("ResolveProjectID: %v", err)
		}
		if id != "p-target" {
			b.Fatalf("ResolveProjectID=%q, want p-target", id)
		}
	}
}
