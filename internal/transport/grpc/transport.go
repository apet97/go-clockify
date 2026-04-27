package grpctransport

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/mcp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Options configures a Serve invocation. Bind is required; Server is the
// shared mcp.Server instance every stream will dispatch against. MaxRecvSize
// caps per-frame inbound bytes; when unset it inherits Server.MaxMessageSize
// (driven by MCP_MAX_MESSAGE_SIZE; MCP_HTTP_MAX_BODY is its deprecated
// alias) and falls back to the 4 MiB (4194304 byte) default if Server is
// also unset.
//
// Authenticator is optional. When non-nil, a grpc.StreamServerInterceptor
// is installed that bridges the shared internal/authn contract onto gRPC
// metadata — see authStreamInterceptor for the wire details and ADR 012
// for the rationale. Leaving it nil preserves the Wave 3 behaviour of
// relying on an external mTLS gateway / service mesh for authn.
type Options struct {
	Bind string
	// Listener, if non-nil, is used in place of net.Listen("tcp", Bind).
	// Enables tests to use bufconn or ephemeral-port TCP listeners and
	// learn the bound address before the server accepts. When Listener
	// is nil, Bind is required.
	Listener             net.Listener
	Server               *mcp.Server
	MaxRecvSize          int
	Authenticator        authn.Authenticator
	ReauthInterval       time.Duration
	ForwardTenantHeader  string
	ForwardSubjectHeader string
	TLSConfig            *tls.Config
}

// Serve starts the gRPC transport on the given bind and blocks until ctx
// is cancelled. On cancellation it gracefully drains in-flight streams with
// a 10s budget before returning.
//
// The transport exposes one bidirectional streaming method (Exchange) whose
// frames are raw JSON-RPC 2.0 bytes marshalled via the bytesCodec. Each
// client stream registers its own streamNotifier via Server.AddNotifier so
// server-initiated notifications (tools/list_changed, notifications/progress,
// notifications/resources/updated) fan out to every active stream. The
// notifier is automatically removed when the stream closes.
func Serve(ctx context.Context, opts Options) error {
	if opts.Listener == nil && opts.Bind == "" {
		return errors.New("grpctransport: Bind or Listener is required")
	}
	if opts.Server == nil {
		return errors.New("grpctransport: Server is required")
	}
	if opts.MaxRecvSize <= 0 {
		if opts.Server != nil && opts.Server.MaxMessageSize > 0 {
			opts.MaxRecvSize = int(opts.Server.MaxMessageSize)
		} else {
			opts.MaxRecvSize = 4194304
		}
	}

	handler := &exchangeServer{srv: opts.Server}
	desc := buildServiceDesc()
	healthDesc := buildHealthServiceDesc()
	hs := &healthServer{srv: opts.Server}

	serverOpts := []grpc.ServerOption{
		grpc.ForceServerCodec(bytesCodec{}),
		grpc.MaxRecvMsgSize(opts.MaxRecvSize),
	}
	if opts.TLSConfig != nil {
		serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(opts.TLSConfig)))
	}
	if opts.Authenticator != nil {
		serverOpts = append(serverOpts, grpc.StreamInterceptor(authStreamInterceptor(opts.Authenticator, authInterceptorConfig{
			reauthInterval:       opts.ReauthInterval,
			forwardTenantHeader:  opts.ForwardTenantHeader,
			forwardSubjectHeader: opts.ForwardSubjectHeader,
			mtls:                 opts.TLSConfig != nil && opts.TLSConfig.ClientAuth == tls.RequireAndVerifyClientCert,
		})))
	}
	grpcSrv := grpc.NewServer(serverOpts...)
	grpcSrv.RegisterService(&desc, handler)
	grpcSrv.RegisterService(&healthDesc, hs)

	ln := opts.Listener
	if ln == nil {
		var err error
		ln, err = net.Listen("tcp", opts.Bind)
		if err != nil {
			return fmt.Errorf("grpctransport: listen %s: %w", opts.Bind, err)
		}
	}

	go func() {
		<-ctx.Done()
		slog.Info("grpc_shutdown", "reason", "context cancelled")
		stopped := make(chan struct{})
		go func() {
			grpcSrv.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-time.After(10 * time.Second):
			slog.Warn("grpc_shutdown_timeout", "action", "forcing Stop")
			grpcSrv.Stop()
		}
	}()

	slog.Info("grpc_start", "bind", ln.Addr().String())
	if err := grpcSrv.Serve(ln); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return err
	}
	return nil
}

