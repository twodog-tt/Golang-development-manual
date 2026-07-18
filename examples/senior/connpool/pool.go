// Package connpool 提供一个有 maxOpen/maxIdle 边界的简易连接池
// （面试手写题 S-CODE-05）。
package connpool

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrPoolClosed = errors.New("connpool: pool closed")
	ErrNilConn    = errors.New("connpool: factory returned nil connection")
)

// Conn 抽象连接，生产可替换为 net.Conn 或 sql.DB 包装。
type Conn interface {
	Close() error
}

type Factory func() (Conn, error)

type Pool struct {
	idle    chan Conn
	slots   chan struct{}
	done    chan struct{}
	factory Factory

	mu     sync.Mutex
	closed bool
}

func New(maxOpen, maxIdle int, factory Factory) (*Pool, error) {
	if maxOpen <= 0 {
		return nil, errors.New("connpool: maxOpen must be positive")
	}
	if maxIdle < 0 || maxIdle > maxOpen {
		return nil, errors.New("connpool: maxIdle must be between 0 and maxOpen")
	}
	if factory == nil {
		return nil, errors.New("connpool: factory must not be nil")
	}
	return &Pool{
		idle:    make(chan Conn, maxIdle),
		slots:   make(chan struct{}, maxOpen),
		done:    make(chan struct{}),
		factory: factory,
	}, nil
}

func (p *Pool) Get(ctx context.Context) (Conn, error) {
	if ctx == nil {
		return nil, errors.New("connpool: nil context")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if p.isClosed() {
		return nil, ErrPoolClosed
	}

	// 优先复用空闲连接，避免有 idle 时仍无谓创建新连接。
	select {
	case c := <-p.idle:
		return p.checkout(c)
	default:
	}

	// idle 为空时：未达 maxOpen 可占 slot 创建；否则等待归还。
	select {
	case c := <-p.idle:
		return p.checkout(c)
	case p.slots <- struct{}{}:
		return p.open()
	case <-p.done:
		return nil, ErrPoolClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *Pool) open() (Conn, error) {
	if p.isClosed() {
		p.releaseSlot()
		return nil, ErrPoolClosed
	}

	c, err := p.factory()
	if err != nil {
		p.releaseSlot()
		return nil, err
	}
	if c == nil {
		p.releaseSlot()
		return nil, ErrNilConn
	}
	if p.isClosed() {
		_ = p.closeAndRelease(c)
		return nil, ErrPoolClosed
	}
	return c, nil
}

func (p *Pool) checkout(c Conn) (Conn, error) {
	if c == nil {
		p.releaseSlot()
		return nil, ErrNilConn
	}
	if p.isClosed() {
		_ = p.closeAndRelease(c)
		return nil, ErrPoolClosed
	}
	return c, nil
}

// Put 归还一个由 Get 借出的健康连接。坏连接应调用 Discard。
func (p *Pool) Put(c Conn) {
	if c == nil {
		return
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		_ = p.closeAndRelease(c)
		return
	}
	select {
	case p.idle <- c:
		p.mu.Unlock()
	default:
		p.mu.Unlock()
		_ = p.closeAndRelease(c)
	}
}

// Discard 关闭一个由 Get 借出的坏连接，并释放 maxOpen slot。
func (p *Pool) Discard(c Conn) error {
	if c == nil {
		return nil
	}
	return p.closeAndRelease(c)
}

// Close 阻止新的 Get、唤醒等待者并关闭当前 idle 连接。
// 已借出的连接会在之后 Put/Discard 时关闭；本方法不等待借用方。
func (p *Pool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	close(p.done)
	p.mu.Unlock()

	var firstErr error
	for {
		select {
		case c := <-p.idle:
			if err := p.closeAndRelease(c); err != nil && firstErr == nil {
				firstErr = err
			}
		default:
			return firstErr
		}
	}
}

func (p *Pool) isClosed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closed
}

func (p *Pool) closeAndRelease(c Conn) error {
	err := c.Close()
	p.releaseSlot()
	return err
}

func (p *Pool) releaseSlot() {
	select {
	case <-p.slots:
	default:
		panic("connpool: connection returned more than once")
	}
}
