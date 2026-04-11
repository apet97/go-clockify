//go:build fips

package main

import (
	"crypto/fips140"
	"fmt"
	"log/slog"
	"os"
)

// fipsStartupCheck is called from main's init chain when the binary
// was built with -tags=fips. It asserts that crypto/fips140 reports
// FIPS 140-3 mode is active; otherwise the binary exits fatally on
// startup. This protects operators who thought they were running a
// FIPS-compliant binary but forgot to build with GOFIPS140 or set
// GODEBUG=fips140=on at runtime.
//
// The `fips` build tag is paired with a second goreleaser `build`
// entry in .goreleaser.yaml that sets GOFIPS140=latest at compile
// time so the default FIPS binaries ship with the frozen FIPS
// cryptographic module. See ADR 011.
func fipsStartupCheck() {
	if !fips140.Enabled() {
		fmt.Fprintln(os.Stderr, "fatal: -tags=fips binary but crypto/fips140.Enabled() returned false")
		fmt.Fprintln(os.Stderr, "hint: rebuild with GOFIPS140=latest, or set GODEBUG=fips140=on in the environment")
		os.Exit(1)
	}
	slog.Info("fips140_enabled",
		"version", fips140.Version(),
		"enforced", fips140.Enforced(),
	)
}
