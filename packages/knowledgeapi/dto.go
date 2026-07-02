package knowledgeapi

import (
	"time"

	"github.com/YASSERRMD/verdex/packages/embedding"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// APIVersionV1 is the first stable knowledgeapi contract version, carried
// on every response DTO's Version field so a future breaking change can be
// introduced as APIVersionV2 without disturbing existing consumers. This
// mirrors packages/gateway's APIVersion convention.
const APIVersionV1 = "v1"

// PageRequest carries the page/per_page pagination parameters used by
// every list-returning method, mirroring packages/gateway's
// ParsePagination convention exactly (same field names, same defaulting
// and capping behaviour applied in paginate.go).
type PageRequest struct {
	// Page is the 1-based page number. Zero means "use the default page".
	Page int
	// PerPage is the page size. Zero means "use the default page size".
	PerPage int
}

// PageMeta mirrors packages/gateway's PaginationMeta shape, carried on
// every list response DTO.
type PageMeta struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// NodeDTO is knowledgeapi's own wire representation of an irac.Node,
// deliberately decoupled from irac.Node's own shape so this package's
// contract can evolve independently of the internal tree schema.
type NodeDTO struct {
	Version    string    `json:"version"`
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	CaseID     string    `json:"case_id"`
	Text       string    `json:"text"`
	Confidence float64   `json:"confidence"`
	CreatedAt  time.Time `json:"created_at"`
}

// nodeDTOFromNode converts an irac.Node into a NodeDTO.
func nodeDTOFromNode(n irac.Node) NodeDTO {
	return NodeDTO{
		Version:    APIVersionV1,
		ID:         n.ID,
		Type:       string(n.Type),
		CaseID:     n.CaseID,
		Text:       n.Text,
		Confidence: n.Confidence,
		CreatedAt:  n.CreatedAt,
	}
}

// EdgeDTO is knowledgeapi's own wire representation of an irac.Edge.
type EdgeDTO struct {
	Version string `json:"version"`
	Type    string `json:"type"`
	FromID  string `json:"from_id"`
	ToID    string `json:"to_id"`
}

// edgeDTOFromEdge converts an irac.Edge into an EdgeDTO.
func edgeDTOFromEdge(e irac.Edge) EdgeDTO {
	return EdgeDTO{
		Version: APIVersionV1,
		Type:    string(e.Type),
		FromID:  e.FromID,
		ToID:    e.ToID,
	}
}

// GetTreeRequest requests a case's full node/edge set, optionally filtered
// by node type and paginated over the node list.
type GetTreeRequest struct {
	CaseID         string
	NodeTypeFilter string
	Page           PageRequest
}

// GetTreeResponse is the paginated node/edge listing for a case's
// reasoning tree.
type GetTreeResponse struct {
	Version string    `json:"version"`
	CaseID  string    `json:"case_id"`
	Nodes   []NodeDTO `json:"nodes"`
	Edges   []EdgeDTO `json:"edges"`
	Meta    PageMeta  `json:"meta"`
}

// GetNodeRequest requests a single node by ID within a case.
type GetNodeRequest struct {
	CaseID string
	NodeID string
}

// GetNodeResponse wraps a single resolved node.
type GetNodeResponse struct {
	Version string  `json:"version"`
	Node    NodeDTO `json:"node"`
}

// LookupPathsRequest requests treeindex's materialized paths from a start
// node, following a given edge type, optionally bounded to a max depth.
type LookupPathsRequest struct {
	CaseID     string
	FromNodeID string
	EdgeType   string
	MaxDepth   int // zero means "unbounded" (LookupPaths, not LookupPathsWithDepth)
	Page       PageRequest
}

// PathHopDTO is one hop within a PathDTO.
type PathHopDTO struct {
	FromID   string `json:"from_id"`
	ToID     string `json:"to_id"`
	EdgeType string `json:"edge_type"`
}

// PathDTO is knowledgeapi's own wire representation of a treeindex.Path.
type PathDTO struct {
	Kind  string       `json:"kind"`
	Root  string       `json:"root"`
	Nodes []NodeDTO    `json:"nodes"`
	Hops  []PathHopDTO `json:"hops"`
}

// LookupPathsResponse is the paginated set of materialized paths matching
// a LookupPathsRequest.
type LookupPathsResponse struct {
	Version string    `json:"version"`
	CaseID  string    `json:"case_id"`
	Paths   []PathDTO `json:"paths"`
	Meta    PageMeta  `json:"meta"`
}

// RetrieveRequest is knowledgeapi's own DTO for a hybrid retrieval
// request, decoupled from hybridretrieval.HybridQuery's internal shape.
// Exactly one of Vector or AnchorNodeID must be set (Vector may be paired
// with AnchorNodeID for a combined semantic+structural query).
type RetrieveRequest struct {
	CaseID            string
	Vector            embedding.EmbeddingVector
	AnchorNodeID      string
	ExpansionHops     []string
	MaxExpansionDepth int
	TopK              int
	Page              PageRequest
}

// RetrievedItemDTO is knowledgeapi's own wire representation of a
// hybridretrieval.Item.
type RetrievedItemDTO struct {
	NodeID        string  `json:"node_id"`
	NodeType      string  `json:"node_type"`
	Text          string  `json:"text"`
	Path          string  `json:"path"`
	CombinedScore float64 `json:"combined_score"`
	AnchorNodeID  string  `json:"anchor_node_id,omitempty"`
	Explanation   string  `json:"explanation"`
}

// RetrieveResponse is the paginated, fused retrieval result for a
// RetrieveRequest.
type RetrieveResponse struct {
	Version            string             `json:"version"`
	CaseID             string             `json:"case_id"`
	Items              []RetrievedItemDTO `json:"items"`
	VectorHitCount     int                `json:"vector_hit_count"`
	ExpansionSeedCount int                `json:"expansion_seed_count"`
	ExpansionSkipped   bool               `json:"expansion_skipped"`
	ExpansionTruncated bool               `json:"expansion_truncated"`
	Meta               PageMeta           `json:"meta"`
}

// ResolveCitationRequest requests citation resolution and verification for
// a single node within a case.
type ResolveCitationRequest struct {
	CaseID string
	NodeID string
}

// CitationDTO is knowledgeapi's own wire representation of a resolved,
// verified citation.
type CitationDTO struct {
	Version            string  `json:"version"`
	NodeID             string  `json:"node_id"`
	CaseID             string  `json:"case_id"`
	Citation           string  `json:"citation"`
	Origin             string  `json:"origin"`
	Certainty          string  `json:"certainty"`
	VerificationStatus string  `json:"verification_status"`
	Verified           bool    `json:"verified"`
	ConfidenceScore    float64 `json:"confidence_score"`
}

// ResolveCitationResponse wraps a single resolved citation.
type ResolveCitationResponse struct {
	Version  string      `json:"version"`
	Citation CitationDTO `json:"citation"`
}

// ValidationStatusRequest requests the current integrity/validation status
// of a case's assembled tree.
type ValidationStatusRequest struct {
	CaseID string
}

// FindingDTO is knowledgeapi's own wire representation of a
// treevalidation.Finding.
type FindingDTO struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	NodeID   string `json:"node_id,omitempty"`
}

// ValidationStatusResponse reports whether a case's tree can be finalized
// per treevalidation.CanFinalize, plus the full Finding list.
type ValidationStatusResponse struct {
	Version     string       `json:"version"`
	CaseID      string       `json:"case_id"`
	CanFinalize bool         `json:"can_finalize"`
	Summary     string       `json:"summary"`
	Findings    []FindingDTO `json:"findings"`
}
