// Package ratelimit implements a per-key token bucket used by HTTP middleware.
// Buckets refill continuously; stale buckets are evicted by a janitor so the
// map cannot grow unboundedly under scanner traffic.
package ratelimit

import (
	"sync"
	"time"
)

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

type Limiter struct {
	mu        sync.Mutex
	buckets   map[string]*bucket
	rate      float64 // tokens added per second
	burst     float64 // bucket capacity
	now       func() time.Time
	lastSweep time.Time
}

// New creates a limiter allowing `burst` immediate requests and a sustained
// rate of `perMinute` requests per minute per key.
func New(perMinute int, burst int) *Limiter {
	return &Limiter{
		buckets: make(map[string]*bucket),
		rate:    float64(perMinute) / 60.0,
		burst:   float64(burst),
		now:     time.Now,
	}
}

// Allow reports whether the caller identified by key may proceed.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, lastSeen: now}
		l.buckets[key] = b
	}

	// Refill.
	elapsed := now.Sub(b.lastSeen).Seconds()
	b.tokens = min(l.burst, b.tokens+elapsed*l.rate)
	b.lastSeen = now

	// Opportunistic sweep every minute to drop idle buckets.
	if now.Sub(l.lastSweep) > time.Minute {
		for k, v := range l.buckets {
			if now.Sub(v.lastSeen) > 10*time.Minute {
				delete(l.buckets, k)
			}
		}
		l.lastSweep = now
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
