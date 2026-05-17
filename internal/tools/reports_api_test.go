package tools

import (
	"strings"
	"testing"
	"time"
)

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

func TestBuildReportsAPIBody_CoercesPlainDate(t *testing.T) {
	body, err := buildReportsAPIBody(map[string]any{
		"date_range_start": "2026-05-01",
		"date_range_end":   "2026-05-17",
	}, "", false, time.UTC)
	if err != nil {
		t.Fatalf("buildReportsAPIBody: %v", err)
	}
	if body["dateRangeStart"] != "2026-05-01T00:00:00.000" || body["dateRangeEnd"] != "2026-05-17T23:59:59.999" {
		t.Fatalf("plain dates not coerced: %#v", body)
	}
}

func TestBuildReportsAPIBody_RejectsGarbageDate(t *testing.T) {
	_, err := buildReportsAPIBody(map[string]any{
		"date_range_start": "not-a-date",
		"date_range_end":   "2026-05-17",
	}, "", false, time.UTC)
	if err == nil || !strings.Contains(err.Error(), "could not parse date") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestValidateWeeklyDateRange_Message(t *testing.T) {
	err := validateWeeklyDateRange(map[string]any{
		"dateRangeStart": "2026-05-01T00:00:00.000",
		"dateRangeEnd":   "2026-05-09T00:00:00.000",
	}, time.UTC)
	if err == nil || !strings.Contains(err.Error(), "7 days") || !strings.Contains(err.Error(), "omit `end`") {
		t.Fatalf("expected clearer weekly guidance, got %v", err)
	}
}

func TestValidateWeeklyDateRange_ExactWeekPasses(t *testing.T) {
	if err := validateWeeklyDateRange(map[string]any{
		"dateRangeStart": "2026-05-01T00:00:00.000",
		"dateRangeEnd":   "2026-05-08T00:00:00.000",
	}, time.UTC); err != nil {
		t.Fatalf("exact week rejected: %v", err)
	}
}
