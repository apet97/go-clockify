package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/config"
	logslog "github.com/apet97/go-clockify/internal/logging"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/tools"
)

// version is populated at build time via ldflags:
//
//	go build -ldflags "-X main.version=v1.2.3" ./cmd/clockify-mcp
var version = "dev"

func effectiveVersion() string {
	if version != "" && version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			return bi.Main.Version
		}
	}
	return version
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Println(effectiveVersion())
			os.Exit(0)
		case "--help", "-h":
			printHelp()
			os.Exit(0)
		case "doctor":
			os.Exit(runDoctor(os.Args[2:], os.Stdout, os.Stderr))
		}
	}

	// Configure log level. The stdio runtime defaults to warn so a clean
	// session emits nothing on stderr — MCP clients and contract auditors
	// treat stderr noise as a fault signal. Startup/config failures still
	// surface: they are written directly via writeStartupError, not slog.
	// Operators who want startup diagnostics set MCP_LOG_LEVEL=info|debug.
	rawLevel := os.Getenv("MCP_LOG_LEVEL")
	logLevel := parseLogLevel(rawLevel)
	if rawLevel != "" && !isKnownLogLevel(rawLevel) {
		fmt.Fprintf(os.Stderr, "warning: unknown MCP_LOG_LEVEL %q, defaulting to warn\n", rawLevel)
	}

	// Configure slog to stderr. The chosen handler is always wrapped in a
	// RedactingHandler so that any attribute matching a well-known secret
	// key (authorization, api_key, bearer, token, ...) is masked before it
	// reaches the underlying encoder. This is defence-in-depth; hot-path
	// code should still avoid logging secrets explicitly.
	var logHandler slog.Handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	logHandler = logslog.NewRedactingHandler(logHandler)
	slog.SetDefault(slog.New(logHandler))

	if err := run(); err != nil {
		writeStartupError(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return runWithContext(ctx, os.Stdin, os.Stdout)
}

// startupConfigError marks a failure that occurred while loading the
// one-user configuration (a fixable environment problem), as distinct
// from a runtime failure after the server started. main() uses it to
// decide whether to point the operator at `clockify-mcp doctor`.
type startupConfigError struct{ err error }

func (e *startupConfigError) Error() string { return e.err.Error() }
func (e *startupConfigError) Unwrap() error { return e.err }

// writeStartupError reports a startup failure on w. Configuration
// failures additionally suggest `clockify-mcp doctor`, mirroring the
// recovery line the doctor subcommand already prints; runtime failures
// are reported as-is.
func writeStartupError(w io.Writer, err error) {
	_, _ = fmt.Fprintf(w, "error: %v\n", err)
	var cfgErr *startupConfigError
	if errors.As(err, &cfgErr) {
		_, _ = fmt.Fprintln(w, "hint: run 'clockify-mcp doctor' to validate the one-user configuration")
	}
}

func runWithContext(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	cfg, err := config.LoadOneUser()
	if err != nil {
		return &startupConfigError{err}
	}
	effective := effectiveVersion()
	client := clockify.NewClient(cfg.APIKey, cfg.BaseURL, 30*time.Second, 2)
	client.SetUserAgent("clockify-mcp-one-user/" + effective)
	client.SetCircuitBreaker(cfg.CircuitBreaker)
	defer client.Close()

	service := tools.New(client, cfg.WorkspaceID)
	service.EnableRawWrites = cfg.EnableRawWrites
	service.RawWriteDocumentedOnly = cfg.RawWriteDocumentedOnly
	service.Toolset = cfg.Toolset
	service.WebhookValidateDNS = true
	service.WebhookAllowedDomains = cfg.WebhookAllowedDomains
	if cfg.Timezone != "" {
		loc, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			return err
		}
		service.DefaultTimezone = loc
	}
	fullRegistry := service.FullAccessRegistry()
	advertisedRegistry := service.RegistryForToolset(cfg.Toolset)
	server := mcp.NewServer(effective, fullRegistry)
	server.SetAdvertisedTools(advertisedRegistry)
	server.StaticToolList = true
	server.MaxInFlightToolCalls = cfg.MaxInFlightToolCalls
	server.ConfigureToolLimits(cfg.ToolRateLimitPerMinute)
	server.ToolTimeout = cfg.ToolTimeout
	server.MaxMessageSize = cfg.MaxMessageSize
	server.MaxToolResultBytes = cfg.MaxToolResultBytes
	server.ResourceProvider = service
	service.EmitResourceUpdate = server.NotifyResourceUpdated
	service.EmitResourceListChanged = server.NotifyResourcesListChanged
	service.SubscriptionGate = server.HasResourceSubscription
	service.Notifier = server

	slog.Info("one_user_server_start",
		"version", effective,
		"transport", "stdio",
		"workspace", cfg.WorkspaceID,
		"base_url", cfg.BaseURL,
		"toolset", cfg.Toolset,
	)

	return server.Run(ctx, stdin, stdout)
}

