package hybridretrieval

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/traversal"
)

// graphHit is one node discovered by expanding a single seed, carrying
// the best (highest-scoring) traversal.Path that reached it and which
// seed the expansion started from.
type graphHit struct {
	nodeID   string
	node     traversal.PathNode
	score    float64
	anchorID string
	explain  string
}

// buildExpansionQuery translates query's ExpansionHops/MaxExpansionDepth/
// resolvers into a traversal.Query rooted at seedNodeID. Returns an error
// only if query.ExpansionHops contains an unrecognized ExpansionHop
// (exhaustively validated so a future new hop constant cannot silently
// no-op).
func buildExpansionQuery(query HybridQuery, seedNodeID string) (traversal.Query, error) {
	tq := traversal.NewQuery(query.CaseID, seedNodeID)
	for _, hop := range query.ExpansionHops {
		switch hop {
		case ExpansionGoverningRule:
			tq = tq.ViaGoverningRule()
		case ExpansionControllingPrecedent:
			tq = tq.ViaControllingPrecedent()
		case ExpansionDistinguishingFacts:
			tq = tq.ViaDistinguishingFacts()
		default:
			return traversal.Query{}, fmt.Errorf("hybridretrieval: expansion hop %q: %w", hop, errUnrecognizedExpansionHop)
		}
	}
	if query.MaxExpansionDepth > 0 {
		tq = tq.WithMaxDepth(query.MaxExpansionDepth)
	}
	if query.PrecedentResolver != nil {
		tq = tq.WithPrecedentResolver(query.PrecedentResolver)
	}
	if query.DistinguishingFactResolver != nil {
		tq = tq.WithDistinguishingFactResolver(query.DistinguishingFactResolver)
	}
	return tq, nil
}

// expandSeed runs one traversal.Walker.Execute call from seedNodeID and
// returns every non-start node reached, capped at maxPerAnchor (0 means
// unlimited), ranked by descending traversal.Path.Score. A seed with no
// configured ExpansionHops, or whose start node is not found (e.g. a
// vector hit whose ID doesn't exist as a graph node — should not happen
// in practice, but defensively tolerated), yields no hits rather than an
// error: a hybrid query's graph-expansion phase is best-effort
// augmentation of vector recall, not a hard requirement.
func expandSeed(ctx context.Context, walker *traversal.Walker, query HybridQuery, seedNodeID string, maxPerAnchor int) ([]graphHit, error) {
	if len(query.ExpansionHops) == 0 {
		return nil, nil
	}

	tq, err := buildExpansionQuery(query, seedNodeID)
	if err != nil {
		return nil, err
	}

	result, err := walker.Execute(ctx, tq)
	if err != nil {
		// A missing/foreign start node is a routine "nothing to expand"
		// outcome for this best-effort phase, not a hard failure of the
		// whole hybrid query.
		if isExpectedExpansionErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("hybridretrieval: expand seed %q: %w", seedNodeID, err)
	}

	best := make(map[string]graphHit)
	for _, path := range result.Paths {
		if len(path.Nodes) < 2 {
			continue // zero-hop path: nothing beyond the seed itself
		}
		end := path.Nodes[len(path.Nodes)-1]
		if existing, ok := best[end.ID]; ok && existing.score >= path.Score {
			continue
		}
		best[end.ID] = graphHit{
			nodeID:   end.ID,
			node:     end,
			score:    path.Score,
			anchorID: seedNodeID,
			explain:  path.Explain(),
		}
	}

	hits := make([]graphHit, 0, len(best))
	for _, h := range best {
		hits = append(hits, h)
	}
	sortGraphHitsByScoreDesc(hits)

	if maxPerAnchor > 0 && len(hits) > maxPerAnchor {
		hits = hits[:maxPerAnchor]
	}
	return hits, nil
}

// isExpectedExpansionErr reports whether err is a Walker.Execute failure
// this package treats as "nothing to expand" rather than propagating as a
// hard error: specifically, the seed node not existing in the case's
// graph. Any other error (e.g. a resolver failing) is propagated.
func isExpectedExpansionErr(err error) bool {
	return isErrStartNodeNotFound(err)
}
