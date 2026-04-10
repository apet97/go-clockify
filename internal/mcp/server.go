package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type ToolHandler func(context.Context, map[string]any) (any, error)

type ToolDescriptor struct {
	Tool            Tool
	Handler         ToolHandler
	ReadOnlyHint    bool
	DestructiveHint bool
	IdempotentHint  bool
}

type Server struct {
	Version      string
	Enforcement  Enforcement                     // nil = no filtering or enforcement
	Activator    Activator                       // nil = activation unrestricted
	ToolTimeout  time.Duration                   // per-call timeout; 0 = default 45s
	ReadyChecker func(ctx context.Context) error // optional upstream health check for /ready

	// MaxInFlightToolCalls bounds the number of concurrently-running
	// tools/call goroutines spawned by the stdio dispatch loop.
	// Acquired before the goroutine is created so bursty input cannot
	// amplify goroutine count. 0 = unlimited.
	MaxInFlightToolCalls int

	mu          sync.RWMutex
	tools       map[string]ToolDescriptor
	initialized atomic.Bool
	encoder     *json.Encoder // stored for push notifications
	encoderMu   sync.Mutex    // protects concurrent encoder writes
	requestSeq  atomic.Int64  // monotonic request ID for log correlation

	toolCallSem chan struct{} // dispatch-layer goroutine cap; nil = unlimited

	// readiness cache
	readyMu     sync.Mutex
	readyCached bool
	readyAt     time.Time
}

func NewServer(version string, descriptors []ToolDescriptor, enforcement Enforcement, activator Activator) *Server {
	toolMap := make(map[string]ToolDescriptor, len(descriptors))
	for _, d := range descriptors {
		toolMap[d.Tool.Name] = d
	}
	return &Server{
		Version:     version,
		Enforcement: enforcement,
		Activator:   activator,
		tools:       toolMap,
	}
}

// Run processes JSON-RPC requests from r and writes responses to w.
// It respects ctx cancellation for graceful shutdown — when ctx is
// cancelled, the loop exits even if stdin is blocking.
func (s *Server) Run(ctx context.Context, r io.Reader, w io.Writer) error {
	if s.MaxInFlightToolCalls > 0 && s.toolCallSem == nil {
		s.toolCallSem = make(chan struct{}, s.MaxInFlightToolCalls)
	}

	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	s.encoderMu.Lock()
	s.encoder = json.NewEncoder(w)
	s.encoderMu.Unlock()

	// Channel-based approach: scan lines in a goroutine so we can
	// select on ctx.Done() in the main loop.
	type scanResult struct {
		line []byte
		ok   bool
	}
	lines := make(chan scanResult, 1)

	go func() {
		defer close(lines)
		for scanner.Scan() {
			cpy := make([]byte, len(scanner.Bytes()))
			copy(cpy, scanner.Bytes())
			select {
			case lines <- scanResult{line: cpy, ok: true}:
			case <-ctx.Done():
				return
			}
		}
		// Signal EOF.
		select {
		case lines <- scanResult{ok: false}:
		case <-ctx.Done():
		}
	}()

	var wg sync.WaitGroup
	defer wg.Wait()

	for {
		select {
		case <-ctx.Done():
			return nil
		case result, chanOpen := <-lines:
			if !chanOpen || !result.ok {
				return scanner.Err()
			}
			if len(strings.TrimSpace(string(result.line))) == 0 {
				continue
			}

			var req Request
			if err := json.Unmarshal(result.line, &req); err != nil {
				if err := s.writeResponse(Response{JSONRPC: "2.0", Error: &RPCError{Code: -32700, Message: "invalid JSON"}}); err != nil {
					return err
				}
				continue
			}
			if rpcErr := validateRequest(req); rpcErr != nil {
				if err := s.writeResponse(Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErr}); err != nil {
					return err
				}
				continue
			}

			if req.Method == "tools/call" {
				// Acquire a dispatch-layer slot BEFORE spawning the
				// goroutine so bursty input cannot amplify goroutine
				// count. Context cancellation prevents shutdown deadlock.
				if s.toolCallSem != nil {
					select {
					case s.toolCallSem <- struct{}{}:
					case <-ctx.Done():
						return nil
					}
				}
				wg.Add(1)
				go func(r Request) {
					defer wg.Done()
					if s.toolCallSem != nil {
						defer func() { <-s.toolCallSem }()
					}
					resp := s.handle(ctx, r)
					if r.ID != nil || resp.Error != nil {
						if err := s.writeResponse(resp); err != nil {
							slog.Warn("async_response_failed", "error", err.Error())
						}
					}
				}(req)
				continue
			}

			resp := s.handle(ctx, req)
			if req.ID == nil && resp.Error == nil && resp.Result == nil {
				continue
			}
			if err := s.writeResponse(resp); err != nil {
				return err
			}
		}
	}
}