// parseLogLevel maps MCP_LOG_LEVEL to a slog level. An unset or unrecognised
// value resolves to warn: the stdio runtime keeps stderr quiet by default so a
// clean MCP session produces no diagnostic noise. Operators opt into info or
// debug explicitly.
func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}

func isKnownLogLevel(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug", "info", "warn", "warning", "error":
		return true
	}
	return false
}

func runDoctor(args []string, stdout, stderr io.Writer) int {
	live := false
	for _, arg := range args {
		switch arg {
		case "--live":
			live = true
		default:
			_, _ = fmt.Fprintf(stderr, "unknown doctor argument %q\n", arg)
			_, _ = fmt.Fprintln(stderr, "usage: clockify-mcp doctor [--live]")
			return 2
		}
	}
	cfg, err := config.LoadOneUser()
	if err != nil {
		_, _ = fmt.Fprintf(stdout, "clockify-mcp doctor - one-user configuration\n\n")
		_, _ = fmt.Fprintf(stdout, "Result: ERROR\n")
		_, _ = fmt.Fprintf(stdout, "Recovery: set CLOCKIFY_API_KEY and CLOCKIFY_WORKSPACE_ID for the one pinned workspace, then run doctor again.\n")
		_, _ = fmt.Fprintf(stderr, "config error: %v\n", err)
		return 2
	}
	client := clockify.NewClient(cfg.APIKey, cfg.BaseURL, 30*time.Second, 0)
	defer client.Close()
	service := tools.New(client, cfg.WorkspaceID)
	registryLoaded := len(service.FullAccessRegistry())
	advertisedSurface := len(service.RegistryForToolset(cfg.Toolset))

	_, _ = fmt.Fprintf(stdout, "clockify-mcp doctor - one-user configuration\n\n")
	_, _ = fmt.Fprintf(stdout, "CLOCKIFY_API_KEY       set (redacted)\n")
	_, _ = fmt.Fprintf(stdout, "CLOCKIFY_WORKSPACE_ID  %s\n", cfg.WorkspaceID)
	_, _ = fmt.Fprintf(stdout, "CLOCKIFY_TIMEZONE      %s\n", optionalValue(cfg.Timezone, "(local/default)"))
	_, _ = fmt.Fprintf(stdout, "CLOCKIFY_BASE_URL      %s\n", cfg.BaseURL)
	_, _ = fmt.Fprintf(stdout, "MCP_LOG_LEVEL          %s\n", optionalValue(cfg.LogLevel, "warn"))
	_, _ = fmt.Fprintf(stdout, "CLOCKIFY_TOOL_TIMEOUT  %s\n", cfg.ToolTimeout)
	_, _ = fmt.Fprintf(stdout, "CLOCKIFY_MAX_IN_FLIGHT_TOOL_CALLS  %d\n", cfg.MaxInFlightToolCalls)
	_, _ = fmt.Fprintf(stdout, "CLOCKIFY_MAX_MESSAGE_SIZE          %d\n", cfg.MaxMessageSize)
	_, _ = fmt.Fprintf(stdout, "CLOCKIFY_MAX_TOOL_RESULT_BYTES     %d\n", cfg.MaxToolResultBytes)
	_, _ = fmt.Fprintf(stdout, "CLOCKIFY_TOOL_RATE_LIMIT_PER_MINUTE %d\n", cfg.ToolRateLimitPerMinute)
	_, _ = fmt.Fprintf(stdout, "CLOCKIFY_TOOLSET                   %s\n", cfg.Toolset)
	_, _ = fmt.Fprintf(stdout, "Registry loaded:    %d tools\n", registryLoaded)
	_, _ = fmt.Fprintf(stdout, "Advertised surface: %d tools (toolset=%s)\n", advertisedSurface, cfg.Toolset)
	_, _ = fmt.Fprintf(stdout, "CLOCKIFY_ENABLE_RAW_WRITES         %t\n", cfg.EnableRawWrites)
	_, _ = fmt.Fprintf(stdout, "CLOCKIFY_RAW_WRITE_DOCUMENTED_ONLY %t\n", cfg.RawWriteDocumentedOnly)
	_, _ = fmt.Fprintf(stdout, "CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS   %s\n", optionalList(cfg.WebhookAllowedDomains, "(none)"))
	_, _ = fmt.Fprintf(stdout, "CLOCKIFY_CIRCUIT_BREAKER           %s\n", circuitBreakerStatus(cfg.CircuitBreaker.Enabled))
	_, _ = fmt.Fprintf(stdout, "CLOCKIFY_CIRCUIT_BREAKER_FAILURE_THRESHOLD  %d\n", cfg.CircuitBreaker.FailureThreshold)
	_, _ = fmt.Fprintf(stdout, "CLOCKIFY_CIRCUIT_BREAKER_OPEN_DURATION      %s\n", cfg.CircuitBreaker.OpenDuration)
	_, _ = fmt.Fprintf(stdout, "CLOCKIFY_CIRCUIT_BREAKER_HALF_OPEN_PROBES   %d\n", cfg.CircuitBreaker.HalfOpenProbes)
	if live {
		if code := runDoctorLive(cfg, stdout, stderr); code != 0 {
			return code
		}
	}
	_, _ = fmt.Fprintf(stdout, "\nResult: OK\n")
	return 0
}

