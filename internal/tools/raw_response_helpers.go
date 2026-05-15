package tools

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
)

// parseExportFilename extracts the filename from a Content-Disposition
// header like "filename=Clockify_Time_Report_Summary_11%2F15%2F2023-12%2F07%2F2023.pdf".
// Returns empty string if absent. Slashes arrive percent-encoded; we
// URL-decode them so callers see a sensible filename.
func parseExportFilename(cd string) string {
	if cd == "" {
		return ""
	}
	_, fn, found := strings.Cut(cd, "filename=")
	if !found {
		return ""
	}
	if before, _, hasSemi := strings.Cut(fn, ";"); hasSemi {
		fn = before
	}
	fn = strings.Trim(fn, `"`)
	if decoded, err := url.QueryUnescape(fn); err == nil {
		return decoded
	}
	return fn
}

// stringFromAny returns the string view of v when v is already a string.
// Used when reading optional string fields out of map[string]any envelopes
// without panicking on the wrong type — a missing key or non-string value
// becomes an empty string and the caller decides what to do next.
func stringFromAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// inferRawContentType tries to identify the MIME type of a raw response
// body that has no explicit Content-Type. PDF leads with "%PDF" and is
// not always sniffed correctly by net/http, so we special-case it before
// falling back to http.DetectContentType.
func inferRawContentType(body []byte) string {
	if len(body) >= 4 && string(body[:4]) == "%PDF" {
		return "application/pdf"
	}
	return http.DetectContentType(body)
}

// documentedRawResponse packages a raw HTTP response (header + body) into
// the agent-facing envelope shape that export-style tools share. It keeps
// the body as base64-encoded bytes alongside size and content-type metadata
// so downstream callers can decide whether to forward, surface, or save.
//
// Used by report-export and invoice-export tools that receive binary
// payloads (PDF, CSV, images) from Clockify and need to hand them back
// through the MCP wire safely.
func documentedRawResponse(header http.Header, body []byte) map[string]any {
	contentType := strings.TrimSpace(header.Get("Content-Type"))
	inferred := inferRawContentType(body)
	if contentType == "" || (inferred != "" && strings.Contains(strings.ToLower(contentType), "json")) {
		contentType = inferred
	}
	filename := parseExportFilename(header.Get("Content-Disposition"))
	if filename == "" && contentType == "application/pdf" {
		filename = "document.pdf"
	}
	encoded := base64.StdEncoding.EncodeToString(body)
	return map[string]any{
		"contentType":  contentType,
		"filename":     filename,
		"bytes":        len(body),
		"bodyEncoding": "base64",
		"base64Bytes":  len(encoded),
		"truncated":    false,
		"body":         encoded,
	}
}
