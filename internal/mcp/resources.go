package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Resource describes a concrete, static MCP resource. Dynamic (parametric)
// resources should be surfaced via ResourceTemplate instead.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourceTemplate describes a parametric MCP resource using RFC 6570 URI
// template syntax — e.g. `clockify://workspace/{workspaceId}/entry/{entryId}`.
type ResourceTemplate struct {
	URITemplate string `json:"uriTemplate"`
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
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

// resourceSubscriptions tracks resources/subscribe state per (Notifier, URI)
// so notifications/resources/updated only fire for peers that asked for a
// given URI. This struct owns the in-memory side of the per-notifier
// guarantee.
//
// hasAnyURI / refcount keep the cheap "is anyone interested in this URI?"
// shortcut the tools layer relies on for the emitResourceUpdate skip-fetch
// gate (HasResourceSubscription); the per-stream filter only happens at
// delivery time in NotifyResourceUpdated.
type resourceSubscriptions struct {
	mu         sync.RWMutex
	byNotifier map[Notifier]map[string]struct{}
	refcount   map[string]int
}

// broadcastSubscriber is the sentinel used when a subscribe call arrives
// without a Notifier in context (test code that exercises the handler
// directly without going through a transport). Recording the subscription
// against this sentinel restores the pre-fix server-wide behaviour for
// those callers: NotifyResourceUpdated will fan out to every currently
// registered notifier. Production transports always thread a real Notifier,
// so this sentinel never hits in those paths.
var broadcastSubscriber Notifier = broadcastSentinel{}

type broadcastSentinel struct{}

func (broadcastSentinel) Notify(string, any) error { return nil }

func (r *resourceSubscriptions) add(n Notifier, uri string) {
	if n == nil {
		n = broadcastSubscriber
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.byNotifier == nil {
		r.byNotifier = make(map[Notifier]map[string]struct{})
	}
	if r.refcount == nil {
		r.refcount = make(map[string]int)
	}
	set, ok := r.byNotifier[n]
	if !ok {
		set = make(map[string]struct{})
		r.byNotifier[n] = set
	}
	if _, dup := set[uri]; dup {
		return
	}
	set[uri] = struct{}{}
	r.refcount[uri]++
}

func (r *resourceSubscriptions) remove(n Notifier, uri string) {
	if n == nil {
		n = broadcastSubscriber
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	set, ok := r.byNotifier[n]
	if !ok {
		return
	}
	if _, present := set[uri]; !present {
		return
	}
	delete(set, uri)
	if len(set) == 0 {
		delete(r.byNotifier, n)
	}
	if r.refcount[uri] > 0 {
		r.refcount[uri]--
		if r.refcount[uri] == 0 {
			delete(r.refcount, uri)
		}
	}
}

// dropNotifier removes every subscription owned by n, decrementing per-URI
// refcounts. Called from the notifierHub remove closure when a stream
// closes so HasResourceSubscription stops lying about active subscribers.
func (r *resourceSubscriptions) dropNotifier(n Notifier) {
	if n == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	set, ok := r.byNotifier[n]
	if !ok {
		return
	}
	for uri := range set {
		if r.refcount[uri] > 0 {
			r.refcount[uri]--
			if r.refcount[uri] == 0 {
				delete(r.refcount, uri)
			}
		}
	}
	delete(r.byNotifier, n)
}

// hasAnyURI reports whether any notifier is currently subscribed to uri.
// Used by HasResourceSubscription to gate the tools-layer skip-fetch path
// without paying the per-notifier filter cost.
func (r *resourceSubscriptions) hasAnyURI(uri string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.refcount[uri] > 0
}

// targetsFor returns the notifiers subscribed to uri. The broadcast
// sentinel, when present, expands to "deliver to every currently registered
// notifier" via the broadcastFanout bool. Returns (nil, false) when no
// subscription exists so callers can skip param construction.
func (r *resourceSubscriptions) targetsFor(uri string) (targets []Notifier, broadcastFanout bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.refcount[uri] == 0 {
		return nil, false
	}
	for n, set := range r.byNotifier {
		if _, ok := set[uri]; !ok {
			continue
		}
		if n == broadcastSubscriber {
			broadcastFanout = true
			continue
		}
		targets = append(targets, n)
	}
	return targets, broadcastFanout
}

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
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	items, err := s.ResourceProvider.ListResources(timeoutCtx)
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
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	items, err := s.ResourceProvider.ListResourceTemplates(timeoutCtx)
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
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	contents, err := s.ResourceProvider.ReadResource(timeoutCtx, params.URI)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("resources/read: %v", err)}
	}
	if contents == nil {
		contents = []ResourceContents{}
	}
	return map[string]any{"contents": contents}, nil
}

func (s *Server) handleResourcesSubscribe(ctx context.Context, raw any) (any, *RPCError) {
	if s.ResourceProvider == nil {
		return nil, &RPCError{Code: -32601, Message: "resources capability disabled"}
	}
	var params resourceURIParams
	if err := decodeParams(raw, &params); err != nil || params.URI == "" {
		return nil, &RPCError{Code: -32602, Message: "invalid resources/subscribe params: missing uri"}
	}
	n, _ := notifierFromContext(ctx)
	s.resourceSubs.add(n, params.URI)
	return map[string]any{}, nil
}

