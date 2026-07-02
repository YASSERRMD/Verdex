package irac

import "time"

// NodeType classifies which position in the Issue-Rule-Application-
// Conclusion (IRAC) reasoning tree a Node occupies. This mirrors
// packages/evidence's EvidenceType convention of a small string-backed
// enum with one constant per recognized kind.
type NodeType string

const (
	// NodeIssue identifies a legal or factual question the reasoning tree
	// must resolve.
	NodeIssue NodeType = "issue"

	// NodeRule identifies a legal rule, statute, or precedent invoked to
	// resolve an Issue.
	NodeRule NodeType = "rule"

	// NodeFact identifies a factual assertion drawn from the case record.
	NodeFact NodeType = "fact"

	// NodeApplication identifies the reasoning step that applies a Rule to
	// a set of Facts.
	NodeApplication NodeType = "application"

	// NodeConclusion identifies the outcome reasoned from an Application.
	// Every ConclusionNode carries the mandatory draft_analysis label (see
	// guardrail.go) — this package never produces verdict or directive
	// language.
	NodeConclusion NodeType = "conclusion"
)

// allNodeTypes is the exhaustive set of recognized NodeType values, used by
// IsValid and by validate.go's constraint table.
var allNodeTypes = map[NodeType]struct{}{
	NodeIssue:       {},
	NodeRule:        {},
	NodeFact:        {},
	NodeApplication: {},
	NodeConclusion:  {},
}

// IsValid reports whether t is one of the recognized NodeType constants.
func (t NodeType) IsValid() bool {
	_, ok := allNodeTypes[t]
	return ok
}

// AllNodeTypes returns every recognized NodeType, in the fixed order
// declared above (NodeIssue first, NodeConclusion last).
func AllNodeTypes() []NodeType {
	return []NodeType{
		NodeIssue,
		NodeRule,
		NodeFact,
		NodeApplication,
		NodeConclusion,
	}
}

// Node is the common shape shared by every node in an IRAC reasoning tree,
// regardless of its NodeType. Concrete typed wrappers (IssueNode, RuleNode,
// FactNode, ApplicationNode, ConclusionNode) embed Node and add
// type-specific fields.
type Node struct {
	// ID uniquely identifies this node within its case's reasoning tree.
	ID string `json:"id"`

	// Type identifies which IRAC position this node occupies.
	Type NodeType `json:"type"`

	// CaseID identifies the case this node belongs to.
	CaseID string `json:"case_id"`

	// Text is the human-readable content of this node (the issue
	// question, the rule statement, the fact assertion, the application
	// reasoning, or the conclusion text).
	Text string `json:"text"`

	// CreatedAt is the timestamp this node was created.
	CreatedAt time.Time `json:"created_at"`
}

// GetID returns the node's ID. Implements the NodeLike interface so
// heterogeneous concrete node types can be handled uniformly.
func (n Node) GetID() string { return n.ID }

// GetType returns the node's NodeType. Implements the NodeLike interface.
func (n Node) GetType() NodeType { return n.Type }

// NodeLike is implemented by Node and every concrete typed wrapper
// (IssueNode, RuleNode, FactNode, ApplicationNode, ConclusionNode),
// letting callers (e.g. ValidateTree) operate over a heterogeneous
// collection of tree nodes via their common ID/Type without needing to
// know the concrete type.
type NodeLike interface {
	GetID() string
	GetType() NodeType
}

// IssueNode is a Node of Type NodeIssue: a legal or factual question the
// reasoning tree must resolve.
type IssueNode struct {
	Node
}

// NewIssueNode constructs an IssueNode with Type fixed to NodeIssue.
func NewIssueNode(id, caseID, text string, createdAt time.Time) IssueNode {
	return IssueNode{Node: Node{
		ID:        id,
		Type:      NodeIssue,
		CaseID:    caseID,
		Text:      text,
		CreatedAt: createdAt,
	}}
}

// RuleNode is a Node of Type NodeRule: a legal rule, statute, or precedent
// invoked to resolve an Issue. RuleNode additionally tags the jurisdiction
// and legal family it derives from (see jurisdiction.go).
type RuleNode struct {
	Node

	// JurisdictionCode identifies the jurisdiction this rule derives its
	// authority from. Opaque string here (no hard dependency on
	// packages/jurisdiction) — see jurisdiction.go.
	JurisdictionCode string `json:"jurisdiction_code"`

	// LegalFamily classifies the legal tradition this rule derives from
	// (e.g. "common_law", "civil_law"). Opaque string here (no hard
	// dependency on packages/jurisdiction) — see jurisdiction.go.
	LegalFamily string `json:"legal_family"`
}

// NewRuleNode constructs a RuleNode with Type fixed to NodeRule.
func NewRuleNode(id, caseID, text, jurisdictionCode, legalFamily string, createdAt time.Time) RuleNode {
	return RuleNode{
		Node: Node{
			ID:        id,
			Type:      NodeRule,
			CaseID:    caseID,
			Text:      text,
			CreatedAt: createdAt,
		},
		JurisdictionCode: jurisdictionCode,
		LegalFamily:      legalFamily,
	}
}

// FactNode is a Node of Type NodeFact: a factual assertion drawn from the
// case record.
type FactNode struct {
	Node
}

// NewFactNode constructs a FactNode with Type fixed to NodeFact.
func NewFactNode(id, caseID, text string, createdAt time.Time) FactNode {
	return FactNode{Node: Node{
		ID:        id,
		Type:      NodeFact,
		CaseID:    caseID,
		Text:      text,
		CreatedAt: createdAt,
	}}
}

// ApplicationNode is a Node of Type NodeApplication: the reasoning step
// that applies a Rule to a set of Facts.
type ApplicationNode struct {
	Node
}

// NewApplicationNode constructs an ApplicationNode with Type fixed to
// NodeApplication.
func NewApplicationNode(id, caseID, text string, createdAt time.Time) ApplicationNode {
	return ApplicationNode{Node: Node{
		ID:        id,
		Type:      NodeApplication,
		CaseID:    caseID,
		Text:      text,
		CreatedAt: createdAt,
	}}
}

// ConclusionNode is a Node of Type NodeConclusion: the outcome reasoned
// from an Application. ConclusionNode is deliberately NOT constructible as
// a bare struct literal from outside this package in normal usage — always
// use NewConclusionNode (guardrail.go), which unconditionally attaches the
// mandatory draft_analysis Label per CONTRIBUTING.md's non-binding
// guardrail.
type ConclusionNode struct {
	Node

	// Label is the mandatory non-binding-guardrail label. Always equals
	// DraftAnalysisLabel (see guardrail.go). Verdict or directive language
	// is rejected by construction: NewConclusionNode is the only exported
	// way to build a ConclusionNode with this field set correctly.
	Label string `json:"label"`
}
