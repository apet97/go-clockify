package mcp

import (
	"context"
	"fmt"
)

type Request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Method  string    `json:"method,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *RPCError `json:"error,omitempty"`
}

type RPCError struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data,omitempty"`
}

// InvalidParamsError is returned from Enforcement.BeforeCall when the tool's
// input arguments fail schema validation. The tools/call dispatch translates
// it into a JSON-RPC -32602 (invalid params) response, with Pointer exposed
// under error.data.pointer so clients can locate the offending field.
//
// Pointer is an RFC 6901 JSON Pointer (e.g. "/workspace_id"). An empty
// pointer means the root value itself was rejected.
type InvalidParamsError struct {
	Pointer string
	Message string
}

func (e *InvalidParamsError) Error() string {
	if e.Pointer == "" {
		return "invalid params: " + e.Message
	}
	return fmt.Sprintf("invalid params at %s: %s", e.Pointer, e.Message)
}

type InitializeParams struct {
	ProtocolVersion string         `json:"protocolVersion,omitempty"`
	Capabilities    map[string]any `json:"capabilities,omitempty"`
	ClientInfo      map[string]any `json:"clientInfo,omitempty"`
	Meta            *RequestMeta   `json:"_meta,omitempty"`
}

type ToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Meta      *RequestMeta   `json:"_meta,omitempty"`
}

// RequestMeta is the MCP _meta object that can attach side-channel hints to
// any request. progressToken is the only field used today — clients supply
// one to opt into notifications/progress from long-running tool handlers.
type RequestMeta struct {
	ProgressToken any `json:"progressToken,omitempty"`
}

// ProgressToken is the opaque client-supplied token echoed back on every
// notifications/progress. Either a string or a number per the MCP spec.
type ProgressToken = any

type progressTokenCtxKey struct{}

// WithProgressToken attaches the client-supplied progressToken (if any) to
// ctx so downstream tool handlers can emit notifications/progress keyed off
// the same value.
func WithProgressToken(ctx context.Context, token ProgressToken) context.Context {
	if token == nil {
		return ctx
	}
	return context.WithValue(ctx, progressTokenCtxKey{}, token)
}

// ProgressTokenFromContext returns the progressToken supplied in the current
// tools/call _meta, or (nil, false) when the client did not opt in.
func ProgressTokenFromContext(ctx context.Context) (ProgressToken, bool) {
	v := ctx.Value(progressTokenCtxKey{})
	if v == nil {
		return nil, false
	}
	return v, true
}

type Tool struct {
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	InputSchema  map[string]any `json:"inputSchema,omitempty"`
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
	Annotations  map[string]any `json:"annotations,omitempty"`
}

// ToolHints carries semantic hints about a tool's behavior.
type ToolHints struct {
	ReadOnly    bool
	Destructive bool
	Idempotent  bool
	// AuditKeys is forwarded from ToolDescriptor.AuditKeys so the audit
	// recorder can capture action-defining argument values (role, status,
	// quantity, unit_price, …) beyond the implicit *_id suffix scan in
	// resourceIDs. Empty for tools whose argument shape is fully
	// described by *_id keys.
	AuditKeys []string
}

// AuditPhase identifies which side of a non-read-only tool call an
// audit record was emitted on. Two-phase audit (intent → outcome)
// is what makes MCP_AUDIT_DURABILITY=fail_closed actually prevent
// mutation when audit persistence is broken: an intent record is
// written before the handler runs, and a fail_closed deployment
// short-circuits the handler when that intent persistence fails.
//
//	PhaseIntent   — pre-handler write; "we are about to call this tool"
//	PhaseOutcome  — post-handler write; "result was succeeded/failed"
//
// Empty Phase ("") is preserved for backward compatibility with
// audit consumers that pre-date the phased model and treat every
// record as a single-shot outcome.
type AuditPhase = string

const (
	PhaseIntent  AuditPhase = "intent"
	PhaseOutcome AuditPhase = "outcome"
)

type AuditEvent struct {
	Tool        string
	Action      string
	Outcome     string
	Phase       AuditPhase
	Reason      string
	ResourceIDs map[string]string
	Metadata    map[string]string
}

// Auditor records non-read-only tool-call events for compliance and audit
// trail purposes. RecordAudit returns an error when persistence fails so the
// server can make the failure observable (log + metric) and optionally fail
// the call when AuditDurabilityMode is "fail_closed".
//
// Rationale: a void return means persistence errors are silently lost.
// Returning an error makes audit degradation visible without mandating that
// every deployment fail-close on it — the server's AuditDurabilityMode field
// controls the actual behavior.
type Auditor interface {
	RecordAudit(AuditEvent) error
}

// Activator handles dynamic tool activation (group enable, visibility toggle).
// A nil Activator means activation is unrestricted.
type Activator interface {
	// IsGroupAllowed reports whether a Tier 2 group may be activated.
	IsGroupAllowed(group string) bool
	// OnActivate is called when tools are dynamically registered.
	OnActivate(names []string)
}

// Enforcement handles the tool-call enforcement pipeline.
// The server delegates all filtering, gating, and post-processing
// to this interface, keeping the protocol core free of domain logic.
//
// A nil Enforcement means no filtering or enforcement.
type Enforcement interface {
	// FilterTool reports whether a tool should be listed in tools/list.
	FilterTool(name string, hints ToolHints) bool
	// BeforeCall runs before the tool handler. It may:
	//   - block the call by returning a non-nil error
	//   - short-circuit with a result (e.g., dry-run preview)
	//   - return (nil, noop, nil) to proceed normally
	// The returned release function must be called when the call completes.
	//
	// schema is the tool's advertised InputSchema (may be nil) — the
	// pipeline uses it for runtime JSON-schema validation. Pipelines that
	// receive nil skip validation and behave as before.
	BeforeCall(ctx context.Context, name string, args map[string]any, hints ToolHints, schema map[string]any, lookupHandler func(string) (ToolHandler, bool)) (result any, release func(), err error)
	// AfterCall post-processes a successful tool result (e.g., truncation).
	AfterCall(result any) (any, error)
}
