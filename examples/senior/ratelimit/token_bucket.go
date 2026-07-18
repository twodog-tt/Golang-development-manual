// Package ratelimit 令牌桶限流器（面试手写题 S-CODE-02）。
package ratelimit

import (
	"context"
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

// NewTokenBucket 对编程错误使用 panic；生产库也可改成返回 (*TokenBucket, error)。
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

// Wait 阻塞直到获得一个令牌或 ctx 取消。
func (tb *TokenBucket) Wait(ctx context.Context) error {
	if ctx == nil {
		panic("ratelimit: nil context")
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		wait := tb.takeOrDelay()
		if wait == 0 {
			return nil
		}

		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		}
	}
}

// takeOrDelay 在令牌可用时消费一个令牌，否则返回理论等待时间。
// 多个 waiter 醒来后仍需重新竞争和计算。
func (tb *TokenBucket) takeOrDelay() time.Duration {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()
	if tb.tokens >= 1 {
		tb.tokens--
		return 0
	}

	wait := time.Duration((1 - tb.tokens) / tb.rate * float64(time.Second))
	if wait <= 0 {
		return time.Nanosecond
	}
	return wait
}
