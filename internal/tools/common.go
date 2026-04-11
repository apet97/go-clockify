package tools

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/dedupe"
	"github.com/apet97/go-clockify/internal/mcp"
)

type Service struct {
	Client          *clockify.Client
	WorkspaceID     string
	DefaultTimezone *time.Location        // from CLOCKIFY_TIMEZONE; nil = system timezone
	DedupeConfig    *dedupe.Config        // optional, set during wiring
	PolicyDescribe  func() map[string]any // set during wiring; returns policy description
	ActivateGroup   func(context.Context, string) (ActivationResult, error)
	ActivateTool    func(context.Context, string) (ActivationResult, error)
	// ReportMaxEntries is the hard cap on the number of time entries a report
	// tool will aggregate. 0 disables the cap. Wired from CLOCKIFY_REPORT_MAX_ENTRIES.
	ReportMaxEntries int
	mu               sync.Mutex
	cachedUser       *clockify.User
	cachedWSID       string
}

type ActivationResult struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Group     string `json:"group,omitempty"`
	ToolCount int    `json:"toolCount"`
}

type ResultEnvelope struct {
	OK     bool           `json:"ok"`
	Action string         `json:"action"`
	Data   any            `json:"data,omitempty"`
	Meta   map[string]any `json:"meta,omitempty"`
}

type WorkspaceContext struct {
	WorkspaceID string `json:"workspaceId"`
}

type IdentityData struct {
	User        clockify.User `json:"user"`
	WorkspaceID string        `json:"workspaceId"`
}

type WeeklySummaryData struct {
	Range         DateRange            `json:"range"`
	Totals        SummaryTotals        `json:"totals"`
	ByDay         []DaySummary         `json:"byDay"`
	ByProject     []ProjectSummary     `json:"byProject"`
	Entries       []clockify.TimeEntry `json:"entries,omitempty"`
	UnassignedKey string               `json:"unassignedKey,omitempty"`
}

type SummaryData struct {
	Range     DateRange            `json:"range"`
	Totals    SummaryTotals        `json:"totals"`
	ByProject []ProjectSummary     `json:"byProject"`
	Entries   []clockify.TimeEntry `json:"entries,omitempty"`
}

type QuickReportData struct {
	Range               DateRange            `json:"range"`
	Totals              SummaryTotals        `json:"totals"`
	TopProject          *ProjectSummary      `json:"topProject,omitempty"`
	RunningEntries      []clockify.TimeEntry `json:"runningEntries,omitempty"`
	EntriesSample       []clockify.TimeEntry `json:"entriesSample,omitempty"`
	ProjectsRepresented int                  `json:"projectsRepresented"`
}

type LogTimeData struct {
	Entry           clockify.TimeEntry `json:"entry"`
	ResolvedProject string             `json:"resolvedProject,omitempty"`
}

type FindAndUpdateEntryData struct {
	Entry         clockify.TimeEntry `json:"entry"`
	MatchedBy     map[string]any     `json:"matchedBy"`
	UpdatedFields []string           `json:"updatedFields"`
}

type DateRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type SummaryTotals struct {
	Entries        int     `json:"entries"`
	RunningEntries int     `json:"runningEntries"`
	TotalSeconds   int64   `json:"totalSeconds"`
	TotalHours     float64 `json:"totalHours"`
}

type ProjectSummary struct {
	ProjectID    string  `json:"projectId,omitempty"`
	ProjectName  string  `json:"projectName"`
	Entries      int     `json:"entries"`
	TotalSeconds int64   `json:"totalSeconds"`
	TotalHours   float64 `json:"totalHours"`
}

type DaySummary struct {
	Date         string  `json:"date"`
	Entries      int     `json:"entries"`
	TotalSeconds int64   `json:"totalSeconds"`
	TotalHours   float64 `json:"totalHours"`
}

type findAndUpdateArgs struct {
	DescriptionContains string
	ExactDescription    string
	EntryID             string
	StartAfter          string
	StartBefore         string
	NewDescription      string
	ProjectID           string
	Project             string
	Start               string
	End                 string
	Billable            *bool
	DryRun              bool
}

func New(client *clockify.Client, workspaceID string) *Service {
	return &Service{Client: client, WorkspaceID: workspaceID}
}

// baseAnnotations returns the common annotation map every tool carries.
// openWorldHint is always true because every Clockify MCP tool touches the
// external Clockify API (not a closed local system), and title is derived
// from the tool name for display in MCP clients that render a tool picker.
// Callers overlay hint fields (readOnlyHint, destructiveHint, idempotentHint)
// on top of this base so each descriptor ends up with a complete annotation
// set instead of a sparse one that spec-strict clients misinterpret.
func baseAnnotations(name string) map[string]any {
	return map[string]any{
		"title":         titleFromName(name),
		"openWorldHint": true,
	}
}

