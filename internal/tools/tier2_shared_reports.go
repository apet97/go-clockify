package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

// sharedReportTypes is the live-confirmed enum the upstream accepts
// for the create/update body's "type" field (lab probe 2026-05-03,
// fixtures/shared-reports/create-2-with-filter.json error message).
// Documented in the JSON Schema descriptor so the AI client knows
// the full surface; the previous handler advertised only the
// SUMMARY/DETAILED/WEEKLY subset.
var sharedReportTypes = []string{
	"DETAILED", "WEEKLY", "SUMMARY", "SCHEDULED",
	"EXPENSE_DETAILED", "EXPENSE_RECEIPT",
	"PTO_REQUESTS", "PTO_BALANCE",
	"ATTENDANCE", "INVOICE_EXPENSE", "INVOICE_TIME",
	"PROJECT", "TEAM_FULL", "TEAM_LIMITED", "TEAM_GROUPS",
	"INVOICES", "KIOSK_PIN_LIST", "KIOSK_ASSIGNEES",
	"USER_DATA_EXPORT",
}

// sharedReportFilterSchema is the JSON Schema fragment for the
// upstream "filter" body field (singular — the Java DTO is
// ReportFilterV1). exportType / dateRangeStart / dateRangeEnd are
// required by the live API; everything else is optional and accepted
// as additional properties.
func sharedReportFilterSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Required filter object. Live-required: exportType (JSON_V1|PDF|CSV|XLSX), dateRangeStart (ISO 8601), dateRangeEnd (ISO 8601). Optional: summaryFilter / detailedFilter / weeklyFilter / projects / users / etc.",
		"required":    []string{"exportType", "dateRangeStart", "dateRangeEnd"},
		"properties": map[string]any{
			"exportType":     map[string]any{"type": "string", "enum": []string{"JSON_V1", "PDF", "CSV", "XLSX"}},
			"dateRangeStart": map[string]any{"type": "string", "description": "ISO 8601 timestamp"},
			"dateRangeEnd":   map[string]any{"type": "string", "description": "ISO 8601 timestamp"},
		},
		"additionalProperties": true,
	}
}

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
			"required": []string{"name", "report_type", "filter"},
			"properties": map[string]any{
				"name":        map[string]any{"type": "string", "description": "Report name"},
				"report_type": map[string]any{"type": "string", "description": "Report type — common: SUMMARY, DETAILED, WEEKLY. Full upstream enum below.", "enum": sharedReportTypes},
				"filter":      sharedReportFilterSchema(),
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.createSharedReport(ctx, args)
		}},

		// 4. Update shared report (RW)
		{Tool: toolRW("clockify_update_shared_report", "Update an existing shared report (PUT semantics is merge — partial body preserves the existing filter)", map[string]any{
			"type":     "object",
			"required": []string{"report_id"},
			"properties": map[string]any{
				"report_id":   map[string]any{"type": "string"},
				"name":        map[string]any{"type": "string"},
				"report_type": map[string]any{"type": "string", "enum": sharedReportTypes},
				"filter":      map[string]any{"type": "object", "description": "Optional filter object — same shape as create. Send only the keys you want to change.", "additionalProperties": true},
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

		// 6. Export shared report (RO).
		// Lab probe (2026-05-03) confirmed the export path is the
		// bare /shared-reports/{id} (no workspace) plus
		// ?exportType=PDF|CSV|XLSX|JSON_V1; the previously assumed
		// /export segment returns 404. Non-JSON formats return binary
		// (PDF/XLSX) or text (CSV) — the handler returns a binary-aware
		// envelope {contentType, filename, bytes, body(base64)} for
		// those, and a decoded JSON object for JSON_V1.
		{Tool: toolRO("clockify_export_shared_report", "Export a shared report. JSON returns the decoded object; PDF/CSV/XLSX return a binary-aware envelope with base64-encoded body.", map[string]any{
			"type":     "object",
			"required": []string{"report_id"},
			"properties": map[string]any{
				"report_id": map[string]any{"type": "string"},
				"format":    map[string]any{"type": "string", "description": "Export format. Accepts: json (default, returns decoded JSON_V1), pdf, csv, xlsx (alias: excel).", "enum": []string{"json", "pdf", "csv", "xlsx", "excel"}},
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

	// Single-get path has no workspace segment. The workspace-prefixed
	// per-id path (/v1/workspaces/{ws}/shared-reports/{id}) DOES exist
	// — PUT and DELETE land there — but it returns 405 for GET. The
	// bare-id path is the only GET-compatible route. Confirmed live
	// 2026-05-03; see findings/shared-reports.md. exportType=JSON_V1
	// forces a JSON body; other values return PDF/CSV/XLSX (handled
	// in exportSharedReport, not here).
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
	filter, hasFilter := args["filter"].(map[string]any)
	if !hasFilter || filter == nil {
		return ResultEnvelope{}, fmt.Errorf("filter is required (must include exportType, dateRangeStart, dateRangeEnd)")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	// Body keys match the upstream Java DTOs: "type" (not
	// reportType — server rejects reportType with 400) and "filter"
	// (singular, ReportFilterV1). See findings/shared-reports.md
	// changes #24-#27.
	body := map[string]any{
		"name":   name,
		"type":   reportType,
		"filter": filter,
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

	// PUT semantics is merge — partial body preserves the existing
	// filter (live-confirmed 2026-05-03). Send only the fields the
	// caller passed; mirror the create-side body keys (type / filter).
	body := map[string]any{}
	if v := stringArg(args, "name"); v != "" {
		body["name"] = v
	}
	if v := stringArg(args, "report_type"); v != "" {
		body["type"] = v
	}
	if filter, hasFilter := args["filter"].(map[string]any); hasFilter {
		body["filter"] = filter
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
	deletePath, err := paths.Workspace(wsID, "shared-reports", reportID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	// Dry-run preview: fetch via the bare-id GET (the only GET-
	// compatible per-id route — workspace-prefixed GET is 405). Don't
	// delete.
	if dryrun.Enabled(args) {
		var report map[string]any
		if err := s.Client.GetReports(ctx, "/shared-reports/"+reportID, map[string]string{"exportType": "JSON_V1"}, &report); err != nil {
			return ResultEnvelope{}, err
		}
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_shared_report",
			Data:   dryrun.WrapResult(report, "clockify_delete_shared_report"),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	if err := s.Client.DeleteReports(ctx, deletePath); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_delete_shared_report", map[string]any{
		"deleted":  true,
		"reportId": reportID,
	}, map[string]any{"workspaceId": wsID}), nil
}

// sharedReportExportTypeFor maps the user-facing format token to the
// upstream exportType enum value. The upstream enum is the one
// observed live (2026-05-03) on /v1/shared-reports/{id}?exportType=…
// The "excel" alias preserves the historical user-facing token.
func sharedReportExportTypeFor(format string) (exportType string, isJSON bool) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "json", "json_v1":
		return "JSON_V1", true
	case "pdf":
		return "PDF", false
	case "csv":
		return "CSV", false
	case "xlsx", "excel":
		return "XLSX", false
	default:
		// Unknown format — pass through verbatim and let the upstream
		// reject. Treated as binary (non-JSON) so the handler doesn't
		// silently base64-wrap a JSON response or vice versa.
		return strings.ToUpper(format), false
	}
}

// parseExportFilename extracts the filename from a Content-Disposition
// header like "filename=Clockify_Time_Report_Summary_11%2F15%2F2023-12%2F07%2F2023.pdf".
// Returns empty string if absent. Slashes arrive percent-encoded; we
// URL-decode them so callers see a sensible filename.
func parseExportFilename(cd string) string {
	if cd == "" {
		return ""
	}
	_, fn, found := strings.Cut(cd, "filename=")
	if !found {
		return ""
	}
	if before, _, hasSemi := strings.Cut(fn, ";"); hasSemi {
		fn = before
	}
	fn = strings.Trim(fn, `"`)
	if decoded, err := url.QueryUnescape(fn); err == nil {
		return decoded
	}
	return fn
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
	exportType, isJSON := sharedReportExportTypeFor(format)

	// Bare-id path (no workspace segment). The previously assumed
	// /workspaces/{ws}/shared-reports/{id}/export route returns 404
	// — there is no /export segment. Live-confirmed 2026-05-03; see
	// fixtures/shared-reports/discover_ws-prefixed-export.json
	// (404) and export-{pdf,csv,xlsx}.{headers.txt,binary-summary.json}.
	path := "/shared-reports/" + reportID
	query := map[string]string{"exportType": exportType}

	if isJSON {
		var export map[string]any
		if err := s.Client.GetReports(ctx, path, query, &export); err != nil {
			return ResultEnvelope{}, err
		}
		return ok("clockify_export_shared_report", export, map[string]any{
			"workspaceId": wsID,
			"reportId":    reportID,
			"format":      "json",
			"exportType":  exportType,
		}), nil
	}

	raw, err := s.Client.GetReportsRaw(ctx, path, query)
	if err != nil {
		return ResultEnvelope{}, err
	}
	contentType := raw.Header.Get("Content-Type")
	filename := parseExportFilename(raw.Header.Get("Content-Disposition"))
	envelope := map[string]any{
		"contentType": contentType,
		"filename":    filename,
		"bytes":       len(raw.Body),
		"body":        base64.StdEncoding.EncodeToString(raw.Body),
	}
	return ok("clockify_export_shared_report", envelope, map[string]any{
		"workspaceId": wsID,
		"reportId":    reportID,
		"format":      strings.ToLower(format),
		"exportType":  exportType,
	}), nil
}
