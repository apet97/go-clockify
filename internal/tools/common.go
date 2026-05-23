package tools

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

const (
	oneUserToolGuide                = "clockify_tools_guide"
	oneUserToolLogWork              = "clockify_log_work"
	oneUserToolReviewDay            = "clockify_review_day"
	oneUserToolEntriesCreateFromGap = "clockify_entries_create"
	oneUserToolSwitchWork           = "clockify_switch_work"
	oneUserToolEntriesTimerStatus   = "clockify_entries_timer_status"
	oneUserToolEntriesCreate        = "clockify_entries_create"
	oneUserToolEntriesTimerStart    = "clockify_entries_timer_start"
	oneUserToolFixEntry             = "clockify_fix_entry"
)

func timeEntryPutPayload(entry clockify.TimeEntry) map[string]any {
	payload := map[string]any{
		"start":       entry.TimeInterval.Start,
		"description": entry.Description,
		"projectId":   entry.ProjectID,
		"billable":    entry.Billable,
	}
	if entry.TimeInterval.End != "" {
		payload["end"] = entry.TimeInterval.End
	}
	if entry.TaskID != "" {
		payload["taskId"] = entry.TaskID
	}
	// type is REGULAR or BREAK; the live API leaves it untouched when
	// omitted on PUT but explicitly preserving it makes the fetch-then-
	// update flow round-trip identically. Empty strings are skipped so
	// entries the API hadn't materialised a type for don't trip the
	// "type required" validator on a regression-prone code path.
	if entry.Type != "" {
		payload["type"] = entry.Type
	}
	if len(entry.TagIDs) > 0 {
		payload["tagIds"] = entry.TagIDs
	}
	if entry.CustomFieldValues != nil {
		payload["customFields"] = entry.CustomFieldValues
	}
	return payload
}

type Service struct {
	Client          *clockify.Client
	WorkspaceID     string
	DefaultTimezone *time.Location // from CLOCKIFY_TIMEZONE; nil falls back to time.Now().Location() for flexible date/time inputs.
	// WebhookValidateDNS, when true, makes CreateWebhook/UpdateWebhook
	// resolve the webhook host via the system resolver and reject any
	// reply that contains a private/reserved IP. Config defaults this
	// on for every profile so a hostname pointing at 169.254.169.254
	// (cloud-metadata) or 10.0.0.x cannot turn the Clockify webhook
	// delivery into an SSRF probe. Operators can explicitly opt out
	// for trusted air-gapped tests.
	WebhookValidateDNS bool
	// WebhookHostResolver overrides the LookupIPAddr call for tests.
	// nil = use net.DefaultResolver.
	WebhookHostResolver func(context.Context, string) ([]netip.Addr, error)
	// WebhookAllowedDomains is an optional escape-hatch list of webhook
	// hostnames that bypass the WebhookValidateDNS private-IP check.
	// Each entry is matched against the parsed URL's host (lowercased)
	// either by exact equality (`webhook.example.com`) or by suffix
	// when the entry begins with a dot (`.example.com` matches
	// `webhook.example.com` and `api.example.com` but NOT
	// `attacker.example.com.evil.com`). Empty list = no bypass; the
	// DNS check applies to every host.
	WebhookAllowedDomains []string
	// EnableRawTools allows the raw API fallback tools to run at all.
	EnableRawTools bool
	// EnableRawGet allows the raw API fallback to use GET. It is separate
	// from EnableRawWrites because workspace reads can expose sensitive state.
	EnableRawGet bool
	// EnableRawWrites allows the raw API fallback to use mutating HTTP
	// methods. The raw-tools gate must also be enabled.
	EnableRawWrites bool
	// RawWriteDocumentedOnly restricts raw mutating methods to routes present
	// in the generated OpenAPI allowlist. Raw GET remains unaffected.
	RawWriteDocumentedOnly bool
	// Toolset selects the startup registry surface. Empty/all exposes the
	// full owner workbench; smaller values are filtered by RegistryForToolset.
	Toolset string
	// ToolRateLimitDisabled is true only when the operator explicitly sets
	// CLOCKIFY_TOOL_RATE_LIMIT_PER_MINUTE=0.
	ToolRateLimitDisabled bool
	// ToolRateLimits reports the active per-risk invocation rate buckets.
	ToolRateLimits map[string]int
	// AuditLogPath is the optional local JSONL path used by the MCP runtime.
	AuditLogPath string
	// ConfirmationMode reports the central high-risk confirmation posture.
	ConfirmationMode string
	// Notifier delivers server→client notifications (progress, resource updates,
	// etc.) emitted by tool handlers. nil = drop silently.
	Notifier mcp.Notifier
	// EmitResourceUpdate publishes notifications/resources/updated for a URI
	// with an optional delta envelope. Wired to Server.NotifyResourceUpdated
	// so the subscription gate lives in the protocol core rather than in
	// every mutation handler. nil = drop silently.
	EmitResourceUpdate func(uri string, delta mcp.ResourceUpdateDelta)
	// EmitResourceListChanged publishes notifications/resources/list_changed
	// when the set of available resources changes (e.g. a new demo-run
	// resource appears). nil disables the notification.
	EmitResourceListChanged func()
	// SubscriptionGate reports whether any client is currently subscribed
	// to a URI. When wired (Server.HasResourceSubscription),
	// emitResourceUpdate short-circuits before the ReadResource round-trip
	// so unsubscribed mutations don't pay for a redundant fetch.
	SubscriptionGate func(uri string) bool
	// EntryFinancialReports forces entry financial enrichment to call the
	// reports host even when the client is pointed at a non-canonical base URL.
	// Production Clockify calls auto-enable this path; tests and local proxies
	// opt in explicitly so unrelated fake handlers do not receive surprise
	// reports-api requests.
	EntryFinancialReports bool
	mu                    sync.RWMutex
	cachedUser            *clockify.User
	cachedWSID            string
	resolveMu             sync.RWMutex
	resolveCache          map[resolveKey]resolveEntry
	// resourceCache stores the last-emitted state per subscribed URI so the
	// delta-sync emit helper can diff before publishing. See W3-03c and ADR 013.
	resourceCache      *resourceStateCache
	demoResources      map[string]demoResourceState
	registryOnce       sync.Once
	registry           []mcp.ToolDescriptor
	registryErr        error
	toolsResourceOnce  sync.Once
	toolsResourceCache map[string]any
}