func circuitBreakerStatus(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

func runDoctorLive(cfg config.OneUserConfig, stdout, stderr io.Writer) int {
	_, _ = fmt.Fprintf(stdout, "\nLive checks:\n")
	// Honour Ctrl+C / SIGTERM during the live round-trips, mirroring the
	// main server path; the tool timeout is layered on top.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, cfg.ToolTimeout)
	defer cancel()
	client := clockify.NewClient(cfg.APIKey, cfg.BaseURL, 30*time.Second, 0)
	client.SetUserAgent("clockify-mcp-one-user/" + effectiveVersion() + " doctor")
	defer client.Close()

	var user clockify.User
	if err := client.Get(ctx, "/user", nil, &user); err != nil {
		_, _ = fmt.Fprintf(stderr, "live auth check failed: %v\n", redactAPIKey(err, cfg.APIKey))
		return 3
	}
	_, _ = fmt.Fprintf(stdout, "  GET /user                         OK\n")
	_, _ = fmt.Fprintf(stdout, "    user_id: %s\n", optionalValue(user.ID, "(unknown)"))

	var workspace clockify.Workspace
	workspacePath := "/workspaces/" + cfg.WorkspaceID
	if err := client.Get(ctx, workspacePath, nil, &workspace); err != nil {
		_, _ = fmt.Fprintf(stderr, "workspace check failed: %v\n", redactAPIKey(err, cfg.APIKey))
		if apiStatus(err) == http.StatusUnauthorized || apiStatus(err) == http.StatusForbidden {
			return 3
		}
		return 4
	}
	_, _ = fmt.Fprintf(stdout, "  GET /workspaces/{workspaceId}     OK\n")
	_, _ = fmt.Fprintf(stdout, "    workspace_id: %s\n", optionalValue(workspace.ID, cfg.WorkspaceID))
	_, _ = fmt.Fprintf(stdout, "    workspace_name: %s\n", optionalValue(workspace.Name, "(unknown)"))
	_, _ = fmt.Fprintf(stdout, "    feature_plan: %s\n", optionalValue(workspace.FeatureSubscriptionType, "(unknown)"))
	_, _ = fmt.Fprintf(stdout, "    feature_flags: %s\n", optionalList(workspace.Features, "(none returned)"))
	_, _ = fmt.Fprintf(stdout, "    optional_features: %s\n", doctorFeatureSummary(workspace.Features))

	var ownerWorkspaces []clockify.Workspace
	if err := client.Get(ctx, "/workspaces", map[string]string{"roles": "OWNER"}, &ownerWorkspaces); err != nil {
		_, _ = fmt.Fprintf(stderr, "owner role check failed: %v\n", redactAPIKey(err, cfg.APIKey))
		if apiStatus(err) == http.StatusUnauthorized || apiStatus(err) == http.StatusForbidden {
			return 3
		}
		return 5
	}
	for _, ws := range ownerWorkspaces {
		if ws.ID == cfg.WorkspaceID {
			_, _ = fmt.Fprintf(stdout, "  GET /workspaces?roles=OWNER       OK (owner)\n")
			return 0
		}
	}
	// Best-effort workspace-admin probe. The owner probe above uses
	// roles=OWNER; WORKSPACE_ADMIN is the documented admin role filter. If
	// Clockify rejects this role value the call simply errors and doctor
	// degrades to the member_or_unknown verdict — the admin probe never fails
	// doctor.
	var adminWorkspaces []clockify.Workspace
	if err := client.Get(ctx, "/workspaces", map[string]string{"roles": "WORKSPACE_ADMIN"}, &adminWorkspaces); err == nil {
		for _, ws := range adminWorkspaces {
			if ws.ID == cfg.WorkspaceID {
				_, _ = fmt.Fprintf(stdout, "  GET /workspaces?roles=WORKSPACE_ADMIN OK (workspace_admin)\n")
				return 0
			}
		}
	}
	_, _ = fmt.Fprintf(stdout, "  GET /workspaces roles probe       OK with warning (member_or_unknown)\n")
	_, _ = fmt.Fprintf(stdout, "    note: this API key is neither the owner of the configured workspace\n")
	_, _ = fmt.Fprintf(stdout, "    nor confirmed as a workspace admin. Read and own-time-entry tools\n")
	_, _ = fmt.Fprintf(stdout, "    work; owner/admin-only operations may be denied by Clockify.\n")
	_, _ = fmt.Fprintf(stdout, "    Tool families that may be denied:\n")
	_, _ = fmt.Fprintf(stdout, "      admin    - clockify_users_role, clockify_users_deactivate, clockify_users_invite, clockify_groups_*\n")
	_, _ = fmt.Fprintf(stdout, "      billing  - clockify_invoices_*, clockify_expenses_* (workspace plan dependent)\n")
	_, _ = fmt.Fprintf(stdout, "      settings - clockify_workspace_settings (write paths)\n")
	return 0
}

func doctorFeatureSummary(features []string) string {
	status := doctorFeatureStatus(features)
	keys := []string{"invoices", "expenses", "timeOff", "scheduling", "approvals", "webhooks", "customFields", "reports"}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+status[key])
	}
	return strings.Join(parts, ",")
}

