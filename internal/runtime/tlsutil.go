package runtime

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// loadCACertPool reads a PEM-encoded CA bundle from disk and returns a
// pool suitable for tls.Config.ClientCAs. Both the streamable HTTP and
// gRPC transports use this when MCP_AUTH_MODE=mtls and the operator has
// supplied MCP_MTLS_CA_CERT_PATH.
func loadCACertPool(path string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read mTLS CA cert %q: %w", path, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("mTLS CA cert %q: no valid PEM certificates found", path)
	}
	return pool, nil
}

// buildServerTLSConfig assembles a *tls.Config suitable for an MCP
// network listener. certPath/keyPath are the server's leaf certificate
// and private key (required when TLS is desired). caPath is the client
// CA bundle for mTLS verification — empty disables client cert
// verification (server-auth only TLS).
//
// minVersion lets callers pick the TLS floor: tls.VersionTLS13 for
// gRPC (matching the prior runGRPC behaviour) or tls.VersionTLS12 for
// streamable HTTP (broadest compatibility while still rejecting
// SSL/TLS 1.0/1.1).
//
// When requireClientCert is true and caPath is set, the listener will
// require + verify a client certificate during the TLS handshake, so
// the mtls authenticator can read r.TLS.VerifiedChains without the
// per-request "verified mTLS client certificate required" error path.
func buildServerTLSConfig(certPath, keyPath, caPath string, requireClientCert bool, minVersion uint16) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("load TLS keypair: %w", err)
	}
	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   minVersion,
	}
	if requireClientCert {
		if caPath == "" {
			return nil, fmt.Errorf("require client cert without MCP_MTLS_CA_CERT_PATH; cannot verify client certificates")
		}
		pool, err := loadCACertPool(caPath)
		if err != nil {
			return nil, err
		}
		cfg.ClientCAs = pool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return cfg, nil
}
