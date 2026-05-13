package mcp

import (
	"fmt"
	"strings"
	"sync"
)

// PromptArgument describes one substitution variable a prompt accepts.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PromptMessage is one turn in a prompt's canned message sequence. Role is
// typically "user" or "assistant"; content mirrors the MCP content-part shape.
type PromptMessage struct {
	Role    string            `json:"role"`
	Content PromptMessagePart `json:"content"`
}

// PromptMessagePart is a single content part inside a PromptMessage. Only
// text content is supported in this server; clients that want images or
// resource links can request a richer prompt via a follow-up tool call.
type PromptMessagePart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Prompt is a registered prompt template with canned messages whose bodies
// may contain `{{name}}` placeholders substituted at prompts/get time.
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
	Messages    []PromptMessage  `json:"messages"`
}

// promptRegistry stores the built-in Clockify prompts. Registration is
// done in init() and guarded by a mutex so tests can add fixture prompts
// without racing reads from the dispatch path.
type promptRegistry struct {
	mu      sync.RWMutex
	prompts map[string]Prompt
	order   []string
}

func newPromptRegistry() *promptRegistry {
	r := &promptRegistry{prompts: map[string]Prompt{}}
	for _, p := range builtinPrompts() {
		r.prompts[p.Name] = p
		r.order = append(r.order, p.Name)
	}
	return r
}

func (r *promptRegistry) list() []Prompt {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Prompt, 0, len(r.order))
	for _, name := range r.order {
		p := r.prompts[name]
		// Clone messages so mutation downstream cannot corrupt the registry.
		p.Messages = append([]PromptMessage(nil), p.Messages...)
		out = append(out, p)
	}
	return out
}

func (r *promptRegistry) get(name string) (Prompt, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.prompts[name]
	if !ok {
		return Prompt{}, false
	}
	p.Messages = append([]PromptMessage(nil), p.Messages...)
	return p, true
}

// applyArgs substitutes `{{name}}` placeholders inside every message body
// with the corresponding value from args. Unknown placeholders remain as-is
// so clients can see which variables were missing; required arguments that
// are absent return an error.
func applyArgs(p Prompt, args map[string]any) ([]PromptMessage, error) {
	for _, a := range p.Arguments {
		if !a.Required {
			continue
		}
		v, ok := args[a.Name]
		if !ok || v == nil || v == "" {
			return nil, fmt.Errorf("missing required argument %q", a.Name)
		}
	}
	out := make([]PromptMessage, len(p.Messages))
	for i, m := range p.Messages {
		out[i] = PromptMessage{Role: m.Role, Content: PromptMessagePart{Type: m.Content.Type, Text: substituteArgs(m.Content.Text, args)}}
	}
	return out, nil
}

func substituteArgs(text string, args map[string]any) string {
	for k, v := range args {
		needle := "{{" + k + "}}"
		if strings.Contains(text, needle) {
			text = strings.ReplaceAll(text, needle, fmt.Sprintf("%v", v))
		}
	}
	return text
}

