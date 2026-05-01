package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/apet97/go-clockify/internal/bootstrap"
)

type countingJSONValue struct {
	count *atomic.Int32
	value string
}

func (v countingJSONValue) MarshalJSON() ([]byte, error) {
	v.count.Add(1)
	return json.Marshal(v.value)
}

func TestDispatchToolsListSerializedCacheReusesResultAndKeepsRequestID(t *testing.T) {
	var schemaMarshals atomic.Int32
	server := NewServer("test", []ToolDescriptor{
		descriptorWithCountingSchema("a_tool", &schemaMarshals),
	}, nil, nil)
	initializeSerializedCacheTestServer(t, server)

	first := dispatchToolsListForSerializedCacheTest(t, server, 1)
	if first.ID != float64(1) {
		t.Fatalf("first response id: got %#v want 1", first.ID)
	}
	if got := schemaMarshals.Load(); got != 1 {
		t.Fatalf("first tools/list should marshal schema once, got %d", got)
	}

	second := dispatchToolsListForSerializedCacheTest(t, server, "request-2")
	if second.ID != "request-2" {
		t.Fatalf("second response id: got %#v want request-2", second.ID)
	}
	if got := schemaMarshals.Load(); got != 1 {
		t.Fatalf("cached tools/list result should not remarshal schema, got %d", got)
	}
	if !reflect.DeepEqual(first.Names, second.Names) {
		t.Fatalf("cached response changed tool list: first=%v second=%v", first.Names, second.Names)
	}
}

func TestDispatchToolsListSerializedCacheInvalidatesOnTier1Activation(t *testing.T) {
	var visibleMarshals atomic.Int32
	var hiddenMarshals atomic.Int32
	bc := &bootstrap.Config{
		Mode:        bootstrap.Custom,
		CustomTools: map[string]bool{"visible_tool": true},
	}
	bc.SetTier1Tools(map[string]bool{
		"visible_tool": true,
		"hidden_tool":  true,
	})
	server := NewServer("test", []ToolDescriptor{
		descriptorWithCountingSchema("visible_tool", &visibleMarshals),
		descriptorWithCountingSchema("hidden_tool", &hiddenMarshals),
	}, &testEnforcement{bootstrap: bc}, &testActivator{bootstrap: bc})
	server.SetNotifier(&stubNotifier{})
	initializeSerializedCacheTestServer(t, server)

	before := dispatchToolsListForSerializedCacheTest(t, server, 1)
	if !reflect.DeepEqual(before.Names, []string{"visible_tool"}) {
		t.Fatalf("before activation names: got %v", before.Names)
	}
	dispatchToolsListForSerializedCacheTest(t, server, 2)
	if got := visibleMarshals.Load(); got != 1 {
		t.Fatalf("cached visible tool should be serialized once before activation, got %d", got)
	}
	if got := hiddenMarshals.Load(); got != 0 {
		t.Fatalf("filtered hidden tool should not be serialized before activation, got %d", got)
	}

	if err := server.ActivateTier1Tool("hidden_tool"); err != nil {
		t.Fatalf("activate tier1: %v", err)
	}
	after := dispatchToolsListForSerializedCacheTest(t, server, "after-activation")
	if after.ID != "after-activation" {
		t.Fatalf("after activation id: got %#v want after-activation", after.ID)
	}
	if !reflect.DeepEqual(after.Names, []string{"hidden_tool", "visible_tool"}) {
		t.Fatalf("after activation names: got %v", after.Names)
	}
	if got := visibleMarshals.Load(); got != 2 {
		t.Fatalf("activation should invalidate and remarshal visible tool, got %d", got)
	}
	if got := hiddenMarshals.Load(); got != 1 {
		t.Fatalf("activated hidden tool should be serialized once, got %d", got)
	}
}

func TestDispatchToolsListSerializedCacheHonorsFilteredList(t *testing.T) {
	var visibleMarshals atomic.Int32
	var hiddenMarshals atomic.Int32
	bc := &bootstrap.Config{
		Mode:        bootstrap.Custom,
		CustomTools: map[string]bool{"visible_tool": true},
	}
	bc.SetTier1Tools(map[string]bool{
		"visible_tool": true,
		"hidden_tool":  true,
	})
	server := NewServer("test", []ToolDescriptor{
		descriptorWithCountingSchema("visible_tool", &visibleMarshals),
		descriptorWithCountingSchema("hidden_tool", &hiddenMarshals),
	}, &testEnforcement{bootstrap: bc}, &testActivator{bootstrap: bc})
	initializeSerializedCacheTestServer(t, server)

	first := dispatchToolsListForSerializedCacheTest(t, server, "filter-1")
	second := dispatchToolsListForSerializedCacheTest(t, server, "filter-2")
	for _, got := range [][]string{first.Names, second.Names} {
		if !reflect.DeepEqual(got, []string{"visible_tool"}) {
			t.Fatalf("filtered names: got %v", got)
		}
	}
	if second.ID != "filter-2" {
		t.Fatalf("second response id: got %#v want filter-2", second.ID)
	}
	if got := visibleMarshals.Load(); got != 1 {
		t.Fatalf("visible tool should be serialized once from cache, got %d", got)
	}
	if got := hiddenMarshals.Load(); got != 0 {
		t.Fatalf("filtered hidden tool should never be serialized, got %d", got)
	}
}

func descriptorWithCountingSchema(name string, marshals *atomic.Int32) ToolDescriptor {
	return ToolDescriptor{
		Tool: Tool{
			Name:        name,
			Description: name,
			InputSchema: map[string]any{
				"type": countingJSONValue{count: marshals, value: "object"},
			},
			Annotations: map[string]any{"readOnlyHint": true},
		},
		ReadOnlyHint: true,
		Handler:      func(context.Context, map[string]any) (any, error) { return nil, nil },
	}
}

func initializeSerializedCacheTestServer(t *testing.T, server *Server) {
	t.Helper()
	req, err := json.Marshal(Request{
		JSONRPC: "2.0",
		ID:      0,
		Method:  "initialize",
		Params:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("marshal initialize: %v", err)
	}
	if _, err := server.DispatchMessage(context.Background(), req); err != nil {
		t.Fatalf("initialize dispatch: %v", err)
	}
}

type serializedCacheToolsListResponse struct {
	ID    any
	Names []string
}

func dispatchToolsListForSerializedCacheTest(t *testing.T, server *Server, id any) serializedCacheToolsListResponse {
	t.Helper()
	req, err := json.Marshal(Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "tools/list",
		Params:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("marshal tools/list: %v", err)
	}
	raw, err := server.DispatchMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("dispatch tools/list: %v", err)
	}
	var resp struct {
		ID     any             `json:"id"`
		Result toolsListResult `json:"result"`
		Error  *RPCError       `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal tools/list response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("tools/list error: %+v", resp.Error)
	}
	names := make([]string, 0, len(resp.Result.Tools))
	for _, tool := range resp.Result.Tools {
		names = append(names, tool.Name)
	}
	return serializedCacheToolsListResponse{ID: resp.ID, Names: names}
}
