package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

// stubResourceProvider lets resource tests drive the server without pulling
// in the full tools.Service stack.
type stubResourceProvider struct {
	resources []Resource
	templates []ResourceTemplate
	reads     map[string][]ResourceContents
	err       error
}

func (s *stubResourceProvider) ListResources(_ context.Context) ([]Resource, error) {
	return s.resources, s.err
}
func (s *stubResourceProvider) ListResourceTemplates(_ context.Context) ([]ResourceTemplate, error) {
	return s.templates, s.err
}
func (s *stubResourceProvider) ReadResource(_ context.Context, uri string) ([]ResourceContents, error) {
	if s.err != nil {
		return nil, s.err
	}
	if c, ok := s.reads[uri]; ok {
		return c, nil
	}
	return nil, nil
}

// recordingNotifier captures every notification sent so tests can assert on
// server-initiated emissions.
type recordingNotifier struct {
	calls []struct {
		Method string
		Params any
	}
}

func (r *recordingNotifier) Notify(method string, params any) error {
	r.calls = append(r.calls, struct {
		Method string
		Params any
	}{method, params})
	return nil
}

func newResourceTestServer(provider ResourceProvider) *Server {
	server := NewServer("test", nil, nil, nil)
	server.ResourceProvider = provider
	server.initialized.Store(true)
	return server
}

func TestInitializeAdvertisesResourcesCapability(t *testing.T) {
	provider := &stubResourceProvider{}
	server := NewServer("test", nil, nil, nil)
	server.ResourceProvider = provider

	result := server.handleInitialize(map[string]any{})
	caps, ok := result["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("capabilities: %T", result["capabilities"])
	}
	resources, ok := caps["resources"].(map[string]any)
	if !ok {
		t.Fatalf("resources capability not advertised: %+v", caps)
	}
	if resources["subscribe"] != true || resources["listChanged"] != true {
		t.Fatalf("resources capability shape: %+v", resources)
	}
}

func TestInitializeOmitsResourcesWhenProviderNil(t *testing.T) {
	server := NewServer("test", nil, nil, nil)
	result := server.handleInitialize(map[string]any{})
	caps := result["capabilities"].(map[string]any)
	if _, present := caps["resources"]; present {
		t.Fatalf("resources capability should be omitted when provider is nil: %+v", caps)
	}
}

func TestResourcesListReturnsProviderEntries(t *testing.T) {
	provider := &stubResourceProvider{
		resources: []Resource{
			{URI: "clockify://workspace/abc", Name: "Current workspace", MimeType: "application/json"},
		},
	}
	server := newResourceTestServer(provider)
	resp := server.handle(context.Background(), Request{JSONRPC: "2.0", ID: 1, Method: "resources/list"})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	result := resp.Result.(map[string]any)
	list, ok := result["resources"].([]Resource)
	if !ok {
		t.Fatalf("resources type: %T", result["resources"])
	}
	if len(list) != 1 || list[0].URI != "clockify://workspace/abc" {
		t.Fatalf("resources: %+v", list)
	}
}

func TestResourcesTemplatesList(t *testing.T) {
	provider := &stubResourceProvider{
		templates: []ResourceTemplate{
			{URITemplate: "clockify://workspace/{workspaceId}", Name: "Workspace"},
			{URITemplate: "clockify://workspace/{workspaceId}/entry/{entryId}", Name: "Entry"},
		},
	}
	server := newResourceTestServer(provider)
	resp := server.handle(context.Background(), Request{JSONRPC: "2.0", ID: 1, Method: "resources/templates/list"})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	result := resp.Result.(map[string]any)
	tmpls, _ := result["resourceTemplates"].([]ResourceTemplate)
	if len(tmpls) != 2 {
		t.Fatalf("templates: %+v", tmpls)
	}
}

func TestResourcesReadHappyPath(t *testing.T) {
	provider := &stubResourceProvider{
		reads: map[string][]ResourceContents{
			"clockify://workspace/abc": {
				{URI: "clockify://workspace/abc", MimeType: "application/json", Text: `{"id":"abc"}`},
			},
		},
	}
	server := newResourceTestServer(provider)
	resp := server.handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "resources/read",
		Params:  map[string]any{"uri": "clockify://workspace/abc"},
	})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	result := resp.Result.(map[string]any)
	contents, _ := result["contents"].([]ResourceContents)
	if len(contents) != 1 || contents[0].Text != `{"id":"abc"}` {
		t.Fatalf("contents: %+v", contents)
	}
}

func TestResourcesReadMissingURIRejected(t *testing.T) {
	server := newResourceTestServer(&stubResourceProvider{})
	resp := server.handle(context.Background(), Request{
		JSONRPC: "2.0", ID: 1, Method: "resources/read", Params: map[string]any{},
	})
	if resp.Error == nil || resp.Error.Code != -32602 {
		t.Fatalf("expected -32602, got %+v", resp.Error)
	}
}

