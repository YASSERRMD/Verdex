package application

import (
	"context"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// ApplicationService orchestrates the full rule-linkage-and-application
// pipeline:
//
//	match issue to rules -> build application nodes -> link
//	  precedents/distinguishing facts -> resolve rule chains -> weight by
//	  legal family -> score confidence -> persist subgraph
//	  -> return []irac.ApplicationNode
//
// This mirrors packages/issue's IssueExtractionService and
// packages/fact's FactConstructionService orchestration pattern: a
// single entry point wires together this package's otherwise
// independent, individually testable building blocks (MatchIssueToRules,
// BuildApplicationNode, NewPrecedentIssueLink, NewDistinguishingFact,
// RuleChain.Validate, WeightByLegalFamily, ComputeConfidence,
// PersistApplicationSubgraph).
type ApplicationService struct {
	// Store persists the built irac.ApplicationNodes and their edges. If
	// nil, a fresh graph.InMemoryGraphStore is used.
	Store graph.GraphStore
}

// NewApplicationService constructs an ApplicationService with a fresh
// in-memory graph.GraphStore.
func NewApplicationService() *ApplicationService {
	return &ApplicationService{Store: graph.NewInMemoryGraphStore()}
}

// ApplyRequest carries the input to ApplicationService.ApplyRules.
type ApplyRequest struct {
	// Issue is the irac.IssueNode being resolved. Required.
	Issue irac.IssueNode

	// Rules is the set of candidate OriginatedRules to match against
	// Issue. Required (non-empty).
	Rules []OriginatedRule

	// Facts is the set of irac.FactNodes available to support an
	// application of the matched rule(s). Required (non-empty).
	Facts []irac.FactNode

	// DominantFamily is the case's dominant legal family (e.g.
	// "common_law", "civil_law"), used by WeightByLegalFamily. Empty
	// means neutral weighting (see weight.go).
	DominantFamily string

	// TopN caps how many of the highest-scoring rule matches are built
	// into ApplicationNodes. Zero or negative means "every rule that
	// scored above zero".
	TopN int

	// Chain optionally supplies a RuleChain that must be validated (no
	// cycles) before any application node is built. A zero-value Chain
	// (empty Rules) is treated as "no chain supplied" and skipped.
	Chain RuleChain

	// PrecedentRationales optionally supplies a rationale for a
	// PrecedentIssueLink, keyed by the underlying irac.RuleNode.ID, for
	// every precedent-origin rule that should be explicitly linked (see
	// precedent_link.go). Rules with no entry here are still applied,
	// just without a recorded PrecedentIssueLink.
	PrecedentRationales map[string]string

	// DistinguishingRationales optionally supplies a rationale for a
	// DistinguishingFact, keyed by "<ruleID>:<factID>", for any
	// (precedent rule, fact) pair the caller wants explicitly recorded as
	// distinguishing (see distinguish.go).
	DistinguishingRationales map[string]string

	// RuleGovernsIssueExists optionally lists irac.RuleNode.IDs whose
	// Rule--governs-->Issue edge to Issue already exists in Store, so
	// PersistApplicationSubgraph does not try to recreate it.
	RuleGovernsIssueExists map[string]bool

	// CreatedAt stamps every built irac.ApplicationNode's CreatedAt. If
	// zero, time.Now() is used.
	CreatedAt time.Time
}

// ApplyResult bundles the persisted irac.ApplicationNodes with the
// linkage bookkeeping the pipeline produced alongside them.
type ApplyResult struct {
	Nodes               []irac.ApplicationNode
	Matches             []RuleMatch
	PrecedentLinks      []PrecedentIssueLink
	DistinguishingFacts []DistinguishingFact
}

// ApplyRules runs the full pipeline over req and returns the resulting
// irac.ApplicationNodes, persisted via s.Store.
//
// Returns ErrEmptyInput if req.Issue.Text, req.Rules, or req.Facts is
// empty, ErrNoMatchingRules if every candidate rule scores 0 against the
// issue, ErrCyclicChain if req.Chain is non-empty and cyclic, or
// ErrPersistFailed if persistence fails partway through.
func (s *ApplicationService) ApplyRules(ctx context.Context, req ApplyRequest) ([]irac.ApplicationNode, error) {
	result, err := s.ApplyRulesDetailed(ctx, req)
	return result.Nodes, err
}

// ApplyRulesDetailed runs the same pipeline as ApplyRules but returns the
// full ApplyResult, including the RuleMatches, PrecedentIssueLinks, and
// DistinguishingFacts the pipeline produced.
func (s *ApplicationService) ApplyRulesDetailed(ctx context.Context, req ApplyRequest) (ApplyResult, error) {
	if req.Issue.Text == "" || len(req.Rules) == 0 || len(req.Facts) == 0 {
		return ApplyResult{}, ErrEmptyInput
	}

	store := s.Store
	if store == nil {
		store = graph.NewInMemoryGraphStore()
	}
	createdAt := req.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	// Resolve rule chain (if any) before doing any other work.
	if len(req.Chain.Rules) > 0 {
		if err := req.Chain.Validate(); err != nil {
			return ApplyResult{}, err
		}
	}

	// 1. match issue to rules
	matches := MatchIssueToRules(req.Issue, req.Rules)

	positive := make([]RuleMatch, 0, len(matches))
	for _, m := range matches {
		if m.Score > 0 {
			positive = append(positive, m)
		}
	}
	if len(positive) == 0 {
		return ApplyResult{}, ErrNoMatchingRules
	}
	if req.TopN > 0 && len(positive) > req.TopN {
		positive = positive[:req.TopN]
	}

	result := ApplyResult{Matches: positive}

	for _, match := range positive {
		// 2. build application node
		node, err := BuildApplicationNode(req.Issue, match.Rule, req.Facts)
		if err != nil {
			continue
		}

		// 3. link precedents/distinguishing facts
		if match.Rule.Origin == OriginPrecedent {
			if rationale, ok := req.PrecedentRationales[match.Rule.Rule.ID]; ok {
				link, err := NewPrecedentIssueLink(req.Issue.ID, match.Rule, rationale, createdAt)
				if err == nil {
					result.PrecedentLinks = append(result.PrecedentLinks, link)
				}
			}
			for _, fact := range req.Facts {
				key := match.Rule.Rule.ID + ":" + fact.ID
				if rationale, ok := req.DistinguishingRationales[key]; ok {
					df, err := NewDistinguishingFact(fact, match.Rule, rationale, createdAt)
					if err == nil {
						result.DistinguishingFacts = append(result.DistinguishingFacts, df)
					}
				}
			}
		}

		// 4/5/6. weight by legal family + score confidence
		node = ApplyConfidence(node, match, req.DominantFamily)

		// 7. persist subgraph
		governsExists := req.RuleGovernsIssueExists[match.Rule.Rule.ID]
		if err := PersistApplicationSubgraph(ctx, store, node, match.Rule, req.Issue.ID, req.Facts, governsExists); err != nil {
			return result, err
		}

		result.Nodes = append(result.Nodes, node)
	}

	return result, nil
}
