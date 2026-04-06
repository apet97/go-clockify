package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
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
			ReadOnlyHint: true,
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
			ReadOnlyHint: true,
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
			ReadOnlyHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.ListWebhookEvents(ctx, args)
			},
		},
		// 7. Test webhook (RW)
		{
			Tool: toolRW("clockify_test_webhook", "Send a test delivery to a webhook", map[string]any{
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

// validateWebhookURL checks that a webhook URL uses HTTPS and doesn't target
// private or loopback addresses.
func validateWebhookURL(url string) error {
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("webhook URL must use HTTPS")
	}
	host := strings.TrimPrefix(url, "https://")
	host = strings.Split(host, "/")[0]
	host = strings.Split(host, ":")[0]

	blocked := []string{"localhost", "127.0.0.1", "10.", "192.168.", "0.0.0.0"}
	for _, b := range blocked {
		if strings.HasPrefix(host, b) || host == b {
			return fmt.Errorf("webhook URL cannot target private/loopback addresses")
		}
	}

	// Check 172.16-31.x.x range
	if strings.HasPrefix(host, "172.") {
		parts := strings.Split(host, ".")
		if len(parts) >= 2 {
			if n, err := strconv.Atoi(parts[1]); err == nil && n >= 16 && n <= 31 {
				return fmt.Errorf("webhook URL cannot target private addresses")
			}
		}
	}
	return nil
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

	var webhooks []map[string]any
	if err := s.Client.Get(ctx, "/workspaces/"+wsID+"/webhooks", query, &webhooks); err != nil {
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

	var webhook map[string]any
	if err := s.Client.Get(ctx, "/workspaces/"+wsID+"/webhooks/"+webhookID, nil, &webhook); err != nil {
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
	if err := validateWebhookURL(url); err != nil {
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

	var result map[string]any
	if err := s.Client.Post(ctx, "/workspaces/"+wsID+"/webhooks", payload, &result); err != nil {
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
		if err := validateWebhookURL(url); err != nil {
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

	var result map[string]any
	if err := s.Client.Put(ctx, "/workspaces/"+wsID+"/webhooks/"+webhookID, payload, &result); err != nil {
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

	if dryrun.Enabled(args) {
		var webhook map[string]any
		if err := s.Client.Get(ctx, "/workspaces/"+wsID+"/webhooks/"+webhookID, nil, &webhook); err != nil {
			return ResultEnvelope{}, err
		}
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_webhook",
			Data:   dryrun.WrapResult(webhook, "clockify_delete_webhook"),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	if err := s.Client.Delete(ctx, "/workspaces/"+wsID+"/webhooks/"+webhookID); err != nil {
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

	var events []map[string]any
	if err := s.Client.Get(ctx, "/workspaces/"+wsID+"/webhooks/events", nil, &events); err != nil {
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

	var result map[string]any
	if err := s.Client.Post(ctx, "/workspaces/"+wsID+"/webhooks/"+webhookID+"/test", nil, &result); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_test_webhook", result, map[string]any{
		"workspaceId": wsID,
		"webhookId":   webhookID,
	}), nil
}
