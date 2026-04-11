package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/jsonmergepatch"
	"github.com/apet97/go-clockify/internal/mcp"
)

// diffResourceState computes the wire-format delta between two cached
// serialisations of a subscribed resource. Thin wrapper around
// jsonmergepatch.DiffOrFull that lives in the tools layer so
// emitResourceUpdate doesn't reach into the sub-package directly for
// every mutation path (easier to swap the differ later).
func diffResourceState(prev, curr []byte) ([]byte, string, error) {
	return jsonmergepatch.DiffOrFull(prev, curr)
}

// entryResourceURI builds the canonical resource URI for a single time
// entry. Returns empty if any required piece is missing so callers can
// safely skip the notification emit step.
func entryResourceURI(workspaceID, entryID string) string {
	if workspaceID == "" || entryID == "" {
		return ""
	}
	return fmt.Sprintf("clockify://workspace/%s/entry/%s", workspaceID, entryID)
}

// projectResourceURI builds the canonical resource URI for a project.
func projectResourceURI(workspaceID, projectID string) string {
	if workspaceID == "" || projectID == "" {
		return ""
	}
	return fmt.Sprintf("clockify://workspace/%s/project/%s", workspaceID, projectID)
}

// userResourceURI builds the canonical resource URI for a user in a
// workspace. Matches the `clockify://workspace/{workspaceId}/user/{userId}`
// template advertised in ListResourceTemplates. Returns empty when either
// piece is missing so tier 2 mutation handlers can safely skip the emit
// step instead of publishing a malformed URI.
func userResourceURI(workspaceID, userID string) string {
	if workspaceID == "" || userID == "" {
		return ""
	}
	return fmt.Sprintf("clockify://workspace/%s/user/%s", workspaceID, userID)
}

// weeklyReportResourceURI builds the canonical resource URI for an
// aggregated weekly report keyed by the ISO Monday YYYY-MM-DD date.
// Matches the `clockify://workspace/{workspaceId}/report/weekly/{weekStart}`
// template in ListResourceTemplates.
func weeklyReportResourceURI(workspaceID, weekStart string) string {
	if workspaceID == "" || weekStart == "" {
		return ""
	}
	return fmt.Sprintf("clockify://workspace/%s/report/weekly/%s", workspaceID, weekStart)
}

// isoWeekStart returns the Monday 00:00 local time that starts the ISO
// week containing t, localised to loc. Matches the semantics of
// weekBounds() in reports.go — callers on both paths get the same
// weekStart string when they format the result as YYYY-MM-DD.
func isoWeekStart(t time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	local := t.In(loc)
	weekday := int(local.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -(weekday - 1))
}

// weeklyReportURIsForEntry returns the weekly-report URIs that should
// be invalidated when a time entry at [startRaw, endRaw) is created or
// mutated. Typically one URI; two when the entry spans two ISO weeks
// (rare, e.g. a Sunday 23:00 → Monday 01:00 entry). Returns nil when
// the start cannot be parsed so callers silently skip the emit — a
// malformed timestamp should not block the primary entry-URI path.
//
// endRaw may be empty (running timer); in that case only the start
// week is invalidated because the entry hasn't crossed a boundary yet.
func weeklyReportURIsForEntry(workspaceID, startRaw, endRaw string, loc *time.Location) []string {
	if workspaceID == "" || strings.TrimSpace(startRaw) == "" {
		return nil
	}
	start, err := time.Parse(time.RFC3339, startRaw)
	if err != nil {
		return nil
	}
	startWeek := isoWeekStart(start, loc).Format("2006-01-02")
	uris := []string{weeklyReportResourceURI(workspaceID, startWeek)}
	if strings.TrimSpace(endRaw) == "" {
		return uris
	}
	end, err := time.Parse(time.RFC3339, endRaw)
	if err != nil {
		return uris
	}
	endWeek := isoWeekStart(end, loc).Format("2006-01-02")
	if endWeek != startWeek {
		uris = append(uris, weeklyReportResourceURI(workspaceID, endWeek))
	}
	return uris
}

