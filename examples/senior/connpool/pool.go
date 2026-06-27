// Package connpool 基于 channel 的简易连接池（面试手写题 S-CODE-05）。
package connpool

import (
	"context"
	"errors"
	"sync"
)

var ErrPoolClosed = errors.New("connpool: pool closed")

// Conn 抽象连接，生产可替换为 net.Conn 或 sql.DB 包装。
type Conn interface {
	Close() error
}

type factory func() (Conn, error)

type Pool struct {
	ch      chan Conn
	factory factory
	mu      sync.Mutex
	closed  bool
}

func New(maxIdle int, factory factory) (*Pool, error) {
	if maxIdle <= 0 {
		return nil, errors.New("connpool: maxIdle must be positive")
	}
	return &Pool{
		ch:      make(chan Conn, maxIdle),
		factory: factory,
	}, nil
}

func (p *Pool) Get(ctx context.Context) (Conn, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, ErrPoolClosed
	}
	p.mu.Unlock()

	select {
	case c := <-p.ch:
		return c, nil
	default:
		return p.factory()
	}
}

func (p *Pool) Put(c Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		c.Close()
		return
	}
	select {
	case p.ch <- c:
	default:
		c.Close()
	}
}

func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.closed = true
	close(p.ch)
	for c := range p.ch {
		c.Close()
	}
}
