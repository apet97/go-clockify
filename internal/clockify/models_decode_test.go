package clockify_test

import (
	"encoding/json"
	"testing"

	"github.com/apet97/go-clockify/internal/clockify"
)

// TestWorkspaceTypedFieldsDecode verifies that the currencies array and the
// subdomain object decode into their concrete typed structs. The shapes mirror
// the Clockify workspace response (see internal/testclockify/fake_server.go for
// the in-repo workspace fixture, which this extends with the currency/subdomain
// blocks the live API also returns).
func TestWorkspaceTypedFieldsDecode(t *testing.T) {
	raw := []byte(`{
		"id": "ws-1",
		"name": "Test Workspace",
		"currencies": [
			{"id": "cur-1", "code": "USD", "isDefault": true},
			{"id": "cur-2", "code": "EUR"}
		],
		"subdomain": {"name": "acme", "enabled": true},
		"workspaceSettings": {"weekStart": "MONDAY"},
		"memberships": []
	}`)

	var w clockify.Workspace
	if err := json.Unmarshal(raw, &w); err != nil {
		t.Fatalf("decode workspace: %v", err)
	}

	if len(w.Currencies) != 2 {
		t.Fatalf("currencies: expected 2 entries, got %d (%#v)", len(w.Currencies), w.Currencies)
	}
	if w.Currencies[0].ID != "cur-1" || w.Currencies[0].Code != "USD" || !w.Currencies[0].IsDefault {
		t.Fatalf("currencies[0]: expected default USD cur-1, got %#v", w.Currencies[0])
	}
	if w.Currencies[1].Code != "EUR" || w.Currencies[1].IsDefault {
		t.Fatalf("currencies[1]: expected non-default EUR, got %#v", w.Currencies[1])
	}
	if w.Subdomain == nil || w.Subdomain.Name != "acme" || !w.Subdomain.Enabled {
		t.Fatalf("subdomain: expected acme enabled, got %#v", w.Subdomain)
	}
	// WorkspaceSettings stays intentionally untyped (any); just confirm it
	// still populates so the typed siblings did not disturb decoding.
	if w.WorkspaceSettings == nil {
		t.Fatal("workspaceSettings: expected populated any value, got nil")
	}
}

// TestWorkspaceTypedFieldsOmitWhenAbsent confirms the typed currency slice and
// subdomain pointer stay nil (and marshal away) when the upstream payload omits
// them, preserving the original `omitempty` wire behaviour.
func TestWorkspaceTypedFieldsOmitWhenAbsent(t *testing.T) {
	var w clockify.Workspace
	if err := json.Unmarshal([]byte(`{"id": "ws-1", "name": "Test Workspace"}`), &w); err != nil {
		t.Fatalf("decode minimal workspace: %v", err)
	}
	if w.Currencies != nil || w.Subdomain != nil {
		t.Fatalf("expected nil currencies/subdomain on minimal workspace, got %#v / %#v", w.Currencies, w.Subdomain)
	}

	out, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal minimal workspace: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("re-decode marshalled workspace: %v", err)
	}
	for _, key := range []string{"currencies", "subdomain"} {
		if _, ok := m[key]; ok {
			t.Fatalf("expected %q omitted from marshalled minimal workspace, got %s", key, out)
		}
	}
}

// TestClientEntityCCEmailsDecode verifies that ccEmails decodes into []string,
// the concrete shape the client create/update handlers already produce.
func TestClientEntityCCEmailsDecode(t *testing.T) {
	raw := []byte(`{
		"id": "client-1",
		"name": "Client One",
		"ccEmails": ["a@example.com", "b@example.com"],
		"currencyCode": "USD"
	}`)

	var c clockify.ClientEntity
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode client: %v", err)
	}
	if len(c.CCEmails) != 2 || c.CCEmails[0] != "a@example.com" || c.CCEmails[1] != "b@example.com" {
		t.Fatalf("ccEmails: expected two addresses, got %#v", c.CCEmails)
	}
}

// TestClientEntityCCEmailsRoundTrip confirms the []string field still omits
// cleanly when absent and round-trips identically when present, preserving the
// original wire behaviour for the client create/update payload builders.
func TestClientEntityCCEmailsRoundTrip(t *testing.T) {
	var empty clockify.ClientEntity
	if err := json.Unmarshal([]byte(`{"id": "client-1", "name": "Client One"}`), &empty); err != nil {
		t.Fatalf("decode minimal client: %v", err)
	}
	if empty.CCEmails != nil {
		t.Fatalf("expected nil ccEmails on minimal client, got %#v", empty.CCEmails)
	}
	out, err := json.Marshal(empty)
	if err != nil {
		t.Fatalf("marshal minimal client: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("re-decode marshalled client: %v", err)
	}
	if _, ok := m["ccEmails"]; ok {
		t.Fatalf("expected ccEmails omitted from marshalled minimal client, got %s", out)
	}

	populated := clockify.ClientEntity{ID: "client-1", Name: "Client One", CCEmails: []string{"a@example.com"}}
	out, err = json.Marshal(populated)
	if err != nil {
		t.Fatalf("marshal populated client: %v", err)
	}
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("re-decode populated client: %v", err)
	}
	if string(m["ccEmails"]) != `["a@example.com"]` {
		t.Fatalf("ccEmails round-trip mismatch: got %s", m["ccEmails"])
	}
}
