package tools

import (
	"strconv"
	"testing"
)

func TestResourceStateCacheGetPut(t *testing.T) {
	c := newResourceStateCache(3)
	c.put("a", []byte(`{"v":1}`))
	got, ok := c.get("a")
	if !ok || string(got) != `{"v":1}` {
		t.Fatalf("get a: %q ok=%v", got, ok)
	}
	c.put("a", []byte(`{"v":2}`))
	got, _ = c.get("a")
	if string(got) != `{"v":2}` {
		t.Fatalf("get a after update: %q", got)
	}
}

func TestResourceStateCacheEviction(t *testing.T) {
	c := newResourceStateCache(3)
	for i := 0; i < 5; i++ {
		c.put("k"+strconv.Itoa(i), []byte(strconv.Itoa(i)))
	}
	if c.len() != 3 {
		t.Fatalf("expected 3 entries, got %d", c.len())
	}
	// k0 and k1 should have been evicted (LRU order).
	if _, ok := c.get("k0"); ok {
		t.Fatalf("k0 should have been evicted")
	}
	if _, ok := c.get("k1"); ok {
		t.Fatalf("k1 should have been evicted")
	}
	for _, k := range []string{"k2", "k3", "k4"} {
		if _, ok := c.get(k); !ok {
			t.Fatalf("%s should still be present", k)
		}
	}
}

func TestResourceStateCacheLRURefresh(t *testing.T) {
	c := newResourceStateCache(3)
	c.put("a", []byte("1"))
	c.put("b", []byte("2"))
	c.put("c", []byte("3"))
	// Touch a so b is the oldest.
	if _, ok := c.get("a"); !ok {
		t.Fatalf("missing a")
	}
	c.put("d", []byte("4"))
	if _, ok := c.get("b"); ok {
		t.Fatalf("b should have been evicted after a-touch + d-put")
	}
	for _, k := range []string{"a", "c", "d"} {
		if _, ok := c.get(k); !ok {
			t.Fatalf("%s should still be present", k)
		}
	}
}

func TestResourceStateCacheDrop(t *testing.T) {
	c := newResourceStateCache(3)
	c.put("a", []byte("1"))
	c.drop("a")
	if _, ok := c.get("a"); ok {
		t.Fatalf("a should have been dropped")
	}
	if c.len() != 0 {
		t.Fatalf("expected len 0, got %d", c.len())
	}
}

func TestResourceStateCacheNilSafe(t *testing.T) {
	var c *resourceStateCache
	c.put("a", []byte("1"))      // no panic
	if _, ok := c.get("a"); ok { // always miss
		t.Fatal("nil cache should not return hits")
	}
	c.drop("a")       // no panic
	if c.len() != 0 { // returns 0
		t.Fatalf("expected 0")
	}
}
