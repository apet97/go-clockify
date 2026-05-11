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
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/bootstrap"
	"github.com/apet97/go-clockify/internal/confirmation"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/jsonschema"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/metrics"
	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/ratelimit"
	"github.com/apet97/go-clockify/internal/truncate"
)

// ErrConfirmationRequired is returned by BeforeCall when a high-risk
// tool call (per mcp.RiskClass.IsHighRisk()) arrives without a
// confirmation token. It is the "you should preview first" signal —
// not a bug, not a policy denial, just a request for the operator's
// agent to round-trip through dry_run:true to obtain a token. See
// docs/adr/0018-risk-class-confirmation-tokens.md.
var ErrConfirmationRequired = errors.New("confirmation token required")

// Pipeline implements mcp.Enforcement by composing the safety subsystems.
type Pipeline struct {
	Policy     *policy.Policy
	Bootstrap  *bootstrap.Config
	RateLimit  *ratelimit.RateLimiter
	DryRun     dryrun.Config
	Truncation truncate.Config
	// Confirmation gates non-dry-run high-risk tool calls behind an
	// HMAC token minted on the dry-run preview. nil disables the
	// gate entirely (development / opted-out deployments); the
	// runtime is responsible for emitting the deprecation/risk
	// warning when nil is observed under a hosted profile. See
	// docs/adr/0018-risk-class-confirmation-tokens.md.
	Confirmation *confirmation.Signer
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
//  2. Confirmation gate for high-risk non-dry-run calls (ADR 0018)
//  3. Rate limit acquire
//  4. Dry-run intercept / confirmation-token mint on high-risk dry-run
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

	// 2. Confirmation gate. Runs after policy (so a denied tool returns
	// the policy error rather than leaking "is this tool high-risk?"
	// via the confirmation prompt) and before rate limiting (so a
	// missing token does not consume a rate-limit slot). The gate is
	// inert for low-risk tools and when no signer is configured. On
	// non-dry-run high-risk calls without a valid token, we return
	// ErrConfirmationRequired (or an underlying confirmation error)
	// and the handler is never invoked. The dry-run path is handled
	// in the dry-run intercept below.
	dryRunRequested := dryrun.Enabled(args)
	highRisk := hints.RiskClass.IsHighRisk()
	if p.Confirmation != nil && highRisk && !dryRunRequested {
		if err := p.verifyConfirmationToken(ctx, name, args, hints); err != nil {
			return nil, nil, err
		}
	}

	// 3. Rate limit -- per tenant+subject when a Principal is on the
	// context, global-only fallback otherwise. Scope label distinguishes the
	// two rejection layers so dashboards can tell a noisy tenant from a
	// global saturation event.
	var release func()
	if p.RateLimit != nil {
		subject := ""
		if principal, ok := authn.PrincipalFromContext(ctx); ok && principal != nil {
			subject = rateLimitSubjectKey(principal)
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

	// 4. Dry-run intercept (only when CLOCKIFY_DRY_RUN is enabled).
	// For high-risk dry-run calls we additionally mint a confirmation
	// token and enrich the result. For high-risk dry-run calls on
	// non-destructive tools (the send_invoice / test_webhook case where
	// CheckDryRun passes the flag through to the handler), we
	// short-circuit here with a minimal envelope + token so the agent
	// always has a path to a token.
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
			if p.Confirmation != nil && highRisk {
				result = p.mintConfirmationEnvelope(ctx, result, name, args, hints)
			}
			return result, release, nil
		}
	}
	if p.Confirmation != nil && highRisk && dryRunRequested && !hints.Destructive {
		// Non-destructive high-risk dry-run: CheckDryRun left the flag
		// in args for the handler, but we cannot let the handler run
		// without first minting. Strip the flag here and produce a
		// minimal preview envelope wrapped with the confirmation
		// metadata. Operators that want a richer preview can call the
		// read counterpart of the tool explicitly.
		delete(args, "dry_run")
		minimal := dryrun.MinimalResult(name, args)
		envelope := p.mintConfirmationEnvelope(ctx, minimal, name, args, hints)
		return envelope, release, nil
	}

	return nil, release, nil
}

// verifyConfirmationToken extracts the confirmation_token from args,
// verifies it against the expected binding, and on success strips
// the token from args so the handler never observes it.
func (p *Pipeline) verifyConfirmationToken(ctx context.Context, name string, args map[string]any, hints mcp.ToolHints) error {
	rawToken, _ := args["confirmation_token"].(string)
	if rawToken == "" {
		return ErrConfirmationRequired
	}
	tenant, subject, session := bindingFromContext(ctx)
	if _, err := p.Confirmation.Verify(confirmation.VerifyInput{
		Tool:      name,
		ArgsHash:  confirmation.BuildArgumentFingerprint(args),
		RiskClass: uint32(hints.RiskClass),
		Tenant:    tenant,
		Subject:   subject,
		Session:   session,
		Token:     rawToken,
	}); err != nil {
		return fmt.Errorf("confirmation token rejected: %w", err)
	}
	delete(args, "confirmation_token")
	return nil
}

