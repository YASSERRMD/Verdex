package bulkimport_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/bulkimport"
)

func TestEngine_RegisterJob(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	job, err := engine.RegisterJob(ctxWithUser(admin), tenantID, "2019-2022 archive export", 10)
	if err != nil {
		t.Fatalf("RegisterJob: %v", err)
	}
	if job.Status != bulkimport.StatusPending {
		t.Fatalf("new job Status = %v, want StatusPending", job.Status)
	}
	if job.TotalRecords != 10 {
		t.Fatalf("job.TotalRecords = %d, want 10", job.TotalRecords)
	}
	if job.ID == uuid.Nil {
		t.Fatal("job.ID is nil")
	}

	got, err := engine.GetJob(ctxWithUser(admin), tenantID, job.ID)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got.ID != job.ID {
		t.Fatalf("GetJob returned wrong job: %v", got)
	}
}

func TestEngine_RegisterJob_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	auditor := auditorUser(tenantID)

	_, err := engine.RegisterJob(ctxWithUser(auditor), tenantID, "some corpus", 1)
	if !errors.Is(err, bulkimport.ErrForbidden) {
		t.Fatalf("RegisterJob() with view-only user error = %v, want ErrForbidden", err)
	}
}

func TestEngine_RegisterJob_Unauthenticated(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	_, err := engine.RegisterJob(t.Context(), tenantID, "some corpus", 1)
	if !errors.Is(err, bulkimport.ErrUnauthenticated) {
		t.Fatalf("RegisterJob() unauthenticated error = %v, want ErrUnauthenticated", err)
	}
}

func TestEngine_RegisterJob_BlankSourceDescriptionRejected(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	_, err := engine.RegisterJob(ctxWithUser(admin), tenantID, "   ", 1)
	if !errors.Is(err, bulkimport.ErrInvalidJob) {
		t.Fatalf("RegisterJob() blank source error = %v, want ErrInvalidJob", err)
	}
}

func TestEngine_RunBatch_ProcessesInChunks(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	records := sampleSourceRecords(10)
	source := bulkimport.NewInMemoryRecordSource(records)

	job, err := engine.RegisterJob(ctxWithUser(admin), tenantID, "chunked corpus", len(records))
	if err != nil {
		t.Fatalf("RegisterJob: %v", err)
	}

	// First batch of 4: should read 4, not be done, Status Running.
	result, err := engine.RunBatch(ctxWithUser(admin), tenantID, job.ID, source, 4)
	if err != nil {
		t.Fatalf("RunBatch #1: %v", err)
	}
	if result.RecordsRead != 4 {
		t.Fatalf("batch #1 RecordsRead = %d, want 4", result.RecordsRead)
	}
	if result.Done {
		t.Fatal("batch #1 reported Done, want false")
	}
	if result.Job.Status != bulkimport.StatusRunning {
		t.Fatalf("batch #1 job Status = %v, want StatusRunning", result.Job.Status)
	}
	if result.Job.Cursor != 4 {
		t.Fatalf("batch #1 job Cursor = %d, want 4", result.Job.Cursor)
	}
	if result.Imported != 4 {
		t.Fatalf("batch #1 Imported = %d, want 4", result.Imported)
	}

	// Second batch of 4: cursor advances to 8.
	result, err = engine.RunBatch(ctxWithUser(admin), tenantID, job.ID, source, 4)
	if err != nil {
		t.Fatalf("RunBatch #2: %v", err)
	}
	if result.Job.Cursor != 8 {
		t.Fatalf("batch #2 job Cursor = %d, want 8", result.Job.Cursor)
	}
	if result.Done {
		t.Fatal("batch #2 reported Done, want false")
	}

	// Third batch of 4: only 2 remain, source reports done.
	result, err = engine.RunBatch(ctxWithUser(admin), tenantID, job.ID, source, 4)
	if err != nil {
		t.Fatalf("RunBatch #3: %v", err)
	}
	if result.RecordsRead != 2 {
		t.Fatalf("batch #3 RecordsRead = %d, want 2", result.RecordsRead)
	}
	if !result.Done {
		t.Fatal("batch #3 reported Done = false, want true")
	}
	if result.Job.Status != bulkimport.StatusCompleted {
		t.Fatalf("batch #3 job Status = %v, want StatusCompleted", result.Job.Status)
	}
	if result.Job.ImportedRecords != 10 {
		t.Fatalf("final job ImportedRecords = %d, want 10", result.Job.ImportedRecords)
	}
	if result.Job.ProcessedRecords != 10 {
		t.Fatalf("final job ProcessedRecords = %d, want 10", result.Job.ProcessedRecords)
	}

	recs, err := engine.ListRecords(ctxWithUser(admin), tenantID, job.ID)
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if len(recs) != 10 {
		t.Fatalf("ListRecords returned %d records, want 10", len(recs))
	}
	for i, r := range recs {
		if r.SourceIndex != i {
			t.Fatalf("record %d has SourceIndex %d, want %d", i, r.SourceIndex, i)
		}
		if r.Outcome != bulkimport.OutcomeImported {
			t.Fatalf("record %d Outcome = %v, want OutcomeImported", i, r.Outcome)
		}
	}
}

