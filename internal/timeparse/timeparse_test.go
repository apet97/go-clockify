package timeparse

import (
	"math"
	"testing"
	"time"
)

func TestParseDatetime(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("failed to load location: %v", err)
	}

	now := time.Now().In(loc)
	todayMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).UTC()
	yesterdayMidnight := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, loc).UTC()

	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(t *testing.T, got time.Time)
	}{
		{
			name:  "now",
			input: "now",
			check: func(t *testing.T, got time.Time) {
				diff := math.Abs(float64(time.Since(got)))
				if diff > float64(2*time.Second) {
					t.Errorf("expected within 2s of now, got diff=%v", time.Duration(diff))
				}
			},
		},
		{
			name:  "today",
			input: "today",
			check: func(t *testing.T, got time.Time) {
				if !got.Equal(todayMidnight) {
					t.Errorf("expected %v, got %v", todayMidnight, got)
				}
			},
		},
		{
			name:  "yesterday",
			input: "yesterday",
			check: func(t *testing.T, got time.Time) {
				if !got.Equal(yesterdayMidnight) {
					t.Errorf("expected %v, got %v", yesterdayMidnight, got)
				}
			},
		},
		{
			name:  "today with time",
			input: "today 14:30",
			check: func(t *testing.T, got time.Time) {
				want := time.Date(now.Year(), now.Month(), now.Day(), 14, 30, 0, 0, loc).UTC()
				if !got.Equal(want) {
					t.Errorf("expected %v, got %v", want, got)
				}
			},
		},
		{
			name:  "yesterday with time",
			input: "yesterday 09:00",
			check: func(t *testing.T, got time.Time) {
				want := time.Date(now.Year(), now.Month(), now.Day()-1, 9, 0, 0, 0, loc).UTC()
				if !got.Equal(want) {
					t.Errorf("expected %v, got %v", want, got)
				}
			},
		},
		{
			name:  "bare HH:MM",
			input: "14:30",
			check: func(t *testing.T, got time.Time) {
				want := time.Date(now.Year(), now.Month(), now.Day(), 14, 30, 0, 0, loc).UTC()
				if !got.Equal(want) {
					t.Errorf("expected %v, got %v", want, got)
				}
			},
		},
		{
			name:  "RFC3339",
			input: "2026-04-01T09:00:00Z",
			check: func(t *testing.T, got time.Time) {
				want := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
				if !got.Equal(want) {
					t.Errorf("expected %v, got %v", want, got)
				}
			},
		},
		{
			name:  "datetime T without seconds",
			input: "2026-04-01T14:30",
			check: func(t *testing.T, got time.Time) {
				want := time.Date(2026, 4, 1, 14, 30, 0, 0, loc).UTC()
				if !got.Equal(want) {
					t.Errorf("expected %v, got %v", want, got)
				}
			},
		},
		{
			name:  "datetime space without seconds",
			input: "2026-04-01 14:30",
			check: func(t *testing.T, got time.Time) {
				want := time.Date(2026, 4, 1, 14, 30, 0, 0, loc).UTC()
				if !got.Equal(want) {
					t.Errorf("expected %v, got %v", want, got)
				}
			},
		},
		{
			name:  "date only",
			input: "2026-04-01",
			check: func(t *testing.T, got time.Time) {
				want := time.Date(2026, 4, 1, 0, 0, 0, 0, loc).UTC()
				if !got.Equal(want) {
					t.Errorf("expected %v, got %v", want, got)
				}
			},
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "garbage",
			input:   "garbage",
			wantErr: true,
		},
		{
			name:  "case insensitive NOW",
			input: "NOW",
			check: func(t *testing.T, got time.Time) {
				diff := math.Abs(float64(time.Since(got)))
				if diff > float64(2*time.Second) {
					t.Errorf("expected within 2s of now, got diff=%v", time.Duration(diff))
				}
			},
		},
		{
			name:  "case insensitive Today",
			input: "Today",
			check: func(t *testing.T, got time.Time) {
				if !got.Equal(todayMidnight) {
					t.Errorf("expected %v, got %v", todayMidnight, got)
				}
			},
		},
		{
			name:  "case insensitive YESTERDAY",
			input: "YESTERDAY",
			check: func(t *testing.T, got time.Time) {
				if !got.Equal(yesterdayMidnight) {
					t.Errorf("expected %v, got %v", yesterdayMidnight, got)
				}
			},
		},
		{
			name:  "today with HH:MM:SS",
			input: "today 14:30:45",
			check: func(t *testing.T, got time.Time) {
				want := time.Date(now.Year(), now.Month(), now.Day(), 14, 30, 45, 0, loc).UTC()
				if !got.Equal(want) {
					t.Errorf("expected %v, got %v", want, got)
				}
			},
		},
		{
			name:  "datetime T with seconds",
			input: "2026-04-01T14:30:45",
			check: func(t *testing.T, got time.Time) {
				want := time.Date(2026, 4, 1, 14, 30, 45, 0, loc).UTC()
				if !got.Equal(want) {
					t.Errorf("expected %v, got %v", want, got)
				}
			},
		},
		{
			name:  "datetime space with seconds",
			input: "2026-04-01 14:30:45",
			check: func(t *testing.T, got time.Time) {
				want := time.Date(2026, 4, 1, 14, 30, 45, 0, loc).UTC()
				if !got.Equal(want) {
					t.Errorf("expected %v, got %v", want, got)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseDatetime(tc.input, loc)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got %v", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}
			if got.Location() != time.UTC {
				t.Errorf("expected UTC, got location %v", got.Location())
			}
			if tc.check != nil {
				tc.check(t, got)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{
			name:  "ISO PT1H30M",
			input: "PT1H30M",
			want:  1*time.Hour + 30*time.Minute,
		},
		{
			name:  "ISO PT45M",
			input: "PT45M",
			want:  45 * time.Minute,
		},
		{
			name:  "ISO PT2H",
			input: "PT2H",
			want:  2 * time.Hour,
		},
		{
			name:  "ISO PT1H30M15S",
			input: "PT1H30M15S",
			want:  1*time.Hour + 30*time.Minute + 15*time.Second,
		},
		{
			name:  "Go format 2h30m",
			input: "2h30m",
			want:  2*time.Hour + 30*time.Minute,
		},
		{
			name:  "Go format 45m",
			input: "45m",
			want:  45 * time.Minute,
		},
		{
			name:    "empty",
			input:   "",
			wantErr: true,
		},
		{
			name:    "garbage",
			input:   "garbage",
			wantErr: true,
		},
		{
			name:  "ISO lowercase pt1h",
			input: "pt1h",
			want:  1 * time.Hour,
		},
		{
			name:  "ISO seconds only",
			input: "PT30S",
			want:  30 * time.Second,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseDuration(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got %v", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("ParseDuration(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestFormatISO(t *testing.T) {
	input := time.Date(2026, 4, 1, 14, 30, 0, 0, time.UTC)
	got := FormatISO(input)
	want := "2026-04-01T14:30:00Z"
	if got != want {
		t.Errorf("FormatISO() = %q, want %q", got, want)
	}

	// Non-UTC input should still produce UTC output.
	loc, _ := time.LoadLocation("America/New_York")
	eastern := time.Date(2026, 4, 1, 10, 30, 0, 0, loc)
	got2 := FormatISO(eastern)
	if got2 != want {
		t.Errorf("FormatISO(eastern) = %q, want %q", got2, want)
	}
}

func TestParseTimeOfDay(t *testing.T) {
	tests := []struct {
		input               string
		wantH, wantM, wantS int
		wantErr             bool
	}{
		{"14:30", 14, 30, 0, false},
		{"14:30:45", 14, 30, 45, false},
		{"00:00", 0, 0, 0, false},
		{"23:59:59", 23, 59, 59, false},
		{"24:00", 0, 0, 0, true},
		{"12:60", 0, 0, 0, true},
		{"12:30:60", 0, 0, 0, true},
		{"abc", 0, 0, 0, true},
		{"12", 0, 0, 0, true},
		{"12:30:45:00", 0, 0, 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			h, m, s, err := parseTimeOfDay(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			if h != tc.wantH || m != tc.wantM || s != tc.wantS {
				t.Errorf("parseTimeOfDay(%q) = (%d,%d,%d), want (%d,%d,%d)",
					tc.input, h, m, s, tc.wantH, tc.wantM, tc.wantS)
			}
		})
	}
}

func TestParseISO8601Duration(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"PT1H", 1 * time.Hour, false},
		{"PT30M", 30 * time.Minute, false},
		{"PT1H30M", 1*time.Hour + 30*time.Minute, false},
		{"PT1H30M15S", 1*time.Hour + 30*time.Minute + 15*time.Second, false},
		{"pt2h", 2 * time.Hour, false},
		{"PT", 0, true},   // no components
		{"PT5", 0, true},  // trailing digits
		{"P1H", 0, true},  // missing T
		{"PTxH", 0, true}, // non-numeric
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseISO8601Duration(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %v", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("parseISO8601Duration(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
