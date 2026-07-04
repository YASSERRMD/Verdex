package corpusupdater_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/corpusupdater"
)

func TestNewEngine_NilStores(t *testing.T) {
	t.Parallel()
	if _, err := corpusupdater.NewEngine(nil, corpusupdater.NewInMemoryAmendmentRepository(), nil); !errors.Is(err, corpusupdater.ErrNilStore) {
		t.Errorf("NewEngine(nil jobs, ...) = %v, want ErrNilStore", err)
	}
	if _, err := corpusupdater.NewEngine(corpusupdater.NewInMemoryJobRepository(), nil, nil); !errors.Is(err, corpusupdater.ErrNilStore) {
		t.Errorf("NewEngine(..., nil amendments, ...) = %v, want ErrNilStore", err)
	}
}

func TestCreateJob_RequiresAuthentication(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	_, err := engine.CreateJob(context.Background(), tenantID, corpusupdater.CorpusUpdateJob{
		JurisdictionCode: "AE-DXB",
		TargetCorpus:     corpusupdater.CorpusStatute,
	})
	if !errors.Is(err, corpusupdater.ErrUnauthenticated) {
		t.Errorf("CreateJob with no ctx user = %v, want ErrUnauthenticated", err)
	}
}

func TestCreateJob_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(auditorUser(tenantID))

	_, err := engine.CreateJob(ctx, tenantID, corpusupdater.CorpusUpdateJob{
		JurisdictionCode: "AE-DXB",
		TargetCorpus:     corpusupdater.CorpusStatute,
	})
	if !errors.Is(err, corpusupdater.ErrForbidden) {
		t.Errorf("CreateJob as auditor (view-only) = %v, want ErrForbidden", err)
	}
}

func TestCreateJob_RejectsCrossTenantActor(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	otherTenant := uuid.New()
	ctx := ctxWithUser(adminUser(otherTenant))

	_, err := engine.CreateJob(ctx, tenantID, corpusupdater.CorpusUpdateJob{
		JurisdictionCode: "AE-DXB",
		TargetCorpus:     corpusupdater.CorpusStatute,
	})
	if !errors.Is(err, corpusupdater.ErrCrossTenantAccess) {
		t.Errorf("CreateJob from other tenant's admin = %v, want ErrCrossTenantAccess", err)
	}
}

func TestCreateJob_Success(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(adminUser(tenantID))

	job, err := engine.CreateJob(ctx, tenantID, corpusupdater.CorpusUpdateJob{
		JurisdictionCode:  "AE-DXB",
		TargetCorpus:      corpusupdater.CorpusStatute,
		SourceDescription: "Official Gazette Issue 412",
	})
	if err != nil {
		t.Fatalf("CreateJob() = %v, want nil error", err)
	}
	if job.ID == uuid.Nil {
		t.Error("CreateJob() did not assign an ID")
	}
	if job.Status != corpusupdater.StatusPending {
		t.Errorf("CreateJob() Status = %q, want %q", job.Status, corpusupdater.StatusPending)
	}
	if job.TenantID != tenantID {
		t.Errorf("CreateJob() TenantID = %v, want %v", job.TenantID, tenantID)
	}

	fetched, err := engine.GetJob(ctx, tenantID, job.ID)
	if err != nil {
		t.Fatalf("GetJob() = %v, want nil error", err)
	}
	if fetched.ID != job.ID {
		t.Errorf("GetJob() ID = %v, want %v", fetched.ID, job.ID)
	}
}

func TestCreateJob_InvalidJobRejected(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(adminUser(tenantID))

	_, err := engine.CreateJob(ctx, tenantID, corpusupdater.CorpusUpdateJob{
		JurisdictionCode: "",
		TargetCorpus:     corpusupdater.CorpusStatute,
	})
	if !errors.Is(err, corpusupdater.ErrInvalidJob) {
		t.Errorf("CreateJob with blank jurisdiction = %v, want ErrInvalidJob", err)
	}
}

