package mcp

import (
	"context"
	"fmt"
)

// Request is a decoded JSON-RPC 2.0 request or notification on the stdio
// transport. A nil ID marks a notification (no response is written).
type Request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response (or notification, when Method is set
// and ID is empty). Exactly one of Result or Error is populated for a reply.
type Response struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Method  string    `json:"method,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *RPCError `json:"error,omitempty"`
}

// RPCError is the JSON-RPC 2.0 error object carried in Response.Error. Data
// holds optional machine-readable details (e.g. a JSON Pointer for invalid
// params).
type RPCError struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data,omitempty"`
}

// JSON-RPC error codes used by this server. The standard codes
// (-32600..-32603, -32700) come from the spec; the server-specific codes
// below occupy the implementation-defined range for session, transport, and
// authorization failures.
const (
	RPCCodeSessionInvalid       = -32001
	RPCCodeServerNotInitialized = -32002
	RPCCodeOriginNotAllowed     = -32010
	RPCCodeHostNotAllowed       = -32011
	RPCCodeRateLimited          = -32012
	RPCCodeRequestTooLarge      = -32013
	RPCCodeSessionPrincipal     = -32014
	RPCCodeMethodNotAllowed     = -32015
	RPCCodeUnauthenticated      = -32020
	RPCCodeForbidden            = -32021
	RPCCodeServiceUnavailable   = -32030
)

// InvalidParamsError is returned by the tools/call dispatch when a tool's
// input arguments fail runtime JSON-schema validation. The dispatch
// translates it into a JSON-RPC -32602 (invalid params) response, with
// Pointer exposed under error.data.pointer so clients can locate the
// offending field.
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

// InitializeParams is the params payload of an MCP initialize request: the
// client's requested protocol version, advertised capabilities, and client
// info, plus optional _meta.
type InitializeParams struct {
	ProtocolVersion string         `json:"protocolVersion,omitempty"`
	Capabilities    map[string]any `json:"capabilities,omitempty"`
	ClientInfo      map[string]any `json:"clientInfo,omitempty"`
	Meta            *RequestMeta   `json:"_meta,omitempty"`
}

// ToolCallParams is the params payload of a tools/call request: the tool Name
// and its Arguments map, plus optional _meta (e.g. a progressToken).
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

// Tool is the wire representation of a single entry in a tools/list result:
// its name, optional title, description, input/output JSON schemas, and
// annotations (e.g. read-only or destructive hints).
type Tool struct {
	Name         string         `json:"name"`
	Title        string         `json:"title,omitempty"`
	Description  string         `json:"description"`
	InputSchema  map[string]any `json:"inputSchema,omitempty"`
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
	Annotations  map[string]any `json:"annotations,omitempty"`
}
