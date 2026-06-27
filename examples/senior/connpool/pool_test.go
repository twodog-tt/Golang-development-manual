package connpool

import (
	"context"
	"testing"
)

type fakeConn struct{ id int }

func (f *fakeConn) Close() error { return nil }

func TestPool_Reuse(t *testing.T) {
	n := 0
	p, err := New(2, func() (Conn, error) {
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

	if n > 2 {
		t.Fatalf("expected reuse, factory called %d times", n)
	}
	p.Put(c2)
	p.Put(c3)
	p.Close()
}