func TestListJobs_ScopesByTenantAndJurisdiction(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(adminUser(tenantID))

	_, err := engine.CreateJob(ctx, tenantID, corpusupdater.CorpusUpdateJob{JurisdictionCode: "AE-DXB", TargetCorpus: corpusupdater.CorpusStatute})
	if err != nil {
		t.Fatalf("CreateJob() = %v", err)
	}
	_, err = engine.CreateJob(ctx, tenantID, corpusupdater.CorpusUpdateJob{JurisdictionCode: "AE-AUH", TargetCorpus: corpusupdater.CorpusPrecedent})
	if err != nil {
		t.Fatalf("CreateJob() = %v", err)
	}

	all, err := engine.ListJobs(ctx, tenantID, "")
	if err != nil {
		t.Fatalf("ListJobs() = %v", err)
	}
	if len(all) != 2 {
		t.Errorf("ListJobs(\"\") returned %d jobs, want 2", len(all))
	}

	scoped, err := engine.ListJobs(ctx, tenantID, "AE-DXB")
	if err != nil {
		t.Fatalf("ListJobs() = %v", err)
	}
	if len(scoped) != 1 {
		t.Errorf("ListJobs(\"AE-DXB\") returned %d jobs, want 1", len(scoped))
	}
}

// stageJobWithAmendment is a small test helper creating a job in
// StatusPending and staging one amendment onto it, returning both.
func stageJobWithAmendment(t *testing.T, engine *corpusupdater.Engine, ctx context.Context, tenantID uuid.UUID, amendment corpusupdater.Amendment) (corpusupdater.CorpusUpdateJob, corpusupdater.Amendment) {
	t.Helper()
	job, err := engine.CreateJob(ctx, tenantID, corpusupdater.CorpusUpdateJob{
		JurisdictionCode: "AE-DXB",
		TargetCorpus:     corpusupdater.CorpusStatute,
	})
	if err != nil {
		t.Fatalf("CreateJob() = %v", err)
	}
	staged, err := engine.StageAmendment(ctx, tenantID, job.ID, amendment)
	if err != nil {
		t.Fatalf("StageAmendment() = %v", err)
	}
	return job, staged
}

func TestStageAmendment_Success(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(adminUser(tenantID))

	job, amendment := stageJobWithAmendment(t, engine, ctx, tenantID, corpusupdater.Amendment{
		TargetID:      "rule-1",
		ChangeType:    corpusupdater.ChangeTypeAmend,
		NewText:       "Updated text.",
		Citation:      "Federal Decree-Law No. 45 of 2023",
		EffectiveDate: time.Now(),
	})

	if amendment.JobID != job.ID {
		t.Errorf("StageAmendment() JobID = %v, want %v", amendment.JobID, job.ID)
	}
	if amendment.TargetCorpus != corpusupdater.CorpusStatute {
		t.Errorf("StageAmendment() TargetCorpus = %q, want inherited from job", amendment.TargetCorpus)
	}
	if amendment.Applied {
		t.Error("StageAmendment() Applied = true, want false for a freshly staged amendment")
	}

	list, err := engine.ListAmendments(ctx, tenantID, job.ID)
	if err != nil {
		t.Fatalf("ListAmendments() = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListAmendments() returned %d amendments, want 1", len(list))
	}
}

func TestStageAmendment_RejectedAfterApplying(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(adminUser(tenantID))

	job, err := engine.CreateJob(ctx, tenantID, corpusupdater.CorpusUpdateJob{
		JurisdictionCode: "AE-DXB",
		TargetCorpus:     corpusupdater.CorpusStatute,
	})
	if err != nil {
		t.Fatalf("CreateJob() = %v", err)
	}
	if err := engine.ValidateJob(ctx, tenantID, job.ID, nil); err != nil {
		t.Fatalf("ValidateJob() = %v", err)
	}

	// Job is now StatusApplying; staging should be rejected.
	_, err = engine.StageAmendment(ctx, tenantID, job.ID, corpusupdater.Amendment{
		TargetID:      "rule-1",
		ChangeType:    corpusupdater.ChangeTypeAmend,
		Citation:      "Some citation",
		EffectiveDate: time.Now(),
	})
	if !errors.Is(err, corpusupdater.ErrInvalidJobTransition) {
		t.Errorf("StageAmendment() on StatusApplying job = %v, want ErrInvalidJobTransition", err)
	}
}

