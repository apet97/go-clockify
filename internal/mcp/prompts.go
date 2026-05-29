package mcp

import (
	"fmt"
	"sort"
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

// promptRegistry stores the built-in one-user Clockify prompts. Registration is
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
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})
	for _, k := range keys {
		v := args[k]
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
			Name:        "demo-full-workspace-story",
			Description: "Seed a deterministic demo workspace story, review it, and report the IDs.",
			Arguments: []PromptArgument{
				{Name: "run_id", Description: "Stable demo run id. Default phase1.", Required: false},
				{Name: "prefix", Description: "Explicit demo object prefix.", Required: false},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "First call `clockify_demo_seed` to create or reuse the deterministic demo fixture. Use returned IDs in later calls, especially clientId, projectId, taskId, tagId, and entryId. If a paid feature is unavailable, report it and continue with the available workspace data. Summarize what changed, including IDs and changed.created, changed.updated, changed.deleted, or changed.reused."}},
			},
		},
		{
			Name:        "setup-client-project-task",
			Description: "Create or reuse a client/project/task/tag work package from names or IDs.",
			Arguments: []PromptArgument{
				{Name: "client", Description: "Client name or ID.", Required: false},
				{Name: "project", Description: "Project name or ID.", Required: false},
				{Name: "task", Description: "Task name or ID.", Required: false},
				{Name: "tag", Description: "Tag name.", Required: false},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "First call `clockify_create_work_package` with the client, project, task, and tag names or IDs you have. Use returned IDs in later calls instead of repeating names. If a paid feature is unavailable, report it and continue with the created or reused core work objects. Summarize what changed, including IDs and changed.created, changed.updated, changed.deleted, or changed.reused."}},
			},
		},
		{
			Name:        "log-week",
			Description: "Log a week of finished work with workflow tools.",
			Arguments: []PromptArgument{
				{Name: "week_start", Description: "Week start date, if known.", Required: false},
				{Name: "project", Description: "Project name or ID.", Required: false},
				{Name: "task", Description: "Task name or ID.", Required: false},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "First call `clockify_log_work` for the first finished entry, then repeat it for each remaining entry in the week. Use returned IDs from setup or earlier log calls whenever possible. If a paid feature is unavailable, report it and continue logging the entries that can be recorded. Summarize what changed, including IDs and changed.created, changed.updated, changed.deleted, or changed.reused."}},
			},
		},
		{
			Name:        "safe-daily-time-tracking",
			Description: "Review and adjust daily time tracking using only core time-entry tools.",
			Arguments: []PromptArgument{
				{Name: "date", Description: "Day to review, if known.", Required: false},
				{Name: "project", Description: "Project name or ID.", Required: false},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "First call `clockify_status`, then use only core time-tracking tools such as `clockify_review_day`, `clockify_log_work`, `clockify_start_work`, `clockify_stop_work`, and `clockify_fix_entry`. Do not use invoice, expense, admin, webhook, or raw fallback tools for this workflow. Use returned IDs from status, review, or prior entry calls whenever possible. If a paid feature is unavailable, report it and continue with daily time tracking only. Summarize what changed, including IDs and changed.created, changed.updated, changed.deleted, or changed.reused when any write follows."}},
			},
		},
		{
			Name:        "invoice-client",
			Description: "Create an invoice workflow for client work and degrade cleanly when invoicing is unavailable.",
			Arguments: []PromptArgument{
				{Name: "client", Description: "Client name or ID.", Required: false},
				{Name: "start", Description: "Start date or datetime.", Required: false},
				{Name: "end", Description: "End date or datetime.", Required: false},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "First call `clockify_status` to check workspace feature availability, then call `clockify_invoice_client_work` with the client name or ID, currency, and date range you have. Use returned IDs in later calls, including clientId, invoiceId, and imported entry IDs when present. If a paid feature is unavailable, report it and continue by summarizing invoice-ready work instead of failing the workflow. Summarize what changed, including IDs and changed.created, changed.updated, changed.deleted, or changed.reused."}},
			},
		},
		{
			Name:        "cleanup-demo",
			Description: "Clean deterministic demo objects by run id or prefix.",
			Arguments: []PromptArgument{
				{Name: "run_id", Description: "Stable demo run id. Default phase1.", Required: false},
				{Name: "prefix", Description: "Explicit demo prefix.", Required: false},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "First call `clockify_demo_cleanup` with the run id or prefix you have. Use returned IDs in later calls only if follow-up cleanup or verification is needed. If a paid feature is unavailable, report it and continue cleaning the core demo objects. Summarize what changed, including IDs and changed.created, changed.updated, changed.deleted, or changed.reused."}},
			},
		},
		{
			Name:        "review-week",
			Description: "Review weekly totals, issues, and suggested fixes.",
			Arguments: []PromptArgument{
				{Name: "week_start", Description: "Week start date, if known.", Required: false},
				{Name: "user", Description: "User name, email, or ID.", Required: false},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "First call `clockify_review_week` for the requested week. Use returned IDs in later calls, especially entryId, projectId, taskId, and tagId values referenced by issues or next actions. If a paid feature is unavailable, report it and continue with the local entry-based review. Summarize what changed or what should change next, including IDs and changed.created, changed.updated, changed.deleted, or changed.reused when any write follows."}},
			},
		},
		{
			Name:        "create-expense",
			Description: "Record an expense with category/project names or IDs.",
			Arguments: []PromptArgument{
				{Name: "category", Description: "Expense category name or ID.", Required: false},
				{Name: "project", Description: "Project name or ID.", Required: false},
				{Name: "amount", Description: "Expense amount.", Required: false},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "First call `clockify_record_expense` with the category, project, amount, date, notes, and receipt/file details only when the expense tool schema requires them. Use returned IDs in later calls, including expenseId, categoryId, projectId, taskId, and userId. If a paid feature is unavailable, report it and continue with a clear summary of the expense that could not be recorded. Summarize what changed, including IDs and changed.created, changed.updated, changed.deleted, or changed.reused."}},
			},
		},
		{
			Name:        "request-time-off",
			Description: "Request time off with a policy name or ID.",
			Arguments: []PromptArgument{
				{Name: "policy", Description: "Time-off policy name or ID.", Required: false},
				{Name: "start", Description: "Start date.", Required: false},
				{Name: "end", Description: "End date.", Required: false},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "First call `clockify_request_time_off` with the policy name or ID, dates, and note you have. Use returned IDs in later calls, including requestId, policyId, and userId. If a paid feature is unavailable, report it and continue with the requested dates and reason for manual handling. Summarize what changed, including IDs and changed.created, changed.updated, changed.deleted, or changed.reused."}},
			},
		},
		{
			Name:        "schedule-week",
			Description: "Schedule work for a week using names or IDs.",
			Arguments: []PromptArgument{
				{Name: "week_start", Description: "Week start date, if known.", Required: false},
				{Name: "project", Description: "Project name or ID.", Required: false},
				{Name: "user", Description: "User name, email, or ID.", Required: false},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "First call `clockify_schedule_work` with the user, project, dates, and hours you have. Use returned IDs in later calls, including assignmentId, userId, and projectId. If a paid feature is unavailable, report it and continue with a readable schedule plan. Summarize what changed, including IDs and changed.created, changed.updated, changed.deleted, or changed.reused."}},
			},
		},
		{
			Name:        "setup-webhook",
			Description: "Create a webhook from a simple name, callback URL, and event.",
			Arguments: []PromptArgument{
				{Name: "name", Description: "Webhook name.", Required: false},
				{Name: "url", Description: "HTTPS callback URL.", Required: false},
				{Name: "event", Description: "Clockify webhook event.", Required: false},
			},
			Messages: []PromptMessage{
				{Role: "user", Content: PromptMessagePart{Type: "text", Text: "First call `clockify_setup_webhook` with dry_run=true, the webhook name, HTTPS callback URL, and event you have. Review the preview and DNS/URL validation result before making a non-dry-run create call. Use returned IDs in later calls, including webhookId. If a paid feature is unavailable, report it and continue with the event and callback details for manual setup. Summarize what changed, including IDs and changed.created, changed.updated, changed.deleted, or changed.reused."}},
			},
		},
	}
}

// promptGetParams is the decoded body of a prompts/get request.
type promptGetParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func (s *Server) handlePromptsList() any {
	return map[string]any{"prompts": s.prompts.list()}
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
