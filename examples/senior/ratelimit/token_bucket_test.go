package ratelimit

import (
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
