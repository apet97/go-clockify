// Package runtime holds the shared production wiring for the one-user
// Clockify MCP server. cmd/clockify-mcp/main.go and the live test harness
// both build their *mcp.Server through BuildServer so there is exactly one
// place that decides the toolset filter, advertised-tool enforcement, audit
// logging, rate limits, and resource/notifier wiring. Keeping that wiring in
// one function means tests exercise the same server the binary serves, not a
// hand-rolled approximation.
//
// This package may import internal/{clockify,config,mcp,tools,safety,logging}.
// It must never pull in controlplane/oidc/grpc/vault/policy/postgres/auth
// dependencies: cmd/clockify-mcp depends on it, and that command's dependency
// purity is a release gate.
package runtime

import (
	"fmt"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/config"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/safety"
	"github.com/apet97/go-clockify/internal/tools"
)

// BuiltServer bundles a fully-wired one-user MCP server with the underlying
// service and the resources that must be released when the process (or test)
// is done with it. Callers must invoke Close exactly once, typically via
// defer, after Server.Run returns.
type BuiltServer struct {
	// Server is the wired MCP stdio server, ready for Server.Run.
	Server *mcp.Server
	// Service is the underlying tools.Service, exposed so callers (notably
	// the live test harness) can make direct, out-of-band Clockify calls
	// for setup, cleanup, and orthogonal assertions.
	Service *tools.Service

	client      *clockify.Client
	auditLogger *mcp.AuditLogger
}

// Close releases the Clockify client and, if configured, the audit logger.
// It is safe to call once; an error closing the audit logger is returned so
// callers can decide whether to surface it.
func (b *BuiltServer) Close() error {
	if b == nil {
		return nil
	}
	var err error
	if b.auditLogger != nil {
		err = b.auditLogger.Close()
	}
	if b.client != nil {
		b.client.Close()
	}
	return err
}

