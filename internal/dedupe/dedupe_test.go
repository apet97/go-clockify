package dedupe

import (
	"os"
	"testing"

	"github.com/apet97/go-clockify/internal/clockify"
)

// helper to clear all dedupe-related env vars before each test.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"CLOCKIFY_DEDUPE_MODE",
		"CLOCKIFY_DEDUPE_LOOKBACK",
		"CLOCKIFY_OVERLAP_CHECK",
	} {
		os.Unsetenv(key)
	}
}

// --- ConfigFromEnv tests ---

func TestConfigFromEnvDefaults(t *testing.T) {
	clearEnv(t)
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Mode != Warn {
		t.Errorf("Mode = %d, want Warn (%d)", cfg.Mode, Warn)
	}
	if cfg.LookbackCount != 25 {
		t.Errorf("LookbackCount = %d, want 25", cfg.LookbackCount)
	}
	if !cfg.OverlapCheck {
		t.Error("OverlapCheck = false, want true")
	}
}

func TestConfigFromEnvBlock(t *testing.T) {
	clearEnv(t)
	os.Setenv("CLOCKIFY_DEDUPE_MODE", "block")
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Mode != Block {
		t.Errorf("Mode = %d, want Block (%d)", cfg.Mode, Block)
	}
}

func TestConfigFromEnvOff(t *testing.T) {
	clearEnv(t)
	os.Setenv("CLOCKIFY_DEDUPE_MODE", "off")
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Mode != Off {
		t.Errorf("Mode = %d, want Off (%d)", cfg.Mode, Off)
	}
}

// --- CheckDuplicate tests ---

func TestNoDuplicate(t *testing.T) {
	entries := []clockify.TimeEntry{
		{
			ID:          "e1",
			Description: "Meeting with client",
			ProjectID:   "p1",
			TimeInterval: clockify.TimeInterval{
				Start: "2026-04-06T09:00:00Z",
				End:   "2026-04-06T10:00:00Z",
			},
		},
	}
	result := CheckDuplicate(entries, "Different task", "p1", "2026-04-06T09:00:00Z")
	if result.IsDuplicate {
		t.Error("expected no duplicate, got duplicate")
	}
}

func TestDuplicateFound(t *testing.T) {
	entries := []clockify.TimeEntry{
		{
			ID:          "e1",
			Description: "Code review",
			ProjectID:   "p1",
			TimeInterval: clockify.TimeInterval{
				Start: "2026-04-06T14:00:00Z",
				End:   "2026-04-06T15:00:00Z",
			},
		},
	}
	result := CheckDuplicate(entries, "Code review", "p1", "2026-04-06T14:00:30Z")
	if !result.IsDuplicate {
		t.Error("expected duplicate, got none")
	}
	if result.ExistingEntryID != "e1" {
		t.Errorf("ExistingEntryID = %q, want %q", result.ExistingEntryID, "e1")
	}
}

func TestDuplicateProjectMismatch(t *testing.T) {
	entries := []clockify.TimeEntry{
		{
			ID:          "e1",
			Description: "Code review",
			ProjectID:   "p1",
			TimeInterval: clockify.TimeInterval{
				Start: "2026-04-06T14:00:00Z",
				End:   "2026-04-06T15:00:00Z",
			},
		},
	}
	result := CheckDuplicate(entries, "Code review", "p2", "2026-04-06T14:00:00Z")
	if result.IsDuplicate {
		t.Error("expected no duplicate when project differs")
	}
}

func TestDuplicateTimeMismatch(t *testing.T) {
	entries := []clockify.TimeEntry{
		{
			ID:          "e1",
			Description: "Code review",
			ProjectID:   "p1",
			TimeInterval: clockify.TimeInterval{
				Start: "2026-04-06T14:00:00Z",
				End:   "2026-04-06T15:00:00Z",
			},
		},
	}
	// Different minute: 14:05 vs 14:00
	result := CheckDuplicate(entries, "Code review", "p1", "2026-04-06T14:05:00Z")
	if result.IsDuplicate {
		t.Error("expected no duplicate when start minute differs")
	}
}

// --- CheckOverlap tests ---

func TestOverlapFound(t *testing.T) {
	entries := []clockify.TimeEntry{
		{
			ID:          "e1",
			Description: "Existing work",
			ProjectID:   "p1",
			TimeInterval: clockify.TimeInterval{
				Start: "2026-04-06T09:00:00Z",
				End:   "2026-04-06T11:00:00Z",
			},
		},
	}
	// Proposed: 10:00-12:00 overlaps with 09:00-11:00
	result := CheckOverlap(entries, "p1", "2026-04-06T10:00:00Z", "2026-04-06T12:00:00Z")
	if !result.HasOverlap {
		t.Error("expected overlap, got none")
	}
	if result.OverlapEntryID != "e1" {
		t.Errorf("OverlapEntryID = %q, want %q", result.OverlapEntryID, "e1")
	}
	if result.Description != "Existing work" {
		t.Errorf("Description = %q, want %q", result.Description, "Existing work")
	}
}

func TestNoOverlap(t *testing.T) {
	entries := []clockify.TimeEntry{
		{
			ID:          "e1",
			Description: "Morning work",
			ProjectID:   "p1",
			TimeInterval: clockify.TimeInterval{
				Start: "2026-04-06T09:00:00Z",
				End:   "2026-04-06T10:00:00Z",
			},
		},
	}
	// Proposed: 10:00-11:00 is adjacent but not overlapping
	result := CheckOverlap(entries, "p1", "2026-04-06T10:00:00Z", "2026-04-06T11:00:00Z")
	if result.HasOverlap {
		t.Error("expected no overlap for adjacent ranges")
	}
}

func TestOverlapSkipsRunningTimer(t *testing.T) {
	entries := []clockify.TimeEntry{
		{
			ID:          "e1",
			Description: "Running timer",
			ProjectID:   "p1",
			TimeInterval: clockify.TimeInterval{
				Start: "2026-04-06T08:00:00Z",
				End:   "", // running timer
			},
		},
	}
	result := CheckOverlap(entries, "p1", "2026-04-06T08:30:00Z", "2026-04-06T09:30:00Z")
	if result.HasOverlap {
		t.Error("expected running timer to be skipped")
	}
}

func TestEmptyEntries(t *testing.T) {
	var entries []clockify.TimeEntry

	dupResult := CheckDuplicate(entries, "Anything", "p1", "2026-04-06T09:00:00Z")
	if dupResult.IsDuplicate {
		t.Error("expected no duplicate with empty entries")
	}

	overlapResult := CheckOverlap(entries, "p1", "2026-04-06T09:00:00Z", "2026-04-06T10:00:00Z")
	if overlapResult.HasOverlap {
		t.Error("expected no overlap with empty entries")
	}
}
