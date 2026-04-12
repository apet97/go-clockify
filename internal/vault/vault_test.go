package vault

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/apet97/go-clockify/internal/controlplane"
)

// TestResolveBackends covers every backend dispatch path in Resolve, including
// the unsupported-backend error branch.
func TestResolveBackends(t *testing.T) {
	cases := []struct {
		name    string
		ref     controlplane.CredentialRef
		setup   func(t *testing.T)
		wantErr bool
		wantKey string
	}{
		{
			name: "inline_string",
			ref: controlplane.CredentialRef{
				Backend:   "inline",
				Reference: "abc-key",
				Workspace: "ws-1",
				BaseURL:   "https://api.example.com",
			},
			wantKey: "abc-key",
		},
		{
			name: "inline_json_payload",
			ref: controlplane.CredentialRef{
				Backend:   "INLINE",
				Reference: `{"api_key":"json-key","workspace_id":"ws-2","base_url":"https://api2.example.com"}`,
			},
			wantKey: "json-key",
		},
		{
			name: "unsupported_backend",
			ref: controlplane.CredentialRef{
				Backend:   "weird",
				Reference: "x",
			},
			wantErr: true,
		},
		{
			name: "inline_empty_reference",
			ref: controlplane.CredentialRef{
				Backend: "inline",
			},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			m, err := Resolve(tc.ref)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", m)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if m.APIKey != tc.wantKey {
				t.Fatalf("APIKey: got %q want %q", m.APIKey, tc.wantKey)
			}
		})
	}
}

// TestResolveEnvBackend exercises the env backend including JSON payload
// decoding from an env var and the empty/missing env failure paths.
func TestResolveEnvBackend(t *testing.T) {
	t.Setenv("VAULT_TEST_PLAIN", "  plain-key  ")
	t.Setenv("VAULT_TEST_JSON", `{"api_key":"env-json-key","workspace_id":"ws-env"}`)
	t.Setenv("VAULT_TEST_EMPTY", "")

	m, err := Resolve(controlplane.CredentialRef{Backend: "env", Reference: "VAULT_TEST_PLAIN", Workspace: "fallback"})
	if err != nil {
		t.Fatalf("plain env: %v", err)
	}
	if m.APIKey != "plain-key" || m.Workspace != "fallback" {
		t.Fatalf("plain env unexpected: %+v", m)
	}

	m, err = Resolve(controlplane.CredentialRef{Backend: "env", Reference: "VAULT_TEST_JSON", Workspace: "fallback", BaseURL: "https://default.example.com"})
	if err != nil {
		t.Fatalf("json env: %v", err)
	}
	if m.APIKey != "env-json-key" || m.Workspace != "ws-env" || m.BaseURL != "https://default.example.com" {
		t.Fatalf("json env unexpected: %+v", m)
	}

	if _, err := Resolve(controlplane.CredentialRef{Backend: "env", Reference: "VAULT_TEST_EMPTY"}); err == nil {
		t.Fatal("expected empty-env error")
	}
	if _, err := Resolve(controlplane.CredentialRef{Backend: "env", Reference: ""}); err == nil {
		t.Fatal("expected empty-reference error")
	}
}

// TestResolveFileBackend covers the file backend with both plain and JSON
// payloads, plus error paths for empty file and missing file.
func TestResolveFileBackend(t *testing.T) {
	dir := t.TempDir()

	plainPath := filepath.Join(dir, "plain.key")
	if err := os.WriteFile(plainPath, []byte("file-plain-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	jsonPath := filepath.Join(dir, "payload.json")
	if err := os.WriteFile(jsonPath, []byte(`{"api_key":"file-json-key","workspace_id":"ws-file"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	emptyPath := filepath.Join(dir, "empty")
	if err := os.WriteFile(emptyPath, []byte("   \n"), 0o600); err != nil {
		t.Fatal(err)
	}

	m, err := Resolve(controlplane.CredentialRef{Backend: "file", Reference: plainPath, Workspace: "ws-default"})
	if err != nil {
		t.Fatalf("plain file: %v", err)
	}
	if m.APIKey != "file-plain-key" || m.Workspace != "ws-default" {
		t.Fatalf("plain file unexpected: %+v", m)
	}

	m, err = Resolve(controlplane.CredentialRef{Backend: "file", Reference: jsonPath})
	if err != nil {
		t.Fatalf("json file: %v", err)
	}
	if m.APIKey != "file-json-key" || m.Workspace != "ws-file" {
		t.Fatalf("json file unexpected: %+v", m)
	}

	if _, err := Resolve(controlplane.CredentialRef{Backend: "file", Reference: emptyPath}); err == nil {
		t.Fatal("expected empty-file error")
	}
	if _, err := Resolve(controlplane.CredentialRef{Backend: "file", Reference: filepath.Join(dir, "missing")}); err == nil {
		t.Fatal("expected missing-file error")
	}
	if _, err := Resolve(controlplane.CredentialRef{Backend: "file", Reference: ""}); err == nil {
		t.Fatal("expected empty-reference error")
	}
}

// TestDecodeMaterialMissingAPIKey ensures the JSON decoder rejects payloads
// without an api_key field.
func TestDecodeMaterialMissingAPIKey(t *testing.T) {
	if _, err := Resolve(controlplane.CredentialRef{
		Backend:   "inline",
		Reference: `{"workspace_id":"ws-only"}`,
	}); err == nil {
		t.Fatal("expected api_key missing error")
	}
}
