package tools

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"strings"

	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/safety"
)

func (s *Service) rawAPIDescriptors() []mcp.ToolDescriptor {
	rawRequest := firstSliceDescriptor(9001, toolRW("clockify_api_request", "Raw method fallback for documented Clockify endpoints. Report paths (/workspaces/{workspaceId}/reports*, /workspaces/{workspaceId}/shared-reports) route to reports.api.clockify.me; other paths route to api.clockify.me. Path must stay within /user or the pinned workspace (/workspaces/{workspaceId}/...); other workspaces and hosts are rejected. Raw PATCH to reports paths is unsupported.", objectSchema(map[string]any{"required": []string{"method", "path"}, "properties": map[string]any{
		"method":        map[string]any{"type": "string", "enum": []string{"GET", "POST", "PUT", "PATCH", "DELETE"}},
		"path":          map[string]any{"type": "string"},
		"query":         map[string]any{"type": "object", "additionalProperties": true, "description": "Query object. Values may be string, number, boolean, or an array of those; arrays become repeated query parameters (status=active&status=archived)."},
		"body":          map[string]any{"type": "object", "additionalProperties": true},
		"dry_run":       map[string]any{"type": "boolean", "description": "Preview a raw mutating request without calling Clockify."},
		"confirm_token": map[string]any{"type": "string", "description": "Short-lived token returned by a dry_run preview for this exact raw request.", "minLength": 16, "maxLength": 512},
	}})), s.RawAPIRequest)
	rawRequest.SafetyRequirementFunc = func(args map[string]any) safety.Requirement {
		method := strings.ToUpper(strings.TrimSpace(stringArg(args, "method")))
		return safety.RequirementForRisk([]string{"write"}, false, method)
	}
	return []mcp.ToolDescriptor{
		firstSliceDescriptor(9000, toolRO("clockify_api_get", "Raw GET fallback for documented Clockify endpoints. Report paths (/workspaces/{workspaceId}/reports*, /workspaces/{workspaceId}/shared-reports) route to reports.api.clockify.me; other paths route to api.clockify.me. Path must stay within /user or the pinned workspace (/workspaces/{workspaceId}/...); other workspaces and hosts are rejected.", objectSchema(map[string]any{"required": []string{"path"}, "properties": map[string]any{
			"path":  map[string]any{"type": "string"},
			"query": map[string]any{"type": "object", "additionalProperties": true, "description": "Query object. Values may be string, number, boolean, or an array of those; arrays become repeated query parameters (status=active&status=archived)."},
		}})), s.RawAPIGet),
		rawRequest,
	}
}

// RawAPIGet handles clockify_api_get: the raw GET fallback for documented
// Clockify endpoints, workspace-fenced to /user or the pinned workspace.
func (s *Service) RawAPIGet(ctx context.Context, args map[string]any) (any, error) {
	return s.rawAPI(ctx, "GET", args)
}

// RawAPIRequest handles the raw method/path fallback tool: it requires an
// explicit HTTP method in args and dispatches through the workspace-fenced raw
// API path, subject to the raw-tools and raw-write enablement gates.
func (s *Service) RawAPIRequest(ctx context.Context, args map[string]any) (any, error) {
	method := strings.ToUpper(strings.TrimSpace(stringArg(args, "method")))
	if method == "" {
		return nil, fmt.Errorf("method is required")
	}
	return s.rawAPI(ctx, method, args)
}

