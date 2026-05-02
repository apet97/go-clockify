package authn

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ProtectedResourceMetadata is the body served at
// /.well-known/oauth-protected-resource per the MCP OAuth 2.1 profile
// (RFC 9728). Clients fetch this document unauthenticated to discover
// which authorization server issues tokens for the resource and which
// bearer methods the resource accepts.
type ProtectedResourceMetadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers,omitempty"`
	BearerMethodsSupported []string `json:"bearer_methods_supported,omitempty"`
	ScopesSupported        []string `json:"scopes_supported,omitempty"`
	ResourceName           string   `json:"resource_name,omitempty"`
}

// ProtectedResourceHandler returns an http.Handler that serves the
// metadata document for an OIDC-protected MCP resource. The endpoint
// MUST be unauthenticated per RFC 9728 §3.
//
// Returns nil when the supplied config has no resource URI configured —
// callers should not mount the endpoint in that case.
func ProtectedResourceHandler(cfg Config) http.Handler {
	if cfg.OIDCResourceURI == "" {
		return nil
	}
	doc := ProtectedResourceMetadata{
		Resource:               cfg.OIDCResourceURI,
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "Clockify Go MCP Server",
	}
	if cfg.OIDCIssuer != "" {
		doc.AuthorizationServers = []string{cfg.OIDCIssuer}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_ = json.NewEncoder(w).Encode(doc)
		}
	})
}

// WriteUnauthorized emits an HTTP 401 with a WWW-Authenticate header in
// RFC 6750 §3 form. errCode and errDesc become the `error` and
// `error_description` parameters; pass empty strings to omit them. The
// realm is fixed to "clockify-mcp" to match the metadata document.
func WriteUnauthorized(w http.ResponseWriter, errCode, errDesc string) {
	parts := []string{`Bearer realm="clockify-mcp"`}
	if errCode != "" {
		parts = append(parts, fmt.Sprintf(`error=%q`, errCode))
	}
	if errDesc != "" {
		parts = append(parts, fmt.Sprintf(`error_description=%q`, sanitizeHeaderValue(errDesc)))
	}
	w.Header().Set("WWW-Authenticate", strings.Join(parts, ", "))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	body := map[string]string{"error": "unauthorized"}
	if errCode != "" {
		body["error"] = errCode
	}
	if errDesc != "" {
		body["error_description"] = errDesc
	}
	_ = json.NewEncoder(w).Encode(body)
}

// sanitizeHeaderValue strips characters that would break the header
// quoting rules in RFC 7235. Newlines, double-quotes, and backslashes
// are replaced with spaces / removed.
func sanitizeHeaderValue(s string) string {
	r := strings.NewReplacer(
		"\n", " ",
		"\r", " ",
		`"`, "'",
		`\`, "/",
	)
	return r.Replace(s)
}
