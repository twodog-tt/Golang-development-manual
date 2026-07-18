package rpcpool

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type endpointFunc struct {
	name  string
	calls atomic.Int32
	read  func(context.Context, string) (string, error)
}

func (e *endpointFunc) Name() string {
	return e.name
}

func (e *endpointFunc) Read(ctx context.Context, key string) (string, error) {
	e.calls.Add(1)
	return e.read(ctx, key)
}

func TestReadHedgesSlowEndpoint(t *testing.T) {
	slow := &endpointFunc{
		name: "slow",
		read: func(ctx context.Context, _ string) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		},
	}
	fast := &endpointFunc{
		name: "fast",
		read: func(context.Context, string) (string, error) {
			return "value", nil
		},
	}
	pool, err := New([]Endpoint{slow, fast}, 5*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	value, err := pool.Read(context.Background(), "key")
	if err != nil {
		t.Fatal(err)
	}
	if value != "value" {
		t.Fatalf("value = %q, want value", value)
	}
	if slow.calls.Load() != 1 || fast.calls.Load() != 1 {
		t.Fatalf("calls slow=%d fast=%d", slow.calls.Load(), fast.calls.Load())
	}
}

func TestReadDoesNotHedgeFastSuccess(t *testing.T) {
	first := &endpointFunc{
		name: "first",
		read: func(context.Context, string) (string, error) {
			return "value", nil
		},
	}
	second := &endpointFunc{
		name: "second",
		read: func(context.Context, string) (string, error) {
			return "unused", nil
		},
	}
	pool, _ := New([]Endpoint{first, second}, 50*time.Millisecond)

	value, err := pool.Read(context.Background(), "key")
	if err != nil || value != "value" {
		t.Fatalf("value=%q error=%v", value, err)
	}
	if second.calls.Load() != 0 {
		t.Fatalf("second endpoint calls = %d, want 0", second.calls.Load())
	}
}

func TestReadReturnsJoinedFailures(t *testing.T) {
	errA := errors.New("a failed")
	errB := errors.New("b failed")
	a := &endpointFunc{
		name: "a",
		read: func(context.Context, string) (string, error) {
			return "", errA
		},
	}
	b := &endpointFunc{
		name: "b",
		read: func(context.Context, string) (string, error) {
			return "", errB
		},
	}
	pool, _ := New([]Endpoint{a, b}, time.Second)

	_, err := pool.Read(context.Background(), "key")
	if !errors.Is(err, errA) || !errors.Is(err, errB) {
		t.Fatalf("error = %v, want both endpoint failures", err)
	}
}

func TestReadHonorsParentCancellation(t *testing.T) {
	endpoint := &endpointFunc{
		name: "blocked",
		read: func(ctx context.Context, _ string) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		},
	}
	pool, _ := New([]Endpoint{endpoint}, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := pool.Read(ctx, "key")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}
