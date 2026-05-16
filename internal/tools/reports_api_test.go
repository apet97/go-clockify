package tools

import "testing"

// TestValidateReportDateRangeRejectsInverted locks P3-6: a backwards absolute
// report date range is rejected before it reaches Clockify; an absent or
// unparseable range is left for the API to handle (reads stay forgiving).
func TestValidateReportDateRangeRejectsInverted(t *testing.T) {
	if err := validateReportDateRange(map[string]any{
		"dateRangeStart": "2026-05-20",
		"dateRangeEnd":   "2026-05-01",
	}); err == nil {
		t.Fatal("expected an error for an inverted report date range")
	}
	if err := validateReportDateRange(map[string]any{
		"dateRangeStart": "2026-05-01T09:00:00Z",
		"dateRangeEnd":   "2026-05-20T17:00:00Z",
	}); err != nil {
		t.Fatalf("valid range rejected: %v", err)
	}
	if err := validateReportDateRange(map[string]any{}); err != nil {
		t.Fatalf("absent range must not error (left to the API): %v", err)
	}
	if err := validateReportDateRange(map[string]any{
		"dateRangeStart": "not-a-date",
		"dateRangeEnd":   "also-bad",
	}); err != nil {
		t.Fatalf("unparseable range must not error (left to the API): %v", err)
	}
}
