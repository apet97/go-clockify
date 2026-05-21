//go:build livee2e

package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/config"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/tools"
)

// liveMCPHarness drives MCP-protocol calls (initialize → tools/call) against
// a real Clockify backend using the one-user stdio server. It mirrors the
// production wiring in cmd/clockify-mcp/main.go but lives in test scope so
// suites can assert tool behavior over the wire.
type liveMCPHarness struct {
	t       *testing.T
	Service *tools.Service
	Server  *mcp.Server

	mu     sync.Mutex
	nextID int
}

// liveMCPOptions is the harness configuration knob struct. It is empty
// today because the one-user MCP exposes a single full-access surface; the
// type is preserved so callers can request features explicitly when new
// knobs land.
type liveMCPOptions struct{}

func setupLiveMCPHarness(t *testing.T, _ liveMCPOptions) *liveMCPHarness {
	t.Helper()
	if os.Getenv("CLOCKIFY_API_KEY") == "" {
		t.Skip("Skipping live MCP-path tests since CLOCKIFY_API_KEY is not set")
	}
	if os.Getenv("CLOCKIFY_RUN_LIVE_E2E") != "1" {
		t.Skip("Skipping live MCP-path tests unless CLOCKIFY_RUN_LIVE_E2E=1")
	}

	MarkLiveTestRan()

	cfg, err := config.LoadOneUser()
	if err != nil {
		t.Fatalf("config.LoadOneUser: %v", err)
	}

	client := clockify.NewClient(cfg.APIKey, cfg.BaseURL, 30*time.Second, 2)
	t.Cleanup(client.Close)

	service := tools.New(client, cfg.WorkspaceID)
	if cfg.Timezone != "" {
		if loc, err := time.LoadLocation(cfg.Timezone); err == nil {
			service.DefaultTimezone = loc
		}
	}

	server := mcp.NewServer("livee2e", service.FullAccessRegistry())
	server.StaticToolList = true
	server.ResourceProvider = service
	service.EmitResourceUpdate = server.NotifyResourceUpdated
	service.SubscriptionGate = server.HasResourceSubscription

	return &liveMCPHarness{t: t, Service: service, Server: server}
}