// EmitProgress publishes a notifications/progress if a progressToken was
// supplied with the current tools/call and the Service has a Notifier wired.
// No-op otherwise. total < 0 signals an indeterminate total.
func (s *Service) EmitProgress(ctx context.Context, progress, total float64, message string) {
	if s == nil || s.Notifier == nil {
		return
	}
	// Stop emitting once the call is cancelled or has timed out.
	if ctx.Err() != nil {
		return
	}
	token, ok := mcp.ProgressTokenFromContext(ctx)
	if !ok {
		return
	}
	// When the notifier can gate progress (the MCP server does), drop
	// non-increasing or flooding notifications instead of forwarding them.
	if gate, ok := s.Notifier.(mcp.ProgressGate); ok {
		if !gate.AllowProgress(token, progress) {
			return
		}
	}
	params := map[string]any{
		"progressToken": token,
		"progress":      progress,
	}
	if total >= 0 {
		params["total"] = total
	}
	if message != "" {
		params["message"] = message
	}
	if err := s.Notifier.Notify("notifications/progress", params); err != nil {
		slog.Warn("progress_notify_failed", "err", err)
	}
}

// ResultEnvelope is the canonical shape every tool handler returns.
// OK is the boolean success flag, Action mirrors the tool name for
// client-side dispatch, Data carries the typed payload (struct or map)
// and Meta is reserved for cross-cutting metadata (pagination cursors,
// fingerprint hashes, etc.). Wire-locked by the per-tool outputSchemas
// in output_schemas.go; mutate this struct and every schemaFor[T]
// surface has to be reviewed for drift.
type ResultEnvelope struct {
	OK     bool           `json:"ok"`
	Action string         `json:"action"`
	Data   any            `json:"data,omitempty"`
	Meta   map[string]any `json:"meta,omitempty"`
}

