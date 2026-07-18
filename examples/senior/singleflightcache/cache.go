package singleflightcache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrInvalidConfig = errors.New("singleflightcache: ttl and load timeout must be positive")

type entry[V any] struct {
	value     V
	expiresAt time.Time
}

type call[V any] struct {
	done chan struct{}
	val  V
	err  error
}

// Group merges concurrent work for the same key inside one process.
// The waiting context does not cancel the shared function.
type Group[K comparable, V any] struct {
	mu    sync.Mutex
	calls map[K]*call[V]
}

func (g *Group[K, V]) Do(
	ctx context.Context,
	key K,
	fn func() (V, error),
) (value V, shared bool, err error) {
	if err := ctx.Err(); err != nil {
		return value, false, err
	}

	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[K]*call[V])
	}
	if current, ok := g.calls[key]; ok {
		g.mu.Unlock()
		select {
		case <-current.done:
			return current.val, true, current.err
		case <-ctx.Done():
			return value, true, ctx.Err()
		}
	}

	current := &call[V]{done: make(chan struct{})}
	g.calls[key] = current
	g.mu.Unlock()

	var panicValue any
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				panicValue = recovered
				current.err = fmt.Errorf("singleflightcache: loader panic: %v", recovered)
			}
		}()
		current.val, current.err = fn()
	}()

	g.mu.Lock()
	delete(g.calls, key)
	close(current.done)
	g.mu.Unlock()

	if panicValue != nil {
		panic(panicValue)
	}
	return current.val, false, current.err
}

type Cache[K comparable, V any] struct {
	mu          sync.RWMutex
	entries     map[K]entry[V]
	ttl         time.Duration
	loadTimeout time.Duration
	now         func() time.Time
	group       Group[K, V]
}

func New[K comparable, V any](ttl, loadTimeout time.Duration) (*Cache[K, V], error) {
	if ttl <= 0 || loadTimeout <= 0 {
		return nil, ErrInvalidConfig
	}
	return &Cache[K, V]{
		entries:     make(map[K]entry[V]),
		ttl:         ttl,
		loadTimeout: loadTimeout,
		now:         time.Now,
	}, nil
}

// Get returns value, whether it joined an in-flight load, and an error.
// loader receives a context detached from request cancellation but bounded by
// loadTimeout. This prevents the first waiter from cancelling work for all.
func (c *Cache[K, V]) Get(
	ctx context.Context,
	key K,
	loader func(context.Context, K) (V, error),
) (V, bool, error) {
	if value, ok := c.load(key); ok {
		return value, false, nil
	}

	return c.group.Do(ctx, key, func() (V, error) {
		if value, ok := c.load(key); ok {
			return value, nil
		}

		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.loadTimeout)
		defer cancel()

		value, err := loader(loadCtx, key)
		if err != nil {
			var zero V
			return zero, err
		}
		c.store(key, value)
		return value, nil
	})
}

func (c *Cache[K, V]) load(key K) (V, bool) {
	now := c.now()
	c.mu.RLock()
	item, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || !now.Before(item.expiresAt) {
		var zero V
		return zero, false
	}
	return item.value, true
}

func (c *Cache[K, V]) store(key K, value V) {
	c.mu.Lock()
	c.entries[key] = entry[V]{
		value:     value,
		expiresAt: c.now().Add(c.ttl),
	}
	c.mu.Unlock()
}
