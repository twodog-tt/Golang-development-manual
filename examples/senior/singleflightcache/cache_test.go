package singleflightcache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCacheMergesConcurrentLoads(t *testing.T) {
	cache, err := New[string, string](time.Minute, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	var loads atomic.Int32
	start := make(chan struct{})
	release := make(chan struct{})
	loader := func(context.Context, string) (string, error) {
		if loads.Add(1) == 1 {
			close(start)
		}
		<-release
		return "value", nil
	}

	const callers = 20
	var wg sync.WaitGroup
	wg.Add(callers)
	errs := make(chan error, callers)
	for range callers {
		go func() {
			defer wg.Done()
			value, _, err := cache.Get(context.Background(), "key", loader)
			if err != nil {
				errs <- err
				return
			}
			if value != "value" {
				errs <- errors.New("unexpected value")
			}
		}()
	}

	<-start
	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if got := loads.Load(); got != 1 {
		t.Fatalf("loader calls = %d, want 1", got)
	}
}

func TestWaiterCanCancelWithoutCancellingSharedLoad(t *testing.T) {
	cache, err := New[string, string](time.Minute, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	leaderDone := make(chan error, 1)
	loader := func(ctx context.Context, _ string) (string, error) {
		close(started)
		select {
		case <-release:
			return "value", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	go func() {
		value, _, err := cache.Get(context.Background(), "key", loader)
		if err == nil && value != "value" {
			err = errors.New("leader received unexpected value")
		}
		leaderDone <- err
	}()
	<-started

	waitCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, shared, err := cache.Get(waitCtx, "key", loader)
	if !shared {
		t.Fatal("waiter did not join shared call")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiter error = %v, want deadline exceeded", err)
	}

	close(release)
	if err := <-leaderDone; err != nil {
		t.Fatal(err)
	}
}

func TestLoaderFailureIsNotCached(t *testing.T) {
	cache, err := New[string, string](time.Minute, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	wantErr := errors.New("backend unavailable")
	var loads atomic.Int32
	loader := func(context.Context, string) (string, error) {
		loads.Add(1)
		return "", wantErr
	}

	for range 2 {
		_, _, err := cache.Get(context.Background(), "key", loader)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	}
	if got := loads.Load(); got != 2 {
		t.Fatalf("loader calls = %d, want 2", got)
	}
}
