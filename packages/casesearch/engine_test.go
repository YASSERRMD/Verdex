package casesearch_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/casesearch"
	"github.com/YASSERRMD/verdex/packages/identity"
)

func TestNewEngine_RejectsNilDependencies(t *testing.T) {
	repo := caselifecycle.NewInMemoryRepository()
	resolver := func(_ context.Context, _ string) (casesearch.CaseSearcher, error) {
		return &fakeSearcher{}, nil
	}

	if _, err := casesearch.NewEngine(nil, resolver); !errors.Is(err, casesearch.ErrNilRepository) {
		t.Fatalf("expected ErrNilRepository, got %v", err)
	}
	if _, err := casesearch.NewEngine(repo, nil); !errors.Is(err, casesearch.ErrNilResolver) {
		t.Fatalf("expected ErrNilResolver, got %v", err)
	}
}

func TestEngine_Search_RequiresAuthentication(t *testing.T) {
	repo := caselifecycle.NewInMemoryRepository()
	engine, err := casesearch.NewEngine(repo, func(context.Context, string) (casesearch.CaseSearcher, error) {
		return &fakeSearcher{}, nil
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	_, err = engine.Search(context.Background(), uuid.New(), casesearch.NewQuery("breach"))
	if !errors.Is(err, casesearch.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestEngine_Search_RejectsEmptyTenantID(t *testing.T) {
	repo := caselifecycle.NewInMemoryRepository()
	engine, _ := casesearch.NewEngine(repo, func(context.Context, string) (casesearch.CaseSearcher, error) {
		return &fakeSearcher{}, nil
	})

	ctx := ctxWithUser(newTestUser(uuid.New(), identity.RoleJudge))
	_, err := engine.Search(ctx, uuid.Nil, casesearch.NewQuery("breach"))
	if !errors.Is(err, casesearch.ErrEmptyTenantID) {
		t.Fatalf("expected ErrEmptyTenantID, got %v", err)
	}
}

func TestEngine_Search_RejectsEmptyQuery(t *testing.T) {
	repo := caselifecycle.NewInMemoryRepository()
	engine, _ := casesearch.NewEngine(repo, func(context.Context, string) (casesearch.CaseSearcher, error) {
		return &fakeSearcher{}, nil
	})

	tenantID := uuid.New()
	ctx := ctxWithUser(newTestUser(tenantID, identity.RoleJudge))

	_, err := engine.Search(ctx, tenantID, casesearch.NewQuery(""))
	if !errors.Is(err, casesearch.ErrEmptyQuery) {
		t.Fatalf("expected ErrEmptyQuery, got %v", err)
	}
}

func TestEngine_Search_KeywordMode_MatchesOnContent(t *testing.T) {
	repo := caselifecycle.NewInMemoryRepository()
	tenantID := uuid.New()
	jurisdictionID := uuid.New()

	matching := seedCase(t, repo, tenantID, jurisdictionID, "civil", "Doe v. Acme Corp")
	nonMatching := seedCase(t, repo, tenantID, jurisdictionID, "civil", "Roe v. Beta LLC")

	resolver := func(_ context.Context, caseID string) (casesearch.CaseSearcher, error) {
		if caseID == matching.ID.String() {
			return &fakeSearcher{keywordHits: []casesearch.Hit{
				{NodeID: "n1", Text: "The tenant breached the lease agreement.", Score: 0.9},
			}}, nil
		}
		if caseID == nonMatching.ID.String() {
			return &fakeSearcher{keywordHits: nil}, nil
		}
		return nil, errors.New("unexpected case")
	}

	engine, err := casesearch.NewEngine(repo, resolver)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	ctx := ctxWithUser(newTestUser(tenantID, identity.RoleJudge))
	results, err := engine.Search(ctx, tenantID, casesearch.NewQuery("breached").WithMode(casesearch.ModeKeyword))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results.Items) != 1 {
		t.Fatalf("expected 1 result, got %d: %+v", len(results.Items), results.Items)
	}
	if results.Items[0].CaseID != matching.ID {
		t.Fatalf("expected matching case %s, got %s", matching.ID, results.Items[0].CaseID)
	}
	if results.Items[0].Snippet == "" {
		t.Fatal("expected a non-empty snippet")
	}
	if results.Mode != casesearch.ModeKeyword {
		t.Fatalf("expected resolved Mode = keyword, got %q", results.Mode)
	}
}

func TestEngine_Search_SemanticMode_ComposesRealHybridRetrieval(t *testing.T) {
	// Full composition with the real hybridretrieval/treeindex stack is
	// covered in knowledgeapi_test.go's TestKnowledgeAPISearcher_
	// SearchSemantic_ComposesRealHybridRetrieval. This test exercises
	// Engine's dispatch to ModeSemantic against a fake searcher, keeping
	// the two concerns (Engine's cross-case orchestration vs.
	// KnowledgeAPISearcher's composition with the real retrieval stack)
	// independently testable.
	repo := caselifecycle.NewInMemoryRepository()
	tenantID := uuid.New()
	jurisdictionID := uuid.New()

	c := seedCase(t, repo, tenantID, jurisdictionID, "civil", "Doe v. Acme Corp")

	resolver := func(_ context.Context, _ string) (casesearch.CaseSearcher, error) {
		return &fakeSearcher{semanticHits: []casesearch.Hit{
			{NodeID: "n1", Text: "governing rule on landlord obligations", Score: 0.75},
		}}, nil
	}

	engine, _ := casesearch.NewEngine(repo, resolver)
	ctx := ctxWithUser(newTestUser(tenantID, identity.RoleJudge))

	results, err := engine.Search(ctx, tenantID, casesearch.NewQuery("landlord duties").WithMode(casesearch.ModeSemantic))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results.Items) != 1 || results.Items[0].CaseID != c.ID {
		t.Fatalf("expected 1 result for seeded case, got %+v", results.Items)
	}
}

func TestEngine_Search_FiltersNarrowResults(t *testing.T) {
	repo := caselifecycle.NewInMemoryRepository()
	tenantID := uuid.New()
	civilJurisdiction := uuid.New()
	criminalJurisdiction := uuid.New()

	civilCase := seedCase(t, repo, tenantID, civilJurisdiction, "civil", "Civil Matter")
	seedCase(t, repo, tenantID, criminalJurisdiction, "criminal", "Criminal Matter")

	resolver := func(_ context.Context, _ string) (casesearch.CaseSearcher, error) {
		return &fakeSearcher{keywordHits: []casesearch.Hit{{NodeID: "n1", Text: "dispute", Score: 1}}}, nil
	}
	engine, _ := casesearch.NewEngine(repo, resolver)
	ctx := ctxWithUser(newTestUser(tenantID, identity.RoleJudge))

	results, err := engine.Search(ctx, tenantID, casesearch.NewQuery("dispute").
		WithMode(casesearch.ModeKeyword).
		WithFilter(casesearch.Filter{CategoryCode: "civil"}))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results.Items) != 1 || results.Items[0].CaseID != civilCase.ID {
		t.Fatalf("expected only the civil case, got %+v", results.Items)
	}
}

func TestEngine_Search_PartyFilter_NarrowsToMatchingParty(t *testing.T) {
	repo := caselifecycle.NewInMemoryRepository()
	tenantID := uuid.New()
	jurisdictionID := uuid.New()

	acmeCase := seedCase(t, repo, tenantID, jurisdictionID, "civil", "Doe v. Acme Corp")
	betaCase := seedCase(t, repo, tenantID, jurisdictionID, "civil", "Roe v. Beta LLC")

	partyLookup := func(_ context.Context, caseID string) ([]string, error) {
		switch caseID {
		case acmeCase.ID.String():
			return []string{"Jane Doe", "Acme Corp"}, nil
		case betaCase.ID.String():
			return []string{"John Roe", "Beta LLC"}, nil
		}
		return nil, nil
	}

	resolver := func(_ context.Context, _ string) (casesearch.CaseSearcher, error) {
		return &fakeSearcher{keywordHits: []casesearch.Hit{{NodeID: "n1", Text: "matter", Score: 1}}}, nil
	}
	engine, _ := casesearch.NewEngine(repo, resolver)
	engine = engine.WithPartyLookup(partyLookup)

	ctx := ctxWithUser(newTestUser(tenantID, identity.RoleJudge))
	results, err := engine.Search(ctx, tenantID, casesearch.NewQuery("matter").
		WithMode(casesearch.ModeKeyword).
		WithFilter(casesearch.Filter{PartyName: "Acme"}))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results.Items) != 1 || results.Items[0].CaseID != acmeCase.ID {
		t.Fatalf("expected only the Acme case, got %+v", results.Items)
	}
}