// WeeklySummaryData is the structured payload for clockify_weekly_
// summary: a date range, total counts, per-day and per-project rollups,
// suggested follow-up actions for the agent, and (optionally) the raw
// entries so callers can drill into specific records without a second
// round-trip.
type WeeklySummaryData struct {
	Range            DateRange            `json:"range"`
	Totals           SummaryTotals        `json:"totals"`
	ByDay            []DaySummary         `json:"byDay"`
	ByProject        []ProjectSummary     `json:"byProject"`
	SuggestedActions []ToolSuggestion     `json:"suggestedActions"`
	Entries          []clockify.TimeEntry `json:"entries,omitempty"`
	UnassignedKey    string               `json:"unassignedKey,omitempty"`
}

// QuickReportData powers clockify_reports_summary: a single-glance
// snapshot with totals, the top project, any running entries, and a
// short entry sample so an agent can answer "what did I just do?"
// without a full detailed report round-trip.
type QuickReportData struct {
	Range               DateRange        `json:"range"`
	Totals              SummaryTotals    `json:"totals"`
	TopProject          *ProjectSummary  `json:"topProject,omitempty"`
	RunningEntries      []EntryView      `json:"runningEntries,omitempty"`
	EntriesSample       []EntryView      `json:"entriesSample,omitempty"`
	ProjectsRepresented int              `json:"projectsRepresented"`
	SuggestedActions    []ToolSuggestion `json:"suggestedActions"`
}

// FindAndUpdateEntryData is the structured payload for
// clockify_find_and_update_entry. Entry is the matched time entry;
// MatchedBy explains which finder predicate identified it; UpdatedFields
// lists the fields that actually changed. When dry_run:true is set,
// Current + Proposed + DryRun carry the preview-only diff so a
// downstream confirmation step can stage the mutation before applying.
type FindAndUpdateEntryData struct {
	Entry          EntryView               `json:"entry"`
	MatchedBy      map[string]any          `json:"matchedBy"`
	UpdatedFields  []string                `json:"updatedFields"`
	MatchedEntryID string                  `json:"matched_entry_id,omitempty"`
	Current        *TimeEntryUpdatePreview `json:"current,omitempty"`
	Proposed       map[string]any          `json:"proposed_changes,omitempty"`
	DryRun         bool                    `json:"dry_run,omitempty"`
	Note           string                  `json:"note,omitempty"`
	Validation     *ValidationView         `json:"validation,omitempty"`
}

// TimeEntryUpdatePreview is the projected "current" shape of a time
// entry shown alongside a proposed-change diff during a dry-run update.
type TimeEntryUpdatePreview struct {
	Description     string   `json:"description"`
	ProjectID       string   `json:"project_id,omitempty"`
	TaskID          string   `json:"task_id,omitempty"`
	TagIDs          []string `json:"tag_ids,omitempty"`
	Start           string   `json:"start,omitempty"`
	End             string   `json:"end,omitempty"`
	Billable        bool     `json:"billable"`
	BillableState   string   `json:"billable_state,omitempty"`
	BillablePresent bool     `json:"billable_present,omitempty"`
}

// DateRange is the inclusive [Start, End] window used by every summary
// / report payload to describe the period the rollup spans.
type DateRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// SummaryTotals aggregates entry counts and tracked time across the
// containing DateRange. RunningEntries is the subset of Entries that
// are still in progress (no End yet); TotalSeconds is the canonical
// duration, TotalHours is the convenience float derived from it.
type SummaryTotals struct {
	Entries        int     `json:"entries"`
	RunningEntries int     `json:"runningEntries"`
	TotalSeconds   int64   `json:"totalSeconds"`
	TotalHours     float64 `json:"totalHours"`
}

// ProjectSummary is the per-project rollup row used by WeeklySummaryData
// and QuickReportData. ProjectID is omitempty so entries that did not
// resolve to a project (e.g. tracked with no project tag) still appear
// in the rollup keyed only by ProjectName.
type ProjectSummary struct {
	ProjectID    string  `json:"projectId,omitempty"`
	ProjectName  string  `json:"projectName"`
	ClientID     string  `json:"clientId,omitempty"`
	ClientName   string  `json:"clientName,omitempty"`
	Entries      int     `json:"entries"`
	TotalSeconds int64   `json:"totalSeconds"`
	TotalHours   float64 `json:"totalHours"`
}

