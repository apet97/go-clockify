package mcp

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/controlplane"
)

// TestStreamableHTTPNativeTLS proves that the streamable HTTP transport
// terminates TLS in-process when StreamableHTTPOptions.TLSConfig is non-nil.
// The /health endpoint is reachable over HTTPS via tls.Dial; a plain HTTP
// dial against the same listener fails the handshake.
func TestStreamableHTTPNativeTLS(t *testing.T) {
	t.Parallel()
	ln, addr := newLoopbackListener(t)
	serverCert := newSelfSignedCert(t, "127.0.0.1")
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{serverCert.tlsCert},
		MinVersion:   tls.VersionTLS12,
	}

	stop, done := serveStreamableForTLSTest(t, ln, tlsCfg)
	defer stop()

	// Successful TLS dial → 200 on /health.
	pool := x509.NewCertPool()
	pool.AddCert(serverCert.x509)
	clientTLS := &tls.Config{RootCAs: pool, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12}
	resp := getOverTLS(t, addr, "/health", clientTLS, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from /health, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// Plain HTTP against a TLS listener: Go's HTTP server detects this
	// and replies with a 400 + "Client sent an HTTP request to an HTTPS
	// server" message before closing. That body, not silence, is the
	// proof that TLS termination is active.
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial plain: %v", err)
	}
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("GET /health HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n")); err != nil {
		t.Fatalf("write plain: %v", err)
	}
	buf := make([]byte, 256)
	n, _ := conn.Read(buf)
	got := string(buf[:n])
	if !strings.Contains(got, "400 Bad Request") || !strings.Contains(got, "HTTPS") {
		t.Fatalf("expected 400/HTTPS rejection from TLS listener over plain HTTP, got %q", got)
	}
	_ = conn.Close()

	// Drive shutdown and wait for ServeStreamableHTTP to return.
	stop()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("ServeStreamableHTTP did not return after shutdown")
	}
}

// TestStreamableHTTPNativeMTLS proves the listener-level mTLS handshake.
// A client without a cert is rejected at the TLS layer (no
// authenticator round-trip; no MCP error envelope). A client with a
// valid client cert reaches /health.
func TestStreamableHTTPNativeMTLS(t *testing.T) {
	t.Parallel()
	ln, addr := newLoopbackListener(t)
	serverCert := newSelfSignedCert(t, "127.0.0.1")
	caCert := newSelfSignedCA(t)
	clientCert := newClientCert(t, caCert)

	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(caCert.x509)
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{serverCert.tlsCert},
		ClientCAs:    clientCAs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}

	stop, done := serveStreamableForTLSTest(t, ln, tlsCfg)
	defer stop()

	rootPool := x509.NewCertPool()
	rootPool.AddCert(serverCert.x509)

	// Client without a cert → request fails. With TLS 1.2 the failure
	// surfaces during the handshake; with TLS 1.3 it can be deferred to
	// the first read. Either way, the request must not reach an HTTP
	// status — accept either error path here as evidence that the
	// listener-level mTLS check is active.
	noCertCfg := &tls.Config{RootCAs: rootPool, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12}
	if conn, err := tlsDial(addr, noCertCfg); err == nil {
		// TLS 1.2 normally fails the handshake; if it slipped through,
		// confirm the next byte read errors out (server tore the
		// connection down). Either is acceptable.
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		if _, werr := conn.Write([]byte("GET /health HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n")); werr == nil {
			buf := make([]byte, 16)
			if _, rerr := conn.Read(buf); rerr == nil {
				t.Fatalf("mTLS without client cert produced response %q; expected handshake or read failure", string(buf))
			}
		}
		_ = conn.Close()
	}

	// Client with a valid cert → /health returns 200.
	withCertCfg := &tls.Config{
		RootCAs:      rootPool,
		ServerName:   "127.0.0.1",
		Certificates: []tls.Certificate{clientCert.tlsCert},
		MinVersion:   tls.VersionTLS12,
	}
	resp := getOverTLS(t, addr, "/health", withCertCfg, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from /health with valid client cert, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	stop()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("ServeStreamableHTTP did not return after shutdown")
	}
}

