package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/controlplane"
	"github.com/apet97/go-clockify/internal/mcp"
)

func buildMockServer() *mcp.Server {
	tool := mcp.ToolDescriptor{
		Tool: mcp.Tool{
			Name:        "mock_tool",
			Description: "Mock tool for E2E",
			InputSchema: map[string]any{"type": "object"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]string{"status": "ok"}, nil
		},
	}
	server := mcp.NewServer("test-version", []mcp.ToolDescriptor{tool}, nil, nil)
	// Bypass initialization guard for simpler transport testing
	// In strict tests, we would send 'initialize' but doing it raw via JSON is fine.
	return server
}

type rpcReq struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int            `json:"id,omitempty"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

type rpcResp struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int            `json:"id,omitempty"`
	Result  map[string]any `json:"result,omitempty"`
	Error   map[string]any `json:"error,omitempty"`
}

func doJSONRPC(req rpcReq) []byte {
	b, _ := json.Marshal(req)
	return b
}

func TestE2EStdioLifecycle(t *testing.T) {
	server := buildMockServer()

	// Create pipe
	pr, pw := io.Pipe()
	var out bytes.Buffer

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run(ctx, pr, &out)
	}()

	// 1. Initialize
	pw.Write(doJSONRPC(rpcReq{JSONRPC: "2.0", ID: 1, Method: "initialize"}))
	pw.Write([]byte("\n"))

	// 2. tools/list
	pw.Write(doJSONRPC(rpcReq{JSONRPC: "2.0", ID: 2, Method: "tools/list"}))
	pw.Write([]byte("\n"))

	// 3. tools/call
	pw.Write(doJSONRPC(rpcReq{JSONRPC: "2.0", ID: 3, Method: "tools/call", Params: map[string]any{"name": "mock_tool"}}))
	pw.Write([]byte("\n"))

	time.Sleep(100 * time.Millisecond) // Let output buffer
	cancel()                           // Shutdown
	<-errCh

	output := out.String()
	if !strings.Contains(output, "protocolVersion") {
		t.Errorf("expected initialize response in output")
	}
	if !strings.Contains(output, "\"mock_tool\"") {
		t.Errorf("expected tools/list to contain mock_tool")
	}
	if !strings.Contains(output, "\"status\":\"ok\"") {
		t.Errorf("expected tools/call to return status:ok")
	}
}

func TestE2ELegacyHTTPLifecycle(t *testing.T) {
	server := buildMockServer()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		// Use a specific port for the legacy test
		errCh <- server.ServeHTTP(ctx, "127.0.0.1:28080", nil, "test-token", nil, true, 4194304, mcp.InlineMetricsOptions{})
	}()

	// Wait for server to start
	url := "http://127.0.0.1:28080/mcp"
	for i := 0; i < 10; i++ {
		time.Sleep(100 * time.Millisecond)
		resp, err := http.Get("http://127.0.0.1:28080/health")
		if err == nil && resp.StatusCode == 200 {
			break
		}
	}

	sendReq := func(req rpcReq) rpcResp {
		httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(doJSONRPC(req)))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer test-token")
		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			t.Fatalf("http post failed: %v", err)
		}
		defer resp.Body.Close()
		var res rpcResp
		json.NewDecoder(resp.Body).Decode(&res)
		return res
	}

	// 1. Initialize
	initRes := sendReq(rpcReq{JSONRPC: "2.0", ID: 1, Method: "initialize"})
	if initRes.Result["protocolVersion"] == nil {
		t.Fatal("expected initialize payload")
	}

	// 2. tools/list
	listRes := sendReq(rpcReq{JSONRPC: "2.0", ID: 2, Method: "tools/list"})
	if listRes.Result["tools"] == nil {
		t.Fatal("expected tools array")
	}

	// 3. tools/call
	callRes := sendReq(rpcReq{JSONRPC: "2.0", ID: 3, Method: "tools/call", Params: map[string]any{"name": "mock_tool"}})
	if callRes.Result["isError"] == true {
		t.Fatal("expected successful tool call")
	}

	cancel()
	<-errCh
}

func TestE2EStreamableHTTPLifecycle(t *testing.T) {
	server := buildMockServer()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		memStore, _ := controlplane.Open("memory")
		auth, _ := authn.New(authn.Config{Mode: authn.ModeStaticBearer, BearerToken: "test-token"})
		errCh <- mcp.ServeStreamableHTTP(ctx, mcp.StreamableHTTPOptions{
			Bind:          "127.0.0.1:28081",
			MaxBodySize:   4194304,
			ControlPlane:  memStore,
			Authenticator: auth,
			Factory: func(ctx context.Context, principal authn.Principal, id string) (*mcp.StreamableSessionRuntime, error) {
				return &mcp.StreamableSessionRuntime{Server: server}, nil
			},
		})
	}()

	// Wait for server to start
	url := "http://127.0.0.1:28081"
	for i := 0; i < 10; i++ {
		time.Sleep(100 * time.Millisecond)
		resp, err := http.Get(url + "/health")
		if err == nil && resp.StatusCode == 200 {
			break
		}
	}

	var sessionID string
	sendReq := func(req rpcReq) *http.Response {
		httpReq, _ := http.NewRequest("POST", url+"/mcp", bytes.NewReader(doJSONRPC(req)))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer test-token")
		if sessionID != "" {
			httpReq.Header.Set("X-MCP-Session-ID", sessionID)
		}
		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			t.Fatalf("http post failed: %v", err)
		}
		return resp
	}

	initResp := sendReq(rpcReq{JSONRPC: "2.0", ID: 1, Method: "initialize"})
	sessionID = initResp.Header.Get("X-MCP-Session-ID")
	if sessionID == "" {
		t.Fatalf("expected X-MCP-Session-ID on initialize")
	}
	initResp.Body.Close()

	// Connect SSE
	sseReq, _ := http.NewRequest("GET", url+"/mcp", nil)
	sseReq.Header.Set("Authorization", "Bearer test-token")
	sseReq.Header.Set("X-MCP-Session-ID", sessionID)
	sseResp, err := http.DefaultClient.Do(sseReq)
	if err != nil {
		t.Fatalf("sse connect failed: %v", err)
	}
	defer sseResp.Body.Close()

	// Send other requests
	sendReq(rpcReq{JSONRPC: "2.0", ID: 2, Method: "tools/list"}).Body.Close()
	sendReq(rpcReq{JSONRPC: "2.0", ID: 3, Method: "tools/call", Params: map[string]any{"name": "mock_tool"}}).Body.Close()

	cancel()
	<-errCh
}
