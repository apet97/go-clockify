package tools

import (
	"fmt"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/timeparse"
)

func loadLocation(name string, defaultTZ *time.Location) (*time.Location, error) {
	if strings.TrimSpace(name) == "" {
		if defaultTZ != nil {
			return defaultTZ, nil
		}
		return time.Now().Location(), nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %q: %w", name, err)
	}
	return loc, nil
}

func (s *Service) locationFromArgs(args map[string]any) (*time.Location, error) {
	return loadLocation(stringArg(args, "timezone"), s.DefaultTimezone)
}

func parseFlexibleDateTime(raw string, loc *time.Location) (time.Time, error) {
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts.In(loc), nil
	}
	// A bare YYYY-MM-DD is a calendar date, not an instant; anchor it at
	// 00:00:00Z so callers that reformat as UTC keep the same calendar date
	// instead of shifting it back by the local UTC offset.
	if d, err := time.ParseInLocation("2006-01-02", raw, time.UTC); err == nil {
		return d, nil
	}
	return time.Time{}, fmt.Errorf("expected RFC3339 or YYYY-MM-DD date, got %q", raw)
}

func parseRangeInLocation(args map[string]any, loc *time.Location) (time.Time, time.Time, error) {
	if loc == nil {
		loc = time.UTC
	}
	startRaw := stringArg(args, "start")
	endRaw := stringArg(args, "end")
	if startRaw == "" || endRaw == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("start and end are required")
	}
	start, err := timeparse.ParseDatetime(startRaw, loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("could not parse date %q for start — use YYYY-MM-DD or RFC3339", startRaw)
	}
	end, err := timeparse.ParseDatetime(endRaw, loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("could not parse date %q for end — use YYYY-MM-DD or RFC3339", endRaw)
	}
	if !end.After(start) && isBareDateString(endRaw) {
		end = end.AddDate(0, 0, 1)
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("end must be after start")
	}
	return start.UTC(), end.UTC(), nil
}

func isBareDateString(raw string) bool {
	raw = strings.TrimSpace(raw)
	if len(raw) != len("2006-01-02") {
		return false
	}
	_, err := time.Parse("2006-01-02", raw)
	return err == nil
}

// entryRangeQuery builds the base date-range query for time-entry reports.
// Pagination params are set by the paginator in aggregateEntriesRange; this
// helper intentionally does NOT set page or page-size.
func entryRangeQuery(start, end time.Time) map[string]string {
	return map[string]string{
		"start": start.UTC().Format(time.RFC3339),
		"end":   end.UTC().Format(time.RFC3339),
	}
}
