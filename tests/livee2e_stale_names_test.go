package e2e_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestLiveE2EFilesDoNotMentionRemovedOneUserTools(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	testsDir := filepath.Dir(file)

	const p = "clockify_"
	removed := []string{
		p + "whoami",
		p + "get_" + "workspace",
		p + "list_" + "projects",
		p + "create_" + "client",
		p + "create_" + "project",
		p + "start_" + "timer",
		p + "stop_" + "timer",
		p + "delete_" + "entry",
		p + "get_" + "entry",
		p + "activate_" + "group",
		p + "policy_" + "info",
		p + "list_" + "tools",
		p + "log_" + "time",
		p + "resolve_" + "name",
		p + "timesheet_" + "review",
		p + "switch_" + "project",
		p + "timer_" + "status",
	}

	entries, err := os.ReadDir(testsDir)
	if err != nil {
		t.Fatalf("read tests dir: %v", err)
	}

	var failures []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if entry.Name() == filepath.Base(file) {
			continue
		}
		path := filepath.Join(testsDir, entry.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		src := string(b)
		if !strings.Contains(src, "//go:build livee2e") {
			continue
		}
		for _, name := range removed {
			pattern := regexp.MustCompile(`(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(name) + `([^A-Za-z0-9_]|$)`)
			if pattern.MatchString(src) {
				failures = append(failures, entry.Name()+": "+name)
			}
		}
	}

	if len(failures) > 0 {
		t.Fatalf("livee2e files mention removed one-user tools:\n%s", strings.Join(failures, "\n"))
	}
}
