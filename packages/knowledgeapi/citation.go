package knowledgeapi

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/citation"
)

// ResolveCitation resolves and verifies the citation for a single node
// within this KnowledgeAPI's case, composing packages/citation's Resolver
// and Verify function rather than reimplementing either: the configured
// citation.Resolver (see WithCitationResolver) produces the citation text
// and Certainty, and Verify independently confirms the underlying node
// still exists in the case-scoped GraphStore under the claimed case — the
// anti-hallucination guarantee packages/citation exists to provide. The
// final ConfidenceScore folds both outcomes together via
// citation.ScoreConfidence.
//
// Because both the node lookup and Verify go through this KnowledgeAPI's
// case-scoped GraphStore, a request for a node belonging to a different
// case surfaces as a verification failure (StatusWrongCase) or a
// knowledgeisolation.ErrCrossCaseAccess propagated from the store,
// depending on whether the node is shared-law — the citation endpoint
// never bypasses the case-isolation boundary any other read on this
// KnowledgeAPI respects.
func (api *KnowledgeAPI) ResolveCitation(ctx context.Context, req ResolveCitationRequest) (ResolveCitationResponse, error) {
	if _, err := authorize(ctx); err != nil {
		return ResolveCitationResponse{}, err
	}
	if req.CaseID == "" || req.CaseID != api.caseID {
		return ResolveCitationResponse{}, ErrEmptyCaseID
	}
	if req.NodeID == "" {
		return ResolveCitationResponse{}, ErrEmptyNodeID
	}

	node, err := api.store.GetNode(ctx, req.NodeID)
	if err != nil {
		return ResolveCitationResponse{}, err
	}

	resolver := api.citationResolver
	if resolver == nil {
		resolver = citation.NoResolver
	}

	resolved, err := resolver(ctx, node)
	if err != nil {
		return ResolveCitationResponse{}, err
	}

	unit := citation.CitedUnit{
		NodeID:   node.ID,
		CaseID:   api.caseID,
		NodeType: node.Type,
		Text:     node.Text,
		Origin:   resolved.Origin,
		Citation: resolved.Text,
	}

	verification, err := citation.Verify(ctx, api.store, unit)
	if err != nil {
		return ResolveCitationResponse{}, err
	}

	confidence := citation.ScoreConfidence(node.Confidence, resolved.Certainty, verification)

	return ResolveCitationResponse{
		Version: APIVersionV1,
		Citation: CitationDTO{
			Version:            APIVersionV1,
			NodeID:             unit.NodeID,
			CaseID:             unit.CaseID,
			Citation:           unit.Citation,
			Origin:             string(unit.Origin),
			Certainty:          string(resolved.Certainty),
			VerificationStatus: string(verification.Status),
			Verified:           verification.Verified(),
			ConfidenceScore:    confidence.Score,
		},
	}, nil
}