func doctorFeatureStatus(features []string) map[string]string {
	signals := map[string][]string{
		"invoices":     []string{"INVOICE", "INVOIC"},
		"expenses":     []string{"EXPENSE"},
		"timeOff":      []string{"TIME_OFF", "TIMEOFF", "TIME OFF"},
		"scheduling":   []string{"SCHEDULING"},
		"approvals":    []string{"APPROVAL"},
		"webhooks":     []string{"WEBHOOK"},
		"customFields": []string{"CUSTOM_FIELD", "CUSTOMFIELD"},
		"reports":      []string{"REPORT"},
	}
	out := make(map[string]string, len(signals))
	for key := range signals {
		out[key] = "unknown"
	}
	if len(features) == 0 {
		return out
	}
	for key := range signals {
		out[key] = "not_advertised"
	}
	for _, raw := range features {
		feature := strings.ToUpper(strings.TrimSpace(raw))
		for key, needles := range signals {
			for _, needle := range needles {
				if feature == needle || strings.HasPrefix(feature, needle+"_") {
					out[key] = "available"
				}
			}
		}
	}
	return out
}

func apiStatus(err error) int {
	var apiErr *clockify.APIError
	if errors.As(err, &apiErr) && apiErr != nil {
		return apiErr.StatusCode
	}
	return 0
}

