package config

// EnvSpec is the single source of truth for every environment variable the
// server honours. Help text (cmd/clockify-mcp/help_generated.go), the README
// essentials table, the config-doc-parity CI gate, and the default-parity
// test all derive from AllSpecs(). Adding a new Getenv in config.go without
// an entry here trips TestEnvSpec_CoversEveryGetenv.
type EnvSpec struct {
	Name         string
	Group        string
	Default      string
	Help         string
	Enum         []string
	AppliesTo    []string
	Deprecated   bool
	Replacement  string
	EssentialDoc bool
}

// AllSpecs returns the registry in declaration order. Order drives help
// output grouping; the README renderer sorts alphabetically by Name.
func AllSpecs() []EnvSpec {
	return []EnvSpec{
		// --- Core ---
		{Name: "CLOCKIFY_API_KEY", Group: "Core", Help: "API key (required for stdio/http/grpc; optional for streamable_http)", EssentialDoc: true},
		{Name: "CLOCKIFY_WORKSPACE_ID", Group: "Core", Default: "auto", Help: "Workspace ID (auto-detected if only one)", EssentialDoc: true},
		{Name: "CLOCKIFY_BASE_URL", Group: "Core", Default: "https://api.clockify.me/api/v1", Help: "API base URL; HTTPS required unless loopback or CLOCKIFY_INSECURE=1"},
		{Name: "CLOCKIFY_TIMEZONE", Group: "Core", Help: "IANA timezone for time parsing"},
		{Name: "CLOCKIFY_INSECURE", Group: "Core", Enum: []string{"0", "1"}, Default: "0", Help: "Allow non-HTTPS base URLs"},
		{Name: "CLOCKIFY_SANITIZE_UPSTREAM_ERRORS", Group: "Core", Enum: []string{"0", "1"}, Default: "0", Help: "When 1, omit upstream Clockify response bodies from MCP tool-error responses (still logged server-side). Hosted profiles (shared-service, prod-postgres) default to 1."},
		{Name: "CLOCKIFY_WEBHOOK_VALIDATE_DNS", Group: "Core", Enum: []string{"0", "1"}, Default: "0", Help: "When 1, CreateWebhook/UpdateWebhook resolve the webhook host via DNS and reject any reply with a private/reserved IP (SSRF protection). Hosted profiles (shared-service, prod-postgres) default to 1."},
		{Name: "CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS", Group: "Core", Help: "Comma-separated escape-hatch list of webhook hostnames that bypass the CLOCKIFY_WEBHOOK_VALIDATE_DNS private-IP check. Each entry matches exact (webhook.example.com) or as a leading-dot suffix (.example.com matches every subdomain but anchors at a full DNS label). Use case: split-horizon DNS where a known-trusted hostname resolves to a private IP only on the control-plane network."},

		// --- Safety ---
		{Name: "CLOCKIFY_POLICY", Group: "Safety", Enum: []string{"read_only", "time_tracking_safe", "safe_core", "standard", "full"}, Default: "standard", Help: "Tool-access policy tier", EssentialDoc: true},
		{Name: "CLOCKIFY_DRY_RUN", Group: "Safety", Default: "enabled", Help: "Enable dry-run preview support for destructive tools when callers pass dry_run:true", EssentialDoc: true},
		{Name: "CLOCKIFY_DEDUPE_MODE", Group: "Safety", Enum: []string{"warn", "block", "off"}, Default: "warn", Help: "Duplicate entry detection", EssentialDoc: true},
		{Name: "CLOCKIFY_DEDUPE_LOOKBACK", Group: "Safety", Default: "25", Help: "Recent entries to scan for duplicates"},
		{Name: "CLOCKIFY_OVERLAP_CHECK", Group: "Safety", Default: "true", Help: "Overlapping entry detection"},
		{Name: "CLOCKIFY_DENY_TOOLS", Group: "Safety", Help: "Comma-separated tool names to block"},
		{Name: "CLOCKIFY_DENY_GROUPS", Group: "Safety", Help: "Comma-separated groups to block"},
		{Name: "CLOCKIFY_ALLOW_GROUPS", Group: "Safety", Help: "Comma-separated groups to allow"},

		// --- Performance ---
		{Name: "CLOCKIFY_MAX_CONCURRENT", Group: "Performance", Default: "10", Help: "Concurrent tool-call limit (0=disabled)"},
		{Name: "CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT", Group: "Performance", Default: "100ms", Help: "Wait-for-slot timeout [1ms,30s]"},
		{Name: "CLOCKIFY_RATE_LIMIT", Group: "Performance", Default: "120", Help: "Tool calls per 60s window (0=disabled)", EssentialDoc: true},
		{Name: "CLOCKIFY_TOKEN_BUDGET", Group: "Performance", Default: "8000", Help: "Response token budget (0=disabled)"},
		{Name: "CLOCKIFY_TOOL_TIMEOUT", Group: "Performance", Default: "45s", Help: "Per-tool deadline [5s,10m]"},
		{Name: "MCP_MAX_INFLIGHT_TOOL_CALLS", Group: "Performance", Default: "64", Help: "Stdio dispatch goroutine cap (0=disabled)"},
		{Name: "CLOCKIFY_REPORT_MAX_ENTRIES", Group: "Performance", Default: "10000", Help: "Hard cap on aggregated report entries (0=unbounded)"},
		{Name: "CLOCKIFY_SUBJECT_IDLE_TTL", Group: "Performance", Default: "1h", Help: "Idle cutoff for per-subject rate limiter reap"},
		{Name: "CLOCKIFY_SUBJECT_SWEEP_INTERVAL", Group: "Performance", Default: "5m", Help: "Reaper sweep frequency"},
		{Name: "CLOCKIFY_DELTA_FORMAT", Group: "Performance", Enum: []string{"merge", "jsonpatch"}, Default: "merge", Help: "Resource-notification diff algorithm"},

		// --- Bootstrap ---
		{Name: "CLOCKIFY_BOOTSTRAP_MODE", Group: "Bootstrap", Enum: []string{"full_tier1", "minimal", "custom"}, Default: "full_tier1", Help: "Initial tool surface", EssentialDoc: true},
		{Name: "CLOCKIFY_BOOTSTRAP_TOOLS", Group: "Bootstrap", Help: "Tool list for custom bootstrap mode"},

		// --- Transport ---
		{Name: "MCP_TRANSPORT", Group: "Transport", Enum: []string{"stdio", "http", "streamable_http", "grpc"}, Default: "stdio", Help: "Transport mode; http is legacy POST-only (deprecated)", EssentialDoc: true},
		{Name: "MCP_HTTP_BIND", Group: "Transport", Default: ":8080", AppliesTo: []string{"http", "streamable_http"}, Help: "HTTP listen address", EssentialDoc: true},
		{Name: "MCP_GRPC_BIND", Group: "Transport", Default: ":9090", AppliesTo: []string{"grpc"}, Help: "gRPC listen address (requires -tags=grpc)", EssentialDoc: true},
		{Name: "MCP_MAX_MESSAGE_SIZE", Group: "Transport", Default: "4194304", Help: "Max request size in bytes (primary knob); 0 < N <= 104857600", EssentialDoc: true},
		{Name: "MCP_HTTP_MAX_BODY", Group: "Transport", Default: "4194304", Help: "Deprecated alias for MCP_MAX_MESSAGE_SIZE", Deprecated: true, Replacement: "MCP_MAX_MESSAGE_SIZE", EssentialDoc: true},
		{Name: "MCP_ALLOWED_ORIGINS", Group: "Transport", AppliesTo: []string{"http", "streamable_http"}, Help: "Comma-separated CORS origins"},
		{Name: "MCP_ALLOW_ANY_ORIGIN", Group: "Transport", Enum: []string{"0", "1"}, Default: "0", Help: "Allow all origins"},
		{Name: "MCP_STRICT_HOST_CHECK", Group: "Transport", Enum: []string{"0", "1"}, Default: "0", Help: "Require Host header match"},
		{Name: "MCP_HTTP_LEGACY_POLICY", Group: "Transport", Enum: []string{"allow", "warn", "deny"}, Default: "warn", Help: "Legacy HTTP startup behaviour (defaults to deny when ENVIRONMENT=prod)", EssentialDoc: true},

		// --- Auth ---
		{Name: "MCP_AUTH_MODE", Group: "Auth", Enum: []string{"static_bearer", "oidc", "forward_auth", "mtls"}, Help: "Authentication mode (per-transport support varies; see matrix)", EssentialDoc: true},
		{Name: "MCP_BEARER_TOKEN", Group: "Auth", Help: "Bearer token (>=16 chars) for static_bearer"},
		{Name: "MCP_OIDC_ISSUER", Group: "Auth", Help: "OIDC issuer URL (required for streamable_http + oidc)"},
		{Name: "MCP_OIDC_AUDIENCE", Group: "Auth", Help: "Optional OIDC audience"},
		{Name: "MCP_OIDC_JWKS_URL", Group: "Auth", Help: "Optional JWKS URL override"},
		{Name: "MCP_OIDC_JWKS_PATH", Group: "Auth", Help: "Local JWKS file (tests/dev only)"},
		{Name: "MCP_OIDC_VERIFY_CACHE_TTL", Group: "Auth", Default: "60s", Help: "OIDC verify cache TTL [1s,5m]", EssentialDoc: true},
		{Name: "MCP_OIDC_STRICT", Group: "Auth", Enum: []string{"0", "1"}, Default: "0", Help: "When 1, fail config load if oidc selected without MCP_OIDC_AUDIENCE or MCP_RESOURCE_URI; reject tokens missing exp claim"},
		{Name: "MCP_REQUIRE_TENANT_CLAIM", Group: "Auth", Enum: []string{"0", "1"}, Default: "0", Help: "When 1, oidc tokens missing the tenant claim are rejected (no fallback to MCP_DEFAULT_TENANT_ID)"},
		{Name: "MCP_RESOURCE_URI", Group: "Auth", Help: "RFC 8707 resource indicator"},
		{Name: "MCP_TENANT_CLAIM", Group: "Auth", Default: "tenant_id", Help: "OIDC claim name for tenant"},
		{Name: "MCP_SUBJECT_CLAIM", Group: "Auth", Default: "sub", Help: "OIDC claim name for subject"},
		{Name: "MCP_DEFAULT_TENANT_ID", Group: "Auth", Default: "default", Help: "Fallback tenant for non-OIDC modes"},
		{Name: "MCP_FORWARD_TENANT_HEADER", Group: "Auth", Default: "X-Forwarded-Tenant", Help: "forward_auth tenant header"},
		{Name: "MCP_FORWARD_SUBJECT_HEADER", Group: "Auth", Default: "X-Forwarded-User", Help: "forward_auth subject header"},
		{Name: "MCP_MTLS_TENANT_HEADER", Group: "Auth", Default: "X-Tenant-ID", Help: "mTLS tenant header override (only consulted when MCP_MTLS_TENANT_SOURCE selects header)"},
		{Name: "MCP_MTLS_TENANT_SOURCE", Group: "Auth", Enum: []string{"cert", "header", "header_or_cert"}, Default: "cert", Help: "Where the mtls authenticator reads the tenant from: cert SAN/Org (default; safe for direct mTLS), header (only behind a trusted proxy), header_or_cert (migration window)"},
		{Name: "MCP_REQUIRE_MTLS_TENANT", Group: "Auth", Enum: []string{"0", "1"}, Default: "0", Help: "When 1, reject mtls clients whose configured tenant source yields no tenant (no fallback to MCP_DEFAULT_TENANT_ID)"},
		{Name: "MCP_GRPC_TLS_CERT", Group: "Auth", AppliesTo: []string{"grpc"}, Help: "gRPC TLS server cert path (required for grpc + mtls)"},
		{Name: "MCP_GRPC_TLS_KEY", Group: "Auth", AppliesTo: []string{"grpc"}, Help: "gRPC TLS server key path (required for grpc + mtls)"},
		{Name: "MCP_HTTP_TLS_CERT", Group: "Auth", AppliesTo: []string{"streamable_http"}, Help: "Streamable HTTP TLS server cert path (required for streamable_http + mtls; enables HTTPS when set)"},
		{Name: "MCP_HTTP_TLS_KEY", Group: "Auth", AppliesTo: []string{"streamable_http"}, Help: "Streamable HTTP TLS server key path (required for streamable_http + mtls)"},
		{Name: "MCP_MTLS_CA_CERT_PATH", Group: "Auth", Help: "Client CA bundle for mtls verification (required for grpc + mtls and streamable_http + mtls)"},
		{Name: "MCP_DISABLE_INLINE_SECRETS", Group: "Auth", Enum: []string{"0", "1"}, Default: "0", Help: "When 1, reject credential refs with backend=inline (hosted-service hardening; prefer env/file/external vault backends)"},
		{Name: "MCP_EXPOSE_AUTH_ERRORS", Group: "Auth", Enum: []string{"0", "1"}, Default: "0", Help: "When 1, expose detailed auth failure reasons to clients (development only); default 0 returns 'authentication failed' to clients while logging the underlying reason server-side."},
		{Name: "MCP_GRPC_REAUTH_INTERVAL", Group: "Auth", Default: "0", AppliesTo: []string{"grpc"}, Help: "gRPC stream reauth interval (0=disabled)"},

		// --- Metrics ---
		{Name: "MCP_METRICS_BIND", Group: "Metrics", Help: "Dedicated metrics listener (optional; recommended for streamable_http)", EssentialDoc: true},
		{Name: "MCP_METRICS_AUTH_MODE", Group: "Metrics", Enum: []string{"none", "static_bearer"}, Default: "static_bearer (when MCP_METRICS_BIND set)", Help: "Auth mode for dedicated metrics listener", EssentialDoc: true},
		{Name: "MCP_METRICS_BEARER_TOKEN", Group: "Metrics", Help: "Bearer token (>=16 chars) for static_bearer metrics", EssentialDoc: true},
		{Name: "MCP_HTTP_INLINE_METRICS_ENABLED", Group: "Metrics", Enum: []string{"0", "1"}, Default: "0", Help: "Expose /metrics on the main HTTP listener", EssentialDoc: true},
		{Name: "MCP_HTTP_INLINE_METRICS_AUTH_MODE", Group: "Metrics", Enum: []string{"inherit_main_bearer", "static_bearer", "none"}, Default: "inherit_main_bearer", Help: "Auth mode for inline /metrics", EssentialDoc: true},
		{Name: "MCP_HTTP_INLINE_METRICS_BEARER_TOKEN", Group: "Metrics", Help: "Separate bearer for inline /metrics when static_bearer"},

		// --- Control Plane / Audit ---
		{Name: "MCP_CONTROL_PLANE_DSN", Group: "ControlPlane", Default: "memory", Help: "Control-plane DSN: memory, file://<path>, postgres://...", EssentialDoc: true},
		{Name: "MCP_CONTROL_PLANE_AUDIT_CAP", Group: "ControlPlane", Default: "0", Help: "File/memory audit cap (0=unbounded). Postgres uses retention instead.", EssentialDoc: true},
		{Name: "MCP_CONTROL_PLANE_AUDIT_RETENTION", Group: "ControlPlane", Default: "720h", Help: "Audit retention [1h,8760h]; 0=off", EssentialDoc: true},
		{Name: "MCP_SESSION_TTL", Group: "ControlPlane", Default: "30m", AppliesTo: []string{"streamable_http"}, Help: "Session TTL [1m,24h]"},
		{Name: "MCP_ALLOW_DEV_BACKEND", Group: "ControlPlane", Enum: []string{"0", "1"}, Help: "Permit memory/file backends for streamable_http (single-process only)", EssentialDoc: true},
		{Name: "MCP_AUDIT_DURABILITY", Group: "Audit", Enum: []string{"best_effort", "fail_closed"}, Default: "best_effort", Help: "Audit persist-failure behaviour (defaults to fail_closed when ENVIRONMENT=prod)", EssentialDoc: true},

		// --- Logging / Deploy ---
		{Name: "MCP_LOG_LEVEL", Group: "Logging", Enum: []string{"debug", "info", "warn", "error"}, Default: "info", Help: "Log level"},
		{Name: "MCP_LOG_FORMAT", Group: "Logging", Enum: []string{"text", "json"}, Default: "text", Help: "Log format (stderr; PII-scrubbed)", EssentialDoc: true},
		{Name: "ENVIRONMENT", Group: "Deploy", Help: "Set to 'prod' to enforce postgres:// DSN"},

		// --- Profile ---
		{Name: "MCP_PROFILE", Group: "Profile", Enum: ProfileNames(),
			Help:         "Apply a bundle of pinned defaults for a named deployment shape; explicit env overrides still win",
			EssentialDoc: true},
	}
}
