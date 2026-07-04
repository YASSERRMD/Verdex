package perf

import (
	"errors"
	"sort"
	"sync"
	"testing"
	"time"
)

func TestBatcher_SizeTriggeredFlush(t *testing.T) {
	var mu sync.Mutex
	var flushed [][]int

	b := NewBatcher(BatcherConfig{MaxSize: 3, MaxWait: time.Hour}, func(items []int) {
		mu.Lock()
		defer mu.Unlock()
		batch := append([]int(nil), items...)
		flushed = append(flushed, batch)
	})

	for i := 1; i <= 3; i++ {
		if err := b.Add(i); err != nil {
			t.Fatalf("Add returned unexpected error: %v", err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(flushed) != 1 {
		t.Fatalf("expected exactly one size-triggered flush, got %d", len(flushed))
	}
	if len(flushed[0]) != 3 {
		t.Fatalf("expected flushed batch of size 3, got %d", len(flushed[0]))
	}
}

func TestBatcher_TimeTriggeredFlush(t *testing.T) {
	flushed := make(chan []int, 1)

	b := NewBatcher(BatcherConfig{MaxSize: 1000, MaxWait: 20 * time.Millisecond}, func(items []int) {
		batch := append([]int(nil), items...)
		flushed <- batch
	})
	defer b.Stop()

	if err := b.Add(42); err != nil {
		t.Fatalf("Add returned unexpected error: %v", err)
	}

	select {
	case batch := <-flushed:
		if len(batch) != 1 || batch[0] != 42 {
			t.Fatalf("expected time-triggered batch [42], got %v", batch)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for time-triggered flush")
	}
}

func TestBatcher_StopFlushesRemainder(t *testing.T) {
	var mu sync.Mutex
	var flushed []int

	b := NewBatcher(BatcherConfig{MaxSize: 1000, MaxWait: time.Hour}, func(items []int) {
		mu.Lock()
		defer mu.Unlock()
		flushed = append(flushed, items...)
	})

	if err := b.Add(1); err != nil {
		t.Fatalf("Add returned unexpected error: %v", err)
	}
	if err := b.Add(2); err != nil {
		t.Fatalf("Add returned unexpected error: %v", err)
	}
	b.Stop()

	mu.Lock()
	defer mu.Unlock()
	if len(flushed) != 2 {
		t.Fatalf("expected Stop to flush the 2 remaining items, got %d", len(flushed))
	}
}

func TestBatcher_AddAfterStopReturnsError(t *testing.T) {
	b := NewBatcher(BatcherConfig{MaxSize: 10, MaxWait: time.Hour}, func([]int) {})
	b.Stop()

	if err := b.Add(1); !errors.Is(err, ErrBatcherClosed) {
		t.Fatalf("expected ErrBatcherClosed, got %v", err)
	}
}

// TestBatcher_ConcurrentAddsExactlyOnce spawns many goroutines adding items
// concurrently and asserts every item is eventually flushed exactly once:
// no item lost, no item duplicated.
func TestBatcher_ConcurrentAddsExactlyOnce(t *testing.T) {
	const goroutines = 50
	const perGoroutine = 20
	const total = goroutines * perGoroutine

	var mu sync.Mutex
	seen := make(map[int]int) // item -> count flushed

	b := NewBatcher(BatcherConfig{MaxSize: 7, MaxWait: 5 * time.Millisecond}, func(items []int) {
		mu.Lock()
		defer mu.Unlock()
		for _, item := range items {
			seen[item]++
		}
	})

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				item := g*perGoroutine + i
				if err := b.Add(item); err != nil {
					t.Errorf("Add returned unexpected error: %v", err)
				}
			}
		}(g)
	}
	wg.Wait()
	b.Stop()

	mu.Lock()
	defer mu.Unlock()

	if len(seen) != total {
		t.Fatalf("expected %d distinct items flushed, got %d", total, len(seen))
	}

	var duplicated, missing []int
	for item := 0; item < total; item++ {
		count, ok := seen[item]
		if !ok {
			missing = append(missing, item)
			continue
		}
		if count != 1 {
			duplicated = append(duplicated, item)
		}
	}
	sort.Ints(missing)
	sort.Ints(duplicated)

	if len(missing) > 0 {
		t.Fatalf("items never flushed: %v", missing)
	}
	if len(duplicated) > 0 {
		t.Fatalf("items flushed more than once: %v", duplicated)
	}
}

func TestNewBatcher_PanicsOnInvalidConfig(t *testing.T) {
	assertPanics(t, func() { NewBatcher(BatcherConfig{MaxSize: 0, MaxWait: time.Second}, func([]int) {}) })
	assertPanics(t, func() { NewBatcher(BatcherConfig{MaxSize: 1, MaxWait: 0}, func([]int) {}) })
	assertPanics(t, func() { NewBatcher[int](BatcherConfig{MaxSize: 1, MaxWait: time.Second}, nil) })
}

func assertPanics(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected a panic, got none")
		}
	}()
	fn()
}
