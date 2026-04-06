package clockify

import (
	"fmt"
	"strings"
	"time"
)

type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Status     string
	Body       string
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Body == "" {
		return fmt.Sprintf("clockify %s %s failed: %s", e.Method, e.Path, e.Status)
	}
	return fmt.Sprintf("clockify %s %s failed: %s: %s", e.Method, e.Path, e.Status, e.Body)
}

func trimBody(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 1000 {
		return s[:1000] + "..."
	}
	return s
}
