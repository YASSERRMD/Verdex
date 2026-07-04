package bulkimport_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/bulkimport"
)

func TestEngine_GetProgress_ZeroTotalIsIndeterminate(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	job, err := engine.RegisterJob(ctxWithUser(admin), tenantID, "unknown-size corpus", 0)
	if err != nil {
		t.Fatalf("RegisterJob: %v", err)
	}

	progress, err := engine.GetProgress(ctxWithUser(admin), tenantID, job.ID)
	if err != nil {
		t.Fatalf("GetProgress: %v", err)
	}
	if progress.PercentComplete != 0 {
		t.Fatalf("PercentComplete for zero-total job = %v, want 0 (indeterminate)", progress.PercentComplete)
	}
}

func TestProgress_EstimatedTimeRemaining(t *testing.T) {
	t.Parallel()
	start := time.Now().Add(-10 * time.Minute)
	now := time.Now()

	progress := bulkimport.Progress{
		Total:         100,
		Processed:     50,
		StartedAt:     start,
		LastUpdatedAt: now,
	}

	eta, ok := progress.EstimatedTimeRemaining()
	if !ok {
		t.Fatal("EstimatedTimeRemaining() ok = false, want true")
	}
	// 50 records in ~10 minutes => ~10 more minutes for the remaining 50.
	if eta < 8*time.Minute || eta > 12*time.Minute {
		t.Fatalf("EstimatedTimeRemaining() = %v, want approximately 10m", eta)
	}
}

func TestProgress_EstimatedTimeRemaining_InsufficientData(t *testing.T) {
	t.Parallel()

	cases := []bulkimport.Progress{
		{Total: 100, Processed: 0, StartedAt: time.Now()},
		{Total: 0, Processed: 10, StartedAt: time.Now()},
		{Total: 100, Processed: 10},
	}
	for i, p := range cases {
		if _, ok := p.EstimatedTimeRemaining(); ok {
			t.Errorf("case %d: EstimatedTimeRemaining() ok = true, want false for %+v", i, p)
		}
	}
}

func TestProgress_EstimatedTimeRemaining_AlreadyDone(t *testing.T) {
	t.Parallel()
	start := time.Now().Add(-time.Hour)
	progress := bulkimport.Progress{
		Total:         10,
		Processed:     10,
		StartedAt:     start,
		LastUpdatedAt: time.Now(),
	}
	eta, ok := progress.EstimatedTimeRemaining()
	if !ok {
		t.Fatal("EstimatedTimeRemaining() ok = false, want true")
	}
	if eta != 0 {
		t.Fatalf("EstimatedTimeRemaining() for a fully processed job = %v, want 0", eta)
	}
}

// TestEngine_GetProgress_RequiresViewPermission proves an
// unauthenticated caller cannot read progress even for a job in their
// own tenant.
func TestEngine_GetProgress_RequiresViewPermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	job, err := engine.RegisterJob(ctxWithUser(admin), tenantID, "corpus", 10)
	if err != nil {
		t.Fatalf("RegisterJob: %v", err)
	}

	noRoleUser := newTestUser(tenantID)
	if _, err := engine.GetProgress(ctxWithUser(noRoleUser), tenantID, job.ID); err == nil {
		t.Fatal("GetProgress() with no-permission user succeeded, want an error")
	}
}

func TestEngine_GetProgress_UnknownJob(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	if _, err := engine.GetProgress(ctxWithUser(admin), tenantID, uuid.New()); err == nil {
		t.Fatal("GetProgress() for unknown job succeeded, want an error")
	}
}