// mintConfirmationEnvelope wraps a dry-run preview result with the
// confirmation metadata clients need to execute the call: a non-empty
// confirmation_token, a confirmation_expires_at RFC3339 timestamp,
// the risk-class names that triggered the gate, and an instructional
// confirmation_note. Mint failures (rare — random nonce generation)
// fall back to returning the unwrapped preview with
// confirmation_required=false so the dry-run UX still works in
// degraded mode; the operator-facing log explains the regression.
func (p *Pipeline) mintConfirmationEnvelope(ctx context.Context, preview any, name string, args map[string]any, hints mcp.ToolHints) map[string]any {
	envelope := normalizeConfirmationEnvelope(preview)
	tenant, subject, session := bindingFromContext(ctx)
	token, expiresAt, err := p.Confirmation.Mint(confirmation.MintInput{
		Tool:      name,
		ArgsHash:  confirmation.BuildArgumentFingerprint(args),
		RiskClass: uint32(hints.RiskClass),
		Tenant:    tenant,
		Subject:   subject,
		Session:   session,
	})
	if err != nil {
		envelope["confirmation_required"] = false
		envelope["confirmation_error"] = "mint_failed"
		return envelope
	}
	envelope["confirmation_required"] = true
	envelope["confirmation_token"] = token
	envelope["confirmation_expires_at"] = expiresAt.UTC().Format(time.RFC3339)
	envelope["confirmation_risk_class"] = riskClassNames(hints.RiskClass)
	envelope["confirmation_note"] = "Re-submit the same arguments with dry_run:false and confirmation_token to execute."
	return envelope
}

// normalizeConfirmationEnvelope coerces the dry-run preview into a
// map[string]any so the confirmation_* fields can be added at the
// top level. Non-map previews (rare — executeDryRun always returns a
// map[string]any today) are nested under a `preview` key so the
// envelope shape stays stable for client consumers.
func normalizeConfirmationEnvelope(preview any) map[string]any {
	if m, ok := preview.(map[string]any); ok {
		// Caller owns preview; copy to avoid mutating the original
		// when the caller cached it.
		out := make(map[string]any, len(m)+5)
		for k, v := range m {
			out[k] = v
		}
		return out
	}
	return map[string]any{"preview": preview}
}

// bindingFromContext returns the tenant, subject, and session that
// scope the confirmation token to a single principal. Empty strings
// when no principal is on context (stdio, single-tenant CLI usage);
// the verifier requires byte-identical strings on both ends, so a
// dry-run minted without a principal verifies the same way later.
func bindingFromContext(ctx context.Context) (tenant, subject, session string) {
	principal, ok := authn.PrincipalFromContext(ctx)
	if !ok || principal == nil {
		return "", "", ""
	}
	return principal.TenantID, principal.Subject, principal.SessionID
}

// riskClassNames flattens a RiskClass bitmask into the lower-case
// strings already used by docs/policy/production-tool-scope.md and
// the annotations.riskClass surface (internal/tools/common.go). Used
// in the confirmation envelope so clients can render which classes
// triggered the gate without re-deriving from a bitmask integer.
func riskClassNames(rc mcp.RiskClass) []string {
	type entry struct {
		bit  mcp.RiskClass
		name string
	}
	all := []entry{
		{mcp.RiskRead, "read"},
		{mcp.RiskWrite, "write"},
		{mcp.RiskBilling, "billing"},
		{mcp.RiskAdmin, "admin"},
		{mcp.RiskPermissionChange, "permission_change"},
		{mcp.RiskExternalSideEffect, "external_side_effect"},
		{mcp.RiskDestructive, "destructive"},
	}
	out := make([]string, 0, 2)
	for _, e := range all {
		if rc.Has(e.bit) {
			out = append(out, e.name)
		}
	}
	return out
}

func rateLimitSubjectKey(principal *authn.Principal) string {
	if principal == nil || principal.Subject == "" {
		return ""
	}
	if principal.TenantID == "" {
		return principal.Subject
	}
	return principal.TenantID + "\x00" + principal.Subject
}

// AfterCall applies post-processing (truncation) to a successful result.
//
// Tool handlers return typed structs (e.g. ResultEnvelope) which the truncate
// package's type switch can't walk. We first marshal once to estimate the
// response size; under-budget results return unchanged. Over-budget results
// are JSON-roundtripped into a generic map[string]any / []any tree before
// calling Truncate so the walker sees the whole structure.
//
// On marshal/unmarshal failure, local/default deployments fail open and return
// the original result unchanged. Hosted profiles set FailClosedOnError so a
// truncation failure returns a tool error instead of sending an over-budget
// payload to the client.
func (p *Pipeline) AfterCall(result any) (any, error) {
	if !p.Truncation.Enabled {
		return result, nil
	}
	b, err := json.Marshal(result)
	if err != nil {
		return p.handleTruncationFailure("marshal_failed", err, result)
	}
	if p.Truncation.TokenBudget > 0 && estimatedTokensFromJSONLen(len(b)) <= p.Truncation.TokenBudget {
		return result, nil
	}
	generic, ok := normalizeJSONTree(result)
	if !ok {
		if err := json.Unmarshal(b, &generic); err != nil {
			return p.handleTruncationFailure("unmarshal_failed", err, result)
		}
	}
	truncated, wasTruncated := p.Truncation.Truncate(generic)
	if wasTruncated {
		slog.Debug("response_truncated", "budget", p.Truncation.TokenBudget)
	}
	return truncated, nil
}

func (p *Pipeline) handleTruncationFailure(reason string, err error, original any) (any, error) {
	metrics.TruncationSkippedTotal.Inc(reason)
	event := "truncate_" + reason
	if p.Truncation.FailClosedOnError {
		slog.Error(event, "error", err.Error(), "fail_closed", true)
		return nil, fmt.Errorf("response truncation %s: %w", strings.TrimSuffix(reason, "_failed"), err)
	}
	slog.Warn(event, "error", err.Error(), "fail_closed", false)
	return original, nil
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
