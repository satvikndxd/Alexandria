package ratelimit

import (
	"testing"
	"time"
)

func TestBurstThenThrottle(t *testing.T) {
	l := New(60, 3) // 1/sec sustained, burst 3
	now := time.Unix(0, 0)
	l.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if !l.Allow("k") {
			t.Fatalf("request %d within burst should pass", i)
		}
	}
	if l.Allow("k") {
		t.Fatal("burst exhausted, request should be throttled")
	}

	now = now.Add(2 * time.Second) // refills 2 tokens
	if !l.Allow("k") || !l.Allow("k") {
		t.Fatal("refilled tokens should allow two requests")
	}
	if l.Allow("k") {
		t.Fatal("third request after refill should be throttled")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	l := New(60, 1)
	if !l.Allow("a") {
		t.Fatal("first request for key a should pass")
	}
	if !l.Allow("b") {
		t.Fatal("key b must not share key a's bucket")
	}
}
