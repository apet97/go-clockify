package mcp

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/apet97/go-clockify/internal/metrics"
)

// recordAuditBestEffort records an audit event and always returns nil.
// Persistence failures are logged and metered but never propagated to the
// caller. Use this for error-outcome calls (the mutation didn't succeed, so
// failing the response on audit failure would be doubly confusing).
func (s *Server) recordAuditBestEffort(tool, action, outcome, reason string, args map[string]any, hints ToolHints) {
	_ = s.emitAudit(tool, action, outcome, "", reason, args, hints)
}

// recordAuditIntent writes a pre-handler "we are about to call this
// tool" record. Audit phase = PhaseIntent. The return value is honoured
// at the call site: in fail_closed mode a non-nil error short-circuits
// the handler so the mutation never happens; in best_effort mode the
// caller logs and continues. Read-only tools and a nil Auditor are
// no-ops.
//
// Why a separate intent record matters: with a single post-handler
// audit, fail_closed delivers a vague "audit persistence failed"
// after the mutation has already taken effect. With an intent record,
// fail_closed actually prevents the mutation when the audit pipeline
// is broken.
func (s *Server) recordAuditIntent(tool, action string, args map[string]any, hints ToolHints) error {
	intentErr := s.emitAudit(tool, action, "attempted", PhaseIntent, "", args, hints)
	if intentErr != nil && s.AuditDurabilityMode == "fail_closed" {
		return fmt.Errorf("audit intent persistence failed; refusing to execute mutation: %w", intentErr)
	}
	return nil
}

// recordAuditOutcome writes the post-handler outcome (success/failure)
// record paired with an earlier recordAuditIntent. Audit phase =
// PhaseOutcome. The outcome record is best-effort even in fail_closed
// mode: the mutation has already happened, so failing the call here
// would only confuse the client. Operators rely on the slog
// audit_persist_failed event to detect outcome-record loss.
func (s *Server) recordAuditOutcome(tool, action, outcome, reason string, args map[string]any, hints ToolHints) {
	_ = s.emitAudit(tool, action, outcome, PhaseOutcome, reason, args, hints)
}

// emitAudit is the shared core: increments the attempt counter, calls the
// Auditor, and on failure increments the failure counter and logs a structured
// error. Returns the persistence error (or nil) so callers can act on it.
func (s *Server) emitAudit(tool, action, outcome string, phase AuditPhase, reason string, args map[string]any, hints ToolHints) error {
	if s.Auditor == nil || hints.ReadOnly {
		return nil
	}
	metrics.AuditEventsTotal.Inc()
	err := s.Auditor.RecordAudit(AuditEvent{
		Tool:        tool,
		Action:      action,
		Outcome:     outcome,
		Phase:       phase,
		Reason:      reason,
		ResourceIDs: resourceIDs(args),
		Metadata: map[string]string{
			"tenant_id":  s.AuditTenantID,
			"subject":    s.AuditSubject,
			"session_id": s.AuditSessionID,
			"transport":  s.AuditTransport,
		},
	})
	if err != nil {
		metrics.AuditFailuresTotal.Inc("persist_error")
		// audit_outcome is the canonical field operators filter on:
		//   "not_durable" → mutation happened, audit write failed (best_effort)
		//                  or returned to caller (fail_closed). See
		//                  docs/runbooks/audit-durability.md for recovery.
		slog.Error("audit_persist_failed",
			"tool", tool,
			"outcome", outcome,
			"phase", phase,
			"audit_outcome", "not_durable",
			"durability_mode", s.AuditDurabilityMode,
			"error", err.Error(),
		)
	}
	return err
}

// resourceIDs extracts resource identifiers from tool-call arguments
// for the audit record. The suffix check is case-sensitive on "_id"
// because every tool schema under internal/tools/ declares its
// identifier properties as snake_case lowercase (expense_id, entry_id,
// approval_id, …). A case-insensitive match was historically used but
// added strings.ToLower on every audit arg key for zero benefit —
// dropping it removes the per-key allocation from the hot path behind
// BenchmarkPipelineBeforeCall.
//
// A future tool schema introducing an UPPER_ID or camelCase_Id tag
// would silently lose that resource from audit coverage; the
// enforcement CI grep (and TestResourceIDs_LowercaseSuffixContract) is
// the gate against that regression.
func resourceIDs(args map[string]any) map[string]string {
	if len(args) == 0 {
		return nil
	}
	ids := map[string]string{}
	for k, v := range args {
		if !strings.HasSuffix(k, "_id") {
			continue
		}
		value, ok := v.(string)
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		ids[k] = value
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}
