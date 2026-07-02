package timeline

import (
	"context"
	"errors"
	"testing"
)

func TestInMemoryTimelineStore_SaveGetRoundTrip(t *testing.T) {
	store := NewInMemoryTimelineStore()
	ctx := context.Background()

	graph := CaseGraph{
		CaseID:  "case-1",
		Parties: []Party{{ID: "p1", Role: PartyFirst, Name: "Jane Doe"}},
		Facts:   []PartyFact{{ID: "f1", PartyID: "p1", SegmentID: "s1", Text: "paid rent"}},
		Events:  []Event{{ID: "e1", Description: "rent paid"}},
		Claims:  []Claim{{ID: "c1", PartyID: "p1", Description: "no breach", FactIDs: []string{"f1"}}},
		Conflicts: []Conflict{
			{ID: "f1|f2", FactAID: "f1", FactBID: "f2", Subject: "rent-payment"},
		},
		Relationships: []Relationship{
			{ID: "r1", PartyAID: "p1", PartyBID: "p2", Kind: KindLandlordTenant},
		},
	}

	if err := store.SaveGraph(ctx, graph); err != nil {
		t.Fatalf("SaveGraph() error = %v", err)
	}

	got, err := store.GetGraph(ctx, "case-1")
	if err != nil {
		t.Fatalf("GetGraph() error = %v", err)
	}

	if got.CaseID != graph.CaseID {
		t.Errorf("CaseID = %q, want %q", got.CaseID, graph.CaseID)
	}
	if len(got.Parties) != 1 || got.Parties[0].ID != "p1" {
		t.Errorf("Parties = %+v, want 1 party p1", got.Parties)
	}
	if len(got.Facts) != 1 {
		t.Errorf("Facts = %+v, want 1 fact", got.Facts)
	}
	if len(got.Events) != 1 {
		t.Errorf("Events = %+v, want 1 event", got.Events)
	}
	if len(got.Claims) != 1 {
		t.Errorf("Claims = %+v, want 1 claim", got.Claims)
	}
	if len(got.Conflicts) != 1 {
		t.Errorf("Conflicts = %+v, want 1 conflict", got.Conflicts)
	}
	if len(got.Relationships) != 1 {
		t.Errorf("Relationships = %+v, want 1 relationship", got.Relationships)
	}
}

func TestInMemoryTimelineStore_GetGraph_NotFound(t *testing.T) {
	store := NewInMemoryTimelineStore()

	_, err := store.GetGraph(context.Background(), "missing")
	if !errors.Is(err, ErrCaseNotFound) {
		t.Fatalf("GetGraph() error = %v, want ErrCaseNotFound", err)
	}
}

func TestInMemoryTimelineStore_SaveGraph_EmptyCaseID(t *testing.T) {
	store := NewInMemoryTimelineStore()

	err := store.SaveGraph(context.Background(), CaseGraph{})
	if !errors.Is(err, ErrEmptyInput) {
		t.Fatalf("SaveGraph() error = %v, want ErrEmptyInput", err)
	}
}

func TestInMemoryTimelineStore_SaveGraph_Overwrites(t *testing.T) {
	store := NewInMemoryTimelineStore()
	ctx := context.Background()

	_ = store.SaveGraph(ctx, CaseGraph{CaseID: "case-1", Parties: []Party{{ID: "p1", Role: PartyFirst, Name: "A"}}})
	_ = store.SaveGraph(ctx, CaseGraph{CaseID: "case-1", Parties: []Party{{ID: "p1", Role: PartyFirst, Name: "A"}, {ID: "p2", Role: PartySecond, Name: "B"}}})

	got, err := store.GetGraph(ctx, "case-1")
	if err != nil {
		t.Fatalf("GetGraph() error = %v", err)
	}
	if len(got.Parties) != 2 {
		t.Errorf("Parties = %+v, want 2 (later SaveGraph should overwrite)", got.Parties)
	}
}

func TestInMemoryTimelineStore_ListCaseIDs(t *testing.T) {
	store := NewInMemoryTimelineStore()
	ctx := context.Background()

	empty, err := store.ListCaseIDs(ctx)
	if err != nil {
		t.Fatalf("ListCaseIDs() error = %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("ListCaseIDs() on empty store = %d, want 0", len(empty))
	}

	_ = store.SaveGraph(ctx, CaseGraph{CaseID: "case-1"})
	_ = store.SaveGraph(ctx, CaseGraph{CaseID: "case-2"})

	all, err := store.ListCaseIDs(ctx)
	if err != nil {
		t.Fatalf("ListCaseIDs() error = %v", err)
	}
	if len(all) != 2 {
		t.Errorf("ListCaseIDs() = %d, want 2", len(all))
	}
}

func TestInMemoryTimelineStore_DeleteGraph(t *testing.T) {
	store := NewInMemoryTimelineStore()
	ctx := context.Background()

	_ = store.SaveGraph(ctx, CaseGraph{CaseID: "case-1"})

	if err := store.DeleteGraph(ctx, "case-1"); err != nil {
		t.Fatalf("DeleteGraph() error = %v", err)
	}

	_, err := store.GetGraph(ctx, "case-1")
	if !errors.Is(err, ErrCaseNotFound) {
		t.Fatalf("GetGraph() after DeleteGraph() error = %v, want ErrCaseNotFound", err)
	}

	if err := store.DeleteGraph(ctx, "case-1"); !errors.Is(err, ErrCaseNotFound) {
		t.Fatalf("DeleteGraph() again error = %v, want ErrCaseNotFound", err)
	}
}
