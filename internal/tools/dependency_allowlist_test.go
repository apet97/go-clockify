package tools

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// allowedDirectDependencies pins the root module's direct dependency set. Each
// entry must carry a justification: the root binary is a local one-user stdio
// MCP, so a new direct dependency is a deliberate decision, not an accident.
//
// The separate tools/govulncheck module is intentionally NOT covered here.
var allowedDirectDependencies = map[string]string{
	"gopkg.in/yaml.v3": "scripts/gen-raw-allowlist parses docs/openapi/clockify-openapi.yaml to regenerate the raw-write allowlist",
}

// TestRootModuleDirectDependenciesAreAllowlisted fails when go.mod gains an
// un-justified direct dependency, or when an allowlist entry no longer matches
// a direct require (a stale entry).
func TestRootModuleDirectDependenciesAreAllowlisted(t *testing.T) {
	root := dependencyTestRepoRoot(t)

	cmd := exec.Command("go", "mod", "edit", "-json")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go mod edit -json: %v", err)
	}
	var mod struct {
		Require []struct {
			Path     string
			Version  string
			Indirect bool
		}
	}
	if err := json.Unmarshal(out, &mod); err != nil {
		t.Fatalf("parse go mod edit -json output: %v", err)
	}

	direct := map[string]bool{}
	for _, r := range mod.Require {
		if r.Indirect {
			continue
		}
		direct[r.Path] = true
		if _, ok := allowedDirectDependencies[r.Path]; !ok {
			t.Errorf("root module declares un-allowlisted direct dependency %q; "+
				"add it to allowedDirectDependencies with a justification, or remove it", r.Path)
		}
	}
	for path := range allowedDirectDependencies {
		if !direct[path] {
			t.Errorf("allowlisted dependency %q is no longer a direct require in go.mod; "+
				"remove the stale allowlist entry", path)
		}
	}
}

// dependencyTestRepoRoot walks up from this test file until it finds the
// directory holding go.mod (the repository root). If the tools test package
// already defines an equivalent repo-root helper, reuse that instead and drop
// this function.
func dependencyTestRepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test file")
		}
		dir = parent
	}
}
