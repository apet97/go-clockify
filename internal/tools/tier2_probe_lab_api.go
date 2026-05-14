package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
)

type documentedAPIOperation struct {
	Method          string
	Path            string
	OperationID     string
	LiveStatus      string
	RiskClass       []string
	SourceFiles     []string
	PromotedToTyped []string
}

func probeLabAPIHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		{Tool: toolRO("clockify_list_documented_api_operations", "List the allowlisted method/path operations from the probe-lab OpenAPI and DOC source set.", documentedAPIListSchema()), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ListDocumentedAPIOperations(ctx, args)
		}},
		{Tool: toolRO("clockify_call_documented_read_api", "Call an allowlisted documented GET endpoint. Use operation from clockify_list_documented_api_operations; path_params fill template variables.", documentedAPICallSchema([]string{http.MethodGet}, false)), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.CallDocumentedReadAPI(ctx, args)
		}},
		{Tool: toolRW("clockify_call_documented_write_api", "Call an allowlisted documented POST, PUT, or PATCH endpoint. Supports json_body, form_body, query, raw_response, and dry_run.", documentedAPICallSchema([]string{http.MethodPost, http.MethodPut, http.MethodPatch}, true)), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.CallDocumentedWriteAPI(ctx, args)
		}},
		{Tool: toolDestructive("clockify_call_documented_delete_api", "Call an allowlisted documented DELETE endpoint. Supports json_body, query, and dry_run.", documentedAPICallSchema([]string{http.MethodDelete}, true)), ReadOnlyHint: false, DestructiveHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.CallDocumentedDeleteAPI(ctx, args)
		}},
		{Tool: toolRW("clockify_upload_image", "Upload an image file through Clockify's documented /file/image multipart endpoint.", uploadImageSchema()), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.UploadImage(ctx, args)
		}},
	}
}

func uploadImageSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"filename":     map[string]any{"type": "string", "description": "Original filename sent in the multipart file part."},
			"content_type": map[string]any{"type": "string", "description": "MIME type for the file part, for example image/png."},
			"data_base64":  map[string]any{"type": "string", "description": "Base64-encoded file bytes. Files are bounded by the client multipart cap."},
			"dry_run":      map[string]any{"type": "boolean", "description": "Preview the upload metadata without sending bytes upstream."},
		},
		"required": []string{"filename", "data_base64"},
	}
}

func documentedAPIListSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"method":   map[string]any{"type": "string", "enum": []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}},
			"contains": map[string]any{"type": "string", "description": "Case-insensitive substring filter against METHOD path."},
		},
	}
}

func documentedAPICallSchema(methods []string, allowDryRun bool) map[string]any {
	props := map[string]any{
		"operation":    map[string]any{"type": "string", "description": "Exact allowlisted operation string, e.g. GET /workspaces/{workspaceId}/users/{userId}/managers."},
		"method":       map[string]any{"type": "string", "enum": methods, "description": "Alternative to operation; must be paired with path."},
		"path":         map[string]any{"type": "string", "description": "Alternative to operation; documented path template, e.g. /workspaces/{workspaceId}/clients."},
		"workspace_id": map[string]any{"type": "string", "description": "Overrides the configured workspace_id when the path has {workspaceId}."},
		"path_params": map[string]any{
			"type":                 "object",
			"additionalProperties": map[string]any{"type": "string"},
			"description":          "Values for non-workspace path template variables, keyed by placeholder name.",
		},
		"query": map[string]any{
			"type":                 "object",
			"additionalProperties": true,
			"description":          "Query parameters. Array values are sent as repeated query keys.",
		},
		"json_body": map[string]any{
			"type":                 "object",
			"additionalProperties": true,
			"description":          "JSON request body sent as-is after method/path allowlist validation.",
		},
		"form_body": map[string]any{
			"type":                 "object",
			"additionalProperties": true,
			"description":          "Multipart form fields. Array values are sent as repeated fields.",
		},
		"files": map[string]any{
			"type":        "array",
			"description": "Multipart file parts as base64 objects. Each item has field, filename, content_type, and data_base64.",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"field":        map[string]any{"type": "string"},
					"filename":     map[string]any{"type": "string"},
					"content_type": map[string]any{"type": "string"},
					"data_base64":  map[string]any{"type": "string"},
				},
				"required": []string{"field", "filename", "data_base64"},
			},
		},
		"raw_response": map[string]any{"type": "boolean", "description": "Return base64 body and response headers instead of JSON-decoding."},
	}
	if allowDryRun {
		props["dry_run"] = map[string]any{"type": "boolean", "description": "Preview the resolved request without sending it upstream."}
	}
	return map[string]any{
		"type":       "object",
		"properties": props,
		"anyOf": []any{
			map[string]any{"required": []string{"operation"}},
			map[string]any{"required": []string{"method", "path"}},
		},
	}
}

