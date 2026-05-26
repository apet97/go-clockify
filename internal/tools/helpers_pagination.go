package tools

import (
	"context"
	"maps"
	"reflect"
)

// defaultAutoPaginateMaxRows bounds an auto-paginated scan when the
// caller does not pass max_rows. Mirrors the production cap on the
// internal ListAll helper in the clockify client package.
const defaultAutoPaginateMaxRows = 5000

// maxAutoPaginateMaxRows is the hard ceiling on max_rows. A caller can
// raise the row cap above the default but not above this — keeps a
// single MCP tool call bounded in memory and round-trip time.
const maxAutoPaginateMaxRows = 50000

// autoPaginatePageSize is the page size used by every auto-paginate
// scan, regardless of caller-supplied page_size. Matches Clockify's
// documented per-endpoint maximum, so a scan completes in the minimum
// number of round-trips. Caller's page_size is preserved on the
// returned meta so the agent sees its requested shape.
const autoPaginatePageSize = 200

// paginationSchema returns a JSON schema with standard `page` /
// `page_size` integer properties plus the `auto_paginate` and
// `max_rows` knobs, merged with the caller's extras. The extras map
// may supply additional `properties` (merged) and `required`
// (concatenated).
func paginationSchema(extras map[string]any) map[string]any {
	props := map[string]any{
		"page":          map[string]any{"type": "integer", "description": "Page number (default 1)"},
		"page_size":     map[string]any{"type": "integer", "description": "Items per page (default 50, max 200)"},
		"auto_paginate": map[string]any{"type": "boolean", "description": "When true, walk every page server-side and return one consolidated result. Bounded by max_rows."},
		"max_rows":      map[string]any{"type": "integer", "description": "Row cap for auto_paginate scans (default 5000, hard cap 50000)."},
	}
	schema := map[string]any{"type": "object", "properties": props}
	if extras == nil {
		return schema
	}
	if extra, ok := extras["properties"].(map[string]any); ok {
		maps.Copy(props, extra)
	}
	if req, ok := extras["required"].([]string); ok && len(req) > 0 {
		schema["required"] = req
	}
	return schema
}

