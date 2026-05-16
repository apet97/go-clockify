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
	// to 2 allocs/op on a 154-tool registry.
	out := make([]byte, 0, len(prefix)+len(idBytes)+len(resultKey)+len(result)+1)
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
		tools = append(tools, s.tools[key].Tool)
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
			out = make(map[string]any, len(args)+1)
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

// InFlightToolCalls reports the current depth of the stdio dispatch
// semaphore. Returns 0 when the semaphore is disabled.
func (s *Server) InFlightToolCalls() int {
	if s.toolCallSem == nil {
		return 0
	}
	return len(s.toolCallSem)
}
