package casesearch

import (
	"context"
	"strings"

	"github.com/YASSERRMD/verdex/packages/embedding"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
)

// EmbedFunc turns free-text search input into a query vector for semantic
// search, typically backed by whatever packages/embedding.Service or
// provider embedding client the deployment already uses to build
// vectorindex records. casesearch does not depend on packages/embedding's
// Service interface directly (nor on any model provider) — callers supply
// this function so casesearch stays provider-agnostic, mirroring how
// packages/hybridretrieval itself takes an already-embedded
// embedding.EmbeddingVector rather than raw text.
type EmbedFunc func(ctx context.Context, text string) (embedding.EmbeddingVector, error)

// KnowledgeAPISearcher adapts a single case's *knowledgeapi.KnowledgeAPI
// into a CaseSearcher, the reference implementation callers are expected
// to use in production (see doc.go). It is the seam through which
// ModeKeyword/ModeSemantic/ModeIssueRule are actually answered: keyword
// mode walks the case's tree via GetTree and matches text locally;
// semantic mode embeds the query with Embed and delegates to
// KnowledgeAPI.Retrieve; issue/rule mode delegates to
// KnowledgeAPI.LookupPaths.
type KnowledgeAPISearcher struct {
	api   *knowledgeapi.KnowledgeAPI
	embed EmbedFunc
}

// NewKnowledgeAPISearcher builds a KnowledgeAPISearcher over api. embed
// may be nil, in which case SearchSemantic returns ErrNilEmbedFunc — a
// deployment that has not wired up an embedding function can still use
// ModeKeyword and ModeIssueRule.
func NewKnowledgeAPISearcher(api *knowledgeapi.KnowledgeAPI, embed EmbedFunc) (*KnowledgeAPISearcher, error) {
	if api == nil {
		return nil, wrapf("NewKnowledgeAPISearcher", ErrNilRepository)
	}
	return &KnowledgeAPISearcher{api: api, embed: embed}, nil
}

// SearchKeyword implements CaseSearcher by walking the case's full tree
// (paginating through knowledgeapi.GetTree) and matching text as a
// case-insensitive substring against each node's text.
func (s *KnowledgeAPISearcher) SearchKeyword(ctx context.Context, text string, topK int) ([]Hit, error) {
	needle := strings.ToLower(strings.TrimSpace(text))
	if needle == "" {
		return nil, nil
	}

	var hits []Hit
	const pageSize = 200
	pageNum := 1
	for {
		resp, err := s.api.GetTree(ctx, knowledgeapi.GetTreeRequest{
			CaseID: s.api.CaseID(),
			Page:   knowledgeapi.PageRequest{Page: pageNum, PerPage: pageSize},
		})
		if err != nil {
			return nil, err
		}
		for _, node := range resp.Nodes {
			if strings.Contains(strings.ToLower(node.Text), needle) {
				hits = append(hits, Hit{
					NodeID:      node.ID,
					NodeType:    node.Type,
					Text:        node.Text,
					Score:       keywordScore(node.Text, needle),
					Explanation: "keyword match on node text",
				})
			}
		}
		if pageNum >= resp.Meta.TotalPages || resp.Meta.TotalPages == 0 {
			break
		}
		pageNum++
	}

	sortHitsByScore(hits)
	return capHits(hits, topK), nil
}

// keywordScore is a small deterministic relevance heuristic for
// ModeKeyword: the fraction of needle-length coverage relative to the
// total text length, so a short node consisting almost entirely of the
// match ranks above a long node containing the match once. This is
// intentionally simple (no TF-IDF/BM25) — see doc/case-search.md for why
// a heavier ranking model was judged out of scope for v1's keyword mode.
func keywordScore(text, needle string) float64 {
	if len(text) == 0 {
		return 0
	}
	count := strings.Count(strings.ToLower(text), needle)
	coverage := float64(count*len(needle)) / float64(len(text))
	if coverage > 1 {
		coverage = 1
	}
	// Base score keeps every keyword hit within (0,1], with coverage
	// providing a secondary tiebreaker.
	return 0.5 + 0.5*coverage
}