func TestResourcesDisabledWhenProviderNil(t *testing.T) {
	server := NewServer("test", nil, nil, nil)
	server.initialized.Store(true)
	resp := server.handle(context.Background(), Request{JSONRPC: "2.0", ID: 1, Method: "resources/list"})
	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Fatalf("expected -32601 method not found when disabled, got %+v", resp.Error)
	}
}

func TestResourcesSubscribeAndNotify(t *testing.T) {
	notifier := &recordingNotifier{}
	server := newResourceTestServer(&stubResourceProvider{})
	server.SetNotifier(notifier)

	// Subscribe to one URI.
	resp := server.handle(context.Background(), Request{
		JSONRPC: "2.0", ID: 1, Method: "resources/subscribe",
		Params: map[string]any{"uri": "clockify://workspace/abc/entry/42"},
	})
	if resp.Error != nil {
		t.Fatalf("subscribe: %+v", resp.Error)
	}

	// A different URI should not fire a notification.
	server.NotifyResourceUpdated("clockify://workspace/abc/entry/99", ResourceUpdateDelta{})
	if len(notifier.calls) != 0 {
		t.Fatalf("unexpected notification for unsubscribed URI: %+v", notifier.calls)
	}

	// Legacy payload shape: empty delta → only {"uri": ...}.
	server.NotifyResourceUpdated("clockify://workspace/abc/entry/42", ResourceUpdateDelta{})
	if len(notifier.calls) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifier.calls))
	}
	if notifier.calls[0].Method != "notifications/resources/updated" {
		t.Fatalf("method: %q", notifier.calls[0].Method)
	}
	params, _ := notifier.calls[0].Params.(map[string]any)
	if params["uri"] != "clockify://workspace/abc/entry/42" {
		t.Fatalf("params: %+v", params)
	}
	if _, hasFormat := params["format"]; hasFormat {
		t.Fatalf("legacy payload should not carry format key: %+v", params)
	}
	if _, hasPatch := params["patch"]; hasPatch {
		t.Fatalf("legacy payload should not carry patch key: %+v", params)
	}

	// New wire format: merge-patch envelope.
	server.NotifyResourceUpdated(
		"clockify://workspace/abc/entry/42",
		ResourceUpdateDelta{Format: "merge", Patch: map[string]any{"description": "updated"}},
	)
	if len(notifier.calls) != 2 {
		t.Fatalf("expected 2 notifications after merge delta, got %d", len(notifier.calls))
	}
	mergeParams, _ := notifier.calls[1].Params.(map[string]any)
	if mergeParams["format"] != "merge" {
		t.Fatalf("expected format=merge, got %v", mergeParams["format"])
	}
	patch, _ := mergeParams["patch"].(map[string]any)
	if patch["description"] != "updated" {
		t.Fatalf("unexpected patch payload: %+v", mergeParams)
	}

	// FormatNone and FormatDeleted omit the patch field but still
	// carry the format code so clients know to re-fetch or drop state.
	server.NotifyResourceUpdated(
		"clockify://workspace/abc/entry/42",
		ResourceUpdateDelta{Format: "none"},
	)
	noneParams, _ := notifier.calls[2].Params.(map[string]any)
	if noneParams["format"] != "none" {
		t.Fatalf("expected format=none, got %v", noneParams["format"])
	}
	if _, hasPatch := noneParams["patch"]; hasPatch {
		t.Fatalf("format=none must not carry patch: %+v", noneParams)
	}

	// Unsubscribe — further notifications must not fire.
	resp = server.handle(context.Background(), Request{
		JSONRPC: "2.0", ID: 2, Method: "resources/unsubscribe",
		Params: map[string]any{"uri": "clockify://workspace/abc/entry/42"},
	})
	if resp.Error != nil {
		t.Fatalf("unsubscribe: %+v", resp.Error)
	}
	server.NotifyResourceUpdated("clockify://workspace/abc/entry/42", ResourceUpdateDelta{})
	if len(notifier.calls) != 3 {
		t.Fatalf("notification fired after unsubscribe: %+v", notifier.calls)
	}
}

func TestResourcesReadRoundTripsThroughJSON(t *testing.T) {
	// Smoke: ensure the server's response marshals cleanly through encoding/json
	// (a previous bug had ResourceContents slipping a typed zero into Blob).
	provider := &stubResourceProvider{
		reads: map[string][]ResourceContents{
			"clockify://workspace/abc": {
				{URI: "clockify://workspace/abc", MimeType: "application/json", Text: `{"id":"abc"}`},
			},
		},
	}
	server := newResourceTestServer(provider)
	resp := server.handle(context.Background(), Request{
		JSONRPC: "2.0", ID: 1, Method: "resources/read",
		Params: map[string]any{"uri": "clockify://workspace/abc"},
	})
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !contains(data, `"text":"{\"id\":\"abc\"}"`) {
		t.Fatalf("unexpected marshalled body: %s", data)
	}
}

func contains(haystack []byte, needle string) bool {
	return len(haystack) >= len(needle) && stringIndex(string(haystack), needle) >= 0
}

func stringIndex(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
