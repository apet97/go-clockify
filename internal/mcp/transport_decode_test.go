package mcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeSingleRequestRejectsTrailingJSONValue(t *testing.T) {
	body := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"} {"jsonrpc":"2.0","id":2,"method":"ping"}`)

	if _, tooLarge, err := decodeSingleRequest(body); err == nil || tooLarge {
		t.Fatalf("decodeSingleRequest trailing value err=%v tooLarge=%v, want invalid JSON without tooLarge", err, tooLarge)
	}
}

func TestDecodeSingleRequestAllowsTrailingWhitespace(t *testing.T) {
	req, tooLarge, err := decodeSingleRequest(strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"ping\"}\n\t "))
	if err != nil || tooLarge {
		t.Fatalf("decodeSingleRequest whitespace err=%v tooLarge=%v", err, tooLarge)
	}
	if req.Method != "ping" {
		t.Fatalf("Method=%q, want ping", req.Method)
	}
}

func TestDecodeSingleRequestReportsMaxBytes(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Body = http.MaxBytesReader(rec, req.Body, 8)

	if _, tooLarge, err := decodeSingleRequest(req.Body); err == nil || !tooLarge {
		t.Fatalf("decodeSingleRequest oversized err=%v tooLarge=%v, want tooLarge", err, tooLarge)
	}
}
