package tools

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/apet97/go-clockify/internal/mcp"
)

func (s *Service) rawAPIDescriptors() []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		firstSliceDescriptor(9000, toolRO("clockify_api_get", "Raw GET fallback for documented Clockify endpoints. Path must stay within /user or the pinned workspace (/workspaces/{workspaceId}/...); other workspaces and hosts are rejected.", objectSchema(map[string]any{"required": []string{"path"}, "properties": map[string]any{
			"path":  map[string]any{"type": "string"},
			"query": map[string]any{"type": "object", "additionalProperties": true},
		}})), s.RawAPIGet),
		firstSliceDescriptor(9001, toolRW("clockify_api_request", "Raw method fallback for documented Clockify endpoints. Path must stay within /user or the pinned workspace (/workspaces/{workspaceId}/...); other workspaces and hosts are rejected.", objectSchema(map[string]any{"required": []string{"method", "path"}, "properties": map[string]any{
			"method": map[string]any{"type": "string", "enum": []string{"GET", "POST", "PUT", "PATCH", "DELETE"}},
			"path":   map[string]any{"type": "string"},
			"query":  map[string]any{"type": "object", "additionalProperties": true},
			"body":   map[string]any{"type": "object", "additionalProperties": true},
		}})), s.RawAPIRequest),
	}
}

func (s *Service) RawAPIGet(ctx context.Context, args map[string]any) (any, error) {
	return s.rawAPI(ctx, "GET", args)
}

func (s *Service) RawAPIRequest(ctx context.Context, args map[string]any) (any, error) {
	method := strings.ToUpper(strings.TrimSpace(stringArg(args, "method")))
	if method == "" {
		return nil, fmt.Errorf("method is required")
	}
	return s.rawAPI(ctx, method, args)
}

func (s *Service) rawAPI(ctx context.Context, method string, args map[string]any) (any, error) {
	if method != "GET" && (s == nil || !s.EnableRawWrites) {
		return nil, fmt.Errorf("raw API writes are disabled; use workflow or domain tools first, or set CLOCKIFY_ENABLE_RAW_WRITES=true to allow %s", method)
	}
	path, err := safeRawPath(s.WorkspaceID, stringArg(args, "path"))
	if err != nil {
		return nil, err
	}
	if method != "GET" && s.RawWriteDocumentedOnly && !isDocumentedRawWriteRoute(method, path) {
		return nil, fmt.Errorf("raw write %s %s is not a documented Clockify endpoint; use a typed domain tool if one exists, or set CLOCKIFY_RAW_WRITE_DOCUMENTED_ONLY=false only for endpoints you have confirmed are documented or probed", method, path)
	}
	query := rawQuery(args["query"])
	body, _ := args["body"].(map[string]any)
	var data any
	switch method {
	case "GET":
		err = s.Client.Get(ctx, path, query, &data)
	case "POST":
		err = s.Client.Post(ctx, path, body, &data)
	case "PUT":
		err = s.Client.Put(ctx, path, body, &data)
	case "PATCH":
		err = s.Client.Patch(ctx, path, body, &data)
	case "DELETE":
		var deleted any
		err = s.Client.DeleteWithQueryCapture(ctx, path, query, &deleted)
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
	for key, value := range idsFromData(data, "raw_api") {
		ids[key] = value
	}
	return result(action, "raw_api", ids, data, changedFor(rawChange(method), "raw_api", data, ids), nil, nil), nil
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

func rawQuery(raw any) map[string]string {
	m, ok := raw.(map[string]any)
	if !ok || len(m) == 0 {
		return nil
	}
	query := make(map[string]string, len(m))
	for key, value := range m {
		query[key] = scalarToString(value)
	}
	return query
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
