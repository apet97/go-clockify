package runtime

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/bootstrap"
	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/config"
	"github.com/apet97/go-clockify/internal/controlplane"
	"github.com/apet97/go-clockify/internal/dedupe"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/enforcement"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/ratelimit"
	"github.com/apet97/go-clockify/internal/tools"
	"github.com/apet97/go-clockify/internal/truncate"
	"github.com/apet97/go-clockify/internal/vault"
)

// runtimeDeps bundles config-derived state shared by every transport:
// rate limit, dedupe/dry-run/truncation knobs, policy + bootstrap, and
// the auditor hook that streamable_http attaches its control-plane
// audit sink to. The `version` field carries the build-time ldflag
// value so tenantRuntime can set a per-client User-Agent without
// reaching back into package main.
type runtimeDeps struct {
	cfg       config.Config
	dd        dedupe.Config
	dc        dryrun.Config
	tc        truncate.Config
	rl        *ratelimit.RateLimiter
	policy    *policy.Policy
	bootstrap bootstrap.Config
	auditor   mcp.Auditor
	version   string
}

type controlPlaneAuditor struct {
	store controlplane.Store
}

func (a controlPlaneAuditor) RecordAudit(event mcp.AuditEvent) error {
	if a.store == nil {
		return nil
	}
	tenantID := event.Metadata["tenant_id"]
	subject := event.Metadata["subject"]
	sessionID := event.Metadata["session_id"]
	transport := event.Metadata["transport"]
	return a.store.AppendAuditEvent(controlplane.AuditEvent{
		ID:          fmt.Sprintf("%d-%s-%s", time.Now().UnixNano(), sessionID, event.Tool),
		At:          time.Now().UTC(),
		TenantID:    tenantID,
		Subject:     subject,
		SessionID:   sessionID,
		Transport:   transport,
		Tool:        event.Tool,
		Action:      event.Action,
		Outcome:     event.Outcome,
		Phase:       event.Phase,
		Reason:      event.Reason,
		ResourceIDs: event.ResourceIDs,
		Metadata:    event.Metadata,
	})
}

func newService(client *clockify.Client, workspaceID string, timezone string, dd dedupe.Config, pol *policy.Policy, reportMaxEntries int) *tools.Service {
	service := tools.New(client, workspaceID)
	if timezone != "" {
		loc, _ := time.LoadLocation(timezone)
		service.DefaultTimezone = loc
	}
	service.DedupeConfig = &dd
	service.PolicyDescribe = pol.Describe
	service.ReportMaxEntries = reportMaxEntries
	return service
}

func buildServer(version string, deps runtimeDeps, service *tools.Service, pol *policy.Policy, bc *bootstrap.Config) *mcp.Server {
	registry := service.Registry()
	tier1Names := make(map[string]bool, len(registry))
	for _, d := range registry {
		tier1Names[d.Tool.Name] = true
	}
	pol.SetTier1Tools(tier1Names)
	bc.SetTier1Tools(tier1Names)
	pipeline := &enforcement.Pipeline{
		Policy:     pol,
		Bootstrap:  bc,
		RateLimit:  deps.rl,
		DryRun:     deps.dc,
		Truncation: deps.tc,
	}
	gate := &enforcement.Gate{
		Policy:    pol,
		Bootstrap: bc,
	}
	server := mcp.NewServer(version, registry, pipeline, gate)
	server.ToolTimeout = deps.cfg.ToolTimeout
	server.MaxInFlightToolCalls = deps.cfg.MaxInFlightToolCalls
	server.MaxMessageSize = deps.cfg.MaxMessageSize
	server.StrictHostCheck = deps.cfg.StrictHostCheck
	server.Auditor = deps.auditor
	server.AuditDurabilityMode = deps.cfg.AuditDurabilityMode
	server.ResourceProvider = service
	service.Notifier = server
	service.EmitResourceUpdate = server.NotifyResourceUpdated
	service.SubscriptionGate = server.HasResourceSubscription

	service.ActivateGroup = func(_ context.Context, group string) (tools.ActivationResult, error) {
		descriptors, ok := service.Tier2Handlers(group)
		if !ok {
			return tools.ActivationResult{}, fmt.Errorf("unknown group: %s", group)
		}
		if err := server.ActivateGroup(group, descriptors); err != nil {
			return tools.ActivationResult{}, err
		}
		return tools.ActivationResult{
			Kind:      "group",
			Name:      group,
			Group:     group,
			ToolCount: len(descriptors),
		}, nil
	}

	service.ActivateTool = func(_ context.Context, name string) (tools.ActivationResult, error) {
		if tier1Names[name] {
			if err := server.ActivateTier1Tool(name); err != nil {
				return tools.ActivationResult{}, err
			}
			return tools.ActivationResult{Kind: "tool", Name: name, ToolCount: 1}, nil
		}
		for groupName := range tools.Tier2Groups {
			descriptors, ok := service.Tier2Handlers(groupName)
			if !ok {
				continue
			}
			for _, d := range descriptors {
				if d.Tool.Name != name {
					continue
				}
				if err := server.ActivateGroup(groupName, descriptors); err != nil {
					return tools.ActivationResult{}, err
				}
				return tools.ActivationResult{
					Kind:      "tool",
					Name:      name,
					Group:     groupName,
					ToolCount: len(descriptors),
				}, nil
			}
		}
		return tools.ActivationResult{}, fmt.Errorf("unknown tool: %s", name)
	}

	return server
}

