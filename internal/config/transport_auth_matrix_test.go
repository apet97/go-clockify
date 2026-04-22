package config

import (
	"strings"
	"testing"
)

// TestTransportAuthMatrix exhaustively exercises every {transport x
// auth_mode} cell and asserts each one either loads cleanly or fails
// with a config-level error naming the mismatch. Before A5 the
// transport/auth surface relied on spot tests that let combinations
// like "http + mtls" or "stdio + static_bearer" drift between config
// intent and runtime behaviour. This test locks the matrix in as a
// contract so future config changes cannot silently widen or narrow
// the supported set.
func TestTransportAuthMatrix(t *testing.T) {
	type cell struct {
		transport string
		authMode  string
		// extra env beyond the minimum CLOCKIFY_API_KEY that this cell
		// requires to reach a successful Load (e.g. bearer token,
		// control-plane DSN, OIDC issuer).
		extra map[string]string
		// want: "ok" means Load succeeds; anything else is a substring
		// required in the returned error, keyed off the operator-facing
		// message so a regression in the error string surfaces as a
		// test failure.
		want string
	}

	bearer := "abcdef0123456789abcdef"
	cases := []cell{
		// --- stdio ---------------------------------------------------
		{"stdio", "", nil, "ok"},
		{"stdio", "static_bearer", nil, "MCP_AUTH_MODE is only valid for HTTP transports"},
		{"stdio", "oidc", nil, "MCP_AUTH_MODE is only valid for HTTP transports"},
		{"stdio", "forward_auth", nil, "MCP_AUTH_MODE is only valid for HTTP transports"},
		{"stdio", "mtls", nil, "MCP_AUTH_MODE is only valid for HTTP transports"},

		// --- legacy http --------------------------------------------
		{"http", "static_bearer", map[string]string{"MCP_BEARER_TOKEN": bearer}, "ok"},
		{"http", "oidc", nil, "ok"}, // OIDC issuer is only required for streamable_http in Load
		{"http", "forward_auth", nil, "ok"},
		{"http", "mtls", nil, "MCP_AUTH_MODE=mtls is not supported with MCP_TRANSPORT=http"},
		{"http", "invalid", nil, "invalid MCP_AUTH_MODE"},
		// legacy http + static_bearer without token is rejected.
		{"http", "static_bearer", nil, "MCP_BEARER_TOKEN is required for static bearer auth"},

		// --- streamable_http ----------------------------------------
		// streamable_http against a dev DSN (memory / file / bare path)
		// requires an explicit MCP_ALLOW_DEV_BACKEND=1 acknowledgement
		// from the operator. Without it, Load() fails closed so the
		// operator sees the mismatch on startup instead of at first
		// request. See docs/adr/0014-prod-fail-closed-defaults.md.
		{"streamable_http", "static_bearer", map[string]string{
			"MCP_BEARER_TOKEN":      bearer,
			"MCP_CONTROL_PLANE_DSN": "memory",
			"MCP_ALLOW_DEV_BACKEND": "1",
		}, "ok"},
		{"streamable_http", "oidc", map[string]string{
			"MCP_OIDC_ISSUER":       "https://issuer.example",
			"MCP_CONTROL_PLANE_DSN": "memory",
			"MCP_ALLOW_DEV_BACKEND": "1",
		}, "ok"},
		{"streamable_http", "oidc", map[string]string{
			"MCP_CONTROL_PLANE_DSN": "memory",
			"MCP_ALLOW_DEV_BACKEND": "1",
		}, "MCP_OIDC_ISSUER is required"},
		{"streamable_http", "forward_auth", map[string]string{
			"MCP_CONTROL_PLANE_DSN": "memory",
			"MCP_ALLOW_DEV_BACKEND": "1",
		}, "ok"},
		{"streamable_http", "mtls", map[string]string{
			"MCP_CONTROL_PLANE_DSN": "memory",
			"MCP_ALLOW_DEV_BACKEND": "1",
		}, "ok"},
		// Dev DSN without the ack flag — fails closed with a message
		// that names both escape hatches (flag or postgres://).
		{"streamable_http", "static_bearer", map[string]string{
			"MCP_BEARER_TOKEN":      bearer,
			"MCP_CONTROL_PLANE_DSN": "memory",
		}, "dev backend) is disallowed by default"},
		// Empty DSN (defaulted to "memory" by Load) with no ack flag —
		// same fail-closed path; proves the silent-memory-fallback is
		// gone.
		{"streamable_http", "static_bearer", map[string]string{
			"MCP_BEARER_TOKEN": bearer,
		}, "dev backend) is disallowed by default"},
		// Production DSN (postgres://) always passes without the flag.
		{"streamable_http", "static_bearer", map[string]string{
			"MCP_BEARER_TOKEN":      bearer,
			"MCP_CONTROL_PLANE_DSN": "postgres://user:pw@db.example:5432/mcp",
		}, "ok"},

		// --- grpc ---------------------------------------------------
		{"grpc", "static_bearer", map[string]string{"MCP_BEARER_TOKEN": bearer}, "ok"},
		{"grpc", "oidc", nil, "ok"},
		{"grpc", "forward_auth", nil, "ok"},
		{"grpc", "mtls", nil, "ok"},
	}

	for _, tc := range cases {
		name := tc.transport + "_" + tc.authMode
		if tc.authMode == "" {
			name = tc.transport + "_default"
		}
		t.Run(name, func(t *testing.T) {
			envs := map[string]string{"CLOCKIFY_API_KEY": "test-key"}
			envs["MCP_TRANSPORT"] = tc.transport
			if tc.authMode != "" {
				envs["MCP_AUTH_MODE"] = tc.authMode
			}
			for k, v := range tc.extra {
				envs[k] = v
			}
			// Clear any env inherited from other tests so each cell is
			// deterministic. Covers the leak between cases that share
			// the same process env.
			for _, k := range []string{
				"MCP_BEARER_TOKEN", "MCP_CONTROL_PLANE_DSN", "MCP_OIDC_ISSUER",
				"MCP_OIDC_AUDIENCE", "MCP_OIDC_JWKS_URL", "MCP_OIDC_JWKS_PATH",
				"MCP_RESOURCE_URI", "MCP_OIDC_VERIFY_CACHE_TTL",
				"MCP_ALLOW_DEV_BACKEND",
			} {
				if _, present := envs[k]; !present {
					t.Setenv(k, "")
				}
			}
			setEnvs(t, envs)

			_, err := Load()
			if tc.want == "ok" {
				if err != nil {
					t.Fatalf("%s + %s: expected OK, got error: %v",
						tc.transport, tc.authMode, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("%s + %s: expected error containing %q, got nil",
					tc.transport, tc.authMode, tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%s + %s: expected error containing %q, got %q",
					tc.transport, tc.authMode, tc.want, err.Error())
			}
		})
	}
}
