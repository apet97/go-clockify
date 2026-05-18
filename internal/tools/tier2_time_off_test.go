package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestUpdateTimeOffRequestRehydratesSparsePatch(t *testing.T) {
	const policyID = "65b382b606de527a7ee2b610"
	const requestID = "65b382b606de527a7ee2b611"
	var sawPatch, sawGet bool

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/requests/"+requestID):
			sawPatch = true
			// Sparse PATCH body, exactly as Clockify returns.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":     requestID,
				"status": map[string]any{"statusType": "APPROVED"},
			})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/time-off/requests/"+requestID):
			sawGet = true
			// Hydrated read body.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":       requestID,
				"policyId": policyID,
				"status":   map[string]any{"statusType": "APPROVED"},
				"policy":   map[string]any{"id": policyID, "name": "PTO"},
			})
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer upstream.Close()

	svc := New(clockify.NewClient("test-key", upstream.URL, time.Second, 0), "65b382b606de527a7ee2b60e")

	res, err := svc.updateTimeOffRequest(context.Background(), map[string]any{
		"policy_id":  policyID,
		"request_id": requestID,
		"status":     "APPROVED",
	})
	if err != nil {
		t.Fatalf("updateTimeOffRequest: %v", err)
	}
	if !sawPatch {
		t.Error("expected a PATCH to the request")
	}
	if !sawGet {
		t.Error("expected a re-hydration GET after the PATCH")
	}
	if !res.OK {
		t.Errorf("result not OK: %+v", res)
	}
}
