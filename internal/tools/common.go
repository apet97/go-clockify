package tools

import (
	"context"
	"fmt"
	"maps"
	"math"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/apet97/go-clockify/internal/bootstrap"
	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/dedupe"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/timeparse"
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
	DefaultTimezone *time.Location        // from CLOCKIFY_TIMEZONE; nil falls back to time.Now().Location() for flexible date/time inputs.
	DedupeConfig    *dedupe.Config        // optional, set during wiring
	PolicyDescribe  func() map[string]any // set during wiring; returns policy description
	Bootstrap       *bootstrap.Config     // set during wiring; describes current bootstrap visibility
	ActivateGroup   func(context.Context, string) (ActivationResult, error)
	ActivateTool    func(context.Context, string) (ActivationResult, error)
	DeactivateGroup func(context.Context, string) (DeactivationResult, error)
	GroupActivation func(string) (allowed bool, reason string)
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
	// DNS check applies to every host. Operators use this to admit
	// known-trusted hostnames in split-horizon DNS environments where
	// the hostname legitimately resolves to a private IP only on the
	// control-plane network. See `docs/runbooks/webhook-dns-validation.md`
	// §4b for the use case. Production wiring (env var + Config field)
	// follows in a later commit; tests inject this field directly.
	WebhookAllowedDomains []string
	// Notifier delivers server→client notifications (progress, resource updates,
	// etc.) emitted by tool handlers. nil = drop silently.
	Notifier mcp.Notifier
	// EmitResourceUpdate publishes notifications/resources/updated for a URI
	// with an optional delta envelope. Wired from runtime.go to
	// Server.NotifyResourceUpdated so the subscription gate lives in the
	// protocol core rather than in every mutation handler. nil = drop silently
	// (tests without a Server wired).
	EmitResourceUpdate func(uri string, delta mcp.ResourceUpdateDelta)
	// SubscriptionGate reports whether any client is currently subscribed
	// to a URI. When wired (runtime.go sets it to
	// Server.HasResourceSubscription), emitResourceUpdate short-circuits
	// before the ReadResource round-trip so unsubscribed mutations don't
	// pay for a redundant fetch. nil = gate disabled; every emit pays for
	// the re-read (W3-era behaviour, preserved for tests).
	SubscriptionGate func(uri string) bool
	// ReportMaxEntries is the hard cap on the number of time entries a report
	// tool will aggregate. 0 disables the cap. Wired from CLOCKIFY_REPORT_MAX_ENTRIES.
	ReportMaxEntries int
	// DocumentedAPIWrites enables generic probe_lab_api write/delete calls.
	// Runtime defaults this off for hosted profiles and on for local profiles.
	DocumentedAPIWrites bool
	// DeltaFormat selects the diff algorithm for resource notifications.
	// "merge" (default) uses RFC 7396 merge patch; "jsonpatch" uses RFC 6902.
	DeltaFormat  string
	mu           sync.RWMutex
	cachedUser   *clockify.User
	cachedWSID   string
	resolveMu    sync.RWMutex
	resolveCache map[resolveKey]resolveEntry
	// resourceCache stores the last-emitted state per subscribed URI so the
	// delta-sync emit helper can diff before publishing. See W3-03c and ADR 013.
	resourceCache *resourceStateCache
	tier2CacheMu  sync.Mutex
	tier2Cache    map[string][]mcp.ToolDescriptor
}

