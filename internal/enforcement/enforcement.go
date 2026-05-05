// Package enforcement provides the concrete Enforcement and Activator
// implementations that compose the safety subsystems (policy, rate limiting,
// dry-run, truncation, bootstrap) into the MCP server's pluggable interfaces.
//
// This package sits between the protocol core (mcp) and the domain-specific
// safety packages, keeping both layers decoupled.
package enforcement

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/bootstrap"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/jsonschema"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/metrics"
	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/ratelimit"
	"github.com/apet97/go-clockify/internal/truncate"
)

// Pipeline implements mcp.Enforcement by composing the safety subsystems.
type Pipeline struct {
	Policy     *policy.Policy
	Bootstrap  *bootstrap.Config
	RateLimit  *ratelimit.RateLimiter
	DryRun     dryrun.Config
	Truncation truncate.Config
}

func (p *Pipeline) Clone() *Pipeline {
	if p == nil {
		return nil
	}
	return &Pipeline{
		Policy:     p.Policy.Clone(),
		Bootstrap:  p.Bootstrap.Clone(),
		RateLimit:  p.RateLimit,
		DryRun:     p.DryRun,
		Truncation: p.Truncation,
	}
}

// FilterTool reports whether a tool should be listed in tools/list.
func (p *Pipeline) FilterTool(name string, hints mcp.ToolHints) bool {
	if p.Bootstrap != nil && !p.Bootstrap.IsVisible(name) {
		return false
	}
	if p.Policy != nil && !p.Policy.IsAllowed(name, hints.ReadOnly) {
		return false
	}
	return true
}

// BeforeCall runs the enforcement pipeline before a tool handler:
//  0. Schema validation (W2-01)
//  1. Policy gate
//  2. Rate limit acquire
//  3. Dry-run intercept
func (p *Pipeline) BeforeCall(ctx context.Context, name string, args map[string]any, hints mcp.ToolHints, schema map[string]any, lookupHandler func(string) (mcp.ToolHandler, bool)) (any, func(), error) {
	// 0. JSON-schema validation. Runs before the policy gate so malformed
	// calls never consume a rate-limit slot or trigger a dry-run preview.
	// A nil schema means the caller opted out (legacy tests); the absence
	// of validation is then indistinguishable from the pre-W2-01 behavior.
	if schema != nil {
		if err := jsonschema.Validate(schema, args); err != nil {
			var ve *jsonschema.ValidationError
			if errors.As(err, &ve) {
				return nil, nil, &mcp.InvalidParamsError{
					Pointer: ve.Pointer,
					Message: ve.Message,
				}
			}
			return nil, nil, &mcp.InvalidParamsError{Pointer: "", Message: err.Error()}
		}
	}

	// 1. Policy check
	if p.Policy != nil && !p.Policy.IsAllowed(name, hints.ReadOnly) {
		reason := p.Policy.BlockReason(name, hints.ReadOnly)
		return nil, nil, fmt.Errorf("tool blocked by policy: %s", reason)
	}

	// 2. Rate limit — per-subject when a Principal is on the context,
	// global-only fallback otherwise. Scope label distinguishes the two
	// rejection layers so dashboards can tell a noisy tenant from a global
	// saturation event.
	var release func()
	if p.RateLimit != nil {
		subject := ""
		if principal, ok := authn.PrincipalFromContext(ctx); ok && principal != nil {
			subject = principal.Subject
		}
		rel, scope, err := p.RateLimit.AcquireForSubject(ctx, subject)
		if err != nil {
			kind := "window"
			if errors.Is(err, ratelimit.ErrConcurrencyLimitExceeded) {
				kind = "concurrency"
			}
			scopeLabel := "global"
			if scope == ratelimit.ScopePerToken {
				scopeLabel = "per_token"
			}
			metrics.RateLimitRejections.Inc(kind, scopeLabel)
			return nil, nil, fmt.Errorf("rate limited: %w", err)
		}
		release = rel
	}

	// 3. Dry-run intercept (only when CLOCKIFY_DRY_RUN is enabled)
	if p.DryRun.Enabled {
		action, isDryRun := dryrun.CheckDryRun(name, args, hints.Destructive)
		if isDryRun {
			result, err := p.executeDryRun(ctx, action, name, args, hints, lookupHandler)
			if err != nil {
				if release != nil {
					release()
				}
				return nil, nil, err
			}
			return result, release, nil
		}
	}

	return nil, release, nil
}

// AfterCall applies post-processing (truncation) to a successful result.
//
// Tool handlers return typed structs (e.g. ResultEnvelope) which the truncate
// package's type switch can't walk. We first marshal once to estimate the
// response size; under-budget results return unchanged. Over-budget results
// are JSON-roundtripped into a generic map[string]any / []any tree before
// calling Truncate so the walker sees the whole structure.
//
// On marshal/unmarshal failure we fail open and return the original result
// unchanged — dropping a tool response because truncation misbehaved would be
// worse than returning an over-budget payload.
func (p *Pipeline) AfterCall(result any) (any, error) {
	if !p.Truncation.Enabled {
		return result, nil
	}
	b, err := json.Marshal(result)
	if err != nil {
		metrics.TruncationSkippedTotal.Inc("marshal_failed")
		slog.Warn("truncate_marshal_failed", "error", err.Error())
		return result, nil
	}
	if p.Truncation.TokenBudget > 0 && estimatedTokensFromJSONLen(len(b)) <= p.Truncation.TokenBudget {
		return result, nil
	}
	generic, ok := normalizeJSONTree(result)
	if !ok {
		if err := json.Unmarshal(b, &generic); err != nil {
			metrics.TruncationSkippedTotal.Inc("unmarshal_failed")
			slog.Warn("truncate_unmarshal_failed", "error", err.Error())
			return result, nil
		}
	}
	truncated, wasTruncated := p.Truncation.Truncate(generic)
	if wasTruncated {
		slog.Debug("response_truncated", "budget", p.Truncation.TokenBudget)
	}
	return truncated, nil
}

