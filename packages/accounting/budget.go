package accounting

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// BudgetConfig holds the budget limits for a single tenant.
//
// Nil limit pointers mean "no limit".  AlertThresholdPct is a value between
// 0 and 100 (e.g. 80 means alert when 80 % of any limit is consumed).
type BudgetConfig struct {
	// TenantID identifies the tenant this config applies to.
	TenantID uuid.UUID
	// DailyTokenLimit is the maximum total tokens allowed per calendar day.
	DailyTokenLimit *int
	// MonthlyTokenLimit is the maximum total tokens allowed per calendar month.
	MonthlyTokenLimit *int
	// DailyCostLimitUSD is the maximum estimated cost per calendar day.
	DailyCostLimitUSD *float64
	// MonthlyCostLimitUSD is the maximum estimated cost per calendar month.
	MonthlyCostLimitUSD *float64
	// HardStop causes Check to return ErrBudgetExceeded when any limit is
	// breached.  When false, only a warning is emitted.
	HardStop bool
	// AlertThresholdPct is the percentage of any limit at which a warning
	// alert is fired (e.g. 80).  Values outside [1, 100] are clamped to 100
	// (i.e. no warning).
	AlertThresholdPct float64
}

// TokenUsage carries the current usage totals for a single Check evaluation.
type TokenUsage struct {
	// DailyTokens is the total tokens consumed today (including the current call).
	DailyTokens int
	// MonthlyTokens is the total tokens consumed this month.
	MonthlyTokens int
	// DailyCostUSD is the estimated cost incurred today.
	DailyCostUSD float64
	// MonthlyCostUSD is the estimated cost incurred this month.
	MonthlyCostUSD float64
}

// BudgetChecker evaluates whether a tenant is within budget.
type BudgetChecker interface {
	// Check returns:
	//   allowed=false, err=ErrBudgetExceeded if HardStop is true and any limit
	//   is exceeded.
	//   alert=true if any limit is above its AlertThresholdPct.
	//   Otherwise allowed=true, alert=false, err=nil.
	Check(ctx context.Context, tenantID uuid.UUID, usage TokenUsage) (allowed bool, alert bool, err error)
}

// periodUsage tracks aggregated token and cost totals for a single period bucket.
type periodUsage struct {
	tokens  int
	costUSD float64
}

// tenantState holds per-period aggregates for a single tenant.
type tenantState struct {
	mu      sync.RWMutex
	daily   map[string]*periodUsage // key: "YYYY-MM-DD"
	monthly map[string]*periodUsage // key: "YYYY-MM"
}

func newTenantState() *tenantState {
	return &tenantState{
		daily:   make(map[string]*periodUsage),
		monthly: make(map[string]*periodUsage),
	}
}

// InMemoryBudgetChecker is a BudgetChecker that keeps live usage totals in
// memory.  It is safe for concurrent use.
type InMemoryBudgetChecker struct {
	mu      sync.RWMutex
	configs map[uuid.UUID]BudgetConfig
	states  map[uuid.UUID]*tenantState
}

// NewInMemoryBudgetChecker constructs an InMemoryBudgetChecker with the given
// BudgetConfig list.  It is not required that every tenant has a config; tenants
// without a config are always allowed through without alerts.
func NewInMemoryBudgetChecker(configs []BudgetConfig) *InMemoryBudgetChecker {
	c := &InMemoryBudgetChecker{
		configs: make(map[uuid.UUID]BudgetConfig, len(configs)),
		states:  make(map[uuid.UUID]*tenantState),
	}
	for _, cfg := range configs {
		c.configs[cfg.TenantID] = cfg
	}
	return c
}

// SetConfig upserts a BudgetConfig for a tenant at runtime.
func (c *InMemoryBudgetChecker) SetConfig(cfg BudgetConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.configs[cfg.TenantID] = cfg
}

// RecordUsage adds token and cost deltas to a tenant's in-memory state.
// It is called by AccountingService after every successful LLM call.
func (c *InMemoryBudgetChecker) RecordUsage(tenantID uuid.UUID, tokens int, costUSD float64, at time.Time) {
	dayKey := at.UTC().Format("2006-01-02")
	monthKey := at.UTC().Format("2006-01")

	c.mu.Lock()
	st, ok := c.states[tenantID]
	if !ok {
		st = newTenantState()
		c.states[tenantID] = st
	}
	c.mu.Unlock()

	st.mu.Lock()
	defer st.mu.Unlock()

	if _, ok := st.daily[dayKey]; !ok {
		st.daily[dayKey] = &periodUsage{}
	}
	st.daily[dayKey].tokens += tokens
	st.daily[dayKey].costUSD += costUSD

	if _, ok := st.monthly[monthKey]; !ok {
		st.monthly[monthKey] = &periodUsage{}
	}
	st.monthly[monthKey].tokens += tokens
	st.monthly[monthKey].costUSD += costUSD
}

