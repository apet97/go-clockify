package tools

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
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

// documentedRawResponse packages a raw HTTP response (header + body) into the
// agent-facing envelope shape that export-style tools share. Large binary
// formats are written to a local temp file and returned by path; text-ish
// formats keep the historical base64 envelope.
func documentedRawResponse(header http.Header, body []byte) map[string]any {
	return documentedRawResponseWithOptions(header, body, false)
}

func documentedReportRawResponse(header http.Header, body []byte) map[string]any {
	return documentedRawResponseWithOptions(header, body, true)
}

func documentedRawResponseWithOptions(header http.Header, body []byte, storeCSVAsFile bool) map[string]any {
	contentType := strings.TrimSpace(header.Get("Content-Type"))
	inferred := inferRawContentType(body)
	if contentType == "" || (inferred != "" && strings.Contains(strings.ToLower(contentType), "json")) {
		contentType = inferred
	}
	filename := parseExportFilename(header.Get("Content-Disposition"))
	if filename == "" && contentType == "application/pdf" {
		filename = "document.pdf"
	}
	if shouldStoreRawResponseAsFile(contentType, storeCSVAsFile) {
		path, err := writeRawResponseTempFile(filename, contentType, body)
		if err == nil {
			return map[string]any{
				"contentType":  contentType,
				"filename":     filename,
				"bytes":        len(body),
				"bodyEncoding": "file",
				"truncated":    false,
				"path":         path,
			}
		}
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

func shouldStoreRawResponseAsFile(contentType string, storeCSVAsFile bool) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if ct == "" {
		return false
	}
	if strings.Contains(ct, "csv") && storeCSVAsFile {
		return true
	}
	if strings.Contains(ct, "text/") || strings.Contains(ct, "json") {
		return false
	}
	for _, marker := range []string{"pdf", "zip", "spreadsheet", "excel", "octet-stream"} {
		if strings.Contains(ct, marker) {
			return true
		}
	}
	return false
}

func writeRawResponseTempFile(filename, contentType string, body []byte) (string, error) {
	pruneStaleExports()
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		switch {
		case strings.Contains(contentType, "pdf"):
			ext = ".pdf"
		case strings.Contains(contentType, "zip"):
			ext = ".zip"
		case strings.Contains(contentType, "spreadsheet"), strings.Contains(contentType, "excel"):
			ext = ".xlsx"
		case strings.Contains(contentType, "csv"):
			ext = ".csv"
		default:
			ext = ".bin"
		}
	}
	file, err := os.CreateTemp("", "clockify-export-*"+ext)
	if err != nil {
		return "", err
	}
	if _, err := file.Write(body); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

// pruneStaleExports best-effort removes clockify export temp files older than
// one hour so binary-export results do not accumulate in the temp directory.
func pruneStaleExports() {
	matches, err := filepath.Glob(filepath.Join(os.TempDir(), "clockify-export-*"))
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-time.Hour)
	for _, name := range matches {
		if info, statErr := os.Stat(name); statErr == nil && !info.IsDir() && info.ModTime().Before(cutoff) {
			_ = os.Remove(name)
		}
	}
}