func builtinPrompts() []Prompt {
	return []Prompt{
		{
			Name:        "log-week-from-calendar",
			Description: "Draft time entries for one week by walking a calendar reference and mapping each event to a Clockify project.",
			Arguments: []PromptArgument{
				{Name: "week_start", Description: "ISO date (YYYY-MM-DD) for the Monday of the week to log.", Required: true},
				{Name: "calendar_uri", Description: "Where the upstream calendar data lives (ICS URL, Google Calendar id, etc.).", Required: true},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "Walk the calendar at {{calendar_uri}} for the week starting {{week_start}}. For each finished event that should be tracked in Clockify, draft a `clockify_log_time` call with project/project_id when known, start/end from the calendar, and the event description. Ask me for clarification if the project name is ambiguous. Do not execute any write tool without my confirmation. If Clockify reports an overlap, inspect the affected entries and use `allow_overlap:true` only after explicit confirmation."}},
			},
		},
		{
			Name:        "weekly-review",
			Description: "Summarise the current user's Clockify week and flag anomalies (gaps, overtime, untagged entries).",
			Arguments: []PromptArgument{
				{Name: "week_start", Description: "ISO date (YYYY-MM-DD) for the Monday of the week to review.", Required: true},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "Use `clockify_timesheet_review` for week_start={{week_start}} first. Report total hours, issues, and suggestedActions from that structured review. If you need Reports API weekly totals, call `clockify_weekly_summary` with `week_start={{week_start}}` and `weekly_filter:{\"group\":\"PROJECT\",\"subgroup\":\"TIME\"}`."}},
			},
		},
		{
			Name:        "find-unbilled-hours",
			Description: "Find time entries in a date range that are not yet marked billable.",
			Arguments: []PromptArgument{
				{Name: "since", Description: "ISO date lower bound.", Required: true},
				{Name: "until", Description: "ISO date upper bound.", Required: true},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "List every billable-eligible time entry between {{since}} and {{until}} that has `billable=false`. Group by project and report total unbilled hours. Use `clockify_list_entries`."}},
			},
		},
		{
			Name:        "find-duplicate-entries",
			Description: "Scan recent time entries for probable duplicates (same project, overlapping time, similar description).",
			Arguments: []PromptArgument{
				{Name: "lookback_days", Description: "How many days of history to scan. Default 14.", Required: false},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "Use `clockify_timesheet_review` with explicit `start` and `end` covering the requested lookback window. If `lookback_days` is absent, use the last 14 days. Inspect overlap issues and relatedEntryIds, then describe each suspected duplicate pair. Do not delete anything."}},
			},
		},
		{
			Name:        "generate-timesheet-report",
			Description: "Produce a formatted timesheet for a given week in one of the supported export formats.",
			Arguments: []PromptArgument{
				{Name: "week_start", Description: "ISO date (YYYY-MM-DD) for the Monday of the week.", Required: true},
				{Name: "format", Description: "One of `pdf`, `csv`, `md`.", Required: true},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "Build a timesheet for week_start={{week_start}} in {{format}} format. Use `clockify_weekly_summary` with `weekly_filter:{\"group\":\"PROJECT\",\"subgroup\":\"TIME\"}` for Reports API totals, then use `clockify_timesheet_review` to flag gaps or overlaps before rendering every day of the week including zero-hour days."}},
			},
		},
		{
			Name:        "invoice-review-and-send",
			Description: "Review invoice readiness, draft invoice changes, and require dry-run/confirmation before external invoice actions.",
			Arguments: []PromptArgument{
				{Name: "client", Description: "Client name or ID to invoice.", Required: true},
				{Name: "since", Description: "ISO date lower bound.", Required: true},
				{Name: "until", Description: "ISO date upper bound.", Required: true},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "Review invoice readiness for client {{client}} from {{since}} through {{until}}. Use invoice and report tools to identify billable entries, draft invoice line items, and surface missing client/project metadata. Do not call `clockify_send_invoice` until a dry-run preview is shown and I explicitly confirm the external side effect."}},
			},
		},
		{
			Name:        "approval-cycle",
			Description: "Plan an approval workflow by listing approval requests and previewing approve/reject actions before execution.",
			Arguments: []PromptArgument{
				{Name: "period", Description: "Approval period or date range to inspect.", Required: true},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "Plan the approval cycle for {{period}}. Use `clockify_list_approval_requests` and `clockify_get_approval_request` first, summarize pending/blocked requests, and use dry_run:true for any `clockify_approve_timesheet`, `clockify_reject_timesheet`, or `clockify_withdraw_approval` proposal before execution."}},
			},
		},
		{
			Name:        "time-off-review",
			Description: "Review time-off policies, balances, and requests before drafting safe request/status changes.",
			Arguments: []PromptArgument{
				{Name: "user", Description: "User name or ID whose time-off state should be reviewed.", Required: true},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "Review time-off state for {{user}}. Resolve the user, inspect policies, balances, and existing requests, then explain any plan/role limits. Draft create/status/delete request tool calls only with dry_run:true first and require explicit confirmation before any write."}},
			},
		},
		{
			Name:        "scheduling-capacity-review",
			Description: "Review scheduling assignments and per-user capacity totals before proposing schedule changes.",
			Arguments: []PromptArgument{
				{Name: "user", Description: "User name or ID to inspect.", Required: true},
				{Name: "week_start", Description: "ISO date (YYYY-MM-DD) for the week to inspect.", Required: true},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "Review scheduling capacity for {{user}} in the week starting {{week_start}}. Prefer typed scheduling tools, including per-user capacity totals, and avoid raw documented API writes unless the operator has enabled them. Summarize overloads, gaps, and any proposed assignment writes with dry_run:true before execution."}},
			},
		},
		{
			Name:        "webhook-rollout-check",
			Description: "Plan a webhook rollout with DNS validation, dry-run payload review, and token-safe output handling.",
			Arguments: []PromptArgument{
				{Name: "url", Description: "Webhook delivery URL to validate.", Required: true},
				{Name: "event", Description: "Clockify webhook event to configure.", Required: true},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "Plan a webhook rollout for {{event}} to {{url}}. Use webhook list/get tools to avoid duplicates, verify that DNS validation will pass, mask any auth token, and preview create/update/test/delete operations with dry_run:true before execution. Treat `clockify_test_webhook` as an external side effect requiring confirmation."}},
			},
		},
	}
}

// promptGetParams is the decoded body of a prompts/get request.
type promptGetParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func (s *Server) handlePromptsList() (any, *RPCError) {
	return map[string]any{"prompts": s.prompts.list()}, nil
}

func (s *Server) handlePromptsGet(raw any) (any, *RPCError) {
	var params promptGetParams
	if err := decodeParams(raw, &params); err != nil || params.Name == "" {
		return nil, &RPCError{Code: -32602, Message: "invalid prompts/get params: missing name"}
	}
	p, ok := s.prompts.get(params.Name)
	if !ok {
		return nil, &RPCError{Code: -32602, Message: fmt.Sprintf("prompt not found: %s", params.Name)}
	}
	messages, err := applyArgs(p, params.Arguments)
	if err != nil {
		return nil, &RPCError{Code: -32602, Message: err.Error()}
	}
	return map[string]any{
		"description": p.Description,
		"messages":    messages,
	}, nil
}
