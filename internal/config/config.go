package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/resolve"
)

const DefaultBaseURL = "https://api.clockify.me/api/v1"

type Config struct {
	// Clockify
	APIKey         string
	WorkspaceID    string
	BaseURL        string
	RequestTimeout time.Duration
	MaxRetries     int
	Insecure       bool
	Timezone       string

	// Profile is the resolved value of MCP_PROFILE for this load cycle
	// (empty when unset). Surfaces the deployment posture to packages
	// outside config (e.g. runtime/tenantRuntime, which needs to know
	// whether hosted-mode validation should apply to per-tenant
	// baseURLs). Read-only after Load; do not mutate.
	Profile string

	// MCP transport
	Transport       string
	AuthMode        string
	HTTPBind        string
	GRPCBind        string
	BearerToken     string
	AllowedOrigins  []string
	AllowAnyOrigin  bool
	StrictHostCheck bool
	// BehindHTTPSProxy lets the HTTP transports emit
	// Strict-Transport-Security on plaintext responses because a
	// trusted upstream proxy is terminating TLS for us. Wired from
	// MCP_BEHIND_HTTPS_PROXY=1.
	BehindHTTPSProxy   bool
	MaxMessageSize     int64
	MetricsBind        string
	MetricsAuthMode    string
	MetricsBearerToken string
	// HTTP admission limits are process-local app-layer guards for
	// streamable_http and legacy http. Hosted profiles enable them by
	// default to reject obvious auth-probe and SSE-hold abuse before
	// JSON-RPC dispatch; cross-replica hosted quotas still belong at
	// the gateway/load-balancer layer.
	HTTPRateLimitPerIP         int
	HTTPRateLimitPerPrincipal  int
	HTTPRateLimitGETPerSession int
	HTTPRequireProtocolVersion bool
	DefaultProtocolVersion     string

	// Streamable HTTP session caps (per-replica and per-principal).
	// 0 = disabled (matches legacy behaviour). When non-zero,
	// streamSessionManager.create rejects initialize requests beyond
	// the cap with HTTP 503 + Retry-After (replica cap) or HTTP 429
	// + Retry-After (principal cap). See ADR/Wave 5 notes.
	MaxSessionsPerReplica   int
	MaxSessionsPerPrincipal int

	// Enterprise shared-service
	ControlPlaneDSN string
	// ControlPlaneAuditCap is the max number of audit events retained
	// in the file-backed control-plane store. 0 keeps the historical
	// unbounded behaviour; non-zero enables FIFO eviction. Wired from
	// MCP_CONTROL_PLANE_AUDIT_CAP. Postgres deployments ignore this
	// field in favour of time-based retention (B2).
	ControlPlaneAuditCap int
	// ControlPlaneAuditRetention caps the age of retained audit
	// events. The B2 reaper calls Store.RetainAudit(ctx, retention)
	// on a one-hour ticker; events with `at < now - retention` are
	// removed from the backend. Zero disables retention. Wired from
	// MCP_CONTROL_PLANE_AUDIT_RETENTION; default 720h (30 days).
	ControlPlaneAuditRetention time.Duration
	SessionTTL                 time.Duration
	TenantClaim                string
	SubjectClaim               string
	DefaultTenantID            string
	OIDCIssuer                 string
	OIDCAudience               string
	OIDCJWKSURL                string
	OIDCJWKSPath               string
	// OIDCJWKSAllowPrivate permits an explicit MCP_OIDC_JWKS_URL to
	// target loopback/private/reserved addresses. Default false blocks
	// local-network and metadata-service SSRF probes caused by
	// misconfigured JWKS overrides. Wired from
	// MCP_OIDC_JWKS_ALLOW_PRIVATE=1.
	OIDCJWKSAllowPrivate bool
	// OIDCResourceURI is the canonical URI clients use to address this
	// MCP server. When set, every OIDC token must list this URI in its
	// audience claim — RFC 8707 / MCP OAuth 2.1 resource indicator
	// binding. Wired from MCP_RESOURCE_URI.
	OIDCResourceURI string
	// OIDCVerifyCacheTTL is the hard ceiling on cached OIDC verify
	// results. Larger values amortise the per-request verify cost but
	// extend the window before a revoked token is re-checked. Zero
	// selects the conservative 60s default baked into authn; values
	// outside [1s, 5m] are rejected at config load. Hosted profiles
	// clamp explicit values above 60s back to 60s so revocation cannot
	// drift past the hosted-service contract. Wired from
	// MCP_OIDC_VERIFY_CACHE_TTL.
	OIDCVerifyCacheTTL time.Duration
	// OIDCJWKSCacheTTL is the lifetime of the in-memory JWKS document
	// cache. Zero leaves authn at its 5-minute default; explicit values
	// are validated at Load() to lie in [1m, 24h]. Hosted services
	// that rotate keys frequently can shorten the window so a
	// rotation lands without process restart. Pairs with the F2
	// kid-miss-triggered refresh for in-flight rotation handling.
	// Wired from MCP_OIDC_JWKS_CACHE_TTL.
	OIDCJWKSCacheTTL time.Duration
	// OIDCStrict opts the server into stricter OIDC behaviour for
	// shared-service / hosted deployments. When true, config.Load
	// rejects oidc + (no audience + no resource URI) and the OIDC
	// authenticator rejects tokens missing an `exp` claim. Wired from
	// MCP_OIDC_STRICT=1.
	OIDCStrict bool
	// OIDCRequireKID rejects OIDC JWTs whose JOSE header omits kid.
	// Hosted profiles enable it by default so a single-key JWKS cannot
	// accept kid-less tokens through the compatibility fallback.
	OIDCRequireKID       bool
	DisableInlineSecrets bool
	// ExposeAuthErrors controls whether HTTP transports include detailed
	// authenticator failure reasons in unauthenticated client responses.
	// Default false returns a generic "authentication failed" description
	// while server-side logs retain the detailed reason for operators.
	ExposeAuthErrors bool
	// SanitizeUpstreamErrors controls whether tool errors returned to MCP
	// clients omit the upstream Clockify response body. Default false
	// (verbose, useful for local development); hosted profiles
	// (shared-service, prod-postgres) flip it to true so a 4xx response
	// body cannot leak per-tenant info across tenant boundaries. The
	// full APIError is always logged server-side regardless. Wired from
	// CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=1, with the hosted-profile
	// default applied when the operator hasn't overridden it.
	SanitizeUpstreamErrors bool
	// WebhookValidateDNS, when true, makes CreateWebhook/UpdateWebhook
	// resolve the host and reject any reply containing a private,
	// reserved, link-local, or loopback IP. Default false (literal-IP
	// check only); hosted profiles flip it on so a hostname pointing
	// at 169.254.169.254 (cloud-metadata) or a private-range A record
	// can't turn the Clockify outbound webhook delivery into an SSRF
	// probe. Wired from CLOCKIFY_WEBHOOK_VALIDATE_DNS=1, with the
	// hosted-profile default applied when the operator hasn't overridden.
	WebhookValidateDNS bool
	// WebhookAllowedDomains is the per-deployment escape-hatch list of
	// hostnames that bypass the WebhookValidateDNS private-IP check.
	// Supplied as a comma-separated list via
	// CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS; whitespace around each entry is
	// trimmed and empty entries are skipped. Each entry matches a host
	// either exactly (`webhook.example.com`) or as a leading-dot
	// suffix that anchors a full DNS label (`.example.com` matches
	// `webhook.example.com` and `api.eu.example.com` but NOT
	// `attacker.example.com.evil.com`). Empty list = no bypass.
	// Use case: split-horizon DNS where a known-trusted hostname
	// resolves to a private IP only on the control-plane network.
	// See docs/runbooks/webhook-dns-validation.md §4b.
	WebhookAllowedDomains []string
	// RequireTenantClaim, when true, makes the OIDC authenticator
	// reject any token whose tenant claim is absent — instead of
	// quietly falling back to MCP_DEFAULT_TENANT_ID. Wired from
	// MCP_REQUIRE_TENANT_CLAIM=1; the default fallback is preserved
	// for backward-compat with self-hosted single-tenant deployments.
	RequireTenantClaim   bool
	ForwardTenantHeader  string
	ForwardSubjectHeader string
	// RequireForwardTenantClaim, when true, makes forward_auth reject
	// requests missing the forward tenant header instead of falling back
	// to MCP_DEFAULT_TENANT_ID. Wired from
	// MCP_REQUIRE_FORWARD_TENANT_CLAIM=1; hosted profiles set it by
	// default so proxy header drift cannot collapse tenants.
	RequireForwardTenantClaim bool
	// ForwardAuthTrustedProxies is the parsed CIDR allow-list for
	// the forward_auth authenticator. The authenticator rejects any
	// request whose source address is not inside one of these networks
	// before reading X-Forwarded-User / X-Forwarded-Tenant. An empty
	// MCP_FORWARD_AUTH_TRUSTED_PROXIES value is allowed only on a
	// loopback bind and is narrowed to loopback CIDRs at config load.
	// Wired from MCP_FORWARD_AUTH_TRUSTED_PROXIES (comma-separated).
	ForwardAuthTrustedProxies []*net.IPNet
	MTLSTenantHeader          string
	// MTLSTenantSource selects how the mtls authenticator derives the
	// tenant identifier. "cert" (default) uses the verified client
	// certificate (URI SAN → Subject O fallback). "header" honours the
	// MTLSTenantHeader. "header_or_cert" tries the header first, then
	// the cert. Wired from MCP_MTLS_TENANT_SOURCE.
	MTLSTenantSource string
	// RequireMTLSTenant rejects authentication when the configured
	// tenant source(s) yield no tenant. Wired from
	// MCP_REQUIRE_MTLS_TENANT=1; default false preserves the
	// historical "fall back to MCP_DEFAULT_TENANT_ID" behaviour for
	// self-hosted single-tenant deployments.
	RequireMTLSTenant bool

	// Tool execution
	ToolTimeout               time.Duration
	ConcurrencyAcquireTimeout time.Duration

	// Dispatch-layer concurrency bound for stdio tools/call. 0 disables.
	MaxInFlightToolCalls int

	// Hard cap on entries aggregated by report tools. 0 disables.
	ReportMaxEntries int

	// DeltaFormat selects the resource notification diff algorithm.
	// "merge" (default) = RFC 7396 merge patch. "jsonpatch" = RFC 6902.
	DeltaFormat string

	// GRPCReauthInterval is how often long-lived gRPC streams re-validate
	// their auth token. 0 = disabled (per-stream validation only).
	GRPCReauthInterval time.Duration
	// GRPCPeerCIDRAllow optionally restricts gRPC calls to peers whose
	// source IP is inside one of these CIDRs. Empty preserves the default
	// behavior. Wired from MCP_GRPC_PEER_CIDR_ALLOW.
	GRPCPeerCIDRAllow []*net.IPNet

	// AuditDurabilityMode controls behavior when audit persistence fails for
	// a successful non-read-only tool call.
	// "best_effort" (default): log + metric; the call still reports success.
	// "fail_closed": the call returns an error so the client knows the audit
	// trail is incomplete. The mutation already happened; this prevents
	// silent untracked mutations when the intent record is not durable.
	// "fail_closed_strict": also returns an error when the post-mutation
	// outcome record cannot be persisted.
	AuditDurabilityMode string

	// HTTPInlineMetricsEnabled controls whether /metrics is mounted on the
	// main HTTP listener when MCP_TRANSPORT=http. Default: false (disabled).
	// The dedicated metrics listener (MCP_METRICS_BIND) is the preferred
	// pattern; this is a compatibility escape hatch that requires explicit
	// operator intent.
	HTTPInlineMetricsEnabled bool
	// HTTPInlineMetricsAuthMode governs auth for inline main-listener /metrics.
	// "inherit_main_bearer" (default when enabled): require the same bearer
	// token as the /mcp endpoint.
	// "static_bearer": require a separate MCP_HTTP_INLINE_METRICS_BEARER_TOKEN.
	// "none": unauthenticated — operator must opt in explicitly; startup warns.
	HTTPInlineMetricsAuthMode string
	// HTTPInlineMetricsBearerToken is the separate token for inline metrics
	// when HTTPInlineMetricsAuthMode == "static_bearer".
	HTTPInlineMetricsBearerToken string

	// HTTPLegacyPolicy governs startup behavior when MCP_TRANSPORT=http.
	// "warn" (default): emit structured startup warnings about legacy HTTP
	// limitations and recommend streamable_http.
	// "deny": refuse to start; operator must switch to streamable_http or
	// explicitly set MCP_HTTP_LEGACY_POLICY=allow.
	// "allow": permit startup without deny or warn behavior.
	HTTPLegacyPolicy string

	// GRPCTLSCert and GRPCTLSKey are paths to the server TLS cert and
	// private key for the gRPC transport. Both must be set together.
	GRPCTLSCert string
	GRPCTLSKey  string
	// HTTPTLSCert and HTTPTLSKey are paths to the server TLS cert and
	// private key for the streamable_http transport. Both must be set
	// together. Required when MCP_AUTH_MODE=mtls on streamable_http.
	// Legacy http (MCP_TRANSPORT=http) does not support TLS — see the
	// HTTPTLSCert validation in Load().
	HTTPTLSCert string
	HTTPTLSKey  string
	// MTLSCACertPath is the path to the CA cert for client certificate
	// verification. Required when MCP_AUTH_MODE=mtls on gRPC or
	// streamable_http.
	MTLSCACertPath string
}

