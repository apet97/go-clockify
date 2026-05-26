package tools

import (
	"context"
	"fmt"
	"maps"
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

type WebhookLogView map[string]any

func webhookHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. List webhooks (RO)
		{
			Tool: toolRO("clockify_list_webhooks", "List webhooks in the workspace, paginated via page and page_size.", map[string]any{
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
			Tool: toolRO("clockify_get_webhook", "Get a webhook by ID from the pinned workspace.", map[string]any{
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
			Tool: toolRW("clockify_create_webhook", "Create a new webhook subscription that delivers Clockify events to an external URL. URL must use HTTPS and cannot target private/loopback addresses. Supports dry_run:true.", map[string]any{
				"type":     "object",
				"required": []string{"name", "url", "webhook_event"},
				"properties": map[string]any{
					"url": map[string]any{"type": "string", "description": "HTTPS callback URL for webhook delivery"},
					"webhook_event": map[string]any{
						"type":        "string",
						"description": "Single Clockify webhook event type, e.g. NEW_TIME_ENTRY",
					},
					"trigger_source_type": map[string]any{
						"type":        "string",
						"description": "Trigger source type (default WORKSPACE_ID)",
						"enum":        []string{"PROJECT_ID", "USER_ID", "TAG_ID", "TASK_ID", "WORKSPACE_ID", "ASSIGNMENT_ID", "EXPENSE_ID"},
					},
					"trigger_source": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Trigger source IDs (default: current workspace ID)",
					},
					"name": map[string]any{"type": "string", "description": "Webhook name (required by live API, length 2..30)", "minLength": 2, "maxLength": 30},
					"auth_token": map[string]any{
						"type":        "string",
						"description": "Optional HMAC signing secret for webhook delivery; sent upstream as authToken and never returned unmasked",
					},
					"dry_run": map[string]any{"type": "boolean", "description": "Validate and preview the webhook payload without creating it"},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.CreateWebhook(ctx, args)
			},
		},
		// 4. Update webhook (RW)
		{
			Tool: toolRW("clockify_update_webhook", "Update an existing webhook subscription. Changes affect every future outbound delivery to the configured URL. Supports dry_run:true.", map[string]any{
				"type":     "object",
				"required": []string{"webhook_id"},
				"properties": map[string]any{
					"webhook_id": map[string]any{"type": "string", "description": "Webhook ID"},
					"url":        map[string]any{"type": "string", "description": "New HTTPS callback URL"},
					"webhook_event": map[string]any{
						"type":        "string",
						"description": "Single Clockify webhook event type",
					},
					"trigger_source_type": map[string]any{
						"type":        "string",
						"description": "Trigger source type",
						"enum":        []string{"PROJECT_ID", "USER_ID", "TAG_ID", "TASK_ID", "WORKSPACE_ID", "ASSIGNMENT_ID", "EXPENSE_ID"},
					},
					"trigger_source": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Trigger source IDs",
					},
					"name": map[string]any{"type": "string", "description": "New name for the webhook (length 2..30)", "minLength": 2, "maxLength": 30},
					"auth_token": map[string]any{
						"type":        "string",
						"description": "Optional replacement HMAC signing secret; sent upstream as authToken and never returned unmasked",
					},
					"dry_run": map[string]any{"type": "boolean", "description": "Validate and preview the webhook update without applying it"},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.UpdateWebhook(ctx, args)
			},
		},
		// 5. Delete webhook (destructive)
		{
			Tool: toolDestructive("clockify_delete_webhook", "Permanently delete a webhook subscription. Destructive: stops all future outbound deliveries to the configured URL. Supports dry_run preview.", map[string]any{
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
		// 6. List webhook delivery logs (RO)
		{
			Tool: toolRO("clockify_list_webhook_logs", "List delivery logs for a webhook", map[string]any{
				"type":     "object",
				"required": []string{"webhook_id"},
				"properties": map[string]any{
					"webhook_id":     map[string]any{"type": "string", "description": "Webhook ID"},
					"from":           map[string]any{"type": "string", "description": "Only logs after this RFC3339 timestamp"},
					"to":             map[string]any{"type": "string", "description": "Only logs before this RFC3339 timestamp"},
					"status":         map[string]any{"type": "string", "enum": []string{"ALL", "SUCCEEDED", "FAILED"}},
					"sort_by_newest": map[string]any{"type": "boolean"},
					"page":           map[string]any{"type": "integer", "description": "Page number (default 1)"},
					"page_size":      map[string]any{"type": "integer", "description": "Items per page (default 50)"},
				},
			}),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.ListWebhookLogs(ctx, args)
			},
		},
		// 7. List webhook events (RO)
		{
			Tool: toolRO("clockify_list_webhook_events", "List available webhook event types", map[string]any{
				"type": "object",
			}),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.ListWebhookEvents(ctx, args)
			},
		},
		// 8. Test webhook (unsupported by Clockify API)
		{
			Tool: toolRW("clockify_test_webhook", "Explain that Clockify does not expose a webhook test delivery/test-send endpoint; no external side effect occurs, so trigger a real event or inspect delivery logs in the Clockify UI.", map[string]any{
				"type":     "object",
				"required": []string{"webhook_id"},
				"properties": map[string]any{
					"webhook_id": map[string]any{"type": "string", "description": "Webhook ID to test"},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.TestWebhook(ctx, args)
			},
		},
	}
}

// validateWebhookURLForService runs validateWebhookURL plus, when DNS-aware
// validation is enabled, resolves the host and rejects any reply containing
// a private, reserved, link-local, or loopback IP. Same isPublicWebhookAddr
// classifier as the literal-IP path so the contract is identical across
// both modes.
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
	// legitimately-trusted hostname to a private IP. Empty allowlist =
	// no bypass.
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
		if reason := webhookAddrRejectionReason(a); reason != "" {
			return fmt.Errorf("webhook host %q resolves to private/reserved address %s (rejected_by=%s)", host, a.String(), reason)
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
		return fmt.Errorf("webhook URL cannot target private/loopback addresses (rejected_by=localhost)")
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if reason := webhookAddrRejectionReason(addr); reason != "" {
			return fmt.Errorf("webhook URL cannot target private, loopback, link-local, or reserved addresses (rejected_by=%s)", reason)
		}
	}
	return nil
}

func webhookAddrRejectionReason(addr netip.Addr) string {
	addr = addr.Unmap()
	if !addr.IsValid() || !addr.IsGlobalUnicast() {
		if addr.IsLoopback() {
			return "loopback"
		}
		if addr.IsUnspecified() {
			return "unspecified"
		}
		if addr.IsMulticast() {
			return "multicast"
		}
		if addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() {
			return "link-local"
		}
		return "not-global-unicast"
	}
	if addr.IsLoopback() {
		return "loopback"
	}
	if addr.IsPrivate() {
		return "private"
	}
	if addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() {
		return "link-local"
	}
	if addr.IsMulticast() {
		return "multicast"
	}
	if addr.IsUnspecified() {
		return "unspecified"
	}

	blockedPrefixes := []struct {
		cidr   string
		reason string
	}{
		{"0.0.0.0/8", "current-network"},
		{"100.64.0.0/10", "carrier-grade-nat"},
		{"169.254.0.0/16", "link-local"},
		{"192.0.0.0/24", "ietf-protocol-assignment"},
		{"192.0.2.0/24", "documentation"},
		{"198.18.0.0/15", "benchmarking"},
		{"198.51.100.0/24", "documentation"},
		{"203.0.113.0/24", "documentation"},
		{"224.0.0.0/4", "multicast"},
		{"240.0.0.0/4", "reserved"},
		{"::/128", "unspecified"},
		{"::1/128", "loopback"},
		{"fe80::/10", "link-local"},
		{"fc00::/7", "private"},
		{"ff00::/8", "multicast"},
		{"2001:db8::/32", "documentation"},
	}
	for _, prefix := range blockedPrefixes {
		p := netip.MustParsePrefix(prefix.cidr)
		if p.Contains(addr) {
			return prefix.reason
		}
	}

	return ""
}

func webhookEventArg(args map[string]any) (string, error) {
	if event := stringArg(args, "webhook_event"); event != "" {
		return event, nil
	}
	// Backwards-compatible handler path for callers that bypass the MCP
	// schema and still pass the pre-live-test `events` array. The live API
	// accepts one webhookEvent string per webhook.
	events, _, err := strictStringSliceArg(args, "events")
	if err != nil {
		return "", err
	}
	if len(events) > 0 {
		return events[0], nil
	}
	return "", nil
}

func webhookTriggerSourceArgs(args map[string]any, workspaceID string) (string, []string, error) {
	sourceType := stringArg(args, "trigger_source_type")
	if sourceType == "" {
		sourceType = "WORKSPACE_ID"
	}
	source, _, err := strictStringSliceArg(args, "trigger_source")
	if err != nil {
		return "", nil, err
	}
	if len(source) == 0 {
		source = []string{workspaceID}
	}
	return sourceType, source, nil
}

func webhookUserEventRequiresUserSource(event string) bool {
	switch strings.ToUpper(strings.TrimSpace(event)) {
	case "USER_EMAIL_CHANGED", "USER_UPDATED":
		return true
	default:
		return false
	}
}

func validateWebhookUserEventTriggerSource(event, sourceType string, source []string) error {
	if !webhookUserEventRequiresUserSource(event) {
		return nil
	}
	if strings.ToUpper(strings.TrimSpace(sourceType)) != "USER_ID" {
		return fmt.Errorf("%s webhooks require trigger_source_type=USER_ID and a nonempty trigger_source user id", strings.ToUpper(strings.TrimSpace(event)))
	}
	for _, item := range source {
		if strings.TrimSpace(item) != "" {
			return nil
		}
	}
	return fmt.Errorf("%s webhooks require trigger_source_type=USER_ID and a nonempty trigger_source user id", strings.ToUpper(strings.TrimSpace(event)))
}

func webhookStringFromPayload(payload map[string]any, key string) string {
	if v, ok := payload[key].(string); ok {
		return v
	}
	return ""
}

func webhookStringSliceFromPayload(payload map[string]any, key string) []string {
	raw, ok := payload[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{v}
	default:
		return nil
	}
}

// ListWebhooks returns webhooks for the workspace.
func (s *Service) ListWebhooks(ctx context.Context, args map[string]any) (ToolResult, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	page, pageSize := paginationFromArgs(args)

	query := map[string]string{
		"page":      strconv.Itoa(page),
		"page-size": strconv.Itoa(pageSize),
	}

	path, err := paths.Workspace(wsID, "webhooks")
	if err != nil {
		return ToolResult{}, err
	}
	// Upstream returns {workspaceWebhookCount: N, webhooks: [...]}.
	// Probe evidence: clockify-api-probe-lab/findings/webhooks.md
	// (rev 2 2026-05-02).
	var envelope struct {
		WorkspaceWebhookCount int              `json:"workspaceWebhookCount"`
		Webhooks              []map[string]any `json:"webhooks"`
	}
	if err := s.Client.Get(ctx, path, query, &envelope); err != nil {
		return ToolResult{}, err
	}
	all := envelope.Webhooks
	if all == nil {
		all = []map[string]any{}
	}
	items := all
	if len(all) > pageSize {
		start := (page - 1) * pageSize
		end := min(start+pageSize, len(all))
		items = []map[string]any{}
		if start < len(all) {
			items = all[start:end]
		}
	}
	for _, webhook := range items {
		maskWebhookAuthToken(webhook)
	}

	return ok("clockify_list_webhooks", items, emptyListMeta(map[string]any{
		"workspaceId":           wsID,
		"count":                 len(items),
		"total":                 max(envelope.WorkspaceWebhookCount, len(all)),
		"workspaceWebhookCount": envelope.WorkspaceWebhookCount,
		"page":                  page,
		"pageSize":              pageSize,
		"has_more":              len(items) == pageSize,
	}, "clockify_setup_webhook")), nil
}

// GetWebhook retrieves a single webhook by ID.
func (s *Service) GetWebhook(ctx context.Context, args map[string]any) (ToolResult, error) {
	webhookID := stringArg(args, "webhook_id")
	if err := resolve.ValidateID(webhookID, "webhook_id"); err != nil {
		return ToolResult{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	path, err := paths.Workspace(wsID, "webhooks", webhookID)
	if err != nil {
		return ToolResult{}, err
	}
	var webhook map[string]any
	if err := s.Client.Get(ctx, path, nil, &webhook); err != nil {
		return ToolResult{}, err
	}
	maskWebhookAuthToken(webhook)

	return ok("clockify_get_webhook", webhook, map[string]any{"workspaceId": wsID}), nil
}

func maskWebhookAuthToken(webhook map[string]any) {
	raw, ok := webhook["authToken"].(string)
	if !ok {
		raw, ok = webhook["auth_token"].(string)
	}
	delete(webhook, "auth_token")
	if !ok || raw == "" {
		delete(webhook, "authToken")
		if _, exists := webhook["secret_token_present"]; !exists {
			webhook["secret_token_present"] = false
		}
		return
	}
	webhook["secret_token_present"] = true
	if len(raw) <= 4 {
		webhook["auth_token_suffix"] = ""
		delete(webhook, "authToken")
		return
	}
	webhook["auth_token_suffix"] = raw[len(raw)-4:]
	delete(webhook, "authToken")
}

func webhookDryRunPreview(tool string, payload map[string]any) map[string]any {
	previewPayload := maps.Clone(payload)
	maskWebhookAuthToken(previewPayload)
	return map[string]any{
		"dry_run": true,
		"tool":    tool,
		"payload": previewPayload,
		"note":    "No changes were made.",
	}
}

func webhookLogViewsFromRaw(items []map[string]any) []WebhookLogView {
	out := make([]WebhookLogView, 0, len(items))
	for _, item := range items {
		statusCode, _ := reportInt(firstPresent(item, "statusCode", "status_code"))
		status := "UNKNOWN"
		if statusCode >= 200 && statusCode < 300 {
			status = "SUCCEEDED"
		} else if statusCode > 0 {
			status = "FAILED"
		}
		view := WebhookLogView(maps.Clone(item))
		view["delivery"] = map[string]any{
			"id":              firstReportString(item, "id", "_id"),
			"webhook_id":      firstReportString(item, "webhookId", "webhook_id"),
			"event_status_id": firstReportString(item, "webhookEventStatusId", "webhook_event_status_id"),
			"responded_at":    firstReportString(item, "respondedAt", "responded_at"),
			"status_code":     statusCode,
			"status":          status,
			"source":          "webhook_logs_api",
		}
		view["request"] = map[string]any{"body": firstReportString(item, "requestBody", "request_body")}
		view["response"] = map[string]any{"body": firstReportString(item, "responseBody", "response_body")}
		view["raw"] = maps.Clone(item)
		out = append(out, view)
	}
	return out
}

// CreateWebhook creates a new webhook with URL validation.
func (s *Service) CreateWebhook(ctx context.Context, args map[string]any) (ToolResult, error) {
	url := stringArg(args, "url")
	if url == "" {
		return ToolResult{}, fmt.Errorf("url is required")
	}
	if err := s.validateWebhookURLForService(ctx, url); err != nil {
		return ToolResult{}, err
	}
	name := stringArg(args, "name")
	if name == "" {
		return ToolResult{}, fmt.Errorf("name is required")
	}

	webhookEvent, err := webhookEventArg(args)
	if err != nil {
		return ToolResult{}, err
	}
	if webhookEvent == "" {
		return ToolResult{}, fmt.Errorf("webhook_event is required (the webhook event type, e.g. NEW_TIME_ENTRY)")
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	triggerSourceType, triggerSource, err := webhookTriggerSourceArgs(args, wsID)
	if err != nil {
		return ToolResult{}, err
	}
	if err := validateWebhookUserEventTriggerSource(webhookEvent, triggerSourceType, triggerSource); err != nil {
		return ToolResult{}, err
	}

	payload := map[string]any{
		"url":               url,
		"webhookEvent":      webhookEvent,
		"triggerSourceType": triggerSourceType,
		"triggerSource":     triggerSource,
		"name":              name,
	}
	if authToken := stringArg(args, "auth_token"); authToken != "" {
		payload["authToken"] = authToken
	}
	if dryrun.Enabled(args) {
		return ok("clockify_create_webhook", webhookDryRunPreview("clockify_create_webhook", payload), map[string]any{"workspaceId": wsID}), nil
	}

	path, err := paths.Workspace(wsID, "webhooks")
	if err != nil {
		return ToolResult{}, err
	}
	var result map[string]any
	if err := s.Client.Post(ctx, path, payload, &result); err != nil {
		return ToolResult{}, err
	}
	maskWebhookAuthToken(result)

	return ok("clockify_create_webhook", result, map[string]any{"workspaceId": wsID}), nil
}

// UpdateWebhook updates an existing webhook.
func (s *Service) UpdateWebhook(ctx context.Context, args map[string]any) (ToolResult, error) {
	webhookID := stringArg(args, "webhook_id")
	if err := resolve.ValidateID(webhookID, "webhook_id"); err != nil {
		return ToolResult{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	path, err := paths.Workspace(wsID, "webhooks", webhookID)
	if err != nil {
		return ToolResult{}, err
	}

	var existing map[string]any
	if err := s.Client.Get(ctx, path, nil, &existing); err != nil {
		return ToolResult{}, err
	}

	payload := map[string]any{}
	for _, key := range []string{"name", "url", "webhookEvent", "triggerSourceType", "triggerSource"} {
		if v, ok := existing[key]; ok {
			payload[key] = v
		}
	}
	changed := false
	if url := stringArg(args, "url"); url != "" {
		if err := s.validateWebhookURLForService(ctx, url); err != nil {
			return ToolResult{}, err
		}
		payload["url"] = url
		changed = true
	}
	event, err := webhookEventArg(args)
	if err != nil {
		return ToolResult{}, err
	}
	if event != "" {
		payload["webhookEvent"] = event
		changed = true
	}
	if sourceType := stringArg(args, "trigger_source_type"); sourceType != "" {
		payload["triggerSourceType"] = sourceType
		changed = true
	}
	source, _, err := strictStringSliceArg(args, "trigger_source")
	if err != nil {
		return ToolResult{}, err
	}
	if len(source) > 0 {
		payload["triggerSource"] = source
		changed = true
	}
	if name := stringArg(args, "name"); name != "" {
		payload["name"] = name
		changed = true
	}
	if authToken := stringArg(args, "auth_token"); authToken != "" {
		payload["authToken"] = authToken
		changed = true
	}

	if !changed {
		return ToolResult{}, fmt.Errorf("at least one field (url, webhook_event, trigger_source_type, trigger_source, name, auth_token) must be provided for update")
	}
	if err := validateWebhookUserEventTriggerSource(
		webhookStringFromPayload(payload, "webhookEvent"),
		webhookStringFromPayload(payload, "triggerSourceType"),
		webhookStringSliceFromPayload(payload, "triggerSource"),
	); err != nil {
		return ToolResult{}, err
	}
	if dryrun.Enabled(args) {
		current := maps.Clone(existing)
		maskWebhookAuthToken(current)
		return ok("clockify_update_webhook", map[string]any{
			"dry_run": true,
			"tool":    "clockify_update_webhook",
			"current": current,
			"payload": webhookDryRunPreview("clockify_update_webhook", payload)["payload"],
			"note":    "No changes were made.",
		}, map[string]any{"workspaceId": wsID, "webhookId": webhookID}), nil
	}
	var result map[string]any
	if err := s.Client.Put(ctx, path, payload, &result); err != nil {
		return ToolResult{}, err
	}
	maskWebhookAuthToken(result)

	return ok("clockify_update_webhook", result, map[string]any{"workspaceId": wsID, "webhookId": webhookID}), nil
}

// DeleteWebhook deletes a webhook. Supports dry-run (preview via GET).
func (s *Service) DeleteWebhook(ctx context.Context, args map[string]any) (ToolResult, error) {
	webhookID := stringArg(args, "webhook_id")
	if err := resolve.ValidateID(webhookID, "webhook_id"); err != nil {
		return ToolResult{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	webhookPath, err := paths.Workspace(wsID, "webhooks", webhookID)
	if err != nil {
		return ToolResult{}, err
	}

	if dryrun.Enabled(args) {
		var webhook map[string]any
		if err := s.Client.Get(ctx, webhookPath, nil, &webhook); err != nil {
			return ToolResult{}, err
		}
		maskWebhookAuthToken(webhook)
		return ToolResult{
			OK:     true,
			Action: "clockify_delete_webhook",
			Data:   dryrun.WrapResult(webhook, "clockify_delete_webhook"),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	if err := s.Client.Delete(ctx, webhookPath); err != nil {
		return ToolResult{}, err
	}

	return ok("clockify_delete_webhook", map[string]any{"deleted": true, "webhookId": webhookID}, map[string]any{"workspaceId": wsID}), nil
}

// webhookEventEnum is the static list of webhook event types Clockify
// supports today. Clockify exposes no dynamic events-listing endpoint
// (probed: /workspaces/{ws}/webhooks/events 400s, the per-webhook
// /webhooks/{id}/events 404s), so go-clockify's tool surfaces the
// authoritative enum from WEBHOOKDOC.md instead. Values are the
// case-sensitive enum that subscribe-create accepts as `webhookEvent`.
var webhookEventEnum = []string{
	"APPROVAL_REQUEST_STATUS_UPDATED",
	"ASSIGNMENT_CREATED",
	"ASSIGNMENT_DELETED",
	"ASSIGNMENT_PUBLISHED",
	"ASSIGNMENT_UPDATED",
	"BALANCE_UPDATED",
	"BILLABLE_RATE_UPDATED",
	"CLIENT_DELETED",
	"CLIENT_UPDATED",
	"COST_RATE_UPDATED",
	"EXPENSE_CREATED",
	"EXPENSE_DELETED",
	"EXPENSE_RESTORED",
	"EXPENSE_UPDATED",
	"INVOICE_UPDATED",
	"LIMITED_USERS_ADDED_TO_WORKSPACE",
	"NEW_APPROVAL_REQUEST",
	"NEW_CLIENT",
	"NEW_INVOICE",
	"NEW_PROJECT",
	"NEW_TAG",
	"NEW_TASK",
	"NEW_TIME_ENTRY",
	"NEW_TIMER_STARTED",
	"PROJECT_DELETED",
	"PROJECT_UPDATED",
	"TAG_DELETED",
	"TAG_UPDATED",
	"TASK_DELETED",
	"TASK_UPDATED",
	"TIME_ENTRY_DELETED",
	"TIME_ENTRY_RESTORED",
	"TIME_ENTRY_SPLIT",
	"TIME_ENTRY_UPDATED",
	"TIME_OFF_REQUEST_APPROVED",
	"TIME_OFF_REQUEST_REJECTED",
	"TIME_OFF_REQUEST_STARTED",
	"TIME_OFF_REQUEST_UPDATED",
	"TIME_OFF_REQUEST_WITHDRAWN",
	"TIME_OFF_REQUESTED",
	"TIMER_STOPPED",
	"USER_ACTIVATED_ON_WORKSPACE",
	"USER_DEACTIVATED_ON_WORKSPACE",
	"USER_DELETED_FROM_WORKSPACE",
	"USER_EMAIL_CHANGED",
	"USER_GROUP_CREATED",
	"USER_GROUP_DELETED",
	"USER_GROUP_UPDATED",
	"USER_JOINED_WORKSPACE",
	"USER_UPDATED",
	"USERS_INVITED_TO_WORKSPACE",
}

// ListWebhookLogs returns delivery attempts for a webhook via the documented
// POST logs search route.
func (s *Service) ListWebhookLogs(ctx context.Context, args map[string]any) (ToolResult, error) {
	webhookID := stringArg(args, "webhook_id")
	if err := resolve.ValidateID(webhookID, "webhook_id"); err != nil {
		return ToolResult{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	page, pageSize := paginationFromArgs(args)
	body := map[string]any{}
	if v := strings.TrimSpace(stringArg(args, "from")); v != "" {
		body["from"] = v
	}
	if v := strings.TrimSpace(stringArg(args, "to")); v != "" {
		body["to"] = v
	}
	if v := strings.ToUpper(strings.TrimSpace(stringArg(args, "status"))); v != "" {
		if v != "ALL" && v != "SUCCEEDED" && v != "FAILED" {
			return ToolResult{}, fmt.Errorf("status must be ALL, SUCCEEDED, or FAILED")
		}
		body["status"] = v
	}
	if v, ok := args["sort_by_newest"].(bool); ok {
		body["sortByNewest"] = v
	}
	path, err := paths.Workspace(wsID, "webhooks", webhookID, "logs")
	if err != nil {
		return ToolResult{}, err
	}
	query := map[string]string{
		"page": strconv.Itoa(page),
		"size": strconv.Itoa(pageSize),
	}
	var logs []map[string]any
	if err := s.Client.PostWithQuery(ctx, path, query, body, &logs); err != nil {
		return ToolResult{}, err
	}
	meta := addPaginationMeta(map[string]any{"workspaceId": wsID, "webhookId": webhookID, "count": len(logs)}, args, page, pageSize)
	return ok("clockify_list_webhook_logs", webhookLogViewsFromRaw(logs), meta), nil
}

// ListWebhookEvents returns the static enum of webhook event types
// the Clockify webhooks API accepts. The upstream exposes no
// listing endpoint — see findings/webhooks.md (#15).
func (s *Service) ListWebhookEvents(ctx context.Context, _ map[string]any) (ToolResult, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	// Return a fresh copy so callers can't mutate the package-level slice.
	events := append([]string(nil), webhookEventEnum...)
	return ok("clockify_list_webhook_events", events, map[string]any{
		"workspaceId": wsID,
		"count":       len(events),
	}), nil
}

// TestWebhook reports the absence of a Clockify webhook test-send endpoint.
func (s *Service) TestWebhook(ctx context.Context, args map[string]any) (ToolResult, error) {
	webhookID := stringArg(args, "webhook_id")
	if err := resolve.ValidateID(webhookID, "webhook_id"); err != nil {
		return ToolResult{}, err
	}

	return ToolResult{}, fmt.Errorf("unsupported: Clockify does not expose a webhook test-send endpoint; trigger a real event or inspect delivery in the Clockify UI")
}
