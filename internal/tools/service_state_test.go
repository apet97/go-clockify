package tools

import (
	"reflect"
	"testing"
)

func TestServiceInternalStateIsGroupedByConcern(t *testing.T) {
	serviceType := reflect.TypeOf(Service{})
	legacyFields := []string{
		"cachedUser",
		"cachedWSID",
		"resolveCache",
		"resourceCache",
		"demoResources",
		"registryOnce",
		"toolsResourceCache",
	}
	for _, field := range legacyFields {
		if _, ok := serviceType.FieldByName(field); ok {
			t.Fatalf("Service should keep %s inside a concern-specific state holder", field)
		}
	}
	for _, field := range []string{"identity", "resolver", "resources", "registry"} {
		if _, ok := serviceType.FieldByName(field); !ok {
			t.Fatalf("Service missing grouped state field %s", field)
		}
	}
}