func Load() (Config, error) {
	// Apply the selected deployment profile first so every subsequent
	// os.Getenv in this function sees profile defaults for any key
	// the operator did not set explicitly. Explicit values always
	// win because applyProfile only writes to unset keys. Profile is
	// opt-in: with MCP_PROFILE empty the behaviour is unchanged.
	profileName := strings.TrimSpace(os.Getenv("MCP_PROFILE"))
	if profileName != "" {
		if _, err := applyProfile(profileName); err != nil {
			return Config{}, err
		}
	}

	cfg := Config{
		APIKey:      os.Getenv("CLOCKIFY_API_KEY"),
		WorkspaceID: os.Getenv("CLOCKIFY_WORKSPACE_ID"),
		BaseURL:     strings.TrimRight(os.Getenv("CLOCKIFY_BASE_URL"), "/"),
		Insecure:    os.Getenv("CLOCKIFY_INSECURE") == "1",
		Profile:     profileName,
	}
	// Hosted-profile guardrail: a multi-tenant deployment that points at a
	// remote HTTP endpoint with TLS off is almost certainly a misconfiguration
	// and a credential-leak risk. Refuse to start.
	if cfg.Insecure && isHostedProfile(profileName) {
		return Config{}, fmt.Errorf("CLOCKIFY_INSECURE=1 is refused under MCP_PROFILE=%s; use HTTPS or switch profile", profileName)
	}
	if cfg.WorkspaceID != "" {
		if err := resolve.ValidateID(cfg.WorkspaceID, "CLOCKIFY_WORKSPACE_ID"); err != nil {
			return Config{}, fmt.Errorf("invalid CLOCKIFY_WORKSPACE_ID: %w", err)
		}
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}

	cfg.RequestTimeout = 30 * time.Second
	cfg.MaxRetries = 3
	if v := strings.TrimSpace(os.Getenv("CLOCKIFY_MAX_RETRIES")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 10 {
			return Config{}, fmt.Errorf("invalid CLOCKIFY_MAX_RETRIES %q: must be an integer between 0 and 10", v)
		}
		cfg.MaxRetries = n
	}

	// Timezone
	cfg.Timezone = os.Getenv("CLOCKIFY_TIMEZONE")
	if cfg.Timezone != "" {
		if _, err := time.LoadLocation(cfg.Timezone); err != nil {
			return Config{}, fmt.Errorf("invalid CLOCKIFY_TIMEZONE %q: %w", cfg.Timezone, err)
		}
	}

	// MCP transport settings
	cfg.Transport = os.Getenv("MCP_TRANSPORT")
	if cfg.Transport == "" {
		cfg.Transport = "stdio"
	}
	switch cfg.Transport {
	case "stdio", "http", "streamable_http", "grpc":
		// valid
	default:
		return Config{}, fmt.Errorf("invalid MCP_TRANSPORT %q: must be \"stdio\", \"http\", \"streamable_http\", or \"grpc\"", cfg.Transport)
	}
	if cfg.Transport != "streamable_http" && cfg.APIKey == "" {
		return Config{}, fmt.Errorf("CLOCKIFY_API_KEY is required")
	}
	// single-tenant-http bootstraps the only tenant from the env
	// API key (bootstrapDefaultTenant in internal/runtime/service.go).
	// streamable_http otherwise tolerates an empty key because hosted
	// shared-service tenants resolve credentials through the control
	// plane — but for this profile, an empty key means /ready never
	// validates Clockify and the first session fails with "tenant not
	// found". Fail at config load instead of letting the pod become
	// "ready" with no real backend behind it.
	if profileName == "single-tenant-http" && cfg.APIKey == "" {
		return Config{}, fmt.Errorf(
			"CLOCKIFY_API_KEY is required for MCP_PROFILE=single-tenant-http " +
				"(profile bootstraps the default tenant from the env API key)")
	}
	if cfg.APIKey != "" {
		if err := ValidateBaseURL(cfg.BaseURL, ValidateBaseURLOptions{
			Hosted:        IsHostedProfile(profileName),
			AllowInsecure: cfg.Insecure,
		}); err != nil {
			return Config{}, err
		}
	}

	cfg.AuthMode = strings.TrimSpace(os.Getenv("MCP_AUTH_MODE"))
	if cfg.AuthMode == "" {
		switch cfg.Transport {
		case "streamable_http":
			cfg.AuthMode = "oidc"
		case "http":
			cfg.AuthMode = "static_bearer"
		default:
			cfg.AuthMode = ""
		}
	}
	switch cfg.AuthMode {
	case "", "static_bearer", "oidc", "forward_auth", "mtls":
	default:
		return Config{}, fmt.Errorf("invalid MCP_AUTH_MODE %q", cfg.AuthMode)
	}
	if cfg.Transport == "stdio" && cfg.AuthMode != "" {
		return Config{}, fmt.Errorf("MCP_AUTH_MODE is only valid for HTTP transports")
	}
	if cfg.Transport == "grpc" {
		switch cfg.AuthMode {
		case "", "static_bearer", "oidc", "forward_auth":
			// static_bearer and oidc use Authorization metadata.
			// forward_auth reads x-forwarded-* metadata keys.
		case "mtls":
			// Supported when the gRPC server is TLS-enabled
			// (MCP_GRPC_TLS_CERT + MCP_GRPC_TLS_KEY set).
		}
	}
	// Legacy HTTP transport accepts the full auth matrix except mtls:
	// mTLS authenticates off r.TLS.VerifiedChains, and the legacy HTTP
	// path does not wire its own TLS listener. Fail at config load so
	// operators see the mismatch up front instead of every request
	// 401'ing at runtime.
	if cfg.Transport == "http" && cfg.AuthMode == "mtls" {
		return Config{}, fmt.Errorf("MCP_AUTH_MODE=mtls is not supported with MCP_TRANSPORT=http (no native TLS wiring); terminate TLS in a reverse proxy and use MCP_AUTH_MODE=forward_auth, or use MCP_TRANSPORT=grpc with MCP_GRPC_TLS_CERT/MCP_GRPC_TLS_KEY")
	}

	cfg.HTTPBind = os.Getenv("MCP_HTTP_BIND")
	if cfg.HTTPBind == "" {
		cfg.HTTPBind = ":8080"
	}

	cfg.GRPCBind = os.Getenv("MCP_GRPC_BIND")
	if cfg.GRPCBind == "" {
		cfg.GRPCBind = ":9090"
	}

	cfg.BearerToken = os.Getenv("MCP_BEARER_TOKEN")
	if cfg.Transport != "stdio" && cfg.AuthMode == "static_bearer" {
		if cfg.BearerToken == "" {
			return Config{}, fmt.Errorf("MCP_BEARER_TOKEN is required for static bearer auth")
		}
		if len(cfg.BearerToken) < 16 {
			return Config{}, fmt.Errorf("MCP_BEARER_TOKEN must be at least 16 characters for security")
		}
	}
	cfg.MetricsBind = strings.TrimSpace(os.Getenv("MCP_METRICS_BIND"))
	cfg.MetricsAuthMode = strings.TrimSpace(os.Getenv("MCP_METRICS_AUTH_MODE"))

	if cfg.MetricsBind != "" {
		if cfg.MetricsAuthMode == "" {
			cfg.MetricsAuthMode = "static_bearer"
		}
		switch cfg.MetricsAuthMode {
		case "none", "static_bearer":
		default:
			return Config{}, fmt.Errorf("invalid MCP_METRICS_AUTH_MODE %q", cfg.MetricsAuthMode)
		}
		cfg.MetricsBearerToken = strings.TrimSpace(os.Getenv("MCP_METRICS_BEARER_TOKEN"))
		if cfg.MetricsAuthMode == "static_bearer" {
			if cfg.MetricsBearerToken == "" {
				return Config{}, fmt.Errorf("MCP_METRICS_BEARER_TOKEN is required when MCP_METRICS_AUTH_MODE=static_bearer")
			}
			if len(cfg.MetricsBearerToken) < 16 {
				return Config{}, fmt.Errorf("MCP_METRICS_BEARER_TOKEN must be at least 16 characters for security")
			}
		}
	}

	cfg.ControlPlaneDSN = strings.TrimSpace(os.Getenv("MCP_CONTROL_PLANE_DSN"))
	if cfg.ControlPlaneDSN == "" {
		cfg.ControlPlaneDSN = "memory"
	}
	if v := strings.TrimSpace(os.Getenv("MCP_CONTROL_PLANE_AUDIT_CAP")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return Config{}, fmt.Errorf("invalid MCP_CONTROL_PLANE_AUDIT_CAP %q: must be a non-negative integer", v)
		}
		cfg.ControlPlaneAuditCap = n
	}
	cfg.ControlPlaneAuditRetention = 720 * time.Hour
	if raw := strings.TrimSpace(os.Getenv("MCP_CONTROL_PLANE_AUDIT_RETENTION")); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid MCP_CONTROL_PLANE_AUDIT_RETENTION %q: %w", raw, err)
		}
		if d != 0 && (d < time.Hour || d > 8760*time.Hour) {
			return Config{}, fmt.Errorf("MCP_CONTROL_PLANE_AUDIT_RETENTION must be 0 or between 1h and 8760h, got %s", d)
		}
		cfg.ControlPlaneAuditRetention = d
	}
	cfg.SessionTTL = 30 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("MCP_SESSION_TTL")); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid MCP_SESSION_TTL %q: %w", raw, err)
		}
		if d < time.Minute || d > 24*time.Hour {
			return Config{}, fmt.Errorf("MCP_SESSION_TTL must be between 1m and 24h")
		}
		cfg.SessionTTL = d
	}
	cfg.TenantClaim = strings.TrimSpace(os.Getenv("MCP_TENANT_CLAIM"))
	if cfg.TenantClaim == "" {
		cfg.TenantClaim = "tenant_id"
	}
	cfg.SubjectClaim = strings.TrimSpace(os.Getenv("MCP_SUBJECT_CLAIM"))
	if cfg.SubjectClaim == "" {
		cfg.SubjectClaim = "sub"
	}
	cfg.DefaultTenantID = strings.TrimSpace(os.Getenv("MCP_DEFAULT_TENANT_ID"))
	if cfg.DefaultTenantID == "" {
		cfg.DefaultTenantID = "default"
	}
	cfg.OIDCIssuer = strings.TrimSpace(os.Getenv("MCP_OIDC_ISSUER"))
	cfg.OIDCAudience = strings.TrimSpace(os.Getenv("MCP_OIDC_AUDIENCE"))
	cfg.OIDCJWKSURL = strings.TrimSpace(os.Getenv("MCP_OIDC_JWKS_URL"))
	cfg.OIDCJWKSPath = strings.TrimSpace(os.Getenv("MCP_OIDC_JWKS_PATH"))
	cfg.OIDCJWKSAllowPrivate = os.Getenv("MCP_OIDC_JWKS_ALLOW_PRIVATE") == "1"
	cfg.OIDCResourceURI = strings.TrimSpace(os.Getenv("MCP_RESOURCE_URI"))
	if v := strings.TrimSpace(os.Getenv("MCP_OIDC_VERIFY_CACHE_TTL")); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid MCP_OIDC_VERIFY_CACHE_TTL %q: %w", v, err)
		}
		if d < time.Second || d > 5*time.Minute {
			return Config{}, fmt.Errorf("MCP_OIDC_VERIFY_CACHE_TTL must be between 1s and 5m, got %s", d)
		}
		cfg.OIDCVerifyCacheTTL = d
	}
	if isHostedProfile(profileName) && cfg.OIDCVerifyCacheTTL > time.Minute {
		cfg.OIDCVerifyCacheTTL = time.Minute
	}
	if v := strings.TrimSpace(os.Getenv("MCP_OIDC_JWKS_CACHE_TTL")); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid MCP_OIDC_JWKS_CACHE_TTL %q: %w", v, err)
		}
		if d < time.Minute || d > 24*time.Hour {
			return Config{}, fmt.Errorf("MCP_OIDC_JWKS_CACHE_TTL must be between 1m and 24h, got %s", d)
		}
		cfg.OIDCJWKSCacheTTL = d
	}
	cfg.ForwardTenantHeader = strings.TrimSpace(os.Getenv("MCP_FORWARD_TENANT_HEADER"))
	cfg.ForwardSubjectHeader = strings.TrimSpace(os.Getenv("MCP_FORWARD_SUBJECT_HEADER"))
	cfg.RequireForwardTenantClaim = os.Getenv("MCP_REQUIRE_FORWARD_TENANT_CLAIM") == "1"
	if raw := strings.TrimSpace(os.Getenv("MCP_FORWARD_AUTH_TRUSTED_PROXIES")); raw != "" {
		nets, err := parseCIDRList(raw)
		if err != nil {
			return Config{}, fmt.Errorf("MCP_FORWARD_AUTH_TRUSTED_PROXIES: %w", err)
		}
		cfg.ForwardAuthTrustedProxies = nets
	}
	if cfg.AuthMode == "forward_auth" && len(cfg.ForwardAuthTrustedProxies) == 0 {
		if !isLoopbackBind(bindForForwardAuth(cfg)) {
			return Config{}, fmt.Errorf("MCP_AUTH_MODE=forward_auth with %s=%q requires MCP_FORWARD_AUTH_TRUSTED_PROXIES to be set (or bind to loopback)", bindEnvForForwardAuth(cfg), bindForForwardAuth(cfg))
		}
		cfg.ForwardAuthTrustedProxies = loopbackTrustedProxies()
	}
	cfg.MTLSTenantHeader = strings.TrimSpace(os.Getenv("MCP_MTLS_TENANT_HEADER"))
	cfg.MTLSTenantSource = strings.TrimSpace(os.Getenv("MCP_MTLS_TENANT_SOURCE"))
	if cfg.MTLSTenantSource == "" {
		cfg.MTLSTenantSource = "cert"
	}
	switch cfg.MTLSTenantSource {
	case "cert", "header", "header_or_cert":
	default:
		return Config{}, fmt.Errorf("invalid MCP_MTLS_TENANT_SOURCE %q: must be \"cert\", \"header\", or \"header_or_cert\"", cfg.MTLSTenantSource)
	}
	cfg.RequireMTLSTenant = os.Getenv("MCP_REQUIRE_MTLS_TENANT") == "1"
	cfg.OIDCStrict = os.Getenv("MCP_OIDC_STRICT") == "1"
	cfg.OIDCRequireKID = os.Getenv("MCP_OIDC_REQUIRE_KID") == "1"
	cfg.RequireTenantClaim = os.Getenv("MCP_REQUIRE_TENANT_CLAIM") == "1"
	cfg.DisableInlineSecrets = os.Getenv("MCP_DISABLE_INLINE_SECRETS") == "1"
	cfg.ExposeAuthErrors = os.Getenv("MCP_EXPOSE_AUTH_ERRORS") == "1"
	if cfg.ExposeAuthErrors && isHostedProfile(profileName) {
		if os.Getenv("MCP_EXPOSE_AUTH_ERRORS_BREAK_GLASS") != "1" {
			return Config{}, fmt.Errorf("MCP_EXPOSE_AUTH_ERRORS=1 is refused under MCP_PROFILE=%s; set MCP_EXPOSE_AUTH_ERRORS_BREAK_GLASS=1 only for a documented temporary incident response", profileName)
		}
		slog.Warn("hosted_break_glass_enabled",
			"profile", profileName,
			"setting", "MCP_EXPOSE_AUTH_ERRORS",
			"break_glass_env", "MCP_EXPOSE_AUTH_ERRORS_BREAK_GLASS")
	}
	// CLOCKIFY_SANITIZE_UPSTREAM_ERRORS: explicit override always wins.
	// If unset, hosted profiles default to sanitised output to keep a
	// per-tenant Clockify response body off the MCP wire.
	if raw := os.Getenv("CLOCKIFY_SANITIZE_UPSTREAM_ERRORS"); raw != "" {
		cfg.SanitizeUpstreamErrors = raw == "1" || strings.EqualFold(raw, "true")
	} else if isHostedProfile(profileName) {
		cfg.SanitizeUpstreamErrors = true
	}
	// CLOCKIFY_WEBHOOK_VALIDATE_DNS: explicit override wins; default
	// to true for every profile so hostname-based webhook SSRF checks
	// are active unless an operator opts out.
	if raw := os.Getenv("CLOCKIFY_WEBHOOK_VALIDATE_DNS"); raw != "" {
		cfg.WebhookValidateDNS = raw == "1" || strings.EqualFold(raw, "true")
	} else {
		cfg.WebhookValidateDNS = true
	}
	if !cfg.WebhookValidateDNS && isHostedProfile(profileName) {
		if os.Getenv("CLOCKIFY_WEBHOOK_VALIDATE_DNS_BREAK_GLASS") != "1" {
			return Config{}, fmt.Errorf("CLOCKIFY_WEBHOOK_VALIDATE_DNS=0 is refused under MCP_PROFILE=%s; prefer CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS or set CLOCKIFY_WEBHOOK_VALIDATE_DNS_BREAK_GLASS=1 only for a documented temporary incident response", profileName)
		}
		slog.Warn("hosted_break_glass_enabled",
			"profile", profileName,
			"setting", "CLOCKIFY_WEBHOOK_VALIDATE_DNS",
			"break_glass_env", "CLOCKIFY_WEBHOOK_VALIDATE_DNS_BREAK_GLASS")
	}
	// CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS: comma-separated escape-hatch
	// list of hostnames that bypass the WebhookValidateDNS private-IP
	// check. Whitespace around each entry is trimmed and empty
	// entries are dropped here — the validator (`isWebhookHostAllowed`
	// in internal/tools/tier2_webhooks.go) ALSO trims and skips
	// empties as defence-in-depth, but doing it once at config-load
	// time keeps the parsed surface small.
	if raw := os.Getenv("CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS"); raw != "" {
		for item := range strings.SplitSeq(raw, ",") {
			item = strings.TrimSpace(item)
			if item != "" {
				cfg.WebhookAllowedDomains = append(cfg.WebhookAllowedDomains, item)
			}
		}
	}
	// Strict mode: an oidc deployment without either MCP_OIDC_AUDIENCE
	// or MCP_RESOURCE_URI accepts any valid issuer-signed token,
	// regardless of whether the token was minted for this server. That
	// is the C1 finding from the 2026-04-25 audit. Force one or the
	// other when the operator opts into strict mode.
	if cfg.AuthMode == "oidc" && cfg.OIDCStrict && cfg.OIDCAudience == "" && cfg.OIDCResourceURI == "" {
		return Config{}, fmt.Errorf("MCP_OIDC_STRICT=1 requires MCP_OIDC_AUDIENCE or MCP_RESOURCE_URI to bind tokens to this server")
	}
	if cfg.AuthMode == "oidc" && cfg.Transport == "streamable_http" && cfg.OIDCIssuer == "" {
		return Config{}, fmt.Errorf("MCP_OIDC_ISSUER is required when MCP_TRANSPORT=streamable_http and MCP_AUTH_MODE=oidc")
	}
	// Fail-closed dev-backend guard for streamable_http and grpc. A dev
	// DSN (memory/file/bare path) cannot back a multi-process deployment
	// correctly — session state, audit events, and rate-limit counters
	// diverge across replicas. Both transports are deployed multi-replica
	// behind a load balancer in production (private-network-grpc profile
	// pairs grpc with fail_closed audit, which a memory backend cannot
	// honour across pod restarts). Require an explicit acknowledgement
	// so an operator who wants the single-process path knows they are
	// on it. See docs/adr/0014-prod-fail-closed-defaults.md.
	//
	// Belt + suspenders: runtime.BuildStore repeats the same check so
	// a caller that bypasses Load() (e.g. a custom wiring path) still
	// refuses to start against a dev DSN.
	if (cfg.Transport == "streamable_http" || cfg.Transport == "grpc") &&
		IsDevControlPlaneDSN(cfg.ControlPlaneDSN) &&
		os.Getenv("MCP_ALLOW_DEV_BACKEND") != "1" {
		return Config{}, fmt.Errorf(
			"MCP_TRANSPORT=%q with MCP_CONTROL_PLANE_DSN=%q (dev backend) is disallowed by default; set MCP_ALLOW_DEV_BACKEND=1 to acknowledge the single-process limits, or point MCP_CONTROL_PLANE_DSN at a production backend (postgres://...)",
			cfg.Transport, cfg.ControlPlaneDSN)
	}

	if origins := os.Getenv("MCP_ALLOWED_ORIGINS"); origins != "" {
		parts := strings.Split(origins, ",")
		cfg.AllowedOrigins = make([]string, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				cfg.AllowedOrigins = append(cfg.AllowedOrigins, trimmed)
			}
		}
	}

	cfg.AllowAnyOrigin = os.Getenv("MCP_ALLOW_ANY_ORIGIN") == "1"
	if cfg.AllowAnyOrigin && isHostedProfile(profileName) {
		return Config{}, fmt.Errorf("MCP_ALLOW_ANY_ORIGIN=1 is refused under MCP_PROFILE=%s; configure MCP_ALLOWED_ORIGINS instead", profileName)
	}
	strictHostCheck, err := optionalBoolEnv("MCP_STRICT_HOST_CHECK")
	if err != nil {
		return Config{}, err
	}
	cfg.StrictHostCheck = strictHostCheck
	cfg.BehindHTTPSProxy = os.Getenv("MCP_BEHIND_HTTPS_PROXY") == "1"

	// Strict host check + empty allowlist on a non-loopback bind would
	// reject every request — isHostAllowed admits only loopback hosts
	// and entries derived from MCP_ALLOWED_ORIGINS. Catching this at
	// config load gives the operator a clear actionable error instead
	// of an opaque 403 from every probe and client.
	if cfg.StrictHostCheck &&
		(cfg.Transport == "http" || cfg.Transport == "streamable_http") &&
		!cfg.AllowAnyOrigin &&
		len(cfg.AllowedOrigins) == 0 &&
		!isLoopbackBind(cfg.HTTPBind) {
		return Config{}, fmt.Errorf(
			"MCP_STRICT_HOST_CHECK=1 with MCP_HTTP_BIND=%q requires MCP_ALLOWED_ORIGINS to be set "+
				"(or MCP_ALLOW_ANY_ORIGIN=1, or a loopback bind); empty allowlist would reject every non-loopback request",
			cfg.HTTPBind)
	}

	cfg.MaxMessageSize = 4194304 // 4 MB default
	mbs := os.Getenv("MCP_MAX_MESSAGE_SIZE")
	if mbs == "" {
		mbs = os.Getenv("MCP_HTTP_MAX_BODY") // deprecated fallback
	}
	if mbs != "" {
		v, err := strconv.ParseInt(mbs, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("invalid MCP_MAX_MESSAGE_SIZE: %w", err)
		}
		if v <= 0 {
			return Config{}, fmt.Errorf("MCP_MAX_MESSAGE_SIZE must be greater than 0")
		}
		if v > 104857600 {
			return Config{}, fmt.Errorf("MCP_MAX_MESSAGE_SIZE must be at most 100 MB")
		}
		cfg.MaxMessageSize = v
	}
	if n, err := nonNegativeInt("MCP_HTTP_RATELIMIT_PER_IP", os.Getenv("MCP_HTTP_RATELIMIT_PER_IP"), 1_000_000); err != nil {
		return Config{}, err
	} else {
		cfg.HTTPRateLimitPerIP = n
	}
	if n, err := nonNegativeInt("MCP_HTTP_RATELIMIT_PER_PRINCIPAL", os.Getenv("MCP_HTTP_RATELIMIT_PER_PRINCIPAL"), 1_000_000); err != nil {
		return Config{}, err
	} else {
		cfg.HTTPRateLimitPerPrincipal = n
	}
	if n, err := nonNegativeInt("MCP_HTTP_RATELIMIT_GET_PER_SESSION", os.Getenv("MCP_HTTP_RATELIMIT_GET_PER_SESSION"), 10_000); err != nil {
		return Config{}, err
	} else {
		cfg.HTTPRateLimitGETPerSession = n
	}
	if n, err := nonNegativeInt("MCP_MAX_SESSIONS_PER_REPLICA", os.Getenv("MCP_MAX_SESSIONS_PER_REPLICA"), 1_000_000); err != nil {
		return Config{}, err
	} else {
		cfg.MaxSessionsPerReplica = n
	}
	if n, err := nonNegativeInt("MCP_MAX_SESSIONS_PER_PRINCIPAL", os.Getenv("MCP_MAX_SESSIONS_PER_PRINCIPAL"), 100_000); err != nil {
		return Config{}, err
	} else {
		cfg.MaxSessionsPerPrincipal = n
	}
	cfg.HTTPRequireProtocolVersion = os.Getenv("MCP_HTTP_REQUIRE_PROTOCOL_VERSION") == "1"
	cfg.DefaultProtocolVersion = strings.TrimSpace(os.Getenv("MCP_DEFAULT_PROTOCOL_VERSION"))
	if cfg.DefaultProtocolVersion != "" && !mcp.IsSupportedProtocolVersion(cfg.DefaultProtocolVersion) {
		return Config{}, fmt.Errorf("invalid MCP_DEFAULT_PROTOCOL_VERSION %q: must be one of %s",
			cfg.DefaultProtocolVersion,
			strings.Join(mcp.SupportedProtocolVersions, ", "))
	}

	// Tool timeout
	cfg.ToolTimeout = 45 * time.Second
	if tt := os.Getenv("CLOCKIFY_TOOL_TIMEOUT"); tt != "" {
		d, err := time.ParseDuration(tt)
		if err != nil {
			return Config{}, fmt.Errorf("invalid CLOCKIFY_TOOL_TIMEOUT %q: %w", tt, err)
		}
		if d < 5*time.Second || d > 10*time.Minute {
			return Config{}, fmt.Errorf("CLOCKIFY_TOOL_TIMEOUT must be between 5s and 10m")
		}
		cfg.ToolTimeout = d
	}

	cfg.ConcurrencyAcquireTimeout = 100 * time.Millisecond
	if tt := os.Getenv("CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT"); tt != "" {
		d, err := time.ParseDuration(tt)
		if err != nil {
			return Config{}, fmt.Errorf("invalid CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT %q: %w", tt, err)
		}
		if d < time.Millisecond || d > 30*time.Second {
			return Config{}, fmt.Errorf("CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT must be between 1ms and 30s")
		}
		cfg.ConcurrencyAcquireTimeout = d
	}

	cfg.MaxInFlightToolCalls = 64
	if v := os.Getenv("MCP_MAX_INFLIGHT_TOOL_CALLS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid MCP_MAX_INFLIGHT_TOOL_CALLS %q: %w", v, err)
		}
		if n < 0 {
			return Config{}, fmt.Errorf("MCP_MAX_INFLIGHT_TOOL_CALLS must be >= 0")
		}
		if n > 10000 {
			return Config{}, fmt.Errorf("MCP_MAX_INFLIGHT_TOOL_CALLS must be <= 10000")
		}
		cfg.MaxInFlightToolCalls = n
	}

	cfg.ReportMaxEntries = 10000
	if v := os.Getenv("CLOCKIFY_REPORT_MAX_ENTRIES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid CLOCKIFY_REPORT_MAX_ENTRIES %q: %w", v, err)
		}
		if n < 0 {
			return Config{}, fmt.Errorf("CLOCKIFY_REPORT_MAX_ENTRIES must be >= 0")
		}
		cfg.ReportMaxEntries = n
	}

	cfg.GRPCTLSCert = os.Getenv("MCP_GRPC_TLS_CERT")
	cfg.GRPCTLSKey = os.Getenv("MCP_GRPC_TLS_KEY")
	cfg.HTTPTLSCert = os.Getenv("MCP_HTTP_TLS_CERT")
	cfg.HTTPTLSKey = os.Getenv("MCP_HTTP_TLS_KEY")
	cfg.MTLSCACertPath = os.Getenv("MCP_MTLS_CA_CERT_PATH")
	if (cfg.GRPCTLSCert == "") != (cfg.GRPCTLSKey == "") {
		return Config{}, fmt.Errorf("MCP_GRPC_TLS_CERT and MCP_GRPC_TLS_KEY must both be set or both empty")
	}
	if (cfg.HTTPTLSCert == "") != (cfg.HTTPTLSKey == "") {
		return Config{}, fmt.Errorf("MCP_HTTP_TLS_CERT and MCP_HTTP_TLS_KEY must both be set or both empty")
	}
	// Legacy http transport never terminates TLS in-process. Operators
	// reaching for MCP_HTTP_TLS_CERT here are confusing legacy http with
	// streamable_http — surface the mismatch at startup rather than
	// silently ignoring the cert paths.
	if cfg.HTTPTLSCert != "" && cfg.Transport != "streamable_http" {
		return Config{}, fmt.Errorf("MCP_HTTP_TLS_CERT requires MCP_TRANSPORT=streamable_http; the legacy http transport does not terminate TLS")
	}
	// streamable_http + mtls requires the full TLS cert material so
	// the listener can complete the mTLS handshake. Without it the
	// server starts but every request fails with "verified mTLS client
	// certificate required". Fail at config load instead.
	if cfg.Transport == "streamable_http" && cfg.AuthMode == "mtls" {
		switch {
		case cfg.HTTPTLSCert == "":
			return Config{}, fmt.Errorf("MCP_TRANSPORT=streamable_http with MCP_AUTH_MODE=mtls requires MCP_HTTP_TLS_CERT")
		case cfg.HTTPTLSKey == "":
			return Config{}, fmt.Errorf("MCP_TRANSPORT=streamable_http with MCP_AUTH_MODE=mtls requires MCP_HTTP_TLS_KEY")
		case cfg.MTLSCACertPath == "":
			return Config{}, fmt.Errorf("MCP_TRANSPORT=streamable_http with MCP_AUTH_MODE=mtls requires MCP_MTLS_CA_CERT_PATH")
		}
	}
	// gRPC + mtls has historically allowed startup without cert
	// material; the old behaviour was a misleading "ok" cell in the
	// transport×auth matrix. Lock the requirement in here.
	if cfg.Transport == "grpc" && cfg.AuthMode == "mtls" {
		switch {
		case cfg.GRPCTLSCert == "":
			return Config{}, fmt.Errorf("MCP_TRANSPORT=grpc with MCP_AUTH_MODE=mtls requires MCP_GRPC_TLS_CERT")
		case cfg.GRPCTLSKey == "":
			return Config{}, fmt.Errorf("MCP_TRANSPORT=grpc with MCP_AUTH_MODE=mtls requires MCP_GRPC_TLS_KEY")
		case cfg.MTLSCACertPath == "":
			return Config{}, fmt.Errorf("MCP_TRANSPORT=grpc with MCP_AUTH_MODE=mtls requires MCP_MTLS_CA_CERT_PATH")
		}
	}
	if cfg.Transport == "grpc" && cfg.GRPCTLSCert == "" && !isLoopbackBind(cfg.GRPCBind) {
		return Config{}, fmt.Errorf("MCP_TRANSPORT=grpc refuses plaintext MCP_GRPC_BIND=%q on non-loopback; set MCP_GRPC_TLS_CERT and MCP_GRPC_TLS_KEY, or bind to loopback for local development", cfg.GRPCBind)
	}

	if v := os.Getenv("MCP_GRPC_REAUTH_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid MCP_GRPC_REAUTH_INTERVAL %q: %w", v, err)
		}
		cfg.GRPCReauthInterval = d
	}
	if raw := strings.TrimSpace(os.Getenv("MCP_GRPC_PEER_CIDR_ALLOW")); raw != "" {
		nets, err := parseCIDRList(raw)
		if err != nil {
			return Config{}, fmt.Errorf("MCP_GRPC_PEER_CIDR_ALLOW: %w", err)
		}
		cfg.GRPCPeerCIDRAllow = nets
	}

	cfg.DeltaFormat = strings.ToLower(strings.TrimSpace(os.Getenv("CLOCKIFY_DELTA_FORMAT")))
	if cfg.DeltaFormat == "" {
		cfg.DeltaFormat = "merge"
	}
	switch cfg.DeltaFormat {
	case "merge", "jsonpatch":
	default:
		return Config{}, fmt.Errorf("invalid CLOCKIFY_DELTA_FORMAT %q (must be merge or jsonpatch)", cfg.DeltaFormat)
	}

	// Audit durability mode. Default is "best_effort" in dev so the
	// out-of-the-box path works without a persistent store; in
	// production (ENVIRONMENT=prod) the default flips to "fail_closed"
	// so an audit-persist failure aborts the caller instead of silently
	// recording a lost event. An explicit MCP_AUDIT_DURABILITY=best_effort
	// in prod is still honoured — the operator has chosen it deliberately.
	// See docs/adr/0014-prod-fail-closed-defaults.md.
	cfg.AuditDurabilityMode = strings.TrimSpace(os.Getenv("MCP_AUDIT_DURABILITY"))
	if cfg.AuditDurabilityMode == "" {
		if os.Getenv("ENVIRONMENT") == "prod" {
			cfg.AuditDurabilityMode = "fail_closed"
		} else {
			cfg.AuditDurabilityMode = "best_effort"
		}
	}
	switch cfg.AuditDurabilityMode {
	case "best_effort", "fail_closed", "fail_closed_strict":
	default:
		return Config{}, fmt.Errorf("invalid MCP_AUDIT_DURABILITY %q: must be \"best_effort\", \"fail_closed\", or \"fail_closed_strict\"", cfg.AuditDurabilityMode)
	}

	// Inline /metrics on the main HTTP listener (MCP_TRANSPORT=http only)
	inlineMetricsEnabled, err := optionalBoolEnv("MCP_HTTP_INLINE_METRICS_ENABLED")
	if err != nil {
		return Config{}, err
	}
	cfg.HTTPInlineMetricsEnabled = inlineMetricsEnabled
	cfg.HTTPInlineMetricsAuthMode = strings.TrimSpace(os.Getenv("MCP_HTTP_INLINE_METRICS_AUTH_MODE"))
	if cfg.HTTPInlineMetricsEnabled && cfg.HTTPInlineMetricsAuthMode == "" {
		cfg.HTTPInlineMetricsAuthMode = "inherit_main_bearer"
	}
	switch cfg.HTTPInlineMetricsAuthMode {
	case "", "inherit_main_bearer", "static_bearer", "none":
	default:
		return Config{}, fmt.Errorf("invalid MCP_HTTP_INLINE_METRICS_AUTH_MODE %q: must be \"inherit_main_bearer\", \"static_bearer\", or \"none\"", cfg.HTTPInlineMetricsAuthMode)
	}
	cfg.HTTPInlineMetricsBearerToken = strings.TrimSpace(os.Getenv("MCP_HTTP_INLINE_METRICS_BEARER_TOKEN"))
	if cfg.HTTPInlineMetricsEnabled && cfg.HTTPInlineMetricsAuthMode == "static_bearer" {
		if cfg.HTTPInlineMetricsBearerToken == "" {
			return Config{}, fmt.Errorf("MCP_HTTP_INLINE_METRICS_BEARER_TOKEN is required when MCP_HTTP_INLINE_METRICS_AUTH_MODE=static_bearer")
		}
		if len(cfg.HTTPInlineMetricsBearerToken) < 16 {
			return Config{}, fmt.Errorf("MCP_HTTP_INLINE_METRICS_BEARER_TOKEN must be at least 16 characters for security")
		}
	}
	// Setting inline metrics options outside of legacy HTTP transport is a
	// no-op at runtime but not a config error — operators may share config
	// across environments.
	//
	// However, when inline metrics ARE active (Transport=http) and the
	// auth mode is the default "inherit_main_bearer", the inline handler
	// (transport_http.go inlineMetricsHandler) compares against
	// opts.MainBearerToken — which is only populated when MCP_AUTH_MODE=
	// static_bearer (transport_http.go:106). With OIDC, forward_auth, or
	// mtls main auth, MainBearerToken is empty and every scrape gets 401.
	// Catch the misconfiguration at Load() so the operator sees an
	// actionable error instead of silently dead /metrics.
	if cfg.HTTPInlineMetricsEnabled &&
		cfg.HTTPInlineMetricsAuthMode == "inherit_main_bearer" &&
		cfg.Transport == "http" &&
		cfg.AuthMode != "static_bearer" {
		return Config{}, fmt.Errorf(
			"MCP_HTTP_INLINE_METRICS_AUTH_MODE=inherit_main_bearer requires MCP_AUTH_MODE=static_bearer (current: %q); "+
				"pick MCP_HTTP_INLINE_METRICS_AUTH_MODE=static_bearer with MCP_HTTP_INLINE_METRICS_BEARER_TOKEN, "+
				"or MCP_HTTP_INLINE_METRICS_AUTH_MODE=none, or use the dedicated MCP_METRICS_BIND listener instead",
			cfg.AuthMode)
	}

	// Legacy HTTP transport policy. Default is "warn" in dev so the
	// legacy path keeps working with a visible deprecation log; in
	// production (ENVIRONMENT=prod) the default flips to "deny" so a
	// prod server using MCP_TRANSPORT=http refuses to start without
	// an explicit MCP_HTTP_LEGACY_POLICY=allow acknowledgement. This
	// matches the streamable_http fail-closed guard above. See
	// docs/adr/0014-prod-fail-closed-defaults.md.
	cfg.HTTPLegacyPolicy = strings.TrimSpace(os.Getenv("MCP_HTTP_LEGACY_POLICY"))
	if cfg.HTTPLegacyPolicy == "" {
		if os.Getenv("ENVIRONMENT") == "prod" {
			cfg.HTTPLegacyPolicy = "deny"
		} else {
			cfg.HTTPLegacyPolicy = "warn"
		}
	}
	switch cfg.HTTPLegacyPolicy {
	case "allow", "warn", "deny":
	default:
		return Config{}, fmt.Errorf("invalid MCP_HTTP_LEGACY_POLICY %q: must be \"allow\", \"warn\", or \"deny\"", cfg.HTTPLegacyPolicy)
	}

	// Production-strict enforcement
	if os.Getenv("ENVIRONMENT") == "prod" {
		if !strings.HasPrefix(cfg.ControlPlaneDSN, "postgres://") {
			return Config{}, fmt.Errorf("in production (ENVIRONMENT=prod), MCP_CONTROL_PLANE_DSN must be a postgres:// URI")
		}
		if os.Getenv("MCP_ALLOW_DEV_BACKEND") == "1" {
			return Config{}, fmt.Errorf("in production (ENVIRONMENT=prod), MCP_ALLOW_DEV_BACKEND=1 is prohibited")
		}
	}

	return cfg, nil
}

