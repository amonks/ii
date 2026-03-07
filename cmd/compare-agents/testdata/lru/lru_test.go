package lru

import (
	"fmt"
	"testing"
)

func TestBasicGetPut(t *testing.T) {
	c := New[string, int](3)

	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)

	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Errorf("Get(a) = %d, %v; want 1, true", v, ok)
	}
	if v, ok := c.Get("b"); !ok || v != 2 {
		t.Errorf("Get(b) = %d, %v; want 2, true", v, ok)
	}
	if v, ok := c.Get("c"); !ok || v != 3 {
		t.Errorf("Get(c) = %d, %v; want 3, true", v, ok)
	}
	if _, ok := c.Get("d"); ok {
		t.Error("Get(d) should return false for missing key")
	}
	if c.Len() != 3 {
		t.Errorf("Len() = %d; want 3", c.Len())
	}
}

func TestEviction(t *testing.T) {
	c := New[string, int](2)

	c.Put("a", 1)
	c.Put("b", 2)
	// Cache is full: [b, a]. Adding "c" should evict "a".
	c.Put("c", 3)

	if _, ok := c.Get("a"); ok {
		t.Error("expected a to be evicted")
	}
	if v, ok := c.Get("b"); !ok || v != 2 {
		t.Errorf("Get(b) = %d, %v; want 2, true", v, ok)
	}
	if v, ok := c.Get("c"); !ok || v != 3 {
		t.Errorf("Get(c) = %d, %v; want 3, true", v, ok)
	}
	if c.Len() != 2 {
		t.Errorf("Len() = %d; want 2", c.Len())
	}
}

func TestUpdateExisting(t *testing.T) {
	c := New[string, int](3)

	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	// List order: [c, b, a]

	// Update "a" — should move it to front.
	c.Put("a", 10)
	// Expected list order: [a, c, b]

	// Check the value was updated.
	if e := c.items["a"]; e.value != 10 {
		t.Errorf("items[a].value = %d; want 10", e.value)
	}

	// Verify the list order reflects the move-to-front.
	// (Don't use Get here — Get itself calls moveToFront,
	// which would mask the bug.)
	got := listOrder(c)
	want := []string{"a", "c", "b"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("list order after update = %v; want %v", got, want)
	}
}

func TestEvictionAfterUpdate(t *testing.T) {
	c := New[string, int](3)

	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	// List order: [c, b, a]

	// Update "a" — moves it to front.
	c.Put("a", 10)
	// Expected list order: [a, c, b]

	// Now add "d" — should evict "b" (the LRU).
	c.Put("d", 4)
	// Expected list order: [d, a, c]

	if _, ok := c.Get("b"); ok {
		t.Error("expected b to be evicted (LRU after a was updated)")
	}
	if v, ok := c.Get("a"); !ok || v != 10 {
		t.Errorf("Get(a) = %d, %v; want 10, true", v, ok)
	}
	if v, ok := c.Get("c"); !ok || v != 3 {
		t.Errorf("Get(c) = %d, %v; want 3, true", v, ok)
	}
	if v, ok := c.Get("d"); !ok || v != 4 {
		t.Errorf("Get(d) = %d, %v; want 4, true", v, ok)
	}
	if c.Len() != 3 {
		t.Errorf("Len() = %d; want 3", c.Len())
	}
}

// listOrder walks the linked list from head to tail and returns keys in order.
func listOrder[K comparable, V any](c *Cache[K, V]) []K {
	var keys []K
	for e := c.head; e != nil; e = e.next {
		keys = append(keys, e.key)
	}
	return keys
}
