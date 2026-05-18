package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestCreateTag(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/tags" && r.Method == http.MethodPost:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["name"] != "urgent" {
				t.Fatalf("expected name 'urgent', got %v", body["name"])
			}
			respondJSON(t, w, clockify.Tag{ID: "t1", Name: "urgent"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.CreateTag(context.Background(), map[string]any{
		"name": "urgent",
	})
	if err != nil {
		t.Fatalf("create tag failed: %v", err)
	}
	tag, ok := result.Data.(clockify.Tag)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if tag.ID != "t1" || tag.Name != "urgent" {
		t.Fatalf("unexpected tag: %+v", tag)
	}
}
