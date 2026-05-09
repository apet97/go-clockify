//go:build grpcreflection

package grpctransport

import (
	"strings"
	"testing"

	"google.golang.org/grpc"
)

func TestRegisterOptionalReflectionRegistersReflectionService(t *testing.T) {
	srv := grpc.NewServer()
	registerOptionalReflection(srv)

	for name := range srv.GetServiceInfo() {
		if strings.HasPrefix(name, "grpc.reflection.") {
			return
		}
	}
	t.Fatalf("reflection service not registered; services=%v", srv.GetServiceInfo())
}
