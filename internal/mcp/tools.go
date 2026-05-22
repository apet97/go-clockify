package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
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
	Tools      []Tool `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"`
}

func (s *Server) tryMarshalCachedToolsListResponse(req Request) ([]byte, bool, error) {
	if req.Method != "tools/list" || req.ID == nil || !s.initialized.Load() || !toolsListParamsEmpty(req.Params) {
		return nil, false, nil
	}
	out, err := s.marshalCachedToolsListResponse(req.ID)
	return out, true, err
}

const DefaultToolsListPageSize = 80

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
	out, err := json.Marshal(s.toolsListPageLocked(0))
	if err != nil {
		return nil, err
	}
	s.toolListResultJSON = out
	s.toolListResultJSONValid = true
	return s.toolListResultJSON, nil
}

func (s *Server) handleToolsList(raw any) (toolsListResult, *RPCError) {
	cursor, err := parseToolsListCursor(raw)
	if err != nil {
		return toolsListResult{}, &RPCError{Code: -32602, Message: err.Error()}
	}
	s.mu.RLock()
	if s.toolListCacheValid {
		out := s.toolsListPageLocked(cursor)
		s.mu.RUnlock()
		return out, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.toolListCacheValid {
		s.toolListCache = s.buildToolListLocked()
		s.toolListCacheValid = true
	}
	return s.toolsListPageLocked(cursor), nil
}

func (s *Server) toolsListPageLocked(cursor int) toolsListResult {
	tools := s.toolListCache
	pageSize := s.toolsListPageSize()
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(tools) {
		cursor = len(tools)
	}
	end := len(tools)
	if pageSize > 0 && len(tools) > pageSize && cursor+pageSize < end {
		end = cursor + pageSize
	}
	result := toolsListResult{Tools: cloneToolList(tools[cursor:end])}
	if end < len(tools) {
		result.NextCursor = strconv.Itoa(end)
	}
	return result
}

func (s *Server) toolsListPageSize() int {
	if s.ToolsListPageSize <= 0 {
		return DefaultToolsListPageSize
	}
	return s.ToolsListPageSize
}

func parseToolsListCursor(raw any) (int, error) {
	if raw == nil {
		return 0, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return 0, fmt.Errorf("invalid tools/list params")
	}
	if len(m) == 0 {
		return 0, nil
	}
	rawCursor, ok := m["cursor"]
	if !ok || rawCursor == nil {
		return 0, nil
	}
	cursorText, ok := rawCursor.(string)
	if !ok {
		return 0, fmt.Errorf("invalid tools/list cursor")
	}
	cursorText = strings.TrimSpace(cursorText)
	if cursorText == "" {
		return 0, fmt.Errorf("invalid tools/list cursor")
	}
	cursor, err := strconv.Atoi(cursorText)
	if err != nil || cursor < 0 {
		return 0, fmt.Errorf("invalid tools/list cursor")
	}
	return cursor, nil
}

func toolsListParamsEmpty(raw any) bool {
	if raw == nil {
		return true
	}
	m, ok := raw.(map[string]any)
	return ok && len(m) == 0
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
			if descriptorInTier(descriptor, "default") {
				tool.OutputSchema = compactToolResultOutputSchema()
			} else {
				tool.OutputSchema = nil
			}
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

func descriptorInTier(descriptor ToolDescriptor, tier string) bool {
	for _, candidate := range descriptor.Tiers {
		if candidate == tier {
			return true
		}
	}
	return false
}

func compactToolResultOutputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"ok":        map[string]any{"type": "boolean"},
			"supported": map[string]any{"type": "boolean"},
			"performed": map[string]any{"type": "boolean"},
			"action":    map[string]any{"type": "string"},
			"ids":       map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
			"data":      map[string]any{},
			"changed":   map[string]any{"type": "object"},
			"warnings":  map[string]any{"type": "array"},
			"next":      map[string]any{"type": "array"},
			"meta":      map[string]any{"type": "object"},
			"error":     map[string]any{"type": "object"},
			"recovery":  map[string]any{"type": "object"},
		},
		"required": []string{"ok", "action"},
	}
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
	advertised := s.advertisedTools == nil || s.advertisedTools[params.Name]
	s.mu.RUnlock()
	if !ok || (s.EnforceAdvertisedTools && !advertised) {
		outcome = "tool_error"
		logError := "unknown_tool"
		if ok {
			outcome = "tool_not_advertised"
			logError = "not_advertised"
		}
		slog.Warn("tool_call", "tool", params.Name, "error", logError, "req_id", reqID)
		return nil, &UnknownToolError{Name: params.Name}
	}

	if s.toolRateLimiter != nil {
		allowed, retryAfterSeconds := s.toolRateLimiter.allow(d.RiskClass)
		if !allowed {
			outcome = "rate_limited"
			slog.Warn("tool_call", "tool", params.Name, "error", "rate_limited", "req_id", reqID)
			return rateLimitedEnvelope(params.Name, d.RiskClass, retryAfterSeconds), nil
		}
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
		s.auditToolCall(d, params.Arguments, "error", outcome)
		return nil, err
	}
	slog.Info("tool_call", "tool", params.Name, "duration_ms", duration.Milliseconds(), "req_id", reqID)
	s.auditToolCall(d, params.Arguments, "success", "")

	return result, nil
}

func (s *Server) auditToolCall(descriptor ToolDescriptor, args map[string]any, result, errorCode string) {
	if s == nil || s.AuditLogger == nil {
		return
	}
	if err := s.AuditLogger.Record(descriptor, args, result, errorCode); err != nil {
		slog.Warn("audit_log_write_failed", "tool", descriptor.Tool.Name, "error", err.Error())
	}
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
