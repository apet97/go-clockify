//go:build legacy_platform

package runtime

import (
	"crypto/tls"
	"testing"
)

func TestStreamableHTTPMinTLSVersion(t *testing.T) {
	tests := []struct {
		profile string
		want    uint16
	}{
		{profile: "shared-service", want: tls.VersionTLS13},
		{profile: "prod-postgres", want: tls.VersionTLS13},
		{profile: "single-tenant-http", want: tls.VersionTLS12},
		{profile: "", want: tls.VersionTLS12},
	}
	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			if got := streamableHTTPMinTLSVersion(tt.profile); got != tt.want {
				t.Fatalf("streamableHTTPMinTLSVersion(%q)=%x want %x", tt.profile, got, tt.want)
			}
		})
	}
}
