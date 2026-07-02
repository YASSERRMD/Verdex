package ingestion

import (
	"errors"
	"testing"
	"time"
)

func TestInMemoryRecoveryStore_CheckpointAndLoad(t *testing.T) {
	store := NewInMemoryRecoveryStore()

	if _, ok := store.Load("job-1"); ok {
		t.Fatal("expected no checkpoint before Checkpoint")
	}

	state := WorkflowState{JobID: "job-1", CaseID: "case-1", Stage: StageNormalize, UpdatedAt: time.Now()}
	store.Checkpoint(state)

	got, ok := store.Load("job-1")
	if !ok {
		t.Fatal("expected checkpoint after Checkpoint")
	}
	if got.Stage != StageNormalize {
		t.Errorf("Stage = %s, want %s", got.Stage, StageNormalize)
	}
}

func TestInMemoryRecoveryStore_CheckpointOverwrites(t *testing.T) {
	store := NewInMemoryRecoveryStore()
	store.Checkpoint(WorkflowState{JobID: "job-1", Stage: StageIntake})
	store.Checkpoint(WorkflowState{JobID: "job-1", Stage: StageSegment})

	got, _ := store.Load("job-1")
	if got.Stage != StageSegment {
		t.Errorf("Stage = %s, want %s", got.Stage, StageSegment)
	}
}

func TestInMemoryRecoveryStore_Delete(t *testing.T) {
	store := NewInMemoryRecoveryStore()
	store.Checkpoint(WorkflowState{JobID: "job-1", Stage: StageComplete})
	store.Delete("job-1")

	if _, ok := store.Load("job-1"); ok {
		t.Error("expected no checkpoint after Delete")
	}
}

func TestInMemoryRecoveryStore_LoadReturnsIndependentCopy(t *testing.T) {
	store := NewInMemoryRecoveryStore()
	store.Checkpoint(WorkflowState{
		JobID: "job-1",
		Segments: []ClassifiedSegment{
			{},
		},
	})

	got, _ := store.Load("job-1")
	got.Segments[0].Classification.SegmentID = "mutated"

	got2, _ := store.Load("job-1")
	if got2.Segments[0].Classification.SegmentID == "mutated" {
		t.Error("Load did not return an independent copy")
	}
}

func TestPlanResume_NoCheckpoint(t *testing.T) {
	store := NewInMemoryRecoveryStore()
	_, err := PlanResume(store, "unknown-job")
	if !errors.Is(err, ErrNotResumable) {
		t.Errorf("err = %v, want ErrNotResumable", err)
	}
}

func TestPlanResume_NilStore(t *testing.T) {
	_, err := PlanResume(nil, "job-1")
	if !errors.Is(err, ErrNotResumable) {
		t.Errorf("err = %v, want ErrNotResumable", err)
	}
}

func TestPlanResume_CompleteIsNotResumable(t *testing.T) {
	store := NewInMemoryRecoveryStore()
	store.Checkpoint(WorkflowState{JobID: "job-1", Stage: StageComplete})

	_, err := PlanResume(store, "job-1")
	if !errors.Is(err, ErrNotResumable) {
		t.Errorf("err = %v, want ErrNotResumable", err)
	}
}

func TestPlanResume_FailedStageResumesFromThatStage(t *testing.T) {
	store := NewInMemoryRecoveryStore()
	store.Checkpoint(WorkflowState{JobID: "job-1", Stage: StageSegment, FailureReason: "boom"})

	plan, err := PlanResume(store, "job-1")
	if err != nil {
		t.Fatalf("PlanResume: %v", err)
	}
	if plan.FromStage != StageSegment {
		t.Errorf("FromStage = %s, want %s", plan.FromStage, StageSegment)
	}
	if plan.JobID != "job-1" {
		t.Errorf("JobID = %q, want %q", plan.JobID, "job-1")
	}
}
