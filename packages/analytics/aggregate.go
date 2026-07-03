package analytics

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
)

// Aggregator computes Metrics over a tenant's caseload by reading
// through a caselifecycle.Repository, the same repository interface
// packages/casesearch's query engine reads through (List). Aggregator
// does not duplicate casesearch's text/semantic query construction; it
// only needs List's tenant+filter scoping, which caselifecycle.
// Repository already exposes directly.
type Aggregator struct {
	repo caselifecycle.Repository
}

// NewAggregator constructs an Aggregator backed by repo.
func NewAggregator(repo caselifecycle.Repository) *Aggregator {
	return &Aggregator{repo: repo}
}

// Aggregate computes Metrics for every case visible to tenantID
// matching filters. Requires ctx to carry an authenticated
// identity.User holding viewPermission — checked first via
// RequireViewPermission, so an unauthorized caller never triggers a
// repository read.
func (a *Aggregator) Aggregate(ctx context.Context, tenantID uuid.UUID, filters Filters) (*Metrics, error) {
	if err := RequireViewPermission(ctx); err != nil {
		return nil, err
	}
	if tenantID == uuid.Nil {
		return nil, ErrEmptyTenantID
	}

	cases, err := a.repo.List(ctx, tenantID, filters.toCaseFilter())
	if err != nil {
		return nil, err
	}

	m := &Metrics{
		TenantID:    tenantID,
		GeneratedAt: time.Now().UTC(),
		Filters:     filters,
		TotalCases:  len(cases),
	}

	byState := make(map[caselifecycle.State]int)
	byCategory := make(map[string]int)
	byJurisdiction := make(map[uuid.UUID]map[caselifecycle.State]int)
	byDay := make(map[string]int)

	for _, c := range cases {
		byState[c.State]++
		byCategory[c.CategoryID]++

		if byJurisdiction[c.JurisdictionID] == nil {
			byJurisdiction[c.JurisdictionID] = make(map[caselifecycle.State]int)
		}
		byJurisdiction[c.JurisdictionID][c.State]++

		day := c.CreatedAt.UTC().Format("2006-01-02")
		byDay[day]++
	}

	m.ByState = flattenStateCounts(byState)
	m.ByCategory = flattenCategoryCounts(byCategory)
	m.ByJurisdiction = flattenJurisdictionBreakdown(byJurisdiction)
	m.CreatedTrend = flattenDailyCounts(byDay)

	return m, nil
}

func flattenStateCounts(m map[caselifecycle.State]int) []StateCount {
	out := make([]StateCount, 0, len(m))
	for state, count := range m {
		out = append(out, StateCount{State: state, Count: count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].State < out[j].State })
	return out
}

func flattenCategoryCounts(m map[string]int) []CategoryCount {
	out := make([]CategoryCount, 0, len(m))
	for category, count := range m {
		out = append(out, CategoryCount{CategoryID: category, Count: count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CategoryID < out[j].CategoryID })
	return out
}

func flattenJurisdictionBreakdown(m map[uuid.UUID]map[caselifecycle.State]int) []JurisdictionBreakdown {
	out := make([]JurisdictionBreakdown, 0, len(m))
	for jurisdictionID, states := range m {
		total := 0
		for _, c := range states {
			total += c
		}
		out = append(out, JurisdictionBreakdown{
			JurisdictionID: jurisdictionID,
			Count:          total,
			ByState:        flattenStateCounts(states),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].JurisdictionID.String() < out[j].JurisdictionID.String()
	})
	return out
}

func flattenDailyCounts(m map[string]int) []DailyCaseCount {
	out := make([]DailyCaseCount, 0, len(m))
	for date, count := range m {
		out = append(out, DailyCaseCount{Date: date, Count: count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}