// CurrentUsage returns the live usage totals for a tenant at the given time.
func (c *InMemoryBudgetChecker) CurrentUsage(tenantID uuid.UUID, at time.Time) TokenUsage {
	dayKey := at.UTC().Format("2006-01-02")
	monthKey := at.UTC().Format("2006-01")

	c.mu.RLock()
	st, ok := c.states[tenantID]
	c.mu.RUnlock()
	if !ok {
		return TokenUsage{}
	}

	st.mu.RLock()
	defer st.mu.RUnlock()

	var u TokenUsage
	if d, ok := st.daily[dayKey]; ok {
		u.DailyTokens = d.tokens
		u.DailyCostUSD = d.costUSD
	}
	if m, ok := st.monthly[monthKey]; ok {
		u.MonthlyTokens = m.tokens
		u.MonthlyCostUSD = m.costUSD
	}
	return u
}

// ResetState replaces all in-memory state with the provided aggregates.
// Used by ReconcileJob after a full re-aggregation pass.
func (c *InMemoryBudgetChecker) ResetState(tenantID uuid.UUID, daily map[string]periodUsage, monthly map[string]periodUsage) {
	c.mu.Lock()
	st := newTenantState()
	for k, v := range daily {
		v2 := v
		st.daily[k] = &v2
	}
	for k, v := range monthly {
		v2 := v
		st.monthly[k] = &v2
	}
	c.states[tenantID] = st
	c.mu.Unlock()
}

// Check implements BudgetChecker.
func (c *InMemoryBudgetChecker) Check(ctx context.Context, tenantID uuid.UUID, usage TokenUsage) (allowed bool, alert bool, err error) {
	c.mu.RLock()
	cfg, hasCfg := c.configs[tenantID]
	c.mu.RUnlock()

	if !hasCfg {
		return true, false, nil
	}

	exceeded, alertNeeded := evalLimits(cfg, usage)

	if exceeded && cfg.HardStop {
		return false, true, fmt.Errorf("%w for tenant %s", ErrBudgetExceeded, tenantID)
	}
	if exceeded {
		// Soft overage: allow but flag alert
		return true, true, nil
	}
	return true, alertNeeded, nil
}

// evalLimits returns (exceeded, alertNeeded) based on a BudgetConfig and current usage.
func evalLimits(cfg BudgetConfig, u TokenUsage) (exceeded bool, alertNeeded bool) {
	thresh := cfg.AlertThresholdPct
	if thresh <= 0 || thresh > 100 {
		thresh = 100
	}

	check := func(current, limit int, currentCost, limitCost float64) {
		if limit > 0 {
			pct := float64(current) / float64(limit) * 100
			if pct >= 100 {
				exceeded = true
			} else if pct >= thresh {
				alertNeeded = true
			}
		}
		if limitCost > 0 {
			pct := currentCost / limitCost * 100
			if pct >= 100 {
				exceeded = true
			} else if pct >= thresh {
				alertNeeded = true
			}
		}
	}

	dailyTokenLimit := 0
	if cfg.DailyTokenLimit != nil {
		dailyTokenLimit = *cfg.DailyTokenLimit
	}
	monthlyTokenLimit := 0
	if cfg.MonthlyTokenLimit != nil {
		monthlyTokenLimit = *cfg.MonthlyTokenLimit
	}
	dailyCostLimit := 0.0
	if cfg.DailyCostLimitUSD != nil {
		dailyCostLimit = *cfg.DailyCostLimitUSD
	}
	monthlyCostLimit := 0.0
	if cfg.MonthlyCostLimitUSD != nil {
		monthlyCostLimit = *cfg.MonthlyCostLimitUSD
	}

	check(u.DailyTokens, dailyTokenLimit, u.DailyCostUSD, dailyCostLimit)
	check(u.MonthlyTokens, monthlyTokenLimit, u.MonthlyCostUSD, monthlyCostLimit)
	return exceeded, alertNeeded
}