func (h *liveMCPHarness) clearRunningTimer(ctx context.Context) {
	h.t.Helper()
	out, err := h.Service.EntriesRunning(ctx, map[string]any{})
	if err != nil {
		h.t.Fatalf("preflight running timer check: %v", err)
	}
	result, ok := out.(tools.ToolResult)
	if !ok {
		h.t.Fatalf("preflight running timer check returned %T", out)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["running"] != true {
		return
	}
	entryID := liveRunningEntryID(data["entry"])
	if entryID == "" {
		h.t.Fatalf("preflight running timer check could not extract entry id: %#v", data["entry"])
	}
	if _, err := h.Service.DeleteEntry(ctx, map[string]any{"entry_id": entryID}); err != nil {
		h.t.Fatalf("preflight delete stale running timer: %v", err)
	}
}

func (h *liveMCPHarness) requiredTimeEntryCustomFields(ctx context.Context, valuePrefix string) []any {
	h.t.Helper()
	wsID, err := h.Service.ResolveWorkspaceID(ctx)
	if err != nil {
		h.t.Fatalf("resolve workspace: %v", err)
	}
	var fields []map[string]any
	if err := h.Service.Client.Get(ctx, "/workspaces/"+wsID+"/custom-fields", map[string]string{"page": "1", "page-size": "200"}, &fields); err != nil {
		h.t.Fatalf("list custom fields: %v", err)
	}
	out := make([]any, 0)
	for _, field := range fields {
		required, _ := field["required"].(bool)
		if !required || !liveCustomFieldAppliesToTimeEntry(field) {
			continue
		}
		status := strings.ToUpper(strings.TrimSpace(liveStringFromAny(field["status"])))
		if status != "" && status != "VISIBLE" {
			continue
		}
		fieldID := liveFirstNonEmptyString(
			liveStringFromAny(field["id"]),
			liveStringFromAny(field["_id"]),
			liveStringFromAny(field["customFieldId"]),
			liveStringFromAny(field["custom_field_id"]),
		)
		if fieldID == "" {
			h.t.Fatalf("required time-entry custom field is missing an id")
		}
		value, ok := liveCustomFieldTestValue(field, valuePrefix)
		if !ok {
			h.t.Fatalf("required time-entry custom field %q has unsupported type/allowed values", liveStringFromAny(field["name"]))
		}
		out = append(out, map[string]any{"field_id": fieldID, "value": value})
	}
	return out
}

func addRequiredTimeEntryCustomFields(args map[string]any, customFields []any) map[string]any {
	if len(customFields) > 0 {
		args["custom_fields"] = customFields
	}
	return args
}

func liveCustomFieldAppliesToTimeEntry(field map[string]any) bool {
	for _, key := range []string{"entityType", "entity_type", "targetEntityType", "target_entity_type", "resourceType", "resource_type", "documentCode", "document_code"} {
		if liveScopeIncludesTimeEntry(field[key]) {
			return true
		}
	}
	for _, key := range []string{"entityTypes", "entity_types", "targetEntityTypes", "target_entity_types", "appliesTo", "applies_to"} {
		if liveScopeIncludesTimeEntry(field[key]) {
			return true
		}
	}
	return false
}

func liveScopeIncludesTimeEntry(raw any) bool {
	switch v := raw.(type) {
	case string:
		return liveIsTimeEntryScope(v)
	case []any:
		for _, item := range v {
			if liveScopeIncludesTimeEntry(item) {
				return true
			}
		}
	case []string:
		for _, item := range v {
			if liveIsTimeEntryScope(item) {
				return true
			}
		}
	}
	return false
}

func liveIsTimeEntryScope(scope string) bool {
	switch strings.ToUpper(strings.TrimSpace(scope)) {
	case "TIME_ENTRY", "TIMEENTRY", "TIME_ENTRIES", "TIMEENTRY_CUSTOM_FIELD_VALUE":
		return true
	default:
		return false
	}
}

func liveCustomFieldTestValue(field map[string]any, prefix string) (any, bool) {
	switch strings.ToUpper(strings.TrimSpace(liveStringFromAny(field["type"]))) {
	case "TXT":
		return prefix + " custom field", true
	case "LINK":
		return "https://example.com/" + strings.ToLower(strings.ReplaceAll(prefix, " ", "-")), true
	case "NUMBER":
		return float64(1), true
	case "CHECKBOX":
		return true, true
	case "DROPDOWN_SINGLE":
		value, ok := liveFirstAllowedCustomFieldValue(field)
		return value, ok
	case "DROPDOWN_MULTIPLE":
		value, ok := liveFirstAllowedCustomFieldValue(field)
		if !ok {
			return nil, false
		}
		return []any{value}, true
	default:
		return nil, false
	}
}

func liveFirstAllowedCustomFieldValue(field map[string]any) (string, bool) {
	items, _ := field["allowedValues"].([]any)
	for _, item := range items {
		switch v := item.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return v, true
			}
		case map[string]any:
			if value := liveFirstNonEmptyString(
				liveStringFromAny(v["id"]),
				liveStringFromAny(v["_id"]),
				liveStringFromAny(v["value"]),
				liveStringFromAny(v["name"]),
			); value != "" {
				return value, true
			}
		}
	}
	return "", false
}

func liveStringFromAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func liveFirstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func liveRunningEntryID(entry any) string {
	switch value := entry.(type) {
	case clockify.TimeEntry:
		return value.ID
	case map[string]any:
		if id, ok := value["id"].(string); ok {
			return id
		}
	}
	return ""
}

func (h *liveMCPHarness) nextRequestID() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextID++
	return h.nextID
}

// rawCall sends an initialize handshake + tools/call request through the
// stdio loop and returns the parsed response.
func (h *liveMCPHarness) rawCall(ctx context.Context, tool string, args map[string]any) (mcp.Response, error) {
	id := h.nextRequestID()
	if args == nil {
		args = map[string]any{}
	}
	init := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": mcp.SupportedProtocolVersions[0],
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "livee2e", "version": "test"},
		},
	}
	call := map[string]any{
		"jsonrpc": "2.0",
		"id":      id + 1,
		"method":  "tools/call",
		"params":  map[string]any{"name": tool, "arguments": args},
	}
	var input bytes.Buffer
	if err := writeJSON(&input, init); err != nil {
		return mcp.Response{}, err
	}
	if err := writeJSON(&input, call); err != nil {
		return mcp.Response{}, err
	}
	var output bytes.Buffer
	runCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	if err := h.Server.Run(runCtx, &input, &output); err != nil && err != io.EOF {
		return mcp.Response{}, err
	}
	dec := json.NewDecoder(&output)
	for {
		var resp mcp.Response
		if err := dec.Decode(&resp); err != nil {
			if err == io.EOF {
				break
			}
			return mcp.Response{}, fmt.Errorf("decode mcp response: %w", err)
		}
		if respID, ok := numericID(resp.ID); ok && respID == id+1 {
			return resp, nil
		}
	}
	return mcp.Response{}, fmt.Errorf("no response found for tool %s", tool)
}

func writeJSON(w io.Writer, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	_, err = w.Write([]byte{'\n'})
	return err
}

func numericID(id any) (int, bool) {
	switch v := id.(type) {
	case float64:
		return int(v), true
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	case int:
		return v, true
	}
	return 0, false
}

