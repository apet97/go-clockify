package e2e_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/tests/harness"
)

// TestListChanged_ParityAcrossTransports fires a server-initiated
// notifications/tools/list_changed via the harness's underlying
// mcp.Server (reached through the optional SharedServer interface)
// and verifies every notification-capable transport delivers it on
// its Notifications() channel.
//
// Legacy HTTP is excluded by design: its POST-only request/response
// model has no server→client stream, so the server drops list_changed
// via droppingNotifier. Clients of legacy HTTP must re-poll tools/list
// on their own schedule. gRPC is excluded for the same reason as
// cancellation — the Exchange loop serialises dispatch.
func TestListChanged_ParityAcrossTransports(t *testing.T) {
	cases := map[string]harness.Factory{
		"stdio":           harness.NewStdio,
		"streamable_http": harness.NewStreamable,
	}

	for name, factory := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			h, err := factory(ctx, harness.Options{
				BearerToken: strings.Repeat("n", 16),
			})
			if err != nil {
				if errors.Is(err, harness.ErrGRPCUnavailable) {
					t.Skip("gRPC harness unavailable")
				}
				t.Fatalf("factory: %v", err)
			}
			defer func() { _ = h.Close() }()

			if _, err := h.Initialize(ctx); err != nil {
				t.Fatalf("initialize: %v", err)
			}

			// Fire list_changed directly through the server's notifier
			// hub. This validates delivery fanout — request/response
			// flows are already covered by parity_test.go.
			sharer, ok := h.(harness.ServerSharer)
			if !ok {
				t.Fatalf("%s: transport does not expose SharedServer", h.Name())
			}
			srv, ok := sharer.SharedServer()
			if !ok {
				t.Fatalf("%s: SharedServer returned !ok", h.Name())
			}
			if err := srv.Notify("notifications/tools/list_changed", map[string]any{}); err != nil {
				t.Fatalf("%s Notify: %v", h.Name(), err)
			}

			notifs := h.Notifications()
			if notifs == nil {
				t.Fatalf("%s: no Notifications() channel", h.Name())
			}
			deadline := time.After(2 * time.Second)
			for {
				select {
				case n := <-notifs:
					if n.Method != "notifications/tools/list_changed" {
						// Ignore unrelated frames (e.g. keepalive or
						// other notifications).
						continue
					}
					if len(n.Params) > 0 {
						var m map[string]any
						if err := json.Unmarshal(n.Params, &m); err != nil {
							t.Fatalf("%s: list_changed params not an object: %v raw=%s", h.Name(), err, string(n.Params))
						}
					}
					return
				case <-deadline:
					t.Fatalf("%s: notifications/tools/list_changed never delivered within 2s", h.Name())
				}
			}
		})
	}
}
