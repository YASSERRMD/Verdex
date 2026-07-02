package knowledgeapi

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/hybridretrieval"
)

// knownExpansionHops is the exhaustive set of ExpansionHop string values
// this package accepts on a RetrieveRequest, mirroring
// hybridretrieval.ExpansionHop's own recognized constants.
var knownExpansionHops = map[string]hybridretrieval.ExpansionHop{
	string(hybridretrieval.ExpansionGoverningRule):        hybridretrieval.ExpansionGoverningRule,
	string(hybridretrieval.ExpansionControllingPrecedent): hybridretrieval.ExpansionControllingPrecedent,
	string(hybridretrieval.ExpansionDistinguishingFacts):  hybridretrieval.ExpansionDistinguishingFacts,
}

// Retrieve runs a fused semantic-plus-structural retrieval query via
// hybridretrieval.Retriever.Retrieve, scoped to this KnowledgeAPI's case.
// The underlying Retriever must have been constructed over this same
// case's case-scoped GraphStore and CaseScopedVectorStore (see
// NewKnowledgeAPI's doc comment), so every fused item is guaranteed to
// respect knowledgeisolation's cross-case boundary the same way any other
// case-scoped read does.
//
// Returns ErrEmptyQuery if req carries neither a query Vector nor an
// AnchorNodeID (hybridretrieval.HybridQuery requires at least one
// semantic or structural starting point).
func (api *KnowledgeAPI) Retrieve(ctx context.Context, req RetrieveRequest) (RetrieveResponse, error) {
	if _, err := authorize(ctx); err != nil {
		return RetrieveResponse{}, err
	}
	if req.CaseID == "" || req.CaseID != api.caseID {
		return RetrieveResponse{}, ErrEmptyCaseID
	}
	if len(req.Vector) == 0 && req.AnchorNodeID == "" {
		return RetrieveResponse{}, ErrEmptyQuery
	}

	page, perPage, err := normalizePage(req.Page)
	if err != nil {
		return RetrieveResponse{}, err
	}

	query := hybridretrieval.NewHybridQuery(api.caseID, req.Vector)
	if req.AnchorNodeID != "" {
		query = query.WithAnchor(req.AnchorNodeID)
	}
	if req.MaxExpansionDepth > 0 {
		query = query.WithMaxExpansionDepth(req.MaxExpansionDepth)
	}
	if req.TopK > 0 {
		query = query.WithTopK(req.TopK)
	}

	hops := make([]hybridretrieval.ExpansionHop, 0, len(req.ExpansionHops))
	for _, h := range req.ExpansionHops {
		if hop, ok := knownExpansionHops[h]; ok {
			hops = append(hops, hop)
		}
	}
	for _, hop := range hops {
		query = query.WithExpansion(hop)
	}

	result, err := api.retriever.Retrieve(ctx, query)
	if err != nil {
		return RetrieveResponse{}, err
	}

	pageItems, meta := paginate(result.Items, page, perPage)
	itemDTOs := make([]RetrievedItemDTO, 0, len(pageItems))
	for _, item := range pageItems {
		itemDTOs = append(itemDTOs, retrievedItemDTOFromItem(item))
	}

	return RetrieveResponse{
		Version:            APIVersionV1,
		CaseID:             api.caseID,
		Items:              itemDTOs,
		VectorHitCount:     result.VectorHitCount,
		ExpansionSeedCount: result.ExpansionSeedCount,
		ExpansionSkipped:   result.ExpansionSkipped,
		ExpansionTruncated: result.ExpansionTruncated,
		Meta:               meta,
	}, nil
}

// retrievedItemDTOFromItem converts a hybridretrieval.Item into a
// RetrievedItemDTO.
func retrievedItemDTOFromItem(item hybridretrieval.Item) RetrievedItemDTO {
	return RetrievedItemDTO{
		NodeID:        item.NodeID,
		NodeType:      string(item.NodeType),
		Text:          item.Text,
		Path:          string(item.Path),
		CombinedScore: item.CombinedScore,
		AnchorNodeID:  item.AnchorNodeID,
		Explanation:   item.Explanation,
	}
}
