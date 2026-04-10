// Package logging provides a stdlib-only slog.Handler decorator that scrubs
// sensitive values from log attributes before they reach the underlying
// handler. Operators can wrap any slog.Handler with NewRedactingHandler to
// gain defence-in-depth PII/secret redaction without modifying call sites.
//
// The redactor matches by attribute key (case-insensitive) against a list of
// well-known secret field names and replaces the value with a short mask.
// Nested map, slog.Group, and []any values are walked recursively.
//
// This is a belt-and-braces layer. Hot-path code should still avoid logging
// secrets explicitly; the redactor exists so that a future code change which
// accidentally includes a header map or error body in a log statement does
// not leak credentials.
package logging

import (
	"context"
	"log/slog"
	"strings"
)

// DefaultSensitiveKeys is the built-in list of attribute keys whose values
// will be masked. Case-insensitive, substring match.
var DefaultSensitiveKeys = []string{
	"authorization",
	"auth",
	"api_key",
	"apikey",
	"x-api-key",
	"bearer",
	"token",
	"secret",
	"password",
	"passphrase",
	"cookie",
	"set-cookie",
	"credential",
	"session",
	"csrf",
	"private_key",
	"privatekey",
	"client_secret",
	"refresh_token",
	"access_token",
	"id_token",
}

// RedactingHandler wraps a slog.Handler and scrubs sensitive values from
// attributes before delegating to the inner handler.
type RedactingHandler struct {
	inner     slog.Handler
	sensitive []string
	mask      string
}

// NewRedactingHandler wraps inner with the default sensitive-key list and
// "[REDACTED]" as the replacement value.
func NewRedactingHandler(inner slog.Handler) *RedactingHandler {
	return &RedactingHandler{
		inner:     inner,
		sensitive: DefaultSensitiveKeys,
		mask:      "[REDACTED]",
	}
}

// WithSensitiveKeys returns a copy that matches additional key patterns.
// Existing defaults are retained; pass a new handler with WithMask to replace
// the mask string.
func (h *RedactingHandler) WithSensitiveKeys(keys ...string) *RedactingHandler {
	cp := *h
	merged := make([]string, 0, len(h.sensitive)+len(keys))
	merged = append(merged, h.sensitive...)
	merged = append(merged, keys...)
	cp.sensitive = merged
	return &cp
}

// WithMask returns a copy that uses a different replacement string.
func (h *RedactingHandler) WithMask(mask string) *RedactingHandler {
	cp := *h
	cp.mask = mask
	return &cp
}

// Enabled delegates to the inner handler.
func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle scrubs the record's attributes in-place before forwarding to the
// inner handler. Because slog.Record's Attrs method walks a shared slice we
// build a new record with the scrubbed attrs.
func (h *RedactingHandler) Handle(ctx context.Context, r slog.Record) error {
	nr := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		nr.AddAttrs(h.scrubAttr(a))
		return true
	})
	return h.inner.Handle(ctx, nr)
}

// WithAttrs scrubs the static attrs applied at handler construction time.
func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	scrubbed := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		scrubbed[i] = h.scrubAttr(a)
	}
	return &RedactingHandler{
		inner:     h.inner.WithAttrs(scrubbed),
		sensitive: h.sensitive,
		mask:      h.mask,
	}
}

// WithGroup delegates to the inner handler.
func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{
		inner:     h.inner.WithGroup(name),
		sensitive: h.sensitive,
		mask:      h.mask,
	}
}

func (h *RedactingHandler) scrubAttr(a slog.Attr) slog.Attr {
	if h.isSensitive(a.Key) {
		return slog.String(a.Key, h.mask)
	}
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		out := make([]slog.Attr, len(attrs))
		for i, inner := range attrs {
			out[i] = h.scrubAttr(inner)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(out...)}
	}
	if a.Value.Kind() == slog.KindAny {
		if m, ok := a.Value.Any().(map[string]any); ok {
			return slog.Attr{Key: a.Key, Value: slog.AnyValue(h.scrubMap(m))}
		}
	}
	return a
}

func (h *RedactingHandler) scrubMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if h.isSensitive(k) {
			out[k] = h.mask
			continue
		}
		if nested, ok := v.(map[string]any); ok {
			out[k] = h.scrubMap(nested)
			continue
		}
		out[k] = v
	}
	return out
}

// isSensitive does a case-insensitive substring check against the configured
// key list. Substring matching is deliberate — it catches variants like
// `x-api-key`, `oauth_access_token`, `clockify_api_key`.
func (h *RedactingHandler) isSensitive(key string) bool {
	lk := strings.ToLower(key)
	for _, s := range h.sensitive {
		if strings.Contains(lk, strings.ToLower(s)) {
			return true
		}
	}
	return false
}
