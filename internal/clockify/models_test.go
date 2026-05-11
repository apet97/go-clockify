package clockify

import (
	"testing"
	"time"
)

// TestTimeEntry_DurationSeconds covers the five branches in
// TimeEntry.DurationSeconds: a closed interval that returns the wall
// duration, a running entry that derives the value from time.Now,
// the unparseable-start short-circuit, the end-before-start clamp,
// and a multi-day span (proves we never overflow the int64 result
// for realistic Clockify values).
func TestTimeEntry_DurationSeconds(t *testing.T) {
	t.Run("closed interval", func(t *testing.T) {
		entry := TimeEntry{TimeInterval: TimeInterval{
			Start: "2026-05-11T08:00:00Z",
			End:   "2026-05-11T09:30:00Z",
		}}
		got := entry.DurationSeconds()
		want := int64(5400)
		if got != want {
			t.Fatalf("closed interval: got %d, want %d", got, want)
		}
	})

	t.Run("running entry falls back to now", func(t *testing.T) {
		start := time.Now().UTC().Add(-15 * time.Minute)
		entry := TimeEntry{TimeInterval: TimeInterval{
			Start: start.Format(time.RFC3339),
		}}
		got := entry.DurationSeconds()
		// Allow a 5 s window on either side of the 900 s expectation;
		// CI clocks aren't atomic and we don't want a flaky test.
		if got < 895 || got > 905 {
			t.Fatalf("running entry: got %d seconds, want ~900 (±5)", got)
		}
	})

	t.Run("unparseable start returns zero", func(t *testing.T) {
		entry := TimeEntry{TimeInterval: TimeInterval{
			Start: "not-a-timestamp",
			End:   "2026-05-11T09:30:00Z",
		}}
		if got := entry.DurationSeconds(); got != 0 {
			t.Fatalf("unparseable start: got %d, want 0", got)
		}
	})

	t.Run("end before start clamps to zero", func(t *testing.T) {
		entry := TimeEntry{TimeInterval: TimeInterval{
			Start: "2026-05-11T09:00:00Z",
			End:   "2026-05-11T08:00:00Z",
		}}
		if got := entry.DurationSeconds(); got != 0 {
			t.Fatalf("end<start: got %d, want 0 (no negative durations)", got)
		}
	})

	t.Run("multi-day span", func(t *testing.T) {
		entry := TimeEntry{TimeInterval: TimeInterval{
			Start: "2026-05-09T00:00:00Z",
			End:   "2026-05-11T00:00:00Z",
		}}
		got := entry.DurationSeconds()
		want := int64(2 * 24 * 3600)
		if got != want {
			t.Fatalf("multi-day span: got %d, want %d", got, want)
		}
	})
}

// TestTimeEntry_IsRunning is the trivial companion: a missing End
// string is the Clockify wire signal for a running entry. Pin it so
// future refactors that flip the meaning (e.g. an explicit IsRunning
// JSON field) keep this contract visible.
func TestTimeEntry_IsRunning(t *testing.T) {
	cases := []struct {
		name string
		end  string
		want bool
	}{
		{"empty end is running", "", true},
		{"set end is closed", "2026-05-11T09:00:00Z", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := TimeEntry{TimeInterval: TimeInterval{End: tc.end}}
			if got := entry.IsRunning(); got != tc.want {
				t.Fatalf("IsRunning(end=%q): got %v, want %v", tc.end, got, tc.want)
			}
		})
	}
}
