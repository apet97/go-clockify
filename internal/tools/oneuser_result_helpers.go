package tools

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

func aliasArg(args map[string]any, from, to string) map[string]any {
	return aliasArgs(args, map[string]string{from: to})
}

func aliasArgs(args map[string]any, aliases map[string]string) map[string]any {
	out := make(map[string]any, len(args)+len(aliases))
	for key, value := range args {
		out[key] = value
	}
	for from, to := range aliases {
		if _, ok := out[to]; ok {
			continue
		}
		if value, ok := out[from]; ok {
			out[to] = value
		}
	}
	return out
}

func aliasHandler(action, entity, change string, handler func(context.Context, map[string]any) (ResultEnvelope, error)) mcp.ToolHandler {
	return func(ctx context.Context, args map[string]any) (any, error) {
		out, err := handler(ctx, args)
		if err != nil {
			return nil, err
		}
		return standardizeDomainResult(action, entity, change, out, args), nil
	}
}

func (s *Service) EntriesMarkInvoiced(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	entryIDs, found, err := strictStringSliceArg(args, "time_entry_ids")
	if err != nil {
		return ResultEnvelope{}, err
	}
	if !found || len(entryIDs) == 0 {
		return ResultEnvelope{}, fmt.Errorf("time_entry_ids is required")
	}
	invoiced, found := optionalBoolArg(args, "invoiced")
	if !found {
		return ResultEnvelope{}, fmt.Errorf("invoiced is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "time-entries", "invoiced")
	if err != nil {
		return ResultEnvelope{}, err
	}
	body := map[string]any{"timeEntryIds": entryIDs, "invoiced": invoiced}
	var upstream map[string]any
	if err := s.Client.Patch(ctx, path, body, &upstream); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_entries_mark_invoiced", map[string]any{
		"updated":       true,
		"timeEntryIds":  entryIDs,
		"invoiced":      invoiced,
		"upstreamReply": upstream,
	}, map[string]any{"workspaceId": wsID, "entryId": entryIDs[0]}), nil
}

func (s *Service) UsersInvite(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	emails, found, err := strictStringSliceArg(args, "emails")
	if err != nil {
		return ResultEnvelope{}, err
	}
	if !found {
		if email := strings.TrimSpace(stringArg(args, "email")); email != "" {
			emails = []string{email}
		}
	}
	if len(emails) == 0 {
		return ResultEnvelope{}, fmt.Errorf("email or emails is required")
	}
	baseArgs := map[string]any{}
	if sendEmail, ok := optionalBoolArg(args, "send_email"); ok {
		baseArgs["send_email"] = sendEmail
	}
	if len(emails) == 1 {
		baseArgs["email"] = emails[0]
		out, err := s.InviteUser(ctx, baseArgs)
		if err != nil {
			return ResultEnvelope{}, err
		}
		out.Action = "clockify_users_invite"
		return out, nil
	}
	var wsID string
	invitations := make([]any, 0, len(emails))
	for _, email := range emails {
		nextArgs := make(map[string]any, len(baseArgs)+1)
		for key, value := range baseArgs {
			nextArgs[key] = value
		}
		nextArgs["email"] = email
		out, err := s.InviteUser(ctx, nextArgs)
		if err != nil {
			return ResultEnvelope{}, err
		}
		if id, _ := out.Meta["workspaceId"].(string); id != "" {
			wsID = id
		}
		invitations = append(invitations, out.Data)
	}
	return ok("clockify_users_invite", map[string]any{
		"invitations": invitations,
		"count":       len(invitations),
	}, map[string]any{"workspaceId": wsID}), nil
}