func TestEngine_RunBatch_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)
	auditor := auditorUser(tenantID)

	job, err := engine.RegisterJob(ctxWithUser(admin), tenantID, "corpus", 1)
	if err != nil {
		t.Fatalf("RegisterJob: %v", err)
	}

	source := bulkimport.NewInMemoryRecordSource(sampleSourceRecords(1))
	_, err = engine.RunBatch(ctxWithUser(auditor), tenantID, job.ID, source, 4)
	if !errors.Is(err, bulkimport.ErrForbidden) {
		t.Fatalf("RunBatch() with view-only user error = %v, want ErrForbidden", err)
	}
}

func TestEngine_RunBatch_InvalidBatchSizeRejected(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	job, err := engine.RegisterJob(ctxWithUser(admin), tenantID, "corpus", 1)
	if err != nil {
		t.Fatalf("RegisterJob: %v", err)
	}
	source := bulkimport.NewInMemoryRecordSource(sampleSourceRecords(1))

	if _, err := engine.RunBatch(ctxWithUser(admin), tenantID, job.ID, source, 0); !errors.Is(err, bulkimport.ErrInvalidBatchSize) {
		t.Fatalf("RunBatch(batchSize=0) error = %v, want ErrInvalidBatchSize", err)
	}
	if _, err := engine.RunBatch(ctxWithUser(admin), tenantID, job.ID, nil, 4); !errors.Is(err, bulkimport.ErrNilSource) {
		t.Fatalf("RunBatch(nil source) error = %v, want ErrNilSource", err)
	}
}

func TestEngine_RunBatch_RejectsInvalidRecords(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	job, err := engine.RegisterJob(ctxWithUser(admin), tenantID, "corpus with bad rows", 3)
	if err != nil {
		t.Fatalf("RegisterJob: %v", err)
	}

	records := []bulkimport.SourceRecord{
		{PayloadRef: "ref-1", CaseNumber: "CASE-1", Jurisdiction: "dubai-courts"},
		// Missing CaseNumber and Jurisdiction -- should be rejected.
		{PayloadRef: "ref-2"},
		{PayloadRef: "ref-3", CaseNumber: "CASE-3", Jurisdiction: "dubai-courts"},
	}
	source := bulkimport.NewInMemoryRecordSource(records)

	result, err := engine.RunBatch(ctxWithUser(admin), tenantID, job.ID, source, 10)
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}
	if result.Imported != 2 {
		t.Fatalf("Imported = %d, want 2", result.Imported)
	}
	if result.Rejected != 1 {
		t.Fatalf("Rejected = %d, want 1", result.Rejected)
	}
	if result.Job.FailedRecords != 1 {
		t.Fatalf("job.FailedRecords = %d, want 1", result.Job.FailedRecords)
	}

	recs, err := engine.ListRecords(ctxWithUser(admin), tenantID, job.ID)
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	rejected := recs[1]
	if rejected.Outcome != bulkimport.OutcomeRejected {
		t.Fatalf("rejected record Outcome = %v, want OutcomeRejected", rejected.Outcome)
	}
	if len(rejected.ValidationErrors) == 0 {
		t.Fatal("rejected record has no ValidationErrors, want at least one")
	}
	foundCaseNumberErr := false
	for _, ve := range rejected.ValidationErrors {
		if ve.Field == "case_number" {
			foundCaseNumberErr = true
		}
	}
	if !foundCaseNumberErr {
		t.Fatalf("rejected record ValidationErrors = %+v, want a case_number error", rejected.ValidationErrors)
	}
}

