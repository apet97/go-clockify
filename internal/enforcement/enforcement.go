// Package enforcement provides the concrete Enforcement and Activator
// implementations that compose the safety subsystems (policy, rate limiting,
// dry-run, truncation, bootstrap) into the MCP server's pluggable interfaces.
//
// This package sits between the protocol core (mcp) and the domain-specific
// safety packages, keeping both layers decoupled.
package enforcement

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/apet97/go-clockify/internal/bootstrap"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
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
//  1. Policy gate
//  2. Rate limit acquire
//  3. Dry-run intercept
func (p *Pipeline) BeforeCall(ctx context.Context, name string, args map[string]any, hints mcp.ToolHints, lookupHandler func(string) (mcp.ToolHandler, bool)) (any, func(), error) {
	// 1. Policy check
	if p.Policy != nil && !p.Policy.IsAllowed(name, hints.ReadOnly) {
		reason := p.Policy.BlockReason(name, hints.ReadOnly)
		return nil, nil, fmt.Errorf("tool blocked by policy: %s", reason)
	}

	// 2. Rate limit
	var release func()
	if p.RateLimit != nil {
		rel, err := p.RateLimit.Acquire(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("rate limited: %s", err)
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
func (p *Pipeline) AfterCall(result any) (any, error) {
	if p.Truncation.Enabled {
		truncated, _ := p.Truncation.Truncate(result)
		return truncated, nil
	}
	return result, nil
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
