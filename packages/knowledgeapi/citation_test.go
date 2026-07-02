package knowledgeapi_test

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/citation"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
)

// TestResolveCitation_VerifiedNode proves ResolveCitation composes
// packages/citation's Resolver and Verify: a real node in the store
// resolves to StatusVerified with the resolver's citation text attached.
func TestResolveCitation_VerifiedNode(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t, "case-a")
	f.seedNode(t, irac.Node{
		ID: "rule-1", Type: irac.NodeRule, CaseID: "case-a",
		Text: "Notice must be in writing.", Confidence: 0.95,
	})

	resolver := func(_ context.Context, node irac.Node) (citation.ResolvedCitation, error) {
		return citation.ResolvedCitation{
			Text:      "Act 12, s.5(a)",
			Origin:    citation.OriginStatute,
			Certainty: citation.CertaintyExact,
		}, nil
	}

	api := f.api.WithCitationResolver(resolver)

	ctx := authedContext(identity.RoleJudge)
	resp, err := api.ResolveCitation(ctx, knowledgeapi.ResolveCitationRequest{CaseID: "case-a", NodeID: "rule-1"})
	if err != nil {
		t.Fatalf("ResolveCitation: %v", err)
	}

	if resp.Citation.Citation != "Act 12, s.5(a)" {
		t.Errorf("expected citation text, got %q", resp.Citation.Citation)
	}
	if resp.Citation.VerificationStatus != string(citation.StatusVerified) {
		t.Errorf("expected verified status, got %q", resp.Citation.VerificationStatus)
	}
	if !resp.Citation.Verified {
		t.Errorf("expected Verified true")
	}
	if resp.Citation.Certainty != string(citation.CertaintyExact) {
		t.Errorf("expected exact certainty, got %q", resp.Citation.Certainty)
	}
	if resp.Citation.ConfidenceScore <= 0 {
		t.Errorf("expected positive confidence score, got %f", resp.Citation.ConfidenceScore)
	}
}

// TestResolveCitation_DefaultResolver_NoCitation proves that without an
// explicit WithCitationResolver, ResolveCitation still succeeds (using
// citation.NoResolver), just with no citation text and CertaintyNone.
func TestResolveCitation_DefaultResolver_NoCitation(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t, "case-a")
	f.seedNode(t, irac.Node{ID: "fact-1", Type: irac.NodeFact, CaseID: "case-a", Text: "The lease began in March."})

	ctx := authedContext(identity.RoleJudge)
	resp, err := f.api.ResolveCitation(ctx, knowledgeapi.ResolveCitationRequest{CaseID: "case-a", NodeID: "fact-1"})
	if err != nil {
		t.Fatalf("ResolveCitation: %v", err)
	}
	if resp.Citation.Citation != "" {
		t.Errorf("expected empty citation, got %q", resp.Citation.Citation)
	}
	if resp.Citation.Certainty != string(citation.CertaintyNone) {
		t.Errorf("expected CertaintyNone, got %q", resp.Citation.Certainty)
	}
}

// TestResolveCitation_UnknownNode_PropagatesNotFound proves a request for
// a node that does not exist propagates the store's not-found error
// rather than fabricating a citation.
func TestResolveCitation_UnknownNode_PropagatesNotFound(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t, "case-a")
	ctx := authedContext(identity.RoleJudge)

	_, err := f.api.ResolveCitation(ctx, knowledgeapi.ResolveCitationRequest{CaseID: "case-a", NodeID: "does-not-exist"})
	if err == nil {
		t.Fatal("expected an error for an unknown node")
	}
}
