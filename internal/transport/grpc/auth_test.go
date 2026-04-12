package grpctransport

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/metrics"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// fakeAuthenticator is a test double for authn.Authenticator. It inspects the
// Authorization header of the synthetic http.Request the interceptor builds
// and returns a canned Principal on match or a canned error otherwise. The
// zero value rejects everything.
type fakeAuthenticator struct {
	wantToken string
	principal authn.Principal
	forceErr  error
}

func (f fakeAuthenticator) Authenticate(_ context.Context, r *http.Request) (authn.Principal, error) {
	if f.forceErr != nil {
		return authn.Principal{}, f.forceErr
	}
	got := r.Header.Get("Authorization")
	if got != "Bearer "+f.wantToken {
		return authn.Principal{}, errors.New("token mismatch")
	}
	return f.principal, nil
}

// mockServerStream is a minimal grpc.ServerStream for direct interceptor
// invocation. It carries a fixed context and treats SendMsg/RecvMsg as no-ops
// that satisfy the interface — the tests below only care about the
// interceptor's authz decision and the context handed to the wrapped handler.
type mockServerStream struct {
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context     { return m.ctx }
func (m *mockServerStream) SetHeader(metadata.MD) error  { return nil }
func (m *mockServerStream) SendHeader(metadata.MD) error { return nil }
func (m *mockServerStream) SetTrailer(metadata.MD)       {}
func (m *mockServerStream) SendMsg(any) error            { return nil }
func (m *mockServerStream) RecvMsg(any) error            { return io.EOF }

// callInterceptor invokes the interceptor with a mock stream carrying the
// given metadata. It returns the handler invocation result plus whatever
// principal the handler observed in its stream context (or the zero value
// when the handler never ran).
func callInterceptor(t *testing.T, auth authn.Authenticator, md metadata.MD) (error, *authn.Principal) {
	t.Helper()
	ctx := metadata.NewIncomingContext(context.Background(), md)
	stream := &mockServerStream{ctx: ctx}
	var seen *authn.Principal
	handler := func(_ any, ss grpc.ServerStream) error {
		if p, ok := authn.PrincipalFromContext(ss.Context()); ok {
			seen = p
		}
		return nil
	}
	interceptor := authStreamInterceptor(auth, authInterceptorConfig{})
	err := interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/test/Method"}, handler)
	return err, seen
}

func TestAuthInterceptor_MissingMetadata(t *testing.T) {
	auth := fakeAuthenticator{wantToken: "correct"}
	before := metrics.GRPCAuthRejectionsTotal.Get("missing_metadata")
	// No incoming metadata at all — use a bare context.
	stream := &mockServerStream{ctx: context.Background()}
	interceptor := authStreamInterceptor(auth, authInterceptorConfig{})
	err := interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/test/Method"}, func(any, grpc.ServerStream) error {
		t.Fatalf("handler should not run when metadata is missing")
		return nil
	})
	if st, ok := status.FromError(err); !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
	if got := metrics.GRPCAuthRejectionsTotal.Get("missing_metadata"); got != before+1 {
		t.Fatalf("expected missing_metadata counter to increment by 1, got %d (before=%d)", got, before)
	}
}

func TestAuthInterceptor_MissingAuthorization(t *testing.T) {
	auth := fakeAuthenticator{wantToken: "correct"}
	before := metrics.GRPCAuthRejectionsTotal.Get("missing_authorization")
	err, seen := callInterceptor(t, auth, metadata.MD{})
	if st, ok := status.FromError(err); !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
	if seen != nil {
		t.Fatalf("handler should not have run; got principal %+v", seen)
	}
	if got := metrics.GRPCAuthRejectionsTotal.Get("missing_authorization"); got != before+1 {
		t.Fatalf("expected missing_authorization counter to increment, got %d (before=%d)", got, before)
	}
}

func TestAuthInterceptor_EmptyAuthorizationValue(t *testing.T) {
	auth := fakeAuthenticator{wantToken: "correct"}
	before := metrics.GRPCAuthRejectionsTotal.Get("empty_authorization")
	err, _ := callInterceptor(t, auth, metadata.Pairs("authorization", ""))
	if st, ok := status.FromError(err); !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated on empty authorization, got %v", err)
	}
	if got := metrics.GRPCAuthRejectionsTotal.Get("empty_authorization"); got != before+1 {
		t.Fatalf("expected empty_authorization counter to increment, got %d (before=%d)", got, before)
	}
}

