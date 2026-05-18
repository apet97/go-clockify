package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestDecodeListSupportsBareAndWrappedClockifyShapes(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		keys []string
		want int
	}{
		{
			name: "bare array",
			raw:  `[{"id":"p1","name":"MCP-LIVE-project"}]`,
			want: 1,
		},
		{
			name: "invoice wrapper",
			raw:  `{"total":1,"invoices":[{"id":"i1","number":"MCP-LIVE-1"}]}`,
			keys: []string{"invoices"},
			want: 1,
		},
		{
			name: "expense nested wrapper",
			raw:  `{"expenses":{"expenses":[{"id":"e1","notes":"MCP-LIVE-expense"}],"count":1}}`,
			keys: []string{"expenses", "expenses"},
			want: 1,
		},
		{
			name: "webhook wrapper",
			raw:  `{"workspaceWebhookCount":1,"webhooks":[{"id":"w1","name":"MCP-LIVE-webhook"}]}`,
			keys: []string{"webhooks"},
			want: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeList(json.RawMessage(tc.raw), tc.keys)
			if err != nil {
				t.Fatalf("decodeList: %v", err)
			}
			if len(got) != tc.want {
				t.Fatalf("len = %d, want %d", len(got), tc.want)
			}
		})
	}
}

func TestMatchesPrefixUsesCollectionSpecificFields(t *testing.T) {
	item := map[string]any{"id": "i1", "number": "MCP-LIVE-1"}
	if !matchesPrefix(item, "MCP-LIVE", []string{"name", "number"}) {
		t.Fatal("invoice number should match the live cleanup prefix")
	}
	if matchesPrefix(item, "MCP-LIVE", nil) {
		t.Fatal("default name-only match should not match an invoice number")
	}
}

func TestCleanErrorRedactsWorkspaceID(t *testing.T) {
	s := &sweeper{workspaceID: "65b382b606de527a7ee2b60e"}
	got := s.cleanError(errors.New("GET /workspaces/65b382b606de527a7ee2b60e/invoices failed"))
	if strings.Contains(got, s.workspaceID) {
		t.Fatalf("workspace id was not redacted: %s", got)
	}
	if !strings.Contains(got, "{workspaceId}") {
		t.Fatalf("redacted placeholder missing: %s", got)
	}
}

func TestArchivePayloadIncludesClientName(t *testing.T) {
	got := archivePayload(collection{label: "clients"}, namedObject{ID: "c1", Name: "MCP-LIVE-client"})
	if got["archived"] != true {
		t.Fatalf("archived flag missing: %#v", got)
	}
	if got["name"] != "MCP-LIVE-client" {
		t.Fatalf("client archive payload must preserve name for Clockify PUT validation: %#v", got)
	}

	project := archivePayload(collection{label: "projects"}, namedObject{ID: "p1", Name: "MCP-LIVE-project"})
	if _, ok := project["name"]; ok {
		t.Fatalf("project archive payload should not add a name: %#v", project)
	}
}
