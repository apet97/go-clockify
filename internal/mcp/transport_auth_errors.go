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
		"auth_failure_category", authFailureCategory(err),
	}
	args = append(args, attrs...)
	slog.Warn("http_auth_failed", args...)
}

func authFailureCategory(err error) string {
	if err == nil {
		return "unknown"
	}
	reason := strings.ToLower(err.Error())
	switch {
	case strings.Contains(reason, "missing") && strings.Contains(reason, "authorization"):
		return "missing_credentials"
	case strings.Contains(reason, "bearer"):
		return "invalid_bearer"
	case strings.Contains(reason, "expired") || strings.Contains(reason, " exp "):
		return "expired_token"
	case strings.Contains(reason, "audience") || strings.Contains(reason, "resource"):
		return "audience_mismatch"
	case strings.Contains(reason, "issuer") || strings.Contains(reason, "jwks") ||
		strings.Contains(reason, "signature") || strings.Contains(reason, "signed"):
		return "token_verification"
	case strings.Contains(reason, "tenant"):
		return "tenant_claim"
	case strings.Contains(reason, "claim") || strings.Contains(reason, "subject"):
		return "claim_validation"
	case strings.Contains(reason, "mtls") || strings.Contains(reason, "certificate") ||
		strings.Contains(reason, "cert"):
		return "client_certificate"
	default:
		return "invalid_token"
	}
}
