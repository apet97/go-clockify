package tools

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

const resolveAggregateProjectBucketCount = 100

func BenchmarkResolveAggregateProjectBuckets(b *testing.B) {
	projects := make([]clockify.Project, 0, resolveAggregateProjectBucketCount)
	for i := range resolveAggregateProjectBucketCount {
		id := fmt.Sprintf("p%03d", i)
		projects = append(projects, clockify.Project{ID: id, Name: "Project " + id, ClientID: "c1", ClientName: "Client"})
	}

	var projectListRequests atomic.Int64
	client, cleanup := newTestClient(b, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/projects" && r.Method == http.MethodGet:
			projectListRequests.Add(1)
			respondJSON(b, w, projects)
		case strings.HasPrefix(r.URL.Path, "/workspaces/ws1/projects/") && r.Method == http.MethodGet:
			id := strings.TrimPrefix(r.URL.Path, "/workspaces/ws1/projects/")
			for _, project := range projects {
				if project.ID == id {
					respondJSON(b, w, project)
					return
				}
			}
			http.NotFound(w, r)
		default:
			b.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	b.ReportAllocs()
	for b.Loop() {
		result := &aggregateResult{ByProject: make(map[string]*projectBucket, len(projects))}
		for _, project := range projects {
			result.ByProject[project.ID] = &projectBucket{ID: project.ID, Name: "(no project)"}
		}
		svc.resolveAggregateProjectBuckets(context.Background(), "ws1", result)
		if len(result.ByProject) != len(projects) {
			b.Fatalf("ByProject size=%d, want %d", len(result.ByProject), len(projects))
		}
		if result.ByProject["p099"].Name != "Project p099" {
			b.Fatalf("project name = %q", result.ByProject["p099"].Name)
		}
	}
	if projectListRequests.Load() > int64(b.N) {
		b.Fatalf("project list requests=%d, want at most %d", projectListRequests.Load(), b.N)
	}
}

func BenchmarkAggregateEntriesRange_1000EntriesNoRawRetention(b *testing.B) {
	base := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	entries := seedEntries(base, 1000, 60)
	client, cleanup := newTestClient(b, newPaginatedHandler(b, chunkEntries(entries, reportPageSize)))
	defer cleanup()

	svc := New(client, "ws1")
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	b.ReportAllocs()
	for b.Loop() {
		agg, _, _, err := svc.aggregateEntriesRange(context.Background(), start, end, time.UTC, aggregateOptions{
			PageSize:       reportPageSize,
			IncludeEntries: false,
			SampleEntries:  10,
		})
		if err != nil {
			b.Fatalf("aggregate failed: %v", err)
		}
		if agg.EntriesCount != len(entries) {
			b.Fatalf("EntriesCount = %d, want %d", agg.EntriesCount, len(entries))
		}
		if got, want := agg.TotalSeconds, int64(len(entries)*60); got != want {
			b.Fatalf("TotalSeconds = %d, want %d", got, want)
		}
		if len(agg.Entries) != 0 {
			b.Fatalf("IncludeEntries=false retained %d raw entries", len(agg.Entries))
		}
	}
}
