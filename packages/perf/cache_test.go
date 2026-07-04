package perf

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestCache_SetGet(t *testing.T) {
	c := NewCache[string, int](time.Minute)

	if _, ok := c.Get("missing"); ok {
		t.Fatal("expected Get on empty cache to report false")
	}

	c.Set("a", 1)
	got, ok := c.Get("a")
	if !ok {
		t.Fatal("expected Get to find key just Set")
	}
	if got != 1 {
		t.Fatalf("expected value 1, got %d", got)
	}
}

func TestCache_Invalidate(t *testing.T) {
	c := NewCache[string, int](time.Minute)
	c.Set("a", 1)
	c.Invalidate("a")

	if _, ok := c.Get("a"); ok {
		t.Fatal("expected Get after Invalidate to report false")
	}

	// Invalidating an absent key must not panic or error.
	c.Invalidate("does-not-exist")
}

func TestCache_ExpiryWithInjectedClock(t *testing.T) {
	current := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return current }

	c := newCacheWithClock[string, string](10*time.Second, clock)
	c.Set("k", "v")

	// Still within TTL: must be found.
	current = current.Add(9 * time.Second)
	if _, ok := c.Get("k"); !ok {
		t.Fatal("expected entry to still be valid 9s into a 10s TTL")
	}

	// Past TTL: must report a miss.
	current = current.Add(2 * time.Second)
	if _, ok := c.Get("k"); ok {
		t.Fatal("expected entry to have expired 11s into a 10s TTL")
	}
}

func TestCache_ZeroTTLNeverExpires(t *testing.T) {
	current := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return current }

	c := newCacheWithClock[string, string](0, clock)
	c.Set("k", "v")

	current = current.Add(365 * 24 * time.Hour)
	if _, ok := c.Get("k"); !ok {
		t.Fatal("expected a zero-TTL entry to never expire")
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := NewCache[int, int](time.Minute)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c.Set(i, i*2)
			c.Get(i)
			c.Invalidate(i)
		}(i)
	}
	wg.Wait()
}

func TestCache_Len(t *testing.T) {
	c := NewCache[string, int](time.Minute)
	for i := 0; i < 5; i++ {
		c.Set(strconv.Itoa(i), i)
	}
	if got := c.Len(); got != 5 {
		t.Fatalf("expected Len 5, got %d", got)
	}
}