func TestValidateJob_AllValidTransitionsToApplying(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(adminUser(tenantID))

	job, _ := stageJobWithAmendment(t, engine, ctx, tenantID, corpusupdater.Amendment{
		TargetID:      "rule-1",
		ChangeType:    corpusupdater.ChangeTypeAmend,
		NewText:       "Updated text.",
		Citation:      "Federal Decree-Law No. 45 of 2023",
		EffectiveDate: time.Now(),
	})

	if err := engine.ValidateJob(ctx, tenantID, job.ID, nil); err != nil {
		t.Fatalf("ValidateJob() = %v, want nil", err)
	}

	fetched, err := engine.GetJob(ctx, tenantID, job.ID)
	if err != nil {
		t.Fatalf("GetJob() = %v", err)
	}
	if fetched.Status != corpusupdater.StatusApplying {
		t.Errorf("GetJob().Status = %q after ValidateJob, want %q", fetched.Status, corpusupdater.StatusApplying)
	}
}

func TestValidateJob_InvalidAmendmentFailsJob(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(adminUser(tenantID))

	job, _ := stageJobWithAmendment(t, engine, ctx, tenantID, corpusupdater.Amendment{
		ChangeType:    corpusupdater.ChangeTypeAmend,
		NewText:       "Updated text.",
		Citation:      "Federal Decree-Law No. 45 of 2023",
		EffectiveDate: time.Now(),
		// TargetID deliberately left blank: invalid for ChangeTypeAmend.
	})

	err := engine.ValidateJob(ctx, tenantID, job.ID, nil)
	if !errors.Is(err, corpusupdater.ErrMissingTargetID) {
		t.Errorf("ValidateJob() with invalid amendment = %v, want ErrMissingTargetID", err)
	}

	fetched, getErr := engine.GetJob(ctx, tenantID, job.ID)
	if getErr != nil {
		t.Fatalf("GetJob() = %v", getErr)
	}
	if fetched.Status != corpusupdater.StatusFailed {
		t.Errorf("GetJob().Status = %q after failed validation, want %q", fetched.Status, corpusupdater.StatusFailed)
	}
	if fetched.FailureReason == "" {
		t.Error("GetJob().FailureReason is empty, want the validation error recorded")
	}
}

func TestApplyAmendment_SnapshotsPreviousTextAndEmbedsOnce(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(adminUser(tenantID))
	store := newFakeTextStore()
	embedder := &fakeEmbedder{}

	// Seed the store with the target's pre-amendment text.
	if err := store.SetText(ctx, corpusupdater.CorpusStatute, "rule-1", corpusupdater.CorpusText{
		Text: "Old rule text.", Citation: "Old citation",
	}); err != nil {
		t.Fatalf("SetText() = %v", err)
	}

	job, amendment := stageJobWithAmendment(t, engine, ctx, tenantID, corpusupdater.Amendment{
		TargetID:      "rule-1",
		ChangeType:    corpusupdater.ChangeTypeAmend,
		NewText:       "New rule text.",
		Citation:      "Federal Decree-Law No. 45 of 2023",
		EffectiveDate: time.Now(),
	})
	_ = job

	applied, err := engine.ApplyAmendment(ctx, tenantID, amendment, store, embedder, nil, nil)
	if err != nil {
		t.Fatalf("ApplyAmendment() = %v", err)
	}

	if applied.PreviousText != "Old rule text." {
		t.Errorf("ApplyAmendment() PreviousText = %q, want %q", applied.PreviousText, "Old rule text.")
	}
	if applied.PreviousCitation != "Old citation" {
		t.Errorf("ApplyAmendment() PreviousCitation = %q, want %q", applied.PreviousCitation, "Old citation")
	}
	if !applied.Applied {
		t.Error("ApplyAmendment() Applied = false, want true")
	}

	current := store.get(corpusupdater.CorpusStatute, "rule-1")
	if current.Text != "New rule text." {
		t.Errorf("store text after ApplyAmendment = %q, want %q", current.Text, "New rule text.")
	}

	if embedder.calls != 1 {
		t.Errorf("Embedder.Embed called %d times, want exactly 1", embedder.calls)
	}
	if len(embedder.texts) != 1 || len(embedder.texts[0]) != 1 || embedder.texts[0][0] != "New rule text." {
		t.Errorf("Embedder.Embed called with %v, want [[\"New rule text.\"]]", embedder.texts)
	}
}