func (s *Service) ListDocumentedAPIOperations(_ context.Context, args map[string]any) (ResultEnvelope, error) {
	methodFilter := strings.ToUpper(strings.TrimSpace(stringArg(args, "method")))
	contains := strings.ToLower(strings.TrimSpace(stringArg(args, "contains")))
	operations := make([]map[string]any, 0, len(documentedAPIOperations))
	for _, op := range documentedAPIOperations {
		if methodFilter != "" && op.Method != methodFilter {
			continue
		}
		key := op.key()
		if contains != "" && !strings.Contains(strings.ToLower(key), contains) {
			continue
		}
		operations = append(operations, map[string]any{
			"operation":         key,
			"operation_id":      op.OperationID,
			"method":            op.Method,
			"path":              op.Path,
			"reports_host":      op.reportsHost(),
			"live_status":       op.LiveStatus,
			"risk_class":        op.RiskClass,
			"source_files":      op.SourceFiles,
			"promoted_to_typed": op.PromotedToTyped,
		})
	}
	return ok("clockify_list_documented_api_operations", operations, map[string]any{
		"count":      len(operations),
		"total":      len(documentedAPIOperations),
		"source":     "openapi-docs",
		"sourceRows": "docs/openapi/sources/{realOPENAPI,AIII,clockify-api-probe-lab,newfolderwithfindings}",
	}), nil
}

func (s *Service) CallDocumentedReadAPI(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	return s.callDocumentedAPI(ctx, args, "clockify_call_documented_read_api", map[string]bool{http.MethodGet: true})
}

func (s *Service) CallDocumentedWriteAPI(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	return s.callDocumentedAPI(ctx, args, "clockify_call_documented_write_api", map[string]bool{
		http.MethodPost:  true,
		http.MethodPut:   true,
		http.MethodPatch: true,
	})
}

func (s *Service) CallDocumentedDeleteAPI(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	return s.callDocumentedAPI(ctx, args, "clockify_call_documented_delete_api", map[string]bool{http.MethodDelete: true})
}

