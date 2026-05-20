package tools

import (
	"context"

	"github.com/apet97/go-clockify/internal/mcp"
)

func timeOffHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. clockify_list_time_off_requests (RO)
		{
			Tool: withOutputSchema(toolRO("clockify_list_time_off_requests",
				"List time off requests with an optional status filter, paginated via page and page_size.",
				map[string]any{"type": "object", "properties": map[string]any{
					"status":    map[string]any{"type": "string", "enum": []string{"PENDING", "APPROVED", "REJECTED", "DENIED", "ALL"}, "description": "Filter by status: PENDING, APPROVED, REJECTED, ALL (default ALL). DENIED is accepted as a legacy alias for REJECTED and is translated before the upstream POST."},
					"user_id":   map[string]any{"type": "string", "description": "Filter by user ID or name/email"},
					"page":      map[string]any{"type": "integer"},
					"page_size": map[string]any{"type": "integer"},
				}}), envelopeSchemaFor[[]TimeOffRequestView]("clockify_list_time_off_requests")),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.listTimeOffRequests(ctx, args)
			},
		},
		// 2. clockify_get_time_off_request (RO)
		{
			Tool: withOutputSchema(toolRO("clockify_get_time_off_request",
				"Get a time off request by policy ID and request ID",
				map[string]any{"type": "object", "required": []string{"policy_id", "request_id"}, "properties": map[string]any{
					"policy_id":  map[string]any{"type": "string"},
					"request_id": map[string]any{"type": "string"},
				}}), envelopeSchemaFor[TimeOffRequestView]("clockify_get_time_off_request")),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.getTimeOffRequest(ctx, args)
			},
		},
		// 3. clockify_create_time_off_request (RW)
		{
			Tool: toolRW("clockify_create_time_off_request",
				"Create a time off request under a policy. Changes leave balances/approval workflow.",
				map[string]any{"type": "object", "required": []string{"policy_id", "start", "end", "note"}, "properties": map[string]any{
					"policy_id": map[string]any{"type": "string"},
					"start":     map[string]any{"type": "string", "description": "Start date (YYYY-MM-DD or RFC3339)"},
					"end":       map[string]any{"type": "string", "description": "End date (YYYY-MM-DD or RFC3339)"},
					"note":      map[string]any{"type": "string", "description": "Required note/reason"},
					"half_day":  map[string]any{"type": "boolean", "description": "Request half day"},
					"timezone":  timezoneInputProperty(),
					"dry_run":   map[string]any{"type": "boolean", "description": "Preview request creation without making changes"},
				}}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.createTimeOffRequest(ctx, args)
			},
		},
		// 4. clockify_update_time_off_request (RW)
		{
			Tool: toolRW("clockify_update_time_off_request",
				"Update an existing time off request approval status.",
				map[string]any{"type": "object", "required": []string{"policy_id", "request_id", "status"}, "properties": map[string]any{
					"policy_id":  map[string]any{"type": "string"},
					"request_id": map[string]any{"type": "string"},
					"status": map[string]any{
						"type":        "string",
						"description": "Status to set via Clockify's PATCH route",
						"enum":        []string{"APPROVED", "REJECTED"},
					},
					"dry_run": map[string]any{"type": "boolean", "description": "Preview the update payload without calling Clockify."},
				}}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.updateTimeOffRequest(ctx, args)
			},
		},
		// 5. clockify_delete_time_off_request (destructive)
		{
			Tool: toolDestructive("clockify_delete_time_off_request",
				"Permanently delete a time off request from the workspace policy. Admin scope; destructive; supports dry_run preview.",
				map[string]any{"type": "object", "required": []string{"policy_id", "request_id"}, "properties": map[string]any{
					"policy_id":  map[string]any{"type": "string"},
					"request_id": map[string]any{"type": "string"},
					"dry_run":    map[string]any{"type": "boolean", "description": "Preview deletion without making changes"},
				}}),
			ReadOnlyHint:    false,
			DestructiveHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.deleteTimeOffRequest(ctx, args)
			},
		},
		// 6. clockify_approve_time_off (RW)
		{
			Tool: toolRW("clockify_approve_time_off",
				"Approve a pending time off request and update its approval state.",
				map[string]any{"type": "object", "required": []string{"policy_id", "request_id"}, "properties": map[string]any{
					"policy_id":  map[string]any{"type": "string"},
					"request_id": map[string]any{"type": "string"},
					"note":       map[string]any{"type": "string", "description": "Optional approval note"},
				}}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.approveTimeOff(ctx, args)
			},
		},
		// 7. clockify_deny_time_off (RW)
		{
			Tool: toolRW("clockify_deny_time_off",
				"Deny a pending time off request and update its approval state.",
				map[string]any{"type": "object", "required": []string{"policy_id", "request_id"}, "properties": map[string]any{
					"policy_id":  map[string]any{"type": "string"},
					"request_id": map[string]any{"type": "string"},
					"note":       map[string]any{"type": "string", "description": "Optional denial reason"},
				}}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.denyTimeOff(ctx, args)
			},
		},
		// 8. clockify_list_time_off_policies (RO)
		{
			Tool: withOutputSchema(toolRO("clockify_list_time_off_policies",
				"List time off policies for the workspace, paginated via page and page_size.",
				map[string]any{"type": "object", "properties": map[string]any{
					"page":      map[string]any{"type": "integer"},
					"page_size": map[string]any{"type": "integer"},
				}}), envelopeSchemaFor[[]CompactTimeOffPolicyView]("clockify_list_time_off_policies")),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.listTimeOffPolicies(ctx, args)
			},
		},
		// 9. clockify_get_time_off_policy (RO)
		{
			Tool: withOutputSchema(toolRO("clockify_get_time_off_policy",
				"Get a time off policy by ID",
				map[string]any{"type": "object", "required": []string{"policy_id"}, "properties": map[string]any{
					"policy_id": map[string]any{"type": "string"},
				}}), envelopeSchemaFor[TimeOffPolicyView]("clockify_get_time_off_policy")),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.getTimeOffPolicy(ctx, args)
			},
		},
		// 10. clockify_create_time_off_policy (RW)
		{
			Tool: toolRW("clockify_create_time_off_policy",
				"Create a simplified time off policy for the current user. Supports approval and days_per_year accrual basics; use clockify_api_request for advanced Clockify fields such as color, icon, expiration, half days, approval stages, or filters.",
				map[string]any{"type": "object", "required": []string{"name"}, "properties": map[string]any{
					"name":              map[string]any{"type": "string"},
					"accrual":           map[string]any{"type": "boolean", "description": "Whether the policy uses accrual"},
					"auto_approve":      map[string]any{"type": "boolean", "description": "Auto-approve requests"},
					"days_per_year":     map[string]any{"type": "number"},
					"negative_balance":  map[string]any{"type": "boolean", "description": "Allow negative balances"},
					"requires_approval": map[string]any{"type": "boolean"},
					"time_unit":         map[string]any{"type": "string", "description": "DAYS or HOURS"},
				}}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.createTimeOffPolicy(ctx, args)
			},
		},
		// 11. clockify_update_time_off_policy (RW)
		{
			Tool: toolRW("clockify_update_time_off_policy",
				"Update a time off policy by merging supplied fields into the current upstream body. Preserves advanced Clockify fields outside this simplified schema; use clockify_api_request for full-surface updates.",
				map[string]any{"type": "object", "required": []string{"policy_id"}, "properties": map[string]any{
					"policy_id":         map[string]any{"type": "string"},
					"name":              map[string]any{"type": "string"},
					"accrual":           map[string]any{"type": "boolean"},
					"auto_approve":      map[string]any{"type": "boolean"},
					"days_per_year":     map[string]any{"type": "number"},
					"negative_balance":  map[string]any{"type": "boolean"},
					"requires_approval": map[string]any{"type": "boolean"},
					"time_unit":         map[string]any{"type": "string"},
				}}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.updateTimeOffPolicy(ctx, args)
			},
		},
		// 12. clockify_time_off_balance (RO)
		{
			Tool: withOutputSchema(toolRO("clockify_time_off_balance",
				"Get time off balance. user_id defaults to the current user; omit policy_id to return all policy balances for that user (page through them with page/page_size).",
				map[string]any{"type": "object", "properties": map[string]any{
					"policy_id": map[string]any{"type": "string", "description": "Policy ID. Omit to return all policy balances for the resolved user."},
					"user_id":   map[string]any{"type": "string", "description": "User ID or name/email. Default: current user."},
					"page":      map[string]any{"type": "integer", "description": "Page number when policy_id is omitted (default 1)."},
					"page_size": map[string]any{"type": "integer", "description": "Balances per page when policy_id is omitted (default 50, max 200)."},
				}}), envelopeOpenMap("clockify_time_off_balance")),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.timeOffBalance(ctx, args)
			},
		},
		// 13. clockify_time_off_balance_update (RW; admin/billing impact)
		{
			Tool: withOutputSchema(toolRW("clockify_time_off_balance_update",
				"Adjust time off balances for one or more users under a policy. Admin and billing impact: balances drive future PTO accrual and approval. Supports dry_run preview.",
				map[string]any{"type": "object", "required": []string{"policy_id", "user_ids", "value", "note"}, "properties": map[string]any{
					"policy_id": map[string]any{"type": "string"},
					"user_ids":  map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string"}, "description": "User IDs (or names/emails) whose balance to adjust under this policy."},
					"value":     map[string]any{"type": "number", "minimum": -10000, "maximum": 10000, "description": "Absolute balance value to set (units determined by the policy time-unit, e.g. days or hours). Clockify constrains this to the range [-10000, 10000]."},
					"note":      map[string]any{"type": "string", "description": "Required note explaining the adjustment; surfaced in Clockify audit history."},
					"dry_run":   map[string]any{"type": "boolean", "description": "Preview the PATCH payload without calling Clockify."},
				}}), envelopeOpenMap("clockify_time_off_balance_update")),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.updateTimeOffBalance(ctx, args)
			},
		},
	}
}