func TestApplyAmendment_RepealSkipsEmbedding(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(adminUser(tenantID))
	store := newFakeTextStore()
	embedder := &fakeEmbedder{}

	job, amendment := stageJobWithAmendment(t, engine, ctx, tenantID, corpusupdater.Amendment{
		TargetID:      "rule-1",
		ChangeType:    corpusupdater.ChangeTypeRepeal,
		Citation:      "Repealing instrument",
		EffectiveDate: time.Now(),
	})
	_ = job

	if _, err := engine.ApplyAmendment(ctx, tenantID, amendment, store, embedder, nil, nil); err != nil {
		t.Fatalf("ApplyAmendment() = %v", err)
	}
	if embedder.calls != 0 {
		t.Errorf("Embedder.Embed called %d times for a repeal, want 0", embedder.calls)
	}
}

func TestApplyAmendment_FiresNotificationWithAffectedCases(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(adminUser(tenantID))
	store := newFakeTextStore()
	sink := &fakeNotificationSink{}

	caseID := uuid.New()
	resolver := func(_ context.Context, corpus corpusupdater.CorpusTarget, targetID string) ([]uuid.UUID, error) {
		if corpus == corpusupdater.CorpusStatute && targetID == "rule-1" {
			return []uuid.UUID{caseID}, nil
		}
		return nil, nil
	}

	_, amendment := stageJobWithAmendment(t, engine, ctx, tenantID, corpusupdater.Amendment{
		TargetID:      "rule-1",
		ChangeType:    corpusupdater.ChangeTypeAmend,
		NewText:       "New text.",
		Citation:      "Some citation",
		EffectiveDate: time.Now(),
	})

	if _, err := engine.ApplyAmendment(ctx, tenantID, amendment, store, nil, sink, resolver); err != nil {
		t.Fatalf("ApplyAmendment() = %v", err)
	}

	if len(sink.notifications) != 1 {
		t.Fatalf("NotificationSink received %d notifications, want 1", len(sink.notifications))
	}
	n := sink.notifications[0]
	if len(n.AffectedCaseIDs) != 1 || n.AffectedCaseIDs[0] != caseID {
		t.Errorf("ChangeNotification.AffectedCaseIDs = %v, want [%v]", n.AffectedCaseIDs, caseID)
	}
	if n.TargetID != "rule-1" {
		t.Errorf("ChangeNotification.TargetID = %q, want %q", n.TargetID, "rule-1")
	}
}

func TestApplyAmendment_NilNotificationResolverYieldsEmptyList(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(adminUser(tenantID))
	store := newFakeTextStore()
	sink := &fakeNotificationSink{}

	_, amendment := stageJobWithAmendment(t, engine, ctx, tenantID, corpusupdater.Amendment{
		TargetID:      "rule-1",
		ChangeType:    corpusupdater.ChangeTypeAmend,
		NewText:       "New text.",
		Citation:      "Some citation",
		EffectiveDate: time.Now(),
	})

	if _, err := engine.ApplyAmendment(ctx, tenantID, amendment, store, nil, sink, nil); err != nil {
		t.Fatalf("ApplyAmendment() = %v", err)
	}
	if len(sink.notifications) != 1 {
		t.Fatalf("NotificationSink received %d notifications, want 1", len(sink.notifications))
	}
	if len(sink.notifications[0].AffectedCaseIDs) != 0 {
		t.Errorf("AffectedCaseIDs = %v, want empty with nil resolver", sink.notifications[0].AffectedCaseIDs)
	}
}

