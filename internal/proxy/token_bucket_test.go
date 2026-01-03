package proxy

import (
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"database_firewall/internal/config"
)

func testRateLimiter(rate, capacity int64) *TokenBucketLimiter {
	cfg := &config.RateLimiterConfig{
		RateLimiter: config.RateLimiterC{
			TokenBucketLimiter: config.TokenBucketLimiterC{
				Rate:     rate,
				Capacity: capacity,
			},
		},
	}
	return NewTokenBucketLimiter(cfg)
}

func TestTokenBucketLimiter_BurstCapacity(t *testing.T) {
	rl := testRateLimiter(1, 3)
	ip := net.ParseIP("10.0.0.1")

	for i := 0; i < 3; i++ {
		if !rl.Allow(ip) {
			t.Fatalf("expected allow at token %d", i)
		}
	}

	if rl.Allow(ip) {
		t.Fatal("expected rejection after capacity exhausted")
	}
}

func TestTokenBucketLimiter_ZeroRateUnlimited(t *testing.T) {
	rl := testRateLimiter(0, 1)
	ip := net.ParseIP("10.0.0.1")

	for i := 0; i < 1000; i++ {
		if !rl.Allow(ip) {
			t.Fatal("expected unlimited allows when rate == 0")
		}
	}
}

func TestTokenBucketLimiter_Refill(t *testing.T) {
	rl := testRateLimiter(10, 1) // 10 tokens/sec
	ip := net.ParseIP("10.0.0.1")

	if !rl.Allow(ip) {
		t.Fatal("expected initial allow")
	}

	if rl.Allow(ip) {
		t.Fatal("expected rejection before refill")
	}

	time.Sleep(120 * time.Millisecond)

	if !rl.Allow(ip) {
		t.Fatal("expected allow after refill")
	}
}

func TestTokenBucketLimiter_PerIPIsolation(t *testing.T) {
	rl := testRateLimiter(1, 1)

	ip1 := net.ParseIP("10.0.0.1")
	ip2 := net.ParseIP("10.0.0.2")

	if !rl.Allow(ip1) {
		t.Fatal("ip1 should be allowed")
	}

	if !rl.Allow(ip2) {
		t.Fatal("ip2 should be allowed independently")
	}

	if rl.Allow(ip1) {
		t.Fatal("ip1 should now be rate-limited")
	}
}

func TestTokenBucketLimiter_Concurrency(t *testing.T) {
	rl := testRateLimiter(1, 1)
	ip := net.ParseIP("10.0.0.1")

	const goroutines = 100
	start := make(chan struct{})
	var wg sync.WaitGroup

	var allowed int64

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if rl.Allow(ip) {
				atomic.AddInt64(&allowed, 1)
			}
		}()
	}

	close(start)
	wg.Wait()

	if allowed != 1 {
		t.Fatalf("expected exactly 1 allow, got %d", allowed)
	}
}
