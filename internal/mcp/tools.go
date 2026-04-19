package mcp

import (
	"context"
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
		s.recordAuditBestEffort(params.Name, "tools/call", outcome, "unknown_tool", params.Arguments, ToolHints{})
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
			case strings.Contains(err.Error(), "blocked by policy"):
				outcome = "policy_denied"
			default:
				outcome = "tool_error"
			}
			s.recordAuditBestEffort(params.Name, "tools/call", outcome, err.Error(), params.Arguments, hints)
			slog.Warn("tool_call", "tool", params.Name, "error", err.Error(), "req_id", reqID)
			return nil, err
		}
		if result != nil {
			outcome = "dry_run"
			s.recordAuditBestEffort(params.Name, "tools/call", outcome, "dry_run_intercepted", params.Arguments, hints)
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
		// Audit the failure; best_effort always (the call failed; fail_closed
		// only applies to successful mutations).
		s.recordAuditBestEffort(params.Name, "tools/call", outcome, err.Error(), params.Arguments, hints)
		slog.Warn("tool_call", "tool", params.Name, "error", err.Error(), "duration_ms", duration.Milliseconds(), "req_id", reqID)
		return nil, err
	}
	slog.Info("tool_call", "tool", params.Name, "duration_ms", duration.Milliseconds(), "req_id", reqID)
	if !d.ReadOnlyHint {
		// For a successful non-read-only call, audit failure behavior depends
		// on AuditDurabilityMode. In "fail_closed" mode, a persistence failure
		// causes this function to return an error so the client knows the audit
		// trail is incomplete. In "best_effort" mode (default), the failure is
		// logged and counted but the call is reported as successful.
		if auditErr := s.recordAuditWithDurability(params.Name, "tools/call", outcome, "", params.Arguments, hints); auditErr != nil {
			return nil, auditErr
		}
		slog.Info("audit", "tool", params.Name, "destructive", d.DestructiveHint, "req_id", reqID)
	}

	// Post-processing (truncation)
	if s.Enforcement != nil {
		result, _ = s.Enforcement.AfterCall(result)
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

// notifyToolsChanged delivers notifications/tools/list_changed through the
// configured Notifier. If no notifier is installed (e.g. a test harness that
// calls ActivateGroup directly without running a transport), the notification
// is dropped and counted so the gap is visible in /metrics.
func (s *Server) notifyToolsChanged() {
	if s.hub.len() == 0 {
		metrics.ProtocolErrorsTotal.Inc("notification_dropped_no_notifier")
		slog.Warn("notification_dropped",
			"method", "notifications/tools/list_changed",
			"reason", "no_notifier_installed",
		)
		return
	}
	if err := s.Notify("notifications/tools/list_changed", map[string]any{}); err != nil {
		slog.Warn("notification_failed",
			"method", "notifications/tools/list_changed",
			"error", err.Error(),
		)
	}
}
