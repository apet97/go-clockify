package e2e_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/controlplane"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/tests/harness"
)

// TestSSE_ResumeReplaysBacklog exercises the streamable HTTP SSE
// Last-Event-ID resumability contract end-to-end:
//
//  1. Initialize a session and open the SSE stream.
//  2. Fire a notification from the server; read the event; remember its
//     id: header as lastID.
//  3. Close the SSE connection.
//  4. While no subscriber is attached, fire a second notification — it
//     queues in sessionEventHub's ring buffer.
//  5. Reopen the SSE stream with Last-Event-ID: lastID.
//  6. Assert the replay delivers the queued seq=2 event in order.
//  7. Fire a third notification and assert it flows through the newly
//     established live subscription.
//
// This closes a real gap: the parity suite exercises Notify fanout but
// not replay after a dropped connection, which is the whole point of
// Last-Event-ID on MCP Streamable HTTP (2025-03-26 §3.3).
func TestSSE_ResumeReplaysBacklog(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	srv, baseURL, bearer, sessionID, stop := startStreamableForTest(t, ctx)
	defer stop()

	// --- Phase 1: initial live delivery ---

	resp1 := openSSE(t, ctx, baseURL, bearer, sessionID, "")
	reader1 := bufio.NewReader(resp1.Body)

	if err := srv.Notify("notifications/resources/updated", map[string]any{"seq": 1}); err != nil {
		t.Fatalf("Notify seq=1: %v", err)
	}

	ev1 := readSSEEvent(t, reader1, 3*time.Second)
	if ev1.event != "notifications/resources/updated" {
		t.Fatalf("phase1: wrong event name %q (want notifications/resources/updated)", ev1.event)
	}
	if seq := ev1.seq(t); seq != 1 {
		t.Fatalf("phase1: seq=%d, want 1", seq)
	}
	lastID := ev1.id
	if lastID == "" {
		t.Fatalf("phase1: empty id — resume is impossible without it")
	}

	// Drop the SSE connection. The handler exits on r.Context().Done()
	// which the Close() below triggers via the http client.
	_ = resp1.Body.Close()
	// Small dwell so the server observes the disconnect and the session
	// hub has zero subscribers when the next Notify runs.
	time.Sleep(100 * time.Millisecond)

	// --- Phase 2: notification with NO subscriber attached ---

	if err := srv.Notify("notifications/resources/updated", map[string]any{"seq": 2}); err != nil {
		t.Fatalf("Notify seq=2: %v", err)
	}

	// --- Phase 3: resume with Last-Event-ID, expect the gap frame to replay ---

	resp2 := openSSE(t, ctx, baseURL, bearer, sessionID, lastID)
	defer func() { _ = resp2.Body.Close() }()
	reader2 := bufio.NewReader(resp2.Body)

	ev2 := readSSEEvent(t, reader2, 3*time.Second)
	if ev2.event != "notifications/resources/updated" {
		t.Fatalf("phase3: wrong event name %q", ev2.event)
	}
	if seq := ev2.seq(t); seq != 2 {
		t.Fatalf("phase3: expected replayed seq=2, got seq=%d (event id=%q, lastID=%q)",
			seq, ev2.id, lastID)
	}

	// --- Phase 4: live delivery resumes on the reconnected stream ---

	if err := srv.Notify("notifications/resources/updated", map[string]any{"seq": 3}); err != nil {
		t.Fatalf("Notify seq=3: %v", err)
	}
	ev3 := readSSEEvent(t, reader2, 3*time.Second)
	if seq := ev3.seq(t); seq != 3 {
		t.Fatalf("phase4: expected live seq=3, got seq=%d", seq)
	}
}

// --- helpers ---

