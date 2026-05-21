package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/jsonschema"
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
	// Pre-size the response buffer so the JSON-RPC envelope around the
	// (large) cached tools/list payload allocates exactly once instead
	// of growing through ~5 doublings — measured 18 allocs/op shrinks
	// to 2 allocs/op on a 156-tool registry.
	capHint, err := checkedByteBufferCapacity(len(prefix), len(idBytes), len(resultKey), len(result), 1)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, capHint)
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
	items := make([]struct {
		name string
		tool Tool
		prio int
	}, 0, len(s.tools))
	priorityAware := false
	for name, descriptor := range s.tools {
		if s.advertisedTools != nil && !s.advertisedTools[name] {
			continue
		}
		prio, ok := toolPriority(descriptor.Tool)
		if !ok {
			prio = 50
		} else {
			priorityAware = true
		}
		tool := descriptor.Tool
		if !descriptor.AdvertiseOutputSchema {
			tool.OutputSchema = nil
		}
		items = append(items, struct {
			name string
			tool Tool
			prio int
		}{name: name, tool: tool, prio: prio})
	}
	if priorityAware {
		sort.Slice(items, func(i, j int) bool {
			if items[i].prio != items[j].prio {
				return items[i].prio < items[j].prio
			}
			return items[i].name < items[j].name
		})
	} else {
		sort.Slice(items, func(i, j int) bool {
			return items[i].name < items[j].name
		})
	}

	tools := make([]Tool, 0, len(items))
	for _, item := range items {
		tools = append(tools, item.tool)
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
	for i := range in {
		out[i] = cloneTool(in[i])
	}
	return out
}

func cloneTool(in Tool) Tool {
	out := in
	out.InputSchema = deepCopyAnyMap(out.InputSchema)
	out.OutputSchema = deepCopyAnyMap(out.OutputSchema)
	out.Annotations = deepCopyAnyMap(out.Annotations)
	return out
}

func deepCopyAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = deepCopyAnyValue(v)
	}
	return out
}

func deepCopyAnyValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return deepCopyAnyMap(t)
	case []any:
		if t == nil {
			return []any(nil)
		}
		cp := make([]any, len(t))
		for i := range t {
			cp[i] = deepCopyAnyValue(t[i])
		}
		return cp
	case []string:
		if t == nil {
			return []string(nil)
		}
		cp := make([]string, len(t))
		copy(cp, t)
		return cp
	default:
		return v
	}
}

func (s *Server) invalidateToolListCacheLocked() {
	s.toolListCache = nil
	s.toolListCacheValid = false
	s.toolListResultJSON = nil
	s.toolListResultJSONValid = false
}

func (s *Server) callTool(ctx context.Context, params ToolCallParams) (any, error) {
	ctx, span := tracing.Default.Start(ctx, "mcp.tools/call")
	span.SetAttribute("tool.name", params.Name)
	defer span.End()

	reqID := s.requestSeq.Add(1)
	outcome := "success"
	defer func() {
		span.SetAttribute("outcome", outcome)
	}()

	s.mu.RLock()
	d, ok := s.tools[params.Name]
	s.mu.RUnlock()
	if !ok {
		outcome = "tool_error"
		slog.Warn("tool_call", "tool", params.Name, "error", "unknown_tool", "req_id", reqID)
		return nil, &UnknownToolError{Name: params.Name}
	}

	if s.toolRateLimiter != nil && !s.toolRateLimiter.allow() {
		outcome = "rate_limited"
		slog.Warn("tool_call", "tool", params.Name, "error", "rate_limited", "req_id", reqID)
		return rateLimitedEnvelope(params.Name), nil
	}

	if params.Arguments == nil {
		params.Arguments = map[string]any{}
	}
	if err := jsonschema.Validate(d.Tool.InputSchema, schemaValidationArguments(d.Tool.InputSchema, params.Arguments)); err != nil {
		var ve *jsonschema.ValidationError
		if errors.As(err, &ve) {
			err = &InvalidParamsError{Pointer: ve.Pointer, Message: ve.Message}
		}
		outcome = "invalid_params"
		slog.Warn("tool_call", "tool", params.Name, "error", err.Error(), "req_id", reqID)
		return nil, err
	}

	start := time.Now()
	timeout := s.ToolTimeout
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if s.toolFamilyCaps != nil {
		release, acqErr := s.toolFamilyCaps.acquire(callCtx, d.RiskClass)
		if acqErr != nil {
			outcome = "timeout"
			slog.Warn("tool_call", "tool", params.Name, "error", acqErr.Error(), "req_id", reqID)
			return nil, acqErr
		}
		defer release()
	}

	result, err := d.Handler(callCtx, params.Arguments)
	duration := time.Since(start)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded) || errors.Is(callCtx.Err(), context.DeadlineExceeded):
			outcome = "timeout"
		case errors.Is(err, context.Canceled) || errors.Is(callCtx.Err(), context.Canceled):
			outcome = "cancelled"
		default:
			outcome = "tool_error"
		}
		slog.Warn("tool_call", "tool", params.Name, "error", err.Error(), "duration_ms", duration.Milliseconds(), "req_id", reqID)
		return nil, err
	}
	slog.Info("tool_call", "tool", params.Name, "duration_ms", duration.Milliseconds(), "req_id", reqID)

	return result, nil
}

func schemaValidationArguments(schema map[string]any, args map[string]any) map[string]any {
	if len(schema) == 0 || len(args) == 0 {
		return args
	}
	props, _ := schema["properties"].(map[string]any)
	if len(props) == 0 {
		return args
	}
	required := schemaRequiredNames(schema)
	if len(required) == 0 {
		return args
	}
	var out map[string]any
	for _, name := range required {
		if _, ok := args[name]; ok {
			continue
		}
		alias := name + "_id"
		if _, hasAliasProp := props[alias]; !hasAliasProp {
			continue
		}
		value, hasAliasValue := args[alias]
		if !hasAliasValue {
			continue
		}
		if out == nil {
			// Measured by BenchmarkDispatchToolsCallAliasArgument; the small
			// copy keeps jsonschema validation unchanged while supporting _id aliases.
			out = make(map[string]any, schemaValidationCopyCapacity(len(args)))
			for key, value := range args {
				out[key] = value
			}
		}
		out[name] = value
	}
	if out == nil {
		return args
	}
	return out
}

func schemaRequiredNames(schema map[string]any) []string {
	switch required := schema["required"].(type) {
	case []string:
		return required
	case []any:
		out := make([]string, 0, len(required))
		for _, value := range required {
			if str, ok := value.(string); ok {
				out = append(out, str)
			}
		}
		return out
	default:
		return nil
	}
}

func checkedByteBufferCapacity(parts ...int) (int, error) {
	total := 0
	for _, part := range parts {
		if part < 0 || total > math.MaxInt-part {
			return 0, fmt.Errorf("byte buffer capacity overflow")
		}
		total += part
	}
	return total, nil
}

func schemaValidationCopyCapacity(argsLen int) int {
	if argsLen >= math.MaxInt {
		return argsLen
	}
	return argsLen + 1
}

// InFlightToolCalls reports the current depth of the stdio dispatch
// semaphore. Returns 0 when the semaphore is disabled.
func (s *Server) InFlightToolCalls() int {
	if s.toolCallSem == nil {
		return 0
	}
	return len(s.toolCallSem)
}
