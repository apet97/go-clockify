package tools

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/timeparse"
)

// auditLogActionEnum is the closed set of audit-log action types the
// Clockify audit-log API accepts in AuditLogGetRequestV1.actions.
var auditLogActionEnum = []string{
	"CREATE_TIME_PERSONAL_TIMER", "CREATE_TIME_PERSONAL_MANUAL", "CREATE_TIME_IMPORT",
	"CREATE_TIME_KIOSK", "CREATE_TIME_FOR_OTHER", "RESTORE_TIME", "RESTORE_TIME_FOR_OTHER",
	"UPDATE_TIME_PERSONAL", "UPDATE_TIME_FOR_OTHER", "DELETE_TIME_PERSONAL", "DELETE_TIME_FOR_OTHER",
	"CREATE_PROJECT", "CREATE_PROJECT_IMPORT", "CREATE_PROJECT_QUICKBOOKS", "UPDATE_PROJECT", "DELETE_PROJECT",
	"CREATE_TASK", "CREATE_TASK_IMPORT", "UPDATE_TASK", "DELETE_TASK",
	"CREATE_CLIENT", "CREATE_CLIENT_IMPORT", "CREATE_CLIENT_QUICKBOOKS", "UPDATE_CLIENT", "DELETE_CLIENT",
	"CREATE_TAG", "CREATE_TAG_IMPORT", "UPDATE_TAG", "DELETE_TAG",
	"CREATE_EXPENSE", "CREATE_EXPENSE_FOR_OTHER", "RESTORE_EXPENSE", "RESTORE_EXPENSE_FOR_OTHER",
	"UPDATE_EXPENSE", "UPDATE_EXPENSE_FOR_OTHER", "DELETE_EXPENSE", "DELETE_EXPENSE_FOR_OTHER",
}

// entityChangeTypeEnum is the closed set of entity types the experimental
// entity-changes API filters on via the repeated `type` query parameter.
var entityChangeTypeEnum = []string{
	"CLIENTS", "PROJECTS", "TAGS", "TASKS", "SCHEDULED_ASSIGNMENT",
	"TIME_ENTRY", "TIME_ENTRY_RATE", "TIME_ENTRY_CUSTOM_FIELD_VALUE",
}

// auditLogDescriptors exposes the Clockify audit-log report and the
// experimental entity-changes feed as native domain tools. They slot into
// the registry after the high-value domain tools and before the timer and
// report helpers so the raw API fallback stays last.
func (s *Service) auditLogDescriptors() []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		nativeDomainTool(1200, toolRO(
			"clockify_audit_logs_search",
			"Search the workspace audit log for create/update/delete actions by author and date range. Reads the dedicated Clockify audit-log API.",
			auditLogsSearchSchema(),
		), "audit_log", "", s.AuditLogsSearch),
		nativeDomainTool(1201, toolRO(
			"clockify_entity_changes_list",
			"List entities created, updated, or deleted within a date range. Experimental Clockify API: the response is a bare array of change documents whose fields vary by entity type.",
			entityChangesListSchema(),
		), "entity_change", "", s.EntityChangesList),
	}
}

func auditLogsSearchSchema() map[string]any {
	return objectSchema(map[string]any{
		"required": []string{"actions", "author_ids", "start", "end"},
		"properties": map[string]any{
			"actions": map[string]any{
				"type":        "array",
				"minItems":    1,
				"items":       map[string]any{"type": "string", "enum": auditLogActionEnum},
				"description": "Audit-log action types to include; at least one is required.",
			},
			"author_ids": map[string]any{
				"type":        "array",
				"minItems":    1,
				"items":       map[string]any{"type": "string"},
				"description": `User IDs whose actions to match; at least one is required. Use "SYSTEM" to include system-generated audit entries.`,
			},
			"authors_contains": map[string]any{
				"type":        "string",
				"enum":        []string{"CONTAINS", "DOES_NOT_CONTAIN"},
				"description": "Treat author_ids as an include (CONTAINS) or exclude (DOES_NOT_CONTAIN) filter. Default CONTAINS.",
			},
			"start": map[string]any{"type": "string", "description": flexibleDatetimeDescription + " The start-to-end window must not exceed 31 days; split longer periods into separate calls."},
			"end":   map[string]any{"type": "string", "description": flexibleDatetimeDescription + " The start-to-end window must not exceed 31 days; split longer periods into separate calls."},
			"page":  map[string]any{"type": "integer", "minimum": 1, "description": "Result page, 1-indexed (Clockify audit-log default 1)."},
		},
	})
}

func entityChangesListSchema() map[string]any {
	return objectSchema(map[string]any{
		"required": []string{"change_type", "entity_types", "start", "end"},
		"properties": map[string]any{
			"change_type": map[string]any{
				"type":        "string",
				"enum":        []string{"created", "updated", "deleted"},
				"description": "Which change feed to read: created, updated, or deleted entities.",
			},
			"entity_types": map[string]any{
				"type":        "array",
				"minItems":    1,
				"items":       map[string]any{"type": "string", "enum": entityChangeTypeEnum},
				"description": "Entity types to include; at least one is required.",
			},
			"start": map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"end":   map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"page":  map[string]any{"type": "integer", "minimum": 0, "description": "Result page, 0-indexed (Clockify entity-changes default 0)."},
			"limit": map[string]any{"type": "integer", "minimum": 1, "description": "Maximum results per page (Clockify entity-changes default 50)."},
		},
	})
}

