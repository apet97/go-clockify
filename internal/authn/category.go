package authn

import "strings"

// FailureCategory classifies an authentication error into a coarse-grained
// label for logs/metrics. It must never echo the raw error text to clients —
// callers strip details before responding (e.g. mcp.writeAuthFailure or the
// gRPC interceptor) and use this only for server-side observability.
//
// The categorisation is intentionally pattern-matched against lowercased
// substrings of err.Error() rather than typed sentinels because authn errors
// are constructed via fmt.Errorf at multiple sites (oidc, mtls, forward_auth,
// static_bearer). Adding a new sentinel everywhere would be churn for a
// helper whose only consumer is structured logging.
func FailureCategory(err error) string {
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
	case strings.Contains(reason, "metadata"):
		return "missing_credentials"
	default:
		return "invalid_token"
	}
}
