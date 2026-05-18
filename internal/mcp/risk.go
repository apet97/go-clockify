package mcp

// RiskClass categorises tools beyond the three MCP boolean hints. It is a
// bitmask so a tool can carry multiple attributes (for example a billing
// action that also triggers an external side effect). Consumers can
// pattern-match against bits for logging, filtering, or UX hints.
type RiskClass uint32

const (
	RiskRead               RiskClass = 1 << iota // safe, idempotent reads
	RiskWrite                                    // ordinary mutating writes
	RiskSensitiveRead                            // reads users, invoices, balances, webhooks, or similar sensitive state
	RiskBilling                                  // touches invoices / payments
	RiskAdmin                                    // workspace-admin scope (deactivate, group/user mgmt)
	RiskPermissionChange                         // role / permission changes
	RiskExternalSideEffect                       // triggers outbound delivery (email, webhook test)
	RiskDestructive                              // irreversible delete-style operations
)

// Has reports whether the receiver carries every bit in mask.
func (r RiskClass) Has(mask RiskClass) bool { return r&mask == mask }

// RiskHighMask is retained as metadata for clients that want to visually
// distinguish billing, admin, external-side-effect, and destructive tools.
const RiskHighMask = RiskBilling | RiskAdmin | RiskPermissionChange | RiskExternalSideEffect | RiskDestructive

// IsHighRisk reports whether any high-risk metadata bit is set.
func (r RiskClass) IsHighRisk() bool { return r&RiskHighMask != 0 }
