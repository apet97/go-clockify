// Package auditbridge converts mcp.AuditEvent records into the
// controlplane.AuditEvent shape the durable audit store accepts.
//
// The bridge owns one invariant the rest of the system depends on: the
// synthesised external_id must encode (sessionID, tool, phase, outcome)
// so the Postgres store's `ON CONFLICT (external_id) DO NOTHING` does
// NOT collapse the two-phase intent + outcome rows emitted in rapid
// succession. PhaseIntent and PhaseOutcome share the same (sessionID,
// tool) tuple at the same wall-clock nanosecond on a fast handler;
// without phase + outcome in the ID, the second INSERT is a no-op and
// the audit trail silently drops the outcome record.
//
// Centralising this here means the runtime auditor (production) and
// the live audit-phase contract test use the same conversion. A drift
// between them — historically: the live test reimplemented the runtime
// helper inline — is exactly the kind of regression that would let the
// Postgres-row collapse return without surfacing in unit coverage.
package auditbridge

import (
	"fmt"
	"time"

	"github.com/apet97/go-clockify/internal/controlplane"
	"github.com/apet97/go-clockify/internal/mcp"
)

// ToControlPlaneEvent converts an mcp.AuditEvent into the durable
// controlplane.AuditEvent shape, pinning the supplied wall-clock to
// both the synthesised ID and the At timestamp so intent/outcome rows
// for the same call land at the exact same nanosecond and are
// distinguished only by phase + outcome.
//
// Callers should pass `time.Now().UTC()` once per RecordAudit
// invocation (not once per field) so the two-phase pair stays
// nanosecond-aligned. Tests may pass a fixed time for reproducibility.
//
// Tenant / subject / session / transport are pulled from event.Metadata
// using the keys the mcp.Server populates — see audit.go's emitAudit.
// Missing keys yield empty strings, which the Postgres store accepts
// (the columns are NOT NULL but default to ” via migration 001).
func ToControlPlaneEvent(event mcp.AuditEvent, now time.Time) controlplane.AuditEvent {
	tenantID := event.Metadata["tenant_id"]
	subject := event.Metadata["subject"]
	sessionID := event.Metadata["session_id"]
	transport := event.Metadata["transport"]

	return controlplane.AuditEvent{
		ID:          fmt.Sprintf("%d-%s-%s-%s-%s", now.UnixNano(), sessionID, event.Tool, event.Phase, event.Outcome),
		At:          now,
		TenantID:    tenantID,
		Subject:     subject,
		SessionID:   sessionID,
		Transport:   transport,
		Tool:        event.Tool,
		Action:      event.Action,
		Outcome:     event.Outcome,
		Phase:       event.Phase,
		Reason:      event.Reason,
		ResourceIDs: event.ResourceIDs,
		Metadata:    event.Metadata,
	}
}
