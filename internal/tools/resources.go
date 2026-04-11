package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

const clockifyResourceScheme = "clockify://"

// ListResources returns a small, immediately-navigable set of concrete
// resources pinned to the Service's current workspace. Parametric resources
// (per-id entry, project, weekly report) live in ListResourceTemplates.
func (s *Service) ListResources(ctx context.Context) ([]mcp.Resource, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	return []mcp.Resource{
		{
			URI:         fmt.Sprintf("clockify://workspace/%s", wsID),
			Name:        "Current workspace",
			Description: "Active Clockify workspace for this MCP session.",
			MimeType:    "application/json",
		},
		{
			URI:         fmt.Sprintf("clockify://workspace/%s/user/current", wsID),
			Name:        "Current user",
			Description: "The user owning the API key this server is authenticated with.",
			MimeType:    "application/json",
		},
	}, nil
}

// ListResourceTemplates returns the five parametric URI templates clients
// can dereference by substituting concrete IDs.
func (s *Service) ListResourceTemplates(_ context.Context) ([]mcp.ResourceTemplate, error) {
	return []mcp.ResourceTemplate{
		{
			URITemplate: "clockify://workspace/{workspaceId}",
			Name:        "Workspace",
			Description: "A Clockify workspace by id.",
			MimeType:    "application/json",
		},
		{
			URITemplate: "clockify://workspace/{workspaceId}/user/{userId}",
			Name:        "User",
			Description: "A Clockify user in a workspace. Use `current` as the userId to get the authenticated user.",
			MimeType:    "application/json",
		},
		{
			URITemplate: "clockify://workspace/{workspaceId}/project/{projectId}",
			Name:        "Project",
			Description: "A Clockify project by id.",
			MimeType:    "application/json",
		},
		{
			URITemplate: "clockify://workspace/{workspaceId}/entry/{entryId}",
			Name:        "Time entry",
			Description: "A single time entry by id.",
			MimeType:    "application/json",
		},
		{
			URITemplate: "clockify://workspace/{workspaceId}/report/weekly/{weekStart}",
			Name:        "Weekly report",
			Description: "Aggregated weekly report keyed by ISO week-start date (YYYY-MM-DD).",
			MimeType:    "application/json",
		},
	}, nil
}

// ReadResource parses a clockify:// URI and fetches the underlying entity
// from Clockify, returning it as a single JSON-encoded ResourceContents entry.
// Unknown or malformed URIs return a -32602-equivalent error.
func (s *Service) ReadResource(ctx context.Context, uri string) ([]mcp.ResourceContents, error) {
	rest, ok := strings.CutPrefix(uri, clockifyResourceScheme)
	if !ok {
		return nil, fmt.Errorf("unsupported URI scheme: %q", uri)
	}
	segments := strings.Split(rest, "/")
	// Every supported URI starts with "workspace/{id}".
	if len(segments) < 2 || segments[0] != "workspace" || segments[1] == "" {
		return nil, fmt.Errorf("invalid clockify resource URI: %q", uri)
	}
	workspaceID := segments[1]

	// clockify://workspace/{id}
	if len(segments) == 2 {
		var out map[string]any
		if err := s.Client.Get(ctx, "/workspaces/"+workspaceID, nil, &out); err != nil {
			return nil, err
		}
		return encodeResource(uri, out)
	}

	if len(segments) < 4 {
		return nil, fmt.Errorf("invalid clockify resource URI: %q", uri)
	}

	kind := segments[2]
	id := segments[3]

	switch kind {
	case "user":
		if id == "current" {
			user, err := s.getCurrentUser(ctx)
			if err != nil {
				return nil, err
			}
			return encodeResource(uri, user)
		}
		var user clockify.User
		if err := s.Client.Get(ctx, "/workspaces/"+workspaceID+"/users/"+id, nil, &user); err != nil {
			return nil, err
		}
		return encodeResource(uri, user)

	case "project":
		var project clockify.Project
		if err := s.Client.Get(ctx, "/workspaces/"+workspaceID+"/projects/"+id, nil, &project); err != nil {
			return nil, err
		}
		return encodeResource(uri, project)

	case "entry":
		var entry clockify.TimeEntry
		if err := s.Client.Get(ctx, "/workspaces/"+workspaceID+"/time-entries/"+id, nil, &entry); err != nil {
			return nil, err
		}
		return encodeResource(uri, entry)

	case "report":
		// clockify://workspace/{id}/report/weekly/{weekStart}
		if len(segments) != 5 || segments[3] != "weekly" || segments[4] == "" {
			return nil, fmt.Errorf("invalid weekly report URI: %q", uri)
		}
		weekStart := segments[4]
		return s.readWeeklyReportResource(ctx, uri, workspaceID, weekStart)

	default:
		return nil, fmt.Errorf("unknown resource kind %q in URI %q", kind, uri)
	}
}

// readWeeklyReportResource wires the existing tool-layer weekly-report path
// into a resource read so clients reading `clockify://workspace/{ws}/report/weekly/{weekStart}`
// get the same aggregated shape as `clockify_weekly_summary`.
func (s *Service) readWeeklyReportResource(ctx context.Context, uri, workspaceID, weekStart string) ([]mcp.ResourceContents, error) {
	prev := s.WorkspaceID
	s.WorkspaceID = workspaceID
	defer func() { s.WorkspaceID = prev }()
	env, err := s.WeeklySummary(ctx, map[string]any{"week_start": weekStart})
	if err != nil {
		return nil, err
	}
	return encodeResource(uri, env.Data)
}

func encodeResource(uri string, payload any) ([]mcp.ResourceContents, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode resource: %w", err)
	}
	return []mcp.ResourceContents{{
		URI:      uri,
		MimeType: "application/json",
		Text:     string(body),
	}}, nil
}
