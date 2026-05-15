package tools

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

func customFieldHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. List custom fields
		{
			Tool: toolRO("clockify_list_custom_fields",
				"List custom fields in the workspace with optional pagination",
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"page":      map[string]any{"type": "integer", "description": "Page number (default 1)"},
						"page_size": map[string]any{"type": "integer", "description": "Items per page (default 50)"},
					},
				}),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.ListCustomFields(ctx, args)
			},
		},
		// 2. Get custom field by ID
		{
			Tool: toolRO("clockify_get_custom_field",
				"Get a custom field by ID",
				requiredSchema("field_id")),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.GetCustomField(ctx, args)
			},
		},
		// 3. Create custom field
		{
			Tool: toolRW("clockify_create_custom_field",
				"Create a custom field. Valid type values are TXT, NUMBER, DROPDOWN_SINGLE, DROPDOWN_MULTIPLE, CHECKBOX, or LINK. allowed_values is required for dropdown fields.",
				map[string]any{
					"type":     "object",
					"required": []string{"name", "field_type"},
					"properties": map[string]any{
						"name": map[string]any{"type": "string", "description": "Field name"},
						"field_type": map[string]any{
							"type":        "string",
							"description": "Upstream-accepted custom-field type",
							"enum":        []string{"TXT", "NUMBER", "DROPDOWN_SINGLE", "DROPDOWN_MULTIPLE", "CHECKBOX", "LINK"},
						},
						"allowed_values":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Allowed values; required for DROPDOWN_SINGLE and DROPDOWN_MULTIPLE"},
						"required":                map[string]any{"type": "boolean", "description": "Whether the field is required"},
						"status":                  customFieldStatusSchema("Custom-field visibility; defaults to VISIBLE so values can be set immediately"),
						"workspace_default_value": map[string]any{"description": "Workspace-wide default value. Type follows field_type: TXT/LINK/DROPDOWN_SINGLE need a string, NUMBER needs a number, CHECKBOX needs a boolean, DROPDOWN_MULTIPLE needs a list of strings. Dropdown defaults must be a subset of allowed_values."},
					},
				}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.CreateCustomField(ctx, args)
			},
		},
		// 4. Update custom field
		{
			Tool: toolRW("clockify_update_custom_field",
				"Update an existing custom field by ID",
				map[string]any{
					"type":     "object",
					"required": []string{"field_id"},
					"properties": map[string]any{
						"field_id": map[string]any{"type": "string"},
						"name":     map[string]any{"type": "string"},
						"field_type": map[string]any{
							"type":        "string",
							"description": "Upstream-accepted custom-field type; defaults to the current field type because Clockify requires type on update",
							"enum":        []string{"TXT", "NUMBER", "DROPDOWN_SINGLE", "DROPDOWN_MULTIPLE", "CHECKBOX", "LINK"},
						},
						"allowed_values":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"required":                map[string]any{"type": "boolean"},
						"status":                  customFieldStatusSchema("Custom-field visibility"),
						"workspace_default_value": map[string]any{"description": "Workspace-wide default value. Type follows the (resolved) field_type: TXT/LINK/DROPDOWN_SINGLE need a string, NUMBER needs a number, CHECKBOX needs a boolean, DROPDOWN_MULTIPLE needs a list of strings. Dropdown defaults must be a subset of allowed_values."},
					},
				}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.UpdateCustomField(ctx, args)
			},
		},
		// 5. Delete custom field (destructive)
		{
			Tool: toolDestructive("clockify_delete_custom_field",
				"Delete a custom field by ID (supports dry_run preview)",
				map[string]any{
					"type":     "object",
					"required": []string{"field_id"},
					"properties": map[string]any{
						"field_id": map[string]any{"type": "string"},
						"dry_run":  map[string]any{"type": "boolean", "description": "Preview without deleting"},
					},
				}),
			ReadOnlyHint:    false,
			DestructiveHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.DeleteCustomField(ctx, args)
			},
		},
		// 6. Set custom field value on a project or entry
		{
			Tool: toolRW("clockify_set_custom_field_value",
				"Set a custom field value on a project or time entry. Project values use the documented custom-fields route; entry values preserve the existing entry and replace customFields.",
				map[string]any{
					"type":     "object",
					"required": []string{"field_id", "value"},
					"properties": map[string]any{
						"field_id":   map[string]any{"type": "string", "description": "Custom field ID"},
						"value":      map[string]any{"description": "Value to set (type depends on field_type)"},
						"project_id": map[string]any{"type": "string", "description": "Project ID (set value on project)"},
						"entry_id":   map[string]any{"type": "string", "description": "Time entry ID (set value on entry)"},
					},
				}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.SetCustomFieldValue(ctx, args)
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (s *Service) ListCustomFields(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
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

	path, err := paths.Workspace(wsID, "custom-fields")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out []map[string]any
	if err := s.Client.Get(ctx, path, query, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_list_custom_fields", out, map[string]any{
		"workspaceId": wsID,
		"count":       len(out),
		"page":        page,
		"pageSize":    pageSize,
	}), nil
}

func (s *Service) GetCustomField(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	fieldID := stringArg(args, "field_id")
	if err := resolve.ValidateID(fieldID, "field_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	field, scanned, err := s.findCustomFieldByID(ctx, wsID, fieldID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_get_custom_field", field, map[string]any{
		"workspaceId": wsID,
		"scanned":     scanned,
	}), nil
}

func (s *Service) findCustomFieldByID(ctx context.Context, wsID, fieldID string) (map[string]any, int, error) {
	path, err := paths.Workspace(wsID, "custom-fields")
	if err != nil {
		return nil, 0, err
	}
	var fields []map[string]any
	if err := s.Client.Get(ctx, path, nil, &fields); err != nil {
		return nil, 0, err
	}
	for _, field := range fields {
		if id, _ := field["id"].(string); id == fieldID {
			return field, len(fields), nil
		}
	}
	return nil, len(fields), fmt.Errorf("custom field %s not found", fieldID)
}

// validCustomFieldTypes lists the upstream-accepted enum for the
// custom-field `type` body field. The historical TEXT/DROPDOWN aliases
// are rejected by Clockify with code 3002 listing the accepted values
// — fail locally with a clearer message instead of round-tripping the
// 400.
var validCustomFieldTypes = map[string]bool{
	"TXT":               true,
	"NUMBER":            true,
	"DROPDOWN_SINGLE":   true,
	"DROPDOWN_MULTIPLE": true,
	"CHECKBOX":          true,
	"LINK":              true,
}

// validateCustomFieldDefaultValue enforces the per-type rules described
// in the canonical openapi CustomFieldValue schema: NUMBER requires a
// number, CHECKBOX requires a boolean, DROPDOWN_MULTIPLE requires a list
// of strings, and TXT/LINK/DROPDOWN_SINGLE require a string. For dropdown
// types the default must also be a subset of allowed_values.
//
// allowedValues may be nil for non-dropdown types; a nil/empty allowed
// list for dropdown types is permitted (the upstream itself enforces the
// allowed-list invariant after our schema check).
func validateCustomFieldDefaultValue(fieldType string, raw any, allowedValues []string) (any, error) {
	if raw == nil {
		return nil, nil
	}
	switch fieldType {
	case "TXT", "LINK":
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("workspace_default_value must be a string for %s, got %T", fieldType, raw)
		}
		return s, nil
	case "NUMBER":
		switch v := raw.(type) {
		case float64:
			return v, nil
		case int:
			return float64(v), nil
		case int64:
			return float64(v), nil
		case int32:
			return float64(v), nil
		default:
			return nil, fmt.Errorf("workspace_default_value must be a number for NUMBER, got %T", raw)
		}
	case "CHECKBOX":
		b, ok := raw.(bool)
		if !ok {
			return nil, fmt.Errorf("workspace_default_value must be a boolean for CHECKBOX, got %T", raw)
		}
		return b, nil
	case "DROPDOWN_SINGLE":
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("workspace_default_value must be a string for DROPDOWN_SINGLE, got %T", raw)
		}
		if len(allowedValues) > 0 && !slices.Contains(allowedValues, s) {
			return nil, fmt.Errorf("workspace_default_value %q must be one of allowed_values %v", s, allowedValues)
		}
		return s, nil
	case "DROPDOWN_MULTIPLE":
		var items []string
		switch v := raw.(type) {
		case []string:
			items = append(items, v...)
		case []any:
			for _, x := range v {
				s, ok := x.(string)
				if !ok {
					return nil, fmt.Errorf("workspace_default_value items must be strings for DROPDOWN_MULTIPLE, got %T in list", x)
				}
				items = append(items, s)
			}
		default:
			return nil, fmt.Errorf("workspace_default_value must be an array of strings for DROPDOWN_MULTIPLE, got %T", raw)
		}
		if len(allowedValues) > 0 {
			for _, item := range items {
				if !slices.Contains(allowedValues, item) {
					return nil, fmt.Errorf("workspace_default_value %q must be one of allowed_values %v", item, allowedValues)
				}
			}
		}
		return items, nil
	}
	return nil, fmt.Errorf("workspace_default_value cannot be validated for unsupported field_type %q", fieldType)
}

