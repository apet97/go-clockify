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
		Name:        "shared_reports",
		Description: "Shared report management — create, update, export, delete",
		Keywords:    []string{"shared", "report", "export", "csv", "pdf"},
		ToolNames: []string{
			"clockify_list_shared_reports",
			"clockify_get_shared_report",
			"clockify_create_shared_report",
			"clockify_update_shared_report",
			"clockify_delete_shared_report",
			"clockify_export_shared_report",
		},
		Builder: sharedReportHandlers,
	})
}

func sharedReportHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. List shared reports (RO)
		{Tool: toolRO("clockify_list_shared_reports", "List shared reports in the workspace with pagination", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"page":      map[string]any{"type": "integer", "description": "Page number (default 1)"},
				"page_size": map[string]any{"type": "integer", "description": "Items per page (default 50)"},
			},
		}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.listSharedReports(ctx, args)
		}},

		// 2. Get shared report (RO)
		{Tool: toolRO("clockify_get_shared_report", "Get a single shared report by ID", map[string]any{
			"type":       "object",
			"required":   []string{"report_id"},
			"properties": map[string]any{"report_id": map[string]any{"type": "string"}},
		}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.getSharedReport(ctx, args)
		}},

		// 3. Create shared report (RW)
		{Tool: toolRW("clockify_create_shared_report", "Create a new shared report", map[string]any{
			"type":     "object",
			"required": []string{"name", "report_type"},
			"properties": map[string]any{
				"name":        map[string]any{"type": "string", "description": "Report name"},
				"report_type": map[string]any{"type": "string", "description": "Report type (e.g. SUMMARY, DETAILED, WEEKLY)"},
				"filters":     map[string]any{"type": "object", "description": "Optional filter object (project IDs, user IDs, date range, etc.)"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.createSharedReport(ctx, args)
		}},

		// 4. Update shared report (RW)
		{Tool: toolRW("clockify_update_shared_report", "Update an existing shared report", map[string]any{
			"type":     "object",
			"required": []string{"report_id"},
			"properties": map[string]any{
				"report_id":   map[string]any{"type": "string"},
				"name":        map[string]any{"type": "string"},
				"report_type": map[string]any{"type": "string"},
				"filters":     map[string]any{"type": "object"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.updateSharedReport(ctx, args)
		}},

		// 5. Delete shared report (destructive, dry-run preview)
		{Tool: toolDestructive("clockify_delete_shared_report", "Delete a shared report by ID", map[string]any{
			"type":     "object",
			"required": []string{"report_id"},
			"properties": map[string]any{
				"report_id": map[string]any{"type": "string"},
				"dry_run":   map[string]any{"type": "boolean", "description": "If true, preview the deletion without executing it"},
			},
		}), ReadOnlyHint: false, DestructiveHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.deleteSharedReport(ctx, args)
		}},

		// 6. Export shared report (RO)
		{Tool: toolRO("clockify_export_shared_report", "Export a shared report in a specified format", map[string]any{
			"type":     "object",
			"required": []string{"report_id"},
			"properties": map[string]any{
				"report_id": map[string]any{"type": "string"},
				"format":    map[string]any{"type": "string", "description": "Export format: csv, json, pdf, or excel (default json)"},
			},
		}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.exportSharedReport(ctx, args)
		}},
	}
}

// ---------------------------------------------------------------------------
// Shared report handlers
// ---------------------------------------------------------------------------

func (s *Service) listSharedReports(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	page := intArg(args, "page", 1)
	pageSize := intArg(args, "page_size", 50)

	// pageSize is camelCase here; the reports API silently ignores
	// page-size (hyphenated) and returns the default 50.
	query := map[string]string{
		"page":     fmt.Sprintf("%d", page),
		"pageSize": fmt.Sprintf("%d", pageSize),
	}

	path, err := paths.Workspace(wsID, "shared-reports")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var envelope struct {
		Reports []map[string]any `json:"reports"`
		Count   int              `json:"count"`
	}
	if err := s.Client.GetReports(ctx, path, query, &envelope); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_list_shared_reports", envelope.Reports, map[string]any{
		"workspaceId": wsID,
		"count":       len(envelope.Reports),
		"total":       envelope.Count,
		"page":        page,
	}), nil
}

func (s *Service) getSharedReport(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	reportID := stringArg(args, "report_id")
	if err := resolve.ValidateID(reportID, "report_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	// Single-get path has no workspace segment per
	// findings/shared-reports.md (the workspace-prefixed path 404s
	// even on the reports host). exportType=JSON_V1 forces a JSON
	// body — other values return PDF/CSV/XLSX.
	path := "/shared-reports/" + reportID
	query := map[string]string{"exportType": "JSON_V1"}
	var report map[string]any
	if err := s.Client.GetReports(ctx, path, query, &report); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_get_shared_report", report, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) createSharedReport(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	name := stringArg(args, "name")
	reportType := stringArg(args, "report_type")
	if name == "" || reportType == "" {
		return ResultEnvelope{}, fmt.Errorf("name and report_type are required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{
		"name":       name,
		"reportType": reportType,
	}
	if filters, ok := args["filters"].(map[string]any); ok {
		body["filters"] = filters
	}

	path, err := paths.Workspace(wsID, "shared-reports")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var created map[string]any
	if err := s.Client.PostReports(ctx, path, body, &created); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_create_shared_report", created, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) updateSharedReport(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	reportID := stringArg(args, "report_id")
	if err := resolve.ValidateID(reportID, "report_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{}
	if v := stringArg(args, "name"); v != "" {
		body["name"] = v
	}
	if v := stringArg(args, "report_type"); v != "" {
		body["reportType"] = v
	}
	if filters, ok := args["filters"].(map[string]any); ok {
		body["filters"] = filters
	}

	path, err := paths.Workspace(wsID, "shared-reports", reportID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var updated map[string]any
	if err := s.Client.PutReports(ctx, path, body, &updated); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_update_shared_report", updated, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) deleteSharedReport(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	reportID := stringArg(args, "report_id")
	if err := resolve.ValidateID(reportID, "report_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	reportPath, err := paths.Workspace(wsID, "shared-reports", reportID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	// Dry-run: fetch the report for preview, don't delete.
	if dryrun.Enabled(args) {
		var report map[string]any
		if err := s.Client.GetReports(ctx, reportPath, nil, &report); err != nil {
			return ResultEnvelope{}, err
		}
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_shared_report",
			Data:   dryrun.WrapResult(report, "clockify_delete_shared_report"),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	if err := s.Client.DeleteReports(ctx, reportPath); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_delete_shared_report", map[string]any{
		"deleted":  true,
		"reportId": reportID,
	}, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) exportSharedReport(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	reportID := stringArg(args, "report_id")
	if err := resolve.ValidateID(reportID, "report_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	format := stringArg(args, "format")
	if format == "" {
		format = "json"
	}

	query := map[string]string{"format": format}

	path, err := paths.Workspace(wsID, "shared-reports", reportID, "export")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var export map[string]any
	if err := s.Client.GetReports(ctx, path, query, &export); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_export_shared_report", export, map[string]any{
		"workspaceId": wsID,
		"reportId":    reportID,
		"format":      format,
	}), nil
}
