package runtime_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/config"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/runtime"
	"github.com/apet97/go-clockify/internal/testclockify"
)

// fakeConfig returns a one-user config pointed at an in-process fake Clockify
// server (a loopback host the config allowlist permits) so the builder can be
// exercised with no live credentials.
func fakeConfig(t *testing.T, toolset string) config.OneUserConfig {
	t.Helper()
	fake := testclockify.NewServer("00000000000000000000abcd")
	t.Cleanup(fake.Close)
	return config.OneUserConfig{
		APIKey:               "fake-key",
		WorkspaceID:          fake.WorkspaceID,
		BaseURL:              fake.URL,
		Toolset:              toolset,
		ToolTimeout:          config.DefaultToolTimeout,
		MaxInFlightToolCalls: config.DefaultMaxInFlightToolCalls,
		MaxMessageSize:       config.DefaultMaxMessageSize,
		MaxToolResultBytes:   config.DefaultMaxToolResultBytes,
	}
}

// runRequests feeds the JSON-RPC lines through the built server's stdio Run
// loop (closing stdin so Run flushes and exits) and returns every decoded
// response. This drives the exact production server object the binary serves
// rather than reaching into unexported handlers.
func runRequests(t *testing.T, built *runtime.BuiltServer, lines ...map[string]any) []mcp.Response {
	t.Helper()
	var input bytes.Buffer
	for _, line := range lines {
		b, err := json.Marshal(line)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		input.Write(b)
		input.WriteByte('\n')
	}
	var output bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := built.Server.Run(ctx, &input, &output); err != nil && err != io.EOF {
		t.Fatalf("Server.Run: %v", err)
	}
	dec := json.NewDecoder(&output)
	var responses []mcp.Response
	for {
		var resp mcp.Response
		if err := dec.Decode(&resp); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("decode response: %v", err)
		}
		responses = append(responses, resp)
	}
	return responses
}

func responseByID(t *testing.T, responses []mcp.Response, id int) mcp.Response {
	t.Helper()
	for _, resp := range responses {
		if n, ok := numericResponseID(resp.ID); ok && n == id {
			return resp
		}
	}
	t.Fatalf("no response found for id %d (got %d responses)", id, len(responses))
	return mcp.Response{}
}

func numericResponseID(id any) (int, bool) {
	switch v := id.(type) {
	case float64:
		return int(v), true
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	case int:
		return v, true
	}
	return 0, false
}

func initializeLine(id int) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": mcp.SupportedProtocolVersions[0],
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "runtime-test", "version": "0"},
		},
	}
}

func toolsListNames(t *testing.T, resp mcp.Response) ([]string, string) {
	t.Helper()
	raw, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal tools/list result: %v", err)
	}
	var payload struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
		NextCursor string `json:"nextCursor"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal tools/list result: %v", err)
	}
	names := make([]string, 0, len(payload.Tools))
	for _, tool := range payload.Tools {
		names = append(names, tool.Name)
	}
	return names, payload.NextCursor
}

// TestBuildServerDefaultToolsetAdvertisesExactlySixteen proves the production
// builder enforces the 16-tool advertised surface in the default toolset, just
// like cmd/clockify-mcp/main.go. It drives tools/list through the same server
// object the binary serves, so a regression that drops advertised-tool
// filtering (or changes the default surface size) fails here, not just in the
// shell smoke test. The 16-tool surface must fit one page (no nextCursor).
func TestBuildServerDefaultToolsetAdvertisesExactlySixteen(t *testing.T) {
	cfg := fakeConfig(t, "default")
	built, err := runtime.BuildServer(cfg, "test")
	if err != nil {
		t.Fatalf("BuildServer: %v", err)
	}
	defer func() { _ = built.Close() }()

	responses := runRequests(t, built,
		initializeLine(1),
		map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{}},
	)
	listResp := responseByID(t, responses, 2)
	if listResp.Error != nil {
		t.Fatalf("tools/list returned error: %v", listResp.Error)
	}
	names, nextCursor := toolsListNames(t, listResp)
	if nextCursor != "" {
		t.Fatalf("default toolset tools/list unexpectedly paginated (nextCursor=%q); the 16-tool surface must fit one page", nextCursor)
	}
	if len(names) != 16 {
		t.Fatalf("default toolset advertised %d tools, want exactly 16: %v", len(names), names)
	}
}

// TestBuildServerDefaultToolsetRejectsUnadvertisedCall confirms the built
// server enforces EnforceAdvertisedTools for the default toolset: a tools/call
// for a tool that is loaded in the full startup registry but not advertised in
// the default surface (clockify_clients_list) is rejected by the toolset gate,
// never dispatched.
func TestBuildServerDefaultToolsetRejectsUnadvertisedCall(t *testing.T) {
	cfg := fakeConfig(t, "default")
	built, err := runtime.BuildServer(cfg, "test")
	if err != nil {
		t.Fatalf("BuildServer: %v", err)
	}
	defer func() { _ = built.Close() }()

	responses := runRequests(t, built,
		initializeLine(1),
		map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{}},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      3,
			"method":  "tools/call",
			"params":  map[string]any{"name": "clockify_clients_list", "arguments": map[string]any{}},
		},
	)

	// Guard: clockify_clients_list must NOT be in the advertised surface, so
	// this asserts a real enforcement boundary rather than a happy-path call.
	names, _ := toolsListNames(t, responseByID(t, responses, 2))
	for _, name := range names {
		if name == "clockify_clients_list" {
			t.Fatalf("clockify_clients_list unexpectedly advertised in the default surface; pick a tool outside the default toolset")
		}
	}

	callResp := responseByID(t, responses, 3)
	if callResp.Error == nil {
		t.Fatalf("expected tools/call for unadvertised clockify_clients_list to be rejected, got result %+v", callResp.Result)
	}
	// The toolset gate (EnforceAdvertisedTools) maps an unadvertised name to an
	// UnknownToolError, which surfaces as a JSON-RPC -32602 "unknown tool: ...".
	if !strings.Contains(callResp.Error.Message, "unknown tool") ||
		!strings.Contains(callResp.Error.Message, "clockify_clients_list") {
		t.Fatalf("expected an unadvertised-tool rejection naming clockify_clients_list, got %q", callResp.Error.Message)
	}
}
