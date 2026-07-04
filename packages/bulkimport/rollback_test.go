package bulkimport_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/bulkimport"
)

// TestEngine_Rollback_ReversesImportedRecords proves Rollback performs
// a real state transition rather than a no-op (task 7): every
// Imported record moves to RolledBack, the job's ImportedRecords count
// decrements accordingly, and the job itself moves to
// StatusRolledBack.
func TestEngine_Rollback_ReversesImportedRecords(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	job, err := engine.RegisterJob(ctxWithUser(admin), tenantID, "corpus to roll back", 5)
	if err != nil {
		t.Fatalf("RegisterJob: %v", err)
	}
	source := bulkimport.NewInMemoryRecordSource(sampleSourceRecords(5))

	result, err := engine.RunBatch(ctxWithUser(admin), tenantID, job.ID, source, 100)
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}
	if result.Job.Status != bulkimport.StatusCompleted {
		t.Fatalf("job.Status before rollback = %v, want StatusCompleted", result.Job.Status)
	}
	if result.Job.ImportedRecords != 5 {
		t.Fatalf("job.ImportedRecords before rollback = %d, want 5", result.Job.ImportedRecords)
	}

	rolledBack, err := engine.Rollback(ctxWithUser(admin), tenantID, job.ID)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if rolledBack.Status != bulkimport.StatusRolledBack {
		t.Fatalf("job.Status after rollback = %v, want StatusRolledBack", rolledBack.Status)
	}
	if rolledBack.ImportedRecords != 0 {
		t.Fatalf("job.ImportedRecords after rollback = %d, want 0", rolledBack.ImportedRecords)
	}

	recs, err := engine.ListRecords(ctxWithUser(admin), tenantID, job.ID)
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	for _, r := range recs {
		if r.Outcome != bulkimport.OutcomeRolledBack {
			t.Fatalf("record %s Outcome = %v, want OutcomeRolledBack", r.ID, r.Outcome)
		}
		if r.CreatedCaseID != uuid.Nil {
			t.Fatalf("record %s CreatedCaseID = %v, want uuid.Nil after rollback", r.ID, r.CreatedCaseID)
		}
	}
}

func TestEngine_Rollback_RequiresCompletedOrFailed(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	job, err := engine.RegisterJob(ctxWithUser(admin), tenantID, "still pending", 5)
	if err != nil {
		t.Fatalf("RegisterJob: %v", err)
	}

	if _, err := engine.Rollback(ctxWithUser(admin), tenantID, job.ID); !errors.Is(err, bulkimport.ErrJobNotRollbackable) {
		t.Fatalf("Rollback on pending job error = %v, want ErrJobNotRollbackable", err)
	}
}

func TestEngine_Rollback_FromFailedJob(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	job, err := engine.RegisterJob(ctxWithUser(admin), tenantID, "corpus that failed", 5)
	if err != nil {
		t.Fatalf("RegisterJob: %v", err)
	}
	source := bulkimport.NewInMemoryRecordSource(sampleSourceRecords(5))
	if _, err := engine.RunBatch(ctxWithUser(admin), tenantID, job.ID, source, 3); err != nil {
		t.Fatalf("RunBatch: %v", err)
	}

	failed, err := engine.FailJob(ctxWithUser(admin), tenantID, job.ID, "operator aborted")
	if err != nil {
		t.Fatalf("FailJob: %v", err)
	}
	if failed.Status != bulkimport.StatusFailed {
		t.Fatalf("job.Status = %v, want StatusFailed", failed.Status)
	}

	rolledBack, err := engine.Rollback(ctxWithUser(admin), tenantID, job.ID)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if rolledBack.Status != bulkimport.StatusRolledBack {
		t.Fatalf("job.Status after rollback = %v, want StatusRolledBack", rolledBack.Status)
	}
	if rolledBack.ImportedRecords != 0 {
		t.Fatalf("job.ImportedRecords after rollback = %d, want 0", rolledBack.ImportedRecords)
	}
}