func (c Config) Fingerprint() map[string]any {
	return map[string]any{
		"transport":                     c.Transport,
		"auth_mode":                     c.AuthMode,
		"http_bind":                     c.HTTPBind,
		"grpc_bind":                     c.GRPCBind,
		"metrics_bind":                  c.MetricsBind,
		"metrics_auth_mode":             c.MetricsAuthMode,
		"clockify_base_url":             c.BaseURL,
		"workspace_id":                  c.WorkspaceID,
		"timezone":                      c.Timezone,
		"policy_claim_tenant":           c.TenantClaim,
		"policy_claim_subject":          c.SubjectClaim,
		"default_tenant_id":             c.DefaultTenantID,
		"control_plane_dsn":             sanitizeDSNForFingerprint(c.ControlPlaneDSN),
		"session_ttl":                   c.SessionTTL.String(),
		"allow_any_origin":              c.AllowAnyOrigin,
		"strict_host_check":             c.StrictHostCheck,
		"max_message_size":              c.MaxMessageSize,
		"tool_timeout":                  c.ToolTimeout.String(),
		"max_inflight_tool_calls":       c.MaxInFlightToolCalls,
		"report_max_entries":            c.ReportMaxEntries,
		"audit_durability_mode":         c.AuditDurabilityMode,
		"http_legacy_policy":            c.HTTPLegacyPolicy,
		"http_inline_metrics_enabled":   c.HTTPInlineMetricsEnabled,
		"http_inline_metrics_auth_mode": c.HTTPInlineMetricsAuthMode,
	}
}

