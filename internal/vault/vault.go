package vault

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/apet97/go-clockify/internal/controlplane"
)

type Material struct {
	APIKey    string
	Workspace string
	BaseURL   string
}

// Options tunes credential resolution behaviour. Zero value preserves
// the historical permissive default (all three backends enabled).
type Options struct {
	// DisableInline rejects credential refs with backend=inline.
	// Intended for hosted-service deployments where operators must
	// use env/file/external vault backends so secrets never land in
	// control-plane records or logs. Driven by the
	// MCP_DISABLE_INLINE_SECRETS env var.
	DisableInline bool
}

// Resolve returns the credential material for the given ref under the
// permissive default (all backends enabled). Retained for back-compat;
// new call sites should prefer ResolveWithOptions so they can opt out
// of the inline backend for hosted-service hardening.
func Resolve(ref controlplane.CredentialRef) (Material, error) {
	return ResolveWithOptions(ref, Options{})
}

// ResolveWithOptions is the Options-aware variant of Resolve. In the
// default Options zero value it behaves exactly like Resolve.
func ResolveWithOptions(ref controlplane.CredentialRef, opts Options) (Material, error) {
	switch strings.ToLower(strings.TrimSpace(ref.Backend)) {
	case "inline":
		if opts.DisableInline {
			return Material{}, fmt.Errorf("inline credential backend disabled by MCP_DISABLE_INLINE_SECRETS=1")
		}
		return inline(ref)
	case "env":
		return fromEnv(ref)
	case "file":
		return fromFile(ref)
	default:
		return Material{}, fmt.Errorf("unsupported vault backend %q", ref.Backend)
	}
}

func inline(ref controlplane.CredentialRef) (Material, error) {
	if ref.Reference == "" {
		return Material{}, fmt.Errorf("inline credential reference is empty")
	}
	if strings.HasPrefix(strings.TrimSpace(ref.Reference), "{") {
		return decodeMaterial(ref.Reference, ref)
	}
	return Material{
		APIKey:    ref.Reference,
		Workspace: ref.Workspace,
		BaseURL:   ref.BaseURL,
	}, nil
}

func fromEnv(ref controlplane.CredentialRef) (Material, error) {
	name := strings.TrimSpace(ref.Reference)
	if name == "" {
		return Material{}, fmt.Errorf("env credential reference is empty")
	}
	value := os.Getenv(name)
	if value == "" {
		return Material{}, fmt.Errorf("env credential %q is empty", name)
	}
	if strings.HasPrefix(strings.TrimSpace(value), "{") {
		return decodeMaterial(value, ref)
	}
	return Material{
		APIKey:    strings.TrimSpace(value),
		Workspace: ref.Workspace,
		BaseURL:   ref.BaseURL,
	}, nil
}

func fromFile(ref controlplane.CredentialRef) (Material, error) {
	path := strings.TrimSpace(ref.Reference)
	if path == "" {
		return Material{}, fmt.Errorf("file credential reference is empty")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Material{}, fmt.Errorf("read credential file: %w", err)
	}
	raw := strings.TrimSpace(string(b))
	if raw == "" {
		return Material{}, fmt.Errorf("credential file %q is empty", path)
	}
	if strings.HasPrefix(raw, "{") {
		return decodeMaterial(raw, ref)
	}
	return Material{
		APIKey:    raw,
		Workspace: ref.Workspace,
		BaseURL:   ref.BaseURL,
	}, nil
}

func decodeMaterial(raw string, ref controlplane.CredentialRef) (Material, error) {
	var payload struct {
		APIKey    string `json:"api_key"`
		Workspace string `json:"workspace_id"`
		BaseURL   string `json:"base_url"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return Material{}, fmt.Errorf("decode credential payload: %w", err)
	}
	if strings.TrimSpace(payload.APIKey) == "" {
		return Material{}, fmt.Errorf("credential payload is missing api_key")
	}
	out := Material{
		APIKey:    strings.TrimSpace(payload.APIKey),
		Workspace: strings.TrimSpace(payload.Workspace),
		BaseURL:   strings.TrimSpace(payload.BaseURL),
	}
	if out.Workspace == "" {
		out.Workspace = ref.Workspace
	}
	if out.BaseURL == "" {
		out.BaseURL = ref.BaseURL
	}
	return out, nil
}
