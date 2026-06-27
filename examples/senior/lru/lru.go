// Package lru 提供并发安全的 LRU 缓存（面试手写题 S-CODE-01）。
package lru

import (
	"container/list"
	"sync"
)

type entry struct {
	key   string
	value interface{}
}

// Cache 基于 map + 双向链表，所有操作 O(1)。
type Cache struct {
	cap   int
	mu    sync.Mutex
	items map[string]*list.Element
	order *list.List
}

func New(capacity int) *Cache {
	if capacity <= 0 {
		panic("lru: capacity must be positive")
	}
	return &Cache{
		cap:   capacity,
		items: make(map[string]*list.Element),
		order: list.New(),
	}
}

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(elem)
	return elem.Value.(*entry).value, true
}

func (c *Cache) Put(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		elem.Value.(*entry).value = value
		c.order.MoveToFront(elem)
		return
	}

	elem := c.order.PushFront(&entry{key: key, value: value})
	c.items[key] = elem

	if c.order.Len() > c.cap {
		back := c.order.Back()
		c.order.Remove(back)
		delete(c.items, back.Value.(*entry).key)
	}
}

func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}
