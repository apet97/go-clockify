package main

import (
	"fmt"

	"github.com/apet97/go-clockify/internal/config"
	svcruntime "github.com/apet97/go-clockify/internal/runtime"
)

var grpcBuildAvailable = svcruntime.GRPCBuildAvailable

func validateBuildCapabilities(cfg config.Config) error {
	if cfg.Transport == "grpc" && !grpcBuildAvailable {
		return fmt.Errorf("MCP_TRANSPORT=grpc requires a binary built with -tags=grpc; use a clockify-mcp-grpc* release artifact or see docs/deploy/profile-private-network-grpc.md")
	}
	return nil
}
