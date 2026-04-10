package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

type customMap map[string]string
type customSlice []map[string]any

func TestRedactingHandlerTopLevelSensitiveAttrs(t *testing.T) {
	record := renderRecord(t,
		slog.String("authorization", "Bearer secret"),
		slog.String("trace_id", "abc123"),
	)

	if got := record["authorization"]; got != "[REDACTED]" {
		t.Fatalf("expected authorization redacted, got %v", got)
	}
	if got := record["trace_id"]; got != "abc123" {
		t.Fatalf("expected non-sensitive attr unchanged, got %v", got)
	}
}

func TestRedactingHandlerGroupedAttrs(t *testing.T) {
	record := renderRecord(t,
		slog.Group("headers",
			slog.String("Authorization", "Bearer secret"),
			slog.String("trace_id", "abc123"),
		),
	)

	headers, ok := record["headers"].(map[string]any)
	if !ok {
		t.Fatalf("expected headers object, got %T", record["headers"])
	}
	if got := headers["Authorization"]; got != "[REDACTED]" {
		t.Fatalf("expected grouped authorization redacted, got %v", got)
	}
	if got := headers["trace_id"]; got != "abc123" {
		t.Fatalf("expected grouped non-sensitive attr unchanged, got %v", got)
	}
}

func TestRedactingHandlerNestedMapsAndSlices(t *testing.T) {
	record := renderRecord(t,
		slog.Any("payload", map[string]any{
			"project": "Alpha",
			"nested": map[string]any{
				"api_key": "secret-key",
				"items": []any{
					map[string]any{"access_token": "token-1"},
					"safe",
					[]any{map[string]any{"client_secret": "token-2"}},
				},
			},
		}),
	)

	payload, ok := record["payload"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload object, got %T", record["payload"])
	}
	if got := payload["project"]; got != "Alpha" {
		t.Fatalf("expected non-sensitive payload field unchanged, got %v", got)
	}

	nested, ok := payload["nested"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested object, got %T", payload["nested"])
	}
	if got := nested["api_key"]; got != "[REDACTED]" {
		t.Fatalf("expected nested api_key redacted, got %v", got)
	}

	items, ok := nested["items"].([]any)
	if !ok {
		t.Fatalf("expected nested items slice, got %T", nested["items"])
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map element, got %T", items[0])
	}
	if got := first["access_token"]; got != "[REDACTED]" {
		t.Fatalf("expected slice map token redacted, got %v", got)
	}
	if got := items[1]; got != "safe" {
		t.Fatalf("expected safe slice element unchanged, got %v", got)
	}
	deeper, ok := items[2].([]any)
	if !ok {
		t.Fatalf("expected nested slice, got %T", items[2])
	}
	deeperMap, ok := deeper[0].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map, got %T", deeper[0])
	}
	if got := deeperMap["client_secret"]; got != "[REDACTED]" {
		t.Fatalf("expected deeply nested secret redacted, got %v", got)
	}
}

func TestRedactingHandlerCaseInsensitiveSubstringMatching(t *testing.T) {
	record := renderRecord(t,
		slog.String("OAuth_Access_Token", "secret-token"),
		slog.Any("payload", map[string]any{
			"dbPasswordHash": "sensitive",
		}),
	)

	if got := record["OAuth_Access_Token"]; got != "[REDACTED]" {
		t.Fatalf("expected case-insensitive token redacted, got %v", got)
	}
	payload, ok := record["payload"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload object, got %T", record["payload"])
	}
	if got := payload["dbPasswordHash"]; got != "[REDACTED]" {
		t.Fatalf("expected substring password match redacted, got %v", got)
	}
}

func TestRedactingHandlerWithAttrsAndWithGroup(t *testing.T) {
	var attrsBuf bytes.Buffer
	attrsHandler := NewRedactingHandler(slog.NewJSONHandler(&attrsBuf, nil)).
		WithAttrs([]slog.Attr{{
			Key:   "authorization",
			Value: slog.StringValue("Bearer secret"),
		}})
	slog.New(attrsHandler).LogAttrs(t.Context(), slog.LevelInfo, "test")

	var attrsRecord map[string]any
	if err := json.Unmarshal(attrsBuf.Bytes(), &attrsRecord); err != nil {
		t.Fatalf("unmarshal attrs record: %v", err)
	}
	if got := attrsRecord["authorization"]; got != "[REDACTED]" {
		t.Fatalf("expected WithAttrs authorization redacted, got %v", got)
	}

	var groupBuf bytes.Buffer
	groupHandler := NewRedactingHandler(slog.NewJSONHandler(&groupBuf, nil)).WithGroup("http")
	slog.New(groupHandler).LogAttrs(t.Context(), slog.LevelInfo, "test", slog.String("token", "secret-token"))

	var groupRecord map[string]any
	if err := json.Unmarshal(groupBuf.Bytes(), &groupRecord); err != nil {
		t.Fatalf("unmarshal group record: %v", err)
	}
	httpGroup, ok := groupRecord["http"].(map[string]any)
	if !ok {
		t.Fatalf("expected http group, got %T", groupRecord["http"])
	}
	if got := httpGroup["token"]; got != "[REDACTED]" {
		t.Fatalf("expected WithGroup token redacted, got %v", got)
	}
}

func TestRedactingHandlerWithMaskAndSensitiveKeys(t *testing.T) {
	var buf bytes.Buffer
	handler := NewRedactingHandler(slog.NewJSONHandler(&buf, nil)).
		WithMask("[MASKED]").
		WithSensitiveKeys("traceparent")
	logger := slog.New(handler)

	logger.LogAttrs(t.Context(), slog.LevelInfo, "test",
		slog.String("traceparent", "00-secret"),
		slog.String("request_id", "req-1"),
	)

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal log record: %v", err)
	}
	if got := record["traceparent"]; got != "[MASKED]" {
		t.Fatalf("expected custom-sensitive key to use custom mask, got %v", got)
	}
	if got := record["request_id"]; got != "req-1" {
		t.Fatalf("expected non-sensitive attr unchanged, got %v", got)
	}
}

func TestRedactingHandlerReflectMapAndSlicePaths(t *testing.T) {
	record := renderRecord(t,
		slog.Any("payload", customMap{
			"refresh_token": "secret-token",
			"project":       "Alpha",
		}),
		slog.Any("items", customSlice{
			{"private_key": "secret-key"},
			{"project": "Beta"},
		}),
	)

	payload, ok := record["payload"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload object, got %T", record["payload"])
	}
	if got := payload["refresh_token"]; got != "[REDACTED]" {
		t.Fatalf("expected reflect-map secret redacted, got %v", got)
	}
	if got := payload["project"]; got != "Alpha" {
		t.Fatalf("expected reflect-map safe value unchanged, got %v", got)
	}

	items, ok := record["items"].([]any)
	if !ok {
		t.Fatalf("expected items slice, got %T", record["items"])
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first reflect-slice element map, got %T", items[0])
	}
	if got := first["private_key"]; got != "[REDACTED]" {
		t.Fatalf("expected reflect-slice secret redacted, got %v", got)
	}
}

func renderRecord(t *testing.T, attrs ...slog.Attr) map[string]any {
	t.Helper()

	var buf bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewJSONHandler(&buf, nil)))
	logger.LogAttrs(t.Context(), slog.LevelInfo, "test", attrs...)

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal log record: %v", err)
	}
	return record
}
