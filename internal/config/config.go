package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
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

	// MCP transport
	Transport          string
	AuthMode           string
	HTTPBind           string
	GRPCBind           string
	BearerToken        string
	AllowedOrigins     []string
	AllowAnyOrigin     bool
	StrictHostCheck    bool
	MaxMessageSize     int64
	MetricsBind        string
	MetricsAuthMode    string
	MetricsBearerToken string

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
	// OIDCResourceURI is the canonical URI clients use to address this
	// MCP server. When set, every OIDC token must list this URI in its
	// audience claim — RFC 8707 / MCP OAuth 2.1 resource indicator
	// binding. Wired from MCP_RESOURCE_URI.
	OIDCResourceURI string
	// OIDCVerifyCacheTTL is the hard ceiling on cached OIDC verify
	// results. Larger values amortise the per-request verify cost but
	// extend the window before a revoked token is re-checked. Zero
	// selects the conservative 60s default baked into authn; values
	// outside [1s, 5m] are clamped at the authn layer. Wired from
	// MCP_OIDC_VERIFY_CACHE_TTL.
	OIDCVerifyCacheTTL   time.Duration
	ForwardTenantHeader  string
	ForwardSubjectHeader string
	MTLSTenantHeader     string

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

	// AuditDurabilityMode controls behavior when audit persistence fails for
	// a successful non-read-only tool call.
	// "best_effort" (default): log + metric; the call still reports success.
	// "fail_closed": the call returns an error so the client knows the audit
	// trail is incomplete. The mutation already happened; this prevents
	// silent untracked mutations.
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
	// MTLSCACertPath is the path to the CA cert for client certificate
	// verification. Required when MCP_AUTH_MODE=mtls on gRPC.
	MTLSCACertPath string
}

func Load() (Config, error) {
	cfg := Config{
		APIKey:      os.Getenv("CLOCKIFY_API_KEY"),
		WorkspaceID: os.Getenv("CLOCKIFY_WORKSPACE_ID"),
		BaseURL:     strings.TrimRight(os.Getenv("CLOCKIFY_BASE_URL"), "/"),
		Insecure:    os.Getenv("CLOCKIFY_INSECURE") == "1",
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}

	cfg.RequestTimeout = 30 * time.Second
	cfg.MaxRetries = 3

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
	if cfg.APIKey != "" {
		if err := validateBaseURL(cfg.BaseURL, cfg.Insecure); err != nil {
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
	cfg.ForwardTenantHeader = strings.TrimSpace(os.Getenv("MCP_FORWARD_TENANT_HEADER"))
	cfg.ForwardSubjectHeader = strings.TrimSpace(os.Getenv("MCP_FORWARD_SUBJECT_HEADER"))
	cfg.MTLSTenantHeader = strings.TrimSpace(os.Getenv("MCP_MTLS_TENANT_HEADER"))
	if cfg.AuthMode == "oidc" && cfg.Transport == "streamable_http" && cfg.OIDCIssuer == "" {
		return Config{}, fmt.Errorf("MCP_OIDC_ISSUER is required when MCP_TRANSPORT=streamable_http and MCP_AUTH_MODE=oidc")
	}
	// Fail-closed dev-backend guard for streamable_http. A dev DSN
	// (memory/file/bare path) cannot back a multi-process deployment
	// correctly — session state, audit events, and rate-limit counters
	// diverge across replicas. Require an explicit acknowledgement so
	// an operator who wants the single-process path knows they are on
	// it. See docs/adr/0014-prod-fail-closed-defaults.md.
	//
	// Belt + suspenders: runtime.BuildStore repeats the same check so
	// a caller that bypasses Load() (e.g. a custom wiring path) still
	// refuses to start against a dev DSN.
	if cfg.Transport == "streamable_http" &&
		IsDevControlPlaneDSN(cfg.ControlPlaneDSN) &&
		os.Getenv("MCP_ALLOW_DEV_BACKEND") != "1" {
		return Config{}, fmt.Errorf(
			"MCP_TRANSPORT=streamable_http with MCP_CONTROL_PLANE_DSN=%q (dev backend) is disallowed by default; set MCP_ALLOW_DEV_BACKEND=1 to acknowledge the single-process limits, or point MCP_CONTROL_PLANE_DSN at a production backend (postgres://...)",
			cfg.ControlPlaneDSN)
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
	strictHostCheck, err := optionalBoolEnv("MCP_STRICT_HOST_CHECK")
	if err != nil {
		return Config{}, err
	}
	cfg.StrictHostCheck = strictHostCheck

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
	cfg.MTLSCACertPath = os.Getenv("MCP_MTLS_CA_CERT_PATH")
	if (cfg.GRPCTLSCert == "") != (cfg.GRPCTLSKey == "") {
		return Config{}, fmt.Errorf("MCP_GRPC_TLS_CERT and MCP_GRPC_TLS_KEY must both be set or both empty")
	}

	if v := os.Getenv("MCP_GRPC_REAUTH_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid MCP_GRPC_REAUTH_INTERVAL %q: %w", v, err)
		}
		cfg.GRPCReauthInterval = d
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
	case "best_effort", "fail_closed":
	default:
		return Config{}, fmt.Errorf("invalid MCP_AUDIT_DURABILITY %q: must be \"best_effort\" or \"fail_closed\"", cfg.AuditDurabilityMode)
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
		"control_plane_dsn":             c.ControlPlaneDSN,
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

func validateBaseURL(raw string, insecure bool) error {
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
	if isLoopbackHost(u.Hostname()) || insecure {
		return nil
	}
	return fmt.Errorf("insecure CLOCKIFY_BASE_URL requires loopback host or CLOCKIFY_INSECURE=1")
}

func isLoopbackHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
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