// ValidateBaseURLOptions controls how ValidateBaseURL interprets a
// candidate Clockify API URL. Hosted reflects whether the deployment
// is one of the multi-tenant profiles (shared-service, prod-postgres);
// hosted mode demands HTTPS unconditionally — no loopback bypass and
// no CLOCKIFY_INSECURE escape — so a tenant that supplies an http://
// or loopback baseURL via control-plane credentials cannot smuggle
// cleartext traffic through a production gateway. AllowInsecure
// honours the operator's CLOCKIFY_INSECURE flag for self-hosted
// deployments where the operator owns both ends of the wire.
type ValidateBaseURLOptions struct {
	Hosted        bool
	AllowInsecure bool
}

// ValidateBaseURL enforces the URL-safety contract for Clockify base
// URLs across config-load and per-tenant credential resolution. Same
// behaviour for self-hosted profiles as the historical helper; hosted
// profiles refuse anything that isn't HTTPS regardless of loopback or
// the insecure escape hatch.
func ValidateBaseURL(raw string, opts ValidateBaseURLOptions) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid CLOCKIFY_BASE_URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("CLOCKIFY_BASE_URL must be an absolute URL")
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme != "http" {
		return fmt.Errorf("unsupported CLOCKIFY_BASE_URL scheme: %s", u.Scheme)
	}
	if opts.Hosted {
		return fmt.Errorf("CLOCKIFY_BASE_URL must use https under a hosted profile (no loopback or insecure bypass)")
	}
	if isLoopbackHost(u.Hostname()) || opts.AllowInsecure {
		return nil
	}
	return fmt.Errorf("insecure CLOCKIFY_BASE_URL requires loopback host or CLOCKIFY_INSECURE=1")
}

