package accounting_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/accounting"
)

func newTestService(cfg *accounting.BudgetConfig) (*accounting.AccountingService, *accounting.InMemoryRepository, *accounting.InMemoryBudgetChecker, *captureAlertSink) {
	repo := accounting.NewInMemoryRepository()
	var configs []accounting.BudgetConfig
	if cfg != nil {
		configs = append(configs, *cfg)
	}
	checker := accounting.NewInMemoryBudgetChecker(configs)
	sink := &captureAlertSink{}
	svc := accounting.NewAccountingService(repo, checker, sink)
	return svc, repo, checker, sink
}

// captureAlertSink records every alert event sent to it.
type captureAlertSink struct {
	events []accounting.AlertEvent
}

func (s *captureAlertSink) Send(_ context.Context, event accounting.AlertEvent) error {
	s.events = append(s.events, event)
	return nil
}

func makeRecord(tenantID uuid.UUID, caseID *uuid.UUID, tokens int) accounting.UsageRecord {
	cost := float64(tokens) / 1_000_000.0 * 3.0
	return accounting.UsageRecord{
		ID:           uuid.New(),
		TenantID:     tenantID,
		CaseID:       caseID,
		ProviderID:   "test-provider",
		TaskType:     "chat",
		InputTokens:  tokens / 2,
		OutputTokens: tokens / 2,
		TotalTokens:  tokens,
		CostUSD:      &cost,
		CreatedAt:    time.Now().UTC(),
	}
}

// TestRecordUsage_Retrievable verifies that a recorded usage is retrievable
// via GetCaseUsage and GetUsageSummary.
func TestRecordUsage_Retrievable(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newTestService(nil)
	ctx := context.Background()

	tenantID := uuid.New()
	caseID := uuid.New()
	record := makeRecord(tenantID, &caseID, 1000)

	if err := svc.RecordUsage(ctx, record); err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}

	summary, err := svc.GetCaseUsage(ctx, caseID)
	if err != nil {
		t.Fatalf("GetCaseUsage: %v", err)
	}
	if summary.TotalTokens != 1000 {
		t.Errorf("TotalTokens: got %d, want 1000", summary.TotalTokens)
	}
	if summary.RequestCount != 1 {
		t.Errorf("RequestCount: got %d, want 1", summary.RequestCount)
	}
}

// TestBudgetCheck_BlocksOnExceeded verifies that RecordUsage returns
// ErrBudgetExceeded when a HardStop limit is breached.
func TestBudgetCheck_BlocksOnExceeded(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tenantID := uuid.New()
	limit := 100
	cfg := &accounting.BudgetConfig{
		TenantID:        tenantID,
		DailyTokenLimit: &limit,
		HardStop:        true,
		AlertThresholdPct: 80,
	}
	svc, _, _, _ := newTestService(cfg)

	// First call: 50 tokens — allowed.
	r1 := makeRecord(tenantID, nil, 50)
	if err := svc.RecordUsage(ctx, r1); err != nil {
		t.Fatalf("first RecordUsage should succeed: %v", err)
	}

	// Second call: another 60 tokens → total 110 > 100 → exceeded.
	r2 := makeRecord(tenantID, nil, 60)
	err := svc.RecordUsage(ctx, r2)
	if err == nil {
		t.Fatal("expected ErrBudgetExceeded, got nil")
	}
}

// TestAlertFiringAtThreshold verifies that an alert is sent when usage
// crosses AlertThresholdPct without triggering a HardStop.
func TestAlertFiringAtThreshold(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tenantID := uuid.New()
	limit := 100
	cfg := &accounting.BudgetConfig{
		TenantID:          tenantID,
		DailyTokenLimit:   &limit,
		HardStop:          false, // soft warning only
		AlertThresholdPct: 80,
	}
	svc, _, _, sink := newTestService(cfg)

	// 85 tokens → 85 % > 80 % threshold → alert should fire.
	r := makeRecord(tenantID, nil, 85)
	if err := svc.RecordUsage(ctx, r); err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}

	if len(sink.events) == 0 {
		t.Fatal("expected at least one alert event, got none")
	}
	for _, ev := range sink.events {
		if ev.AlertType != accounting.AlertTypeBudgetWarning && ev.AlertType != accounting.AlertTypeBudgetExceeded {
			t.Errorf("unexpected AlertType: %q", ev.AlertType)
		}
	}
}

// TestReconcile_Idempotent verifies that running ReconcileJob twice produces
// the same in-memory state as running it once.
func TestReconcile_Idempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tenantID := uuid.New()
	svc, repo, checker, _ := newTestService(nil)

	for i := 0; i < 3; i++ {
		r := makeRecord(tenantID, nil, 100)
		if err := svc.RecordUsage(ctx, r); err != nil {
			t.Fatalf("RecordUsage: %v", err)
		}
	}

	job := accounting.NewReconcileJob(repo, checker)

	count1, err := job.Run(ctx)
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}

	usage1 := checker.CurrentUsage(tenantID, time.Now().UTC())

	count2, err := job.Run(ctx)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}

	usage2 := checker.CurrentUsage(tenantID, time.Now().UTC())

	if count1 != count2 {
		t.Errorf("reconcile counts differ: %d vs %d", count1, count2)
	}
	if usage1.DailyTokens != usage2.DailyTokens {
		t.Errorf("daily tokens differ after second run: %d vs %d", usage1.DailyTokens, usage2.DailyTokens)
	}
	if usage1.DailyTokens != 300 {
		t.Errorf("expected 300 daily tokens, got %d", usage1.DailyTokens)
	}
}

// TestExportUsage verifies that ExportUsage returns only records within the
// requested time range.
func TestExportUsage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _, _, _ := newTestService(nil)

	tenantID := uuid.New()
	now := time.Now().UTC()

	for i := 0; i < 5; i++ {
		r := makeRecord(tenantID, nil, 100)
		r.CreatedAt = now.Add(time.Duration(-i) * 24 * time.Hour)
		if err := svc.RecordUsage(ctx, r); err != nil {
			t.Fatalf("RecordUsage: %v", err)
		}
	}

	// Export only last 2 days.
	from := now.Add(-2 * 24 * time.Hour)
	to := now.Add(time.Hour)
	exported, err := svc.ExportUsage(ctx, tenantID, from, to)
	if err != nil {
		t.Fatalf("ExportUsage: %v", err)
	}
	// Should include records from today, yesterday, and 2 days ago (3 records).
	if len(exported) != 3 {
		t.Errorf("expected 3 exported records, got %d", len(exported))
	}
}
