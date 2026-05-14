//go:build legacy_platform

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

// TestListChanged_ParityAcrossTransports fires server-initiated list_changed
// notifications for tools, resources, and prompts via the harness's underlying
// mcp.Server (reached through the optional SharedServer interface)
// and verifies every notification-capable transport delivers them on
// its Notifications() channel.
//
// Legacy HTTP is excluded by design: its POST-only request/response
// model has no server→client stream, so the server drops list_changed frames
// via droppingNotifier. Clients of legacy HTTP must re-poll list endpoints on
// their own schedule.
func TestListChanged_ParityAcrossTransports(t *testing.T) {
	cases := map[string]harness.Factory{
		"stdio":           harness.NewStdio,
		"streamable_http": harness.NewStreamable,
		"grpc":            harness.NewGRPC,
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
			want := map[string]bool{
				"notifications/tools/list_changed":     false,
				"notifications/resources/list_changed": false,
				"notifications/prompts/list_changed":   false,
			}
			allSeen := func() bool {
				for _, seen := range want {
					if !seen {
						return false
					}
				}
				return true
			}
			for method := range want {
				if err := srv.Notify(method, map[string]any{}); err != nil {
					t.Fatalf("%s Notify(%s): %v", h.Name(), method, err)
				}
			}

			notifs := h.Notifications()
			if notifs == nil {
				t.Fatalf("%s: no Notifications() channel", h.Name())
			}
			deadline := time.After(2 * time.Second)
			for {
				select {
				case n := <-notifs:
					if _, ok := want[n.Method]; !ok {
						// Ignore unrelated frames (e.g. keepalive or
						// other notifications).
						continue
					}
					if len(n.Params) > 0 {
						var m map[string]any
						if err := json.Unmarshal(n.Params, &m); err != nil {
							t.Fatalf("%s: %s params not an object: %v raw=%s", h.Name(), n.Method, err, string(n.Params))
						}
					}
					want[n.Method] = true
					if allSeen() {
						return
					}
				case <-deadline:
					var missing []string
					for method, seen := range want {
						if !seen {
							missing = append(missing, method)
						}
					}
					t.Fatalf("%s: list_changed notifications never delivered within 2s: %s", h.Name(), strings.Join(missing, ", "))
				}
			}
		})
	}
}
