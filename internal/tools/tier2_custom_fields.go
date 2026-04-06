package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"goclmcp/internal/dryrun"
	"goclmcp/internal/mcp"
)

func init() {
	registerTier2Group(Tier2Group{
		Name:        "custom_fields",
		Description: "Custom metadata fields for entries and projects",
		Keywords:    []string{"custom", "field", "metadata", "dropdown", "value"},
		Builder:     customFieldHandlers,
	})
}

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
			ReadOnlyHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.ListCustomFields(ctx, args)
			},
		},
		// 2. Get custom field by ID
		{
			Tool: toolRO("clockify_get_custom_field",
				"Get a custom field by ID",
				requiredSchema("field_id")),
			ReadOnlyHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.GetCustomField(ctx, args)
			},
		},
		// 3. Create custom field
		{
			Tool: toolRW("clockify_create_custom_field",
				"Create a new custom field (TEXT, NUMBER, DROPDOWN, CHECKBOX, or LINK)",
				map[string]any{
					"type":     "object",
					"required": []string{"name", "field_type"},
					"properties": map[string]any{
						"name":           map[string]any{"type": "string", "description": "Field name"},
						"field_type":     map[string]any{"type": "string", "description": "One of: TEXT, NUMBER, DROPDOWN, CHECKBOX, LINK"},
						"allowed_values": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Allowed values for DROPDOWN type"},
						"required":       map[string]any{"type": "boolean", "description": "Whether the field is required"},
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
						"field_id":       map[string]any{"type": "string"},
						"name":           map[string]any{"type": "string"},
						"allowed_values": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"required":       map[string]any{"type": "boolean"},
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
				"Set a custom field value on a specific project or time entry",
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

	var out []map[string]any
	if err := s.Client.Get(ctx, "/workspaces/"+wsID+"/custom-fields", query, &out); err != nil {
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
	if fieldID == "" {
		return ResultEnvelope{}, fmt.Errorf("field_id is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	var out map[string]any
	if err := s.Client.Get(ctx, "/workspaces/"+wsID+"/custom-fields/"+fieldID, nil, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_get_custom_field", out, map[string]any{"workspaceId": wsID}), nil
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

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{
		"name": name,
		"type": fieldType,
	}
	if vals, ok := args["allowed_values"].([]any); ok && len(vals) > 0 {
		body["allowedValues"] = vals
	}
	if req, ok := args["required"].(bool); ok {
		body["required"] = req
	}

	var out map[string]any
	if err := s.Client.Post(ctx, "/workspaces/"+wsID+"/custom-fields", body, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_create_custom_field", out, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) UpdateCustomField(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	fieldID := stringArg(args, "field_id")
	if fieldID == "" {
		return ResultEnvelope{}, fmt.Errorf("field_id is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{}
	if name := stringArg(args, "name"); name != "" {
		body["name"] = name
	}
	if vals, ok := args["allowed_values"].([]any); ok {
		body["allowedValues"] = vals
	}
	if req, ok := args["required"].(bool); ok {
		body["required"] = req
	}

	var out map[string]any
	if err := s.Client.Put(ctx, "/workspaces/"+wsID+"/custom-fields/"+fieldID, body, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_update_custom_field", out, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) DeleteCustomField(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	fieldID := stringArg(args, "field_id")
	if fieldID == "" {
		return ResultEnvelope{}, fmt.Errorf("field_id is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		// Preview: fetch the field then wrap as dry-run result
		var field map[string]any
		if err := s.Client.Get(ctx, "/workspaces/"+wsID+"/custom-fields/"+fieldID, nil, &field); err != nil {
			return ResultEnvelope{}, err
		}
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_custom_field",
			Data:   dryrun.WrapResult(field, "clockify_delete_custom_field"),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	if err := s.Client.Delete(ctx, "/workspaces/"+wsID+"/custom-fields/"+fieldID); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_delete_custom_field", map[string]any{
		"deleted": true,
		"fieldId": fieldID,
	}, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) SetCustomFieldValue(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	fieldID := stringArg(args, "field_id")
	if fieldID == "" {
		return ResultEnvelope{}, fmt.Errorf("field_id is required")
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

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{
		"customFieldId": fieldID,
		"value":         value,
	}

	var path string
	if projectID != "" {
		path = "/workspaces/" + wsID + "/projects/" + projectID + "/custom-fields"
	} else {
		path = "/workspaces/" + wsID + "/time-entries/" + entryID + "/custom-fields"
	}

	var out map[string]any
	if err := s.Client.Put(ctx, path, body, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_set_custom_field_value", out, map[string]any{"workspaceId": wsID}), nil
}
