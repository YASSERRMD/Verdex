package casesearch_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/casesearch"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// newTestUser builds an identity.User with the given permission-bearing
// role, scoped to tenantID, mirroring
// packages/caselifecycle's helpers_test.go newTestUser convention.
func newTestUser(tenantID uuid.UUID, roles ...identity.Role) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "reviewer@example.test",
		Name:     "Test User",
		Roles:    roles,
		Status:   identity.UserStatusActive,
	}
}

// ctxWithUser returns a context carrying user, mirroring how an HTTP
// middleware layer would attach the authenticated actor.
func ctxWithUser(user *identity.User) context.Context {
	return identity.WithUser(context.Background(), user)
}

// seedCase creates and persists a new draft case for tenantID via repo,
// with the given title/category/jurisdiction, returning the persisted
// Case.
func seedCase(t *testing.T, repo caselifecycle.Repository, tenantID, jurisdictionID uuid.UUID, category, title string) *caselifecycle.Case {
	t.Helper()

	c, err := caselifecycle.NewCase(caselifecycle.NewCaseInput{
		TenantID:       tenantID,
		JurisdictionID: jurisdictionID,
		CategoryID:     category,
		Title:          title,
		CreatedBy:      uuid.New(),
	})
	if err != nil {
		t.Fatalf("NewCase: %v", err)
	}
	if err := repo.Create(context.Background(), tenantID, c); err != nil {
		t.Fatalf("repo.Create: %v", err)
	}
	return c
}

// fakeSearcher is a hand-rolled casesearch.CaseSearcher for tests that
// don't need a full knowledgeapi.KnowledgeAPI fixture: it returns a
// caller-supplied fixed set of Hits per mode, letting engine_test.go
// exercise Engine.Search's ranking/filtering/pagination logic in
// isolation from the real retrieval stack (which knowledgeapi_test.go
// covers separately, composing the real hybridretrieval/treeindex
// types).
type fakeSearcher struct {
	keywordHits   []casesearch.Hit
	semanticHits  []casesearch.Hit
	issueRuleHits []casesearch.Hit
	err           error
}

func (f *fakeSearcher) SearchKeyword(_ context.Context, _ string, topK int) ([]casesearch.Hit, error) {
	if f.err != nil {
		return nil, f.err
	}
	return capHitsForTest(f.keywordHits, topK), nil
}

func (f *fakeSearcher) SearchSemantic(_ context.Context, _ string, topK int) ([]casesearch.Hit, error) {
	if f.err != nil {
		return nil, f.err
	}
	return capHitsForTest(f.semanticHits, topK), nil
}

func (f *fakeSearcher) SearchIssueOrRule(_ context.Context, _ string, topK int) ([]casesearch.Hit, error) {
	if f.err != nil {
		return nil, f.err
	}
	return capHitsForTest(f.issueRuleHits, topK), nil
}

func capHitsForTest(hits []casesearch.Hit, topK int) []casesearch.Hit {
	if topK <= 0 || len(hits) <= topK {
		return hits
	}
	return hits[:topK]
}

var _ casesearch.CaseSearcher = (*fakeSearcher)(nil)
