// Package lru implements a generic least-recently-used cache.
package lru

// entry is a node in the doubly-linked list.
type entry[K comparable, V any] struct {
	key        K
	value      V
	prev, next *entry[K, V]
}

// Cache is a fixed-capacity LRU cache.
type Cache[K comparable, V any] struct {
	capacity int
	items    map[K]*entry[K, V]
	// head is the most recently used; tail is the least recently used.
	head, tail *entry[K, V]
}

// New creates a new LRU cache with the given capacity.
func New[K comparable, V any](capacity int) *Cache[K, V] {
	if capacity <= 0 {
		panic("lru: capacity must be positive")
	}
	return &Cache[K, V]{
		capacity: capacity,
		items:    make(map[K]*entry[K, V]),
	}
}

// Get retrieves a value and marks it as recently used.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	e, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	c.moveToFront(e)
	return e.value, true
}

// Put inserts or updates a key-value pair.
func (c *Cache[K, V]) Put(key K, value V) {
	if e, ok := c.items[key]; ok {
		// Update existing entry.
		e.value = value
		return
	}

	// New entry.
	e := &entry[K, V]{key: key, value: value}
	c.pushFront(e)
	c.items[key] = e

	if len(c.items) > c.capacity {
		c.evictLRU()
	}
}

// Len returns the number of entries in the cache.
func (c *Cache[K, V]) Len() int {
	return len(c.items)
}

// moveToFront unlinks an entry and pushes it to front.
func (c *Cache[K, V]) moveToFront(e *entry[K, V]) {
	c.unlink(e)
	c.pushFront(e)
}

// unlink removes an entry from the linked list without
// removing it from the map.
func (c *Cache[K, V]) unlink(e *entry[K, V]) {
	if e.prev != nil {
		e.prev.next = e.next
	} else {
		c.head = e.next
	}
	if e.next != nil {
		e.next.prev = e.prev
	} else {
		c.tail = e.prev
	}
	e.prev = nil
	e.next = nil
}

// pushFront inserts an entry at the head of the list.
func (c *Cache[K, V]) pushFront(e *entry[K, V]) {
	e.next = c.head
	e.prev = nil
	if c.head != nil {
		c.head.prev = e
	}
	c.head = e
	if c.tail == nil {
		c.tail = e
	}
}

// evictLRU removes the tail entry.
func (c *Cache[K, V]) evictLRU() {
	if c.tail == nil {
		return
	}
	e := c.tail
	c.unlink(e)
	delete(c.items, e.key)
}