// serveStreamableForTLSTest wires a minimal authenticator, an in-memory
// control-plane store, and starts ServeStreamableHTTP in a goroutine on
// the supplied listener. Returns a stop function (cancels the server's
// context) and a done channel that closes when ServeStreamableHTTP
// returns. The /health endpoint is always served regardless of
// authentication.
func serveStreamableForTLSTest(t *testing.T, ln net.Listener, tlsCfg *tls.Config) (stop func(), done <-chan struct{}) {
	t.Helper()
	authenticator, err := authn.New(authn.Config{
		Mode:            authn.ModeStaticBearer,
		BearerToken:     testBearerToken,
		DefaultTenantID: "tenant-tls",
	})
	if err != nil {
		t.Fatalf("authenticator: %v", err)
	}
	store, err := controlplane.Open("memory")
	if err != nil {
		t.Fatalf("control plane: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	doneCh := make(chan struct{})
	var once sync.Once
	go func() {
		defer close(doneCh)
		err := ServeStreamableHTTP(ctx, StreamableHTTPOptions{
			Version:       "tls-test",
			Listener:      ln,
			MaxBodySize:   2097152,
			SessionTTL:    30 * time.Minute,
			Authenticator: authenticator,
			ControlPlane:  store,
			TLSConfig:     tlsCfg,
			Factory: func(_ context.Context, principal authn.Principal, _ string) (*StreamableSessionRuntime, error) {
				return &StreamableSessionRuntime{
					Server:          NewServer("tls-test", nil, nil, nil),
					Close:           func() {},
					TenantID:        principal.TenantID,
					WorkspaceID:     "ws1",
					ClockifyBaseURL: "https://api.clockify.me/api/v1",
				}, nil
			},
		})
		if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, context.Canceled) {
			t.Logf("ServeStreamableHTTP returned: %v", err)
		}
	}()
	stop = func() { once.Do(cancel) }
	return stop, doneCh
}

func newLoopbackListener(t *testing.T) (net.Listener, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return ln, ln.Addr().String()
}

type ephemeralCert struct {
	tlsCert tls.Certificate
	x509    *x509.Certificate
	key     *ecdsa.PrivateKey
}

func newSelfSignedCert(t *testing.T, host string) ephemeralCert {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP(host)},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	return ephemeralCert{
		tlsCert: tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: parsed},
		x509:    parsed,
		key:     key,
	}
}

func newSelfSignedCA(t *testing.T) ephemeralCert {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ca key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "test-mtls-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create ca cert: %v", err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse ca cert: %v", err)
	}
	return ephemeralCert{
		tlsCert: tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: parsed},
		x509:    parsed,
		key:     key,
	}
}

func newClientCert(t *testing.T, ca ephemeralCert) ephemeralCert {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "test-mtls-client", Organization: []string{"tenant-tls"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.x509, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatalf("create client cert: %v", err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse client cert: %v", err)
	}
	return ephemeralCert{
		tlsCert: tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: parsed},
		x509:    parsed,
		key:     key,
	}
}

func tlsDial(addr string, cfg *tls.Config) (*tls.Conn, error) {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return nil, err
	}
	tc := tls.Client(conn, cfg)
	_ = tc.SetDeadline(time.Now().Add(3 * time.Second))
	if err := tc.Handshake(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return tc, nil
}

// getOverTLS speaks raw HTTP/1.1 over a tls.Conn and parses the response.
// Avoids spinning up an http.Client with cookies / connection pooling so
// the test stays focused on the handshake outcome.
func getOverTLS(t *testing.T, addr, path string, cfg *tls.Config, _ http.Header) *http.Response {
	t.Helper()
	conn, err := tlsDial(addr, cfg)
	if err != nil {
		t.Fatalf("tls dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	req := "GET " + path + " HTTP/1.1\r\nHost: " + addr + "\r\nConnection: close\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatalf("write request: %v", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return resp
}
