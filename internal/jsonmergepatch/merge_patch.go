// Package jsonmergepatch is a hand-rolled, stdlib-only implementation of
// JSON Merge Patch (RFC 7396). It powers delta-sync notifications on the
// MCP `notifications/resources/updated` channel: the server caches the
// previously-emitted serialisation of a subscribed resource and emits the
// smallest JSON Merge Patch that, when applied to that cache entry,
// yields the fresh state. The sub-package is deliberately kept tiny and
// self-contained so ADR 001's stdlib-only default build is preserved.
//
// Two exported functions live here:
//
//   - Diff(prev, curr []byte) ([]byte, error)
//   - Apply(prev, patch []byte) ([]byte, error)
//
// Diff emits a merge patch according to RFC 7396 §2: members present in
// curr but not in prev, or with a different value, appear in the patch
// verbatim. Members present in prev but absent from curr appear as JSON
// null in the patch to signal removal. Objects are walked recursively;
// arrays and non-object scalars are compared as atomic values and copied
// whole when they differ.
//
// RFC 7396 has one notable limitation: there is no way to encode a JSON
// null value in the target document because null is reserved to mean
// delete. The DiffOrFull helper below handles this by falling back to
// a full replacement when the curr document contains any null value.
// The fallback is an additive signalling layer on top of the base RFC:
// callers that only need a textually-compliant merge patch should use
// Diff directly, while callers that want a lossless delta should use
// DiffOrFull and inspect the returned format code.
package jsonmergepatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// FormatNone means no delta was computed because there was no prior
// state (a fresh subscription or a cache eviction). Callers should emit
// the notification with format="none" and let the client fetch.
const FormatNone = "none"

// FormatMerge means the patch is a minimal RFC 7396 merge patch
// producible via Diff. Apply(prev, patch) == curr.
const FormatMerge = "merge"

// FormatFull means the current document contains a null value that RFC
// 7396 cannot encode losslessly. The patch field is the full curr
// document; clients should replace their cached state wholesale.
const FormatFull = "full"

// FormatDeleted signals that the subscribed URI's backing resource was
// deleted. The patch field is absent and clients should drop their
// cached state.
const FormatDeleted = "deleted"

// Diff returns the minimal RFC 7396 merge patch that, when applied to
// prev, yields curr. Both inputs must be valid JSON documents. The
// returned bytes are also a valid JSON document (either an object, or
// for non-object curr, curr itself).
//
// Semantics for non-object inputs follow RFC 7396 §2 paragraph 1: if
// either prev or curr is not a JSON object, the patch is simply curr.
// This matches the merge-patch apply algorithm: MergePatch(any, curr)
// when curr is not an object replaces the entire target with curr.
func Diff(prev, curr []byte) ([]byte, error) {
	var prevAny, currAny any
	if err := json.Unmarshal(prev, &prevAny); err != nil {
		return nil, fmt.Errorf("jsonmergepatch: decode prev: %w", err)
	}
	if err := json.Unmarshal(curr, &currAny); err != nil {
		return nil, fmt.Errorf("jsonmergepatch: decode curr: %w", err)
	}
	prevObj, prevIsObj := prevAny.(map[string]any)
	currObj, currIsObj := currAny.(map[string]any)
	if !prevIsObj || !currIsObj {
		return compactJSON(curr)
	}
	patch := diffObjects(prevObj, currObj)
	if patch == nil {
		// No changes at all — emit an empty object patch so Apply is a
		// no-op and callers can still distinguish "no delta needed"
		// from "delta failed".
		return []byte("{}"), nil
	}
	return json.Marshal(patch)
}

// DiffOrFull wraps Diff with a null-scan: if curr contains any JSON
// null value anywhere in the document tree, the returned patch is curr
// itself and the format code is FormatFull. Otherwise the patch is the
// minimal merge patch and format is FormatMerge. Empty patches (no
// changes) still return FormatMerge with an empty-object body so the
// caller can distinguish from "cannot compute".
func DiffOrFull(prev, curr []byte) (patch []byte, format string, err error) {
	if containsNull(curr) {
		compact, err := compactJSON(curr)
		if err != nil {
			return nil, "", err
		}
		return compact, FormatFull, nil
	}
	p, err := Diff(prev, curr)
	if err != nil {
		return nil, "", err
	}
	return p, FormatMerge, nil
}

