package tools

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	neturl "net/url"
	"strconv"
	"strings"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

func init() {
	registerTier2Group(Tier2Group{
		Name:        "webhooks",
		Description: "Webhook management",
		Keywords:    []string{"webhook", "event", "subscribe", "notification", "callback"},
		Builder:     webhookHandlers,
	})
}

func webhookHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. List webhooks (RO)
		{
			Tool: toolRO("clockify_list_webhooks", "List webhooks in the workspace", map[string]any{
				"type": "object",
				"properties": map[string]any{
					"page":      map[string]any{"type": "integer", "description": "Page number (default 1)"},
					"page_size": map[string]any{"type": "integer", "description": "Items per page (default 50)"},
				},
			}),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.ListWebhooks(ctx, args)
			},
		},
		// 2. Get webhook (RO)
		{
			Tool: toolRO("clockify_get_webhook", "Get a webhook by ID", map[string]any{
				"type":     "object",
				"required": []string{"webhook_id"},
				"properties": map[string]any{
					"webhook_id": map[string]any{"type": "string", "description": "Webhook ID"},
				},
			}),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.GetWebhook(ctx, args)
			},
		},
		// 3. Create webhook (RW)
		{
			Tool: toolRW("clockify_create_webhook", "Create a new webhook. URL must use HTTPS and cannot target private/loopback addresses.", map[string]any{
				"type":     "object",
				"required": []string{"url", "events"},
				"properties": map[string]any{
					"url": map[string]any{"type": "string", "description": "HTTPS callback URL for webhook delivery"},
					"events": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "List of event types to subscribe to",
					},
					"name": map[string]any{"type": "string", "description": "Optional name for the webhook"},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.CreateWebhook(ctx, args)
			},
		},
		// 4. Update webhook (RW)
		{
			Tool: toolRW("clockify_update_webhook", "Update an existing webhook", map[string]any{
				"type":     "object",
				"required": []string{"webhook_id"},
				"properties": map[string]any{
					"webhook_id": map[string]any{"type": "string", "description": "Webhook ID"},
					"url":        map[string]any{"type": "string", "description": "New HTTPS callback URL"},
					"events": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "New list of event types",
					},
					"name": map[string]any{"type": "string", "description": "New name for the webhook"},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.UpdateWebhook(ctx, args)
			},
		},
		// 5. Delete webhook (destructive)
		{
			Tool: toolDestructive("clockify_delete_webhook", "Delete a webhook", map[string]any{
				"type":     "object",
				"required": []string{"webhook_id"},
				"properties": map[string]any{
					"webhook_id": map[string]any{"type": "string", "description": "Webhook ID to delete"},
					"dry_run":    map[string]any{"type": "boolean", "description": "Preview the webhook without deleting"},
				},
			}),
			ReadOnlyHint:    false,
			DestructiveHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.DeleteWebhook(ctx, args)
			},
		},
		// 6. List webhook events (RO)
		{
			Tool: toolRO("clockify_list_webhook_events", "List available webhook event types", map[string]any{
				"type": "object",
			}),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.ListWebhookEvents(ctx, args)
			},
		},
		// 7. Test webhook (RW)
		{
			Tool: toolRW("clockify_test_webhook", "Send a test delivery to a webhook. The /test POST is an external side effect (the configured target receives the test payload), so dry_run:true is supported and returns the current webhook record without sending.", map[string]any{
				"type":     "object",
				"required": []string{"webhook_id"},
				"properties": map[string]any{
					"webhook_id": map[string]any{"type": "string", "description": "Webhook ID to test"},
					"dry_run":    map[string]any{"type": "boolean", "description": "Preview only; returns the webhook record without sending the test delivery"},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.TestWebhook(ctx, args)
			},
		},
	}
}

