package tools

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

// captureNotifier records every notification the tools layer emits during a
// test so assertions can count + inspect them.
type captureNotifier struct {
	calls []struct {
		Method string
		Params map[string]any
	}
}

func (c *captureNotifier) Notify(method string, params any) error {
	p, _ := params.(map[string]any)
	c.calls = append(c.calls, struct {
		Method string
		Params map[string]any
	}{method, p})
	return nil
}

func TestReportsEmitsProgressWhenTokenPresent(t *testing.T) {
	// Three pages of entries, each 50 long (== default page-size floor) so
	// aggregateEntriesRange walks all three before seeing a short tail.
	page1 := makeEntries(50, "2026-04-06")
	page2 := makeEntries(50, "2026-04-07")
	page3 := makeEntries(3, "2026-04-08") // short tail stops pagination

	client, cleanup := newTestClient(t, newPaginatedHandler(t, [][]clockify.TimeEntry{page1, page2, page3}))
	defer cleanup()

	notifier := &captureNotifier{}
	svc := &Service{Client: client, WorkspaceID: "ws1", Notifier: notifier}

	ctx := mcp.WithProgressToken(context.Background(), "tok-1")
	_, _, _, err := svc.aggregateEntriesRange(ctx,
		mustDate("2026-04-06"), mustDate("2026-04-13"),
		time.UTC, aggregateOptions{PageSize: 50, IncludeEntries: false, MaxEntries: 0})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}

	if len(notifier.calls) != 3 {
		t.Fatalf("expected 3 progress notifications, got %d", len(notifier.calls))
	}
	for i, call := range notifier.calls {
		if call.Method != "notifications/progress" {
			t.Fatalf("call %d method: %q", i, call.Method)
		}
		if call.Params["progressToken"] != "tok-1" {
			t.Fatalf("call %d token: %+v", i, call.Params)
		}
		got, ok := call.Params["progress"].(float64)
		if !ok || got != float64(i+1) {
			t.Fatalf("call %d progress: %+v", i, call.Params["progress"])
		}
		// Total must be absent (indeterminate).
		if _, present := call.Params["total"]; present {
			t.Fatalf("call %d should not report total mid-walk: %+v", i, call.Params)
		}
	}
}

func TestReportsNoProgressWithoutToken(t *testing.T) {
	page1 := makeEntries(3, "2026-04-06")
	client, cleanup := newTestClient(t, newPaginatedHandler(t, [][]clockify.TimeEntry{page1}))
	defer cleanup()

	notifier := &captureNotifier{}
	svc := &Service{Client: client, WorkspaceID: "ws1", Notifier: notifier}

	_, _, _, err := svc.aggregateEntriesRange(context.Background(),
		mustDate("2026-04-06"), mustDate("2026-04-13"),
		time.UTC, aggregateOptions{PageSize: 50, IncludeEntries: false, MaxEntries: 0})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(notifier.calls) != 0 {
		t.Fatalf("expected zero notifications without token, got %d", len(notifier.calls))
	}
}

func TestReportsNoProgressWithoutNotifier(t *testing.T) {
	page1 := makeEntries(3, "2026-04-06")
	client, cleanup := newTestClient(t, newPaginatedHandler(t, [][]clockify.TimeEntry{page1}))
	defer cleanup()

	// Service with nil Notifier — EmitProgress must no-op even with token.
	svc := &Service{Client: client, WorkspaceID: "ws1"}
	ctx := mcp.WithProgressToken(context.Background(), "tok-2")
	_, _, _, err := svc.aggregateEntriesRange(ctx,
		mustDate("2026-04-06"), mustDate("2026-04-13"),
		time.UTC, aggregateOptions{PageSize: 50, IncludeEntries: false, MaxEntries: 0})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
}

func makeEntries(n int, day string) []clockify.TimeEntry {
	entries := make([]clockify.TimeEntry, 0, n)
	start := day + "T09:00:00Z"
	end := day + "T10:00:00Z"
	for i := 0; i < n; i++ {
		entries = append(entries, clockify.TimeEntry{
			ID:          day + "-e" + itoaPad(i),
			Description: "test",
			ProjectID:   "p1",
			ProjectName: "Alpha",
			TimeInterval: clockify.TimeInterval{
				Start: start,
				End:   end,
			},
		})
	}
	return entries
}

func itoaPad(i int) string {
	return string(rune('0'+(i/10))) + string(rune('0'+(i%10)))
}

func mustDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// failNotifier is an mcp.Notifier whose Notify always fails, used to prove
// the tools layer logs (rather than silently drops) delivery errors.
type failNotifier struct{}

func (failNotifier) Notify(string, any) error { return errors.New("transport closed") }

// recordHandler is a slog.Handler that records every emitted record so a
// test can assert a particular warning was logged.
type recordHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}

func (h *recordHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordHandler) WithGroup(string) slog.Handler      { return h }

func (h *recordHandler) has(msg string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, r := range h.records {
		if r.Message == msg {
			return true
		}
	}
	return false
}

// TestEmitProgressLogsNotifyFailure proves a failed progress notification is
// logged rather than silently discarded.
func TestEmitProgressLogsNotifyFailure(t *testing.T) {
	h := &recordHandler{}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	defer slog.SetDefault(prev)

	svc := &Service{Notifier: failNotifier{}}
	ctx := mcp.WithProgressToken(context.Background(), "tok-fail")
	svc.EmitProgress(ctx, 1, -1, "")

	if !h.has("progress_notify_failed") {
		t.Fatal("expected a progress_notify_failed warning to be logged")
	}
}
