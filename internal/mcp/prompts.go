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

// promptRegistry stores the five built-in Clockify prompts. Registration is
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
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "Walk the calendar at {{calendar_uri}} for the week starting {{week_start}}. For each event that should be tracked in Clockify, draft a `clockify_log_time` call that resolves the project by name. Ask me for clarification if the project name is ambiguous. Do not execute any write tool without my confirmation."}},
			},
		},
		{
			Name:        "weekly-review",
			Description: "Summarise the current user's Clockify week and flag anomalies (gaps, overtime, untagged entries).",
			Arguments: []PromptArgument{
				{Name: "week_start", Description: "ISO date (YYYY-MM-DD) for the Monday of the week to review.", Required: true},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "Use `clockify_weekly_summary` for week_start={{week_start}}. Report: total hours logged, top 3 projects by hours, any day with more than 10 hours logged, any entry missing a project, and any gap of more than 2 working hours between entries on weekdays."}},
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
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "Pull the last {{lookback_days}} days of my time entries via `clockify_list_entries` and report any pair with overlapping start/end ranges on the same project. Describe each suspected duplicate pair — do not delete anything."}},
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
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "Build a timesheet for week_start={{week_start}} in {{format}} format. Use `clockify_weekly_summary` for the totals and render rows for every day of the week including zero-hour days."}},
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
