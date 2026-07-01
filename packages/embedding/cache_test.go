package embedding_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/embedding"
)

func TestInMemoryCache_SetGet(t *testing.T) {
	ctx := context.Background()
	c := embedding.NewInMemoryCache()

	e := embedding.EmbeddedText{
		ContentHash: "hash1",
		Text:        "hello world",
		Vector:      embedding.EmbeddingVector{0.1, 0.2, 0.3},
		Dimensions:  3,
		ModelID:     "m1",
		ProviderID:  "p1",
		Version:     1,
	}

	if err := c.Set(ctx, e.ContentHash, e); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := c.Get(ctx, e.ContentHash)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Text != e.Text {
		t.Errorf("Text: want %q, got %q", e.Text, got.Text)
	}
	if got.ModelID != e.ModelID {
		t.Errorf("ModelID: want %q, got %q", e.ModelID, got.ModelID)
	}
	if len(got.Vector) != len(e.Vector) {
		t.Errorf("Vector length: want %d, got %d", len(e.Vector), len(got.Vector))
	}
}

func TestInMemoryCache_GetMissingReturnsErrCacheMiss(t *testing.T) {
	ctx := context.Background()
	c := embedding.NewInMemoryCache()

	_, err := c.Get(ctx, "nonexistent")
	if !errors.Is(err, embedding.ErrCacheMiss) {
		t.Errorf("expected ErrCacheMiss, got %v", err)
	}
}

func TestInMemoryCache_Delete(t *testing.T) {
	ctx := context.Background()
	c := embedding.NewInMemoryCache()

	e := embedding.EmbeddedText{ContentHash: "h2", Text: "test", Dimensions: 0}
	_ = c.Set(ctx, "h2", e)

	if err := c.Delete(ctx, "h2"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := c.Get(ctx, "h2")
	if !errors.Is(err, embedding.ErrCacheMiss) {
		t.Errorf("expected ErrCacheMiss after delete, got %v", err)
	}
}

func TestInMemoryCache_DeleteNonExistentIsNoOp(t *testing.T) {
	ctx := context.Background()
	c := embedding.NewInMemoryCache()

	// Should not panic or return an error.
	if err := c.Delete(ctx, "ghost"); err != nil {
		t.Errorf("expected nil from Delete on absent key, got %v", err)
	}
}

func TestInMemoryCache_Flush(t *testing.T) {
	ctx := context.Background()
	c := embedding.NewInMemoryCache()

	for i := 0; i < 5; i++ {
		e := embedding.EmbeddedText{ContentHash: string(rune('a' + i)), Text: "x"}
		_ = c.Set(ctx, e.ContentHash, e)
	}
	if c.Len() != 5 {
		t.Fatalf("expected 5 entries before flush, got %d", c.Len())
	}

	if err := c.Flush(ctx); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if c.Len() != 0 {
		t.Errorf("expected 0 entries after flush, got %d", c.Len())
	}
}

func TestInMemoryCache_SetOverwrites(t *testing.T) {
	ctx := context.Background()
	c := embedding.NewInMemoryCache()

	e1 := embedding.EmbeddedText{ContentHash: "h", Text: "original", Version: 1}
	e2 := embedding.EmbeddedText{ContentHash: "h", Text: "updated", Version: 2}

	_ = c.Set(ctx, "h", e1)
	_ = c.Set(ctx, "h", e2)

	got, err := c.Get(ctx, "h")
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 2 {
		t.Errorf("expected Version 2 after overwrite, got %d", got.Version)
	}
}

func TestCacheKey_Deterministic(t *testing.T) {
	k1 := embedding.CacheKey("same text", "model-a")
	k2 := embedding.CacheKey("same text", "model-a")
	if k1 != k2 {
		t.Errorf("CacheKey not deterministic: %q vs %q", k1, k2)
	}
}

func TestCacheKey_DifferentForDifferentInputs(t *testing.T) {
	k1 := embedding.CacheKey("text a", "model-a")
	k2 := embedding.CacheKey("text a", "model-b")
	k3 := embedding.CacheKey("text b", "model-a")

	if k1 == k2 {
		t.Error("CacheKey collision: same text, different model")
	}
	if k1 == k3 {
		t.Error("CacheKey collision: different text, same model")
	}
}
