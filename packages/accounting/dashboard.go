package accounting

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// ProviderSummary aggregates token usage for a single LLM provider within a
// tenant's dashboard view.
type ProviderSummary struct {
	ProviderID        string  `json:"provider_id"`
	TotalInputTokens  int     `json:"total_input_tokens"`
	TotalOutputTokens int     `json:"total_output_tokens"`
	TotalTokens       int     `json:"total_tokens"`
	EstimatedCostUSD  float64 `json:"estimated_cost_usd"`
	RequestCount      int     `json:"request_count"`
}

// TaskSummary aggregates token usage for a single task type within a tenant's
// dashboard view.
type TaskSummary struct {
	TaskType          string  `json:"task_type"`
	TotalInputTokens  int     `json:"total_input_tokens"`
	TotalOutputTokens int     `json:"total_output_tokens"`
	TotalTokens       int     `json:"total_tokens"`
	EstimatedCostUSD  float64 `json:"estimated_cost_usd"`
	RequestCount      int     `json:"request_count"`
}

// DailyTrend is the token and cost totals for a single calendar day.
type DailyTrend struct {
	Date              string  `json:"date"` // YYYY-MM-DD
	TotalTokens       int     `json:"total_tokens"`
	EstimatedCostUSD  float64 `json:"estimated_cost_usd"`
	RequestCount      int     `json:"request_count"`
}

// TenantDashboard is a JSON-serialisable view of a tenant's usage,
// broken down by provider, task type, and daily trend.
type TenantDashboard struct {
	TenantID         uuid.UUID         `json:"tenant_id"`
	GeneratedAt      time.Time         `json:"generated_at"`
	ByProvider       []ProviderSummary `json:"by_provider"`
	ByTaskType       []TaskSummary     `json:"by_task_type"`
	Last7DaysTrend   []DailyTrend      `json:"last_7_days_trend"`
	TotalTokens      int               `json:"total_tokens"`
	EstimatedCostUSD float64           `json:"estimated_cost_usd"`
	RequestCount     int               `json:"request_count"`
}

// fullRecordLister is a subset of InMemoryRepository used by DashboardAPI.
type fullRecordLister interface {
	AllRecords(ctx context.Context) ([]UsageRecord, error)
}

// DashboardAPI generates dashboard views from the accounting record store.
type DashboardAPI struct {
	fetcher fullRecordLister
}

// NewDashboardAPI constructs a DashboardAPI backed by the given record fetcher.
func NewDashboardAPI(fetcher fullRecordLister) *DashboardAPI {
	return &DashboardAPI{fetcher: fetcher}
}

// GetTenantDashboard returns a TenantDashboard for the given tenant.
// It scans all stored records and is O(n) in the total record count.
func (d *DashboardAPI) GetTenantDashboard(ctx context.Context, tenantID uuid.UUID) (*TenantDashboard, error) {
	records, err := d.fetcher.AllRecords(ctx)
	if err != nil {
		return nil, fmt.Errorf("dashboard: fetch records: %w", err)
	}

	now := time.Now().UTC()
	cutoff := now.AddDate(0, 0, -6).Truncate(24 * time.Hour) // 7 days inclusive

	providerMap := make(map[string]*ProviderSummary)
	taskMap := make(map[string]*TaskSummary)
	trendMap := make(map[string]*DailyTrend)

	dash := &TenantDashboard{
		TenantID:    tenantID,
		GeneratedAt: now,
	}

	for _, r := range records {
		if r.TenantID != tenantID {
			continue
		}
		cost := 0.0
		if r.CostUSD != nil {
			cost = *r.CostUSD
		}

		// Overall totals.
		dash.TotalTokens += r.TotalTokens
		dash.EstimatedCostUSD += cost
		dash.RequestCount++

		// By provider.
		ps, ok := providerMap[r.ProviderID]
		if !ok {
			ps = &ProviderSummary{ProviderID: r.ProviderID}
			providerMap[r.ProviderID] = ps
		}
		ps.TotalInputTokens += r.InputTokens
		ps.TotalOutputTokens += r.OutputTokens
		ps.TotalTokens += r.TotalTokens
		ps.EstimatedCostUSD += cost
		ps.RequestCount++

		// By task type.
		ts, ok := taskMap[r.TaskType]
		if !ok {
			ts = &TaskSummary{TaskType: r.TaskType}
			taskMap[r.TaskType] = ts
		}
		ts.TotalInputTokens += r.InputTokens
		ts.TotalOutputTokens += r.OutputTokens
		ts.TotalTokens += r.TotalTokens
		ts.EstimatedCostUSD += cost
		ts.RequestCount++

		// 7-day trend.
		day := r.CreatedAt.UTC().Truncate(24 * time.Hour)
		if !day.Before(cutoff) {
			dayKey := day.Format("2006-01-02")
			dt, ok := trendMap[dayKey]
			if !ok {
				dt = &DailyTrend{Date: dayKey}
				trendMap[dayKey] = dt
			}
			dt.TotalTokens += r.TotalTokens
			dt.EstimatedCostUSD += cost
			dt.RequestCount++
		}
	}

	// Flatten provider map into a sorted slice.
	for _, ps := range providerMap {
		dash.ByProvider = append(dash.ByProvider, *ps)
	}
	sort.Slice(dash.ByProvider, func(i, j int) bool {
		return dash.ByProvider[i].ProviderID < dash.ByProvider[j].ProviderID
	})

	// Flatten task map.
	for _, ts := range taskMap {
		dash.ByTaskType = append(dash.ByTaskType, *ts)
	}
	sort.Slice(dash.ByTaskType, func(i, j int) bool {
		return dash.ByTaskType[i].TaskType < dash.ByTaskType[j].TaskType
	})

	// Build 7-day trend (fill gaps with zero entries).
	for i := 6; i >= 0; i-- {
		dayStr := now.AddDate(0, 0, -i).UTC().Format("2006-01-02")
		if dt, ok := trendMap[dayStr]; ok {
			dash.Last7DaysTrend = append(dash.Last7DaysTrend, *dt)
		} else {
			dash.Last7DaysTrend = append(dash.Last7DaysTrend, DailyTrend{Date: dayStr})
		}
	}

	return dash, nil
}
