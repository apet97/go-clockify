//go:build pprof

package mcp

import (
	"io"
	"net/http"
	"net/http/httptest"
	// Side-imported so /debug/pprof/* is registered on http.DefaultServeMux
	// inside the test binary. Production builds never link this file; the
	// //go:build pprof guard means it only participates in `go test -tags=pprof`.
	_ "net/http/pprof"
	"strings"
	"testing"
)

// TestMountExtrasPprof proves that the ExtraHandler plumbing
// (transport_extra.go) wires pprof through observeHTTPH correctly and that
// the resulting handler still returns a live profile. Covers the integration
// path both transports use, so the single test protects ServeHTTP and
// ServeStreamableHTTP at once.
func TestMountExtrasPprof(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/baseline", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "baseline-ok")
	})
	mountExtras(mux, []ExtraHandler{
		{Pattern: "/debug/pprof/", Handler: http.DefaultServeMux},
		// nil entry — mountExtras must skip it without panicking.
		{Pattern: "/debug/skip", Handler: nil},
		// empty-pattern entry — also skipped.
		{Pattern: "", Handler: http.DefaultServeMux},
	})

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	t.Run("goroutine profile reachable", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/debug/pprof/goroutine?debug=1")
		if err != nil {
			t.Fatalf("pprof get: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !strings.Contains(string(body), "goroutine profile") {
			head := string(body)
			if len(head) > 200 {
				head = head[:200]
			}
			t.Fatalf("response missing 'goroutine profile' marker: %s", head)
		}
	})

	t.Run("cmdline profile reachable", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/debug/pprof/cmdline")
		if err != nil {
			t.Fatalf("cmdline get: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("baseline handler still reachable after mounting pprof", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/baseline")
		if err != nil {
			t.Fatalf("baseline get: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("baseline status: %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "baseline-ok" {
			t.Fatalf("baseline body: %q", body)
		}
	})
}

// TestExtraHandlerFieldWiringCompileTime is a no-op at runtime — its purpose
// is to force the compiler to verify that Server.ExtraHTTPHandlers and
// StreamableHTTPOptions.ExtraHandlers accept []ExtraHandler. If either field
// is renamed or retyped without updating the pprof wiring in
// cmd/clockify-mcp/, this file stops compiling under -tags=pprof and CI
// catches the regression before runtime.
func TestExtraHandlerFieldWiringCompileTime(t *testing.T) {
	var s Server
	s.ExtraHTTPHandlers = []ExtraHandler{{Pattern: "/x", Handler: http.DefaultServeMux}}
	opts := StreamableHTTPOptions{ExtraHandlers: []ExtraHandler{{Pattern: "/x", Handler: http.DefaultServeMux}}}
	if len(s.ExtraHTTPHandlers) != 1 || len(opts.ExtraHandlers) != 1 {
		t.Fatal("field wiring unexpectedly lost elements")
	}
}
