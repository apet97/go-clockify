// Command gen-config-docs renders config/help documentation from the
// internal/config EnvSpec registry. It is the only writer of
// cmd/clockify-mcp/help_generated.go and the bounded CONFIG-TABLE block
// in README.md. Run via `go run ./cmd/gen-config-docs -mode=all`; the
// config-doc-parity CI gate fails on uncommitted drift.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/apet97/go-clockify/internal/config"
)

const (
	readmeBeginMarker = "<!-- CONFIG-TABLE BEGIN"
	readmeEndMarker   = "<!-- CONFIG-TABLE END -->"
)

func main() {
	mode := flag.String("mode", "all", "help | readme | all")
	flag.Parse()
	specs := config.AllSpecs()

	switch *mode {
	case "help":
		fmt.Print(RenderHelp(specs))
	case "readme":
		fmt.Print(RenderREADMETable(specs))
	case "all":
		writeFile(filepath.Join("cmd", "clockify-mcp", "help_generated.go"), renderHelpGo(specs))
		if err := writeREADME(specs); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown mode %q\n", *mode)
		os.Exit(2)
	}
}

func writeFile(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		panic(err)
	}
}

// writeREADME rewrites the region between readmeBeginMarker and
// readmeEndMarker with a freshly rendered table. The markers must exist
// already; adding them is a one-time manual step (see Task 3).
func writeREADME(specs []config.EnvSpec) error {
	const path = "README.md"
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	body := string(raw)
	i := strings.Index(body, readmeBeginMarker)
	j := strings.Index(body, readmeEndMarker)
	if i < 0 || j < 0 {
		return fmt.Errorf("%s: missing CONFIG-TABLE markers; add %q and %q once, then rerun", path, readmeBeginMarker, readmeEndMarker)
	}
	rebuilt := body[:i] + RenderREADMETable(specs) + body[j+len(readmeEndMarker):]
	return os.WriteFile(path, []byte(rebuilt), 0o644)
}
