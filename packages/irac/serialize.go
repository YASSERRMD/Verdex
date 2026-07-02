package irac

import (
	"encoding/json"
	"fmt"
)

// treeEnvelopeVersion is the schema version stamped on every marshaled
// tree envelope. Bump this if the envelope's on-disk shape changes in a
// backward-incompatible way.
const treeEnvelopeVersion = 1

// nodeEnvelope is the stable, self-describing JSON shape for a single
// node inside a tree envelope: a NodeType discriminator ("kind") plus the
// node's own JSON representation nested under "node". Encoding this way
// (rather than relying on Go's type system) keeps the format independent
// of any particular language's discriminated-union support, and is
// lossless because every concrete node type's fields round-trip through
// its own struct's json tags unchanged.
type nodeEnvelope struct {
	Kind NodeType        `json:"kind"`
	Node json.RawMessage `json:"node"`
}

// TreeEnvelope is the stable JSON envelope produced by MarshalTree and
// consumed by UnmarshalTree: a case's full IRAC reasoning tree at one
// revision, including every node (heterogeneous by NodeType), every edge,
// and the revision metadata itself.
type TreeEnvelope struct {
	Version  int            `json:"version"`
	Revision TreeRevision   `json:"revision"`
	Nodes    []nodeEnvelope `json:"nodes"`
	Edges    []Edge         `json:"edges"`
}

// MarshalTree encodes nodes, edges, and revision into a stable JSON
// envelope. Each node is tagged with its NodeType so UnmarshalTree can
// reconstruct the correct concrete Go type on the way back in. Returns an
// error if any node's concrete type does not match its NodeLike.GetType()
// (e.g. a NodeConclusion-tagged value that is not actually a
// ConclusionNode), since that combination could never have been produced
// by this package's constructors and would not round-trip correctly.
func MarshalTree(nodes []NodeLike, edges []Edge, revision TreeRevision) ([]byte, error) {
	envelopes := make([]nodeEnvelope, 0, len(nodes))
	for _, n := range nodes {
		raw, err := marshalNode(n)
		if err != nil {
			return nil, err
		}
		envelopes = append(envelopes, nodeEnvelope{Kind: n.GetType(), Node: raw})
	}

	env := TreeEnvelope{
		Version:  treeEnvelopeVersion,
		Revision: revision,
		Nodes:    envelopes,
		Edges:    edges,
	}
	return json.Marshal(env)
}

// marshalNode encodes a single NodeLike value as JSON, verifying its
// concrete Go type agrees with its declared NodeType.
func marshalNode(n NodeLike) (json.RawMessage, error) {
	switch n.GetType() {
	case NodeIssue:
		v, ok := n.(IssueNode)
		if !ok {
			return nil, fmt.Errorf("irac: node %q declares type %q but is not an IssueNode", n.GetID(), n.GetType())
		}
		return json.Marshal(v)
	case NodeRule:
		v, ok := n.(RuleNode)
		if !ok {
			return nil, fmt.Errorf("irac: node %q declares type %q but is not a RuleNode", n.GetID(), n.GetType())
		}
		return json.Marshal(v)
	case NodeFact:
		v, ok := n.(FactNode)
		if !ok {
			return nil, fmt.Errorf("irac: node %q declares type %q but is not a FactNode", n.GetID(), n.GetType())
		}
		return json.Marshal(v)
	case NodeApplication:
		v, ok := n.(ApplicationNode)
		if !ok {
			return nil, fmt.Errorf("irac: node %q declares type %q but is not an ApplicationNode", n.GetID(), n.GetType())
		}
		return json.Marshal(v)
	case NodeConclusion:
		v, ok := n.(ConclusionNode)
		if !ok {
			return nil, fmt.Errorf("irac: node %q declares type %q but is not a ConclusionNode", n.GetID(), n.GetType())
		}
		if !v.HasGuardrailLabel() {
			return nil, fmt.Errorf("%w: node %q", ErrMissingGuardrailLabel, n.GetID())
		}
		return json.Marshal(v)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownNodeType, n.GetType())
	}
}

// UnmarshalTree decodes a JSON envelope produced by MarshalTree back into
// its nodes, edges, and revision metadata. Each node is reconstructed as
// its correct concrete Go type (IssueNode, RuleNode, FactNode,
// ApplicationNode, or ConclusionNode) based on the envelope's "kind"
// discriminator. Returns ErrMissingGuardrailLabel if a decoded
// ConclusionNode is missing its draft_analysis label, and
// ErrUnknownNodeType if an envelope's "kind" is not recognized.
func UnmarshalTree(data []byte) (nodes []NodeLike, edges []Edge, revision TreeRevision, err error) {
	var env TreeEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, nil, TreeRevision{}, err
	}

	out := make([]NodeLike, 0, len(env.Nodes))
	for _, ne := range env.Nodes {
		n, err := unmarshalNode(ne)
		if err != nil {
			return nil, nil, TreeRevision{}, err
		}
		out = append(out, n)
	}

	return out, env.Edges, env.Revision, nil
}

// unmarshalNode decodes a single nodeEnvelope into its correct concrete
// NodeLike Go type based on its Kind discriminator.
func unmarshalNode(ne nodeEnvelope) (NodeLike, error) {
	switch ne.Kind {
	case NodeIssue:
		var v IssueNode
		if err := json.Unmarshal(ne.Node, &v); err != nil {
			return nil, err
		}
		return v, nil
	case NodeRule:
		var v RuleNode
		if err := json.Unmarshal(ne.Node, &v); err != nil {
			return nil, err
		}
		return v, nil
	case NodeFact:
		var v FactNode
		if err := json.Unmarshal(ne.Node, &v); err != nil {
			return nil, err
		}
		return v, nil
	case NodeApplication:
		var v ApplicationNode
		if err := json.Unmarshal(ne.Node, &v); err != nil {
			return nil, err
		}
		return v, nil
	case NodeConclusion:
		var v ConclusionNode
		if err := json.Unmarshal(ne.Node, &v); err != nil {
			return nil, err
		}
		if !v.HasGuardrailLabel() {
			return nil, fmt.Errorf("%w: node %q", ErrMissingGuardrailLabel, v.GetID())
		}
		return v, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownNodeType, ne.Kind)
	}
}
