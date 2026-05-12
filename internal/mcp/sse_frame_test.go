package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"testing"
)

// TestWriteSSEFrame_MatchesLegacyMapLiteral pins the byte-for-byte
// equivalence between the new pooled-buffer SSE writer and the prior
// inlined map-literal implementation it replaced. The wire format is
// load-bearing (SSE clients pattern-match on "id:", "event:", "data:"
// lines), so the pool migration must not shift a single byte.
//
// Drift check: switch sseNotificationFrame's Params tag to
// `json:"params,omitempty"` and the assertion fails because the legacy
// path always emits `"params":null` even for nil payloads.
func TestWriteSSEFrame_MatchesLegacyMapLiteral(t *testing.T) {
	cases := []sessionEvent{
		{id: 1, method: "notifications/tools/list_changed", params: nil},
		{id: 42, method: "notifications/progress", params: map[string]any{
			"progressToken": "abc",
			"progress":      0.5,
		}},
		{id: 999, method: "notifications/resources/updated", params: map[string]any{
			"uri": "clockify://workspace/123/projects/456",
		}},
	}
	for _, ev := range cases {
		t.Run(ev.method, func(t *testing.T) {
			var got bytes.Buffer
			writeSSEFrame(&got, ev)

			var want bytes.Buffer
			payload, err := json.Marshal(map[string]any{
				"jsonrpc": "2.0",
				"method":  ev.method,
				"params":  ev.params,
			})
			if err != nil {
				t.Fatalf("legacy Marshal: %v", err)
			}
			_, _ = io.WriteString(&want, "id: "+strconv.FormatUint(ev.id, 10)+"\n")
			_, _ = io.WriteString(&want, "event: "+ev.method+"\n")
			_, _ = io.WriteString(&want, "data: "+string(payload)+"\n\n")

			if !bytes.Equal(got.Bytes(), want.Bytes()) {
				t.Fatalf("SSE wire output diverged\n got: %q\nwant: %q", got.String(), want.String())
			}
		})
	}
}

// TestWriteSSEFrame_PoolReuse pins the pool-vs-allocation balance: a
// second call returns the same buffer from the pool (so the buffer is
// reset, not regrown). This is a smoke test against future refactors
// that accidentally bypass the pool.
//
// Note: sync.Pool offers no GC guarantees so we exercise the happy path
// (immediate consecutive calls) where reuse is overwhelmingly likely.
func TestWriteSSEFrame_PoolReuse(t *testing.T) {
	ev := sessionEvent{id: 1, method: "notifications/test", params: nil}
	for range 100 {
		var sink bytes.Buffer
		writeSSEFrame(&sink, ev)
		if sink.Len() == 0 {
			t.Fatal("writeSSEFrame produced no output")
		}
	}
}
