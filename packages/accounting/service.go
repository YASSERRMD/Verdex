package accounting

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AccountingService is the primary application service for token accounting.
// It coordinates record persistence, budget enforcement, and alert delivery.
//
// Construct an AccountingService with NewAccountingService.
type AccountingService struct {
	repo    Repository
	checker *InMemoryBudgetChecker
	alerts  AlertSink
}

// NewAccountingService constructs an AccountingService.
//
//   - repo is used to persist and query UsageRecord values.
//   - checker is the in-memory budget state tracker (may be nil for no budget checks).
//   - alerts is the sink for budget warning / exceeded events (may be nil for no alerts).
func NewAccountingService(repo Repository, checker *InMemoryBudgetChecker, alerts AlertSink) *AccountingService {
	if alerts == nil {
		alerts = NoOpAlertSink{}
	}
	return &AccountingService{
		repo:    repo,
		checker: checker,
		alerts:  alerts,
	}
}

// RecordUsage persists a UsageRecord, updates in-memory budget state, checks
// the budget, and fires alerts as needed.
//
// If the budget has a HardStop and is exceeded, RecordUsage returns
// ErrBudgetExceeded after persisting the record (the record is always
// persisted for audit purposes).
func (s *AccountingService) RecordUsage(ctx context.Context, record UsageRecord) error {
	if record.ID == uuid.Nil {
		record.ID = uuid.New()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}

	if err := s.repo.SaveRecord(ctx, record); err != nil {
		return fmt.Errorf("accounting: save record: %w", err)
	}

	if s.checker == nil {
		return nil
	}

	cost := 0.0
	if record.CostUSD != nil {
		cost = *record.CostUSD
	}
	s.checker.RecordUsage(record.TenantID, record.TotalTokens, cost, record.CreatedAt)

	usage := s.checker.CurrentUsage(record.TenantID, record.CreatedAt)
	allowed, alert, budgetErr := s.checker.Check(ctx, record.TenantID, usage)

	if alert {
		alertType := AlertTypeBudgetWarning
		if !allowed {
			alertType = AlertTypeBudgetExceeded
		}
		event := buildAlertEvent(record.TenantID, alertType, usage, s.checker, record.TenantID)
		_ = s.alerts.Send(ctx, event)
	}

	if budgetErr != nil {
		return budgetErr
	}
	return nil
}

// GetUsageSummary returns a UsageSummary for tenantID over a named period.
// period must be either "daily" (today) or "monthly" (this calendar month).
func (s *AccountingService) GetUsageSummary(ctx context.Context, tenantID uuid.UUID, period string) (*UsageSummary, error) {
	now := time.Now().UTC()
	var from, to time.Time
	switch period {
	case "daily":
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		to = from.Add(24 * time.Hour)
	case "monthly":
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		to = from.AddDate(0, 1, 0)
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidPeriod, period)
	}
	return s.repo.SumByTenant(ctx, tenantID, from, to)
}

// GetCaseUsage returns the aggregate token usage for a judicial case.
func (s *AccountingService) GetCaseUsage(ctx context.Context, caseID uuid.UUID) (*UsageSummary, error) {
	return s.repo.SumByCase(ctx, caseID)
}

// ExportUsage returns all UsageRecord values for tenantID whose CreatedAt
// falls within [from, to].  It pages through the repository using a fixed
// batch size.
func (s *AccountingService) ExportUsage(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]UsageRecord, error) {
	const batchSize = 500
	var all []UsageRecord
	offset := 0

	for {
		batch, err := s.repo.ListRecords(ctx, tenantID, batchSize, offset)
		if err != nil {
			return nil, fmt.Errorf("accounting: export usage: %w", err)
		}
		for _, rec := range batch {
			if !rec.CreatedAt.Before(from) && !rec.CreatedAt.After(to) {
				all = append(all, rec)
			}
		}
		if len(batch) < batchSize {
			break
		}
		offset += batchSize
	}
	return all, nil
}

// buildAlertEvent constructs an AlertEvent from current usage totals.
func buildAlertEvent(tenantID uuid.UUID, alertType string, u TokenUsage, checker *InMemoryBudgetChecker, _ uuid.UUID) AlertEvent {
	event := AlertEvent{
		TenantID:       tenantID,
		AlertType:      alertType,
		CurrentUsage:   u.DailyTokens + u.MonthlyTokens,
		CurrentCostUSD: u.DailyCostUSD + u.MonthlyCostUSD,
		CreatedAt:      time.Now().UTC(),
	}
	if checker != nil {
		checker.mu.RLock()
		if cfg, ok := checker.configs[tenantID]; ok {
			if cfg.DailyTokenLimit != nil {
				event.Limit = *cfg.DailyTokenLimit
			}
			if cfg.DailyCostLimitUSD != nil {
				event.LimitUSD = *cfg.DailyCostLimitUSD
			}
		}
		checker.mu.RUnlock()
	}
	return event
}
