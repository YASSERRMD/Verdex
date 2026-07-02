package irac

import "time"

// Provenance records how a node came to exist: which upstream process
// generated it, when, and from which other nodes (if any) it was derived.
// Attached to every concrete node type alongside a Confidence score, so
// every claim in the reasoning tree is both scored and traceable back to
// its generating process — mirroring packages/timeline's Event.Confidence
// convention, extended with a fuller provenance record.
type Provenance struct {
	// GeneratedBy identifies the process, model, or rule engine that
	// produced this node (e.g. "irac-issue-extractor-v1",
	// "human-reviewer"). No hard dependency on packages/provider — this is
	// an opaque label, not a live provider binding.
	GeneratedBy string `json:"generated_by"`

	// GeneratedAt is the timestamp this node's content was generated.
	GeneratedAt time.Time `json:"generated_at"`

	// UpstreamNodeIDs lists the IDs of nodes this node was derived from,
	// when applicable (e.g. an ApplicationNode's UpstreamNodeIDs might
	// list the FactNode and RuleNode IDs it was built from). Empty when
	// this node was not derived from other nodes in the tree (e.g. a Fact
	// extracted directly from source text).
	UpstreamNodeIDs []string `json:"upstream_node_ids,omitempty"`
}

// IsValid reports whether p has a non-empty GeneratedBy and a non-zero
// GeneratedAt.
func (p Provenance) IsValid() bool {
	return p.GeneratedBy != "" && !p.GeneratedAt.IsZero()
}

// ValidConfidence reports whether c lies in the closed interval [0, 1],
// mirroring packages/timeline's Event.Confidence convention.
func ValidConfidence(c float64) bool {
	return c >= 0 && c <= 1
}