// paginationFromArgs extracts page/page_size from a tool args map. Public list
// tools share a conservative 200-item cap because they cover Clockify endpoint
// families with different pagination ceilings; bulk workflow/report scans use
// dedicated paginated helpers instead of this generic user-facing knob.
func paginationFromArgs(args map[string]any) (page, pageSize int) {
	page = max(intArg(args, "page", 1), 1)
	pageSize = intArg(args, "page_size", 50)
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

// autoPaginateArg reads the optional `auto_paginate: true` knob. Every
// list tool's input schema exposes the field via paginationSchema; a
// false / missing / non-boolean value keeps the historical single-page
// behaviour.
func autoPaginateArg(args map[string]any) bool {
	if args == nil {
		return false
	}
	if v, ok := args["auto_paginate"].(bool); ok {
		return v
	}
	return false
}

// maxRowsArg reads the optional `max_rows` cap used by an
// auto-paginated scan. Defaults to defaultAutoPaginateMaxRows; clamps
// non-positive values up to the default and over-the-hard-cap values
// down to maxAutoPaginateMaxRows.
func maxRowsArg(args map[string]any) int {
	n := intArg(args, "max_rows", defaultAutoPaginateMaxRows)
	if n <= 0 {
		return defaultAutoPaginateMaxRows
	}
	if n > maxAutoPaginateMaxRows {
		return maxAutoPaginateMaxRows
	}
	return n
}

// listPageFunc is the per-page list signature shared by every
// first-slice list helper (listClients / listProjects / …). Returning
// (items, page, pageSize, err) lets the auto-paginate loop walk pages
// without re-implementing pagination per handler.
type listPageFunc[T any] func(ctx context.Context, args map[string]any) ([]T, int, int, error)

// runListWithAutoPaginate executes a list call. When the caller passes
// `auto_paginate: true` it walks pages using `list`, capped by
// `max_rows` (default 5000, hard cap 50000); otherwise it runs the
// single-page path unchanged. Returns the items, the *caller's*
// requested page / page_size (so meta still reflects what they asked
// for), the `autoPaginated` flag (true when the loop ran), and the
// `truncated` flag (true when max_rows stopped the scan before the
// last page).
func runListWithAutoPaginate[T any](
	ctx context.Context,
	args map[string]any,
	list listPageFunc[T],
) (items []T, page int, pageSize int, autoPaginated bool, truncated bool, err error) {
	requestedPage, requestedPageSize := paginationFromArgs(args)
	if !autoPaginateArg(args) {
		items, page, pageSize, err = list(ctx, args)
		if page == 0 {
			page = requestedPage
		}
		if pageSize == 0 {
			pageSize = requestedPageSize
		}
		return items, page, pageSize, false, false, err
	}

	maxRows := maxRowsArg(args)
	scanArgs := copyArgs(args)
	scanArgs["page_size"] = float64(autoPaginatePageSize)
	delete(scanArgs, "auto_paginate")
	delete(scanArgs, "max_rows")

	var all []T
	for pageNumber := 1; pageNumber <= listAllSafetyStopPages; pageNumber++ {
		scanArgs["page"] = float64(pageNumber)
		batch, _, batchPageSize, batchErr := list(ctx, scanArgs)
		if batchErr != nil {
			return nil, requestedPage, requestedPageSize, true, false, batchErr
		}
		all = append(all, batch...)
		if len(all) >= maxRows {
			if len(all) > maxRows {
				all = all[:maxRows]
			}
			return all, requestedPage, requestedPageSize, true, true, nil
		}
		if batchPageSize == 0 || len(batch) < batchPageSize {
			return all, requestedPage, requestedPageSize, true, false, nil
		}
	}
	return all, requestedPage, requestedPageSize, true, true, nil
}

// addAutoPaginateMeta stamps the auto-paginate knobs onto a list-tool
// meta map so callers (and the MCP client schema) can see the loop ran
// and whether it stopped at max_rows.
func addAutoPaginateMeta(meta map[string]any, autoPaginated, truncated bool, maxRows int) map[string]any {
	if !autoPaginated {
		return meta
	}
	if meta == nil {
		meta = map[string]any{}
	}
	meta["auto_paginate"] = true
	meta["max_rows"] = maxRows
	meta["has_more"] = false
	if truncated {
		meta["truncated"] = true
	}
	return meta
}

// listAllSafetyStopPages mirrors the safety stop used by the clockify
// client package's ListAll. Defined locally to avoid an import cycle.
const listAllSafetyStopPages = 1000

// toolResultListHandler is the shape native list handlers in
// oneuser_native_descriptors.go expose: they take args, hit Clockify
// once, and return a populated ToolResult whose Data is a slice (e.g.
// []CompactInvoiceView, []map[string]any). runAutoPaginatedToolResult
// drives the loop for any handler matching this shape without
// requiring each handler to grow a per-page helper.
type toolResultListHandler func(ctx context.Context, args map[string]any) (ToolResult, error)

// runAutoPaginatedToolResult executes a ToolResult-returning list
// handler. When auto_paginate is not set, returns the handler's result
// untouched. When set, walks pages by repeatedly invoking the handler
// with page=1,2,3..., concatenating Data slices via reflection, and
// stops when (a) the page comes back short of autoPaginatePageSize, (b)
// max_rows is reached, or (c) the safety-stop ceiling is hit. The
// caller is responsible for stamping addAutoPaginateMeta on the
// returned ToolResult's Meta map.
//
// Slice merging uses reflection because handlers return concrete typed
// slices (e.g. []CompactInvoiceView) inside an `any`-typed Data field.
// When Data is not a slice (e.g. handler returned a map or error
// envelope), the first page is returned unchanged and the loop exits.
func runAutoPaginatedToolResult(
	ctx context.Context,
	args map[string]any,
	handler toolResultListHandler,
) (result ToolResult, autoPaginated bool, truncated bool, err error) {
	if !autoPaginateArg(args) {
		result, err = handler(ctx, args)
		return result, false, false, err
	}
	maxRows := maxRowsArg(args)
	scanArgs := copyArgs(args)
	scanArgs["page_size"] = float64(autoPaginatePageSize)
	delete(scanArgs, "auto_paginate")
	delete(scanArgs, "max_rows")

	var merged ToolResult
	var mergedData reflect.Value
	for pageNumber := 1; pageNumber <= listAllSafetyStopPages; pageNumber++ {
		scanArgs["page"] = float64(pageNumber)
		partial, perr := handler(ctx, scanArgs)
		if perr != nil {
			return ToolResult{}, true, false, perr
		}
		batch := reflect.ValueOf(partial.Data)
		if pageNumber == 1 {
			merged = partial
			if !batch.IsValid() || batch.Kind() != reflect.Slice {
				return partial, true, false, nil
			}
			mergedData = batch
		} else if batch.IsValid() && batch.Kind() == reflect.Slice {
			mergedData = reflect.AppendSlice(mergedData, batch)
		}
		if mergedData.Len() >= maxRows {
			if mergedData.Len() > maxRows {
				mergedData = mergedData.Slice(0, maxRows)
			}
			merged.Data = mergedData.Interface()
			return merged, true, true, nil
		}
		if !batch.IsValid() || batch.Kind() != reflect.Slice || batch.Len() < autoPaginatePageSize {
			merged.Data = mergedData.Interface()
			return merged, true, false, nil
		}
	}
	merged.Data = mergedData.Interface()
	return merged, true, true, nil
}

// autoPaginated wraps a ToolResult-returning list handler with the
// auto_paginate loop and stamps the resulting meta knobs. Used at
// descriptor-registration time so per-handler bodies stay focused on
// the single-page case. When auto_paginate is not requested, the
// wrapped handler is invoked exactly once and its output passes
// through untouched.
func autoPaginated(handler toolResultListHandler) toolResultListHandler {
	return func(ctx context.Context, args map[string]any) (ToolResult, error) {
		merged, auto, truncated, err := runAutoPaginatedToolResult(ctx, args, handler)
		if err != nil {
			return ToolResult{}, err
		}
		if !auto {
			return merged, nil
		}
		merged.Meta = addAutoPaginateMeta(merged.Meta, true, truncated, maxRowsArg(args))
		if data := reflect.ValueOf(merged.Data); data.IsValid() && data.Kind() == reflect.Slice {
			if merged.Meta == nil {
				merged.Meta = map[string]any{}
			}
			merged.Meta["count"] = data.Len()
		}
		return merged, nil
	}
}

// injectAutoPaginateSchemaProps adds the auto_paginate + max_rows
// properties to any list-tool input schema that already declares
// `page`. Idempotent; existing entries are preserved. Used to bring
// hand-written native list schemas into line with paginationSchema()
// without rewriting every literal.
func injectAutoPaginateSchemaProps(schema map[string]any) {
	if schema == nil {
		return
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return
	}
	if _, hasPage := props["page"]; !hasPage {
		return
	}
	if _, hasAuto := props["auto_paginate"]; !hasAuto {
		props["auto_paginate"] = map[string]any{
			"type":        "boolean",
			"description": "When true, walk every page server-side and return one consolidated result. Bounded by max_rows.",
		}
	}
	if _, hasMax := props["max_rows"]; !hasMax {
		props["max_rows"] = map[string]any{
			"type":        "integer",
			"description": "Row cap for auto_paginate scans (default 5000, hard cap 50000).",
		}
	}
}

func addPaginationMeta(meta map[string]any, args map[string]any, page, pageSize int) map[string]any {
	if meta == nil {
		meta = map[string]any{}
	}
	meta["page"] = page
	meta["pageSize"] = pageSize
	count, hasCount := reportInt(meta["count"])
	total, hasTotal := reportInt(meta["total"])
	if !hasTotal && hasCount {
		total = ((page - 1) * pageSize) + count
		if count == pageSize {
			// Full page, no upstream count: more rows may exist, so expose
			// only a lower bound — never an authoritative `total`.
			meta["total_min"] = total
			meta["total_is_lower_bound"] = true
		} else {
			// Short page: every row was seen, so this is the real total.
			meta["total"] = total
		}
	}
	if _, ok := meta["has_more"]; !ok && hasCount {
		if hasTotal {
			meta["has_more"] = page*pageSize < total
		} else {
			meta["has_more"] = count == pageSize
		}
	}
	pagination := map[string]any{
		"page":              page,
		"page_size":         pageSize,
		"applied_page_size": pageSize,
	}
	if _, ok := args["page_size"]; ok {
		requestedPageSize := intArg(args, "page_size", 50)
		pagination["requested_page_size"] = requestedPageSize
		if requestedPageSize != pageSize {
			pagination["clamped"] = true
		}
	}
	meta["pagination"] = pagination
	return meta
}
