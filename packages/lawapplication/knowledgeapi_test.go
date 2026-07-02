package lawapplication_test

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/citation"
	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
	"github.com/YASSERRMD/verdex/packages/firstpartyagent"
	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/hybridretrieval"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/issueagent"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
	"github.com/YASSERRMD/verdex/packages/knowledgeisolation"
	"github.com/YASSERRMD/verdex/packages/lawapplication"
	"github.com/YASSERRMD/verdex/packages/secondpartyagent"
	"github.com/YASSERRMD/verdex/packages/treeindex"
	"github.com/YASSERRMD/verdex/packages/vectorindex"
)

// kaFixture wires a full KnowledgeAPI over an in-memory GraphStore and
// VectorStore for a single case, mirroring
// packages/firstpartyagent/helpers_test.go's fixture composition
// exactly, so this package's end-to-end test exercises the real
// knowledgeapi.GetTree / ResolveCitation boundary a production caller
// would use to build a lawapplication.Request.
type kaFixture struct {
	caseID string
	inner  *graph.InMemoryGraphStore
	api    *knowledgeapi.KnowledgeAPI
}

func newKAFixture(t *testing.T, caseID string) *kaFixture {
	t.Helper()

	inner := graph.NewInMemoryGraphStore()
	store, err := knowledgeisolation.NewCaseScopedStore(inner, caseID, nil)
	if err != nil {
		t.Fatalf("NewCaseScopedStore: %v", err)
	}

	vectors, err := knowledgeisolation.NewCaseScopedVectorStore(
		vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{}), caseID, nil,
	)
	if err != nil {
		t.Fatalf("NewCaseScopedVectorStore: %v", err)
	}

	indexer, err := treeindex.NewIndexer(store, treeindex.IndexerOptions{})
	if err != nil {
		t.Fatalf("NewIndexer: %v", err)
	}

	retriever, err := hybridretrieval.NewRetriever(vectors, store)
	if err != nil {
		t.Fatalf("NewRetriever: %v", err)
	}

	api, err := knowledgeapi.NewKnowledgeAPI(caseID, store, vectors, indexer, retriever)
	if err != nil {
		t.Fatalf("NewKnowledgeAPI: %v", err)
	}
	api = api.WithCitationResolver(fakeKACitationResolver)

	return &kaFixture{caseID: caseID, inner: inner, api: api}
}

// fakeKACitationResolver resolves every rule node into a deterministic,
// verifiable citation, mirroring firstpartyagent's own test resolver.
func fakeKACitationResolver(_ context.Context, node irac.Node) (citation.ResolvedCitation, error) {
	origin := citation.OriginStatute
	if node.Type == irac.NodeRule {
		origin = citation.OriginStatute
	}
	return citation.ResolvedCitation{
		Text:      "Fake Reporter " + node.ID,
		Origin:    origin,
		Certainty: citation.CertaintyExact,
	}, nil
}

func (f *kaFixture) seedIssue(t *testing.T, id, text string) {
	t.Helper()
	node := irac.NewIssueNode(id, f.caseID, text, time.Now(), 0.9, irac.Provenance{})
	if err := f.inner.CreateNode(context.Background(), node.Node); err != nil {
		t.Fatalf("seed issue %s: %v", id, err)
	}
}

func (f *kaFixture) seedRule(t *testing.T, id, text string) {
	t.Helper()
	node := irac.NewRuleNode(id, f.caseID, text, "US-CA", "common_law", time.Now(), 0.9, irac.Provenance{})
	if err := f.inner.CreateNode(context.Background(), node.Node); err != nil {
		t.Fatalf("seed rule %s: %v", id, err)
	}
}

func (f *kaFixture) seedFact(t *testing.T, id, text string, confidence float64) {
	t.Helper()
	node := irac.NewFactNode(id, f.caseID, text, time.Now(), confidence, irac.Provenance{})
	if err := f.inner.CreateNode(context.Background(), node.Node); err != nil {
		t.Fatalf("seed fact %s: %v", id, err)
	}
}

func (f *kaFixture) seedGoverns(t *testing.T, ruleID, issueID string) {
	t.Helper()
	edge := irac.Edge{FromID: ruleID, ToID: issueID, Type: irac.EdgeGoverns}
	if err := f.inner.CreateEdge(context.Background(), edge); err != nil {
		t.Fatalf("seed governs edge %s->%s: %v", ruleID, issueID, err)
	}
}

