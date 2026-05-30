package mcp

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AuditLogMode selects which tool invocations the AuditLogger records.
type AuditLogMode string

// Audit log modes: Off disables the logger, SideEffectsOnly records only
// write/high-risk tools, and All records every tool call.
const (
	AuditLogModeOff             AuditLogMode = "off"
	AuditLogModeSideEffectsOnly AuditLogMode = "side_effects_only"
	AuditLogModeAll             AuditLogMode = "all"
)

const maxAuditLogBytes int64 = 10 * 1024 * 1024

// AuditLogger appends JSON-lines audit records for tool invocations to a
// local file, rotating it once it exceeds maxAuditLogBytes. It records only a
// redacted workspace-ID suffix, never the full ID. A nil *AuditLogger is a
// valid no-op receiver.
type AuditLogger struct {
	mu                sync.Mutex
	path              string
	mode              AuditLogMode
	workspaceIDSuffix string
}

type auditRecord struct {
	Timestamp         string         `json:"ts"`
	Tool              string         `json:"tool"`
	Risk              []string       `json:"risk,omitempty"`
	WorkspaceIDSuffix string         `json:"workspaceIdSuffix,omitempty"`
	Arguments         map[string]any `json:"arguments,omitempty"`
	DryRun            bool           `json:"dryRun,omitempty"`
	Confirmation      map[string]any `json:"confirmation,omitempty"`
	Result            string         `json:"result"`
	ErrorCode         string         `json:"errorCode,omitempty"`
	Upstream          map[string]any `json:"upstream,omitempty"`
}

// NewAuditLogger returns an AuditLogger writing to path in the given mode,
// recording only a redacted suffix of workspaceID. It returns (nil, nil) when
// path is empty or mode is off, so callers can treat the result as an
// always-valid (possibly no-op) logger.
func NewAuditLogger(path string, mode AuditLogMode, workspaceID string) (*AuditLogger, error) {
	path = strings.TrimSpace(path)
	mode = normalizeAuditLogMode(mode)
	if path == "" || mode == AuditLogModeOff {
		return nil, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return &AuditLogger{
		path:              path,
		mode:              mode,
		workspaceIDSuffix: auditIDSuffix(workspaceID),
	}, nil
}

func normalizeAuditLogMode(mode AuditLogMode) AuditLogMode {
	switch mode {
	case AuditLogModeAll, AuditLogModeSideEffectsOnly:
		return mode
	default:
		return AuditLogModeOff
	}
}

// Close releases any resources held by the logger. The file-per-record design
// keeps no open handle, so Close is currently a no-op that satisfies io.Closer.
func (l *AuditLogger) Close() error {
	return nil
}

// Record writes an audit entry for a tool invocation with no confirmation
// metadata. It is a thin wrapper over RecordWithConfirmation.
func (l *AuditLogger) Record(descriptor ToolDescriptor, args map[string]any, result, errorCode string) error {
	return l.RecordWithConfirmation(descriptor, args, result, errorCode, nil)
}

// RecordWithConfirmation appends one audit record for the given tool, filtering
// by mode (side-effects-only skips non-write, non-high-risk tools). It captures
// only the descriptor's allow-listed AuditKeys from args, the dry-run flag, and
// any high-risk confirmation metadata. A nil logger or off mode is a no-op.
func (l *AuditLogger) RecordWithConfirmation(descriptor ToolDescriptor, args map[string]any, result, errorCode string, confirmation map[string]any) error {
	if l == nil || l.mode == AuditLogModeOff {
		return nil
	}
	if l.mode == AuditLogModeSideEffectsOnly && !descriptor.RiskClass.Has(RiskWrite) && !descriptor.RiskClass.IsHighRisk() {
		return nil
	}
	record := auditRecord{
		Timestamp:         time.Now().UTC().Format(time.RFC3339),
		Tool:              descriptor.Tool.Name,
		Risk:              riskClassNames(descriptor.RiskClass),
		WorkspaceIDSuffix: l.workspaceIDSuffix,
		Arguments:         auditArguments(descriptor.AuditKeys, args),
		DryRun:            boolAuditArg(args, "dry_run", "dryRun"),
		Confirmation:      auditConfirmation(confirmation),
		Result:            result,
		ErrorCode:         errorCode,
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.rotateIfNeededLocked(); err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	if err := enc.Encode(record); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func auditConfirmation(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		if key != "tokenSuffix" && (auditKeyBlocked(key) || auditValueBlocked(value)) {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (l *AuditLogger) rotateIfNeededLocked() error {
	info, err := os.Stat(l.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.Size() < maxAuditLogBytes {
		return nil
	}
	rotated := l.path + ".1"
	_ = os.Remove(rotated)
	return os.Rename(l.path, rotated)
}

func auditArguments(keys []string, args map[string]any) map[string]any {
	if len(keys) == 0 || len(args) == 0 {
		return nil
	}
	out := make(map[string]any, len(keys))
	for _, key := range keys {
		if auditKeyBlocked(key) {
			continue
		}
		if value, ok := args[key]; ok && !auditValueBlocked(value) {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func auditKeyBlocked(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	for _, needle := range []string{"api_key", "apikey", "authorization", "bearer", "email", "secret", "token", "webhook_secret"} {
		if strings.Contains(k, needle) {
			return true
		}
	}
	return false
}

func auditValueBlocked(value any) bool {
	s, ok := value.(string)
	if !ok {
		return false
	}
	return strings.Contains(s, "@")
}

func boolAuditArg(args map[string]any, keys ...string) bool {
	for _, key := range keys {
		if value, ok := args[key].(bool); ok {
			return value
		}
	}
	return false
}

func auditIDSuffix(id string) string {
	id = strings.TrimSpace(id)
	if len(id) < 4 {
		return ""
	}
	return "..." + id[len(id)-4:]
}

func riskClassNames(rc RiskClass) []string {
	names := []string{}
	for _, item := range []struct {
		bit  RiskClass
		name string
	}{
		{RiskRead, "read"},
		{RiskWrite, "write"},
		{RiskSensitiveRead, "sensitive_read"},
		{RiskBilling, "billing"},
		{RiskAdmin, "admin"},
		{RiskPermissionChange, "permission_change"},
		{RiskExternalSideEffect, "external_side_effect"},
		{RiskDestructive, "destructive"},
	} {
		if rc.Has(item.bit) {
			names = append(names, item.name)
		}
	}
	return names
}