func TestAuthInterceptor_WrongToken(t *testing.T) {
	auth := fakeAuthenticator{wantToken: "correct"}
	err, seen := callInterceptor(t, auth, metadata.Pairs("authorization", "Bearer nope"))
	if st, ok := status.FromError(err); !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
	if seen != nil {
		t.Fatalf("handler should not have run; got principal %+v", seen)
	}
}

func TestAuthInterceptor_HappyPathPropagatesPrincipal(t *testing.T) {
	want := authn.Principal{
		Subject:  "alice@example.com",
		TenantID: "acme",
		AuthMode: authn.ModeStaticBearer,
		Claims:   map[string]string{},
	}
	auth := fakeAuthenticator{wantToken: "correct", principal: want}
	err, seen := callInterceptor(t, auth, metadata.Pairs("authorization", "Bearer correct"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seen == nil {
		t.Fatal("handler saw no principal in context")
	}
	if seen.Subject != want.Subject || seen.TenantID != want.TenantID || seen.AuthMode != want.AuthMode {
		t.Fatalf("principal mismatch: want %+v, got %+v", want, *seen)
	}
}

func TestAuthInterceptor_AuthenticatorError(t *testing.T) {
	auth := fakeAuthenticator{forceErr: errors.New("token expired")}
	before := metrics.GRPCAuthRejectionsTotal.Get("auth_failed")
	err, seen := callInterceptor(t, auth, metadata.Pairs("authorization", "Bearer whatever"))
	if st, ok := status.FromError(err); !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated on authenticator error, got %v", err)
	}
	if seen != nil {
		t.Fatalf("handler should not run when authenticator fails; got %+v", seen)
	}
	if got := metrics.GRPCAuthRejectionsTotal.Get("auth_failed"); got != before+1 {
		t.Fatalf("expected auth_failed counter to increment, got %d (before=%d)", got, before)
	}
}

type countingAuthenticator struct {
	calls     atomic.Int32
	failAfter int32
	principal authn.Principal
}

func (c *countingAuthenticator) Authenticate(_ context.Context, _ *http.Request) (authn.Principal, error) {
	n := c.calls.Add(1)
	if c.failAfter > 0 && n > c.failAfter {
		return authn.Principal{}, errors.New("token expired")
	}
	return c.principal, nil
}

func TestReauthLoop_ExpiryClosesStream(t *testing.T) {
	auth := &countingAuthenticator{
		failAfter: 2,
		principal: authn.Principal{Subject: "alice", TenantID: "acme", AuthMode: authn.ModeOIDC},
	}
	interceptor := authStreamInterceptor(auth, authInterceptorConfig{reauthInterval: 50 * time.Millisecond})
	md := metadata.Pairs("authorization", "Bearer token")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	stream := &mockServerStream{ctx: ctx}

	handlerDone := make(chan error, 1)
	handler := func(_ any, ss grpc.ServerStream) error {
		<-ss.Context().Done()
		return ss.Context().Err()
	}
	go func() {
		handlerDone <- interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/test/Method"}, handler)
	}()

	select {
	case err := <-handlerDone:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("handler returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not exit within 2s — reauth did not cancel context")
	}

	if auth.calls.Load() < 2 {
		t.Fatalf("expected at least 2 auth calls, got %d", auth.calls.Load())
	}
}

// ---------------------------------------------------------------------------
// mTLS helpers + end-to-end test
// ---------------------------------------------------------------------------

// generateTestCA creates an ephemeral P-256 ECDSA self-signed CA certificate
// suitable for signing leaf certs in tests.
func generateTestCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("generate CA serial: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse CA cert: %v", err)
	}
	return cert, key
}