// EmitProgress publishes a notifications/progress if a progressToken was
// supplied with the current tools/call and the Service has a Notifier wired.
// No-op otherwise. total < 0 signals an indeterminate total.
func (s *Service) EmitProgress(ctx context.Context, progress, total float64, message string) {
	if s == nil || s.Notifier == nil {
		return
	}
	token, ok := mcp.ProgressTokenFromContext(ctx)
	if !ok {
		return
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
	_ = s.Notifier.Notify("notifications/progress", params)
}

// ActivationResult is the payload returned by a Service activator
// (ActivateGroup / ActivateTool) describing which tools came online.
// The shape feeds activationPayload + activationMessage so the MCP
// response envelope carries a self-describing list of newly-visible
// tools rather than a bare "ok".
type ActivationResult struct {
	Kind  string `json:"kind"`
	Name  string `json:"name"`
	Group string `json:"group,omitempty"`
	// ToolCount is len(ActivatedTools); kept as a separate field for
	// backwards compatibility with clients that read it directly.
	ToolCount int `json:"toolCount"`
	// ActivatedTools enumerates every tool name brought online by this
	// activation. For Tier-2 tool-name activations, this is the full
	// containing group — the LLM sees exactly what other capabilities
	// it just gained alongside the requested one. Empty for Tier-1
	// single-tool activation by design.
	ActivatedTools []string `json:"activatedTools,omitempty"`
	// TotalVisibleTools is the post-activation tools/list count after
	// bootstrap and policy filtering. Zero means the activator could not
	// compute the session total and activationPayload omits it.
	TotalVisibleTools int `json:"totalVisibleTools,omitempty"`
	// VisibleActivatedTools is ActivatedTools filtered through the same
	// post-activation tools/list visibility gate. nil preserves legacy
	// activation callbacks; non-nil means activationPayload should expose
	// this filtered set as activated_tools.
	VisibleActivatedTools []string `json:"visibleActivatedTools,omitempty"`
	// ActivatedToolsHiddenByBootstrap names activated tools that remained
	// hidden because the active bootstrap config did not expose them.
	ActivatedToolsHiddenByBootstrap []string `json:"activatedToolsHiddenByBootstrap,omitempty"`
	// ActivatedToolsBlockedByPolicy names activated tools that remained
	// hidden because policy enforcement filtered them from tools/list.
	ActivatedToolsBlockedByPolicy []string `json:"activatedToolsBlockedByPolicy,omitempty"`
}

// DeactivationResult mirrors ActivationResult for the deactivation
// path: the Service.DeactivateGroup callback returns one of these to
// describe which tools were removed from the visible tools/list.
type DeactivationResult struct {
	Kind              string   `json:"kind"`
	Name              string   `json:"name"`
	Group             string   `json:"group,omitempty"`
	ToolCount         int      `json:"toolCount"`
	DeactivatedTools  []string `json:"deactivatedTools,omitempty"`
	TotalVisibleTools int      `json:"totalVisibleTools,omitempty"`
}

// ResultEnvelope is the canonical shape every Tier 1 / Tier 2 tool
// handler returns. OK is the boolean success flag, Action mirrors the
// tool name for client-side dispatch, Data carries the typed payload
// (struct or map) and Meta is reserved for cross-cutting metadata
// (pagination cursors, fingerprint hashes, etc.). Wire-locked by the
// per-tool outputSchemas in output_schemas.go; mutate this struct and
// every schemaFor[T] surface has to be reviewed for drift.
type ResultEnvelope struct {
	OK     bool           `json:"ok"`
	Action string         `json:"action"`
	Data   any            `json:"data,omitempty"`
	Meta   map[string]any `json:"meta,omitempty"`
}

// WorkspaceContext is the lightweight envelope returned by
// clockify_get_workspace when the caller only needs the resolved
// workspace identifier (full workspace details live behind
// clockify_list_workspaces).
type WorkspaceContext struct {
	WorkspaceID string `json:"workspaceId"`
}

// IdentityData pairs the upstream Clockify user with the resolved
// workspace so a single clockify_whoami response carries everything
// an agent needs to ground subsequent tool calls.
type IdentityData struct {
	User        clockify.User `json:"user"`
	WorkspaceID string        `json:"workspaceId"`
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

// SummaryData is the structured payload for clockify_summary_report
// and clockify_detailed_report: per-project rollups plus optional raw
// entries. The WeeklySummaryData variant adds a per-day axis on top.
type SummaryData struct {
	Range            DateRange            `json:"range"`
	Totals           SummaryTotals        `json:"totals"`
	ByProject        []ProjectSummary     `json:"byProject"`
	SuggestedActions []ToolSuggestion     `json:"suggestedActions"`
	Entries          []clockify.TimeEntry `json:"entries,omitempty"`
}

// QuickReportData powers clockify_quick_report: a single-glance
// snapshot with totals, the top project, any running entries, and a
// short entry sample so an agent can answer "what did I just do?"
// without a full detailed report round-trip.
type QuickReportData struct {
	Range               DateRange            `json:"range"`
	Totals              SummaryTotals        `json:"totals"`
	TopProject          *ProjectSummary      `json:"topProject,omitempty"`
	RunningEntries      []clockify.TimeEntry `json:"runningEntries,omitempty"`
	EntriesSample       []clockify.TimeEntry `json:"entriesSample,omitempty"`
	ProjectsRepresented int                  `json:"projectsRepresented"`
	SuggestedActions    []ToolSuggestion     `json:"suggestedActions"`
}

// LogTimeData is the structured payload for clockify_log_time. Entry
// is the created TimeEntry; ResolvedProject names the project the
// agent supplied (by name or ID) so the caller can confirm name
// resolution succeeded.
type LogTimeData struct {
	Entry           clockify.TimeEntry `json:"entry"`
	ResolvedProject string             `json:"resolvedProject,omitempty"`
}

// FindAndUpdateEntryData is the structured payload for
// clockify_find_and_update_entry. Entry is the matched time entry;
// MatchedBy explains which finder predicate identified it; UpdatedFields
// lists the fields that actually changed. When dry_run:true is set,
// Current + Proposed + DryRun carry the preview-only diff so a
// downstream confirmation step can stage the mutation before applying.
type FindAndUpdateEntryData struct {
	Entry          clockify.TimeEntry      `json:"entry"`
	MatchedBy      map[string]any          `json:"matchedBy"`
	UpdatedFields  []string                `json:"updatedFields"`
	MatchedEntryID string                  `json:"matched_entry_id,omitempty"`
	Current        *TimeEntryUpdatePreview `json:"current,omitempty"`
	Proposed       map[string]any          `json:"proposed_changes,omitempty"`
	DryRun         bool                    `json:"dry_run,omitempty"`
	Note           string                  `json:"note,omitempty"`
}

// TimeEntryUpdatePreview is the projected "current" shape of a time
// entry shown alongside a proposed-change diff during a dry-run update.
type TimeEntryUpdatePreview struct {
	Description string `json:"description"`
	ProjectID   string `json:"project_id,omitempty"`
	Start       string `json:"start,omitempty"`
	End         string `json:"end,omitempty"`
	Billable    bool   `json:"billable"`
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

// ProjectSummary is the per-project rollup row used by SummaryData and
// WeeklySummaryData. ProjectID is omitempty so entries that did not
// resolve to a project (e.g. tracked with no project tag) still appear
// in the rollup keyed only by ProjectName.
type ProjectSummary struct {
	ProjectID    string  `json:"projectId,omitempty"`
	ProjectName  string  `json:"projectName"`
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
	Start               string
	End                 string
	Billable            *bool
	DryRun              bool
}

func New(client *clockify.Client, workspaceID string) *Service {
	return &Service{
		Client:              client,
		WorkspaceID:         workspaceID,
		DocumentedAPIWrites: true,
		resourceCache:       newResourceStateCache(1024),
	}
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

// newTool is the shared core of the four risk-class builders below.
// It overlays the supplied hint values on the baseAnnotations(name)
// scaffold and returns the assembled mcp.Tool. Centralising the
// overlay keeps the readOnly/destructive/idempotent hint matrix
// consistent — when a fifth risk class is added it becomes a one-
// line addition here, not another near-duplicate top-level helper.
func newTool(name, desc string, schema map[string]any, readOnly, destructive, idempotent bool) mcp.Tool {
	ann := baseAnnotations(name)
	ann["readOnlyHint"] = readOnly
	ann["destructiveHint"] = destructive
	ann["idempotentHint"] = idempotent
	return mcp.Tool{Name: name, Description: desc, InputSchema: schema, Annotations: ann}
}

func toolRO(name, desc string, schema map[string]any) mcp.Tool {
	return newTool(name, desc, schema, true, false, true)
}

// toolRW marks a write tool as non-destructive and non-idempotent.
// Absent this explicit declaration, MCP spec-strict clients assume
// destructive for write tools and may require extra confirmation
// for every call.
func toolRW(name, desc string, schema map[string]any) mcp.Tool {
	return newTool(name, desc, schema, false, false, false)
}

// toolRWIdem marks a write tool as idempotent. Use for PUT/PATCH-style updates
// and tools whose handlers produce the same end state on repeated calls
// (e.g. clockify_stop_timer when no timer is running becomes a no-op).
func toolRWIdem(name, desc string, schema map[string]any) mcp.Tool {
	return newTool(name, desc, schema, false, false, true)
}

func toolDestructive(name, desc string, schema map[string]any) mcp.Tool {
	return newTool(name, desc, schema, false, true, false)
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
		if in[i].Tool.InputSchema != nil {
			tightenInputSchema(in[i].Tool.InputSchema)
		}
		applyRiskMetadata(&in[i])
		applyAgentToolMetadata(&in[i])
	}
	return in
}

var agentToolMetadata = map[string]map[string]any{
	"clockify_log_time": {
		"bestToolFor": []string{"log finished past work", "create a complete time entry"},
		"preferOver":  []string{"clockify_add_entry"},
	},
	"clockify_timesheet_review": {
		"bestToolFor": []string{"find gaps and overlaps", "plan timesheet cleanup"},
	},
	"clockify_timesheet_fill_gap": {
		"bestToolFor": []string{"fill a reviewed timesheet gap"},
		"preferAfter": []string{"clockify_timesheet_review"},
	},
	"clockify_list_tools": {
		"bestToolFor": []string{"discover available tools", "preflight Tier-2 activation"},
		"preferOver":  []string{"clockify_search_tools"},
	},
	"clockify_resolve_name": {
		"bestToolFor": []string{"resolve human names to Clockify IDs"},
		"preferOver":  []string{"clockify_resolve_debug"},
	},
	"clockify_resolve_debug": {
		"compatibilityShim": true,
		"primaryTool":       "clockify_resolve_name",
	},
	"clockify_search_tools": {
		"compatibilityShim": true,
		"primaryTool":       "clockify_list_tools",
	},
	"clockify_set_project_memberships": {
		"compatibilityShim": true,
		"primaryTool":       "clockify_update_project_memberships",
	},
}

func applyAgentToolMetadata(d *mcp.ToolDescriptor) {
	meta, ok := agentToolMetadata[d.Tool.Name]
	if !ok {
		return
	}
	if d.Tool.Annotations == nil {
		d.Tool.Annotations = map[string]any{}
	}
	for key, value := range meta {
		if _, exists := d.Tool.Annotations[key]; !exists {
			d.Tool.Annotations[key] = value
		}
	}
}

// applyRiskMetadata populates RiskClass and AuditKeys for a descriptor.
// The default risk class is derived from the three MCP boolean hints; per-tool
// overrides in riskOverrides win because billing/admin/permission_change
// distinctions cannot be expressed as booleans.
func applyRiskMetadata(d *mcp.ToolDescriptor) {
	if d.RiskClass == 0 {
		switch {
		case d.DestructiveHint:
			d.RiskClass = mcp.RiskDestructive
		case d.ReadOnlyHint:
			d.RiskClass = mcp.RiskRead
		default:
			d.RiskClass = mcp.RiskWrite
		}
	}
	if override, ok := riskOverrides[d.Tool.Name]; ok {
		if override.class != 0 {
			d.RiskClass = override.class
		}
		if len(override.auditKeys) > 0 && len(d.AuditKeys) == 0 {
			d.AuditKeys = append([]string(nil), override.auditKeys...)
		}
	}
	if d.Tool.Annotations == nil {
		d.Tool.Annotations = map[string]any{}
	}
	if names := riskClassAnnotationNames(d.RiskClass); len(names) > 0 {
		d.Tool.Annotations["riskClass"] = names
	}
	d.Tool.Annotations["dryRun"] = schemaHasDryRun(d.Tool.InputSchema)

	// Confirmation-token discoverability surface (ADR 0018 Q4):
	//   - annotations.requiresConfirmationToken signals to MCP clients
	//     that the tool's execution path requires a server-issued
	//     token obtained via dry_run:true first.
	//   - annotations.confirmationRiskClass repeats the lower-case
	//     risk-class names so clients can render which class
	//     triggered the gate without re-deriving from the bitmask.
	//   - InputSchema.properties.confirmation_token is added as an
	//     optional string so spec-strict clients can pass the token
	//     back without tripping additionalProperties:false. Adding
	//     to properties (not required) means the schema gate accepts
	//     both the dry-run preview call (no token) and the execution
	//     call (token present).
	// Read-only tools skip the schema injection because they can never
	// be high-risk and never execute side effects worth gating.
	if d.RiskClass.IsHighRisk() {
		d.Tool.Annotations["requiresConfirmationToken"] = true
		d.Tool.Annotations["confirmationRiskClass"] = riskClassAnnotationNames(d.RiskClass)
		if !d.ReadOnlyHint {
			ensureConfirmationTokenSchemaProperty(d.Tool.InputSchema)
		}
	}
}

// ensureConfirmationTokenSchemaProperty adds an optional
// confirmation_token string property to the tool's InputSchema so
// spec-strict clients can echo a minted token back through the
// execution call without tripping additionalProperties:false. The
// helper is idempotent — a descriptor that already declares the
// property keeps the caller's metadata.
func ensureConfirmationTokenSchemaProperty(schema map[string]any) {
	if schema == nil {
		return
	}
	if typ, _ := schema["type"].(string); typ != "" && typ != "object" {
		return
	}
	props, _ := schema["properties"].(map[string]any)
	if props == nil {
		props = map[string]any{}
		schema["properties"] = props
	}
	if _, exists := props["confirmation_token"]; exists {
		return
	}
	props["confirmation_token"] = map[string]any{
		"type":        "string",
		"description": "Confirmation token returned by a prior dry_run:true call. Required when executing high-risk tools (see annotations.requiresConfirmationToken).",
	}
}

func riskClassAnnotationNames(rc mcp.RiskClass) []string {
	if rc == 0 {
		return nil
	}
	type entry struct {
		bit  mcp.RiskClass
		name string
	}
	all := []entry{
		{mcp.RiskRead, "read"},
		{mcp.RiskWrite, "write"},
		{mcp.RiskSensitiveRead, "sensitive_read"},
		{mcp.RiskBilling, "billing"},
		{mcp.RiskAdmin, "admin"},
		{mcp.RiskPermissionChange, "permission_change"},
		{mcp.RiskExternalSideEffect, "external_side_effect"},
		{mcp.RiskDestructive, "destructive"},
	}
	out := make([]string, 0, 2)
	for _, e := range all {
		if rc.Has(e.bit) {
			out = append(out, e.name)
		}
	}
	return out
}

func schemaHasDryRun(schema map[string]any) bool {
	if schema == nil {
		return false
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return false
	}
	_, ok = props["dry_run"]
	return ok
}

func dryrunPreviewPayload(tool string, payload map[string]any) map[string]any {
	return map[string]any{
		"dry_run": true,
		"tool":    tool,
		"payload": maps.Clone(payload),
		"note":    "No changes were made.",
	}
}

// tightenInputSchema mutates a JSON schema tree in place to meet the MCP
// spec B2 requirements for Tier 1 + Tier 2 tools:
//   - every object schema gets `additionalProperties: false` unless explicitly set
//   - `page` and `page_size` integer properties gain `minimum`/`maximum` bounds
//   - string properties whose description mentions RFC3339 gain
//     `format: "date-time"`, UNLESS the description also documents a
//     flexible parser (e.g. "natural language" or "YYYY-MM-DD"). The
//     validator at internal/jsonschema enforces format: date-time via
//     strict time.Parse(time.RFC3339, ...), so adding the format to a
//     field whose handler accepts wider input would reject valid calls
//     before the handler ever runs.
//   - `color` properties whose description mentions Hex gain the 6-hex pattern
//
// The walker handles nested objects, arrays (via `items`), and anyOf
// subschemas. It never
// overwrites an explicit value — callers can opt out of any single rule
// by setting it themselves.
func tightenInputSchema(schema map[string]any) {
	if schema == nil {
		return
	}
	if typ, _ := schema["type"].(string); typ == "object" {
		if _, set := schema["additionalProperties"]; !set {
			schema["additionalProperties"] = false
		}
		if props, ok := schema["properties"].(map[string]any); ok {
			for name, raw := range props {
				prop, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				applyPropertyConstraints(name, prop)
				tightenInputSchema(prop)
			}
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		tightenInputSchema(items)
	}
	if options, ok := schema["anyOf"].([]any); ok {
		for _, option := range options {
			if sub, ok := option.(map[string]any); ok {
				tightenInputSchema(sub)
			}
		}
	}
}

// applyPropertyConstraints adds spec-driven constraints to a single
// property schema based on its name and description. Only untouched keys
// are added — explicit values stay as declared.
func applyPropertyConstraints(name string, prop map[string]any) {
	switch name {
	case "page":
		if _, set := prop["minimum"]; !set {
			prop["minimum"] = 1
		}
	case "page_size":
		if _, set := prop["minimum"]; !set {
			prop["minimum"] = 1
		}
		if _, set := prop["maximum"]; !set {
			prop["maximum"] = 200
		}
	case "color":
		if desc, _ := prop["description"].(string); strings.Contains(strings.ToLower(desc), "hex") {
			if _, set := prop["pattern"]; !set {
				prop["pattern"] = "^#[0-9a-fA-F]{6}$"
			}
		}
	}
	// Generic RFC3339 timestamp detection — any string property whose
	// description calls out an RFC3339 timestamp gains format: date-time,
	// unless the description ALSO documents a flexible parser (natural
	// language like "now"/"today" or a YYYY-MM-DD short date). The
	// jsonschema validator enforces format: date-time via strict
	// time.Parse(time.RFC3339, ...) before the handler runs, so adding
	// the format to a flexible-parsing field would reject valid input
	// like start="now" on clockify_add_entry. Handlers using
	// timeparse.ParseDatetime / parseFlexibleDateTime accept the wider
	// surface; the schema must not be tighter than the parser.
	if typ, _ := prop["type"].(string); typ == "string" {
		desc, _ := prop["description"].(string)
		if desc != "" && strings.Contains(desc, "RFC3339") && !descriptionAdvertisesFlexibleTime(desc) {
			if _, set := prop["format"]; !set {
				prop["format"] = "date-time"
			}
		}
	}
	// Generic maxLength bounds on common free-text fields. Centralised
	// here so every Tier 1 + Tier 2 descriptor inherits the same ceiling
	// without each handler hand-declaring it. Bounds chosen from observed
	// Clockify-API limits and RFC defaults; an explicit handler-side
	// maxLength always wins.
	//
	// Skipped on purpose:
	//   - project/client/tag lookup identifiers (Clockify accepts UUIDs);
	//   - the free-form `query` field on clockify_list_tools (multi-word
	//     search queries must stay flexible);
	//   - flexible-time string fields (handled separately above).
	if ceil, ok := freeTextMaxLength[name]; ok {
		if typ, _ := prop["type"].(string); typ == "string" {
			if _, set := prop["maxLength"]; !set {
				prop["maxLength"] = ceil
			}
		}
	}
}

// freeTextMaxLength is the central table of conservative ceilings on
// common free-text property names. The values must NEVER be relaxed in
// place — TestRegistryFreeTextFieldsHaveMaxLength enforces them, and a
// future reviewer who needs a higher ceiling on a specific tool should
// declare maxLength explicitly on that tool's descriptor (which is
// honoured here because applyPropertyConstraints never overwrites an
// existing key).
var freeTextMaxLength = map[string]int{
	"description":          2000, // entry/log descriptions etc.
	"description_contains": 2000, // filter form of description
	"exact_description":    2000, // exact-match variant
	"new_description":      2000, // update form
	"name":                 150,
	"note":                 500,
	"address":              256,
	"email":                254, // RFC 5321 max localpart+domain
	"url":                  2048,
	"webhook_url":          2048,
	"redirect_url":         2048,
}

// descriptionAdvertisesFlexibleTime reports whether a property's
// description tells callers they can pass non-RFC3339 input. Handlers
// that document such flexibility use timeparse.ParseDatetime or
// parseFlexibleDateTime; the jsonschema validator must skip its
// format: date-time enforcement for these fields so the schema gate
// does not reject valid input the handler would accept.
func descriptionAdvertisesFlexibleTime(desc string) bool {
	lower := strings.ToLower(desc)
	if strings.Contains(lower, "natural language") {
		return true
	}
	// Match the literal token "YYYY-MM-DD" (case-insensitive); handlers
	// like clockify_weekly_summary's week_start parse it via
	// parseFlexibleDateTime.
	if strings.Contains(lower, "yyyy-mm-dd") {
		return true
	}
	return false
}

// schemaString returns a JSON Schema string property with an optional
// description. Empty desc emits `{"type":"string"}` so the wire output
// matches the dozens of inline literals that omit description today.
func schemaString(desc string) map[string]any {
	out := map[string]any{"type": "string"}
	if desc != "" {
		out["description"] = desc
	}
	return out
}

// schemaEnum returns a JSON Schema string property constrained to the
// supplied enum values, with an optional description. Validators
// (jsonschema.Validate) reject inputs outside the enum at parse time,
// shifting the responsibility off the handler's switch statement and
// surfacing the contract to MCP clients via tools/list.
func schemaEnum(desc string, values ...string) map[string]any {
	out := schemaString(desc)
	out["enum"] = values
	return out
}

// schemaObject returns a JSON Schema object node with optional required
// fields and properties. Empty required/props slots are omitted from
// the output so the wire shape stays identical to the dozens of inline
// `map[string]any{"type":"object", …}` literals already across the
// registry.
func schemaObject(required []string, properties map[string]any) map[string]any {
	out := map[string]any{"type": "object"}
	if len(required) > 0 {
		out["required"] = required
	}
	if len(properties) > 0 {
		out["properties"] = properties
	}
	return out
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
		maps.Copy(props, extra)
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

// paginationFromArgs extracts page/page_size from a tool args map. Public list
// tools share a conservative 200-item cap because they cover Clockify endpoint
// families with different pagination ceilings; bulk workflow/report scans use
// dedicated paginated helpers instead of this generic user-facing knob.
func paginationFromArgs(args map[string]any) (page, pageSize int) {
	page = max(intArg(args, "page", 1), 1)
	pageSize = intArg(args, "page_size", 50)
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

func addPaginationMeta(meta map[string]any, args map[string]any, page, pageSize int) map[string]any {
	if meta == nil {
		meta = map[string]any{}
	}
	pagination := map[string]any{
		"page":              page,
		"page_size":         pageSize,
		"applied_page_size": pageSize,
	}
	if _, ok := args["page_size"]; ok {
		requestedPageSize := intArg(args, "page_size", 50)
		pagination["requested_page_size"] = requestedPageSize
		if requestedPageSize != pageSize {
			pagination["clamped"] = true
		}
	}
	meta["pagination"] = pagination
	return meta
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

func (s *Service) locationFromArgs(args map[string]any) (*time.Location, error) {
	return loadLocation(stringArg(args, "timezone"), s.DefaultTimezone)
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
	return parseRangeInLocation(args, time.UTC)
}

func parseRangeInLocation(args map[string]any, loc *time.Location) (time.Time, time.Time, error) {
	if loc == nil {
		loc = time.UTC
	}
	startRaw := stringArg(args, "start")
	endRaw := stringArg(args, "end")
	if startRaw == "" || endRaw == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("start and end are required")
	}
	start, err := timeparse.ParseDatetime(startRaw, loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start: %w", err)
	}
	end, err := timeparse.ParseDatetime(endRaw, loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end: %w", err)
	}
	if !end.After(start) && isBareDateString(endRaw) {
		end = end.AddDate(0, 0, 1)
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("end must be after start")
	}
	return start.UTC(), end.UTC(), nil
}

func isBareDateString(raw string) bool {
	raw = strings.TrimSpace(raw)
	if len(raw) != len("2006-01-02") {
		return false
	}
	_, err := time.Parse("2006-01-02", raw)
	return err == nil
}

func parseStartEndInLocation(args map[string]any, loc *time.Location) (time.Time, time.Time, error) {
	return parseRangeInLocation(args, loc)
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