func authedKAContext() context.Context {
	user := &identity.User{
		Email:     "test@example.com",
		Name:      "Test User",
		Roles:     []identity.Role{identity.RoleAdvocate},
		Status:    identity.UserStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return identity.WithUser(context.Background(), user)
}

// citationLookupFromKnowledgeAPI adapts a *knowledgeapi.KnowledgeAPI into
// a lawapplication.CitationLookupFunc, the composition pattern a real
// caller is expected to use (see doc/law-application.md).
func citationLookupFromKnowledgeAPI(ctx context.Context, api *knowledgeapi.KnowledgeAPI, caseID string) lawapplication.CitationLookupFunc {
	return func(ruleID string) (string, lawapplication.Origin, bool, string, error) {
		resp, err := api.ResolveCitation(ctx, knowledgeapi.ResolveCitationRequest{CaseID: caseID, NodeID: ruleID})
		if err != nil {
			return "", lawapplication.OriginUnknown, false, "", err
		}
		return resp.Citation.Citation, lawapplication.Origin(resp.Citation.Origin), resp.Citation.Verified, resp.Citation.VerificationStatus, nil
	}
}

// TestApply_EndToEndWithKnowledgeAPI seeds a full IRAC tree (Issue, two
// Rules governing it, two Facts) via an in-memory GraphStore, reads it
// back out through the real knowledgeapi.GetTree/ResolveCitation
// boundary exactly as a production orchestrator would, builds fixture
// ArgumentSets and an evidenceweighing.Result by hand, and runs Apply
// end-to-end — covering the full composition contract this package
// documents in doc/law-application.md.
func TestApply_EndToEndWithKnowledgeAPI(t *testing.T) {
	caseID := "case-ka-1"
	f := newKAFixture(t, caseID)
	ctx := authedKAContext()

	f.seedIssue(t, "issue-1", "Was notice reasonable?")
	f.seedRule(t, "rule-statute", "42 U.S.C. section 1983 requires reasonable notice.")
	f.seedRule(t, "rule-precedent", "In Smith v. Jones, the court held notice must be actual.")
	f.seedFact(t, "fact-1", "The landlord mailed notice five days before entry.", 0.8)
	f.seedFact(t, "fact-2", "The tenant testified they never received notice.", 0.6)
	f.seedGoverns(t, "rule-statute", "issue-1")
	f.seedGoverns(t, "rule-precedent", "issue-1")

	treeResp, err := f.api.GetTree(ctx, knowledgeapi.GetTreeRequest{CaseID: caseID})
	if err != nil {
		t.Fatalf("GetTree: %v", err)
	}

	var rules []lawapplication.RuleRef
	var governingRuleIDs []string
	for _, n := range treeResp.Nodes {
		if n.Type != string(irac.NodeRule) {
			continue
		}
		rules = append(rules, lawapplication.RuleRef{ID: n.ID, Text: n.Text})
	}
	for _, e := range treeResp.Edges {
		if e.Type == string(irac.EdgeGoverns) {
			governingRuleIDs = append(governingRuleIDs, e.FromID)
		}
	}

	firstParty := firstpartyagent.ArgumentSet{
		CaseID:  caseID,
		PartyID: "plaintiff",
		Arguments: []firstpartyagent.Argument{
			{
				ID:                "arg-p1",
				IssueNodeID:       "issue-1",
				PartyID:           "plaintiff",
				SupportingFactIDs: []string{"fact-1"},
				SupportingRuleIDs: []string{"rule-statute"},
			},
		},
	}
	secondParty := secondpartyagent.ArgumentSet{
		CaseID:  caseID,
		PartyID: "defendant",
		Arguments: []secondpartyagent.Argument{
			{
				ID:                "arg-p2",
				IssueNodeID:       "issue-1",
				PartyID:           "defendant",
				SupportingFactIDs: []string{"fact-2"},
				SupportingRuleIDs: []string{"rule-precedent"},
			},
		},
	}

	evidence := evidenceweighing.Result{
		CaseID: caseID,
		FactWeights: []evidenceweighing.FactWeight{
			{FactNodeID: "fact-1", Weight: 0.85},
			{FactNodeID: "fact-2", Weight: 0.5, Contradicted: true},
		},
	}

	req := lawapplication.Request{
		CaseID: caseID,
		Issues: []lawapplication.IssueInput{
			{
				Issue:            issueagent.FramedIssue{SourceIssueNodeID: "issue-1", GoverningQuestions: []string{"Was notice reasonable?"}},
				GoverningRuleIDs: governingRuleIDs,
			},
		},
		Rules:          rules,
		FirstParty:     firstParty,
		SecondParty:    secondParty,
		Evidence:       evidence,
		LegalFamily:    lawapplication.CommonLawFamily,
		CitationLookup: citationLookupFromKnowledgeAPI(ctx, f.api, caseID),
	}

	result, err := lawapplication.Apply(req)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(result.IssueApplications) != 1 {
		t.Fatalf("len(IssueApplications) = %d, want 1", len(result.IssueApplications))
	}
	ia := result.IssueApplications[0]

	if len(ia.ControllingRuleIDs) != 2 {
		t.Errorf("ControllingRuleIDs = %v, want 2 rules", ia.ControllingRuleIDs)
	}
	if len(ia.Citations) != 2 {
		t.Fatalf("Citations = %v, want 2", ia.Citations)
	}
	for _, c := range ia.Citations {
		if !c.Resolved || !c.Verified {
			t.Errorf("citation %+v should resolve and verify via the real knowledgeapi boundary", c)
		}
	}
	if len(ia.Conflicts) != 1 {
		t.Errorf("Conflicts = %v, want 1 (opposing parties invoked different rules)", ia.Conflicts)
	}
	if len(ia.ElementFactMap) != 2 {
		t.Errorf("ElementFactMap = %v, want 2 entries", ia.ElementFactMap)
	}

	// Persist and retrieve via the Repository, exercising Phase 055's
	// intended read path.
	repo := lawapplication.NewInMemoryRepository()
	if err := repo.Save(ctx, result); err != nil {
		t.Fatalf("Save: %v", err)
	}
	saved, err := repo.Get(ctx, caseID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if saved.CaseID != caseID {
		t.Errorf("saved.CaseID = %q, want %q", saved.CaseID, caseID)
	}
}
