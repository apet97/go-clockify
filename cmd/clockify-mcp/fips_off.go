//go:build !fips

package main

// fipsStartupCheck is the default-build no-op stub. The default
// binary does not assert FIPS mode — operators who want FIPS
// guarantees must rebuild with `-tags=fips` (ideally also with
// GOFIPS140=latest) so the startup check in fips_on.go fires. See
// ADR 011.
func fipsStartupCheck() {}
