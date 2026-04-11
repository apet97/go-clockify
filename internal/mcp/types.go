package mcp

import "context"

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
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type InitializeParams struct {
	ProtocolVersion string         `json:"protocolVersion,omitempty"`
	Capabilities    map[string]any `json:"capabilities,omitempty"`
	ClientInfo      map[string]any `json:"clientInfo,omitempty"`
}

type ToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
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
}

type AuditEvent struct {
	Tool        string
	Action      string
	Outcome     string
	Reason      string
	ResourceIDs map[string]string
	Metadata    map[string]string
}

type Auditor interface {
	RecordAudit(AuditEvent)
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
	BeforeCall(ctx context.Context, name string, args map[string]any, hints ToolHints, lookupHandler func(string) (ToolHandler, bool)) (result any, release func(), err error)
	// AfterCall post-processes a successful tool result (e.g., truncation).
	AfterCall(result any) (any, error)
}
