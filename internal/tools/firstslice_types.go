package tools

import "github.com/apet97/go-clockify/internal/clockify"

// ToolResult is the single success envelope every tool handler
// returns. Post-T2.1 the legacy ResultEnvelope name was retired and
// ToolResult is now the only success type. OK is the boolean success
// flag; Action mirrors the tool name for client-side dispatch; Data
// carries the typed payload; Meta carries cross-cutting metadata
// (pagination cursors, fingerprint hashes). Entity / IDs / Changed /
// Warnings / Next are optional — every field uses omitempty (and
// Changed uses omitzero for value-type omission), so a narrow handler
// that fills only {OK, Action, Data, Meta} emits the historical
// four-key wire shape unchanged.
type ToolResult struct {
	OK       bool              `json:"ok"`
	Action   string            `json:"action"`
	Entity   string            `json:"entity,omitempty"`
	IDs      map[string]string `json:"ids,omitempty"`
	Data     any               `json:"data,omitempty"`
	Meta     map[string]any    `json:"meta,omitempty"`
	Changed  ChangeSet         `json:"changed,omitzero"`
	Warnings []Warning         `json:"warnings,omitempty"`
	Next     []NextAction      `json:"next,omitempty"`
}

// ToolError is the recoverable-failure envelope a tool handler returns
// (ok:false): the action, any partial IDs, a structured Error, and a
// RecoveryHint guiding the caller toward a fix. Supported/Performed flag the
// "unsupported endpoint" and "no-op" cases.
type ToolError struct {
	OK        bool              `json:"ok"`
	Supported *bool             `json:"supported,omitempty"`
	Performed *bool             `json:"performed,omitempty"`
	Action    string            `json:"action"`
	IDs       map[string]string `json:"ids,omitempty"`
	Error     ErrorInfo         `json:"error"`
	Recovery  RecoveryHint      `json:"recovery"`
	Warnings  []Warning         `json:"warnings,omitempty"`
}

// ChangeSet records the entities a write touched, grouped by created, updated,
// deleted, and reused, for the ToolResult.changed block.
type ChangeSet struct {
	Created []EntityRef `json:"created,omitempty"`
	Updated []EntityRef `json:"updated,omitempty"`
	Deleted []EntityRef `json:"deleted,omitempty"`
	Reused  []EntityRef `json:"reused,omitempty"`
}

// EntityRef identifies one affected entity in a ChangeSet by type, id, and
// optional name.
type EntityRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// Warning is a non-fatal advisory attached to a tool result: an optional code
// and a human-readable message.
type Warning struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

// NextAction suggests a follow-up tool call (with optional args and a reason)
// to chain after the current result.
type NextAction struct {
	Tool   string         `json:"tool"`
	Args   map[string]any `json:"args,omitempty"`
	Reason string         `json:"reason,omitempty"`
}

// ErrorInfo is the structured error detail in a ToolError: a stable code and a
// client-facing message.
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// RecoveryHint guides a caller toward recovering from a ToolError: a free-text
// hint, an optional suggested tool/args to retry, and retryability metadata.
type RecoveryHint struct {
	Hint              string         `json:"hint"`
	Tool              string         `json:"tool,omitempty"`
	Args              map[string]any `json:"args,omitempty"`
	Retryable         bool           `json:"retryable,omitempty"`
	RetryAfterSeconds int            `json:"retryAfterSeconds,omitempty"`
}

type statusData struct {
	User                  clockify.User      `json:"user"`
	Workspace             clockify.Workspace `json:"workspace"`
	ActiveWorkspaceID     string             `json:"activeWorkspaceId,omitempty"`
	DefaultWorkspaceID    string             `json:"defaultWorkspaceId,omitempty"`
	Timezone              string             `json:"timezone"`
	WeekStart             string             `json:"weekStart,omitempty"`
	CurrentTimer          any                `json:"currentTimer,omitempty"`
	FeatureSubscription   string             `json:"featureSubscriptionType,omitempty"`
	FeatureStatus         map[string]string  `json:"featureStatus"`
	RecommendedFirstTools []string           `json:"recommendedFirstTools"`
	OperationalLimits     map[string]any     `json:"operationalLimits,omitempty"`
}