// validateWebhookURLForService runs validateWebhookURL plus, when the
// service is configured for DNS-aware validation (hosted profiles),
// resolves the host and rejects any reply containing a private,
// reserved, link-local, or loopback IP. Same isPublicWebhookAddr
// classifier as the literal-IP path so the contract is identical
// across both modes.
//
// Note: there is an inherent TOCTOU window between this resolve and
// the upstream Clockify→host delivery — DNS rebinding is not fully
// closed by this gate. It does close the easy case where the operator
// or a malicious user supplies an explicitly hostile hostname like
// metadata.google.internal or a domain that resolves to 169.254.169.254.
func (s *Service) validateWebhookURLForService(ctx context.Context, url string) error {
	if err := validateWebhookURL(url); err != nil {
		return err
	}
	if !s.WebhookValidateDNS {
		return nil
	}
	parsed, err := neturl.Parse(url)
	if err != nil {
		return fmt.Errorf("invalid webhook URL: %w", err)
	}
	host := strings.ToLower(parsed.Hostname())
	if _, ipErr := netip.ParseAddr(host); ipErr == nil {
		// IP literals were already classified by validateWebhookURL.
		return nil
	}

	// Operator escape hatch: known-trusted hostnames bypass the
	// private-IP check. Used when split-horizon DNS resolves a
	// legitimately-trusted hostname to a private IP only on the
	// control-plane network (see docs/runbooks/webhook-dns-validation.md
	// §4b). Empty allowlist = no bypass.
	if isWebhookHostAllowed(host, s.WebhookAllowedDomains) {
		return nil
	}

	resolver := s.WebhookHostResolver
	if resolver == nil {
		resolver = defaultWebhookResolver
	}
	addrs, err := resolver(ctx, host)
	if err != nil {
		return fmt.Errorf("webhook host %q DNS lookup failed: %w", host, err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("webhook host %q resolved to no addresses", host)
	}
	for _, a := range addrs {
		if !isPublicWebhookAddr(a) {
			return fmt.Errorf("webhook host %q resolves to private/reserved address %s", host, a.String())
		}
	}
	return nil
}

// isWebhookHostAllowed reports whether host matches any entry in the
// caller-supplied allowlist. Match modes:
//
//   - Exact: `webhook.example.com` matches host `webhook.example.com`.
//   - Suffix (entry begins with `.`): `.example.com` matches
//     `webhook.example.com` and `api.example.com`. The leading dot is
//     load-bearing — it forces the suffix to be a *full* DNS label,
//     so `.example.com` does NOT match `attacker.example.com.evil.com`
//     because that string ends with `.evil.com`, not `.example.com`.
//
// Empty / whitespace-only entries are skipped so an operator's typo
// in `CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS=,foo.com` doesn't silently
// match every host. Comparison is case-insensitive on the entry side
// (host is already lowercased by the caller).
func isWebhookHostAllowed(host string, allow []string) bool {
	for _, d := range allow {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" {
			continue
		}
		if host == d {
			return true
		}
		if strings.HasPrefix(d, ".") && strings.HasSuffix(host, d) {
			return true
		}
	}
	return false
}

// defaultWebhookResolver wraps net.DefaultResolver.LookupIPAddr in the
// netip-returning shape the service callback expects.
func defaultWebhookResolver(ctx context.Context, host string) ([]netip.Addr, error) {
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	out := make([]netip.Addr, 0, len(addrs))
	for _, a := range addrs {
		ip, ok := netip.AddrFromSlice(a.IP)
		if !ok {
			continue
		}
		out = append(out, ip)
	}
	return out, nil
}

// validateWebhookURL checks that a webhook URL uses HTTPS and doesn't target
// private or loopback addresses.
func validateWebhookURL(url string) error {
	parsed, err := neturl.Parse(url)
	if err != nil {
		return fmt.Errorf("invalid webhook URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("webhook URL must use HTTPS")
	}
	if parsed.Host == "" {
		return fmt.Errorf("webhook URL must include a host")
	}
	if parsed.User != nil {
		return fmt.Errorf("webhook URL must not contain embedded credentials")
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("webhook URL cannot target private/loopback addresses")
	}
	if addr, err := netip.ParseAddr(host); err == nil && !isPublicWebhookAddr(addr) {
		return fmt.Errorf("webhook URL cannot target private, loopback, link-local, or reserved addresses")
	}
	return nil
}

func isPublicWebhookAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() || !addr.IsGlobalUnicast() {
		return false
	}
	if addr.IsPrivate() || addr.IsLoopback() || addr.IsMulticast() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsUnspecified() {
		return false
	}

	blockedPrefixes := []string{
		"0.0.0.0/8",
		"100.64.0.0/10",
		"169.254.0.0/16",
		"192.0.0.0/24",
		"192.0.2.0/24",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"224.0.0.0/4",
		"240.0.0.0/4",
		"::/128",
		"::1/128",
		"fe80::/10",
		"fc00::/7",
		"ff00::/8",
		"2001:db8::/32",
	}
	for _, prefix := range blockedPrefixes {
		p := netip.MustParsePrefix(prefix)
		if p.Contains(addr) {
			return false
		}
	}

	return true
}

// stringSliceArg extracts a []string from args. Handles both []string and
// []any (as JSON-decoded arrays arrive as []any).
func stringSliceArg(args map[string]any, key string) []string {
	v, ok := args[key]
	if !ok {
		return nil
	}
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		out := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out
	default:
		return nil
	}
}

// ListWebhooks returns webhooks for the workspace.
func (s *Service) ListWebhooks(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	page := intArg(args, "page", 1)
	pageSize := intArg(args, "page_size", 50)

	query := map[string]string{
		"page":      strconv.Itoa(page),
		"page-size": strconv.Itoa(pageSize),
	}

	path, err := paths.Workspace(wsID, "webhooks")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var webhooks []map[string]any
	if err := s.Client.Get(ctx, path, query, &webhooks); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_list_webhooks", webhooks, map[string]any{
		"workspaceId": wsID,
		"count":       len(webhooks),
		"page":        page,
		"pageSize":    pageSize,
	}), nil
}

