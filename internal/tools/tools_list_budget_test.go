package tools

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

// toolsListByteBudget is the fixed cap for the serialized tools/list result.
// The full 156-tool registry must stay well under it.
const toolsListByteBudget = 512 * 1024

func TestToolsListPayloadWithinByteBudget(t *testing.T) {
	svc := New(clockify.NewClient("k", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	server := mcp.NewServer("test", svc.FullAccessRegistry())
	server.StaticToolList = true

	responses := runOneUserProtocol(t, server, []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	})

	raw, err := json.Marshal(responses[1])
	if err != nil {
		t.Fatalf("marshal tools/list response: %v", err)
	}
	if len(raw) >= toolsListByteBudget {
		t.Fatalf("tools/list payload is %d bytes, must stay under %d", len(raw), toolsListByteBudget)
	}
	if len(raw) >= 4*1024*1024 {
		t.Fatalf("tools/list payload is %d bytes, exceeds the 4 MiB default message size", len(raw))
	}
}
