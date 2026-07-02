package evidence_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidence"
)

func TestInMemoryClassificationStore_SaveGetRoundTrip(t *testing.T) {
	store := evidence.NewInMemoryClassificationStore()
	ctx := context.Background()

	c := evidence.Classification{SegmentID: "seg-1", Type: evidence.TypeWitnessStatement, Confidence: 0.9}
	if err := store.Save(ctx, c); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := store.Get(ctx, "seg-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != c {
		t.Errorf("Get() = %+v, want %+v", got, c)
	}
}

func TestInMemoryClassificationStore_Get_NotFound(t *testing.T) {
	store := evidence.NewInMemoryClassificationStore()

	_, err := store.Get(context.Background(), "missing")
	if !errors.Is(err, evidence.ErrSegmentNotFound) {
		t.Fatalf("Get() error = %v, want ErrSegmentNotFound", err)
	}
}

func TestInMemoryClassificationStore_Save_EmptySegmentID(t *testing.T) {
	store := evidence.NewInMemoryClassificationStore()

	err := store.Save(context.Background(), evidence.Classification{Type: evidence.TypeOther})
	if !errors.Is(err, evidence.ErrEmptyInput) {
		t.Fatalf("Save() error = %v, want ErrEmptyInput", err)
	}
}

func TestInMemoryClassificationStore_Save_Overwrites(t *testing.T) {
	store := evidence.NewInMemoryClassificationStore()
	ctx := context.Background()

	_ = store.Save(ctx, evidence.Classification{SegmentID: "seg-1", Type: evidence.TypeArgument})
	_ = store.Save(ctx, evidence.Classification{SegmentID: "seg-1", Type: evidence.TypeWitnessStatement})

	got, err := store.Get(ctx, "seg-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Type != evidence.TypeWitnessStatement {
		t.Errorf("Type = %q, want %q (later Save should overwrite)", got.Type, evidence.TypeWitnessStatement)
	}
}

func TestInMemoryClassificationStore_List(t *testing.T) {
	store := evidence.NewInMemoryClassificationStore()
	ctx := context.Background()

	empty, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("List() on empty store = %d records, want 0", len(empty))
	}

	_ = store.Save(ctx, evidence.Classification{SegmentID: "seg-1", Type: evidence.TypeArgument})
	_ = store.Save(ctx, evidence.Classification{SegmentID: "seg-2", Type: evidence.TypeWitnessStatement})

	all, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) != 2 {
		t.Errorf("List() = %d records, want 2", len(all))
	}
}

func TestInMemoryClassificationStore_Delete(t *testing.T) {
	store := evidence.NewInMemoryClassificationStore()
	ctx := context.Background()

	_ = store.Save(ctx, evidence.Classification{SegmentID: "seg-1", Type: evidence.TypeArgument})

	if err := store.Delete(ctx, "seg-1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := store.Get(ctx, "seg-1")
	if !errors.Is(err, evidence.ErrSegmentNotFound) {
		t.Fatalf("Get() after Delete() error = %v, want ErrSegmentNotFound", err)
	}

	if err := store.Delete(ctx, "seg-1"); !errors.Is(err, evidence.ErrSegmentNotFound) {
		t.Fatalf("Delete() again error = %v, want ErrSegmentNotFound", err)
	}
}
