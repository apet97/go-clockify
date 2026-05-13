package tools

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"strings"

	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/paths"
)

type EntityChangesData struct {
	Kind    string             `json:"kind"`
	Types   []string           `json:"types,omitempty"`
	Changes []EntityChangeView `json:"changes"`
	Raw     []map[string]any   `json:"raw,omitempty"`
}

type EntityChangeView map[string]any

func init() {
	registerTier2Group(Tier2Group{
		Name:        "change_tracking",
		Description: "Read-only created, updated, and deleted entity change feeds",
		Keywords:    []string{"changes", "audit", "created", "updated", "deleted", "sync"},
		ToolNames: []string{
			"clockify_entity_changes",
		},
		Builder: changeTrackingHandlers,
	})
}

func changeTrackingHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		{Tool: withOutputSchema(toolRO("clockify_entity_changes", "List created, updated, or deleted Clockify entities for sync/audit enrichment", map[string]any{
			"type":     "object",
			"required": []string{"change_kind", "start", "end"},
			"properties": map[string]any{
				"change_kind": map[string]any{"type": "string", "enum": []string{"created", "updated", "deleted"}},
				"types":       stringArraySchema("Clockify entity types, e.g. TIME_ENTRY, PROJECT, TASK, CLIENT"),
				"start":       map[string]any{"type": "string", "description": "Start timestamp/date for the change window"},
				"end":         map[string]any{"type": "string", "description": "End timestamp/date for the change window"},
				"page":        map[string]any{"type": "integer", "minimum": 1},
				"limit":       map[string]any{"type": "integer", "minimum": 1, "maximum": 1000},
			},
		}), envelopeSchemaFor[EntityChangesData]("clockify_entity_changes")), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.entityChanges(ctx, args)
		}},
	}
}

func (s *Service) entityChanges(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	kind := strings.ToLower(strings.TrimSpace(stringArg(args, "change_kind")))
	switch kind {
	case "created", "updated", "deleted":
	default:
		return ResultEnvelope{}, fmt.Errorf("change_kind must be created, updated, or deleted")
	}
	start := strings.TrimSpace(stringArg(args, "start"))
	end := strings.TrimSpace(stringArg(args, "end"))
	if start == "" || end == "" {
		return ResultEnvelope{}, fmt.Errorf("start and end are required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "entities", kind)
	if err != nil {
		return ResultEnvelope{}, err
	}
	query := url.Values{}
	query.Set("start", start)
	query.Set("end", end)
	if page := intArg(args, "page", 1); page > 0 {
		query.Set("page", fmt.Sprintf("%d", page))
	}
	if limit := intArg(args, "limit", 100); limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", limit))
	}
	types, _, err := strictStringSliceArg(args, "types")
	if err != nil {
		return ResultEnvelope{}, err
	}
	for i := range types {
		types[i] = strings.ToUpper(strings.TrimSpace(types[i]))
		if types[i] != "" {
			query.Add("type", types[i])
		}
	}
	var changes []map[string]any
	if err := s.Client.GetValues(ctx, path, query, &changes); err != nil {
		return ResultEnvelope{}, err
	}
	views := make([]EntityChangeView, 0, len(changes))
	for _, change := range changes {
		views = append(views, entityChangeViewFromRaw(kind, change))
	}
	return ok("clockify_entity_changes", EntityChangesData{
		Kind:    kind,
		Types:   types,
		Changes: views,
		Raw:     changes,
	}, map[string]any{
		"workspaceId": wsID,
		"count":       len(views),
		"kind":        kind,
	}), nil
}

func entityChangeViewFromRaw(kind string, raw map[string]any) EntityChangeView {
	view := EntityChangeView(maps.Clone(raw))
	view["change_kind"] = kind
	view["audit_metadata"] = map[string]any{
		"kind":       kind,
		"entityType": firstReportString(raw, "type", "entityType", "entity_type"),
		"createdAt":  firstReportString(raw, "createdAt", "created_at"),
		"updatedAt":  firstReportString(raw, "updatedAt", "updated_at"),
		"deletedAt":  firstReportString(raw, "deletedAt", "deleted_at"),
	}
	if code := firstReportString(raw, "documentCode", "document_code", "code"); code != "" {
		view["document_code"] = code
	}
	view["entity"] = map[string]any{
		"id":   cleanReportID(firstPresent(raw, "entityId", "entity_id", "id", "_id")),
		"type": firstReportString(raw, "type", "entityType", "entity_type"),
		"name": firstReportString(raw, "name", "entityName", "entity_name", "description"),
	}
	if row := entityChangeTimeEntryRow(raw); row != nil {
		entry := reportEntryViewFromRow(row)
		view["time_entry"] = entry
		if entry.Approval != nil {
			view["approval"] = entry.Approval
		}
	}
	if custom := customFieldsFromFirst(raw, "customFieldValues", "customFields", "custom_field_values"); len(custom) > 0 {
		view["custom_fields_normalized"] = custom
	}
	view["raw"] = maps.Clone(raw)
	return view
}

func entityChangeTimeEntryRow(raw map[string]any) map[string]any {
	for _, key := range []string{"timeEntry", "time_entry", "timeEntryDto", "entity", "payload"} {
		if nested, ok := raw[key].(map[string]any); ok {
			entityType := strings.ToUpper(firstReportString(nested, "type", "entityType"))
			if entityType == "" || entityType == "TIME_ENTRY" || reportRowEntryID(nested) != "" {
				return nested
			}
		}
	}
	entityType := strings.ToUpper(firstReportString(raw, "type", "entityType", "entity_type"))
	if entityType == "TIME_ENTRY" || reportRowEntryID(raw) != "" {
		return raw
	}
	return nil
}
