package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

// approvalFilterStates is the exact set Clockify's approval-requests list
// endpoint accepts for its status filter. REJECTED requests revert to
// UNSUBMITTED and are not listable; WITHDRAWN_SUBMISSION is not a valid
// filter value. Live-verified 2026-05-18.
var approvalFilterStates = []string{"PENDING", "APPROVED", "WITHDRAWN_APPROVAL"}

func approvalHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. List approval requests (RO)
		{Tool: withOutputSchema(toolRO("clockify_list_approval_requests", "List approval requests with optional status filter and pagination", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status": approvalFilterStateSchema(),
				"sort_column": map[string]any{
					"type":        "string",
					"enum":        []string{"ID", "USER_ID", "START", "UPDATED_AT"},
					"description": "Documented approval list sort column",
				},
				"sort_order": map[string]any{
					"type":        "string",
					"enum":        []string{"ASCENDING", "DESCENDING"},
					"description": "Documented approval list sort order",
				},
				"page":      map[string]any{"type": "integer", "description": "Page number (default 1)"},
				"page_size": map[string]any{"type": "integer", "description": "Items per page (default 50)"},
			},
		}), approvalViewEnvelope("clockify_list_approval_requests", true)), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.listApprovalRequests(ctx, args)
		}},

		// 2. Get approval request (RO)
		{Tool: withOutputSchema(toolRO("clockify_get_approval_request", "Get a single approval request by ID by scanning the documented approval-requests list endpoint", map[string]any{
			"type":     "object",
			"required": []string{"approval_id"},
			"properties": map[string]any{
				"approval_id": map[string]any{"type": "string"},
				"status":      approvalFilterStateSchema(),
				"page_size":   map[string]any{"type": "integer", "description": "Page size for the documented list scan (default 100, max 200)"},
			},
		}), approvalViewEnvelope("clockify_get_approval_request", false)), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.getApprovalRequest(ctx, args)
		}},

		// 3. Submit for approval (RW)
		{Tool: withOutputSchema(toolRW("clockify_submit_for_approval", "Submit the caller's timesheet for approval and create/update an approval request.", approvalPeriodInputSchema(false)), approvalViewEnvelope("clockify_submit_for_approval", false)), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.submitForApproval(ctx, args)
		}},

		// 4. Resubmit caller entries for approval (RW)
		{Tool: withOutputSchema(toolRW("clockify_resubmit_for_approval", "Resubmit rejected or withdrawn caller entries and update approval state.", approvalPeriodInputSchema(false)), approvalViewEnvelope("clockify_resubmit_for_approval", false)), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.resubmitForApproval(ctx, args)
		}},

		// 5. Submit another user's timesheet for approval (RW)
		{Tool: withOutputSchema(toolRW("clockify_submit_for_user_approval", "Submit another user's timesheet for approval and create/update an approval request.", approvalPeriodInputSchema(true)), approvalViewEnvelope("clockify_submit_for_user_approval", false)), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.submitForUserApproval(ctx, args)
		}},

		// 6. Resubmit another user's entries for approval (RW)
		{Tool: withOutputSchema(toolRW("clockify_resubmit_for_user_approval", "Resubmit another user's rejected or withdrawn entries and update approval state.", approvalPeriodInputSchema(true)), approvalViewEnvelope("clockify_resubmit_for_user_approval", false)), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.resubmitForUserApproval(ctx, args)
		}},

		// 7. Approve timesheet (RW, confirm-pattern dry-run)
		{Tool: withOutputSchema(toolRW("clockify_approve_timesheet", "Approve a pending timesheet approval request and update its state.", map[string]any{
			"type":     "object",
			"required": []string{"approval_id"},
			"properties": map[string]any{
				"approval_id": map[string]any{"type": "string"},
				"note":        map[string]any{"type": "string", "description": "Optional approval note sent as documented PATCH note"},
				"dry_run":     map[string]any{"type": "boolean", "description": "If true, preview the approval without executing it"},
			},
		}), approvalViewEnvelope("clockify_approve_timesheet", false)), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.approveTimesheet(ctx, args)
		}},

		// 8. Reject timesheet (RW, confirm-pattern dry-run)
		{Tool: withOutputSchema(toolRW("clockify_reject_timesheet", "Reject a pending timesheet approval request and update its state.", map[string]any{
			"type":     "object",
			"required": []string{"approval_id"},
			"properties": map[string]any{
				"approval_id": map[string]any{"type": "string"},
				"reason":      map[string]any{"type": "string", "description": "Reason for rejection; sent as documented PATCH note"},
				"note":        map[string]any{"type": "string", "description": "Optional note alias; reason wins when both are supplied"},
				"dry_run":     map[string]any{"type": "boolean", "description": "If true, preview the rejection without executing it"},
			},
		}), approvalViewEnvelope("clockify_reject_timesheet", false)), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.rejectTimesheet(ctx, args)
		}},

		// 9. Withdraw approval (RW)
		{Tool: withOutputSchema(toolRW("clockify_withdraw_approval", "Withdraw a submitted or already-approved approval request and update its state.", map[string]any{
			"type":     "object",
			"required": []string{"approval_id"},
			"properties": map[string]any{
				"approval_id": map[string]any{"type": "string"},
				"state":       map[string]any{"type": "string", "enum": []string{"WITHDRAWN_SUBMISSION", "WITHDRAWN_APPROVAL"}, "description": "Documented withdrawal state. Defaults to WITHDRAWN_SUBMISSION."},
				"note":        map[string]any{"type": "string"},
				"dry_run":     map[string]any{"type": "boolean", "description": "If true, preview the withdrawal without executing it"},
			},
		}), approvalViewEnvelope("clockify_withdraw_approval", false)), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
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
	items, page, pageSize, err := s.listApprovalRequestsRaw(ctx, wsID, args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	for i := range items {
		delete(items[i], "entries")
	}
	return ok("clockify_list_approval_requests", approvalViewsFromRaw(items), emptyListMeta(map[string]any{
		"workspaceId": wsID,
		"count":       len(items),
		"page":        page,
		"pageSize":    pageSize,
	}, "clockify_approvals_submit")), nil
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
	approval, pages, scanned, err := s.findApprovalRequest(ctx, wsID, approvalID, args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	entryCount := stripApprovalEntries(approval)
	meta := map[string]any{"workspaceId": wsID, "pagesScanned": pages, "itemsScanned": scanned}
	if entryCount > 0 {
		meta["entriesOmitted"] = entryCount
		meta["entriesHint"] = "Time entries are omitted to keep the response small; use clockify_reports_detailed for entry-level detail."
	}
	return ok("clockify_get_approval_request", approvalViewFromRaw(approval), meta), nil
}

func (s *Service) submitForApproval(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	return s.postApprovalPeriod(ctx, args, "clockify_submit_for_approval", "approval-requests")
}

func (s *Service) resubmitForApproval(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	return s.postApprovalPeriod(ctx, args, "clockify_resubmit_for_approval", "approval-requests", "resubmit-entries-for-approval")
}

func (s *Service) resubmitApprovalOneUser(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	approvalID, err := requiredIDArg(args, "approval_id")
	if err != nil {
		return ResultEnvelope{}, err
	}
	entryIDs, found, err := strictStringSliceArg(args, "entry_ids")
	if err != nil {
		return ResultEnvelope{}, err
	}
	if !found || len(entryIDs) == 0 {
		return ResultEnvelope{}, fmt.Errorf("entry_ids is required and must contain at least one time entry ID")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	// Preflight: look up the approval's current state so a failed resubmit
	// names the actual status and points at the right recovery, instead of
	// surfacing the upstream "no active approval request for period" with no
	// context. Best-effort — a lookup failure must not block the resubmit.
	status := ""
	if approval, _, _, ferr := s.findApprovalRequest(ctx, wsID, approvalID, args); ferr == nil {
		status = strings.ToUpper(strings.TrimSpace(firstReportString(approval, "status", "state")))
	}
	if status == "APPROVED" {
		return ResultEnvelope{}, fmt.Errorf("approval %s is already APPROVED; there is nothing to resubmit", approvalID)
	}
	path, err := paths.Workspace(wsID, "approval-requests", "resubmit-entries-for-approval")
	if err != nil {
		return ResultEnvelope{}, err
	}
	body := nativeBodyFromArgs(args, "approval_id", "entry_ids", "expense_ids", "period", "period_start", "note")
	var updated any
	if err := s.Client.Post(ctx, path, body, &updated); err != nil {
		if status == "REJECTED" || strings.HasPrefix(status, "WITHDRAWN") {
			return ResultEnvelope{}, fmt.Errorf("approval %s is %s: the resubmit endpoint re-submits entries for an active approval period; submit a fresh request for this period with clockify_approvals_submit", approvalID, status)
		}
		return ResultEnvelope{}, err
	}
	return ok("clockify_approvals_resubmit", approvalDataView(updated), map[string]any{
		"workspaceId": wsID,
		"approvalId":  approvalID,
	}), nil
}

func (s *Service) submitForUserApproval(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	return s.postUserApprovalPeriod(ctx, args, "clockify_submit_for_user_approval", "approval-requests", "users")
}

func (s *Service) resubmitForUserApproval(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	return s.postUserApprovalPeriod(ctx, args, "clockify_resubmit_for_user_approval", "approval-requests", "users", "__USER__", "resubmit-entries-for-approval")
}

func (s *Service) postApprovalPeriod(ctx context.Context, args map[string]any, action string, parts ...string) (ResultEnvelope, error) {
	body, err := approvalPeriodBody(args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, parts...)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if dryrun.Enabled(args) {
		return ok(action, dryrunPreviewPayload(action, body), map[string]any{"workspaceId": wsID}), nil
	}
	var created any
	if err := s.Client.Post(ctx, path, body, &created); err != nil {
		return ResultEnvelope{}, err
	}
	return ok(action, approvalDataView(created), map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) postUserApprovalPeriod(ctx context.Context, args map[string]any, action string, parts ...string) (ResultEnvelope, error) {
	userRef := stringArg(args, "user_id")
	if userRef == "" {
		return ResultEnvelope{}, fmt.Errorf("user_id is required")
	}
	body, err := approvalPeriodBody(args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	userID, err := s.resolveUserID(ctx, wsID, userRef)
	if err != nil {
		return ResultEnvelope{}, err
	}
	for i, part := range parts {
		if part == "__USER__" {
			parts[i] = userID
		}
	}
	if len(parts) >= 2 && parts[len(parts)-1] == "users" {
		parts = append(parts, userID)
	}
	path, err := paths.Workspace(wsID, parts...)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if dryrun.Enabled(args) {
		return ok(action, dryrunPreviewPayload(action, body), map[string]any{"workspaceId": wsID, "userId": userID}), nil
	}
	var created any
	if err := s.Client.Post(ctx, path, body, &created); err != nil {
		return ResultEnvelope{}, err
	}
	return ok(action, approvalDataView(created), map[string]any{"workspaceId": wsID, "userId": userID}), nil
}

func (s *Service) approveTimesheet(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	return s.patchApprovalState(ctx, args, "clockify_approve_timesheet", "APPROVED", stringArg(args, "note"))
}

func (s *Service) rejectTimesheet(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	note := firstNonEmptyString(stringArg(args, "reason"), stringArg(args, "note"))
	return s.patchApprovalState(ctx, args, "clockify_reject_timesheet", "REJECTED", note)
}

func (s *Service) withdrawApproval(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	state := firstNonEmptyString(stringArg(args, "state"), "WITHDRAWN_SUBMISSION")
	if state != "WITHDRAWN_SUBMISSION" && state != "WITHDRAWN_APPROVAL" {
		return ResultEnvelope{}, fmt.Errorf("state must be WITHDRAWN_SUBMISSION or WITHDRAWN_APPROVAL")
	}
	return s.patchApprovalState(ctx, args, "clockify_withdraw_approval", state, stringArg(args, "note"))
}

func (s *Service) patchApprovalState(ctx context.Context, args map[string]any, action, state, note string) (ResultEnvelope, error) {
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
	if dryrun.Enabled(args) {
		approval, pages, scanned, err := s.findApprovalRequest(ctx, wsID, approvalID, args)
		if err != nil {
			return ResultEnvelope{}, err
		}
		entryCount := stripApprovalEntries(approval)
		meta := map[string]any{"workspaceId": wsID, "pagesScanned": pages, "itemsScanned": scanned, "state": state}
		if entryCount > 0 {
			meta["entriesOmitted"] = entryCount
			meta["entriesHint"] = "Time entries are omitted to keep the response small; use clockify_reports_detailed for entry-level detail."
		}
		return ResultEnvelope{
			OK:     true,
			Action: action,
			Data:   dryrun.WrapResult(approvalViewFromRaw(approval), action),
			Meta:   meta,
		}, nil
	}

	body := map[string]any{"state": state}
	if note != "" {
		body["note"] = note
	}
	var result map[string]any
	if err := s.Client.Patch(ctx, path, body, &result); err != nil {
		return ResultEnvelope{}, err
	}
	return ok(action, approvalViewFromRaw(result), map[string]any{"workspaceId": wsID, "state": state}), nil
}

func approvalFilterStateSchema() map[string]any {
	return map[string]any{
		"type":        "string",
		"enum":        approvalFilterStates,
		"description": "Approval request filter state. These are the only status filter values Clockify's approval-requests API accepts.",
	}
}

func approvalPeriodInputSchema(forUser bool) map[string]any {
	required := []string{"period", "period_start"}
	props := map[string]any{
		"period": map[string]any{
			"type":        "string",
			"description": "Approval period; must match workspace approval settings",
			"enum":        []string{"WEEKLY", "SEMI_MONTHLY", "MONTHLY"},
		},
		"period_start": map[string]any{"type": "string", "description": "Period start in RFC3339/millisecond form, e.g. 2026-05-01T00:00:00.000Z"},
		"start":        map[string]any{"type": "string", "description": "Legacy alias for period_start; when used alone the period defaults to WEEKLY"},
		"dry_run":      map[string]any{"type": "boolean", "description": "Preview the approval submission without making changes"},
	}
	if forUser {
		required = append([]string{"user_id"}, required...)
		props["user_id"] = map[string]any{"type": "string", "description": "User ID, name, or email"}
	}
	return map[string]any{"type": "object", "required": required, "properties": props}
}

func approvalPeriodBody(args map[string]any) (map[string]any, error) {
	period := strings.TrimSpace(stringArg(args, "period"))
	periodStart := strings.TrimSpace(stringArg(args, "period_start"))
	if period == "" || periodStart == "" {
		// Backwards-compatible direct-handler fallback for pre-live-test
		// callers. The live API accepts period/periodStart, not start/end.
		if legacyStart := strings.TrimSpace(stringArg(args, "start")); legacyStart != "" {
			period = firstNonEmptyString(period, "WEEKLY")
			periodStart = firstNonEmptyString(periodStart, legacyStart)
		}
	}
	if period == "" || periodStart == "" {
		return nil, fmt.Errorf("period and period_start are required")
	}
	return map[string]any{
		"period":      period,
		"periodStart": periodStart,
	}, nil
}

func (s *Service) listApprovalRequestsRaw(ctx context.Context, wsID string, args map[string]any) ([]map[string]any, int, int, error) {
	page := max(intArg(args, "page", 1), 1)
	pageSize := intArg(args, "page_size", 50)
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	query := map[string]string{
		"page":      fmt.Sprintf("%d", page),
		"page-size": fmt.Sprintf("%d", pageSize),
	}
	if v := strings.TrimSpace(stringArg(args, "status")); v != "" {
		query["status"] = v
	}
	if v := strings.TrimSpace(stringArg(args, "sort_column")); v != "" {
		query["sort-column"] = v
	}
	if v := strings.TrimSpace(stringArg(args, "sort_order")); v != "" {
		query["sort-order"] = v
	}

	path, err := paths.Workspace(wsID, "approval-requests")
	if err != nil {
		return nil, page, pageSize, err
	}
	var items []map[string]any
	if err := s.Client.Get(ctx, path, query, &items); err != nil {
		return nil, page, pageSize, err
	}
	return items, page, pageSize, nil
}

func (s *Service) findApprovalRequest(ctx context.Context, wsID, approvalID string, args map[string]any) (map[string]any, int, int, error) {
	pageSize := intArg(args, "page_size", 100)
	if pageSize < 1 {
		pageSize = 100
	}
	if pageSize > 200 {
		pageSize = 200
	}
	status := strings.TrimSpace(stringArg(args, "status"))
	statuses := []string{status}
	if status == "" {
		statuses = approvalFilterStates
	}
	pagesScanned := 0
	scanned := 0
	for _, scanStatus := range statuses {
		for page := 1; page <= aggregatePageSafetyStop; page++ {
			pagesScanned++
			scanArgs := map[string]any{"page": page, "page_size": pageSize}
			if scanStatus != "" {
				scanArgs["status"] = scanStatus
			}
			items, _, _, err := s.listApprovalRequestsRaw(ctx, wsID, scanArgs)
			if err != nil {
				return nil, pagesScanned, scanned, err
			}
			scanned += len(items)
			for _, item := range items {
				if approvalRequestID(item) == approvalID {
					return item, pagesScanned, scanned, nil
				}
			}
			if len(items) < pageSize {
				break
			}
		}
	}
	return nil, pagesScanned, scanned, fmt.Errorf("approval request %q not found via documented approval-requests list", approvalID)
}

// stripApprovalEntries removes the heavy embedded time-entry array from a raw
// approval map and returns how many entries were dropped, so callers can record
// meta.entriesOmitted. The list endpoint embeds every entry in the period,
// which can exceed the result-size cap.
func stripApprovalEntries(approval map[string]any) int {
	n := 0
	switch e := approval["entries"].(type) {
	case []any:
		n = len(e)
	case []map[string]any:
		n = len(e)
	}
	delete(approval, "entries")
	return n
}

func approvalRequestID(raw map[string]any) string {
	request := approvalRequestMap(raw)
	return firstNonEmptyString(firstReportString(request, "id", "_id", "get_id"), firstReportString(raw, "id", "_id", "get_id"))
}