// Apply applies a merge patch to prev and returns the updated document.
// Mirrors the RFC 7396 §1 MergePatch algorithm. Used by tests and
// available to downstream callers that want to round-trip Diff.
func Apply(prev, patch []byte) ([]byte, error) {
	var prevAny, patchAny any
	if err := json.Unmarshal(prev, &prevAny); err != nil {
		return nil, fmt.Errorf("jsonmergepatch: decode prev: %w", err)
	}
	if err := json.Unmarshal(patch, &patchAny); err != nil {
		return nil, fmt.Errorf("jsonmergepatch: decode patch: %w", err)
	}
	result := applyAny(prevAny, patchAny)
	return json.Marshal(result)
}

// diffObjects computes the recursive merge-patch delta between two JSON
// objects. Returns nil if they are deep-equal (no patch required).
func diffObjects(prev, curr map[string]any) map[string]any {
	patch := map[string]any{}
	// Additions and modifications.
	for key, currVal := range curr {
		prevVal, existed := prev[key]
		if !existed {
			patch[key] = currVal
			continue
		}
		if deepEqual(prevVal, currVal) {
			continue
		}
		prevObj, prevIsObj := prevVal.(map[string]any)
		currObj, currIsObj := currVal.(map[string]any)
		if prevIsObj && currIsObj {
			// Recursive object diff. Note: nested deltas are nested
			// objects, never nil, because diffObjects returns a
			// non-nil map when the inputs differ.
			if sub := diffObjects(prevObj, currObj); len(sub) > 0 {
				patch[key] = sub
			}
			continue
		}
		// Arrays and scalar changes are represented as full
		// replacements in RFC 7396.
		patch[key] = currVal
	}
	// Removals: keys in prev missing from curr become explicit nulls.
	for key := range prev {
		if _, stillThere := curr[key]; stillThere {
			continue
		}
		patch[key] = nil
	}
	if len(patch) == 0 {
		return nil
	}
	return patch
}

// applyAny is the recursive merge-patch apply algorithm.
func applyAny(prev, patch any) any {
	patchObj, patchIsObj := patch.(map[string]any)
	if !patchIsObj {
		// Non-object patch replaces target wholesale.
		return patch
	}
	prevObj, prevIsObj := prev.(map[string]any)
	if !prevIsObj {
		prevObj = map[string]any{}
	}
	out := make(map[string]any, len(prevObj)+len(patchObj))
	for k, v := range prevObj {
		out[k] = v
	}
	for k, v := range patchObj {
		if v == nil {
			delete(out, k)
			continue
		}
		if existing, ok := out[k]; ok {
			out[k] = applyAny(existing, v)
			continue
		}
		out[k] = applyAny(nil, v)
	}
	return out
}

// deepEqual compares two decoded JSON values for structural equality.
// Uses encoding/json round-tripping to avoid the usual float/int nullity
// foot-guns; both sides have already come through json.Unmarshal so they
// live in the same ecosystem of Go types.
func deepEqual(a, b any) bool {
	aBytes, errA := json.Marshal(a)
	if errA != nil {
		return false
	}
	bBytes, errB := json.Marshal(b)
	if errB != nil {
		return false
	}
	return bytes.Equal(aBytes, bBytes)
}

// containsNull reports whether raw (valid JSON) contains any JSON null
// value anywhere in its tree. Used by DiffOrFull to decide between the
// merge-patch and full-replace wire formats.
func containsNull(raw []byte) bool {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return false
	}
	return hasNull(v)
}

func hasNull(v any) bool {
	switch val := v.(type) {
	case nil:
		return true
	case map[string]any:
		for _, inner := range val {
			if hasNull(inner) {
				return true
			}
		}
	case []any:
		for _, inner := range val {
			if hasNull(inner) {
				return true
			}
		}
	}
	return false
}

// compactJSON re-marshals a JSON document so the output is whitespace-
// free and deterministic. Used when the wire format needs to carry a
// full document verbatim (DiffOrFull → FormatFull, non-object Diff
// inputs).
func compactJSON(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return nil, fmt.Errorf("jsonmergepatch: compact: %w", err)
	}
	if buf.Len() == 0 {
		return nil, errors.New("jsonmergepatch: empty input")
	}
	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	return out, nil
}
