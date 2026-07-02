package ontology

import "github.com/YASSERRMD/verdex/packages/irac"

// ConceptLink records that a Concept was linked to an irac reasoning-tree
// node (a rule or an issue, typically) at some confidence. This package
// never imports packages/graph: a ConceptLink is a plain local struct
// that references an irac node by ID/type via the irac.NodeLike
// interface, mirroring packages/application's Origin/OriginatedRule
// pattern of wrapping irac types without depending on the graph store.
//
// A Concept is not one of the five fixed IRAC node types (Issue, Rule,
// Fact, Application, Conclusion — fixed in Phase 031 and never modified
// downstream), so it cannot be a graph.GraphStore node itself. Linking is
// therefore modeled as this package's own association record.
type ConceptLink struct {
	// ConceptID identifies the linked Concept.
	ConceptID string `json:"concept_id"`

	// NodeID is the linked irac node's ID, captured from
	// irac.NodeLike.GetID() at link time.
	NodeID string `json:"node_id"`

	// NodeType is the linked irac node's NodeType, captured from
	// irac.NodeLike.GetType() at link time.
	NodeType irac.NodeType `json:"node_type"`

	// Confidence is this link's confidence score, in the closed interval
	// [0, 1]. Mirrors packages/irac's Node.Confidence convention.
	Confidence float64 `json:"confidence"`
}

// LinkConcept constructs a ConceptLink associating concept with node,
// capturing node's ID and NodeType at call time via the irac.NodeLike
// interface so this package never needs the node's concrete type.
func LinkConcept(concept Concept, node irac.NodeLike, confidence float64) ConceptLink {
	return ConceptLink{
		ConceptID:  concept.ID,
		NodeID:     node.GetID(),
		NodeType:   node.GetType(),
		Confidence: confidence,
	}
}
