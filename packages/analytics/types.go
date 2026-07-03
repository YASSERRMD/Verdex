package analytics

import (
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
)

// Filters narrows Aggregate to a subset of a tenant's cases. Zero-value
// fields are treated as "no filter on this field", mirroring
// caselifecycle.CaseFilter and casesearch.Filter's "empty means
// unrestricted" convention.
type Filters struct {
	// JurisdictionID, if non-nil, restricts aggregation to cases under
	// this jurisdiction.
	JurisdictionID uuid.UUID

	// CategoryID, if non-empty, restricts aggregation to cases with
	// this category.
	CategoryID string

	// State, if non-empty, restricts aggregation to cases in this
	// lifecycle state.
	State caselifecycle.State
}

// toCaseFilter converts f into the caselifecycle.CaseFilter Aggregate
// passes to Repository.List.
func (f Filters) toCaseFilter() caselifecycle.CaseFilter {
	return caselifecycle.CaseFilter{
		State:          f.State,
		JurisdictionID: f.JurisdictionID,
		CategoryID:     f.CategoryID,
	}
}

// StateCount is the number of cases in a single caselifecycle.State.
type StateCount struct {
	State caselifecycle.State `json:"state"`
	Count int                 `json:"count"`
}

// CategoryCount is the number of cases classified under a single
// category. CategoryID matches caselifecycle.Case.CategoryID
// (packages/category taxonomy code); an empty CategoryID groups cases
// that have not yet been categorized.
type CategoryCount struct {
	CategoryID string `json:"category_id"`
	Count      int    `json:"count"`
}

// JurisdictionBreakdown is the caseload for a single jurisdiction,
// further broken down by state so callers can see, e.g., "12 active,
// 3 under review" per jurisdiction without a second query.
type JurisdictionBreakdown struct {
	JurisdictionID uuid.UUID `json:"jurisdiction_id"`
	Count          int       `json:"count"`

	// ByState is the JurisdictionBreakdown's cases grouped by
	// caselifecycle.State, sorted by State ascending.
	ByState []StateCount `json:"by_state"`
}

// DailyCaseCount is the number of cases created on a single calendar
// day (UTC), mirroring accounting.DailyTrend's shape for consistency
// across this package's and accounting's dashboard payloads.
type DailyCaseCount struct {
	Date  string `json:"date"` // YYYY-MM-DD
	Count int    `json:"count"`
}

// Metrics is the aggregated caseload view Aggregate returns: total
// count plus breakdowns by state, category, jurisdiction, and a
// creation-date trend. This is newly computed by this package (see
// doc.go) — it does not wrap any other package's type.
type Metrics struct {
	// TenantID is the tenant these metrics are scoped to.
	TenantID uuid.UUID `json:"tenant_id"`

	// GeneratedAt is when Aggregate computed this Metrics.
	GeneratedAt time.Time `json:"generated_at"`

	// Filters is the Filters Aggregate was called with, echoed back so
	// callers/exports can record what was requested.
	Filters Filters `json:"filters"`

	// TotalCases is the total number of cases matching Filters.
	TotalCases int `json:"total_cases"`

	// ByState is TotalCases broken down by caselifecycle.State, sorted
	// by State ascending.
	ByState []StateCount `json:"by_state"`

	// ByCategory is TotalCases broken down by category, sorted by
	// CategoryID ascending.
	ByCategory []CategoryCount `json:"by_category"`

	// ByJurisdiction is TotalCases broken down by jurisdiction, sorted
	// by JurisdictionID string form ascending.
	ByJurisdiction []JurisdictionBreakdown `json:"by_jurisdiction"`

	// CreatedTrend is TotalCases broken down by creation date, oldest
	// first, covering every distinct day present in the matching
	// cases (not bucketed to a fixed trailing window, unlike
	// accounting's fixed 7-day trend, since case creation is much
	// lower-volume than token usage).
	CreatedTrend []DailyCaseCount `json:"created_trend"`
}