func TestApplyJob_AppliesOnlyEffectiveAmendments(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(adminUser(tenantID))
	store := newFakeTextStore()
	embedder := &fakeEmbedder{}

	job, err := engine.CreateJob(ctx, tenantID, corpusupdater.CorpusUpdateJob{
		JurisdictionCode: "AE-DXB",
		TargetCorpus:     corpusupdater.CorpusStatute,
	})
	if err != nil {
		t.Fatalf("CreateJob() = %v", err)
	}

	pastAmendment, err := engine.StageAmendment(ctx, tenantID, job.ID, corpusupdater.Amendment{
		TargetID:      "rule-past",
		ChangeType:    corpusupdater.ChangeTypeAmend,
		NewText:       "Past text.",
		Citation:      "Past citation",
		EffectiveDate: time.Now().Add(-24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("StageAmendment() = %v", err)
	}
	futureAmendment, err := engine.StageAmendment(ctx, tenantID, job.ID, corpusupdater.Amendment{
		TargetID:      "rule-future",
		ChangeType:    corpusupdater.ChangeTypeAmend,
		NewText:       "Future text.",
		Citation:      "Future citation",
		EffectiveDate: time.Now().Add(365 * 24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("StageAmendment() = %v", err)
	}

	if err := engine.ValidateJob(ctx, tenantID, job.ID, nil); err != nil {
		t.Fatalf("ValidateJob() = %v", err)
	}
	if err := engine.ApplyJob(ctx, tenantID, job.ID, store, embedder, nil, nil); err != nil {
		t.Fatalf("ApplyJob() = %v", err)
	}

	fetched, err := engine.GetJob(ctx, tenantID, job.ID)
	if err != nil {
		t.Fatalf("GetJob() = %v", err)
	}
	if fetched.Status != corpusupdater.StatusApplied {
		t.Errorf("GetJob().Status = %q, want %q", fetched.Status, corpusupdater.StatusApplied)
	}

	amendments, err := engine.ListAmendments(ctx, tenantID, job.ID)
	if err != nil {
		t.Fatalf("ListAmendments() = %v", err)
	}
	byID := make(map[uuid.UUID]corpusupdater.Amendment, len(amendments))
	for _, a := range amendments {
		byID[a.ID] = a
	}
	if !byID[pastAmendment.ID].Applied {
		t.Error("past-effective-date amendment was not applied")
	}
	if byID[futureAmendment.ID].Applied {
		t.Error("future-effective-date amendment was applied, want it to stay staged only")
	}

	effective, err := engine.EffectiveAmendments(ctx, tenantID, job.ID, time.Now())
	if err != nil {
		t.Fatalf("EffectiveAmendments() = %v", err)
	}
	if len(effective) != 1 || effective[0].ID != pastAmendment.ID {
		t.Errorf("EffectiveAmendments() = %v, want just the past amendment", effective)
	}
}

func TestRollback_RevertsAppliedAmendmentsAndTransitionsJob(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(adminUser(tenantID))
	store := newFakeTextStore()

	if err := store.SetText(ctx, corpusupdater.CorpusStatute, "rule-1", corpusupdater.CorpusText{
		Text: "Original text.", Citation: "Original citation",
	}); err != nil {
		t.Fatalf("SetText() = %v", err)
	}

	job, err := engine.CreateJob(ctx, tenantID, corpusupdater.CorpusUpdateJob{
		JurisdictionCode: "AE-DXB",
		TargetCorpus:     corpusupdater.CorpusStatute,
	})
	if err != nil {
		t.Fatalf("CreateJob() = %v", err)
	}
	if _, err := engine.StageAmendment(ctx, tenantID, job.ID, corpusupdater.Amendment{
		TargetID:      "rule-1",
		ChangeType:    corpusupdater.ChangeTypeAmend,
		NewText:       "Amended text.",
		Citation:      "Amending citation",
		EffectiveDate: time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("StageAmendment() = %v", err)
	}
	if err := engine.ValidateJob(ctx, tenantID, job.ID, nil); err != nil {
		t.Fatalf("ValidateJob() = %v", err)
	}
	if err := engine.ApplyJob(ctx, tenantID, job.ID, store, nil, nil, nil); err != nil {
		t.Fatalf("ApplyJob() = %v", err)
	}

	// Confirm the amendment actually landed before rolling back.
	if got := store.get(corpusupdater.CorpusStatute, "rule-1"); got.Text != "Amended text." {
		t.Fatalf("store text after ApplyJob = %q, want %q", got.Text, "Amended text.")
	}

	if err := engine.Rollback(ctx, tenantID, job.ID, store); err != nil {
		t.Fatalf("Rollback() = %v", err)
	}

	restored := store.get(corpusupdater.CorpusStatute, "rule-1")
	if restored.Text != "Original text." {
		t.Errorf("store text after Rollback = %q, want %q", restored.Text, "Original text.")
	}
	if restored.Citation != "Original citation" {
		t.Errorf("store citation after Rollback = %q, want %q", restored.Citation, "Original citation")
	}

	fetched, err := engine.GetJob(ctx, tenantID, job.ID)
	if err != nil {
		t.Fatalf("GetJob() = %v", err)
	}
	if fetched.Status != corpusupdater.StatusRolledBack {
		t.Errorf("GetJob().Status = %q after Rollback, want %q", fetched.Status, corpusupdater.StatusRolledBack)
	}

	amendments, err := engine.ListAmendments(ctx, tenantID, job.ID)
	if err != nil {
		t.Fatalf("ListAmendments() = %v", err)
	}
	if len(amendments) != 1 || !amendments[0].RolledBack {
		t.Errorf("amendment RolledBack = %v, want true after Rollback", amendments)
	}
}

func TestRollback_RejectedWhenJobNotApplied(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(adminUser(tenantID))
	store := newFakeTextStore()

	job, err := engine.CreateJob(ctx, tenantID, corpusupdater.CorpusUpdateJob{
		JurisdictionCode: "AE-DXB",
		TargetCorpus:     corpusupdater.CorpusStatute,
	})
	if err != nil {
		t.Fatalf("CreateJob() = %v", err)
	}

	// Job is still StatusPending: Rollback should refuse.
	if err := engine.Rollback(ctx, tenantID, job.ID, store); !errors.Is(err, corpusupdater.ErrJobNotApplied) {
		t.Errorf("Rollback() on StatusPending job = %v, want ErrJobNotApplied", err)
	}
}

func TestRollback_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	adminCtx := ctxWithUser(adminUser(tenantID))
	store := newFakeTextStore()

	job, err := engine.CreateJob(adminCtx, tenantID, corpusupdater.CorpusUpdateJob{
		JurisdictionCode: "AE-DXB",
		TargetCorpus:     corpusupdater.CorpusStatute,
	})
	if err != nil {
		t.Fatalf("CreateJob() = %v", err)
	}

	auditorCtx := ctxWithUser(auditorUser(tenantID))
	if err := engine.Rollback(auditorCtx, tenantID, job.ID, store); !errors.Is(err, corpusupdater.ErrForbidden) {
		t.Errorf("Rollback() as auditor = %v, want ErrForbidden", err)
	}
}

func TestAuditSink_RecordsJobAndAmendmentActivity(t *testing.T) {
	t.Parallel()
	engine, auditStore, tenantID := newTestEngineWithAudit(t)
	ctx := ctxWithUser(adminUser(tenantID))

	job, err := engine.CreateJob(ctx, tenantID, corpusupdater.CorpusUpdateJob{
		JurisdictionCode: "AE-DXB",
		TargetCorpus:     corpusupdater.CorpusStatute,
	})
	if err != nil {
		t.Fatalf("CreateJob() = %v", err)
	}
	if _, err := engine.StageAmendment(ctx, tenantID, job.ID, corpusupdater.Amendment{
		TargetID:      "rule-1",
		ChangeType:    corpusupdater.ChangeTypeAmend,
		NewText:       "New text.",
		Citation:      "Some citation",
		EffectiveDate: time.Now(),
	}); err != nil {
		t.Fatalf("StageAmendment() = %v", err)
	}

	events, err := auditStore.Query(ctx, tenantID, auditlog.Filter{})
	if err != nil {
		t.Fatalf("auditStore.Query() = %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("audit trail has %d events, want at least 2 (job create + amendment stage)", len(events))
	}

	sawJobCreate, sawAmendmentStage := false, false
	for _, ev := range events {
		switch ev.Action {
		case "corpusupdater.job_create":
			sawJobCreate = true
		case "corpusupdater.amendment_stage":
			sawAmendmentStage = true
		}
	}
	if !sawJobCreate {
		t.Error("audit trail missing corpusupdater.job_create event")
	}
	if !sawAmendmentStage {
		t.Error("audit trail missing corpusupdater.amendment_stage event")
	}
}
