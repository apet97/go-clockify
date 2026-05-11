package config

import (
	"fmt"
	"maps"
	"os"
	"sort"
	"strings"
)

// Profile is a named bundle of env-var defaults for a supported deployment
// shape. Applying a profile sets each of its Env keys only when the key is
// currently unset in the process environment; explicit values (from shell,
// container env, systemd EnvironmentFile, etc.) always win. Five canonical
// profile names are registered: local-stdio, single-tenant-http,
// shared-service, private-network-grpc, prod-postgres (the last is a
// shared-service alias with ENVIRONMENT=prod). docs/deploy/ carries a note
// per profile shape; prod-postgres is documented inside the shared-service
// note, and docs/deploy/profile-self-hosted.md is a legacy-shape upgrade
// pointer without a corresponding registered profile.
type Profile struct {
	// Name is the stable identifier passed via --profile=<name> or
	// MCP_PROFILE=<name>. Hyphenated for CLI ergonomics.
	Name string

	// Summary is the one-line description shown in --help and by
	// the doctor subcommand. Keep under 80 characters.
	Summary string

	// Env is the map of env-var keys to the profile's default value.
	// Only unset keys receive a default; an operator who sets any of
	// these explicitly gets their value through. Keys MUST appear in
	// AllSpecs() — an un-specced key here trips the
	// TestProfile_KeysAreSpecced invariant.
	Env map[string]string
}

// allProfilesSlice is the declaration-order list of canonical profiles.
// Kept private so AllProfiles() can return a defensive copy.
var allProfilesSlice = []Profile{
	{
		Name:    "local-stdio",
		Summary: "Single-user local stdio server — no control plane, no auth, time_tracking_safe policy",
		Env: map[string]string{
			"MCP_TRANSPORT":        "stdio",
			"CLOCKIFY_POLICY":      "time_tracking_safe",
			"MCP_AUDIT_DURABILITY": "best_effort",
		},
	},
	{
		Name:    "single-tenant-http",
		Summary: "Single-tenant streamable HTTP with file-backed control plane and static bearer auth",
		Env: map[string]string{
			"MCP_TRANSPORT":          "streamable_http",
			"MCP_AUTH_MODE":          "static_bearer",
			"MCP_CONTROL_PLANE_DSN":  "file:///var/lib/clockify-mcp/cp.json",
			"MCP_ALLOW_DEV_BACKEND":  "1",
			"MCP_AUDIT_DURABILITY":   "best_effort",
			"MCP_HTTP_LEGACY_POLICY": "deny",
			// Small-team HTTP still sends tool errors over an MCP wire
			// to remote clients. Keep Clockify response bodies in
			// server-side logs, not in client-visible error envelopes.
			"CLOCKIFY_SANITIZE_UPSTREAM_ERRORS": "1",
			// The profile is API-key backed, so inline control-plane
			// credentials are unnecessary risk. Operators can override
			// explicitly during migration.
			"MCP_DISABLE_INLINE_SECRETS": "1",
			"CLOCKIFY_POLICY":            "time_tracking_safe",
		},
	},
	{
		Name:    "shared-service",
		Summary: "Multi-tenant streamable HTTP with postgres control plane, OIDC strict, tenant-claim required, fail-closed audit",
		Env: map[string]string{
			"MCP_TRANSPORT":          "streamable_http",
			"MCP_AUTH_MODE":          "oidc",
			"MCP_AUDIT_DURABILITY":   "fail_closed",
			"MCP_HTTP_LEGACY_POLICY": "deny",
			// Hosted-service strict gates. Each of these defaults
			// could be set via env separately, but a multi-tenant
			// hosted deployment that turns any of them off is doing
			// something dangerous (see ADR-0015 §"hosted strict gates"
			// for the rationale matrix). Bundling them in the profile
			// makes the strict posture the default and forces an
			// explicit operator opt-out.
			//
			// MCP_OIDC_STRICT=1 — reject tokens missing exp; require
			//   audience or resource URI binding (Config.Load gate).
			// MCP_OIDC_REQUIRE_KID=1 — reject kid-less JWTs instead
			//   of using the single-key JWKS compatibility fallback.
			// MCP_REQUIRE_TENANT_CLAIM=1 — reject OIDC tokens without
			//   a tenant claim instead of falling back to the shared
			//   default tenant (catastrophic in multi-tenant mode).
			// MCP_REQUIRE_FORWARD_TENANT_CLAIM=1 — same fail-closed
			//   posture if an operator overrides the hosted profile to
			//   use forward_auth behind a reverse proxy.
			// MCP_DISABLE_INLINE_SECRETS=1 — reject credential refs
			//   with backend=inline. Inline secrets sit in the
			//   control-plane DB and survive operator forgetfulness;
			//   external vault references rotate on revoke.
			// CLOCKIFY_POLICY=time_tracking_safe — AI-facing untrusted
			//   default. safe_core / standard / full remain available
			//   for trusted-team deployments via explicit override.
			// MCP_HTTP_RATELIMIT_* — process-local admission limits for
			//   pre-auth HTTP probes, authenticated request floods, and
			//   SSE stream holds. Cross-replica hosted quotas still live
			//   at the gateway/load-balancer layer.
			"MCP_OIDC_STRICT":                    "1",
			"MCP_OIDC_REQUIRE_KID":               "1",
			"MCP_REQUIRE_TENANT_CLAIM":           "1",
			"MCP_REQUIRE_FORWARD_TENANT_CLAIM":   "1",
			"MCP_DISABLE_INLINE_SECRETS":         "1",
			"MCP_HTTP_RATELIMIT_PER_IP":          "600",
			"MCP_HTTP_RATELIMIT_PER_PRINCIPAL":   "300",
			"MCP_HTTP_RATELIMIT_GET_PER_SESSION": "4",
			"CLOCKIFY_POLICY":                    "time_tracking_safe",
			// MCP_TENANT_POLICY_CEILING=time_tracking_safe — hosted
			// multi-tenant ceiling. Tenant records may NARROW their
			// PolicyMode below this but cannot BROADEN past it, even
			// if the Postgres row is corrupted or an operator pushes
			// PolicyMode=standard for a single tenant. See ADR 0021.
			"MCP_TENANT_POLICY_CEILING": "time_tracking_safe",
		},
	},
	{
		Name:    "private-network-grpc",
		Summary: "gRPC transport for private-network callers (requires -tags=grpc build); mTLS by default",
		Env: map[string]string{
			"MCP_TRANSPORT":           "grpc",
			"MCP_AUTH_MODE":           "mtls",
			"MCP_AUDIT_DURABILITY":    "fail_closed",
			"MCP_REQUIRE_MTLS_TENANT": "1",
		},
	},
	{
		Name:    "prod-postgres",
		Summary: "Shared-service strict + ENVIRONMENT=prod — postgres DSN required, fail-closed everywhere",
		Env: map[string]string{
			"MCP_TRANSPORT":                      "streamable_http",
			"MCP_AUTH_MODE":                      "oidc",
			"MCP_AUDIT_DURABILITY":               "fail_closed",
			"MCP_HTTP_LEGACY_POLICY":             "deny",
			"ENVIRONMENT":                        "prod",
			"MCP_OIDC_STRICT":                    "1",
			"MCP_OIDC_REQUIRE_KID":               "1",
			"MCP_REQUIRE_TENANT_CLAIM":           "1",
			"MCP_REQUIRE_FORWARD_TENANT_CLAIM":   "1",
			"MCP_DISABLE_INLINE_SECRETS":         "1",
			"MCP_HTTP_RATELIMIT_PER_IP":          "600",
			"MCP_HTTP_RATELIMIT_PER_PRINCIPAL":   "300",
			"MCP_HTTP_RATELIMIT_GET_PER_SESSION": "4",
			"CLOCKIFY_POLICY":                    "time_tracking_safe",
			// Hosted multi-tenant ceiling — see ADR 0021 and the
			// shared-service profile above. prod-postgres inherits
			// the same default because it is the strict variant of
			// shared-service.
			"MCP_TENANT_POLICY_CEILING": "time_tracking_safe",
		},
	},
}

