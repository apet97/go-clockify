package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

func userAdminHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. List user groups (RO)
		{
			Tool:         toolRO("clockify_list_user_groups", "List user groups in the workspace", userGroupListSchema()),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.ListUserGroups(ctx, args)
			},
		},
		// 2. Create user group (RW)
		{
			Tool: toolRW("clockify_create_user_group", "Create a new user group. Supports dry_run:true.", map[string]any{
				"type":     "object",
				"required": []string{"name"},
				"properties": map[string]any{
					"name":    map[string]any{"type": "string", "description": "Name for the new user group"},
					"dry_run": map[string]any{"type": "boolean", "description": "Preview the user group payload without creating it"},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.CreateUserGroup(ctx, args)
			},
		},
		// 3. Update user group (RW)
		{
			Tool: toolRW("clockify_update_user_group", "Update a user group name", map[string]any{
				"type":     "object",
				"required": []string{"group_id", "name"},
				"properties": map[string]any{
					"group_id": map[string]any{"type": "string", "description": "User group ID"},
					"name":     map[string]any{"type": "string", "description": "New name for the user group"},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.UpdateUserGroup(ctx, args)
			},
		},
		// 4. Delete user group (destructive)
		{
			Tool: toolDestructive("clockify_delete_user_group", "Permanently delete a user group. Admin scope; destructive; supports dry_run preview.", map[string]any{
				"type":     "object",
				"required": []string{"group_id"},
				"properties": map[string]any{
					"group_id": map[string]any{"type": "string", "description": "User group ID to delete"},
					"dry_run":  map[string]any{"type": "boolean", "description": "Preview without making changes"},
				},
			}),
			ReadOnlyHint:    false,
			DestructiveHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.DeleteUserGroup(ctx, args)
			},
		},
		// 5. Invite user to workspace (RW + external side effect)
		{
			Tool: toolRW("clockify_invite_user", "Invite/add a user to the workspace by email. Supports dry_run:true before sending an invitation email.", map[string]any{
				"type":     "object",
				"required": []string{"email"},
				"properties": map[string]any{
					"email":      map[string]any{"type": "string", "format": "email", "description": "Email address to add to the workspace"},
					"send_email": map[string]any{"type": "boolean", "description": "Whether Clockify should send an invitation email. Defaults to true."},
					"dry_run":    map[string]any{"type": "boolean", "description": "Preview the invitation payload without adding a user or sending email"},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.InviteUser(ctx, args)
			},
		},
		// 6. Add user to group (RW)
		{
			Tool: toolRW("clockify_add_user_to_group", "Add a user to a user group", map[string]any{
				"type":     "object",
				"required": []string{"group_id", "user_id"},
				"properties": map[string]any{
					"group_id": map[string]any{"type": "string", "description": "User group ID"},
					"user_id":  map[string]any{"type": "string", "description": "User ID to add"},
					"dry_run":  map[string]any{"type": "boolean", "description": "Preview adding the user without making changes"},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.AddUserToGroup(ctx, args)
			},
		},
		// 7. Remove user from group (destructive)
		{
			Tool: toolDestructive("clockify_remove_user_from_group", "Remove a user from a user group. Admin and permission-change impact; destructive; supports dry_run preview.", map[string]any{
				"type":     "object",
				"required": []string{"group_id", "user_id"},
				"properties": map[string]any{
					"group_id": map[string]any{"type": "string", "description": "User group ID"},
					"user_id":  map[string]any{"type": "string", "description": "User ID to remove"},
					"dry_run":  map[string]any{"type": "boolean", "description": "Preview without making changes"},
				},
			}),
			ReadOnlyHint:    false,
			DestructiveHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.RemoveUserFromGroup(ctx, args)
			},
		},
		// 7. Update user role (RW)
		{
			Tool: toolRW("clockify_update_user_role", "Update a user's workspace role. Supports dry_run:true.", map[string]any{
				"type":     "object",
				"required": []string{"user_id", "role"},
				"properties": map[string]any{
					"user_id": map[string]any{"type": "string", "description": "User ID"},
					"role": map[string]any{
						"type":        "string",
						"description": "Role to assign: WORKSPACE_ADMIN, PROJECT_MANAGER, TEAM_MANAGER, or REGULAR",
						"enum":        []string{"WORKSPACE_ADMIN", "PROJECT_MANAGER", "TEAM_MANAGER", "REGULAR"},
					},
					"entity_id":   map[string]any{"type": "string", "description": "Role entityId. Defaults to workspace ID for WORKSPACE_ADMIN and user_id for TEAM_MANAGER; required for PROJECT_MANAGER."},
					"project_id":  map[string]any{"type": "string", "description": "Project ID used as entityId when role is PROJECT_MANAGER."},
					"source_type": map[string]any{"type": "string", "enum": []string{"USER_GROUP"}, "description": "Optional role sourceType for user-group backed manager roles."},
					"remove_role": map[string]any{"type": "string", "enum": []string{"WORKSPACE_ADMIN", "PROJECT_MANAGER", "TEAM_MANAGER"}, "description": "For role=REGULAR, remove this specific elevated role instead of discovering current grants."},
					"role_grants": map[string]any{
						"type":        "array",
						"description": "For role=REGULAR, explicit elevated grants to remove. Each item needs role and entity_id/entityId.",
						"items": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"required":             []string{"role"},
							"properties": map[string]any{
								"role":        map[string]any{"type": "string", "enum": []string{"WORKSPACE_ADMIN", "PROJECT_MANAGER", "TEAM_MANAGER"}},
								"entity_id":   map[string]any{"type": "string"},
								"entityId":    map[string]any{"type": "string"},
								"project_id":  map[string]any{"type": "string"},
								"projectId":   map[string]any{"type": "string"},
								"source_type": map[string]any{"type": "string", "enum": []string{"USER_GROUP"}},
								"sourceType":  map[string]any{"type": "string", "enum": []string{"USER_GROUP"}},
								"source":      map[string]any{"type": "object", "additionalProperties": true},
							},
						},
					},
					"dry_run": map[string]any{"type": "boolean", "description": "Preview the role change without applying it"},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.UpdateUserRole(ctx, args)
			},
		},
		// 8. Deactivate user (RW, confirm pattern dry-run)
		{
			Tool: toolRW("clockify_deactivate_user", "Deactivate a workspace user and remove access. Supports dry_run preview.", map[string]any{
				"type":     "object",
				"required": []string{"user_id"},
				"properties": map[string]any{
					"user_id": map[string]any{"type": "string", "description": "User ID to deactivate"},
					"dry_run": map[string]any{"type": "boolean", "description": "Preview without making changes"},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.DeactivateUser(ctx, args)
			},
		},
		// 9. List user managers (RO)
		{
			Tool: toolRO("clockify_list_user_managers", "List managers assigned to a user", map[string]any{
				"type":     "object",
				"required": []string{"user_id"},
				"properties": map[string]any{
					"user_id":     map[string]any{"type": "string", "description": "User ID"},
					"page":        map[string]any{"type": "integer", "description": "Page number (default 1)"},
					"page_size":   map[string]any{"type": "integer", "description": "Items per page (default 50)"},
					"sort_column": map[string]any{"type": "string", "enum": []string{"ID", "EMAIL", "NAME", "NAME_LOWERCASE", "ACCESS", "HOURLYRATE", "COSTRATE"}},
					"sort_order":  map[string]any{"type": "string", "enum": []string{"ASCENDING", "DESCENDING"}},
				},
			}),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.ListUserManagers(ctx, args)
			},
		},
		// 10. Get member profile (RO)
		{
			Tool: toolRO("clockify_get_member_profile", "Get a user's workspace-scoped member profile", map[string]any{
				"type":     "object",
				"required": []string{"user_id"},
				"properties": map[string]any{
					"user_id": map[string]any{"type": "string", "description": "User ID"},
				},
			}),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.GetMemberProfile(ctx, args)
			},
		},
		// 11. Update member profile (RW)
		{
			Tool: toolRW("clockify_update_member_profile", "Update a user's workspace-scoped member profile. Supports dry_run:true.", map[string]any{
				"type":     "object",
				"required": []string{"user_id"},
				"properties": map[string]any{
					"user_id":              map[string]any{"type": "string", "description": "User ID"},
					"name":                 map[string]any{"type": "string", "description": "Profile display name"},
					"image_url":            map[string]any{"type": "string", "description": "Profile image URL"},
					"remove_profile_image": map[string]any{"type": "boolean", "description": "Remove the profile image"},
					"week_start":           map[string]any{"type": "string", "enum": memberProfileDayEnums(), "description": "First day of the week"},
					"work_capacity":        map[string]any{"type": "string", "description": "Daily work capacity, for example PT7H"},
					"working_days": map[string]any{
						"type":        "array",
						"description": "Array of working day enum strings; live Clockify rejects a JSON-encoded string here.",
						"items":       map[string]any{"type": "string", "enum": memberProfileDayEnums()},
					},
					"user_custom_fields": map[string]any{
						"type":        "array",
						"description": "Member profile custom field values",
						"items":       map[string]any{"type": "object", "additionalProperties": true},
					},
					"dry_run": map[string]any{"type": "boolean", "description": "Preview the profile payload without applying it"},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.UpdateMemberProfile(ctx, args)
			},
		},
	}
}

var validMemberProfileDays = map[string]bool{
	"MONDAY":    true,
	"TUESDAY":   true,
	"WEDNESDAY": true,
	"THURSDAY":  true,
	"FRIDAY":    true,
	"SATURDAY":  true,
	"SUNDAY":    true,
}

func memberProfileDayEnums() []string {
	return []string{"MONDAY", "TUESDAY", "WEDNESDAY", "THURSDAY", "FRIDAY", "SATURDAY", "SUNDAY"}
}

func normalizeMemberProfileDay(raw, field string) (string, error) {
	day := strings.ToUpper(strings.TrimSpace(raw))
	if day == "" {
		return "", nil
	}
	if !validMemberProfileDays[day] {
		return "", fmt.Errorf("%s must be one of MONDAY, TUESDAY, WEDNESDAY, THURSDAY, FRIDAY, SATURDAY, SUNDAY; got %q", field, raw)
	}
	return day, nil
}

func normalizeMemberProfileDays(raw []string, field string) ([]string, error) {
	days := make([]string, 0, len(raw))
	for _, item := range raw {
		day, err := normalizeMemberProfileDay(item, field)
		if err != nil {
			return nil, err
		}
		if day != "" {
			days = append(days, day)
		}
	}
	return days, nil
}

// ListUserGroups returns user groups for the workspace.
func (s *Service) ListUserGroups(ctx context.Context, args map[string]any) (ToolResult, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	query, page, pageSize := userGroupListQuery(args)

	path, err := paths.Workspace(wsID, "user-groups")
	if err != nil {
		return ToolResult{}, err
	}
	var groups []map[string]any
	if err := s.Client.Get(ctx, path, query, &groups); err != nil {
		return ToolResult{}, err
	}

	return ok("clockify_list_user_groups", groups, emptyListMeta(map[string]any{
		"workspaceId": wsID,
		"count":       len(groups),
		"page":        page,
		"pageSize":    pageSize,
	}, "clockify_groups_create")), nil
}

// CreateUserGroup creates a new user group.
func (s *Service) CreateUserGroup(ctx context.Context, args map[string]any) (ToolResult, error) {
	name := stringArg(args, "name")
	if name == "" {
		return ToolResult{}, fmt.Errorf("name is required")
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	payload := map[string]any{"name": name}
	if dryrun.Enabled(args) {
		return ok("clockify_create_user_group", dryrunPreviewPayload("clockify_create_user_group", payload), map[string]any{"workspaceId": wsID}), nil
	}
	path, err := paths.Workspace(wsID, "user-groups")
	if err != nil {
		return ToolResult{}, err
	}
	var result map[string]any
	if err := s.Client.Post(ctx, path, payload, &result); err != nil {
		return ToolResult{}, err
	}

	if gid, _ := result["id"].(string); gid != "" {
		s.emitResourceUpdateWithState(groupResourceURI(wsID, gid), result)
	}
	return ok("clockify_create_user_group", result, map[string]any{"workspaceId": wsID}), nil
}

// UpdateUserGroup updates a user group's name.
func (s *Service) UpdateUserGroup(ctx context.Context, args map[string]any) (ToolResult, error) {
	groupID := stringArg(args, "group_id")
	if err := resolve.ValidateID(groupID, "group_id"); err != nil {
		return ToolResult{}, err
	}
	name := stringArg(args, "name")
	if name == "" {
		return ToolResult{}, fmt.Errorf("name is required")
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	payload := map[string]any{"name": name}
	path, err := paths.Workspace(wsID, "user-groups", groupID)
	if err != nil {
		return ToolResult{}, err
	}
	var result any
	if err := s.Client.Put(ctx, path, payload, &result); err != nil {
		return ToolResult{}, err
	}

	s.emitResourceUpdateWithState(groupResourceURI(wsID, groupID), result)
	return ok("clockify_update_user_group", result, map[string]any{"workspaceId": wsID}), nil
}

// DeleteUserGroup deletes a user group. Supports dry-run (minimal fallback).
func (s *Service) DeleteUserGroup(ctx context.Context, args map[string]any) (ToolResult, error) {
	groupID := stringArg(args, "group_id")
	if err := resolve.ValidateID(groupID, "group_id"); err != nil {
		return ToolResult{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	if dryrun.Enabled(args) {
		return ToolResult{
			OK:     true,
			Action: "clockify_delete_user_group",
			Data:   dryrun.MinimalResult("clockify_delete_user_group", map[string]any{"group_id": groupID}),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	path, err := paths.Workspace(wsID, "user-groups", groupID)
	if err != nil {
		return ToolResult{}, err
	}
	if err := s.Client.Delete(ctx, path); err != nil {
		return ToolResult{}, err
	}

	s.emitResourceDeleted(groupResourceURI(wsID, groupID))
	return ok("clockify_delete_user_group", map[string]any{"deleted": true, "groupId": groupID}, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) InviteUser(ctx context.Context, args map[string]any) (ToolResult, error) {
	email := strings.TrimSpace(stringArg(args, "email"))
	if email == "" {
		return ToolResult{}, fmt.Errorf("email is required")
	}
	sendEmail := true
	if v, ok := args["send_email"].(bool); ok {
		sendEmail = v
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	payload := map[string]any{"email": email}
	if dryrun.Enabled(args) {
		preview := dryrunPreviewPayload("clockify_invite_user", payload)
		preview["send_email"] = sendEmail
		return ok("clockify_invite_user", preview, map[string]any{"workspaceId": wsID, "sendEmail": sendEmail}), nil
	}
	path, err := paths.Workspace(wsID, "users")
	if err != nil {
		return ToolResult{}, err
	}
	var result any
	if err := s.Client.PostWithQuery(ctx, path, map[string]string{"send-email": strconv.FormatBool(sendEmail)}, payload, &result); err != nil {
		return ToolResult{}, err
	}
	return ok("clockify_invite_user", result, map[string]any{"workspaceId": wsID, "sendEmail": sendEmail}), nil
}

// AddUserToGroup adds a user to a user group.
func (s *Service) AddUserToGroup(ctx context.Context, args map[string]any) (ToolResult, error) {
	groupID := stringArg(args, "group_id")
	if err := resolve.ValidateID(groupID, "group_id"); err != nil {
		return ToolResult{}, err
	}
	userID := stringArg(args, "user_id")
	if err := resolve.ValidateID(userID, "user_id"); err != nil {
		return ToolResult{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	payload := map[string]any{"userId": userID}
	path, err := paths.Workspace(wsID, "user-groups", groupID, "users")
	if err != nil {
		return ToolResult{}, err
	}
	if dryrun.Enabled(args) {
		return ok("clockify_add_user_to_group", dryrunPreviewPayload("clockify_add_user_to_group", payload), map[string]any{
			"workspaceId": wsID,
			"groupId":     groupID,
			"userId":      userID,
		}), nil
	}
	var result map[string]any
	if err := s.Client.Post(ctx, path, payload, &result); err != nil {
		return ToolResult{}, err
	}

	s.emitResourceUpdate(ctx, groupResourceURI(wsID, groupID))
	return ok("clockify_add_user_to_group", result, map[string]any{
		"workspaceId": wsID,
		"groupId":     groupID,
		"userId":      userID,
	}), nil
}

// RemoveUserFromGroup removes a user from a user group. Supports dry-run (minimal fallback).
func (s *Service) RemoveUserFromGroup(ctx context.Context, args map[string]any) (ToolResult, error) {
	groupID := stringArg(args, "group_id")
	if err := resolve.ValidateID(groupID, "group_id"); err != nil {
		return ToolResult{}, err
	}
	userID := stringArg(args, "user_id")
	if err := resolve.ValidateID(userID, "user_id"); err != nil {
		return ToolResult{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	if dryrun.Enabled(args) {
		return ToolResult{
			OK:     true,
			Action: "clockify_remove_user_from_group",
			Data:   dryrun.MinimalResult("clockify_remove_user_from_group", map[string]any{"group_id": groupID, "user_id": userID}),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	path, err := paths.Workspace(wsID, "user-groups", groupID, "users", userID)
	if err != nil {
		return ToolResult{}, err
	}
	if err := s.Client.Delete(ctx, path); err != nil {
		return ToolResult{}, err
	}

	s.emitResourceUpdate(ctx, groupResourceURI(wsID, groupID))
	return ok("clockify_remove_user_from_group", map[string]any{"removed": true, "groupId": groupID, "userId": userID}, map[string]any{"workspaceId": wsID}), nil
}

// UpdateUserRole updates a user's workspace role.
func (s *Service) UpdateUserRole(ctx context.Context, args map[string]any) (ToolResult, error) {
	userID := stringArg(args, "user_id")
	if err := resolve.ValidateID(userID, "user_id"); err != nil {
		return ToolResult{}, err
	}
	role := strings.ToUpper(strings.TrimSpace(stringArg(args, "role")))
	validRoles := map[string]bool{
		"WORKSPACE_ADMIN": true,
		"PROJECT_MANAGER": true,
		"TEAM_MANAGER":    true,
		"REGULAR":         true,
	}
	if !validRoles[role] {
		return ToolResult{}, fmt.Errorf("role must be one of WORKSPACE_ADMIN, PROJECT_MANAGER, TEAM_MANAGER, REGULAR; got %q", role)
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	if role == "REGULAR" {
		return s.stripUserRoles(ctx, args, wsID, userID)
	}

	entityID, err := roleEntityID(role, args, wsID, userID)
	if err != nil {
		return ToolResult{}, err
	}
	payload := map[string]any{"entityId": entityID, "role": role}
	if sourceType := strings.TrimSpace(stringArg(args, "source_type")); sourceType != "" {
		payload["sourceType"] = sourceType
	}
	if dryrun.Enabled(args) {
		return ok("clockify_update_user_role", dryrunPreviewPayload("clockify_update_user_role", payload), map[string]any{
			"workspaceId": wsID,
			"userId":      userID,
			"role":        role,
		}), nil
	}

	// Self-modification guard: defense in depth on top of the upstream
	// 403. The Clockify web UI is the right surface for an operator to
	// change their own role; doing it via an API key risks the operator
	// stranding themselves (e.g. demoting from WORKSPACE_ADMIN to
	// TEAM_MANAGER and losing the surface to undo). Run only on the
	// real-action path so dry-run previews stay HTTP-free; the audit
	// trail records the reject before any role POST issues.
	if self, lookupErr := s.getCurrentUser(ctx); lookupErr == nil && self.ID != "" && self.ID == userID {
		return ToolResult{}, fmt.Errorf(
			"refusing to change the API key owner's own workspace role at " +
				"the MCP layer; use the Clockify web UI if this is intentional " +
				"(self-modification can strip the operator's ability to undo)",
		)
	}
	path, err := paths.Workspace(wsID, "users", userID, "roles")
	if err != nil {
		return ToolResult{}, err
	}
	var result any
	if err := s.Client.Post(ctx, path, payload, &result); err != nil {
		return ToolResult{}, err
	}

	s.emitResourceUpdateWithState(userResourceURI(wsID, userID), result)
	return ok("clockify_update_user_role", result, map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
		"role":        role,
	}), nil
}

type userRoleGrant struct {
	Role       string `json:"role"`
	EntityID   string `json:"entityId"`
	SourceType string `json:"sourceType,omitempty"`
}

func roleEntityID(role string, args map[string]any, wsID, userID string) (string, error) {
	if entityID := firstNonEmptyString(strings.TrimSpace(stringArg(args, "entity_id")), strings.TrimSpace(stringArg(args, "entityId"))); entityID != "" {
		return entityID, nil
	}
	switch role {
	case "WORKSPACE_ADMIN":
		return wsID, nil
	case "TEAM_MANAGER":
		return userID, nil
	case "PROJECT_MANAGER":
		if projectID := strings.TrimSpace(stringArg(args, "project_id")); projectID != "" {
			return projectID, nil
		}
		return "", fmt.Errorf("entity_id or project_id is required when role is PROJECT_MANAGER")
	default:
		return "", fmt.Errorf("unsupported manager role %q", role)
	}
}

func (s *Service) stripUserRoles(ctx context.Context, args map[string]any, wsID, userID string) (ToolResult, error) {
	if dryrun.Enabled(args) {
		grants, _, err := roleStripGrantsFromArgs(args, wsID, userID)
		if err != nil {
			return ToolResult{}, err
		}
		preview := map[string]any{
			"user_id": userID,
			"role":    "REGULAR",
		}
		if len(grants) > 0 {
			preview["role_grants"] = grants
		} else {
			preview["note"] = "The real run will fetch current elevated role grants, then issue one DELETE-with-body request per grant."
		}
		return ok("clockify_update_user_role", dryrunPreviewPayload("clockify_update_user_role", preview), map[string]any{
			"workspaceId": wsID,
			"userId":      userID,
			"role":        "REGULAR",
		}), nil
	}
	if self, lookupErr := s.getCurrentUser(ctx); lookupErr == nil && self.ID != "" && self.ID == userID {
		return ToolResult{}, fmt.Errorf(
			"refusing to change the API key owner's own workspace role at " +
				"the MCP layer; use the Clockify web UI if this is intentional " +
				"(self-modification can strip the operator's ability to undo)",
		)
	}

	grants, explicit, err := roleStripGrantsFromArgs(args, wsID, userID)
	if err != nil {
		return ToolResult{}, err
	}
	if !explicit {
		grants, err = s.currentUserRoleGrants(ctx, wsID, userID)
		if err != nil {
			return ToolResult{}, err
		}
	}
	if len(grants) == 0 {
		return ok("clockify_update_user_role", map[string]any{
			"id":             userID,
			"userId":         userID,
			"role":           "REGULAR",
			"alreadyRegular": true,
			"removedRoles":   []userRoleGrant{},
		}, map[string]any{"workspaceId": wsID, "userId": userID, "role": "REGULAR"}), nil
	}

	path, err := paths.Workspace(wsID, "users", userID, "roles")
	if err != nil {
		return ToolResult{}, err
	}
	for _, grant := range grants {
		body := map[string]any{"entityId": grant.EntityID, "role": grant.Role}
		if grant.SourceType != "" {
			body["sourceType"] = grant.SourceType
		}
		if err := s.Client.DeleteWithBody(ctx, path, body, nil); err != nil {
			return ToolResult{}, err
		}
	}
	result := map[string]any{
		"id":           userID,
		"userId":       userID,
		"role":         "REGULAR",
		"removedRoles": grants,
	}
	s.emitResourceUpdateWithState(userResourceURI(wsID, userID), result)
	return ok("clockify_update_user_role", result, map[string]any{"workspaceId": wsID, "userId": userID, "role": "REGULAR"}), nil
}

func roleStripGrantsFromArgs(args map[string]any, wsID, userID string) ([]userRoleGrant, bool, error) {
	rows := mapSlice(args["role_grants"])
	if len(rows) > 0 {
		grants := make([]userRoleGrant, 0, len(rows))
		for _, row := range rows {
			grant, err := roleGrantFromMap(row, wsID, userID)
			if err != nil {
				return nil, true, err
			}
			grants = append(grants, grant)
		}
		return grants, true, nil
	}
	removeRole := strings.ToUpper(strings.TrimSpace(stringArg(args, "remove_role")))
	if removeRole == "" {
		return nil, false, nil
	}
	row := map[string]any{
		"role":       removeRole,
		"entity_id":  firstNonEmptyString(stringArg(args, "entity_id"), stringArg(args, "project_id")),
		"sourceType": stringArg(args, "source_type"),
	}
	grant, err := roleGrantFromMap(row, wsID, userID)
	return []userRoleGrant{grant}, true, err
}

func roleGrantFromMap(row map[string]any, wsID, userID string) (userRoleGrant, error) {
	role := normalizeManagerRoleName(firstReportString(row, "role", "name"))
	if role == "" {
		if nested, ok := row["role"].(map[string]any); ok {
			role = normalizeManagerRoleName(firstReportString(nested, "role", "name"))
		}
	}
	if role == "" {
		return userRoleGrant{}, fmt.Errorf("role_grants item is missing a supported role")
	}
	args := map[string]any{
		"entity_id":   firstReportString(row, "entityId", "entity_id", "projectId", "project_id"),
		"source_type": firstReportString(row, "sourceType", "source_type"),
	}
	if source, ok := row["source"].(map[string]any); ok {
		if args["entity_id"] == "" {
			args["entity_id"] = firstReportString(source, "id")
		}
		if args["source_type"] == "" {
			args["source_type"] = firstReportString(source, "type")
		}
	}
	entityID, err := roleEntityID(role, args, wsID, userID)
	if err != nil {
		return userRoleGrant{}, err
	}
	return userRoleGrant{Role: role, EntityID: entityID, SourceType: strings.TrimSpace(fmt.Sprint(args["source_type"]))}, nil
}

func (s *Service) currentUserRoleGrants(ctx context.Context, wsID, userID string) ([]userRoleGrant, error) {
	path, err := paths.Workspace(wsID, "users", "info")
	if err != nil {
		return nil, err
	}
	for page := 1; page <= 20; page++ {
		body := map[string]any{
			"page":         page,
			"pageSize":     200,
			"includeRoles": true,
			"memberships":  "NONE",
			"status":       "ALL",
		}
		var users []map[string]any
		if err := s.Client.Post(ctx, path, body, &users); err != nil {
			return nil, err
		}
		for _, user := range users {
			if firstReportString(user, "id", "userId", "user_id") != userID {
				continue
			}
			return roleGrantsFromRaw(user, wsID, userID), nil
		}
		if len(users) < 200 {
			break
		}
	}
	return nil, fmt.Errorf("user %s not found while fetching current roles; pass role_grants with explicit role/entity_id values or retry with a live user ID", userID)
}

func roleGrantsFromRaw(user map[string]any, wsID, userID string) []userRoleGrant {
	rows := mapSlice(user["roles"])
	out := make([]userRoleGrant, 0, len(rows))
	for _, row := range rows {
		grant, err := roleGrantFromMap(row, wsID, userID)
		if err == nil {
			out = append(out, grant)
		}
	}
	return out
}

func normalizeManagerRoleName(raw string) string {
	role := strings.ToUpper(strings.TrimSpace(raw))
	switch {
	case role == "WORKSPACE_ADMIN" || role == "PROJECT_MANAGER" || role == "TEAM_MANAGER":
		return role
	case strings.Contains(role, "ADMIN"):
		return "WORKSPACE_ADMIN"
	case strings.Contains(role, "PROJECT"):
		return "PROJECT_MANAGER"
	case strings.Contains(role, "TEAM"):
		return "TEAM_MANAGER"
	default:
		return ""
	}
}

// DeactivateUser deactivates a user. Supports dry-run (confirm pattern).
func (s *Service) DeactivateUser(ctx context.Context, args map[string]any) (ToolResult, error) {
	userID := stringArg(args, "user_id")
	if err := resolve.ValidateID(userID, "user_id"); err != nil {
		return ToolResult{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	if dryrun.Enabled(args) {
		return ToolResult{
			OK:     true,
			Action: "clockify_deactivate_user",
			Data: map[string]any{
				"dry_run": true,
				"tool":    "clockify_deactivate_user",
				"args":    map[string]any{"user_id": userID},
				"note":    "This is a dry-run preview. The user would be deactivated. No changes were made.",
			},
			Meta: map[string]any{"workspaceId": wsID, "userId": userID},
		}, nil
	}

	// Self-deactivation guard: defense in depth on top of the upstream
	// 403. Deactivating the API key owner is a fast path to a workspace
	// lockout — the web UI undo requires a still-active admin login.
	// Skip the lookup on the dry-run preview path so previews remain
	// HTTP-free; the real action path always checks before issuing
	// the PUT.
	if self, lookupErr := s.getCurrentUser(ctx); lookupErr == nil && self.ID != "" && self.ID == userID {
		return ToolResult{}, fmt.Errorf(
			"refusing to deactivate the API key owner at the MCP layer; " +
				"use the Clockify web UI if this is intentional " +
				"(self-deactivation can lock the operator out of the workspace)",
		)
	}

	payload := map[string]any{"status": "INACTIVE"}
	path, err := paths.Workspace(wsID, "users", userID)
	if err != nil {
		return ToolResult{}, err
	}
	var result map[string]any
	if err := s.Client.Put(ctx, path, payload, &result); err != nil {
		return ToolResult{}, err
	}

	s.emitResourceUpdateWithState(userResourceURI(wsID, userID), result)
	return ok("clockify_deactivate_user", result, map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
	}), nil
}

// ListUserManagers returns managers assigned to a workspace user.
func (s *Service) ListUserManagers(ctx context.Context, args map[string]any) (ToolResult, error) {
	userID := stringArg(args, "user_id")
	if err := resolve.ValidateID(userID, "user_id"); err != nil {
		return ToolResult{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	page, pageSize := paginationFromArgs(args)
	query := map[string]string{
		"page":      strconv.Itoa(page),
		"page-size": strconv.Itoa(pageSize),
	}
	if v := strings.TrimSpace(stringArg(args, "sort_column")); v != "" {
		query["sort-column"] = v
	}
	if v := strings.TrimSpace(stringArg(args, "sort_order")); v != "" {
		query["sort-order"] = v
	}
	path, err := paths.Workspace(wsID, "users", userID, "managers")
	if err != nil {
		return ToolResult{}, err
	}
	var result []map[string]any
	if err := s.Client.Get(ctx, path, query, &result); err != nil {
		return ToolResult{}, err
	}
	meta := addPaginationMeta(map[string]any{"workspaceId": wsID, "userId": userID, "count": len(result)}, args, page, pageSize)
	return ok("clockify_list_user_managers", userManagerViewsFromRaw(result), meta), nil
}

// GetMemberProfile returns a workspace-scoped member profile for a user.
func (s *Service) GetMemberProfile(ctx context.Context, args map[string]any) (ToolResult, error) {
	userID := stringArg(args, "user_id")
	if err := resolve.ValidateID(userID, "user_id"); err != nil {
		return ToolResult{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	path, err := paths.Workspace(wsID, "member-profile", userID)
	if err != nil {
		return ToolResult{}, err
	}
	var result map[string]any
	if err := s.Client.Get(ctx, path, nil, &result); err != nil {
		return ToolResult{}, err
	}
	return ok("clockify_get_member_profile", memberProfileViewFromRaw(result), map[string]any{"workspaceId": wsID, "userId": userID}), nil
}

// UpdateMemberProfile updates live-supported workspace member profile fields.
func (s *Service) UpdateMemberProfile(ctx context.Context, args map[string]any) (ToolResult, error) {
	userID := stringArg(args, "user_id")
	if err := resolve.ValidateID(userID, "user_id"); err != nil {
		return ToolResult{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	// image_url and remove_profile_image are mutually exclusive — sending
	// both leaves the upstream's resulting image undefined.
	if strings.TrimSpace(stringArg(args, "image_url")) != "" && boolArg(args, "remove_profile_image") {
		return ToolResult{}, fmt.Errorf("image_url and remove_profile_image are mutually exclusive: set one or the other, not both")
	}

	payload := map[string]any{}
	setIfString(payload, args, "name", "name")
	setIfString(payload, args, "image_url", "imageUrl")
	setIfBool(payload, args, "remove_profile_image", "removeProfileImage")
	setIfString(payload, args, "work_capacity", "workCapacity")
	if weekStart := stringArg(args, "week_start"); strings.TrimSpace(weekStart) != "" {
		day, err := normalizeMemberProfileDay(weekStart, "week_start")
		if err != nil {
			return ToolResult{}, err
		}
		payload["weekStart"] = day
	}
	if workingDays, ok, err := strictStringSliceArg(args, "working_days"); err != nil {
		return ToolResult{}, err
	} else if ok {
		days, err := normalizeMemberProfileDays(workingDays, "working_days")
		if err != nil {
			return ToolResult{}, err
		}
		payload["workingDays"] = days
	}
	if customFields, ok, err := mapSliceArg(args, "user_custom_fields"); err != nil {
		return ToolResult{}, err
	} else if ok {
		payload["userCustomFields"] = customFields
	}
	if len(payload) == 0 {
		return ToolResult{}, fmt.Errorf("at least one member profile field must be provided")
	}

	if dryrun.Enabled(args) {
		return ok("clockify_update_member_profile", dryrunPreviewPayload("clockify_update_member_profile", payload), map[string]any{
			"workspaceId": wsID,
			"userId":      userID,
		}), nil
	}

	path, err := paths.Workspace(wsID, "member-profile", userID)
	if err != nil {
		return ToolResult{}, err
	}
	var result map[string]any
	if err := s.Client.Patch(ctx, path, payload, &result); err != nil {
		return ToolResult{}, err
	}
	s.emitResourceUpdateWithState(userResourceURI(wsID, userID), result)
	return ok("clockify_update_member_profile", memberProfileViewFromRaw(result), map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
	}), nil
}