// startStreamableForTest boots a streamable HTTP server on an ephemeral
// port, initializes a session, and returns the pieces the test needs:
// the shared *mcp.Server (for srv.Notify), the base URL, the bearer
// token, the established session ID, and a stop function.
func startStreamableForTest(t *testing.T, ctx context.Context) (*mcp.Server, string, string, string, func()) {
	t.Helper()
	srv := mcp.NewServer("sse-resume-test", nil, nil, nil)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	baseURL := "http://" + ln.Addr().String()
	bearer := strings.Repeat("r", 32)

	store, err := controlplane.Open("memory")
	if err != nil {
		_ = ln.Close()
		t.Fatalf("controlplane open: %v", err)
	}
	auth, err := authn.New(authn.Config{Mode: authn.ModeStaticBearer, BearerToken: bearer})
	if err != nil {
		_ = ln.Close()
		t.Fatalf("authn: %v", err)
	}

	runCtx, runCancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		_ = mcp.ServeStreamableHTTP(runCtx, mcp.StreamableHTTPOptions{
			Listener:      ln,
			MaxBodySize:   4194304,
			Authenticator: auth,
			ControlPlane:  store,
			SessionTTL:    5 * time.Minute,
			Factory: func(_ context.Context, _ authn.Principal, _ string) (*mcp.StreamableSessionRuntime, error) {
				return &mcp.StreamableSessionRuntime{Server: srv}, nil
			},
		})
		close(done)
	}()

	if err := harness.WaitForHTTP200(ctx, baseURL+"/health"); err != nil {
		runCancel()
		_ = ln.Close()
		t.Fatalf("health: %v", err)
	}

	// Initialize to establish the session.
	initBody, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "sse-resume-test", "version": "test"},
		},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/mcp", bytes.NewReader(initBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		runCancel()
		_ = ln.Close()
		t.Fatalf("initialize: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	sessionID := resp.Header.Get(mcp.MCPSessionIDHeader)
	if sessionID == "" {
		runCancel()
		_ = ln.Close()
		t.Fatalf("initialize: no %s header", mcp.MCPSessionIDHeader)
	}

	stop := func() {
		runCancel()
		_ = ln.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}
	return srv, baseURL, bearer, sessionID, stop
}

// openSSE opens a GET /mcp subscription. lastEventID == "" skips the
// Last-Event-ID header (fresh subscribe); otherwise it's sent verbatim.
// The http client has no timeout — SSE is long-lived and the test
// controls lifetime via the ctx and the response body Close.
func openSSE(t *testing.T, ctx context.Context, baseURL, bearer, sessionID, lastEventID string) *http.Response {
	t.Helper()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/mcp", nil)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set(mcp.MCPSessionIDHeader, sessionID)
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("openSSE: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("openSSE: status %d body=%s", resp.StatusCode, string(body))
	}
	return resp
}

// sseEvent captures the fields of one SSE frame the test cares about.
type sseEvent struct {
	id    string
	event string
	data  []byte
}

// seq decodes data as {"jsonrpc":"2.0","method":…,"params":{"seq":N,…}}
// and returns params.seq. Fails the test if the shape is wrong.
func (e sseEvent) seq(t *testing.T) int {
	t.Helper()
	var frame struct {
		Params struct {
			Seq int `json:"seq"`
		} `json:"params"`
	}
	if err := json.Unmarshal(e.data, &frame); err != nil {
		t.Fatalf("decode sse data %q: %v", string(e.data), err)
	}
	return frame.Params.Seq
}

// readSSEEvent consumes one complete `id:` / `event:` / `data:` frame
// from r and returns it. Keepalive comments (":") are skipped. Blank
// lines flush the current frame. Returns on the first flushed frame
// that carries data.
func readSSEEvent(t *testing.T, r *bufio.Reader, timeout time.Duration) sseEvent {
	t.Helper()
	done := make(chan sseEvent, 1)
	errCh := make(chan error, 1)
	go func() {
		var cur sseEvent
		var dataBuf bytes.Buffer
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				errCh <- err
				return
			}
			line = strings.TrimRight(line, "\r\n")
			switch {
			case line == "":
				if dataBuf.Len() == 0 && cur.id == "" && cur.event == "" {
					continue // blank line before any fields — skip
				}
				cur.data = append([]byte(nil), bytes.TrimRight(dataBuf.Bytes(), "\n")...)
				done <- cur
				return
			case strings.HasPrefix(line, ":"):
				// keepalive / session marker
			case strings.HasPrefix(line, "id:"):
				cur.id = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
			case strings.HasPrefix(line, "event:"):
				cur.event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				dataBuf.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
				dataBuf.WriteByte('\n')
			}
		}
	}()
	select {
	case ev := <-done:
		return ev
	case err := <-errCh:
		t.Fatalf("SSE read error: %v", err)
	case <-time.After(timeout):
		t.Fatalf("timed out waiting %s for SSE frame", timeout)
	}
	return sseEvent{}
}
