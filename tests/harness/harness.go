// Package harness provides a unified TransportHarness interface that lets
// one test body exercise stdio, legacy HTTP, streamable HTTP, and gRPC in
// parallel. Each factory binds an OS-assigned port (or bufconn for gRPC) so
// tests can run with t.Parallel() without colliding on fixed ports.
//
// The harness is test-only; it is NOT imported by production code.
package harness

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/apet97/go-clockify/internal/mcp"
)

// Response is the decoded JSON-RPC envelope returned by harness operations.
// Result is kept as raw JSON so each test can unmarshal into whatever
// struct it cares about; tests that only want a sanity check can compare
// against Error == nil.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// RPCError is the JSON-RPC 2.0 error envelope.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Options controls per-harness construction. Defaults are chosen to match
// the production defaults (4 MiB size cap, stdio-style single-user tools).
type Options struct {
	// MaxMessageSize, if >0, configures the transport's size cap and the
	// server's MaxMessageSize. Zero leaves both at their default (4 MiB).
	MaxMessageSize int64

	// BearerToken, if non-empty, enables static-bearer auth on transports
	// that require it (legacy HTTP, streamable HTTP, gRPC). stdio ignores
	// this field.
	BearerToken string

	// Tools lists the ToolDescriptors the mock server should expose. Nil
	// means the harness registers only the default mock_tool.
	Tools []mcp.ToolDescriptor
}

// ServerSharer is an optional interface implemented by transports whose
// underlying mcp.Server is reachable from the test body — used for
// firing server-initiated notifications in parity tests. stdio and
// streamable HTTP implement it directly; legacy HTTP and gRPC return
// (nil, false) so tests can skip.
type ServerSharer interface {
	SharedServer() (*mcp.Server, bool)
}

// Transport is the contract every transport adapter satisfies. Callers
// issue Initialize → ListTools → CallTool and optionally Cancel; tests
// that need to assert server-initiated notifications read from
// Notifications(). MaxSupportedSize is what the transport was configured
// to enforce — tests use it to compute at-limit / over-limit boundaries
// without hard-coding numbers.
type Transport interface {
	Name() string
	Initialize(ctx context.Context) (Response, error)
	ListTools(ctx context.Context) (Response, error)
	CallTool(ctx context.Context, name string, args map[string]any) (Response, error)
	CallToolAsync(ctx context.Context, name string, args map[string]any) (requestID int, done <-chan Response, err error)
	Cancel(ctx context.Context, requestID int) error
	// Notifications returns a channel of server→client frames (for
	// transports that support them: stdio, streamable HTTP, gRPC).
	// Legacy HTTP returns nil — callers must handle the nil case
	// explicitly rather than blocking forever on <-nil.
	Notifications() <-chan Response
	MaxSupportedSize() int64
	Close() error
}

// Factory constructs a Transport for a given Options; used by the parity
// matrix so one test can iterate factories without knowing their types.
type Factory func(ctx context.Context, opts Options) (Transport, error)

// buildMockServer returns an mcp.Server wired with the tool set in opts.
// A default mock_tool is registered when opts.Tools is empty so simple
// harness users don't have to think about tool registration.
func buildMockServer(opts Options) *mcp.Server {
	tools := opts.Tools
	if len(tools) == 0 {
		tools = []mcp.ToolDescriptor{{
			Tool: mcp.Tool{
				Name:        "mock_tool",
				Description: "Mock tool for harness E2E tests",
				InputSchema: map[string]any{"type": "object"},
			},
			Handler: func(_ context.Context, _ map[string]any) (any, error) {
				return map[string]string{"status": "ok"}, nil
			},
		}}
	}
	srv := mcp.NewServer("harness-test", tools, nil, nil)
	if opts.MaxMessageSize > 0 {
		srv.MaxMessageSize = opts.MaxMessageSize
	}
	return srv
}

// MockTool is a convenience constructor for tests that need a minimal
// ToolDescriptor with a fixed handler.
func MockTool(name string, handler mcp.ToolHandler) mcp.ToolDescriptor {
	return mcp.ToolDescriptor{
		Tool: mcp.Tool{
			Name:        name,
			Description: "Mock tool " + name,
			InputSchema: map[string]any{"type": "object"},
		},
		Handler: handler,
	}
}

// BlockingTool returns a ToolDescriptor whose handler signals start on
// the given channel, then blocks until its context is cancelled. The
// handler returns the context's error, so a successful cancellation
// surfaces as an RPC error with code=-32603 (internal error carrying
// context.Canceled) or an isError=true result. Used by cancellation
// parity tests to prove that Cancel() / ctx cancellation actually
// aborts in-flight work rather than sitting there until the per-tool
// 45s timeout.
func BlockingTool(name string, started chan<- struct{}) mcp.ToolDescriptor {
	return mcp.ToolDescriptor{
		Tool: mcp.Tool{
			Name:        name,
			Description: "Blocking tool " + name + " — blocks until context cancelled",
			InputSchema: map[string]any{"type": "object"},
		},
		Handler: func(ctx context.Context, _ map[string]any) (any, error) {
			select {
			case started <- struct{}{}:
			default:
			}
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
}

// encodeRequest marshals a JSON-RPC 2.0 request with id, method, and params.
func encodeRequest(id int, method string, params any) ([]byte, error) {
	envelope := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		envelope["params"] = params
	}
	b, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// encodeNotification marshals a JSON-RPC 2.0 notification (no id).
func encodeNotification(method string, params any) ([]byte, error) {
	envelope := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		envelope["params"] = params
	}
	b, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// ToolName extracts the "name" field from a tools/list result.tools[] entry
// decoded into a generic map. Returns "" if the shape is unexpected.
func ToolName(m map[string]any) string {
	if s, ok := m["name"].(string); ok {
		return s
	}
	return ""
}

// ToolsFromListResult decodes a tools/list Response.Result into a flat
// list of tool descriptors (each a generic map) for assertions.
func ToolsFromListResult(resp Response) ([]map[string]any, error) {
	var body struct {
		Tools []map[string]any `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		return nil, err
	}
	return body.Tools, nil
}

// ContainsTool returns true iff the decoded tools list includes one whose
// name equals the given value.
func ContainsTool(tools []map[string]any, name string) bool {
	for _, t := range tools {
		if ToolName(t) == name {
			return true
		}
	}
	return false
}

// ProtocolVersion unmarshals an initialize Response.Result and extracts the
// protocolVersion string.
func ProtocolVersion(resp Response) (string, error) {
	var body struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		return "", err
	}
	return body.ProtocolVersion, nil
}

// IsNotFound returns true when the error message matches the common
// JSON-RPC "method not found" / "tool not found" shapes the transports
// emit. Used by parity tests that check invalid-params behaviour without
// hardcoding error-code numbers.
func IsNotFound(err *RPCError) bool {
	if err == nil {
		return false
	}
	return err.Code == -32601 || strings.Contains(strings.ToLower(err.Message), "not found")
}