func requirePresentArgs(args map[string]any, keys ...string) error {
	for _, key := range keys {
		value, ok := args[key]
		if !ok {
			return fmt.Errorf("%s is required", key)
		}
		if str, ok := value.(string); ok && strings.TrimSpace(str) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	return nil
}

func requiredIDArg(args map[string]any, key string) (string, error) {
	value := strings.TrimSpace(stringArg(args, key))
	if value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	if err := resolve.ValidateID(value, key); err != nil {
		return "", err
	}
	return value, nil
}

func nativeBodyFromArgs(args map[string]any, keys ...string) map[string]any {
	body := map[string]any{}
	if raw, ok := args["body"].(map[string]any); ok {
		for key, value := range raw {
			body[key] = value
		}
	}
	for _, key := range keys {
		if key == "body" {
			continue
		}
		if value, ok := args[key]; ok {
			body[bodyName(key)] = value
		}
	}
	if len(body) == 0 {
		return nil
	}
	return body
}

func standardizeDomainResult(action, entity, change string, out any, args map[string]any) ToolResult {
	if current, ok := out.(ToolResult); ok {
		current.Action = action
		current.Data = sanitizeResultValue(current.Data)
		current.Meta = sanitizeResultMeta(current.Meta)
		if dryRunResult(args, current.Data) {
			current.Changed = ChangeSet{}
		}
		return current
	}
	ids := map[string]string{}
	data := out
	var meta map[string]any
	var warnings []Warning
	if env, ok := out.(ResultEnvelope); ok {
		data = env.Data
		if len(env.Meta) > 0 {
			meta = env.Meta
		}
		for key, value := range env.Meta {
			if str, ok := value.(string); ok && str != "" {
				ids[key] = str
			}
		}
	}
	for key, value := range args {
		if strings.HasSuffix(key, "_id") {
			if str, ok := value.(string); ok && str != "" {
				ids[idKey(key)] = str
			}
		}
	}
	for key, value := range idsFromData(data, entity) {
		ids[key] = value
	}
	changed := ChangeSet{}
	if !dryRunResult(args, data) {
		changed = changedFor(change, entity, data, ids)
	}
	return result(action, entity, ids, data, changed, warnings, nil, meta)
}

func dryRunResult(args map[string]any, data any) bool {
	if boolArg(args, "dry_run") {
		return true
	}
	if m, ok := data.(map[string]any); ok {
		if dry, _ := m["dry_run"].(bool); dry {
			return true
		}
	}
	return false
}

func changedFor(change, entity string, data any, ids map[string]string) ChangeSet {
	if change == "" {
		return ChangeSet{}
	}
	ref := EntityRef{Type: entity, ID: firstEntityID(entity, ids), Name: entityName(data)}
	switch change {
	case "created":
		return ChangeSet{Created: []EntityRef{ref}}
	case "updated":
		return ChangeSet{Updated: []EntityRef{ref}}
	case "deleted":
		return ChangeSet{Deleted: []EntityRef{ref}}
	case "reused":
		return ChangeSet{Reused: []EntityRef{ref}}
	default:
		return ChangeSet{}
	}
}

func idsFromData(data any, entity string) map[string]string {
	out := map[string]string{}
	switch m := data.(type) {
	case clockify.TimeEntry:
		addEntryIDs(out, m.ID, m.WorkspaceID, m.UserID, m.ProjectID, m.TaskID)
	case clockify.User:
		if m.ID != "" {
			out["userId"] = m.ID
		}
	case UserView:
		if m.ID != "" {
			out["userId"] = m.ID
		}
		if m.ActiveWorkspace != "" {
			out["workspaceId"] = m.ActiveWorkspace
		}
	case WorkspaceView:
		if m.ID != "" {
			out["workspaceId"] = m.ID
		}
	case EntryView:
		addEntryIDs(out, m.ID, m.WorkspaceID, m.UserID, m.ProjectID, m.TaskID)
	case map[string]any:
		addEntityIDFromMap(out, entity, m)
	case InvoiceView:
		addEntityIDFromMap(out, entity, map[string]any(m))
	case InvoiceItemView:
		addEntityIDFromMap(out, entity, map[string]any(m))
	case InvoicePaymentView:
		addEntityIDFromMap(out, entity, map[string]any(m))
	case InvoiceSettingsView:
		addEntityIDFromMap(out, entity, map[string]any(m))
	case ExpenseView:
		addEntityIDFromMap(out, entity, map[string]any(m))
	case TimeOffPolicyView:
		addEntityIDFromMap(out, entity, map[string]any(m))
	case TimeOffRequestView:
		addEntityIDFromMap(out, entity, map[string]any(m))
	case TimeOffBalanceView:
		addEntityIDFromMap(out, entity, map[string]any(m))
	case AssignmentView:
		addEntityIDFromMap(out, entity, map[string]any(m))
	case ApprovalView:
		if m.ID != "" {
			out["approvalId"] = m.ID
		}
	case []ApprovalView:
		for _, item := range m {
			if item.ID != "" {
				out["approvalId"] = item.ID
				break
			}
		}
	case WebhookLogView:
		addEntityIDFromMap(out, entity, map[string]any(m))
	case []map[string]any:
		for _, item := range m {
			if id := oneUserStringFromMap(item, "id", "_id"); id != "" {
				out[idKey(entity+"_id")] = id
				break
			}
		}
	case []AssignmentView:
		for _, item := range m {
			if id := oneUserStringFromMap(map[string]any(item), "id", "_id"); id != "" {
				out[idKey(entity+"_id")] = id
				break
			}
		}
	case []any:
		for _, item := range m {
			if asMap, ok := item.(map[string]any); ok {
				if id := oneUserStringFromMap(asMap, "id", "_id"); id != "" {
					out[idKey(entity+"_id")] = id
					break
				}
			}
		}
	}
	return out
}

func addEntityIDFromMap(out map[string]string, entity string, m map[string]any) {
	for _, key := range []string{
		"workspaceId",
		"userId",
		"clientId",
		"projectId",
		"taskId",
		"tagId",
		"entryId",
		"invoiceId",
		"invoiceItemId",
		"paymentId",
		"expenseId",
		"categoryId",
		"customFieldId",
		"policyId",
		"requestId",
		"assignmentId",
		"approvalId",
		"webhookId",
		"groupId",
		"holidayId",
	} {
		if value := oneUserStringFromMap(m, key, snakeIDKey(key)); value != "" {
			out[key] = value
		}
	}
	if id := oneUserStringFromMap(m, "id", "_id"); id != "" {
		out[idKey(entity+"_id")] = id
	}
}

func snakeIDKey(key string) string {
	var b strings.Builder
	for i, r := range key {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

func addEntryIDs(out map[string]string, entryID, workspaceID, userID, projectID, taskID string) {
	if entryID != "" {
		out["entryId"] = entryID
	}
	if workspaceID != "" {
		out["workspaceId"] = workspaceID
	}
	if userID != "" {
		out["userId"] = userID
	}
	if projectID != "" {
		out["projectId"] = projectID
	}
	if taskID != "" {
		out["taskId"] = taskID
	}
}

func firstEntityID(entity string, ids map[string]string) string {
	preferred := idKey(entity + "_id")
	if ids[preferred] != "" {
		return ids[preferred]
	}
	keys := make([]string, 0, len(ids))
	for key := range ids {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if strings.HasSuffix(strings.ToLower(key), "id") && key != "workspaceId" {
			return ids[key]
		}
	}
	return ids["workspaceId"]
}

func entityName(data any) string {
	switch v := data.(type) {
	case clockify.TimeEntry:
		return v.Description
	case EntryView:
		return v.Description
	case map[string]any:
		return oneUserStringFromMap(v, "name", "description", "number")
	case InvoiceView:
		return oneUserStringFromMap(map[string]any(v), "name", "description", "number")
	case InvoiceItemView:
		return oneUserStringFromMap(map[string]any(v), "name", "description", "number")
	case InvoicePaymentView:
		return oneUserStringFromMap(map[string]any(v), "name", "description", "number")
	case ExpenseView:
		return oneUserStringFromMap(map[string]any(v), "name", "description", "number")
	case TimeOffPolicyView:
		return oneUserStringFromMap(map[string]any(v), "name", "description", "number")
	case TimeOffRequestView:
		return oneUserStringFromMap(map[string]any(v), "name", "description", "number")
	case AssignmentView:
		return oneUserStringFromMap(map[string]any(v), "name", "description", "number")
	}
	return ""
}

func oneUserStringFromMap(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := m[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func idKey(name string) string {
	name = strings.TrimSuffix(name, "_id")
	name = strings.TrimSuffix(name, "Id")
	parts := strings.Split(name, "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "") + "Id"
}

func bodyName(name string) string {
	switch name {
	case "client_id":
		return "clientId"
	case "project_id":
		return "projectId"
	case "task_id":
		return "taskId"
	case "tag_ids":
		return "tagIds"
	case "time_entry_ids":
		return "timeEntryIds"
	case "expense_ids":
		return "expenseIds"
	case "user_ids":
		return "userIds"
	case "user_group_ids":
		return "userGroupIds"
	case "entry_ids":
		return "entryIds"
	case "approval_id":
		return "approvalRequestId"
	case "time_entry_group_type":
		return "timeEntryGroupType"
	case "is_public":
		return "isPublic"
	case "send_email":
		return "sendEmail"
	default:
		parts := strings.Split(name, "_")
		for i := 1; i < len(parts); i++ {
			if parts[i] != "" {
				parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
			}
		}
		return strings.Join(parts, "")
	}
}

func scalarToString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, scalarToString(item))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprint(value)
	}
}
