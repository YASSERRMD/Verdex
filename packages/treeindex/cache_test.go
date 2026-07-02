package treeindex

import "testing"

func TestNewLRUCache_InvalidCapacity(t *testing.T) {
	if _, err := newLRUCache(0); err != ErrInvalidCacheCapacity {
		t.Errorf("newLRUCache(0): expected ErrInvalidCacheCapacity, got %v", err)
	}
	if _, err := newLRUCache(-1); err != ErrInvalidCacheCapacity {
		t.Errorf("newLRUCache(-1): expected ErrInvalidCacheCapacity, got %v", err)
	}
}

func TestLRUCache_GetPutMiss(t *testing.T) {
	c, err := newLRUCache(2)
	if err != nil {
		t.Fatalf("newLRUCache: %v", err)
	}

	key := lookupKey{caseID: "case-1", fromNodeID: "rule-1", edgeType: "governs"}
	if _, ok := c.get(key); ok {
		t.Fatal("expected a miss on an empty cache")
	}

	want := []Path{{CaseID: "case-1"}}
	c.put(key, want)

	got, ok := c.get(key)
	if !ok {
		t.Fatal("expected a hit after put")
	}
	if len(got) != len(want) {
		t.Errorf("got %d paths, want %d", len(got), len(want))
	}
}

func TestLRUCache_EvictsLeastRecentlyUsed(t *testing.T) {
	c, err := newLRUCache(2)
	if err != nil {
		t.Fatalf("newLRUCache: %v", err)
	}

	k1 := lookupKey{caseID: "case-1", fromNodeID: "a"}
	k2 := lookupKey{caseID: "case-1", fromNodeID: "b"}
	k3 := lookupKey{caseID: "case-1", fromNodeID: "c"}

	c.put(k1, []Path{{CaseID: "case-1"}})
	c.put(k2, []Path{{CaseID: "case-1"}})

	// Touch k1 so it becomes most-recently-used, leaving k2 as the least
	// recently used entry.
	if _, ok := c.get(k1); !ok {
		t.Fatal("expected hit on k1")
	}

	// Inserting k3 should evict k2 (least recently used), not k1.
	c.put(k3, []Path{{CaseID: "case-1"}})

	if _, ok := c.get(k2); ok {
		t.Error("expected k2 to have been evicted")
	}
	if _, ok := c.get(k1); !ok {
		t.Error("expected k1 to still be cached")
	}
	if _, ok := c.get(k3); !ok {
		t.Error("expected k3 to still be cached")
	}
	if got := c.len(); got != 2 {
		t.Errorf("len() = %d, want 2", got)
	}
}

func TestLRUCache_PurgeCase(t *testing.T) {
	c, err := newLRUCache(10)
	if err != nil {
		t.Fatalf("newLRUCache: %v", err)
	}

	c.put(lookupKey{caseID: "case-1", fromNodeID: "a"}, []Path{{CaseID: "case-1"}})
	c.put(lookupKey{caseID: "case-2", fromNodeID: "b"}, []Path{{CaseID: "case-2"}})

	c.purgeCase("case-1")

	if _, ok := c.get(lookupKey{caseID: "case-1", fromNodeID: "a"}); ok {
		t.Error("expected case-1's entry to be purged")
	}
	if _, ok := c.get(lookupKey{caseID: "case-2", fromNodeID: "b"}); !ok {
		t.Error("expected case-2's entry to survive purging case-1")
	}
	if got := c.len(); got != 1 {
		t.Errorf("len() = %d, want 1", got)
	}
}
