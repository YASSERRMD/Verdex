package bulkimport_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/bulkimport"
)

func TestStatus_CanTransitionTo(t *testing.T) {
	t.Parallel()

	cases := []struct {
		from bulkimport.Status
		to   bulkimport.Status
		want bool
	}{
		{bulkimport.StatusPending, bulkimport.StatusRunning, true},
		{bulkimport.StatusPending, bulkimport.StatusPaused, false},
		{bulkimport.StatusPending, bulkimport.StatusCompleted, false},
		{bulkimport.StatusRunning, bulkimport.StatusPaused, true},
		{bulkimport.StatusRunning, bulkimport.StatusCompleted, true},
		{bulkimport.StatusRunning, bulkimport.StatusFailed, true},
		{bulkimport.StatusPaused, bulkimport.StatusRunning, true},
		{bulkimport.StatusPaused, bulkimport.StatusCompleted, false},
		{bulkimport.StatusCompleted, bulkimport.StatusRolledBack, true},
		{bulkimport.StatusCompleted, bulkimport.StatusRunning, false},
		{bulkimport.StatusFailed, bulkimport.StatusRolledBack, true},
		{bulkimport.StatusFailed, bulkimport.StatusRunning, true},
		{bulkimport.StatusRolledBack, bulkimport.StatusRunning, false},
		{bulkimport.StatusRolledBack, bulkimport.StatusPending, false},
	}

	for _, tc := range cases {
		if got := tc.from.CanTransitionTo(tc.to); got != tc.want {
			t.Errorf("%s.CanTransitionTo(%s) = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}

func TestStatus_IsTerminal(t *testing.T) {
	t.Parallel()

	terminal := []bulkimport.Status{bulkimport.StatusCompleted, bulkimport.StatusFailed, bulkimport.StatusRolledBack}
	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("%s.IsTerminal() = false, want true", s)
		}
	}

	nonTerminal := []bulkimport.Status{bulkimport.StatusPending, bulkimport.StatusRunning, bulkimport.StatusPaused}
	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Errorf("%s.IsTerminal() = true, want false", s)
		}
	}
}

func TestStatus_IsValid(t *testing.T) {
	t.Parallel()
	if bulkimport.Status("bogus").IsValid() {
		t.Error(`Status("bogus").IsValid() = true, want false`)
	}
	if !bulkimport.StatusPending.IsValid() {
		t.Error("StatusPending.IsValid() = false, want true")
	}
}

func TestImportJob_Validate(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	now := time.Now()

	valid := bulkimport.NewImportJob(tenantID, "a real source", uuid.New(), 10, now)
	if err := valid.Validate(); err != nil {
		t.Errorf("Validate() on well-formed job = %v, want nil", err)
	}

	missingTenant := valid
	missingTenant.TenantID = uuid.Nil
	if err := missingTenant.Validate(); !errors.Is(err, bulkimport.ErrEmptyTenantID) {
		t.Errorf("Validate() with nil TenantID = %v, want ErrEmptyTenantID", err)
	}

	blankSource := valid
	blankSource.SourceDescription = "   "
	if err := blankSource.Validate(); !errors.Is(err, bulkimport.ErrInvalidJob) {
		t.Errorf("Validate() with blank SourceDescription = %v, want ErrInvalidJob", err)
	}

	badStatus := valid
	badStatus.Status = bulkimport.Status("nonsense")
	if err := badStatus.Validate(); !errors.Is(err, bulkimport.ErrInvalidJob) {
		t.Errorf("Validate() with invalid Status = %v, want ErrInvalidJob", err)
	}

	negativeCounts := valid
	negativeCounts.Cursor = -1
	if err := negativeCounts.Validate(); !errors.Is(err, bulkimport.ErrInvalidJob) {
		t.Errorf("Validate() with negative Cursor = %v, want ErrInvalidJob", err)
	}

	var nilJob *bulkimport.ImportJob
	if err := nilJob.Validate(); !errors.Is(err, bulkimport.ErrInvalidJob) {
		t.Errorf("Validate() on nil *ImportJob = %v, want ErrInvalidJob", err)
	}
}

func TestImportJob_Clone_IsIndependent(t *testing.T) {
	t.Parallel()
	now := time.Now()
	job := bulkimport.NewImportJob(uuid.New(), "src", uuid.New(), 5, now)
	job.StartedAt = &now

	cp := job.Clone()
	newTime := now.Add(time.Hour)
	*cp.StartedAt = newTime

	if job.StartedAt.Equal(newTime) {
		t.Fatal("mutating clone's StartedAt affected the original job")
	}
}

func TestImportRecord_Validate(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	jobID := uuid.New()

	valid := bulkimport.ImportRecord{
		TenantID:         tenantID,
		JobID:            jobID,
		ValidationStatus: bulkimport.ValidationPending,
		Outcome:          bulkimport.OutcomePending,
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("Validate() on well-formed record = %v, want nil", err)
	}

	missingJob := valid
	missingJob.JobID = uuid.Nil
	if err := missingJob.Validate(); !errors.Is(err, bulkimport.ErrInvalidRecord) {
		t.Errorf("Validate() with nil JobID = %v, want ErrInvalidRecord", err)
	}

	badOutcome := valid
	badOutcome.Outcome = bulkimport.Outcome("nonsense")
	if err := badOutcome.Validate(); !errors.Is(err, bulkimport.ErrInvalidRecord) {
		t.Errorf("Validate() with invalid Outcome = %v, want ErrInvalidRecord", err)
	}

	var nilRecord *bulkimport.ImportRecord
	if err := nilRecord.Validate(); !errors.Is(err, bulkimport.ErrInvalidRecord) {
		t.Errorf("Validate() on nil *ImportRecord = %v, want ErrInvalidRecord", err)
	}
}

func TestImportRecord_Clone_IsIndependent(t *testing.T) {
	t.Parallel()
	rec := bulkimport.ImportRecord{
		PartyNames:       []string{"a", "b"},
		ValidationErrors: []bulkimport.ValidationError{{Field: "x", Reason: "y"}},
	}
	cp := rec.Clone()
	cp.PartyNames[0] = "mutated"
	cp.ValidationErrors[0].Reason = "mutated"

	if rec.PartyNames[0] == "mutated" {
		t.Fatal("mutating clone's PartyNames affected the original record")
	}
	if rec.ValidationErrors[0].Reason == "mutated" {
		t.Fatal("mutating clone's ValidationErrors affected the original record")
	}
}
