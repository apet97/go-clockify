package tools

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// TestClientsListAutoPaginateWalksEveryPage exercises the
// first-slice ClientsList handler against a fake Clockify server,
// proving auto_paginate: true rolls every page into one ToolResult
// and stamps the meta knobs.
func TestClientsListAutoPaginateWalksEveryPage(t *testing.T) {
	var requests atomic.Int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if !strings.HasSuffix(r.URL.Path, "/workspaces/ws1/clients") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		page := r.URL.Query().Get("page")
		switch page {
		case "1":
			body := make([]map[string]any, autoPaginatePageSize)
			for i := range body {
				body[i] = map[string]any{"id": "c1", "name": "Page1"}
			}
			respondJSON(t, w, body)
		case "2":
			respondJSON(t, w, []map[string]any{
				{"id": "c2a", "name": "Page2-A"},
				{"id": "c2b", "name": "Page2-B"},
			})
		default:
			t.Fatalf("unexpected page %q", page)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	out, err := svc.ClientsList(context.Background(), map[string]any{"auto_paginate": true})
	if err != nil {
		t.Fatalf("ClientsList: %v", err)
	}
	result, ok := out.(ToolResult)
	if !ok {
		t.Fatalf("result type = %T, want ToolResult", out)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("expected 2 HTTP requests, got %d", got)
	}
	if v, _ := result.Meta["auto_paginate"].(bool); !v {
		t.Fatalf("meta.auto_paginate = %v, want true", result.Meta["auto_paginate"])
	}
	if v, _ := result.Meta["has_more"].(bool); v {
		t.Fatalf("meta.has_more = %v, want false (auto-paginate consumed every page)", result.Meta["has_more"])
	}
	if v, _ := result.Meta["truncated"].(bool); v {
		t.Fatalf("meta.truncated = %v, want false (no cap hit)", result.Meta["truncated"])
	}
	if got, _ := result.Meta["count"].(int); got != autoPaginatePageSize+2 {
		t.Fatalf("meta.count = %d, want %d", got, autoPaginatePageSize+2)
	}
}

// TestClientsListAutoPaginateRespectsMaxRows proves max_rows trims
// the consolidated result and flags truncated=true in meta so the
// agent sees there is more data upstream.
func TestClientsListAutoPaginateRespectsMaxRows(t *testing.T) {
	var requests atomic.Int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		body := make([]map[string]any, autoPaginatePageSize)
		for i := range body {
			body[i] = map[string]any{"id": "c", "name": "row"}
		}
		respondJSON(t, w, body)
	})
	defer cleanup()

	svc := New(client, "ws1")
	out, err := svc.ClientsList(context.Background(), map[string]any{
		"auto_paginate": true,
		"max_rows":      float64(50),
	})
	if err != nil {
		t.Fatalf("ClientsList: %v", err)
	}
	result := out.(ToolResult)
	if got := requests.Load(); got != 1 {
		t.Fatalf("expected 1 HTTP request before truncation, got %d", got)
	}
	if v, _ := result.Meta["truncated"].(bool); !v {
		t.Fatalf("meta.truncated = %v, want true", result.Meta["truncated"])
	}
	if got, _ := result.Meta["count"].(int); got != 50 {
		t.Fatalf("meta.count = %d, want 50", got)
	}
}

// TestClientsListWithoutAutoPaginateUnchanged guards the single-page
// path: a call without auto_paginate must hit the server exactly once
// and not stamp the new meta knobs.
func TestClientsListWithoutAutoPaginateUnchanged(t *testing.T) {
	var requests atomic.Int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		respondJSON(t, w, []map[string]any{{"id": "c1", "name": "Solo"}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	out, err := svc.ClientsList(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("ClientsList: %v", err)
	}
	result := out.(ToolResult)
	if got := requests.Load(); got != 1 {
		t.Fatalf("expected 1 request, got %d", got)
	}
	if _, present := result.Meta["auto_paginate"]; present {
		t.Fatalf("meta.auto_paginate present on single-page path: %+v", result.Meta)
	}
	if _, present := result.Meta["truncated"]; present {
		t.Fatalf("meta.truncated present on single-page path: %+v", result.Meta)
	}
}
