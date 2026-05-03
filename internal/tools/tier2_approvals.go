package tools

import (
	"context"
	"fmt"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

func init() {
	registerTier2Group(Tier2Group{
		Name:        "approvals",
		Description: "Timesheet approval workflows",
		Keywords:    []string{"approval", "timesheet", "approve", "reject", "submit"},
		ToolNames: []string{
			"clockify_list_approval_requests",
			"clockify_get_approval_request",
			"clockify_submit_for_approval",
			"clockify_approve_timesheet",
			"clockify_reject_timesheet",
			"clockify_withdraw_approval",
		},
		Builder: approvalHandlers,
	})
}

func approvalHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. List approval requests (RO)
		{Tool: toolRO("clockify_list_approval_requests", "List approval requests with optional status filter and pagination", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status":    map[string]any{"type": "string", "description": "Filter by status (e.g. PENDING, APPROVED, REJECTED, WITHDRAWN)"},
				"page":      map[string]any{"type": "integer", "description": "Page number (default 1)"},
				"page_size": map[string]any{"type": "integer", "description": "Items per page (default 50)"},
			},
		}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.listApprovalRequests(ctx, args)
		}},

		// 2. Get approval request (RO)
		{Tool: toolRO("clockify_get_approval_request", "Get a single approval request by ID", map[string]any{
			"type":       "object",
			"required":   []string{"approval_id"},
			"properties": map[string]any{"approval_id": map[string]any{"type": "string"}},
		}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.getApprovalRequest(ctx, args)
		}},

		// 3. Submit for approval (RW)
		{Tool: toolRW("clockify_submit_for_approval", "Submit a timesheet for approval for a configured approval period", map[string]any{
			"type":     "object",
			"required": []string{"period", "period_start"},
			"properties": map[string]any{
				"period": map[string]any{
					"type":        "string",
					"description": "Approval period; must match workspace approval settings",
					"enum":        []string{"WEEKLY", "SEMI_MONTHLY", "MONTHLY"},
				},
				"period_start": map[string]any{"type": "string", "description": "Period start in RFC3339/millisecond form, e.g. 2026-05-01T00:00:00.000Z"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.submitForApproval(ctx, args)
		}},

		// 4. Approve timesheet (RW, confirm-pattern dry-run)
		{Tool: toolRW("clockify_approve_timesheet", "Approve a pending timesheet approval request", map[string]any{
			"type":     "object",
			"required": []string{"approval_id"},
			"properties": map[string]any{
				"approval_id": map[string]any{"type": "string"},
				"dry_run":     map[string]any{"type": "boolean", "description": "If true, preview the approval without executing it"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.approveTimesheet(ctx, args)
		}},

		// 5. Reject timesheet (RW, confirm-pattern dry-run)
		{Tool: toolRW("clockify_reject_timesheet", "Reject a pending timesheet approval request", map[string]any{
			"type":     "object",
			"required": []string{"approval_id"},
			"properties": map[string]any{
				"approval_id": map[string]any{"type": "string"},
				"reason":      map[string]any{"type": "string", "description": "Reason for rejection"},
				"dry_run":     map[string]any{"type": "boolean", "description": "If true, preview the rejection without executing it"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.rejectTimesheet(ctx, args)
		}},

		// 6. Withdraw approval (RW)
		{Tool: toolRW("clockify_withdraw_approval", "Withdraw a previously submitted approval request", map[string]any{
			"type":     "object",
			"required": []string{"approval_id"},
			"properties": map[string]any{
				"approval_id": map[string]any{"type": "string"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.withdrawApproval(ctx, args)
		}},
	}
}

// ---------------------------------------------------------------------------
// Approval handlers
// ---------------------------------------------------------------------------

func (s *Service) listApprovalRequests(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	page := intArg(args, "page", 1)
	pageSize := intArg(args, "page_size", 50)

	query := map[string]string{
		"page":      fmt.Sprintf("%d", page),
		"page-size": fmt.Sprintf("%d", pageSize),
	}
	if v := stringArg(args, "status"); v != "" {
		query["status"] = v
	}

	path, err := paths.Workspace(wsID, "approval-requests")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var items []map[string]any
	if err := s.Client.Get(ctx, path, query, &items); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_list_approval_requests", items, map[string]any{
		"workspaceId": wsID,
		"count":       len(items),
		"page":        page,
	}), nil
}

func (s *Service) getApprovalRequest(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	approvalID := stringArg(args, "approval_id")
	if err := resolve.ValidateID(approvalID, "approval_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	path, err := paths.Workspace(wsID, "approval-requests", approvalID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var approval map[string]any
	if err := s.Client.Get(ctx, path, nil, &approval); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_get_approval_request", approval, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) submitForApproval(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	period := stringArg(args, "period")
	periodStart := stringArg(args, "period_start")
	if period == "" || periodStart == "" {
		// Backwards-compatible direct-handler fallback for pre-live-test
		// callers. The live API accepts period/periodStart, not start/end.
		if legacyStart := stringArg(args, "start"); legacyStart != "" {
			period = "WEEKLY"
			periodStart = legacyStart
		}
	}
	if period == "" || periodStart == "" {
		return ResultEnvelope{}, fmt.Errorf("period and period_start are required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{
		"period":      period,
		"periodStart": periodStart,
	}

	path, err := paths.Workspace(wsID, "approval-requests")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var created any
	if err := s.Client.Post(ctx, path, body, &created); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_submit_for_approval", created, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) approveTimesheet(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	approvalID := stringArg(args, "approval_id")
	if err := resolve.ValidateID(approvalID, "approval_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	approvalPath, err := paths.Workspace(wsID, "approval-requests", approvalID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	// Confirm-pattern dry-run: fetch and return preview without mutating.
	if dryrun.Enabled(args) {
		var approval map[string]any
		if err := s.Client.Get(ctx, approvalPath, nil, &approval); err != nil {
			return ResultEnvelope{}, err
		}
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_approve_timesheet",
			Data:   dryrun.WrapResult(approval, "clockify_approve_timesheet"),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	body := map[string]any{"status": "APPROVED"}
	var result map[string]any
	if err := s.Client.Put(ctx, approvalPath, body, &result); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_approve_timesheet", result, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) rejectTimesheet(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	approvalID := stringArg(args, "approval_id")
	if err := resolve.ValidateID(approvalID, "approval_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	approvalPath, err := paths.Workspace(wsID, "approval-requests", approvalID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	// Confirm-pattern dry-run: fetch and return preview without mutating.
	if dryrun.Enabled(args) {
		var approval map[string]any
		if err := s.Client.Get(ctx, approvalPath, nil, &approval); err != nil {
			return ResultEnvelope{}, err
		}
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_reject_timesheet",
			Data:   dryrun.WrapResult(approval, "clockify_reject_timesheet"),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	body := map[string]any{"status": "REJECTED"}
	if reason := stringArg(args, "reason"); reason != "" {
		body["reason"] = reason
	}

	var result map[string]any
	if err := s.Client.Put(ctx, approvalPath, body, &result); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_reject_timesheet", result, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) withdrawApproval(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	approvalID := stringArg(args, "approval_id")
	if err := resolve.ValidateID(approvalID, "approval_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{"status": "WITHDRAWN"}
	path, err := paths.Workspace(wsID, "approval-requests", approvalID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var result map[string]any
	if err := s.Client.Put(ctx, path, body, &result); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_withdraw_approval", result, map[string]any{"workspaceId": wsID}), nil
}
