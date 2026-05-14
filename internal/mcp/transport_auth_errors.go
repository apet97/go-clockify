//go:build legacy_platform

package mcp

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/apet97/go-clockify/internal/authn"
)

const genericAuthFailureDescription = "authentication failed"

func writeAuthFailure(w http.ResponseWriter, err error, expose bool) {
	desc := genericAuthFailureDescription
	if expose && err != nil {
		desc = err.Error()
	}
	w.Header().Set("WWW-Authenticate", authn.UnauthorizedHeaderValue("invalid_token", desc))
	writeJSONRPCErrorData(w, http.StatusUnauthorized, RPCCodeUnauthenticated, desc, map[string]any{
		"oauth_error": "invalid_token",
	})
}

func bearerValue(authHeader string) (string, bool) {
	authHeader = strings.TrimSpace(authHeader)
	scheme, token, ok := strings.Cut(authHeader, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
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
