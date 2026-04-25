package mcp

import (
	"log/slog"
	"net/http"

	"github.com/apet97/go-clockify/internal/authn"
)

const genericAuthFailureDescription = "authentication failed"

func writeAuthFailure(w http.ResponseWriter, err error, expose bool) {
	desc := genericAuthFailureDescription
	if expose && err != nil {
		desc = err.Error()
	}
	authn.WriteUnauthorized(w, "invalid_token", desc)
}

func logHTTPAuthFailure(transport string, r *http.Request, err error, attrs ...any) {
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	args := []any{
		"transport", transport,
		"method", r.Method,
		"path", r.URL.Path,
		"status", http.StatusUnauthorized,
		"reason", "auth_failed",
		"auth_failed", reason,
		"auth_failure_category", authn.FailureCategory(err),
	}
	args = append(args, attrs...)
	slog.Warn("http_auth_failed", args...)
}
