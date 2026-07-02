package application

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// PersistApplicationSubgraph persists a single built irac.ApplicationNode
// via store.CreateNode, then creates every legal edge this package's
// pipeline is responsible for around it, per
// packages/irac/edge.go's legalEdgeTriples constraint table:
//
//   - Application --applies_to--> Rule  (irac.EdgeAppliesTo)
//   - Application --applies_to--> Fact  (irac.EdgeAppliesTo, one per fact)
//   - Rule --governs--> Issue           (irac.EdgeGoverns), only if
//     ruleGovernsIssueExists is false, so a rule already linked to the
//     issue by an earlier application (e.g. a rule reused across
//     multiple ApplicationNodes for the same issue) is not re-linked.
//
// Fact --supports--> Application edges are intentionally NOT created
// here: that triple's source is a FactNode, and packages/fact's own
// PersistFacts already owns creating Fact--supports-->Application edges
// (see packages/fact/persist.go) once ApplicationNode IDs are known —
// duplicating that responsibility here would risk the two packages
// disagreeing about which facts support which applications.
//
// rule and facts must already exist in store (this function does not
// persist them — packages/statute, packages/precedent, and
// packages/fact each own persisting their own node types); ruleGovernsIssueExists
// tells this function whether the Rule--governs-->Issue edge has already
// been created by an earlier call for the same (rule, issue) pair.
//
// Returns ErrPersistFailed (wrapping the underlying store error) if any
// CreateNode/CreateEdge call fails; nodes/edges already persisted before
// the failing call are not rolled back.
func PersistApplicationSubgraph(
	ctx context.Context,
	store graph.GraphStore,
	node irac.ApplicationNode,
	rule OriginatedRule,
	issueID string,
	facts []irac.FactNode,
	ruleGovernsIssueExists bool,
) error {
	if err := store.CreateNode(ctx, node.Node); err != nil {
		return wrapPersistError(err)
	}

	if err := createLegalEdge(ctx, store, node.ID, irac.NodeApplication, rule.Rule.ID, irac.NodeRule, irac.EdgeAppliesTo); err != nil {
		return err
	}

	for _, f := range facts {
		if err := createLegalEdge(ctx, store, node.ID, irac.NodeApplication, f.ID, irac.NodeFact, irac.EdgeAppliesTo); err != nil {
			return err
		}
	}

	if !ruleGovernsIssueExists {
		if err := createLegalEdge(ctx, store, rule.Rule.ID, irac.NodeRule, issueID, irac.NodeIssue, irac.EdgeGoverns); err != nil {
			return err
		}
	}

	return nil
}

// createLegalEdge creates a single irac.Edge from (fromID, fromType) to
// (toID, toType) of the given edgeType, first checking
// irac.IsLegalEdgeTriple so this package never asks a graph.GraphStore
// to persist an edge outside packages/irac/edge.go's constraint table.
// Returns ErrIllegalEdge (not forwarded to the store at all) if the
// triple is illegal, or a wrapped ErrPersistFailed if store.CreateEdge
// itself fails.
func createLegalEdge(ctx context.Context, store graph.GraphStore, fromID string, fromType irac.NodeType, toID string, toType irac.NodeType, edgeType irac.EdgeType) error {
	if !irac.IsLegalEdgeTriple(fromType, edgeType, toType) {
		return ErrIllegalEdge
	}
	edge := irac.Edge{FromID: fromID, ToID: toID, Type: edgeType}
	if err := store.CreateEdge(ctx, edge); err != nil {
		return wrapPersistError(err)
	}
	return nil
}

// wrapPersistError wraps err with ErrPersistFailed so callers can test
// errors.Is(err, ErrPersistFailed) regardless of the underlying
// graph.GraphStore implementation's own error value, mirroring
// packages/issue/persist.go and packages/fact/persist.go's identical
// helper.
func wrapPersistError(err error) error {
	return &persistError{underlying: err}
}

// persistError implements error and errors.Unwrap so both
// ErrPersistFailed and the underlying store error can be matched via
// errors.Is.
type persistError struct {
	underlying error
}

func (e *persistError) Error() string {
	return ErrPersistFailed.Error() + ": " + e.underlying.Error()
}

func (e *persistError) Unwrap() []error {
	return []error{ErrPersistFailed, e.underlying}
}