func TestEngine_Search_PartyFilter_NoPartyLookupConfigured_MatchesNothing(t *testing.T) {
	repo := caselifecycle.NewInMemoryRepository()
	tenantID := uuid.New()
	seedCase(t, repo, tenantID, uuid.New(), "civil", "Doe v. Acme Corp")

	resolver := func(_ context.Context, _ string) (casesearch.CaseSearcher, error) {
		return &fakeSearcher{keywordHits: []casesearch.Hit{{NodeID: "n1", Text: "matter", Score: 1}}}, nil
	}
	engine, _ := casesearch.NewEngine(repo, resolver)
	ctx := ctxWithUser(newTestUser(tenantID, identity.RoleJudge))

	results, err := engine.Search(ctx, tenantID, casesearch.NewQuery("matter").
		WithMode(casesearch.ModeKeyword).
		WithFilter(casesearch.Filter{PartyName: "Acme"}))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results.Items) != 0 {
		t.Fatalf("expected 0 results with no PartyLookup configured, got %d", len(results.Items))
	}
}

func TestEngine_Search_IssueRuleMode_FindsCasesApplyingRule(t *testing.T) {
	repo := caselifecycle.NewInMemoryRepository()
	tenantID := uuid.New()
	jurisdictionID := uuid.New()

	applying := seedCase(t, repo, tenantID, jurisdictionID, "civil", "Applies Rule 5")
	notApplying := seedCase(t, repo, tenantID, jurisdictionID, "civil", "Does Not Apply Rule 5")

	resolver := func(_ context.Context, caseID string) (casesearch.CaseSearcher, error) {
		if caseID == applying.ID.String() {
			return &fakeSearcher{issueRuleHits: []casesearch.Hit{
				{NodeID: "app-1", NodeType: "application", Text: "applies rule 5 to the facts", Score: 1.0},
			}}, nil
		}
		return &fakeSearcher{issueRuleHits: nil}, nil
	}
	_ = notApplying

	engine, _ := casesearch.NewEngine(repo, resolver)
	ctx := ctxWithUser(newTestUser(tenantID, identity.RoleJudge))

	results, err := engine.Search(ctx, tenantID, casesearch.NewQuery("").WithIssueOrRule("rule-5"))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results.Items) != 1 || results.Items[0].CaseID != applying.ID {
		t.Fatalf("expected only the applying case, got %+v", results.Items)
	}
	if results.Mode != casesearch.ModeIssueRule {
		t.Fatalf("expected ModeAuto to resolve to ModeIssueRule, got %q", results.Mode)
	}
}