// exchangeServer implements the mcpServerIface contract registered by the
// hand-wired ServiceDesc. Each incoming stream gets its own Exchange loop.
type exchangeServer struct {
	srv *mcp.Server
}

// Exchange runs the JSON-RPC loop for one client stream. It installs a
// per-stream Notifier on the shared mcp.Server so server-initiated messages
// reach this caller, then reads frames until the client closes (io.EOF) or
// the RPC context cancels.
//
// Each inbound frame is dispatched in its own goroutine via
// mcp.Server.DispatchMessage so a long-running tools/call does not block
// reads of subsequent frames — critically, notifications/cancelled can
// reach the dispatcher while a handler is still blocked. All outbound
// frames (dispatch replies and server-initiated notifications) funnel
// through a single send-pump goroutine because grpc.ServerStream requires
// single-writer semantics.
//
// Errors from DispatchMessage are logged and the stream continues: dispatch
// errors mean a json.Marshal of the response failed, which is unexpected
// but not worth terminating every other in-flight request on the stream.
// Individual tool errors are already encoded inside the reply body per the
// MCP spec (result.isError) and never surface here.
func (e *exchangeServer) Exchange(stream grpc.ServerStream) error {
	ctx := stream.Context()

	// Buffered send channel absorbs notification bursts without stalling
	// the hub; 64 is headroom for typical notification fan-out (metrics
	// events, list_changed, progress). The capacity is not load-bearing —
	// Notify blocks on ctx when full.
	sends := make(chan []byte, 64)
	sendDone := make(chan error, 1)

	// Single-writer send pump. Every stream.SendMsg call on this stream
	// flows through here.
	go func() {
		defer close(sendDone)
		for frame := range sends {
			if err := stream.SendMsg(&frame); err != nil {
				sendDone <- err
				// Drain any remaining queued frames so senders don't
				// block forever after the stream has failed.
				for range sends {
				}
				return
			}
		}
	}()

	notifier := newStreamNotifier(ctx, sends)
	removeNotifier := e.srv.AddNotifier(notifier)
	defer removeNotifier()

	var wg sync.WaitGroup
	defer func() {
		// Wait for every dispatch goroutine to finish pushing (or giving
		// up via ctx) before closing the send channel, then wait for the
		// pump to drain. Returning earlier would race the pump into
		// SendMsg-after-close which gRPC treats as a hard error.
		wg.Wait()
		close(sends)
		<-sendDone
	}()

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		var frame []byte
		if err := stream.RecvMsg(&frame); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if len(frame) == 0 {
			continue
		}
		wg.Add(1)
		go func(f []byte) {
			defer wg.Done()
			reply, derr := e.srv.DispatchMessage(ctx, f)
			if derr != nil {
				slog.Error("grpc_dispatch_error", "err", derr)
				return
			}
			if reply == nil {
				// Notification from the client (no id) — no reply.
				return
			}
			select {
			case sends <- reply:
			case <-ctx.Done():
			}
		}(frame)
	}
}

// streamNotifier adapts an mcp.Notifier onto the per-stream send channel.
// Encoding produces a JSON-RPC 2.0 notification object (jsonrpc+method+params,
// no id); the send pump handles single-writer serialisation onto the
// underlying grpc.ServerStream.
type streamNotifier struct {
	ctx   context.Context
	sends chan<- []byte
}

func newStreamNotifier(ctx context.Context, sends chan<- []byte) *streamNotifier {
	return &streamNotifier{ctx: ctx, sends: sends}
}

func (n *streamNotifier) Notify(method string, params any) error {
	envelope := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		envelope["params"] = params
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("grpctransport: notify marshal: %w", err)
	}
	// Block on the send channel rather than dropping: a full queue means
	// SendMsg is slow, and dropping would violate the notifier's "best
	// effort in order" contract with the hub. ctx.Done is the escape
	// hatch so a dead stream cannot hold the notifier hub forever.
	select {
	case n.sends <- payload:
		return nil
	case <-n.ctx.Done():
		return n.ctx.Err()
	}
}
