package mcp

import (
	"context"
	"fmt"
	"sync"
)

// Resource describes a concrete, static MCP resource. Dynamic (parametric)
// resources should be surfaced via ResourceTemplate instead.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourceTemplate describes a parametric MCP resource using RFC 6570 URI
// template syntax — e.g. `clockify://workspace/{workspaceId}/entry/{entryId}`.
type ResourceTemplate struct {
	URITemplate string `json:"uriTemplate"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourceContents is one chunk of resource body. Text and Blob are mutually
// exclusive — text/* content uses Text, binary content uses base64 in Blob.
type ResourceContents struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

// ResourceProvider backs the MCP resources/* method family. Implementations
// live outside the protocol core (tools.Service implements it for Clockify).
// A nil ResourceProvider on Server means the resources capability is off.
type ResourceProvider interface {
	ListResources(ctx context.Context) ([]Resource, error)
	ListResourceTemplates(ctx context.Context) ([]ResourceTemplate, error)
	ReadResource(ctx context.Context, uri string) ([]ResourceContents, error)
}

// resourceSubscriptions is a lightweight subscription set used by Server to
// gate notifications/resources/updated emission — only subscribed URIs are
// broadcast. Concurrent-safe via an internal sync.Map.
type resourceSubscriptions struct {
	m sync.Map // key: string uri, value: struct{}
}

func (r *resourceSubscriptions) add(uri string)      { r.m.Store(uri, struct{}{}) }
func (r *resourceSubscriptions) remove(uri string)   { r.m.Delete(uri) }
func (r *resourceSubscriptions) has(uri string) bool { _, ok := r.m.Load(uri); return ok }

// resourcesListParams is the decoded body of a resources/* request that
// targets a specific URI (resources/read, resources/subscribe,
// resources/unsubscribe).
type resourceURIParams struct {
	URI string `json:"uri"`
}

func (s *Server) handleResourcesList(ctx context.Context) (any, *RPCError) {
	if s.ResourceProvider == nil {
		return nil, &RPCError{Code: -32601, Message: "resources capability disabled"}
	}
	items, err := s.ResourceProvider.ListResources(ctx)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("resources/list: %v", err)}
	}
	if items == nil {
		items = []Resource{}
	}
	return map[string]any{"resources": items}, nil
}

func (s *Server) handleResourcesTemplatesList(ctx context.Context) (any, *RPCError) {
	if s.ResourceProvider == nil {
		return nil, &RPCError{Code: -32601, Message: "resources capability disabled"}
	}
	items, err := s.ResourceProvider.ListResourceTemplates(ctx)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("resources/templates/list: %v", err)}
	}
	if items == nil {
		items = []ResourceTemplate{}
	}
	return map[string]any{"resourceTemplates": items}, nil
}

func (s *Server) handleResourcesRead(ctx context.Context, raw any) (any, *RPCError) {
	if s.ResourceProvider == nil {
		return nil, &RPCError{Code: -32601, Message: "resources capability disabled"}
	}
	var params resourceURIParams
	if err := decodeParams(raw, &params); err != nil || params.URI == "" {
		return nil, &RPCError{Code: -32602, Message: "invalid resources/read params: missing uri"}
	}
	contents, err := s.ResourceProvider.ReadResource(ctx, params.URI)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("resources/read: %v", err)}
	}
	if contents == nil {
		contents = []ResourceContents{}
	}
	return map[string]any{"contents": contents}, nil
}

func (s *Server) handleResourcesSubscribe(raw any) (any, *RPCError) {
	if s.ResourceProvider == nil {
		return nil, &RPCError{Code: -32601, Message: "resources capability disabled"}
	}
	var params resourceURIParams
	if err := decodeParams(raw, &params); err != nil || params.URI == "" {
		return nil, &RPCError{Code: -32602, Message: "invalid resources/subscribe params: missing uri"}
	}
	s.resourceSubs.add(params.URI)
	return map[string]any{}, nil
}

func (s *Server) handleResourcesUnsubscribe(raw any) (any, *RPCError) {
	if s.ResourceProvider == nil {
		return nil, &RPCError{Code: -32601, Message: "resources capability disabled"}
	}
	var params resourceURIParams
	if err := decodeParams(raw, &params); err != nil || params.URI == "" {
		return nil, &RPCError{Code: -32602, Message: "invalid resources/unsubscribe params: missing uri"}
	}
	s.resourceSubs.remove(params.URI)
	return map[string]any{}, nil
}

// NotifyResourceUpdated publishes notifications/resources/updated if the URI
// has an active subscription. Transports/tool handlers call this after a
// mutation that invalidates a cached resource view. Safe to call before the
// notifier is wired — the call silently no-ops.
func (s *Server) NotifyResourceUpdated(uri string) {
	if uri == "" || !s.resourceSubs.has(uri) {
		return
	}
	if s.notifier == nil {
		return
	}
	_ = s.notifier.Notify("notifications/resources/updated", map[string]any{"uri": uri})
}
