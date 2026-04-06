package dedupe

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"goclmcp/internal/clockify"
)

type Mode int

const (
	Warn Mode = iota
	Block
	Off
)

type Config struct {
	Mode          Mode
	LookbackCount int
	OverlapCheck  bool
}

type DuplicateResult struct {
	IsDuplicate     bool
	ExistingEntryID string
}

type OverlapResult struct {
	HasOverlap     bool
	OverlapEntryID string
	Description    string
}

// ConfigFromEnv reads deduplication configuration from environment variables.
//
//   - CLOCKIFY_DEDUPE_MODE: "warn" (default), "block", "off" — case-insensitive
//   - CLOCKIFY_DEDUPE_LOOKBACK: default 25, must be > 0
//   - CLOCKIFY_OVERLAP_CHECK: "true"/"1"/"warn" (default true), "false"/"0"/"off"
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		Mode:          Warn,
		LookbackCount: 25,
		OverlapCheck:  true,
	}

	if v := os.Getenv("CLOCKIFY_DEDUPE_MODE"); v != "" {
		switch strings.ToLower(v) {
		case "warn":
			cfg.Mode = Warn
		case "block":
			cfg.Mode = Block
		case "off":
			cfg.Mode = Off
		default:
			return cfg, fmt.Errorf("invalid CLOCKIFY_DEDUPE_MODE: %q (want warn, block, off)", v)
		}
	}

	if v := os.Getenv("CLOCKIFY_DEDUPE_LOOKBACK"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("invalid CLOCKIFY_DEDUPE_LOOKBACK: %w", err)
		}
		if n <= 0 {
			return cfg, fmt.Errorf("CLOCKIFY_DEDUPE_LOOKBACK must be > 0, got %d", n)
		}
		cfg.LookbackCount = n
	}

	if v := os.Getenv("CLOCKIFY_OVERLAP_CHECK"); v != "" {
		switch strings.ToLower(v) {
		case "true", "1", "warn":
			cfg.OverlapCheck = true
		case "false", "0", "off":
			cfg.OverlapCheck = false
		default:
			return cfg, fmt.Errorf("invalid CLOCKIFY_OVERLAP_CHECK: %q", v)
		}
	}

	return cfg, nil
}

// CheckDuplicate checks whether a proposed time entry duplicates an existing one.
// A duplicate is defined as an exact match on all three of:
//  1. Description (case-sensitive)
//  2. ProjectID (or both empty)
//  3. Start time to the minute (first 16 chars of ISO 8601)
func CheckDuplicate(entries []clockify.TimeEntry, description, projectID, startISO string) *DuplicateResult {
	for _, entry := range entries {
		if entry.Description != description {
			continue
		}
		if entry.ProjectID != projectID {
			continue
		}
		// Guard: only compare start times if both are long enough.
		if len(startISO) >= 16 && len(entry.TimeInterval.Start) >= 16 {
			if entry.TimeInterval.Start[:16] != startISO[:16] {
				continue
			}
		} else {
			// If startISO is too short, skip the start comparison (don't match).
			continue
		}
		return &DuplicateResult{IsDuplicate: true, ExistingEntryID: entry.ID}
	}
	return &DuplicateResult{IsDuplicate: false}
}

// CheckOverlap checks whether a proposed time range overlaps with any existing
// entry on the same project. Running timers (empty end time) are skipped.
// String comparison works for ISO 8601 timestamps.
func CheckOverlap(entries []clockify.TimeEntry, projectID, startISO, endISO string) *OverlapResult {
	for _, entry := range entries {
		if entry.ProjectID != projectID {
			continue
		}
		// Skip running timers.
		if entry.TimeInterval.End == "" {
			continue
		}
		// Overlap exists when the ranges are NOT disjoint.
		// Disjoint: endISO <= entry.Start  OR  startISO >= entry.End
		if !(endISO <= entry.TimeInterval.Start || startISO >= entry.TimeInterval.End) {
			return &OverlapResult{
				HasOverlap:     true,
				OverlapEntryID: entry.ID,
				Description:    entry.Description,
			}
		}
	}
	return &OverlapResult{HasOverlap: false}
}
