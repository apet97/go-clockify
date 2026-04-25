package vault

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/apet97/go-clockify/internal/controlplane"
)

// TestResolveWithOptions_InlineDisabled locks in the hosted-service
// hardening switch (MCP_DISABLE_INLINE_SECRETS=1). With the option
// set, any credential ref with backend=inline is rejected at
// resolution time regardless of the reference content; env and file
// backends continue to work.
func TestResolveWithOptions_InlineDisabled(t *testing.T) {
	t.Run("inline_rejected_when_disabled", func(t *testing.T) {
		ref := controlplane.CredentialRef{Backend: "inline", Reference: "abc-key"}
		_, err := ResolveWithOptions(ref, Options{DisableInline: true})
		if err == nil {
			t.Fatal("expected inline rejection when DisableInline=true")
		}
	})
	t.Run("inline_allowed_when_enabled", func(t *testing.T) {
		ref := controlplane.CredentialRef{Backend: "inline", Reference: "abc-key"}
		m, err := ResolveWithOptions(ref, Options{DisableInline: false})
		if err != nil {
			t.Fatalf("inline should resolve when DisableInline=false: %v", err)
		}
		if m.APIKey != "abc-key" {
			t.Errorf("APIKey = %q, want abc-key", m.APIKey)
		}
	})
	t.Run("env_backend_unaffected_by_flag", func(t *testing.T) {
		t.Setenv("VAULT_TEST_ENVVAR", "env-key")
		ref := controlplane.CredentialRef{Backend: "env", Reference: "VAULT_TEST_ENVVAR"}
		m, err := ResolveWithOptions(ref, Options{DisableInline: true})
		if err != nil {
			t.Fatalf("env backend must work with DisableInline=true: %v", err)
		}
		if m.APIKey != "env-key" {
			t.Errorf("APIKey = %q, want env-key", m.APIKey)
		}
	})
	t.Run("file_backend_unaffected_by_flag", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "cred")
		if err := os.WriteFile(path, []byte("file-key\n"), 0o600); err != nil {
			t.Fatalf("write temp file: %v", err)
		}
		ref := controlplane.CredentialRef{Backend: "file", Reference: path}
		m, err := ResolveWithOptions(ref, Options{DisableInline: true})
		if err != nil {
			t.Fatalf("file backend must work with DisableInline=true: %v", err)
		}
		if m.APIKey != "file-key" {
			t.Errorf("APIKey = %q, want file-key", m.APIKey)
		}
	})
	t.Run("legacy_resolve_preserves_inline", func(t *testing.T) {
		// The back-compat Resolve wrapper must keep the permissive
		// default. Callers who haven't migrated to ResolveWithOptions
		// see no behaviour change.
		ref := controlplane.CredentialRef{Backend: "inline", Reference: "abc-key"}
		if _, err := Resolve(ref); err != nil {
			t.Fatalf("legacy Resolve should permit inline: %v", err)
		}
	})
}

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