func (s *Service) callDocumentedAPI(ctx context.Context, args map[string]any, toolName string, allowed map[string]bool) (ResultEnvelope, error) {
	requested, err := documentedOperationFromArgs(args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if !allowed[requested.Method] {
		return ResultEnvelope{}, fmt.Errorf("%s does not allow %s operations", toolName, requested.Method)
	}
	if requested.Method != http.MethodGet && !s.DocumentedAPIWrites && !dryrun.Enabled(args) {
		return ResultEnvelope{}, fmt.Errorf("documented API write/delete calls are disabled; set CLOCKIFY_DOCUMENTED_API_WRITES=1 to enable this raw escape hatch")
	}
	op, found := documentedOperationByKey(requested.key())
	if !found {
		return ResultEnvelope{}, fmt.Errorf("operation %q is not in the documented Clockify allowlist", requested.key())
	}

	pathParams, err := documentedStringMap(args, "path_params")
	if err != nil {
		return ResultEnvelope{}, err
	}
	wsID := strings.TrimSpace(stringArg(args, "workspace_id"))
	resolvedPath, wsID, err := s.resolveDocumentedAPIPath(ctx, op.Path, wsID, pathParams)
	if err != nil {
		return ResultEnvelope{}, err
	}
	query, err := documentedValuesArg(args, "query")
	if err != nil {
		return ResultEnvelope{}, err
	}
	jsonBody, hasJSON := args["json_body"]
	formBody, hasForm, err := documentedFormValuesArg(args, "form_body")
	if err != nil {
		return ResultEnvelope{}, err
	}
	files, hasFiles, err := documentedFilesArg(args, "files")
	if err != nil {
		return ResultEnvelope{}, err
	}
	if hasJSON && (hasForm || hasFiles) {
		return ResultEnvelope{}, fmt.Errorf("json_body is mutually exclusive with form_body/files")
	}
	if hasJSON && jsonBody == nil {
		hasJSON = false
	}

	meta := map[string]any{
		"source":      "probe-lab-api",
		"operation":   op.key(),
		"method":      op.Method,
		"path":        resolvedPath,
		"reportsHost": op.reportsHost(),
	}
	if wsID != "" {
		meta["workspaceId"] = wsID
	}
	if dryrun.Enabled(args) && op.Method != http.MethodGet {
		preview := map[string]any{
			"method":       op.Method,
			"path":         resolvedPath,
			"query":        query,
			"reports_host": op.reportsHost(),
		}
		if hasJSON {
			preview["json_body"] = jsonBody
		}
		if hasForm {
			preview["form_body"] = formBody
		}
		if hasFiles {
			preview["files"] = documentedFilePreview(files)
		}
		return ok(toolName, dryrun.Preview(toolName, preview), meta), nil
	}

	if boolArg(args, "raw_response") {
		raw, err := s.Client.RequestRawValues(ctx, op.reportsHost(), op.Method, resolvedPath, query, documentedRequestBody(hasJSON, jsonBody))
		if err != nil {
			return ResultEnvelope{}, err
		}
		return ok(toolName, documentedRawResponse(raw.Header, raw.Body), meta), nil
	}

	var out any
	if hasForm || hasFiles {
		if err := s.Client.RequestMultipartValuesWithFiles(ctx, op.reportsHost(), op.Method, resolvedPath, query, formBody, files, &out); err != nil {
			return ResultEnvelope{}, err
		}
	} else if err := s.Client.RequestJSONValues(ctx, op.reportsHost(), op.Method, resolvedPath, query, documentedRequestBody(hasJSON, jsonBody), &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok(toolName, out, meta), nil
}

func (s *Service) UploadImage(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	filename := strings.TrimSpace(stringArg(args, "filename"))
	if filename == "" {
		return ResultEnvelope{}, fmt.Errorf("filename is required")
	}
	contentType := strings.TrimSpace(stringArg(args, "content_type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(stringArg(args, "data_base64")))
	if err != nil {
		return ResultEnvelope{}, fmt.Errorf("data_base64 must be valid base64: %w", err)
	}
	meta := map[string]any{
		"filename":    filename,
		"contentType": contentType,
		"bytes":       len(data),
	}
	if dryrun.Enabled(args) {
		return ok("clockify_upload_image", dryrun.Preview("clockify_upload_image", meta), meta), nil
	}
	var out any
	if err := s.Client.PostMultipartWithFiles(ctx, "/file/image", nil, []clockify.MultipartFile{{
		FieldName:   "file",
		Filename:    filename,
		ContentType: contentType,
		Data:        data,
	}}, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_upload_image", out, meta), nil
}

func documentedRequestBody(hasBody bool, body any) any {
	if !hasBody {
		return nil
	}
	return body
}

func documentedRawResponse(header http.Header, body []byte) map[string]any {
	contentType := strings.TrimSpace(header.Get("Content-Type"))
	inferred := inferRawContentType(body)
	if contentType == "" || (inferred != "" && strings.Contains(strings.ToLower(contentType), "json")) {
		contentType = inferred
	}
	filename := parseExportFilename(header.Get("Content-Disposition"))
	if filename == "" && contentType == "application/pdf" {
		filename = "document.pdf"
	}
	return map[string]any{
		"contentType": contentType,
		"filename":    filename,
		"bytes":       len(body),
		"body":        base64.StdEncoding.EncodeToString(body),
	}
}

func inferRawContentType(body []byte) string {
	if len(body) >= 4 && string(body[:4]) == "%PDF" {
		return "application/pdf"
	}
	return http.DetectContentType(body)
}

func documentedOperationFromArgs(args map[string]any) (documentedAPIOperation, error) {
	if raw := strings.TrimSpace(stringArg(args, "operation")); raw != "" {
		method, path, ok := strings.Cut(raw, " ")
		if !ok {
			return documentedAPIOperation{}, fmt.Errorf("operation must be formatted as METHOD /path")
		}
		return documentedAPIOperation{
			Method: strings.ToUpper(strings.TrimSpace(method)),
			Path:   normalizeDocumentedAPIPath(strings.TrimSpace(path)),
		}, nil
	}
	method := strings.ToUpper(strings.TrimSpace(stringArg(args, "method")))
	path := normalizeDocumentedAPIPath(strings.TrimSpace(stringArg(args, "path")))
	if method == "" || path == "" {
		return documentedAPIOperation{}, fmt.Errorf("operation is required, or method and path must both be supplied")
	}
	return documentedAPIOperation{Method: method, Path: path}, nil
}

func normalizeDocumentedAPIPath(path string) string {
	if strings.HasPrefix(path, "/api/v1/") {
		path = strings.TrimPrefix(path, "/api/v1")
	} else if strings.HasPrefix(path, "/v1/") {
		path = strings.TrimPrefix(path, "/v1")
	}
	return canonicalDocumentedAPIPath(path)
}

func canonicalDocumentedAPIPath(path string) string {
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
		previous := ""
		if i > 0 {
			previous = segments[i-1]
		}
		switch name {
		case "ws", "wsId", "workspace", "workspace_id", "workspaceId":
			name = "workspaceId"
		case "uid", "user", "user_id", "userId":
			name = "userId"
		default:
			name = canonicalDocumentedParam(previous, name)
		}
		segments[i] = "{" + name + "}"
	}
	return strings.Join(segments, "/")
}

func canonicalDocumentedParam(segment, fallback string) string {
	switch segment {
	case "addons":
		return "addonId"
	case "approval-requests":
		return "approvalId"
	case "assignments":
		return "assignmentId"
	case "categories":
		return "categoryId"
	case "clients":
		return "clientId"
	case "custom-fields":
		return "customFieldId"
	case "expenses":
		return "expenseId"
	case "files":
		return "fileId"
	case "holidays":
		return "holidayId"
	case "invoices":
		return "invoiceId"
	case "items":
		return "order"
	case "payments":
		return "paymentId"
	case "policies":
		return "policyId"
	case "projects":
		return "projectId"
	case "requests":
		return "requestId"
	case "shared-reports":
		return "sharedReportId"
	case "tags":
		return "tagId"
	case "tasks":
		return "taskId"
	case "time-entries":
		return "timeEntryId"
	case "user":
		return "userId"
	case "user-groups":
		return "groupId"
	case "users":
		return "userId"
	case "webhooks":
		return "webhookId"
	default:
		return fallback
	}
}

func documentedOperationByKey(key string) (documentedAPIOperation, bool) {
	for _, op := range documentedAPIOperations {
		if op.key() == key {
			return documentedAPIOperation{Method: op.Method, Path: normalizeDocumentedAPIPath(op.Path)}, true
		}
	}
	return documentedAPIOperation{}, false
}

func (op documentedAPIOperation) key() string {
	return op.Method + " " + normalizeDocumentedAPIPath(op.Path)
}

func (op documentedAPIOperation) reportsHost() bool {
	return strings.Contains(op.Path, "/reports/") || strings.Contains(op.Path, "/shared-reports")
}

func (s *Service) resolveDocumentedAPIPath(ctx context.Context, template, workspaceID string, pathParams map[string]string) (string, string, error) {
	out := template
	for {
		start := strings.Index(out, "{")
		if start < 0 {
			break
		}
		end := strings.Index(out[start:], "}")
		if end < 0 {
			return "", workspaceID, fmt.Errorf("malformed documented path template %q", template)
		}
		end += start
		name := out[start+1 : end]
		value := ""
		if name == "workspaceId" {
			value = strings.TrimSpace(workspaceID)
			if value == "" {
				var err error
				value, err = s.ResolveWorkspaceID(ctx)
				if err != nil {
					return "", workspaceID, err
				}
				workspaceID = value
			}
		} else {
			value = strings.TrimSpace(pathParams[name])
		}
		if value == "" {
			return "", workspaceID, fmt.Errorf("path_params.%s is required for %s", name, template)
		}
		out = out[:start] + url.PathEscape(value) + out[end+1:]
	}
	return out, workspaceID, nil
}

func documentedStringMap(args map[string]any, key string) (map[string]string, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return map[string]string{}, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object", key)
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		text, err := documentedScalar(v)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", key, k, err)
		}
		out[k] = text
	}
	return out, nil
}

func documentedValuesArg(args map[string]any, key string) (url.Values, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return url.Values{}, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object", key)
	}
	return documentedValuesFromMap(key, m)
}

func documentedFormValuesArg(args map[string]any, key string) (url.Values, bool, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil, false, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, true, fmt.Errorf("%s must be an object", key)
	}
	values, err := documentedValuesFromMap(key, m)
	if err != nil {
		return nil, true, err
	}
	return values, true, nil
}