func (s *Server) handleResourcesUnsubscribe(ctx context.Context, raw any) (any, *RPCError) {
	if s.ResourceProvider == nil {
		return nil, &RPCError{Code: -32601, Message: "resources capability disabled"}
	}
	var params resourceURIParams
	if err := decodeParams(raw, &params); err != nil || params.URI == "" {
		return nil, &RPCError{Code: -32602, Message: "invalid resources/unsubscribe params: missing uri"}
	}
	n, _ := notifierFromContext(ctx)
	s.resourceSubs.remove(n, params.URI)
	return map[string]any{}, nil
}

// ResourceUpdateDelta carries the optional delta envelope the server can
// attach to a notifications/resources/updated payload. When Format is
// empty the legacy payload shape is emitted ({"uri": ...}); otherwise the
// envelope is merged into the notification params so MCP clients can
// apply a minimal JSON Merge Patch (RFC 7396) against their cached
// resource state instead of re-fetching the whole document.
//
// Format values are defined in internal/jsonmergepatch (FormatNone /
// FormatMerge / FormatFull / FormatDeleted). The protocol core does not
// interpret them; it passes the envelope through to the notifier. Format
// validation and payload shape are the tools-layer caller's responsibility.
//
// This extension is additive and backwards compatible: clients that only
// read the `uri` field keep working. No MCP protocol version bump is
// required.
type ResourceUpdateDelta struct {
	// Format is one of FormatNone / FormatMerge / FormatFull /
	// FormatDeleted. Empty means do not emit a delta envelope — legacy
	// payload shape {"uri": "..."} is used.
	Format string
	// Patch is the wire-format payload for FormatMerge / FormatFull. It
	// is emitted verbatim under the "patch" key. Pre-decoded (already
	// a Go value) so marshalling the notification doesn't require a
	// re-parse step.
	Patch any
}

// HasResourceSubscription reports whether any client is currently subscribed
// to uri. The tool layer calls this before re-reading a resource in
// emitResourceUpdate so unsubscribed mutations don't pay for a redundant
// ReadResource round-trip. Coarse by design — it answers "does anyone want
// this URI?" so the read is skipped when nobody does; the per-stream filter
// that prevents cross-talk happens inside NotifyResourceUpdated.
func (s *Server) HasResourceSubscription(uri string) bool {
	return uri != "" && s.resourceSubs.hasAnyURI(uri)
}

// NotifyResourceUpdated publishes notifications/resources/updated to every
// notifier that called resources/subscribe for uri. Tool handlers call
// this after a mutation that invalidates a cached resource view. Safe
// to call before any notifier is wired — the call silently no-ops when
// nothing is subscribed.
//
// Subscription scope is per-notifier; a peer that did NOT subscribe to
// uri never receives the notification. tools/list_changed continues to
// fan out broadcast through Server.Notify; only resources/updated takes
// this per-URI path.
//
// When delta.Format is non-empty the notification params include the
// envelope:
//
//	{
//	  "uri": "clockify://workspace/ws/entry/id",
//	  "format": "merge",
//	  "patch": { "description": "new text", "billable": true }
//	}
//
// Empty delta preserves the legacy payload shape {"uri": "..."} so
// existing clients and tests remain unchanged.
func (s *Server) NotifyResourceUpdated(uri string, delta ResourceUpdateDelta) {
	if uri == "" {
		return
	}
	targets, broadcast := s.resourceSubs.targetsFor(uri)
	if len(targets) == 0 && !broadcast {
		return
	}
	params := map[string]any{"uri": uri}
	if delta.Format != "" {
		params["format"] = delta.Format
		if delta.Format != "none" && delta.Format != "deleted" {
			params["patch"] = delta.Patch
		}
	}
	// Broadcast and per-notifier paths are mutually exclusive. The
	// broadcastSubscriber sentinel only appears when at least one
	// subscribe call landed without a Notifier in ctx (legacy
	// test-only path). Because s.Notify fans out to every registered
	// notifier — which is a strict superset of the per-notifier
	// targets — taking the broadcast branch alone delivers to every
	// interested party at-most-once. Production transports always
	// thread ctx so broadcast is always false; the branch exists for
	// pre-fix test backwards-compat.
	if broadcast {
		if err := s.Notify("notifications/resources/updated", params); err != nil {
			slog.Warn("resource_update_notify_failed", "uri", uri, "err", err)
		}
		return
	}
	for _, n := range targets {
		if err := n.Notify("notifications/resources/updated", params); err != nil {
			slog.Warn("resource_update_notify_failed", "uri", uri, "err", err)
		}
	}
}

// NotifyResourcesListChanged emits notifications/resources/list_changed to
// every registered notifier. The one-user server calls this when the set of
// available resources changes (a demo-run resource is added) so the advertised
// resources.listChanged capability is truthful. No-op when resources are off.
func (s *Server) NotifyResourcesListChanged() {
	if s.ResourceProvider == nil {
		return
	}
	if err := s.Notify("notifications/resources/list_changed", nil); err != nil {
		slog.Warn("resources_list_changed_notify_failed", "err", err)
	}
}
