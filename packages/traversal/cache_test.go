package traversal_test

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/traversal"
)

// countingStore wraps an InMemoryGraphStore and counts GetNode calls, so
// tests can assert that ExecuteCached serves a second identical call
// entirely from cache without touching the underlying store again.
type countingStore struct {
	*graph.InMemoryGraphStore
	getNodeCalls int
}

func (s *countingStore) GetNode(ctx context.Context, id string) (irac.Node, error) {
	s.getNodeCalls++
	return s.InMemoryGraphStore.GetNode(ctx, id)
}

func newCountingStore() *countingStore {
	return &countingStore{InMemoryGraphStore: graph.NewInMemoryGraphStore()}
}

func TestCache_HitAndMiss(t *testing.T) {
	store := newCountingStore()
	caseID := "case-cache"
	issueID, _, _, _, _ := seedCleanTree(t, store, caseID)

	cache, err := traversal.NewCache(traversal.CacheOptions{})
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}

	walker, err := traversal.NewWalker(store)
	if err != nil {
		t.Fatalf("NewWalker: %v", err)
	}
	walker = walker.WithCache(cache)

	query := traversal.NewQuery(caseID, issueID).ViaGoverningRule()

	first, err := walker.ExecuteCached(context.Background(), query)
	if err != nil {
		t.Fatalf("ExecuteCached (first): %v", err)
	}
	callsAfterFirst := store.getNodeCalls
	if callsAfterFirst == 0 {
		t.Fatalf("expected the first call to touch the store")
	}

	second, err := walker.ExecuteCached(context.Background(), query)
	if err != nil {
		t.Fatalf("ExecuteCached (second): %v", err)
	}
	if store.getNodeCalls != callsAfterFirst {
		t.Errorf("expected the second call to be served from cache without touching the store, but GetNode calls went from %d to %d", callsAfterFirst, store.getNodeCalls)
	}

	if len(first.Paths) != len(second.Paths) {
		t.Errorf("expected cached result to match the original: first=%d paths, second=%d paths", len(first.Paths), len(second.Paths))
	}
	if cache.Len() != 1 {
		t.Errorf("expected exactly 1 cache entry, got %d", cache.Len())
	}
}

func TestCache_RevisionBumpInvalidates(t *testing.T) {
	store := newCountingStore()
	caseID := "case-cache-revision"
	issueID, _, _, _, _ := seedCleanTree(t, store, caseID)

	cache, err := traversal.NewCache(traversal.CacheOptions{})
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	walker, err := traversal.NewWalker(store)
	if err != nil {
		t.Fatalf("NewWalker: %v", err)
	}
	walker = walker.WithCache(cache)

	query := traversal.NewQuery(caseID, issueID).ViaGoverningRule()

	if _, err := walker.ExecuteCached(context.Background(), query); err != nil {
		t.Fatalf("ExecuteCached (first): %v", err)
	}
	callsAfterFirst := store.getNodeCalls

	// Bump the case's revision, simulating a tree change.
	if err := traversal.ReindexOnRevision(cache, irac.NewInitialRevision(caseID, timeNow())); err != nil {
		t.Fatalf("ReindexOnRevision: %v", err)
	}

	if _, err := walker.ExecuteCached(context.Background(), query); err != nil {
		t.Fatalf("ExecuteCached (after revision bump): %v", err)
	}
	if store.getNodeCalls == callsAfterFirst {
		t.Errorf("expected a revision bump to force a fresh traversal, but GetNode calls stayed at %d", store.getNodeCalls)
	}
}

func TestReindexOnRevision_EmptyCaseID(t *testing.T) {
	cache, err := traversal.NewCache(traversal.CacheOptions{})
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	err = traversal.ReindexOnRevision(cache, irac.TreeRevision{})
	if err != traversal.ErrEmptyCaseID {
		t.Fatalf("expected ErrEmptyCaseID, got %v", err)
	}
}

func TestReindexOnRevision_NilCache(t *testing.T) {
	err := traversal.ReindexOnRevision(nil, irac.NewInitialRevision("case-1", timeNow()))
	if err != nil {
		t.Fatalf("expected nil cache to be a no-op, got %v", err)
	}
}

func TestNewCache_NegativeCapacity(t *testing.T) {
	_, err := traversal.NewCache(traversal.CacheOptions{Capacity: -1})
	if err != traversal.ErrInvalidCacheCapacity {
		t.Fatalf("expected ErrInvalidCacheCapacity, got %v", err)
	}
}

func TestWalker_ExecuteCached_NoCacheConfigured(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	caseID := "case-no-cache"
	issueID, _, _, _, _ := seedCleanTree(t, store, caseID)

	walker, err := traversal.NewWalker(store)
	if err != nil {
		t.Fatalf("NewWalker: %v", err)
	}

	query := traversal.NewQuery(caseID, issueID).ViaGoverningRule()
	result, err := walker.ExecuteCached(context.Background(), query)
	if err != nil {
		t.Fatalf("ExecuteCached: %v", err)
	}
	if len(result.Paths) != 1 {
		t.Fatalf("expected 1 path even without a cache configured, got %d", len(result.Paths))
	}
}