// BuildServer constructs the production one-user MCP server from a loaded
// config and a resolved version string. It is the single source of truth for
// the server wiring previously inlined in cmd/clockify-mcp/main.go: client
// construction, the toolset filter, advertised-tool enforcement, the
// confirmation store, risk rate limits, audit logging, size limits, and the
// resource/notifier hookup.
//
// On success the caller owns the returned *BuiltServer and must call Close
// when done. On error nothing is leaked: any partially-opened client is
// closed before returning.
func BuildServer(cfg config.OneUserConfig, version string) (*BuiltServer, error) {
	client := NewClockifyClient(cfg, 2)
	client.SetUserAgent("clockify-mcp-one-user/" + version)
	client.SetCircuitBreaker(cfg.CircuitBreaker)

	service := tools.New(client, cfg.WorkspaceID)
	rawToolsEnabled := cfg.EnableRawTools || cfg.Toolset == "all"
	service.EnableRawWrites = cfg.EnableRawWrites
	service.EnableRawTools = rawToolsEnabled
	service.EnableRawGet = rawToolsEnabled && (cfg.EnableRawGet || cfg.Toolset == "all")
	service.RawWriteDocumentedOnly = cfg.RawWriteDocumentedOnly
	service.Toolset = cfg.Toolset
	service.ToolRateLimitDisabled = cfg.ToolRateLimitDisabled
	service.ToolRateLimits = map[string]int{
		"read":          cfg.ReadRateLimitPerMinute,
		"write":         cfg.WriteRateLimitPerMinute,
		"billing_admin": cfg.BillingAdminRateLimitPerMinute,
		"destructive":   cfg.DestructiveRateLimitPerMinute,
	}
	service.ConfirmationMode = "required"
	service.WebhookValidateDNS = true
	service.WebhookAllowedDomains = cfg.WebhookAllowedDomains
	if cfg.Timezone != "" {
		loc, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			client.Close()
			return nil, err
		}
		service.DefaultTimezone = loc
	}

	fullRegistry, err := service.FullAccessRegistryChecked()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("invalid full tool registry: %w", err)
	}
	advertisedRegistry := service.RegistryForToolset(cfg.Toolset)
	if rawToolsEnabled && cfg.Toolset != "all" {
		advertisedRegistry = AppendRawDescriptors(advertisedRegistry, fullRegistry)
	}
	if err := tools.ValidateRegistry(advertisedRegistry); err != nil {
		client.Close()
		return nil, fmt.Errorf("invalid advertised tool registry: %w", err)
	}

	server := mcp.NewServer(version, fullRegistry)
	server.SetAdvertisedTools(advertisedRegistry)
	server.EnforceAdvertisedTools = cfg.Toolset != "all"
	server.ConfirmationStore = safety.NewTokenStore(safety.TokenStoreOptions{})
	server.WorkspaceIDForSafety = cfg.WorkspaceID
	server.ConfirmationMode = "required"
	server.StaticToolList = true
	server.MaxInFlightToolCalls = cfg.MaxInFlightToolCalls
	server.ConfigureRiskRateLimits(mcp.RiskRateLimits{
		ReadPerMinute:         cfg.ReadRateLimitPerMinute,
		WritePerMinute:        cfg.WriteRateLimitPerMinute,
		BillingAdminPerMinute: cfg.BillingAdminRateLimitPerMinute,
		DestructivePerMinute:  cfg.DestructiveRateLimitPerMinute,
	})

	auditLogger, err := mcp.NewAuditLogger(cfg.AuditLogPath, mcp.AuditLogMode(cfg.AuditLogMode), cfg.WorkspaceID)
	if err != nil {
		client.Close()
		return nil, err
	}
	if auditLogger != nil {
		server.AuditLogger = auditLogger
		service.AuditLogPath = cfg.AuditLogPath
	}

	server.ToolTimeout = cfg.ToolTimeout
	server.MaxMessageSize = cfg.MaxMessageSize
	server.MaxToolResultBytes = cfg.MaxToolResultBytes
	server.ResourceProvider = service
	service.EmitResourceUpdate = server.NotifyResourceUpdated
	service.EmitResourceListChanged = server.NotifyResourcesListChanged
	service.SubscriptionGate = server.HasResourceSubscription
	service.Notifier = server

	return &BuiltServer{
		Server:      server,
		Service:     service,
		client:      client,
		auditLogger: auditLogger,
	}, nil
}

// NewClockifyClient builds the Clockify HTTP client from the one-user config.
// maxRetries is configurable so the doctor path can opt out of retries while
// the server path retries transient failures.
func NewClockifyClient(cfg config.OneUserConfig, maxRetries int) *clockify.Client {
	timeout := cfg.ToolTimeout
	if timeout <= 0 {
		timeout = config.DefaultToolTimeout
	}
	client := clockify.NewClient(cfg.APIKey, cfg.BaseURL, timeout, maxRetries)
	responseCap := max(cfg.MaxToolResultBytes, clockify.DefaultMaxResponseBodyBytes)
	client.SetMaxResponseBodyBytes(responseCap)
	return client
}

// AppendRawDescriptors adds the raw API fallback descriptors
// (clockify_api_get, clockify_api_request) to the advertised surface when raw
// tools are enabled outside toolset=all. It preserves registry order and
// never duplicates a descriptor already present.
func AppendRawDescriptors(advertised, full []mcp.ToolDescriptor) []mcp.ToolDescriptor {
	seen := make(map[string]bool, len(advertised)+2)
	for _, descriptor := range advertised {
		seen[descriptor.Tool.Name] = true
	}
	out := append([]mcp.ToolDescriptor(nil), advertised...)
	for _, descriptor := range full {
		switch descriptor.Tool.Name {
		case "clockify_api_get", "clockify_api_request":
			if !seen[descriptor.Tool.Name] {
				out = append(out, descriptor)
				seen[descriptor.Tool.Name] = true
			}
		}
	}
	return out
}
