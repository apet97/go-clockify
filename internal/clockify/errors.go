package clockify

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type APIError struct {
	Method      string
	Path        string
	StatusCode  int
	Status      string
	Body        string
	RetryAfter  time.Duration
	Translation *ErrorTranslation
}

type ErrorTranslation struct {
	Message     string   `json:"message"`
	Remediation string   `json:"remediation,omitempty"`
	Refs        []string `json:"refs,omitempty"`
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

func (e *APIError) ErrorTranslation() any {
	if e == nil {
		return ErrorTranslation{}
	}
	if e.Translation != nil {
		return *e.Translation
	}
	return TranslateAPIError(e.StatusCode, e.Body)
}

func TranslateAPIError(statusCode int, body string) ErrorTranslation {
	message := "Clockify request failed."
	if compact, ok := compactUpstreamErrorBody(body); ok {
		message = compact
	}
	lower := strings.ToLower(message + " " + body)
	switch {
	case statusCode == 401 || strings.Contains(lower, "unauthorized"):
		return ErrorTranslation{Message: message, Remediation: "Check that the Clockify API key is valid and has access to the selected workspace.", Refs: []string{"CLOCKIFY_API_KEY", "CLOCKIFY_WORKSPACE_ID"}}
	case statusCode == 403 || strings.Contains(lower, "permission") || strings.Contains(lower, "forbidden"):
		return ErrorTranslation{Message: message, Remediation: "Use an account with the required Clockify role or ask a workspace admin to grant the needed permission.", Refs: []string{"Clockify workspace roles"}}
	case statusCode == 404:
		return ErrorTranslation{Message: message, Remediation: "Confirm the workspace/entity ID and whether the entity was archived or deleted.", Refs: []string{"clockify_tools_guide"}}
	case statusCode == 429 || strings.Contains(lower, "rate"):
		return ErrorTranslation{Message: message, Remediation: "Retry after the upstream rate limit cools down; narrow broad report/list requests when possible.", Refs: []string{"Clockify API rate limits"}}
	case strings.Contains(lower, "plan") || strings.Contains(lower, "feature") || strings.Contains(lower, "subscription"):
		return ErrorTranslation{Message: message, Remediation: "Check whether the workspace plan enables this Clockify feature.", Refs: []string{"clockify_workspace_governance"}}
	case strings.Contains(lower, "locked"):
		return ErrorTranslation{Message: message, Remediation: "The entry or period is locked. Review workspace lock settings or request an unlock from an admin.", Refs: []string{"clockify_workspace_governance", "clockify_audit_entries"}}
	default:
		return ErrorTranslation{Message: message, Remediation: "Inspect the tool arguments and Clockify permissions, then retry with a narrower request if this was a report/list call."}
	}
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
