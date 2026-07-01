package accounting

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// UsageSummary is an aggregated view of token consumption for either a single
// case or an entire tenant over a named period.
type UsageSummary struct {
	// TenantID is the tenant these totals belong to.
	TenantID uuid.UUID `json:"tenant_id"`
	// CaseID is non-nil when the summary is scoped to a specific case.
	CaseID *uuid.UUID `json:"case_id,omitempty"`
	// Period is a human-readable label such as "2024-01" or "2024-01-15".
	Period string `json:"period"`
	// TotalInputTokens is the sum of all InputTokens in the aggregated records.
	TotalInputTokens int `json:"total_input_tokens"`
	// TotalOutputTokens is the sum of all OutputTokens in the aggregated records.
	TotalOutputTokens int `json:"total_output_tokens"`
	// TotalTokens is the sum of all TotalTokens in the aggregated records.
	TotalTokens int `json:"total_tokens"`
	// EstimatedCostUSD is the sum of all non-nil CostUSD values.
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
	// RequestCount is the number of UsageRecord instances that were aggregated.
	RequestCount int `json:"request_count"`
}

// periodLabel returns a period label string for the given time and granularity.
// granularity must be "daily" or "monthly".
func periodLabel(t time.Time, granularity string) string {
	switch granularity {
	case "monthly":
		return t.UTC().Format("2006-01")
	default:
		return t.UTC().Format("2006-01-02")
	}
}

// AggregateByCasePeriod groups records by (CaseID, daily period) and returns
// one UsageSummary per group.  Records without a CaseID are skipped.
func AggregateByCasePeriod(records []UsageRecord) []UsageSummary {
	type key struct {
		caseID uuid.UUID
		period string
	}
	buckets := make(map[key]*UsageSummary)
	order := make([]key, 0)

	for _, r := range records {
		if r.CaseID == nil {
			continue
		}
		k := key{caseID: *r.CaseID, period: periodLabel(r.CreatedAt, "daily")}
		s, ok := buckets[k]
		if !ok {
			cid := *r.CaseID
			s = &UsageSummary{
				TenantID: r.TenantID,
				CaseID:   &cid,
				Period:   k.period,
			}
			buckets[k] = s
			order = append(order, k)
		}
		s.TotalInputTokens += r.InputTokens
		s.TotalOutputTokens += r.OutputTokens
		s.TotalTokens += r.TotalTokens
		if r.CostUSD != nil {
			s.EstimatedCostUSD += *r.CostUSD
		}
		s.RequestCount++
	}

	result := make([]UsageSummary, 0, len(order))
	for _, k := range order {
		result = append(result, *buckets[k])
	}
	return result
}

// AggregateByTenantPeriod groups records by (TenantID, period) using the
// provided granularity ("daily" or "monthly") and returns one UsageSummary
// per group.
func AggregateByTenantPeriod(records []UsageRecord, granularity string) ([]UsageSummary, error) {
	if granularity != "daily" && granularity != "monthly" {
		return nil, fmt.Errorf("%w: %q", ErrInvalidPeriod, granularity)
	}

	type key struct {
		tenantID uuid.UUID
		period   string
	}
	buckets := make(map[key]*UsageSummary)
	order := make([]key, 0)

	for _, r := range records {
		k := key{tenantID: r.TenantID, period: periodLabel(r.CreatedAt, granularity)}
		s, ok := buckets[k]
		if !ok {
			s = &UsageSummary{
				TenantID: r.TenantID,
				Period:   k.period,
			}
			buckets[k] = s
			order = append(order, k)
		}
		s.TotalInputTokens += r.InputTokens
		s.TotalOutputTokens += r.OutputTokens
		s.TotalTokens += r.TotalTokens
		if r.CostUSD != nil {
			s.EstimatedCostUSD += *r.CostUSD
		}
		s.RequestCount++
	}

	result := make([]UsageSummary, 0, len(order))
	for _, k := range order {
		result = append(result, *buckets[k])
	}
	return result, nil
}