func bootstrapDefaultTenant(store controlplane.Store, cfg config.Config) error {
	if cfg.APIKey == "" || store == nil {
		return nil
	}
	if _, ok := store.Tenant(cfg.DefaultTenantID); ok {
		return nil
	}
	refID := cfg.DefaultTenantID + "-clockify"
	if err := store.PutCredentialRef(controlplane.CredentialRef{
		ID:        refID,
		Backend:   "env",
		Reference: "CLOCKIFY_API_KEY",
		Workspace: cfg.WorkspaceID,
		BaseURL:   cfg.BaseURL,
	}); err != nil {
		return err
	}
	return store.PutTenant(controlplane.TenantRecord{
		ID:              cfg.DefaultTenantID,
		CredentialRefID: refID,
		WorkspaceID:     cfg.WorkspaceID,
		BaseURL:         cfg.BaseURL,
		Timezone:        cfg.Timezone,
		PolicyMode:      string(depsafePolicyMode()),
	})
}

func depsafePolicyMode() policy.Mode {
	mode := policy.Standard
	if raw := strings.TrimSpace(os.Getenv("CLOCKIFY_POLICY")); raw != "" {
		mode = policy.Mode(raw)
	}
	return mode
}

func tenantRuntime(_ context.Context, principalTenant string, deps runtimeDeps, store controlplane.Store) (*mcp.StreamableSessionRuntime, error) {
	tenant, ok := store.Tenant(principalTenant)
	if !ok {
		return nil, fmt.Errorf("tenant %q not found in control plane", principalTenant)
	}
	ref, ok := store.CredentialRef(tenant.CredentialRefID)
	if !ok {
		return nil, fmt.Errorf("credential ref %q not found for tenant %q", tenant.CredentialRefID, tenant.ID)
	}
	material, err := vault.Resolve(ref)
	if err != nil {
		return nil, err
	}
	baseURL := material.BaseURL
	if baseURL == "" {
		baseURL = tenant.BaseURL
	}
	if baseURL == "" {
		baseURL = deps.cfg.BaseURL
	}
	workspaceID := material.Workspace
	if workspaceID == "" {
		workspaceID = tenant.WorkspaceID
	}
	client := clockify.NewClient(material.APIKey, baseURL, deps.cfg.RequestTimeout, deps.cfg.MaxRetries)
	client.SetUserAgent("clockify-mcp-go/" + deps.version)

	pol := deps.policy.Clone()
	if tenant.PolicyMode != "" {
		pol.Mode = policy.Mode(tenant.PolicyMode)
	}
	if len(tenant.DenyTools) > 0 {
		pol.DeniedTools = map[string]bool{}
		for _, item := range tenant.DenyTools {
			pol.DeniedTools[item] = true
		}
	}
	if len(tenant.DenyGroups) > 0 {
		pol.DeniedGroups = map[string]bool{}
		for _, item := range tenant.DenyGroups {
			pol.DeniedGroups[item] = true
		}
	}
	if len(tenant.AllowGroups) > 0 {
		pol.AllowedGroups = map[string]bool{}
		for _, item := range tenant.AllowGroups {
			pol.AllowedGroups[item] = true
		}
	}
	bc := deps.bootstrap.Clone()
	service := newService(client, workspaceID, firstNonEmpty(tenant.Timezone, deps.cfg.Timezone), deps.dd, pol, deps.cfg.ReportMaxEntries)
	service.DeltaFormat = deps.cfg.DeltaFormat
	server := buildServer(deps.version, deps, service, pol, bc)
	return &mcp.StreamableSessionRuntime{
		Server:          server,
		Close:           client.Close,
		TenantID:        tenant.ID,
		WorkspaceID:     workspaceID,
		ClockifyBaseURL: baseURL,
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
