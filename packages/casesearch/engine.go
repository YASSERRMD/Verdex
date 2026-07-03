package casesearch

import (
	"context"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
)

// Engine is casesearch's main entry point: Search composes
// caselifecycle.Repository (cross-case metadata + filtering) with a
// CaseSearcherResolver (per-case content search) to answer a Query. See
// doc.go for the full composition contract.
//
// Engine is safe for concurrent use if its Repository and resolver are.
type Engine struct {
	cases    caselifecycle.Repository
	resolver CaseSearcherResolver

	// partyLookup resolves party names for Filter.PartyName. Nil means a
	// Query with a non-empty PartyName matches no cases (see Filter's doc
	// comment).
	partyLookup PartyLookup
}

// NewEngine constructs an Engine over cases and resolver. Returns
// ErrNilRepository if cases is nil, or ErrNilResolver if resolver is
// nil.
func NewEngine(cases caselifecycle.Repository, resolver CaseSearcherResolver) (*Engine, error) {
	if cases == nil {
		return nil, ErrNilRepository
	}
	if resolver == nil {
		return nil, ErrNilResolver
	}
	return &Engine{cases: cases, resolver: resolver}, nil
}

// WithPartyLookup returns a shallow copy of e configured to resolve
// Filter.PartyName via lookup instead of the default (nil, which matches
// no cases for a party-filtered query).
func (e *Engine) WithPartyLookup(lookup PartyLookup) *Engine {
	clone := *e
	clone.partyLookup = lookup
	return &clone
}

// Search runs q against every case tenantID can see, applying Filter to
// narrow the candidate set (via caselifecycle.Repository.List and, for
// PartyName, the configured PartyLookup) and then content-matching each
// candidate case via the CaseSearcherResolver in the mode Query.Mode
// resolves to.
//
// Access scoping: every result is drawn from caselifecycle.Repository.
// List(ctx, tenantID, ...), which itself refuses to return cases outside
// tenantID (see caselifecycle.Repository's tenant-isolation contract);
// Search additionally requires ctx to carry an authenticated
// identity.User holding identity.PermViewCase (authorize, access.go), so
// a request with no viewing permission is rejected before any repository
// call. A tenantID mismatched against the authenticated user is the
// caller's bug, not something Search can detect from ctx alone — callers
// (an HTTP handler resolving tenantID from the authenticated session) are
// responsible for never passing a tenantID other than the caller's own.
//
// Returns ErrEmptyTenantID if tenantID is uuid.Nil, ErrUnauthenticated/
// ErrForbidden per authorize, or ErrEmptyQuery/ErrInvalidMode/
// ErrInvalidPage if q fails validation.
func (e *Engine) Search(ctx context.Context, tenantID uuid.UUID, q Query) (Results, error) {
	if err := authorize(ctx); err != nil {
		return Results{}, err
	}
	if tenantID == uuid.Nil {
		return Results{}, ErrEmptyTenantID
	}
	if err := q.validate(); err != nil {
		return Results{}, err
	}
	page, err := q.Page.normalize()
	if err != nil {
		return Results{}, err
	}

	mode := q.Mode.resolve(q)
	topKPerCase := q.TopKPerCase
	if topKPerCase <= 0 {
		topKPerCase = DefaultTopKPerCase
	}

	candidates, err := e.cases.List(ctx, tenantID, caseFilterFromQuery(q))
	if err != nil {
		return Results{}, err
	}

	candidates, err = e.applyPartyFilter(ctx, candidates, q.Filter.PartyName)
	if err != nil {
		return Results{}, err
	}
	candidates = applyDateFilter(candidates, q.Filter)

	results := make([]Result, 0, len(candidates))
	skipped := 0
	hasContentQuery := strings.TrimSpace(q.Text) != "" || strings.TrimSpace(q.IssueOrRuleID) != ""

	for _, c := range candidates {
		result := resultFromCase(c, mode)

		if !hasContentQuery {
			// Filter-only search: every candidate that survived List +
			// party/date filtering is a match, ranked by recency.
			result.Score = 1.0
			results = append(results, result)
			continue
		}

		searcher, serr := e.resolver(ctx, c.ID.String())
		if serr != nil || searcher == nil {
			skipped++
			continue
		}

		hits, herr := searchCase(ctx, searcher, mode, q, topKPerCase)
		if herr != nil {
			skipped++
			continue
		}
		if len(hits) == 0 {
			continue
		}

		sortHitsByScore(hits)
		result.Hits = hits
		result.Score = hits[0].Score
		result.Snippet = ExtractSnippet(hits[0].Text, q.Text)
		results = append(results, result)
	}

	rankResults(results)

	total := len(results)
	pageItems := paginateResults(results, page)

	return Results{
		Items:        pageItems,
		TotalMatches: total,
		Page:         page,
		Mode:         mode,
		SkippedCases: skipped,
	}, nil
}

