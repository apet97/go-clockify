//go:build grpcreflection

package grpctransport

import (
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func registerOptionalReflection(s *grpc.Server) {
	reflection.Register(s)
	slog.Warn("grpc_reflection_enabled", "scope", "development_only")
}
