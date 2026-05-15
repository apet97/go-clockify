package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

func TestOneUserMCPTimeoutReleasesInFlightSlot(t *testing.T) {
	var userRequests int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			userRequests++
			if userRequests == 1 {
				<-r.Context().Done()
				return
			}
			respondJSON(t, w, map[string]any{"id": "user1", "name": "Test User"})
		default:
			t.Fatalf("unexpected upstream request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer upstream.Close()

	svc := New(clockify.NewClient("test-key", upstream.URL, 5*time.Second, 0), "65b382b606de527a7ee2b60e")
	server := mcp.NewServer("test", svc.FullAccessRegistry(), nil, nil)
	server.MaxInFlightToolCalls = 1
	server.ToolTimeout = 20 * time.Millisecond

	input := strings.NewReader(strings.Join([]string{
		`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"clockify_status","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"clockify_tools_guide","arguments":{}}}`,
	}, "\n"))

	var output lockedWriter
	runCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := server.Run(runCtx, input, &output); err != nil {
		t.Fatalf("server.Run: %v", err)
	}

	responses := decodeResponsesByID(t, output.String())
	timeoutResp := responses[float64(1)]
	if timeoutResp.Error != nil {
		t.Fatalf("timeout should be a tool-result envelope, got rpc error: %+v", timeoutResp.Error)
	}
	timeoutResult, ok := timeoutResp.Result.(map[string]any)
	if !ok {
		t.Fatalf("timeout result = %T, want object", timeoutResp.Result)
	}
	content, ok := timeoutResult["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("timeout result missing content: %+v", timeoutResult)
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	if first["type"] != "text" || strings.TrimSpace(text) == "" {
		t.Fatalf("timeout content shape = %+v", first)
	}
	structured, ok := timeoutResult["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("timeout result missing structuredContent: %+v", timeoutResult)
	}
	if structured["ok"] != false {
		t.Fatalf("timeout structuredContent should be ok:false, got %+v", structured)
	}
	if _, ok := structured["error"].(map[string]any); !ok {
		t.Fatalf("timeout structuredContent missing error object: %+v", structured)
	}
	if _, ok := structured["recovery"].(map[string]any); !ok {
		t.Fatalf("timeout structuredContent missing recovery object: %+v", structured)
	}

	nextResp := responses[float64(2)]
	if nextResp.Error != nil {
		t.Fatalf("subsequent call after timeout returned rpc error: %+v", nextResp.Error)
	}
	nextResult, ok := nextResp.Result.(map[string]any)
	if !ok {
		t.Fatalf("subsequent result = %T, want object", nextResp.Result)
	}
	if nextResult["isError"] == true {
		t.Fatalf("subsequent call was still blocked/error-shaped: %+v", nextResult)
	}
	if got := server.InFlightToolCalls(); got != 0 {
		t.Fatalf("in-flight semaphore depth after timeout cleanup = %d, want 0", got)
	}
}

type lockedWriter struct {
	mu sync.Mutex
	b  strings.Builder
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (w *lockedWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}

func decodeResponsesByID(t *testing.T, raw string) map[any]mcp.Response {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(raw))
	responses := map[any]mcp.Response{}
	for {
		var resp mcp.Response
		if err := dec.Decode(&resp); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("decode response stream: %v\n%s", err, raw)
		}
		if resp.ID != nil {
			responses[resp.ID] = resp
		}
	}
	for _, id := range []float64{1, 2} {
		if _, ok := responses[id]; !ok {
			t.Fatalf("missing response id %.0f in stream:\n%s", id, raw)
		}
	}
	return responses
}
