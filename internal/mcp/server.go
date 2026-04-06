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

	"github.com/apet97/go-clockify/internal/bootstrap"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/ratelimit"
	"github.com/apet97/go-clockify/internal/truncate"
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
	Version    string
	Policy     *policy.Policy
	Bootstrap  *bootstrap.Config
	RateLimit  *ratelimit.RateLimiter
	Truncation truncate.Config
	DryRun     dryrun.Config

	mu          sync.RWMutex
	tools       map[string]ToolDescriptor
	initialized atomic.Bool
	encoder     *json.Encoder // stored for push notifications
	encoderMu   sync.Mutex    // protects concurrent encoder writes
	requestSeq  atomic.Int64  // monotonic request ID for log correlation
}

func NewServer(version string, pol *policy.Policy, descriptors []ToolDescriptor,
	rl *ratelimit.RateLimiter, tc truncate.Config, dc dryrun.Config, bc *bootstrap.Config) *Server {
	toolMap := make(map[string]ToolDescriptor, len(descriptors))
	for _, d := range descriptors {
		toolMap[d.Tool.Name] = d
	}
	return &Server{
		Version:    version,
		Policy:     pol,
		Bootstrap:  bc,
		RateLimit:  rl,
		Truncation: tc,
		DryRun:     dc,
		tools:      toolMap,
	}
}

// Run processes JSON-RPC requests from r and writes responses to w.
// It respects ctx cancellation for graceful shutdown — when ctx is
// cancelled, the loop exits even if stdin is blocking.
func (s *Server) Run(ctx context.Context, r io.Reader, w io.Writer) error {
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

			if req.Method == "tools/call" {
				wg.Add(1)
				go func(r Request) {
					defer wg.Done()
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
		// Bootstrap filter
		if s.Bootstrap != nil && !s.Bootstrap.IsVisible(key) {
			continue
		}
		// Policy filter
		if s.Policy != nil && !s.Policy.IsAllowed(d.Tool.Name, d.ReadOnlyHint) {
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

	// 1. Policy check
	if s.Policy != nil && !s.Policy.IsAllowed(d.Tool.Name, d.ReadOnlyHint) {
		reason := "blocked by policy"
		if s.Policy != nil {
			reason = s.Policy.BlockReason(d.Tool.Name, d.ReadOnlyHint)
		}
		return nil, fmt.Errorf("tool blocked by policy: %s", reason)
	}

	// 2. Rate limit
	if s.RateLimit != nil {
		release, err := s.RateLimit.Acquire(ctx)
		if err != nil {
			return nil, fmt.Errorf("rate limited: %s", err)
		}
		defer release()
	}

	// 3. Dry-run intercept
	if params.Arguments == nil {
		params.Arguments = map[string]any{}
	}
	if action, isDryRun := dryrun.CheckDryRun(params.Name, params.Arguments, d.DestructiveHint); isDryRun {
		return s.handleDryRun(ctx, action, params, d)
	}

	// 4. Dispatch
	start := time.Now()

	callCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	result, err := d.Handler(callCtx, params.Arguments)
	duration := time.Since(start)

	// 5. Log with request ID for correlation
	if err != nil {
		slog.Warn("tool_call", "tool", params.Name, "error", err.Error(), "duration_ms", duration.Milliseconds(), "req_id", reqID)
		return nil, err
	}
	slog.Info("tool_call", "tool", params.Name, "duration_ms", duration.Milliseconds(), "req_id", reqID)

	// 6. Truncation
	if s.Truncation.Enabled {
		result, _ = s.Truncation.Truncate(result)
	}

	return result, nil
}

func (s *Server) handleDryRun(ctx context.Context, action dryrun.Action, params ToolCallParams, d ToolDescriptor) (any, error) {
	switch action {
	case dryrun.NotDestructive:
		return nil, dryrun.NotDestructiveError(params.Name)
	case dryrun.ConfirmPattern:
		// Remove confirm flag and call handler normally for preview
		delete(params.Arguments, "confirm")
		result, err := d.Handler(ctx, params.Arguments)
		if err != nil {
			return nil, err
		}
		return dryrun.WrapResult(result, params.Name), nil
	case dryrun.PreviewTool:
		previewTool, ok := dryrun.PreviewToolFor(params.Name)
		if !ok {
			return dryrun.MinimalResult(params.Name, params.Arguments), nil
		}
		s.mu.RLock()
		previewD, exists := s.tools[previewTool]
		s.mu.RUnlock()
		if !exists {
			return dryrun.MinimalResult(params.Name, params.Arguments), nil
		}
		previewArgs := dryrun.BuildPreviewArgs(params.Arguments)
		result, err := previewD.Handler(ctx, previewArgs)
		if err != nil {
			return nil, err
		}
		return dryrun.WrapResult(result, params.Name), nil
	case dryrun.MinimalFallback:
		return dryrun.MinimalResult(params.Name, params.Arguments), nil
	default:
		return dryrun.MinimalResult(params.Name, params.Arguments), nil
	}
}

// ActivateGroup registers a group of tool descriptors dynamically and
// sends a tools/list_changed notification to the client.
func (s *Server) ActivateGroup(groupName string, descriptors []ToolDescriptor) error {
	if s.Policy != nil && !s.Policy.IsGroupAllowed(groupName) {
		return fmt.Errorf("group '%s' is blocked by policy", groupName)
	}
	s.mu.Lock()
	for _, d := range descriptors {
		s.tools[d.Tool.Name] = d
	}
	s.mu.Unlock()
	// Send tools/list_changed notification
	s.notifyToolsChanged()
	slog.Info("group_activated", "group", groupName, "tools_added", len(descriptors))
	return nil
}

// ActivateTier1Tool marks a single tool as visible. The tool must already
// be registered; bootstrap manages the actual visibility.
func (s *Server) ActivateTier1Tool(name string) {
	// Nothing to do for the tool map — deferred tools are already registered.
	// This is about making them visible in tools/list via bootstrap.
	// The bootstrap config manages visibility.
	slog.Info("tier1_tool_activated", "tool", name)
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
