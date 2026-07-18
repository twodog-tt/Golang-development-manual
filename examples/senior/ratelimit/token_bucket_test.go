package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTokenBucket_BurstThenThrottle(t *testing.T) {
	tb := NewTokenBucket(10, 3) // 10/s, burst 3

	for i := 0; i < 3; i++ {
		if !tb.Allow() {
			t.Fatalf("burst allow %d failed", i)
		}
	}
	if tb.Allow() {
		t.Fatal("fourth immediate request should be rejected")
	}

	time.Sleep(150 * time.Millisecond)
	if !tb.Allow() {
		t.Fatal("should refill after wait")
	}
}

func TestTokenBucket_WaitHonorsContext(t *testing.T) {
	tb := NewTokenBucket(0.1, 1)
	if !tb.Allow() {
		t.Fatal("initial token should be available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := tb.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want deadline exceeded, got %v", err)
	}
}

func TestTokenBucket_WaitGetsRefilledToken(t *testing.T) {
	tb := NewTokenBucket(100, 1)
	if !tb.Allow() {
		t.Fatal("initial token should be available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := tb.Wait(ctx); err != nil {
		t.Fatalf("wait: %v", err)
	}
}
