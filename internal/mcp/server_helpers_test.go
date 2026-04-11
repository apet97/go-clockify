package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestToolNameFromRequest covers the panic-recovery helper that extracts a
// tool name from a Request for log correlation.
func TestToolNameFromRequest(t *testing.T) {
	cases := []struct {
		name string
		req  Request
		want string
	}{
		{
			name: "non_tools_call_returns_method",
			req:  Request{Method: "tools/list"},
			want: "tools/list",
		},
		{
			name: "nil_params_returns_method",
			req:  Request{Method: "tools/call"},
			want: "tools/call",
		},
		{
			name: "good_params_returns_tool_name",
			req:  Request{Method: "tools/call", Params: map[string]any{"name": "clockify_log_time"}},
			want: "clockify_log_time",
		},
		{
			name: "missing_name_returns_unknown",
			req:  Request{Method: "tools/call", Params: map[string]any{"arguments": map[string]any{}}},
			want: "unknown",
		},
		{
			name: "wrong_type_params_returns_unknown",
			req:  Request{Method: "tools/call", Params: "not-a-map"},
			want: "unknown",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := toolNameFromRequest(tc.req); got != tc.want {
				t.Fatalf("toolNameFromRequest: got %q want %q", got, tc.want)
			}
		})
	}
}

// TestResourceIDs covers the audit helper that extracts every `*_id` field
// (case-insensitive) from a tool's argument map. Non-string and empty values
// are skipped; mismatched suffixes are ignored.
func TestResourceIDs(t *testing.T) {
	t.Run("nil_args_returns_nil", func(t *testing.T) {
		if got := resourceIDs(nil); got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
	})
	t.Run("empty_args_returns_nil", func(t *testing.T) {
		if got := resourceIDs(map[string]any{}); got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
	})
	t.Run("only_id_fields_present", func(t *testing.T) {
		args := map[string]any{
			"entry_id":    "abc123",
			"PROJECT_ID":  "proj-7",
			"description": "ignored",
			"tag":         "ignored",
			"empty_id":    "   ",
			"nonstring_id": 42,
		}
		got := resourceIDs(args)
		if len(got) != 2 {
			t.Fatalf("expected 2 ids, got %+v", got)
		}
		if got["entry_id"] != "abc123" || got["PROJECT_ID"] != "proj-7" {
			t.Fatalf("unexpected ids: %+v", got)
		}
	})
	t.Run("only_skipped_returns_nil", func(t *testing.T) {
		args := map[string]any{"empty_id": "  ", "nonstring_id": 42}
		if got := resourceIDs(args); got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
	})
}

// TestInFlightToolCalls covers both branches: nil semaphore returns 0,
// occupied semaphore returns the current depth.
func TestInFlightToolCalls(t *testing.T) {
	s := NewServer("test", nil, nil, nil)
	if got := s.InFlightToolCalls(); got != 0 {
		t.Fatalf("nil sem: got %d want 0", got)
	}
	s.toolCallSem = make(chan struct{}, 4)
	s.toolCallSem <- struct{}{}
	s.toolCallSem <- struct{}{}
	if got := s.InFlightToolCalls(); got != 2 {
		t.Fatalf("got %d want 2", got)
	}
}

// TestIsReadyCached exercises the readiness cache accessor.
func TestIsReadyCached(t *testing.T) {
	s := NewServer("test", nil, nil, nil)
	if s.IsReadyCached() {
		t.Fatal("expected false on fresh server")
	}
	s.readyMu.Lock()
	s.readyCached = true
	s.readyAt = time.Now()
	s.readyMu.Unlock()
	if !s.IsReadyCached() {
		t.Fatal("expected true after setting cache")
	}
}

// TestActivateTier1Tool covers the unknown-tool error branch and the
// happy path that triggers a tools/list_changed notification through the
// installed notifier.
func TestActivateTier1Tool(t *testing.T) {
	t.Run("unknown_tool_errors", func(t *testing.T) {
		s := NewServer("test", nil, nil, nil)
		if err := s.ActivateTier1Tool("nope"); err == nil {
			t.Fatal("expected unknown-tool error")
		}
	})
	t.Run("notifies_on_success", func(t *testing.T) {
		descriptors := []ToolDescriptor{{
			Tool:    Tool{Name: "clockify_test"},
			Handler: func(ctx context.Context, _ map[string]any) (any, error) { return nil, nil },
		}}
		s := NewServer("test", descriptors, nil, nil)
		stub := &stubNotifier{}
		s.SetNotifier(stub)
		if err := s.ActivateTier1Tool("clockify_test"); err != nil {
			t.Fatalf("activate: %v", err)
		}
		if stub.calls != 1 {
			t.Fatalf("expected 1 notification, got %d", stub.calls)
		}
		if stub.lastMethod != "notifications/tools/list_changed" {
			t.Fatalf("unexpected method: %q", stub.lastMethod)
		}
	})
}

// TestDroppingNotifierNotify covers the legacy-HTTP notifier that drops
// notifications and increments the metric counter without erroring.
func TestDroppingNotifierNotify(t *testing.T) {
	n := droppingNotifier{}
	if err := n.Notify("notifications/tools/list_changed", nil); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

// TestEncoderNotifierNilEncoder verifies the encoderNotifier no-ops cleanly
// when its encoder pointer is nil (race-free shutdown path). The mutex must
// be non-nil because the production wiring always shares the parent server's.
func TestEncoderNotifierNilEncoder(t *testing.T) {
	var mu sync.Mutex
	var enc *json.Encoder
	en := encoderNotifier{mu: &mu, encoder: &enc}
	if err := en.Notify("foo", nil); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

// TestEncoderNotifierWithBuffer exercises the happy path: encoderNotifier
// writes through a real json.Encoder pointing at a bytes.Buffer.
func TestEncoderNotifierWithBuffer(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex
	enc := json.NewEncoder(&buf)
	encPtr := enc
	n := encoderNotifier{mu: &mu, encoder: &encPtr}
	if err := n.Notify("notifications/tools/list_changed", map[string]any{"k": "v"}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if !strings.Contains(buf.String(), "notifications/tools/list_changed") {
		t.Fatalf("expected encoded method, got %q", buf.String())
	}
	// Empty params branch.
	buf.Reset()
	if err := n.Notify("notifications/tools/list_changed", nil); err != nil {
		t.Fatalf("Notify nil: %v", err)
	}
	if !strings.Contains(buf.String(), "params") {
		t.Fatalf("expected empty params object, got %q", buf.String())
	}
}

// TestNotifyToolsChangedWithoutNotifier exercises the metric+log drop path
// when no notifier is installed (e.g., a test harness calling ActivateGroup
// directly).
func TestNotifyToolsChangedWithoutNotifier(t *testing.T) {
	s := NewServer("test", []ToolDescriptor{{
		Tool:    Tool{Name: "clockify_one"},
		Handler: func(ctx context.Context, _ map[string]any) (any, error) { return nil, nil },
	}}, nil, nil)
	// no SetNotifier — should drop without panicking and without erroring.
	s.notifyToolsChanged()
}

// stubNotifier records every Notify call for assertions.
type stubNotifier struct {
	calls      int
	lastMethod string
	failNext   bool
}

func (s *stubNotifier) Notify(method string, _ any) error {
	s.calls++
	s.lastMethod = method
	if s.failNext {
		s.failNext = false
		return errors.New("stub failure")
	}
	return nil
}
