package ingestion

import (
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/segmentation"
)

func TestStage_IndexAndTerminal(t *testing.T) {
	tests := []struct {
		name      string
		stage     Stage
		wantIndex int
		wantTerm  bool
	}{
		{"intake", StageIntake, 0, false},
		{"extraction", StageExtraction, 1, false},
		{"normalize", StageNormalize, 2, false},
		{"segment", StageSegment, 3, false},
		{"classify", StageClassify, 4, false},
		{"complete", StageComplete, 5, true},
		{"failed", StageFailed, -1, true},
		{"unknown", Stage("bogus"), -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.stage.Index(); got != tt.wantIndex {
				t.Errorf("Index() = %d, want %d", got, tt.wantIndex)
			}
			if got := tt.stage.IsTerminal(); got != tt.wantTerm {
				t.Errorf("IsTerminal() = %v, want %v", got, tt.wantTerm)
			}
		})
	}
}

func TestNextStage(t *testing.T) {
	tests := []struct {
		name     string
		from     Stage
		wantNext Stage
		wantOK   bool
	}{
		{"intake to extraction", StageIntake, StageExtraction, true},
		{"extraction to normalize", StageExtraction, StageNormalize, true},
		{"normalize to segment", StageNormalize, StageSegment, true},
		{"segment to classify", StageSegment, StageClassify, true},
		{"classify to complete", StageClassify, StageComplete, true},
		{"complete has no next", StageComplete, StageFailed, false},
		{"failed has no next", StageFailed, StageFailed, false},
		{"unknown has no next", Stage("bogus"), StageFailed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next, ok := NextStage(tt.from)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && next != tt.wantNext {
				t.Errorf("next = %s, want %s", next, tt.wantNext)
			}
		})
	}
}

func TestWorkflowState_Advance(t *testing.T) {
	now := time.Now().UTC()
	state := WorkflowState{
		JobID:            "job-1",
		CaseID:           "case-1",
		Stage:            StageIntake,
		AttemptsForStage: 2,
		FailureReason:    "boom",
	}

	next := state.Advance(StageExtraction, now)

	if next.Stage != StageExtraction {
		t.Errorf("Stage = %s, want %s", next.Stage, StageExtraction)
	}
	if next.AttemptsForStage != 0 {
		t.Errorf("AttemptsForStage = %d, want 0", next.AttemptsForStage)
	}
	if next.FailureReason != "" {
		t.Errorf("FailureReason = %q, want empty", next.FailureReason)
	}
	if !next.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", next.UpdatedAt, now)
	}
	// original state must be unmutated (value receiver).
	if state.Stage != StageIntake {
		t.Errorf("original state.Stage mutated: %s", state.Stage)
	}
}

func TestWorkflowState_Advance_ToFailedPreservesReason(t *testing.T) {
	state := WorkflowState{Stage: StageClassify, FailureReason: "transient"}
	next := state.Advance(StageFailed, time.Now())
	if next.FailureReason != "transient" {
		t.Errorf("FailureReason = %q, want %q", next.FailureReason, "transient")
	}
}

func TestWorkflowState_Clone(t *testing.T) {
	state := WorkflowState{
		JobID: "job-1",
		Segments: []ClassifiedSegment{
			{Segment: segmentation.Segment{ID: "seg-1", Text: "hello"}},
		},
	}

	clone := state.Clone()
	clone.Segments[0].Segment.Text = "mutated"

	if state.Segments[0].Segment.Text != "hello" {
		t.Errorf("Clone did not deep-copy Segments: original = %q", state.Segments[0].Segment.Text)
	}
}