// callOK invokes the tool and fails the test if the call returns a JSON-RPC
// error or a non-OK result envelope.
func (h *liveMCPHarness) callOK(ctx context.Context, tool string, args map[string]any) map[string]any {
	h.t.Helper()
	resp, err := h.rawCall(ctx, tool, args)
	if err != nil {
		h.t.Fatalf("%s: %v", tool, err)
	}
	if resp.Error != nil {
		h.t.Fatalf("%s returned error: %s", tool, resp.Error.Message)
	}
	if resp.Result == nil {
		h.t.Fatalf("%s returned no result", tool)
	}
	raw, ok := resp.Result.(map[string]any)
	if !ok {
		h.t.Fatalf("%s result has unexpected type %T", tool, resp.Result)
	}
	content, _ := raw["content"].([]any)
	if len(content) == 0 {
		h.t.Fatalf("%s result has no content", tool)
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	if text == "" {
		h.t.Fatalf("%s result has no text content", tool)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		h.t.Fatalf("%s parse envelope: %v\nraw=%s", tool, err, text)
	}
	if ok, _ := envelope["ok"].(bool); !ok {
		h.t.Fatalf("%s returned ok=false: %v", tool, envelope)
	}
	return envelope
}

// callExpectError invokes the tool and returns the error message. Fails the
// test if the call succeeds.
func (h *liveMCPHarness) callExpectError(ctx context.Context, tool string, args map[string]any) string {
	h.t.Helper()
	resp, err := h.rawCall(ctx, tool, args)
	if err != nil {
		return err.Error()
	}
	if resp.Error != nil {
		return resp.Error.Message
	}
	if resp.Result == nil {
		return "no result"
	}
	raw, _ := resp.Result.(map[string]any)
	content, _ := raw["content"].([]any)
	if len(content) == 0 {
		return ""
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	if text == "" {
		return ""
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		return err.Error()
	}
	if ok, _ := envelope["ok"].(bool); ok {
		h.t.Fatalf("%s unexpectedly succeeded: %v", tool, envelope)
	}
	if errObj, _ := envelope["error"].(map[string]any); errObj != nil {
		if msg, _ := errObj["message"].(string); msg != "" {
			return msg
		}
	}
	return text
}

// extractDataMap pulls the "data" field out of a result envelope as a map.
func extractDataMap(t *testing.T, envelope map[string]any) map[string]any {
	t.Helper()
	sc := structuredContentMap(t, envelope)
	data, _ := sc["data"].(map[string]any)
	if data == nil {
		t.Fatalf("envelope has no data map: %v", sc)
	}
	return data
}

// listProjectsRaw fetches projects directly through the Clockify client,
// bypassing the MCP path. Useful for cleanup and orthogonal assertions.
func (h *liveMCPHarness) listProjectsRaw(ctx context.Context) []clockify.Project {
	wsID, err := h.Service.ResolveWorkspaceID(ctx)
	if err != nil {
		h.t.Fatalf("resolve workspace: %v", err)
	}
	var projects []clockify.Project
	if err := h.Service.Client.Get(ctx, "/workspaces/"+wsID+"/projects", nil, &projects); err != nil {
		h.t.Fatalf("list projects: %v", err)
	}
	return projects
}

func (h *liveMCPHarness) getEntryRaw(ctx context.Context, entryID string) (clockify.TimeEntry, error) {
	var entry clockify.TimeEntry
	wsID, err := h.Service.ResolveWorkspaceID(ctx)
	if err != nil {
		return entry, err
	}
	if err := h.Service.Client.Get(ctx, "/workspaces/"+wsID+"/time-entries/"+entryID, nil, &entry); err != nil {
		return entry, err
	}
	return entry, nil
}

func (h *liveMCPHarness) deleteEntryRaw(ctx context.Context, entryID string) error {
	wsID, err := h.Service.ResolveWorkspaceID(ctx)
	if err != nil {
		return err
	}
	return h.Service.Client.Delete(ctx, "/workspaces/"+wsID+"/time-entries/"+entryID)
}

func (h *liveMCPHarness) deleteProjectRaw(ctx context.Context, projectID string) error {
	wsID, err := h.Service.ResolveWorkspaceID(ctx)
	if err != nil {
		return err
	}
	return h.Service.Client.Delete(ctx, "/workspaces/"+wsID+"/projects/"+projectID)
}

func (h *liveMCPHarness) deleteClientRaw(ctx context.Context, clientID string) error {
	wsID, err := h.Service.ResolveWorkspaceID(ctx)
	if err != nil {
		return err
	}
	return h.Service.Client.Delete(ctx, "/workspaces/"+wsID+"/clients/"+clientID)
}
