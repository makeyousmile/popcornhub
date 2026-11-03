package websocket

import (
	"sync"
	"time"
)

// TokenBucket implements a token bucket rate limiter for user control commands.
type TokenBucket struct {
	mu         sync.Mutex
	capacity   float64
	tokens     float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

func NewTokenBucket(capacity float64, refillInterval time.Duration) *TokenBucket {
	rate := 1.0 / refillInterval.Seconds()
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: rate,
		lastRefill: time.Now(),
	}
}

func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.lastRefill = now

	// Refill tokens
	tb.tokens = tb.tokens + elapsed*tb.refillRate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}

	// Consume 1 token
	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}

	return false
}

// UserRateLimiter manages token buckets per user.
type UserRateLimiter struct {
	mu             sync.Mutex
	interval       time.Duration
	buckets        map[string]*TokenBucket
	lastAccessTime map[string]time.Time
}

func NewUserRateLimiter(interval time.Duration) *UserRateLimiter {
	limiter := &UserRateLimiter{
		interval:       interval,
		buckets:        make(map[string]*TokenBucket),
		lastAccessTime: make(map[string]time.Time),
	}

	// Periodic cleanup of stale buckets every 5 minutes
	go limiter.cleanupLoop()

	return limiter
}

func (l *UserRateLimiter) Allow(userID string) bool {
	l.mu.Lock()
	bucket, ok := l.buckets[userID]
	if !ok {
		bucket = NewTokenBucket(1.0, l.interval)
		l.buckets[userID] = bucket
	}
	l.lastAccessTime[userID] = time.Now()
	l.mu.Unlock()

	return bucket.Allow()
}

func (l *UserRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		l.mu.Lock()
		cutoff := time.Now().Add(-10 * time.Minute)
		for uid, t := range l.lastAccessTime {
			if t.Before(cutoff) {
				delete(l.buckets, uid)
				delete(l.lastAccessTime, uid)
			}
		}
		l.mu.Unlock()
	}
}
