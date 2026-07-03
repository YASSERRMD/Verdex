package grounding_test

import (
	"context"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/synthesisagent"
)

const testCaseID = "case-grounding"

func newTestUser(roles ...identity.Role) *identity.User {
	return &identity.User{
		Email:     "test@example.com",
		Name:      "Test User",
		Roles:     roles,
		Status:    identity.UserStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func authedContext() context.Context {
	return identity.WithUser(context.Background(), newTestUser(identity.RoleAdvocate))
}

func unauthedContext() context.Context {
	return context.Background()
}

func testProvenance() irac.Provenance {
	return irac.Provenance{GeneratedBy: "test-fixture", GeneratedAt: time.Now()}
}

// fatalfer is the minimal subset of *testing.T this package's helpers
// need, mirroring packages/reasoningtrace/helpers_test.go's convention.
type fatalfer interface {
	Fatalf(string, ...any)
}

// seededStore returns an InMemoryGraphStore pre-populated with a
// realistic, fully-formed tree for testCaseID: one issue, one fact node
// whose text carries a concrete figure and date, and one rule node with a
// resolvable citation.
func seededStore(t fatalfer) *graph.InMemoryGraphStore {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()

	nodes := []irac.Node{
		irac.NewIssueNode("issue-1", testCaseID, "Was the contract validly formed?", time.Now(), 0.9, testProvenance()).Node,
		irac.NewFactNode("fact-1", testCaseID, "The parties signed a written memorandum on 2024-03-15 for $4,500.00.", time.Now(), 0.9, testProvenance()).Node,
		irac.NewRuleNode("rule-1", testCaseID, "A contract for the sale of goods over $500 requires a signed writing.", "US-CA", "common_law", time.Now(), 0.9, testProvenance()).Node,
	}
	for _, n := range nodes {
		if err := store.CreateNode(ctx, n); err != nil {
			t.Fatalf("CreateNode(%q): %v", n.ID, err)
		}
	}
	return store
}

// groundedOpinion returns a synthesisagent.Opinion for testCaseID whose
// single conclusion's Text, SupportingFactIDs, and SupportingRuleIDs are
// all fully consistent with seededStore's fixture: the figure ($4,500.00)
// and date (2024-03-15) both appear verbatim in fact-1's Text, and both
// referenced node IDs exist.
func groundedOpinion() synthesisagent.Opinion {
	return synthesisagent.Opinion{
		CaseID: testCaseID,
		Conclusions: []synthesisagent.TentativeConclusion{
			{
				IssueNodeID:       "issue-1",
				Text:              "The parties signed a written memorandum on 2024-03-15 for $4,500.00, satisfying the writing requirement.",
				FavoredParty:      "plaintiff",
				Confidence:        0.8,
				SupportingFactIDs: []string{"fact-1"},
				SupportingRuleIDs: []string{"rule-1"},
				Grounded:          true,
			},
		},
		GeneratedAt: time.Now(),
	}
}
