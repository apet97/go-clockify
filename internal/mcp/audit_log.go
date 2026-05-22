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

type AuditLogMode string

const (
	AuditLogModeOff             AuditLogMode = "off"
	AuditLogModeSideEffectsOnly AuditLogMode = "side_effects_only"
	AuditLogModeAll             AuditLogMode = "all"
)

const maxAuditLogBytes int64 = 10 * 1024 * 1024

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
	Result            string         `json:"result"`
	ErrorCode         string         `json:"errorCode,omitempty"`
	Upstream          map[string]any `json:"upstream,omitempty"`
}

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

func (l *AuditLogger) Close() error {
	return nil
}

func (l *AuditLogger) Record(descriptor ToolDescriptor, args map[string]any, result, errorCode string) error {
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