func redactAPIKey(err error, apiKey string) string {
	msg := fmt.Sprint(err)
	apiKey = strings.TrimSpace(apiKey)
	if apiKey != "" {
		msg = strings.ReplaceAll(msg, apiKey, "[redacted]")
	}
	return msg
}

func optionalValue(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func optionalList(values []string, fallback string) string {
	if len(values) == 0 {
		return fallback
	}
	return strings.Join(values, ",")
}

// printHelp emits the one-user branch help banner.
func printHelp() {
	// Writes to stderr never fail in any actionable way at this call
	// site — if the OS-level write has gone south, help-text drops
	// are the least of our problems. Explicit _, _ = satisfies the
	// errcheck linter.
	w := os.Stderr
	_, _ = fmt.Fprintf(w, "clockify-mcp %s — one-user full-access Clockify MCP\n\n", effectiveVersion())
	_, _ = fmt.Fprintln(w, "Usage:")
	_, _ = fmt.Fprintln(w, "  clockify-mcp                 Start the stdio MCP server")
	_, _ = fmt.Fprintln(w, "  clockify-mcp doctor          Validate the one-user configuration")
	_, _ = fmt.Fprintln(w, "  clockify-mcp doctor --live   Validate config plus live auth/workspace/owner checks")
	_, _ = fmt.Fprintln(w, "  clockify-mcp --version | -v  Print version and exit")
	_, _ = fmt.Fprintln(w, "  clockify-mcp --help    | -h  Print this help and exit")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "Environment:")
	_, _ = fmt.Fprintln(w, "  CLOCKIFY_API_KEY       required")
	_, _ = fmt.Fprintln(w, "  CLOCKIFY_WORKSPACE_ID  required")
	_, _ = fmt.Fprintln(w, "  CLOCKIFY_TIMEZONE      optional")
	_, _ = fmt.Fprintln(w, "  CLOCKIFY_BASE_URL      optional, defaults to https://api.clockify.me/api/v1")
	_, _ = fmt.Fprintln(w, "  CLOCKIFY_TOOL_TIMEOUT  optional, defaults to 45s, allowed 5s..10m")
	_, _ = fmt.Fprintln(w, "  CLOCKIFY_MAX_IN_FLIGHT_TOOL_CALLS  optional, defaults to 4")
	_, _ = fmt.Fprintln(w, "  CLOCKIFY_MAX_MESSAGE_SIZE          optional, defaults to 4194304, allowed 1..104857600")
	_, _ = fmt.Fprintln(w, "  CLOCKIFY_MAX_TOOL_RESULT_BYTES     optional, defaults to 50000, allowed 1..104857600")
	_, _ = fmt.Fprintln(w, "  CLOCKIFY_TOOLSET                   optional: default, core, business, admin, all; defaults to default")
	_, _ = fmt.Fprintln(w, "  CLOCKIFY_ENABLE_RAW_WRITES         optional, defaults to false")
	_, _ = fmt.Fprintln(w, "  CLOCKIFY_RAW_WRITE_DOCUMENTED_ONLY optional, defaults to true")
	_, _ = fmt.Fprintln(w, "  CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS   optional comma-separated allowlist")
	_, _ = fmt.Fprintln(w, "  MCP_LOG_LEVEL          optional: debug, info, warn, error (default warn)")
}