// DaySummary is the per-day rollup row used by WeeklySummaryData.
// Date is RFC 3339 YYYY-MM-DD in the configured timezone.
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
	TaskID              string
	TagIDs              []string
	Start               string
	End                 string
	Billable            *bool
	DryRun              bool
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func New(client *clockify.Client, workspaceID string) *Service {
	return &Service{
		Client:             client,
		WorkspaceID:        workspaceID,
		WebhookValidateDNS: true,
		resourceCache:      newResourceStateCache(1024),
		demoResources:      map[string]demoResourceState{},
	}
}

func stringArg(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func boolArg(args map[string]any, key string) bool {
	v, _ := args[key].(bool)
	return v
}

// numberFromAny coerces any JSON-decoded numeric value to float64. It is the
// single numeric-coercion point for tool arguments: the MCP server decodes
// JSON-RPC with json.Decoder.UseNumber(), so numbers arrive over stdio as
// json.Number, never float64 — every numeric extractor routes through here so
// that wire values and direct-constructed test values behave identically.
func numberFromAny(v any) (float64, bool) {
	switch x := v.(type) {
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int8:
		return float64(x), true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint8:
		return float64(x), true
	case uint16:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	default:
		return 0, false
	}
}

// jsonTypeName names the JSON wire type of a decoded value ("number",
// "string", ...) so validation errors stay agent-readable instead of leaking
// a Go type like "json.Number" or "[]interface {}".
func jsonTypeName(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case json.Number, float64, float32,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64:
		return "number"
	case map[string]any:
		return "object"
	case []any, []string:
		return "array"
	default:
		return "value"
	}
}

func intArg(args map[string]any, key string, fallback int) int {
	v, ok := args[key]
	if !ok {
		return fallback
	}
	f, ok := numberFromAny(v)
	if !ok || math.IsNaN(f) || math.IsInf(f, 0) || f < math.MinInt || f > math.MaxInt {
		return fallback
	}
	return int(f)
}

func ok(action string, data any, meta map[string]any) ResultEnvelope {
	return ResultEnvelope{OK: true, Action: action, Data: sanitizeResultValue(data), Meta: sanitizeResultMeta(meta)}
}

func sanitizeResultMeta(meta map[string]any) map[string]any {
	if len(meta) == 0 {
		return meta
	}
	sanitized, _ := sanitizeResultValue(meta).(map[string]any)
	return sanitized
}

// emptyListMeta annotates a list-tool meta map with a nextAction hint when
// the result was empty, so the model is told what to try next rather than
// receiving a bare [] (Axiom 11). It reads the integer "count" the list
// handlers already record; a non-zero or absent count leaves meta untouched.
// createTool is the tool that creates the missing entity, or "" when the
// domain has no create companion.
func emptyListMeta(meta map[string]any, createTool string) map[string]any {
	if count, ok := meta["count"].(int); !ok || count != 0 {
		return meta
	}
	if createTool != "" {
		meta["nextAction"] = "No results matched the request. Use " + createTool + " to create one, or retry with broader filters or a wider date range."
	} else {
		meta["nextAction"] = "No results matched the request. Retry with broader filters or a wider date range."
	}
	return meta
}

func sanitizeResultValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			if sensitiveResultKey(key) {
				continue
			}
			out[key] = sanitizeResultValue(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = sanitizeResultValue(item)
		}
		return out
	case []map[string]any:
		out := make([]map[string]any, len(v))
		for i, item := range v {
			out[i], _ = sanitizeResultValue(item).(map[string]any)
		}
		return out
	case string:
		if strings.Contains(strings.ToLower(v), "bearer ") {
			return "[redacted]"
		}
		return v
	default:
		return value
	}
}

func sensitiveResultKey(key string) bool {
	normalized := strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(key))
	switch normalized {
	case "accesstoken", "apikey", "authtoken", "bearer", "clientsecret", "cookie", "cookies", "credential", "credentials", "authorization", "idtoken", "password", "refreshtoken", "secret", "token", "xaddontoken", "xapikey":
		return true
	default:
		return false
	}
}

func hours(seconds int64) float64 {
	return float64(seconds) / 3600.0
}