func TestEngine_Search_RankingOrdersByDescendingScore(t *testing.T) {
	repo := caselifecycle.NewInMemoryRepository()
	tenantID := uuid.New()
	jurisdictionID := uuid.New()

	low := seedCase(t, repo, tenantID, jurisdictionID, "civil", "Low Relevance")
	high := seedCase(t, repo, tenantID, jurisdictionID, "civil", "High Relevance")
	mid := seedCase(t, repo, tenantID, jurisdictionID, "civil", "Mid Relevance")

	scores := map[string]float64{
		low.ID.String():  0.2,
		high.ID.String(): 0.95,
		mid.ID.String():  0.5,
	}

	resolver := func(_ context.Context, caseID string) (casesearch.CaseSearcher, error) {
		return &fakeSearcher{keywordHits: []casesearch.Hit{
			{NodeID: "n1", Text: "term", Score: scores[caseID]},
		}}, nil
	}

	engine, _ := casesearch.NewEngine(repo, resolver)
	ctx := ctxWithUser(newTestUser(tenantID, identity.RoleJudge))

	results, err := engine.Search(ctx, tenantID, casesearch.NewQuery("term").WithMode(casesearch.ModeKeyword))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results.Items) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results.Items))
	}
	if results.Items[0].CaseID != high.ID || results.Items[1].CaseID != mid.ID || results.Items[2].CaseID != low.ID {
		t.Fatalf("expected descending score order high,mid,low; got %s,%s,%s",
			results.Items[0].Title, results.Items[1].Title, results.Items[2].Title)
	}
}

