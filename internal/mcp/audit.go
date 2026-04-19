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
	_ = s.emitAudit(tool, action, outcome, reason, args, hints)
}

// recordAuditWithDurability records an audit event and returns an error when
// AuditDurabilityMode is "fail_closed" and persistence fails. For read-only
// hints or when the auditor is nil it is always a no-op.
func (s *Server) recordAuditWithDurability(tool, action, outcome, reason string, args map[string]any, hints ToolHints) error {
	auditErr := s.emitAudit(tool, action, outcome, reason, args, hints)
	if auditErr != nil && s.AuditDurabilityMode == "fail_closed" {
		return fmt.Errorf("audit persistence failed; mutation outcome unverifiable: %w", auditErr)
	}
	return nil
}

// emitAudit is the shared core: increments the attempt counter, calls the
// Auditor, and on failure increments the failure counter and logs a structured
// error. Returns the persistence error (or nil) so callers can act on it.
func (s *Server) emitAudit(tool, action, outcome, reason string, args map[string]any, hints ToolHints) error {
	if s.Auditor == nil || hints.ReadOnly {
		return nil
	}
	metrics.AuditEventsTotal.Inc()
	err := s.Auditor.RecordAudit(AuditEvent{
		Tool:        tool,
		Action:      action,
		Outcome:     outcome,
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
		slog.Error("audit_persist_failed",
			"tool", tool,
			"outcome", outcome,
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