func normalizeJSONTree(v any) (any, bool) {
	if v == nil {
		return nil, true
	}
	return normalizeJSONValue(reflect.ValueOf(v))
}

func normalizeJSONValue(v reflect.Value) (any, bool) {
	if !v.IsValid() {
		return nil, true
	}
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil, true
		}
		v = v.Elem()
	}
	if v.CanInterface() {
		if marshaler, ok := v.Interface().(json.Marshaler); ok {
			b, err := marshaler.MarshalJSON()
			if err != nil {
				return nil, false
			}
			var out any
			if err := json.Unmarshal(b, &out); err != nil {
				return nil, false
			}
			return out, true
		}
	}

	switch v.Kind() {
	case reflect.Bool:
		return v.Bool(), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return float64(v.Uint()), true
	case reflect.Float32, reflect.Float64:
		return v.Float(), true
	case reflect.String:
		return v.String(), true
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return nil, false
		}
		out := make(map[string]any, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			child, ok := normalizeJSONValue(iter.Value())
			if !ok {
				return nil, false
			}
			out[iter.Key().String()] = child
		}
		return out, true
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.IsNil() {
			return nil, true
		}
		out := make([]any, v.Len())
		for i := 0; i < v.Len(); i++ {
			child, ok := normalizeJSONValue(v.Index(i))
			if !ok {
				return nil, false
			}
			out[i] = child
		}
		return out, true
	case reflect.Struct:
		out := make(map[string]any, v.NumField())
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			if field.PkgPath != "" {
				continue
			}
			name, omitEmpty, skip := jsonFieldName(field)
			if skip {
				continue
			}
			fv := v.Field(i)
			if omitEmpty && fv.IsZero() {
				continue
			}
			child, ok := normalizeJSONValue(fv)
			if !ok {
				return nil, false
			}
			out[name] = child
		}
		return out, true
	default:
		return nil, false
	}
}

func jsonFieldName(field reflect.StructField) (name string, omitEmpty bool, skip bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}
	name = field.Name
	if tag != "" {
		parts := strings.Split(tag, ",")
		if parts[0] != "" {
			name = parts[0]
		}
		for _, opt := range parts[1:] {
			if opt == "omitempty" {
				omitEmpty = true
				break
			}
		}
	}
	return name, omitEmpty, false
}

func estimatedTokensFromJSONLen(n int) int {
	return (n + 3) / 4
}

func (p *Pipeline) executeDryRun(ctx context.Context, action dryrun.Action, name string, args map[string]any, hints mcp.ToolHints, lookupHandler func(string) (mcp.ToolHandler, bool)) (any, error) {
	switch action {
	case dryrun.NotDestructive:
		return nil, dryrun.NotDestructiveError(name)
	case dryrun.ConfirmPattern:
		// ConfirmPattern uses minimal fallback — the tool is NOT executed.
		// This avoids the dangerous pattern of executing a mutation and then
		// claiming "No changes were made" in the dry-run envelope.
		return dryrun.MinimalResult(name, args), nil
	case dryrun.PreviewTool:
		previewTool, ok := dryrun.PreviewToolFor(name)
		if !ok {
			return dryrun.MinimalResult(name, args), nil
		}
		handler, ok := lookupHandler(previewTool)
		if !ok {
			return dryrun.MinimalResult(name, args), nil
		}
		previewArgs := dryrun.BuildPreviewArgs(args)
		result, err := handler(ctx, previewArgs)
		if err != nil {
			return nil, err
		}
		return dryrun.WrapResult(result, name), nil
	case dryrun.MinimalFallback:
		return dryrun.MinimalResult(name, args), nil
	default:
		return dryrun.MinimalResult(name, args), nil
	}
}

// Gate implements mcp.Activator using policy and bootstrap.
type Gate struct {
	Policy    *policy.Policy
	Bootstrap *bootstrap.Config
}

func (g *Gate) Clone() *Gate {
	if g == nil {
		return nil
	}
	return &Gate{
		Policy:    g.Policy.Clone(),
		Bootstrap: g.Bootstrap.Clone(),
	}
}

// IsGroupAllowed checks whether the policy permits activating a group.
func (g *Gate) IsGroupAllowed(group string) bool {
	if g.Policy != nil {
		return g.Policy.IsGroupAllowed(group)
	}
	return true
}

// OnActivate marks tools as visible in the bootstrap layer.
func (g *Gate) OnActivate(names []string) {
	if g.Bootstrap != nil {
		g.Bootstrap.ActivateTools(names)
	}
	slog.Debug("tools_activated", "count", len(names))
}

// OnDeactivate removes runtime visibility markers for dynamically
// registered tools.
func (g *Gate) OnDeactivate(names []string) {
	if g.Bootstrap != nil {
		g.Bootstrap.DeactivateTools(names)
	}
	slog.Debug("tools_deactivated", "count", len(names))
}