func documentedFilesArg(args map[string]any, key string) ([]clockify.MultipartFile, bool, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil, false, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, true, fmt.Errorf("%s must be an array", key)
	}
	files := make([]clockify.MultipartFile, 0, len(items))
	for i, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, true, fmt.Errorf("%s[%d] must be an object", key, i)
		}
		field := strings.TrimSpace(stringFromAny(m["field"]))
		filename := strings.TrimSpace(stringFromAny(m["filename"]))
		contentType := strings.TrimSpace(stringFromAny(m["content_type"]))
		dataRaw := strings.TrimSpace(stringFromAny(m["data_base64"]))
		if field == "" || filename == "" || dataRaw == "" {
			return nil, true, fmt.Errorf("%s[%d] requires field, filename, and data_base64", key, i)
		}
		data, err := base64.StdEncoding.DecodeString(dataRaw)
		if err != nil {
			return nil, true, fmt.Errorf("%s[%d].data_base64 must be valid base64: %w", key, i, err)
		}
		files = append(files, clockify.MultipartFile{
			FieldName:   field,
			Filename:    filename,
			ContentType: contentType,
			Data:        data,
		})
	}
	return files, true, nil
}

func documentedFilePreview(files []clockify.MultipartFile) []map[string]any {
	out := make([]map[string]any, 0, len(files))
	for _, file := range files {
		out = append(out, map[string]any{
			"field":        file.FieldName,
			"filename":     file.Filename,
			"content_type": file.ContentType,
			"bytes":        len(file.Data),
		})
	}
	return out
}

func stringFromAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func documentedValuesFromMap(parent string, m map[string]any) (url.Values, error) {
	values := url.Values{}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := m[k]
		switch typed := v.(type) {
		case []any:
			for i, item := range typed {
				text, err := documentedScalar(item)
				if err != nil {
					return nil, fmt.Errorf("%s.%s[%d]: %w", parent, k, i, err)
				}
				values.Add(k, text)
			}
		case []string:
			for _, item := range typed {
				values.Add(k, item)
			}
		default:
			text, err := documentedScalar(v)
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", parent, k, err)
			}
			values.Add(k, text)
		}
	}
	return values, nil
}

func documentedScalar(v any) (string, error) {
	switch typed := v.(type) {
	case string:
		return typed, nil
	case bool:
		return strconv.FormatBool(typed), nil
	case int:
		return strconv.Itoa(typed), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32), nil
	default:
		return "", fmt.Errorf("must be string, number, or boolean")
	}
}
