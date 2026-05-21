package mcp

import (
	"math"
	"testing"
)

func TestCheckedByteBufferCapacityRejectsOverflow(t *testing.T) {
	got, err := checkedByteBufferCapacity(1, 2, 3)
	if err != nil {
		t.Fatalf("checkedByteBufferCapacity returned unexpected error: %v", err)
	}
	if got != 6 {
		t.Fatalf("checkedByteBufferCapacity = %d, want 6", got)
	}

	if _, err := checkedByteBufferCapacity(math.MaxInt, 1); err == nil {
		t.Fatal("expected overflow error")
	}
}

func TestSchemaValidationCopyCapacityAvoidsOverflow(t *testing.T) {
	if got := schemaValidationCopyCapacity(3); got != 4 {
		t.Fatalf("schemaValidationCopyCapacity(3) = %d, want 4", got)
	}
	if got := schemaValidationCopyCapacity(math.MaxInt); got != math.MaxInt {
		t.Fatalf("schemaValidationCopyCapacity(MaxInt) = %d, want %d", got, math.MaxInt)
	}
}
