package mcp

import (
	"encoding/json"
	"fmt"
)

func (s *Server) applyToolResultSizeGuard(toolName string, result any) (any, map[string]any) {
	budget := s.MaxToolResultBytes
	if budget <= 0 {
		return result, nil
	}
	raw, err := json.Marshal(result)
	if err != nil || len(raw) <= budget {
		return result, nil
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return result, resultTooLargeEnvelope(toolName, len(raw), budget)
	}
	if truncateEnvelopeDataToBudget(envelope, budget) {
		return envelope, nil
	}
	return result, resultTooLargeEnvelope(toolName, len(raw), budget)
}

func truncateEnvelopeDataToBudget(envelope map[string]any, budget int) bool {
	data, ok := envelope["data"]
	if !ok {
		return false
	}
	switch value := data.(type) {
	case []any:
		return shrinkListToBudget(envelope, value, budget, func(items []any) {
			envelope["data"] = items
		})
	case map[string]any:
		key, items := largestArrayField(value)
		if key == "" {
			return false
		}
		return shrinkListToBudget(envelope, items, budget, func(kept []any) {
			value[key] = kept
			if _, ok := value["count"]; ok {
				value["count"] = len(kept)
			}
		})
	default:
		return false
	}
}

func largestArrayField(value map[string]any) (string, []any) {
	var key string
	var items []any
	for k, raw := range value {
		candidate, ok := raw.([]any)
		if !ok || len(candidate) == 0 {
			continue
		}
		if len(candidate) > len(items) {
			key = k
			items = candidate
		}
	}
	return key, items
}

func shrinkListToBudget(envelope map[string]any, items []any, budget int, set func([]any)) bool {
	if len(items) == 0 {
		return false
	}
	total := len(items)
	if total == 1 {
		return false
	}
	best := 0
	low, high := 1, total-1
	for low <= high {
		kept := low + (high-low)/2
		set(items[:kept])
		addSizeCapMeta(envelope, kept, total-kept)
		raw, err := json.Marshal(envelope)
		if err == nil && len(raw) <= budget {
			best = kept
			low = kept + 1
			continue
		}
		high = kept - 1
	}
	if best > 0 {
		set(items[:best])
		addSizeCapMeta(envelope, best, total-best)
		return true
	}
	set(items[:1])
	addSizeCapMeta(envelope, 1, total-1)
	return false
}

func addSizeCapMeta(envelope map[string]any, returned, dropped int) {
	meta, _ := envelope["meta"].(map[string]any)
	if meta == nil {
		meta = map[string]any{}
		envelope["meta"] = meta
	}
	meta["truncated"] = true
	meta["size_capped"] = true
	meta["returned"] = returned
	meta["dropped"] = dropped
	meta["next_hint"] = "Response was size-capped by CLOCKIFY_MAX_TOOL_RESULT_BYTES; narrow filters, lower page_size or max_rows, or page through results to continue."
}

func resultTooLargeEnvelope(toolName string, actualBytes, budget int) map[string]any {
	return map[string]any{
		"ok":     false,
		"action": toolName,
		"error": map[string]any{
			"code":    "result_too_large",
			"message": fmt.Sprintf("%s produced a %d-byte result, exceeding CLOCKIFY_MAX_TOOL_RESULT_BYTES=%d, and the payload could not be safely truncated.", toolName, actualBytes, budget),
		},
		"recovery": map[string]any{
			"hint":      "Retry with narrower filters, a lower page_size or max_rows value, or a more specific get/list tool.",
			"retryable": true,
		},
		"meta": map[string]any{
			"resultBytes":          actualBytes,
			"maxToolResultBytes":   budget,
			"size_capped":          true,
			"truncationAttempted":  true,
			"truncationSuccessful": false,
		},
	}
}
