package authn

import (
	"errors"
	"testing"
)

// TestFailureCategory pins the substring-matching contract behind the
// authn.FailureCategory log/metric label. The function is the
// observability primitive for every transport's auth failure path
// (mcp.writeAuthFailure, gRPC interceptor, streamable HTTP), so the
// label space must stay stable across releases — dashboards and alert
// rules filter on these strings. A drift in the case ordering or a new
// substring slipping into the wrong branch silently re-buckets failures
// and makes prior dashboards lie.
//
// Each case names the failure shape (left), the err.Error() literal
// FailureCategory will see (middle), and the bucket label that must
// come back (right). The literals deliberately mirror real authn error
// text from authn.go / oauth_resource.go so a copy-edit in those error
// messages would surface here as a regression rather than as a quiet
// label change.
func TestFailureCategory(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil_returns_unknown", nil, "unknown"},
		{"missing_authorization_header", errors.New("missing Authorization header"), "missing_credentials"},
		{"missing_authorization_metadata", errors.New("missing authorization metadata"), "missing_credentials"},
		{"missing_metadata", errors.New("missing metadata"), "missing_credentials"},
		{"invalid_bearer_format", errors.New("invalid bearer token format"), "invalid_bearer"},
		{"expired_token", errors.New("token is expired"), "expired_token"},
		{"jwt_exp_claim_failed", errors.New("token failed exp validation"), "expired_token"},
		{"audience_mismatch", errors.New("audience claim does not match expected"), "audience_mismatch"},
		{"resource_indicator_missing", errors.New("resource indicator does not match"), "audience_mismatch"},
		{"issuer_mismatch", errors.New("issuer claim does not match"), "token_verification"},
		{"jwks_lookup_failed", errors.New("jwks key not found"), "token_verification"},
		{"signature_invalid", errors.New("invalid signature on token"), "token_verification"},
		{"signed_with_unknown_key", errors.New("signed with unknown key"), "token_verification"},
		{"tenant_claim_missing", errors.New("tenant claim missing or empty"), "tenant_claim"},
		{"claim_validation_failed", errors.New("required claim missing"), "claim_validation"},
		{"subject_invalid", errors.New("subject claim malformed"), "claim_validation"},
		{"mtls_handshake_failed", errors.New("mtls handshake failed"), "client_certificate"},
		// "missing client certificate" lacks "authorization" so it falls
		// past the missing_credentials guard (which is missing+authorization)
		// and lands on the certificate branch — which is the operationally
		// useful bucket for an mTLS path with no client cert presented.
		{"client_certificate_missing", errors.New("missing client certificate"), "client_certificate"},
		{"cert_san_invalid", errors.New("cert URI SAN malformed"), "client_certificate"},
		{"raw_certificate_invalid", errors.New("certificate verification failed"), "client_certificate"},
		{"unknown_falls_through", errors.New("unexpected garbage"), "invalid_token"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FailureCategory(tc.err); got != tc.want {
				t.Fatalf("FailureCategory(%q) = %q, want %q", errMsg(tc.err), got, tc.want)
			}
		})
	}
}

// TestFailureCategory_CaseInsensitive confirms the lowercasing pre-pass.
// Real authn errors come from fmt.Errorf at multiple sites with mixed
// casing ("Authorization", "Bearer", "JWKS", "Audience"); the bucketing
// must not depend on which site produced the message.
func TestFailureCategory_CaseInsensitive(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{errors.New("MISSING AUTHORIZATION HEADER"), "missing_credentials"},
		{errors.New("Bearer token Malformed"), "invalid_bearer"},
		{errors.New("AUDIENCE Mismatch"), "audience_mismatch"},
		{errors.New("JWKS Lookup Failed"), "token_verification"},
		{errors.New("Tenant Claim Missing"), "tenant_claim"},
	}
	for _, tc := range cases {
		if got := FailureCategory(tc.err); got != tc.want {
			t.Fatalf("FailureCategory(%q) = %q, want %q", tc.err.Error(), got, tc.want)
		}
	}
}

func errMsg(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}