func TestEngine_RunBatch_SkipsTrueDuplicates(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	job, err := engine.RegisterJob(ctxWithUser(admin), tenantID, "corpus with dup", 2)
	if err != nil {
		t.Fatalf("RegisterJob: %v", err)
	}

	// Second record duplicates the first's case number/jurisdiction/
	// parties, just with different capitalization/whitespace and
	// party order -- ComputeDedupKey must still treat it as a
	// duplicate.
	records := []bulkimport.SourceRecord{
		{PayloadRef: "ref-1", CaseNumber: "CASE-100", Jurisdiction: "Dubai Courts", PartyNames: []string{"Jane Doe", "Acme LLC"}},
		{PayloadRef: "ref-2", CaseNumber: " case-100 ", Jurisdiction: "dubai courts", PartyNames: []string{"acme llc", "jane doe"}},
	}
	source := bulkimport.NewInMemoryRecordSource(records)

	result, err := engine.RunBatch(ctxWithUser(admin), tenantID, job.ID, source, 10)
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}
	if result.Imported != 1 {
		t.Fatalf("Imported = %d, want 1", result.Imported)
	}
	if result.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1", result.Skipped)
	}
	if result.Job.SkippedRecords != 1 {
		t.Fatalf("job.SkippedRecords = %d, want 1", result.Job.SkippedRecords)
	}

	recs, err := engine.ListRecords(ctxWithUser(admin), tenantID, job.ID)
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if recs[0].Outcome != bulkimport.OutcomeImported {
		t.Fatalf("recs[0].Outcome = %v, want OutcomeImported", recs[0].Outcome)
	}
	if recs[1].Outcome != bulkimport.OutcomeSkippedDuplicate {
		t.Fatalf("recs[1].Outcome = %v, want OutcomeSkippedDuplicate", recs[1].Outcome)
	}
	if recs[0].DedupKey != recs[1].DedupKey {
		t.Fatalf("dedup keys differ: %q vs %q, want equal", recs[0].DedupKey, recs[1].DedupKey)
	}
}

func TestEngine_GetProgress(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	job, err := engine.RegisterJob(ctxWithUser(admin), tenantID, "progress corpus", 10)
	if err != nil {
		t.Fatalf("RegisterJob: %v", err)
	}
	source := bulkimport.NewInMemoryRecordSource(sampleSourceRecords(10))

	if _, err := engine.RunBatch(ctxWithUser(admin), tenantID, job.ID, source, 5); err != nil {
		t.Fatalf("RunBatch: %v", err)
	}

	progress, err := engine.GetProgress(ctxWithUser(admin), tenantID, job.ID)
	if err != nil {
		t.Fatalf("GetProgress: %v", err)
	}
	if progress.Total != 10 {
		t.Fatalf("progress.Total = %d, want 10", progress.Total)
	}
	if progress.Processed != 5 {
		t.Fatalf("progress.Processed = %d, want 5", progress.Processed)
	}
	if progress.PercentComplete != 50 {
		t.Fatalf("progress.PercentComplete = %v, want 50", progress.PercentComplete)
	}
	if progress.Status != bulkimport.StatusRunning {
		t.Fatalf("progress.Status = %v, want StatusRunning", progress.Status)
	}
}

func TestEngine_AuditSink_RecordsBatchRunAndRollback(t *testing.T) {
	t.Parallel()
	engine, auditStore, tenantID := newTestEngineWithAudit(t)
	admin := adminUser(tenantID)

	job, err := engine.RegisterJob(ctxWithUser(admin), tenantID, "audited corpus", 3)
	if err != nil {
		t.Fatalf("RegisterJob: %v", err)
	}
	source := bulkimport.NewInMemoryRecordSource(sampleSourceRecords(3))
	if _, err := engine.RunBatch(ctxWithUser(admin), tenantID, job.ID, source, 100); err != nil {
		t.Fatalf("RunBatch: %v", err)
	}
	if _, err := engine.Rollback(ctxWithUser(admin), tenantID, job.ID); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	events, err := auditStore.Query(ctxWithUser(admin), tenantID, auditlog.Filter{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(events) < 3 {
		t.Fatalf("audit events count = %d, want at least 3 (register + batch run + rollback)", len(events))
	}
}
