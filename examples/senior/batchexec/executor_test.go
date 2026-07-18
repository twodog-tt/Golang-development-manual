package batchexec

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestMapOrderedPreservesInputOrderAndLimit(t *testing.T) {
	items := []int{5, 4, 3, 2, 1}
	var active atomic.Int32
	var maximum atomic.Int32

	got, err := MapOrdered(context.Background(), items, 2, 1, func(_ context.Context, value int) (int, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			old := maximum.Load()
			if current <= old || maximum.CompareAndSwap(old, current) {
				break
			}
		}
		time.Sleep(time.Duration(value) * time.Millisecond)
		return value * 10, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []int{50, 40, 30, 20, 10}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("result[%d] = %d, want %d", index, got[index], want[index])
		}
	}
	if maximum.Load() > 2 {
		t.Fatalf("maximum concurrency = %d, want <= 2", maximum.Load())
	}
}

func TestMapOrderedCancelsOnFirstError(t *testing.T) {
	wantErr := errors.New("boom")
	started := make(chan struct{}, 3)

	_, err := MapOrdered(context.Background(), []int{1, 2, 3, 4}, 2, 0, func(ctx context.Context, value int) (int, error) {
		started <- struct{}{}
		if value == 2 {
			return 0, wantErr
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(time.Second):
			return value, nil
		}
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestMapOrderedParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := MapOrdered(ctx, []int{1}, 1, 0, func(context.Context, int) (int, error) {
		t.Fatal("callback should not run")
		return 0, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}
