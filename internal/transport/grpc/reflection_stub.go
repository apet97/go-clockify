//go:build !grpcreflection

package grpctransport

import "google.golang.org/grpc"

func registerOptionalReflection(*grpc.Server) {}
