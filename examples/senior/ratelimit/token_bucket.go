// Package ratelimit 令牌桶限流器（面试手写题 S-CODE-02）。
package ratelimit

import (
	"sync"
	"time"
)

// TokenBucket 按固定速率补充令牌，允许短时 burst。
type TokenBucket struct {
	mu         sync.Mutex
	rate       float64 // tokens per second
	burst      float64
	tokens     float64
	lastRefill time.Time
}

func NewTokenBucket(ratePerSec float64, burst int) *TokenBucket {
	if ratePerSec <= 0 || burst <= 0 {
		panic("ratelimit: rate and burst must be positive")
	}
	return &TokenBucket{
		rate:       ratePerSec,
		burst:      float64(burst),
		tokens:     float64(burst),
		lastRefill: time.Now(),
	}
}

func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	if elapsed <= 0 {
		return
	}
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.burst {
		tb.tokens = tb.burst
	}
	tb.lastRefill = now
}

// Allow 尝试消耗 1 个令牌，成功返回 true。
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	if tb.tokens < 1 {
		return false
	}
	tb.tokens -= 1
	return true
}

// Wait 阻塞直到获得令牌或 ctx 取消（生产可用 time.Timer + select）。
func (tb *TokenBucket) Wait() {
	for !tb.Allow() {
		time.Sleep(time.Millisecond)
	}
}