// AuditLogsSearch backs clockify_audit_logs_search. It posts an
// AuditLogGetRequestV1 filter to the dedicated Clockify audit-log host. The
// live API returns a bare JSON array of audit-log entries, not the
// PageableV1ListAuditLogDtoV1 envelope the OpenAPI document describes; a live
// probe against the sacrificial workspace confirmed the bare-array shape.
func (s *Service) AuditLogsSearch(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	actions, _, err := strictStringSliceArg(args, "actions")
	if err != nil {
		return ResultEnvelope{}, err
	}
	if len(actions) == 0 {
		return ResultEnvelope{}, fmt.Errorf("actions is required (at least one audit-log action type)")
	}
	authorIDs, _, err := strictStringSliceArg(args, "author_ids")
	if err != nil {
		return ResultEnvelope{}, err
	}
	if len(authorIDs) == 0 {
		return ResultEnvelope{}, fmt.Errorf(`author_ids is required (user IDs, or "SYSTEM" for system entries)`)
	}
	contains := strings.ToUpper(strings.TrimSpace(stringArg(args, "authors_contains")))
	if contains == "" {
		contains = "CONTAINS"
	}
	if contains != "CONTAINS" && contains != "DOES_NOT_CONTAIN" {
		return ResultEnvelope{}, fmt.Errorf("authors_contains must be CONTAINS or DOES_NOT_CONTAIN")
	}
	startTime, endTime, err := parseRangeInLocation(args, s.location())
	if err != nil {
		return ResultEnvelope{}, err
	}
	// The Clockify audit-log API rejects windows wider than 31 days with an
	// opaque 400. Catch it here with a recovery instruction instead.
	const auditLogMaxRangeDays = 31
	if endTime.Sub(startTime) > auditLogMaxRangeDays*24*time.Hour {
		return ResultEnvelope{}, fmt.Errorf(
			"audit log date range of %.0f days exceeds the Clockify maximum of %d days; split the request into %d-day windows",
			endTime.Sub(startTime).Hours()/24, auditLogMaxRangeDays, auditLogMaxRangeDays)
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "audit-log")
	if err != nil {
		return ResultEnvelope{}, err
	}
	body := map[string]any{
		"actions": actions,
		"authors": map[string]any{"authorIds": authorIDs, "contains": contains},
		"start":   timeparse.FormatISO(startTime),
		"end":     timeparse.FormatISO(endTime),
	}
	if _, present := args["page"]; present {
		body["page"] = intArg(args, "page", 1)
	}
	var entries []map[string]any
	if err := s.Client.PostAuditLog(ctx, path, body, &entries); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_audit_logs_search", entries, map[string]any{
		"workspaceId": wsID,
		"count":       len(entries),
	}), nil
}

// EntityChangesList backs clockify_entity_changes_list. It reads the
// experimental entities/{created,updated,deleted} feed on the primary host.
// The live API returns a bare array of change documents; the OpenAPI
// envelope shape was never observed in practice, so the handler decodes a
// plain array and surfaces a recovery envelope if Clockify ever changes it.
func (s *Service) EntityChangesList(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	changeType := strings.ToLower(strings.TrimSpace(stringArg(args, "change_type")))
	switch changeType {
	case "created", "updated", "deleted":
	default:
		return ResultEnvelope{}, fmt.Errorf("change_type must be created, updated, or deleted")
	}
	entityTypes, _, err := strictStringSliceArg(args, "entity_types")
	if err != nil {
		return ResultEnvelope{}, err
	}
	if len(entityTypes) == 0 {
		return ResultEnvelope{}, fmt.Errorf("entity_types is required (at least one entity type)")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "entities", changeType)
	if err != nil {
		return ResultEnvelope{}, err
	}
	// start/end are required (Axiom 14: the resolved date range must be
	// explicit). parseRangeInLocation also enforces end > start.
	startTime, endTime, err := parseRangeInLocation(args, s.location())
	if err != nil {
		return ResultEnvelope{}, err
	}
	query := url.Values{}
	for _, entityType := range entityTypes {
		query.Add("type", entityType)
	}
	query.Set("start", timeparse.FormatISO(startTime))
	query.Set("end", timeparse.FormatISO(endTime))
	page := intArg(args, "page", 0)
	limit := intArg(args, "limit", 50)
	if _, present := args["page"]; present {
		query.Set("page", strconv.Itoa(page))
	}
	if _, present := args["limit"]; present {
		query.Set("limit", strconv.Itoa(limit))
	}
	var changes []map[string]any
	if err := s.Client.GetValues(ctx, path, query, &changes); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_entity_changes_list", changes, map[string]any{
		"workspaceId":   wsID,
		"changeType":    changeType,
		"count":         len(changes),
		"page":          page,
		"limit":         limit,
		"has_more_hint": len(changes) == limit,
	}), nil
}
