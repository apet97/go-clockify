package tools

import (
	"context"
	"errors"
	"testing"
)

func TestAutoPaginateArgRespectsFlag(t *testing.T) {
	if autoPaginateArg(nil) {
		t.Fatalf("nil args should not auto-paginate")
	}
	if autoPaginateArg(map[string]any{}) {
		t.Fatalf("empty args should not auto-paginate")
	}
	if autoPaginateArg(map[string]any{"auto_paginate": false}) {
		t.Fatalf("auto_paginate=false should not auto-paginate")
	}
	if !autoPaginateArg(map[string]any{"auto_paginate": true}) {
		t.Fatalf("auto_paginate=true should auto-paginate")
	}
}

func TestMaxRowsArgDefaultsAndClamps(t *testing.T) {
	if got := maxRowsArg(nil); got != defaultAutoPaginateMaxRows {
		t.Fatalf("nil args max_rows = %d, want %d", got, defaultAutoPaginateMaxRows)
	}
	if got := maxRowsArg(map[string]any{}); got != defaultAutoPaginateMaxRows {
		t.Fatalf("empty args max_rows = %d, want %d", got, defaultAutoPaginateMaxRows)
	}
	if got := maxRowsArg(map[string]any{"max_rows": float64(1234)}); got != 1234 {
		t.Fatalf("explicit max_rows=1234 = %d", got)
	}
	if got := maxRowsArg(map[string]any{"max_rows": float64(0)}); got != defaultAutoPaginateMaxRows {
		t.Fatalf("max_rows=0 (invalid) = %d, want default %d", got, defaultAutoPaginateMaxRows)
	}
	if got := maxRowsArg(map[string]any{"max_rows": float64(-50)}); got != defaultAutoPaginateMaxRows {
		t.Fatalf("max_rows=-50 (invalid) = %d, want default %d", got, defaultAutoPaginateMaxRows)
	}
	if got := maxRowsArg(map[string]any{"max_rows": float64(1_000_000)}); got != maxAutoPaginateMaxRows {
		t.Fatalf("max_rows over hard cap = %d, want hard cap %d", got, maxAutoPaginateMaxRows)
	}
}

func TestAutoPaginateSchemaIncludesKnobs(t *testing.T) {
	schema := paginationSchema(nil)
	props, _ := schema["properties"].(map[string]any)
	if _, ok := props["auto_paginate"]; !ok {
		t.Fatalf("paginationSchema missing auto_paginate property")
	}
	if _, ok := props["max_rows"]; !ok {
		t.Fatalf("paginationSchema missing max_rows property")
	}
}

func TestRunListWithAutoPaginateSinglePagePathPreservesPageMeta(t *testing.T) {
	calls := 0
	list := func(_ context.Context, args map[string]any) ([]int, int, int, error) {
		calls++
		page, pageSize := paginationFromArgs(args)
		if page == 1 {
			return []int{1, 2, 3}, page, pageSize, nil
		}
		t.Fatalf("single-page path requested page %d", page)
		return nil, 0, 0, nil
	}
	items, page, pageSize, auto, truncated, err := runListWithAutoPaginate(context.Background(), map[string]any{}, list)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
	if auto || truncated {
		t.Fatalf("single-page path should not set auto/truncated; auto=%v truncated=%v", auto, truncated)
	}
	if len(items) != 3 || page != 1 || pageSize != 50 {
		t.Fatalf("items=%v page=%d pageSize=%d", items, page, pageSize)
	}
}

func TestRunListWithAutoPaginateWalksAllPages(t *testing.T) {
	calls := 0
	// Auto-paginate scans use autoPaginatePageSize (200) regardless of
	// caller's page_size. The mock returns a full 200-row first page
	// and a short 50-row second page so the loop terminates naturally.
	list := func(_ context.Context, args map[string]any) ([]int, int, int, error) {
		calls++
		page, pageSize := paginationFromArgs(args)
		if pageSize != autoPaginatePageSize {
			t.Fatalf("auto-paginate scan should request page_size=%d, got %d", autoPaginatePageSize, pageSize)
		}
		switch page {
		case 1:
			batch := make([]int, autoPaginatePageSize)
			for i := range batch {
				batch[i] = i
			}
			return batch, page, pageSize, nil
		case 2:
			return make([]int, 50), page, pageSize, nil
		default:
			t.Fatalf("unexpected page %d", page)
			return nil, 0, 0, nil
		}
	}
	args := map[string]any{"auto_paginate": true, "page_size": float64(50)}
	items, page, pageSize, auto, truncated, err := runListWithAutoPaginate(context.Background(), args, list)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
	if !auto || truncated {
		t.Fatalf("auto=%v truncated=%v, want auto=true truncated=false", auto, truncated)
	}
	if got := len(items); got != autoPaginatePageSize+50 {
		t.Fatalf("items count = %d, want %d", got, autoPaginatePageSize+50)
	}
	if page != 1 || pageSize != 50 {
		t.Fatalf("returned caller-requested page=%d pageSize=%d, want page=1 pageSize=50", page, pageSize)
	}
}

func TestRunListWithAutoPaginateTruncatesAtMaxRows(t *testing.T) {
	// Every page is a "full" 200-row batch, so the loop only stops at
	// max_rows. Caller asks for max_rows=300 → expect 2 pages * 200
	// rows = 400, trimmed to 300, with truncated=true.
	list := func(_ context.Context, args map[string]any) ([]int, int, int, error) {
		page, pageSize := paginationFromArgs(args)
		batch := make([]int, pageSize)
		for i := range batch {
			batch[i] = (page-1)*pageSize + i
		}
		return batch, page, pageSize, nil
	}
	args := map[string]any{
		"auto_paginate": true,
		"max_rows":      float64(300),
	}
	items, _, _, auto, truncated, err := runListWithAutoPaginate(context.Background(), args, list)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !auto || !truncated {
		t.Fatalf("auto=%v truncated=%v, want both true", auto, truncated)
	}
	if got := len(items); got != 300 {
		t.Fatalf("items count = %d, want 300 max_rows cap", got)
	}
}

func TestRunListWithAutoPaginatePropagatesError(t *testing.T) {
	want := errors.New("boom")
	list := func(_ context.Context, _ map[string]any) ([]int, int, int, error) {
		return nil, 0, 0, want
	}
	_, _, _, _, _, got := runListWithAutoPaginate(context.Background(), map[string]any{"auto_paginate": true}, list)
	if !errors.Is(got, want) {
		t.Fatalf("err = %v, want %v", got, want)
	}
}
