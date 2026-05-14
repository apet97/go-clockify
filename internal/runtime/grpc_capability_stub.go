//go:build legacy_platform && !grpc

package runtime

// GRPCBuildAvailable reports whether this binary links the real gRPC
// transport. False in the default build, where runtime.runGRPC is only a
// diagnostic stub.
const GRPCBuildAvailable = false
