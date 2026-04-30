package timeparse

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseDatetime parses a natural-language or structured datetime string and
// returns the result in UTC.  The provided location is used when the input
// has no explicit timezone (e.g. "today", "14:30", "2006-01-02 15:04").
//
// Resolution chain (first match wins):
//  1. ""            -> error
//  2. "now"         -> time.Now().UTC()
//  3. "today"       -> midnight today in loc, converted to UTC
//  4. "yesterday"   -> midnight yesterday in loc, converted to UTC
//  5. "today HH:MM[:SS]"     -> today at that time in loc, to UTC
//  6. "yesterday HH:MM[:SS]" -> yesterday at that time in loc, to UTC
//  7. "HH:MM"       -> today at that time in loc, to UTC
//  8. RFC3339        -> parsed, to UTC
//  9. "2006-01-02T15:04"     -> in loc, to UTC
//
// 10. "2006-01-02 15:04"     -> in loc, to UTC
// 11. "2006-01-02T15:04:05"  -> in loc, to UTC
// 12. "2006-01-02 15:04:05"  -> in loc, to UTC
// 13. "2006-01-02"           -> midnight in loc, to UTC
// 14. error
func ParseDatetime(s string, loc *time.Location) (time.Time, error) {
	// 1. empty
	if s == "" {
		return time.Time{}, fmt.Errorf("empty datetime string")
	}

	lower := strings.ToLower(s)

	// 2. now
	if strings.EqualFold(s, "now") {
		return time.Now().UTC(), nil
	}

	now := time.Now().In(loc)
	todayMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	yesterdayMidnight := todayMidnight.AddDate(0, 0, -1)

	// 3. today (exact)
	if strings.EqualFold(s, "today") {
		return todayMidnight.UTC(), nil
	}

	// 4. yesterday (exact)
	if strings.EqualFold(s, "yesterday") {
		return yesterdayMidnight.UTC(), nil
	}

	// 5. today HH:MM[:SS]
	if strings.HasPrefix(lower, "today ") {
		timeStr := strings.TrimSpace(s[len("today "):])
		h, m, sec, err := parseTimeOfDay(timeStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid time in %q: %w", s, err)
		}
		t := time.Date(now.Year(), now.Month(), now.Day(), h, m, sec, 0, loc)
		return t.UTC(), nil
	}

	// 6. yesterday HH:MM[:SS]
	if strings.HasPrefix(lower, "yesterday ") {
		timeStr := strings.TrimSpace(s[len("yesterday "):])
		h, m, sec, err := parseTimeOfDay(timeStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid time in %q: %w", s, err)
		}
		y := todayMidnight.AddDate(0, 0, -1)
		t := time.Date(y.Year(), y.Month(), y.Day(), h, m, sec, 0, loc)
		return t.UTC(), nil
	}

	// 7. bare HH:MM (exactly 5 chars: digit digit colon digit digit)
	if len(s) == 5 && isDigit(s[0]) && isDigit(s[1]) && s[2] == ':' && isDigit(s[3]) && isDigit(s[4]) {
		h, m, _, err := parseTimeOfDay(s)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid time %q: %w", s, err)
		}
		t := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, loc)
		return t.UTC(), nil
	}

	// 8. date-only fast path: "2006-01-02" is the only supported format of
	// length 10.  RFC3339 needs ≥20 chars; all datetime-with-time layouts
	// need ≥16.  Jumping straight to the date parse avoids one RFC3339 and
	// four layout parse attempts — each failure allocates a *time.ParseError.
	if len(s) == 10 {
		if t, err := time.ParseInLocation("2006-01-02", s, loc); err == nil {
			return t.UTC(), nil
		}
		return time.Time{}, fmt.Errorf("unrecognized datetime format: %q", s)
	}

	// 9. RFC3339
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}

	// 10-13. datetime layouts without timezone
	layouts := []string{
		"2006-01-02T15:04",    // 10
		"2006-01-02 15:04",    // 11
		"2006-01-02T15:04:05", // 12
		"2006-01-02 15:04:05", // 13
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t.UTC(), nil
		}
	}

	// 14. give up
	return time.Time{}, fmt.Errorf("unrecognized datetime format: %q", s)
}

// ParseDuration parses a duration string in either ISO 8601 (PT...) format
// or Go duration format (e.g. "2h30m", "45m").
func ParseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	// ISO 8601 duration
	if strings.HasPrefix(strings.ToUpper(s), "PT") {
		return parseISO8601Duration(s)
	}

	// Go stdlib duration
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("unrecognized duration format: %q", s)
	}
	return d, nil
}

// FormatISO formats a time as an RFC3339 / ISO 8601 string in UTC.
func FormatISO(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// parseTimeOfDay parses "HH:MM" or "HH:MM:SS" into hour, minute, second.
func parseTimeOfDay(s string) (int, int, int, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, 0, 0, fmt.Errorf("expected HH:MM or HH:MM:SS, got %q", s)
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid hour in %q: %w", s, err)
	}
	min, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid minute in %q: %w", s, err)
	}

	sec := 0
	if len(parts) == 3 {
		sec, err = strconv.Atoi(parts[2])
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid second in %q: %w", s, err)
		}
	}

	if hour < 0 || hour > 23 {
		return 0, 0, 0, fmt.Errorf("hour %d out of range 0-23", hour)
	}
	if min < 0 || min > 59 {
		return 0, 0, 0, fmt.Errorf("minute %d out of range 0-59", min)
	}
	if sec < 0 || sec > 59 {
		return 0, 0, 0, fmt.Errorf("second %d out of range 0-59", sec)
	}

	return hour, min, sec, nil
}

// parseISO8601Duration parses a "PT..." duration string such as "PT1H30M",
// "PT45M", "PT2H", or "PT1H30M15S".
func parseISO8601Duration(s string) (time.Duration, error) {
	upper := strings.ToUpper(s)
	if len(upper) < 3 || upper[:2] != "PT" {
		return 0, fmt.Errorf("ISO 8601 duration must start with PT: %q", s)
	}

	rest := upper[2:]
	var total time.Duration
	var numBuf strings.Builder

	for i := 0; i < len(rest); i++ {
		ch := rest[i]
		switch {
		case ch >= '0' && ch <= '9':
			numBuf.WriteByte(ch)
		case ch == 'H':
			n, err := strconv.Atoi(numBuf.String())
			if err != nil || numBuf.Len() == 0 {
				return 0, fmt.Errorf("invalid hours in duration %q", s)
			}
			total += time.Duration(n) * time.Hour
			numBuf.Reset()
		case ch == 'M':
			n, err := strconv.Atoi(numBuf.String())
			if err != nil || numBuf.Len() == 0 {
				return 0, fmt.Errorf("invalid minutes in duration %q", s)
			}
			total += time.Duration(n) * time.Minute
			numBuf.Reset()
		case ch == 'S':
			n, err := strconv.Atoi(numBuf.String())
			if err != nil || numBuf.Len() == 0 {
				return 0, fmt.Errorf("invalid seconds in duration %q", s)
			}
			total += time.Duration(n) * time.Second
			numBuf.Reset()
		default:
			return 0, fmt.Errorf("unexpected character %q in duration %q", string(ch), s)
		}
	}

	if numBuf.Len() > 0 {
		return 0, fmt.Errorf("trailing digits without unit in duration %q", s)
	}

	if total == 0 {
		return 0, fmt.Errorf("zero or empty duration %q", s)
	}

	return total, nil
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
