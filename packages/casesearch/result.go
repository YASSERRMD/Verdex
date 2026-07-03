package casesearch

import (
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
)

// Result is one ranked case-search hit: a case plus the best-matching
// content within it.
type Result struct {
	// CaseID identifies the matched case.
	CaseID uuid.UUID

	// Title, Reference, CategoryID, JurisdictionID, State, and CreatedAt
	// are copied from the matched caselifecycle.Case at search time.
	Title          string
	Reference      string
	CategoryID     string
	JurisdictionID uuid.UUID
	State          string
	CreatedAt      time.Time

	// Mode is the concrete Mode this result was produced by (ModeAuto is
	// never returned here — it is always resolved to a concrete mode
	// before matching, see Mode.resolve).
	Mode Mode

	// Score is the case-level relevance score: the best (highest) Hit
	// score among this case's matches, or 1.0 for a filter-only match
	// with no content-level hits. Results are sorted by descending Score.
	Score float64

	// Snippet is a short, highlighted excerpt of the best-matching hit's
	// text, built by ExtractSnippet. Empty for a filter-only match with
	// no Text query.
	Snippet string

	// Hits is every content match Search kept for this case (capped at
	// Query.TopKPerCase), sorted by descending Score.
	Hits []Hit
}

// Results is the outcome of Engine.Search: every matched case, ranked and
// paginated, plus bookkeeping about how the search was bounded.
type Results struct {
	// Items is this page of ranked Result values.
	Items []Result

	// TotalMatches is the total number of matched cases across every
	// page, before pagination.
	TotalMatches int

	// Page echoes the normalized Page this response corresponds to.
	Page Page

	// Mode is the concrete Mode Search resolved Query.Mode to.
	Mode Mode

	// SkippedCases is how many candidate cases were excluded from
	// content matching because their CaseSearcherResolver call failed
	// (see CaseSearcherResolver's per-case error tolerance). This is
	// exposed so callers/UIs can surface "N cases could not be searched"
	// rather than silently under-reporting matches.
	SkippedCases int
}

// resultFromCase builds a Result shell (no Hits/Score/Snippet yet) from a
// caselifecycle.Case.
func resultFromCase(c *caselifecycle.Case, mode Mode) Result {
	return Result{
		CaseID:         c.ID,
		Title:          c.Title,
		Reference:      c.Reference,
		CategoryID:     c.CategoryID,
		JurisdictionID: c.JurisdictionID,
		State:          string(c.State),
		CreatedAt:      c.CreatedAt,
		Mode:           mode,
	}
}