func TestEngine_Search_Pagination(t *testing.T) {
	repo := caselifecycle.NewInMemoryRepository()
	tenantID := uuid.New()
	jurisdictionID := uuid.New()

	for i := 0; i < 5; i++ {
		seedCase(t, repo, tenantID, jurisdictionID, "civil", "Case")
	}

	resolver := func(_ context.Context, _ string) (casesearch.CaseSearcher, error) {
		return &fakeSearcher{keywordHits: []casesearch.Hit{{NodeID: "n1", Text: "term", Score: 1}}}, nil
	}
	engine, _ := casesearch.NewEngine(repo, resolver)
	ctx := ctxWithUser(newTestUser(tenantID, identity.RoleJudge))

	page1, err := engine.Search(ctx, tenantID, casesearch.NewQuery("term").
		WithMode(casesearch.ModeKeyword).
		WithPage(casesearch.Page{Number: 1, Size: 2}))
	if err != nil {
		t.Fatalf("Search page 1: %v", err)
	}
	if len(page1.Items) != 2 {
		t.Fatalf("expected 2 items on page 1, got %d", len(page1.Items))
	}
	if page1.TotalMatches != 5 {
		t.Fatalf("expected TotalMatches = 5, got %d", page1.TotalMatches)
	}

	page3, err := engine.Search(ctx, tenantID, casesearch.NewQuery("term").
		WithMode(casesearch.ModeKeyword).
		WithPage(casesearch.Page{Number: 3, Size: 2}))
	if err != nil {
		t.Fatalf("Search page 3: %v", err)
	}
	if len(page3.Items) != 1 {
		t.Fatalf("expected 1 item on page 3 (5 total, size 2), got %d", len(page3.Items))
	}
}

func TestEngine_Search_TenantIsolation_BlocksCrossTenantResults(t *testing.T) {
	repo := caselifecycle.NewInMemoryRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()
	jurisdictionID := uuid.New()

	seedCase(t, repo, tenantA, jurisdictionID, "civil", "Tenant A Case")
	seedCase(t, repo, tenantB, jurisdictionID, "civil", "Tenant B Case")

	resolver := func(_ context.Context, _ string) (casesearch.CaseSearcher, error) {
		return &fakeSearcher{keywordHits: []casesearch.Hit{{NodeID: "n1", Text: "matter", Score: 1}}}, nil
	}
	engine, _ := casesearch.NewEngine(repo, resolver)

	ctx := ctxWithUser(newTestUser(tenantA, identity.RoleJudge))
	results, err := engine.Search(ctx, tenantA, casesearch.NewQuery("matter").WithMode(casesearch.ModeKeyword))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results.Items) != 1 {
		t.Fatalf("expected exactly 1 result scoped to tenant A, got %d", len(results.Items))
	}
	for _, item := range results.Items {
		if item.Title == "Tenant B Case" {
			t.Fatal("cross-tenant leakage: tenant A's search returned tenant B's case")
		}
	}
}

func TestEngine_Search_SkipsCasesWhoseResolverFails(t *testing.T) {
	repo := caselifecycle.NewInMemoryRepository()
	tenantID := uuid.New()
	jurisdictionID := uuid.New()

	broken := seedCase(t, repo, tenantID, jurisdictionID, "civil", "Not Yet Indexed")
	ok := seedCase(t, repo, tenantID, jurisdictionID, "civil", "Indexed")

	resolver := func(_ context.Context, caseID string) (casesearch.CaseSearcher, error) {
		if caseID == broken.ID.String() {
			return nil, errors.New("case not indexed")
		}
		return &fakeSearcher{keywordHits: []casesearch.Hit{{NodeID: "n1", Text: "matter", Score: 1}}}, nil
	}
	engine, _ := casesearch.NewEngine(repo, resolver)
	ctx := ctxWithUser(newTestUser(tenantID, identity.RoleJudge))

	results, err := engine.Search(ctx, tenantID, casesearch.NewQuery("matter").WithMode(casesearch.ModeKeyword))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results.Items) != 1 || results.Items[0].CaseID != ok.ID {
		t.Fatalf("expected only the indexed case, got %+v", results.Items)
	}
	if results.SkippedCases != 1 {
		t.Fatalf("expected SkippedCases = 1, got %d", results.SkippedCases)
	}
}

func TestEngine_Search_FilterOnly_NoTextQuery_ListsMatchingCases(t *testing.T) {
	repo := caselifecycle.NewInMemoryRepository()
	tenantID := uuid.New()
	jurisdictionID := uuid.New()

	civil := seedCase(t, repo, tenantID, jurisdictionID, "civil", "Civil Case")
	seedCase(t, repo, tenantID, jurisdictionID, "criminal", "Criminal Case")

	resolver := func(_ context.Context, _ string) (casesearch.CaseSearcher, error) {
		t.Fatal("resolver should not be called for a filter-only search")
		return nil, nil
	}
	engine, _ := casesearch.NewEngine(repo, resolver)
	ctx := ctxWithUser(newTestUser(tenantID, identity.RoleJudge))

	q := casesearch.Query{
		AllowEmptyText: true,
		Filter:         casesearch.Filter{CategoryCode: "civil"},
	}
	results, err := engine.Search(ctx, tenantID, q)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results.Items) != 1 || results.Items[0].CaseID != civil.ID {
		t.Fatalf("expected only the civil case, got %+v", results.Items)
	}
}