func (s *Service) rawAPI(ctx context.Context, method string, args map[string]any) (any, error) {
	if s == nil || !s.EnableRawTools {
		return nil, fmt.Errorf("raw API tools are disabled; use workflow or domain tools first, or set CLOCKIFY_ENABLE_RAW_TOOLS=true")
	}
	if method == "GET" && !s.EnableRawGet {
		return nil, fmt.Errorf("raw API GET is disabled; use workflow or domain tools first, or set CLOCKIFY_ENABLE_RAW_GET=true")
	}
	if method != "GET" && !s.EnableRawWrites {
		return nil, fmt.Errorf("raw API writes are disabled; use workflow or domain tools first, or set CLOCKIFY_ENABLE_RAW_WRITES=true to allow %s", method)
	}
	path, err := safeRawPath(s.WorkspaceID, stringArg(args, "path"))
	if err != nil {
		return nil, err
	}
	if method == "GET" && sensitiveRawReadPath(path) && !rawSensitiveReadToolsetAllowed(s.Toolset) {
		return nil, fmt.Errorf("raw GET %s may expose sensitive workspace data; use a typed tool, or select CLOCKIFY_TOOLSET=admin or all", path)
	}
	if method != "GET" && s.RawWriteDocumentedOnly && !isDocumentedRawWriteRoute(method, path) {
		return nil, fmt.Errorf("raw write %s %s is not a documented Clockify endpoint; use a typed domain tool if one exists, or set CLOCKIFY_RAW_WRITE_DOCUMENTED_ONLY=false only for endpoints you have confirmed are documented or probed", method, path)
	}
	query := rawQuery(args["query"])
	body, _ := args["body"].(map[string]any)
	if method != "GET" && boolArg(args, "dry_run") {
		data := map[string]any{
			"dry_run":          true,
			"method":           method,
			"path":             path,
			"query_hash":       safety.HashCanonical(query.Encode()),
			"body_hash":        safety.HashCanonical(body),
			"documented_route": !s.RawWriteDocumentedOnly || isDocumentedRawWriteRoute(method, path),
		}
		return result(actionForRawMethod(method), "raw_api", map[string]string{"workspaceId": s.WorkspaceID}, data, ChangeSet{}, []Warning{{Code: "dry_run", Message: "No raw API request was sent to Clockify."}}, nil), nil
	}
	// Report and shared-report paths live on the reports host. Routing them
	// through the main-host verbs returns a 404, so dispatch via the
	// reports-host helpers instead.
	reportsHost := isReportsPath(path)
	var data any
	switch method {
	case "GET":
		if reportsHost {
			err = s.Client.GetReportsValues(ctx, path, query, &data)
		} else {
			err = s.Client.GetValues(ctx, path, query, &data)
		}
	case "POST":
		if reportsHost {
			err = s.Client.PostReports(ctx, path, body, &data)
		} else {
			err = s.Client.Post(ctx, path, body, &data)
		}
	case "PUT":
		if reportsHost {
			err = s.Client.PutReports(ctx, path, body, &data)
		} else {
			err = s.Client.Put(ctx, path, body, &data)
		}
	case "PATCH":
		if reportsHost {
			// The Clockify reports host has no PATCH endpoint, so there is no
			// reports-host helper to route through. Reject deterministically
			// instead of issuing a PATCH that the main host would 404.
			return nil, fmt.Errorf("unsupported: raw PATCH to reports endpoints is not implemented; use the typed clockify_reports_* tools")
		}
		err = s.Client.Patch(ctx, path, body, &data)
	case "DELETE":
		var deleted any
		if reportsHost {
			err = s.Client.DeleteReportsCaptureValues(ctx, path, query, &deleted)
		} else {
			err = s.Client.DeleteWithQueryCaptureValues(ctx, path, query, &deleted)
		}
		if deleted != nil {
			data = deleted
		} else {
			data = map[string]any{"deleted": true}
		}
	default:
		return nil, fmt.Errorf("unsupported method %s", method)
	}
	if err != nil {
		return nil, err
	}
	action := "clockify_api_request"
	if method == "GET" {
		action = "clockify_api_get"
	}
	ids := map[string]string{"workspaceId": s.WorkspaceID}
	maps.Copy(ids, idsFromData(data, "raw_api"))
	return result(action, "raw_api", ids, data, changedFor(rawChange(method), "raw_api", data, ids), nil, nil), nil
}

