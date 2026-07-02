package ingestion

import (
	"errors"
	"testing"
)

func TestInMemoryStatusStore_PutAndGet(t *testing.T) {
	store := NewInMemoryStatusStore()
	if _, ok := store.Get("job-1"); ok {
		t.Fatal("expected no state before Put")
	}

	store.Put(WorkflowState{JobID: "job-1", CaseID: "case-1", Stage: StageSegment})
	got, ok := store.Get("job-1")
	if !ok {
		t.Fatal("expected state after Put")
	}
	if got.Stage != StageSegment {
		t.Errorf("Stage = %s, want %s", got.Stage, StageSegment)
	}
}

func TestInMemoryStatusStore_ListByCase(t *testing.T) {
	store := NewInMemoryStatusStore()
	store.Put(WorkflowState{JobID: "job-1", CaseID: "case-A", Stage: StageIntake})
	store.Put(WorkflowState{JobID: "job-2", CaseID: "case-A", Stage: StageComplete})
	store.Put(WorkflowState{JobID: "job-3", CaseID: "case-B", Stage: StageFailed})

	got := store.ListByCase("case-A")
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}

	got = store.ListByCase("case-unknown")
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestIngestionStatusAPI_GetStatus_NotFound(t *testing.T) {
	api := NewIngestionStatusAPI(NewInMemoryStatusStore())
	_, err := api.GetStatus("unknown")
	if !errors.Is(err, ErrJobNotFound) {
		t.Errorf("err = %v, want ErrJobNotFound", err)
	}
}

func TestIngestionStatusAPI_GetStatus_Found(t *testing.T) {
	store := NewInMemoryStatusStore()
	store.Put(WorkflowState{JobID: "job-1", CaseID: "case-1", Stage: StageClassify})

	api := NewIngestionStatusAPI(store)
	got, err := api.GetStatus("job-1")
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if got.Stage != StageClassify {
		t.Errorf("Stage = %s, want %s", got.Stage, StageClassify)
	}
}

func TestIngestionStatusAPI_GetCaseStatus(t *testing.T) {
	store := NewInMemoryStatusStore()
	store.Put(WorkflowState{JobID: "job-1", CaseID: "case-1", Stage: StageIntake})
	store.Put(WorkflowState{JobID: "job-2", CaseID: "case-1", Stage: StageComplete})

	api := NewIngestionStatusAPI(store)
	got := api.GetCaseStatus("case-1")
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}

func TestNewIngestionStatusAPI_NilStore(t *testing.T) {
	api := NewIngestionStatusAPI(nil)
	_, err := api.GetStatus("job-1")
	if !errors.Is(err, ErrJobNotFound) {
		t.Errorf("err = %v, want ErrJobNotFound", err)
	}
}
