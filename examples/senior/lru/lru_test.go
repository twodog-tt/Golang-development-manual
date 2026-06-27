package lru

import "testing"

func TestLRU_EvictOldest(t *testing.T) {
	c := New(2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)

	if _, ok := c.Get("a"); ok {
		t.Fatal("a should be evicted")
	}
	if v, ok := c.Get("b"); !ok || v != 2 {
		t.Fatalf("b = %v, ok=%v", v, ok)
	}
}

func TestLRU_GetRefreshesOrder(t *testing.T) {
	c := New(2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Get("a")
	c.Put("c", 3)

	if _, ok := c.Get("b"); ok {
		t.Fatal("b should be evicted after a was refreshed")
	}
}