// titleFromName converts a snake_case tool name into a human-readable title.
// "clockify_list_entries" → "List Entries", "clockify_quick_report" → "Quick
// Report". Custom per-tool titles can be added later by overriding the
// "title" key after the base annotations are copied.
func titleFromName(name string) string {
	stripped := strings.TrimPrefix(name, "clockify_")
	if stripped == "" {
		return name
	}
	parts := strings.Split(stripped, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

func toolRO(name, desc string, schema map[string]any) mcp.Tool {
	ann := baseAnnotations(name)
	ann["readOnlyHint"] = true
	ann["destructiveHint"] = false
	ann["idempotentHint"] = true
	return mcp.Tool{Name: name, Description: desc, InputSchema: schema, Annotations: ann}
}

func toolRW(name, desc string, schema map[string]any) mcp.Tool {
	ann := baseAnnotations(name)
	ann["readOnlyHint"] = false
	// Explicitly declare non-destructive. Absent this, MCP spec-strict
	// clients assume destructive for write tools and may require extra
	// confirmation for every call.
	ann["destructiveHint"] = false
	ann["idempotentHint"] = false
	return mcp.Tool{Name: name, Description: desc, InputSchema: schema, Annotations: ann}
}

// toolRWIdem marks a write tool as idempotent. Use for PUT/PATCH-style updates
// and tools whose handlers produce the same end state on repeated calls
// (e.g. clockify_stop_timer when no timer is running becomes a no-op).
func toolRWIdem(name, desc string, schema map[string]any) mcp.Tool {
	ann := baseAnnotations(name)
	ann["readOnlyHint"] = false
	ann["destructiveHint"] = false
	ann["idempotentHint"] = true
	return mcp.Tool{Name: name, Description: desc, InputSchema: schema, Annotations: ann}
}

func toolDestructive(name, desc string, schema map[string]any) mcp.Tool {
	ann := baseAnnotations(name)
	ann["readOnlyHint"] = false
	ann["destructiveHint"] = true
	ann["idempotentHint"] = false
	return mcp.Tool{Name: name, Description: desc, InputSchema: schema, Annotations: ann}
}

func normalizeDescriptors(in []mcp.ToolDescriptor) []mcp.ToolDescriptor {
	for i := range in {
		if value, ok := in[i].Tool.Annotations["readOnlyHint"].(bool); ok {
			in[i].ReadOnlyHint = value
		}
		if value, ok := in[i].Tool.Annotations["destructiveHint"].(bool); ok {
			in[i].DestructiveHint = value
		}
		if value, ok := in[i].Tool.Annotations["idempotentHint"].(bool); ok {
			in[i].IdempotentHint = value
		}
	}
	return in
}

func requiredSchema(field string) map[string]any {
	return map[string]any{"type": "object", "required": []string{field}, "properties": map[string]any{field: map[string]any{"type": "string"}}}
}

// paginationSchema returns a JSON schema with standard `page`/`page_size`
// integer properties merged with the caller's extras. The extras map may
// supply additional `properties` (merged) and `required` (concatenated).
func paginationSchema(extras map[string]any) map[string]any {
	props := map[string]any{
		"page":      map[string]any{"type": "integer", "description": "Page number (default 1)"},
		"page_size": map[string]any{"type": "integer", "description": "Items per page (default 50, max 200)"},
	}
	schema := map[string]any{"type": "object", "properties": props}
	if extras == nil {
		return schema
	}
	if extra, ok := extras["properties"].(map[string]any); ok {
		for k, v := range extra {
			props[k] = v
		}
	}
	if req, ok := extras["required"].([]string); ok && len(req) > 0 {
		schema["required"] = req
	}
	return schema
}

func stringArg(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func boolArg(args map[string]any, key string) bool {
	v, _ := args[key].(bool)
	return v
}

// paginationFromArgs extracts page/page_size from a tool args map, applying
// the standard defaults (page=1, page_size=50) and a hard cap of 200.
func paginationFromArgs(args map[string]any) (page, pageSize int) {
	page = intArg(args, "page", 1)
	if page < 1 {
		page = 1
	}
	pageSize = intArg(args, "page_size", 50)
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

func intArg(args map[string]any, key string, fallback int) int {
	v, ok := args[key]
	if !ok {
		return fallback
	}
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) || x < math.MinInt || x > math.MaxInt {
			return fallback
		}
		return int(x)
	default:
		return fallback
	}
}

func ok(action string, data any, meta map[string]any) ResultEnvelope {
	if meta == nil {
		meta = map[string]any{}
	}
	return ResultEnvelope{OK: true, Action: action, Data: data, Meta: meta}
}

func hours(seconds int64) float64 {
	return float64(seconds) / 3600.0
}

func loadLocation(name string, defaultTZ *time.Location) (*time.Location, error) {
	if strings.TrimSpace(name) == "" {
		if defaultTZ != nil {
			return defaultTZ, nil
		}
		return time.Now().Location(), nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %q: %w", name, err)
	}
	return loc, nil
}

func parseFlexibleDateTime(raw string, loc *time.Location) (time.Time, error) {
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts.In(loc), nil
	}
	if d, err := time.ParseInLocation("2006-01-02", raw, loc); err == nil {
		return d, nil
	}
	return time.Time{}, fmt.Errorf("expected RFC3339 or YYYY-MM-DD date, got %q", raw)
}

func parseRange(args map[string]any) (time.Time, time.Time, error) {
	startRaw := stringArg(args, "start")
	endRaw := stringArg(args, "end")
	if startRaw == "" || endRaw == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("start and end are required")
	}
	start, err := time.Parse(time.RFC3339, startRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("start must be RFC3339: %w", err)
	}
	end, err := time.Parse(time.RFC3339, endRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("end must be RFC3339: %w", err)
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("end must be after start")
	}
	return start.UTC(), end.UTC(), nil
}

func parseStartEnd(args map[string]any) (time.Time, time.Time, error) {
	return parseRange(args)
}

// entryRangeQuery builds the base date-range query for time-entry reports.
// Pagination params are set by the paginator in aggregateEntriesRange; this
// helper intentionally does NOT set page or page-size.
func entryRangeQuery(start, end time.Time) map[string]string {
	return map[string]string{
		"start": start.UTC().Format(time.RFC3339),
		"end":   end.UTC().Format(time.RFC3339),
	}
}
