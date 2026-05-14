package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/metrics"
	"github.com/apet97/go-clockify/internal/ratelimit"
	"github.com/apet97/go-clockify/internal/tracing"
)

type UnknownToolError struct {
	Name string
}

func (e *UnknownToolError) Error() string {
	if strings.TrimSpace(e.Name) == "" {
		return "unknown tool"
	}
	return fmt.Sprintf("unknown tool: %s", e.Name)
}

func (s *Server) listTools() []Tool {
	s.mu.RLock()
	if s.toolListCacheValid {
		out := cloneToolList(s.toolListCache)
		s.mu.RUnlock()
		return out
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.toolListCacheValid {
		return cloneToolList(s.toolListCache)
	}
	s.toolListCache = s.buildToolListLocked()
	s.toolListCacheValid = true
	return cloneToolList(s.toolListCache)
}

// VisibleToolCount reports the current tools/list count after enforcement
// filtering. Activators use it after a successful activation so responses can
// distinguish a group-local tool count from the session-visible total.
func (s *Server) VisibleToolCount() int {
	return len(s.listTools())
}

// VisibleToolNames reports the current tools/list names after enforcement
// filtering. Activation responses use it to avoid promising tools that the
// client still cannot call because bootstrap or policy filtering hides them.
func (s *Server) VisibleToolNames() map[string]bool {
	tools := s.listTools()
	out := make(map[string]bool, len(tools))
	for _, tool := range tools {
		out[tool.Name] = true
	}
	return out
}

type toolsListResult struct {
	Tools []Tool `json:"tools"`
}

func (s *Server) tryMarshalCachedToolsListResponse(req Request) ([]byte, bool, error) {
	if req.Method != "tools/list" || req.ID == nil || !s.initialized.Load() {
		return nil, false, nil
	}
	out, err := s.marshalCachedToolsListResponse(req.ID)
	return out, true, err
}

func (s *Server) marshalCachedToolsListResponse(id any) ([]byte, error) {
	result, err := s.toolsListResultJSONBytes()
	if err != nil {
		return nil, err
	}
	idBytes, err := json.Marshal(id)
	if err != nil {
		return nil, err
	}

	const prefix = `{"jsonrpc":"2.0","id":`
	const resultKey = `,"result":`
	var out []byte
	out = append(out, prefix...)
	out = append(out, idBytes...)
	out = append(out, resultKey...)
	out = append(out, result...)
	out = append(out, '}')
	return out, nil
}

func (s *Server) toolsListResultJSONBytes() ([]byte, error) {
	s.mu.RLock()
	if s.toolListResultJSONValid {
		out := s.toolListResultJSON
		s.mu.RUnlock()
		return out, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.toolListResultJSONValid {
		return s.toolListResultJSON, nil
	}
	if !s.toolListCacheValid {
		s.toolListCache = s.buildToolListLocked()
		s.toolListCacheValid = true
	}
	out, err := json.Marshal(toolsListResult{Tools: s.toolListCache})
	if err != nil {
		return nil, err
	}
	s.toolListResultJSON = out
	s.toolListResultJSONValid = true
	return s.toolListResultJSON, nil
}

func (s *Server) buildToolListLocked() []Tool {
	keys := make([]string, 0, len(s.tools))
	priorityAware := false
	for k := range s.tools {
		keys = append(keys, k)
		if _, ok := toolPriority(s.tools[k].Tool); ok {
			priorityAware = true
		}
	}
	if priorityAware {
		sort.Slice(keys, func(i, j int) bool {
			left, lok := toolPriority(s.tools[keys[i]].Tool)
			right, rok := toolPriority(s.tools[keys[j]].Tool)
			if !lok {
				left = 50
			}
			if !rok {
				right = 50
			}
			if left != right {
				return left < right
			}
			return keys[i] < keys[j]
		})
	} else {
		sort.Strings(keys)
	}

	tools := make([]Tool, 0, len(keys))
	for _, key := range keys {
		d := s.tools[key]
		if s.Enforcement != nil && !s.Enforcement.FilterTool(key, ToolHints{
			ReadOnly:    d.ReadOnlyHint,
			Destructive: d.DestructiveHint,
			Idempotent:  d.IdempotentHint,
			RiskClass:   d.RiskClass,
		}) {
			continue
		}
		tools = append(tools, d.Tool)
	}
	return tools
}

func toolPriority(tool Tool) (int, bool) {
	if tool.Annotations == nil {
		return 0, false
	}
	switch v := tool.Annotations["priority"].(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func cloneToolList(in []Tool) []Tool {
	out := make([]Tool, len(in))
	copy(out, in)
	return out
}

func (s *Server) invalidateToolListCacheLocked() {
	s.toolListCache = nil
	s.toolListCacheValid = false
	s.toolListResultJSON = nil
	s.toolListResultJSONValid = false
}

func (s *Server) invokeHandler(ctx context.Context, _ string, d ToolDescriptor, args map[string]any, _ ToolHints, _ int64) (any, error) {
	return d.Handler(ctx, args)
}

func (s *Server) callTool(ctx context.Context, params ToolCallParams) (any, error) {
	ctx, span := tracing.Default.Start(ctx, "mcp.tools/call")
	span.SetAttribute("tool.name", params.Name)
	defer span.End()

	reqID := s.requestSeq.Add(1)
	callStart := time.Now()
	outcome := "success"
	defer func() {
		span.SetAttribute("outcome", outcome)
		metrics.ToolCallsTotal.Inc(params.Name, outcome)
		metrics.ToolCallDuration.Observe(time.Since(callStart).Seconds(), params.Name)
	}()

	s.mu.RLock()
	d, ok := s.tools[params.Name]
	s.mu.RUnlock()
	if !ok {
		outcome = "tool_error"
		slog.Warn("tool_call", "tool", params.Name, "error", "unknown_tool", "req_id", reqID)
		return nil, &UnknownToolError{Name: params.Name}
	}

	if params.Arguments == nil {
		params.Arguments = map[string]any{}
	}

	hints := ToolHints{
		ReadOnly:    d.ReadOnlyHint,
		Destructive: d.DestructiveHint,
		Idempotent:  d.IdempotentHint,
		RiskClass:   d.RiskClass,
	}

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
		result, rel, err := s.Enforcement.BeforeCall(ctx, params.Name, params.Arguments, hints, d.Tool.InputSchema, lookup)
		if rel != nil {
			release = rel
			defer release()
		}
		if err != nil {
			var ipe *InvalidParamsError
			switch {
			case errors.As(err, &ipe):
				outcome = "invalid_params"
			case errors.Is(err, ratelimit.ErrRateLimitExceeded), errors.Is(err, ratelimit.ErrConcurrencyLimitExceeded):
				outcome = "rate_limited"
			default:
				outcome = "tool_error"
			}
			slog.Warn("tool_call", "tool", params.Name, "error", err.Error(), "req_id", reqID)
			return nil, err
		}
		if result != nil {
			outcome = "dry_run"
			slog.Info("tool_call", "tool", params.Name, "intercepted", true, "req_id", reqID)
			return result, nil
		}
	}

	start := time.Now()
	timeout := s.ToolTimeout
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := s.invokeHandler(callCtx, params.Name, d, params.Arguments, hints, reqID)
	duration := time.Since(start)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded) || errors.Is(callCtx.Err(), context.DeadlineExceeded):
			outcome = "timeout"
			metrics.Cancellations.Inc("timeout")
		case errors.Is(err, context.Canceled) || errors.Is(callCtx.Err(), context.Canceled):
			outcome = "cancelled"
			metrics.Cancellations.Inc("context_cancelled")
		default:
			outcome = "tool_error"
		}
		slog.Warn("tool_call", "tool", params.Name, "error", err.Error(), "duration_ms", duration.Milliseconds(), "req_id", reqID)
		return nil, err
	}
	slog.Info("tool_call", "tool", params.Name, "duration_ms", duration.Milliseconds(), "req_id", reqID)

	if s.Enforcement != nil {
		processed, err := s.Enforcement.AfterCall(result)
		if err != nil {
			return nil, err
		}
		result = processed
	}
	return result, nil
}

// InFlightToolCalls reports the current depth of the stdio dispatch
// semaphore. Returns 0 when the semaphore is disabled.
func (s *Server) InFlightToolCalls() int {
	if s.toolCallSem == nil {
		return 0
	}
	return len(s.toolCallSem)
}
