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

func init() {
	registerTier2Group(Tier2Group{
		Name:        "groups_holidays",
		Description: "User groups (admin view) and workspace holidays",
		Keywords:    []string{"group", "holiday", "public holiday", "recurring", "user group"},
		Builder:     groupsHolidaysHandlers,
	})
}

func groupsHolidaysHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. List user groups (admin)
		{
			Tool: toolRO("clockify_list_user_groups_admin",
				"List user groups in the workspace (admin view)",
				map[string]any{"type": "object"}),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, _ map[string]any) (any, error) {
				return s.ListUserGroupsAdmin(ctx)
			},
		},
		// 2. Get user group by ID
		{
			Tool: toolRO("clockify_get_user_group",
				"Get a user group by ID",
				requiredSchema("group_id")),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.GetUserGroup(ctx, args)
			},
		},
		// 3. Create user group (admin)
		{
			Tool: toolRW("clockify_create_user_group_admin",
				"Create a new user group with optional member user IDs",
				map[string]any{
					"type":     "object",
					"required": []string{"name"},
					"properties": map[string]any{
						"name":     map[string]any{"type": "string", "description": "Group name"},
						"user_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "User IDs to add as members"},
					},
				}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.CreateUserGroupAdmin(ctx, args)
			},
		},
		// 4. Update user group (admin)
		{
			Tool: toolRW("clockify_update_user_group_admin",
				"Update an existing user group by ID",
				map[string]any{
					"type":     "object",
					"required": []string{"group_id"},
					"properties": map[string]any{
						"group_id": map[string]any{"type": "string"},
						"name":     map[string]any{"type": "string"},
						"user_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Replace member list with these user IDs"},
					},
				}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.UpdateUserGroupAdmin(ctx, args)
			},
		},
		// 5. Delete user group (destructive, dry-run minimal)
		{
			Tool: toolDestructive("clockify_delete_user_group_admin",
				"Delete a user group by ID (supports dry_run preview)",
				map[string]any{
					"type":     "object",
					"required": []string{"group_id"},
					"properties": map[string]any{
						"group_id": map[string]any{"type": "string"},
						"dry_run":  map[string]any{"type": "boolean", "description": "Preview without deleting"},
					},
				}),
			ReadOnlyHint:    false,
			DestructiveHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.DeleteUserGroupAdmin(ctx, args)
			},
		},
		// 6. List holidays
		{
			Tool: toolRO("clockify_list_holidays",
				"List holidays configured in the workspace",
				map[string]any{"type": "object"}),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, _ map[string]any) (any, error) {
				return s.ListHolidays(ctx)
			},
		},
		// 7. Create holiday
		{
			Tool: toolRW("clockify_create_holiday",
				"Create a new holiday in the workspace",
				map[string]any{
					"type":     "object",
					"required": []string{"name", "date"},
					"properties": map[string]any{
						"name":      map[string]any{"type": "string", "description": "Holiday name"},
						"date":      map[string]any{"type": "string", "description": "Date in YYYY-MM-DD format"},
						"recurring": map[string]any{"type": "boolean", "description": "Whether the holiday recurs annually"},
					},
				}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.CreateHoliday(ctx, args)
			},
		},
		// 8. Delete holiday (destructive, dry-run minimal)
		{
			Tool: toolDestructive("clockify_delete_holiday",
				"Delete a holiday by ID (supports dry_run preview)",
				map[string]any{
					"type":     "object",
					"required": []string{"holiday_id"},
					"properties": map[string]any{
						"holiday_id": map[string]any{"type": "string"},
						"dry_run":    map[string]any{"type": "boolean", "description": "Preview without deleting"},
					},
				}),
			ReadOnlyHint:    false,
			DestructiveHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.DeleteHoliday(ctx, args)
			},
		},
	}
}

// ---------------------------------------------------------------------------
// User Group Handlers
// ---------------------------------------------------------------------------

func (s *Service) ListUserGroupsAdmin(ctx context.Context) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	path, err := paths.Workspace(wsID, "user-groups")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out []map[string]any
	if err := s.Client.Get(ctx, path, nil, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_list_user_groups_admin", out, map[string]any{
		"workspaceId": wsID,
		"count":       len(out),
	}), nil
}

func (s *Service) GetUserGroup(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	groupID := stringArg(args, "group_id")
	if err := resolve.ValidateID(groupID, "group_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	path, err := paths.Workspace(wsID, "user-groups", groupID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out map[string]any
	if err := s.Client.Get(ctx, path, nil, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_get_user_group", out, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) CreateUserGroupAdmin(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return ResultEnvelope{}, fmt.Errorf("name is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{"name": name}
	if userIDs, ok := args["user_ids"].([]any); ok && len(userIDs) > 0 {
		body["userIds"] = userIDs
	}

	path, err := paths.Workspace(wsID, "user-groups")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out map[string]any
	if err := s.Client.Post(ctx, path, body, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_create_user_group_admin", out, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) UpdateUserGroupAdmin(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	groupID := stringArg(args, "group_id")
	if err := resolve.ValidateID(groupID, "group_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{}
	if name := stringArg(args, "name"); name != "" {
		body["name"] = name
	}
	if userIDs, ok := args["user_ids"].([]any); ok {
		body["userIds"] = userIDs
	}

	path, err := paths.Workspace(wsID, "user-groups", groupID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out map[string]any
	if err := s.Client.Put(ctx, path, body, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_update_user_group_admin", out, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) DeleteUserGroupAdmin(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	groupID := stringArg(args, "group_id")
	if err := resolve.ValidateID(groupID, "group_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_user_group_admin",
			Data:   dryrun.MinimalResult("clockify_delete_user_group_admin", map[string]any{"group_id": groupID}),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	path, err := paths.Workspace(wsID, "user-groups", groupID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.Client.Delete(ctx, path); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_delete_user_group_admin", map[string]any{
		"deleted": true,
		"groupId": groupID,
	}, map[string]any{"workspaceId": wsID}), nil
}

// ---------------------------------------------------------------------------
// Holiday Handlers
// ---------------------------------------------------------------------------

func (s *Service) ListHolidays(ctx context.Context) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	path, err := paths.Workspace(wsID, "holidays")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out []map[string]any
	if err := s.Client.Get(ctx, path, nil, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_list_holidays", out, map[string]any{
		"workspaceId": wsID,
		"count":       len(out),
	}), nil
}

func (s *Service) CreateHoliday(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return ResultEnvelope{}, fmt.Errorf("name is required")
	}
	date := strings.TrimSpace(stringArg(args, "date"))
	if date == "" {
		return ResultEnvelope{}, fmt.Errorf("date is required")
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{
		"name": name,
		"date": date,
	}
	if recurring, ok := args["recurring"].(bool); ok {
		body["recurring"] = recurring
	}

	path, err := paths.Workspace(wsID, "holidays")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out map[string]any
	if err := s.Client.Post(ctx, path, body, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_create_holiday", out, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) DeleteHoliday(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	holidayID := stringArg(args, "holiday_id")
	if err := resolve.ValidateID(holidayID, "holiday_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_holiday",
			Data:   dryrun.MinimalResult("clockify_delete_holiday", map[string]any{"holiday_id": holidayID}),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	path, err := paths.Workspace(wsID, "holidays", holidayID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.Client.Delete(ctx, path); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_delete_holiday", map[string]any{
		"deleted":   true,
		"holidayId": holidayID,
	}, map[string]any{"workspaceId": wsID}), nil
}