// writeResponse thread-safely encodes a response to the output encoder.
func (s *Server) writeResponse(resp Response) error {
	s.encoderMu.Lock()
	defer s.encoderMu.Unlock()
	if s.encoder == nil {
		return nil
	}
	return s.encoder.Encode(resp)
}

func (s *Server) handle(ctx context.Context, req Request) Response {
	resp := Response{JSONRPC: "2.0", ID: req.ID}

	switch req.Method {
	case "initialize":
		s.initialized.Store(true)
		resp.Result = map[string]any{
			"protocolVersion": "2025-06-18",
			"serverInfo":      map[string]any{"name": "clockify-go-mcp", "version": s.Version},
			"capabilities":    map[string]any{"tools": map[string]any{}},
		}
	case "notifications/initialized":
		return Response{}
	case "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		resp.Result = map[string]any{"tools": s.listTools()}
	case "tools/call":
		// Guard: reject tools/call before initialization (spec compliance)
		if !s.initialized.Load() {
			resp.Error = &RPCError{Code: -32002, Message: "server not initialized: send initialize first"}
			return resp
		}

		var params ToolCallParams
		if err := decodeParams(req.Params, &params); err != nil {
			resp.Error = &RPCError{Code: -32602, Message: "invalid tools/call params"}
			return resp
		}
		result, err := s.callTool(ctx, params)
		if err != nil {
			// MCP spec: tool errors return content with isError: true
			resp.Result = map[string]any{
				"content": []map[string]any{{
					"type": "text",
					"text": err.Error(),
				}},
				"isError": true,
			}
		} else {
			resp.Result = map[string]any{"content": []map[string]any{{"type": "text", "text": mustJSON(result)}}}
		}
	default:
		resp.Error = &RPCError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)}
	}

	return resp
}

func (s *Server) listTools() []Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.tools))
	for k := range s.tools {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	tools := make([]Tool, 0, len(keys))
	for _, key := range keys {
		d := s.tools[key]
		if s.Enforcement != nil && !s.Enforcement.FilterTool(key, ToolHints{
			ReadOnly:    d.ReadOnlyHint,
			Destructive: d.DestructiveHint,
			Idempotent:  d.IdempotentHint,
		}) {
			continue
		}
		tools = append(tools, d.Tool)
	}
	return tools
}

