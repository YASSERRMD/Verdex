package segmentation_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/segmentation"
)

func TestAssignOrder(t *testing.T) {
	segs := []segmentation.Segment{
		{ID: "a", Text: "first"},
		{ID: "b", Text: "second"},
		{ID: "c", Text: "third"},
	}

	got := segmentation.AssignOrder(segs)

	wantSequence := []int{0, 1, 2}
	wantPrev := []string{"", "a", "b"}
	wantNext := []string{"b", "c", ""}

	for i, s := range got {
		if s.Sequence != wantSequence[i] {
			t.Errorf("segment[%d].Sequence = %d, want %d", i, s.Sequence, wantSequence[i])
		}
		if s.PrevID != wantPrev[i] {
			t.Errorf("segment[%d].PrevID = %q, want %q", i, s.PrevID, wantPrev[i])
		}
		if s.NextID != wantNext[i] {
			t.Errorf("segment[%d].NextID = %q, want %q", i, s.NextID, wantNext[i])
		}
	}

	// Sequence must be strictly increasing.
	for i := 1; i < len(got); i++ {
		if got[i].Sequence <= got[i-1].Sequence {
			t.Errorf("Sequence not strictly increasing at index %d: %d <= %d", i, got[i].Sequence, got[i-1].Sequence)
		}
	}
}

func TestAssignOrder_SingleSegment(t *testing.T) {
	segs := []segmentation.Segment{{ID: "only", Text: "solo"}}
	got := segmentation.AssignOrder(segs)
	if got[0].Sequence != 0 || got[0].PrevID != "" || got[0].NextID != "" {
		t.Errorf("single segment order = %+v, want Sequence=0, PrevID=\"\", NextID=\"\"", got[0])
	}
}

func TestAssignOrder_Empty(t *testing.T) {
	got := segmentation.AssignOrder(nil)
	if len(got) != 0 {
		t.Errorf("AssignOrder(nil) = %d segments, want 0", len(got))
	}
}

func TestAssignOrder_DoesNotMutateInput(t *testing.T) {
	segs := []segmentation.Segment{
		{ID: "a", Text: "first"},
		{ID: "b", Text: "second"},
	}
	_ = segmentation.AssignOrder(segs)

	if segs[0].NextID != "" {
		t.Errorf("input segment[0].NextID mutated to %q", segs[0].NextID)
	}
	if segs[1].PrevID != "" {
		t.Errorf("input segment[1].PrevID mutated to %q", segs[1].PrevID)
	}
}

func TestValidateOrder(t *testing.T) {
	tests := []struct {
		name    string
		segs    []segmentation.Segment
		wantErr bool
	}{
		{
			name: "valid-order",
			segs: []segmentation.Segment{
				{ID: "a", Sequence: 0, NextID: "b"},
				{ID: "b", Sequence: 1, PrevID: "a", NextID: "c"},
				{ID: "c", Sequence: 2, PrevID: "b"},
			},
			wantErr: false,
		},
		{
			name: "gap-in-sequence",
			segs: []segmentation.Segment{
				{ID: "a", Sequence: 0},
				{ID: "b", Sequence: 2},
			},
			wantErr: true,
		},
		{
			name: "broken-prev-link",
			segs: []segmentation.Segment{
				{ID: "a", Sequence: 0, NextID: "b"},
				{ID: "b", Sequence: 1, PrevID: "wrong"},
			},
			wantErr: true,
		},
		{
			name:    "empty",
			segs:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := segmentation.ValidateOrder(tt.segs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOrder() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAssignOrder_ThenValidateOrder(t *testing.T) {
	segs := make([]segmentation.Segment, 5)
	for i := range segs {
		segs[i] = segmentation.Segment{ID: string(rune('a' + i)), Text: "segment"}
	}
	ordered := segmentation.AssignOrder(segs)
	if err := segmentation.ValidateOrder(ordered); err != nil {
		t.Errorf("ValidateOrder(AssignOrder(segs)) = %v, want nil", err)
	}
}
