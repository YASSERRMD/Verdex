package bulkimport_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/bulkimport"
)

// TestEngine_RunBatch_ResumesFromMidJobCursor simulates a crash after
// the first batch and proves a fresh RunBatch call against the same
// persisted ImportJob resumes from its checkpointed Cursor rather than
// restarting from zero or reprocessing already-imported records (task
// 4's explicit resumability requirement).
func TestEngine_RunBatch_ResumesFromMidJobCursor(t *testing.T) {
	t.Parallel()
	jobs := bulkimport.NewInMemoryJobRepository()
	recordRepo := bulkimport.NewInMemoryRecordRepository()
	engine, err := bulkimport.NewEngine(jobs, recordRepo, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	tenantID := uuid.New()
	admin := adminUser(tenantID)

	allRecords := sampleSourceRecords(10)
	source := bulkimport.NewInMemoryRecordSource(allRecords)

	job, err := engine.RegisterJob(ctxWithUser(admin), tenantID, "resumable corpus", len(allRecords))
	if err != nil {
		t.Fatalf("RegisterJob: %v", err)
	}

	// Process the first 6 records, then simulate a crash: construct a
	// brand-new Engine instance sharing only the persisted
	// repositories, with no in-process state carried over.
	if _, err := engine.RunBatch(ctxWithUser(admin), tenantID, job.ID, source, 6); err != nil {
		t.Fatalf("RunBatch (pre-crash): %v", err)
	}

	resumedEngine, err := bulkimport.NewEngine(jobs, recordRepo, nil)
	if err != nil {
		t.Fatalf("NewEngine (resumed): %v", err)
	}

	midJob, err := resumedEngine.GetJob(ctxWithUser(admin), tenantID, job.ID)
	if err != nil {
		t.Fatalf("GetJob (resumed): %v", err)
	}
	if midJob.Cursor != 6 {
		t.Fatalf("resumed job Cursor = %d, want 6 (checkpoint from before the simulated crash)", midJob.Cursor)
	}

	// A fresh RecordSource instance simulates re-opening the same
	// underlying corpus after a restart; ReadAt must still be
	// deterministic by index for resume correctness.
	resumedSource := bulkimport.NewInMemoryRecordSource(allRecords)
	result, err := resumedEngine.RunBatch(ctxWithUser(admin), tenantID, job.ID, resumedSource, 100)
	if err != nil {
		t.Fatalf("RunBatch (resumed): %v", err)
	}
	if !result.Done {
		t.Fatal("resumed RunBatch did not report Done, want true")
	}
	if result.RecordsRead != 4 {
		t.Fatalf("resumed RunBatch RecordsRead = %d, want 4 (only the remaining records)", result.RecordsRead)
	}
	if result.Job.Cursor != 10 {
		t.Fatalf("final job Cursor = %d, want 10", result.Job.Cursor)
	}
	if result.Job.ImportedRecords != 10 {
		t.Fatalf("final job ImportedRecords = %d, want 10 (6 pre-crash + 4 resumed, no double-counting)", result.Job.ImportedRecords)
	}

	recs, err := resumedEngine.ListRecords(ctxWithUser(admin), tenantID, job.ID)
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if len(recs) != 10 {
		t.Fatalf("total records after resume = %d, want exactly 10 (no duplicated processing of the first 6)", len(recs))
	}
}

func TestEngine_PauseAndResumeJob(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	job, err := engine.RegisterJob(ctxWithUser(admin), tenantID, "pausable corpus", 10)
	if err != nil {
		t.Fatalf("RegisterJob: %v", err)
	}
	source := bulkimport.NewInMemoryRecordSource(sampleSourceRecords(10))

	if _, err := engine.RunBatch(ctxWithUser(admin), tenantID, job.ID, source, 4); err != nil {
		t.Fatalf("RunBatch: %v", err)
	}

	paused, err := engine.Pause(ctxWithUser(admin), tenantID, job.ID)
	if err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if paused.Status != bulkimport.StatusPaused {
		t.Fatalf("paused.Status = %v, want StatusPaused", paused.Status)
	}
	if paused.Cursor != 4 {
		t.Fatalf("paused.Cursor = %d, want 4 (preserved across pause)", paused.Cursor)
	}

	// RunBatch on a Paused job should fail.
	if _, err := engine.RunBatch(ctxWithUser(admin), tenantID, job.ID, source, 4); !errors.Is(err, bulkimport.ErrJobNotRunnable) {
		t.Fatalf("RunBatch on paused job error = %v, want ErrJobNotRunnable", err)
	}

	resumed, err := engine.ResumeJob(ctxWithUser(admin), tenantID, job.ID)
	if err != nil {
		t.Fatalf("ResumeJob: %v", err)
	}
	if resumed.Status != bulkimport.StatusRunning {
		t.Fatalf("resumed.Status = %v, want StatusRunning", resumed.Status)
	}

	result, err := engine.RunBatch(ctxWithUser(admin), tenantID, job.ID, source, 100)
	if err != nil {
		t.Fatalf("RunBatch after resume: %v", err)
	}
	if !result.Done || result.Job.Cursor != 10 {
		t.Fatalf("RunBatch after resume = %+v, want Done=true Cursor=10", result)
	}
}