func actionForRawMethod(method string) string {
	if method == "GET" {
		return "clockify_api_get"
	}
	return "clockify_api_request"
}

func rawSensitiveReadToolsetAllowed(toolset string) bool {
	switch strings.ToLower(strings.TrimSpace(toolset)) {
	case "admin", "all":
		return true
	default:
		return false
	}
}

func sensitiveRawReadPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 || parts[0] != "workspaces" {
		return false
	}
	segments := map[string]bool{
		"approvals":          true,
		"audit-log":          true,
		"balances":           true,
		"billing":            true,
		"groups":             true,
		"invoices":           true,
		"payments":           true,
		"settings":           true,
		"time-off":           true,
		"users":              true,
		"webhooks":           true,
		"workspace-settings": true,
	}
	for _, segment := range parts[2:] {
		if segments[segment] {
			return true
		}
	}
	return false
}

func safeRawPath(workspaceID, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("path is required")
	}
	raw = strings.ReplaceAll(raw, "{workspaceId}", workspaceID)
	raw = strings.TrimPrefix(raw, "/api/v1")
	raw = strings.TrimPrefix(raw, "/v1")
	if strings.Contains(raw, "://") || strings.Contains(raw, "\\") || strings.Contains(raw, "..") || strings.ContainsAny(raw, "?#") {
		return "", fmt.Errorf("raw API path must be a relative Clockify API path")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.IsAbs() || u.Host != "" {
		return "", fmt.Errorf("raw API path must not include a host")
	}
	path, err := url.PathUnescape(u.EscapedPath())
	if err != nil {
		return "", fmt.Errorf("raw API path must be a valid escaped path: %w", err)
	}
	for _, r := range path {
		if r < 0x20 || r == 0x7F {
			return "", fmt.Errorf("raw API path must not contain control characters")
		}
	}
	if strings.Contains(path, "\\") {
		return "", fmt.Errorf("raw API path must be a relative Clockify API path")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	for i, segment := range strings.Split(path, "/") {
		if i > 0 && segment == "" {
			return "", fmt.Errorf("raw API path must not contain duplicated slashes")
		}
		if segment == "." || segment == ".." {
			return "", fmt.Errorf("raw API path must not contain dot segments")
		}
	}
	if !strings.HasPrefix(path, "/workspaces/"+workspaceID+"/") && path != "/workspaces/"+workspaceID && path != "/user" {
		return "", fmt.Errorf("raw API path must stay within /user or the pinned workspace API")
	}
	return path, nil
}

// rawQuery converts the user-supplied query object into url.Values so repeated
// keys survive. A scalar (string/number/bool) becomes a single value; an array
// becomes repeated values for the same key (status=active&status=archived); an
// empty array contributes no values. A map[string]string would silently collapse
// arrays into a comma-joined scalar, which breaks Clockify filters that expect
// repeated keys.
func rawQuery(raw any) url.Values {
	m, ok := raw.(map[string]any)
	if !ok || len(m) == 0 {
		return nil
	}
	query := url.Values{}
	for key, value := range m {
		if items, ok := value.([]any); ok {
			for _, item := range items {
				query.Add(key, scalarToString(item))
			}
			continue
		}
		query.Add(key, scalarToString(value))
	}
	if len(query) == 0 {
		return nil
	}
	return query
}

// isReportsPath reports whether a raw path targets the Clockify reports host.
// Report and shared-report paths live on reports.api.clockify.me, not the main
// api.clockify.me host; routing them to the main host returns a 404.
func isReportsPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// Path shape: /workspaces/{wsId}/reports/... or
	// /workspaces/{wsId}/shared-reports/...
	return len(parts) >= 3 && parts[0] == "workspaces" &&
		(parts[2] == "reports" || parts[2] == "shared-reports")
}

func rawChange(method string) string {
	switch method {
	case "POST":
		return "created"
	case "PUT", "PATCH":
		return "updated"
	case "DELETE":
		return "deleted"
	default:
		return ""
	}
}
