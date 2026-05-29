package tools

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestUpdateDemoResourceEmitsListChanged(t *testing.T) {
	var count int
	svc := &Service{
		EmitResourceListChanged: func() { count++ },
	}

	svc.updateDemoResource("run-a", "prefix", "seeded", ToolResult{})
	if count != 1 {
		t.Fatalf("fresh run count=%d want 1", count)
	}

	svc.updateDemoResource("run-a", "prefix", "seeded", ToolResult{})
	if count != 1 {
		t.Fatalf("repeat run count=%d want 1", count)
	}

	svc.updateDemoResource(defaultDemoResourceRunID, "prefix", "seeded", ToolResult{})
	if count != 1 {
		t.Fatalf("default run count=%d want 1", count)
	}
}

func TestAuditTailText_EmptyWhenUnconfigured(t *testing.T) {
	svc := &Service{}
	text, err := svc.auditTailText()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if text != "" {
		t.Fatalf("expected empty text, got %q", text)
	}
}

func TestAuditTailText_EmptyWhenMissing(t *testing.T) {
	dir := t.TempDir()
	svc := &Service{AuditLogPath: filepath.Join(dir, "does-not-exist.jsonl")}
	text, err := svc.auditTailText()
	if err != nil {
		t.Fatalf("expected nil err for missing file, got: %v", err)
	}
	if text != "" {
		t.Fatalf("expected empty text, got %q", text)
	}
}

func TestAuditTailText_ReturnsContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	body := "{\"event\":\"one\"}\n{\"event\":\"two\"}\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write audit log: %v", err)
	}
	svc := &Service{AuditLogPath: path}
	text, err := svc.auditTailText()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if text != body {
		t.Fatalf("text = %q, want %q", text, body)
	}
}

// TestAuditTailText_WrapsIOErrorAsGeneric pins the security-relevant contract
// from the 2026-05-29 audit: when ReadFile fails with anything other than
// IsNotExist, the surface error must be the generic sentinel — the raw
// filesystem error is logged via slog (redactor-covered) but never returned
// to the MCP client.
func TestAuditTailText_WrapsIOErrorAsGeneric(t *testing.T) {
	// Point AuditLogPath at a directory; os.ReadFile returns EISDIR which is
	// not IsNotExist and would otherwise leak as "read /tmp/...: is a directory".
	dir := t.TempDir()
	svc := &Service{AuditLogPath: dir}
	text, err := svc.auditTailText()
	if err == nil {
		t.Fatalf("expected error, got nil (text=%q)", text)
	}
	if !errors.Is(err, errAuditTailUnavailable) {
		t.Fatalf("expected errAuditTailUnavailable, got %T: %v", err, err)
	}
	if text != "" {
		t.Fatalf("expected empty text on error, got %q", text)
	}
	// Ensure no raw filesystem detail leaks into the surface message.
	msg := err.Error()
	if got := msg; got == "" {
		t.Fatalf("error message must be non-empty")
	}
	for _, leak := range []string{dir, "is a directory", "permission denied", "EISDIR"} {
		if leak != "" && strings.Contains(msg, leak) {
			t.Fatalf("surface error message %q leaks detail %q", msg, leak)
		}
	}
}

func TestDemoResourcesListIsCappedAt50(t *testing.T) {
	svc := New(clockify.NewClient("k", "http://127.0.0.1:1", time.Second, 0), "ws1")
	for i := range 70 {
		svc.updateDemoResource(fmt.Sprintf("run-%03d", i), "p", "ok", ToolResult{})
	}
	got := len(svc.demoResourcesList())
	if got > maxDemoResourcesListed {
		t.Fatalf("demoResourcesList returned %d resources, want <= %d", got, maxDemoResourcesListed)
	}
	if got != maxDemoResourcesListed {
		t.Fatalf("demoResourcesList returned %d resources, want exactly %d", got, maxDemoResourcesListed)
	}
}