// GetWebhook retrieves a single webhook by ID.
func (s *Service) GetWebhook(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	webhookID := stringArg(args, "webhook_id")
	if err := resolve.ValidateID(webhookID, "webhook_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	path, err := paths.Workspace(wsID, "webhooks", webhookID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var webhook map[string]any
	if err := s.Client.Get(ctx, path, nil, &webhook); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_get_webhook", webhook, map[string]any{"workspaceId": wsID}), nil
}

// CreateWebhook creates a new webhook with URL validation.
func (s *Service) CreateWebhook(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	url := stringArg(args, "url")
	if url == "" {
		return ResultEnvelope{}, fmt.Errorf("url is required")
	}
	if err := s.validateWebhookURLForService(ctx, url); err != nil {
		return ResultEnvelope{}, err
	}

	events := stringSliceArg(args, "events")
	if len(events) == 0 {
		return ResultEnvelope{}, fmt.Errorf("events is required and must contain at least one event type")
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	payload := map[string]any{
		"url":    url,
		"events": events,
	}
	if name := stringArg(args, "name"); name != "" {
		payload["name"] = name
	}

	path, err := paths.Workspace(wsID, "webhooks")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var result map[string]any
	if err := s.Client.Post(ctx, path, payload, &result); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_create_webhook", result, map[string]any{"workspaceId": wsID}), nil
}

// UpdateWebhook updates an existing webhook.
func (s *Service) UpdateWebhook(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	webhookID := stringArg(args, "webhook_id")
	if err := resolve.ValidateID(webhookID, "webhook_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	payload := map[string]any{}
	if url := stringArg(args, "url"); url != "" {
		if err := s.validateWebhookURLForService(ctx, url); err != nil {
			return ResultEnvelope{}, err
		}
		payload["url"] = url
	}
	if events := stringSliceArg(args, "events"); len(events) > 0 {
		payload["events"] = events
	}
	if name := stringArg(args, "name"); name != "" {
		payload["name"] = name
	}

	if len(payload) == 0 {
		return ResultEnvelope{}, fmt.Errorf("at least one field (url, events, name) must be provided for update")
	}

	path, err := paths.Workspace(wsID, "webhooks", webhookID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var result map[string]any
	if err := s.Client.Put(ctx, path, payload, &result); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_update_webhook", result, map[string]any{"workspaceId": wsID, "webhookId": webhookID}), nil
}

// DeleteWebhook deletes a webhook. Supports dry-run (preview via GET).
func (s *Service) DeleteWebhook(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	webhookID := stringArg(args, "webhook_id")
	if err := resolve.ValidateID(webhookID, "webhook_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	webhookPath, err := paths.Workspace(wsID, "webhooks", webhookID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		var webhook map[string]any
		if err := s.Client.Get(ctx, webhookPath, nil, &webhook); err != nil {
			return ResultEnvelope{}, err
		}
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_webhook",
			Data:   dryrun.WrapResult(webhook, "clockify_delete_webhook"),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	if err := s.Client.Delete(ctx, webhookPath); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_delete_webhook", map[string]any{"deleted": true, "webhookId": webhookID}, map[string]any{"workspaceId": wsID}), nil
}

// ListWebhookEvents returns the available webhook event types.
func (s *Service) ListWebhookEvents(ctx context.Context, _ map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	path, err := paths.Workspace(wsID, "webhooks", "events")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var events []map[string]any
	if err := s.Client.Get(ctx, path, nil, &events); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_list_webhook_events", events, map[string]any{
		"workspaceId": wsID,
		"count":       len(events),
	}), nil
}

// TestWebhook sends a test delivery to a webhook.
func (s *Service) TestWebhook(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	webhookID := stringArg(args, "webhook_id")
	if err := resolve.ValidateID(webhookID, "webhook_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		previewPath, err := paths.Workspace(wsID, "webhooks", webhookID)
		if err != nil {
			return ResultEnvelope{}, err
		}
		var webhook map[string]any
		if err := s.Client.Get(ctx, previewPath, nil, &webhook); err != nil {
			return ResultEnvelope{}, err
		}
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_test_webhook",
			Data:   dryrun.WrapResult(webhook, "clockify_test_webhook"),
			Meta: map[string]any{
				"workspaceId": wsID,
				"webhookId":   webhookID,
			},
		}, nil
	}

	testPath, err := paths.Workspace(wsID, "webhooks", webhookID, "test")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var result map[string]any
	if err := s.Client.Post(ctx, testPath, nil, &result); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_test_webhook", result, map[string]any{
		"workspaceId": wsID,
		"webhookId":   webhookID,
	}), nil
}