// AllProfiles returns a defensive copy of the profile registry in
// declaration order. Used by help output, the doctor subcommand, and
// the profile-keys parity test.
func AllProfiles() []Profile {
	out := make([]Profile, len(allProfilesSlice))
	for i, p := range allProfilesSlice {
		clone := Profile{Name: p.Name, Summary: p.Summary, Env: make(map[string]string, len(p.Env))}
		maps.Copy(clone.Env, p.Env)
		out[i] = clone
	}
	return out
}

// ProfileNames returns the profile names in a stable sorted order,
// suitable for error messages and the EnvSpec Enum list.
func ProfileNames() []string {
	names := make([]string, 0, len(allProfilesSlice))
	for _, p := range allProfilesSlice {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	return names
}

// ProfileByName resolves a profile by its name. Returns an actionable
// error naming the valid choices when the name is unknown.
func ProfileByName(name string) (*Profile, error) {
	for i := range allProfilesSlice {
		if allProfilesSlice[i].Name == name {
			p := allProfilesSlice[i]
			return &p, nil
		}
	}
	return nil, fmt.Errorf("unknown profile %q; valid: %s", name, strings.Join(ProfileNames(), ", "))
}

// hostedProfiles is the set of profile names that target a multi-tenant /
// shared deployment posture. These profiles forbid certain operator
// shortcuts that would be merely awkward in a single-user local install
// but actively dangerous in production (e.g. CLOCKIFY_INSECURE=1, see
// Load()).
var hostedProfiles = map[string]bool{
	"shared-service": true,
	"prod-postgres":  true,
}

// IsHostedProfile reports whether the given profile name is one of the
// hosted profiles whose strict gates apply (currently shared-service
// and prod-postgres). Exported so packages outside config (runtime,
// authn) can take the same posture decision without re-implementing
// the registry.
func IsHostedProfile(name string) bool { return hostedProfiles[name] }

// isHostedProfile is the legacy unexported alias kept for the existing
// internal callers; new code should call IsHostedProfile.
func isHostedProfile(name string) bool { return IsHostedProfile(name) }

// applyProfile materialises the named profile's defaults into the
// process environment for any currently-unset key. Called exactly once
// at the top of Load() when MCP_PROFILE is set. Explicit env overrides
// are preserved (the "if unset" check is strict on empty string, which
// matches how Load() already distinguishes "default" from "explicit").
//
// Returns the resolved *Profile so the caller can log which profile
// was applied. Unknown profile names surface as a Load() error, not a
// silent fallback.
func applyProfile(name string) (*Profile, error) {
	p, err := ProfileByName(name)
	if err != nil {
		return nil, err
	}
	for k, v := range p.Env {
		if os.Getenv(k) == "" {
			if err := os.Setenv(k, v); err != nil {
				return nil, fmt.Errorf("applyProfile %s: set %s: %w", name, k, err)
			}
		}
	}
	return p, nil
}
