package clockify

import (
	"encoding/json"
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
	if compact, ok := compactUpstreamErrorBody(e.Body); ok {
		return fmt.Sprintf("clockify %s failed: %s: %s", e.Method, e.Status, compact)
	}
	return fmt.Sprintf("clockify %s %s failed: %s: %s", e.Method, e.Path, e.Status, e.Body)
}

// Sanitized returns the error string without the upstream response body,
// suitable for hosted/shared deployments where the body might leak
// per-tenant information across tenants. Server-side logs still call
// Error() so operators can see the full diagnostic; the sanitised form
// is only what we hand to the MCP client.
func (e *APIError) Sanitized() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("clockify %s %s failed: %s", e.Method, e.Path, e.Status)
}

func trimBody(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 1000 {
		return s[:1000] + "..."
	}
	return s
}

func compactUpstreamErrorBody(body string) (string, bool) {
	var payload struct {
		Message string `json:"message"`
		Code    any    `json:"code"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return "", false
	}
	message := strings.TrimSpace(payload.Message)
	if message == "" {
		return "", false
	}
	if payload.Code == nil {
		return message, true
	}
	return fmt.Sprintf("%s (upstream_code=%v)", message, payload.Code), true
}
