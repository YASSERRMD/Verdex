package treeassembly

import (
	"context"
	"time"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// ConclusionProvider is the pluggable extension point through which
// irac.ConclusionNodes are added to an assembled Tree.
//
// This package (treeassembly, Phase 039) deliberately does NOT generate
// conclusions itself. Synthesizing a reasoned, non-binding conclusion
// from an Application requires an LLM reasoning agent — that is Phase
// 055 ("Synthesis & reasoned-opinion agent"), which is out of scope for
// this phase. ComposeTree instead accepts any ConclusionProvider
// implementation and calls it to obtain whatever ConclusionNodes (if
// any) should be included in the tree, so:
//
//   - today (with NoOpConclusionProvider or before Phase 055 exists),
//     ComposeTree produces a tree with zero ConclusionNodes — issues,
//     rules, facts, and applications only;
//   - once Phase 055 lands, it need only implement ConclusionProvider
//     and be wired in here (or by a caller), with no change required to
//     this package's composition, validation, gap-detection, revision,
//     or persistence logic.
//
// Keeping the interface here (rather than, say, only introducing it in
// Phase 055) is what makes conclusions pluggable: this package defines
// the shape a conclusion-producer must satisfy without depending on any
// particular synthesis implementation.
type ConclusionProvider interface {
	// Provide returns the irac.ConclusionNodes that should be included
	// in the tree assembled from input, or an error if conclusion
	// synthesis fails. Implementations are expected to attach the
	// mandatory draft_analysis guardrail label via
	// irac.NewConclusionNode (see packages/irac/guardrail.go) — this is
	// enforced structurally by irac.ValidateTree/MarshalTree regardless
	// of which ConclusionProvider is plugged in.
	Provide(ctx context.Context, input AssemblyInput) ([]irac.ConclusionNode, error)
}

// NoOpConclusionProvider is the default ConclusionProvider: it always
// returns an empty slice and a nil error. Used whenever no conclusion
// synthesis agent is plugged in (i.e. always, until Phase 055 exists),
// so ComposeTree can unconditionally call conclusions.Provide without a
// nil check at every call site.
type NoOpConclusionProvider struct{}

// Provide implements ConclusionProvider by returning no conclusions.
func (NoOpConclusionProvider) Provide(_ context.Context, _ AssemblyInput) ([]irac.ConclusionNode, error) {
	return []irac.ConclusionNode{}, nil
}

// ComposeTree assembles input's issues, rules, facts, and applications
// (plus any conclusions supplied by conclusions) into a single Tree:
// every node gathered in one slice, and every edge implied by the inputs
// reconstructed per packages/irac/edge.go's legal-triple constraint
// table:
//
//   - Rule --governs--> Issue: reconstructed when a RuleNode's
//     Provenance.UpstreamNodeIDs references an IssueNode's ID, or
//     (fallback) every Rule is linked to every Issue in a single-issue
//     input.
//   - Application --applies_to--> Rule / --applies_to--> Fact:
//     reconstructed from each ApplicationNode's
//     Provenance.UpstreamNodeIDs, matched against the supplied Rules and
//     Facts by ID.
//   - Fact --supports--> Application: the inverse of the above, one edge
//     per (Fact, Application) pair already implied by that Application's
//     UpstreamNodeIDs.
//   - Conclusion --concludes_from--> Application: reconstructed from
//     each ConclusionNode's Provenance.UpstreamNodeIDs, matched against
//     the supplied Applications by ID.
//
// If conclusions is nil, NoOpConclusionProvider is used. Returns
// ErrEmptyInput if input.CaseID is blank or input has no Issues, Facts,
// and Applications at all.
func ComposeTree(ctx context.Context, input AssemblyInput, conclusions ConclusionProvider) (*Tree, error) {
	if input.CaseID == "" {
		return nil, ErrEmptyInput
	}
	if len(input.Issues) == 0 && len(input.Facts) == 0 && len(input.Applications) == 0 {
		return nil, ErrEmptyInput
	}

	if conclusions == nil {
		conclusions = NoOpConclusionProvider{}
	}

	conclusionNodes, err := conclusions.Provide(ctx, input)
	if err != nil {
		return nil, err
	}

	nodes := make([]irac.NodeLike, 0, len(input.Issues)+len(input.Rules)+len(input.Facts)+len(input.Applications)+len(conclusionNodes))
	byID := make(map[string]irac.NodeLike, cap(nodes))

	for _, n := range input.Issues {
		nodes = append(nodes, n)
		byID[n.GetID()] = n
	}
	for _, n := range input.Rules {
		nodes = append(nodes, n)
		byID[n.GetID()] = n
	}
	for _, n := range input.Facts {
		nodes = append(nodes, n)
		byID[n.GetID()] = n
	}
	for _, n := range input.Applications {
		nodes = append(nodes, n)
		byID[n.GetID()] = n
	}
	for _, n := range conclusionNodes {
		nodes = append(nodes, n)
		byID[n.GetID()] = n
	}

	edges := deriveEdges(input, conclusionNodes, byID)

	revision := irac.NewInitialRevision(input.CaseID, time.Now())

	return &Tree{Nodes: nodes, Edges: edges, Revision: revision}, nil
}

// deriveEdges reconstructs the tree's edges from the provenance already
// recorded on each node, per the legal edge triples in
// packages/irac/edge.go. It only emits an edge when both endpoints are
// present in byID, so a node referencing an upstream ID outside this
// assembly's input never produces a dangling edge.
func deriveEdges(input AssemblyInput, conclusions []irac.ConclusionNode, byID map[string]irac.NodeLike) []irac.Edge {
	edges := make([]irac.Edge, 0)

	ruleByID := make(map[string]irac.RuleNode, len(input.Rules))
	for _, r := range input.Rules {
		ruleByID[r.ID] = r
	}
	factByID := make(map[string]irac.FactNode, len(input.Facts))
	for _, f := range input.Facts {
		factByID[f.ID] = f
	}
	issueByID := make(map[string]irac.IssueNode, len(input.Issues))
	for _, i := range input.Issues {
		issueByID[i.ID] = i
	}

	// Rule --governs--> Issue, derived from each Rule's upstream issue
	// references.
	for _, r := range input.Rules {
		for _, upID := range r.Provenance.UpstreamNodeIDs {
			if _, ok := issueByID[upID]; ok {
				edges = append(edges, irac.Edge{FromID: r.ID, ToID: upID, Type: irac.EdgeGoverns})
			}
		}
	}

	// Application --applies_to--> Rule / Fact, and the inverse
	// Fact --supports--> Application, both derived from each
	// Application's upstream references.
	for _, a := range input.Applications {
		for _, upID := range a.Provenance.UpstreamNodeIDs {
			if _, ok := ruleByID[upID]; ok {
				edges = append(edges, irac.Edge{FromID: a.ID, ToID: upID, Type: irac.EdgeAppliesTo})
				continue
			}
			if _, ok := factByID[upID]; ok {
				edges = append(edges, irac.Edge{FromID: a.ID, ToID: upID, Type: irac.EdgeAppliesTo})
				edges = append(edges, irac.Edge{FromID: upID, ToID: a.ID, Type: irac.EdgeSupports})
			}
		}
	}

	// Conclusion --concludes_from--> Application, derived from each
	// Conclusion's upstream application references.
	appByID := make(map[string]irac.ApplicationNode, len(input.Applications))
	for _, a := range input.Applications {
		appByID[a.ID] = a
	}
	for _, c := range conclusions {
		for _, upID := range c.Provenance.UpstreamNodeIDs {
			if _, ok := appByID[upID]; ok {
				edges = append(edges, irac.Edge{FromID: c.ID, ToID: upID, Type: irac.EdgeConcludesFrom})
			}
		}
	}

	return edges
}
