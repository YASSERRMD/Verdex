package irac

// EdgeType classifies how two nodes in the IRAC reasoning tree relate to
// one another. This mirrors NodeType's string-backed enum convention.
type EdgeType string

const (
	// EdgeGoverns links a RuleNode to the IssueNode it governs
	// (Rule -> Issue).
	EdgeGoverns EdgeType = "governs"

	// EdgeAppliesTo links an ApplicationNode to the FactNode or RuleNode
	// it applies (Application -> Fact, Application -> Rule).
	EdgeAppliesTo EdgeType = "applies_to"

	// EdgeSupports links a FactNode to the ApplicationNode it supports
	// (Fact -> Application).
	EdgeSupports EdgeType = "supports"

	// EdgeConcludesFrom links a ConclusionNode to the ApplicationNode it
	// was reasoned from (Conclusion -> Application).
	EdgeConcludesFrom EdgeType = "concludes_from"
)

// allEdgeTypes is the exhaustive set of recognized EdgeType values, used
// by IsValid.
var allEdgeTypes = map[EdgeType]struct{}{
	EdgeGoverns:       {},
	EdgeAppliesTo:     {},
	EdgeSupports:      {},
	EdgeConcludesFrom: {},
}

// IsValid reports whether t is one of the recognized EdgeType constants.
func (t EdgeType) IsValid() bool {
	_, ok := allEdgeTypes[t]
	return ok
}

// AllEdgeTypes returns every recognized EdgeType, in the fixed order
// declared above (EdgeGoverns first, EdgeConcludesFrom last).
func AllEdgeTypes() []EdgeType {
	return []EdgeType{
		EdgeGoverns,
		EdgeAppliesTo,
		EdgeSupports,
		EdgeConcludesFrom,
	}
}

// Edge is a directed relationship between two nodes in the IRAC reasoning
// tree.
type Edge struct {
	// FromID is the ID of the source node.
	FromID string `json:"from_id"`

	// ToID is the ID of the target node.
	ToID string `json:"to_id"`

	// Type identifies the kind of relationship this edge represents.
	Type EdgeType `json:"type"`
}

// edgeTriple identifies one legal (source node type, edge type, target
// node type) combination.
type edgeTriple struct {
	From NodeType
	Edge EdgeType
	To   NodeType
}

// legalEdgeTriples is the constraint table declaring every (FromNodeType,
// EdgeType, ToNodeType) combination that is legal in an IRAC reasoning
// tree:
//
//   - Rule --governs--> Issue
//   - Application --applies_to--> Fact
//   - Application --applies_to--> Rule
//   - Fact --supports--> Application
//   - Conclusion --concludes_from--> Application
//
// ValidateTree (validate.go) rejects any edge whose triple is not present
// here.
var legalEdgeTriples = map[edgeTriple]struct{}{
	{From: NodeRule, Edge: EdgeGoverns, To: NodeIssue}:                   {},
	{From: NodeApplication, Edge: EdgeAppliesTo, To: NodeFact}:           {},
	{From: NodeApplication, Edge: EdgeAppliesTo, To: NodeRule}:           {},
	{From: NodeFact, Edge: EdgeSupports, To: NodeApplication}:            {},
	{From: NodeConclusion, Edge: EdgeConcludesFrom, To: NodeApplication}: {},
}

// IsLegalEdgeTriple reports whether an edge of type edgeType from a node
// of type fromType to a node of type toType is permitted by the
// constraint table.
func IsLegalEdgeTriple(fromType NodeType, edgeType EdgeType, toType NodeType) bool {
	_, ok := legalEdgeTriples[edgeTriple{From: fromType, Edge: edgeType, To: toType}]
	return ok
}

// LegalEdgeTriples returns every legal (FromNodeType, EdgeType,
// ToNodeType) triple in the constraint table. Useful for tests and
// documentation generation.
func LegalEdgeTriples() []struct {
	From NodeType
	Edge EdgeType
	To   NodeType
} {
	out := make([]struct {
		From NodeType
		Edge EdgeType
		To   NodeType
	}, 0, len(legalEdgeTriples))
	for triple := range legalEdgeTriples {
		out = append(out, struct {
			From NodeType
			Edge EdgeType
			To   NodeType
		}{From: triple.From, Edge: triple.Edge, To: triple.To})
	}
	return out
}