// SearchSemantic implements CaseSearcher by embedding text via the
// configured EmbedFunc and delegating to KnowledgeAPI.Retrieve.
func (s *KnowledgeAPISearcher) SearchSemantic(ctx context.Context, text string, topK int) ([]Hit, error) {
	if s.embed == nil {
		return nil, ErrNilEmbedFunc
	}
	vec, err := s.embed(ctx, text)
	if err != nil {
		return nil, err
	}

	resp, err := s.api.Retrieve(ctx, knowledgeapi.RetrieveRequest{
		CaseID: s.api.CaseID(),
		Vector: vec,
		TopK:   topK,
	})
	if err != nil {
		return nil, err
	}

	hits := make([]Hit, 0, len(resp.Items))
	for _, item := range resp.Items {
		hits = append(hits, Hit{
			NodeID:      item.NodeID,
			NodeType:    item.NodeType,
			Text:        item.Text,
			Score:       item.CombinedScore,
			Explanation: item.Explanation,
		})
	}
	sortHitsByScore(hits)
	return capHits(hits, topK), nil
}

// SearchIssueOrRule implements CaseSearcher by delegating to
// KnowledgeAPI.LookupPaths rooted at rootNodeID, flattening every
// non-root node across every returned path into a Hit.
func (s *KnowledgeAPISearcher) SearchIssueOrRule(ctx context.Context, rootNodeID string, topK int) ([]Hit, error) {
	resp, err := s.api.LookupPaths(ctx, knowledgeapi.LookupPathsRequest{
		CaseID:     s.api.CaseID(),
		FromNodeID: rootNodeID,
		Page:       knowledgeapi.PageRequest{Page: 1, PerPage: topK},
	})
	if err != nil {
		if isCaseNotIndexedOrNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	var hits []Hit
	seen := make(map[string]struct{})
	for _, path := range resp.Paths {
		for i, node := range path.Nodes {
			if i == 0 {
				// The root itself (the issue/rule/statute node) is the
				// anchor, not a hit — callers already know it matched by
				// virtue of the path existing.
				continue
			}
			if _, dup := seen[node.ID]; dup {
				continue
			}
			seen[node.ID] = struct{}{}
			hits = append(hits, Hit{
				NodeID:      node.ID,
				NodeType:    node.Type,
				Text:        node.Text,
				Score:       issueRuleScore(i),
				Explanation: "reached via issue/rule path " + path.Kind,
			})
		}
	}

	sortHitsByScore(hits)
	return capHits(hits, topK), nil
}

// issueRuleScore ranks issue/rule path hits by proximity to the root:
// closer nodes (smaller path index) score higher, mirroring the intuition
// that a fact/application directly hanging off the matched rule is more
// relevant than one several hops deeper.
func issueRuleScore(pathIndex int) float64 {
	score := 1.0 / float64(pathIndex+1)
	return score
}

// isCaseNotIndexedOrNotFound reports whether err indicates "this case has
// no data for the requested root node" rather than a genuine failure,
// matching the doc comment on CaseSearcherResolver: SearchIssueOrRule
// finding nothing for a case is normal, not an error.
func isCaseNotIndexedOrNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "not indexed") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "node id")
}

// sortHitsByScore sorts hits by descending Score in place.
func sortHitsByScore(hits []Hit) {
	// Simple insertion sort is sufficient: hit lists here are bounded by
	// a case's tree size and typically already near-sorted from the
	// underlying retrieval/path lookup.
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j].Score > hits[j-1].Score; j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}
}

// capHits truncates hits to at most topK entries. topK <= 0 means
// DefaultTopKPerCase.
func capHits(hits []Hit, topK int) []Hit {
	if topK <= 0 {
		topK = DefaultTopKPerCase
	}
	if len(hits) > topK {
		hits = hits[:topK]
	}
	return hits
}

var _ CaseSearcher = (*KnowledgeAPISearcher)(nil)
