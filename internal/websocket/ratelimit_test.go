package websocket

import (
	"sync"
	"testing"
	"time"
)

func TestTokenBucket_RateLimiting(t *testing.T) {
	interval := 100 * time.Millisecond
	tb := NewTokenBucket(1.0, interval)

	// First allow should succeed
	if !tb.Allow() {
		t.Fatalf("expected first token to be allowed")
	}

	// Immediate second attempt should be rejected
	if tb.Allow() {
		t.Fatalf("expected immediate second token to be rejected")
	}

	// Wait for refill
	time.Sleep(150 * time.Millisecond)

	if !tb.Allow() {
		t.Fatalf("expected token after refill to be allowed")
	}
}

func TestUserRateLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewUserRateLimiter(50 * time.Millisecond)
	users := []string{"user-1", "user-2", "user-3", "user-4"}

	var wg sync.WaitGroup
	for _, u := range users {
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(uid string) {
				defer wg.Done()
				_ = limiter.Allow(uid)
			}(u)
		}
	}
	wg.Wait()
}

func TestLocalRoom_CurrentPosition_Concurrency(t *testing.T) {
	room := &LocalRoom{
		isPlaying:   true,
		positionSec: 100.0,
		updatedAt:   time.Now().Add(-10 * time.Second),
		clients:     make(map[*RoomClient]bool),
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pos, playing := room.CurrentPosition()
			if !playing {
				t.Errorf("expected playing to be true")
			}
			if pos < 109.0 {
				t.Errorf("expected pos to be >= 109.0, got %f", pos)
			}
		}()
	}
	wg.Wait()
}
