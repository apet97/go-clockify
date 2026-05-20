package tools

func reportInputSchema() map[string]any {
	return objectSchema(map[string]any{
		"properties": map[string]any{
			"date_range_start":  map[string]any{"type": "string", "description": "Report range start. " + flexibleDatetimeDescription},
			"date_range_end":    map[string]any{"type": "string", "description": "Report range end. " + flexibleDatetimeDescription},
			"start":             map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"end":               map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"export_type":       map[string]any{"type": "string", "enum": []string{"JSON", "JSON_V1", "PDF", "CSV", "XLSX", "ZIP"}, "description": "Report export type"},
			"max_rollups":       map[string]any{"type": "integer", "minimum": 0, "description": "Keep only the N largest project rollups by tracked time (default 15). Aggregates still cover every rollup. Pass 0 for the full, unbounded list."},
			"summary_filter":    map[string]any{"type": "object", "additionalProperties": true},
			"detailed_filter":   map[string]any{"type": "object", "additionalProperties": true},
			"attendance_filter": map[string]any{"type": "object", "additionalProperties": true},
		},
	})
}
