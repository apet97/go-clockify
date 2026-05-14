//go:build legacy_platform && grpc

package runtime

// GRPCBuildAvailable reports whether this binary links the real gRPC
// transport. The default build exposes a stub so config can name grpc, but
// process entrypoints use this flag to fail before rollout when the wrong
// artifact is selected.
const GRPCBuildAvailable = true