func (s *Server) callTool(ctx context.Context, params ToolCallParams) (any, error) {
	reqID := s.requestSeq.Add(1)

	s.mu.RLock()
	d, ok := s.tools[params.Name]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", params.Name)
	}

	if params.Arguments == nil {
		params.Arguments = map[string]any{}
	}

	hints := ToolHints{
		ReadOnly:    d.ReadOnlyHint,
		Destructive: d.DestructiveHint,
		Idempotent:  d.IdempotentHint,
	}

	// Enforcement: policy gate, rate limit, dry-run intercept
	var release func()
	if s.Enforcement != nil {
		lookup := func(name string) (ToolHandler, bool) {
			s.mu.RLock()
			td, found := s.tools[name]
			s.mu.RUnlock()
			if !found {
				return nil, false
			}
			return td.Handler, true
		}
		result, rel, err := s.Enforcement.BeforeCall(ctx, params.Name, params.Arguments, hints, lookup)
		if rel != nil {
			release = rel
			defer release()
		}
		if err != nil {
			slog.Warn("tool_call", "tool", params.Name, "error", err.Error(), "req_id", reqID)
			return nil, err
		}
		if result != nil {
			slog.Info("tool_call", "tool", params.Name, "intercepted", true, "req_id", reqID)
			return result, nil
		}
	}

	// Dispatch
	start := time.Now()
	timeout := s.ToolTimeout
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := d.Handler(callCtx, params.Arguments)
	duration := time.Since(start)

	if err != nil {
		slog.Warn("tool_call", "tool", params.Name, "error", err.Error(), "duration_ms", duration.Milliseconds(), "req_id", reqID)
		return nil, err
	}
	slog.Info("tool_call", "tool", params.Name, "duration_ms", duration.Milliseconds(), "req_id", reqID)
	if !d.ReadOnlyHint {
		slog.Info("audit", "tool", params.Name, "destructive", d.DestructiveHint, "req_id", reqID)
	}

	// Post-processing (truncation)
	if s.Enforcement != nil {
		result, _ = s.Enforcement.AfterCall(result)
	}

	return result, nil
}

// ActivateGroup registers a group of tool descriptors dynamically and
// sends a tools/list_changed notification to the client.
func (s *Server) ActivateGroup(groupName string, descriptors []ToolDescriptor) error {
	if s.Activator != nil && !s.Activator.IsGroupAllowed(groupName) {
		return fmt.Errorf("group '%s' is blocked by policy", groupName)
	}
	s.mu.Lock()
	activatedNames := make([]string, 0, len(descriptors))
	for _, d := range descriptors {
		s.tools[d.Tool.Name] = d
		activatedNames = append(activatedNames, d.Tool.Name)
	}
	s.mu.Unlock()
	if s.Activator != nil {
		s.Activator.OnActivate(activatedNames)
	}
	s.notifyToolsChanged()
	slog.Info("group_activated", "group", groupName, "tools_added", len(descriptors))
	return nil
}

// ActivateTier1Tool marks a single registered tool as visible.
func (s *Server) ActivateTier1Tool(name string) error {
	s.mu.Lock()
	if _, exists := s.tools[name]; !exists {
		s.mu.Unlock()
		return fmt.Errorf("unknown tool: %s", name)
	}
	s.mu.Unlock()
	if s.Activator != nil {
		s.Activator.OnActivate([]string{name})
	}
	s.notifyToolsChanged()
	slog.Info("tier1_tool_activated", "tool", name)
	return nil
}

func (s *Server) notifyToolsChanged() {
	s.encoderMu.Lock()
	defer s.encoderMu.Unlock()
	if s.encoder == nil {
		return
	}
	if err := s.encoder.Encode(Response{
		JSONRPC: "2.0",
		Method:  "notifications/tools/list_changed",
	}); err != nil {
		slog.Warn("notification_failed", "method", "tools/list_changed", "error", err.Error())
	}
}

func decodeParams(raw any, out any) error {
	b, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func mustJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(b)
}

// validateRequest checks JSON-RPC 2.0 version and id type per spec.
// Returns an RPCError if validation fails, or nil if valid.
func validateRequest(req Request) *RPCError {
	if req.JSONRPC != "2.0" {
		return &RPCError{Code: -32600, Message: "invalid request: jsonrpc must be \"2.0\""}
	}
	if req.ID != nil {
		switch req.ID.(type) {
		case string, float64:
			// valid per JSON-RPC 2.0 spec
		default:
			return &RPCError{Code: -32600, Message: "invalid request: id must be a string or number"}
		}
	}
	return nil
}