// validateBaseURL preserves the legacy two-arg signature used by the
// pre-export call sites and tests; new code should call
// ValidateBaseURL with explicit options.
func validateBaseURL(raw string, insecure bool) error {
	return ValidateBaseURL(raw, ValidateBaseURLOptions{AllowInsecure: insecure})
}

// parseCIDRList parses a comma-separated CIDR list (e.g.
// "10.0.0.0/8, 172.16.0.0/12, fd00::/8") into a slice of *net.IPNet.
// Empty input yields an empty slice. Whitespace around commas is
// trimmed; bare IPs without a prefix length are not accepted —
// operators must be explicit ("10.0.0.5/32"). Used by
// MCP_FORWARD_AUTH_TRUSTED_PROXIES and MCP_GRPC_PEER_CIDR_ALLOW.
func parseCIDRList(raw string) ([]*net.IPNet, error) {
	parts := strings.Split(raw, ",")
	out := make([]*net.IPNet, 0, len(parts))
	for _, part := range parts {
		entry := strings.TrimSpace(part)
		if entry == "" {
			continue
		}
		_, ipNet, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q: %w", entry, err)
		}
		out = append(out, ipNet)
	}
	return out, nil
}

func loopbackTrustedProxies() []*net.IPNet {
	nets, err := parseCIDRList("127.0.0.0/8,::1/128")
	if err != nil {
		panic(err)
	}
	return nets
}

func isLoopbackHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

// isLoopbackBind reports whether a Go HTTP bind address resolves to the
// loopback interface. The empty host (":8080") and "0.0.0.0"/"::"
// bind to ALL interfaces and are NOT loopback. Used by the strict
// host-check preflight in Load() so a strict policy with no allowlist
// is only acceptable when the listener is unreachable from off-host.
func isLoopbackBind(bind string) bool {
	host, _, err := net.SplitHostPort(bind)
	if err != nil {
		// Falls through for malformed binds; the listener layer will
		// reject them with a clearer error than this preflight would.
		return false
	}
	if host == "" {
		return false
	}
	return isLoopbackHost(host)
}

func bindForForwardAuth(cfg Config) string {
	if cfg.Transport == "grpc" {
		return cfg.GRPCBind
	}
	return cfg.HTTPBind
}

func bindEnvForForwardAuth(cfg Config) string {
	if cfg.Transport == "grpc" {
		return "MCP_GRPC_BIND"
	}
	return "MCP_HTTP_BIND"
}

func optionalBoolEnv(key string) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("invalid %s %q: must be a boolean", key, raw)
	}
	return value, nil
}

func nonNegativeInt(key, raw string, max int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", key, raw, err)
	}
	if value < 0 {
		return 0, fmt.Errorf("%s must be >= 0", key)
	}
	if max > 0 && value > max {
		return 0, fmt.Errorf("%s must be <= %d", key, max)
	}
	return value, nil
}

// sanitizeDSNForFingerprint returns a copy of the control-plane DSN
// safe for inclusion in the startup-log fingerprint. Postgres DSNs may
// carry a password in the URL userinfo
// (postgres://user:password@host/db); when present, the password is
// replaced with [REDACTED]. Dev backends (memory, file://) are returned
// unchanged because they carry no credential material.
func sanitizeDSNForFingerprint(dsn string) string {
	if dsn == "" {
		return dsn
	}
	if IsDevControlPlaneDSN(dsn) {
		return dsn
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return dsn
	}
	if u.User != nil {
		if _, hasPassword := u.User.Password(); hasPassword {
			u.User = url.UserPassword(u.User.Username(), "[REDACTED]")
		}
	}
	return u.String()
}
