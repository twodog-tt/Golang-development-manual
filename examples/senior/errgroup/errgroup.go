// Package errgroup 简化版 errgroup（面试手写题 S-CODE-04）。
// 语义：任一 goroutine 返回 error 则取消 context，Wait 返回首个 error。
package errgroup

import (
	"context"
	"sync"
)

type Group struct {
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
	errOnce sync.Once
	err    error
}

func WithContext(parent context.Context) (*Group, context.Context) {
	ctx, cancel := context.WithCancel(parent)
	return &Group{ctx: ctx, cancel: cancel}, ctx
}

func (g *Group) Go(fn func() error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if err := fn(); err != nil {
			g.errOnce.Do(func() {
				g.err = err
				g.cancel()
			})
		}
	}()
}

func (g *Group) Wait() error {
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel()
	}
	return g.err
}