// searchCase dispatches to the CaseSearcher method matching mode.
func searchCase(ctx context.Context, searcher CaseSearcher, mode Mode, q Query, topK int) ([]Hit, error) {
	switch mode {
	case ModeKeyword:
		return searcher.SearchKeyword(ctx, q.Text, topK)
	case ModeSemantic:
		return searcher.SearchSemantic(ctx, q.Text, topK)
	case ModeIssueRule:
		return searcher.SearchIssueOrRule(ctx, q.IssueOrRuleID, topK)
	default:
		return nil, ErrInvalidMode
	}
}

// caseFilterFromQuery projects Query.Filter's caselifecycle-native fields
// (category, jurisdiction, state) into a caselifecycle.CaseFilter. Party
// and date filtering are applied separately (applyPartyFilter,
// applyDateFilter) since caselifecycle.CaseFilter does not model them.
func caseFilterFromQuery(q Query) caselifecycle.CaseFilter {
	return caselifecycle.CaseFilter{
		State:          caselifecycle.State(q.Filter.State),
		JurisdictionID: q.Filter.JurisdictionID,
		CategoryID:     q.Filter.CategoryCode,
	}
}

// applyPartyFilter narrows cases to those with a party matching
// filter.PartyName, using e.partyLookup. An empty PartyName is a no-op.
// A nil partyLookup with a non-empty PartyName narrows to zero cases
// (see Filter's doc comment on PartyName).
func (e *Engine) applyPartyFilter(ctx context.Context, cases []*caselifecycle.Case, partyName string) ([]*caselifecycle.Case, error) {
	if strings.TrimSpace(partyName) == "" {
		return cases, nil
	}
	if e.partyLookup == nil {
		return nil, nil
	}

	out := make([]*caselifecycle.Case, 0, len(cases))
	for _, c := range cases {
		names, err := e.partyLookup(ctx, c.ID.String())
		if err != nil {
			// A case whose parties cannot be resolved is excluded from a
			// party-filtered search rather than failing the whole
			// request, mirroring PartyLookup's documented per-case error
			// tolerance.
			continue
		}
		if matchesPartyName(names, partyName) {
			out = append(out, c)
		}
	}
	return out, nil
}

// applyDateFilter narrows cases to those whose CreatedAt falls within
// [filter.DateFrom, filter.DateTo] (either bound may be zero/unset).
func applyDateFilter(cases []*caselifecycle.Case, filter Filter) []*caselifecycle.Case {
	if filter.DateFrom.IsZero() && filter.DateTo.IsZero() {
		return cases
	}
	out := make([]*caselifecycle.Case, 0, len(cases))
	for _, c := range cases {
		if !filter.DateFrom.IsZero() && c.CreatedAt.Before(filter.DateFrom) {
			continue
		}
		if !filter.DateTo.IsZero() && c.CreatedAt.After(filter.DateTo) {
			continue
		}
		out = append(out, c)
	}
	return out
}

// rankResults sorts results by descending Score, breaking ties by
// descending CreatedAt (most recent first) for determinism. See rank.go
// for the full ranking write-up.
func rankResults(results []Result) {
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})
}

// paginateResults slices results per page's 1-based Number/Size.
func paginateResults(results []Result, page Page) []Result {
	start := (page.Number - 1) * page.Size
	if start >= len(results) {
		return []Result{}
	}
	end := start + page.Size
	if end > len(results) {
		end = len(results)
	}
	return results[start:end]
}
