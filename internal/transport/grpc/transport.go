package grpctransport

import (
	"context"
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
)

// Options configures a Serve invocation. Bind is required; Server is the
// shared mcp.Server instance every stream will dispatch against. MaxRecvSize
// caps per-frame inbound bytes to match the legacy HTTP MCP_HTTP_MAX_BODY
// default (2 MiB) when unset.
//
// Authenticator is optional. When non-nil, a grpc.StreamServerInterceptor
// is installed that bridges the shared internal/authn contract onto gRPC
// metadata — see authStreamInterceptor for the wire details and ADR 012
// for the rationale. Leaving it nil preserves the Wave 3 behaviour of
// relying on an external mTLS gateway / service mesh for authn.
type Options struct {
	Bind          string
	Server        *mcp.Server
	MaxRecvSize   int
	Authenticator authn.Authenticator
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
	if opts.Bind == "" {
		return errors.New("grpctransport: Bind is required")
	}
	if opts.Server == nil {
		return errors.New("grpctransport: Server is required")
	}
	if opts.MaxRecvSize <= 0 {
		opts.MaxRecvSize = 2 * 1024 * 1024
	}

	handler := &exchangeServer{srv: opts.Server}
	desc := buildServiceDesc()
	healthDesc := buildHealthServiceDesc()
	hs := &healthServer{srv: opts.Server}

	serverOpts := []grpc.ServerOption{
		grpc.ForceServerCodec(bytesCodec{}),
		grpc.MaxRecvMsgSize(opts.MaxRecvSize),
	}
	if opts.Authenticator != nil {
		serverOpts = append(serverOpts, grpc.StreamInterceptor(authStreamInterceptor(opts.Authenticator)))
	}
	grpcSrv := grpc.NewServer(serverOpts...)
	grpcSrv.RegisterService(&desc, handler)
	grpcSrv.RegisterService(&healthDesc, hs)

	ln, err := net.Listen("tcp", opts.Bind)
	if err != nil {
		return fmt.Errorf("grpctransport: listen %s: %w", opts.Bind, err)
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

	slog.Info("grpc_start", "bind", opts.Bind)
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
// the RPC context cancels. Each inbound frame is dispatched via
// mcp.Server.DispatchMessage and the reply (if any) sent back on the same
// stream.
//
// Errors from DispatchMessage terminate the stream with a gRPC Unknown
// status; individual tool errors are already encoded inside the reply body
// per the MCP spec (result.isError) and never surface here.
func (e *exchangeServer) Exchange(stream grpc.ServerStream) error {
	ctx := stream.Context()
	notifier := newStreamNotifier(stream)
	removeNotifier := e.srv.AddNotifier(notifier)
	defer removeNotifier()

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
		reply, dispatchErr := e.srv.DispatchMessage(ctx, frame)
		if dispatchErr != nil {
			// DispatchMessage only returns an error if json.Marshal of the
			// response fails — that's genuinely unexpected and worth
			// surfacing as a stream-level failure.
			return fmt.Errorf("dispatch: %w", dispatchErr)
		}
		if reply == nil {
			// Notification from the client (no id) — no reply required.
			continue
		}
		if err := stream.SendMsg(&reply); err != nil {
			return err
		}
	}
}

// streamNotifier wraps a grpc.ServerStream as an mcp.Notifier. Notifications
// are encoded as JSON-RPC 2.0 notification objects (jsonrpc+method+params, no
// id) and sent via the same bytesCodec as request/response frames. A mutex
// guards SendMsg because grpc server streams require single-writer semantics.
type streamNotifier struct {
	stream grpc.ServerStream
	mu     sync.Mutex
}

func newStreamNotifier(stream grpc.ServerStream) *streamNotifier {
	return &streamNotifier{stream: stream}
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
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.stream.SendMsg(&payload)
}
