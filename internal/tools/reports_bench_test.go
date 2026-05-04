package tools

import (
	"context"
	"testing"
	"time"
)

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
