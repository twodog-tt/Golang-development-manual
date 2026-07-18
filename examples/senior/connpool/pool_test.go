package connpool

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type fakeConn struct {
	id     int
	closed atomic.Bool
}

func (f *fakeConn) Close() error {
	f.closed.Store(true)
	return nil
}

func TestPool_Reuse(t *testing.T) {
	n := 0
	p, err := New(2, 2, func() (Conn, error) {
		n++
		return &fakeConn{id: n}, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	c1, _ := p.Get(context.Background())
	c2, _ := p.Get(context.Background())
	p.Put(c1)
	c3, _ := p.Get(context.Background())

	if n != 2 {
		t.Fatalf("expected two physical connections, factory called %d times", n)
	}
	p.Put(c2)
	p.Put(c3)
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPool_MaxOpenHonorsContext(t *testing.T) {
	p, err := New(1, 1, func() (Conn, error) {
		return &fakeConn{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	c, err := p.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := p.Get(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want deadline exceeded, got %v", err)
	}

	p.Put(c)
	reused, err := p.Get(context.Background())
	if err != nil {
		t.Fatalf("expected returned connection to become available: %v", err)
	}
	p.Put(reused)
}

func TestPool_CloseWakesWaiterAndClosesBorrowedOnReturn(t *testing.T) {
	p, err := New(1, 1, func() (Conn, error) {
		return &fakeConn{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	c, err := p.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	fc := c.(*fakeConn)

	waitErr := make(chan error, 1)
	go func() {
		_, err := p.Get(context.Background())
		waitErr <- err
	}()

	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-waitErr; !errors.Is(err, ErrPoolClosed) {
		t.Fatalf("want pool closed, got %v", err)
	}

	p.Put(c)
	if !fc.closed.Load() {
		t.Fatal("borrowed connection should be closed when returned to a closed pool")
	}
}

func TestPool_ValidatesConfiguration(t *testing.T) {
	factory := func() (Conn, error) { return &fakeConn{}, nil }
	if _, err := New(0, 0, factory); err == nil {
		t.Fatal("expected maxOpen validation error")
	}
	if _, err := New(1, 2, factory); err == nil {
		t.Fatal("expected maxIdle validation error")
	}
}
