package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestAuditLoggerRecordsSideEffectToolCall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	logger, err := NewAuditLogger(path, AuditLogModeSideEffectsOnly, "000000000000000000000001")
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	called := false
	server := NewServer("test", []ToolDescriptor{{
		Tool: Tool{
			Name:        "write_tool",
			Description: "write",
			InputSchema: map[string]any{"type": "object"},
		},
		RiskClass: RiskWrite | RiskBilling,
		AuditKeys: []string{"invoice_id", "amount", "email", "api_key"},
		Handler: func(context.Context, map[string]any) (any, error) {
			called = true
			return map[string]any{"ok": true, "action": "write_tool"}, nil
		},
	}})
	server.AuditLogger = logger
	initializeServerForAudit(t, server)

	raw, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write_tool","arguments":{"invoice_id":"inv-1","amount":12500,"email":"user@example.test","api_key":"secret","dry_run":false}}}`))
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
	if len(raw) == 0 {
		t.Fatal("missing call response")
	}

	records := readAuditRecords(t, path)
	if len(records) != 1 {
		t.Fatalf("records=%d want 1: %#v", len(records), records)
	}
	record := records[0]
	if record["tool"] != "write_tool" || record["result"] != "success" {
		t.Fatalf("unexpected audit record: %#v", record)
	}
	if record["workspaceIdSuffix"] != "...0001" {
		t.Fatalf("workspaceIdSuffix = %v", record["workspaceIdSuffix"])
	}
	args, ok := record["arguments"].(map[string]any)
	if !ok {
		t.Fatalf("arguments missing: %#v", record)
	}
	if args["invoice_id"] != "inv-1" || args["amount"] != float64(12500) {
		t.Fatalf("allowed audit args not captured: %#v", args)
	}
	if _, ok := args["api_key"]; ok {
		t.Fatalf("secret audit arg was captured: %#v", args)
	}
	if _, ok := args["email"]; ok {
		t.Fatalf("email audit arg was captured: %#v", args)
	}
}

func TestAuditLoggerModeControlsReadCalls(t *testing.T) {
	readTool := ToolDescriptor{
		Tool: Tool{
			Name:        "read_tool",
			Description: "read",
			InputSchema: map[string]any{"type": "object"},
		},
		RiskClass: RiskRead,
		AuditKeys: []string{"id"},
		Handler: func(context.Context, map[string]any) (any, error) {
			return map[string]any{"ok": true, "action": "read_tool"}, nil
		},
	}

	sideEffectsPath := filepath.Join(t.TempDir(), "side-effects.jsonl")
	sideEffectsLogger, err := NewAuditLogger(sideEffectsPath, AuditLogModeSideEffectsOnly, "000000000000000000000001")
	if err != nil {
		t.Fatal(err)
	}
	defer sideEffectsLogger.Close()
	sideEffectsServer := NewServer("test", []ToolDescriptor{readTool})
	sideEffectsServer.AuditLogger = sideEffectsLogger
	initializeServerForAudit(t, sideEffectsServer)
	if _, err := sideEffectsServer.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"read_tool","arguments":{"id":"r1"}}}`)); err != nil {
		t.Fatal(err)
	}
	if records := readAuditRecords(t, sideEffectsPath); len(records) != 0 {
		t.Fatalf("side_effects_only logged read call: %#v", records)
	}

	allPath := filepath.Join(t.TempDir(), "all.jsonl")
	allLogger, err := NewAuditLogger(allPath, AuditLogModeAll, "000000000000000000000001")
	if err != nil {
		t.Fatal(err)
	}
	defer allLogger.Close()
	allServer := NewServer("test", []ToolDescriptor{readTool})
	allServer.AuditLogger = allLogger
	initializeServerForAudit(t, allServer)
	if _, err := allServer.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"read_tool","arguments":{"id":"r1"}}}`)); err != nil {
		t.Fatal(err)
	}
	if records := readAuditRecords(t, allPath); len(records) != 1 {
		t.Fatalf("all mode records=%d want 1: %#v", len(records), records)
	}
}

func initializeServerForAudit(t *testing.T, server *Server) {
	t.Helper()
	raw, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	if err != nil {
		t.Fatalf("initialize dispatch: %v", err)
	}
	var resp Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal initialize: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("initialize error: %+v", resp.Error)
	}
}

func readAuditRecords(t *testing.T, path string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	var out []map[string]any
	for {
		var record map[string]any
		if err := dec.Decode(&record); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("decode audit record: %v\n%s", err, raw)
		}
		out = append(out, record)
	}
	return out
}
