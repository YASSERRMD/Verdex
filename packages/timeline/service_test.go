package timeline

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/segmentation"
)

func TestTimelineService_BuildTimeline(t *testing.T) {
	svc := NewTimelineService()
	ctx := context.Background()

	req := BuildRequest{
		CaseID: "case-1",
		Parties: []Party{
			{ID: "p1", Role: PartyFirst, Name: "Jane Doe"},
			{ID: "p2", Role: PartySecond, Name: "Acme Corp"},
		},
		Segments: []SegmentAttribution{
			{
				Segment: segmentation.Segment{ID: "s1", Text: "On 2024-03-15 the tenant did not pay rent."},
				PartyID: "p2",
				Subject: "rent-payment",
			},
			{
				Segment: segmentation.Segment{ID: "s2", Text: "On 2024-03-15 the tenant paid rent."},
				PartyID: "p1",
				Subject: "rent-payment",
			},
			{
				Segment: segmentation.Segment{ID: "s3", Text: "The parties dispute the lease terms."},
				PartyID: "p1",
				Subject: "lease-terms",
			},
		},
		Relationships: []Relationship{
			{ID: "r1", PartyAID: "p2", PartyBID: "p1", Kind: KindLandlordTenant},
		},
	}

	result, err := svc.BuildTimeline(ctx, req)
	if err != nil {
		t.Fatalf("BuildTimeline() error = %v", err)
	}

	if len(result.Timeline.Events) != 3 {
		t.Fatalf("len(Timeline.Events) = %d, want 3", len(result.Timeline.Events))
	}
	// Both dated events share the same date (stable order); undated last.
	if result.Timeline.Events[0].ID != "case-1-event-0" || result.Timeline.Events[1].ID != "case-1-event-1" {
		t.Errorf("dated events order = [%s, %s], want [case-1-event-0, case-1-event-1]",
			result.Timeline.Events[0].ID, result.Timeline.Events[1].ID)
	}
	if result.Timeline.Events[2].ID != "case-1-event-2" {
		t.Errorf("undated event = %s, want case-1-event-2 (last)", result.Timeline.Events[2].ID)
	}

	if len(result.Graph.Facts) != 3 {
		t.Fatalf("len(Graph.Facts) = %d, want 3", len(result.Graph.Facts))
	}

	if len(result.Graph.Conflicts) != 1 {
		t.Fatalf("len(Graph.Conflicts) = %d, want 1 (contradictory rent-payment facts on same date)", len(result.Graph.Conflicts))
	}

	if len(result.Graph.Relationships) != 1 {
		t.Errorf("len(Graph.Relationships) = %d, want 1", len(result.Graph.Relationships))
	}

	// Persisted graph should be retrievable via the store round-trip.
	got, err := svc.Store.GetGraph(ctx, "case-1")
	if err != nil {
		t.Fatalf("GetGraph() error = %v", err)
	}
	if got.CaseID != "case-1" {
		t.Errorf("persisted CaseID = %q, want case-1", got.CaseID)
	}
}

func TestTimelineService_BuildTimeline_WithClaims(t *testing.T) {
	svc := NewTimelineService()
	ctx := context.Background()

	req := BuildRequest{
		CaseID: "case-2",
		Parties: []Party{
			{ID: "p1", Role: PartyFirst, Name: "Jane Doe"},
		},
		Segments: []SegmentAttribution{
			{
				Segment: segmentation.Segment{ID: "s1", Text: "The landlord issued a notice to quit."},
				PartyID: "p1",
				Subject: "notice",
			},
		},
		Claims: []Claim{
			{ID: "c1", PartyID: "p1", Description: "landlord acted improperly", FactIDs: []string{"case-2-fact-0"}, EventIDs: []string{"case-2-event-0"}},
		},
	}

	result, err := svc.BuildTimeline(ctx, req)
	if err != nil {
		t.Fatalf("BuildTimeline() error = %v", err)
	}
	if len(result.Graph.Claims) != 1 {
		t.Fatalf("len(Graph.Claims) = %d, want 1", len(result.Graph.Claims))
	}
}

func TestTimelineService_BuildTimeline_Errors(t *testing.T) {
	tests := []struct {
		name    string
		req     BuildRequest
		wantErr error
	}{
		{
			name:    "empty case ID",
			req:     BuildRequest{CaseID: ""},
			wantErr: ErrEmptyInput,
		},
		{
			name: "invalid party",
			req: BuildRequest{
				CaseID:  "case-1",
				Parties: []Party{{ID: "", Role: PartyFirst, Name: "Jane"}},
			},
			wantErr: ErrInvalidParty,
		},
		{
			name: "claim references unknown event",
			req: BuildRequest{
				CaseID: "case-1",
				Claims: []Claim{
					{ID: "c1", PartyID: "p1", Description: "x", EventIDs: []string{"nonexistent"}},
				},
			},
			wantErr: ErrEventNotFound,
		},
		{
			name: "invalid relationship",
			req: BuildRequest{
				CaseID:        "case-1",
				Relationships: []Relationship{{ID: "r1", PartyAID: "p1", PartyBID: "p1", Kind: "x"}},
			},
			wantErr: ErrInvalidParty,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewTimelineService()
			_, err := svc.BuildTimeline(context.Background(), tt.req)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("BuildTimeline() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestTimelineService_BuildTimeline_DeterministicIDs(t *testing.T) {
	svc := NewTimelineService()
	ctx := context.Background()

	req := BuildRequest{
		CaseID: "case-3",
		Segments: []SegmentAttribution{
			{Segment: segmentation.Segment{ID: "s1", Text: "First event happened."}},
			{Segment: segmentation.Segment{ID: "s2", Text: "Second event happened."}},
		},
	}

	r1, err := svc.BuildTimeline(ctx, req)
	if err != nil {
		t.Fatalf("BuildTimeline() error = %v", err)
	}

	svc2 := NewTimelineService()
	r2, err := svc2.BuildTimeline(ctx, req)
	if err != nil {
		t.Fatalf("BuildTimeline() (2nd) error = %v", err)
	}

	if len(r1.Timeline.Events) != len(r2.Timeline.Events) {
		t.Fatalf("event count mismatch: %d vs %d", len(r1.Timeline.Events), len(r2.Timeline.Events))
	}
	for i := range r1.Timeline.Events {
		if r1.Timeline.Events[i].ID != r2.Timeline.Events[i].ID {
			t.Errorf("Events[%d].ID = %q vs %q, want deterministic IDs", i, r1.Timeline.Events[i].ID, r2.Timeline.Events[i].ID)
		}
	}
}