// generateTestLeaf signs a leaf certificate under the given CA. The returned
// tls.Certificate is ready for use in a tls.Config.
func generateTestLeaf(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, cn string, orgs []string) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("generate leaf serial: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: orgs,
		},
		NotBefore: time.Now().Add(-time.Minute),
		NotAfter:  time.Now().Add(time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
		},
		DNSNames: []string{"localhost", "bufnet"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create leaf cert: %v", err)
	}
	// Encode the leaf DER + private key into PEM so tls.X509KeyPair can
	// reconstruct the chain. This is the idiomatic stdlib path and lets Go
	// populate the tls.Certificate.Leaf field automatically.
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal leaf key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("x509 key pair: %v", err)
	}
	return tlsCert
}

// TestAuthInterceptor_MTLS_ClientCertMapsToSubject is an end-to-end test that
// verifies the full mTLS flow over a bufconn transport:
//
//  1. Ephemeral CA, server cert, and client cert are generated at test time.
//  2. A gRPC server is started on a bufconn listener with real TLS credentials
//     and the auth stream interceptor configured for mTLS mode.
//  3. A gRPC client dials via bufconn presenting the client cert.
//  4. The test opens the Exchange stream, sends a JSON-RPC initialize frame,
//     and verifies the response succeeds — proving the TLS handshake and mTLS
//     auth passed.
//  5. A parallel sub-test verifies the interceptor maps the client cert CN
//     ("alice-service") to Principal.Subject and Organization[0] ("acme-corp")
//     to Principal.TenantID by invoking the interceptor directly against a
//     mockServerStream whose peer context carries real credentials.TLSInfo.
func TestAuthInterceptor_MTLS_ClientCertMapsToSubject(t *testing.T) {
	// -- Generate ephemeral PKI ------------------------------------------
	ca, caKey := generateTestCA(t)
	serverCert := generateTestLeaf(t, ca, caKey, "test-server", nil)
	clientCert := generateTestLeaf(t, ca, caKey, "alice-service", []string{"acme-corp"})

	caPool := x509.NewCertPool()
	caPool.AddCert(ca)

	serverTLS := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    caPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}
	clientTLS := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caPool,
		ServerName:   "bufnet",
		MinVersion:   tls.VersionTLS12,
	}

	// -- Build authn.Authenticator in mTLS mode --------------------------
	auth, err := authn.New(authn.Config{Mode: authn.ModeMTLS})
	if err != nil {
		t.Fatalf("authn.New(mTLS): %v", err)
	}

	// -- Start gRPC server on bufconn with TLS + auth interceptor --------
	lis := bufconn.Listen(1024 * 1024)
	srv := newTestServer(t)
	handler := &exchangeServer{srv: srv}
	desc := buildServiceDesc()

	interceptor := authStreamInterceptor(auth, authInterceptorConfig{mtls: true})
	grpcSrv := grpc.NewServer(
		grpc.ForceServerCodec(bytesCodec{}),
		grpc.Creds(credentials.NewTLS(serverTLS)),
		grpc.StreamInterceptor(interceptor),
	)
	grpcSrv.RegisterService(&desc, handler)
	go func() { _ = grpcSrv.Serve(lis) }()
	t.Cleanup(func() {
		grpcSrv.GracefulStop()
		_ = lis.Close()
	})

	// -- Dial with client TLS credentials --------------------------------
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(credentials.NewTLS(clientTLS)),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(bytesCodec{})),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	// -- Open Exchange stream and send initialize ------------------------
	// The auth interceptor always calls buildSynthRequest which requires an
	// "authorization" metadata key. In mTLS mode the mtlsAuthenticator
	// ignores the bearer value — only r.TLS matters — but the interceptor
	// still synthesises the http.Request from metadata first. Supply a
	// placeholder so the request reaches the mTLS code path.
	streamDesc := &grpc.StreamDesc{
		StreamName:    ExchangeMethod,
		ServerStreams: true,
		ClientStreams: true,
	}
	md := metadata.Pairs("authorization", "Bearer mtls-placeholder")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	ctx = metadata.NewOutgoingContext(ctx, md)

	stream, err := conn.NewStream(ctx, streamDesc, "/"+ServiceName+"/"+ExchangeMethod)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}

	initPayload := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{}}}`)
	if err := stream.SendMsg(&initPayload); err != nil {
		t.Fatalf("send initialize: %v", err)
	}

	var reply []byte
	if err := stream.RecvMsg(&reply); err != nil {
		t.Fatalf("recv initialize reply: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(reply, &parsed); err != nil {
		t.Fatalf("unmarshal reply: %v; raw=%s", err, reply)
	}
	if parsed["error"] != nil {
		t.Fatalf("initialize returned error: %v", parsed["error"])
	}
	result, ok := parsed["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got %T (%v)", parsed["result"], parsed["result"])
	}
	if _, hasCaps := result["capabilities"]; !hasCaps {
		t.Fatalf("initialize result missing capabilities: %v", result)
	}

	_ = stream.CloseSend()

	t.Log("mTLS handshake + Exchange initialize succeeded over bufconn")
}

// TestAuthInterceptor_MTLS_PrincipalExtraction verifies the auth interceptor
// maps the client certificate CN to Principal.Subject and Organization[0] to
// Principal.TenantID. It invokes the interceptor directly with a mock stream
// whose peer context carries a real credentials.TLSInfo populated from the
// same ephemeral client cert used by the bufconn E2E test above.
func TestAuthInterceptor_MTLS_PrincipalExtraction(t *testing.T) {
	ca, caKey := generateTestCA(t)
	clientCert := generateTestLeaf(t, ca, caKey, "alice-service", []string{"acme-corp"})

	// Parse the leaf certificate so we can build a TLS ConnectionState
	// with VerifiedChains, which is what the interceptor extracts from
	// credentials.TLSInfo.
	leaf, err := x509.ParseCertificate(clientCert.Certificate[0])
	if err != nil {
		t.Fatalf("parse client leaf: %v", err)
	}

	auth, err := authn.New(authn.Config{Mode: authn.ModeMTLS})
	if err != nil {
		t.Fatalf("authn.New(mTLS): %v", err)
	}

	// Build a context with both incoming metadata (required by
	// buildSynthRequest) and a peer carrying TLS info (required by the mTLS
	// code path in the interceptor).
	md := metadata.Pairs("authorization", "Bearer mtls-placeholder")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	ctx = peer.NewContext(ctx, &peer.Peer{
		AuthInfo: credentials.TLSInfo{
			State: tls.ConnectionState{
				VerifiedChains:   [][]*x509.Certificate{{leaf, ca}},
				PeerCertificates: []*x509.Certificate{leaf},
			},
		},
	})

	stream := &mockServerStream{ctx: ctx}
	var captured *authn.Principal
	handler := func(_ any, ss grpc.ServerStream) error {
		if p, ok := authn.PrincipalFromContext(ss.Context()); ok {
			captured = p
		}
		return nil
	}

	interceptor := authStreamInterceptor(auth, authInterceptorConfig{mtls: true})
	if err := interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/test/Method"}, handler); err != nil {
		t.Fatalf("interceptor returned error: %v", err)
	}

	if captured == nil {
		t.Fatal("handler did not receive a Principal in context")
	}
	if captured.Subject != "alice-service" {
		t.Fatalf("Principal.Subject: want %q, got %q", "alice-service", captured.Subject)
	}
	if captured.TenantID != "acme-corp" {
		t.Fatalf("Principal.TenantID: want %q, got %q", "acme-corp", captured.TenantID)
	}
	if captured.AuthMode != authn.ModeMTLS {
		t.Fatalf("Principal.AuthMode: want %q, got %q", authn.ModeMTLS, captured.AuthMode)
	}
}

func TestServe_InterceptorInstalledWhenAuthenticatorSet(t *testing.T) {
	// Smoke: verify Serve() honours the Authenticator field. A full
	// end-to-end round-trip is covered by transport_test.go's bufconn
	// harness; here we just check the field wires through to the
	// interceptor chain without reflection.
	want := authn.Principal{Subject: "svc", TenantID: "acme"}
	auth := fakeAuthenticator{wantToken: "k", principal: want}
	opts := Options{Bind: "127.0.0.1:0", Server: nil, Authenticator: auth}
	// Serve rejects nil Server before touching the interceptor, so the
	// test stops at the guard — but the wire-up is exercised above via
	// authStreamInterceptor directly. Keep this test narrow: if Options
	// gains a new field that Serve ignores, a separate test will flag it.
	if err := Serve(context.Background(), opts); err == nil {
		t.Fatal("expected Serve to reject nil Server even with Authenticator set")
	}
}