func allowedValuesAsStrings(raw []any) []string {
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

var validCustomFieldStatuses = map[string]bool{
	"VISIBLE":   true,
	"INVISIBLE": true,
	"INACTIVE":  true,
}

func customFieldStatusSchema(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
		"enum":        []string{"VISIBLE", "INVISIBLE", "INACTIVE"},
	}
}

func customFieldStatusArg(args map[string]any, fallback string) (string, error) {
	status := strings.ToUpper(strings.TrimSpace(firstNonEmptyString(stringArg(args, "status"), fallback)))
	if status == "" {
		return "", nil
	}
	if !validCustomFieldStatuses[status] {
		return "", fmt.Errorf("status %q is not one of VISIBLE, INVISIBLE, INACTIVE", status)
	}
	return status, nil
}

func (s *Service) CreateCustomField(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return ResultEnvelope{}, fmt.Errorf("name is required")
	}
	fieldType := strings.ToUpper(strings.TrimSpace(stringArg(args, "field_type")))
	if fieldType == "" {
		return ResultEnvelope{}, fmt.Errorf("field_type is required")
	}
	if !validCustomFieldTypes[fieldType] {
		return ResultEnvelope{}, fmt.Errorf("field_type %q is not one of TXT, NUMBER, DROPDOWN_SINGLE, DROPDOWN_MULTIPLE, CHECKBOX, LINK", fieldType)
	}
	allowedVals, _ := args["allowed_values"].([]any)
	if (fieldType == "DROPDOWN_SINGLE" || fieldType == "DROPDOWN_MULTIPLE") && len(allowedVals) == 0 {
		return ResultEnvelope{}, fmt.Errorf("allowed_values is required for %s", fieldType)
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{
		"name":   name,
		"type":   fieldType,
		"status": "VISIBLE",
	}
	if status, err := customFieldStatusArg(args, "VISIBLE"); err != nil {
		return ResultEnvelope{}, err
	} else if status != "" {
		body["status"] = status
	}
	if len(allowedVals) > 0 {
		body["allowedValues"] = allowedVals
	}
	if req, ok := args["required"].(bool); ok {
		body["required"] = req
	}
	if rawDefault, present := args["workspace_default_value"]; present {
		validated, err := validateCustomFieldDefaultValue(fieldType, rawDefault, allowedValuesAsStrings(allowedVals))
		if err != nil {
			return ResultEnvelope{}, err
		}
		body["workspaceDefaultValue"] = validated
	}

	path, err := paths.Workspace(wsID, "custom-fields")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out map[string]any
	if err := s.Client.Post(ctx, path, body, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_create_custom_field", out, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) UpdateCustomField(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	fieldID := stringArg(args, "field_id")
	if err := resolve.ValidateID(fieldID, "field_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	existing, _, err := s.findCustomFieldByID(ctx, wsID, fieldID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{}
	if name, _ := existing["name"].(string); name != "" {
		body["name"] = name
	}
	fieldType := strings.ToUpper(strings.TrimSpace(stringArg(args, "field_type")))
	if fieldType == "" {
		fieldType, _ = existing["type"].(string)
		fieldType = strings.ToUpper(strings.TrimSpace(fieldType))
	}
	if fieldType == "" {
		return ResultEnvelope{}, fmt.Errorf("field_type is required because Clockify requires type on custom-field update and the current field type could not be resolved")
	}
	if !validCustomFieldTypes[fieldType] {
		return ResultEnvelope{}, fmt.Errorf("field_type %q is not one of TXT, NUMBER, DROPDOWN_SINGLE, DROPDOWN_MULTIPLE, CHECKBOX, LINK", fieldType)
	}
	body["type"] = fieldType
	if vals, ok := existing["allowedValues"].([]any); ok {
		body["allowedValues"] = vals
	}
	if req, ok := existing["required"].(bool); ok {
		body["required"] = req
	}
	if status, err := customFieldStatusArg(args, firstReportString(existing, "status")); err != nil {
		return ResultEnvelope{}, err
	} else if status != "" {
		body["status"] = status
	}
	if name := stringArg(args, "name"); name != "" {
		body["name"] = name
	}
	if vals, ok := args["allowed_values"].([]any); ok {
		body["allowedValues"] = vals
	}
	if req, ok := args["required"].(bool); ok {
		body["required"] = req
	}
	if rawDefault, present := args["workspace_default_value"]; present {
		allowedVals, _ := body["allowedValues"].([]any)
		validated, err := validateCustomFieldDefaultValue(fieldType, rawDefault, allowedValuesAsStrings(allowedVals))
		if err != nil {
			return ResultEnvelope{}, err
		}
		body["workspaceDefaultValue"] = validated
	}

	path, err := paths.Workspace(wsID, "custom-fields", fieldID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out map[string]any
	if err := s.Client.Put(ctx, path, body, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_update_custom_field", out, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) DeleteCustomField(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	fieldID := stringArg(args, "field_id")
	if err := resolve.ValidateID(fieldID, "field_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	fieldPath, err := paths.Workspace(wsID, "custom-fields", fieldID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		field, _, err := s.findCustomFieldByID(ctx, wsID, fieldID)
		if err != nil {
			return ResultEnvelope{}, err
		}
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_custom_field",
			Data:   dryrun.WrapResult(field, "clockify_delete_custom_field"),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	if err := s.Client.Delete(ctx, fieldPath); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_delete_custom_field", map[string]any{
		"deleted": true,
		"fieldId": fieldID,
	}, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) SetCustomFieldValue(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	fieldID := stringArg(args, "field_id")
	if err := resolve.ValidateID(fieldID, "field_id"); err != nil {
		return ResultEnvelope{}, err
	}
	value := args["value"]
	if value == nil {
		return ResultEnvelope{}, fmt.Errorf("value is required")
	}

	projectID := stringArg(args, "project_id")
	entryID := stringArg(args, "entry_id")
	if projectID == "" && entryID == "" {
		return ResultEnvelope{}, fmt.Errorf("either project_id or entry_id is required")
	}
	if projectID != "" {
		if err := resolve.ValidateID(projectID, "project_id"); err != nil {
			return ResultEnvelope{}, err
		}
	}
	if entryID != "" {
		if err := resolve.ValidateID(entryID, "entry_id"); err != nil {
			return ResultEnvelope{}, err
		}
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	field, _, err := s.findCustomFieldByID(ctx, wsID, fieldID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if status := strings.ToUpper(firstReportString(field, "status")); status != "" && status != "VISIBLE" {
		return ResultEnvelope{}, fmt.Errorf("custom field %s has status %s; make it VISIBLE before setting values", fieldID, status)
	}

	if projectID != "" {
		path, err := paths.Workspace(wsID, "projects", projectID, "custom-fields", fieldID)
		if err != nil {
			return ResultEnvelope{}, err
		}
		body := map[string]any{"defaultValue": value}
		var out map[string]any
		if err := s.Client.Patch(ctx, path, body, &out); err != nil {
			return ResultEnvelope{}, err
		}
		return ok("clockify_set_custom_field_value", out, map[string]any{"workspaceId": wsID}), nil
	}

	entryPath, err := paths.Workspace(wsID, "time-entries", entryID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var existing clockify.TimeEntry
	if err := s.Client.Get(ctx, entryPath, nil, &existing); err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.requireCurrentUserEntry(ctx, existing); err != nil {
		return ResultEnvelope{}, err
	}
	previous := existing
	customFields, err := mergeCustomFieldValue(existing.CustomFieldValues, fieldID, value)
	if err != nil {
		return ResultEnvelope{}, err
	}
	existing.CustomFieldValues = customFields
	var updated clockify.TimeEntry
	if err := s.Client.Put(ctx, entryPath, timeEntryPutPayload(existing), &updated); err != nil {
		return ResultEnvelope{}, err
	}
	s.emitResourceUpdateWithState(entryResourceURI(wsID, updated.ID), updated)
	s.emitWeeklyReportsForEntryChange(ctx, wsID, &previous, &updated)
	view, financialMeta := s.enrichEntryView(ctx, wsID, updated)
	return ok("clockify_set_custom_field_value", view, withFinancialMeta(map[string]any{"workspaceId": wsID}, financialMeta)), nil
}

func mergeCustomFieldValue(existing any, fieldID string, value any) ([]map[string]any, error) {
	out := []map[string]any{}
	found := false
	appendField := func(field map[string]any) {
		next := maps.Clone(field)
		if id, _ := next["customFieldId"].(string); id == fieldID {
			next["value"] = value
			found = true
		}
		out = append(out, next)
	}
	switch fields := existing.(type) {
	case nil:
	case []map[string]any:
		for _, field := range fields {
			appendField(field)
		}
	case []any:
		for _, item := range fields {
			field, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("existing custom field values must contain only objects")
			}
			appendField(field)
		}
	default:
		return nil, fmt.Errorf("existing custom field values had unsupported shape %T", existing)
	}
	if !found {
		out = append(out, map[string]any{"customFieldId": fieldID, "value": value})
	}
	return out, nil
}
