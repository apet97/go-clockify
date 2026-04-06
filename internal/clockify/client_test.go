package clockify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientGetSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "test-key" {
			t.Fatalf("missing api key header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"u1","name":"Test"}`))
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	var out map[string]any
	if err := c.Get(context.Background(), "/user", nil, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["id"] != "u1" {
		t.Fatalf("unexpected id: %#v", out)
	}
}

func TestClientAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	var out map[string]any
	err := c.Get(context.Background(), "/user", nil, &out)
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*APIError); !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
}