// emitEntryAndWeekly is a small convenience wrapper used by every
// entry-mutating handler: emit the concrete entry URI, then fan out
// to every weekly-report URI invalidated by the mutation.
func (s *Service) emitEntryAndWeekly(ctx context.Context, wsID, entryID, startRaw, endRaw string) {
	s.emitResourceUpdate(ctx, entryResourceURI(wsID, entryID))
	loc := s.DefaultTimezone
	if loc == nil {
		loc = time.UTC
	}
	for _, uri := range weeklyReportURIsForEntry(wsID, startRaw, endRaw, loc) {
		s.emitResourceUpdate(ctx, uri)
	}
}

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

// emitResourceUpdate re-reads the resource at uri through ResourceProvider,
// diffs it against the last-cached serialisation (resourceCache), and
// publishes a notifications/resources/updated with the smallest RFC 7396
// merge patch, falling back to format=full when the target contains an
// explicit null or format=none when there is no prior cached state.
//
// Silent no-op when:
//   - s.EmitResourceUpdate hook is not wired (tests without a Server)
//   - the resource has no active subscription (the hook delegates to
//     Server.NotifyResourceUpdated which checks the subscription set)
//
// Failure modes (read error, marshal error, diff error) fall through to
// emitting format=none so the subscribed client knows to re-fetch.
// Deletes are signalled via emitResourceDeleted rather than this helper.
func (s *Service) emitResourceUpdate(ctx context.Context, uri string) {
	if s == nil || s.EmitResourceUpdate == nil || uri == "" {
		return
	}
	contents, err := s.ReadResource(ctx, uri)
	if err != nil {
		s.resourceCache.drop(uri)
		s.EmitResourceUpdate(uri, mcp.ResourceUpdateDelta{Format: "none"})
		return
	}
	newState := flattenResourceContents(contents)
	if newState == nil {
		s.EmitResourceUpdate(uri, mcp.ResourceUpdateDelta{Format: "none"})
		return
	}
	prevState, hadPrev := s.resourceCache.get(uri)
	s.resourceCache.put(uri, newState)
	if !hadPrev {
		s.EmitResourceUpdate(uri, mcp.ResourceUpdateDelta{Format: "none"})
		return
	}
	patchBytes, format, err := diffResourceState(prevState, newState)
	if err != nil {
		s.EmitResourceUpdate(uri, mcp.ResourceUpdateDelta{Format: "none"})
		return
	}
	var patchValue any
	if err := json.Unmarshal(patchBytes, &patchValue); err != nil {
		s.EmitResourceUpdate(uri, mcp.ResourceUpdateDelta{Format: "none"})
		return
	}
	s.EmitResourceUpdate(uri, mcp.ResourceUpdateDelta{Format: format, Patch: patchValue})
}

// emitResourceDeleted publishes a notifications/resources/updated with
// format=deleted and drops the cached state so the next subscribe starts
// from scratch. Use for confirmed deletes where re-reading would return
// a 404 and make emitResourceUpdate emit format=none instead.
func (s *Service) emitResourceDeleted(uri string) {
	if s == nil || s.EmitResourceUpdate == nil || uri == "" {
		return
	}
	s.resourceCache.drop(uri)
	s.EmitResourceUpdate(uri, mcp.ResourceUpdateDelta{Format: "deleted"})
}

// flattenResourceContents joins the Text portions of a ResourceContents
// slice into a single byte stream for diffing. Every Clockify resource
// template today produces a single-element slice with JSON in Text, so
// this is effectively a type-narrowing helper; the loop is future-proofing
// for multi-part contents.
func flattenResourceContents(contents []mcp.ResourceContents) []byte {
	if len(contents) == 0 {
		return nil
	}
	if len(contents) == 1 {
		return []byte(contents[0].Text)
	}
	// Multi-part: concatenate inside a wrapping JSON array so the diff
	// still operates on structured data rather than raw string concat.
	parts := make([]any, 0, len(contents))
	for _, c := range contents {
		var decoded any
		if json.Unmarshal([]byte(c.Text), &decoded) == nil {
			parts = append(parts, decoded)
		}
	}
	out, err := json.Marshal(parts)
	if err != nil {
		return nil
	}
	return out
}
